package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestTaskRetriesPreserveKeyPayloadAndAcceptedIdentity(t *testing.T) {
	var mu sync.Mutex
	mutations := 0
	requests := 0
	saved := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		requests++
		if r.Method != "POST" || r.URL.Path != "/api/v1/admin/tasks" || r.Header.Get("Idempotency-Key") != "logical-operation" || r.Header.Get("Authorization") != "Bearer test-pat" {
			t.Error("incorrect HTTP task contract")
		}
		b, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(b, &payload)
		if payload["tenantId"] != "tenant-a" || payload["action"] != "storage.create" {
			t.Error("incorrect task payload")
		}
		if saved == "" {
			saved = string(b)
			mutations++
		} else if saved != string(b) {
			t.Error("retry changed payload")
		}
		w.WriteHeader(202)
		_, _ = w.Write([]byte(`{"accepted":true,"taskId":"same-task","delivery":"pending"}`))
	}))
	defer server.Close()
	client := NewClient(server.URL+"/", "test-pat", false)
	client.IdempotencyKey = "logical-operation"
	data := StorageCreateData{Name: "disk", ImageID: "image", SizeMB: 1024}
	first, err := client.CreateStorageTask("tenant-a", data)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := client.CreateStorageTask("tenant-a", data)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(replay) || mutations != 1 || requests != 2 {
		t.Fatal("unstable retry identity")
	}
}

func TestRedirectDoesNotForwardCredentialsOrReplayLogin(t *testing.T) {
	forwarded := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			http.Redirect(w, r, "/stolen", http.StatusTemporaryRedirect)
			return
		}
		forwarded++
	}))
	defer server.Close()
	client := NewClient(server.URL, "test-pat", false)
	if _, err := client.Login("/login", LoginRequest{Email: "test@example.test", Password: "test-password"}); err == nil {
		t.Fatal("redirect accepted")
	}
	if forwarded != 0 {
		t.Fatal("credentials forwarded on redirect")
	}
}

func TestInvalidServerAndTenantPayloadAreRejected(t *testing.T) {
	for _, base := range []string{"file:///tmp/socket", "https://user:password@example.test", "https://example.test?token=secret"} {
		if _, err := NewClient(base, "pat", false).requestURL("/api/v1/tenant/tasks"); err == nil {
			t.Fatalf("accepted %q", base)
		}
	}
	c := NewClient("https://example.test", "pat", false)
	c.IdempotencyKey = "key"
	if _, err := c.SubmitTask("tenant", json.RawMessage(`{"tenantId":"foreign","action":"echo"}`)); err == nil {
		t.Fatal("tenant override accepted")
	}
}

func TestDeleteVMRejectsDisplayNamesAndConflictingGUIDs(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", "pat", false)
	client.IdempotencyKey = "key"
	for _, data := range []struct{ ref, guid string }{{"display-name", "display-name"}, {"11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"}} {
		_, err := client.DeleteVMTask("tenant-a", "host", data.ref, VMDeleteData{GUID: data.guid})
		if err == nil || !strings.Contains(err.Error(), "canonical Hyper-V GUID") {
			t.Fatal("unsafe target accepted", err)
		}
	}
}

func TestResponseSizeIsBounded(t *testing.T) {
	if _, err := readResponse(strings.NewReader(strings.Repeat("x", (16<<20)+1))); err == nil {
		t.Fatal("oversized response accepted")
	}
}

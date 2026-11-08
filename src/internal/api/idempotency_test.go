package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAutomaticKeysRecoverUncertainSubmissionAcrossClients(t *testing.T) {
	pendingTestHome(t)
	var keys, bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(body))
		if len(keys) == 1 {
			http.Error(w, "uncertain", 503)
			return
		}
		w.WriteHeader(202)
		io.WriteString(w, `{"taskId":"accepted"}`)
	}))
	defer server.Close()
	data := StorageCreateData{Name: "disk", ImageID: "image", SizeMB: 1024}
	for i := 0; i < 3; i++ {
		_, err := NewClient(server.URL, "private-token", false).CreateStorageTask("tenant", data)
		if (i == 0) != (err != nil) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	if len(keys[0]) != 64 || keys[0] != keys[1] || keys[1] == keys[2] {
		t.Fatalf("incorrect operation identity lifecycle: %v", keys)
	}
	if bodies[0] != bodies[1] {
		t.Fatal("recovery changed payload")
	}
}

func TestPendingIdentityScopeAndNoPlaintextSecrets(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("APPDATA", dir)
	first, _, err := pendingTaskKey("https://api/tasks", "secret-token", []byte(`{"password":"secret-body"}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range []struct{ endpoint, credential, body string }{
		{"https://other/tasks", "secret-token", `{"password":"secret-body"}`},
		{"https://api/tasks", "other-token", `{"password":"secret-body"}`},
		{"https://api/tasks", "secret-token", `{"password":"different"}`},
	} {
		key, _, err := pendingTaskKey(scope.endpoint, scope.credential, []byte(scope.body))
		if err != nil || key == first {
			t.Fatal("scope collision", err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(dir, "hvx", "pending-tasks"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".lock") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, "hvx", "pending-tasks", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "secret") {
			t.Fatal("secret persisted")
		}
		info, _ := e.Info()
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
			t.Fatal("record is not private")
		}
	}
}

func TestIncompleteSuccessResponseKeepsIdentity(t *testing.T) {
	pendingTestHome(t)
	var keys []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		if len(keys) == 1 {
			w.Header().Set("Content-Length", "1000")
		}
		w.WriteHeader(202)
		io.WriteString(w, `{"taskId":"accepted"}`)
	}))
	defer server.Close()
	data := StorageCreateData{Name: "disk", ImageID: "image", SizeMB: 1024}
	if _, err := NewClient(server.URL, "pat", false).CreateStorageTask("tenant", data); err == nil {
		t.Fatal("truncated response accepted")
	}
	if _, err := NewClient(server.URL, "pat", false).CreateStorageTask("tenant", data); err != nil {
		t.Fatal(err)
	}
	if keys[0] != keys[1] {
		t.Fatal("lost pending identity after incomplete acceptance response")
	}
}

func TestStaleCleanupCannotRemoveNewOperation(t *testing.T) {
	pendingTestHome(t)
	body := []byte(`{"action":"echo"}`)
	key, cleanup, err := pendingTaskKey("https://api/tasks", "pat", body)
	if err != nil {
		t.Fatal(err)
	}
	_, staleCleanup, err := pendingTaskKey("https://api/tasks", "pat", body)
	if err != nil {
		t.Fatal(err)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	next, _, err := pendingTaskKey("https://api/tasks", "pat", body)
	if err != nil || next == key {
		t.Fatal("missing fresh identity", err)
	}
	if err := staleCleanup(); err != nil {
		t.Fatal(err)
	}
	recovered, _, err := pendingTaskKey("https://api/tasks", "pat", body)
	if err != nil || recovered != next {
		t.Fatal("old response removed new pending identity", err)
	}
}

func pendingTestHome(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("APPDATA", dir)
}

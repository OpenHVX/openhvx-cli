package cli

import (
	"bytes"
	"encoding/json"
	"hvx-cli/internal/config"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"hvx-cli/internal/requests"
)

func TestRetryUsesSavedContentAndKeyWithoutResubmittingAcceptedRequest(t *testing.T) {
	var keys, bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw json.RawMessage
		_ = json.NewDecoder(r.Body).Decode(&raw)
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		bodies = append(bodies, string(raw))
		if len(keys) == 1 {
			http.Error(w, "uncertain", 503)
			return
		}
		w.WriteHeader(202)
		_, _ = w.Write([]byte(`{"taskId":"task-a","accepted":true}`))
	}))
	defer server.Close()
	cfg := testConfig(t, server.URL, "tenant")
	var out, stderr bytes.Buffer
	a := NewApp(cfg, &out, &stderr)
	path := taskFile(t, `{"action":"echo","target":{"kind":"vm"},"data":{"message":"original"}}`)
	if err := a.Execute([]string{"create", "-f", path}); err == nil {
		t.Fatal("expected uncertain submission")
	}
	records, err := requests.List()
	if err != nil || len(records) != 1 {
		t.Fatal("missing saved request", err)
	}
	id := records[0].ID
	if err := os.WriteFile(path, []byte(`{"action":"echo","data":{"message":"changed"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := a.Execute([]string{"retry", id}); err != nil {
		t.Fatal(err)
	}
	if keys[0] != keys[1] || bodies[0] != bodies[1] || !strings.Contains(bodies[1], "original") {
		t.Fatal("retry changed immutable request")
	}
	if err := a.Execute([]string{"retry", id}); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Fatal("accepted request sent again")
	}
	if !strings.Contains(stderr.String(), id) {
		t.Fatal("request identity not displayed before uncertainty")
	}
	cfg.SetToken("tenant", "other-principal")
	if err := a.Execute([]string{"retry", id}); err == nil {
		t.Fatal("request crossed principals")
	}
}

func TestPowerResolvesNamesAndRefusesAmbiguityBeforePosting(t *testing.T) {
	posts := 0
	ambiguous := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			list := []map[string]string{{"kind": "vm", "name": "web01", "agentId": "host-1", "refId": "11111111-1111-4111-8111-111111111111"}}
			if ambiguous {
				list = append(list, map[string]string{"kind": "vm", "name": "web01", "agentId": "host-2", "refId": "22222222-2222-4222-8222-222222222222"})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": list})
			return
		}
		posts++
		var payload struct {
			Action string
			Target struct {
				RefID   string
				AgentID string
			}
			Data struct{ State string }
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if payload.Action != "vm.power" || payload.Data.State != "start" || payload.Target.RefID != "11111111-1111-4111-8111-111111111111" || payload.Target.AgentID != "host-1" {
			t.Error("incorrect resolved command")
		}
		_, _ = w.Write([]byte(`{"taskId":"power-task"}`))
	}))
	defer server.Close()
	a := NewApp(testConfig(t, server.URL, "tenant"), &bytes.Buffer{}, &bytes.Buffer{})
	if err := a.Execute([]string{"start", "vm", "web01"}); err != nil {
		t.Fatal(err)
	}
	ambiguous = true
	if err := a.Execute([]string{"start", "vm", "web01"}); err == nil || posts != 1 {
		t.Fatal("ambiguous name mutated server")
	}
	if err := a.Execute([]string{"start", "vm", "web01", "--agent-id", "host-1"}); err != nil || posts != 2 {
		t.Fatal("agent disambiguation failed", err)
	}
}

func TestSubmissionWaitsAndFreshCommandCreatesFreshIdentity(t *testing.T) {
	var keys []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			keys = append(keys, r.Header.Get("Idempotency-Key"))
			_, _ = w.Write([]byte(`{"taskId":"task-a"}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"taskId":"task-a","status":"done"}}`))
	}))
	defer server.Close()
	a := NewApp(testConfig(t, server.URL, "tenant"), &bytes.Buffer{}, &bytes.Buffer{})
	path := taskFile(t, `{"action":"echo","target":{"kind":"vm"},"data":{}}`)
	for i := 0; i < 2; i++ {
		if err := a.Execute([]string{"create", "-f", path, "--wait", "--timeout", "1s"}); err != nil {
			t.Fatal(err)
		}
	}
	if len(keys) != 2 || keys[0] == keys[1] {
		t.Fatal("new command reused old operation identity")
	}
}

func TestReceiptSurvivesProcessLossAfterServerAcceptance(t *testing.T) {
	if os.Getenv("HVX_RECEIPT_CRASH_HELPER") == "1" {
		cfg, err := config.Load()
		if err != nil {
			os.Exit(2)
		}
		app := NewApp(cfg, io.Discard, io.Discard)
		_ = app.Execute([]string{"create", "-f", os.Getenv("HVX_RECEIPT_TASK_FILE")})
		os.Exit(3)
	}
	received := make(chan struct{}, 1)
	var mu sync.Mutex
	var keys []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if !json.Valid(raw) {
			t.Error("invalid request")
		}
		mu.Lock()
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		n := len(keys)
		mu.Unlock()
		if n == 1 {
			received <- struct{}{}
			<-r.Context().Done()
			return
		}
		_, _ = w.Write([]byte(`{"accepted":true,"taskId":"accepted-before-crash"}`))
	}))
	defer server.Close()
	cfg := testConfig(t, server.URL, "tenant")
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	path := taskFile(t, `{"action":"echo","target":{"kind":"vm"},"data":{}}`)
	child := exec.Command(os.Args[0], "-test.run=^TestReceiptSurvivesProcessLossAfterServerAcceptance$")
	child.Env = append(os.Environ(), "HVX_RECEIPT_CRASH_HELPER=1", "HVX_RECEIPT_TASK_FILE="+path)
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer child.Process.Kill()
	select {
	case <-received:
	case <-time.After(10 * time.Second):
		t.Fatal("child did not reach server")
	}
	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = child.Wait()
	records, err := requests.List()
	if err != nil || len(records) != 1 || records[0].State != "uncertain" {
		t.Fatal("lost uncertain request", err)
	}
	app := NewApp(cfg, io.Discard, io.Discard)
	if err := app.Execute([]string{"retry", records[0].ID}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(keys) != 2 || keys[0] != keys[1] {
		t.Fatal("process restart changed accepted operation identity")
	}
}

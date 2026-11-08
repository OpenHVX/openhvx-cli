package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hvx-cli/internal/config"
)

func testConfig(server, mode string) *config.Config {
	cfg := &config.Config{}
	cfg.SetMode(mode)
	cfg.SetTenantServer(server)
	cfg.SetAdminServer(server)
	cfg.SetCurrentTenant("tenant-a")
	cfg.SetToken("tenant", "tenant-pat")
	cfg.SetToken("admin", "admin-pat")
	return cfg
}
func taskFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "task.json")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestGenericSubmissionUsesTenantHTTPContract(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/v1/tenant/tasks" || r.Header.Get("Authorization") != "Bearer tenant-pat" || r.Header.Get("Idempotency-Key") != "operation-key" {
			t.Error("wrong route or credentials")
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["contractVersion"] != nil || body["action"] != "vm.power" {
			t.Error("CLI must submit HTTP body, not AMQP envelope")
		}
		w.WriteHeader(202)
		_, _ = w.Write([]byte(`{"accepted":true,"taskId":"task-a","delivery":"pending"}`))
	}))
	defer server.Close()
	path := taskFile(t, `{"action":"vm.power","target":{"kind":"vm","agentId":"host","refId":"11111111-1111-4111-8111-111111111111"},"data":{"state":"start"}}`)
	var out, stderr bytes.Buffer
	a := NewApp(testConfig(server.URL, "tenant"), &out, &stderr)
	if err := a.Execute([]string{"--idempotency-key", "operation-key", "-o", "json", "create", "task", "--file", path}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || !strings.Contains(out.String(), `"taskId":"task-a"`) || strings.Contains(out.String(), "tenant-pat") {
		t.Fatal("invalid output")
	}
	if err := a.Execute([]string{"create", "task", "--file", path}); err == nil || calls != 1 {
		t.Fatal("missing key or stale key allowed submission")
	}
}

func TestGenericAdminSubmissionResolvesTenantAndRejectsConflict(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/admin/tasks" || r.Header.Get("Authorization") != "Bearer admin-pat" {
			t.Error("incorrect admin scope")
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["tenantId"] != "tenant-a" {
			t.Error("missing admin tenant")
		}
		_, _ = w.Write([]byte(`{"taskId":"task-a"}`))
	}))
	defer server.Close()
	a := NewApp(testConfig(server.URL, "admin"), &bytes.Buffer{}, &bytes.Buffer{})
	if err := a.Execute([]string{"--idempotency-key=key", "create", "task", "--file", taskFile(t, `{"action":"echo","target":{"kind":"vm"},"data":{}}`)}); err != nil {
		t.Fatal(err)
	}
	err := a.Execute([]string{"--idempotency-key=key", "create", "task", "--tenant", "tenant-b", "--file", taskFile(t, `{"action":"echo","tenantId":"tenant-a"}`)})
	if err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatal("conflicting tenant accepted")
	}
}

func TestTaskDecodingSupportsEnvelopeAndDirectRecord(t *testing.T) {
	for _, raw := range []string{`{"taskId":"task-a","status":"reconciling","outcome":"unknown","data":{"target":{"kind":"vm"}}}`, `{"success":true,"data":{"taskId":"task-a","status":"reconciling","outcome":"unknown","data":{"target":{"kind":"vm"}}}}`} {
		_, task, err := decodeTaskResponse([]byte(raw))
		if err != nil || task.TaskID != "task-a" || task.Outcome != "unknown" {
			t.Fatal("invalid task decoding", err)
		}
	}
}

func TestDescribeTaskSupportsTenantModeAndContextHidesTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tenant/tasks/task-a" {
			t.Error("wrong describe route")
		}
		_, _ = w.Write([]byte(`{"data":{"taskId":"task-a","status":"done"}}`))
	}))
	defer server.Close()
	var out bytes.Buffer
	a := NewApp(testConfig(server.URL, "tenant"), &out, &bytes.Buffer{})
	if err := a.Execute([]string{"describe", "task", "task-a"}); err != nil {
		t.Fatal(err)
	}
	if err := a.Execute([]string{"context", "show"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "tenant-pat") || strings.Contains(out.String(), "admin-pat") {
		t.Fatal("context exposed credentials")
	}
}

func TestWaitPollsWithoutResubmittingAndDoesNotTreatReconciliationAsSuccess(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" {
			t.Error("wait mutated server")
		}
		status := "reconciling"
		if calls > 1 {
			status = "done"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"taskId": "task-a", "status": status}})
	}))
	defer server.Close()
	var out bytes.Buffer
	a := NewApp(testConfig(server.URL, "tenant"), &out, &bytes.Buffer{})
	if err := a.Execute([]string{"wait", "task", "task-a", "--timeout", "1s", "--interval", "1ms"}); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || !strings.Contains(out.String(), "done") {
		t.Fatal("wait terminated on uncertain state")
	}
}

func TestWaitDeadlineCancelsSlowHTTPRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	a := NewApp(testConfig(server.URL, "tenant"), &bytes.Buffer{}, &bytes.Buffer{})
	if err := a.Execute([]string{"wait", "task", "task-a", "--timeout", "20ms"}); err == nil {
		t.Fatal("deadline not enforced")
	}
}

func TestInitSeparatesCredentialsAndNeverPrintsTokens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("HVX_CONFIG", path)
	t.Setenv("HVX_PASSWORD", "test-password")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/login") {
			_, _ = w.Write([]byte(`{"access_token":"temporary-token","expires_in":"1h","user":{"tenantId":"tenant-a"}}`))
			return
		}
		if r.Header.Get("Authorization") != "Bearer temporary-token" {
			t.Error("PAT endpoint did not receive login token")
		}
		_, _ = w.Write([]byte(`{"token":"new-tenant-pat"}`))
	}))
	defer server.Close()
	cfg := testConfig(server.URL, "admin")
	var out bytes.Buffer
	a := NewApp(cfg, &out, &bytes.Buffer{})
	if err := a.Execute([]string{"init", "--mode", "tenant", "--tenant-server", server.URL, "--username", "test@example.test"}); err != nil {
		t.Fatal(err)
	}
	restored, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if restored.Auth.AdminPAT != "admin-pat" || restored.Token() != "new-tenant-pat" {
		t.Fatal("credentials overwritten across modes")
	}
	if strings.Contains(out.String(), "temporary-token") || strings.Contains(out.String(), "new-tenant-pat") {
		t.Fatal("init output exposed token")
	}
}

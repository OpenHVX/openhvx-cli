package requests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSavedBodyIsExactPrivateAndCheckedForCorruption(t *testing.T) {
	t.Setenv("HVX_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	body := []byte("{\n \"action\": \"echo\", \"data\": {\"message\":\"<secret>\"}}\n")
	r, err := New("https://api.example.test", "tenant", "private-pat", false, body, "")
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(r.ID)
	if err != nil || string(loaded.Body) != string(body) {
		t.Fatal("request bytes changed", err)
	}
	path, _ := pathFor(r.ID)
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "private-pat") {
		t.Fatal("credential persisted")
	}
	loaded.Body = []byte(`{"changed":true}`)
	if err := Save(loaded); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(r.ID); err == nil {
		t.Fatal("payload corruption accepted")
	}
	if _, err := Load("../config"); err == nil {
		t.Fatal("path traversal accepted")
	}
}
func TestRequestLockPreventsConcurrentResubmission(t *testing.T) {
	t.Setenv("HVX_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	r, err := New("https://api.example.test", "tenant", "pat", false, []byte(`{}`), "")
	if err != nil {
		t.Fatal(err)
	}
	release, err := Lock(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if second, err := Lock(r.ID); err == nil {
		second()
		t.Fatal("concurrent request lock acquired")
	}
}

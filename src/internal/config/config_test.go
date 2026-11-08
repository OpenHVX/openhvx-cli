package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLegacyCredentialBelongsOnlyToSavedMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("HVX_CONFIG", path)
	if err := os.WriteFile(path, []byte(`{"profile":{"mode":"tenant"},"auth":{"pat":"tenant-secret","insecure":false}}`), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Token() != "tenant-secret" {
		t.Fatal("legacy credential not migrated")
	}
	cfg.SetMode("admin")
	if cfg.Token() != "" {
		t.Fatal("tenant token leaked to admin mode")
	}
	cfg.SetToken("admin", "admin-secret")
	cfg.SetMode("tenant")
	if cfg.Token() != "tenant-secret" {
		t.Fatal("tenant token lost")
	}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	restored, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	restored.SetMode("admin")
	if restored.Token() != "admin-secret" {
		t.Fatal("admin credential lost")
	}
	if restored.Auth.PAT != "" {
		t.Fatal("legacy token retained")
	}
}

func TestEndpointChangesInvalidateOnlyTheirCredential(t *testing.T) {
	cfg := &Config{}
	cfg.SetTenantServer("https://tenant.example.test")
	cfg.SetAdminServer("https://admin.example.test")
	cfg.SetToken("tenant", "tenant-pat")
	cfg.SetToken("admin", "admin-pat")
	cfg.SetTenantServer("https://other.example.test")
	if cfg.Auth.TenantPAT != "" || cfg.Auth.AdminPAT != "admin-pat" {
		t.Fatal("incorrect credential invalidation")
	}
}

func TestSaveReplacesCompletePrivateFileAndCleansTemporaryFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	t.Setenv("HVX_CONFIG", path)
	cfg := &Config{}
	cfg.SetMode("tenant")
	cfg.SetToken("tenant", "test-token")
	for i := 0; i < 3; i++ {
		if err := Save(cfg); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(data) {
		t.Fatal("invalid saved JSON")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "hvx-config-*.tmp"))
	if len(files) != 0 {
		t.Fatal("temporary credentials left behind")
	}
	info, _ := os.Stat(path)
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("unsafe permissions: %v", info.Mode())
	}
}

func TestLegacyFlatConfigRemainsReadable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("HVX_CONFIG", path)
	_ = os.WriteFile(path, []byte(`{"mode":"admin","admin_server":"https://admin.example.test","pat":"legacy-admin","current_tenant_id":"tenant-a"}`), 0600)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Token() != "legacy-admin" || cfg.CurrentTenant() != "tenant-a" || cfg.AdminServer() != "https://admin.example.test" {
		t.Fatal("legacy fields lost")
	}
}

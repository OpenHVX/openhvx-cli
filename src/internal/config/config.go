package config

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Profile describes how the CLI should behave for the current user.
type Profile struct {
	Mode          string `json:"mode"`                    // "tenant" or "admin"
	CurrentTenant string `json:"currentTenant,omitempty"` // default tenant when mode=admin
}

// Endpoint stores the base server URL for an API surface.
type Endpoint struct {
	Server string `json:"server"`
}

// Endpoints regroups the tenant/admin API endpoints.
type Endpoints struct {
	Tenant Endpoint `json:"tenant,omitempty"`
	Admin  Endpoint `json:"admin,omitempty"`
}

// Auth captures the credentials required by the CLI.
type Auth struct {
	PAT       string `json:"pat,omitempty"`
	TenantPAT string `json:"tenantPat,omitempty"`
	AdminPAT  string `json:"adminPat,omitempty"`
	Insecure  bool   `json:"insecure"`
}

// Config describes the hvx CLI configuration in a kubectl-inspired layout.
type NamedContext struct {
	Profile   Profile   `json:"profile"`
	Endpoints Endpoints `json:"endpoints"`
	Auth      Auth      `json:"auth"`
}

type Config struct {
	CurrentContext string                  `json:"currentContext,omitempty"`
	Contexts       map[string]NamedContext `json:"contexts,omitempty"`
	Profile        Profile                 `json:"profile"`
	Endpoints      Endpoints               `json:"endpoints"`
	Auth           Auth                    `json:"auth"`
}

// Path returns the path used to load the configuration file.
func Path() (string, error) {
	if custom := os.Getenv("HVX_CONFIG"); custom != "" {
		return custom, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".config", "hvx", "config.json"), nil
}

// Load reads the CLI configuration from disk (supporting the legacy flat format).
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, err
	}
	defer f.Close()

	raw, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > 1<<20 {
		return nil, errors.New("configuration exceeds 1 MiB")
	}
	var disk diskConfig
	if err := json.Unmarshal(raw, &disk); err != nil {
		return nil, err
	}

	if disk.CurrentContext != "" {
		if _, ok := disk.Contexts[disk.CurrentContext]; !ok {
			return nil, errors.New("currentContext references an unknown context")
		}
	}
	cfg := disk.toConfig()
	return cfg, nil
}

// Save writes the CLI configuration back to disk.
func Save(cfg *Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	cfg.normalize()
	cfg.snapshotContext()

	path, err := Path()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "hvx-config-*.tmp")
	if err != nil {
		return err
	}

	defer os.Remove(tmp.Name())
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cfg); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}

	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}

	return os.Rename(tmp.Name(), path)
}

// Mode returns the normalized profile mode.
func (c *Config) Mode() string {
	return strings.ToLower(strings.TrimSpace(c.Profile.Mode))
}

// SetMode updates the profile mode using normalized values.
func (c *Config) SetMode(mode string) {
	c.Profile.Mode = strings.ToLower(strings.TrimSpace(mode))
}

// TenantServer returns the configured tenant API base URL.
func (c *Config) TenantServer() string {
	return strings.TrimSpace(c.Endpoints.Tenant.Server)
}

// SetTenantServer configures the tenant API server.
func (c *Config) SetTenantServer(server string) {
	if c.TenantServer() != strings.TrimSpace(server) {
		c.Auth.TenantPAT = ""
	}
	c.Endpoints.Tenant.Server = strings.TrimSpace(server)
}

// AdminServer returns the configured admin API base URL.
func (c *Config) AdminServer() string {
	return strings.TrimSpace(c.Endpoints.Admin.Server)
}

// SetAdminServer configures the admin API server.
func (c *Config) SetAdminServer(server string) {
	if c.AdminServer() != strings.TrimSpace(server) {
		c.Auth.AdminPAT = ""
	}
	c.Endpoints.Admin.Server = strings.TrimSpace(server)
}

// CurrentTenant returns the tenant context stored in the profile.
func (c *Config) CurrentTenant() string {
	return strings.TrimSpace(c.Profile.CurrentTenant)
}

// SetCurrentTenant updates the profile tenant context.
func (c *Config) SetCurrentTenant(tenant string) {
	c.Profile.CurrentTenant = strings.TrimSpace(tenant)
}

// Token returns only the credential for the active API surface.
func (c *Config) Token() string {
	if c == nil {
		return ""
	}
	switch c.Mode() {
	case "tenant":
		return strings.TrimSpace(c.Auth.TenantPAT)
	case "admin":
		return strings.TrimSpace(c.Auth.AdminPAT)
	}
	return ""
}

func (c *Config) SetToken(mode, token string) {
	if mode == "tenant" {
		c.Auth.TenantPAT = token
	}
	if mode == "admin" {
		c.Auth.AdminPAT = token
	}
}

func (c *Config) normalize() {
	c.Profile.Mode = strings.ToLower(strings.TrimSpace(c.Profile.Mode))
	c.Profile.CurrentTenant = strings.TrimSpace(c.Profile.CurrentTenant)
	c.Endpoints.Tenant.Server = strings.TrimSpace(c.Endpoints.Tenant.Server)
	c.Endpoints.Admin.Server = strings.TrimSpace(c.Endpoints.Admin.Server)
	c.Auth.PAT = strings.TrimSpace(c.Auth.PAT)
}

type diskConfig struct {
	CurrentContext string                  `json:"currentContext"`
	Contexts       map[string]NamedContext `json:"contexts"`
	Profile        *Profile                `json:"profile"`
	Endpoints      *Endpoints              `json:"endpoints"`
	Auth           *Auth                   `json:"auth"`

	LegacyMode          string `json:"mode"`
	LegacyTenantServer  string `json:"tenant_server"`
	LegacyAdminServer   string `json:"admin_server"`
	LegacyPAT           string `json:"pat"`
	LegacyTenantID      string `json:"tenant_id"`
	LegacyCurrentTenant string `json:"current_tenant_id"`
	LegacyInsecure      bool   `json:"insecure"`
}

func (d *diskConfig) toConfig() *Config {
	cfg := &Config{CurrentContext: d.CurrentContext, Contexts: d.Contexts}
	if d.Profile != nil {
		cfg.Profile = *d.Profile
	}
	if d.Endpoints != nil {
		cfg.Endpoints = *d.Endpoints
	}
	if d.Auth != nil {
		cfg.Auth = *d.Auth
	}
	cfg.backfillLegacy(d)
	// A legacy PAT belongs only to its saved mode; never reuse it after switching.
	if cfg.Mode() == "tenant" && cfg.Auth.TenantPAT == "" {
		cfg.Auth.TenantPAT = cfg.Auth.PAT
	}
	if cfg.Mode() == "admin" && cfg.Auth.AdminPAT == "" {
		cfg.Auth.AdminPAT = cfg.Auth.PAT
	}
	cfg.Auth.PAT = ""
	cfg.normalize()
	if cfg.CurrentContext != "" {
		if entry, ok := cfg.Contexts[cfg.CurrentContext]; ok {
			cfg.Profile = entry.Profile
			cfg.Endpoints = entry.Endpoints
			cfg.Auth = entry.Auth
		}
	}
	return cfg
}

func (c *Config) backfillLegacy(d *diskConfig) {
	if strings.TrimSpace(c.Profile.Mode) == "" {
		c.Profile.Mode = d.LegacyMode
	}
	if strings.TrimSpace(c.Profile.CurrentTenant) == "" {
		if d.LegacyCurrentTenant != "" {
			c.Profile.CurrentTenant = d.LegacyCurrentTenant
		} else if d.LegacyTenantID != "" {
			c.Profile.CurrentTenant = d.LegacyTenantID
		}
	}
	if strings.TrimSpace(c.Endpoints.Tenant.Server) == "" {
		c.Endpoints.Tenant.Server = d.LegacyTenantServer
	}
	if strings.TrimSpace(c.Endpoints.Admin.Server) == "" {
		c.Endpoints.Admin.Server = d.LegacyAdminServer
	}
	if strings.TrimSpace(c.Auth.PAT) == "" {
		c.Auth.PAT = d.LegacyPAT
	}
	if d.Auth == nil && d.LegacyInsecure {
		c.Auth.Insecure = true
	}
}

func (c *Config) snapshotContext() {
	if c.CurrentContext == "" {
		c.CurrentContext = "default"
	}
	if c.Contexts == nil {
		c.Contexts = map[string]NamedContext{}
	}
	c.Contexts[c.CurrentContext] = NamedContext{c.Profile, c.Endpoints, c.Auth}
}
func (c *Config) SaveContext(name string) error {
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, "\r\n\t/") {
		return errors.New("invalid context name")
	}
	c.snapshotContext()
	if _, ok := c.Contexts[name]; ok && name != c.CurrentContext {
		return errors.New("context already exists; use it rather than overwrite its credentials")
	}
	c.CurrentContext = name
	c.snapshotContext()
	return nil
}
func (c *Config) UseContext(name string) error {
	c.snapshotContext()
	entry, ok := c.Contexts[name]
	if !ok {
		return errors.New("unknown context")
	}
	c.CurrentContext = name
	c.Profile = entry.Profile
	c.Endpoints = entry.Endpoints
	c.Auth = entry.Auth
	return nil
}

func (c *Config) CurrentContextOrDefault() string {
	if c.CurrentContext == "" {
		return "default"
	}
	return c.CurrentContext
}

func (c *Config) InitContext(name string) error {
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, "\r\n\t/") {
		return errors.New("invalid context name")
	}
	if c.CurrentContext == name {
		return nil
	}
	if _, ok := c.Contexts[name]; ok {
		return c.UseContext(name)
	}
	if c.Mode() != "" {
		c.snapshotContext()
	}
	c.CurrentContext = name
	c.Profile = Profile{}
	c.Endpoints = Endpoints{}
	c.Auth = Auth{}
	return nil
}

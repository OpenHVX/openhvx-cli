package config

import (
	"encoding/json"
	"errors"
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
	PAT      string `json:"pat"`
	Insecure bool   `json:"insecure"`
}

// Config describes the hvx CLI configuration in a kubectl-inspired layout.
type Config struct {
	Profile   Profile   `json:"profile"`
	Endpoints Endpoints `json:"endpoints"`
	Auth      Auth      `json:"auth"`
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

	var disk diskConfig
	if err := json.NewDecoder(f).Decode(&disk); err != nil {
		return nil, err
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

	path, err := Path()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "hvx-config-*.tmp")
	if err != nil {
		return err
	}

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cfg); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
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
	c.Endpoints.Tenant.Server = strings.TrimSpace(server)
}

// AdminServer returns the configured admin API base URL.
func (c *Config) AdminServer() string {
	return strings.TrimSpace(c.Endpoints.Admin.Server)
}

// SetAdminServer configures the admin API server.
func (c *Config) SetAdminServer(server string) {
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

func (c *Config) normalize() {
	c.Profile.Mode = strings.ToLower(strings.TrimSpace(c.Profile.Mode))
	c.Profile.CurrentTenant = strings.TrimSpace(c.Profile.CurrentTenant)
	c.Endpoints.Tenant.Server = strings.TrimSpace(c.Endpoints.Tenant.Server)
	c.Endpoints.Admin.Server = strings.TrimSpace(c.Endpoints.Admin.Server)
	c.Auth.PAT = strings.TrimSpace(c.Auth.PAT)
}

type diskConfig struct {
	Profile   *Profile   `json:"profile"`
	Endpoints *Endpoints `json:"endpoints"`
	Auth      *Auth      `json:"auth"`

	LegacyMode          string `json:"mode"`
	LegacyTenantServer  string `json:"tenant_server"`
	LegacyAdminServer   string `json:"admin_server"`
	LegacyPAT           string `json:"pat"`
	LegacyTenantID      string `json:"tenant_id"`
	LegacyCurrentTenant string `json:"current_tenant_id"`
	LegacyInsecure      bool   `json:"insecure"`
}

func (d *diskConfig) toConfig() *Config {
	cfg := &Config{}
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
	cfg.normalize()
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
	if d.LegacyInsecure {
		c.Auth.Insecure = true
	}
}

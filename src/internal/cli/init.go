package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"hvx-cli/internal/api"
	"hvx-cli/internal/config"
)

const (
	defaultTenantLoginPath = "/api/v1/tenant/auth/login"
	defaultAdminLoginPath  = "/api/v1/admin/auth/login"
	defaultTenantPATPath   = "/api/v1/tenant/auth/pats"
	defaultAdminPATPath    = "/api/v1/admin/auth/pats"
)

func (a *App) initCommand(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	modeFlag := fs.String("mode", "", "target mode: tenant or admin")
	tenantServer := fs.String("tenant-server", "", "tenant API base URL")
	adminServer := fs.String("admin-server", "", "admin API base URL")
	username := fs.String("username", "", "login username or email")
	password := fs.String("password", "", "login password (or set HVX_PASSWORD)")
	insecure := fs.Bool("insecure", false, "skip TLS verification (dev only)")
	tenantID := fs.String("tenant-id", "", "default tenant ID (admin mode)")
	currentTenant := fs.String("current-tenant-id", "", "current tenant context (admin mode)")
	loginPathOverride := fs.String("login-path", "", "override login API path")
	patPathOverride := fs.String("pat-path", "", "override PAT creation API path")
	patLabel := fs.String("pat-label", "", "label to assign to the generated PAT")
	patExpires := fs.Int("pat-expiry-days", 0, "set PAT expiration in days (optional)")
	if err := fs.Parse(args); err != nil {
		return ErrUsage
	}

	mode := strings.ToLower(strings.TrimSpace(*modeFlag))
	if mode != "tenant" && mode != "admin" {
		return fmt.Errorf("--mode must be 'tenant' or 'admin'")
	}

	user := strings.TrimSpace(*username)
	if user == "" {
		return fmt.Errorf("--username is required")
	}

	pass := *password
	if pass == "" {
		pass = os.Getenv("HVX_PASSWORD")
	}
	if pass == "" {
		return fmt.Errorf("--password is required (or set HVX_PASSWORD)")
	}

	tenantURL := strings.TrimRight(strings.TrimSpace(*tenantServer), "/")
	adminURL := strings.TrimRight(strings.TrimSpace(*adminServer), "/")
	tenantOverride := strings.TrimSpace(*tenantID)
	currentTenantOverride := strings.TrimSpace(*currentTenant)

	var baseURL string
	switch mode {
	case "tenant":
		baseURL = tenantURL
		if baseURL == "" {
			return fmt.Errorf("--tenant-server is required in tenant mode")
		}
	case "admin":
		baseURL = adminURL
		if baseURL == "" {
			return fmt.Errorf("--admin-server is required in admin mode")
		}
	}

	loginPath := strings.TrimSpace(*loginPathOverride)
	if loginPath == "" {
		if mode == "tenant" {
			loginPath = defaultTenantLoginPath
		} else {
			loginPath = defaultAdminLoginPath
		}
	}
	if !strings.HasPrefix(loginPath, "/") {
		loginPath = "/" + loginPath
	}

	client := api.NewClient(baseURL, "", *insecure)
	loginResp, err := client.Login(loginPath, api.LoginRequest{
		Username: user,
		Email:    user,
		Password: pass,
	})
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	patPath := strings.TrimSpace(*patPathOverride)
	if patPath == "" {
		if mode == "tenant" {
			patPath = defaultTenantPATPath
		} else {
			patPath = defaultAdminPATPath
		}
	}
	if !strings.HasPrefix(patPath, "/") {
		patPath = "/" + patPath
	}

	patClient := api.NewClient(baseURL, loginResp.AccessToken, *insecure)
	patReq := api.PATCreateRequest{}
	if label := strings.TrimSpace(*patLabel); label != "" {
		patReq.Label = label
	}
	if *patExpires > 0 {
		patReq.ExpiresInDays = *patExpires
	}
	patResp, err := patClient.CreatePAT(patPath, patReq)
	if err != nil {
		return fmt.Errorf("unable to create PAT: %w", err)
	}

	cfg := a.cfg
	if cfg == nil {
		cfg = &config.Config{}
		a.cfg = cfg
	}

	cfg.SetMode(mode)
	cfg.Auth.PAT = patResp.Token
	cfg.Auth.Insecure = *insecure

	if mode == "tenant" {
		cfg.SetTenantServer(baseURL)
		if adminURL != "" {
			cfg.SetAdminServer(adminURL)
		}
		resolvedTenant := tenantOverride
		if resolvedTenant == "" {
			resolvedTenant = extractTenantFromUser(loginResp.User)
		}
		cfg.SetCurrentTenant(resolvedTenant)
	} else {
		cfg.SetAdminServer(baseURL)
		if tenantURL != "" {
			cfg.SetTenantServer(tenantURL)
		}
		tenantContext := strings.TrimSpace(currentTenantOverride)
		if tenantContext == "" {
			tenantContext = tenantOverride
		}
		cfg.SetCurrentTenant(tenantContext)
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("unable to save config: %w", err)
	}

	configPath, err := config.Path()
	if err != nil {
		configPath = "config.json"
	}

	if a.outputFormat == "json" {
		payload := map[string]string{
			"status": "initialized",
			"mode":   mode,
			"config": configPath,
		}
		if tenantServer := cfg.TenantServer(); tenantServer != "" {
			payload["tenant_server"] = tenantServer
		}
		if adminServer := cfg.AdminServer(); adminServer != "" {
			payload["admin_server"] = adminServer
		}
		if current := cfg.CurrentTenant(); current != "" {
			payload["currentTenant"] = current
		}
		enc := json.NewEncoder(a.stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}

	fmt.Fprintf(a.stdout, "Configuration saved to %s (mode=%s)\n", configPath, mode)
	return nil
}

func extractTenantFromUser(user map[string]any) string {
	if user == nil {
		return ""
	}
	if tenant := firstString(user, "currentTenant", "defaultTenant", "tenantId", "tenant_id"); tenant != "" {
		return tenant
	}
	if ctx, ok := user["context"].(map[string]any); ok {
		if tenant := firstString(ctx, "currentTenant", "defaultTenant", "tenantId", "tenant_id"); tenant != "" {
			return tenant
		}
	}
	if tenants := extractTenantList(user["tenants"]); len(tenants) > 0 {
		return tenants[0]
	}
	return ""
}

func extractTenantList(value any) []string {
	switch v := value.(type) {
	case []any:
		var tenants []string
		for _, item := range v {
			switch t := item.(type) {
			case string:
				if s := strings.TrimSpace(t); s != "" {
					tenants = append(tenants, s)
				}
			case map[string]any:
				if s := firstString(t, "id", "tenantId", "tenant_id", "name"); s != "" {
					tenants = append(tenants, s)
				}
			}
		}
		return tenants
	case []string:
		var tenants []string
		for _, item := range v {
			if s := strings.TrimSpace(item); s != "" {
				tenants = append(tenants, s)
			}
		}
		return tenants
	}
	return nil
}

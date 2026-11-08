package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"hvx-cli/internal/api"
	"hvx-cli/internal/config"
)

func (a *App) contextCommand(args []string) error {
	if len(args) == 0 || args[0] == "show" {
		return a.showContext()
	}
	switch args[0] {
	case "set-mode":
		return a.contextSetMode(args[1:])
	case "set-tenant":
		return a.contextSetTenant(args[1:])
	default:
		return ErrUsage
	}
}

func (a *App) showContext() error {
	cfg := a.cfg
	mode := cfg.Mode()
	var server, tenant string

	switch mode {
	case "tenant":
		server = cfg.TenantServer()
		if server == "" {
			server = "(tenant endpoint not set)"
		}
		tenant = "(PAT tenant)"
	case "admin":
		server = cfg.AdminServer()
		if server == "" {
			server = "(admin endpoint not set)"
		}
		if cfg.CurrentTenant() != "" {
			tenant = cfg.CurrentTenant()
		} else {
			tenant = "(no tenant configured)"
		}
	default:
		return fmt.Errorf("invalid profile.mode in config.json (use 'tenant' or 'admin')")
	}

	if a.outputFormat == "json" {
		payload := map[string]string{
			"mode":   mode,
			"server": server,
			"tenant": tenant,
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

	fmt.Fprintf(a.stdout, "Mode: %s\n", mode)
	fmt.Fprintf(a.stdout, "Server: %s\n", server)
	fmt.Fprintf(a.stdout, "Tenant: %s\n", tenant)
	return nil
}

func (a *App) contextSetMode(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("'hvx context set-mode' requires 'tenant' or 'admin'")
	}
	target := strings.ToLower(strings.TrimSpace(args[0]))
	if target != "tenant" && target != "admin" {
		return fmt.Errorf("invalid mode '%s' (use 'tenant' or 'admin')", target)
	}

	cfg := a.cfg
	if cfg.Mode() == target {
		return a.printModeResult(target, false)
	}

	cfg.SetMode(target)
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("unable to save config: %w", err)
	}
	return a.printModeResult(target, true)
}

func (a *App) contextSetTenant(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("'hvx context set-tenant' requires a TENANT_ID")
	}
	if err := a.ensureAccess("context set-tenant", modeAdmin); err != nil {
		return err
	}

	cfg := a.cfg
	tenantID := strings.TrimSpace(args[0])
	if tenantID == "" {
		return fmt.Errorf("TENANT_ID cannot be empty")
	}

	client := api.NewClient(cfg.AdminServer(), cfg.Auth.PAT, cfg.Auth.Insecure)
	exists, err := tenantExists(client, tenantID)
	if err != nil {
		return fmt.Errorf("unable to verify tenant: %w", err)
	}
	if !exists {
		return fmt.Errorf("tenant '%s' not found", tenantID)
	}

	cfg.SetCurrentTenant(tenantID)
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("unable to save config: %w", err)
	}

	if a.outputFormat == "json" {
		payload := map[string]string{
			"currentTenant": tenantID,
			"status":        "context_updated",
		}
		enc := json.NewEncoder(a.stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}

	fmt.Fprintf(a.stdout, "Current tenant set to %s\n", tenantID)
	return nil
}

func (a *App) printModeResult(mode string, updated bool) error {
	status := "unchanged"
	if updated {
		status = "updated"
	}
	if a.outputFormat == "json" {
		payload := map[string]string{
			"mode":   mode,
			"status": status,
		}
		enc := json.NewEncoder(a.stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}
	if updated {
		fmt.Fprintf(a.stdout, "Mode updated to %s\n", mode)
		return nil
	}
	fmt.Fprintf(a.stdout, "Mode already set to %s\n", mode)
	return nil
}

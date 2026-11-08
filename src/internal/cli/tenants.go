package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"hvx-cli/internal/api"
)

func (a *App) getTenants(args []string) error {
	cfg := a.cfg
	if err := a.ensureAccess("get tenants", modeAdmin); err != nil {
		return err
	}

	fs := flag.NewFlagSet("get tenants", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := parseFlags(fs, args); err != nil {
		return ErrUsage
	}
	if fs.NArg() != 0 {
		return ErrUsage
	}

	client := api.NewClient(cfg.AdminServer(), cfg.Token(), cfg.Auth.Insecure)
	if a.outputFormat == "json" {
		raw, err := client.ListTenantsRaw()
		if err != nil {
			return err
		}
		_, err = a.stdout.Write(raw)
		return err
	}

	tenants, err := client.ListTenants()
	if err != nil {
		return err
	}

	if len(tenants) == 0 {
		fmt.Fprintln(a.stdout, "No tenants found.")
		return nil
	}

	columns := []tableColumn{
		{Header: "ID", Width: 20},
		{Header: "NAME", Width: 0},
		{Header: "CURRENT", Width: 8},
	}
	rows := make([][]string, 0, len(tenants))
	for _, t := range tenants {
		id := strings.TrimSpace(t.TenantID)
		if id == "" {
			id = "(unknown)"
		}
		current := ""
		if a.isTenantSelected(id) {
			current = "*"
		}
		rows = append(rows, []string{id, t.Name, current})
	}

	a.printTable(columns, rows, "")
	return nil
}

func (a *App) createTenant(args []string) error {
	cfg := a.cfg
	if err := a.ensureAccess("create tenant", modeAdmin); err != nil {
		return err
	}

	fs := flag.NewFlagSet("create tenant", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	idFlag := fs.String("id", "", "tenant ID (required)")
	nameFlag := fs.String("name", "", "tenant name")
	descriptionFlag := fs.String("description", "", "tenant description")
	statusFlag := fs.String("status", "", "tenant status")
	quotasPath := fs.String("quotas", "", "path to quotas JSON file")
	metadataPath := fs.String("metadata", "", "path to metadata JSON file")
	if err := parseFlags(fs, args); err != nil {
		return ErrUsage
	}
	if fs.NArg() > 1 {
		return ErrUsage
	}

	name := strings.TrimSpace(*nameFlag)
	if name == "" && fs.NArg() == 1 {
		name = strings.TrimSpace(fs.Arg(0))
	}
	if name == "" {
		return fmt.Errorf("--name is required")
	}

	var (
		quotas   json.RawMessage
		metadata json.RawMessage
		err      error
	)
	if path := strings.TrimSpace(*quotasPath); path != "" {
		quotas, err = loadJSONFile(path)
		if err != nil {
			return fmt.Errorf("failed to read quotas file: %w", err)
		}
	}
	if path := strings.TrimSpace(*metadataPath); path != "" {
		metadata, err = loadJSONFile(path)
		if err != nil {
			return fmt.Errorf("failed to read metadata file: %w", err)
		}
	}

	tenantID := strings.TrimSpace(*idFlag)
	if tenantID == "" {
		return fmt.Errorf("--id is required (used as tenantId)")
	}
	req := api.TenantCreateRequest{
		TenantID:    tenantID,
		Name:        name,
		Description: strings.TrimSpace(*descriptionFlag),
		Status:      strings.TrimSpace(*statusFlag),
		Quotas:      quotas,
		Metadata:    metadata,
	}
	client := api.NewClient(cfg.AdminServer(), cfg.Token(), cfg.Auth.Insecure)
	tenant, err := client.CreateTenant(req)
	if err != nil {
		return err
	}

	if a.outputFormat == "json" {
		payload := map[string]any{
			"status": "created",
			"tenant": tenant,
		}
		enc := json.NewEncoder(a.stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}

	reportID := strings.TrimSpace(tenant.TenantID)
	if reportID == "" {
		reportID = "(pending)"
	}
	fmt.Fprintf(a.stdout, "Tenant created: %s (%s)\n", reportID, tenant.Name)
	return nil
}

func (a *App) deleteTenant(args []string) error {
	cfg := a.cfg
	if err := a.ensureAccess("delete tenant", modeAdmin); err != nil {
		return err
	}

	fs := flag.NewFlagSet("delete tenant", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	idFlag := fs.String("id", "", "tenant ID")
	if err := parseFlags(fs, args); err != nil {
		return ErrUsage
	}
	if fs.NArg() > 1 {
		return ErrUsage
	}

	tenantID := strings.TrimSpace(*idFlag)
	if tenantID == "" && fs.NArg() == 1 {
		tenantID = strings.TrimSpace(fs.Arg(0))
	}
	if tenantID == "" {
		return fmt.Errorf("--id or TENANT_ID argument is required")
	}

	client := api.NewClient(cfg.AdminServer(), cfg.Token(), cfg.Auth.Insecure)
	if err := client.DeleteTenant(tenantID); err != nil {
		return err
	}

	if a.outputFormat == "json" {
		payload := map[string]string{
			"status": "deleted",
			"tenant": tenantID,
		}
		enc := json.NewEncoder(a.stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}

	fmt.Fprintf(a.stdout, "Tenant deleted: %s\n", tenantID)
	return nil
}

func (a *App) createQuotaRecalculation(args []string) error {
	cfg := a.cfg
	if err := a.ensureAccess("create quota-recalc", modeAdmin); err != nil {
		return err
	}

	fs := flag.NewFlagSet("create quota-recalc", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	tenantFlag := fs.String("tenant", "", "target tenant ID")
	if err := parseFlags(fs, args); err != nil {
		return ErrUsage
	}
	if fs.NArg() > 1 {
		return ErrUsage
	}

	override := strings.TrimSpace(*tenantFlag)
	if override == "" {
		if fs.NArg() == 0 {
			return fmt.Errorf("--tenant is required")
		}
		override = fs.Arg(0)
	}

	tenantID, err := a.resolveTenantID("create quota-recalc", override)
	if err != nil {
		return err
	}

	client := api.NewClient(cfg.AdminServer(), cfg.Token(), cfg.Auth.Insecure)
	if err := client.RecalculateQuota(tenantID); err != nil {
		return err
	}

	if a.outputFormat == "json" {
		payload := map[string]string{
			"tenant": tenantID,
			"status": "quota_recalculated",
		}
		enc := json.NewEncoder(a.stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}

	fmt.Fprintf(a.stdout, "Quota recalculated for %s\n", tenantID)
	return nil
}

func tenantExists(client *api.Client, tenantID string) (bool, error) {
	tenantID = strings.TrimSpace(tenantID)
	tenants, err := client.ListTenants()
	if err != nil {
		return false, err
	}
	for _, t := range tenants {
		if strings.EqualFold(strings.TrimSpace(t.TenantID), tenantID) {
			return true, nil
		}
	}
	return false, nil
}

func (a *App) isTenantSelected(tenantID string) bool {
	if a.cfg == nil {
		return false
	}
	cur := strings.TrimSpace(a.cfg.CurrentTenant())
	if cur == "" || strings.TrimSpace(tenantID) == "" {
		return false
	}
	return strings.EqualFold(cur, strings.TrimSpace(tenantID))
}

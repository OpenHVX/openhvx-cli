package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"hvx-cli/internal/api"
)

func (a *App) getQuotas(args []string) error {
	cfg := a.cfg
	if err := a.ensureAccess("get quotas", modeTenant|modeAdmin); err != nil {
		return err
	}

	fs := flag.NewFlagSet("get quotas", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	tenantFlag := fs.String("tenant", "", "tenant override (admin mode only)")
	if err := fs.Parse(args); err != nil {
		return ErrUsage
	}
	if fs.NArg() > 1 {
		return ErrUsage
	}

	overrideTenantID := strings.TrimSpace(*tenantFlag)
	if overrideTenantID == "" && fs.NArg() == 1 {
		overrideTenantID = fs.Arg(0)
	}

	var (
		client *api.Client
		quotas []api.Quota
		err    error
	)

	switch cfg.Mode() {
	case "tenant":
		if strings.TrimSpace(overrideTenantID) != "" {
			return fmt.Errorf("--tenant is only supported in admin mode; tenant identity comes from credentials")
		}
		client = api.NewClient(cfg.TenantServer(), cfg.Token(), cfg.Auth.Insecure)
		tenantID := a.tenantContextID(overrideTenantID)
		if a.outputFormat == "json" {
			raw, err := client.ListTenantQuotasRaw(tenantID)
			if err != nil {
				return err
			}
			_, err = a.stdout.Write(raw)
			return err
		}
		quotas, err = client.ListTenantQuotas(tenantID)

	case "admin":
		tenantID, err := a.resolveTenantID("get quotas", overrideTenantID)
		if err != nil {
			return err
		}
		client = api.NewClient(cfg.AdminServer(), cfg.Token(), cfg.Auth.Insecure)
		if a.outputFormat == "json" {
			raw, err := client.ListAdminQuotasRaw(tenantID)
			if err != nil {
				return err
			}
			_, err = a.stdout.Write(raw)
			return err
		}
		quotas, err = client.ListAdminQuotas(tenantID)
	}

	if err != nil {
		return err
	}

	if len(quotas) == 0 {
		fmt.Fprintln(a.stdout, "No quotas found.")
		return nil
	}

	columns := []tableColumn{
		{Header: "METRIC", Width: 25},
		{Header: "USAGE", Width: 0},
		{Header: "LIMIT", Width: 0},
		{Header: "UNIT", Width: 0},
		{Header: "STATUS", Width: 0},
	}
	rows := make([][]string, 0, len(quotas))
	for _, q := range quotas {
		name := q.Name
		if name == "" {
			name = q.Metric
		}
		unit := strings.TrimSpace(q.Unit)
		if unit == "" {
			unit = "-"
		}
		status := strings.TrimSpace(q.Status)
		if status == "" {
			status = "-"
		}
		rows = append(rows, []string{
			name,
			formatValue(q.Usage),
			formatValue(q.Limit),
			unit,
			status,
		})
	}

	a.printTable(columns, rows, "")
	return nil
}

package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"hvx-cli/internal/config"
)

// ErrUsage indicates the user invoked the CLI with invalid arguments.
var ErrUsage = errors.New("usage")

// usageText explains the available commands.
const usageText = `hvx - OpenHVX CLI client

Usage:
  hvx [-o json] <command> [args]

Primary commands:
  hvx get resources [--tenant TENANT_ID]   # list resources for the active context
  hvx get disks [--tenant TENANT_ID]       # list storage disks for the active context
  hvx get vms [--tenant TENANT_ID]         # list VM runtime information
  hvx get images                           # list VM images
  hvx get quotas [--tenant TENANT_ID]      # print quota usage
  hvx get tenants                          # admin mode: list all tenants
  hvx get task --id TASK_ID                # fetch a task payload in the current mode

  hvx describe image IMAGE_ID [--resolve]  # describe a VM image (raw or summarized)
  hvx describe task TASK_ID                # show task payload/status

  hvx create disk --name NAME --size-mb MB # create a storage disk
  hvx create vm [options]                  # admin mode: submit a VM creation task
  hvx create tenant --name NAME [--id ID]  # admin mode: create a tenant
  hvx delete vm [options]                  # admin mode: submit a VM deletion task
  hvx delete disk --ref-id REF_ID          # admin mode: submit a disk deletion task
  hvx delete tenant TENANT_ID              # admin mode: delete a tenant
  hvx create user [options]                # admin mode: create a tenant user
  hvx create quota-recalc --tenant ID      # admin mode: trigger a quota recalculation

  hvx context show                         # print current profile/endpoints
  hvx context set-mode tenant|admin        # change the active profile mode
  hvx context set-tenant TENANT_ID         # update the tenant context (admin mode)
  hvx init [options]                       # initialize config.json and fetch a PAT

Options:
  -o, --output text|json   # change output format (default: text)

Configuration:
  The CLI loads its config from ~/.config/hvx/config.json (or $HVX_CONFIG)

Example config.json:

{
  "profile": {
    "mode": "tenant",
    "currentTenant": "tenant-xyz"
  },
  "endpoints": {
    "tenant": { "server": "https://api.openhvx.local" },
    "admin":  { "server": "https://admin-api.openhvx.local" }
  },
  "auth": {
    "pat": "hvx_pat_xxx",
    "insecure": true
  }
}

Modes:
  - "tenant" mode:
      * get/resources, get/vms, get/quotas -> tenant endpoint (/api/v1/tenant/*)
  - "admin" mode:
      * reading data requires --tenant or profile.currentTenant
      * create/delete commands target the admin endpoint (/api/v1/admin/*)
`

// Usage returns the CLI usage instructions.
func Usage() string {
	return usageText
}

// App regroups dependencies required to run CLI commands.
type App struct {
	cfg          *config.Config
	stdout       io.Writer
	stderr       io.Writer
	outputFormat string
}

type commandMode uint8

const (
	modeTenant commandMode = 1 << iota
	modeAdmin
)

// NewApp instantiates an App with sane defaults for stdout/stderr.
func NewApp(cfg *config.Config, stdout, stderr io.Writer) *App {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	return &App{
		cfg:          cfg,
		stdout:       stdout,
		stderr:       stderr,
		outputFormat: "text",
	}
}

// Execute dispatches the CLI command according to the provided arguments.
func (a *App) Execute(args []string) error {
	if len(args) == 0 {
		return ErrUsage
	}

	parsed, err := a.parseGlobalOptions(args)
	if err != nil {
		return err
	}

	if len(parsed) == 0 {
		return ErrUsage
	}

	switch parsed[0] {
	case "get":
		return a.getCommand(parsed[1:])
	case "describe":
		return a.describeCommand(parsed[1:])
	case "create":
		return a.createCommand(parsed[1:])
	case "delete":
		return a.deleteCommand(parsed[1:])
	case "context":
		return a.contextCommand(parsed[1:])
	case "init":
		return a.initCommand(parsed[1:])
	default:
		return ErrUsage
	}
}

func (a *App) ensureAccess(command string, allowed commandMode) error {
	cfg := a.cfg
	if cfg == nil || cfg.Auth.PAT == "" {
		return fmt.Errorf("missing auth.pat in config.json")
	}

	switch cfg.Mode() {
	case "tenant":
		if allowed&modeTenant == 0 {
			return fmt.Errorf("the '%s' command requires admin mode", command)
		}
		if cfg.TenantServer() == "" {
			return fmt.Errorf("'endpoints.tenant.server' must be set in config.json for tenant mode")
		}
	case "admin":
		if allowed&modeAdmin == 0 {
			return fmt.Errorf("the '%s' command requires tenant mode", command)
		}
		if cfg.AdminServer() == "" {
			return fmt.Errorf("'endpoints.admin.server' must be set in config.json for admin mode")
		}
	default:
		return fmt.Errorf("invalid profile.mode in config.json (use 'tenant' or 'admin')")
	}

	return nil
}

func (a *App) setOutputFormat(format string) error {
	switch format {
	case "", "text":
		a.outputFormat = "text"
	case "json":
		a.outputFormat = "json"
	default:
		return fmt.Errorf("unsupported output format '%s' (supported: text, json)", format)
	}
	return nil
}

func (a *App) parseGlobalOptions(args []string) ([]string, error) {
	var filtered []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-o" || arg == "--output":
			if i+1 >= len(args) {
				return nil, ErrUsage
			}
			if err := a.setOutputFormat(args[i+1]); err != nil {
				return nil, err
			}
			i++
		case strings.HasPrefix(arg, "-o="):
			if err := a.setOutputFormat(strings.TrimPrefix(arg, "-o=")); err != nil {
				return nil, err
			}
		case strings.HasPrefix(arg, "--output="):
			if err := a.setOutputFormat(strings.TrimPrefix(arg, "--output=")); err != nil {
				return nil, err
			}
		default:
			filtered = append(filtered, arg)
		}
	}
	return filtered, nil
}

func (a *App) resolveTenantID(command, override string) (string, error) {
	tenantID := strings.TrimSpace(override)
	if tenantID != "" {
		return tenantID, nil
	}
	if a.cfg != nil {
		if tenant := a.cfg.CurrentTenant(); tenant != "" {
			return tenant, nil
		}
	}
	return "", fmt.Errorf("no tenant specified (set profile.currentTenant or pass --tenant to 'hvx %s')", command)
}

func (a *App) tenantContextID(override string) string {
	if strings.TrimSpace(override) != "" {
		return strings.TrimSpace(override)
	}
	if a.cfg == nil {
		return ""
	}
	return a.cfg.CurrentTenant()
}

func formatValue(v float64) string {
	return fmt.Sprintf("%g", v)
}

func (a *App) getCommand(args []string) error {
	if len(args) == 0 {
		return ErrUsage
	}
	switch args[0] {
	case "disks":
		return a.getDisks(args[1:])
	case "resources":
		return a.getResources(args[1:])
	case "vms":
		return a.getVMs(args[1:])
	case "images":
		return a.getImages(args[1:])
	case "quotas":
		return a.getQuotas(args[1:])
	case "tenants":
		return a.getTenants(args[1:])
	case "task":
		return a.getTask(args[1:])
	default:
		return fmt.Errorf("unsupported resource '%s' for 'hvx get'", args[0])
	}
}

func (a *App) describeCommand(args []string) error {
	if len(args) == 0 {
		return ErrUsage
	}
	switch args[0] {
	case "image":
		return a.describeImage(args[1:])
	case "task":
		return a.describeTask(args[1:])
	default:
		return fmt.Errorf("unsupported resource '%s' for 'hvx describe'", args[0])
	}
}

func (a *App) createCommand(args []string) error {
	if len(args) == 0 {
		return ErrUsage
	}
	switch args[0] {
	case "disk":
		return a.createDisk(args[1:])
	case "vm":
		return a.createVM(args[1:])
	case "tenant":
		return a.createTenant(args[1:])
	case "user":
		return a.createUser(args[1:])
	case "quota-recalc":
		return a.createQuotaRecalculation(args[1:])
	default:
		return fmt.Errorf("unsupported resource '%s' for 'hvx create'", args[0])
	}
}

func (a *App) deleteCommand(args []string) error {
	if len(args) == 0 {
		return ErrUsage
	}
	switch args[0] {
	case "vm":
		return a.deleteVM(args[1:])
	case "disk":
		return a.deleteDisk(args[1:])
	case "tenant":
		return a.deleteTenant(args[1:])
	default:
		return fmt.Errorf("unsupported resource '%s' for 'hvx delete'", args[0])
	}
}

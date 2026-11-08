package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"hvx-cli/internal/api"
)

func (a *App) resourceList(tenant string) ([]api.Resource, error) {
	if err := a.ensureAccess("resources", modeTenant|modeAdmin); err != nil {
		return nil, err
	}
	if a.cfg.Mode() == "tenant" {
		if tenant != "" {
			return nil, fmt.Errorf("--tenant is admin-only")
		}
		return api.NewClient(a.cfg.TenantServer(), a.cfg.Token(), a.cfg.Auth.Insecure).ListTenantResources("")
	}
	id, err := a.resolveTenantID("resources", tenant)
	if err != nil {
		return nil, err
	}
	return api.NewClient(a.cfg.AdminServer(), a.cfg.Token(), a.cfg.Auth.Insecure).ListAdminResources(id)
}
func (a *App) resolveResource(kind, name, tenant, agent string) (api.Resource, error) {
	list, err := a.resourceList(tenant)
	if err != nil {
		return api.Resource{}, err
	}
	var matches []api.Resource
	for _, r := range list {
		k := r.Kind
		if k == "" {
			k = r.Type
		}
		if kind == "disk" && k == "storage" {
			k = "disk"
		}
		if k != kind || (agent != "" && r.AgentID != agent) {
			continue
		}
		if name == r.Name || name == r.RefID || name == r.ID || (kind == "vm" && (strings.EqualFold(name, r.RefID) || strings.EqualFold(name, r.ID))) {
			matches = append(matches, r)
		}
	}
	if len(matches) != 1 {
		return api.Resource{}, fmt.Errorf("resource %q matched %d %ss; use its canonical reference and --agent-id when ambiguous", name, len(matches), kind)
	}
	r := matches[0]
	if r.RefID == "" {
		r.RefID = r.ID
	}
	if r.RefID == "" || r.AgentID == "" {
		return api.Resource{}, fmt.Errorf("inventory lacks canonical resource identity")
	}
	if kind == "vm" && !api.ValidGUID(r.RefID) {
		return api.Resource{}, fmt.Errorf("inventory VM reference is not a canonical GUID")
	}
	return r, nil
}
func resourceOptions(command string, args []string) (name, tenant, agent string, force bool, err error) {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&tenant, "tenant", "", "Admin tenant override")
	fs.StringVar(&agent, "agent-id", "", "Agent identity")
	fs.BoolVar(&force, "force", false, "Force stop before VM deletion")
	if err = parseFlags(fs, args); err != nil {
		return
	}
	if fs.NArg() != 1 {
		err = fmt.Errorf("resource name or reference is required")
		return
	}
	name = fs.Arg(0)
	return
}
func (a *App) submitResource(action, tenant string, target api.TaskTarget, data any) error {
	if a.cfg.Mode() == "tenant" && tenant != "" {
		return fmt.Errorf("--tenant is admin-only")
	}
	payload := map[string]any{"action": action, "target": target, "data": data}
	server := a.cfg.TenantServer()
	if a.cfg.Mode() == "admin" {
		id, err := a.resolveTenantID(action, tenant)
		if err != nil {
			return err
		}
		payload["tenantId"] = id
		server = a.cfg.AdminServer()
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	client := api.NewClient(server, a.cfg.Token(), a.cfg.Auth.Insecure)
	a.trackTasks(client)
	response, err := client.SubmitTask(a.cfg.Mode(), raw)
	if err != nil {
		return err
	}
	return a.printTaskResponse(action, response)
}
func (a *App) powerVM(state string, args []string) error {
	if len(args) == 0 || args[0] != "vm" {
		return ErrUsage
	}
	name, tenant, agent, force, err := resourceOptions(state+" vm", args[1:])
	if err != nil {
		return err
	}
	if force {
		return fmt.Errorf("--force applies only to delete vm")
	}
	r, err := a.resolveResource("vm", name, tenant, agent)
	if err != nil {
		return err
	}
	return a.submitResource("vm.power", tenant, api.TaskTarget{Kind: "vm", AgentID: r.AgentID, RefID: strings.ToLower(r.RefID)}, map[string]string{"state": state})
}
func (a *App) describeResource(kind string, args []string) error {
	name, tenant, agent, force, err := resourceOptions("describe "+kind, args)
	if err != nil {
		return err
	}
	if force {
		return ErrUsage
	}
	r, err := a.resolveResource(kind, name, tenant, agent)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(a.stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
func (a *App) deleteNamedResource(kind string, args []string) error {
	name, tenant, agent, force, err := resourceOptions("delete "+kind, args)
	if err != nil {
		return err
	}
	r, err := a.resolveResource(kind, name, tenant, agent)
	if err != nil {
		return err
	}
	if kind == "vm" {
		return a.submitResource("vm.delete", tenant, api.TaskTarget{Kind: "vm", AgentID: r.AgentID, RefID: strings.ToLower(r.RefID)}, map[string]bool{"forceStop": force})
	}
	if force {
		return fmt.Errorf("--force applies only to delete vm")
	}
	return a.submitResource("storage.delete", tenant, api.TaskTarget{Kind: "storage", AgentID: r.AgentID, RefID: r.RefID}, map[string]string{"refId": r.RefID})
}

// parseFlags accepts kubectl-style flags before or after positional arguments.
func parseFlags(fs *flag.FlagSet, args []string) error {
	var options, positionals []string
	for i := 0; i < len(args); i++ {
		value := args[i]
		if value == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(value, "-") || value == "-" {
			positionals = append(positionals, value)
			continue
		}
		options = append(options, value)
		name := strings.SplitN(strings.TrimLeft(value, "-"), "=", 2)
		f := fs.Lookup(name[0])
		if f == nil {
			return fmt.Errorf("unknown flag %s", value)
		}
		if len(name) == 2 {
			continue
		}
		if b, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && b.IsBoolFlag() {
			continue
		}
		if i+1 >= len(args) {
			return fmt.Errorf("flag %s requires a value", value)
		}
		i++
		options = append(options, args[i])
	}
	return fs.Parse(append(options, positionals...))
}

func hasNamedResource(args []string) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			return i+1 < len(args)
		}
		if !strings.HasPrefix(arg, "-") {
			return true
		}
		parts := strings.SplitN(strings.TrimLeft(arg, "-"), "=", 2)
		if len(parts) == 2 || parts[0] == "force" {
			continue
		}
		if parts[0] == "tenant" || parts[0] == "agent-id" || parts[0] == "ref-id" || parts[0] == "guid" {
			i++
		}
	}
	return false
}

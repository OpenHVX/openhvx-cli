package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"hvx-cli/internal/api"
)

func (a *App) getResources(args []string) error {
	cfg := a.cfg
	if err := a.ensureAccess("get resources", modeTenant|modeAdmin); err != nil {
		return err
	}

	fs := flag.NewFlagSet("get resources", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	tenantFlag := fs.String("tenant", "", "tenant override (admin mode only)")
	if err := parseFlags(fs, args); err != nil {
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
		client    *api.Client
		resources []api.Resource
		err       error
	)

	switch cfg.Mode() {
	case "tenant":
		if strings.TrimSpace(overrideTenantID) != "" {
			return fmt.Errorf("--tenant is only supported in admin mode; tenant identity comes from credentials")
		}
		client = api.NewClient(cfg.TenantServer(), cfg.Token(), cfg.Auth.Insecure)
		tenantID := a.tenantContextID(overrideTenantID)
		if a.outputFormat == "json" {
			raw, err := client.ListTenantResourcesRaw(tenantID)
			if err != nil {
				return err
			}
			_, err = a.stdout.Write(raw)
			return err
		}
		resources, err = client.ListTenantResources(tenantID)

	case "admin":
		tenantID, err := a.resolveTenantID("get resources", overrideTenantID)
		if err != nil {
			return err
		}
		client = api.NewClient(cfg.AdminServer(), cfg.Token(), cfg.Auth.Insecure)
		if a.outputFormat == "json" {
			raw, err := client.ListAdminResourcesRaw(tenantID)
			if err != nil {
				return err
			}
			_, err = a.stdout.Write(raw)
			return err
		}
		resources, err = client.ListAdminResources(tenantID)
	}

	if err != nil {
		return err
	}

	if len(resources) == 0 {
		if a.outputFormat == "json" {
			return nil
		}
		fmt.Fprintln(a.stdout, "No resources found.")
		return nil
	}

	if a.outputFormat == "json" {
		enc := json.NewEncoder(a.stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resources)
	}

	columns := []tableColumn{
		{Header: "ID", Width: 36},
		{Header: "NAME", Width: 20},
		{Header: "TYPE", Width: 10},
		{Header: "STATE", Width: 12},
		{Header: "TENANT", Width: 0},
	}
	rows := make([][]string, 0, len(resources))
	for _, r := range resources {
		tenant := r.TenantID
		if tenant == "" {
			tenant = r.TenantID2
		}
		id := firstNonEmpty(r.ID, r.RefID, r.Name)
		resType := firstNonEmpty(r.Type, r.Kind)
		state := firstNonEmpty(r.State, r.Status, r.Power)
		rows = append(rows, []string{
			id,
			r.Name,
			resType,
			state,
			tenant,
		})
	}

	a.printTable(columns, rows, "")
	return nil
}

func (a *App) getDisks(args []string) error {
	cfg := a.cfg
	if err := a.ensureAccess("get disks", modeTenant|modeAdmin); err != nil {
		return err
	}

	fs := flag.NewFlagSet("get disks", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	tenantFlag := fs.String("tenant", "", "tenant override (admin mode only)")
	if err := parseFlags(fs, args); err != nil {
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
		disks  []api.Resource
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
			raw, err := client.ListTenantDisksRaw(tenantID)
			if err != nil {
				return err
			}
			_, err = a.stdout.Write(raw)
			return err
		}
		disks, err = client.ListTenantDisks(tenantID)

	case "admin":
		tenantID, err := a.resolveTenantID("get disks", overrideTenantID)
		if err != nil {
			return err
		}
		client = api.NewClient(cfg.AdminServer(), cfg.Token(), cfg.Auth.Insecure)
		if a.outputFormat == "json" {
			raw, err := client.ListAdminDisksRaw(tenantID)
			if err != nil {
				return err
			}
			_, err = a.stdout.Write(raw)
			return err
		}
		disks, err = client.ListAdminDisks(tenantID)
	default:
		return fmt.Errorf("invalid mode '%s'", cfg.Mode())
	}

	if err != nil {
		return err
	}

	if len(disks) == 0 {
		if a.outputFormat == "json" {
			return nil
		}
		fmt.Fprintln(a.stdout, "No disks found.")
		return nil
	}

	if a.outputFormat == "json" {
		enc := json.NewEncoder(a.stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(disks)
	}

	columns := []tableColumn{
		{Header: "ID", Width: 36},
		{Header: "NAME", Width: 20},
		{Header: "SIZE_MB", Width: 10},
		{Header: "ATTACHED_TO", Width: 25},
		{Header: "AGENT", Width: 20},
		{Header: "STATE", Width: 12},
		{Header: "TENANT", Width: 0},
	}
	rows := make([][]string, 0, len(disks))
	for _, d := range disks {
		tenant := d.TenantID
		if tenant == "" {
			tenant = d.TenantID2
		}
		state := firstNonEmpty(d.State, d.Status, d.Power)
		size := "-"
		if d.SizeMB > 0 {
			size = fmt.Sprintf("%d", d.SizeMB)
		}
		id := firstNonEmpty(d.RefID, d.ID, d.Name)
		attached := "-"
		if d.AttachedTo != nil {
			target := firstNonEmpty(d.AttachedTo.Name, d.AttachedTo.RefID)
			if target == "" {
				target = "-"
			}
			if d.AttachedTo.AgentID != "" && target != "-" {
				target = fmt.Sprintf("%s@%s", target, d.AttachedTo.AgentID)
			}
			attached = target
		}

		rows = append(rows, []string{
			id,
			d.Name,
			size,
			attached,
			firstNonEmpty(d.AgentID),
			state,
			tenant,
		})
	}

	a.printTable(columns, rows, "")
	return nil
}

func (a *App) getVMs(args []string) error {
	cfg := a.cfg
	if err := a.ensureAccess("get vms", modeTenant|modeAdmin); err != nil {
		return err
	}

	fs := flag.NewFlagSet("get vms", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	tenantFlag := fs.String("tenant", "", "tenant override (admin mode only)")
	if err := parseFlags(fs, args); err != nil {
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
		vms    []api.VMResource
		err    error
	)

	switch cfg.Mode() {
	case "tenant":
		if strings.TrimSpace(overrideTenantID) != "" {
			return fmt.Errorf("--tenant is only supported in admin mode; tenant identity comes from credentials")
		}
		client = api.NewClient(cfg.TenantServer(), cfg.Token(), cfg.Auth.Insecure)
		tenantID := a.tenantContextID(overrideTenantID)
		vms, err = client.ListTenantVMs(tenantID)
	case "admin":
		tenantID, err := a.resolveTenantID("get vms", overrideTenantID)
		if err != nil {
			return err
		}
		client = api.NewClient(cfg.AdminServer(), cfg.Token(), cfg.Auth.Insecure)
		vms, err = client.ListAdminVMs(tenantID)
	default:
		return fmt.Errorf("invalid mode '%s'", cfg.Mode())
	}

	if err != nil {
		return err
	}

	if a.outputFormat == "json" {
		enc := json.NewEncoder(a.stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(vms)
	}

	if len(vms) == 0 {
		fmt.Fprintln(a.stdout, "No VMs found.")
		return nil
	}

	columns := []tableColumn{
		{Header: "ID", Width: 36},
		{Header: "NAME", Width: 25},
		{Header: "STATE", Width: 12},
		{Header: "VCPUS", Width: 6},
		{Header: "RAMMB", Width: 8},
		{Header: "CPU%", Width: 7},
		{Header: "AGENT", Width: 20},
		{Header: "TENANT", Width: 0},
	}
	rows := make([][]string, 0, len(vms))
	for _, vm := range vms {
		vcpus := "-"
		if vm.CPU.VCPUs != nil {
			vcpus = fmt.Sprintf("%d", *vm.CPU.VCPUs)
		}
		cpuPct := fmt.Sprintf("%.1f", vm.CPUUsagePct)
		state := firstNonEmpty(vm.State, vm.PowerState)
		id := firstNonEmpty(vm.RefID, vm.ID, vm.Name)
		rows = append(rows, []string{
			id,
			vm.Name,
			state,
			vcpus,
			fmt.Sprintf("%d", vm.MemoryMB),
			cpuPct,
			vm.AgentID,
			vm.TenantID,
		})
	}

	a.printTable(columns, rows, "")
	return nil
}

func (a *App) createDisk(args []string) error {
	if err := a.ensureAccess("create disk", modeTenant|modeAdmin); err != nil {
		return err
	}

	fs := flag.NewFlagSet("create disk", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	tenantFlag := fs.String("tenant", "", "tenant ID override (admin mode)")
	name := fs.String("name", "", "Disk name")
	agentID := fs.String("agent-id", "", "Storage connector identity")
	imageID := fs.String("image-id", "", "Image ID to seed the disk")
	sizeMB := fs.Int("size-mb", 0, "Disk size in MB")
	if err := parseFlags(fs, args); err != nil {
		return ErrUsage
	}
	if fs.NArg() > 1 {
		return ErrUsage
	}
	if fs.NArg() == 1 {
		if *name != "" {
			return fmt.Errorf("name supplied twice")
		}
		*name = fs.Arg(0)
	}

	diskName := strings.TrimSpace(*name)
	diskImageID := strings.TrimSpace(*imageID)
	if diskName == "" {
		return fmt.Errorf("--name is required")
	}
	if diskImageID == "" {
		return fmt.Errorf("--image-id is required")
	}
	if *sizeMB <= 0 {
		return fmt.Errorf("--size-mb must be greater than zero")
	}

	data := api.StorageCreateData{
		Name:    diskName,
		ImageID: diskImageID,
		SizeMB:  *sizeMB,
	}

	return a.submitResource("storage.create", *tenantFlag, api.TaskTarget{Kind: "storage", AgentID: *agentID}, data)
}

func (a *App) deleteDisk(args []string) error {
	if hasNamedResource(args) {
		return a.deleteNamedResource("disk", args)
	}
	if err := a.ensureAccess("delete disk", modeTenant|modeAdmin); err != nil {
		return err
	}

	fs := flag.NewFlagSet("delete-disk", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	tenantFlag := fs.String("tenant", "", "tenant ID override (admin mode)")
	agentID := fs.String("agent-id", "", "Storage connector identity (optional when unambiguous)")
	refID := fs.String("ref-id", "", "Disk reference ID")
	if err := parseFlags(fs, args); err != nil {
		return ErrUsage
	}
	if fs.NArg() != 0 {
		return ErrUsage
	}
	diskRefID := strings.TrimSpace(*refID)
	if diskRefID == "" {
		return fmt.Errorf("--ref-id is required")
	}

	return a.submitResource("storage.delete", *tenantFlag, api.TaskTarget{Kind: "storage", AgentID: *agentID, RefID: diskRefID}, api.StorageDeleteData{RefID: diskRefID})
}

func (a *App) createVM(args []string) error {
	if err := a.ensureAccess("create vm", modeTenant|modeAdmin); err != nil {
		return err
	}

	fs := flag.NewFlagSet("create-vm", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	tenantFlag := fs.String("tenant", "", "tenant ID override (admin mode)")
	name := fs.String("name", "", "VM name")
	generation := fs.Int("generation", 2, "VM generation")
	ram := fs.String("ram", "", "RAM amount (e.g. 4GB)")
	cpu := fs.Int("cpu", 0, "CPU count")
	diskID := fs.String("disk-id", "", "Disk reference ID to attach as OS disk")
	ha := fs.Bool("ha", false, "Enable high availability")
	agentID := fs.String("agent-id", "", "Agent/host ID override")
	cloudInitPath := fs.String("cloud-init", "", "Path to cloud-init JSON file")
	cloudHostname := fs.String("hostname", "", "Set cloud-init hostname field")
	cloudUser := fs.String("user", "", "Set cloud-init default user")
	cloudSSHKeys := fs.String("ssh-keys", "", "Comma-separated SSH public keys for cloud-init")
	cloudNetworkMode := fs.String("network-mode", "", "Cloud-init network mode (e.g. dhcp)")

	if err := parseFlags(fs, args); err != nil {
		return ErrUsage
	}
	if fs.NArg() > 1 {
		return ErrUsage
	}
	if fs.NArg() == 1 {
		if *name != "" {
			return fmt.Errorf("name supplied twice")
		}
		*name = fs.Arg(0)
	}
	if *name == "" || *ram == "" || *cpu <= 0 || *diskID == "" {
		return fmt.Errorf("--name, --ram, --cpu, and --disk-id are required")
	}

	var err error

	var cloudInit json.RawMessage
	if *cloudInitPath != "" && (strings.TrimSpace(*cloudHostname) != "" || strings.TrimSpace(*cloudUser) != "" || strings.TrimSpace(*cloudSSHKeys) != "" || strings.TrimSpace(*cloudNetworkMode) != "") {
		return fmt.Errorf("--cloud-init cannot be combined with inline cloud-init flags")
	}
	if *cloudInitPath != "" {
		cloudInit, err = loadJSONFile(*cloudInitPath)
		if err != nil {
			return fmt.Errorf("failed to read cloud-init file: %w", err)
		}
	} else {
		inline := make(map[string]any)
		if host := strings.TrimSpace(*cloudHostname); host != "" {
			inline["hostname"] = host
		}
		if user := strings.TrimSpace(*cloudUser); user != "" {
			inline["user"] = user
		}
		if keys := splitCSV(*cloudSSHKeys); len(keys) > 0 {
			inline["ssh_authorized_keys"] = keys
		}
		if mode := strings.TrimSpace(*cloudNetworkMode); mode != "" {
			inline["network"] = map[string]string{"mode": mode}
		}
		if len(inline) > 0 {
			raw, err := json.Marshal(inline)
			if err != nil {
				return fmt.Errorf("failed to build cloud-init payload: %w", err)
			}
			cloudInit = raw
		}
	}

	data := api.VMCreateData{
		Name:       *name,
		DiskID:     *diskID,
		CPU:        *cpu,
		RAM:        *ram,
		Generation: *generation,
		HighAvail:  *ha,
		CloudInit:  cloudInit,
	}

	return a.submitResource("vm.create", *tenantFlag, api.TaskTarget{Kind: "vm", AgentID: *agentID}, data)
}

func (a *App) deleteVM(args []string) error {
	if hasNamedResource(args) {
		return a.deleteNamedResource("vm", args)
	}
	if err := a.ensureAccess("delete vm", modeTenant|modeAdmin); err != nil {
		return err
	}

	fs := flag.NewFlagSet("delete-vm", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	tenantFlag := fs.String("tenant", "", "tenant ID override (admin mode)")
	refID := fs.String("ref-id", "", "Canonical VM GUID")
	guid := fs.String("guid", "", "Exact VM GUID")
	forceStop := fs.Bool("force", false, "Force stop before delete")
	agentID := fs.String("agent-id", "", "Agent/host ID override")

	if err := parseFlags(fs, args); err != nil {
		return ErrUsage
	}
	if fs.NArg() != 0 {
		return ErrUsage
	}
	if *refID == "" {
		*refID = *guid
	}
	if *guid == "" && *refID != "" {
		*guid = *refID
	}
	if *refID == "" || *guid == "" {
		return fmt.Errorf("--ref-id and --guid are required")
	}

	if !api.ValidGUID(*refID) || !strings.EqualFold(*refID, *guid) {
		return fmt.Errorf("--ref-id and --guid must identify the same canonical Hyper-V GUID")
	}
	data := api.VMDeleteData{GUID: strings.ToLower(*guid), ForceStop: *forceStop}
	return a.submitResource("vm.delete", *tenantFlag, api.TaskTarget{Kind: "vm", AgentID: *agentID, RefID: strings.ToLower(*refID)}, data)
}

func (a *App) printTaskResponse(action string, raw []byte) error {
	if a.wait {
		id, _ := parseTaskResponse(raw)
		if id == "" {
			return fmt.Errorf("acceptance response missing taskId")
		}
		fmt.Fprintf(a.stderr, "Task: %s\n", id)
		return a.waitCommand([]string{"task", id, "--timeout", a.waitTimeout.String()})
	}
	if a.outputFormat == "json" {
		raw = append(bytes.TrimSpace(raw), '\n')
		if _, err := a.stdout.Write(raw); err != nil {
			return err
		}
		return nil
	}

	id, status := parseTaskResponse(raw)
	switch {
	case id != "" && status != "":
		fmt.Fprintf(a.stdout, "%s task submitted (id=%s, status=%s)\n", action, id, status)
	case id != "":
		fmt.Fprintf(a.stdout, "%s task submitted (id=%s)\n", action, id)
	case status != "":
		fmt.Fprintf(a.stdout, "%s task submitted (status=%s)\n", action, status)
	default:
		body := strings.TrimSpace(string(raw))
		if body != "" {
			fmt.Fprintf(a.stdout, "%s task submitted. Response: %s\n", action, body)
		} else {
			fmt.Fprintf(a.stdout, "%s task submitted.\n", action)
		}
	}
	return nil
}

func parseTaskResponse(raw []byte) (string, string) {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", ""
	}
	id := firstString(obj, "id", "taskId", "task_id")
	status := firstString(obj, "status", "state")
	return id, status
}

package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"hvx-cli/internal/api"
)

func (a *App) createUser(args []string) error {
	if err := a.ensureAccess("create user", modeAdmin); err != nil {
		return err
	}

	fs := flag.NewFlagSet("create user", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	email := fs.String("email", "", "user email")
	password := fs.String("password", "", "user password")
	tenant := fs.String("tenant", "", "tenant ID override")
	rolesFlag := fs.String("roles", "", "comma-separated roles")
	if err := fs.Parse(args); err != nil {
		return ErrUsage
	}
	if *email == "" || *password == "" || *rolesFlag == "" {
		return fmt.Errorf("--email, --password, and --roles are required")
	}
	roles := splitCSV(*rolesFlag)
	if len(roles) == 0 {
		return fmt.Errorf("--roles must specify at least one role")
	}

	cfg := a.cfg
	tenantID, err := a.resolveTenantID("create user", *tenant)
	if err != nil {
		return err
	}
	client := api.NewClient(cfg.AdminServer(), cfg.Auth.PAT, cfg.Auth.Insecure)
	payload := api.TenantUserRequest{
		Email:    *email,
		Password: *password,
		TenantID: tenantID,
		Roles:    roles,
	}
	raw, err := client.RegisterTenantUser(payload)
	if err != nil {
		return err
	}

	if a.outputFormat == "json" {
		raw = append(bytes.TrimSpace(raw), '\n')
		_, err = a.stdout.Write(raw)
		return err
	}

	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		fmt.Fprintf(a.stdout, "User %s created for tenant %s.\n", *email, tenantID)
		return nil
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, trimmed, "", "  "); err != nil {
		fmt.Fprintf(a.stdout, "User %s created for tenant %s. Response: %s\n", *email, tenantID, trimmed)
		return nil
	}
	fmt.Fprintf(a.stdout, "User %s created for tenant %s:\n", *email, tenantID)
	pretty.WriteByte('\n')
	_, err = a.stdout.Write(pretty.Bytes())
	return err
}

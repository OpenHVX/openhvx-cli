package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"hvx-cli/internal/api"
)

func (a *App) createTask(args []string) error {
	if err := a.ensureAccess("create task", modeTenant|modeAdmin); err != nil {
		return err
	}
	fs := flag.NewFlagSet("create task", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	path := fs.String("file", "", "HTTP task JSON file, or - for stdin")
	fs.StringVar(path, "f", "", "HTTP task JSON file")
	tenant := fs.String("tenant", "", "Admin tenant override")
	if err := parseFlags(fs, args); err != nil || fs.NArg() != 0 {
		return ErrUsage
	}
	if *path == "" {
		return fmt.Errorf("--file is required")
	}
	var reader io.Reader = os.Stdin
	if *path != "-" {
		f, err := os.Open(*path)
		if err != nil {
			return err
		}
		defer f.Close()
		reader = f
	}
	raw, err := io.ReadAll(io.LimitReader(reader, (1<<20)+1))
	if err != nil {
		return err
	}
	if len(raw) > 1<<20 {
		return fmt.Errorf("task file exceeds 1 MiB")
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil || payload == nil {
		return fmt.Errorf("task must be a JSON object")
	}
	var action string
	if json.Unmarshal(payload["action"], &action) != nil || action == "" {
		return fmt.Errorf("task action is required")
	}
	server := a.cfg.TenantServer()
	if a.cfg.Mode() == "admin" {
		server = a.cfg.AdminServer()
		var fromFile string
		if payload["tenantId"] != nil && json.Unmarshal(payload["tenantId"], &fromFile) != nil {
			return fmt.Errorf("tenantId must be a string")
		}
		if *tenant != "" && fromFile != "" && *tenant != fromFile {
			return fmt.Errorf("--tenant conflicts with task tenantId")
		}
		if fromFile == "" {
			fromFile, err = a.resolveTenantID("create task", *tenant)
			if err != nil {
				return err
			}
		}
		payload["tenantId"], _ = json.Marshal(fromFile)
	} else if *tenant != "" {
		return fmt.Errorf("--tenant is only supported in admin mode")
	}
	raw, err = json.Marshal(payload)
	if err != nil {
		return err
	}
	client := api.NewClient(server, a.cfg.Token(), a.cfg.Auth.Insecure)
	client.IdempotencyKey = a.idempotencyKey
	a.trackTasks(client)
	result, err := client.SubmitTask(a.cfg.Mode(), raw)
	if err != nil {
		return err
	}
	return a.printTaskResponse(action, result)
}

func (a *App) fetchTask(id string) ([]byte, error) {
	if a.cfg.Mode() == "admin" {
		return api.NewClient(a.cfg.AdminServer(), a.cfg.Token(), a.cfg.Auth.Insecure).GetAdminTask(id)
	}
	return api.NewClient(a.cfg.TenantServer(), a.cfg.Token(), a.cfg.Auth.Insecure).GetTenantTask(id)
}

func (a *App) waitCommand(args []string) error {
	if len(args) == 0 || args[0] != "task" {
		return ErrUsage
	}
	if err := a.ensureAccess("wait task", modeTenant|modeAdmin); err != nil {
		return err
	}
	fs := flag.NewFlagSet("wait task", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	id := fs.String("id", "", "Task ID")
	timeout := fs.Duration("timeout", a.waitTimeout, "Overall polling duration")
	interval := fs.Duration("interval", 2*time.Second, "Polling interval")
	rest := args[1:]
	if len(rest) > 0 && rest[0] != "" && rest[0][0] != '-' {
		*id = rest[0]
		rest = rest[1:]
	}
	if err := parseFlags(fs, rest); err != nil || fs.NArg() != 0 {
		return ErrUsage
	}
	if *id == "" || *timeout <= 0 || *interval <= 0 {
		return fmt.Errorf("task ID and positive timeout/interval are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	server := a.cfg.TenantServer()
	if a.cfg.Mode() == "admin" {
		server = a.cfg.AdminServer()
	}
	client := api.NewClient(server, a.cfg.Token(), a.cfg.Auth.Insecure)
	client.Context = ctx
	deadline := time.Now().Add(*timeout)
	for {
		var raw []byte
		var err error
		if a.cfg.Mode() == "admin" {
			raw, err = client.GetAdminTask(*id)
		} else {
			raw, err = client.GetTenantTask(*id)
		}
		if err != nil {
			return err
		}
		_, task, err := decodeTaskResponse(raw)
		if err != nil {
			return err
		}
		switch task.Status {
		case "done", "error":
			if _, err := a.stdout.Write(append(bytes.TrimSpace(raw), '\n')); err != nil {
				return err
			}
			if task.Status == "error" {
				return fmt.Errorf("task %s failed", *id)
			}
			return nil
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("timed out waiting for task %s (status=%s); no task was resubmitted", *id, task.Status)
		}
		delay := *interval
		if delay > remaining {
			delay = remaining
		}
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("timed out waiting for task %s (status=%s); no task was resubmitted", *id, task.Status)
		}
	}
}

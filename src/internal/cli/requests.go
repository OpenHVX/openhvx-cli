package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"hvx-cli/internal/api"
	"hvx-cli/internal/requests"
)

func (a *App) trackTasks(client *api.Client) {
	client.TaskSender = func(path string, body []byte) ([]byte, error) {
		if key := strings.TrimSpace(a.idempotencyKey); a.idempotencyKey != "" && (len(key) < 1 || len(key) > 200 || strings.ContainsAny(key, "\r\n")) {
			return nil, fmt.Errorf("invalid idempotency key")
		}
		mode := "tenant"
		if path == "/api/v1/admin/tasks" {
			mode = "admin"
		}
		r, err := requests.New(client.BaseURL, mode, client.PAT, a.cfg.Auth.Insecure, body, a.idempotencyKey)
		if err != nil {
			return nil, fmt.Errorf("save request before sending: %w", err)
		}
		fmt.Fprintf(a.stderr, "Request: %s (retry: hvx retry %s)\n", r.ID, r.ID)
		return a.sendRecorded(r)
	}
}
func (a *App) sendRecorded(r *requests.Record) ([]byte, error) {
	server := a.cfg.TenantServer()
	if a.cfg.Mode() == "admin" {
		server = a.cfg.AdminServer()
	}
	if r.Server != strings.TrimRight(server, "/") || r.Mode != a.cfg.Mode() || r.CredentialDigest != requests.CredentialDigest(a.cfg.Token()) || r.Insecure != a.cfg.Auth.Insecure {
		return nil, fmt.Errorf("request belongs to different endpoint, credentials, mode, or TLS settings; restore its original context")
	}
	unlock, err := requests.Lock(r.ID)
	if err != nil {
		return nil, err
	}
	defer unlock()
	r, err = requests.Load(r.ID)
	if err != nil {
		return nil, err
	}
	if r.State == "accepted" {
		return r.Response, nil
	}
	// Record uncertainty before transmission: a crash or malformed reply must not
	// make a possibly accepted request look safely unsent.
	r.State = "uncertain"
	if err := requests.Save(r); err != nil {
		return nil, fmt.Errorf("persist request before transmission: %w", err)
	}
	client := api.NewClient(r.Server, a.cfg.Token(), r.Insecure)
	response, sendErr := client.SendRecordedTask("/api/v1/"+r.Mode+"/tasks", r.Body, r.Key)
	if sendErr != nil {
		r.State = "uncertain"
		if err := requests.Save(r); err != nil {
			return nil, fmt.Errorf("submission uncertain and local update failed; retain request %s: %w", r.ID, err)
		}
		return nil, fmt.Errorf("request %s uncertain; retry with hvx retry %s: %w", r.ID, r.ID, sendErr)
	}
	var accepted struct {
		TaskID string `json:"taskId"`
	}
	if err := json.Unmarshal(response, &accepted); err != nil || accepted.TaskID == "" {
		return nil, fmt.Errorf("invalid acceptance response; retain request %s and retry explicitly", r.ID)
	}
	r.Response = response
	r.TaskID = accepted.TaskID
	r.State = "accepted"
	if err := requests.Save(r); err != nil {
		return nil, fmt.Errorf("accepted task %s; save failed, retain request %s: %w", r.TaskID, r.ID, err)
	}
	return response, nil
}
func (a *App) retryRequest(args []string) error {
	if len(args) != 1 {
		return ErrUsage
	}
	if a.idempotencyKey != "" {
		return fmt.Errorf("retry uses the saved key; do not supply --idempotency-key")
	}
	if err := a.ensureAccess("retry", modeTenant|modeAdmin); err != nil {
		return err
	}
	r, err := requests.Load(args[0])
	if err != nil {
		return err
	}
	raw, err := a.sendRecorded(r)
	if err != nil {
		return err
	}
	var body struct {
		Action string `json:"action"`
	}
	_ = json.Unmarshal(r.Body, &body)
	return a.printTaskResponse(body.Action, raw)
}
func (a *App) getRequests(args []string) error {
	if len(args) != 0 {
		return ErrUsage
	}
	records, err := requests.List()
	if err != nil {
		return err
	}
	type summary struct {
		ID     string `json:"requestId"`
		State  string `json:"state"`
		TaskID string `json:"taskId,omitempty"`
		Server string `json:"server"`
		Mode   string `json:"mode"`
	}
	result := []summary{}
	for _, r := range records {
		result = append(result, summary{r.ID, r.State, r.TaskID, r.Server, r.Mode})
	}
	if a.outputFormat == "json" {
		return json.NewEncoder(a.stdout).Encode(result)
	}
	for _, r := range result {
		fmt.Fprintf(a.stdout, "%s\t%s\t%s\t%s\n", r.ID, r.State, stringOrDash(r.TaskID), r.Server)
	}
	return nil
}
func (a *App) getTasks(args []string) error {
	if len(args) != 0 {
		return ErrUsage
	}
	if err := a.ensureAccess("get tasks", modeTenant|modeAdmin); err != nil {
		return err
	}
	records, err := requests.List()
	if err != nil {
		return err
	}
	server := a.cfg.TenantServer()
	if a.cfg.Mode() == "admin" {
		server = a.cfg.AdminServer()
	}
	if a.outputFormat != "json" {
		fmt.Fprintln(a.stderr, "Tasks tracked locally for the active endpoint and credentials:")
	}
	result := []json.RawMessage{}
	seen := map[string]bool{}
	for _, r := range records {
		if r.Server != strings.TrimRight(server, "/") || r.Mode != a.cfg.Mode() || r.CredentialDigest != requests.CredentialDigest(a.cfg.Token()) || r.TaskID == "" || seen[r.TaskID] {
			continue
		}
		seen[r.TaskID] = true
		raw, err := a.fetchTask(r.TaskID)
		if err != nil {
			return err
		}
		if a.outputFormat == "json" {
			result = append(result, json.RawMessage(raw))
			continue
		}
		_, task, err := decodeTaskResponse(raw)
		if err != nil {
			return err
		}
		fmt.Fprintf(a.stdout, "%s\t%s\t%s\n", task.TaskID, task.Action, task.Status)
	}
	if a.outputFormat == "json" {
		return json.NewEncoder(a.stdout).Encode(result)
	}
	return nil
}

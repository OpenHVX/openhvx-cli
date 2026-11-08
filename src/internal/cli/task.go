package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"hvx-cli/internal/api"
)

func (a *App) describeTask(args []string) error {
	if err := a.ensureAccess("describe task", modeTenant|modeAdmin); err != nil {
		return err
	}

	fs := flag.NewFlagSet("describe task", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	idFlag := fs.String("id", "", "task ID")
	if err := fs.Parse(args); err != nil {
		return ErrUsage
	}
	if fs.NArg() > 1 {
		return ErrUsage
	}

	taskID := strings.TrimSpace(*idFlag)
	if taskID == "" {
		if fs.NArg() == 0 {
			return fmt.Errorf("TASK_ID is required")
		}
		taskID = strings.TrimSpace(fs.Arg(0))
	}
	if taskID == "" {
		return fmt.Errorf("TASK_ID is required")
	}

	raw, err := a.fetchTask(taskID)
	if err != nil {
		return err
	}

	if a.outputFormat == "json" {
		raw = append(bytes.TrimSpace(raw), '\n')
		_, err = a.stdout.Write(raw)
		return err
	}

	var pretty bytes.Buffer
	trimmed := bytes.TrimSpace(raw)
	if err := json.Indent(&pretty, trimmed, "", "  "); err != nil {
		if len(trimmed) > 0 {
			trimmed = append(trimmed, '\n')
			_, err = a.stdout.Write(trimmed)
			return err
		}
		fmt.Fprintln(a.stdout, "No task data returned.")
		return nil
	}
	pretty.WriteByte('\n')
	_, err = a.stdout.Write(pretty.Bytes())
	return err
}

func (a *App) getTask(args []string) error {
	if err := a.ensureAccess("get task", modeTenant|modeAdmin); err != nil {
		return err
	}

	fs := flag.NewFlagSet("get task", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	idFlag := fs.String("id", "", "task ID")
	if err := fs.Parse(args); err != nil {
		return ErrUsage
	}
	if fs.NArg() > 1 {
		return ErrUsage
	}

	taskID := strings.TrimSpace(*idFlag)
	if taskID == "" && fs.NArg() == 1 {
		taskID = strings.TrimSpace(fs.Arg(0))
	}
	if taskID == "" {
		return fmt.Errorf("TASK_ID is required")
	}

	cfg := a.cfg
	var (
		raw []byte
		err error
	)

	switch cfg.Mode() {
	case "tenant":
		client := api.NewClient(cfg.TenantServer(), cfg.Token(), cfg.Auth.Insecure)
		raw, err = client.GetTenantTask(taskID)
	case "admin":
		client := api.NewClient(cfg.AdminServer(), cfg.Token(), cfg.Auth.Insecure)
		raw, err = client.GetAdminTask(taskID)
	default:
		return fmt.Errorf("invalid profile.mode in config.json (use 'tenant' or 'admin')")
	}
	if err != nil {
		return err
	}

	if a.outputFormat == "json" {
		raw = append(bytes.TrimSpace(raw), '\n')
		_, err = a.stdout.Write(raw)
		return err
	}

	env, record, err := decodeTaskResponse(raw)
	if err != nil {
		return a.printTaskRawFallback(raw)
	}

	payloadMap := decodeRawMap(record.DataRaw)
	targetMap := extractNestedMap(payloadMap, "target")

	resolvedTaskID := firstNonEmpty(strings.TrimSpace(record.TaskID), strings.TrimSpace(record.ID))
	action := firstNonEmpty(strings.TrimSpace(record.Action), firstString(payloadMap, "action"))
	status := strings.TrimSpace(record.Status)
	if status == "" {
		status = "unknown"
	}
	status = strings.ToUpper(status)

	targetKind := strings.TrimSpace(getTargetField(record.Target, "kind"))
	if targetKind == "" {
		targetKind = strings.TrimSpace(firstString(targetMap, "kind", "type"))
	}
	targetRef := strings.TrimSpace(getTargetField(record.Target, "refId"))
	if targetRef == "" {
		targetRef = strings.TrimSpace(firstString(targetMap, "refId", "refID", "name", "id"))
	}
	targetDesc := "-"
	if targetKind != "" || targetRef != "" {
		var parts []string
		if targetKind != "" {
			parts = append(parts, targetKind)
		}
		if targetRef != "" {
			parts = append(parts, targetRef)
		}
		targetDesc = strings.Join(parts, " / ")
	}

	tenantID := firstNonEmpty(strings.TrimSpace(record.TenantID), firstString(payloadMap, "tenantId", "tenant_id"))
	agentID := firstNonEmpty(
		strings.TrimSpace(record.AgentID),
		strings.TrimSpace(getTargetField(record.Target, "agentId")),
		strings.TrimSpace(firstString(targetMap, "agentId", "agentID")),
	)

	columns := []tableColumn{
		{Header: "TASK", Width: 36},
		{Header: "ACTION", Width: 16},
		{Header: "STATUS", Width: 12},
		{Header: "TARGET", Width: 20},
		{Header: "TENANT", Width: 16},
		{Header: "AGENT", Width: 0},
	}
	rows := [][]string{{
		stringOrDash(resolvedTaskID),
		stringOrDash(action),
		stringOrDash(status),
		stringOrDash(targetDesc),
		stringOrDash(tenantID),
		stringOrDash(agentID),
	}}
	a.printTable(columns, rows, "")
	fmt.Fprintln(a.stdout)

	scope := strings.TrimSpace(env.Scope)
	if scope == "" {
		scope = strings.ToLower(a.cfg.Mode())
	}
	kind := strings.TrimSpace(env.Kind)
	schema := strings.TrimSpace(env.SchemaVersion)

	fmt.Fprintf(a.stdout, "Scope: %s   Kind: %s   Schema: %s\n",
		stringOrDash(scope), stringOrDash(kind), stringOrDash(schema))
	if record.CorrelationID != "" {
		fmt.Fprintf(a.stdout, "Correlation: %s\n", record.CorrelationID)
	}
	if record.RoutingKey != "" {
		fmt.Fprintf(a.stdout, "Routing Key: %s\n", record.RoutingKey)
	}
	fmt.Fprintf(a.stdout, "Quota Hold: %t\n", record.HasQuotaHold)
	if record.ContractVersion > 0 {
		fmt.Fprintf(a.stdout, "Protocol: v%d\n", record.ContractVersion)
	}
	if record.Outcome != "" {
		fmt.Fprintf(a.stdout, "Outcome: %s\n", record.Outcome)
	}
	if err := a.printTaskJSONBlock("Delivery", record.Delivery); err != nil {
		return err
	}

	fmt.Fprintln(a.stdout, "\nTimeline:")
	fmt.Fprintf(a.stdout, "  Queued:    %s\n", formatAPITime(record.QueuedAt))
	fmt.Fprintf(a.stdout, "  Created:   %s\n", formatAPITime(record.CreatedAt))
	fmt.Fprintf(a.stdout, "  Published: %s\n", formatAPITime(record.PublishedAt))
	fmt.Fprintf(a.stdout, "  Updated:   %s\n", formatAPITime(record.UpdatedAt))
	fmt.Fprintf(a.stdout, "  Finished:  %s\n", formatAPITime(record.FinishedAt))

	if errMsg := strings.TrimSpace(record.Error); errMsg != "" {
		fmt.Fprintf(a.stdout, "\nError: %s\n", errMsg)
	}

	if err := a.printTaskJSONBlock("Payload", record.DataRaw); err != nil {
		return err
	}
	if err := a.printTaskJSONBlock("Result", record.ResultRaw); err != nil {
		return err
	}
	return nil
}

type taskEnvelope struct {
	SchemaVersion string          `json:"schemaVersion"`
	Scope         string          `json:"scope"`
	Kind          string          `json:"kind"`
	Success       bool            `json:"success"`
	Data          json.RawMessage `json:"data"`
}

type taskRecord struct {
	ID              string          `json:"_id"`
	TaskID          string          `json:"taskId"`
	TenantID        string          `json:"tenantId"`
	AgentID         string          `json:"agentId"`
	Action          string          `json:"action"`
	Status          string          `json:"status"`
	Outcome         string          `json:"outcome"`
	ContractVersion int             `json:"contractVersion"`
	Delivery        json.RawMessage `json:"delivery"`
	CorrelationID   string          `json:"correlationId"`
	QueuedAt        string          `json:"queuedAt"`
	CreatedAt       string          `json:"createdAt"`
	UpdatedAt       string          `json:"updatedAt"`
	PublishedAt     string          `json:"publishedAt"`
	FinishedAt      string          `json:"finishedAt"`
	Error           string          `json:"error"`
	ResultRaw       json.RawMessage `json:"result"`
	RoutingKey      string          `json:"routingKey"`
	HasQuotaHold    bool            `json:"hasQuotaHold"`
	DataRaw         json.RawMessage `json:"data"`
	Target          *api.TaskTarget `json:"target"`
}

func decodeTaskResponse(raw []byte) (taskEnvelope, taskRecord, error) {
	var env taskEnvelope
	recordRaw := raw
	if err := json.Unmarshal(raw, &env); err == nil {
		var top map[string]json.RawMessage
		_ = json.Unmarshal(raw, &top)
		if len(env.Data) > 0 && top["taskId"] == nil && top["_id"] == nil {
			recordRaw = env.Data
		} else {
			env = taskEnvelope{}
		}
	} else {
		env = taskEnvelope{}
	}

	var record taskRecord
	if err := json.Unmarshal(recordRaw, &record); err != nil {
		return env, taskRecord{}, err
	}
	if strings.TrimSpace(record.TaskID) == "" && strings.TrimSpace(record.ID) == "" {
		return env, taskRecord{}, fmt.Errorf("task payload missing identifiers")
	}
	return env, record, nil
}

func (a *App) printTaskRawFallback(raw []byte) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		fmt.Fprintln(a.stdout, "No task data returned.")
		return nil
	}
	trimmed = append(trimmed, '\n')
	_, err := a.stdout.Write(trimmed)
	return err
}

func decodeRawMap(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	return obj
}

func extractNestedMap(obj map[string]any, key string) map[string]any {
	if obj == nil {
		return nil
	}
	raw, ok := obj[key]
	if !ok {
		return nil
	}
	if nested, ok := raw.(map[string]any); ok {
		return nested
	}
	return nil
}

func getTargetField(target *api.TaskTarget, field string) string {
	if target == nil {
		return ""
	}
	switch field {
	case "kind":
		return target.Kind
	case "refId":
		return target.RefID
	case "agentId":
		return target.AgentID
	default:
		return ""
	}
}

func stringOrDash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}

func formatAPITime(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	if ts, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return ts.UTC().Format("2006-01-02 15:04:05Z07:00")
	}
	return value
}

func (a *App) printTaskJSONBlock(label string, raw json.RawMessage) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	fmt.Fprintf(a.stdout, "\n%s:\n", label)
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, trimmed, "", "  "); err != nil {
		trimmed = append(trimmed, '\n')
		_, err = a.stdout.Write(trimmed)
		return err
	}
	pretty.WriteByte('\n')
	_, err := a.stdout.Write(pretty.Bytes())
	return err
}

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func splitCSV(input string) []string {
	parts := strings.Split(input, ",")
	var values []string
	for _, part := range parts {
		val := strings.TrimSpace(part)
		if val == "" {
			continue
		}
		values = append(values, val)
	}
	return values
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			if s, ok := v.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					return s
				}
			}
		}
	}
	return ""
}

func loadJSONFile(path string) (json.RawMessage, error) {
	if path == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return nil, fmt.Errorf("file is empty")
	}
	var raw json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

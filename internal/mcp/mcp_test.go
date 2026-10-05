package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

type stubAPI struct {
	ro bool
}

func (s stubAPI) OursStatus() (any, error)       { return map[string]any{"jobs": []any{}}, nil }
func (s stubAPI) DriftList() (any, error)        { return map[string]any{"drifts": []any{}}, nil }
func (s stubAPI) JobIncident(float64) (any, error) {
	return map[string]any{"events": []any{}}, nil
}
func (s stubAPI) JobAction(label, action string, confirm bool) (any, error) {
	return map[string]any{"label": label, "action": action, "confirm": confirm, "dry_run": !confirm}, nil
}
func (s stubAPI) MetricsSnapshot() (any, error) { return map[string]any{"text": "deployboard_up 1\n"}, nil }
func (s stubAPI) ReadOnly() bool                { return s.ro }

func TestToolsListAndCall(t *testing.T) {
	in := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"job_action","arguments":{"label":"com.example.web","action":"start"}}}
`)
	var out bytes.Buffer
	srv := &Server{In: in, Out: &out, API: stubAPI{}}
	if err := srv.Run(); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines: %s", len(lines), out.String())
	}
	var list struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Result.Tools) != 5 {
		t.Fatalf("tools=%d", len(list.Result.Tools))
	}
	var call struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[2]), &call); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(call.Result.Content[0].Text, `"dry_run": true`) {
		t.Fatalf("expected dry_run: %s", call.Result.Content[0].Text)
	}
}

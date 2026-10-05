// Package mcp implements a minimal JSON-RPC 2.0 MCP stdio server for Ours-only
// Deployboard tools (Phase 4). No third-party MCP SDK — stdlib only.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// Server handles MCP JSON-RPC over a reader/writer.
type Server struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
	API API
}

// API is the Deployboard surface MCP tools call.
type API interface {
	OursStatus() (any, error)
	DriftList() (any, error)
	JobIncident(hours float64) (any, error)
	JobAction(label, action string, confirm bool) (any, error)
	MetricsSnapshot() (any, error)
	ReadOnly() bool
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Run processes newline-delimited JSON-RPC until EOF.
func (s *Server) Run() error {
	in := s.In
	if in == nil {
		in = os.Stdin
	}
	out := s.Out
	if out == nil {
		out = os.Stdout
	}
	errOut := s.Err
	if errOut == nil {
		errOut = os.Stderr
	}
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	enc := json.NewEncoder(out)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			fmt.Fprintln(errOut, "mcp: bad json:", err)
			continue
		}
		resp := s.handle(req)
		if resp == nil {
			continue // notification
		}
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	return sc.Err()
}

func (s *Server) handle(req rpcRequest) *rpcResponse {
	reply := func(result any, err error) *rpcResponse {
		if len(req.ID) == 0 || string(req.ID) == "null" {
			return nil
		}
		r := &rpcResponse{JSONRPC: "2.0", ID: req.ID}
		if err != nil {
			r.Error = &rpcError{Code: -32000, Message: err.Error()}
			return r
		}
		r.Result = result
		return r
	}

	switch req.Method {
	case "initialize":
		return reply(map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "deployboard", "version": "phase4"},
		}, nil)
	case "notifications/initialized", "initialized":
		return nil
	case "tools/list":
		return reply(map[string]any{"tools": toolSchemas()}, nil)
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return reply(nil, err)
		}
		text, err := s.callTool(p.Name, p.Arguments)
		if err != nil {
			return reply(map[string]any{
				"content": []map[string]any{{"type": "text", "text": err.Error()}},
				"isError": true,
			}, nil)
		}
		return reply(map[string]any{
			"content": []map[string]any{{"type": "text", "text": text}},
		}, nil)
	case "ping":
		return reply(map[string]any{}, nil)
	default:
		if len(req.ID) == 0 || string(req.ID) == "null" {
			return nil
		}
		return &rpcResponse{
			JSONRPC: "2.0", ID: req.ID,
			Error: &rpcError{Code: -32601, Message: "method not found: " + req.Method},
		}
	}
}

func toolSchemas() []map[string]any {
	return []map[string]any{
		{
			"name":        "ours_status",
			"description": "List Ours launchd jobs with status, group, and disabled flag.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "drift_list",
			"description": "List desired-state drifts for Ours jobs.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "job_incident",
			"description": "Recent Ours incident timeline and crash fingerprints.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"hours": map[string]any{"type": "number", "description": "Lookback hours (default 24)"},
				},
			},
		},
		{
			"name":        "job_action",
			"description": "Mutate an Ours job (start/stop/enable/disable/reload). Dry-run unless confirm=true. Honors read-only.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"label":   map[string]any{"type": "string"},
					"action":  map[string]any{"type": "string", "enum": []string{"start", "stop", "enable", "disable", "reload"}},
					"confirm": map[string]any{"type": "boolean"},
				},
				"required": []string{"label", "action"},
			},
		},
		{
			"name":        "metrics_snapshot",
			"description": "Point-in-time Prometheus text exposition from Deployboard.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
	}
}

func (s *Server) callTool(name string, args map[string]any) (string, error) {
	if args == nil {
		args = map[string]any{}
	}
	var (
		result any
		err    error
	)
	switch name {
	case "ours_status":
		result, err = s.API.OursStatus()
	case "drift_list":
		result, err = s.API.DriftList()
	case "job_incident":
		hours := 24.0
		if v, ok := args["hours"].(float64); ok && v > 0 {
			hours = v
		}
		result, err = s.API.JobIncident(hours)
	case "job_action":
		label, _ := args["label"].(string)
		action, _ := args["action"].(string)
		confirm, _ := args["confirm"].(bool)
		result, err = s.API.JobAction(label, action, confirm)
	case "metrics_snapshot":
		result, err = s.API.MetricsSnapshot()
	default:
		return "", fmt.Errorf("unknown tool %q", name)
	}
	if err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

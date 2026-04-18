package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	serverName    = "ws-dev"
	serverVersion = "0.1.0"
	protocolVer   = "2024-11-05"
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolResult struct {
	Content []textContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type Server struct {
	LogDir string // absolute path to log directory
	in     *bufio.Reader
	out    io.Writer
}

func NewServer(logDir string) *Server {
	return &Server{
		LogDir: logDir,
		in:     bufio.NewReader(os.Stdin),
		out:    os.Stdout,
	}
}

func (s *Server) Run() error {
	dec := json.NewDecoder(s.in)
	for {
		var req rpcRequest
		if err := dec.Decode(&req); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if req.Method == "" {
			continue
		}
		if strings.HasPrefix(req.Method, "notifications/") {
			continue
		}
		resp := s.handle(req)
		if resp != nil {
			if err := s.write(resp); err != nil {
				return err
			}
		}
	}
}

func (s *Server) write(resp *rpcResponse) error {
	resp.JSONRPC = "2.0"
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = s.out.Write(data)
	return err
}

func (s *Server) handle(req rpcRequest) *rpcResponse {
	switch req.Method {
	case "initialize":
		return &rpcResponse{ID: req.ID, Result: map[string]any{
			"protocolVersion": protocolVer,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": serverName, "version": serverVersion},
		}}
	case "tools/list":
		return &rpcResponse{ID: req.ID, Result: map[string]any{"tools": toolDefs()}}
	case "tools/call":
		return s.callTool(req)
	case "ping":
		return &rpcResponse{ID: req.ID, Result: map[string]any{}}
	default:
		return &rpcResponse{ID: req.ID, Error: &rpcError{Code: -32601, Message: "method not found: " + req.Method}}
	}
}

func toolDefs() []map[string]any {
	return []map[string]any{
		{
			"name":        "list_logs",
			"description": "List *.log files in the configured log directory (name, size, mtime).",
			"inputSchema": map[string]any{
				"type":                 "object",
				"properties":           map[string]any{},
				"additionalProperties": false,
			},
		},
		{
			"name":        "tail_log",
			"description": "Return the last N lines of <name>.log.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":  map[string]any{"type": "string"},
					"lines": map[string]any{"type": "number", "default": 100},
				},
				"required":             []string{"name"},
				"additionalProperties": false,
			},
		},
		{
			"name":        "truncate_log",
			"description": "Truncate <name>.log in place to 0 bytes.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
				"required":             []string{"name"},
				"additionalProperties": false,
			},
		},
		{
			"name":        "search_log",
			"description": "Regex-search <name>.log with streaming (RE2 syntax).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":        map[string]any{"type": "string"},
					"pattern":     map[string]any{"type": "string"},
					"max_matches": map[string]any{"type": "number", "default": 50},
					"context":     map[string]any{"type": "number", "default": 0},
					"ignore_case": map[string]any{"type": "boolean", "default": false},
				},
				"required":             []string{"name", "pattern"},
				"additionalProperties": false,
			},
		},
	}
}

type callParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (s *Server) callTool(req rpcRequest) *rpcResponse {
	var p callParams
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return &rpcResponse{ID: req.ID, Error: &rpcError{Code: -32602, Message: err.Error()}}
		}
	}
	args := map[string]any{}
	if len(p.Arguments) > 0 {
		_ = json.Unmarshal(p.Arguments, &args)
	}

	switch p.Name {
	case "list_logs":
		logs, err := ListLogs(s.LogDir)
		if err != nil {
			return errResult(req.ID, err)
		}
		b, _ := json.MarshalIndent(logs, "", "  ")
		return textResult(req.ID, string(b))

	case "tail_log":
		name, _ := args["name"].(string)
		if name == "" {
			return errResult(req.ID, fmt.Errorf("name is required"))
		}
		lines := 100
		if v, ok := args["lines"].(float64); ok {
			lines = int(v)
		}
		out, err := TailLog(s.LogDir, name, lines)
		if err != nil {
			return errResult(req.ID, err)
		}
		text := fmt.Sprintf("# %s (%d bytes, last %d lines)\n\n%s", out.Path, out.Bytes, lines, out.Content)
		return textResult(req.ID, text)

	case "truncate_log":
		name, _ := args["name"].(string)
		if name == "" {
			return errResult(req.ID, fmt.Errorf("name is required"))
		}
		out, err := TruncateLog(s.LogDir, name)
		if err != nil {
			return errResult(req.ID, err)
		}
		return textResult(req.ID, fmt.Sprintf("Truncated %s (freed %d bytes)", out.Path, out.BytesFreed))

	case "search_log":
		name, _ := args["name"].(string)
		pattern, _ := args["pattern"].(string)
		if name == "" {
			return errResult(req.ID, fmt.Errorf("name is required"))
		}
		if pattern == "" {
			return errResult(req.ID, fmt.Errorf("pattern is required"))
		}
		opts := SearchOpts{MaxMatches: 50, IgnoreCase: false, Context: 0}
		if v, ok := args["max_matches"].(float64); ok {
			opts.MaxMatches = int(v)
		}
		if v, ok := args["context"].(float64); ok {
			opts.Context = int(v)
		}
		if v, ok := args["ignore_case"].(bool); ok {
			opts.IgnoreCase = v
		}
		res, err := SearchLog(s.LogDir, name, pattern, opts)
		if err != nil {
			return errResult(req.ID, err)
		}
		return textResult(req.ID, FormatSearchResult(res, pattern, opts))

	default:
		return errResult(req.ID, fmt.Errorf("unknown tool: %s", p.Name))
	}
}

func textResult(id json.RawMessage, text string) *rpcResponse {
	return &rpcResponse{ID: id, Result: toolResult{Content: []textContent{{Type: "text", Text: text}}}}
}

func errResult(id json.RawMessage, err error) *rpcResponse {
	return &rpcResponse{ID: id, Result: toolResult{Content: []textContent{{Type: "text", Text: "Error: " + err.Error()}}, IsError: true}}
}

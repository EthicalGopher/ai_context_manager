// Package mcp is a minimal Model Context Protocol server over stdio
// (newline-delimited JSON-RPC 2.0) exposing AICW's tools to agent CLIs.
package mcp

import (
	"bufio"
	"encoding/json"
	"io"

	"aicw/internal/tools"
)

const defaultProtocol = "2024-11-05"

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// Serve reads requests from in and writes responses to out until EOF.
func Serve(in io.Reader, out io.Writer, env tools.Env, version string) error {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 1<<20), 8<<20)
	enc := json.NewEncoder(out)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			enc.Encode(response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{-32700, "parse error"}})
			continue
		}
		if len(req.ID) == 0 { // notification: no reply
			continue
		}
		res := response{JSONRPC: "2.0", ID: req.ID}
		switch req.Method {
		case "initialize":
			var p struct {
				ProtocolVersion string `json:"protocolVersion"`
			}
			json.Unmarshal(req.Params, &p)
			if p.ProtocolVersion == "" {
				p.ProtocolVersion = defaultProtocol
			}
			res.Result = map[string]any{
				"protocolVersion": p.ProtocolVersion,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "aicw", "version": version},
				"instructions":    "AI Context Workspace tools. Call read_context at the start of every session and keep notes current.",
			}
		case "ping":
			res.Result = map[string]any{}
		case "tools/list":
			var list []map[string]any
			for _, t := range tools.All() {
				list = append(list, map[string]any{"name": t.Name, "description": t.Description, "inputSchema": t.Schema})
			}
			res.Result = map[string]any{"tools": list}
		case "tools/call":
			var p struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			if err := json.Unmarshal(req.Params, &p); err != nil {
				res.Error = &rpcError{-32602, "invalid params"}
				break
			}
			text, err := tools.Call(env, p.Name, p.Arguments)
			if err != nil {
				res.Result = map[string]any{"content": []map[string]any{{"type": "text", "text": err.Error()}}, "isError": true}
			} else {
				res.Result = map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}
			}
		default:
			res.Error = &rpcError{-32601, "method not found: " + req.Method}
		}
		if err := enc.Encode(res); err != nil {
			return err
		}
	}
	return sc.Err()
}

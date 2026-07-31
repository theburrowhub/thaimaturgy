// Package mcptools serves the adventure/session tools (thaimaturgy's ToolRouter)
// over the Model Context Protocol on stdio, so the official Claude Code CLI can
// call them during an oracle turn. It depends only on internal/types (not engine)
// to avoid an import cycle; the executor is passed in as an interface.
package mcptools

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"

	"github.com/theburrowhub/thaimaturgy/internal/types"
)

const (
	// ServerName is the MCP server name; tools appear to the CLI as
	// mcp__thaim__<tool>.
	ServerName = "thaim"
	// SubcommandArg is the first CLI argument that puts a thaimaturgy binary into
	// "serve MCP tools over stdio" mode instead of launching its UI.
	SubcommandArg = "__mcp-tools"
)

// ToolProvider is the subset of engine.ToolRouter this server needs. Declaring it
// here (rather than importing engine) keeps the dependency one-way.
type ToolProvider interface {
	GetToolDefinitions() []types.Tool
	Execute(types.ToolCall) types.ToolResult
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Serve reads JSON-RPC messages from in and writes responses to out, exposing tp's
// tools over MCP. after, if non-nil, is invoked after every tools/call (used to
// persist any state the call mutated). It returns when in reaches EOF.
func Serve(in io.Reader, out io.Writer, tp ToolProvider, after func()) error {
	enc := json.NewEncoder(out)
	send := func(id json.RawMessage, result any) {
		_ = enc.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	}

	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 1024*1024), 32*1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var req rpcMessage
		if json.Unmarshal(line, &req) != nil {
			continue
		}
		switch req.Method {
		case "initialize":
			var p struct {
				ProtocolVersion string `json:"protocolVersion"`
			}
			_ = json.Unmarshal(req.Params, &p)
			if p.ProtocolVersion == "" {
				p.ProtocolVersion = "2024-11-05"
			}
			send(req.ID, map[string]any{
				"protocolVersion": p.ProtocolVersion,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": ServerName, "version": "1.0.0"},
			})
		case "tools/list":
			defs := tp.GetToolDefinitions()
			tools := make([]map[string]any, 0, len(defs))
			for _, d := range defs {
				schema := json.RawMessage(d.Parameters)
				if len(schema) == 0 {
					schema = json.RawMessage(`{"type":"object","properties":{}}`)
				}
				tools = append(tools, map[string]any{
					"name":        d.Name,
					"description": d.Description,
					"inputSchema": schema,
				})
			}
			send(req.ID, map[string]any{"tools": tools})
		case "tools/call":
			var p struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			_ = json.Unmarshal(req.Params, &p)
			res := tp.Execute(types.ToolCall{Name: p.Name, Arguments: p.Arguments})
			text := res.Content
			isErr := res.Error != ""
			if isErr {
				text = res.Error
			}
			send(req.ID, map[string]any{
				"content": []any{map[string]any{"type": "text", "text": text}},
				"isError": isErr,
			})
			if after != nil {
				after()
			}
		case "ping":
			send(req.ID, map[string]any{})
		default:
			// Notifications (no id), e.g. notifications/initialized — nothing to do.
		}
	}
	return sc.Err()
}

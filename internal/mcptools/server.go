// Package mcptools serves the adventure/session tools (thaimaturgy's ToolRouter)
// over the Model Context Protocol on stdio, so the official Claude Code CLI can
// call them during an oracle turn. It depends only on internal/types (not engine)
// to avoid an import cycle; the executor is passed in as an interface.
package mcptools

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/theburrowhub/thaimaturgy/internal/jsonstrict"
	"github.com/theburrowhub/thaimaturgy/internal/types"
)

const (
	// ServerName is the MCP server name; tools appear to the CLI as
	// mcp__thaim__<tool>.
	ServerName = "thaim"
	// SubcommandArg is the first CLI argument that puts a thaimaturgy binary into
	// "serve MCP tools over stdio" mode instead of launching its UI.
	SubcommandArg = "__mcp-tools"

	maxProtocolVersionBytes = 128
	maxToolNameBytes        = 256
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
// tools over MCP. after, if non-nil, is invoked after every tools/call before a
// success is returned, so persistence failures cannot be acknowledged to the
// client. It returns when in reaches EOF.
func Serve(in io.Reader, out io.Writer, tp ToolProvider, after func() error) error {
	var namespace [12]byte
	if _, err := rand.Read(namespace[:]); err != nil {
		return err
	}
	return ServeWithNamespace(in, out, tp, after, hex.EncodeToString(namespace[:]))
}

// ServeWithNamespace serves MCP using a host-supplied execution namespace. A
// parent process uses this when an MCP child may be restarted during the same
// logical turn: the same JSON-RPC request ID then reaches the same durable rules
// receipt instead of drawing again. Distinct turns must use distinct namespaces.
func ServeWithNamespace(in io.Reader, out io.Writer, tp ToolProvider, after func() error, namespace string) error {
	if !validExecutionNamespace(namespace) {
		return fmt.Errorf("mcptools: invalid execution namespace")
	}
	return serveWithNamespace(in, out, tp, after, namespace)
}

func validExecutionNamespace(namespace string) bool {
	if namespace == "" || len(namespace) > 96 {
		return false
	}
	for index := 0; index < len(namespace); index++ {
		character := namespace[index]
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') && character != '-' && character != '_' {
			return false
		}
	}
	return true
}

func serveWithNamespace(in io.Reader, out io.Writer, tp ToolProvider, after func() error, namespace string) error {
	enc := json.NewEncoder(out)
	send := func(id json.RawMessage, result any) {
		_ = enc.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	}
	sendError := func(id json.RawMessage, code int, message string) {
		var responseID any
		if len(bytes.TrimSpace(id)) > 0 {
			responseID = id
		}
		_ = enc.Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      responseID,
			"error": map[string]any{
				"code": code, "message": message,
			},
		})
	}

	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 1024*1024), 32*1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		if !json.Valid(line) {
			sendError(nil, -32700, "Parse error")
			continue
		}
		var req rpcMessage
		if err := jsonstrict.Decode(line, &req); err != nil {
			sendError(nil, -32600, "Invalid Request: "+err.Error())
			continue
		}
		if req.JSONRPC != "2.0" || req.Method == "" || !validRPCID(req.ID) {
			sendError(responseID(req.ID), -32600, "Invalid Request")
			continue
		}
		switch req.Method {
		case "initialize":
			protocolVersion, err := initializeProtocolVersion(req.Params)
			if err != nil {
				if hasRequestID(req.ID) {
					sendError(req.ID, -32602, "Invalid params: "+err.Error())
				}
				continue
			}
			if !hasRequestID(req.ID) {
				continue
			}
			send(req.ID, map[string]any{
				"protocolVersion": protocolVersion,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": ServerName, "version": "1.0.0"},
			})
		case "tools/list":
			if err := validateOptionalParamsObject(req.Params); err != nil {
				if hasRequestID(req.ID) {
					sendError(req.ID, -32602, "Invalid params: "+err.Error())
				}
				continue
			}
			if !hasRequestID(req.ID) {
				continue
			}
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
			callID := mcpToolCallID(namespace, req.ID)
			if callID == "" {
				// JSON-RPC notifications have no response channel and a null ID is
				// not a usable idempotency key. Never execute a potentially mutating
				// tool in either case.
				if len(bytes.TrimSpace(req.ID)) != 0 {
					sendError(req.ID, -32600, "tools/call requires a non-null request id")
				}
				continue
			}
			name, arguments, err := decodeToolCallParams(req.Params)
			if err != nil {
				sendError(req.ID, -32602, "Invalid params: "+err.Error())
				continue
			}
			res := tp.Execute(types.ToolCall{ID: callID, Name: name, Arguments: arguments})
			if after != nil {
				if err := after(); err != nil {
					res.Content = ""
					res.Error = fmt.Sprintf("persist tool result: %v", err)
				}
			}
			text := res.Content
			isErr := res.Error != ""
			if isErr {
				text = res.Error
			}
			send(req.ID, map[string]any{
				"content": []any{map[string]any{"type": "text", "text": text}},
				"isError": isErr,
			})
		case "ping":
			if err := validateOptionalParamsObject(req.Params); err != nil {
				if hasRequestID(req.ID) {
					sendError(req.ID, -32602, "Invalid params: "+err.Error())
				}
				continue
			}
			if hasRequestID(req.ID) {
				send(req.ID, map[string]any{})
			}
		case "notifications/initialized":
			// JSON-RPC notification: deliberately no response.
		default:
			if hasRequestID(req.ID) {
				sendError(req.ID, -32601, "Method not found")
			}
		}
	}
	return sc.Err()
}

func hasRequestID(id json.RawMessage) bool {
	id = bytes.TrimSpace(id)
	return len(id) > 0 && !bytes.Equal(id, []byte("null"))
}

func responseID(id json.RawMessage) json.RawMessage {
	if hasRequestID(id) {
		return id
	}
	return nil
}

func validRPCID(id json.RawMessage) bool {
	id = bytes.TrimSpace(id)
	if len(id) == 0 || bytes.Equal(id, []byte("null")) {
		return true
	}
	if id[0] == '"' {
		var value string
		return json.Unmarshal(id, &value) == nil
	}
	var value json.Number
	return json.Unmarshal(id, &value) == nil
}

func validateOptionalParamsObject(raw json.RawMessage) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		if err == nil {
			err = fmt.Errorf("params must be an object")
		}
		return err
	}
	return nil
}

func initializeProtocolVersion(raw json.RawMessage) (string, error) {
	if err := validateOptionalParamsObject(raw); err != nil {
		return "", err
	}
	var object map[string]json.RawMessage
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &object); err != nil {
			return "", err
		}
	}
	version := "2024-11-05"
	if encoded, exists := object["protocolVersion"]; exists {
		if err := json.Unmarshal(encoded, &version); err != nil || version == "" {
			if err == nil {
				err = fmt.Errorf("protocolVersion must be a non-empty string")
			}
			return "", err
		}
	}
	if len(version) > maxProtocolVersionBytes {
		return "", fmt.Errorf("protocolVersion exceeds %d bytes", maxProtocolVersionBytes)
	}
	return version, nil
}

func decodeToolCallParams(raw json.RawMessage) (string, json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		if err == nil {
			err = fmt.Errorf("tools/call params must be an object")
		}
		return "", nil, err
	}
	for key := range object {
		if (key != "name" && strings.EqualFold(key, "name")) ||
			(key != "arguments" && strings.EqualFold(key, "arguments")) {
			return "", nil, fmt.Errorf("tools/call parameter %q uses non-canonical casing", key)
		}
	}
	var name string
	encodedName, exists := object["name"]
	if !exists || json.Unmarshal(encodedName, &name) != nil || name == "" {
		return "", nil, fmt.Errorf("tools/call requires a non-empty exact-case tool name")
	}
	if len(name) > maxToolNameBytes {
		return "", nil, fmt.Errorf("tools/call name exceeds %d bytes", maxToolNameBytes)
	}
	arguments := object["arguments"]
	if len(arguments) == 0 {
		arguments = json.RawMessage(`{}`)
	}
	return name, arguments, nil
}

// mcpToolCallID maps the JSON-RPC request ID to the bounded opaque ID expected
// by the rules host. Hashing accepts both string and numeric JSON-RPC IDs without
// trusting their text as a protocol identifier. A retry with the same request ID
// produces the same tool-call ID within one server instance and can therefore
// reuse an idempotency receipt. The instance namespace prevents a later MCP
// subprocess from colliding when its client restarts JSON-RPC numbering.
func mcpToolCallID(namespace string, requestID json.RawMessage) string {
	requestID = bytes.TrimSpace(requestID)
	if len(requestID) == 0 || bytes.Equal(requestID, []byte("null")) {
		return ""
	}
	digest := sha256.Sum256(requestID)
	return "mcp:" + namespace + ":" + hex.EncodeToString(digest[:12])
}

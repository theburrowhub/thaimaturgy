package mcptools

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/types"
)

type recordingToolProvider struct {
	calls []types.ToolCall
}

func (p *recordingToolProvider) GetToolDefinitions() []types.Tool {
	return []types.Tool{{
		Name: "game_observe", Description: "Observe",
		Parameters: json.RawMessage(`{"type":"object","properties":{}}`),
	}}
}

func (p *recordingToolProvider) Execute(call types.ToolCall) types.ToolResult {
	p.calls = append(p.calls, call)
	return types.ToolResult{ToolCallID: call.ID, Content: `{"status":"resolved"}`}
}

func TestServePropagatesStableOpaqueToolCallID(t *testing.T) {
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":"request 7","method":"tools/call","params":{"name":"game_observe","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":"request 7","method":"tools/call","params":{"name":"game_observe","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"game_observe","arguments":{}}}`,
	}, "\n")
	provider := &recordingToolProvider{}
	var output bytes.Buffer
	afterCalls := 0
	if err := Serve(strings.NewReader(input), &output, provider, func() { afterCalls++ }); err != nil {
		t.Fatal(err)
	}
	if len(provider.calls) != 3 || afterCalls != 3 {
		t.Fatalf("calls=%d after=%d output=%s", len(provider.calls), afterCalls, output.String())
	}
	firstID := provider.calls[0].ID
	if firstID == "" || firstID != provider.calls[1].ID || firstID == provider.calls[2].ID {
		t.Fatalf("derived IDs = %q, %q, %q", provider.calls[0].ID, provider.calls[1].ID, provider.calls[2].ID)
	}
	if strings.ContainsAny(firstID, " \t\n\r\"") || !strings.HasPrefix(firstID, "mcp:") {
		t.Fatalf("tool-call ID is not bounded opaque text: %q", firstID)
	}
}

func TestMCPToolCallIDOmitsMissingAndNullIDs(t *testing.T) {
	if got := mcpToolCallID(nil); got != "" {
		t.Fatalf("nil ID = %q", got)
	}
	if got := mcpToolCallID(json.RawMessage(" null ")); got != "" {
		t.Fatalf("null ID = %q", got)
	}
}

func TestServeNeverExecutesToolCallsWithoutUsableRequestID(t *testing.T) {
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"game_observe","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":null,"method":"tools/call","params":{"name":"game_observe","arguments":{}}}`,
	}, "\n")
	provider := &recordingToolProvider{}
	var output bytes.Buffer
	afterCalls := 0
	if err := Serve(strings.NewReader(input), &output, provider, func() { afterCalls++ }); err != nil {
		t.Fatal(err)
	}
	if len(provider.calls) != 0 || afterCalls != 0 {
		t.Fatalf("calls=%d after=%d", len(provider.calls), afterCalls)
	}
	if !strings.Contains(output.String(), "requires a non-null request id") {
		t.Fatalf("null-ID call did not return a protocol error: %s", output.String())
	}
}

func TestServeNeverExecutesMalformedToolCallParams(t *testing.T) {
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"arguments":{}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":"bad"}`,
	}, "\n")
	provider := &recordingToolProvider{}
	var output bytes.Buffer
	afterCalls := 0
	if err := Serve(strings.NewReader(input), &output, provider, func() { afterCalls++ }); err != nil {
		t.Fatal(err)
	}
	if len(provider.calls) != 0 || afterCalls != 0 {
		t.Fatalf("calls=%d after=%d", len(provider.calls), afterCalls)
	}
	if !strings.Contains(output.String(), "requires a tool name") || !strings.Contains(output.String(), "invalid tools/call params") {
		t.Fatalf("malformed calls did not return errors: %s", output.String())
	}
}

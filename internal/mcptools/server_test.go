package mcptools

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	if err := serveWithNamespace(strings.NewReader(input), &output, provider, func() error { afterCalls++; return nil }, "test-process-a"); err != nil {
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

func TestServeNamespacesIDsAcrossServerInstances(t *testing.T) {
	requestID := json.RawMessage(`"request 7"`)
	first := mcpToolCallID("process-a", requestID)
	second := mcpToolCallID("process-b", requestID)
	if first == second {
		t.Fatalf("separate MCP instances generated the same receipt ID: %q", first)
	}
	if first != mcpToolCallID("process-a", requestID) {
		t.Fatal("an MCP retry within one instance did not keep its receipt ID")
	}
}

func TestServeWithNamespaceKeepsIDsStableAcrossChildRestarts(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":17,"method":"tools/call","params":{"name":"game_observe","arguments":{}}}`
	var first, second recordingToolProvider
	if err := ServeWithNamespace(strings.NewReader(input), &bytes.Buffer{}, &first, nil, "oracle-turn-7"); err != nil {
		t.Fatal(err)
	}
	if err := ServeWithNamespace(strings.NewReader(input), &bytes.Buffer{}, &second, nil, "oracle-turn-7"); err != nil {
		t.Fatal(err)
	}
	if len(first.calls) != 1 || len(second.calls) != 1 || first.calls[0].ID != second.calls[0].ID {
		t.Fatalf("restart IDs: first=%v second=%v", first.calls, second.calls)
	}
	if err := ServeWithNamespace(strings.NewReader(""), &bytes.Buffer{}, &first, nil, "bad namespace"); err == nil {
		t.Fatal("unsafe namespace was accepted")
	}
}

func TestMCPToolCallIDOmitsMissingAndNullIDs(t *testing.T) {
	if got := mcpToolCallID("test", nil); got != "" {
		t.Fatalf("nil ID = %q", got)
	}
	if got := mcpToolCallID("test", json.RawMessage(" null ")); got != "" {
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
	if err := Serve(strings.NewReader(input), &output, provider, func() error { afterCalls++; return nil }); err != nil {
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
	if err := Serve(strings.NewReader(input), &output, provider, func() error { afterCalls++; return nil }); err != nil {
		t.Fatal(err)
	}
	if len(provider.calls) != 0 || afterCalls != 0 {
		t.Fatalf("calls=%d after=%d", len(provider.calls), afterCalls)
	}
	if !strings.Contains(output.String(), "non-empty exact-case tool name") || !strings.Contains(output.String(), "cannot unmarshal string") {
		t.Fatalf("malformed calls did not return errors: %s", output.String())
	}
}

func TestServeRejectsAmbiguousOrInvalidJSONRPCBeforeExecution(t *testing.T) {
	input := strings.Join([]string{
		`{"jsonrpc":`,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","method":"ping","params":{"name":"game_observe","arguments":{}}}`,
		`{"jsonrpc":"1.0","id":2,"method":"tools/call","params":{"name":"game_observe","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":true,"method":"tools/call","params":{"name":"game_observe","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"game_observe","name":"other","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"Name":"game_observe","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"unknown/method","params":{}}`,
	}, "\n")
	provider := &recordingToolProvider{}
	var output bytes.Buffer
	afterCalls := 0
	if err := serveWithNamespace(strings.NewReader(input), &output, provider, func() error { afterCalls++; return nil }, "strict-protocol"); err != nil {
		t.Fatal(err)
	}
	if len(provider.calls) != 0 || afterCalls != 0 {
		t.Fatalf("ambiguous protocol input executed tools: calls=%d after=%d", len(provider.calls), afterCalls)
	}
	for _, marker := range []string{`"code":-32700`, `"code":-32600`, `"code":-32602`, `"code":-32601`} {
		if !strings.Contains(output.String(), marker) {
			t.Errorf("protocol output omitted %s: %s", marker, output.String())
		}
	}
}

func TestMCPMetadataBounds(t *testing.T) {
	protocol := fmt.Sprintf(`{"protocolVersion":%q}`, strings.Repeat("v", maxProtocolVersionBytes+1))
	if _, err := initializeProtocolVersion(json.RawMessage(protocol)); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized protocol error = %v", err)
	}
	params := fmt.Sprintf(`{"name":%q,"arguments":{}}`, strings.Repeat("t", maxToolNameBytes+1))
	if _, _, err := decodeToolCallParams(json.RawMessage(params)); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized tool name error = %v", err)
	}
}

func TestServeTurnsPersistenceFailureIntoToolError(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"game_observe","arguments":{}}}`
	provider := &recordingToolProvider{}
	var output bytes.Buffer
	if err := serveWithNamespace(strings.NewReader(input), &output, provider, func() error {
		return fmt.Errorf("disk unavailable")
	}, "persist-failure"); err != nil {
		t.Fatal(err)
	}
	if len(provider.calls) != 1 || !strings.Contains(output.String(), `"isError":true`) || !strings.Contains(output.String(), "disk unavailable") {
		t.Fatalf("calls=%d output=%s", len(provider.calls), output.String())
	}
}

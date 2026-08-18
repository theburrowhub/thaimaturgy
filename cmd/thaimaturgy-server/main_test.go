package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/mcptools"
)

// TestMCPSubcommandDoesNotStartServer verifies the __mcp-tools dispatch: invoked
// with that arg (as the Claude-CLI backend re-execs this binary), the server must
// run the MCP subcommand — which fails fast on a bogus adventure — and NOT start
// the HTTP server. It builds the binary and runs it against an empty data dir.
func TestMCPSubcommandDoesNotStartServer(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the server binary; skipped in -short")
	}
	bin := filepath.Join(t.TempDir(), "thaimaturgy-server")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	cmd := exec.Command(bin, mcptools.SubcommandArg, "--adventure-id", "nope", "--session", filepath.Join(t.TempDir(), "s.json"))
	cmd.Stdin = strings.NewReader("") // no MCP handshake; it fails before reading stdin
	// A dedicated empty data dir makes the "nope" adventure deterministically absent,
	// and THAIM_ADDR :0 would be a harmless port had it (wrongly) started the server.
	cmd.Env = append(os.Environ(), "THAIM_DATA_DIR="+t.TempDir(), "THAIM_ADDR=127.0.0.1:0")

	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected the subcommand to fail on a bogus adventure; got success:\n%s", out)
	}
	if !strings.Contains(string(out), "mcp-tools:") {
		t.Fatalf("expected an mcp-tools error (subcommand path), got:\n%s", out)
	}
}

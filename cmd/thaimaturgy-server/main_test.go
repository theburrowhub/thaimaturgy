package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/mcptools"
)

func TestIsLoopbackAddr(t *testing.T) {
	cases := []struct {
		addr string
		want bool
		err  bool
	}{
		{"127.0.0.1:8765", true, false},
		{"localhost:8765", true, false},
		{"[::1]:8765", true, false},
		{":8765", false, false},        // wildcard → all interfaces, NOT loopback
		{"0.0.0.0:8765", false, false}, // explicit wildcard
		{"192.168.1.10:8765", false, false},
		{"example.com:8765", false, false}, // hostname → treated as exposed
		{"garbage", false, true},           // no port → error
	}
	for _, c := range cases {
		got, err := isLoopbackAddr(c.addr)
		if (err != nil) != c.err {
			t.Errorf("isLoopbackAddr(%q) err=%v; wantErr=%v", c.addr, err, c.err)
			continue
		}
		if err == nil && got != c.want {
			t.Errorf("isLoopbackAddr(%q) = %v; want %v", c.addr, got, c.want)
		}
	}
}

func TestShutdownSignalsIncludeSIGTERM(t *testing.T) {
	var hasInt, hasTerm bool
	for _, s := range shutdownSignals {
		if s == syscall.SIGINT {
			hasInt = true
		}
		if s == syscall.SIGTERM {
			hasTerm = true
		}
	}
	if !hasTerm {
		t.Error("shutdownSignals must include SIGTERM (Docker/K8s/systemd stop with it)")
	}
	if !hasInt {
		t.Error("shutdownSignals should include SIGINT")
	}
}

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

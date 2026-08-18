// Package mcpserve implements the `__mcp-tools` subcommand shared by the desktop
// GUI and the Telegram bot: when the Claude-CLI oracle backend runs its agentic
// loop, it invokes this same binary as an MCP tools server over stdio. Both
// frontends dispatch to RunSubcommand so the wiring lives in one place.
package mcpserve

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/engine"
	"github.com/theburrowhub/thaimaturgy/internal/mcptools"
	"github.com/theburrowhub/thaimaturgy/internal/rules/runtimecatalog"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

// RunSubcommand serves the session tools over stdio MCP: it loads the adventure
// and the session-state temp file, exposes the engine ToolRouter, and writes the
// (possibly mutated) state back after each tool call.
func RunSubcommand(args []string) error {
	return runSubcommand(args, os.Stdin, os.Stdout)
}

func runSubcommand(args []string, input io.Reader, output io.Writer) error {
	fs := flag.NewFlagSet("mcp-tools", flag.ContinueOnError)
	advID := fs.String("adventure-id", "", "adventure id")
	sessPath := fs.String("session", "", "session state json path")
	dataDirectory := fs.String("data-dir", strings.TrimSpace(os.Getenv("THAIM_DATA_DIR")), "thAImaturgy data directory")
	requestNamespace := fs.String("request-namespace", "", "stable namespace for one parent oracle turn")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*advID) == "" || strings.TrimSpace(*sessPath) == "" {
		return fmt.Errorf("mcp-tools: --adventure-id and --session are required")
	}
	var store *storage.Storage
	var err error
	if strings.TrimSpace(*dataDirectory) == "" {
		store, err = storage.New()
	} else {
		store, err = storage.NewWithPath(strings.TrimSpace(*dataDirectory))
	}
	if err != nil {
		return err
	}
	adv, err := store.LoadAdventure(*advID)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(*sessPath)
	if err != nil {
		return err
	}
	var st domain.SessionState
	if err := json.Unmarshal(data, &st); err != nil {
		return err
	}
	// In virtual-DM mode make sure the party exists, so the character tools have
	// members to target even if this subprocess is the first to touch it.
	if st.EffectiveMode() == domain.ModeVirtualDM {
		st.EnsureParty()
	}
	rulesEnvironment, err := runtimecatalog.Load(context.Background(), store.BasePath())
	if err != nil {
		return fmt.Errorf("mcp-tools: load rules catalog: %w", err)
	}
	session, err := rulesEnvironment.OpenSession(context.Background(), &st, adv, domain.DefaultConfig())
	if err != nil {
		return fmt.Errorf("mcp-tools: open session: %w", err)
	}
	persist := func(state *domain.SessionState) error {
		if err := store.SaveSession(state); err != nil {
			return err
		}
		encoded, err := json.MarshalIndent(state, "", "  ")
		if err != nil {
			return err
		}
		return replaceFile(*sessPath, encoded, 0o600)
	}
	session.PersistRules = persist
	if session.IsModified {
		if err := persist(&st); err != nil {
			return fmt.Errorf("mcp-tools: persist rules binding: %w", err)
		}
		session.IsModified = false
	}
	router := engine.NewToolRouter(session)
	save := func() error { return persist(&st) }
	if *requestNamespace != "" {
		return mcptools.ServeWithNamespace(input, output, router, save, *requestNamespace)
	}
	return mcptools.Serve(input, output, router, save)
}

func replaceFile(path string, data []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".tmp-"+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

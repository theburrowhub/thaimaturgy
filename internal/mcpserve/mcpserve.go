// Package mcpserve implements the `__mcp-tools` subcommand shared by the desktop
// GUI and the Telegram bot: when the Claude-CLI oracle backend runs its agentic
// loop, it invokes this same binary as an MCP tools server over stdio. Both
// frontends dispatch to RunSubcommand so the wiring lives in one place.
package mcpserve

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/engine"
	"github.com/theburrowhub/thaimaturgy/internal/mcptools"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

// RunSubcommand serves the session tools over stdio MCP: it loads the adventure
// and the session-state temp file, exposes the engine ToolRouter, and writes the
// (possibly mutated) state back after each tool call.
func RunSubcommand(args []string) error {
	fs := flag.NewFlagSet("mcp-tools", flag.ContinueOnError)
	advID := fs.String("adventure-id", "", "adventure id")
	sessPath := fs.String("session", "", "session state json path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	store, err := storage.New()
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
	session := domain.NewSession(&st, adv, domain.DefaultConfig())
	router := engine.NewToolRouter(session)
	save := func() {
		if b, err := json.MarshalIndent(&st, "", "  "); err == nil {
			_ = replaceFile(*sessPath, b, 0644)
		}
	}
	return mcptools.Serve(os.Stdin, os.Stdout, router, save)
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

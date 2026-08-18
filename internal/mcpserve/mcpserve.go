// Package mcpserve implements the `__mcp-tools` subcommand shared by the desktop
// GUI and the Telegram bot: when the Claude-CLI oracle backend runs its agentic
// loop, it invokes this same binary as an MCP tools server over stdio. Both
// frontends dispatch to RunSubcommand so the wiring lives in one place.
package mcpserve

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

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
	language := fs.String("language", "", "effective session language (en or es)")
	rulesTimeoutSeconds := fs.Int("rules-timeout-seconds", 0, "effective bounded rules-call timeout in seconds")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*advID) == "" || strings.TrimSpace(*sessPath) == "" {
		return fmt.Errorf("mcp-tools: --adventure-id and --session are required")
	}
	config, err := mcpSessionConfig(*language, *rulesTimeoutSeconds)
	if err != nil {
		return err
	}
	var store *storage.Storage
	if strings.TrimSpace(*dataDirectory) == "" {
		store, err = storage.NewFromEnvironment()
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
	var handoff domain.SessionState
	if err := json.Unmarshal(data, &handoff); err != nil {
		return err
	}
	// A prior child may have been interrupted after publishing the canonical
	// checkpoint (the historical write order) or after publishing only the
	// handoff (the new recoverable order). When canonical rules are ahead, their
	// complete session snapshot is the checkpoint correlated with those receipts;
	// combining it with stale ordinary handoff fields would acknowledge effects
	// such as HP changes without retaining the effect itself.
	st, err := reconcileCanonicalSession(store, &handoff)
	if err != nil {
		return fmt.Errorf("mcp-tools: reconcile canonical checkpoint: %w", err)
	}
	rulesEnvironment, err := runtimecatalog.Load(context.Background(), store.BasePath())
	if err != nil {
		return fmt.Errorf("mcp-tools: load rules catalog: %w", err)
	}
	session, err := rulesEnvironment.OpenSession(context.Background(), st, adv, config)
	if err != nil {
		return fmt.Errorf("mcp-tools: open session: %w", err)
	}
	// Legacy party state belongs only to the exact built-in D&D package. Resolve
	// the package first so a foreign virtual-DM session cannot be mutated merely
	// by starting its MCP tool server.
	if st.EffectiveMode() == domain.ModeVirtualDM {
		if snapshot, ok := st.RulesSnapshot(); ok && engine.IsBuiltinDND5ELock(snapshot.Ruleset) {
			st.EnsureParty()
		}
	}
	persister := newCheckpointPersister(*sessPath, store.SaveSession)
	session.PersistRules = persister.Persist
	// Establish one reconciled snapshot in both destinations before accepting a
	// tool call. Persist writes the handoff first, so any failure before the
	// canonical CAS leaves the parent a complete checkpoint it can merge/retry.
	if err := persister.Persist(st); err != nil {
		return fmt.Errorf("mcp-tools: persist initial handoff: %w", err)
	}
	session.IsModified = false
	router := engine.NewToolRouter(session)
	if err := router.InitializationError(); err != nil {
		return fmt.Errorf("mcp-tools: initialize rules gateway: %w", err)
	}
	// Rules tools already cross PersistRules at each committed checkpoint. The
	// persister fingerprints the last successful snapshot, so this generic MCP
	// after-hook still covers ordinary tools without duplicating rules writes.
	save := func() error { return persister.Persist(st) }
	if *requestNamespace != "" {
		return mcptools.ServeWithNamespace(input, output, router, save, *requestNamespace)
	}
	return mcptools.Serve(input, output, router, save)
}

func mcpSessionConfig(language string, rulesTimeoutSeconds int) (*domain.Config, error) {
	language = strings.TrimSpace(language)
	if language == "" || rulesTimeoutSeconds == 0 {
		return nil, errors.New("mcp-tools: --language and --rules-timeout-seconds are required")
	}
	sessionLanguage := domain.Language(language)
	switch sessionLanguage {
	case domain.LangEnglish, domain.LangSpanish:
	default:
		return nil, fmt.Errorf("mcp-tools: unsupported --language %q (expected en or es)", language)
	}
	if rulesTimeoutSeconds < 1 || rulesTimeoutSeconds > engine.MaxRulesRequestTimeoutSeconds {
		return nil, fmt.Errorf(
			"mcp-tools: --rules-timeout-seconds must be between 1 and %d",
			engine.MaxRulesRequestTimeoutSeconds,
		)
	}
	config := domain.DefaultConfig()
	config.Language = sessionLanguage
	config.RequestTimeoutSeconds = rulesTimeoutSeconds
	return config, nil
}

func reconcileCanonicalSession(store *storage.Storage, handoff *domain.SessionState) (*domain.SessionState, error) {
	if store == nil || handoff == nil {
		return nil, errors.New("storage and handoff session are required")
	}
	canonical, err := store.LoadSession(handoff.Name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return handoff, nil
		}
		return nil, err
	}
	if canonical.Name != handoff.Name {
		return nil, fmt.Errorf("canonical session name %q does not match handoff %q", canonical.Name, handoff.Name)
	}
	if canonical.AdventureID != handoff.AdventureID {
		return nil, fmt.Errorf("canonical adventure %q does not match handoff %q", canonical.AdventureID, handoff.AdventureID)
	}
	canonicalRuntime, canonicalExists, err := canonical.RulesRuntimeSnapshotStrict()
	if err != nil {
		return nil, fmt.Errorf("canonical rules runtime: %w", err)
	}
	handoffRuntime, handoffExists, err := handoff.RulesRuntimeSnapshotStrict()
	if err != nil {
		return nil, fmt.Errorf("handoff rules runtime: %w", err)
	}
	switch {
	case !canonicalExists:
		// A handoff may contain the first package binding that canonical storage
		// has not received yet. It remains the authoritative recoverable snapshot.
		return handoff, nil
	case !handoffExists:
		// Canonical has already committed the first exact binding. Adopt the full
		// correlated snapshot rather than grafting its lock onto stale fields.
		return canonical, nil
	case canonicalRuntime.Generation > handoffRuntime.Generation:
		if err := canonicalRuntime.ValidateDescendantOf(handoffRuntime); err != nil {
			return nil, fmt.Errorf("canonical rules ancestry: %w", err)
		}
		return canonical, nil
	case handoffRuntime.Generation > canonicalRuntime.Generation:
		if err := handoffRuntime.ValidateDescendantOf(canonicalRuntime); err != nil {
			return nil, fmt.Errorf("handoff rules ancestry: %w", err)
		}
		return handoff, nil
	default:
		// Equal generations must be the same durable runtime; this comparison also
		// normalizes nil and empty audit slices through the domain representation.
		if err := canonicalRuntime.ValidateDescendantOf(handoffRuntime); err != nil {
			return nil, fmt.Errorf("equal-generation rules reconciliation: %w", err)
		}
		return handoff, nil
	}
}

type checkpointPersister struct {
	handoffPath   string
	writeHandoff  func(string, []byte, os.FileMode) error
	saveCanonical func(*domain.SessionState) error

	mu             sync.Mutex
	hasLast        bool
	lastSuccessful [sha256.Size]byte
}

func newCheckpointPersister(handoffPath string, saveCanonical func(*domain.SessionState) error) *checkpointPersister {
	return &checkpointPersister{
		handoffPath: handoffPath, writeHandoff: replaceFile, saveCanonical: saveCanonical,
	}
}

// Persist publishes the parent-owned handoff before attempting the canonical
// CAS. If the second write fails, the parent can still merge the complete newer
// handoff and retry; if the first fails, canonical state is untouched. An exact
// successful snapshot is skipped when mcptools invokes its generic after-hook.
func (p *checkpointPersister) Persist(state *domain.SessionState) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if state == nil {
		return errors.New("nil session checkpoint")
	}
	encoded, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal handoff: %w", err)
	}
	digest := sha256.Sum256(encoded)
	if p.hasLast && digest == p.lastSuccessful {
		return nil
	}
	if p.writeHandoff == nil || p.saveCanonical == nil {
		return errors.New("session checkpoint destinations are not configured")
	}
	if err := p.writeHandoff(p.handoffPath, encoded, 0o600); err != nil {
		return fmt.Errorf("write parent handoff: %w", err)
	}
	if err := p.saveCanonical(state); err != nil {
		return fmt.Errorf("write canonical session: %w", err)
	}
	p.lastSuccessful = digest
	p.hasLast = true
	return nil
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
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	// The handoff is itself a crash-recovery checkpoint. Best-effort directory
	// sync mirrors the canonical storage writer without rejecting platforms that
	// do not permit syncing directories.
	if directory, err := os.Open(filepath.Dir(path)); err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}

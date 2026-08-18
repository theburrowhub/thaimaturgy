package mcpserve

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/rules"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

func TestReplaceFileAtomicallyReplacesSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := replaceFile(path, []byte("new"), 0o640); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("snapshot = %q", data)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("snapshot mode = %o", info.Mode().Perm())
	}
}

func TestCheckpointPersisterPublishesHandoffFirstAndRetriesCanonical(t *testing.T) {
	state := domain.NewSessionState("checkpoint", nil)
	state.CurrentRoom = "after-tool"
	handoffPath := filepath.Join(t.TempDir(), "handoff.json")
	var order []string
	canonicalFailures := 1
	persister := &checkpointPersister{
		handoffPath: handoffPath,
		writeHandoff: func(path string, data []byte, mode os.FileMode) error {
			order = append(order, "handoff")
			return replaceFile(path, data, mode)
		},
		saveCanonical: func(*domain.SessionState) error {
			order = append(order, "canonical")
			if canonicalFailures > 0 {
				canonicalFailures--
				return errors.New("canonical unavailable")
			}
			return nil
		},
	}

	if err := persister.Persist(state); err == nil || !strings.Contains(err.Error(), "canonical unavailable") {
		t.Fatalf("first persist error = %v", err)
	}
	if got := mustReadState(t, handoffPath).CurrentRoom; got != "after-tool" {
		t.Fatalf("recoverable handoff room = %q", got)
	}
	if strings.Join(order, ",") != "handoff,canonical" {
		t.Fatalf("first publish order = %v", order)
	}

	if err := persister.Persist(state); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if strings.Join(order, ",") != "handoff,canonical,handoff,canonical" {
		t.Fatalf("retry order = %v", order)
	}
	if err := persister.Persist(state); err != nil {
		t.Fatal(err)
	}
	if len(order) != 4 {
		t.Fatalf("generic MCP after-hook duplicated successful checkpoint: %v", order)
	}
}

func TestCheckpointPersisterNeverTouchesCanonicalAfterHandoffFailure(t *testing.T) {
	canonicalCalls := 0
	persister := &checkpointPersister{
		handoffPath: "unused",
		writeHandoff: func(string, []byte, os.FileMode) error {
			return errors.New("handoff unavailable")
		},
		saveCanonical: func(*domain.SessionState) error {
			canonicalCalls++
			return nil
		},
	}
	if err := persister.Persist(domain.NewSessionState("checkpoint", nil)); err == nil || !strings.Contains(err.Error(), "handoff unavailable") {
		t.Fatalf("persist error = %v", err)
	}
	if canonicalCalls != 0 {
		t.Fatalf("canonical calls after handoff failure = %d", canonicalCalls)
	}
}

func TestCanonicalFailureIsRecoveredFromParentHandoffWithoutRollback(t *testing.T) {
	store, err := storage.NewWithPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parent := domain.NewSessionState("recover-checkpoint", nil)
	payload, err := rules.PayloadFrom(map[string]any{"value": 0})
	if err != nil {
		t.Fatal(err)
	}
	lock := rules.Lock{
		ID: "mcp-recovery", Version: "1.0.0", ProtocolVersion: rules.ProtocolVersion,
		Digest: "sha256:" + strings.Repeat("e", 64),
	}
	if created, err := parent.BindRules(lock, payload); err != nil || !created {
		t.Fatalf("bind created=%v err=%v", created, err)
	}
	if err := store.SaveSession(parent); err != nil {
		t.Fatal(err)
	}
	child := cloneMCPState(t, parent)
	advanceMCPRules(t, child, "handoff-ahead")
	handoffPath := filepath.Join(t.TempDir(), "handoff.json")
	persister := newCheckpointPersister(handoffPath, func(*domain.SessionState) error {
		return errors.New("canonical unavailable")
	})
	if err := persister.Persist(child); err == nil || !strings.Contains(err.Error(), "canonical unavailable") {
		t.Fatalf("child persist error = %v", err)
	}

	recovered := mustReadState(t, handoffPath)
	if err := parent.ImportStructuredChecked(recovered); err != nil {
		t.Fatalf("parent merge: %v", err)
	}
	if err := store.SaveSession(parent); err != nil {
		t.Fatalf("parent canonical retry: %v", err)
	}
	loaded := mustLoadState(t, store, parent.Name)
	runtime, ok := loaded.RulesRuntimeSnapshot()
	if !ok || runtime.Generation != 1 || len(runtime.Receipts) != 1 || runtime.Receipts[0].RequestID != "handoff-ahead" {
		t.Fatalf("recovered runtime: ok=%v runtime=%+v", ok, runtime)
	}
}

func TestRunSubcommandUsesRequestedDataDirectoryAndPersistsNewBinding(t *testing.T) {
	dataDirectory := t.TempDir()
	store, err := storage.NewWithPath(dataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	adventure := &domain.Adventure{
		SchemaVersion: domain.SchemaVersion, ID: "mcp-adventure", Title: "MCP Adventure",
		Ruleset: &rules.Requirement{ID: "pbta", Version: "0.1.0"},
		Zones:   []domain.Zone{{ID: "zone", Name: "Zone", Rooms: []domain.Room{{ID: "room", Name: "Room"}}}},
	}
	if err := os.MkdirAll(store.AdventureDir(adventure.ID), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAdventure(adventure.ID, adventure); err != nil {
		t.Fatal(err)
	}
	state := domain.NewSessionState("mcp-session", adventure)
	sessionPath := filepath.Join(t.TempDir(), "session.json")
	writeStateFile(t, sessionPath, state)

	var output bytes.Buffer
	if err := runSubcommand([]string{
		"--data-dir", dataDirectory,
		"--adventure-id", adventure.ID,
		"--session", sessionPath,
	}, strings.NewReader(""), &output); err != nil {
		t.Fatal(err)
	}
	for label, loaded := range map[string]*domain.SessionState{
		"canonical": mustLoadState(t, store, state.Name),
		"temporary": mustReadState(t, sessionPath),
	} {
		snapshot, ok := loaded.RulesSnapshot()
		if !ok || snapshot.Ruleset.ID != "pbta" || snapshot.Ruleset.Version != "0.1.0" {
			t.Fatalf("%s rules snapshot = %+v ok=%v", label, snapshot, ok)
		}
	}
}

func TestRunSubcommandMissingExactArtifactFailsWithoutRebinding(t *testing.T) {
	dataDirectory := t.TempDir()
	store, err := storage.NewWithPath(dataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	adventure := &domain.Adventure{SchemaVersion: domain.SchemaVersion, ID: "locked", Title: "Locked", System: "D&D 5e"}
	if err := os.MkdirAll(store.AdventureDir(adventure.ID), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAdventure(adventure.ID, adventure); err != nil {
		t.Fatal(err)
	}
	state := domain.NewSessionState("locked-session", adventure)
	empty, _ := rules.PayloadFrom(map[string]any{})
	missing := rules.Lock{
		ID: "missing.rules", Version: "1.0.0", ProtocolVersion: rules.ProtocolVersion,
		Digest: "sha256:" + strings.Repeat("b", 64),
	}
	if created, err := state.BindRules(missing, empty); err != nil || !created {
		t.Fatalf("bind created=%v err=%v", created, err)
	}
	sessionPath := filepath.Join(t.TempDir(), "session.json")
	writeStateFile(t, sessionPath, state)

	err = runSubcommand([]string{"--data-dir", dataDirectory, "--adventure-id", adventure.ID, "--session", sessionPath}, strings.NewReader(""), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "exact session lock") {
		t.Fatalf("error = %v", err)
	}
	snapshot, ok := mustReadState(t, sessionPath).RulesSnapshot()
	if !ok || snapshot.Ruleset != missing {
		t.Fatalf("failed launch mutated lock: %+v ok=%v", snapshot, ok)
	}
}

func TestRunSubcommandReconcilesCanonicalRulesAheadOfStaleHandoff(t *testing.T) {
	dataDirectory := t.TempDir()
	store, err := storage.NewWithPath(dataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	adventure := &domain.Adventure{
		SchemaVersion: domain.SchemaVersion, ID: "mcp-reconcile", Title: "MCP Reconcile",
		Ruleset: &rules.Requirement{ID: "pbta", Version: "0.1.0"},
		Zones:   []domain.Zone{{ID: "zone", Name: "Zone", Rooms: []domain.Room{{ID: "room", Name: "Room"}}}},
	}
	if err := os.MkdirAll(store.AdventureDir(adventure.ID), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAdventure(adventure.ID, adventure); err != nil {
		t.Fatal(err)
	}
	state := domain.NewSessionState("mcp-reconcile", adventure)
	handoffPath := filepath.Join(t.TempDir(), "session.json")
	writeStateFile(t, handoffPath, state)
	args := []string{"--data-dir", dataDirectory, "--adventure-id", adventure.ID, "--session", handoffPath}
	if err := runSubcommand(args, strings.NewReader(""), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	stale := mustReadState(t, handoffPath)
	canonical := mustLoadState(t, store, state.Name)
	advanceMCPRules(t, canonical, "canonical-ahead")
	if err := store.SaveSession(canonical); err != nil {
		t.Fatal(err)
	}
	writeStateFile(t, handoffPath, stale)

	if err := runSubcommand(args, strings.NewReader(""), &bytes.Buffer{}); err != nil {
		t.Fatalf("reconciled launch: %v", err)
	}
	for label, loaded := range map[string]*domain.SessionState{
		"canonical": mustLoadState(t, store, state.Name),
		"handoff":   mustReadState(t, handoffPath),
	} {
		runtime, ok := loaded.RulesRuntimeSnapshot()
		if !ok || runtime.Generation != 1 || len(runtime.Receipts) != 1 || runtime.Receipts[0].RequestID != "canonical-ahead" {
			t.Fatalf("%s runtime rolled back: ok=%v runtime=%+v", label, ok, runtime)
		}
	}
}

func writeStateFile(t *testing.T, path string, state *domain.SessionState) {
	t.Helper()
	encoded, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
}

func mustReadState(t *testing.T, path string) *domain.SessionState {
	t.Helper()
	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var state domain.SessionState
	if err := json.Unmarshal(encoded, &state); err != nil {
		t.Fatal(err)
	}
	return &state
}

func mustLoadState(t *testing.T, store *storage.Storage, name string) *domain.SessionState {
	t.Helper()
	state, err := store.LoadSession(name)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func cloneMCPState(t *testing.T, state *domain.SessionState) *domain.SessionState {
	t.Helper()
	encoded, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	var clone domain.SessionState
	if err := json.Unmarshal(encoded, &clone); err != nil {
		t.Fatal(err)
	}
	return &clone
}

func advanceMCPRules(t *testing.T, state *domain.SessionState, requestID string) {
	t.Helper()
	handle, receipt, err := state.BeginRulesRequest(
		context.Background(), requestID, "game_submit_intent", "sha256:"+strings.Repeat("c", 64),
	)
	if err != nil || receipt != nil {
		t.Fatalf("begin receipt=%v err=%v", receipt, err)
	}
	if _, err := state.CommitRulesRequest(handle, domain.RulesCommit{
		State: handle.Snapshot.State, ResolutionID: requestID,
		Result: &domain.RulesStoredResult{Content: `{"status":"resolved"}`},
	}); err != nil {
		t.Fatal(err)
	}
}

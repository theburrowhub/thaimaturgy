package mcpserve

import (
	"bytes"
	"encoding/json"
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

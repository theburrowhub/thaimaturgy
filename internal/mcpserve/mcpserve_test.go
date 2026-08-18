package mcpserve

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/engine"
	"github.com/theburrowhub/thaimaturgy/internal/rules"
	"github.com/theburrowhub/thaimaturgy/internal/rules/bundlepack"
	"github.com/theburrowhub/thaimaturgy/internal/rules/dnd5e"
	"github.com/theburrowhub/thaimaturgy/internal/rules/pbta"
	"github.com/theburrowhub/thaimaturgy/internal/rules/runtimecatalog"
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
		"--language", "es",
		"--rules-timeout-seconds", "17",
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

	err = runSubcommand([]string{
		"--data-dir", dataDirectory, "--adventure-id", adventure.ID, "--session", sessionPath,
		"--language", "en", "--rules-timeout-seconds", "90",
	}, strings.NewReader(""), &bytes.Buffer{})
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
	args := []string{
		"--data-dir", dataDirectory, "--adventure-id", adventure.ID, "--session", handoffPath,
		"--language", "en", "--rules-timeout-seconds", "90",
	}
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

func TestMCPSessionConfigRequiresExplicitBoundedContext(t *testing.T) {
	config, err := mcpSessionConfig(" es ", 17)
	if err != nil {
		t.Fatal(err)
	}
	if config.Language != domain.LangSpanish || config.RequestTimeoutSeconds != 17 {
		t.Fatalf("session config language=%q timeout=%d", config.Language, config.RequestTimeoutSeconds)
	}

	tests := []struct {
		name     string
		language string
		timeout  int
		contains string
	}{
		{name: "missing language", timeout: 17, contains: "are required"},
		{name: "missing timeout", language: "es", contains: "are required"},
		{name: "unsupported language", language: "fr", timeout: 17, contains: "expected en or es"},
		{name: "negative timeout", language: "es", timeout: -1, contains: "must be between"},
		{name: "unbounded timeout", language: "es", timeout: engine.MaxRulesRequestTimeoutSeconds + 1, contains: "must be between"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := mcpSessionConfig(test.language, test.timeout)
			if err == nil || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("error = %v, want substring %q", err, test.contains)
			}
		})
	}
	if err := runSubcommand(
		[]string{"--adventure-id", "unused", "--session", "unused.json"},
		strings.NewReader(""), &bytes.Buffer{},
	); err == nil || !strings.Contains(err.Error(), "--language and --rules-timeout-seconds are required") {
		t.Fatalf("subcommand without explicit context error = %v", err)
	}
}

func TestMCPPBTAEndToEndPersistsExactCatalogAcrossReload(t *testing.T) {
	dataDirectory := t.TempDir()
	store, err := storage.NewWithPath(dataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	adventure := &domain.Adventure{
		SchemaVersion: domain.SchemaVersion,
		ID:            "mcp-pbta-e2e",
		Title:         "MCP PbtA E2E",
		Ruleset:       &rules.Requirement{ID: pbta.PackageID, Version: pbta.PackageVersion},
		Zones: []domain.Zone{{
			ID: "zone", Name: "Zone", Rooms: []domain.Room{{ID: "room", Name: "Room"}},
		}},
	}
	if err := os.MkdirAll(store.AdventureDir(adventure.ID), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAdventure(adventure.ID, adventure); err != nil {
		t.Fatal(err)
	}
	state := domain.NewSessionState("mcp-pbta-e2e-session", adventure)
	state.SetMode(domain.ModeVirtualDM)
	handoffPath := filepath.Join(t.TempDir(), "session.json")
	writeStateFile(t, handoffPath, state)
	args := []string{
		"--data-dir", dataDirectory,
		"--adventure-id", adventure.ID,
		"--session", handoffPath,
		"--request-namespace", "pbta-e2e-turn",
		"--language", "es",
		"--rules-timeout-seconds", "17",
	}
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"game_list_actions","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"game_submit_intent","arguments":{"action_id":"move.resolve","arguments":{"modifier":1}}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"game_observe","arguments":{}}}`,
	}, "\n")

	firstOutput := runMCPSubcommand(t, args, input)
	firstResponses := decodeMCPResponses(t, firstOutput)
	if len(firstResponses) != 5 {
		t.Fatalf("MCP response count = %d; output=%s", len(firstResponses), firstOutput)
	}
	var initialized struct {
		ServerInfo struct {
			Name string `json:"name"`
		} `json:"serverInfo"`
	}
	mustUnmarshalMCPResult(t, firstResponses, "1", &initialized)
	if initialized.ServerInfo.Name != "thaim" {
		t.Fatalf("initialized server = %q", initialized.ServerInfo.Name)
	}

	var listedTools struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	mustUnmarshalMCPResult(t, firstResponses, "2", &listedTools)
	toolNames := make(map[string]bool, len(listedTools.Tools))
	for _, tool := range listedTools.Tools {
		toolNames[tool.Name] = true
	}
	for _, name := range []string{
		"game_observe", "game_list_actions", "game_get_action_schema",
		"game_submit_intent", "game_respond", "game_preview", "game_explain",
	} {
		if !toolNames[name] {
			t.Errorf("foreign MCP catalog omitted %q", name)
		}
	}
	for _, name := range []string{
		"roll_dice", "ability_check", "update_party_member", "update_hp", "add_item",
		"remove_item", "set_condition", "remove_condition", "update_gold", "award_xp",
	} {
		if toolNames[name] {
			t.Errorf("foreign MCP catalog exposed D&D-only alias %q", name)
		}
	}

	artifact, err := pbta.NewArtifact()
	if err != nil {
		t.Fatal(err)
	}
	exactLock := artifact.Lock()
	var catalogEnvelope struct {
		Status  string     `json:"status"`
		Ruleset rules.Lock `json:"ruleset"`
		Data    struct {
			Actions []struct {
				ID string `json:"id"`
			} `json:"actions"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(mcpToolText(t, firstResponses, "3")), &catalogEnvelope); err != nil {
		t.Fatal(err)
	}
	if catalogEnvelope.Status != "resolved" || catalogEnvelope.Ruleset != exactLock ||
		len(catalogEnvelope.Data.Actions) != 1 || catalogEnvelope.Data.Actions[0].ID != pbta.ActionMove {
		t.Fatalf("exact PbtA catalog = %+v", catalogEnvelope)
	}

	firstIntent := mcpToolText(t, firstResponses, "4")
	var intentEnvelope struct {
		Status  string     `json:"status"`
		Ruleset rules.Lock `json:"ruleset"`
		Outcome string     `json:"outcome"`
	}
	if err := json.Unmarshal([]byte(firstIntent), &intentEnvelope); err != nil {
		t.Fatal(err)
	}
	if intentEnvelope.Status != "resolved" || intentEnvelope.Ruleset != exactLock ||
		!strings.HasPrefix(intentEnvelope.Outcome, "pbta.move.") {
		t.Fatalf("PbtA intent result = %s", firstIntent)
	}
	var observeEnvelope struct {
		Status  string     `json:"status"`
		Ruleset rules.Lock `json:"ruleset"`
	}
	if err := json.Unmarshal([]byte(mcpToolText(t, firstResponses, "5")), &observeEnvelope); err != nil {
		t.Fatal(err)
	}
	if observeEnvelope.Status != "resolved" || observeEnvelope.Ruleset != exactLock {
		t.Fatalf("PbtA observation = %+v", observeEnvelope)
	}

	canonicalBefore := mustLoadState(t, store, state.Name)
	handoffBefore := mustReadState(t, handoffPath)
	runtimeBefore := mustRulesRuntime(t, canonicalBefore)
	if handoffRuntime := mustRulesRuntime(t, handoffBefore); !reflect.DeepEqual(runtimeBefore, handoffRuntime) {
		t.Fatalf("canonical and parent handoff diverged: canonical=%+v handoff=%+v", runtimeBefore, handoffRuntime)
	}
	if runtimeBefore.Lock != exactLock || len(runtimeBefore.Receipts) != 1 || len(runtimeBefore.RandomDraws) != 1 {
		t.Fatalf("persisted PbtA runtime = %+v", runtimeBefore)
	}
	if len(canonicalBefore.PartySnapshot()) != 0 || len(handoffBefore.PartySnapshot()) != 0 {
		t.Fatalf("foreign virtual-DM MCP launch created legacy party: canonical=%v handoff=%v", canonicalBefore.PartySnapshot(), handoffBefore.PartySnapshot())
	}

	// Restart the real MCP server with the same parent-turn namespace and JSON-RPC
	// request ID. The durable receipt must return the identical result without a
	// second random draw or generation advance.
	secondOutput := runMCPSubcommand(t, args, input)
	secondResponses := decodeMCPResponses(t, secondOutput)
	if secondIntent := mcpToolText(t, secondResponses, "4"); secondIntent != firstIntent {
		t.Fatalf("reloaded idempotent result changed:\nfirst:  %s\nsecond: %s", firstIntent, secondIntent)
	}
	canonicalAfter := mustLoadState(t, store, state.Name)
	handoffAfter := mustReadState(t, handoffPath)
	runtimeAfter := mustRulesRuntime(t, canonicalAfter)
	if !reflect.DeepEqual(runtimeAfter, mustRulesRuntime(t, handoffAfter)) {
		t.Fatal("reloaded canonical and parent handoff rules runtimes diverged")
	}
	if !reflect.DeepEqual(runtimeBefore, runtimeAfter) {
		t.Fatalf("idempotent reload mutated rules runtime: before=%+v after=%+v", runtimeBefore, runtimeAfter)
	}
	if len(canonicalAfter.PartySnapshot()) != 0 || len(handoffAfter.PartySnapshot()) != 0 {
		t.Fatal("reloaded foreign MCP server created legacy party state")
	}
}

func TestMCPDNDMutationRetryIsIdempotentAcrossReload(t *testing.T) {
	dataDirectory := t.TempDir()
	store, err := storage.NewWithPath(dataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	adventure := &domain.Adventure{
		SchemaVersion: domain.SchemaVersion,
		ID:            "mcp-dnd-mutation-e2e",
		Title:         "MCP D&D mutation E2E",
		Ruleset:       &rules.Requirement{ID: dnd5e.PackageID, Version: dnd5e.PackageVersion},
		Zones: []domain.Zone{{
			ID: "zone", Name: "Zone", Rooms: []domain.Room{{ID: "room", Name: "Room"}},
		}},
	}
	if err := os.MkdirAll(store.AdventureDir(adventure.ID), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAdventure(adventure.ID, adventure); err != nil {
		t.Fatal(err)
	}
	state := domain.NewSessionState("mcp-dnd-mutation-session", adventure)
	state.SetMode(domain.ModeVirtualDM)
	character := domain.NewCharacter("Kael", "Elf", "Wizard")
	character.MaxHP, character.CurrentHP = 20, 20
	state.SetParty([]*domain.Character{character})
	handoffPath := filepath.Join(t.TempDir(), "session.json")
	writeStateFile(t, handoffPath, state)
	args := []string{
		"--data-dir", dataDirectory,
		"--adventure-id", adventure.ID,
		"--session", handoffPath,
		"--request-namespace", "dnd-mutation-turn",
		"--language", "en",
		"--rules-timeout-seconds", "17",
	}
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"update_hp","arguments":{"delta":-8}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"update_hp","arguments":{"delta":-8}}}`,
	}, "\n")

	// Establish the exact binding, then retain a pre-mutation handoff to model a
	// child that crashed after publishing only the newer canonical checkpoint.
	runMCPSubcommand(t, args, "")
	staleHandoff := mustReadState(t, handoffPath)
	firstOutput := runMCPSubcommand(t, args, input)
	firstResponses := decodeMCPResponses(t, firstOutput)
	firstResult := mcpToolText(t, firstResponses, "2")
	canonicalBefore := mustLoadState(t, store, state.Name)
	if party := canonicalBefore.PartySnapshot(); len(party) != 1 || party[0].CurrentHP != 12 {
		t.Fatalf("first retry changed HP more than once: %+v", party)
	}
	runtimeBefore := mustRulesRuntime(t, canonicalBefore)
	if len(runtimeBefore.Receipts) != 1 || runtimeBefore.Receipts[0].Tool != "update_hp" {
		t.Fatalf("durable ordinary-tool receipt = %+v", runtimeBefore.Receipts)
	}

	writeStateFile(t, handoffPath, staleHandoff)
	secondOutput := runMCPSubcommand(t, args, input)
	secondResponses := decodeMCPResponses(t, secondOutput)
	if secondResult := mcpToolText(t, secondResponses, "2"); secondResult != firstResult {
		t.Fatalf("restart retry result changed: first=%q second=%q", firstResult, secondResult)
	}
	canonicalAfter := mustLoadState(t, store, state.Name)
	if party := canonicalAfter.PartySnapshot(); len(party) != 1 || party[0].CurrentHP != 12 {
		t.Fatalf("restart retry changed HP again: %+v", party)
	}
	if runtimeAfter := mustRulesRuntime(t, canonicalAfter); !reflect.DeepEqual(runtimeBefore, runtimeAfter) {
		t.Fatalf("restart retry advanced durable runtime: before=%+v after=%+v", runtimeBefore, runtimeAfter)
	}
	if handoff := mustReadState(t, handoffPath); !reflect.DeepEqual(mustRulesRuntime(t, handoff), runtimeBefore) {
		t.Fatal("handoff and canonical receipts diverged after restart")
	} else if party := handoff.PartySnapshot(); len(party) != 1 || party[0].CurrentHP != 12 {
		t.Fatalf("reconciled handoff rolled back the receipt-correlated HP effect: %+v", party)
	}
}

func TestMCPExternalPackageStateReopensAndRetriesExactly(t *testing.T) {
	ctx := context.Background()
	dataDirectory := t.TempDir()
	bundlePath := filepath.Join(t.TempDir(), "simple-d6.rules.zip")
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate MCP test")
	}
	source := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "../../examples/rules/simple-d6"))
	if _, err := bundlepack.Pack(ctx, source, bundlePath, nil); err != nil {
		t.Fatalf("pack external example: %v", err)
	}
	bootstrap, err := runtimecatalog.Load(ctx, dataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	installed, err := bootstrap.Store.InstallFile(ctx, bundlePath)
	if err != nil {
		t.Fatalf("install external example: %v", err)
	}

	store, err := storage.NewWithPath(dataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	adventure := &domain.Adventure{
		SchemaVersion: domain.SchemaVersion,
		ID:            "mcp-simple-d6-e2e",
		Title:         "MCP external E2E",
		Ruleset:       &rules.Requirement{ID: "simple-d6", Version: "0.1.0"},
		Zones: []domain.Zone{{
			ID: "zone", Name: "Zone", Rooms: []domain.Room{{ID: "room", Name: "Room"}},
		}},
	}
	if err := os.MkdirAll(store.AdventureDir(adventure.ID), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAdventure(adventure.ID, adventure); err != nil {
		t.Fatal(err)
	}
	state := domain.NewSessionState("mcp-simple-d6-session", adventure)
	handoffPath := filepath.Join(t.TempDir(), "session.json")
	writeStateFile(t, handoffPath, state)
	args := []string{
		"--data-dir", dataDirectory,
		"--adventure-id", adventure.ID,
		"--session", handoffPath,
		"--request-namespace", "simple-d6-turn",
		"--language", "en",
		"--rules-timeout-seconds", "17",
	}
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"game_submit_intent","arguments":{"action_id":"simple_d6.check","arguments":{"modifier":2,"target":6}}}}`,
	}, "\n")

	firstOutput := runMCPSubcommand(t, args, input)
	firstResponses := decodeMCPResponses(t, firstOutput)
	firstResult := mcpToolText(t, firstResponses, "2")
	canonicalBefore := mustLoadState(t, store, state.Name)
	handoffBefore := mustReadState(t, handoffPath)
	runtimeBefore := mustRulesRuntime(t, canonicalBefore)
	if runtimeBefore.Lock != installed.Loaded.Artifact.Lock() || runtimeBefore.Revision != 1 ||
		len(runtimeBefore.Receipts) != 1 || len(runtimeBefore.RandomDraws) != 1 {
		t.Fatalf("persisted external runtime = %+v", runtimeBefore)
	}
	if !reflect.DeepEqual(runtimeBefore, mustRulesRuntime(t, handoffBefore)) {
		t.Fatal("external canonical and handoff runtimes diverged")
	}

	secondOutput := runMCPSubcommand(t, args, input)
	secondResponses := decodeMCPResponses(t, secondOutput)
	if secondResult := mcpToolText(t, secondResponses, "2"); secondResult != firstResult {
		t.Fatalf("external restart retry changed: first=%q second=%q", firstResult, secondResult)
	}
	canonicalAfter := mustLoadState(t, store, state.Name)
	handoffAfter := mustReadState(t, handoffPath)
	runtimeAfter := mustRulesRuntime(t, canonicalAfter)
	if !reflect.DeepEqual(runtimeBefore, runtimeAfter) || !reflect.DeepEqual(runtimeAfter, mustRulesRuntime(t, handoffAfter)) {
		t.Fatalf("external restart advanced or diverged runtime: before=%+v after=%+v", runtimeBefore, runtimeAfter)
	}
}

type mcpTestResponse struct {
	ID     json.RawMessage `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

func runMCPSubcommand(t *testing.T, args []string, input string) string {
	t.Helper()
	var output bytes.Buffer
	if err := runSubcommand(args, strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	return output.String()
}

func decodeMCPResponses(t *testing.T, output string) map[string]mcpTestResponse {
	t.Helper()
	responses := make(map[string]mcpTestResponse)
	decoder := json.NewDecoder(strings.NewReader(output))
	for {
		var response mcpTestResponse
		if err := decoder.Decode(&response); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatal(err)
		}
		responses[string(response.ID)] = response
	}
	return responses
}

func mustUnmarshalMCPResult(t *testing.T, responses map[string]mcpTestResponse, id string, target any) {
	t.Helper()
	response, ok := responses[id]
	if !ok {
		t.Fatalf("missing MCP response id %s", id)
	}
	if len(response.Error) != 0 && string(response.Error) != "null" {
		t.Fatalf("MCP response id %s error = %s", id, response.Error)
	}
	if err := json.Unmarshal(response.Result, target); err != nil {
		t.Fatal(err)
	}
}

func mcpToolText(t *testing.T, responses map[string]mcpTestResponse, id string) string {
	t.Helper()
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	mustUnmarshalMCPResult(t, responses, id, &result)
	if result.IsError || len(result.Content) != 1 || result.Content[0].Type != "text" {
		t.Fatalf("MCP tool response id %s = %+v", id, result)
	}
	return result.Content[0].Text
}

func mustRulesRuntime(t *testing.T, state *domain.SessionState) domain.RulesSession {
	t.Helper()
	runtime, ok := state.RulesRuntimeSnapshot()
	if !ok {
		t.Fatal("session has no valid rules runtime")
	}
	return runtime
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

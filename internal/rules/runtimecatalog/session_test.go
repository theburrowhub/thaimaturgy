package runtimecatalog

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/rules"
	"github.com/theburrowhub/thaimaturgy/internal/rules/bundlepack"
	"github.com/theburrowhub/thaimaturgy/internal/ruleshost"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

func TestResolveSessionRulesBindsEveryBuiltinRequirement(t *testing.T) {
	environment, err := Load(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	packageIDs := []string{
		"dnd5e", "pf2e", "runequest", "coc7e", "vtm5e",
		"shadowrun6e", "pbta", "gurps4e", "fatecore", "savageworlds",
	}
	for _, packageID := range packageIDs {
		t.Run(packageID, func(t *testing.T) {
			adventure := rulesAdventure(packageID)
			state := domain.NewSessionState("session-"+packageID, adventure)
			implementation, created, err := environment.resolveSessionRules(context.Background(), state, adventure)
			if err != nil {
				t.Fatal(err)
			}
			if implementation == nil || !created {
				t.Fatalf("implementation=%T created=%v", implementation, created)
			}
			runtime, exists, err := state.RulesRuntimeSnapshotStrict()
			if err != nil || !exists {
				t.Fatalf("runtime exists=%v err=%v", exists, err)
			}
			if runtime.Lock.ID != packageID || runtime.Lock.Version != "0.1.0" || runtime.Lock.Digest == "" {
				t.Fatalf("unexpected lock: %+v", runtime.Lock)
			}
			if runtime.Revision != 0 || runtime.Generation != 0 || runtime.State.String() != runtime.InitialState.String() {
				t.Fatalf("unexpected initial runtime: %+v", runtime)
			}
		})
	}
}

func TestResolveSessionRulesPinnedLockWinsOverChangedAdventureRequirement(t *testing.T) {
	environment, err := Load(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	original := rulesAdventure("fatecore")
	state := domain.NewSessionState("pinned", original)
	if _, created, err := environment.resolveSessionRules(context.Background(), state, original); err != nil || !created {
		t.Fatalf("initial bind created=%v err=%v", created, err)
	}
	before, _, _ := state.RulesRuntimeSnapshotStrict()

	changed := *original
	changed.Ruleset = &rules.Requirement{ID: "dnd5e", Version: "0.1.0"}
	implementation, created, err := environment.resolveSessionRules(context.Background(), state, &changed)
	if err != nil {
		t.Fatal(err)
	}
	if implementation == nil || created {
		t.Fatalf("implementation=%T created=%v", implementation, created)
	}
	after, _, _ := state.RulesRuntimeSnapshotStrict()
	if after.Lock != before.Lock || after.Generation != before.Generation || after.State.String() != before.State.String() {
		t.Fatalf("pinned runtime changed: before=%+v after=%+v", before, after)
	}
}

func TestOpenSessionInjectsExactResolverAndMarksOnlyNewBinding(t *testing.T) {
	environment, err := Load(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	adventure := rulesAdventure("pbta")
	state := domain.NewSessionState("open", adventure)
	config := domain.DefaultConfig()

	session, err := environment.OpenSession(context.Background(), state, adventure, config)
	if err != nil {
		t.Fatal(err)
	}
	if session.State != state || session.Adventure != adventure || session.Config != config {
		t.Fatal("OpenSession did not preserve its domain inputs")
	}
	if session.RulesResolver != environment.Catalog || session.DataDirectory != environment.DataDirectory {
		t.Fatalf("runtime injection resolver=%T data=%q", session.RulesResolver, session.DataDirectory)
	}
	if !session.IsModified {
		t.Fatal("new exact binding was not marked for persistence")
	}

	reopened, err := environment.OpenSession(context.Background(), state, adventure, config)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.IsModified {
		t.Fatal("validated pinned session was marked modified")
	}
}

func TestExternalStateSurvivesIndentedStorageAndExactReopen(t *testing.T) {
	ctx := context.Background()
	dataDirectory := t.TempDir()
	bundlePath := filepath.Join(t.TempDir(), "simple-d6.rules.zip")
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate runtime catalog test")
	}
	source := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "../../../examples/rules/simple-d6"))
	if _, err := bundlepack.Pack(ctx, source, bundlePath, nil); err != nil {
		t.Fatalf("pack example: %v", err)
	}
	bootstrap, err := Load(ctx, dataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bootstrap.Store.InstallFile(ctx, bundlePath); err != nil {
		t.Fatalf("install example: %v", err)
	}

	environment, err := Load(ctx, dataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	adventure := rulesAdventure("simple-d6")
	state := domain.NewSessionState("external-round-trip", adventure)
	if _, err := environment.OpenSession(ctx, state, adventure, domain.DefaultConfig()); err != nil {
		t.Fatalf("open new external session: %v", err)
	}
	runtimeState, exists, err := state.RulesRuntimeSnapshotStrict()
	if err != nil || !exists {
		t.Fatalf("initial runtime exists=%v err=%v", exists, err)
	}
	implementation, err := environment.Catalog.Lookup(runtimeState.Lock)
	if err != nil {
		t.Fatal(err)
	}
	eventData := mustRulesPayload(t, map[string]any{
		"intent_id": "persisted-intent", "roll": 4, "modifier": 2,
		"target": 6, "total": 6, "success": true,
	})
	event := rules.Event{Type: "simple_d6.check_resolved", SchemaVersion: 1, Data: eventData}
	reduced, _, err := (ruleshost.Executor{Ruleset: implementation}).Reduce(ctx, rules.Snapshot{
		Ruleset: runtimeState.Lock, Revision: runtimeState.Revision, State: runtimeState.State,
	}, rules.Emission{Events: []rules.Event{event}})
	if err != nil {
		t.Fatalf("reduce external event: %v", err)
	}
	handle, receipt, err := state.BeginRulesRequest(ctx, "persisted-request", "game_submit_intent", "sha256:"+strings.Repeat("b", 64))
	if err != nil || receipt != nil {
		t.Fatalf("begin persisted request receipt=%v err=%v", receipt, err)
	}
	if _, err := state.CommitRulesRequest(handle, domain.RulesCommit{
		State: reduced.State, ResolutionID: "persisted-resolution",
		Principal: rules.Principal{ID: "test:host", Kind: "host"},
		EventBatches: []domain.RulesEventDraft{{
			ResolutionID: "persisted-resolution", Events: []rules.Event{event},
		}},
		Result: &domain.RulesStoredResult{Content: `{"status":"complete"}`},
	}); err != nil {
		t.Fatalf("commit external event: %v", err)
	}

	store, err := storage.NewWithPath(dataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSession(state); err != nil {
		t.Fatalf("save indented session: %v", err)
	}
	restored, err := store.LoadSession(state.Name)
	if err != nil {
		t.Fatalf("load indented session: %v", err)
	}
	restarted, err := Load(ctx, dataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.OpenSession(ctx, restored, adventure, domain.DefaultConfig()); err != nil {
		t.Fatalf("reopen exact external session: %v", err)
	}
	after, exists, err := restored.RulesRuntimeSnapshotStrict()
	if err != nil || !exists || after.Revision != 1 || after.State.String() != reduced.State.String() {
		t.Fatalf("restored runtime exists=%v err=%v state=%+v", exists, err, after)
	}
}

func TestResolveSessionRulesFailsClosedWithoutMutatingState(t *testing.T) {
	environment, err := Load(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	empty := mustRulesPayload(t, map[string]any{})
	missingLock := rules.Lock{
		ID: "missing.rules", Version: "1.0.0", ProtocolVersion: rules.ProtocolVersion,
		Digest: "sha256:" + strings.Repeat("0", 64),
	}

	tests := []struct {
		name      string
		adventure *domain.Adventure
		prepare   func(*testing.T, *domain.SessionState)
		ctx       func() context.Context
		want      string
	}{
		{
			name: "unknown requirement", adventure: &domain.Adventure{ID: "unknown", System: "Mystery system"},
			want: "no valid rules package requirement",
		},
		{
			name: "different adventure", adventure: rulesAdventure("dnd5e"),
			prepare: func(_ *testing.T, state *domain.SessionState) {
				state.AdventureID = "somewhere-else"
			},
			want: "does not match loaded adventure",
		},
		{
			name: "missing exact lock", adventure: rulesAdventure("dnd5e"),
			prepare: func(t *testing.T, state *domain.SessionState) {
				t.Helper()
				if created, err := state.BindRules(missingLock, empty); err != nil || !created {
					t.Fatalf("bind missing lock created=%v err=%v", created, err)
				}
			},
			want: "load exact session lock",
		},
		{
			name: "corrupt runtime", adventure: rulesAdventure("dnd5e"),
			prepare: func(t *testing.T, state *domain.SessionState) {
				t.Helper()
				if _, _, err := environment.resolveSessionRules(context.Background(), state, rulesAdventure("dnd5e")); err != nil {
					t.Fatal(err)
				}
				state.Rules.State = rules.Payload{}
			},
			want: "invalid persisted rules runtime",
		},
		{
			name: "tampered initial state", adventure: rulesAdventure("dnd5e"),
			prepare: func(t *testing.T, state *domain.SessionState) {
				t.Helper()
				if _, _, err := environment.resolveSessionRules(context.Background(), state, rulesAdventure("dnd5e")); err != nil {
					t.Fatal(err)
				}
				tampered := mustRulesPayload(t, map[string]any{"tampered": true})
				state.Rules.InitialState = tampered
				state.Rules.State = tampered
			},
			want: "initial state does not match",
		},
		{
			name: "canceled before bind", adventure: rulesAdventure("dnd5e"),
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			want: context.Canceled.Error(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := domain.NewSessionState("failure", test.adventure)
			if test.prepare != nil {
				test.prepare(t, state)
			}
			before := marshalSessionState(t, state)
			ctx := context.Background()
			if test.ctx != nil {
				ctx = test.ctx()
			}
			_, created, err := environment.resolveSessionRules(ctx, state, test.adventure)
			if err == nil || !strings.Contains(err.Error(), test.want) || created {
				t.Fatalf("created=%v err=%v, want error containing %q", created, err, test.want)
			}
			after := marshalSessionState(t, state)
			if string(after) != string(before) {
				t.Fatalf("failed resolution mutated session\nbefore=%s\nafter=%s", before, after)
			}
		})
	}
}

func TestResolveSessionRulesReturnsCancellationIdentity(t *testing.T) {
	environment, err := Load(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err = environment.resolveSessionRules(ctx, domain.NewSessionState("cancel", rulesAdventure("dnd5e")), rulesAdventure("dnd5e"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func rulesAdventure(packageID string) *domain.Adventure {
	return &domain.Adventure{
		SchemaVersion: domain.SchemaVersion,
		ID:            "adventure-" + packageID,
		Title:         "Rules test",
		Ruleset: &rules.Requirement{
			ID: packageID, Version: rules.VersionConstraint("0.1.0"),
		},
	}
}

func marshalSessionState(t *testing.T, state *domain.SessionState) []byte {
	t.Helper()
	encoded, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func mustRulesPayload(t *testing.T, value any) rules.Payload {
	t.Helper()
	payload, err := rules.PayloadFrom(value)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

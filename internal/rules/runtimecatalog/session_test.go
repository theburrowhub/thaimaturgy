package runtimecatalog

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/rules"
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

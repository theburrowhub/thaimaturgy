package reference_test

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	core "github.com/theburrowhub/thaimaturgy/internal/rules"
	"github.com/theburrowhub/thaimaturgy/internal/rules/coc7e"
	"github.com/theburrowhub/thaimaturgy/internal/rules/fatecore"
	"github.com/theburrowhub/thaimaturgy/internal/rules/gurps4e"
	"github.com/theburrowhub/thaimaturgy/internal/rules/pbta"
	"github.com/theburrowhub/thaimaturgy/internal/rules/pf2e"
	"github.com/theburrowhub/thaimaturgy/internal/rules/ruleskit"
	"github.com/theburrowhub/thaimaturgy/internal/rules/runequest"
	"github.com/theburrowhub/thaimaturgy/internal/rules/savageworlds"
	"github.com/theburrowhub/thaimaturgy/internal/rules/shadowrun6e"
	"github.com/theburrowhub/thaimaturgy/internal/rules/vtm5e"
)

type referenceCase struct {
	name          string
	id            string
	action        string
	newRuleset    func() core.Ruleset
	artifact      func() (core.Artifact, error)
	initialState  func() core.Payload
	arguments     any
	randomReplies [][]int
	wantOutcome   string
}

func referenceCases() []referenceCase {
	return []referenceCase{
		{
			name: "pf2e", id: pf2e.PackageID, action: pf2e.ActionCheck,
			newRuleset: func() core.Ruleset { return pf2e.New() }, artifact: pf2e.NewArtifact, initialState: pf2e.InitialState,
			arguments: map[string]any{"modifier": 5, "dc": 20}, randomReplies: [][]int{{15}}, wantOutcome: "pf2e.check.success",
		},
		{
			name: "runequest", id: runequest.PackageID, action: runequest.ActionSkillTest,
			newRuleset: func() core.Ruleset { return runequest.New() }, artifact: runequest.NewArtifact, initialState: runequest.InitialState,
			arguments: map[string]any{"skill": 60}, randomReplies: [][]int{{12}}, wantOutcome: "runequest.skill.special",
		},
		{
			name: "coc7e", id: coc7e.PackageID, action: coc7e.ActionCheck,
			newRuleset: func() core.Ruleset { return coc7e.New() }, artifact: coc7e.NewArtifact, initialState: coc7e.InitialState,
			arguments: map[string]any{"skill": 60, "difficulty": "hard", "bonus_dice": 1}, randomReplies: [][]int{{3, 8, 2}}, wantOutcome: "coc7e.check.hard_success",
		},
		{
			name: "vtm5e", id: vtm5e.PackageID, action: vtm5e.ActionPoolCheck,
			newRuleset: func() core.Ruleset { return vtm5e.New() }, artifact: vtm5e.NewArtifact, initialState: vtm5e.InitialState,
			arguments: map[string]any{"pool": 4, "hunger": 1, "difficulty": 2}, randomReplies: [][]int{{10, 10, 2, 6}}, wantOutcome: "vtm5e.pool.messy_critical",
		},
		{
			name: "shadowrun6e", id: shadowrun6e.PackageID, action: shadowrun6e.ActionPoolCheck,
			newRuleset: func() core.Ruleset { return shadowrun6e.New() }, artifact: shadowrun6e.NewArtifact, initialState: shadowrun6e.InitialState,
			arguments: map[string]any{"pool": 4, "threshold": 2}, randomReplies: [][]int{{5, 6, 2, 3}}, wantOutcome: "shadowrun6e.pool.success",
		},
		{
			name: "pbta", id: pbta.PackageID, action: pbta.ActionMove,
			newRuleset: func() core.Ruleset { return pbta.New() }, artifact: pbta.NewArtifact, initialState: pbta.InitialState,
			arguments: map[string]any{"modifier": 1}, randomReplies: [][]int{{4, 5}}, wantOutcome: "pbta.move.strong_hit",
		},
		{
			name: "gurps4e", id: gurps4e.PackageID, action: gurps4e.ActionCheck,
			newRuleset: func() core.Ruleset { return gurps4e.New() }, artifact: gurps4e.NewArtifact, initialState: gurps4e.InitialState,
			arguments: map[string]any{"skill": 12}, randomReplies: [][]int{{3, 4, 4}}, wantOutcome: "gurps4e.skill.success",
		},
		{
			name: "fatecore", id: fatecore.PackageID, action: fatecore.ActionResolve,
			newRuleset: func() core.Ruleset { return fatecore.New() }, artifact: fatecore.NewArtifact, initialState: fatecore.InitialState,
			arguments: map[string]any{"skill": 2, "opposition": 2, "invokes": []string{"High Ground"}}, randomReplies: [][]int{{1, 2, 2, 3}}, wantOutcome: "fatecore.action.succeed",
		},
		{
			name: "savageworlds", id: savageworlds.PackageID, action: savageworlds.ActionTraitTest,
			newRuleset: func() core.Ruleset { return savageworlds.New() }, artifact: savageworlds.NewArtifact, initialState: savageworlds.InitialState,
			arguments: map[string]any{"trait_die": 8, "wild_card": false, "target_number": 4}, randomReplies: [][]int{{5}}, wantOutcome: "savageworlds.trait.success",
		},
	}
}

func TestBuiltinArtifactDigestsArePinned(t *testing.T) {
	want := map[string]string{
		"pf2e":         "sha256:71623f28a2fb44db808f8a5211899cd24c987b853d6a07b4dfac0ddf3897abf8",
		"runequest":    "sha256:13f5c26d7aee05f67e649d49ce8a5d03f9cae3d6a8d90eb884b8d83e0acfefc5",
		"coc7e":        "sha256:61355a937eb68935fe50001781fe8ecf60a8d709d2cea1af9d9bda6640edef27",
		"vtm5e":        "sha256:aceaf16503d15d0ee5958f45987e7ad37a2588aa078b4241e78708dc0e8f66a1",
		"shadowrun6e":  "sha256:b7444ff6c5d8b9775bff27f88574c043ea6bbd118d020d637f9b04243dd58cb6",
		"pbta":         "sha256:f1c93a2781a1246b65f0a7b3413e42219c0d935abe4d1dd11bc93f0f683de167",
		"gurps4e":      "sha256:79e29cdf64d7413559c7df301f331a139251e8af5bdd8746cb4bdfdc0bf17314",
		"fatecore":     "sha256:1569b816f62227c41375ef7495d9b58b9a3133b38cde0ec2b242bdcdd49bedbe",
		"savageworlds": "sha256:2912b0820236c09ea2745cb1080abf5c89f1afaa1b26798a15fb95bebdb6dbc4",
	}
	for _, test := range referenceCases() {
		artifact, err := test.artifact()
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		if got := artifact.Digest(); got != want[test.name] {
			t.Errorf("%s digest = %q, want %q", test.name, got, want[test.name])
		}
	}
}

func TestReferenceRulesetsConformToProtocol(t *testing.T) {
	for _, test := range referenceCases() {
		t.Run(test.name, func(t *testing.T) {
			artifact, err := test.artifact()
			if err != nil {
				t.Fatal(err)
			}
			if err := artifact.Validate(); err != nil {
				t.Fatal(err)
			}
			manifest, err := test.newRuleset().Manifest(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(manifest, artifact.Manifest()) {
				t.Fatalf("manifest %#v differs from artifact %#v", manifest, artifact.Manifest())
			}
			if manifest.ID != test.id || manifest.Version != "0.1.0" || manifest.ProtocolVersion != core.ProtocolVersion || manifest.Runtime.Kind != core.RuntimeBuiltin {
				t.Fatalf("unexpected manifest identity: %#v", manifest)
			}
			if !slices.Equal(manifest.Capabilities, []string{test.action}) {
				t.Fatalf("capabilities = %v, want [%s]", manifest.Capabilities, test.action)
			}
			if manifest.Description == "" || strings.Contains(strings.ToLower(manifest.Description), "complete rules") {
				t.Fatalf("manifest must describe bounded reference scope: %#v", manifest)
			}

			snapshot := testSnapshot(artifact, test.initialState())
			principal := testPrincipal()
			ruleset := test.newRuleset()
			actions, err := ruleset.ListActions(context.Background(), core.CatalogRequest{Snapshot: snapshot, Principal: principal})
			if err != nil {
				t.Fatal(err)
			}
			if err := core.ValidateActions(actions); err != nil {
				t.Fatal(err)
			}
			if len(actions) != 1 || actions[0].ID != test.action {
				t.Fatalf("actions = %#v, want only %q", actions, test.action)
			}
			if err := ruleset.ValidateState(context.Background(), core.ValidateStateRequest{Snapshot: snapshot}); err != nil {
				t.Fatal(err)
			}
			projection, err := ruleset.Project(context.Background(), core.ProjectRequest{Snapshot: snapshot, Principal: principal})
			if err != nil || projection.View.String() != "{}" {
				t.Fatalf("projection = %#v, err = %v", projection, err)
			}
			explanation, err := ruleset.Explain(context.Background(), core.ExplainRequest{Snapshot: snapshot, Principal: principal, Reference: test.action, Locale: "en"})
			if err != nil || explanation.Text == "" {
				t.Fatalf("explanation = %#v, err = %v", explanation, err)
			}

			startRequest := core.StartRequest{
				Snapshot: snapshot, Principal: principal,
				Intent: core.Intent{ID: "intent-1", ActionID: test.action, Arguments: mustPayload(t, test.arguments)},
			}
			first, err := test.newRuleset().Start(context.Background(), startRequest)
			if err != nil {
				t.Fatal(err)
			}
			second, err := test.newRuleset().Start(context.Background(), startRequest)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(first, second) {
				t.Fatalf("Start is not deterministic:\nfirst=%#v\nsecond=%#v", first, second)
			}
			if first.Kind != core.StepKindNeedRandom {
				t.Fatalf("start step = %#v, want need_random", first)
			}

			step := roundTripStep(t, first)
			for replyIndex, rolls := range test.randomReplies {
				pending := mustPending(t, step)
				response := core.HostResponse{StepID: step.ID, Kind: step.Kind, Data: mustPayload(t, ruleskit.DiceResponse{Rolls: rolls})}
				request := core.ResumeRequest{Snapshot: snapshot, Principal: principal, Pending: pending, Response: response}
				nextA, err := test.newRuleset().Resume(context.Background(), request)
				if err != nil {
					t.Fatalf("resume %d: %v", replyIndex, err)
				}
				nextB, err := test.newRuleset().Resume(context.Background(), request)
				if err != nil {
					t.Fatalf("repeat resume %d: %v", replyIndex, err)
				}
				if !reflect.DeepEqual(nextA, nextB) {
					t.Fatalf("Resume is not deterministic:\nfirst=%#v\nsecond=%#v", nextA, nextB)
				}
				step = roundTripStep(t, nextA)
			}
			if step.Kind != core.StepKindComplete || step.Complete == nil || step.Complete.Outcome != test.wantOutcome {
				t.Fatalf("completion = %#v, want outcome %q", step, test.wantOutcome)
			}

			migration, err := ruleset.Migrate(context.Background(), core.MigrateRequest{From: artifact.Lock(), State: test.initialState()})
			if err != nil || migration.State.String() != "{}" {
				t.Fatalf("identity migration = %#v, err = %v", migration, err)
			}
			_, err = ruleset.Reduce(context.Background(), core.ReduceRequest{
				Snapshot: snapshot,
				Events:   []core.Event{{Type: "noop", SchemaVersion: 1, Data: mustPayload(t, struct{}{})}},
			})
			if err == nil {
				t.Fatal("stateless reference package accepted an event")
			}
			foreign := artifact.Lock()
			foreign.ID = "other"
			if _, err := ruleset.Migrate(context.Background(), core.MigrateRequest{From: foreign, State: test.initialState()}); err == nil {
				t.Fatal("foreign artifact migration was accepted")
			}
			wrongSnapshot := snapshot
			wrongSnapshot.Ruleset = foreign
			if err := ruleset.ValidateState(context.Background(), core.ValidateStateRequest{Snapshot: wrongSnapshot}); err == nil {
				t.Fatal("snapshot for a foreign lock was accepted")
			}
		})
	}
}

func TestReferenceRulesetsRejectAmbiguousInputsAndRandomResponses(t *testing.T) {
	for _, test := range referenceCases() {
		t.Run(test.name, func(t *testing.T) {
			artifact, err := test.artifact()
			if err != nil {
				t.Fatal(err)
			}
			snapshot := testSnapshot(artifact, test.initialState())
			principal := testPrincipal()
			ruleset := test.newRuleset()

			rejected, err := ruleset.Start(context.Background(), core.StartRequest{
				Snapshot: snapshot, Principal: principal,
				Intent: core.Intent{ID: "invalid-input", ActionID: test.action, Arguments: mustRawPayload(t, `{"unexpected":true}`)},
			})
			if err != nil || rejected.Kind != core.StepKindReject || rejected.Reject == nil {
				t.Fatalf("malformed arguments = %#v, err = %v", rejected, err)
			}

			unknown, err := ruleset.Start(context.Background(), core.StartRequest{
				Snapshot: snapshot, Principal: principal,
				Intent: core.Intent{ID: "unknown-action", ActionID: "unknown.action", Arguments: mustPayload(t, struct{}{})},
			})
			if err != nil || unknown.Kind != core.StepKindReject || unknown.Reject.Code != "unknown.action" {
				t.Fatalf("unknown action = %#v, err = %v", unknown, err)
			}

			started, err := ruleset.Start(context.Background(), core.StartRequest{
				Snapshot: snapshot, Principal: principal,
				Intent: core.Intent{ID: "invalid-random", ActionID: test.action, Arguments: mustPayload(t, test.arguments)},
			})
			if err != nil {
				t.Fatal(err)
			}
			var specification ruleskit.DiceSpecification
			if err := ruleskit.Decode(started.NeedRandom.Specification, &specification); err != nil {
				t.Fatal(err)
			}
			pending := mustPending(t, started)
			badCount := core.ResumeRequest{
				Snapshot: snapshot, Principal: principal, Pending: pending,
				Response: core.HostResponse{StepID: started.ID, Kind: started.Kind, Data: mustPayload(t, ruleskit.DiceResponse{Rolls: nil})},
			}
			if _, err := test.newRuleset().Resume(context.Background(), badCount); err == nil || !strings.Contains(err.Error(), "want") {
				t.Fatalf("wrong cardinality error = %v", err)
			}

			outOfRange := make([]int, specification.Count)
			for i := range outOfRange {
				outOfRange[i] = 1
			}
			outOfRange[0] = specification.Sides + 1
			badFace := badCount
			badFace.Response.Data = mustPayload(t, ruleskit.DiceResponse{Rolls: outOfRange})
			if _, err := test.newRuleset().Resume(context.Background(), badFace); err == nil || !strings.Contains(err.Error(), "between 1 and") {
				t.Fatalf("out-of-range error = %v", err)
			}

			var continuation map[string]any
			if err := json.Unmarshal(started.Continuation.Bytes(), &continuation); err != nil {
				t.Fatal(err)
			}
			continuation["unexpected"] = true
			tampered := badCount
			tampered.Pending.State = mustPayload(t, continuation)
			validRolls := make([]int, specification.Count)
			for i := range validRolls {
				validRolls[i] = 1
			}
			tampered.Response.Data = mustPayload(t, ruleskit.DiceResponse{Rolls: validRolls})
			if _, err := test.newRuleset().Resume(context.Background(), tampered); err == nil || !strings.Contains(err.Error(), "unknown field") {
				t.Fatalf("tampered continuation error = %v", err)
			}

			delete(continuation, "unexpected")
			delete(continuation, "action")
			tampered.Pending.State = mustPayload(t, continuation)
			if _, err := test.newRuleset().Resume(context.Background(), tampered); err == nil || !strings.Contains(err.Error(), "missing required field") {
				t.Fatalf("incomplete continuation error = %v", err)
			}

			badState := snapshot
			badState.State = mustRawPayload(t, `{"unexpected":true}`)
			if err := ruleset.ValidateState(context.Background(), core.ValidateStateRequest{Snapshot: badState}); err == nil {
				t.Fatal("state with unknown field was accepted")
			}
		})
	}
}

func testSnapshot(artifact core.Artifact, state core.Payload) core.Snapshot {
	return core.Snapshot{Ruleset: artifact.Lock(), Revision: 7, State: state}
}

func testPrincipal() core.Principal {
	return core.Principal{ID: "player-1", Kind: "player", Roles: []string{"actor"}}
}

func mustPayload(t *testing.T, value any) core.Payload {
	t.Helper()
	payload, err := core.PayloadFrom(value)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func mustRawPayload(t *testing.T, raw string) core.Payload {
	t.Helper()
	payload, err := core.NewPayload([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func mustPending(t *testing.T, step core.Step) core.PendingStep {
	t.Helper()
	pending, err := step.Pending()
	if err != nil {
		t.Fatal(err)
	}
	return pending
}

func roundTripStep(t *testing.T, step core.Step) core.Step {
	t.Helper()
	encoded, err := json.Marshal(step)
	if err != nil {
		t.Fatal(err)
	}
	var restored core.Step
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	if err := restored.Validate(); err != nil {
		t.Fatal(err)
	}
	return restored
}

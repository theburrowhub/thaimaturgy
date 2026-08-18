package dnd5e

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	core "github.com/theburrowhub/thaimaturgy/internal/rules"
)

func TestManifestArtifactAndActionCatalogAreStable(t *testing.T) {
	artifact, err := NewArtifact()
	if err != nil {
		t.Fatal(err)
	}

	ruleset := New()
	manifest, err := ruleset.Manifest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(manifest, artifact.Manifest()) {
		t.Fatalf("ruleset manifest %#v differs from artifact manifest %#v", manifest, artifact.Manifest())
	}
	if manifest.ID != PackageID || manifest.Version != PackageVersion || manifest.ProtocolVersion != core.ProtocolVersion {
		t.Fatalf("unexpected manifest identity: %#v", manifest)
	}

	actions, err := ruleset.ListActions(context.Background(), core.CatalogRequest{
		Snapshot:  testSnapshot(t),
		Principal: testPrincipal(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := core.ValidateActions(actions); err != nil {
		t.Fatal(err)
	}
	if len(actions) != 2 {
		t.Fatalf("action count = %d, want 2", len(actions))
	}
	gotIDs := []string{actions[0].ID, actions[1].ID}
	wantIDs := []string{ActionDiceRoll, ActionAbilityCheck}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("action IDs = %v, want %v", gotIDs, wantIDs)
	}
	for _, action := range actions {
		if !strings.Contains(action.InputSchema.String(), `"maxLength":16384`) {
			t.Fatalf("action %q does not advertise its text limit: %s", action.ID, action.InputSchema.String())
		}
	}
}

func TestDiceRollSurvivesRestartAndPreservesLegacyResult(t *testing.T) {
	snapshot := testSnapshot(t)
	step := startAction(t, New(), snapshot, ActionDiceRoll, map[string]any{
		"notation": " 2D6+3 ",
		"reason":   "Falling rocks",
	})
	assertRandomRequest(t, step, DiceRandomRequest{Count: 2, Sides: 6})

	// Persist and restore the protocol value, then resume on a fresh ruleset.
	encoded, err := json.Marshal(step)
	if err != nil {
		t.Fatal(err)
	}
	var restored core.Step
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	complete := resumeAction(t, New(), snapshot, restored, DiceRandomResponse{Rolls: []int{4, 5}})
	if complete.Kind != core.StepKindComplete || complete.Complete.Outcome != "dnd5e.dice.rolled" {
		t.Fatalf("unexpected completion: %#v", complete)
	}

	var result DiceRollResult
	decodeTestPayload(t, complete.Complete.Result, &result)
	wantContent := "Rolled 2d6+3: [4+5]+3 = 12"
	want := DiceRollResult{
		Notation: "2d6+3",
		Rolls:    []int{4, 5},
		Modifier: 3,
		Total:    12,
		Traits:   CriticalTraits{},
		Legacy: LegacyResult{
			Content:    wantContent,
			LogMessage: "Falling rocks — " + wantContent,
			LogType:    "roll",
		},
	}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("result = %#v, want %#v", result, want)
	}
}

func TestDiceRollCriticalLegacyAnnotations(t *testing.T) {
	tests := []struct {
		name        string
		roll        int
		wantContent string
		wantTraits  CriticalTraits
	}{
		{
			name:        "critical hit",
			roll:        20,
			wantContent: "Rolled 1d20: [20] = 20 [CRIT!]",
			wantTraits:  CriticalTraits{CriticalHit: true},
		},
		{
			name:        "critical failure",
			roll:        1,
			wantContent: "Rolled 1d20: [1] = 1 [FUMBLE!]",
			wantTraits:  CriticalTraits{CriticalFail: true},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot := testSnapshot(t)
			step := startAction(t, New(), snapshot, ActionDiceRoll, map[string]any{"notation": "D20"})
			complete := resumeAction(t, New(), snapshot, step, DiceRandomResponse{Rolls: []int{test.roll}})
			var result DiceRollResult
			decodeTestPayload(t, complete.Complete.Result, &result)
			if result.Traits != test.wantTraits {
				t.Fatalf("traits = %#v, want %#v", result.Traits, test.wantTraits)
			}
			if result.Legacy.Content != test.wantContent || result.Legacy.LogMessage != test.wantContent || result.Legacy.LogType != "roll" {
				t.Fatalf("legacy = %#v", result.Legacy)
			}
		})
	}
}

func TestAbilityCheckNaturalRollsDoNotOverrideTotalComparison(t *testing.T) {
	tests := []struct {
		name        string
		arguments   map[string]any
		roll        int
		wantSuccess bool
		wantTraits  CriticalTraits
		wantContent string
		wantOutcome string
	}{
		{
			name:        "natural 20 can fail",
			arguments:   map[string]any{"modifier": 0, "dc": 25, "label": "Climb"},
			roll:        20,
			wantSuccess: false,
			wantTraits:  CriticalTraits{CriticalHit: true},
			wantContent: "Climb — Check (DC 25): d20(20)+0 = 20 [FAILURE] [NAT 20]",
			wantOutcome: "dnd5e.ability_check.failure",
		},
		{
			name:        "natural 1 can succeed",
			arguments:   map[string]any{"modifier": 0, "dc": 1},
			roll:        1,
			wantSuccess: true,
			wantTraits:  CriticalTraits{CriticalFail: true},
			wantContent: "Check (DC 1): d20(1)+0 = 1 [SUCCESS] [NAT 1]",
			wantOutcome: "dnd5e.ability_check.success",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot := testSnapshot(t)
			step := startAction(t, New(), snapshot, ActionAbilityCheck, test.arguments)
			assertRandomRequest(t, step, DiceRandomRequest{Count: 1, Sides: 20})
			encoded, err := json.Marshal(step)
			if err != nil {
				t.Fatal(err)
			}
			var restored core.Step
			if err := json.Unmarshal(encoded, &restored); err != nil {
				t.Fatal(err)
			}
			complete := resumeAction(t, New(), snapshot, restored, DiceRandomResponse{Rolls: []int{test.roll}})
			if complete.Complete.Outcome != test.wantOutcome {
				t.Fatalf("outcome = %q, want %q", complete.Complete.Outcome, test.wantOutcome)
			}
			var result AbilityCheckResult
			decodeTestPayload(t, complete.Complete.Result, &result)
			if result.Success != test.wantSuccess || result.Traits != test.wantTraits {
				t.Fatalf("success/traits = %t/%#v, want %t/%#v", result.Success, result.Traits, test.wantSuccess, test.wantTraits)
			}
			wantLegacy := LegacyResult{Content: test.wantContent, LogMessage: test.wantContent, LogType: "roll"}
			if result.Legacy != wantLegacy {
				t.Fatalf("legacy = %#v, want %#v", result.Legacy, wantLegacy)
			}
		})
	}
}

func TestStartReturnsLegacyCompatibleRejections(t *testing.T) {
	tests := []struct {
		name, action  string
		arguments     map[string]any
		code, message string
	}{
		{"missing notation", ActionDiceRoll, map[string]any{}, "invalid.arguments", "missing 'notation'"},
		{"invalid notation", ActionDiceRoll, map[string]any{"notation": "nope"}, "invalid.notation", "invalid dice notation: nope (expected format: NdM or NdM+K)"},
		{"missing modifier", ActionAbilityCheck, map[string]any{"dc": 10}, "invalid.arguments", "missing 'modifier'"},
		{"missing dc", ActionAbilityCheck, map[string]any{"modifier": 2}, "invalid.arguments", "missing 'dc'"},
		{"modifier overflow", ActionAbilityCheck, map[string]any{"modifier": int(^uint(0) >> 1), "dc": 10}, "invalid.arguments", "modifier must be between -1000000 and 1000000"},
		{"negative dc", ActionAbilityCheck, map[string]any{"modifier": 2, "dc": -1}, "invalid.arguments", "dc must be between 0 and 1000000"},
		{"dc overflow", ActionAbilityCheck, map[string]any{"modifier": 2, "dc": int(^uint(0) >> 1)}, "invalid.arguments", "dc must be between 0 and 1000000"},
		{"unknown action", "other.action", map[string]any{}, "unknown.action", "unknown action: other.action"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			step := startAction(t, New(), testSnapshot(t), test.action, test.arguments)
			if step.Kind != core.StepKindReject || step.Reject.Code != test.code || step.Reject.Message != test.message {
				t.Fatalf("rejection = %#v, want %q/%q", step, test.code, test.message)
			}
		})
	}
}

func TestStartRejectsArgumentsOutsideAdvertisedSchema(t *testing.T) {
	tests := []struct {
		name, action, raw, contains string
	}{
		{"unknown field", ActionDiceRoll, `{"notation":"1d20","seed":7}`, "unknown field"},
		{"fractional modifier", ActionAbilityCheck, `{"modifier":1.5,"dc":10}`, "cannot unmarshal number 1.5"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			step, err := New().Start(context.Background(), core.StartRequest{
				Snapshot:  testSnapshot(t),
				Principal: testPrincipal(),
				Intent: core.Intent{
					ID: "intent-strict", ActionID: test.action,
					Arguments: testRawPayload(t, test.raw),
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if step.Kind != core.StepKindReject || step.Reject == nil || !strings.Contains(step.Reject.Message, test.contains) {
				t.Fatalf("rejection = %#v, want message containing %q", step, test.contains)
			}
		})
	}
}

func TestResumeRejectsInvalidRandomResponsesAndContinuations(t *testing.T) {
	snapshot := testSnapshot(t)
	step := startAction(t, New(), snapshot, ActionDiceRoll, map[string]any{"notation": "2d6"})

	tests := []struct {
		name     string
		pending  core.PendingStep
		response core.Payload
		contains string
	}{
		{
			name:     "wrong roll count",
			pending:  mustPending(t, step),
			response: testPayload(t, DiceRandomResponse{Rolls: []int{1}}),
			contains: "received 1 rolls, want 2",
		},
		{
			name:     "out of range roll",
			pending:  mustPending(t, step),
			response: testPayload(t, DiceRandomResponse{Rolls: []int{1, 7}}),
			contains: "roll 1 is 7, want a value between 1 and 6",
		},
		{
			name:     "zero face",
			pending:  mustPending(t, step),
			response: testPayload(t, DiceRandomResponse{Rolls: []int{0, 1}}),
			contains: "roll 0 is 0, want a value between 1 and 6",
		},
		{
			name:     "missing rolls",
			pending:  mustPending(t, step),
			response: testPayload(t, map[string]any{}),
			contains: "received 0 rolls, want 2",
		},
		{
			name:     "non-integer roll",
			pending:  mustPending(t, step),
			response: testRawPayload(t, `{"rolls":[1,2.5]}`),
			contains: "cannot unmarshal number 2.5",
		},
		{
			name:     "unknown response field",
			pending:  mustPending(t, step),
			response: testPayload(t, map[string]any{"rolls": []int{1, 2}, "seed": 3}),
			contains: "unknown field",
		},
		{
			name: "cross action continuation fields",
			pending: core.PendingStep{
				StepID: step.ID,
				Kind:   core.StepKindNeedRandom,
				State: testPayload(t, map[string]any{
					"schema_version": 1,
					"action":         ActionDiceRoll,
					"notation":       "2d6",
					"label":          "tampered",
				}),
			},
			response: testPayload(t, DiceRandomResponse{Rolls: []int{1, 2}}),
			contains: `field "label" is not valid`,
		},
		{
			name: "non-normalized continuation notation",
			pending: core.PendingStep{
				StepID: step.ID,
				Kind:   core.StepKindNeedRandom,
				State: testPayload(t, map[string]any{
					"schema_version": 1,
					"action":         ActionDiceRoll,
					"notation":       " 2D6 ",
				}),
			},
			response: testPayload(t, DiceRandomResponse{Rolls: []int{1, 2}}),
			contains: "not normalized",
		},
		{
			name: "unsupported continuation version",
			pending: core.PendingStep{
				StepID: step.ID,
				Kind:   core.StepKindNeedRandom,
				State: testPayload(t, map[string]any{
					"schema_version": 2,
					"action":         ActionDiceRoll,
					"notation":       "2d6",
				}),
			},
			response: testPayload(t, DiceRandomResponse{Rolls: []int{1, 2}}),
			contains: "schema version 2 is unsupported",
		},
		{
			name: "unknown continuation action",
			pending: core.PendingStep{
				StepID: step.ID,
				Kind:   core.StepKindNeedRandom,
				State: testPayload(t, map[string]any{
					"schema_version": 1,
					"action":         "other.action",
				}),
			},
			response: testPayload(t, DiceRandomResponse{Rolls: []int{1, 2}}),
			contains: `unknown action "other.action"`,
		},
		{
			name: "unknown continuation field",
			pending: core.PendingStep{
				StepID: step.ID,
				Kind:   core.StepKindNeedRandom,
				State: testPayload(t, map[string]any{
					"schema_version": 1,
					"action":         ActionDiceRoll,
					"notation":       "2d6",
					"seed":           99,
				}),
			},
			response: testPayload(t, DiceRandomResponse{Rolls: []int{1, 2}}),
			contains: "unknown field",
		},
		{
			name: "ability continuation missing modifier",
			pending: core.PendingStep{
				StepID: step.ID,
				Kind:   core.StepKindNeedRandom,
				State: testPayload(t, map[string]any{
					"schema_version": 1,
					"action":         ActionAbilityCheck,
					"dc":             10,
				}),
			},
			response: testPayload(t, DiceRandomResponse{Rolls: []int{5}}),
			contains: `missing field "modifier"`,
		},
		{
			name: "ability continuation missing dc",
			pending: core.PendingStep{
				StepID: step.ID,
				Kind:   core.StepKindNeedRandom,
				State: testPayload(t, map[string]any{
					"schema_version": 1,
					"action":         ActionAbilityCheck,
					"modifier":       2,
				}),
			},
			response: testPayload(t, DiceRandomResponse{Rolls: []int{5}}),
			contains: `missing field "dc"`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New().Resume(context.Background(), core.ResumeRequest{
				Snapshot:  snapshot,
				Principal: testPrincipal(),
				Pending:   test.pending,
				Response: core.HostResponse{
					StepID: test.pending.StepID,
					Kind:   test.pending.Kind,
					Data:   test.response,
				},
			})
			if err == nil || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("error = %v, want substring %q", err, test.contains)
			}
		})
	}
}

func TestOversizedOrControlledParserErrorsRemainStructuredRejections(t *testing.T) {
	tests := []string{
		strings.Repeat("x", core.MaxTextBytes*2),
		"bad\x00notation",
	}
	for _, notation := range tests {
		step := startAction(t, New(), testSnapshot(t), ActionDiceRoll, map[string]any{"notation": notation})
		if step.Kind != core.StepKindReject || step.Reject == nil {
			t.Fatalf("step = %#v, want rejection", step)
		}
		if len(step.Reject.Message) > core.MaxTextBytes {
			t.Fatalf("rejection message has %d bytes", len(step.Reject.Message))
		}
		if err := step.Validate(); err != nil {
			t.Fatalf("rejection is not protocol-valid: %v", err)
		}
	}
}

func TestResumeRejectsMismatchedResponseBinding(t *testing.T) {
	snapshot := testSnapshot(t)
	step := startAction(t, New(), snapshot, ActionDiceRoll, map[string]any{"notation": "1d6"})
	pending := mustPending(t, step)
	tests := []struct {
		name, stepID string
		kind         core.StepKind
		contains     string
	}{
		{"step ID", "another-step", pending.Kind, "does not match pending step"},
		{"step kind", pending.StepID, core.StepKindNeedDecision, "does not match pending kind"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New().Resume(context.Background(), core.ResumeRequest{
				Snapshot:  snapshot,
				Principal: testPrincipal(),
				Pending:   pending,
				Response: core.HostResponse{
					StepID: test.stepID,
					Kind:   test.kind,
					Data:   testPayload(t, DiceRandomResponse{Rolls: []int{3}}),
				},
			})
			if err == nil || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("error = %v, want substring %q", err, test.contains)
			}
		})
	}
}

func TestStatelessOperations(t *testing.T) {
	ruleset := New()
	snapshot := testSnapshot(t)
	if InitialState().String() != `{}` {
		t.Fatalf("initial state = %q", InitialState().String())
	}
	if err := ruleset.ValidateState(context.Background(), core.ValidateStateRequest{Snapshot: snapshot}); err != nil {
		t.Fatal(err)
	}
	projection, err := ruleset.Project(context.Background(), core.ProjectRequest{Snapshot: snapshot, Principal: testPrincipal()})
	if err != nil {
		t.Fatal(err)
	}
	if projection.View.String() != `{}` {
		t.Fatalf("projection = %q", projection.View.String())
	}
	explanation, err := ruleset.Explain(context.Background(), core.ExplainRequest{
		Snapshot: snapshot, Principal: testPrincipal(), Reference: ActionAbilityCheck, Locale: "en",
	})
	if err != nil || !strings.Contains(explanation.Text, "Natural 20 and 1") {
		t.Fatalf("explanation = %#v, %v", explanation, err)
	}
	artifact, err := NewArtifact()
	if err != nil {
		t.Fatal(err)
	}
	migrated, err := ruleset.Migrate(context.Background(), core.MigrateRequest{From: artifact.Lock(), State: InitialState()})
	if err != nil || migrated.State.String() != `{}` {
		t.Fatalf("migration = %#v, %v", migrated, err)
	}

	invalid := snapshot
	invalid.State = testPayload(t, map[string]any{"unexpected": true})
	if err := ruleset.ValidateState(context.Background(), core.ValidateStateRequest{Snapshot: invalid}); err == nil {
		t.Fatal("state containing rules-owned data was accepted")
	}
	_, err = ruleset.Reduce(context.Background(), core.ReduceRequest{
		Snapshot: snapshot,
		Events:   []core.Event{{Type: "unsupported.event", SchemaVersion: 1, Data: InitialState()}},
	})
	if err == nil {
		t.Fatal("unsupported event was accepted")
	}

	wrongArtifact := snapshot
	wrongArtifact.Ruleset.Digest = "sha256:" + strings.Repeat("0", 64)
	if err := ruleset.ValidateState(context.Background(), core.ValidateStateRequest{Snapshot: wrongArtifact}); err == nil {
		t.Fatal("snapshot locked to another artifact was accepted")
	}
}

func testSnapshot(t *testing.T) core.Snapshot {
	t.Helper()
	artifact, err := NewArtifact()
	if err != nil {
		t.Fatal(err)
	}
	return core.Snapshot{Ruleset: artifact.Lock(), State: InitialState()}
}

func testPrincipal() core.Principal {
	return core.Principal{ID: "host", Kind: "system"}
}

func startAction(t *testing.T, ruleset *Ruleset, snapshot core.Snapshot, action string, arguments any) core.Step {
	t.Helper()
	step, err := ruleset.Start(context.Background(), core.StartRequest{
		Snapshot:  snapshot,
		Principal: testPrincipal(),
		Intent: core.Intent{
			ID:        "intent-1",
			ActionID:  action,
			Arguments: testPayload(t, arguments),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := step.Validate(); err != nil {
		t.Fatalf("invalid step: %v", err)
	}
	return step
}

func resumeAction(t *testing.T, ruleset *Ruleset, snapshot core.Snapshot, step core.Step, response any) core.Step {
	t.Helper()
	pending := mustPending(t, step)
	complete, err := ruleset.Resume(context.Background(), core.ResumeRequest{
		Snapshot:  snapshot,
		Principal: testPrincipal(),
		Pending:   pending,
		Response: core.HostResponse{
			StepID: pending.StepID,
			Kind:   pending.Kind,
			Data:   testPayload(t, response),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := complete.Validate(); err != nil {
		t.Fatalf("invalid completion: %v", err)
	}
	return complete
}

func mustPending(t *testing.T, step core.Step) core.PendingStep {
	t.Helper()
	pending, err := step.Pending()
	if err != nil {
		t.Fatal(err)
	}
	return pending
}

func assertRandomRequest(t *testing.T, step core.Step, want DiceRandomRequest) {
	t.Helper()
	if step.Kind != core.StepKindNeedRandom || step.NeedRandom == nil {
		t.Fatalf("step kind = %q, want need_random", step.Kind)
	}
	if step.NeedRandom.Method != RandomMethodDiceRoll {
		t.Fatalf("random method = %q", step.NeedRandom.Method)
	}
	var got DiceRandomRequest
	decodeTestPayload(t, step.NeedRandom.Specification, &got)
	if got != want {
		t.Fatalf("random specification = %#v, want %#v", got, want)
	}
}

func testPayload(t *testing.T, value any) core.Payload {
	t.Helper()
	payload, err := core.PayloadFrom(value)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func testRawPayload(t *testing.T, raw string) core.Payload {
	t.Helper()
	payload, err := core.NewPayload([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func decodeTestPayload(t *testing.T, payload core.Payload, dst any) {
	t.Helper()
	if err := json.Unmarshal(payload.Bytes(), dst); err != nil {
		t.Fatal(err)
	}
}

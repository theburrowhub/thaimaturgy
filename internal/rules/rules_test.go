package rules

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

const (
	bundleA = "abc"
	bundleB = ""
	digestA = "sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	digestB = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

func testManifest() Manifest {
	return Manifest{
		ID:              "example.rules",
		Name:            "Example Rules",
		Version:         "1.2.3",
		ProtocolVersion: ProtocolVersion,
		Runtime:         Runtime{Kind: RuntimeBuiltin},
		Capabilities:    []string{"random", "nested.resolution"},
	}
}

func testArtifact(t *testing.T, bundle string) Artifact {
	t.Helper()
	artifact, err := NewArtifact(testManifest(), strings.NewReader(bundle))
	if err != nil {
		t.Fatalf("NewArtifact: %v", err)
	}
	return artifact
}

func testLock(digest string) Lock {
	manifest := testManifest()
	return Lock{
		ID:              manifest.ID,
		Version:         manifest.Version,
		Digest:          digest,
		ProtocolVersion: manifest.ProtocolVersion,
	}
}

func mustPayload(t *testing.T, raw string) Payload {
	t.Helper()
	payload, err := NewPayload([]byte(raw))
	if err != nil {
		t.Fatalf("NewPayload(%q): %v", raw, err)
	}
	return payload
}

func testPrincipal() Principal {
	return Principal{ID: "user:42", Kind: "human", Roles: []string{"participant"}}
}

func testSnapshot(t *testing.T) Snapshot {
	t.Helper()
	return Snapshot{
		Ruleset:  testLock(digestA),
		Revision: 7,
		State:    mustPayload(t, `{"scene":"crossroads"}`),
	}
}

func testIntent(t *testing.T) Intent {
	t.Helper()
	return Intent{
		ID:        "request:1",
		ActionID:  "attempt",
		ActorID:   "actor:1",
		Arguments: mustPayload(t, `{"approach":"careful"}`),
	}
}

func TestManifestValidate(t *testing.T) {
	valid := testManifest()
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid manifest: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Manifest)
	}{
		{"unsafe id", func(m *Manifest) { m.ID = "../rules" }},
		{"noncanonical version", func(m *Manifest) { m.Version = "1.02.3" }},
		{"duplicate capability", func(m *Manifest) { m.Capabilities = []string{"random", "random"} }},
		{"builtin entrypoint", func(m *Manifest) { m.Runtime.Entrypoint = "main.star" }},
		{"traversing entrypoint", func(m *Manifest) {
			m.Runtime = Runtime{Kind: RuntimeStarlark, Entrypoint: "../main.star"}
		}},
		{"windows drive entrypoint", func(m *Manifest) {
			m.Runtime = Runtime{Kind: RuntimeStarlark, Entrypoint: "C:/rules/main.star"}
		}},
		{"backslash entrypoint", func(m *Manifest) {
			m.Runtime = Runtime{Kind: RuntimeStarlark, Entrypoint: `rules\main.star`}
		}},
		{"nul entrypoint", func(m *Manifest) {
			m.Runtime = Runtime{Kind: RuntimeStarlark, Entrypoint: "rules/\x00main.star"}
		}},
		{"control entrypoint", func(m *Manifest) {
			m.Runtime = Runtime{Kind: RuntimeStarlark, Entrypoint: "rules/\x1fmain.star"}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := valid
			test.mutate(&manifest)
			if err := manifest.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}

	external := valid
	external.Runtime = Runtime{Kind: RuntimeKind("lua"), Entrypoint: "rules/main.lua"}
	if err := external.Validate(); err != nil {
		t.Fatalf("extensible runtime kind: %v", err)
	}
}

func TestArtifactAndRequirementHaveSeparateIdentity(t *testing.T) {
	artifact := testArtifact(t, bundleA)
	if err := artifact.Validate(); err != nil {
		t.Fatalf("valid artifact: %v", err)
	}
	if artifact.Digest() != digestA {
		t.Fatalf("digest = %q, want known SHA-256 %q", artifact.Digest(), digestA)
	}
	if _, err := NewArtifact(testManifest(), nil); err == nil {
		t.Fatal("nil artifact reader was accepted")
	}
	lock := artifact.Lock()
	if lock.ProtocolVersion != ProtocolVersion || lock.Digest != digestA {
		t.Fatalf("incomplete lock: %+v", lock)
	}

	requirement := testManifest().Requirement()
	if err := requirement.Validate(); err != nil {
		t.Fatal(err)
	}
	if requirement.Version != VersionConstraint("1.2.3") {
		t.Fatalf("Version = %q", requirement.Version)
	}
	rangeRequirement := Requirement{ID: "example.rules", Version: ">=1.2.0 <2.0.0"}
	if err := rangeRequirement.Validate(); err != nil {
		t.Fatalf("range requirement: %v", err)
	}
}

func TestPayloadIsValidatedAndImmutable(t *testing.T) {
	raw := []byte(`{"value":1}`)
	payload, err := NewPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	raw[2] = 'X'
	got := payload.Bytes()
	got[2] = 'Y'
	if payload.String() != `{"value":1}` {
		t.Fatalf("payload was mutated: %s", payload.String())
	}
	if _, err := NewPayload([]byte(`{"broken"`)); err == nil {
		t.Fatal("invalid JSON was accepted")
	}
	if _, err := NewPayload([]byte(`{"value":1,"value":2}`)); err == nil {
		t.Fatal("duplicate JSON object member was accepted")
	}
	invalidUTF8 := append([]byte(`{"value":"`), 0xff)
	invalidUTF8 = append(invalidUTF8, []byte(`"}`)...)
	if _, err := NewPayload(invalidUTF8); err == nil {
		t.Fatal("invalid UTF-8 was accepted")
	}
	if _, err := NewPayload(make([]byte, MaxPayloadBytes+1)); err == nil {
		t.Fatal("oversized JSON was accepted")
	}
	if _, err := NewPayload([]byte("null")); err == nil {
		t.Fatal("JSON null was accepted as an explicit payload")
	}
	if _, err := PayloadFrom(nil); err == nil {
		t.Fatal("nil was accepted as an explicit payload")
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Payload
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.String() != payload.String() {
		t.Fatalf("round trip = %q, want %q", decoded.String(), payload.String())
	}
}

func TestPayloadCanonicalizesFormattingMemberOrderAndEscapes(t *testing.T) {
	formatted, err := NewPayload([]byte("{\n  \"z\": \"<tag>\",\n  \"a\": [1, 2]\n}"))
	if err != nil {
		t.Fatal(err)
	}
	fromValue, err := PayloadFrom(map[string]any{
		"a": []int{1, 2},
		"z": "<tag>",
	})
	if err != nil {
		t.Fatal(err)
	}
	if formatted.String() != fromValue.String() {
		t.Fatalf("canonical payloads differ: formatted=%q value=%q", formatted.String(), fromValue.String())
	}

	pretty, err := json.MarshalIndent(struct {
		Value Payload `json:"value"`
	}{Value: formatted}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	var restored struct {
		Value Payload `json:"value"`
	}
	if err := json.Unmarshal(pretty, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Value.String() != formatted.String() {
		t.Fatalf("pretty round trip changed payload: got=%q want=%q", restored.Value.String(), formatted.String())
	}
}

func TestOptionalPayloadNullRoundTripRestoresZero(t *testing.T) {
	descriptor := ActionDescriptor{
		ID:          "attempt",
		Label:       "Attempt something",
		InputSchema: mustPayload(t, `{"type":"object"}`),
	}
	raw, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ActionDescriptor
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.Annotations.IsZero() {
		t.Fatalf("optional null decoded as present: %s", decoded.Annotations.String())
	}
	if err := decoded.Validate(); err != nil {
		t.Fatalf("round-tripped descriptor: %v", err)
	}

	var payload Payload
	if err := json.Unmarshal([]byte("null"), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.IsZero() {
		t.Fatal("JSON null did not restore the Payload zero value")
	}
}

func TestProtocolValidation(t *testing.T) {
	descriptor := ActionDescriptor{
		ID:          "attempt",
		Label:       "Attempt something",
		InputSchema: mustPayload(t, `{"type":"object"}`),
		Annotations: mustPayload(t, `{"group":"general"}`),
	}
	if err := descriptor.Validate(); err != nil {
		t.Fatalf("descriptor: %v", err)
	}
	if err := ValidateActions([]ActionDescriptor{descriptor, descriptor}); err == nil {
		t.Fatal("duplicate action IDs were accepted")
	}

	request := StartRequest{
		Snapshot:  testSnapshot(t),
		Principal: testPrincipal(),
		Intent:    testIntent(t),
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("start request: %v", err)
	}

	descriptor.InputSchema = mustPayload(t, `[]`)
	if err := descriptor.Validate(); err == nil {
		t.Fatal("non-object input schema was accepted")
	}
}

func TestExplainIsAuthorityScopedAndEventsAreVersioned(t *testing.T) {
	request := ExplainRequest{
		Snapshot:  testSnapshot(t),
		Principal: testPrincipal(),
		Reference: "journey.outcomes",
		Locale:    "es-ES",
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("explain request: %v", err)
	}
	request.Principal = Principal{}
	if err := request.Validate(); err == nil {
		t.Fatal("explain request without authority was accepted")
	}

	event := Event{Type: "state.changed", Data: mustPayload(t, `{}`)}
	if err := event.Validate(); err == nil {
		t.Fatal("event without schema version was accepted")
	}
	event.SchemaVersion = 1
	if err := event.Validate(); err != nil {
		t.Fatalf("versioned event: %v", err)
	}
}

func TestEveryStepKindValidates(t *testing.T) {
	result := mustPayload(t, `{"ok":true}`)
	continuation := mustPayload(t, `{"phase":"waiting"}`)
	intent := testIntent(t)
	steps := []Step{
		{ID: "step:reject", Kind: StepKindReject, Reject: &Rejection{Code: "not.allowed", Message: "Not allowed"}},
		{ID: "step:random", Kind: StepKindNeedRandom, Continuation: continuation, NeedRandom: &RandomRequest{Method: "table.draw", Specification: mustPayload(t, `{"table":"weather"}`)}},
		{ID: "step:decision", Kind: StepKindNeedDecision, Continuation: continuation, NeedDecision: &DecisionRequest{
			Authority: "user:42", Prompt: "Choose", Options: []DecisionOption{{ID: "left", Label: "Left"}},
		}},
		{ID: "step:adjudicate", Kind: StepKindNeedAdjudication, Continuation: continuation, NeedAdjudication: &AdjudicationRequest{Authority: "referee:1", Prompt: "What follows?"}},
		{ID: "step:child", Kind: StepKindStartChild, Continuation: continuation, StartChild: &ChildRequest{Ruleset: testLock(digestA), Intent: intent}},
		{ID: "step:emit", Kind: StepKindEmit, Continuation: continuation, Emit: &Emission{Events: []Event{{Type: "state.changed", SchemaVersion: 1, Data: result}}}},
		{ID: "step:complete", Kind: StepKindComplete, Complete: &Completion{Outcome: "succeeded", Result: result}},
	}
	for _, step := range steps {
		t.Run(string(step.Kind), func(t *testing.T) {
			if err := step.Validate(); err != nil {
				t.Fatalf("valid step: %v", err)
			}
			wantTerminal := step.Kind == StepKindReject || step.Kind == StepKindComplete
			if step.Terminal() != wantTerminal || step.NeedsResponse() == wantTerminal {
				t.Fatalf("terminal=%v needsResponse=%v", step.Terminal(), step.NeedsResponse())
			}
		})
	}
}

func TestStepRejectsMalformedUnion(t *testing.T) {
	result := mustPayload(t, `{}`)
	step := Step{
		ID:       "step:1",
		Kind:     StepKindComplete,
		Reject:   &Rejection{Code: "no", Message: "No"},
		Complete: &Completion{Outcome: "yes", Result: result},
	}
	if err := step.Validate(); err == nil {
		t.Fatal("multiple variants were accepted")
	}

	step = Step{ID: "step:1", Kind: StepKindNeedRandom, Complete: &Completion{Outcome: "yes", Result: result}}
	if err := step.Validate(); err == nil {
		t.Fatal("mismatched kind was accepted")
	}

	response := HostResponse{StepID: "step:1", Kind: StepKindComplete, Data: result}
	if err := response.Validate(); err == nil {
		t.Fatal("continuation of terminal step was accepted")
	}
	response.Kind = StepKindNeedRandom
	if err := response.Validate(); err != nil {
		t.Fatalf("valid host response: %v", err)
	}

	missingContinuation := Step{
		ID:         "step:random",
		Kind:       StepKindNeedRandom,
		NeedRandom: &RandomRequest{Method: "draw", Specification: result},
	}
	if err := missingContinuation.Validate(); err == nil {
		t.Fatal("resumable step without persisted continuation was accepted")
	}
}

func TestPendingStepSurvivesRoundTripAndRestart(t *testing.T) {
	pending := Step{
		ID:           "step:random",
		Kind:         StepKindNeedRandom,
		Continuation: mustPayload(t, `{"action":"journey","phase":2}`),
		NeedRandom: &RandomRequest{
			Method:        "draw",
			Specification: mustPayload(t, `{"source":"weather"}`),
		},
	}
	raw, err := json.Marshal(pending)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate loading the pending resolution in a fresh process with no live
	// ruleset execution state.
	var restored Step
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if err := restored.Validate(); err != nil {
		t.Fatalf("restored step: %v", err)
	}
	persisted, err := restored.Pending()
	if err != nil {
		t.Fatal(err)
	}
	request := ResumeRequest{
		Snapshot:  testSnapshot(t),
		Principal: testPrincipal(),
		Pending:   persisted,
		Response: HostResponse{
			StepID: restored.ID,
			Kind:   restored.Kind,
			Data:   mustPayload(t, `{"value":"storm"}`),
		},
	}
	requestRaw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var restarted ResumeRequest
	if err := json.Unmarshal(requestRaw, &restarted); err != nil {
		t.Fatal(err)
	}
	if err := restarted.Validate(); err != nil {
		t.Fatalf("restart resume request: %v", err)
	}
	if restarted.Pending.State.String() != `{"action":"journey","phase":2}` {
		t.Fatalf("continuation = %s", restarted.Pending.State.String())
	}
	if restarted.Response.Data.String() != `{"value":"storm"}` {
		t.Fatalf("host response = %s", restarted.Response.Data.String())
	}
	restarted.Response.StepID = "step:other"
	if err := restarted.Validate(); err == nil {
		t.Fatal("response for another pending step was accepted")
	}
	restarted.Response.StepID = restarted.Pending.StepID
	restarted.Response.Kind = StepKindNeedDecision
	if err := restarted.Validate(); err == nil {
		t.Fatal("response of another pending kind was accepted")
	}
}

type stubRuleset struct {
	manifest Manifest
}

func (s *stubRuleset) Manifest(context.Context) (Manifest, error) { return s.manifest, nil }

func (s *stubRuleset) ListActions(context.Context, CatalogRequest) ([]ActionDescriptor, error) {
	return nil, nil
}

func (s *stubRuleset) Start(context.Context, StartRequest) (Step, error) { return Step{}, nil }

func (s *stubRuleset) Resume(context.Context, ResumeRequest) (Step, error) { return Step{}, nil }

func (s *stubRuleset) Project(context.Context, ProjectRequest) (Projection, error) {
	return Projection{}, nil
}

func (s *stubRuleset) Explain(context.Context, ExplainRequest) (Explanation, error) {
	return Explanation{}, nil
}

func (s *stubRuleset) ValidateState(context.Context, ValidateStateRequest) error { return nil }

func (s *stubRuleset) Reduce(context.Context, ReduceRequest) (ReduceResult, error) {
	return ReduceResult{}, nil
}

func (s *stubRuleset) Migrate(context.Context, MigrateRequest) (MigrateResult, error) {
	return MigrateResult{}, nil
}

var _ Ruleset = (*stubRuleset)(nil)

func TestRegistryRejectsReleaseArtifactConflict(t *testing.T) {
	registry := NewRegistry()
	first := &stubRuleset{manifest: testManifest()}
	second := &stubRuleset{manifest: testManifest()}
	artifactA := testArtifact(t, bundleA)
	artifactB := testArtifact(t, bundleB)
	if err := registry.Register(context.Background(), artifactA, first); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(context.Background(), artifactB, second); !errors.Is(err, ErrArtifactConflict) {
		t.Fatalf("artifact conflict error = %v", err)
	}
	if registry.Len() != 1 {
		t.Fatalf("Len = %d, want 1", registry.Len())
	}
	got, err := registry.Lookup(testLock(digestA))
	if err != nil {
		t.Fatal(err)
	}
	if got != first {
		t.Fatal("lookup returned the wrong digest variant")
	}
	if err := registry.Register(context.Background(), artifactA, first); !errors.Is(err, ErrRulesetAlreadyRegistered) {
		t.Fatalf("duplicate error = %v", err)
	}

	missing := testLock(digestA)
	missing.Version = "1.2.4"
	if _, err := registry.Lookup(missing); !errors.Is(err, ErrRulesetNotFound) {
		t.Fatalf("missing error = %v", err)
	}
}

func TestRegistryRejectsIncompatibleProtocolAndCancellation(t *testing.T) {
	registry := NewRegistry()
	incompatible := &stubRuleset{manifest: testManifest()}
	incompatible.manifest.ProtocolVersion = "2.0.0"
	incompatibleArtifact, err := NewArtifact(incompatible.manifest, strings.NewReader(bundleA))
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(context.Background(), incompatibleArtifact, incompatible); !errors.Is(err, ErrIncompatibleProtocol) {
		t.Fatalf("incompatible protocol error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := registry.Register(ctx, testArtifact(t, bundleA), &stubRuleset{manifest: testManifest()}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled registration error = %v", err)
	}
}

func TestRegistryRejectsManifestMismatch(t *testing.T) {
	registry := NewRegistry()
	artifact := testArtifact(t, bundleA)
	ruleset := &stubRuleset{manifest: testManifest()}
	ruleset.manifest.Name = "Substituted Rules"
	if err := registry.Register(context.Background(), artifact, ruleset); !errors.Is(err, ErrManifestMismatch) {
		t.Fatalf("manifest mismatch error = %v", err)
	}
}

func TestRegistryDefensivelyClonesManifest(t *testing.T) {
	registry := NewRegistry()
	ruleset := &stubRuleset{manifest: testManifest()}
	artifactRecord := testArtifact(t, bundleA)
	if err := registry.Register(context.Background(), artifactRecord, ruleset); err != nil {
		t.Fatal(err)
	}
	lock := testLock(digestA)

	// Mutating the provider's original backing array must not change the entry.
	ruleset.manifest.Capabilities[0] = "tampered"
	manifest, err := registry.Manifest(lock)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Capabilities[0] != "random" {
		t.Fatalf("stored capability = %q", manifest.Capabilities[0])
	}

	// Mutating returned data must not affect subsequent readers either.
	manifest.Capabilities[0] = "consumer.tampered"
	again, err := registry.Manifest(lock)
	if err != nil {
		t.Fatal(err)
	}
	if again.Capabilities[0] != "random" {
		t.Fatalf("returned capability alias = %q", again.Capabilities[0])
	}
	artifact, err := registry.Artifact(lock)
	if err != nil {
		t.Fatal(err)
	}
	artifactManifest := artifact.Manifest()
	artifactManifest.Capabilities[0] = "artifact.tampered"
	last, err := registry.Artifact(lock)
	if err != nil {
		t.Fatal(err)
	}
	if last.Manifest().Capabilities[0] != "random" {
		t.Fatalf("artifact capability alias = %q", last.Manifest().Capabilities[0])
	}
}

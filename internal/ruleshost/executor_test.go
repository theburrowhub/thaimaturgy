package ruleshost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/rules"
)

const executorDigest = "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"

type executorRuleset struct {
	reduceCalls   int
	validateCalls int
}

type panicReduceRuleset struct{ *executorRuleset }

func (r panicReduceRuleset) Reduce(context.Context, rules.ReduceRequest) (rules.ReduceResult, error) {
	panic("broken reducer")
}

func executorPayload(t *testing.T, raw string) rules.Payload {
	t.Helper()
	payload, err := rules.NewPayload([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func executorLock() rules.Lock {
	return rules.Lock{ID: "test.rules", Version: "1.0.0", Digest: executorDigest, ProtocolVersion: rules.ProtocolVersion}
}

func (r *executorRuleset) Manifest(context.Context) (rules.Manifest, error) {
	return rules.Manifest{}, errors.New("not used")
}

func (r *executorRuleset) ListActions(context.Context, rules.CatalogRequest) ([]rules.ActionDescriptor, error) {
	return nil, errors.New("not used")
}

func (r *executorRuleset) Start(_ context.Context, request rules.StartRequest) (rules.Step, error) {
	continuation, _ := rules.PayloadFrom(map[string]any{"phase": "random"})
	specification, _ := rules.PayloadFrom(map[string]any{"count": 1})
	return rules.Step{
		ID: request.Intent.ID + ":random", Kind: rules.StepKindNeedRandom, Continuation: continuation,
		NeedRandom: &rules.RandomRequest{Method: "coin.flip", Specification: specification},
	}, nil
}

func (r *executorRuleset) Resume(_ context.Context, request rules.ResumeRequest) (rules.Step, error) {
	switch request.Pending.Kind {
	case rules.StepKindNeedRandom:
		var response struct {
			Face string `json:"face"`
		}
		if err := json.Unmarshal(request.Response.Data.Bytes(), &response); err != nil || response.Face != "heads" {
			return rules.Step{}, fmt.Errorf("unexpected random response: %+v %v", response, err)
		}
		continuation, _ := rules.PayloadFrom(map[string]any{"phase": "emit"})
		data, _ := rules.PayloadFrom(map[string]any{"amount": 1})
		return rules.Step{
			ID: request.Pending.StepID + ":emit", Kind: rules.StepKindEmit, Continuation: continuation,
			Emit: &rules.Emission{Events: []rules.Event{{Type: "counter.incremented", SchemaVersion: 1, Data: data}}},
		}, nil
	case rules.StepKindEmit:
		if request.Snapshot.Revision != 1 || request.Snapshot.State.String() != "{\"value\":1}" {
			return rules.Step{}, fmt.Errorf("resume saw wrong snapshot: %+v", request.Snapshot)
		}
		result, _ := rules.PayloadFrom(map[string]any{"value": 1})
		return rules.Step{
			ID: request.Pending.StepID + ":complete", Kind: rules.StepKindComplete,
			Complete: &rules.Completion{Outcome: "test.completed", Result: result},
		}, nil
	default:
		return rules.Step{}, fmt.Errorf("unexpected pending kind %q", request.Pending.Kind)
	}
}

func (r *executorRuleset) Project(context.Context, rules.ProjectRequest) (rules.Projection, error) {
	return rules.Projection{}, errors.New("not used")
}

func (r *executorRuleset) Explain(context.Context, rules.ExplainRequest) (rules.Explanation, error) {
	return rules.Explanation{}, errors.New("not used")
}

func (r *executorRuleset) ValidateState(_ context.Context, request rules.ValidateStateRequest) error {
	r.validateCalls++
	var state struct {
		Value int `json:"value"`
	}
	if err := json.Unmarshal(request.Snapshot.State.Bytes(), &state); err != nil {
		return err
	}
	if state.Value < 0 {
		return errors.New("negative state")
	}
	return nil
}

func (r *executorRuleset) Reduce(_ context.Context, request rules.ReduceRequest) (rules.ReduceResult, error) {
	r.reduceCalls++
	var state struct {
		Value int `json:"value"`
	}
	if err := json.Unmarshal(request.Snapshot.State.Bytes(), &state); err != nil {
		return rules.ReduceResult{}, err
	}
	for _, event := range request.Events {
		if event.Type != "counter.incremented" {
			return rules.ReduceResult{}, errors.New("unknown event")
		}
		var data struct {
			Amount int `json:"amount"`
		}
		if err := json.Unmarshal(event.Data.Bytes(), &data); err != nil {
			return rules.ReduceResult{}, err
		}
		state.Value += data.Amount
	}
	payload, err := rules.PayloadFrom(state)
	return rules.ReduceResult{State: payload}, err
}

func (r *executorRuleset) Migrate(context.Context, rules.MigrateRequest) (rules.MigrateResult, error) {
	return rules.MigrateResult{}, errors.New("not used")
}

func TestExecutorDrivesGenericRandomEmitReduceAndCompletion(t *testing.T) {
	implementation := &executorRuleset{}
	dispatcher := NewRandomDispatcher()
	draws := 0
	if err := dispatcher.Register("coin.flip", func(_ context.Context, specification rules.Payload) (rules.Payload, error) {
		draws++
		if specification.String() != "{\"count\":1}" {
			t.Fatalf("specification = %s", specification.String())
		}
		return executorPayload(t, "{\"face\":\"heads\"}"), nil
	}); err != nil {
		t.Fatal(err)
	}
	executor := Executor{Ruleset: implementation, Random: dispatcher}
	snapshot := rules.Snapshot{Ruleset: executorLock(), State: executorPayload(t, "{\"value\":0}")}
	principal := rules.Principal{ID: "host:oracle", Kind: "llm"}
	step, err := executor.Start(context.Background(), rules.StartRequest{
		Snapshot:  snapshot,
		Principal: principal,
		Intent:    rules.Intent{ID: "resolution-1", ActionID: "test.run", Arguments: executorPayload(t, "{}")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if step.Kind != rules.StepKindNeedRandom {
		t.Fatalf("start step = %+v", step)
	}
	randomResult, err := executor.ResolveRandom(context.Background(), *step.NeedRandom)
	if err != nil {
		t.Fatal(err)
	}
	pending, _ := step.Pending()
	step, err = executor.Resume(context.Background(), snapshot, principal, pending, randomResult)
	if err != nil || step.Kind != rules.StepKindEmit {
		t.Fatalf("resume random step=%+v err=%v", step, err)
	}
	pending, _ = step.Pending()
	snapshot, acknowledgement, err := executor.Reduce(context.Background(), snapshot, *step.Emit)
	if err != nil {
		t.Fatal(err)
	}
	step, err = executor.Resume(context.Background(), snapshot, principal, pending, acknowledgement)
	if err != nil || step.Kind != rules.StepKindComplete || snapshot.Revision != 1 || snapshot.State.String() != "{\"value\":1}" {
		t.Fatalf("completion step=%+v snapshot=%+v err=%v", step, snapshot, err)
	}
	if draws != 1 || implementation.reduceCalls != 1 || implementation.validateCalls != 1 {
		t.Fatalf("draws=%d reduce=%d validate=%d", draws, implementation.reduceCalls, implementation.validateCalls)
	}
}

func TestExecutorReturnsNoPartialResultWhenRandomFails(t *testing.T) {
	implementation := &executorRuleset{}
	dispatcher := NewRandomDispatcher()
	if err := dispatcher.Register("coin.flip", func(context.Context, rules.Payload) (rules.Payload, error) {
		return rules.Payload{}, errors.New("entropy unavailable")
	}); err != nil {
		t.Fatal(err)
	}
	executor := Executor{Ruleset: implementation, Random: dispatcher}
	step, err := executor.Start(context.Background(), rules.StartRequest{
		Snapshot:  rules.Snapshot{Ruleset: executorLock(), State: executorPayload(t, "{\"value\":0}")},
		Principal: rules.Principal{ID: "host:oracle", Kind: "llm"},
		Intent:    rules.Intent{ID: "resolution-error", ActionID: "test.run", Arguments: executorPayload(t, "{}")},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = executor.ResolveRandom(context.Background(), *step.NeedRandom)
	if err == nil || !strings.Contains(err.Error(), "entropy unavailable") {
		t.Fatalf("random error=%v", err)
	}
}

func TestExecutorContainsRulesetPanicsAndReturnsNoPartialResult(t *testing.T) {
	dispatcher := NewRandomDispatcher()
	if err := dispatcher.Register("coin.flip", func(context.Context, rules.Payload) (rules.Payload, error) {
		return executorPayload(t, "{\"face\":\"heads\"}"), nil
	}); err != nil {
		t.Fatal(err)
	}
	implementation := panicReduceRuleset{executorRuleset: &executorRuleset{}}
	executor := Executor{Ruleset: implementation, Random: dispatcher}
	snapshot := rules.Snapshot{Ruleset: executorLock(), State: executorPayload(t, "{\"value\":0}")}
	principal := rules.Principal{ID: "host:oracle", Kind: "llm"}
	step, err := executor.Start(context.Background(), rules.StartRequest{
		Snapshot: snapshot, Principal: principal,
		Intent: rules.Intent{ID: "resolution-panic", ActionID: "test.run", Arguments: executorPayload(t, "{}")},
	})
	if err != nil {
		t.Fatal(err)
	}
	randomResult, err := executor.ResolveRandom(context.Background(), *step.NeedRandom)
	if err != nil {
		t.Fatal(err)
	}
	pending, _ := step.Pending()
	step, err = executor.Resume(context.Background(), snapshot, principal, pending, randomResult)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = executor.Reduce(context.Background(), snapshot, *step.Emit)
	if err == nil || !strings.Contains(err.Error(), "panicked") {
		t.Fatalf("reduce error=%v", err)
	}
}

func TestRandomDispatcherRejectsDuplicateAndUnknownMethods(t *testing.T) {
	dispatcher := NewRandomDispatcher()
	resolver := func(context.Context, rules.Payload) (rules.Payload, error) {
		return executorPayload(t, "{}"), nil
	}
	if err := dispatcher.Register("coin.flip", resolver); err != nil {
		t.Fatal(err)
	}
	if err := dispatcher.Register("coin.flip", resolver); err == nil {
		t.Fatal("duplicate registration succeeded")
	}
	_, err := dispatcher.ResolveRandom(context.Background(), rules.RandomRequest{
		Method: "cards.draw", Specification: executorPayload(t, "{}"),
	})
	if err == nil {
		t.Fatal("unknown method unexpectedly resolved")
	}
}

func TestReplayReconstructsSnapshotAndRejectsTampering(t *testing.T) {
	data := executorPayload(t, "{\"amount\":1}")
	batches := []ReplayBatch{{
		Sequence: 1, BaseRevision: 0, Revision: 1,
		Events: []rules.Event{{Type: "counter.incremented", SchemaVersion: 1, Data: data}},
	}}
	implementation := &executorRuleset{}
	replayed, err := Replay(context.Background(), implementation, executorLock(), executorPayload(t, "{\"value\":0}"), batches)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Revision != 1 || replayed.State.String() != "{\"value\":1}" {
		t.Fatalf("replayed = %+v state=%s", replayed, replayed.State.String())
	}

	for _, test := range []struct {
		name   string
		mutate func([]ReplayBatch)
	}{
		{name: "revision gap", mutate: func(copy []ReplayBatch) { copy[0].BaseRevision = 2 }},
		{name: "sequence", mutate: func(copy []ReplayBatch) { copy[0].Sequence = 2 }},
		{name: "revision jump", mutate: func(copy []ReplayBatch) { copy[0].Revision = 2 }},
		{name: "empty events", mutate: func(copy []ReplayBatch) { copy[0].Events = nil }},
		{name: "event tamper", mutate: func(copy []ReplayBatch) { copy[0].Events[0].Type = "counter.replaced" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			copy := []ReplayBatch{{
				Sequence: batches[0].Sequence, BaseRevision: batches[0].BaseRevision, Revision: batches[0].Revision,
				Events: append([]rules.Event(nil), batches[0].Events...),
			}}
			test.mutate(copy)
			if _, err := Replay(context.Background(), &executorRuleset{}, executorLock(), executorPayload(t, "{\"value\":0}"), copy); err == nil {
				t.Fatal("tampered replay unexpectedly succeeded")
			}
		})
	}
}

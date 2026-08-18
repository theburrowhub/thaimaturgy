package domain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/rules"
)

const runtimeFingerprint = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"

func beginRuntimeRequest(t *testing.T, state *SessionState, id string) RulesRequestHandle {
	t.Helper()
	handle, receipt, err := state.BeginRulesRequest(context.Background(), id, "game_submit_intent", runtimeFingerprint)
	if err != nil || receipt != nil {
		t.Fatalf("BeginRulesRequest(%q): receipt=%+v err=%v", id, receipt, err)
	}
	return handle
}

func runtimeResult(content string) *RulesStoredResult {
	return &RulesStoredResult{Content: content}
}

func TestRulesRuntimeReceiptSurvivesRoundTripAndRejectsFingerprintConflict(t *testing.T) {
	state := NewSessionState("runtime", nil)
	if _, err := state.BindRules(rulesTestLock(rulesDigestA), rulesTestPayload(t, `{"value":0}`)); err != nil {
		t.Fatal(err)
	}
	handle := beginRuntimeRequest(t, state, "request-1")
	first := runtimeResult(`{"status":"resolved"}`)
	if _, err := state.CommitRulesRequest(handle, RulesCommit{State: handle.Snapshot.State, ResolutionID: "resolution-1", Result: first}); err != nil {
		t.Fatal(err)
	}

	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	var restored SessionState
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	_, receipt, err := restored.BeginRulesRequest(context.Background(), "request-1", "game_submit_intent", runtimeFingerprint)
	if err != nil || receipt == nil || receipt.Result == nil || *receipt.Result != *first {
		t.Fatalf("restored receipt=%+v err=%v", receipt, err)
	}
	_, _, err = restored.BeginRulesRequest(context.Background(), "request-1", "game_submit_intent",
		"sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd")
	if !errors.Is(err, ErrRulesReceiptConflict) {
		t.Fatalf("fingerprint conflict = %v", err)
	}
}

func TestRulesRuntimeReceiptLimitFailsClosedWithoutForgettingOldIDs(t *testing.T) {
	state := NewSessionState("runtime", nil)
	if _, err := state.BindRules(rulesTestLock(rulesDigestA), rulesTestPayload(t, `{}`)); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for i := 0; i < MaxRulesReceipts; i++ {
		state.Rules.Receipts = append(state.Rules.Receipts, RulesReceipt{
			RequestID: fmt.Sprintf("request-%d", i), Tool: "game_submit_intent",
			Fingerprint: runtimeFingerprint, ResolutionID: fmt.Sprintf("resolution-%d", i),
			Result: runtimeResult(fmt.Sprintf("result-%d", i)), CreatedAt: now, UpdatedAt: now,
		})
	}
	if _, _, err := state.BeginRulesRequest(context.Background(), "request-new", "game_submit_intent", runtimeFingerprint); !errors.Is(err, ErrRulesReceiptLimit) {
		t.Fatalf("new request after durable limit = %v", err)
	}
	runtime, ok := state.RulesRuntimeSnapshot()
	if !ok || len(runtime.Receipts) != MaxRulesReceipts {
		t.Fatalf("runtime ok=%v receipts=%d", ok, len(runtime.Receipts))
	}
	if runtime.Receipts[0].RequestID != "request-0" || runtime.Receipts[len(runtime.Receipts)-1].RequestID != fmt.Sprintf("request-%d", MaxRulesReceipts-1) {
		t.Fatalf("bounded receipt set = first %q last %q", runtime.Receipts[0].RequestID, runtime.Receipts[len(runtime.Receipts)-1].RequestID)
	}
	_, receipt, err := state.BeginRulesRequest(context.Background(), "request-0", "game_submit_intent", runtimeFingerprint)
	if err != nil || receipt == nil || receipt.Result == nil || receipt.Result.Content != "result-0" {
		t.Fatalf("retained retry receipt=%+v err=%v", receipt, err)
	}
}

func TestRulesRuntimeNeverEvictsReceiptForActivePendingResolution(t *testing.T) {
	state := NewSessionState("runtime", nil)
	if _, err := state.BindRules(rulesTestLock(rulesDigestA), rulesTestPayload(t, `{}`)); err != nil {
		t.Fatal(err)
	}
	handle := beginRuntimeRequest(t, state, "active-request")
	pending := &RulesPendingResolution{
		ResolutionID: "active-resolution", RequestID: "active-request",
		Principal: rules.Principal{ID: "host:oracle", Kind: "llm"},
		Pending: rules.PendingStep{
			StepID: "active-step", Kind: rules.StepKindNeedDecision,
			State: rulesTestPayload(t, `{"phase":"waiting"}`),
		},
		Request: rulesTestPayload(t, `{"authority":"host:oracle","prompt":"Choose","options":[{"id":"a","label":"A"}]}`), StepCount: 1,
	}
	if _, err := state.CommitRulesRequest(handle, RulesCommit{
		State: handle.Snapshot.State, ResolutionID: "active-resolution", Pending: pending,
		Result: runtimeResult("pending"),
	}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for i := 0; i < MaxRulesReceipts-1; i++ {
		state.Rules.Receipts = append(state.Rules.Receipts, RulesReceipt{
			RequestID: fmt.Sprintf("terminal-%d", i), Tool: "game_submit_intent",
			Fingerprint: runtimeFingerprint, ResolutionID: fmt.Sprintf("terminal-resolution-%d", i),
			Result: runtimeResult("terminal"), CreatedAt: now, UpdatedAt: now,
		})
	}
	runtime, ok := state.RulesRuntimeSnapshot()
	if !ok || len(runtime.Receipts) != MaxRulesReceipts || len(runtime.Pending) != 1 {
		t.Fatalf("runtime after eviction: ok=%v runtime=%+v", ok, runtime)
	}
	found := false
	for _, receipt := range runtime.Receipts {
		found = found || receipt.RequestID == "active-request"
	}
	if !found {
		t.Fatal("receipt for active pending resolution was evicted")
	}
}

func TestRulesRuntimeOptimisticRevisionConflictIsAtomic(t *testing.T) {
	state := NewSessionState("runtime", nil)
	if _, err := state.BindRules(rulesTestLock(rulesDigestA), rulesTestPayload(t, `{"value":0}`)); err != nil {
		t.Fatal(err)
	}
	stale := beginRuntimeRequest(t, state, "stale")
	winner := beginRuntimeRequest(t, state, "winner")
	eventData := rulesTestPayload(t, `{"amount":1}`)
	if _, err := state.CommitRulesRequest(winner, RulesCommit{
		State:     rulesTestPayload(t, `{"value":1}`),
		Principal: rules.Principal{ID: "host:oracle", Kind: "llm"},
		EventBatches: []RulesEventDraft{{ResolutionID: "winner", Events: []rules.Event{{
			Type: "counter.incremented", SchemaVersion: 1, Data: eventData,
		}}}},
		ResolutionID: "winner",
		Result:       runtimeResult("winner"),
		LogEntries:   []LogEntry{{Type: LogSystem, Message: "winner"}},
	}); err != nil {
		t.Fatal(err)
	}
	_, err := state.CommitRulesRequest(stale, RulesCommit{
		State: stale.Snapshot.State, ResolutionID: "stale", Result: runtimeResult("stale"),
		LogEntries: []LogEntry{{Type: LogSystem, Message: "stale"}},
	})
	if !errors.Is(err, ErrRulesRevisionConflict) {
		t.Fatalf("stale commit = %v", err)
	}
	runtime, ok := state.RulesRuntimeSnapshot()
	if !ok || runtime.Revision != 1 || runtime.State.String() != `{"value":1}` || len(runtime.EventBatches) != 1 || len(runtime.Receipts) != 1 {
		t.Fatalf("runtime after conflict: ok=%v value=%+v", ok, runtime)
	}
	log := state.RecentLog(0)
	if len(log) != 1 || log[0].Message != "winner" {
		t.Fatalf("log after conflict = %+v", log)
	}
}

func TestRulesRuntimeGenerationCASRejectsCompetingMetadataTransition(t *testing.T) {
	state := NewSessionState("runtime", nil)
	if _, err := state.BindRules(rulesTestLock(rulesDigestA), rulesTestPayload(t, `{}`)); err != nil {
		t.Fatal(err)
	}
	first := beginRuntimeRequest(t, state, "response-first")
	second := beginRuntimeRequest(t, state, "response-second")
	pendingFor := func(requestID, stepID string) *RulesPendingResolution {
		return &RulesPendingResolution{
			ResolutionID: "shared-resolution", RequestID: requestID, StepCount: 2,
			Principal: rules.Principal{ID: "host:oracle", Kind: "llm"},
			Pending: rules.PendingStep{
				StepID: stepID, Kind: rules.StepKindNeedDecision,
				State: rulesTestPayload(t, `{"phase":"again"}`),
			},
			Request: rulesTestPayload(t, `{"authority":"host:oracle","prompt":"Again?","options":[{"id":"yes","label":"Yes"}]}`),
		}
	}
	if _, err := state.CommitRulesRequest(first, RulesCommit{
		State: first.Snapshot.State, ResolutionID: "shared-resolution",
		Pending: pendingFor("response-first", "step-first"), Result: runtimeResult("needs-input"),
	}); err != nil {
		t.Fatal(err)
	}
	_, err := state.CommitRulesRequest(second, RulesCommit{
		State: second.Snapshot.State, ResolutionID: "shared-resolution",
		Pending: pendingFor("response-second", "step-second"), Result: runtimeResult("must-not-commit"),
	})
	if !errors.Is(err, ErrRulesGenerationConflict) {
		t.Fatalf("competing metadata commit = %v", err)
	}
	runtime, ok := state.RulesRuntimeSnapshot()
	if !ok || runtime.Generation != 1 || len(runtime.Pending) != 1 || runtime.Pending[0].RequestID != "response-first" || len(runtime.Receipts) != 1 {
		t.Fatalf("generation conflict partially mutated state: ok=%v runtime=%+v", ok, runtime)
	}
}

func TestRulesRuntimeInvalidCommitHasNoMutation(t *testing.T) {
	state := NewSessionState("runtime", nil)
	if _, err := state.BindRules(rulesTestLock(rulesDigestA), rulesTestPayload(t, `{"value":0}`)); err != nil {
		t.Fatal(err)
	}
	handle := beginRuntimeRequest(t, state, "invalid")
	_, err := state.CommitRulesRequest(handle, RulesCommit{
		State: rulesTestPayload(t, `{"value":99}`), ResolutionID: "invalid", Result: runtimeResult("bad"),
		LogEntries: []LogEntry{{Type: LogSystem, Message: "must not appear"}},
	})
	if err == nil {
		t.Fatal("unaudited state mutation unexpectedly committed")
	}
	runtime, ok := state.RulesRuntimeSnapshot()
	if !ok || runtime.Generation != 0 || runtime.Revision != 0 || runtime.State.String() != `{"value":0}` || len(runtime.Receipts) != 0 || state.LogLen() != 0 {
		t.Fatalf("invalid commit mutated runtime: ok=%v runtime=%+v log=%d", ok, runtime, state.LogLen())
	}
}

func TestRulesRuntimeRejectsOversizedAggregateBeforeMutation(t *testing.T) {
	state := NewSessionState("runtime", nil)
	if _, err := state.BindRules(rulesTestLock(rulesDigestA), rulesTestPayload(t, `{}`)); err != nil {
		t.Fatal(err)
	}
	handle := beginRuntimeRequest(t, state, "oversized")
	large := rulesTestPayload(t, `"`+strings.Repeat("x", 950_000)+`"`)
	events := make([]rules.Event, 9)
	for i := range events {
		events[i] = rules.Event{Type: "audit.large", SchemaVersion: 1, Data: large}
	}
	_, err := state.CommitRulesRequest(handle, RulesCommit{
		State: handle.Snapshot.State, Principal: rules.Principal{ID: "host:oracle", Kind: "llm"},
		ResolutionID: "oversized", EventBatches: []RulesEventDraft{{ResolutionID: "oversized", Events: events}},
		Result: runtimeResult("must-not-commit"),
	})
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized commit = %v", err)
	}
	runtime, ok := state.RulesRuntimeSnapshot()
	if !ok || runtime.Generation != 0 || runtime.Revision != 0 || len(runtime.Receipts) != 0 || len(runtime.EventBatches) != 0 {
		t.Fatalf("oversized commit mutated runtime: ok=%v runtime=%+v", ok, runtime)
	}
}

func TestRulesRuntimePersistsPendingContinuationAndAudit(t *testing.T) {
	state := NewSessionState("runtime", nil)
	if _, err := state.BindRules(rulesTestLock(rulesDigestA), rulesTestPayload(t, `{}`)); err != nil {
		t.Fatal(err)
	}
	handle := beginRuntimeRequest(t, state, "pending-request")
	continuation := rulesTestPayload(t, `{"phase":"choose"}`)
	request := rulesTestPayload(t, `{"authority":"host:oracle","prompt":"Choose","options":[{"id":"a","label":"A"}]}`)
	pending := &RulesPendingResolution{
		ResolutionID: "resolution-1", RequestID: "pending-request",
		Principal: rules.Principal{ID: "host:oracle", Kind: "llm"},
		Pending:   rules.PendingStep{StepID: "step-1", Kind: rules.StepKindNeedDecision, State: continuation},
		Request:   request, StepCount: 1,
	}
	if _, err := state.CommitRulesRequest(handle, RulesCommit{
		State: handle.Snapshot.State, ResolutionID: "resolution-1", Pending: pending,
		Principal: rules.Principal{ID: "host:oracle", Kind: "llm"},
		RandomDraws: []RulesRandomDraft{{
			ResolutionID: "resolution-1", StepID: "random-1", Method: "coin.flip", Source: "host",
			Specification: rulesTestPayload(t, `{"count":1}`), Result: rulesTestPayload(t, `{"faces":["heads"]}`),
		}},
		Result: runtimeResult(`{"status":"needs_input"}`),
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	var restored SessionState
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	runtime, ok := restored.RulesRuntimeSnapshot()
	if !ok || len(runtime.Pending) != 1 || len(runtime.RandomDraws) != 1 || runtime.Pending[0].Pending.State.String() != continuation.String() {
		t.Fatalf("restored runtime ok=%v runtime=%+v", ok, runtime)
	}
}

func TestImportStructuredUsesHostGenerationForMetadataOnlyCommit(t *testing.T) {
	lock := rulesTestLock(rulesDigestA)
	state := rulesTestPayload(t, `{}`)
	source := NewSessionState("source", nil)
	destination := NewSessionState("destination", nil)
	if _, err := source.BindRules(lock, state); err != nil {
		t.Fatal(err)
	}
	if _, err := destination.BindRules(lock, state); err != nil {
		t.Fatal(err)
	}
	handle := beginRuntimeRequest(t, source, "metadata-only")
	if _, err := source.CommitRulesRequest(handle, RulesCommit{
		State: handle.Snapshot.State, ResolutionID: "metadata-only", Result: runtimeResult("stored"),
	}); err != nil {
		t.Fatal(err)
	}
	if err := destination.ImportStructuredChecked(source); err != nil {
		t.Fatal(err)
	}
	runtime, ok := destination.RulesRuntimeSnapshot()
	if !ok || runtime.Revision != 0 || runtime.Generation != 1 || len(runtime.Receipts) != 1 || runtime.Receipts[0].RequestID != "metadata-only" {
		t.Fatalf("imported runtime: ok=%v runtime=%+v", ok, runtime)
	}
}

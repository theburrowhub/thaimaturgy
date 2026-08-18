package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/rules"
)

const (
	rulesDigestA = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	rulesDigestB = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func rulesTestLock(digest string) rules.Lock {
	return rules.Lock{
		ID:              "example.rules",
		Version:         "1.2.3",
		Digest:          digest,
		ProtocolVersion: rules.ProtocolVersion,
	}
}

func rulesTestPayload(t *testing.T, raw string) rules.Payload {
	t.Helper()
	payload, err := rules.NewPayload([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func TestRulesSessionLegacyJSONAndRoundTrip(t *testing.T) {
	legacy := NewSessionState("legacy", nil)
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"rules"`) {
		t.Fatalf("legacy session unexpectedly contains rules block: %s", raw)
	}
	if _, ok := legacy.RulesSnapshot(); ok {
		t.Fatal("legacy session unexpectedly has a rules snapshot")
	}

	lock := rulesTestLock(rulesDigestA)
	state := rulesTestPayload(t, `{"phase":"opening"}`)
	created, err := legacy.BindRules(lock, state)
	if err != nil || !created {
		t.Fatalf("BindRules created=%v err=%v", created, err)
	}
	raw, err = json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	var restored SessionState
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	snapshot, ok := restored.RulesSnapshot()
	if !ok {
		t.Fatal("restored session has no valid rules snapshot")
	}
	if snapshot.Ruleset != lock || snapshot.Revision != 0 || snapshot.State.String() != state.String() {
		t.Fatalf("restored snapshot = %+v state=%s", snapshot, snapshot.State.String())
	}
}

func TestRulesSessionLegacyUnjournaledRevisionBecomesReplayRoot(t *testing.T) {
	lock := rulesTestLock(rulesDigestA)
	raw, err := json.Marshal(map[string]any{
		"name": "legacy-rules",
		"rules": map[string]any{
			"lock": lock, "revision": 7, "state": map[string]any{"phase": "legacy"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var restored SessionState
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	runtime, ok := restored.RulesRuntimeSnapshot()
	if !ok || runtime.Revision != 0 || runtime.InitialState.String() != `{"phase":"legacy"}` || runtime.State.String() != runtime.InitialState.String() {
		t.Fatalf("legacy replay root: ok=%v runtime=%+v", ok, runtime)
	}
}

func TestBindRulesIsIdempotentAndPreservesState(t *testing.T) {
	session := NewSessionState("bound", nil)
	lock := rulesTestLock(rulesDigestA)
	initial := rulesTestPayload(t, `{"value":1}`)
	if created, err := session.BindRules(lock, initial); err != nil || !created {
		t.Fatalf("first bind created=%v err=%v", created, err)
	}
	replacement := rulesTestPayload(t, `{"value":2}`)
	if created, err := session.BindRules(lock, replacement); err != nil || created {
		t.Fatalf("idempotent bind created=%v err=%v", created, err)
	}
	snapshot, ok := session.RulesSnapshot()
	if !ok {
		t.Fatal("missing snapshot")
	}
	if snapshot.Revision != 0 || snapshot.State.String() != initial.String() {
		t.Fatalf("idempotent bind replaced state: revision=%d state=%s", snapshot.Revision, snapshot.State.String())
	}
}

func TestBindRulesRejectsAnotherLock(t *testing.T) {
	session := NewSessionState("bound", nil)
	initial := rulesTestPayload(t, `{"value":1}`)
	first := rulesTestLock(rulesDigestA)
	if _, err := session.BindRules(first, initial); err != nil {
		t.Fatal(err)
	}

	created, err := session.BindRules(rulesTestLock(rulesDigestB), rulesTestPayload(t, `{"value":2}`))
	if created || !errors.Is(err, ErrRulesLockConflict) {
		t.Fatalf("different lock created=%v err=%v", created, err)
	}
	snapshot, ok := session.RulesSnapshot()
	if !ok || snapshot.Ruleset != first || snapshot.State.String() != initial.String() {
		t.Fatalf("conflicting bind changed snapshot: ok=%v snapshot=%+v", ok, snapshot)
	}
}

func TestBindRulesValidatesLockAndState(t *testing.T) {
	session := NewSessionState("validation", nil)
	badLock := rulesTestLock(rulesDigestA)
	badLock.ProtocolVersion = ""
	if created, err := session.BindRules(badLock, rulesTestPayload(t, `{}`)); err == nil || created {
		t.Fatalf("invalid lock created=%v err=%v", created, err)
	}
	if created, err := session.BindRules(rulesTestLock(rulesDigestA), rules.Payload{}); err == nil || created {
		t.Fatalf("missing state created=%v err=%v", created, err)
	}
	if _, ok := session.RulesSnapshot(); ok {
		t.Fatal("invalid bind left a rules block behind")
	}
}

func TestImportStructuredCopiesOnlyCompatibleRulesBlock(t *testing.T) {
	lock := rulesTestLock(rulesDigestA)
	src := NewSessionState("src", nil)
	if _, err := src.BindRules(lock, rulesTestPayload(t, `{"source":true}`)); err != nil {
		t.Fatal(err)
	}
	dst := NewSessionState("dst", nil)
	if err := dst.ImportStructuredChecked(src); err != nil {
		t.Fatal(err)
	}
	snapshot, ok := dst.RulesSnapshot()
	if !ok || snapshot.Ruleset != lock || snapshot.Revision != 0 || snapshot.State.String() != `{"source":true}` {
		t.Fatalf("imported snapshot: ok=%v snapshot=%+v", ok, snapshot)
	}

	other := NewSessionState("other", nil)
	otherLock := rulesTestLock(rulesDigestB)
	if _, err := other.BindRules(otherLock, rulesTestPayload(t, `{"other":true}`)); err != nil {
		t.Fatal(err)
	}
	if err := dst.ImportStructuredChecked(other); !errors.Is(err, ErrRulesLockConflict) {
		t.Fatalf("different-lock import error = %v", err)
	}
	snapshot, ok = dst.RulesSnapshot()
	if !ok || snapshot.Ruleset != lock || snapshot.State.String() != `{"source":true}` {
		t.Fatalf("import silently changed lock: ok=%v snapshot=%+v", ok, snapshot)
	}
}

func TestImportStructuredTreatsNilAndEmptyRuntimeSlicesAsSameGeneration(t *testing.T) {
	state := NewSessionState("empty-runtime", nil)
	if _, err := state.BindRules(rulesTestLock(rulesDigestA), rulesTestPayload(t, `{"value":0}`)); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	var handoff SessionState
	if err := json.Unmarshal(encoded, &handoff); err != nil {
		t.Fatal(err)
	}
	if err := state.ImportStructuredChecked(&handoff); err != nil {
		t.Fatalf("no-op generation-zero handoff: %v", err)
	}
	runtime, ok := state.RulesRuntimeSnapshot()
	if !ok || runtime.Generation != 0 || runtime.Revision != 0 {
		t.Fatalf("runtime changed during no-op handoff: ok=%v runtime=%+v", ok, runtime)
	}
}

func TestImportStructuredDoesNotRollBackOrRewriteEqualRulesGeneration(t *testing.T) {
	lock := rulesTestLock(rulesDigestA)
	initial := rulesTestPayload(t, `{"value":"initial"}`)
	dst := NewSessionState("dst", nil)
	if _, err := dst.BindRules(lock, initial); err != nil {
		t.Fatal(err)
	}
	handle := beginRuntimeRequest(t, dst, "current")
	if _, err := dst.CommitRulesRequest(handle, RulesCommit{
		State: handle.Snapshot.State, ResolutionID: "current", Result: runtimeResult("current"),
	}); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name       string
		equalEpoch bool
	}{
		{"older", false},
		{"same generation different receipt", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			src := NewSessionState("src", nil)
			if _, err := src.BindRules(lock, initial); err != nil {
				t.Fatal(err)
			}
			if test.equalEpoch {
				dst.CurrentZone = "current-zone"
				src.CurrentZone = "fork-zone"
				handle := beginRuntimeRequest(t, src, "fork")
				if _, err := src.CommitRulesRequest(handle, RulesCommit{
					State: handle.Snapshot.State, ResolutionID: "fork", Result: runtimeResult("fork"),
				}); err != nil {
					t.Fatal(err)
				}
			}
			err := dst.ImportStructuredChecked(src)
			if test.equalEpoch && !errors.Is(err, ErrRulesImportConflict) {
				t.Fatalf("equal-generation fork error = %v", err)
			}
			if !test.equalEpoch && err != nil {
				t.Fatalf("stale import error = %v", err)
			}
			runtime, ok := dst.RulesRuntimeSnapshot()
			if !ok || runtime.Generation != 1 || len(runtime.Receipts) != 1 || runtime.Receipts[0].RequestID != "current" {
				t.Fatalf("rules runtime rolled back/forked: ok=%v runtime=%+v", ok, runtime)
			}
			if test.equalEpoch && dst.CurrentZone != "current-zone" {
				t.Fatalf("fork partially imported ordinary state: zone=%q", dst.CurrentZone)
			}
		})
	}

	higherFork := NewSessionState("higher-fork", nil)
	if _, err := higherFork.BindRules(lock, initial); err != nil {
		t.Fatal(err)
	}
	for _, requestID := range []string{"fork-1", "fork-2"} {
		handle := beginRuntimeRequest(t, higherFork, requestID)
		if _, err := higherFork.CommitRulesRequest(handle, RulesCommit{
			State: handle.Snapshot.State, ResolutionID: requestID, Result: runtimeResult(requestID),
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := dst.ImportStructuredChecked(higherFork); !errors.Is(err, ErrRulesImportConflict) {
		t.Fatalf("higher-generation fork error = %v", err)
	}
	unchanged, ok := dst.RulesRuntimeSnapshot()
	if !ok || unchanged.Generation != 1 || len(unchanged.Receipts) != 1 || unchanged.Receipts[0].RequestID != "current" {
		t.Fatalf("higher fork changed receiver: ok=%v runtime=%+v", ok, unchanged)
	}

	encoded, err := json.Marshal(dst)
	if err != nil {
		t.Fatal(err)
	}
	newer := new(SessionState)
	if err := json.Unmarshal(encoded, newer); err != nil {
		t.Fatal(err)
	}
	handle = beginRuntimeRequest(t, newer, "newer-2")
	if _, err := newer.CommitRulesRequest(handle, RulesCommit{
		State: handle.Snapshot.State, ResolutionID: "newer-2", Result: runtimeResult("newer-2"),
	}); err != nil {
		t.Fatal(err)
	}
	if err := dst.ImportStructuredChecked(newer); err != nil {
		t.Fatal(err)
	}
	runtime, ok := dst.RulesRuntimeSnapshot()
	if !ok || runtime.Generation != 2 || len(runtime.Receipts) != 2 || runtime.Receipts[1].RequestID != "newer-2" {
		t.Fatalf("newer rules runtime was not imported: ok=%v runtime=%+v", ok, runtime)
	}
}

func TestImportStructuredRejectsInvalidRulesWithoutPartialMutation(t *testing.T) {
	lock := rulesTestLock(rulesDigestA)
	initial := rulesTestPayload(t, `{"value":"initial"}`)
	dst := NewSessionState("dst", nil)
	if _, err := dst.BindRules(lock, initial); err != nil {
		t.Fatal(err)
	}
	dst.CurrentZone = "current-zone"

	src := NewSessionState("src", nil)
	if _, err := src.BindRules(lock, initial); err != nil {
		t.Fatal(err)
	}
	src.CurrentZone = "source-zone"
	src.Rules.InitialState = rules.Payload{}
	if err := dst.ImportStructuredChecked(src); err == nil {
		t.Fatal("invalid source rules runtime was accepted")
	}
	if dst.CurrentZone != "current-zone" {
		t.Fatalf("invalid import partially changed ordinary state: zone=%q", dst.CurrentZone)
	}
	runtime, ok := dst.RulesRuntimeSnapshot()
	if !ok || runtime.InitialState.String() != initial.String() || runtime.State.String() != initial.String() {
		t.Fatalf("invalid import changed rules runtime: ok=%v runtime=%+v", ok, runtime)
	}
}

package storage

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/rules"
)

func TestSaveSessionRejectsStaleWriterAfterWaitingForFileLock(t *testing.T) {
	root := t.TempDir()
	initialStore, err := NewWithPath(root)
	if err != nil {
		t.Fatal(err)
	}
	newerStore, err := NewWithPath(root)
	if err != nil {
		t.Fatal(err)
	}
	staleStore, err := NewWithPath(root)
	if err != nil {
		t.Fatal(err)
	}

	newer := newStorageRulesState(t, "ordered")
	stale := cloneStorageState(t, newer)
	if err := initialStore.SaveSession(stale); err != nil {
		t.Fatal(err)
	}
	advanceStorageRules(t, newer, "newer-checkpoint")

	locked := make(chan struct{})
	release := make(chan struct{})
	newerResult := make(chan error, 1)
	go func() {
		newerResult <- newerStore.saveSession(newer, func() {
			close(locked)
			<-release
		})
	}()
	<-locked

	staleStarted := make(chan struct{})
	staleResult := make(chan error, 1)
	go func() {
		close(staleStarted)
		staleResult <- staleStore.SaveSession(stale)
	}()
	<-staleStarted
	close(release)

	if err := <-newerResult; err != nil {
		t.Fatalf("newer writer: %v", err)
	}
	if err := <-staleResult; !errors.Is(err, domain.ErrRulesGenerationConflict) {
		t.Fatalf("stale writer error = %v", err)
	}
	loaded, err := initialStore.LoadSession("ordered")
	if err != nil {
		t.Fatal(err)
	}
	runtime, ok := loaded.RulesRuntimeSnapshot()
	if !ok || runtime.Generation != 1 || len(runtime.Receipts) != 1 || runtime.Receipts[0].RequestID != "newer-checkpoint" {
		t.Fatalf("durable runtime rolled back: ok=%v runtime=%+v", ok, runtime)
	}
}

func TestSaveSessionRejectsEqualGenerationFork(t *testing.T) {
	store, err := NewWithPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first := newStorageRulesState(t, "fork")
	second := cloneStorageState(t, first)
	advanceStorageRules(t, first, "first-branch")
	advanceStorageRules(t, second, "second-branch")
	if err := store.SaveSession(first); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSession(second); !errors.Is(err, domain.ErrRulesImportConflict) {
		t.Fatalf("fork error = %v", err)
	}
}

func TestSaveSessionAllowsLegacyBindingButNeverRemovesIt(t *testing.T) {
	store, err := NewWithPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	legacy := domain.NewSessionState("legacy", sampleAdventure())
	if err := store.SaveSession(legacy); err != nil {
		t.Fatal(err)
	}
	bound := cloneStorageState(t, legacy)
	payload, err := rules.PayloadFrom(map[string]any{"value": 0})
	if err != nil {
		t.Fatal(err)
	}
	if created, err := bound.BindRules(storageRulesLock(), payload); err != nil || !created {
		t.Fatalf("bind created=%v err=%v", created, err)
	}
	if err := store.SaveSession(bound); err != nil {
		t.Fatalf("first binding: %v", err)
	}
	legacy.CurrentRoom = "legacy-writer"
	if err := store.SaveSession(legacy); !errors.Is(err, domain.ErrRulesGenerationConflict) {
		t.Fatalf("legacy rollback error = %v", err)
	}
	loaded, err := store.LoadSession("legacy")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := loaded.RulesSnapshot(); !ok {
		t.Fatal("legacy writer removed the exact rules binding")
	}
}

func newStorageRulesState(t *testing.T, name string) *domain.SessionState {
	t.Helper()
	state := domain.NewSessionState(name, sampleAdventure())
	payload, err := rules.PayloadFrom(map[string]any{"value": 0})
	if err != nil {
		t.Fatal(err)
	}
	if created, err := state.BindRules(storageRulesLock(), payload); err != nil || !created {
		t.Fatalf("bind created=%v err=%v", created, err)
	}
	return state
}

func storageRulesLock() rules.Lock {
	return rules.Lock{
		ID: "storage-test", Version: "1.0.0", ProtocolVersion: rules.ProtocolVersion,
		Digest: "sha256:" + strings.Repeat("a", 64),
	}
}

func cloneStorageState(t *testing.T, state *domain.SessionState) *domain.SessionState {
	t.Helper()
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	var clone domain.SessionState
	if err := json.Unmarshal(data, &clone); err != nil {
		t.Fatal(err)
	}
	return &clone
}

func advanceStorageRules(t *testing.T, state *domain.SessionState, requestID string) {
	t.Helper()
	handle, receipt, err := state.BeginRulesRequest(
		context.Background(), requestID, "game_submit_intent", "sha256:"+strings.Repeat("b", 64),
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

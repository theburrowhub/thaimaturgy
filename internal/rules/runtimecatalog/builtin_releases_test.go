package runtimecatalog

import (
	"context"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/rules"
	"github.com/theburrowhub/thaimaturgy/internal/rules/catalog"
)

func TestBuiltinDefinitionsMatchAppendOnlyReleaseLedger(t *testing.T) {
	releases, err := builtinReleaseLocks()
	if err != nil {
		t.Fatal(err)
	}
	definitions := builtinDefinitions()
	if len(definitions) != len(releases) {
		t.Fatalf("built-in definitions = %d, release ledger entries = %d", len(definitions), len(releases))
	}

	ledger := make(map[builtinReleaseKey]rules.Lock, len(releases))
	for _, lock := range releases {
		ledger[builtinReleaseKey{id: lock.ID, version: lock.Version}] = lock
	}
	seen := make(map[builtinReleaseKey]struct{}, len(definitions))
	for _, definition := range definitions {
		artifact, err := definition.artifact()
		if err != nil {
			t.Fatalf("build %s artifact: %v", definition.id, err)
		}
		lock := artifact.Lock()
		key := builtinReleaseKey{id: lock.ID, version: lock.Version}
		if lock.ID != definition.id {
			t.Errorf("definition %s produced artifact ID %s", definition.id, lock.ID)
		}
		if _, duplicate := seen[key]; duplicate {
			t.Errorf("duplicate definition for %s@%s", lock.ID, lock.Version)
		}
		seen[key] = struct{}{}
		if want, exists := ledger[key]; !exists {
			t.Errorf("definition %s@%s is absent from builtins.lock.json", lock.ID, lock.Version)
		} else if lock != want {
			t.Errorf("definition %s@%s lock = %+v, want %+v", lock.ID, lock.Version, lock, want)
		}
	}
	for _, lock := range releases {
		key := builtinReleaseKey{id: lock.ID, version: lock.Version}
		if _, exists := seen[key]; !exists {
			t.Errorf("release %s@%s has no retained definition", lock.ID, lock.Version)
		}
	}
}

func TestEveryBuiltinReleaseIsRegisteredAndOpenableByExactLock(t *testing.T) {
	ctx := context.Background()
	available := catalog.New()
	if err := registerBuiltins(ctx, available); err != nil {
		t.Fatal(err)
	}
	releases, err := builtinReleaseLocks()
	if err != nil {
		t.Fatal(err)
	}

	want := make(map[rules.Lock]struct{}, len(releases))
	ids := make(map[string]struct{}, len(releases))
	for _, lock := range releases {
		want[lock] = struct{}{}
		ids[lock.ID] = struct{}{}
		implementation, err := available.Lookup(lock)
		if err != nil {
			t.Errorf("exact lookup %s@%s (%s): %v", lock.ID, lock.Version, lock.Digest, err)
			continue
		}
		initial, err := available.InitialState(lock)
		if err != nil {
			t.Errorf("initial state %s@%s: %v", lock.ID, lock.Version, err)
			continue
		}
		if err := implementation.ValidateState(ctx, rules.ValidateStateRequest{Snapshot: rules.Snapshot{
			Ruleset: lock,
			State:   initial,
		}}); err != nil {
			t.Errorf("open %s@%s initial state: %v", lock.ID, lock.Version, err)
		}
	}

	registered := 0
	for id := range ids {
		for _, lock := range available.Locks(id) {
			registered++
			if _, exists := want[lock]; !exists {
				t.Errorf("registered built-in lock missing from builtins.lock.json: %+v", lock)
			}
		}
	}
	if registered != len(releases) {
		t.Errorf("registered built-in locks = %d, release ledger entries = %d", registered, len(releases))
	}
}

func TestBuiltinReleaseLedgerRejectsAmbiguousHistory(t *testing.T) {
	const digest = "sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	for name, raw := range map[string]string{
		"empty":         `[]`,
		"unknown field": `[{"id":"test","version":"1.0.0","digest":"` + digest + `","protocol_version":"1.0.0","extra":true}]`,
		"duplicate release": `[` +
			`{"id":"test","version":"1.0.0","digest":"` + digest + `","protocol_version":"1.0.0"},` +
			`{"id":"test","version":"1.0.0","digest":"` + digest + `","protocol_version":"1.0.0"}` +
			`]`,
		"foreign protocol": `[{"id":"test","version":"1.0.0","digest":"` + digest + `","protocol_version":"2.0.0"}]`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeBuiltinReleaseLocks([]byte(raw)); err == nil {
				t.Fatal("invalid release history was accepted")
			}
		})
	}
}

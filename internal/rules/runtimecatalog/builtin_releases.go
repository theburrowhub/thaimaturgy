package runtimecatalog

import (
	_ "embed"
	"fmt"

	"github.com/theburrowhub/thaimaturgy/internal/jsonstrict"
	"github.com/theburrowhub/thaimaturgy/internal/rules"
)

// builtinReleaseLedgerJSON is an append-only record. Once published, an entry
// and its matching implementation must remain unchanged so exact session locks
// can still be restored by later builds.
//
//go:embed builtins.lock.json
var builtinReleaseLedgerJSON []byte

type builtinReleaseKey struct {
	id      string
	version string
}

type preparedBuiltin struct {
	definition builtinDefinition
	artifact   rules.Artifact
}

func builtinReleaseLocks() ([]rules.Lock, error) {
	return decodeBuiltinReleaseLocks(builtinReleaseLedgerJSON)
}

func decodeBuiltinReleaseLocks(raw []byte) ([]rules.Lock, error) {
	var releases []rules.Lock
	if err := jsonstrict.Decode(raw, &releases); err != nil {
		return nil, fmt.Errorf("decode built-in release ledger: %w", err)
	}
	if len(releases) == 0 {
		return nil, fmt.Errorf("built-in release ledger is empty")
	}
	if len(releases) > rules.MaxCollectionItems {
		return nil, fmt.Errorf("built-in release ledger exceeds %d entries", rules.MaxCollectionItems)
	}
	seen := make(map[builtinReleaseKey]int, len(releases))
	for index, lock := range releases {
		if err := lock.Validate(); err != nil {
			return nil, fmt.Errorf("built-in release ledger entry %d: %w", index, err)
		}
		if lock.ProtocolVersion != rules.ProtocolVersion {
			return nil, fmt.Errorf("built-in release ledger entry %d uses protocol %s, host uses %s", index, lock.ProtocolVersion, rules.ProtocolVersion)
		}
		key := builtinReleaseKey{id: lock.ID, version: lock.Version}
		if previous, exists := seen[key]; exists {
			return nil, fmt.Errorf("built-in release ledger entry %d duplicates %s@%s from entry %d", index, lock.ID, lock.Version, previous)
		}
		seen[key] = index
	}
	return append([]rules.Lock(nil), releases...), nil
}

// prepareBuiltins validates the executable definitions against the immutable
// release ledger before any entry is published to the process catalog.
func prepareBuiltins() ([]preparedBuiltin, error) {
	releases, err := builtinReleaseLocks()
	if err != nil {
		return nil, err
	}
	expected := make(map[builtinReleaseKey]rules.Lock, len(releases))
	for _, lock := range releases {
		expected[builtinReleaseKey{id: lock.ID, version: lock.Version}] = lock
	}

	definitions := builtinDefinitions()
	prepared := make([]preparedBuiltin, 0, len(definitions))
	matched := make(map[builtinReleaseKey]struct{}, len(definitions))
	for _, definition := range definitions {
		artifact, err := definition.artifact()
		if err != nil {
			return nil, fmt.Errorf("build %s artifact: %w", definition.id, err)
		}
		lock := artifact.Lock()
		if lock.ID != definition.id {
			return nil, fmt.Errorf("built-in definition %s produced artifact ID %s", definition.id, lock.ID)
		}
		key := builtinReleaseKey{id: lock.ID, version: lock.Version}
		if _, duplicate := matched[key]; duplicate {
			return nil, fmt.Errorf("duplicate built-in definition for %s@%s", lock.ID, lock.Version)
		}
		ledgerLock, exists := expected[key]
		if !exists {
			return nil, fmt.Errorf("built-in definition %s@%s has no release ledger entry", lock.ID, lock.Version)
		}
		if lock != ledgerLock {
			return nil, fmt.Errorf("built-in definition %s@%s produced lock %+v, ledger requires %+v", lock.ID, lock.Version, lock, ledgerLock)
		}
		matched[key] = struct{}{}
		prepared = append(prepared, preparedBuiltin{definition: definition, artifact: artifact})
	}
	for _, lock := range releases {
		key := builtinReleaseKey{id: lock.ID, version: lock.Version}
		if _, exists := matched[key]; !exists {
			return nil, fmt.Errorf("built-in release %s@%s has no retained implementation", lock.ID, lock.Version)
		}
	}
	return prepared, nil
}

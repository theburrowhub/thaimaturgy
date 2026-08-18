package rules

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
)

var (
	// ErrRulesetNotFound means no registered artifact exactly matches a lock.
	ErrRulesetNotFound = errors.New("rules: ruleset not found")
	// ErrRulesetAlreadyRegistered means the exact artifact is already present.
	ErrRulesetAlreadyRegistered = errors.New("rules: ruleset already registered")
	// ErrArtifactConflict means an ID and version are already bound to another
	// digest or protocol. A release identity cannot equivocate in one registry.
	ErrArtifactConflict = errors.New("rules: artifact conflict")
	// ErrManifestMismatch means the verified artifact manifest differs from the
	// metadata returned by its executable ruleset.
	ErrManifestMismatch = errors.New("rules: artifact manifest mismatch")
	// ErrIncompatibleProtocol means a package targets another host protocol.
	ErrIncompatibleProtocol = errors.New("rules: incompatible protocol")
)

type registryEntry struct {
	artifact Artifact
	ruleset  Ruleset
}

type releaseKey struct {
	id      string
	version string
}

// Registry stores rulesets by exact ID, version, digest, and protocol. Its zero
// value is ready for use and all operations are safe for concurrent callers.
type Registry struct {
	mu       sync.RWMutex
	entries  map[Lock]registryEntry
	releases map[releaseKey]Lock
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry { return &Registry{} }

// Register validates and adds one exact rules artifact. Artifact proves the
// host hashed bundle bytes, and its manifest must exactly match Ruleset.Manifest.
// An ID and version may be bound to only one artifact.
func (r *Registry) Register(ctx context.Context, artifact Artifact, ruleset Ruleset) error {
	if ctx == nil {
		return invalid("context", "must not be nil")
	}
	if ruleset == nil {
		return invalid("ruleset", "must not be nil")
	}
	if err := artifact.Validate(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	manifest, err := ruleset.Manifest(ctx)
	if err != nil {
		return fmt.Errorf("rules: read manifest: %w", err)
	}
	manifest = cloneManifest(manifest)
	if err := manifest.Validate(); err != nil {
		return err
	}
	verifiedManifest := artifact.Manifest()
	if !manifestsEqual(verifiedManifest, manifest) {
		return ErrManifestMismatch
	}
	if manifest.ProtocolVersion != ProtocolVersion {
		return fmt.Errorf("%w: package uses %s, host uses %s", ErrIncompatibleProtocol, manifest.ProtocolVersion, ProtocolVersion)
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	lock := artifact.Lock()
	release := releaseKey{id: manifest.ID, version: manifest.Version}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.entries == nil {
		r.entries = make(map[Lock]registryEntry)
		r.releases = make(map[releaseKey]Lock)
	}
	if existing, exists := r.releases[release]; exists {
		if existing == lock {
			return fmt.Errorf("%w: %s@%s (%s)", ErrRulesetAlreadyRegistered, lock.ID, lock.Version, lock.Digest)
		}
		return fmt.Errorf("%w: %s@%s is already bound to %s", ErrArtifactConflict, lock.ID, lock.Version, existing.Digest)
	}
	r.entries[lock] = registryEntry{artifact: cloneArtifact(artifact), ruleset: ruleset}
	r.releases[release] = lock
	return nil
}

// Lookup returns the ruleset matching every field of lock.
func (r *Registry) Lookup(lock Lock) (Ruleset, error) {
	if err := lock.Validate(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	entry, exists := r.entries[lock]
	r.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("%w: %s@%s (%s)", ErrRulesetNotFound, lock.ID, lock.Version, lock.Digest)
	}
	return entry.ruleset, nil
}

// Manifest returns the validated manifest stored for an exact lock.
func (r *Registry) Manifest(lock Lock) (Manifest, error) {
	if err := lock.Validate(); err != nil {
		return Manifest{}, err
	}
	r.mu.RLock()
	entry, exists := r.entries[lock]
	r.mu.RUnlock()
	if !exists {
		return Manifest{}, fmt.Errorf("%w: %s@%s (%s)", ErrRulesetNotFound, lock.ID, lock.Version, lock.Digest)
	}
	return entry.artifact.Manifest(), nil
}

// Artifact returns a defensive copy of the host-attested artifact record for an
// exact lock.
func (r *Registry) Artifact(lock Lock) (Artifact, error) {
	if err := lock.Validate(); err != nil {
		return Artifact{}, err
	}
	r.mu.RLock()
	entry, exists := r.entries[lock]
	r.mu.RUnlock()
	if !exists {
		return Artifact{}, fmt.Errorf("%w: %s@%s (%s)", ErrRulesetNotFound, lock.ID, lock.Version, lock.Digest)
	}
	return cloneArtifact(entry.artifact), nil
}

func manifestsEqual(a, b Manifest) bool {
	return a.ID == b.ID &&
		a.Name == b.Name &&
		a.Description == b.Description &&
		a.Version == b.Version &&
		a.ProtocolVersion == b.ProtocolVersion &&
		a.Runtime == b.Runtime &&
		slices.Equal(a.Capabilities, b.Capabilities)
}

// Len returns the number of exact artifacts in the registry.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entries)
}

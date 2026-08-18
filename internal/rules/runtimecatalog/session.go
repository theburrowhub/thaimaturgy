package runtimecatalog

import (
	"context"
	"errors"
	"fmt"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/rules"
	"github.com/theburrowhub/thaimaturgy/internal/ruleshost"
)

var ErrAdventureRulesRequired = errors.New("rules runtime catalog: adventure has no valid rules package requirement")

// OpenSession is the only production constructor for a mechanically active
// local session. It resolves or restores the exact package before exposing the
// session to the engine, then injects the immutable process catalog used for all
// later exact lookups. Callers remain responsible for assigning PersistRules to
// their atomic storage operation before constructing an Oracle or ToolRouter.
func (e *Environment) OpenSession(
	ctx context.Context,
	state *domain.SessionState,
	adventure *domain.Adventure,
	config *domain.Config,
) (*domain.Session, error) {
	_, created, err := e.resolveSessionRules(ctx, state, adventure)
	if err != nil {
		return nil, err
	}
	session := domain.NewSession(state, adventure, config)
	session.RulesResolver = e.Catalog
	session.DataDirectory = e.DataDirectory
	if created {
		session.MarkModified()
	}
	return session, nil
}

// resolveSessionRules binds an unpinned session from the adventure requirement,
// or restores a pinned session by exact lock. It validates the package-owned
// state and reconstructs it from the immutable initial state and event journal
// before returning an implementation. No failed path mutates the session.
func (e *Environment) resolveSessionRules(
	ctx context.Context,
	state *domain.SessionState,
	adventure *domain.Adventure,
) (rules.Ruleset, bool, error) {
	if ctx == nil {
		return nil, false, errors.New("rules runtime catalog: nil session context")
	}
	if e == nil || e.Catalog == nil {
		return nil, false, errors.New("rules runtime catalog: unavailable catalog")
	}
	if state == nil {
		return nil, false, errors.New("rules runtime catalog: nil session state")
	}
	if adventure == nil {
		return nil, false, errors.New("rules runtime catalog: nil adventure")
	}
	if state.AdventureID != adventure.ID {
		return nil, false, fmt.Errorf("rules runtime catalog: session adventure %q does not match loaded adventure %q", state.AdventureID, adventure.ID)
	}
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}

	runtime, pinned, err := state.RulesRuntimeSnapshotStrict()
	if err != nil {
		return nil, false, fmt.Errorf("rules runtime catalog: invalid persisted rules runtime: %w", err)
	}
	if pinned {
		implementation, err := e.Catalog.Lookup(runtime.Lock)
		if err != nil {
			return nil, false, fmt.Errorf("rules runtime catalog: load exact session lock: %w", err)
		}
		if err := e.validateRuntime(ctx, implementation, runtime); err != nil {
			return nil, false, err
		}
		return implementation, false, nil
	}

	requirement, ok := adventure.RulesRequirement()
	if !ok {
		return nil, false, ErrAdventureRulesRequired
	}
	lock, implementation, err := e.Catalog.Resolve(requirement)
	if err != nil {
		return nil, false, fmt.Errorf("rules runtime catalog: resolve %s@%s: %w", requirement.ID, requirement.Version, err)
	}
	initial, err := e.Catalog.InitialState(lock)
	if err != nil {
		return nil, false, fmt.Errorf("rules runtime catalog: load initial state: %w", err)
	}
	if err := (ruleshost.Executor{Ruleset: implementation}).ValidateState(ctx, rules.Snapshot{
		Ruleset: lock,
		State:   initial,
	}); err != nil {
		return nil, false, fmt.Errorf("rules runtime catalog: validate initial state: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	created, err := state.BindRules(lock, initial)
	if err != nil {
		return nil, false, fmt.Errorf("rules runtime catalog: bind exact session lock: %w", err)
	}
	if !created {
		return nil, false, errors.New("rules runtime catalog: session binding changed concurrently")
	}
	return implementation, true, nil
}

func (e *Environment) validateRuntime(ctx context.Context, implementation rules.Ruleset, runtime domain.RulesSession) error {
	registeredInitial, err := e.Catalog.InitialState(runtime.Lock)
	if err != nil {
		return fmt.Errorf("rules runtime catalog: load registered initial state: %w", err)
	}
	if registeredInitial.String() != runtime.InitialState.String() {
		return errors.New("rules runtime catalog: persisted initial state does not match the exact package")
	}
	batches := make([]ruleshost.ReplayBatch, len(runtime.EventBatches))
	for index, batch := range runtime.EventBatches {
		batches[index] = ruleshost.ReplayBatch{
			Sequence:     batch.Sequence,
			BaseRevision: batch.BaseRevision,
			Revision:     batch.Revision,
			Events:       append([]rules.Event(nil), batch.Events...),
		}
	}
	replayed, err := ruleshost.Replay(ctx, implementation, runtime.Lock, runtime.InitialState, batches)
	if err != nil {
		return fmt.Errorf("rules runtime catalog: replay persisted rules: %w", err)
	}
	if replayed.Revision != runtime.Revision || replayed.State.String() != runtime.State.String() {
		return errors.New("rules runtime catalog: materialized rules state does not match replay")
	}
	return nil
}

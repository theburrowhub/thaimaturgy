package ruleskit

import (
	"context"
	"fmt"
	"slices"

	"github.com/theburrowhub/thaimaturgy/internal/jsonstrict"
	core "github.com/theburrowhub/thaimaturgy/internal/rules"
)

// StartFunc and ResumeFunc implement the mechanics owned by one reference
// package. Engine validates the protocol envelope and exact package lock before
// invoking them, then validates the returned tagged union.
type StartFunc func(core.StartRequest) (core.Step, error)
type ResumeFunc func(core.ResumeRequest) (core.Step, error)

// Definition is the immutable behavior installed in an Engine.
type Definition struct {
	Artifact     core.Artifact
	Actions      []core.ActionDescriptor
	Explanations map[string]string
	Start        StartFunc
	Resume       ResumeFunc
}

// Engine supplies the state-independent rules.Ruleset operations shared by
// the compact built-in reference packages.
type Engine struct {
	artifact     core.Artifact
	actions      []core.ActionDescriptor
	explanations map[string]string
	start        StartFunc
	resume       ResumeFunc
}

// NewEngine validates a definition once. Invalid built-in definitions are
// programmer errors and panic during construction rather than failing later in
// an unrelated game request.
func NewEngine(definition Definition) *Engine {
	if err := definition.Artifact.Validate(); err != nil {
		panic(fmt.Sprintf("ruleskit: invalid artifact: %v", err))
	}
	if err := core.ValidateActions(definition.Actions); err != nil {
		panic(fmt.Sprintf("ruleskit: invalid action catalog: %v", err))
	}
	if definition.Start == nil || definition.Resume == nil {
		panic("ruleskit: start and resume handlers are required")
	}
	actions := make([]core.ActionDescriptor, len(definition.Actions))
	for i, action := range definition.Actions {
		actions[i] = cloneAction(action)
	}
	explanations := make(map[string]string, len(definition.Explanations))
	for reference, text := range definition.Explanations {
		explanations[reference] = text
	}
	return &Engine{
		artifact:     definition.Artifact,
		actions:      actions,
		explanations: explanations,
		start:        definition.Start,
		resume:       definition.Resume,
	}
}

// Manifest implements rules.Ruleset.
func (e *Engine) Manifest(ctx context.Context) (core.Manifest, error) {
	if err := checkContext(ctx); err != nil {
		return core.Manifest{}, err
	}
	return e.artifact.Manifest(), nil
}

// ListActions implements rules.Ruleset.
func (e *Engine) ListActions(ctx context.Context, request core.CatalogRequest) ([]core.ActionDescriptor, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	if err := e.validateSnapshot(request.Snapshot); err != nil {
		return nil, err
	}
	actions := make([]core.ActionDescriptor, len(e.actions))
	for i, action := range e.actions {
		actions[i] = cloneAction(action)
	}
	return actions, nil
}

// Start implements rules.Ruleset.
func (e *Engine) Start(ctx context.Context, request core.StartRequest) (core.Step, error) {
	if err := checkContext(ctx); err != nil {
		return core.Step{}, err
	}
	if err := request.Validate(); err != nil {
		return core.Step{}, err
	}
	if err := e.validateSnapshot(request.Snapshot); err != nil {
		return core.Step{}, err
	}
	if !slices.ContainsFunc(e.actions, func(action core.ActionDescriptor) bool {
		return action.ID == request.Intent.ActionID
	}) {
		return Reject(request.Intent.ID, "unknown.action", "unknown action: "+request.Intent.ActionID), nil
	}
	step, err := e.start(request)
	if err != nil {
		return core.Step{}, err
	}
	if err := step.Validate(); err != nil {
		return core.Step{}, fmt.Errorf("%s: invalid start step: %w", e.artifact.Manifest().ID, err)
	}
	return step, nil
}

// Resume implements rules.Ruleset.
func (e *Engine) Resume(ctx context.Context, request core.ResumeRequest) (core.Step, error) {
	if err := checkContext(ctx); err != nil {
		return core.Step{}, err
	}
	if err := request.Validate(); err != nil {
		return core.Step{}, err
	}
	if err := e.validateSnapshot(request.Snapshot); err != nil {
		return core.Step{}, err
	}
	step, err := e.resume(request)
	if err != nil {
		return core.Step{}, err
	}
	if err := step.Validate(); err != nil {
		return core.Step{}, fmt.Errorf("%s: invalid resume step: %w", e.artifact.Manifest().ID, err)
	}
	return step, nil
}

// Project implements rules.Ruleset. These reference primitives are stateless,
// so the complete authorized projection is their validated empty state.
func (e *Engine) Project(ctx context.Context, request core.ProjectRequest) (core.Projection, error) {
	if err := checkContext(ctx); err != nil {
		return core.Projection{}, err
	}
	if err := request.Validate(); err != nil {
		return core.Projection{}, err
	}
	if err := e.validateSnapshot(request.Snapshot); err != nil {
		return core.Projection{}, err
	}
	return core.Projection{View: request.Snapshot.State}, nil
}

// Explain implements rules.Ruleset.
func (e *Engine) Explain(ctx context.Context, request core.ExplainRequest) (core.Explanation, error) {
	if err := checkContext(ctx); err != nil {
		return core.Explanation{}, err
	}
	if err := request.Validate(); err != nil {
		return core.Explanation{}, err
	}
	if err := e.validateSnapshot(request.Snapshot); err != nil {
		return core.Explanation{}, err
	}
	text, ok := e.explanations[request.Reference]
	if !ok {
		return core.Explanation{}, fmt.Errorf("%s: unknown rule reference %q", e.artifact.Manifest().ID, request.Reference)
	}
	return core.Explanation{Text: text}, nil
}

// ValidateState implements rules.Ruleset.
func (e *Engine) ValidateState(ctx context.Context, request core.ValidateStateRequest) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	return e.validateSnapshot(request.Snapshot)
}

// Reduce implements rules.Ruleset. Reference resolution packages do not own
// durable campaign state and therefore define no events.
func (e *Engine) Reduce(ctx context.Context, request core.ReduceRequest) (core.ReduceResult, error) {
	if err := checkContext(ctx); err != nil {
		return core.ReduceResult{}, err
	}
	if err := request.Validate(); err != nil {
		return core.ReduceResult{}, err
	}
	if err := e.validateSnapshot(request.Snapshot); err != nil {
		return core.ReduceResult{}, err
	}
	return core.ReduceResult{}, fmt.Errorf("%s: stateless reference package does not accept events", e.artifact.Manifest().ID)
}

// Migrate implements rules.Ruleset as an identity migration for this exact
// immutable artifact. Future versions must add explicit version migrations.
func (e *Engine) Migrate(ctx context.Context, request core.MigrateRequest) (core.MigrateResult, error) {
	if err := checkContext(ctx); err != nil {
		return core.MigrateResult{}, err
	}
	if err := request.Validate(); err != nil {
		return core.MigrateResult{}, err
	}
	if request.From != e.artifact.Lock() {
		return core.MigrateResult{}, fmt.Errorf("%s: no migration from %s@%s", e.artifact.Manifest().ID, request.From.ID, request.From.Version)
	}
	if err := validateEmptyState(request.State); err != nil {
		return core.MigrateResult{}, err
	}
	return core.MigrateResult{State: request.State}, nil
}

func (e *Engine) validateSnapshot(snapshot core.Snapshot) error {
	if err := snapshot.Validate(); err != nil {
		return err
	}
	if snapshot.Ruleset != e.artifact.Lock() {
		return fmt.Errorf("%s: snapshot belongs to a different rules artifact", e.artifact.Manifest().ID)
	}
	return validateEmptyState(snapshot.State)
}

func validateEmptyState(payload core.Payload) error {
	var state struct{}
	if err := jsonstrict.Decode(payload.Bytes(), &state); err != nil {
		return fmt.Errorf("ruleskit: state must be the empty object: %w", err)
	}
	return nil
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("ruleskit: nil context")
	}
	return ctx.Err()
}

func cloneAction(action core.ActionDescriptor) core.ActionDescriptor {
	action.Tags = append([]string(nil), action.Tags...)
	return action
}

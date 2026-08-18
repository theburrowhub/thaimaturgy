package starlarkruntime

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/theburrowhub/thaimaturgy/internal/jsonstrict"
	core "github.com/theburrowhub/thaimaturgy/internal/rules"
	star "go.starlark.net/starlark"
)

const (
	functionManifest      = "manifest"
	functionInitialState  = "initial_state"
	functionListActions   = "list_actions"
	functionStart         = "start"
	functionResume        = "resume"
	functionProject       = "project"
	functionExplain       = "explain"
	functionValidateState = "validate_state"
	functionReduce        = "reduce"
	functionMigrate       = "migrate"
)

var requiredFunctions = []string{
	functionManifest,
	functionInitialState,
	functionListActions,
	functionStart,
	functionResume,
	functionProject,
	functionExplain,
	functionValidateState,
	functionReduce,
	functionMigrate,
}

// Ruleset is an immutable, concurrently callable adapter around frozen
// Starlark module globals.
type Ruleset struct {
	manifest     core.Manifest
	lock         core.Lock
	initialState core.Payload
	functions    map[string]star.Callable
	limits       Limits
}

var _ core.Ruleset = (*Ruleset)(nil)

func newRuleset(ctx context.Context, manifest core.Manifest, lock core.Lock, globals star.StringDict, limits Limits) (*Ruleset, error) {
	functions := make(map[string]star.Callable, len(requiredFunctions))
	for _, name := range requiredFunctions {
		value, ok := globals[name]
		if !ok {
			return nil, fmt.Errorf("%w: entrypoint does not export %s()", ErrContract, name)
		}
		callable, ok := value.(star.Callable)
		if !ok {
			return nil, fmt.Errorf("%w: entrypoint export %s has type %s, want function", ErrContract, name, value.Type())
		}
		functions[name] = callable
	}
	ruleset := &Ruleset{
		manifest:  cloneManifest(manifest),
		lock:      lock,
		functions: functions,
		limits:    limits,
	}
	declaredValue, err := ruleset.call(ctx, functionManifest)
	if err != nil {
		return nil, err
	}
	declaredRaw, err := resultJSON(declaredValue, limits)
	if err != nil {
		return nil, fmt.Errorf("starlark rules: %s: %w", functionManifest, err)
	}
	var declared core.Manifest
	if err := jsonstrict.Decode(declaredRaw, &declared); err != nil {
		return nil, fmt.Errorf("%w: manifest() result: %v", ErrContract, err)
	}
	if err := declared.Validate(); err != nil {
		return nil, fmt.Errorf("%w: manifest() result: %v", ErrContract, err)
	}
	if !manifestsEqual(manifest, declared) {
		return nil, fmt.Errorf("%w: %s and manifest() differ", core.ErrManifestMismatch, ManifestPath)
	}
	initialValue, err := ruleset.call(ctx, functionInitialState)
	if err != nil {
		return nil, err
	}
	initialRaw, err := resultJSON(initialValue, limits)
	if err != nil {
		return nil, fmt.Errorf("starlark rules: %s: %w", functionInitialState, err)
	}
	initialState, err := core.NewPayload(initialRaw)
	if err != nil {
		return nil, fmt.Errorf("%w: initial_state() result: %v", ErrContract, err)
	}
	ruleset.initialState = initialState
	return ruleset, nil
}

func cloneManifest(manifest core.Manifest) core.Manifest {
	manifest.Capabilities = append([]string(nil), manifest.Capabilities...)
	return manifest
}

func manifestsEqual(left, right core.Manifest) bool {
	return left.ID == right.ID &&
		left.Name == right.Name &&
		left.Description == right.Description &&
		left.Version == right.Version &&
		left.ProtocolVersion == right.ProtocolVersion &&
		left.Runtime == right.Runtime &&
		slices.Equal(left.Capabilities, right.Capabilities)
}

func (r *Ruleset) call(ctx context.Context, name string, arguments ...star.Value) (star.Value, error) {
	if ctx == nil {
		return nil, fmt.Errorf("%w: nil context", ErrContract)
	}
	callable := r.functions[name]
	var result star.Value
	err := runStarlark(ctx, "ruleset "+name, r.limits, func(thread *star.Thread) error {
		var callErr error
		result, callErr = star.Call(thread, callable, star.Tuple(arguments), nil)
		return callErr
	})
	if err != nil {
		return nil, fmt.Errorf("starlark rules: %s: %w", name, err)
	}
	return result, nil
}

func (r *Ruleset) invoke(ctx context.Context, name string, request any, destination any) error {
	argument, err := requestValue(request, r.limits)
	if err != nil {
		return fmt.Errorf("starlark rules: %s: %w", name, err)
	}
	result, err := r.call(ctx, name, argument)
	if err != nil {
		return err
	}
	raw, err := resultJSON(result, r.limits)
	if err != nil {
		return fmt.Errorf("starlark rules: %s: %w", name, err)
	}
	if err := decodeResult(raw, destination); err != nil {
		return fmt.Errorf("starlark rules: %s: %w", name, err)
	}
	return nil
}

func (r *Ruleset) validateSnapshot(snapshot core.Snapshot) error {
	if snapshot.Ruleset != r.lock {
		return fmt.Errorf("starlark rules: snapshot lock does not match loaded artifact")
	}
	return nil
}

// Manifest implements rules.Ruleset without rerunning package code.
func (r *Ruleset) Manifest(ctx context.Context) (core.Manifest, error) {
	if ctx == nil {
		return core.Manifest{}, fmt.Errorf("%w: nil context", ErrContract)
	}
	if err := ctx.Err(); err != nil {
		return core.Manifest{}, err
	}
	return cloneManifest(r.manifest), nil
}

// InitialState returns the package-defined JSON state for a newly pinned
// session. Payload is immutable and may be persisted directly by an installer.
func (r *Ruleset) InitialState() core.Payload { return r.initialState }

// ListActions implements rules.Ruleset.
func (r *Ruleset) ListActions(ctx context.Context, request core.CatalogRequest) ([]core.ActionDescriptor, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	if err := r.validateSnapshot(request.Snapshot); err != nil {
		return nil, err
	}
	var result []core.ActionDescriptor
	if err := r.invoke(ctx, functionListActions, request, &result); err != nil {
		return nil, err
	}
	if err := core.ValidateActions(result); err != nil {
		return nil, fmt.Errorf("%w: list_actions result: %v", ErrContract, err)
	}
	return result, nil
}

// Start implements rules.Ruleset.
func (r *Ruleset) Start(ctx context.Context, request core.StartRequest) (core.Step, error) {
	if err := request.Validate(); err != nil {
		return core.Step{}, err
	}
	if err := r.validateSnapshot(request.Snapshot); err != nil {
		return core.Step{}, err
	}
	var result core.Step
	if err := r.invoke(ctx, functionStart, request, &result); err != nil {
		return core.Step{}, err
	}
	if err := result.Validate(); err != nil {
		return core.Step{}, fmt.Errorf("%w: start result: %v", ErrContract, err)
	}
	return result, nil
}

// Resume implements rules.Ruleset.
func (r *Ruleset) Resume(ctx context.Context, request core.ResumeRequest) (core.Step, error) {
	if err := request.Validate(); err != nil {
		return core.Step{}, err
	}
	if err := r.validateSnapshot(request.Snapshot); err != nil {
		return core.Step{}, err
	}
	var result core.Step
	if err := r.invoke(ctx, functionResume, request, &result); err != nil {
		return core.Step{}, err
	}
	if err := result.Validate(); err != nil {
		return core.Step{}, fmt.Errorf("%w: resume result: %v", ErrContract, err)
	}
	return result, nil
}

// Project implements rules.Ruleset.
func (r *Ruleset) Project(ctx context.Context, request core.ProjectRequest) (core.Projection, error) {
	if err := request.Validate(); err != nil {
		return core.Projection{}, err
	}
	if err := r.validateSnapshot(request.Snapshot); err != nil {
		return core.Projection{}, err
	}
	var result core.Projection
	if err := r.invoke(ctx, functionProject, request, &result); err != nil {
		return core.Projection{}, err
	}
	if err := result.Validate(); err != nil {
		return core.Projection{}, fmt.Errorf("%w: project result: %v", ErrContract, err)
	}
	return result, nil
}

// Explain implements rules.Ruleset.
func (r *Ruleset) Explain(ctx context.Context, request core.ExplainRequest) (core.Explanation, error) {
	if err := request.Validate(); err != nil {
		return core.Explanation{}, err
	}
	if err := r.validateSnapshot(request.Snapshot); err != nil {
		return core.Explanation{}, err
	}
	var result core.Explanation
	if err := r.invoke(ctx, functionExplain, request, &result); err != nil {
		return core.Explanation{}, err
	}
	if err := result.Validate(); err != nil {
		return core.Explanation{}, fmt.Errorf("%w: explain result: %v", ErrContract, err)
	}
	return result, nil
}

// ValidateState implements rules.Ruleset. The script returns None for a valid
// state or a non-empty diagnostic string for a ruleset-specific violation.
func (r *Ruleset) ValidateState(ctx context.Context, request core.ValidateStateRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	if err := r.validateSnapshot(request.Snapshot); err != nil {
		return err
	}
	argument, err := requestValue(request, r.limits)
	if err != nil {
		return fmt.Errorf("starlark rules: %s: %w", functionValidateState, err)
	}
	result, err := r.call(ctx, functionValidateState, argument)
	if err != nil {
		return err
	}
	switch result := result.(type) {
	case star.NoneType:
		return nil
	case star.String:
		message := string(result)
		if message == "" || strings.TrimSpace(message) != message || len(message) > core.MaxTextBytes {
			return fmt.Errorf("%w: validate_state diagnostic must be non-empty, trimmed, and at most %d bytes", ErrContract, core.MaxTextBytes)
		}
		return fmt.Errorf("starlark rules: invalid state: %s", message)
	default:
		return fmt.Errorf("%w: validate_state must return None or string, got %s", ErrContract, result.Type())
	}
}

// Reduce implements rules.Ruleset.
func (r *Ruleset) Reduce(ctx context.Context, request core.ReduceRequest) (core.ReduceResult, error) {
	if err := request.Validate(); err != nil {
		return core.ReduceResult{}, err
	}
	if err := r.validateSnapshot(request.Snapshot); err != nil {
		return core.ReduceResult{}, err
	}
	var result core.ReduceResult
	if err := r.invoke(ctx, functionReduce, request, &result); err != nil {
		return core.ReduceResult{}, err
	}
	if err := result.Validate(); err != nil {
		return core.ReduceResult{}, fmt.Errorf("%w: reduce result: %v", ErrContract, err)
	}
	return result, nil
}

// Migrate implements rules.Ruleset.
func (r *Ruleset) Migrate(ctx context.Context, request core.MigrateRequest) (core.MigrateResult, error) {
	if err := request.Validate(); err != nil {
		return core.MigrateResult{}, err
	}
	var result core.MigrateResult
	if err := r.invoke(ctx, functionMigrate, request, &result); err != nil {
		return core.MigrateResult{}, err
	}
	if err := result.Validate(); err != nil {
		return core.MigrateResult{}, fmt.Errorf("%w: migrate result: %v", ErrContract, err)
	}
	return result, nil
}

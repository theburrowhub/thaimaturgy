// Package ruleshost drives pure rulesets through host-owned operations. It is
// intentionally system-neutral: randomness is dispatched by method identifier,
// while state changes are accepted only through Emit -> Reduce.
package ruleshost

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/theburrowhub/thaimaturgy/internal/rules"
)

// RandomResolver performs one host-owned entropy request.
type RandomResolver interface {
	ResolveRandom(context.Context, rules.RandomRequest) (rules.Payload, error)
}

// RandomResolverFunc adapts a function to RandomResolver.
type RandomResolverFunc func(context.Context, rules.RandomRequest) (rules.Payload, error)

func (f RandomResolverFunc) ResolveRandom(ctx context.Context, request rules.RandomRequest) (rules.Payload, error) {
	return f(ctx, request)
}

// RandomMethodFunc handles the specification for one registered random method.
type RandomMethodFunc func(context.Context, rules.Payload) (rules.Payload, error)

// RandomDispatcher is a concurrency-safe, extensible random-method registry.
// It lets packages add dice, cards, tables, tokens, or other entropy sources
// without coupling the transaction driver to a particular RPG system.
type RandomDispatcher struct {
	mu      sync.RWMutex
	methods map[string]RandomMethodFunc
}

func NewRandomDispatcher() *RandomDispatcher {
	return &RandomDispatcher{methods: make(map[string]RandomMethodFunc)}
}

func (d *RandomDispatcher) Register(method string, resolver RandomMethodFunc) error {
	if d == nil {
		return errors.New("ruleshost: nil random dispatcher")
	}
	if method == "" || resolver == nil {
		return errors.New("ruleshost: random method and resolver are required")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, exists := d.methods[method]; exists {
		return fmt.Errorf("ruleshost: random method %q is already registered", method)
	}
	d.methods[method] = resolver
	return nil
}

func (d *RandomDispatcher) ResolveRandom(ctx context.Context, request rules.RandomRequest) (rules.Payload, error) {
	if d == nil {
		return rules.Payload{}, errors.New("ruleshost: no random dispatcher configured")
	}
	d.mu.RLock()
	resolver := d.methods[request.Method]
	d.mu.RUnlock()
	if resolver == nil {
		return rules.Payload{}, fmt.Errorf("ruleshost: unsupported random method %q", request.Method)
	}
	result, err := safeRandomMethod(ctx, resolver, request.Specification)
	if err != nil {
		return rules.Payload{}, err
	}
	if err := result.Validate(); err != nil {
		return rules.Payload{}, fmt.Errorf("ruleshost: invalid random result: %w", err)
	}
	return result, nil
}

// ReplayBatch is the persisted mechanical subset required to rebuild state.
type ReplayBatch struct {
	Sequence     uint32
	BaseRevision uint64
	Revision     uint64
	Events       []rules.Event
}

// Executor invokes one pure ruleset transition or one host operation at a time.
// It deliberately never calls Resume after randomness or Reduce: the caller
// must durably commit the response/PendingStep first, then invoke Resume in a
// fresh transaction.
type Executor struct {
	Ruleset rules.Ruleset
	Random  RandomResolver
}

// Start begins an intent and returns exactly its first step.
func (e Executor) Start(ctx context.Context, request rules.StartRequest) (rules.Step, error) {
	if ctx == nil {
		return rules.Step{}, errors.New("ruleshost: nil context")
	}
	if e.Ruleset == nil {
		return rules.Step{}, errors.New("ruleshost: nil ruleset")
	}
	if err := request.Validate(); err != nil {
		return rules.Step{}, err
	}
	step, err := safeStart(ctx, e.Ruleset, request)
	if err != nil {
		return rules.Step{}, fmt.Errorf("start rules intent: %w", err)
	}
	if err := step.Validate(); err != nil {
		return rules.Step{}, fmt.Errorf("ruleset returned an invalid step: %w", err)
	}
	return step, nil
}

// Resume answers one durably persisted pending step and returns exactly the next
// step. The response must already be committed by the host.
func (e Executor) Resume(ctx context.Context, snapshot rules.Snapshot, principal rules.Principal, pending rules.PendingStep, response rules.Payload) (rules.Step, error) {
	if ctx == nil {
		return rules.Step{}, errors.New("ruleshost: nil context")
	}
	if e.Ruleset == nil {
		return rules.Step{}, errors.New("ruleshost: nil ruleset")
	}
	hostResponse := rules.HostResponse{StepID: pending.StepID, Kind: pending.Kind, Data: response}
	request := rules.ResumeRequest{
		Snapshot: snapshot, Principal: principal, Pending: pending, Response: hostResponse,
	}
	if err := request.Validate(); err != nil {
		return rules.Step{}, err
	}
	step, err := safeResume(ctx, e.Ruleset, request)
	if err != nil {
		return rules.Step{}, fmt.Errorf("resume rules intent: %w", err)
	}
	if err := step.Validate(); err != nil {
		return rules.Step{}, fmt.Errorf("ruleset returned an invalid step: %w", err)
	}
	return step, nil
}

// ResolveRandom performs one generic random request without resuming the
// ruleset. The caller audits and commits the result first.
func (e Executor) ResolveRandom(ctx context.Context, request rules.RandomRequest) (rules.Payload, error) {
	if ctx == nil {
		return rules.Payload{}, errors.New("ruleshost: nil context")
	}
	if e.Random == nil {
		return rules.Payload{}, errors.New("ruleshost: ruleset requested randomness but no resolver is configured")
	}
	result, err := e.Random.ResolveRandom(ctx, request)
	if err != nil {
		return rules.Payload{}, fmt.Errorf("perform random draw: %w", err)
	}
	if err := result.Validate(); err != nil {
		return rules.Payload{}, fmt.Errorf("perform random draw: invalid response: %w", err)
	}
	return result, nil
}

// ValidateState invokes the package validator behind the same panic boundary
// used by transitions.
func (e Executor) ValidateState(ctx context.Context, snapshot rules.Snapshot) error {
	if ctx == nil {
		return errors.New("ruleshost: nil context")
	}
	if e.Ruleset == nil {
		return errors.New("ruleshost: nil ruleset")
	}
	request := rules.ValidateStateRequest{Snapshot: snapshot}
	if err := request.Validate(); err != nil {
		return err
	}
	if err := safeValidateState(ctx, e.Ruleset, request); err != nil {
		return fmt.Errorf("validate rules state: %w", err)
	}
	return nil
}

// Project invokes a package's authority-filtered projection safely.
func (e Executor) Project(ctx context.Context, request rules.ProjectRequest) (rules.Projection, error) {
	if ctx == nil || e.Ruleset == nil {
		return rules.Projection{}, errors.New("ruleshost: project requires context and ruleset")
	}
	if err := request.Validate(); err != nil {
		return rules.Projection{}, err
	}
	projection, err := safeProject(ctx, e.Ruleset, request)
	if err != nil {
		return rules.Projection{}, fmt.Errorf("project rules state: %w", err)
	}
	if err := projection.Validate(); err != nil {
		return rules.Projection{}, fmt.Errorf("ruleset returned invalid projection: %w", err)
	}
	return projection, nil
}

// ListActions invokes and validates a package catalog safely.
func (e Executor) ListActions(ctx context.Context, request rules.CatalogRequest) ([]rules.ActionDescriptor, error) {
	if ctx == nil || e.Ruleset == nil {
		return nil, errors.New("ruleshost: list actions requires context and ruleset")
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	actions, err := safeListActions(ctx, e.Ruleset, request)
	if err != nil {
		return nil, fmt.Errorf("list rules actions: %w", err)
	}
	if err := rules.ValidateActions(actions); err != nil {
		return nil, fmt.Errorf("ruleset returned invalid action catalog: %w", err)
	}
	return actions, nil
}

// Explain invokes and validates package-supplied explanatory text safely.
func (e Executor) Explain(ctx context.Context, request rules.ExplainRequest) (rules.Explanation, error) {
	if ctx == nil || e.Ruleset == nil {
		return rules.Explanation{}, errors.New("ruleshost: explain requires context and ruleset")
	}
	if err := request.Validate(); err != nil {
		return rules.Explanation{}, err
	}
	explanation, err := safeExplain(ctx, e.Ruleset, request)
	if err != nil {
		return rules.Explanation{}, fmt.Errorf("explain rule: %w", err)
	}
	if err := explanation.Validate(); err != nil {
		return rules.Explanation{}, fmt.Errorf("ruleset returned invalid explanation: %w", err)
	}
	return explanation, nil
}

// Reduce applies one emission and validates the candidate snapshot without
// resuming the ruleset. The caller commits it before acknowledging the step.
func (e Executor) Reduce(ctx context.Context, snapshot rules.Snapshot, emission rules.Emission) (rules.Snapshot, rules.Payload, error) {
	if ctx == nil {
		return rules.Snapshot{}, rules.Payload{}, errors.New("ruleshost: nil context")
	}
	if e.Ruleset == nil {
		return rules.Snapshot{}, rules.Payload{}, errors.New("ruleshost: nil ruleset")
	}
	request := rules.ReduceRequest{Snapshot: snapshot, Events: cloneEvents(emission.Events)}
	if err := request.Validate(); err != nil {
		return rules.Snapshot{}, rules.Payload{}, err
	}
	reduced, err := safeReduce(ctx, e.Ruleset, request)
	if err != nil {
		return rules.Snapshot{}, rules.Payload{}, fmt.Errorf("reduce rules event batch: %w", err)
	}
	if err := reduced.Validate(); err != nil {
		return rules.Snapshot{}, rules.Payload{}, fmt.Errorf("ruleset returned invalid reduced state: %w", err)
	}
	candidate := rules.Snapshot{Ruleset: snapshot.Ruleset, Revision: snapshot.Revision + 1, State: reduced.State}
	if err := safeValidateState(ctx, e.Ruleset, rules.ValidateStateRequest{Snapshot: candidate}); err != nil {
		return rules.Snapshot{}, rules.Payload{}, fmt.Errorf("validate reduced rules state: %w", err)
	}
	ack, err := rules.PayloadFrom(struct {
		BaseRevision uint64 `json:"base_revision"`
		Revision     uint64 `json:"revision"`
	}{BaseRevision: snapshot.Revision, Revision: candidate.Revision})
	if err != nil {
		return rules.Snapshot{}, rules.Payload{}, fmt.Errorf("encode emission acknowledgement: %w", err)
	}
	return candidate, ack, nil
}

// PublicRequest returns the authority-visible request for a nonterminal result
// without exposing its continuation.
func PublicRequest(step rules.Step) (rules.Payload, error) {
	if err := step.Validate(); err != nil {
		return rules.Payload{}, err
	}
	switch step.Kind {
	case rules.StepKindNeedDecision:
		return rules.PayloadFrom(step.NeedDecision)
	case rules.StepKindNeedAdjudication:
		return rules.PayloadFrom(step.NeedAdjudication)
	default:
		return rules.Payload{}, fmt.Errorf("ruleshost: step %q does not require an external response", step.Kind)
	}
}

// Replay reconstructs a materialized snapshot from its immutable revision-zero
// state and ordered event batches. It rejects gaps, reordering, malformed
// events, reducer failures, and invalid reconstructed states.
func Replay(ctx context.Context, implementation rules.Ruleset, lock rules.Lock, initial rules.Payload, batches []ReplayBatch) (rules.Snapshot, error) {
	if ctx == nil {
		return rules.Snapshot{}, errors.New("ruleshost: nil replay context")
	}
	if implementation == nil {
		return rules.Snapshot{}, errors.New("ruleshost: nil replay ruleset")
	}
	snapshot := rules.Snapshot{Ruleset: lock, State: initial}
	if err := snapshot.Validate(); err != nil {
		return rules.Snapshot{}, fmt.Errorf("ruleshost: invalid replay root: %w", err)
	}
	if err := safeValidateState(ctx, implementation, rules.ValidateStateRequest{Snapshot: snapshot}); err != nil {
		return rules.Snapshot{}, fmt.Errorf("ruleshost: invalid replay root state: %w", err)
	}
	for i, batch := range batches {
		if batch.Sequence != uint32(i+1) {
			return rules.Snapshot{}, fmt.Errorf("ruleshost: replay batch %d has sequence %d, expected %d", i, batch.Sequence, i+1)
		}
		if batch.BaseRevision != snapshot.Revision {
			return rules.Snapshot{}, fmt.Errorf("ruleshost: replay batch %d starts at revision %d, expected %d", i, batch.BaseRevision, snapshot.Revision)
		}
		if batch.Revision != batch.BaseRevision+1 {
			return rules.Snapshot{}, fmt.Errorf("ruleshost: replay batch %d must advance exactly one revision", i)
		}
		request := rules.ReduceRequest{Snapshot: snapshot, Events: cloneEvents(batch.Events)}
		if err := request.Validate(); err != nil {
			return rules.Snapshot{}, fmt.Errorf("ruleshost: invalid replay batch %d: %w", i, err)
		}
		reduced, err := safeReduce(ctx, implementation, request)
		if err != nil {
			return rules.Snapshot{}, fmt.Errorf("ruleshost: replay batch %d reduce: %w", i, err)
		}
		if err := reduced.Validate(); err != nil {
			return rules.Snapshot{}, fmt.Errorf("ruleshost: replay batch %d returned invalid state: %w", i, err)
		}
		candidate := rules.Snapshot{Ruleset: lock, Revision: batch.Revision, State: reduced.State}
		if err := safeValidateState(ctx, implementation, rules.ValidateStateRequest{Snapshot: candidate}); err != nil {
			return rules.Snapshot{}, fmt.Errorf("ruleshost: replay batch %d state validation: %w", i, err)
		}
		snapshot = candidate
	}
	return snapshot, nil
}

func cloneEvents(events []rules.Event) []rules.Event {
	return append([]rules.Event(nil), events...)
}

func safeStart(ctx context.Context, implementation rules.Ruleset, request rules.StartRequest) (step rules.Step, err error) {
	defer rulesPanicAsError("Start", &err)
	return implementation.Start(ctx, request)
}

func safeResume(ctx context.Context, implementation rules.Ruleset, request rules.ResumeRequest) (step rules.Step, err error) {
	defer rulesPanicAsError("Resume", &err)
	return implementation.Resume(ctx, request)
}

func safeReduce(ctx context.Context, implementation rules.Ruleset, request rules.ReduceRequest) (result rules.ReduceResult, err error) {
	defer rulesPanicAsError("Reduce", &err)
	return implementation.Reduce(ctx, request)
}

func safeValidateState(ctx context.Context, implementation rules.Ruleset, request rules.ValidateStateRequest) (err error) {
	defer rulesPanicAsError("ValidateState", &err)
	return implementation.ValidateState(ctx, request)
}

func safeProject(ctx context.Context, implementation rules.Ruleset, request rules.ProjectRequest) (result rules.Projection, err error) {
	defer rulesPanicAsError("Project", &err)
	return implementation.Project(ctx, request)
}

func safeListActions(ctx context.Context, implementation rules.Ruleset, request rules.CatalogRequest) (result []rules.ActionDescriptor, err error) {
	defer rulesPanicAsError("ListActions", &err)
	return implementation.ListActions(ctx, request)
}

func safeExplain(ctx context.Context, implementation rules.Ruleset, request rules.ExplainRequest) (result rules.Explanation, err error) {
	defer rulesPanicAsError("Explain", &err)
	return implementation.Explain(ctx, request)
}

func rulesPanicAsError(operation string, target *error) {
	if recovered := recover(); recovered != nil {
		*target = fmt.Errorf("ruleshost: ruleset %s panicked: %v", operation, recovered)
	}
}

func safeRandomMethod(ctx context.Context, resolver RandomMethodFunc, specification rules.Payload) (result rules.Payload, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("ruleshost: random resolver panicked: %v", recovered)
		}
	}()
	return resolver(ctx, specification)
}

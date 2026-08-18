package rules

import "fmt"

// StepKind identifies the next host operation in a rules resolution.
type StepKind string

const (
	StepKindReject           StepKind = "reject"
	StepKindNeedRandom       StepKind = "need_random"
	StepKindNeedDecision     StepKind = "need_decision"
	StepKindNeedAdjudication StepKind = "need_adjudication"
	StepKindStartChild       StepKind = "start_child"
	StepKindEmit             StepKind = "emit"
	StepKindComplete         StepKind = "complete"
)

// Rejection is a terminal, expected refusal of an intent. Execution failures
// are returned as Go errors instead.
type Rejection struct {
	Code    string  `json:"code"`
	Message string  `json:"message"`
	Details Payload `json:"details,omitempty"`
}

func (r Rejection) validate() error {
	if err := validateIdentifier("reject.code", r.Code); err != nil {
		return err
	}
	if err := validateText("reject.message", r.Message, true); err != nil {
		return err
	}
	return validateOptionalPayload("reject.details", r.Details)
}

// RandomRequest asks the host for an auditable random result. Method is a
// host/ruleset contract identifier; Specification carries its neutral data, so
// the kernel assumes neither dice nor any other random device.
type RandomRequest struct {
	Method        string  `json:"method"`
	Specification Payload `json:"specification"`
}

func (r RandomRequest) validate() error {
	if err := validateIdentifier("random.method", r.Method); err != nil {
		return err
	}
	return validateRequiredPayload("random.specification", r.Specification)
}

// DecisionOption is one closed choice in a DecisionRequest. Value is optional
// system-specific data; ID alone can be sufficient.
type DecisionOption struct {
	ID    string  `json:"id"`
	Label string  `json:"label"`
	Value Payload `json:"value,omitempty"`
}

func (o DecisionOption) validate() error {
	if err := validateIdentifier("decision.option.id", o.ID); err != nil {
		return err
	}
	if err := validateText("decision.option.label", o.Label, true); err != nil {
		return err
	}
	return validateOptionalPayload("decision.option.value", o.Value)
}

// DecisionRequest asks an authority for a mechanical choice. A decision is
// closed when Options is non-empty and open when Schema is present; at least one
// of them is required.
type DecisionRequest struct {
	Authority string           `json:"authority"`
	Prompt    string           `json:"prompt"`
	Options   []DecisionOption `json:"options,omitempty"`
	Schema    Payload          `json:"schema,omitempty"`
}

func (r DecisionRequest) validate() error {
	if err := validateOpaqueID("decision.authority", r.Authority); err != nil {
		return err
	}
	if err := validateText("decision.prompt", r.Prompt, true); err != nil {
		return err
	}
	if len(r.Options) == 0 && r.Schema.IsZero() {
		return invalid("decision", "must provide options or a schema")
	}
	if len(r.Options) > MaxCollectionItems {
		return invalid("decision.options", "contains more than %d items", MaxCollectionItems)
	}
	seen := make(map[string]struct{}, len(r.Options))
	for i, option := range r.Options {
		if err := option.validate(); err != nil {
			return invalid(fmt.Sprintf("decision.options[%d]", i), "%v", err)
		}
		if _, exists := seen[option.ID]; exists {
			return invalid(fmt.Sprintf("decision.options[%d].id", i), "duplicates %q", option.ID)
		}
		seen[option.ID] = struct{}{}
	}
	return validateJSONObject("decision.schema", r.Schema, false)
}

// AdjudicationRequest asks a designated authority to settle an ambiguity that
// the rules package cannot decide mechanically.
type AdjudicationRequest struct {
	Authority string  `json:"authority"`
	Prompt    string  `json:"prompt"`
	Schema    Payload `json:"schema,omitempty"`
}

func (r AdjudicationRequest) validate() error {
	if err := validateOpaqueID("adjudication.authority", r.Authority); err != nil {
		return err
	}
	if err := validateText("adjudication.prompt", r.Prompt, true); err != nil {
		return err
	}
	return validateJSONObject("adjudication.schema", r.Schema, false)
}

// ChildRequest starts a nested resolution in an exactly pinned rules package.
// The host supplies that package's current snapshot when it executes the step.
type ChildRequest struct {
	Ruleset Lock   `json:"ruleset"`
	Intent  Intent `json:"intent"`
}

func (r ChildRequest) validate() error {
	if err := r.Ruleset.Validate(); err != nil {
		return err
	}
	return r.Intent.Validate()
}

// Event is a rules-owned state transition proposed to the host.
type Event struct {
	Type          string  `json:"type"`
	SchemaVersion uint32  `json:"schema_version"`
	Data          Payload `json:"data"`
}

// Validate checks the event envelope. Event semantics remain ruleset-owned.
func (e Event) Validate() error {
	if err := validateIdentifier("event.type", e.Type); err != nil {
		return err
	}
	if e.SchemaVersion == 0 {
		return invalid("event.schema_version", "must be greater than zero")
	}
	return validateRequiredPayload("event.data", e.Data)
}

// Emission is an atomic, ordered batch of proposed events. The host validates
// and commits it before resuming the resolution.
type Emission struct {
	Events []Event `json:"events"`
}

func (e Emission) validate() error {
	if len(e.Events) == 0 {
		return invalid("emit.events", "must not be empty")
	}
	if len(e.Events) > MaxCollectionItems {
		return invalid("emit.events", "contains more than %d items", MaxCollectionItems)
	}
	for i, event := range e.Events {
		if err := event.Validate(); err != nil {
			return invalid(fmt.Sprintf("emit.events[%d]", i), "%v", err)
		}
	}
	return nil
}

// Completion is the terminal structured result of a resolution.
type Completion struct {
	Outcome string  `json:"outcome"`
	Result  Payload `json:"result"`
}

func (c Completion) validate() error {
	if err := validateIdentifier("complete.outcome", c.Outcome); err != nil {
		return err
	}
	return validateRequiredPayload("complete.result", c.Result)
}

// Step is a tagged union. Exactly one variant pointer must be non-nil and must
// correspond to Kind. Every resumable step carries an opaque Continuation owned
// by the ruleset. The host persists it with the pending resolution; no live VM,
// closure, or in-memory ruleset state is needed to resume after a restart.
type Step struct {
	ID               string               `json:"id"`
	Kind             StepKind             `json:"kind"`
	Continuation     Payload              `json:"continuation,omitempty"`
	Reject           *Rejection           `json:"reject,omitempty"`
	NeedRandom       *RandomRequest       `json:"need_random,omitempty"`
	NeedDecision     *DecisionRequest     `json:"need_decision,omitempty"`
	NeedAdjudication *AdjudicationRequest `json:"need_adjudication,omitempty"`
	StartChild       *ChildRequest        `json:"start_child,omitempty"`
	Emit             *Emission            `json:"emit,omitempty"`
	Complete         *Completion          `json:"complete,omitempty"`
}

// Validate checks the tagged-union invariant and the selected variant.
func (s Step) Validate() error {
	if err := validateOpaqueID("step.id", s.ID); err != nil {
		return err
	}
	variants := 0
	for _, present := range []bool{
		s.Reject != nil,
		s.NeedRandom != nil,
		s.NeedDecision != nil,
		s.NeedAdjudication != nil,
		s.StartChild != nil,
		s.Emit != nil,
		s.Complete != nil,
	} {
		if present {
			variants++
		}
	}
	if variants != 1 {
		return invalid("step", "must contain exactly one variant, got %d", variants)
	}
	if s.Terminal() {
		if !s.Continuation.IsZero() {
			return invalid("step.continuation", "must be omitted for a terminal step")
		}
	} else if s.NeedsResponse() {
		if err := validateRequiredPayload("step.continuation", s.Continuation); err != nil {
			return err
		}
	} else {
		return invalid("step.kind", "unknown value %q", s.Kind)
	}

	switch s.Kind {
	case StepKindReject:
		if s.Reject == nil {
			return invalid("step.kind", "does not match the populated variant")
		}
		return s.Reject.validate()
	case StepKindNeedRandom:
		if s.NeedRandom == nil {
			return invalid("step.kind", "does not match the populated variant")
		}
		return s.NeedRandom.validate()
	case StepKindNeedDecision:
		if s.NeedDecision == nil {
			return invalid("step.kind", "does not match the populated variant")
		}
		return s.NeedDecision.validate()
	case StepKindNeedAdjudication:
		if s.NeedAdjudication == nil {
			return invalid("step.kind", "does not match the populated variant")
		}
		return s.NeedAdjudication.validate()
	case StepKindStartChild:
		if s.StartChild == nil {
			return invalid("step.kind", "does not match the populated variant")
		}
		return s.StartChild.validate()
	case StepKindEmit:
		if s.Emit == nil {
			return invalid("step.kind", "does not match the populated variant")
		}
		return s.Emit.validate()
	case StepKindComplete:
		if s.Complete == nil {
			return invalid("step.kind", "does not match the populated variant")
		}
		return s.Complete.validate()
	default:
		return invalid("step.kind", "unknown value %q", s.Kind)
	}
}

// Terminal reports whether no response may follow this step.
func (s Step) Terminal() bool {
	return s.Kind == StepKindReject || s.Kind == StepKindComplete
}

// NeedsResponse reports whether the host resumes the ruleset after executing
// this step.
func (s Step) NeedsResponse() bool {
	switch s.Kind {
	case StepKindNeedRandom, StepKindNeedDecision, StepKindNeedAdjudication, StepKindStartChild, StepKindEmit:
		return true
	default:
		return false
	}
}

// PendingStep is the minimal persistible continuation of a non-terminal Step.
// State is ruleset-owned and explicitly bound to the originating ID and kind.
type PendingStep struct {
	StepID string   `json:"step_id"`
	Kind   StepKind `json:"kind"`
	State  Payload  `json:"state"`
}

// Validate checks that pending state belongs to a resumable step kind.
func (p PendingStep) Validate() error {
	if err := validateOpaqueID("pending.step_id", p.StepID); err != nil {
		return err
	}
	if !isResumableStepKind(p.Kind) {
		if p.Kind == StepKindReject || p.Kind == StepKindComplete {
			return invalid("pending.kind", "%q is terminal", p.Kind)
		}
		return invalid("pending.kind", "unknown value %q", p.Kind)
	}
	return validateRequiredPayload("pending.state", p.State)
}

// Pending returns the persistible continuation for a valid resumable Step.
func (s Step) Pending() (PendingStep, error) {
	if err := s.Validate(); err != nil {
		return PendingStep{}, err
	}
	if !s.NeedsResponse() {
		return PendingStep{}, invalid("step.kind", "%q is terminal", s.Kind)
	}
	return PendingStep{StepID: s.ID, Kind: s.Kind, State: s.Continuation}, nil
}

// HostResponse contains the result of the host operation requested by a
// non-terminal Step. Kind repeats the originating step kind to prevent
// response-type confusion. It is distinct from the ruleset-owned continuation.
type HostResponse struct {
	StepID string   `json:"step_id"`
	Kind   StepKind `json:"kind"`
	Data   Payload  `json:"data"`
}

// Validate checks that a response answers a resumable step kind.
func (r HostResponse) Validate() error {
	if err := validateOpaqueID("response.step_id", r.StepID); err != nil {
		return err
	}
	if !isResumableStepKind(r.Kind) {
		if r.Kind == StepKindReject || r.Kind == StepKindComplete {
			return invalid("response.kind", "%q is terminal", r.Kind)
		}
		return invalid("response.kind", "unknown value %q", r.Kind)
	}
	return validateRequiredPayload("response.data", r.Data)
}

func isResumableStepKind(kind StepKind) bool {
	switch kind {
	case StepKindNeedRandom, StepKindNeedDecision, StepKindNeedAdjudication, StepKindStartChild, StepKindEmit:
		return true
	default:
		return false
	}
}

// ResumeRequest is the complete input to Ruleset.Resume. Snapshot is the state
// visible after the host performed the previous step (and committed emissions).
// Pending is persisted ruleset state; Response is newly supplied host data.
// Their StepID and Kind must match, preventing cross-resolution responses.
type ResumeRequest struct {
	Snapshot  Snapshot     `json:"snapshot"`
	Principal Principal    `json:"principal"`
	Pending   PendingStep  `json:"pending"`
	Response  HostResponse `json:"response"`
}

// Validate checks the request envelope.
func (r ResumeRequest) Validate() error {
	if err := r.Snapshot.Validate(); err != nil {
		return err
	}
	if err := r.Principal.Validate(); err != nil {
		return err
	}
	if err := r.Pending.Validate(); err != nil {
		return err
	}
	if err := r.Response.Validate(); err != nil {
		return err
	}
	if r.Pending.StepID != r.Response.StepID {
		return invalid("resume.response.step_id", "does not match pending step %q", r.Pending.StepID)
	}
	if r.Pending.Kind != r.Response.Kind {
		return invalid("resume.response.kind", "does not match pending kind %q", r.Pending.Kind)
	}
	return nil
}

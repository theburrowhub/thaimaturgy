package rules

import "fmt"

// ProjectRequest asks for the only rules state view that may be exposed to a
// principal. The ruleset is responsible for omitting unauthorized information.
type ProjectRequest struct {
	Snapshot  Snapshot  `json:"snapshot"`
	Principal Principal `json:"principal"`
}

// Validate checks the projection request envelope.
func (r ProjectRequest) Validate() error {
	if err := r.Snapshot.Validate(); err != nil {
		return err
	}
	return r.Principal.Validate()
}

// Projection is an authorized, ruleset-defined view of state.
type Projection struct {
	View Payload `json:"view"`
}

// Validate checks the projection envelope.
func (p Projection) Validate() error {
	return validateRequiredPayload("projection.view", p.View)
}

// ExplainRequest asks for a localized explanation of a rule reference as
// visible to one principal at a particular snapshot.
type ExplainRequest struct {
	Snapshot  Snapshot  `json:"snapshot"`
	Principal Principal `json:"principal"`
	Reference string    `json:"reference"`
	Locale    string    `json:"locale"`
}

// Validate checks the explanation request envelope.
func (r ExplainRequest) Validate() error {
	if err := r.Snapshot.Validate(); err != nil {
		return err
	}
	if err := r.Principal.Validate(); err != nil {
		return err
	}
	if err := validateIdentifier("explain.reference", r.Reference); err != nil {
		return err
	}
	return validateOpaqueID("explain.locale", r.Locale)
}

// Explanation is human-readable rules text plus optional structured metadata.
type Explanation struct {
	Text string  `json:"text"`
	Data Payload `json:"data,omitempty"`
}

// Validate checks the explanation envelope.
func (e Explanation) Validate() error {
	if err := validateText("explanation.text", e.Text, true); err != nil {
		return err
	}
	return validateOptionalPayload("explanation.data", e.Data)
}

// ValidateStateRequest asks the ruleset to verify all invariants in a snapshot.
type ValidateStateRequest struct {
	Snapshot Snapshot `json:"snapshot"`
}

// Validate checks the request envelope, not ruleset-specific invariants.
func (r ValidateStateRequest) Validate() error { return r.Snapshot.Validate() }

// ReduceRequest asks the ruleset to apply an ordered event batch to a snapshot
// without mutating it. The host owns revisions and persistence.
type ReduceRequest struct {
	Snapshot Snapshot `json:"snapshot"`
	Events   []Event  `json:"events"`
}

// Validate checks the reducer input envelope.
func (r ReduceRequest) Validate() error {
	if err := r.Snapshot.Validate(); err != nil {
		return err
	}
	if len(r.Events) == 0 {
		return invalid("reduce.events", "must not be empty")
	}
	if len(r.Events) > MaxCollectionItems {
		return invalid("reduce.events", "contains more than %d items", MaxCollectionItems)
	}
	for i, event := range r.Events {
		if err := event.Validate(); err != nil {
			return invalid(fmt.Sprintf("reduce.events[%d]", i), "%v", err)
		}
	}
	return nil
}

// ReduceResult contains the new rules-owned state after applying events.
type ReduceResult struct {
	State Payload `json:"state"`
}

// Validate checks the reducer result envelope.
func (r ReduceResult) Validate() error {
	return validateRequiredPayload("reduce_result.state", r.State)
}

// MigrateRequest contains state from an exact older artifact. The target is the
// artifact whose Ruleset receives the request.
type MigrateRequest struct {
	From  Lock    `json:"from"`
	State Payload `json:"state"`
}

// Validate checks the migration input envelope.
func (r MigrateRequest) Validate() error {
	if err := r.From.Validate(); err != nil {
		return err
	}
	return validateRequiredPayload("migrate.state", r.State)
}

// MigrateResult is rules-owned state valid for the target artifact.
type MigrateResult struct {
	State Payload `json:"state"`
}

// Validate checks the migration result envelope.
func (r MigrateResult) Validate() error {
	return validateRequiredPayload("migrate_result.state", r.State)
}

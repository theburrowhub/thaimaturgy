package rules

import "fmt"

// Principal identifies the authority asking the ruleset to act. Kind and Roles
// are host-defined identifiers; the core does not assume players, game masters,
// or any other particular table model.
type Principal struct {
	ID    string   `json:"id"`
	Kind  string   `json:"kind"`
	Roles []string `json:"roles,omitempty"`
}

// Validate checks the principal's structural limits.
func (p Principal) Validate() error {
	if err := validateOpaqueID("principal.id", p.ID); err != nil {
		return err
	}
	if err := validateIdentifier("principal.kind", p.Kind); err != nil {
		return err
	}
	return validateUniqueIdentifiers("principal.roles", p.Roles)
}

// Snapshot is the complete rules-visible state at one host revision. State is
// opaque to the host-facing protocol and belongs to the locked ruleset.
type Snapshot struct {
	Ruleset  Lock    `json:"ruleset"`
	Revision uint64  `json:"revision"`
	State    Payload `json:"state"`
}

// Validate checks the snapshot identity and state encoding.
func (s Snapshot) Validate() error {
	if err := s.Ruleset.Validate(); err != nil {
		return err
	}
	return validateRequiredPayload("snapshot.state", s.State)
}

// ActionDescriptor describes an action currently offered by a ruleset.
// InputSchema is a JSON object interpreted by the caller, normally a JSON
// Schema. Annotations are optional, ruleset-specific metadata.
type ActionDescriptor struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Description string   `json:"description,omitempty"`
	InputSchema Payload  `json:"input_schema"`
	Tags        []string `json:"tags,omitempty"`
	Annotations Payload  `json:"annotations,omitempty"`
}

// Validate checks the descriptor without assigning semantics to its schema.
func (a ActionDescriptor) Validate() error {
	if err := validateIdentifier("action.id", a.ID); err != nil {
		return err
	}
	if err := validateText("action.label", a.Label, true); err != nil {
		return err
	}
	if err := validateText("action.description", a.Description, false); err != nil {
		return err
	}
	if err := validateJSONObject("action.input_schema", a.InputSchema, true); err != nil {
		return err
	}
	if err := validateUniqueIdentifiers("action.tags", a.Tags); err != nil {
		return err
	}
	return validateJSONObject("action.annotations", a.Annotations, false)
}

// ValidateActions validates a bounded action catalog and rejects duplicate IDs.
func ValidateActions(actions []ActionDescriptor) error {
	if len(actions) > MaxCollectionItems {
		return invalid("actions", "contains more than %d items", MaxCollectionItems)
	}
	seen := make(map[string]struct{}, len(actions))
	for i, action := range actions {
		if err := action.Validate(); err != nil {
			return invalid(fmt.Sprintf("actions[%d]", i), "%v", err)
		}
		if _, exists := seen[action.ID]; exists {
			return invalid(fmt.Sprintf("actions[%d].id", i), "duplicates %q", action.ID)
		}
		seen[action.ID] = struct{}{}
	}
	return nil
}

// Intent asks a ruleset to begin one advertised action. ID is a host-generated
// idempotency/correlation identifier. ActorID is optional because not every
// system models actions as belonging to an actor.
type Intent struct {
	ID        string  `json:"id"`
	ActionID  string  `json:"action_id"`
	ActorID   string  `json:"actor_id,omitempty"`
	Arguments Payload `json:"arguments"`
}

// Validate checks the intent's transport structure. Rulesets remain
// responsible for checking action availability and argument semantics.
func (i Intent) Validate() error {
	if err := validateOpaqueID("intent.id", i.ID); err != nil {
		return err
	}
	if err := validateIdentifier("intent.action_id", i.ActionID); err != nil {
		return err
	}
	if i.ActorID != "" {
		if err := validateOpaqueID("intent.actor_id", i.ActorID); err != nil {
			return err
		}
	}
	return validateRequiredPayload("intent.arguments", i.Arguments)
}

// CatalogRequest is the complete input to Ruleset.ListActions.
type CatalogRequest struct {
	Snapshot  Snapshot  `json:"snapshot"`
	Principal Principal `json:"principal"`
}

// Validate checks the request envelope.
func (r CatalogRequest) Validate() error {
	if err := r.Snapshot.Validate(); err != nil {
		return err
	}
	return r.Principal.Validate()
}

// StartRequest is the complete input to Ruleset.Start.
type StartRequest struct {
	Snapshot  Snapshot  `json:"snapshot"`
	Principal Principal `json:"principal"`
	Intent    Intent    `json:"intent"`
}

// Validate checks the request envelope.
func (r StartRequest) Validate() error {
	if err := r.Snapshot.Validate(); err != nil {
		return err
	}
	if err := r.Principal.Validate(); err != nil {
		return err
	}
	return r.Intent.Validate()
}

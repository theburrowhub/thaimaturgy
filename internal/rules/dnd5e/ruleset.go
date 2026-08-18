package dnd5e

import (
	"context"
	"fmt"

	"github.com/theburrowhub/thaimaturgy/internal/diceexpr"
	core "github.com/theburrowhub/thaimaturgy/internal/rules"
)

var _ core.Ruleset = (*Ruleset)(nil)

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("dnd5e: nil context")
	}
	return ctx.Err()
}

// Manifest implements rules.Ruleset.
func (r *Ruleset) Manifest(ctx context.Context) (core.Manifest, error) {
	if err := checkContext(ctx); err != nil {
		return core.Manifest{}, err
	}
	return packageManifest(), nil
}

// ListActions implements rules.Ruleset.
func (r *Ruleset) ListActions(ctx context.Context, request core.CatalogRequest) ([]core.ActionDescriptor, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	if err := validateSnapshot(request.Snapshot); err != nil {
		return nil, err
	}
	actions, err := actionDescriptors()
	if err != nil {
		return nil, err
	}
	if err := core.ValidateActions(actions); err != nil {
		return nil, err
	}
	return actions, nil
}

// Start implements rules.Ruleset.
func (r *Ruleset) Start(ctx context.Context, request core.StartRequest) (core.Step, error) {
	if err := checkContext(ctx); err != nil {
		return core.Step{}, err
	}
	if err := request.Validate(); err != nil {
		return core.Step{}, err
	}
	if err := validateSnapshot(request.Snapshot); err != nil {
		return core.Step{}, err
	}

	var step core.Step
	var err error
	switch request.Intent.ActionID {
	case ActionDiceRoll:
		step, err = r.startDiceRoll(request)
	case ActionAbilityCheck:
		step, err = r.startAbilityCheck(request)
	default:
		step = rejection(request.Intent.ID, "unknown.action", "unknown action: "+request.Intent.ActionID)
	}
	if err != nil {
		return core.Step{}, err
	}
	if err := step.Validate(); err != nil {
		return core.Step{}, err
	}
	return step, nil
}

// Resume implements rules.Ruleset.
func (r *Ruleset) Resume(ctx context.Context, request core.ResumeRequest) (core.Step, error) {
	if err := checkContext(ctx); err != nil {
		return core.Step{}, err
	}
	if err := request.Validate(); err != nil {
		return core.Step{}, err
	}
	if err := validateSnapshot(request.Snapshot); err != nil {
		return core.Step{}, err
	}
	if request.Pending.Kind != core.StepKindNeedRandom {
		return core.Step{}, fmt.Errorf("dnd5e: cannot resume step kind %q", request.Pending.Kind)
	}

	state, err := decodeContinuation(request.Pending.State)
	if err != nil {
		return core.Step{}, fmt.Errorf("dnd5e: decode continuation: %w", err)
	}
	var random DiceRandomResponse
	if err := decodePayload(request.Response.Data, &random); err != nil {
		return core.Step{}, fmt.Errorf("dnd5e: decode random response: %w", err)
	}

	var step core.Step
	switch state.Action {
	case ActionDiceRoll:
		step, err = completeDiceRoll(request.Pending.StepID, state, random.Rolls)
	case ActionAbilityCheck:
		step, err = completeAbilityCheck(request.Pending.StepID, state, random.Rolls)
	default:
		return core.Step{}, fmt.Errorf("dnd5e: continuation contains unknown action %q", state.Action)
	}
	if err != nil {
		return core.Step{}, err
	}
	if err := step.Validate(); err != nil {
		return core.Step{}, err
	}
	return step, nil
}

func completeDiceRoll(id string, state continuation, rolls []int) (core.Step, error) {
	expression, err := diceexpr.Parse(state.Notation)
	if err != nil {
		return core.Step{}, fmt.Errorf("dnd5e: invalid dice continuation: %w", err)
	}
	total, err := expression.Total(rolls)
	if err != nil {
		return core.Step{}, fmt.Errorf("dnd5e: invalid random response: %w", err)
	}
	formatted, err := expression.ResultString(rolls)
	if err != nil {
		return core.Step{}, err
	}
	content := fmt.Sprintf("Rolled %s: %s", expression.String(), formatted)
	traits := CriticalTraits{
		CriticalHit:  expression.IsCriticalHit(rolls),
		CriticalFail: expression.IsCriticalFail(rolls),
	}
	if traits.CriticalHit {
		content += " [CRIT!]"
	} else if traits.CriticalFail {
		content += " [FUMBLE!]"
	}
	logMessage := content
	if state.Reason != "" {
		logMessage = state.Reason + " — " + content
	}
	result, err := payloadFrom(DiceRollResult{
		Notation: expression.String(),
		Rolls:    append([]int(nil), rolls...),
		Modifier: expression.Modifier,
		Total:    total,
		Traits:   traits,
		Legacy: LegacyResult{
			Content:    content,
			LogMessage: logMessage,
			LogType:    "roll",
		},
	})
	if err != nil {
		return core.Step{}, err
	}
	return completion(id, "dnd5e.dice.rolled", result), nil
}

func completeAbilityCheck(id string, state continuation, rolls []int) (core.Step, error) {
	expression, err := diceexpr.Parse("1d20")
	if err != nil {
		return core.Step{}, err
	}
	expression.Modifier = *state.Modifier
	total, err := expression.Total(rolls)
	if err != nil {
		return core.Step{}, fmt.Errorf("dnd5e: invalid random response: %w", err)
	}
	success := total >= *state.DC
	status := "FAILURE"
	if success {
		status = "SUCCESS"
	}
	traits := CriticalTraits{
		CriticalHit:  expression.IsCriticalHit(rolls),
		CriticalFail: expression.IsCriticalFail(rolls),
	}
	criticalText := ""
	if traits.CriticalHit {
		criticalText = " [NAT 20]"
	} else if traits.CriticalFail {
		criticalText = " [NAT 1]"
	}
	content := fmt.Sprintf("Check (DC %d): d20(%d)%+d = %d [%s]%s",
		*state.DC, rolls[0], *state.Modifier, total, status, criticalText)
	if state.Label != "" {
		content = state.Label + " — " + content
	}
	result, err := payloadFrom(AbilityCheckResult{
		Roll:     rolls[0],
		Modifier: *state.Modifier,
		DC:       *state.DC,
		Total:    total,
		Success:  success,
		Traits:   traits,
		Legacy: LegacyResult{
			Content:    content,
			LogMessage: content,
			LogType:    "roll",
		},
	})
	if err != nil {
		return core.Step{}, err
	}
	outcome := "dnd5e.ability_check.failure"
	if success {
		outcome = "dnd5e.ability_check.success"
	}
	return completion(id, outcome, result), nil
}

func completion(id, outcome string, result core.Payload) core.Step {
	return core.Step{
		ID:       id,
		Kind:     core.StepKindComplete,
		Complete: &core.Completion{Outcome: outcome, Result: result},
	}
}

// Project implements rules.Ruleset.
func (r *Ruleset) Project(ctx context.Context, request core.ProjectRequest) (core.Projection, error) {
	if err := checkContext(ctx); err != nil {
		return core.Projection{}, err
	}
	if err := request.Validate(); err != nil {
		return core.Projection{}, err
	}
	if err := validateSnapshot(request.Snapshot); err != nil {
		return core.Projection{}, err
	}
	return core.Projection{View: request.Snapshot.State}, nil
}

// Explain implements rules.Ruleset.
func (r *Ruleset) Explain(ctx context.Context, request core.ExplainRequest) (core.Explanation, error) {
	if err := checkContext(ctx); err != nil {
		return core.Explanation{}, err
	}
	if err := request.Validate(); err != nil {
		return core.Explanation{}, err
	}
	if err := validateSnapshot(request.Snapshot); err != nil {
		return core.Explanation{}, err
	}
	var text string
	switch request.Reference {
	case ActionDiceRoll:
		text = "Roll the requested dice, sum their faces, and apply the numeric modifier. A natural 20 or 1 is annotated only for a single d20."
	case ActionAbilityCheck:
		text = "Roll one d20, add the modifier, and succeed when the total is at least the DC. Natural 20 and 1 are annotations and do not override that comparison."
	default:
		return core.Explanation{}, fmt.Errorf("dnd5e: unknown rule reference %q", request.Reference)
	}
	return core.Explanation{Text: text}, nil
}

// ValidateState implements rules.Ruleset.
func (r *Ruleset) ValidateState(ctx context.Context, request core.ValidateStateRequest) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	return validateSnapshot(request.Snapshot)
}

// Reduce implements rules.Ruleset. This reduced compatibility cut emits no
// state events, so every supplied event is unsupported.
func (r *Ruleset) Reduce(ctx context.Context, request core.ReduceRequest) (core.ReduceResult, error) {
	if err := checkContext(ctx); err != nil {
		return core.ReduceResult{}, err
	}
	if err := request.Validate(); err != nil {
		return core.ReduceResult{}, err
	}
	if err := validateSnapshot(request.Snapshot); err != nil {
		return core.ReduceResult{}, err
	}
	return core.ReduceResult{}, fmt.Errorf("dnd5e: reduced compatibility ruleset does not accept events")
}

// Migrate implements rules.Ruleset as an identity migration for the same exact
// artifact. No cross-version migration exists in this reduced cut.
func (r *Ruleset) Migrate(ctx context.Context, request core.MigrateRequest) (core.MigrateResult, error) {
	if err := checkContext(ctx); err != nil {
		return core.MigrateResult{}, err
	}
	if err := request.Validate(); err != nil {
		return core.MigrateResult{}, err
	}
	artifact, err := NewArtifact()
	if err != nil {
		return core.MigrateResult{}, err
	}
	if request.From != artifact.Lock() {
		return core.MigrateResult{}, fmt.Errorf("dnd5e: no migration from %s@%s", request.From.ID, request.From.Version)
	}
	if err := decodeState(request.State); err != nil {
		return core.MigrateResult{}, err
	}
	return core.MigrateResult{State: request.State}, nil
}

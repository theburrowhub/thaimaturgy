package pbta

import (
	_ "embed"
	"fmt"

	core "github.com/theburrowhub/thaimaturgy/internal/rules"
	"github.com/theburrowhub/thaimaturgy/internal/rules/ruleskit"
)

const (
	PackageID      = "pbta"
	PackageVersion = "0.1.0"
	ActionMove     = "move.resolve"
)

//go:embed ruleset.go
var artifactSource string

const (
	phaseRoll     = "roll"
	phaseDecision = "decision"
)

// Band is one of the conventional 2d6 result bands.
type Band string

const (
	BandMiss      Band = "miss"
	BandWeakHit   Band = "weak_hit"
	BandStrongHit Band = "strong_hit"
)

// Choice is a move-authored closed option for a weak hit. The host and generic
// package do not invent narrative consequences.
type Choice struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type arguments struct {
	Modifier     *int     `json:"modifier"`
	MixedOptions []Choice `json:"mixed_options,omitempty"`
}

type continuation struct {
	SchemaVersion uint32   `json:"schema_version"`
	Action        string   `json:"action"`
	Phase         string   `json:"phase"`
	Authority     string   `json:"authority"`
	Modifier      int      `json:"modifier"`
	Rolls         []int    `json:"rolls,omitempty"`
	Total         int      `json:"total,omitempty"`
	MixedOptions  []Choice `json:"mixed_options,omitempty"`
}

// DecisionResponse is the host payload accepted for a NeedDecision step.
type DecisionResponse struct {
	OptionID string `json:"option_id"`
}

// MoveResult is the structured move result.
type MoveResult struct {
	Rolls        []int   `json:"rolls"`
	Modifier     int     `json:"modifier"`
	Total        int     `json:"total"`
	Band         Band    `json:"band"`
	ChosenOption *Choice `json:"chosen_option,omitempty"`
}

// Ruleset is the built-in generic PbtA resolver.
type Ruleset struct{ *ruleskit.Engine }

var _ core.Ruleset = (*Ruleset)(nil)

// New returns a fresh stateless ruleset instance.
func New() *Ruleset {
	artifact, err := NewArtifact()
	if err != nil {
		panic(err)
	}
	action := core.ActionDescriptor{
		ID:          ActionMove,
		Label:       "Resolve move",
		Description: "Roll 2d6 + modifier into miss, weak-hit, or strong-hit bands; optionally request a move-authored weak-hit choice.",
		InputSchema: ruleskit.MustPayload(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"modifier": map[string]any{"type": "integer", "minimum": -10, "maximum": 10},
				"mixed_options": map[string]any{
					"type": "array", "maxItems": 16,
					"items": map[string]any{
						"type":       "object",
						"properties": map[string]any{"id": map[string]any{"type": "string"}, "label": map[string]any{"type": "string"}},
						"required":   []string{"id", "label"}, "additionalProperties": false,
					},
				},
			},
			"required":             []string{"modifier"},
			"additionalProperties": false,
		}),
		Tags: []string{"random", "move", "decision"},
	}
	return &Ruleset{Engine: ruleskit.NewEngine(ruleskit.Definition{
		Artifact: artifact,
		Actions:  []core.ActionDescriptor{action},
		Explanations: map[string]string{
			ActionMove: "Roll 2d6 + modifier: 6 or less is a miss, 7-9 a weak hit, and 10+ a strong hit. If the move supplies closed 7-9 options, the weak hit pauses for the acting authority to choose one.",
		},
		Start:  start,
		Resume: resume,
	})}
}

// NewArtifact returns the stable host-verifiable built-in artifact.
func NewArtifact() (core.Artifact, error) {
	return ruleskit.NewArtifact(PackageID, "PbtA core resolution", "Generic 2d6 move bands and optional weak-hit choice; no move or playbook text.", PackageVersion, artifactSource, []string{ActionMove})
}

// InitialState returns this stateless package's canonical state.
func InitialState() core.Payload { return ruleskit.InitialState() }

func start(request core.StartRequest) (core.Step, error) {
	var args arguments
	if err := ruleskit.Decode(request.Intent.Arguments, &args); err != nil {
		return ruleskit.RejectArguments(request.Intent.ID, err)
	}
	if args.Modifier == nil {
		return ruleskit.Reject(request.Intent.ID, "invalid.arguments", "modifier is required"), nil
	}
	if rejected, ok := ruleskit.RequireRange(request.Intent.ID, "modifier", *args.Modifier, -10, 10); !ok {
		return rejected, nil
	}
	if len(args.MixedOptions) > 16 {
		return ruleskit.Reject(request.Intent.ID, "invalid.arguments", "mixed_options cannot contain more than 16 choices"), nil
	}
	if err := validateChoices(request.Intent.ID, request.Principal.ID, args.MixedOptions); err != nil {
		return ruleskit.Reject(request.Intent.ID, "invalid.arguments", err.Error()), nil
	}
	return ruleskit.RandomStep(request.Intent.ID, continuation{
		SchemaVersion: 1, Action: ActionMove, Phase: phaseRoll,
		Authority: request.Principal.ID, Modifier: *args.Modifier, MixedOptions: cloneChoices(args.MixedOptions),
	}, 2, 6)
}

func resume(request core.ResumeRequest) (core.Step, error) {
	var state continuation
	if err := ruleskit.DecodeRequired(request.Pending.State, &state, "schema_version", "action", "phase", "authority", "modifier"); err != nil {
		return core.Step{}, fmt.Errorf("pbta: decode continuation: %w", err)
	}
	if state.Phase == phaseDecision {
		if err := ruleskit.DecodeRequired(request.Pending.State, &state, "rolls", "total", "mixed_options"); err != nil {
			return core.Step{}, fmt.Errorf("pbta: decode decision continuation: %w", err)
		}
	}
	if err := validateContinuation(state, request.Pending.Kind, request.Pending.StepID, request.Principal.ID); err != nil {
		return core.Step{}, err
	}
	switch state.Phase {
	case phaseRoll:
		rolls, err := ruleskit.DecodeRolls(request.Response.Data, 2, 6)
		if err != nil {
			return core.Step{}, fmt.Errorf("pbta: %w", err)
		}
		total := rolls[0] + rolls[1] + state.Modifier
		band := bandFor(total)
		if band == BandWeakHit && len(state.MixedOptions) > 0 {
			state.Phase = phaseDecision
			state.Rolls = append([]int(nil), rolls...)
			state.Total = total
			continuationPayload, err := ruleskit.Payload(state)
			if err != nil {
				return core.Step{}, err
			}
			return core.Step{
				ID: request.Pending.StepID, Kind: core.StepKindNeedDecision, Continuation: continuationPayload,
				NeedDecision: &core.DecisionRequest{
					Authority: state.Authority,
					Prompt:    "Choose the move's 7-9 consequence.",
					Options:   decisionOptions(state.MixedOptions),
				},
			}, nil
		}
		return complete(request.Pending.StepID, rolls, state.Modifier, total, band, nil)
	case phaseDecision:
		var response DecisionResponse
		if err := ruleskit.Decode(request.Response.Data, &response); err != nil {
			return core.Step{}, fmt.Errorf("pbta: decode decision response: %w", err)
		}
		for _, option := range state.MixedOptions {
			if option.ID == response.OptionID {
				selected := option
				return complete(request.Pending.StepID, state.Rolls, state.Modifier, state.Total, BandWeakHit, &selected)
			}
		}
		return core.Step{}, fmt.Errorf("pbta: decision option %q was not offered", response.OptionID)
	default:
		return core.Step{}, fmt.Errorf("pbta: unsupported continuation phase %q", state.Phase)
	}
}

func validateContinuation(state continuation, kind core.StepKind, id, authority string) error {
	if state.SchemaVersion != 1 || state.Action != ActionMove || state.Authority != authority || state.Modifier < -10 || state.Modifier > 10 || len(state.MixedOptions) > 16 {
		return fmt.Errorf("pbta: invalid move continuation")
	}
	if err := validateChoices(id, state.Authority, state.MixedOptions); err != nil {
		return fmt.Errorf("pbta: invalid move continuation: %w", err)
	}
	switch state.Phase {
	case phaseRoll:
		if kind != core.StepKindNeedRandom || len(state.Rolls) != 0 || state.Total != 0 {
			return fmt.Errorf("pbta: invalid roll continuation")
		}
	case phaseDecision:
		if kind != core.StepKindNeedDecision || len(state.Rolls) != 2 || state.Rolls[0] < 1 || state.Rolls[0] > 6 || state.Rolls[1] < 1 || state.Rolls[1] > 6 || state.Total != state.Rolls[0]+state.Rolls[1]+state.Modifier || bandFor(state.Total) != BandWeakHit || len(state.MixedOptions) == 0 {
			return fmt.Errorf("pbta: invalid decision continuation")
		}
	default:
		return fmt.Errorf("pbta: unsupported continuation phase %q", state.Phase)
	}
	return nil
}

func validateChoices(id, authority string, choices []Choice) error {
	if len(choices) == 0 {
		return nil
	}
	state := ruleskit.MustPayload(map[string]any{"validation": true})
	step := core.Step{
		ID: id, Kind: core.StepKindNeedDecision, Continuation: state,
		NeedDecision: &core.DecisionRequest{
			Authority: authority, Prompt: "Validate choices.", Options: decisionOptions(choices),
		},
	}
	return step.Validate()
}

func decisionOptions(choices []Choice) []core.DecisionOption {
	options := make([]core.DecisionOption, len(choices))
	for i, choice := range choices {
		options[i] = core.DecisionOption{ID: choice.ID, Label: choice.Label}
	}
	return options
}

func complete(id string, rolls []int, modifier, total int, band Band, choice *Choice) (core.Step, error) {
	return ruleskit.Complete(id, "pbta.move."+string(band), MoveResult{
		Rolls: append([]int(nil), rolls...), Modifier: modifier, Total: total, Band: band, ChosenOption: choice,
	})
}

func bandFor(total int) Band {
	if total >= 10 {
		return BandStrongHit
	}
	if total >= 7 {
		return BandWeakHit
	}
	return BandMiss
}

func cloneChoices(choices []Choice) []Choice { return append([]Choice(nil), choices...) }

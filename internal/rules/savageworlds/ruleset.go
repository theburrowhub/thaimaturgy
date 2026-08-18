package savageworlds

import (
	"fmt"

	core "github.com/theburrowhub/thaimaturgy/internal/rules"
	"github.com/theburrowhub/thaimaturgy/internal/rules/ruleskit"
)

const (
	PackageID       = "savageworlds"
	PackageVersion  = "0.1.0"
	ActionTraitTest = "trait.check"
)

const artifactMaterial = "thaimaturgy builtin rules artifact\npackage=savageworlds\nversion=0.1.0\nprotocol=1.0.0\nactions=trait.check\nabi=1\n"

const (
	phaseTrait        = "trait"
	phaseWild         = "wild"
	maxExplosionRolls = 100
)

// Outcome summarizes the trait check.
type Outcome string

const (
	OutcomeSnakeEyes         Outcome = "snake_eyes"
	OutcomeFailure           Outcome = "failure"
	OutcomeSuccess           Outcome = "success"
	OutcomeSuccessWithRaises Outcome = "success_with_raises"
)

type arguments struct {
	TraitDie     *int  `json:"trait_die"`
	WildCard     *bool `json:"wild_card"`
	Modifier     int   `json:"modifier,omitempty"`
	TargetNumber *int  `json:"target_number"`
}

type continuation struct {
	SchemaVersion uint32 `json:"schema_version"`
	Action        string `json:"action"`
	Phase         string `json:"phase"`
	TraitDie      int    `json:"trait_die"`
	WildCard      bool   `json:"wild_card"`
	Modifier      int    `json:"modifier"`
	TargetNumber  int    `json:"target_number"`
	TraitRolls    []int  `json:"trait_rolls,omitempty"`
	WildRolls     []int  `json:"wild_rolls,omitempty"`
}

// CheckResult is the structured exploding-die result.
type CheckResult struct {
	TraitDie      int     `json:"trait_die"`
	TraitRolls    []int   `json:"trait_rolls"`
	TraitTotal    int     `json:"trait_total"`
	WildCard      bool    `json:"wild_card"`
	WildRolls     []int   `json:"wild_rolls,omitempty"`
	WildTotal     int     `json:"wild_total,omitempty"`
	SelectedDie   string  `json:"selected_die"`
	SelectedTotal int     `json:"selected_total"`
	Modifier      int     `json:"modifier"`
	TargetNumber  int     `json:"target_number"`
	FinalTotal    int     `json:"final_total"`
	Raises        int     `json:"raises"`
	Success       bool    `json:"success"`
	SnakeEyes     bool    `json:"snake_eyes"`
	Outcome       Outcome `json:"outcome"`
}

// Ruleset is the built-in Savage Worlds core resolver.
type Ruleset struct{ *ruleskit.Engine }

var _ core.Ruleset = (*Ruleset)(nil)

// New returns a fresh stateless ruleset instance.
func New() *Ruleset {
	artifact, err := NewArtifact()
	if err != nil {
		panic(err)
	}
	action := core.ActionDescriptor{
		ID:          ActionTraitTest,
		Label:       "Resolve trait check",
		Description: "Roll an exploding trait die and optional exploding Wild Die; keep the higher total and count raises.",
		InputSchema: ruleskit.MustPayload(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"trait_die":     map[string]any{"type": "integer", "enum": []int{4, 6, 8, 10, 12}},
				"wild_card":     map[string]any{"type": "boolean"},
				"modifier":      map[string]any{"type": "integer", "minimum": -20, "maximum": 20},
				"target_number": map[string]any{"type": "integer", "minimum": 1, "maximum": 100},
			},
			"required":             []string{"trait_die", "wild_card", "target_number"},
			"additionalProperties": false,
		}),
		Tags: []string{"random", "exploding_dice", "check"},
	}
	return &Ruleset{Engine: ruleskit.NewEngine(ruleskit.Definition{
		Artifact: artifact,
		Actions:  []core.ActionDescriptor{action},
		Explanations: map[string]string{
			ActionTraitTest: "Roll the trait die and, for Wild Cards, a d6 Wild Die. A maximum face aces and is rolled again, accumulating. Keep the higher die total, apply the modifier, and gain one raise per full 4 over the target. Initial ones on both dice are snake eyes.",
		},
		Start:  start,
		Resume: resume,
	})}
}

// NewArtifact returns the stable host-verifiable built-in artifact.
func NewArtifact() (core.Artifact, error) {
	return ruleskit.NewArtifact(PackageID, "Savage Worlds core resolution", "Exploding trait/Wild dice, target numbers, and raises; no compendium content.", PackageVersion, artifactMaterial, []string{ActionTraitTest})
}

// InitialState returns this stateless package's canonical state.
func InitialState() core.Payload { return ruleskit.InitialState() }

func start(request core.StartRequest) (core.Step, error) {
	var args arguments
	if err := ruleskit.Decode(request.Intent.Arguments, &args); err != nil {
		return ruleskit.RejectArguments(request.Intent.ID, err)
	}
	if args.TraitDie == nil || args.WildCard == nil || args.TargetNumber == nil {
		return ruleskit.Reject(request.Intent.ID, "invalid.arguments", "trait_die, wild_card, and target_number are required"), nil
	}
	if !validTraitDie(*args.TraitDie) {
		return ruleskit.Reject(request.Intent.ID, "invalid.arguments", "trait_die must be one of 4, 6, 8, 10, or 12"), nil
	}
	if rejected, ok := ruleskit.RequireRange(request.Intent.ID, "modifier", args.Modifier, -20, 20); !ok {
		return rejected, nil
	}
	if rejected, ok := ruleskit.RequireRange(request.Intent.ID, "target_number", *args.TargetNumber, 1, 100); !ok {
		return rejected, nil
	}
	return ruleskit.RandomStep(request.Intent.ID, continuation{
		SchemaVersion: 1, Action: ActionTraitTest, Phase: phaseTrait,
		TraitDie: *args.TraitDie, WildCard: *args.WildCard,
		Modifier: args.Modifier, TargetNumber: *args.TargetNumber,
	}, 1, *args.TraitDie)
}

func resume(request core.ResumeRequest) (core.Step, error) {
	if request.Pending.Kind != core.StepKindNeedRandom {
		return core.Step{}, fmt.Errorf("savageworlds: cannot resume step kind %q", request.Pending.Kind)
	}
	var state continuation
	if err := ruleskit.DecodeRequired(request.Pending.State, &state, "schema_version", "action", "phase", "trait_die", "wild_card", "modifier", "target_number"); err != nil {
		return core.Step{}, fmt.Errorf("savageworlds: decode continuation: %w", err)
	}
	if err := validateContinuation(state); err != nil {
		return core.Step{}, err
	}
	switch state.Phase {
	case phaseTrait:
		rolls, err := ruleskit.DecodeRolls(request.Response.Data, 1, state.TraitDie)
		if err != nil {
			return core.Step{}, fmt.Errorf("savageworlds: %w", err)
		}
		state.TraitRolls = append(state.TraitRolls, rolls[0])
		if rolls[0] == state.TraitDie {
			if len(state.TraitRolls) >= maxExplosionRolls {
				return core.Step{}, fmt.Errorf("savageworlds: trait die exceeded %d consecutive aces", maxExplosionRolls)
			}
			return ruleskit.RandomStep(request.Pending.StepID, state, 1, state.TraitDie)
		}
		if state.WildCard {
			state.Phase = phaseWild
			return ruleskit.RandomStep(request.Pending.StepID, state, 1, 6)
		}
		return complete(request.Pending.StepID, state)
	case phaseWild:
		rolls, err := ruleskit.DecodeRolls(request.Response.Data, 1, 6)
		if err != nil {
			return core.Step{}, fmt.Errorf("savageworlds: %w", err)
		}
		state.WildRolls = append(state.WildRolls, rolls[0])
		if rolls[0] == 6 {
			if len(state.WildRolls) >= maxExplosionRolls {
				return core.Step{}, fmt.Errorf("savageworlds: wild die exceeded %d consecutive aces", maxExplosionRolls)
			}
			return ruleskit.RandomStep(request.Pending.StepID, state, 1, 6)
		}
		return complete(request.Pending.StepID, state)
	default:
		return core.Step{}, fmt.Errorf("savageworlds: unsupported continuation phase %q", state.Phase)
	}
}

func validateContinuation(state continuation) error {
	if state.SchemaVersion != 1 || state.Action != ActionTraitTest || !validTraitDie(state.TraitDie) || state.Modifier < -20 || state.Modifier > 20 || state.TargetNumber < 1 || state.TargetNumber > 100 || len(state.TraitRolls) >= maxExplosionRolls || len(state.WildRolls) >= maxExplosionRolls {
		return fmt.Errorf("savageworlds: invalid trait-check continuation")
	}
	if state.Phase == phaseTrait {
		if len(state.WildRolls) != 0 || !allFaces(state.TraitRolls, state.TraitDie) {
			return fmt.Errorf("savageworlds: invalid trait-die continuation")
		}
		return nil
	}
	if state.Phase == phaseWild {
		if !state.WildCard || !finishedExplodingDie(state.TraitRolls, state.TraitDie) || !allFaces(state.WildRolls, 6) {
			return fmt.Errorf("savageworlds: invalid wild-die continuation")
		}
		return nil
	}
	return fmt.Errorf("savageworlds: unsupported continuation phase %q", state.Phase)
}

func complete(id string, state continuation) (core.Step, error) {
	traitTotal := sum(state.TraitRolls)
	wildTotal := sum(state.WildRolls)
	selectedDie, selectedTotal := "trait", traitTotal
	if state.WildCard && wildTotal > traitTotal {
		selectedDie, selectedTotal = "wild", wildTotal
	}
	snakeEyes := state.WildCard && state.TraitRolls[0] == 1 && state.WildRolls[0] == 1
	finalTotal := selectedTotal + state.Modifier
	success := !snakeEyes && finalTotal >= state.TargetNumber
	raises := 0
	if success {
		raises = (finalTotal - state.TargetNumber) / 4
	}
	outcome := OutcomeFailure
	switch {
	case snakeEyes:
		outcome = OutcomeSnakeEyes
	case success && raises > 0:
		outcome = OutcomeSuccessWithRaises
	case success:
		outcome = OutcomeSuccess
	}
	return ruleskit.Complete(id, "savageworlds.trait."+string(outcome), CheckResult{
		TraitDie: state.TraitDie, TraitRolls: append([]int(nil), state.TraitRolls...), TraitTotal: traitTotal,
		WildCard: state.WildCard, WildRolls: append([]int(nil), state.WildRolls...), WildTotal: wildTotal,
		SelectedDie: selectedDie, SelectedTotal: selectedTotal, Modifier: state.Modifier,
		TargetNumber: state.TargetNumber, FinalTotal: finalTotal, Raises: raises,
		Success: success, SnakeEyes: snakeEyes, Outcome: outcome,
	})
}

func validTraitDie(sides int) bool {
	return sides == 4 || sides == 6 || sides == 8 || sides == 10 || sides == 12
}

func allFaces(rolls []int, face int) bool {
	for _, roll := range rolls {
		if roll != face {
			return false
		}
	}
	return true
}

func finishedExplodingDie(rolls []int, face int) bool {
	if len(rolls) == 0 || rolls[len(rolls)-1] == face {
		return false
	}
	return allFaces(rolls[:len(rolls)-1], face)
}

func sum(values []int) int {
	total := 0
	for _, value := range values {
		total += value
	}
	return total
}

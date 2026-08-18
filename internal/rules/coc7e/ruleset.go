package coc7e

import (
	"fmt"

	core "github.com/theburrowhub/thaimaturgy/internal/rules"
	"github.com/theburrowhub/thaimaturgy/internal/rules/ruleskit"
)

const (
	PackageID      = "coc7e"
	PackageVersion = "0.1.0"
	ActionCheck    = "skill.check"
)

const artifactMaterial = "thaimaturgy builtin rules artifact\npackage=coc7e\nversion=0.1.0\nprotocol=1.0.0\nactions=skill.check\nabi=1\n"

// Difficulty identifies the requested success threshold.
type Difficulty string

const (
	DifficultyRegular Difficulty = "regular"
	DifficultyHard    Difficulty = "hard"
	DifficultyExtreme Difficulty = "extreme"
)

// SuccessLevel classifies the selected percentile roll.
type SuccessLevel string

const (
	LevelFumble   SuccessLevel = "fumble"
	LevelFailure  SuccessLevel = "failure"
	LevelRegular  SuccessLevel = "regular_success"
	LevelHard     SuccessLevel = "hard_success"
	LevelExtreme  SuccessLevel = "extreme_success"
	LevelCritical SuccessLevel = "critical"
)

type arguments struct {
	Skill      *int       `json:"skill"`
	Difficulty Difficulty `json:"difficulty"`
	BonusDice  int        `json:"bonus_dice,omitempty"`
}

type continuation struct {
	SchemaVersion uint32     `json:"schema_version"`
	Action        string     `json:"action"`
	Skill         int        `json:"skill"`
	Difficulty    Difficulty `json:"difficulty"`
	BonusDice     int        `json:"bonus_dice"`
}

// CheckResult is the structured percentile result. RawRolls use d10 faces
// 1..10; face 10 represents a zero digit, keeping the host RNG contract
// uniformly one-based.
type CheckResult struct {
	RawRolls   []int        `json:"raw_rolls"`
	Candidates []int        `json:"candidates"`
	Selected   int          `json:"selected"`
	Skill      int          `json:"skill"`
	Difficulty Difficulty   `json:"difficulty"`
	BonusDice  int          `json:"bonus_dice"`
	RegularMax int          `json:"regular_max"`
	HardMax    int          `json:"hard_max"`
	ExtremeMax int          `json:"extreme_max"`
	Level      SuccessLevel `json:"level"`
	Passed     bool         `json:"passed"`
}

// Ruleset is the built-in CoC 7e core resolver.
type Ruleset struct{ *ruleskit.Engine }

var _ core.Ruleset = (*Ruleset)(nil)

// New returns a fresh stateless ruleset instance.
func New() *Ruleset {
	artifact, err := NewArtifact()
	if err != nil {
		panic(err)
	}
	action := core.ActionDescriptor{
		ID:          ActionCheck,
		Label:       "Resolve percentile check",
		Description: "Resolve a regular, hard, or extreme d100 check with up to two bonus or penalty dice.",
		InputSchema: ruleskit.MustPayload(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"skill":      map[string]any{"type": "integer", "minimum": 1, "maximum": 99},
				"difficulty": map[string]any{"type": "string", "enum": []string{"regular", "hard", "extreme"}},
				"bonus_dice": map[string]any{"type": "integer", "minimum": -2, "maximum": 2, "description": "Positive values are bonus dice; negative values are penalty dice."},
			},
			"required":             []string{"skill", "difficulty"},
			"additionalProperties": false,
		}),
		Tags: []string{"random", "percentile", "check"},
	}
	return &Ruleset{Engine: ruleskit.NewEngine(ruleskit.Definition{
		Artifact: artifact,
		Actions:  []core.ActionDescriptor{action},
		Explanations: map[string]string{
			ActionCheck: "Roll units and one or more tens dice. Bonus dice select the lowest candidate and penalty dice the highest. Compare against full, half, or one-fifth skill for regular, hard, or extreme difficulty; 1 is critical and the upper failure band can fumble.",
		},
		Start:  start,
		Resume: resume,
	})}
}

// NewArtifact returns the stable host-verifiable built-in artifact.
func NewArtifact() (core.Artifact, error) {
	return ruleskit.NewArtifact(PackageID, "Call of Cthulhu 7e core resolution", "Percentile difficulty and bonus/penalty dice primitive; no compendium content.", PackageVersion, artifactMaterial, []string{ActionCheck})
}

// InitialState returns this stateless package's canonical state.
func InitialState() core.Payload { return ruleskit.InitialState() }

func start(request core.StartRequest) (core.Step, error) {
	var args arguments
	if err := ruleskit.Decode(request.Intent.Arguments, &args); err != nil {
		return ruleskit.RejectArguments(request.Intent.ID, err)
	}
	if args.Skill == nil {
		return ruleskit.Reject(request.Intent.ID, "invalid.arguments", "skill is required"), nil
	}
	if rejected, ok := ruleskit.RequireRange(request.Intent.ID, "skill", *args.Skill, 1, 99); !ok {
		return rejected, nil
	}
	if !validDifficulty(args.Difficulty) {
		return ruleskit.Reject(request.Intent.ID, "invalid.arguments", "difficulty must be regular, hard, or extreme"), nil
	}
	if rejected, ok := ruleskit.RequireRange(request.Intent.ID, "bonus_dice", args.BonusDice, -2, 2); !ok {
		return rejected, nil
	}
	count := 2 + abs(args.BonusDice)
	return ruleskit.RandomStep(request.Intent.ID, continuation{
		SchemaVersion: 1, Action: ActionCheck, Skill: *args.Skill,
		Difficulty: args.Difficulty, BonusDice: args.BonusDice,
	}, count, 10)
}

func resume(request core.ResumeRequest) (core.Step, error) {
	if request.Pending.Kind != core.StepKindNeedRandom {
		return core.Step{}, fmt.Errorf("coc7e: cannot resume step kind %q", request.Pending.Kind)
	}
	var state continuation
	if err := ruleskit.DecodeRequired(request.Pending.State, &state, "schema_version", "action", "skill", "difficulty", "bonus_dice"); err != nil {
		return core.Step{}, fmt.Errorf("coc7e: decode continuation: %w", err)
	}
	if state.SchemaVersion != 1 || state.Action != ActionCheck || state.Skill < 1 || state.Skill > 99 || !validDifficulty(state.Difficulty) || state.BonusDice < -2 || state.BonusDice > 2 {
		return core.Step{}, fmt.Errorf("coc7e: invalid check continuation")
	}
	rolls, err := ruleskit.DecodeRolls(request.Response.Data, 2+abs(state.BonusDice), 10)
	if err != nil {
		return core.Step{}, fmt.Errorf("coc7e: %w", err)
	}
	ones := rolls[0] % 10
	candidates := make([]int, len(rolls)-1)
	for i, tensFace := range rolls[1:] {
		candidate := (tensFace%10)*10 + ones
		if candidate == 0 {
			candidate = 100
		}
		candidates[i] = candidate
	}
	selected := candidates[0]
	for _, candidate := range candidates[1:] {
		if state.BonusDice > 0 && candidate < selected || state.BonusDice < 0 && candidate > selected {
			selected = candidate
		}
	}
	regularMax := state.Skill
	hardMax := state.Skill / 2
	extremeMax := state.Skill / 5
	level := classify(selected, state.Skill, hardMax, extremeMax)
	passed := passes(level, state.Difficulty)
	return ruleskit.Complete(request.Pending.StepID, "coc7e.check."+string(level), CheckResult{
		RawRolls: append([]int(nil), rolls...), Candidates: candidates, Selected: selected,
		Skill: state.Skill, Difficulty: state.Difficulty, BonusDice: state.BonusDice,
		RegularMax: regularMax, HardMax: hardMax, ExtremeMax: extremeMax,
		Level: level, Passed: passed,
	})
}

func validDifficulty(difficulty Difficulty) bool {
	return difficulty == DifficultyRegular || difficulty == DifficultyHard || difficulty == DifficultyExtreme
}

func classify(roll, skill, hardMax, extremeMax int) SuccessLevel {
	switch {
	case roll == 1:
		return LevelCritical
	case roll == 100 || skill < 50 && roll >= 96:
		return LevelFumble
	case roll <= extremeMax:
		return LevelExtreme
	case roll <= hardMax:
		return LevelHard
	case roll <= skill:
		return LevelRegular
	default:
		return LevelFailure
	}
}

func passes(level SuccessLevel, difficulty Difficulty) bool {
	rank := map[SuccessLevel]int{LevelFumble: 0, LevelFailure: 0, LevelRegular: 1, LevelHard: 2, LevelExtreme: 3, LevelCritical: 4}
	required := map[Difficulty]int{DifficultyRegular: 1, DifficultyHard: 2, DifficultyExtreme: 3}
	return rank[level] >= required[difficulty]
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

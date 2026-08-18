package fatecore

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	core "github.com/theburrowhub/thaimaturgy/internal/rules"
	"github.com/theburrowhub/thaimaturgy/internal/rules/ruleskit"
)

const (
	PackageID      = "fatecore"
	PackageVersion = "0.1.0"
	ActionResolve  = "action.resolve"
)

const artifactMaterial = "thaimaturgy builtin rules artifact\npackage=fatecore\nversion=0.1.0\nprotocol=1.0.0\nactions=action.resolve\nabi=1\n"

// Outcome is Fate's comparison band.
type Outcome string

const (
	OutcomeFail             Outcome = "fail"
	OutcomeTie              Outcome = "tie"
	OutcomeSucceed          Outcome = "succeed"
	OutcomeSucceedWithStyle Outcome = "succeed_with_style"
)

type arguments struct {
	Skill      *int     `json:"skill"`
	Opposition *int     `json:"opposition"`
	Invokes    []string `json:"invokes,omitempty"`
}

type continuation struct {
	SchemaVersion uint32   `json:"schema_version"`
	Action        string   `json:"action"`
	Skill         int      `json:"skill"`
	Opposition    int      `json:"opposition"`
	Invokes       []string `json:"invokes,omitempty"`
}

// ActionResult is the structured Fate result. RawRolls are one-based d3 faces;
// Dice map them to -1, 0, +1 for host RNG interoperability.
type ActionResult struct {
	RawRolls    []int    `json:"raw_rolls"`
	Dice        []int    `json:"dice"`
	DiceTotal   int      `json:"dice_total"`
	Skill       int      `json:"skill"`
	Opposition  int      `json:"opposition"`
	Invokes     []string `json:"invokes"`
	InvokeBonus int      `json:"invoke_bonus"`
	Effort      int      `json:"effort"`
	Shifts      int      `json:"shifts"`
	Outcome     Outcome  `json:"outcome"`
}

// Ruleset is the built-in Fate Core resolver.
type Ruleset struct{ *ruleskit.Engine }

var _ core.Ruleset = (*Ruleset)(nil)

// New returns a fresh stateless ruleset instance.
func New() *Ruleset {
	artifact, err := NewArtifact()
	if err != nil {
		panic(err)
	}
	action := core.ActionDescriptor{
		ID:          ActionResolve,
		Label:       "Resolve Fate action",
		Description: "Roll four Fate dice, add skill and authorized +2 invokes, then compare effort to opposition.",
		InputSchema: ruleskit.MustPayload(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"skill":      map[string]any{"type": "integer", "minimum": -20, "maximum": 20},
				"opposition": map[string]any{"type": "integer", "minimum": -20, "maximum": 20},
				"invokes":    map[string]any{"type": "array", "maxItems": 10, "items": map[string]any{"type": "string", "maxLength": core.MaxTextBytes}},
			},
			"required":             []string{"skill", "opposition"},
			"additionalProperties": false,
		}),
		Tags: []string{"random", "fate_dice", "check"},
	}
	return &Ruleset{Engine: ruleskit.NewEngine(ruleskit.Definition{
		Artifact: artifact,
		Actions:  []core.ActionDescriptor{action},
		Explanations: map[string]string{
			ActionResolve: "Map four d3 faces to Fate dice (-, blank, +), add skill and +2 for each already-authorized invoke, then subtract opposition. Negative shifts fail, zero ties, 1-2 succeeds, and 3+ succeeds with style.",
		},
		Start:  start,
		Resume: resume,
	})}
}

// NewArtifact returns the stable host-verifiable built-in artifact.
func NewArtifact() (core.Artifact, error) {
	return ruleskit.NewArtifact(PackageID, "Fate Core resolution", "Four Fate dice, shifts, outcomes, and explicit invokes; no compendium content.", PackageVersion, artifactMaterial, []string{ActionResolve})
}

// InitialState returns this stateless package's canonical state.
func InitialState() core.Payload { return ruleskit.InitialState() }

func start(request core.StartRequest) (core.Step, error) {
	var args arguments
	if err := ruleskit.Decode(request.Intent.Arguments, &args); err != nil {
		return ruleskit.RejectArguments(request.Intent.ID, err)
	}
	if args.Skill == nil || args.Opposition == nil {
		return ruleskit.Reject(request.Intent.ID, "invalid.arguments", "skill and opposition are required"), nil
	}
	if rejected, ok := ruleskit.RequireRange(request.Intent.ID, "skill", *args.Skill, -20, 20); !ok {
		return rejected, nil
	}
	if rejected, ok := ruleskit.RequireRange(request.Intent.ID, "opposition", *args.Opposition, -20, 20); !ok {
		return rejected, nil
	}
	if err := validateInvokes(args.Invokes); err != nil {
		return ruleskit.Reject(request.Intent.ID, "invalid.arguments", err.Error()), nil
	}
	return ruleskit.RandomStep(request.Intent.ID, continuation{
		SchemaVersion: 1, Action: ActionResolve, Skill: *args.Skill,
		Opposition: *args.Opposition, Invokes: append([]string(nil), args.Invokes...),
	}, 4, 3)
}

func resume(request core.ResumeRequest) (core.Step, error) {
	if request.Pending.Kind != core.StepKindNeedRandom {
		return core.Step{}, fmt.Errorf("fatecore: cannot resume step kind %q", request.Pending.Kind)
	}
	var state continuation
	if err := ruleskit.DecodeRequired(request.Pending.State, &state, "schema_version", "action", "skill", "opposition"); err != nil {
		return core.Step{}, fmt.Errorf("fatecore: decode continuation: %w", err)
	}
	if state.SchemaVersion != 1 || state.Action != ActionResolve || state.Skill < -20 || state.Skill > 20 || state.Opposition < -20 || state.Opposition > 20 {
		return core.Step{}, fmt.Errorf("fatecore: invalid action continuation")
	}
	if err := validateInvokes(state.Invokes); err != nil {
		return core.Step{}, fmt.Errorf("fatecore: invalid action continuation: %w", err)
	}
	rolls, err := ruleskit.DecodeRolls(request.Response.Data, 4, 3)
	if err != nil {
		return core.Step{}, fmt.Errorf("fatecore: %w", err)
	}
	dice := make([]int, len(rolls))
	diceTotal := 0
	for i, roll := range rolls {
		dice[i] = roll - 2
		diceTotal += dice[i]
	}
	invokeBonus := len(state.Invokes) * 2
	effort := diceTotal + state.Skill + invokeBonus
	shifts := effort - state.Opposition
	outcome := OutcomeFail
	switch {
	case shifts >= 3:
		outcome = OutcomeSucceedWithStyle
	case shifts >= 1:
		outcome = OutcomeSucceed
	case shifts == 0:
		outcome = OutcomeTie
	}
	return ruleskit.Complete(request.Pending.StepID, "fatecore.action."+string(outcome), ActionResult{
		RawRolls: append([]int(nil), rolls...), Dice: dice, DiceTotal: diceTotal,
		Skill: state.Skill, Opposition: state.Opposition, Invokes: append([]string(nil), state.Invokes...),
		InvokeBonus: invokeBonus, Effort: effort, Shifts: shifts, Outcome: outcome,
	})
}

func validateInvokes(invokes []string) error {
	if len(invokes) > 10 {
		return fmt.Errorf("invokes cannot contain more than 10 aspects")
	}
	for i, invoke := range invokes {
		if invoke == "" || strings.TrimSpace(invoke) != invoke || len(invoke) > core.MaxTextBytes || !utf8.ValidString(invoke) {
			return fmt.Errorf("invoke %d must be non-empty, bounded UTF-8 without surrounding whitespace", i)
		}
		for _, character := range invoke {
			if unicode.IsControl(character) {
				return fmt.Errorf("invoke %d must not contain control characters", i)
			}
		}
	}
	return nil
}

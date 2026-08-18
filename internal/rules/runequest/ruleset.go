package runequest

import (
	"fmt"

	core "github.com/theburrowhub/thaimaturgy/internal/rules"
	"github.com/theburrowhub/thaimaturgy/internal/rules/ruleskit"
)

const (
	PackageID       = "runequest"
	PackageVersion  = "0.1.0"
	ActionSkillTest = "skill.check"
)

const artifactMaterial = "thaimaturgy builtin rules artifact\npackage=runequest\nversion=0.1.0\nprotocol=1.0.0\nactions=skill.check\nabi=1\n"

// Outcome is the degree returned by a skill check.
type Outcome string

const (
	OutcomeCritical Outcome = "critical"
	OutcomeSpecial  Outcome = "special"
	OutcomeSuccess  Outcome = "success"
	OutcomeFailure  Outcome = "failure"
	OutcomeFumble   Outcome = "fumble"
)

type arguments struct {
	Skill    *int `json:"skill"`
	Modifier int  `json:"modifier,omitempty"`
}

type continuation struct {
	SchemaVersion uint32 `json:"schema_version"`
	Action        string `json:"action"`
	Skill         int    `json:"skill"`
	Modifier      int    `json:"modifier"`
	Target        int    `json:"target"`
}

// CheckResult is the structured d100 result.
type CheckResult struct {
	Roll          int     `json:"roll"`
	Skill         int     `json:"skill"`
	Modifier      int     `json:"modifier"`
	Target        int     `json:"target"`
	CriticalMax   int     `json:"critical_max"`
	SpecialMax    int     `json:"special_max"`
	FumbleMinimum int     `json:"fumble_minimum"`
	Outcome       Outcome `json:"outcome"`
}

// Ruleset is the built-in RuneQuest-style core resolver.
type Ruleset struct{ *ruleskit.Engine }

var _ core.Ruleset = (*Ruleset)(nil)

// New returns a fresh stateless ruleset instance.
func New() *Ruleset {
	artifact, err := NewArtifact()
	if err != nil {
		panic(err)
	}
	action := core.ActionDescriptor{
		ID:          ActionSkillTest,
		Label:       "Resolve percentile skill",
		Description: "Roll d100 under an effective skill and classify critical, special, success, failure, or fumble.",
		InputSchema: ruleskit.MustPayload(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"skill":    map[string]any{"type": "integer", "minimum": 0, "maximum": 500},
				"modifier": map[string]any{"type": "integer", "minimum": -500, "maximum": 500},
			},
			"required":             []string{"skill"},
			"additionalProperties": false,
		}),
		Tags: []string{"random", "percentile", "check"},
	}
	return &Ruleset{Engine: ruleskit.NewEngine(ruleskit.Definition{
		Artifact: artifact,
		Actions:  []core.ActionDescriptor{action},
		Explanations: map[string]string{
			ActionSkillTest: "Roll d100. A roll within 5% of the effective skill is critical and within 20% is special; otherwise roll under the target. The fumble band shrinks as skill rises, while 100 always fumbles.",
		},
		Start:  start,
		Resume: resume,
	})}
}

// NewArtifact returns the stable host-verifiable built-in artifact.
func NewArtifact() (core.Artifact, error) {
	return ruleskit.NewArtifact(PackageID, "RuneQuest core resolution", "Percentile roll-under degree primitive; no proprietary compendium content.", PackageVersion, artifactMaterial, []string{ActionSkillTest})
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
	if rejected, ok := ruleskit.RequireRange(request.Intent.ID, "skill", *args.Skill, 0, 500); !ok {
		return rejected, nil
	}
	if rejected, ok := ruleskit.RequireRange(request.Intent.ID, "modifier", args.Modifier, -500, 500); !ok {
		return rejected, nil
	}
	target := *args.Skill + args.Modifier
	if target < 0 {
		target = 0
	}
	if target > 500 {
		target = 500
	}
	return ruleskit.RandomStep(request.Intent.ID, continuation{
		SchemaVersion: 1, Action: ActionSkillTest,
		Skill: *args.Skill, Modifier: args.Modifier, Target: target,
	}, 1, 100)
}

func resume(request core.ResumeRequest) (core.Step, error) {
	if request.Pending.Kind != core.StepKindNeedRandom {
		return core.Step{}, fmt.Errorf("runequest: cannot resume step kind %q", request.Pending.Kind)
	}
	var state continuation
	if err := ruleskit.DecodeRequired(request.Pending.State, &state, "schema_version", "action", "skill", "modifier", "target"); err != nil {
		return core.Step{}, fmt.Errorf("runequest: decode continuation: %w", err)
	}
	expectedTarget := state.Skill + state.Modifier
	if expectedTarget < 0 {
		expectedTarget = 0
	}
	if expectedTarget > 500 {
		expectedTarget = 500
	}
	if state.SchemaVersion != 1 || state.Action != ActionSkillTest || state.Skill < 0 || state.Skill > 500 || state.Modifier < -500 || state.Modifier > 500 || state.Target != expectedTarget {
		return core.Step{}, fmt.Errorf("runequest: invalid skill-check continuation")
	}
	rolls, err := ruleskit.DecodeRolls(request.Response.Data, 1, 100)
	if err != nil {
		return core.Step{}, fmt.Errorf("runequest: %w", err)
	}
	criticalMax := percentageCeiling(state.Target, 20)
	specialMax := percentageCeiling(state.Target, 5)
	fumbleMinimum := fumbleMinimum(state.Target)
	outcome := OutcomeFailure
	switch roll := rolls[0]; {
	case roll >= fumbleMinimum:
		outcome = OutcomeFumble
	case criticalMax > 0 && roll <= criticalMax:
		outcome = OutcomeCritical
	case specialMax > 0 && roll <= specialMax:
		outcome = OutcomeSpecial
	case roll <= state.Target:
		outcome = OutcomeSuccess
	}
	return ruleskit.Complete(request.Pending.StepID, "runequest.skill."+string(outcome), CheckResult{
		Roll: rolls[0], Skill: state.Skill, Modifier: state.Modifier, Target: state.Target,
		CriticalMax: criticalMax, SpecialMax: specialMax, FumbleMinimum: fumbleMinimum, Outcome: outcome,
	})
}

func percentageCeiling(value, divisor int) int {
	if value <= 0 {
		return 0
	}
	return (value + divisor - 1) / divisor
}

func fumbleMinimum(target int) int {
	if target <= 0 {
		return 96
	}
	step := (target - 1) / 20
	if step > 4 {
		step = 4
	}
	return 96 + step
}

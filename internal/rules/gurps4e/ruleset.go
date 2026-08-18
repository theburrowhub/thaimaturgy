package gurps4e

import (
	_ "embed"
	"fmt"

	core "github.com/theburrowhub/thaimaturgy/internal/rules"
	"github.com/theburrowhub/thaimaturgy/internal/rules/ruleskit"
)

const (
	PackageID      = "gurps4e"
	PackageVersion = "0.1.0"
	ActionCheck    = "skill.check"
)

//go:embed ruleset.go
var artifactSource string

// Outcome classifies a 3d6 check.
type Outcome string

const (
	OutcomeCriticalFailure Outcome = "critical_failure"
	OutcomeFailure         Outcome = "failure"
	OutcomeSuccess         Outcome = "success"
	OutcomeCriticalSuccess Outcome = "critical_success"
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

// CheckResult is the structured 3d6 result.
type CheckResult struct {
	Rolls    []int   `json:"rolls"`
	Total    int     `json:"total"`
	Skill    int     `json:"skill"`
	Modifier int     `json:"modifier"`
	Target   int     `json:"target"`
	Margin   int     `json:"margin"`
	Success  bool    `json:"success"`
	Critical bool    `json:"critical"`
	Outcome  Outcome `json:"outcome"`
}

// Ruleset is the built-in GURPS 4e core resolver.
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
		Label:       "Resolve 3d6 check",
		Description: "Roll 3d6 under effective skill and report margin and critical status.",
		InputSchema: ruleskit.MustPayload(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"skill":    map[string]any{"type": "integer", "minimum": 0, "maximum": 50},
				"modifier": map[string]any{"type": "integer", "minimum": -50, "maximum": 50},
			},
			"required":             []string{"skill"},
			"additionalProperties": false,
		}),
		Tags: []string{"random", "roll_under", "check"},
	}
	return &Ruleset{Engine: ruleskit.NewEngine(ruleskit.Definition{
		Artifact: artifact,
		Actions:  []core.ActionDescriptor{action},
		Explanations: map[string]string{
			ActionCheck: "Roll 3d6 at or below effective skill. Margin is target minus roll. 3-4 are critical successes; 5 at target 15+ and 6 at target 16+ are also critical. 18, some 17s, or failure by 10+ are critical failures.",
		},
		Start:  start,
		Resume: resume,
	})}
}

// NewArtifact returns the stable host-verifiable built-in artifact.
func NewArtifact() (core.Artifact, error) {
	return ruleskit.NewArtifact(PackageID, "GURPS 4e core resolution", "3d6 roll-under margin and critical primitive; no compendium content.", PackageVersion, artifactSource, []string{ActionCheck})
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
	if rejected, ok := ruleskit.RequireRange(request.Intent.ID, "skill", *args.Skill, 0, 50); !ok {
		return rejected, nil
	}
	if rejected, ok := ruleskit.RequireRange(request.Intent.ID, "modifier", args.Modifier, -50, 50); !ok {
		return rejected, nil
	}
	return ruleskit.RandomStep(request.Intent.ID, continuation{
		SchemaVersion: 1, Action: ActionCheck,
		Skill: *args.Skill, Modifier: args.Modifier, Target: *args.Skill + args.Modifier,
	}, 3, 6)
}

func resume(request core.ResumeRequest) (core.Step, error) {
	if request.Pending.Kind != core.StepKindNeedRandom {
		return core.Step{}, fmt.Errorf("gurps4e: cannot resume step kind %q", request.Pending.Kind)
	}
	var state continuation
	if err := ruleskit.DecodeRequired(request.Pending.State, &state, "schema_version", "action", "skill", "modifier", "target"); err != nil {
		return core.Step{}, fmt.Errorf("gurps4e: decode continuation: %w", err)
	}
	if state.SchemaVersion != 1 || state.Action != ActionCheck || state.Skill < 0 || state.Skill > 50 || state.Modifier < -50 || state.Modifier > 50 || state.Target != state.Skill+state.Modifier {
		return core.Step{}, fmt.Errorf("gurps4e: invalid skill-check continuation")
	}
	rolls, err := ruleskit.DecodeRolls(request.Response.Data, 3, 6)
	if err != nil {
		return core.Step{}, fmt.Errorf("gurps4e: %w", err)
	}
	total := rolls[0] + rolls[1] + rolls[2]
	criticalSuccess := total <= 4 || total == 5 && state.Target >= 15 || total == 6 && state.Target >= 16
	criticalFailure := total == 18 || total == 17 && state.Target <= 15 || total-state.Target >= 10
	success := criticalSuccess || !criticalFailure && total <= state.Target
	outcome := OutcomeFailure
	switch {
	case criticalSuccess:
		outcome = OutcomeCriticalSuccess
	case criticalFailure:
		outcome = OutcomeCriticalFailure
	case success:
		outcome = OutcomeSuccess
	}
	return ruleskit.Complete(request.Pending.StepID, "gurps4e.skill."+string(outcome), CheckResult{
		Rolls: append([]int(nil), rolls...), Total: total, Skill: state.Skill, Modifier: state.Modifier,
		Target: state.Target, Margin: state.Target - total, Success: success,
		Critical: criticalSuccess || criticalFailure, Outcome: outcome,
	})
}

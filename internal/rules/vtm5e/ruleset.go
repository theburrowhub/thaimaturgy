package vtm5e

import (
	"fmt"

	core "github.com/theburrowhub/thaimaturgy/internal/rules"
	"github.com/theburrowhub/thaimaturgy/internal/rules/ruleskit"
)

const (
	PackageID       = "vtm5e"
	PackageVersion  = "0.1.0"
	ActionPoolCheck = "pool.check"
)

const artifactMaterial = "thaimaturgy builtin rules artifact\npackage=vtm5e\nversion=0.1.0\nprotocol=1.0.0\nactions=pool.check\nabi=1\n"

// Outcome summarizes the mutually significant V5 result state.
type Outcome string

const (
	OutcomeFailure         Outcome = "failure"
	OutcomeBestialFailure  Outcome = "bestial_failure"
	OutcomeSuccess         Outcome = "success"
	OutcomeCriticalSuccess Outcome = "critical_success"
	OutcomeMessyCritical   Outcome = "messy_critical"
)

type arguments struct {
	Pool       *int `json:"pool"`
	Hunger     *int `json:"hunger"`
	Difficulty *int `json:"difficulty"`
}

type continuation struct {
	SchemaVersion uint32 `json:"schema_version"`
	Action        string `json:"action"`
	Pool          int    `json:"pool"`
	Hunger        int    `json:"hunger"`
	Difficulty    int    `json:"difficulty"`
}

// CheckResult is the structured V5 dice-pool result. Hunger dice occupy the
// first Hunger positions in Rolls, a convention fixed in the continuation.
type CheckResult struct {
	Rolls         []int   `json:"rolls"`
	Pool          int     `json:"pool"`
	Hunger        int     `json:"hunger"`
	Difficulty    int     `json:"difficulty"`
	BaseSuccesses int     `json:"base_successes"`
	CriticalPairs int     `json:"critical_pairs"`
	Successes     int     `json:"successes"`
	Margin        int     `json:"margin"`
	Messy         bool    `json:"messy"`
	Bestial       bool    `json:"bestial"`
	Outcome       Outcome `json:"outcome"`
}

// Ruleset is the built-in V5 core resolver.
type Ruleset struct{ *ruleskit.Engine }

var _ core.Ruleset = (*Ruleset)(nil)

// New returns a fresh stateless ruleset instance.
func New() *Ruleset {
	artifact, err := NewArtifact()
	if err != nil {
		panic(err)
	}
	action := core.ActionDescriptor{
		ID:          ActionPoolCheck,
		Label:       "Resolve hunger dice pool",
		Description: "Roll a d10 pool, count successes and critical pairs, and identify messy criticals or bestial failures.",
		InputSchema: ruleskit.MustPayload(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pool":       map[string]any{"type": "integer", "minimum": 1, "maximum": 100},
				"hunger":     map[string]any{"type": "integer", "minimum": 0, "maximum": 5},
				"difficulty": map[string]any{"type": "integer", "minimum": 1, "maximum": 100},
			},
			"required":             []string{"pool", "hunger", "difficulty"},
			"additionalProperties": false,
		}),
		Tags: []string{"random", "dice_pool", "check"},
	}
	return &Ruleset{Engine: ruleskit.NewEngine(ruleskit.Definition{
		Artifact: artifact,
		Actions:  []core.ActionDescriptor{action},
		Explanations: map[string]string{
			ActionPoolCheck: "Each d10 result of 6 or more is a success. Every pair of tens is worth four successes total. A critical containing a hunger ten is messy; a failed roll containing a hunger one is a bestial failure.",
		},
		Start:  start,
		Resume: resume,
	})}
}

// NewArtifact returns the stable host-verifiable built-in artifact.
func NewArtifact() (core.Artifact, error) {
	return ruleskit.NewArtifact(PackageID, "Vampire 5e core resolution", "Hunger-aware d10 dice-pool primitive; no compendium or setting content.", PackageVersion, artifactMaterial, []string{ActionPoolCheck})
}

// InitialState returns this stateless package's canonical state.
func InitialState() core.Payload { return ruleskit.InitialState() }

func start(request core.StartRequest) (core.Step, error) {
	var args arguments
	if err := ruleskit.Decode(request.Intent.Arguments, &args); err != nil {
		return ruleskit.RejectArguments(request.Intent.ID, err)
	}
	if args.Pool == nil || args.Hunger == nil || args.Difficulty == nil {
		return ruleskit.Reject(request.Intent.ID, "invalid.arguments", "pool, hunger, and difficulty are required"), nil
	}
	if rejected, ok := ruleskit.RequireRange(request.Intent.ID, "pool", *args.Pool, 1, 100); !ok {
		return rejected, nil
	}
	if rejected, ok := ruleskit.RequireRange(request.Intent.ID, "hunger", *args.Hunger, 0, 5); !ok {
		return rejected, nil
	}
	if *args.Hunger > *args.Pool {
		return ruleskit.Reject(request.Intent.ID, "invalid.arguments", "hunger cannot exceed pool"), nil
	}
	if rejected, ok := ruleskit.RequireRange(request.Intent.ID, "difficulty", *args.Difficulty, 1, 100); !ok {
		return rejected, nil
	}
	return ruleskit.RandomStep(request.Intent.ID, continuation{
		SchemaVersion: 1, Action: ActionPoolCheck,
		Pool: *args.Pool, Hunger: *args.Hunger, Difficulty: *args.Difficulty,
	}, *args.Pool, 10)
}

func resume(request core.ResumeRequest) (core.Step, error) {
	if request.Pending.Kind != core.StepKindNeedRandom {
		return core.Step{}, fmt.Errorf("vtm5e: cannot resume step kind %q", request.Pending.Kind)
	}
	var state continuation
	if err := ruleskit.DecodeRequired(request.Pending.State, &state, "schema_version", "action", "pool", "hunger", "difficulty"); err != nil {
		return core.Step{}, fmt.Errorf("vtm5e: decode continuation: %w", err)
	}
	if state.SchemaVersion != 1 || state.Action != ActionPoolCheck || state.Pool < 1 || state.Pool > 100 || state.Hunger < 0 || state.Hunger > 5 || state.Hunger > state.Pool || state.Difficulty < 1 || state.Difficulty > 100 {
		return core.Step{}, fmt.Errorf("vtm5e: invalid pool-check continuation")
	}
	rolls, err := ruleskit.DecodeRolls(request.Response.Data, state.Pool, 10)
	if err != nil {
		return core.Step{}, fmt.Errorf("vtm5e: %w", err)
	}
	baseSuccesses, tens := 0, 0
	hungerTen, hungerOne := false, false
	for i, roll := range rolls {
		if roll >= 6 {
			baseSuccesses++
		}
		if roll == 10 {
			tens++
		}
		if i < state.Hunger {
			hungerTen = hungerTen || roll == 10
			hungerOne = hungerOne || roll == 1
		}
	}
	criticalPairs := tens / 2
	successes := baseSuccesses + criticalPairs*2
	margin := successes - state.Difficulty
	passed := margin >= 0
	messy := passed && criticalPairs > 0 && hungerTen
	bestial := !passed && hungerOne
	outcome := OutcomeFailure
	switch {
	case messy:
		outcome = OutcomeMessyCritical
	case passed && criticalPairs > 0:
		outcome = OutcomeCriticalSuccess
	case passed:
		outcome = OutcomeSuccess
	case bestial:
		outcome = OutcomeBestialFailure
	}
	return ruleskit.Complete(request.Pending.StepID, "vtm5e.pool."+string(outcome), CheckResult{
		Rolls: append([]int(nil), rolls...), Pool: state.Pool, Hunger: state.Hunger, Difficulty: state.Difficulty,
		BaseSuccesses: baseSuccesses, CriticalPairs: criticalPairs, Successes: successes, Margin: margin,
		Messy: messy, Bestial: bestial, Outcome: outcome,
	})
}

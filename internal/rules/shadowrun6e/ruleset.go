package shadowrun6e

import (
	_ "embed"
	"fmt"

	core "github.com/theburrowhub/thaimaturgy/internal/rules"
	"github.com/theburrowhub/thaimaturgy/internal/rules/ruleskit"
)

const (
	PackageID       = "shadowrun6e"
	PackageVersion  = "0.1.0"
	ActionPoolCheck = "pool.check"
)

//go:embed ruleset.go
var artifactSource string

// Outcome summarizes success and glitch state.
type Outcome string

const (
	OutcomeFailure        Outcome = "failure"
	OutcomeSuccess        Outcome = "success"
	OutcomeGlitchFailure  Outcome = "glitch_failure"
	OutcomeGlitchSuccess  Outcome = "glitch_success"
	OutcomeCriticalGlitch Outcome = "critical_glitch"
)

type arguments struct {
	Pool      *int `json:"pool"`
	Threshold *int `json:"threshold"`
}

type continuation struct {
	SchemaVersion uint32 `json:"schema_version"`
	Action        string `json:"action"`
	Pool          int    `json:"pool"`
	Threshold     int    `json:"threshold"`
}

// CheckResult is the structured d6-pool result.
type CheckResult struct {
	Rolls          []int   `json:"rolls"`
	Pool           int     `json:"pool"`
	Threshold      int     `json:"threshold"`
	Hits           int     `json:"hits"`
	Ones           int     `json:"ones"`
	Margin         int     `json:"margin"`
	Success        bool    `json:"success"`
	Glitch         bool    `json:"glitch"`
	CriticalGlitch bool    `json:"critical_glitch"`
	Outcome        Outcome `json:"outcome"`
}

// Ruleset is the built-in Shadowrun 6e core resolver.
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
		Label:       "Resolve d6 pool",
		Description: "Count 5-6 as hits against a threshold and detect glitches from ones.",
		InputSchema: ruleskit.MustPayload(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pool":      map[string]any{"type": "integer", "minimum": 1, "maximum": 100},
				"threshold": map[string]any{"type": "integer", "minimum": 0, "maximum": 100},
			},
			"required":             []string{"pool", "threshold"},
			"additionalProperties": false,
		}),
		Tags: []string{"random", "dice_pool", "check"},
	}
	return &Ruleset{Engine: ruleskit.NewEngine(ruleskit.Definition{
		Artifact: artifact,
		Actions:  []core.ActionDescriptor{action},
		Explanations: map[string]string{
			ActionPoolCheck: "Each 5 or 6 is a hit. Meeting the threshold succeeds. More than half the dice showing 1 causes a glitch; a glitch with no hits is critical.",
		},
		Start:  start,
		Resume: resume,
	})}
}

// NewArtifact returns the stable host-verifiable built-in artifact.
func NewArtifact() (core.Artifact, error) {
	return ruleskit.NewArtifact(PackageID, "Shadowrun 6e core resolution", "d6 hits and glitches primitive; no compendium or setting content.", PackageVersion, artifactSource, []string{ActionPoolCheck})
}

// InitialState returns this stateless package's canonical state.
func InitialState() core.Payload { return ruleskit.InitialState() }

func start(request core.StartRequest) (core.Step, error) {
	var args arguments
	if err := ruleskit.Decode(request.Intent.Arguments, &args); err != nil {
		return ruleskit.RejectArguments(request.Intent.ID, err)
	}
	if args.Pool == nil || args.Threshold == nil {
		return ruleskit.Reject(request.Intent.ID, "invalid.arguments", "pool and threshold are required"), nil
	}
	if rejected, ok := ruleskit.RequireRange(request.Intent.ID, "pool", *args.Pool, 1, 100); !ok {
		return rejected, nil
	}
	if rejected, ok := ruleskit.RequireRange(request.Intent.ID, "threshold", *args.Threshold, 0, 100); !ok {
		return rejected, nil
	}
	return ruleskit.RandomStep(request.Intent.ID, continuation{
		SchemaVersion: 1, Action: ActionPoolCheck, Pool: *args.Pool, Threshold: *args.Threshold,
	}, *args.Pool, 6)
}

func resume(request core.ResumeRequest) (core.Step, error) {
	if request.Pending.Kind != core.StepKindNeedRandom {
		return core.Step{}, fmt.Errorf("shadowrun6e: cannot resume step kind %q", request.Pending.Kind)
	}
	var state continuation
	if err := ruleskit.DecodeRequired(request.Pending.State, &state, "schema_version", "action", "pool", "threshold"); err != nil {
		return core.Step{}, fmt.Errorf("shadowrun6e: decode continuation: %w", err)
	}
	if state.SchemaVersion != 1 || state.Action != ActionPoolCheck || state.Pool < 1 || state.Pool > 100 || state.Threshold < 0 || state.Threshold > 100 {
		return core.Step{}, fmt.Errorf("shadowrun6e: invalid pool-check continuation")
	}
	rolls, err := ruleskit.DecodeRolls(request.Response.Data, state.Pool, 6)
	if err != nil {
		return core.Step{}, fmt.Errorf("shadowrun6e: %w", err)
	}
	hits, ones := 0, 0
	for _, roll := range rolls {
		if roll >= 5 {
			hits++
		}
		if roll == 1 {
			ones++
		}
	}
	success := hits >= state.Threshold
	glitch := ones*2 > state.Pool
	criticalGlitch := glitch && hits == 0
	outcome := OutcomeFailure
	switch {
	case criticalGlitch:
		outcome = OutcomeCriticalGlitch
	case glitch && success:
		outcome = OutcomeGlitchSuccess
	case glitch:
		outcome = OutcomeGlitchFailure
	case success:
		outcome = OutcomeSuccess
	}
	return ruleskit.Complete(request.Pending.StepID, "shadowrun6e.pool."+string(outcome), CheckResult{
		Rolls: append([]int(nil), rolls...), Pool: state.Pool, Threshold: state.Threshold,
		Hits: hits, Ones: ones, Margin: hits - state.Threshold, Success: success,
		Glitch: glitch, CriticalGlitch: criticalGlitch, Outcome: outcome,
	})
}

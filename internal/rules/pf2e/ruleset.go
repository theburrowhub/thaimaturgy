package pf2e

import (
	"fmt"

	core "github.com/theburrowhub/thaimaturgy/internal/rules"
	"github.com/theburrowhub/thaimaturgy/internal/rules/ruleskit"
)

const (
	PackageID      = "pf2e"
	PackageVersion = "0.1.0"
	ActionCheck    = "check.resolve"
)

const artifactMaterial = "thaimaturgy builtin rules artifact\npackage=pf2e\nversion=0.1.0\nprotocol=1.0.0\nactions=check.resolve\nabi=1\n"

// Degree is one of PF2e's ordered degrees of success.
type Degree string

const (
	CriticalFailure Degree = "critical_failure"
	Failure         Degree = "failure"
	Success         Degree = "success"
	CriticalSuccess Degree = "critical_success"
)

type checkArguments struct {
	Modifier *int `json:"modifier"`
	DC       *int `json:"dc"`
}

type continuation struct {
	SchemaVersion uint32 `json:"schema_version"`
	Action        string `json:"action"`
	Modifier      int    `json:"modifier"`
	DC            int    `json:"dc"`
}

// CheckResult is the structured result of ActionCheck.
type CheckResult struct {
	Roll              int    `json:"roll"`
	Modifier          int    `json:"modifier"`
	DC                int    `json:"dc"`
	Total             int    `json:"total"`
	BaseDegree        Degree `json:"base_degree"`
	Degree            Degree `json:"degree"`
	NaturalAdjustment int    `json:"natural_adjustment"`
}

// Ruleset is the built-in PF2e resolution package.
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
		Label:       "Resolve check",
		Description: "Roll d20 + modifier against a DC and determine one of four degrees of success.",
		InputSchema: ruleskit.MustPayload(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"modifier": map[string]any{"type": "integer", "minimum": -100, "maximum": 100},
				"dc":       map[string]any{"type": "integer", "minimum": 0, "maximum": 1000},
			},
			"required":             []string{"modifier", "dc"},
			"additionalProperties": false,
		}),
		Tags: []string{"random", "check"},
	}
	return &Ruleset{Engine: ruleskit.NewEngine(ruleskit.Definition{
		Artifact: artifact,
		Actions:  []core.ActionDescriptor{action},
		Explanations: map[string]string{
			ActionCheck: "Compare d20 + modifier with the DC: DC+10 is a critical success and DC-10 is a critical failure. A natural 20 raises the degree once; a natural 1 lowers it once.",
		},
		Start:  start,
		Resume: resume,
	})}
}

// NewArtifact returns the stable host-verifiable built-in artifact.
func NewArtifact() (core.Artifact, error) {
	return ruleskit.NewArtifact(PackageID, "PF2e core resolution", "Four-degree d20 check primitive; no compendium or setting content.", PackageVersion, artifactMaterial, []string{ActionCheck})
}

// InitialState returns this stateless package's canonical state.
func InitialState() core.Payload { return ruleskit.InitialState() }

func start(request core.StartRequest) (core.Step, error) {
	var arguments checkArguments
	if err := ruleskit.Decode(request.Intent.Arguments, &arguments); err != nil {
		return ruleskit.RejectArguments(request.Intent.ID, err)
	}
	if arguments.Modifier == nil || arguments.DC == nil {
		return ruleskit.Reject(request.Intent.ID, "invalid.arguments", "modifier and dc are required"), nil
	}
	if rejected, ok := ruleskit.RequireRange(request.Intent.ID, "modifier", *arguments.Modifier, -100, 100); !ok {
		return rejected, nil
	}
	if rejected, ok := ruleskit.RequireRange(request.Intent.ID, "dc", *arguments.DC, 0, 1000); !ok {
		return rejected, nil
	}
	return ruleskit.RandomStep(request.Intent.ID, continuation{
		SchemaVersion: 1,
		Action:        ActionCheck,
		Modifier:      *arguments.Modifier,
		DC:            *arguments.DC,
	}, 1, 20)
}

func resume(request core.ResumeRequest) (core.Step, error) {
	if request.Pending.Kind != core.StepKindNeedRandom {
		return core.Step{}, fmt.Errorf("pf2e: cannot resume step kind %q", request.Pending.Kind)
	}
	var state continuation
	if err := ruleskit.DecodeRequired(request.Pending.State, &state, "schema_version", "action", "modifier", "dc"); err != nil {
		return core.Step{}, fmt.Errorf("pf2e: decode continuation: %w", err)
	}
	if state.SchemaVersion != 1 || state.Action != ActionCheck || state.Modifier < -100 || state.Modifier > 100 || state.DC < 0 || state.DC > 1000 {
		return core.Step{}, fmt.Errorf("pf2e: invalid check continuation")
	}
	rolls, err := ruleskit.DecodeRolls(request.Response.Data, 1, 20)
	if err != nil {
		return core.Step{}, fmt.Errorf("pf2e: %w", err)
	}
	total := rolls[0] + state.Modifier
	base := degreeFor(total, state.DC)
	degree := base
	adjustment := 0
	if rolls[0] == 20 {
		degree = shift(base, 1)
		adjustment = 1
	} else if rolls[0] == 1 {
		degree = shift(base, -1)
		adjustment = -1
	}
	return ruleskit.Complete(request.Pending.StepID, "pf2e.check."+string(degree), CheckResult{
		Roll: rolls[0], Modifier: state.Modifier, DC: state.DC, Total: total,
		BaseDegree: base, Degree: degree, NaturalAdjustment: adjustment,
	})
}

func degreeFor(total, dc int) Degree {
	switch {
	case total >= dc+10:
		return CriticalSuccess
	case total >= dc:
		return Success
	case total <= dc-10:
		return CriticalFailure
	default:
		return Failure
	}
}

func shift(degree Degree, amount int) Degree {
	degrees := []Degree{CriticalFailure, Failure, Success, CriticalSuccess}
	index := 0
	for i, candidate := range degrees {
		if candidate == degree {
			index = i
			break
		}
	}
	index += amount
	if index < 0 {
		index = 0
	}
	if index >= len(degrees) {
		index = len(degrees) - 1
	}
	return degrees[index]
}

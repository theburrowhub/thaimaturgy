package reference_test

import (
	"context"
	"encoding/json"
	"testing"

	core "github.com/theburrowhub/thaimaturgy/internal/rules"
	"github.com/theburrowhub/thaimaturgy/internal/rules/coc7e"
	"github.com/theburrowhub/thaimaturgy/internal/rules/fatecore"
	"github.com/theburrowhub/thaimaturgy/internal/rules/gurps4e"
	"github.com/theburrowhub/thaimaturgy/internal/rules/pbta"
	"github.com/theburrowhub/thaimaturgy/internal/rules/pf2e"
	"github.com/theburrowhub/thaimaturgy/internal/rules/ruleskit"
	"github.com/theburrowhub/thaimaturgy/internal/rules/runequest"
	"github.com/theburrowhub/thaimaturgy/internal/rules/savageworlds"
	"github.com/theburrowhub/thaimaturgy/internal/rules/shadowrun6e"
	"github.com/theburrowhub/thaimaturgy/internal/rules/vtm5e"
)

func TestPF2eNaturalFacesShiftOneDegree(t *testing.T) {
	tests := []struct {
		name       string
		arguments  any
		roll       int
		wantBase   pf2e.Degree
		wantDegree pf2e.Degree
		wantShift  int
	}{
		{"natural 20 raises failure", map[string]any{"modifier": 0, "dc": 25}, 20, pf2e.Failure, pf2e.Success, 1},
		{"natural 1 lowers critical success", map[string]any{"modifier": 30, "dc": 20}, 1, pf2e.CriticalSuccess, pf2e.Success, -1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			artifact := mustArtifact(t, pf2e.NewArtifact)
			snapshot := testSnapshot(artifact, pf2e.InitialState())
			step := startMechanic(t, func() core.Ruleset { return pf2e.New() }, snapshot, pf2e.ActionCheck, test.arguments)
			complete := resumeRandom(t, func() core.Ruleset { return pf2e.New() }, snapshot, step, []int{test.roll})
			result := decodeCompletion[pf2e.CheckResult](t, complete)
			if result.BaseDegree != test.wantBase || result.Degree != test.wantDegree || result.NaturalAdjustment != test.wantShift {
				t.Fatalf("result = %#v", result)
			}
		})
	}
}

func TestRuneQuestDegreeBands(t *testing.T) {
	tests := []struct {
		roll int
		want runequest.Outcome
	}{
		{1, runequest.OutcomeCritical},
		{12, runequest.OutcomeSpecial},
		{50, runequest.OutcomeSuccess},
		{90, runequest.OutcomeFailure},
		{98, runequest.OutcomeFumble},
	}
	artifact := mustArtifact(t, runequest.NewArtifact)
	snapshot := testSnapshot(artifact, runequest.InitialState())
	for _, test := range tests {
		t.Run(string(test.want), func(t *testing.T) {
			step := startMechanic(t, func() core.Ruleset { return runequest.New() }, snapshot, runequest.ActionSkillTest, map[string]any{"skill": 60})
			complete := resumeRandom(t, func() core.Ruleset { return runequest.New() }, snapshot, step, []int{test.roll})
			result := decodeCompletion[runequest.CheckResult](t, complete)
			if result.Outcome != test.want || result.CriticalMax != 3 || result.SpecialMax != 12 || result.FumbleMinimum != 98 {
				t.Fatalf("result = %#v", result)
			}
		})
	}
}

func TestCoC7eBonusPenaltyAndDifficulty(t *testing.T) {
	tests := []struct {
		name      string
		skill     int
		bonus     int
		rolls     []int
		wantRoll  int
		wantLevel coc7e.SuccessLevel
		wantPass  bool
	}{
		{"bonus chooses low tens", 60, 1, []int{3, 8, 2}, 23, coc7e.LevelHard, true},
		{"penalty chooses high tens", 60, -1, []int{3, 8, 2}, 83, coc7e.LevelFailure, false},
		{"one is critical", 40, 0, []int{1, 10}, 1, coc7e.LevelCritical, true},
		{"low skill upper band fumbles", 40, 0, []int{8, 9}, 98, coc7e.LevelFumble, false},
	}
	artifact := mustArtifact(t, coc7e.NewArtifact)
	snapshot := testSnapshot(artifact, coc7e.InitialState())
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			step := startMechanic(t, func() core.Ruleset { return coc7e.New() }, snapshot, coc7e.ActionCheck, map[string]any{
				"skill": test.skill, "difficulty": "hard", "bonus_dice": test.bonus,
			})
			complete := resumeRandom(t, func() core.Ruleset { return coc7e.New() }, snapshot, step, test.rolls)
			result := decodeCompletion[coc7e.CheckResult](t, complete)
			if result.Selected != test.wantRoll || result.Level != test.wantLevel || result.Passed != test.wantPass {
				t.Fatalf("result = %#v", result)
			}
		})
	}
}

func TestVTM5eHungerComplications(t *testing.T) {
	tests := []struct {
		name        string
		arguments   any
		rolls       []int
		want        vtm5e.Outcome
		wantMessy   bool
		wantBestial bool
	}{
		{"messy critical", map[string]any{"pool": 4, "hunger": 1, "difficulty": 2}, []int{10, 10, 2, 6}, vtm5e.OutcomeMessyCritical, true, false},
		{"clean critical", map[string]any{"pool": 3, "hunger": 0, "difficulty": 2}, []int{10, 10, 2}, vtm5e.OutcomeCriticalSuccess, false, false},
		{"bestial failure", map[string]any{"pool": 3, "hunger": 1, "difficulty": 1}, []int{1, 2, 3}, vtm5e.OutcomeBestialFailure, false, true},
	}
	artifact := mustArtifact(t, vtm5e.NewArtifact)
	snapshot := testSnapshot(artifact, vtm5e.InitialState())
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			step := startMechanic(t, func() core.Ruleset { return vtm5e.New() }, snapshot, vtm5e.ActionPoolCheck, test.arguments)
			complete := resumeRandom(t, func() core.Ruleset { return vtm5e.New() }, snapshot, step, test.rolls)
			result := decodeCompletion[vtm5e.CheckResult](t, complete)
			if result.Outcome != test.want || result.Messy != test.wantMessy || result.Bestial != test.wantBestial {
				t.Fatalf("result = %#v", result)
			}
		})
	}
}

func TestShadowrun6eGlitchesCanCoincideWithSuccess(t *testing.T) {
	tests := []struct {
		name  string
		rolls []int
		want  shadowrun6e.Outcome
	}{
		{"critical glitch", []int{1, 1, 1, 2, 3}, shadowrun6e.OutcomeCriticalGlitch},
		{"successful glitch", []int{1, 1, 1, 5, 2}, shadowrun6e.OutcomeGlitchSuccess},
	}
	artifact := mustArtifact(t, shadowrun6e.NewArtifact)
	snapshot := testSnapshot(artifact, shadowrun6e.InitialState())
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			step := startMechanic(t, func() core.Ruleset { return shadowrun6e.New() }, snapshot, shadowrun6e.ActionPoolCheck, map[string]any{"pool": 5, "threshold": 1})
			complete := resumeRandom(t, func() core.Ruleset { return shadowrun6e.New() }, snapshot, step, test.rolls)
			result := decodeCompletion[shadowrun6e.CheckResult](t, complete)
			if result.Outcome != test.want || !result.Glitch {
				t.Fatalf("result = %#v", result)
			}
		})
	}
}

func TestPbtAWeakHitChoiceSurvivesRestart(t *testing.T) {
	artifact := mustArtifact(t, pbta.NewArtifact)
	snapshot := testSnapshot(artifact, pbta.InitialState())
	step := startMechanic(t, func() core.Ruleset { return pbta.New() }, snapshot, pbta.ActionMove, map[string]any{
		"modifier": 1,
		"mixed_options": []map[string]any{
			{"id": "pay_cost", "label": "Pay a cost"},
			{"id": "face_danger", "label": "Face a danger"},
		},
	})
	decision := resumeRandom(t, func() core.Ruleset { return pbta.New() }, snapshot, step, []int{3, 3})
	if decision.Kind != core.StepKindNeedDecision || decision.NeedDecision.Authority != testPrincipal().ID || len(decision.NeedDecision.Options) != 2 {
		t.Fatalf("decision = %#v", decision)
	}
	decision = roundTripStep(t, decision)
	pending := mustPending(t, decision)
	badRequest := core.ResumeRequest{
		Snapshot: snapshot, Principal: testPrincipal(), Pending: pending,
		Response: core.HostResponse{StepID: decision.ID, Kind: decision.Kind, Data: mustPayload(t, map[string]any{"option_id": "not_offered"})},
	}
	if _, err := pbta.New().Resume(context.Background(), badRequest); err == nil {
		t.Fatal("unoffered PbtA decision was accepted")
	}
	goodRequest := badRequest
	goodRequest.Response.Data = mustPayload(t, map[string]any{"option_id": "pay_cost"})
	wrongAuthority := goodRequest
	wrongAuthority.Principal = core.Principal{ID: "player-2", Kind: "player"}
	if _, err := pbta.New().Resume(context.Background(), wrongAuthority); err == nil {
		t.Fatal("decision response from a different authority was accepted")
	}
	complete, err := pbta.New().Resume(context.Background(), goodRequest)
	if err != nil {
		t.Fatal(err)
	}
	result := decodeCompletion[pbta.MoveResult](t, complete)
	if result.Band != pbta.BandWeakHit || result.Total != 7 || result.ChosenOption == nil || result.ChosenOption.ID != "pay_cost" {
		t.Fatalf("result = %#v", result)
	}
}

func TestGURPS4eCriticalThresholds(t *testing.T) {
	tests := []struct {
		name  string
		skill int
		rolls []int
		want  gurps4e.Outcome
	}{
		{"three is always critical success", 10, []int{1, 1, 1}, gurps4e.OutcomeCriticalSuccess},
		{"six critical at skill sixteen", 16, []int{2, 2, 2}, gurps4e.OutcomeCriticalSuccess},
		{"eighteen always critical failure", 20, []int{6, 6, 6}, gurps4e.OutcomeCriticalFailure},
	}
	artifact := mustArtifact(t, gurps4e.NewArtifact)
	snapshot := testSnapshot(artifact, gurps4e.InitialState())
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			step := startMechanic(t, func() core.Ruleset { return gurps4e.New() }, snapshot, gurps4e.ActionCheck, map[string]any{"skill": test.skill})
			complete := resumeRandom(t, func() core.Ruleset { return gurps4e.New() }, snapshot, step, test.rolls)
			result := decodeCompletion[gurps4e.CheckResult](t, complete)
			if result.Outcome != test.want || !result.Critical {
				t.Fatalf("result = %#v", result)
			}
		})
	}
}

func TestFateCoreMapsFateDiceAndAppliesInvokes(t *testing.T) {
	artifact := mustArtifact(t, fatecore.NewArtifact)
	snapshot := testSnapshot(artifact, fatecore.InitialState())
	step := startMechanic(t, func() core.Ruleset { return fatecore.New() }, snapshot, fatecore.ActionResolve, map[string]any{
		"skill": 1, "opposition": 4, "invokes": []string{"Prepared", "On the High Ground"},
	})
	complete := resumeRandom(t, func() core.Ruleset { return fatecore.New() }, snapshot, step, []int{1, 2, 3, 3})
	result := decodeCompletion[fatecore.ActionResult](t, complete)
	wantDice := []int{-1, 0, 1, 1}
	if result.DiceTotal != 1 || result.InvokeBonus != 4 || result.Effort != 6 || result.Shifts != 2 || result.Outcome != fatecore.OutcomeSucceed || !equalInts(result.Dice, wantDice) {
		t.Fatalf("result = %#v", result)
	}
}

func TestSavageWorldsAcesBothDiceAndCountsRaises(t *testing.T) {
	artifact := mustArtifact(t, savageworlds.NewArtifact)
	snapshot := testSnapshot(artifact, savageworlds.InitialState())
	step := startMechanic(t, func() core.Ruleset { return savageworlds.New() }, snapshot, savageworlds.ActionTraitTest, map[string]any{
		"trait_die": 8, "wild_card": true, "modifier": 1, "target_number": 4,
	})
	step = resumeRandom(t, func() core.Ruleset { return savageworlds.New() }, snapshot, step, []int{8})
	if step.Kind != core.StepKindNeedRandom {
		t.Fatalf("trait ace did not request another roll: %#v", step)
	}
	step = resumeRandom(t, func() core.Ruleset { return savageworlds.New() }, snapshot, roundTripStep(t, step), []int{3})
	if step.Kind != core.StepKindNeedRandom {
		t.Fatalf("wild die was not requested: %#v", step)
	}
	step = resumeRandom(t, func() core.Ruleset { return savageworlds.New() }, snapshot, roundTripStep(t, step), []int{6})
	if step.Kind != core.StepKindNeedRandom {
		t.Fatalf("wild ace did not request another roll: %#v", step)
	}
	complete := resumeRandom(t, func() core.Ruleset { return savageworlds.New() }, snapshot, roundTripStep(t, step), []int{2})
	result := decodeCompletion[savageworlds.CheckResult](t, complete)
	if result.TraitTotal != 11 || result.WildTotal != 8 || result.SelectedDie != "trait" || result.FinalTotal != 12 || result.Raises != 2 || result.Outcome != savageworlds.OutcomeSuccessWithRaises {
		t.Fatalf("result = %#v", result)
	}
}

func TestSavageWorldsSnakeEyesOverridesModifier(t *testing.T) {
	artifact := mustArtifact(t, savageworlds.NewArtifact)
	snapshot := testSnapshot(artifact, savageworlds.InitialState())
	step := startMechanic(t, func() core.Ruleset { return savageworlds.New() }, snapshot, savageworlds.ActionTraitTest, map[string]any{
		"trait_die": 12, "wild_card": true, "modifier": 20, "target_number": 4,
	})
	step = resumeRandom(t, func() core.Ruleset { return savageworlds.New() }, snapshot, step, []int{1})
	complete := resumeRandom(t, func() core.Ruleset { return savageworlds.New() }, snapshot, step, []int{1})
	result := decodeCompletion[savageworlds.CheckResult](t, complete)
	if !result.SnakeEyes || result.Success || result.Outcome != savageworlds.OutcomeSnakeEyes {
		t.Fatalf("result = %#v", result)
	}
}

func startMechanic(t *testing.T, newRuleset func() core.Ruleset, snapshot core.Snapshot, action string, arguments any) core.Step {
	t.Helper()
	step, err := newRuleset().Start(context.Background(), core.StartRequest{
		Snapshot: snapshot, Principal: testPrincipal(),
		Intent: core.Intent{ID: "mechanic-test", ActionID: action, Arguments: mustPayload(t, arguments)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if step.Kind == core.StepKindReject {
		t.Fatalf("unexpected rejection: %#v", step.Reject)
	}
	return step
}

func resumeRandom(t *testing.T, newRuleset func() core.Ruleset, snapshot core.Snapshot, step core.Step, rolls []int) core.Step {
	t.Helper()
	if step.Kind != core.StepKindNeedRandom {
		t.Fatalf("step kind = %q, want need_random", step.Kind)
	}
	next, err := newRuleset().Resume(context.Background(), core.ResumeRequest{
		Snapshot: snapshot, Principal: testPrincipal(), Pending: mustPending(t, step),
		Response: core.HostResponse{StepID: step.ID, Kind: step.Kind, Data: mustPayload(t, ruleskit.DiceResponse{Rolls: rolls})},
	})
	if err != nil {
		t.Fatal(err)
	}
	return next
}

func decodeCompletion[T any](t *testing.T, step core.Step) T {
	t.Helper()
	if step.Kind != core.StepKindComplete || step.Complete == nil {
		t.Fatalf("step = %#v, want completion", step)
	}
	var result T
	if err := json.Unmarshal(step.Complete.Result.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func mustArtifact(t *testing.T, build func() (core.Artifact, error)) core.Artifact {
	t.Helper()
	artifact, err := build()
	if err != nil {
		t.Fatal(err)
	}
	return artifact
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

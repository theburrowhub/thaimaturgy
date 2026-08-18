package engine

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/rules"
	"github.com/theburrowhub/thaimaturgy/internal/rules/catalog"
	"github.com/theburrowhub/thaimaturgy/internal/rules/coc7e"
	"github.com/theburrowhub/thaimaturgy/internal/rules/dnd5e"
	"github.com/theburrowhub/thaimaturgy/internal/rules/fatecore"
	"github.com/theburrowhub/thaimaturgy/internal/rules/gurps4e"
	"github.com/theburrowhub/thaimaturgy/internal/rules/pbta"
	"github.com/theburrowhub/thaimaturgy/internal/rules/pf2e"
	"github.com/theburrowhub/thaimaturgy/internal/rules/runequest"
	"github.com/theburrowhub/thaimaturgy/internal/rules/savageworlds"
	"github.com/theburrowhub/thaimaturgy/internal/rules/shadowrun6e"
	"github.com/theburrowhub/thaimaturgy/internal/rules/vtm5e"
	"github.com/theburrowhub/thaimaturgy/internal/types"
)

type builtinGatewayCase struct {
	name         string
	artifact     func() (rules.Artifact, error)
	ruleset      func() rules.Ruleset
	initialState func() rules.Payload
	action       string
	arguments    any
	rolls        [][]int
	wantOutcome  string
}

func builtinGatewayCases() []builtinGatewayCase {
	return []builtinGatewayCase{
		{
			name: "dnd5e", artifact: dnd5e.NewArtifact, ruleset: func() rules.Ruleset { return dnd5e.New() }, initialState: dnd5e.InitialState,
			action: dnd5e.ActionDiceRoll, arguments: map[string]any{"notation": "1d20"}, rolls: [][]int{{12}}, wantOutcome: "dnd5e.dice.rolled",
		},
		{
			name: "pf2e", artifact: pf2e.NewArtifact, ruleset: func() rules.Ruleset { return pf2e.New() }, initialState: pf2e.InitialState,
			action: pf2e.ActionCheck, arguments: map[string]any{"modifier": 5, "dc": 20}, rolls: [][]int{{15}}, wantOutcome: "pf2e.check.success",
		},
		{
			name: "runequest", artifact: runequest.NewArtifact, ruleset: func() rules.Ruleset { return runequest.New() }, initialState: runequest.InitialState,
			action: runequest.ActionSkillTest, arguments: map[string]any{"skill": 60}, rolls: [][]int{{12}}, wantOutcome: "runequest.skill.special",
		},
		{
			name: "coc7e", artifact: coc7e.NewArtifact, ruleset: func() rules.Ruleset { return coc7e.New() }, initialState: coc7e.InitialState,
			action: coc7e.ActionCheck, arguments: map[string]any{"skill": 60, "difficulty": "hard", "bonus_dice": 1}, rolls: [][]int{{3, 8, 2}}, wantOutcome: "coc7e.check.hard_success",
		},
		{
			name: "vtm5e", artifact: vtm5e.NewArtifact, ruleset: func() rules.Ruleset { return vtm5e.New() }, initialState: vtm5e.InitialState,
			action: vtm5e.ActionPoolCheck, arguments: map[string]any{"pool": 4, "hunger": 1, "difficulty": 2}, rolls: [][]int{{10, 10, 2, 6}}, wantOutcome: "vtm5e.pool.messy_critical",
		},
		{
			name: "shadowrun6e", artifact: shadowrun6e.NewArtifact, ruleset: func() rules.Ruleset { return shadowrun6e.New() }, initialState: shadowrun6e.InitialState,
			action: shadowrun6e.ActionPoolCheck, arguments: map[string]any{"pool": 4, "threshold": 2}, rolls: [][]int{{5, 6, 2, 3}}, wantOutcome: "shadowrun6e.pool.success",
		},
		{
			name: "pbta", artifact: pbta.NewArtifact, ruleset: func() rules.Ruleset { return pbta.New() }, initialState: pbta.InitialState,
			action: pbta.ActionMove, arguments: map[string]any{"modifier": 1}, rolls: [][]int{{4, 5}}, wantOutcome: "pbta.move.strong_hit",
		},
		{
			name: "gurps4e", artifact: gurps4e.NewArtifact, ruleset: func() rules.Ruleset { return gurps4e.New() }, initialState: gurps4e.InitialState,
			action: gurps4e.ActionCheck, arguments: map[string]any{"skill": 12}, rolls: [][]int{{3, 4, 4}}, wantOutcome: "gurps4e.skill.success",
		},
		{
			name: "fatecore", artifact: fatecore.NewArtifact, ruleset: func() rules.Ruleset { return fatecore.New() }, initialState: fatecore.InitialState,
			action: fatecore.ActionResolve, arguments: map[string]any{"skill": 2, "opposition": 2, "invokes": []string{"High Ground"}}, rolls: [][]int{{1, 2, 2, 3}}, wantOutcome: "fatecore.action.succeed",
		},
		{
			name: "savageworlds", artifact: savageworlds.NewArtifact, ruleset: func() rules.Ruleset { return savageworlds.New() }, initialState: savageworlds.InitialState,
			action: savageworlds.ActionTraitTest, arguments: map[string]any{"trait_die": 8, "wild_card": false, "target_number": 4}, rolls: [][]int{{5}}, wantOutcome: "savageworlds.trait.success",
		},
	}
}

func newBuiltinGatewaySession(t *testing.T, test builtinGatewayCase) (*domain.Session, *ToolRouter) {
	t.Helper()
	artifact, err := test.artifact()
	if err != nil {
		t.Fatal(err)
	}
	implementation := test.ruleset()
	available := catalog.New()
	if err := available.Register(context.Background(), artifact, implementation, test.initialState()); err != nil {
		t.Fatal(err)
	}
	state := domain.NewSessionState("builtin-"+test.name, nil)
	if _, err := state.BindRules(artifact.Lock(), test.initialState()); err != nil {
		t.Fatal(err)
	}
	session := domain.NewSession(state, &domain.Adventure{System: test.name}, domain.DefaultConfig())
	session.RulesResolver = available
	return session, NewToolRouter(session)
}

func TestStableGameGatewayExecutesEveryBuiltinThroughGenericDice(t *testing.T) {
	for _, test := range builtinGatewayCases() {
		t.Run(test.name, func(t *testing.T) {
			session, router := newBuiltinGatewaySession(t, test)
			if router.rules == nil || router.rulesErr != nil {
				t.Fatalf("gateway=%v error=%v", router.rules, router.rulesErr)
			}
			remaining := make([][]int, len(test.rolls))
			for i := range test.rolls {
				remaining[i] = append([]int(nil), test.rolls[i]...)
			}
			router.rules.resolveDice = func(request dnd5e.DiceRandomRequest) (dnd5e.DiceRandomResponse, error) {
				if len(remaining) == 0 {
					return dnd5e.DiceRandomResponse{}, errors.New("unexpected extra dice request")
				}
				rolls := remaining[0]
				remaining = remaining[1:]
				if len(rolls) != request.Count {
					return dnd5e.DiceRandomResponse{}, errors.New("test response cardinality does not match request")
				}
				return dnd5e.DiceRandomResponse{Rolls: rolls}, nil
			}
			arguments, err := json.Marshal(map[string]any{"action_id": test.action, "arguments": test.arguments})
			if err != nil {
				t.Fatal(err)
			}
			result := router.Execute(types.ToolCall{
				ID: "matrix:" + test.name, Name: "game_submit_intent", Arguments: arguments,
			})
			if result.Error != "" || !strings.Contains(result.Content, `"outcome":"`+test.wantOutcome+`"`) {
				t.Fatalf("result = %+v", result)
			}
			if len(remaining) != 0 {
				t.Fatalf("unused deterministic dice responses: %v", remaining)
			}
			runtime, ok := session.State.RulesRuntimeSnapshot()
			if !ok || len(runtime.RandomDraws) != len(test.rolls) || len(runtime.Receipts) != 1 {
				t.Fatalf("runtime: ok=%v draws=%d receipts=%d", ok, len(runtime.RandomDraws), len(runtime.Receipts))
			}
			if test.name == "dnd5e" {
				if !hasTool(router.GetToolDefinitions(), "roll_dice") {
					t.Fatal("exact dnd5e package omitted compatibility alias")
				}
			} else if hasTool(router.GetToolDefinitions(), "roll_dice") {
				t.Fatal("foreign package advertised a D&D compatibility alias")
			}
		})
	}
}

func TestSavageWorldsCompletesLongestValidDoubleExplosion(t *testing.T) {
	var savage builtinGatewayCase
	for _, test := range builtinGatewayCases() {
		if test.name == "savageworlds" {
			savage = test
			break
		}
	}
	session, router := newBuiltinGatewaySession(t, savage)
	// The package permits 99 aces followed by a terminal face for each of the
	// trait and Wild dice. This is 200 audited random exchanges and 201 Steps.
	remaining := make([]int, 0, 200)
	for range 99 {
		remaining = append(remaining, 4)
	}
	remaining = append(remaining, 3)
	for range 99 {
		remaining = append(remaining, 6)
	}
	remaining = append(remaining, 2)
	router.rules.resolveDice = func(request dnd5e.DiceRandomRequest) (dnd5e.DiceRandomResponse, error) {
		if request.Count != 1 || len(remaining) == 0 {
			return dnd5e.DiceRandomResponse{}, errors.New("unexpected exploding-die request")
		}
		face := remaining[0]
		remaining = remaining[1:]
		if face > request.Sides {
			return dnd5e.DiceRandomResponse{}, errors.New("test face exceeds requested die")
		}
		return dnd5e.DiceRandomResponse{Rolls: []int{face}}, nil
	}
	arguments, err := json.Marshal(map[string]any{
		"action_id": savage.action,
		"arguments": map[string]any{"trait_die": 4, "wild_card": true, "target_number": 4},
	})
	if err != nil {
		t.Fatal(err)
	}
	result := router.Execute(types.ToolCall{ID: "savage:max-explosions", Name: "game_submit_intent", Arguments: arguments})
	if result.Error != "" || !strings.Contains(result.Content, `"outcome":"savageworlds.trait.success_with_raises"`) {
		t.Fatalf("result = %+v", result)
	}
	if len(remaining) != 0 {
		t.Fatalf("unused exploding-die responses: %d", len(remaining))
	}
	runtime, ok := session.State.RulesRuntimeSnapshot()
	if !ok || len(runtime.RandomDraws) != 200 {
		t.Fatalf("runtime: ok=%v draws=%d", ok, len(runtime.RandomDraws))
	}
}

func TestForeignRulesPackageNeitherAdvertisesNorExecutesDNDUtilities(t *testing.T) {
	var pf2eCase builtinGatewayCase
	for _, test := range builtinGatewayCases() {
		if test.name == "pf2e" {
			pf2eCase = test
			break
		}
	}
	session, router := newBuiltinGatewaySession(t, pf2eCase)
	session.State.SetMode(domain.ModeVirtualDM)
	logBefore := session.State.LogLen()
	definitions := router.GetToolDefinitions()
	if !hasTool(definitions, "game_submit_intent") {
		t.Fatal("stable game gateway was not advertised")
	}
	for name := range dndUtilityToolNames {
		if hasTool(definitions, name) {
			t.Errorf("D&D utility %q was advertised", name)
		}
		result := router.Execute(types.ToolCall{ID: "foreign:" + name, Name: name, Arguments: json.RawMessage(`{}`)})
		if !strings.Contains(result.Error, "D&D utility is unavailable") {
			t.Errorf("%s error = %q", name, result.Error)
		}
	}
	for _, command := range []string{"/roll 1d20", "/rest long"} {
		result := NewCommandHandler(session).Execute(ParseCommand(command))
		if result.Success || !strings.Contains(result.Message, "only with the exact built-in D&D 5e") {
			t.Errorf("%s result = %+v", command, result)
		}
	}
	if session.State.LogLen() != logBefore || len(session.State.PartySnapshot()) != 0 {
		t.Fatalf("foreign D&D utilities mutated session: log=%d party=%v", session.State.LogLen(), session.State.PartySnapshot())
	}
}

type contextBlockingCatalogRuleset struct{ rules.Ruleset }

func (r contextBlockingCatalogRuleset) ListActions(ctx context.Context, _ rules.CatalogRequest) ([]rules.ActionDescriptor, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestRulesGatewayAppliesConfiguredTimeout(t *testing.T) {
	test := builtinGatewayCases()[0]
	session, _ := newBuiltinGatewaySession(t, test)
	loaded, err := session.RulesResolver.Lookup(session.State.Rules.Lock)
	if err != nil {
		t.Fatal(err)
	}
	session.Config.RequestTimeoutSeconds = 1
	router := &ToolRouter{
		session: session,
		rules:   newRulesGatewayWithRuleset(session, session.State.Rules.Lock, contextBlockingCatalogRuleset{Ruleset: loaded}),
	}
	started := time.Now()
	result := router.Execute(types.ToolCall{ID: "timeout:list", Name: "game_list_actions", Arguments: json.RawMessage(`{}`)})
	elapsed := time.Since(started)
	if !strings.Contains(result.Error, context.DeadlineExceeded.Error()) {
		t.Fatalf("result = %+v", result)
	}
	if elapsed < 750*time.Millisecond || elapsed > 3*time.Second {
		t.Fatalf("configured one-second timeout took %s", elapsed)
	}
}

type exactTestResolver struct {
	lock           rules.Lock
	implementation rules.Ruleset
}

func (r exactTestResolver) Lookup(lock rules.Lock) (rules.Ruleset, error) {
	if lock != r.lock {
		return nil, rules.ErrRulesetNotFound
	}
	return r.implementation, nil
}

func (exactTestResolver) Resolve(rules.Requirement) (rules.Lock, rules.Ruleset, error) {
	return rules.Lock{}, nil, errors.New("not used")
}

func (exactTestResolver) InitialState(rules.Lock) (rules.Payload, error) {
	return rules.Payload{}, errors.New("not used")
}

func TestRulesGatewayRejectsMaterializedStateThatDoesNotReplay(t *testing.T) {
	implementation := &transactionTestRuleset{authority: "host:oracle"}
	state := domain.NewSessionState("corrupt-replay", nil)
	initial := transactionPayload(t, map[string]any{"counter": 0})
	if _, err := state.BindRules(transactionLock(), initial); err != nil {
		t.Fatal(err)
	}
	call := types.ToolCall{ID: "corrupt-replay-event", Name: "game_submit_intent", Arguments: json.RawMessage(`{}`)}
	handle, _, err := state.BeginRulesRequest(context.Background(), call.ID, call.Name, rulesRequestFingerprint(call))
	if err != nil {
		t.Fatal(err)
	}
	data := transactionPayload(t, map[string]any{"amount": 1})
	if _, err := state.CommitRulesRequest(handle, domain.RulesCommit{
		State: transactionPayload(t, map[string]any{"counter": 1}),
		Principal: rules.Principal{
			ID: "host:test", Kind: "host", Roles: []string{"game-master"},
		},
		ResolutionID: call.ID,
		EventBatches: []domain.RulesEventDraft{{
			ResolutionID: call.ID,
			Events:       []rules.Event{{Type: "counter.incremented", SchemaVersion: 1, Data: data}},
		}},
		Result: &domain.RulesStoredResult{Content: "committed"},
	}); err != nil {
		t.Fatal(err)
	}
	// The audit history legitimately reduces to counter=1. Tampering only with
	// the materialized snapshot remains structurally valid, so the gateway's
	// defensive replay must catch it.
	state.Rules.State = transactionPayload(t, map[string]any{"counter": 2})
	session := domain.NewSession(state, &domain.Adventure{System: "test.transaction"}, domain.DefaultConfig())
	session.RulesResolver = exactTestResolver{lock: transactionLock(), implementation: implementation}
	router := NewToolRouter(session)
	if router.rules != nil || router.rulesErr == nil || !strings.Contains(router.rulesErr.Error(), "materialized state does not match event history") {
		t.Fatalf("gateway=%v error=%v", router.rules, router.rulesErr)
	}
}

func TestAutomaticRecoveryLeavesExternalDecisionUnanswered(t *testing.T) {
	implementation := &transactionTestRuleset{authority: "host:oracle"}
	session, router := transactionTestSession(t, implementation)
	started := router.Execute(types.ToolCall{
		ID: "external-choice", Name: "game_submit_intent",
		Arguments: json.RawMessage(`{"action_id":"test.choose","arguments":{}}`),
	})
	if started.Error != "" || !strings.Contains(started.Content, `"status":"needs_input"`) {
		t.Fatalf("started = %+v", started)
	}
	raw, err := json.Marshal(session.State)
	if err != nil {
		t.Fatal(err)
	}
	var restoredState domain.SessionState
	if err := json.Unmarshal(raw, &restoredState); err != nil {
		t.Fatal(err)
	}
	restored := domain.NewSession(&restoredState, session.Adventure, domain.DefaultConfig())
	restored.RulesResolver = exactTestResolver{lock: transactionLock(), implementation: implementation}
	restarted := NewToolRouter(restored)
	if restarted.rules == nil || restarted.rulesErr != nil {
		t.Fatalf("gateway=%v error=%v", restarted.rules, restarted.rulesErr)
	}
	runtime, ok := restored.State.RulesRuntimeSnapshot()
	if !ok || len(runtime.Pending) != 1 || runtime.Pending[0].Response != nil || implementation.resumes.Load() != 0 {
		t.Fatalf("external decision was auto-answered: ok=%v runtime=%+v resumes=%d", ok, runtime, implementation.resumes.Load())
	}
	observed := restarted.Execute(types.ToolCall{ID: "new-process:pending-observe", Name: "game_observe", Arguments: json.RawMessage(`{}`)})
	if observed.Error != "" || !strings.Contains(observed.Content, `"status":"needs_input"`) && !strings.Contains(observed.Content, `"pending":[{`) {
		t.Fatalf("pending decision was not observable: %+v", observed)
	}
}

func TestPinnedSessionWithoutResolverFailsClosed(t *testing.T) {
	session := createTestSession()
	session.RulesResolver = nil
	router := NewToolRouter(session)
	if router.rules != nil || router.rulesErr == nil || !strings.Contains(router.rulesErr.Error(), "no rules resolver") {
		t.Fatalf("gateway=%v error=%v", router.rules, router.rulesErr)
	}
	for name := range dndUtilityToolNames {
		if hasTool(router.GetToolDefinitions(), name) {
			t.Errorf("advertised %q without a live resolver", name)
		}
	}
}

func TestDNDCompatibilityRequiresExactBuiltinArtifact(t *testing.T) {
	artifact, err := dnd5e.NewArtifact()
	if err != nil {
		t.Fatal(err)
	}
	if !IsBuiltinDND5ELock(artifact.Lock()) {
		t.Fatal("exact built-in lock was not recognized")
	}
	forged := artifact.Lock()
	forged.Digest = "sha256:" + strings.Repeat("0", 64)
	if IsBuiltinDND5ELock(forged) {
		t.Fatal("package ID/version enabled D&D utilities without the exact artifact digest")
	}
}

package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/rules"
	"github.com/theburrowhub/thaimaturgy/internal/rules/dnd5e"
	"github.com/theburrowhub/thaimaturgy/internal/types"
)

type startOverrideRuleset struct {
	rules.Ruleset
	start func(context.Context, rules.StartRequest) (rules.Step, error)
}

func (r startOverrideRuleset) Start(ctx context.Context, request rules.StartRequest) (rules.Step, error) {
	return r.start(ctx, request)
}

func hasTool(definitions []types.Tool, name string) bool {
	for _, definition := range definitions {
		if definition.Name == name {
			return true
		}
	}
	return false
}

func deterministicDice(t *testing.T, expected dnd5e.DiceRandomRequest, rolls ...int) func(dnd5e.DiceRandomRequest) (dnd5e.DiceRandomResponse, error) {
	t.Helper()
	return func(request dnd5e.DiceRandomRequest) (dnd5e.DiceRandomResponse, error) {
		if request != expected {
			t.Fatalf("random request = %+v, want %+v", request, expected)
		}
		return dnd5e.DiceRandomResponse{Rolls: append([]int(nil), rolls...)}, nil
	}
}

func TestToolRouterMigratesLegacyDND5EAndPublishesStableGateway(t *testing.T) {
	session := createTestSession()
	if _, ok := session.State.RulesSnapshot(); ok {
		t.Fatal("legacy fixture unexpectedly starts with a rules lock")
	}
	router := NewToolRouter(session)
	if router.rulesErr != nil || router.rules == nil {
		t.Fatalf("rules gateway = %v, err = %v", router.rules, router.rulesErr)
	}
	snapshot, ok := session.State.RulesSnapshot()
	if !ok || snapshot.Ruleset.ID != dnd5e.PackageID || snapshot.State.String() != `{}` {
		t.Fatalf("migrated snapshot: ok=%v snapshot=%+v state=%s", ok, snapshot, snapshot.State.String())
	}
	if !session.IsModified {
		t.Fatal("automatic legacy binding was not marked for persistence")
	}
	definitions := router.GetToolDefinitions()
	for _, name := range []string{"roll_dice", "ability_check", "game_observe", "game_list_actions", "game_submit_intent", "game_preview"} {
		if !hasTool(definitions, name) {
			t.Errorf("missing tool %q", name)
		}
	}

	seen := make(map[string]bool, len(definitions))
	for _, definition := range definitions {
		if seen[definition.Name] {
			t.Errorf("duplicate tool name %q", definition.Name)
		}
		if !json.Valid(definition.Parameters) {
			t.Errorf("tool %q has invalid parameter schema: %s", definition.Name, definition.Parameters)
		}
		seen[definition.Name] = true
	}
}

func TestToolRouterDoesNotGuessUnknownRuleset(t *testing.T) {
	session := createTestSession()
	session.Adventure.System = "Mystery Engine"
	router := NewToolRouter(session)
	if router.rules != nil || router.rulesErr != nil {
		t.Fatalf("unknown system initialized gateway=%v err=%v", router.rules, router.rulesErr)
	}
	if _, ok := session.State.RulesSnapshot(); ok {
		t.Fatal("unknown system was silently pinned to dnd5e")
	}
	if hasTool(router.GetToolDefinitions(), "game_submit_intent") {
		t.Fatal("game tools were advertised without a loaded ruleset")
	}
	for _, name := range []string{"roll_dice", "ability_check"} {
		if hasTool(router.GetToolDefinitions(), name) {
			t.Fatalf("legacy rules alias %q was advertised without a loaded ruleset", name)
		}
		result := router.Execute(types.ToolCall{ID: "unknown:" + name, Name: name, Arguments: json.RawMessage(`{}`)})
		if !strings.Contains(result.Error, "no rules package") {
			t.Fatalf("%s error = %q", name, result.Error)
		}
	}
	result := router.Execute(types.ToolCall{ID: "unknown:1", Name: "game_observe", Arguments: json.RawMessage(`{}`)})
	if !strings.Contains(result.Error, "no rules package") {
		t.Fatalf("game_observe error = %q", result.Error)
	}
}

func TestPinnedMissingArtifactNeverFallsBackToDND5E(t *testing.T) {
	session := createTestSession()
	foreignLock := rules.Lock{
		ID: "other.rules", Version: "1.0.0",
		Digest: "sha256:" + strings.Repeat("a", 64), ProtocolVersion: rules.ProtocolVersion,
	}
	foreignState, err := rules.NewPayload([]byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.State.BindRules(foreignLock, foreignState); err != nil {
		t.Fatal(err)
	}
	router := NewToolRouter(session)
	if router.rules != nil || router.rulesErr == nil || !strings.Contains(router.rulesErr.Error(), "not found") {
		t.Fatalf("gateway=%v error=%v", router.rules, router.rulesErr)
	}
	snapshot, ok := session.State.RulesSnapshot()
	if !ok || snapshot.Ruleset != foreignLock {
		t.Fatalf("pinned lock changed: ok=%v snapshot=%+v", ok, snapshot)
	}
	if hasTool(router.GetToolDefinitions(), "game_submit_intent") {
		t.Fatal("game tools were advertised for a missing pinned artifact")
	}
	for _, name := range []string{"roll_dice", "ability_check"} {
		if hasTool(router.GetToolDefinitions(), name) {
			t.Fatalf("legacy rules alias %q was advertised for a missing pinned artifact", name)
		}
		result := router.Execute(types.ToolCall{
			ID: "missing:" + name, Name: name,
			Arguments: json.RawMessage(`{"notation":"1d20","modifier":0,"dc":10}`),
		})
		if !strings.Contains(result.Error, "rules gateway unavailable") {
			t.Fatalf("%s error = %q", name, result.Error)
		}
	}
	if session.State.LogLen() != 0 {
		t.Fatalf("missing artifact aliases mutated the log: %d", session.State.LogLen())
	}
}

func TestPinnedDND5EStateIsValidatedBeforeToolsAreAdvertised(t *testing.T) {
	session := createTestSession()
	artifact, err := dnd5e.NewArtifact()
	if err != nil {
		t.Fatal(err)
	}
	corruptState, err := rules.NewPayload([]byte(`{"unexpected":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.State.BindRules(artifact.Lock(), corruptState); err != nil {
		t.Fatal(err)
	}
	router := NewToolRouter(session)
	if router.rules != nil || router.rulesErr == nil || !strings.Contains(router.rulesErr.Error(), "validate pinned rules state") {
		t.Fatalf("gateway=%v error=%v", router.rules, router.rulesErr)
	}
	for _, name := range []string{"game_submit_intent", "roll_dice", "ability_check"} {
		if hasTool(router.GetToolDefinitions(), name) {
			t.Fatalf("tool %q was advertised for invalid rules state", name)
		}
	}
}

func TestToolDefinitionsAreDefensiveCopies(t *testing.T) {
	router := NewToolRouter(createTestSession())
	first := router.GetToolDefinitions()
	first[0].Name = "tampered"
	first[0].Parameters[0] = 'x'
	second := router.GetToolDefinitions()
	if second[0].Name == "tampered" || len(second[0].Parameters) == 0 || second[0].Parameters[0] == 'x' {
		t.Fatal("caller mutation changed the router's tool definitions")
	}
}

func TestGameCatalogAndPreviewDoNotDrawOrLog(t *testing.T) {
	session := createTestSession()
	router := NewToolRouter(session)
	session.IsModified = false // Ignore the one-time legacy lock migration.
	draws := 0
	router.rules.resolveDice = func(request dnd5e.DiceRandomRequest) (dnd5e.DiceRandomResponse, error) {
		draws++
		return dnd5e.DiceRandomResponse{Rolls: []int{1}}, nil
	}

	catalog := router.Execute(types.ToolCall{ID: "catalog:1", Name: "game_list_actions", Arguments: json.RawMessage(`{}`)})
	if catalog.Error != "" {
		t.Fatalf("game_list_actions: %s", catalog.Error)
	}
	var listed struct {
		Status string `json:"status"`
		Data   struct {
			Actions []struct {
				ID string `json:"id"`
			} `json:"actions"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(catalog.Content), &listed); err != nil {
		t.Fatal(err)
	}
	if listed.Status != "resolved" || len(listed.Data.Actions) != 2 || listed.Data.Actions[0].ID != dnd5e.ActionDiceRoll || listed.Data.Actions[1].ID != dnd5e.ActionAbilityCheck {
		t.Fatalf("catalog = %+v", listed)
	}

	previewArgs := json.RawMessage(`{"action_id":"dice.roll","arguments":{"notation":"1d20"}}`)
	preview := router.Execute(types.ToolCall{ID: "preview:1", Name: "game_preview", Arguments: previewArgs})
	if preview.Error != "" {
		t.Fatalf("game_preview: %s", preview.Error)
	}
	var projected struct {
		Status string `json:"status"`
		Data   struct {
			NextStep string `json:"next_step"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(preview.Content), &projected); err != nil {
		t.Fatal(err)
	}
	if projected.Status != "resolved" || projected.Data.NextStep != "need_random" {
		t.Fatalf("preview = %s", preview.Content)
	}
	if draws != 0 || session.State.LogLen() != 0 || session.IsModified {
		t.Fatalf("preview had effects: draws=%d log=%d modified=%v", draws, session.State.LogLen(), session.IsModified)
	}
}

func TestGameObserveSchemaAndExplainUseThePinnedRuleset(t *testing.T) {
	router := NewToolRouter(createTestSession())
	tests := []struct {
		name, arguments, contains string
	}{
		{"game_observe", `{}`, `"data":{}`},
		{"game_get_action_schema", `{"action_id":"ability.check"}`, `"id":"ability.check"`},
		{"game_explain", `{"reference":"ability.check","locale":"en"}`, `"text":"Roll one d20`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := router.Execute(types.ToolCall{
				ID: "query:" + test.name, Name: test.name, Arguments: json.RawMessage(test.arguments),
			})
			if result.Error != "" || !strings.Contains(result.Content, test.contains) {
				t.Fatalf("result=%+v, want content containing %q", result, test.contains)
			}
		})
	}
}

func TestGameSubmitIntentReturnsStructuredResultAndCommitsLegacyLog(t *testing.T) {
	session := createTestSession()
	router := NewToolRouter(session)
	session.IsModified = false
	router.rules.resolveDice = deterministicDice(t, dnd5e.DiceRandomRequest{Count: 2, Sides: 6}, 4, 5)

	arguments := json.RawMessage(`{
		"action_id":"dice.roll",
		"arguments":{"notation":"2d6+3","reason":"Damage"}
	}`)
	result := router.Execute(types.ToolCall{ID: "intent:roll:1", Name: "game_submit_intent", Arguments: arguments})
	if result.Error != "" {
		t.Fatalf("game_submit_intent: %s", result.Error)
	}
	var envelope struct {
		Status  string `json:"status"`
		Outcome string `json:"outcome"`
		Data    struct {
			Notation string `json:"notation"`
			Rolls    []int  `json:"rolls"`
			Total    int    `json:"total"`
			Legacy   struct {
				Content    string `json:"content"`
				LogMessage string `json:"log_message"`
			} `json:"legacy"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(result.Content), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Status != "resolved" || envelope.Outcome != "dnd5e.dice.rolled" || envelope.Data.Notation != "2d6+3" || envelope.Data.Total != 12 {
		t.Fatalf("result = %s", result.Content)
	}
	if envelope.Data.Legacy.Content != "Rolled 2d6+3: [4+5]+3 = 12" || envelope.Data.Legacy.LogMessage != "Damage — Rolled 2d6+3: [4+5]+3 = 12" {
		t.Fatalf("legacy projection = %+v", envelope.Data.Legacy)
	}
	log := session.State.RecentLog(1)
	if len(log) != 1 || log[0].Type != domain.LogRoll || log[0].Message != envelope.Data.Legacy.LogMessage || !session.IsModified {
		t.Fatalf("committed log=%+v modified=%v", log, session.IsModified)
	}
}

func TestLegacyDiceAliasesUseRulesetAndPreserveText(t *testing.T) {
	session := createTestSession()
	router := NewToolRouter(session)
	session.IsModified = false
	router.rules.resolveDice = deterministicDice(t, dnd5e.DiceRandomRequest{Count: 1, Sides: 20}, 20)

	roll := router.Execute(types.ToolCall{
		ID: "legacy:roll:1", Name: "roll_dice",
		Arguments: json.RawMessage(`{"notation":"d20","reason":"Initiative"}`),
	})
	if roll.Error != "" || roll.Content != "Rolled 1d20: [20] = 20 [CRIT!]" || roll.ToolCallID != "legacy:roll:1" {
		t.Fatalf("roll_dice = %+v", roll)
	}
	log := session.State.RecentLog(1)
	if len(log) != 1 || log[0].Message != "Initiative — "+roll.Content {
		t.Fatalf("roll log = %+v", log)
	}

	router.rules.resolveDice = deterministicDice(t, dnd5e.DiceRandomRequest{Count: 1, Sides: 20}, 12)
	check := router.Execute(types.ToolCall{
		ID: "legacy:check:1", Name: "ability_check",
		Arguments: json.RawMessage(`{"modifier":3,"dc":15,"label":"Athletics"}`),
	})
	if check.Error != "" || check.Content != "Athletics — Check (DC 15): d20(12)+3 = 15 [SUCCESS]" {
		t.Fatalf("ability_check = %+v", check)
	}
	if session.State.LogLen() != 2 || !session.IsModified {
		t.Fatalf("legacy aliases log=%d modified=%v", session.State.LogLen(), session.IsModified)
	}
}

func TestLegacyAbilityAliasPreservesDefaultAndFractionalCoercion(t *testing.T) {
	session := createTestSession()
	router := NewToolRouter(session)
	router.rules.resolveDice = deterministicDice(t, dnd5e.DiceRandomRequest{Count: 1, Sides: 20}, 1)
	missingModifier := router.Execute(types.ToolCall{
		ID: "legacy:default", Name: "ability_check", Arguments: json.RawMessage(`{"dc":1,"ignored":true}`),
	})
	if missingModifier.Error != "" || missingModifier.Content != "Check (DC 1): d20(1)+0 = 1 [SUCCESS] [NAT 1]" {
		t.Fatalf("default modifier = %+v", missingModifier)
	}

	router.rules.resolveDice = deterministicDice(t, dnd5e.DiceRandomRequest{Count: 1, Sides: 20}, 1)
	fractional := router.Execute(types.ToolCall{
		ID: "legacy:fraction", Name: "ability_check", Arguments: json.RawMessage(`{"modifier":2.9,"dc":3.9}`),
	})
	if fractional.Error != "" || fractional.Content != "Check (DC 3): d20(1)+2 = 3 [SUCCESS] [NAT 1]" {
		t.Fatalf("fractional coercion = %+v", fractional)
	}
}

func TestRulesRequestIDIsIdempotentAndBoundToArguments(t *testing.T) {
	session := createTestSession()
	router := NewToolRouter(session)
	draws := 0
	router.rules.resolveDice = func(request dnd5e.DiceRandomRequest) (dnd5e.DiceRandomResponse, error) {
		draws++
		return dnd5e.DiceRandomResponse{Rolls: []int{7}}, nil
	}
	call := types.ToolCall{
		ID: "retry:1", Name: "game_submit_intent",
		Arguments: json.RawMessage(`{"action_id":"dice.roll","arguments":{"notation":"1d20"}}`),
	}
	first := router.Execute(call)
	second := router.Execute(call)
	if first != second || first.Error != "" {
		t.Fatalf("retry mismatch: first=%+v second=%+v", first, second)
	}
	if draws != 1 || session.State.LogLen() != 1 {
		t.Fatalf("retry repeated effects: draws=%d log=%d", draws, session.State.LogLen())
	}

	call.Arguments = json.RawMessage(`{"action_id":"dice.roll","arguments":{"notation":"1d6"}}`)
	conflict := router.Execute(call)
	if !strings.Contains(conflict.Error, "already used with different") || draws != 1 || session.State.LogLen() != 1 {
		t.Fatalf("request-ID conflict=%+v draws=%d log=%d", conflict, draws, session.State.LogLen())
	}
}

func TestRandomFailuresNeverCommitAndRetriesReuseTheErrorReceipt(t *testing.T) {
	tests := []struct {
		name, contains string
		response       dnd5e.DiceRandomResponse
		err            error
	}{
		{name: "host failure", contains: "entropy unavailable", err: errors.New("entropy unavailable")},
		{name: "invalid host response", contains: "invalid random response", response: dnd5e.DiceRandomResponse{Rolls: []int{21}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			session := createTestSession()
			router := NewToolRouter(session)
			session.IsModified = false
			draws := 0
			router.rules.resolveDice = func(dnd5e.DiceRandomRequest) (dnd5e.DiceRandomResponse, error) {
				draws++
				return test.response, test.err
			}
			call := types.ToolCall{
				ID: "random-error:" + strings.ReplaceAll(test.name, " ", "-"), Name: "game_submit_intent",
				Arguments: json.RawMessage(`{"action_id":"dice.roll","arguments":{"notation":"1d20"}}`),
			}
			first := router.Execute(call)
			second := router.Execute(call)
			if first != second || !strings.Contains(first.Error, test.contains) {
				t.Fatalf("first=%+v second=%+v, want %q", first, second, test.contains)
			}
			if draws != 1 || session.State.LogLen() != 0 || session.IsModified {
				t.Fatalf("failed draw had effects: draws=%d log=%d modified=%v", draws, session.State.LogLen(), session.IsModified)
			}
		})
	}
}

func TestGatewayRejectsNonCanonicalRandomSpecificationBeforeDrawing(t *testing.T) {
	session := createTestSession()
	router := NewToolRouter(session)
	session.IsModified = false
	continuation, err := rules.PayloadFrom(map[string]any{
		"schema_version": 1, "action": dnd5e.ActionDiceRoll, "notation": "1d20",
	})
	if err != nil {
		t.Fatal(err)
	}
	specification, err := rules.NewPayload([]byte(`{"count":1,"COUNT":2,"sides":20}`))
	if err != nil {
		t.Fatal(err)
	}
	original := router.rules.ruleset
	router.rules.ruleset = startOverrideRuleset{
		Ruleset: original,
		start: func(_ context.Context, request rules.StartRequest) (rules.Step, error) {
			return rules.Step{
				ID: request.Intent.ID, Kind: rules.StepKindNeedRandom, Continuation: continuation,
				NeedRandom: &rules.RandomRequest{Method: dnd5e.RandomMethodDiceRoll, Specification: specification},
			}, nil
		},
	}
	draws := 0
	router.rules.resolveDice = func(dnd5e.DiceRandomRequest) (dnd5e.DiceRandomResponse, error) {
		draws++
		return dnd5e.DiceRandomResponse{Rolls: []int{10}}, nil
	}
	result := router.Execute(types.ToolCall{
		ID: "rng-spec:1", Name: "game_submit_intent",
		Arguments: json.RawMessage(`{"action_id":"dice.roll","arguments":{"notation":"1d20"}}`),
	})
	if !strings.Contains(result.Error, `unknown field "COUNT"`) {
		t.Fatalf("result = %+v", result)
	}
	if draws != 0 || session.State.LogLen() != 0 || session.IsModified {
		t.Fatalf("invalid specification had effects: draws=%d log=%d modified=%v", draws, session.State.LogLen(), session.IsModified)
	}
}

func TestRulesRequestReceiptsStayBounded(t *testing.T) {
	router := NewToolRouter(createTestSession())
	for i := 0; i <= maxInMemoryRuleReceipts; i++ {
		call := types.ToolCall{
			ID: fmt.Sprintf("observe:%d", i), Name: "game_observe", Arguments: json.RawMessage(`{}`),
		}
		if result := router.Execute(call); result.Error != "" {
			t.Fatalf("game_observe %d: %s", i, result.Error)
		}
	}
	if got := len(router.rules.receipts); got != maxInMemoryRuleReceipts {
		t.Fatalf("receipt count = %d, want %d", got, maxInMemoryRuleReceipts)
	}
	if _, retained := router.rules.receipts["observe:0"]; retained {
		t.Fatal("oldest receipt was not evicted")
	}
}

func TestGameRespondValidatesItsStableSurface(t *testing.T) {
	router := NewToolRouter(createTestSession())
	tests := []struct {
		name, arguments, contains string
	}{
		{"missing resolution", `{"response":{}}`, "missing 'resolution_id'"},
		{"missing response", `{"resolution_id":"resolution-1"}`, "missing 'response'"},
		{"unknown field", `{"resolution_id":"resolution-1","response":{},"extra":true}`, "unknown field"},
		{"duplicate field", `{"resolution_id":"resolution-1","resolution_id":"resolution-2","response":{}}`, "duplicate JSON field"},
		{"no pending step", `{"resolution_id":"resolution-1","response":{}}`, "no externally pending resolution"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := router.Execute(types.ToolCall{
				ID: "respond:" + test.name, Name: "game_respond", Arguments: json.RawMessage(test.arguments),
			})
			if !strings.Contains(result.Error, test.contains) {
				t.Fatalf("error = %q, want substring %q", result.Error, test.contains)
			}
		})
	}
}

func TestMutatingRulesCallsRequireHostRequestID(t *testing.T) {
	session := createTestSession()
	router := NewToolRouter(session)
	draws := 0
	router.rules.resolveDice = func(dnd5e.DiceRandomRequest) (dnd5e.DiceRandomResponse, error) {
		draws++
		return dnd5e.DiceRandomResponse{Rolls: []int{10}}, nil
	}
	tests := []types.ToolCall{
		{Name: "game_submit_intent", Arguments: json.RawMessage(`{"action_id":"dice.roll","arguments":{"notation":"1d20"}}`)},
		{Name: "game_respond", Arguments: json.RawMessage(`{"resolution_id":"resolution-1","response":{}}`)},
		{Name: "roll_dice", Arguments: json.RawMessage(`{"notation":"1d20"}`)},
		{Name: "ability_check", Arguments: json.RawMessage(`{"modifier":0,"dc":10}`)},
	}
	for _, call := range tests {
		result := router.Execute(call)
		if !strings.Contains(result.Error, "requires a host request ID") {
			t.Fatalf("%s error = %q", call.Name, result.Error)
		}
	}
	if draws != 0 || session.State.LogLen() != 0 {
		t.Fatalf("ID-less calls had effects: draws=%d log=%d", draws, session.State.LogLen())
	}
}

func TestGameExplainDefaultsLocaleWithoutConfig(t *testing.T) {
	session := createTestSession()
	session.Config = nil
	router := NewToolRouter(session)
	result := router.Execute(types.ToolCall{
		ID: "explain:no-config", Name: "game_explain",
		Arguments: json.RawMessage(`{"reference":"ability.check"}`),
	})
	if result.Error != "" || !strings.Contains(result.Content, `"text":"Roll one d20`) {
		t.Fatalf("game_explain = %+v", result)
	}
}

func TestGameIntentRejectionIsStructuredAndHasNoEffects(t *testing.T) {
	session := createTestSession()
	router := NewToolRouter(session)
	session.IsModified = false
	result := router.Execute(types.ToolCall{
		ID: "reject:1", Name: "game_submit_intent",
		Arguments: json.RawMessage(`{"action_id":"dice.roll","arguments":{"notation":"0d6"}}`),
	})
	if result.Error != "" {
		t.Fatalf("mechanical rejection returned transport error: %s", result.Error)
	}
	var envelope struct {
		Status string `json:"status"`
		Data   struct {
			Code string `json:"code"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(result.Content), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Status != "rejected" || envelope.Data.Code != "invalid.notation" || session.State.LogLen() != 0 || session.IsModified {
		t.Fatalf("rejection=%s log=%d modified=%v", result.Content, session.State.LogLen(), session.IsModified)
	}
}

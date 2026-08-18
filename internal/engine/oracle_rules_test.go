package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
	"github.com/theburrowhub/thaimaturgy/internal/rules"
	"github.com/theburrowhub/thaimaturgy/internal/rules/dnd5e"
)

// repeatingToolIDProvider models Gemini's synthetic name/index IDs: every new
// turn reports the same provider-side ID even though it is a distinct call.
type repeatingToolIDProvider struct {
	calls int
}

func TestOracleFailsClosedBeforeProviderWhenPinnedGatewayCannotLoad(t *testing.T) {
	session := createUnboundTestSession()
	session.RulesResolver = createTestSession().RulesResolver
	missing := rules.Lock{
		ID: "missing.rules", Version: "1.0.0", ProtocolVersion: rules.ProtocolVersion,
		Digest: "sha256:" + strings.Repeat("a", 64),
	}
	state, err := rules.PayloadFrom(map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if created, err := session.State.BindRules(missing, state); err != nil || !created {
		t.Fatalf("bind created=%v err=%v", created, err)
	}
	provider := &repeatingToolIDProvider{}
	response := NewOracle(session, provider).Ask(context.Background(), "continue")
	if response.Error == nil || !strings.Contains(response.Error.Error(), "rules gateway unavailable") {
		t.Fatalf("response error = %v", response.Error)
	}
	if provider.calls != 0 {
		t.Fatalf("provider calls after failed rules restore = %d", provider.calls)
	}
}

func TestForeignRulesPromptDoesNotTreatLegacyDNDStateAsAuthoritative(t *testing.T) {
	var pf2eCase builtinGatewayCase
	for _, test := range builtinGatewayCases() {
		if test.name == "pf2e" {
			pf2eCase = test
			break
		}
	}
	session, _ := newBuiltinGatewaySession(t, pf2eCase)
	session.State.Characters = []*domain.Character{{
		Name: "Legacy Borin", Race: "Dwarf", Class: "Fighter", Level: 3,
		MaxHP: 30, CurrentHP: 2, AC: 16,
	}}

	prompt := NewOracle(session, nil).buildSystemPrompt()
	for _, forbidden := range []string{"Legacy Borin", "CURRENT SHEETS", "HP: 2/30"} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("foreign rules prompt exposed legacy D&D state %q", forbidden)
		}
	}
	if !strings.Contains(prompt, "game_observe") {
		t.Fatal("foreign rules prompt omitted the neutral mechanical projection tool")
	}
}

func TestOraclePromptIdentifiesExactMechanicalAuthority(t *testing.T) {
	oracle := NewOracle(createTestSession(), nil)
	prompt := oracle.buildSystemPrompt()
	snapshot, ok := oracle.session.State.RulesSnapshot()
	if !ok {
		t.Fatal("test session was not pinned to its rules artifact")
	}
	for _, required := range []string{
		"=== LOADED RULES PACKAGE ===",
		snapshot.Ruleset.ID + "@" + snapshot.Ruleset.Version,
		snapshot.Ruleset.Digest,
		"game_list_actions",
		"Do not calculate or invent a result outside that interface.",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("rules authority prompt omitted %q", required)
		}
	}
}

func TestMCPToolSubcommandArgsInheritEffectiveRulesContext(t *testing.T) {
	session := createTestSession()
	session.Config.Language = domain.LangSpanish
	session.Config.RequestTimeoutSeconds = 37
	session.DataDirectory = "/tmp/thaim-test-data"

	args, err := mcpToolSubcommandArgs(session, "/tmp/session.json", "oracle-test-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(args) == 0 || args[0] != "__mcp-tools" {
		t.Fatalf("subcommand args = %v", args)
	}
	wantFlags := map[string]string{
		"--adventure-id":          session.State.AdventureID,
		"--session":               "/tmp/session.json",
		"--request-namespace":     "oracle-test-1",
		"--language":              "es",
		"--rules-timeout-seconds": "37",
		"--data-dir":              session.DataDirectory,
	}
	for flag, want := range wantFlags {
		if got, ok := stringFlagValue(args, flag); !ok || got != want {
			t.Errorf("%s = %q, present=%v, want %q; args=%v", flag, got, ok, want, args)
		}
	}
}

func TestEffectiveRulesRequestTimeoutIsBoundedAndSharedWithMCP(t *testing.T) {
	tests := []struct {
		configured int
		want       int
	}{
		{configured: 0, want: DefaultRulesRequestTimeoutSeconds},
		{configured: -1, want: DefaultRulesRequestTimeoutSeconds},
		{configured: 17, want: 17},
		{configured: MaxRulesRequestTimeoutSeconds + 1, want: MaxRulesRequestTimeoutSeconds},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("configured_%d", test.configured), func(t *testing.T) {
			session := createTestSession()
			session.Config.RequestTimeoutSeconds = test.configured
			if got := EffectiveRulesRequestTimeoutSeconds(session); got != test.want {
				t.Fatalf("effective timeout = %d, want %d", got, test.want)
			}
			args, err := mcpToolSubcommandArgs(session, "/tmp/session.json", "oracle-timeout-test")
			if err != nil {
				t.Fatal(err)
			}
			if got, ok := stringFlagValue(args, "--rules-timeout-seconds"); !ok || got != fmt.Sprint(test.want) {
				t.Fatalf("MCP timeout = %q, present=%v, want %d", got, ok, test.want)
			}
		})
	}
}

func TestMCPToolSubcommandArgsRejectUnsupportedLanguage(t *testing.T) {
	session := createTestSession()
	session.Config.Language = domain.Language("fr")
	if _, err := mcpToolSubcommandArgs(session, "/tmp/session.json", "oracle-test"); err == nil || !strings.Contains(err.Error(), "unsupported language") {
		t.Fatalf("error = %v", err)
	}
}

func stringFlagValue(args []string, name string) (string, bool) {
	for index := 0; index+1 < len(args); index++ {
		if args[index] == name {
			return args[index+1], true
		}
	}
	return "", false
}

func (p *repeatingToolIDProvider) Name() string         { return "repeating-tool-id" }
func (p *repeatingToolIDProvider) SupportsTools() bool  { return true }
func (p *repeatingToolIDProvider) SupportsVision() bool { return false }

func (p *repeatingToolIDProvider) Chat(context.Context, providers.ChatRequest) (*providers.ChatResponse, error) {
	p.calls++
	if p.calls%2 == 1 {
		return &providers.ChatResponse{
			FinishReason: "tool_calls",
			ToolCalls: []providers.ToolCallInfo{{
				ID: "roll_dice-0", Type: "function",
				Function: providers.FunctionCall{Name: "roll_dice", Arguments: `{"notation":"1d20"}`},
			}},
		}, nil
	}
	return &providers.ChatResponse{Content: "done", FinishReason: "stop"}, nil
}

func TestOracleNamespacesProviderToolIDsAcrossTurns(t *testing.T) {
	session := createTestSession()
	oracle := NewOracle(session, &repeatingToolIDProvider{})
	draws := 0
	oracle.toolRouter.rules.resolveDice = func(dnd5e.DiceRandomRequest) (dnd5e.DiceRandomResponse, error) {
		draws++
		return dnd5e.DiceRandomResponse{Rolls: []int{draws}}, nil
	}

	for _, input := range []string{"first roll", "second roll"} {
		if response := oracle.Ask(context.Background(), input); response.Error != nil {
			t.Fatalf("Ask(%q): %v", input, response.Error)
		}
	}
	if draws != 2 || session.State.LogLen() != 2 {
		t.Fatalf("distinct turns reused a receipt: draws=%d log=%d", draws, session.State.LogLen())
	}
}

func TestOracleNamespacesPersistedReceiptsAcrossInstances(t *testing.T) {
	session := createTestSession()
	first := NewOracle(session, nil)
	second := NewOracle(session, nil)
	if first.executionNamespace == "" || second.executionNamespace == "" ||
		first.executionNamespace == second.executionNamespace {
		t.Fatalf("oracle namespaces = %q, %q", first.executionNamespace, second.executionNamespace)
	}
	firstID := "oracle:" + first.executionNamespace + ":1:0:0"
	secondID := "oracle:" + second.executionNamespace + ":1:0:0"
	if firstID == secondID {
		t.Fatalf("fresh Oracle instances generated colliding receipt IDs: %q", firstID)
	}
}

func TestMergeSessionStateCannotRollBackNewerParentRules(t *testing.T) {
	session := createTestSession()
	encoded, err := json.Marshal(session.State)
	if err != nil {
		t.Fatal(err)
	}
	var staleChild domain.SessionState
	if err := json.Unmarshal(encoded, &staleChild); err != nil {
		t.Fatal(err)
	}

	handle, receipt, err := session.State.BeginRulesRequest(
		context.Background(), "parent-ahead", "game_submit_intent", "sha256:"+strings.Repeat("d", 64),
	)
	if err != nil || receipt != nil {
		t.Fatalf("begin receipt=%v err=%v", receipt, err)
	}
	if _, err := session.State.CommitRulesRequest(handle, domain.RulesCommit{
		State: handle.Snapshot.State, ResolutionID: "parent-ahead",
		Result: &domain.RulesStoredResult{Content: `{"status":"resolved"}`},
	}); err != nil {
		t.Fatal(err)
	}

	if err := mergeSessionState(session.State, &staleChild, staleChild.LogLen()); err != nil {
		t.Fatal(err)
	}
	runtime, ok := session.State.RulesRuntimeSnapshot()
	if !ok || runtime.Generation != 1 || len(runtime.Receipts) != 1 || runtime.Receipts[0].RequestID != "parent-ahead" {
		t.Fatalf("stale child rolled parent back: ok=%v runtime=%+v", ok, runtime)
	}
}

package engine

import (
	"context"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/providers"
	"github.com/theburrowhub/thaimaturgy/internal/rules/dnd5e"
)

// repeatingToolIDProvider models Gemini's synthetic name/index IDs: every new
// turn reports the same provider-side ID even though it is a distinct call.
type repeatingToolIDProvider struct {
	calls int
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

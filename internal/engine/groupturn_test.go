package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
)

// fakeProvider returns a canned reply and records the last user prompt.
type fakeProvider struct{ lastUser string }

func (f *fakeProvider) Name() string         { return "fake" }
func (f *fakeProvider) SupportsTools() bool  { return true }
func (f *fakeProvider) SupportsVision() bool { return false }
func (f *fakeProvider) Chat(_ context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	for _, m := range req.Messages {
		if m.Role == providers.RoleUser {
			f.lastUser = m.Content
		}
	}
	return &providers.ChatResponse{Content: "The door bursts open.", FinishReason: "stop"}, nil
}

func TestRunGroupTurn(t *testing.T) {
	session := createTestSession()
	session.State.SetMode(domain.ModeVirtualDM)
	session.State.Characters = []*domain.Character{
		domain.NewCharacter("Alden", "Human", "Fighter"),
		domain.NewCharacter("Naivara", "Elf", "Wizard"),
	}
	_, _ = session.State.ClaimCharacter("p1", "Ana", "Alden")
	_, _ = session.State.ClaimCharacter("p2", "Luis", "Naivara")

	fp := &fakeProvider{}
	o := NewOracle(session, fp)

	// No actions declared yet → error, nothing sent.
	if resp := o.RunGroupTurn(context.Background()); resp.Error == nil {
		t.Error("expected error when no actions are declared")
	}

	_, _ = session.State.SubmitAction("p1", "I kick the door")
	_, _ = session.State.SubmitAction("p2", "I ready a shield spell")

	resp := o.RunGroupTurn(context.Background())
	if resp.Error != nil {
		t.Fatalf("group turn failed: %v", resp.Error)
	}
	if resp.Answer != "The door bursts open." {
		t.Errorf("answer = %q", resp.Answer)
	}
	// Both declared actions were aggregated into the DM prompt.
	if !strings.Contains(fp.lastUser, "Alden") || !strings.Contains(fp.lastUser, "kick the door") ||
		!strings.Contains(fp.lastUser, "Naivara") || !strings.Contains(fp.lastUser, "shield spell") {
		t.Errorf("aggregated prompt missing actions:\n%s", fp.lastUser)
	}
	// The round buffer is cleared after a successful turn.
	if len(session.State.RoundActions()) != 0 {
		t.Error("round should be reset after a successful group turn")
	}
}

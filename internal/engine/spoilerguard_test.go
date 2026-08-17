package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
)

// auditFake returns one narration for the generation call and another for the
// anti-spoiler review call (recognized by the auditor system prompt), and counts
// how many times the review ran.
type auditFake struct {
	gen, review string
	reviewCalls int
}

func (f *auditFake) Name() string         { return "audit-fake" }
func (f *auditFake) SupportsTools() bool  { return true }
func (f *auditFake) SupportsVision() bool { return false }
func (f *auditFake) Chat(_ context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	for _, m := range req.Messages {
		if m.Role == providers.RoleSystem && strings.Contains(m.Content, "spoiler auditor") {
			f.reviewCalls++
			return &providers.ChatResponse{Content: f.review, FinishReason: "stop"}, nil
		}
	}
	return &providers.ChatResponse{Content: f.gen, FinishReason: "stop"}, nil
}

func spoilerTestSession() *domain.Session {
	adv := &domain.Adventure{
		SchemaVersion: domain.SchemaVersion, ID: "a", Title: "A", StartRoom: "r1",
		Background: "The mayor secretly serves the cult beneath the town.",
		Zones: []domain.Zone{{ID: "z1", Name: "Town", Rooms: []domain.Room{
			{ID: "r1", Name: "Square", DMNotes: "A trapdoor hides under the well."},
		}}},
		NPCs: []domain.NPC{{ID: "zoara", Name: "Zoara", Role: "cult leader", Secrets: "She is the true villain."}},
	}
	st := domain.NewSessionState("s", adv)
	st.SetMode(domain.ModeVirtualDM)
	return domain.NewSession(st, adv, domain.DefaultConfig())
}

// When enabled in virtual-DM mode, a leaking narration is replaced by the
// reviewer's rewrite — and the persisted history holds the scrubbed text. (#89)
func TestSpoilerGuardRewrites(t *testing.T) {
	s := spoilerTestSession()
	s.Config.SpoilerGuard.Enabled = true
	fp := &auditFake{
		gen:    "A woman named Zoara watches from the shadows, and the mayor serves a cult.",
		review: "A cloaked figure watches from the shadows.",
	}
	o := NewOracle(s, fp)

	resp := o.Ask(context.Background(), "we look around the square")
	if resp.Error != nil {
		t.Fatalf("Ask: %v", resp.Error)
	}
	if resp.Answer != "A cloaked figure watches from the shadows." {
		t.Errorf("answer should be the scrubbed narration, got %q", resp.Answer)
	}
	if fp.reviewCalls != 1 {
		t.Errorf("review should run exactly once, ran %d", fp.reviewCalls)
	}
	msgs := s.State.Conversation.Messages
	last := msgs[len(msgs)-1]
	if last.Role != domain.RoleAssistant || strings.Contains(last.Content, "Zoara") {
		t.Errorf("persisted history should hold the scrubbed text, got %q", last.Content)
	}
}

// Disabled (the default): no review call, the narration passes through verbatim.
func TestSpoilerGuardDisabled(t *testing.T) {
	s := spoilerTestSession() // SpoilerGuard disabled by default
	fp := &auditFake{gen: "A woman named Zoara watches.", review: "scrubbed"}
	o := NewOracle(s, fp)
	resp := o.Ask(context.Background(), "look")
	if resp.Answer != "A woman named Zoara watches." {
		t.Errorf("disabled guard should pass narration through, got %q", resp.Answer)
	}
	if fp.reviewCalls != 0 {
		t.Errorf("disabled guard should not call the reviewer, called %d", fp.reviewCalls)
	}
}

// Enabled but in assistant mode: the human DM is the audience, so nothing is
// reviewed or filtered.
func TestSpoilerGuardOnlyVirtualDM(t *testing.T) {
	s := spoilerTestSession()
	s.State.SetMode(domain.ModeAssistant)
	s.Config.SpoilerGuard.Enabled = true
	fp := &auditFake{gen: "A woman named Zoara watches.", review: "scrubbed"}
	o := NewOracle(s, fp)
	resp := o.Ask(context.Background(), "look")
	if resp.Answer != "A woman named Zoara watches." {
		t.Errorf("assistant mode should not filter, got %q", resp.Answer)
	}
	if fp.reviewCalls != 0 {
		t.Errorf("assistant mode should not call the reviewer, called %d", fp.reviewCalls)
	}
}

// The hidden context the reviewer sees includes the truth, present secrets, and
// the names of not-yet-encountered characters (classic leaks).
func TestSpoilerGuardHiddenContext(t *testing.T) {
	s := spoilerTestSession()
	o := NewOracle(s, &auditFake{})
	hidden := o.hiddenContext()
	// Background (the truth), the current room's hidden notes, and the name/role of
	// a not-yet-met character (its name is itself the classic spoiler).
	for _, want := range []string{"serves the cult", "trapdoor", "Zoara"} {
		if !strings.Contains(hidden, want) {
			t.Errorf("hidden context missing %q:\n%s", want, hidden)
		}
	}
}

// A present NPC's secret IS surfaced (the party is interacting with them).
func TestSpoilerGuardSurfacesPresentSecret(t *testing.T) {
	s := spoilerTestSession()
	// Put Zoara in the current room so she counts as present.
	s.Adventure.Zones[0].Rooms[0].NPCIDs = []string{"zoara"}
	o := NewOracle(s, &auditFake{})
	if hidden := o.hiddenContext(); !strings.Contains(hidden, "true villain") {
		t.Errorf("a present NPC's secret should be in the hidden context:\n%s", hidden)
	}
}

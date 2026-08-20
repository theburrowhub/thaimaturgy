package appservice

import (
	"context"
	"errors"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
)

// planProvider is a stub AI provider: it runs onChat (used to mutate the session
// mid-plan) and returns a fixed party-plan JSON.
type planProvider struct {
	onChat func()
	resp   string
}

func (p *planProvider) Name() string         { return "stub" }
func (p *planProvider) SupportsTools() bool  { return false }
func (p *planProvider) SupportsVision() bool { return false }
func (p *planProvider) Chat(_ context.Context, _ providers.ChatRequest) (*providers.ChatResponse, error) {
	if p.onChat != nil {
		p.onChat()
	}
	return &providers.ChatResponse{Content: p.resp, FinishReason: "stop"}, nil
}

// TestPlanPartyConflict verifies PlanParty refuses to overwrite a party that
// changed while the (long) AI call was in flight.
func TestPlanPartyConflict(t *testing.T) {
	svc, _ := newService(t)
	svc.SetProvider(&planProvider{}) // placeholder; replaced below

	name, err := svc.NewSession("crypt")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	// Close the session at the end so background autosaves are flushed and stop
	// before the temp dir is torn down (avoids a cleanup race on the roster dir).
	defer func() { _ = svc.CloseSession(name) }()
	if err := svc.SetParty(name, []*domain.Character{domain.NewCharacter("Alden", "Human", "Fighter")}); err != nil {
		t.Fatalf("SetParty: %v", err)
	}

	// The provider mutates the party during the AI call, so the baseline PlanParty
	// captured beforehand no longer matches when it goes to apply the plan.
	prov := &planProvider{
		resp: `{"members":[{"name":"Zed","race":"Human","class":"Rogue","level":1}]}`,
		onChat: func() {
			_ = svc.SetParty(name, []*domain.Character{domain.NewCharacter("Bree", "Halfling", "Bard")})
		},
	}
	svc.SetProvider(prov)

	_, err = svc.PlanParty(context.Background(), name, "make a rogue")
	if !errors.Is(err, ErrPartyConflict) {
		t.Fatalf("PlanParty = %v; want ErrPartyConflict", err)
	}
	// The concurrent edit survives (not overwritten by the stale plan).
	party, _ := svc.Party(name)
	if len(party) != 1 || party[0].Name != "Bree" {
		t.Errorf("party = %+v; want the concurrent edit (Bree) preserved", party)
	}
}

// TestSavePartyToRosterLinksByIndex verifies roster IDs are linked back to the
// exact party members (by position) and that the roster receives every member.
func TestSavePartyToRosterLinksByIndex(t *testing.T) {
	svc, store := newService(t)
	name, err := svc.NewSession("crypt")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer func() { _ = svc.CloseSession(name) }()
	if err := svc.SetParty(name, []*domain.Character{
		domain.NewCharacter("Alden", "Human", "Fighter"),
		domain.NewCharacter("Naivara", "Elf", "Wizard"),
	}); err != nil {
		t.Fatalf("SetParty: %v", err)
	}

	if err := svc.SavePartyToRoster(name); err != nil {
		t.Fatalf("SavePartyToRoster: %v", err)
	}

	party, _ := svc.Party(name)
	if len(party) != 2 || party[0].ID == "" || party[1].ID == "" || party[0].ID == party[1].ID {
		t.Fatalf("each member should get a distinct roster ID: %+v", party)
	}
	chars, err := store.ListCharacters()
	if err != nil || len(chars) != 2 {
		t.Fatalf("roster should contain 2 characters, got %d (%v)", len(chars), err)
	}
}

// TestSetPartyRejectsDuplicateID: two members sharing a non-empty roster ID are
// rejected, so no frontend can cause the roster write-back to collide (#129).
func TestSetPartyRejectsDuplicateID(t *testing.T) {
	svc, _ := newService(t)
	name, err := svc.NewSession("crypt")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	a := domain.NewCharacter("Alden", "Human", "Fighter")
	a.ID = "alden-abc123"
	dup := domain.NewCharacter("Alden (copy)", "Human", "Fighter")
	dup.ID = a.ID // same roster id → would clobber on write-back

	if err := svc.SetParty(name, []*domain.Character{a, dup}); !errors.Is(err, ErrDuplicatePartyMember) {
		t.Fatalf("SetParty with duplicate ids = %v; want ErrDuplicatePartyMember", err)
	}
	// Distinct ids (and empty-id members) are fine.
	b := domain.NewCharacter("Bree", "Halfling", "Bard")
	b.ID = "bree-def456"
	if err := svc.SetParty(name, []*domain.Character{a, b, domain.NewCharacter("NoLink", "Elf", "Wizard")}); err != nil {
		t.Fatalf("SetParty with distinct ids = %v; want nil", err)
	}
}

package engine

import (
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// paritySession builds a session with an NPC, an event and a rollable table, so
// the web/Telegram parity commands (/met, /trigger, /table, /begin) can be
// exercised without touching disk.
func paritySession() *domain.Session {
	adv := &domain.Adventure{
		SchemaVersion: domain.SchemaVersion,
		ID:            "test",
		Title:         "Test Adventure",
		Zones: []domain.Zone{{
			ID:   "z1",
			Name: "Zone One",
			Rooms: []domain.Room{
				{ID: "r1", Name: "Entrance", NPCIDs: []string{"guard"}, EventIDs: []string{"ambush"}},
			},
		}},
		NPCs:   []domain.NPC{{ID: "guard", Name: "Gate Guard", Role: "sentry"}},
		Events: []domain.Event{{ID: "ambush", Name: "Bandit Ambush"}},
		Tables: []domain.Table{{
			ID:   "loot",
			Name: "Loot",
			Rows: []domain.TableRow{
				{Roll: "1", Cells: []string{"A rusty dagger"}},
				{Roll: "2", Cells: []string{"A pouch of coins"}},
			},
		}},
	}
	state := domain.NewSessionState("test_session", adv)
	return domain.NewSession(state, adv, domain.DefaultConfig())
}

func TestCommandHandlerMet(t *testing.T) {
	session := paritySession()
	handler := NewCommandHandler(session)

	res := handler.Execute(ParseCommand("/met guard"))
	if !res.Success {
		t.Fatalf("/met guard failed: %s", res.Message)
	}
	if st := session.State.KnownNPCs["guard"]; st == nil || !st.Met {
		t.Errorf("guard should be marked met, got %+v", st)
	}

	if res := handler.Execute(ParseCommand("/met ghost")); res.Success {
		t.Error("marking an unknown NPC as met should fail")
	}
	if res := handler.Execute(ParseCommand("/met")); res.Success {
		t.Error("/met with no argument should fail")
	}
}

func TestCommandHandlerTrigger(t *testing.T) {
	session := paritySession()
	handler := NewCommandHandler(session)

	res := handler.Execute(ParseCommand("/trigger ambush"))
	if !res.Success {
		t.Fatalf("/trigger ambush failed: %s", res.Message)
	}
	if !session.State.TriggeredEvents["ambush"] {
		t.Error("ambush event should be marked triggered")
	}

	if res := handler.Execute(ParseCommand("/trigger nope")); res.Success {
		t.Error("triggering an unknown event should fail")
	}
}

func TestCommandHandlerTable(t *testing.T) {
	session := paritySession()
	handler := NewCommandHandler(session)

	before := session.State.LogLen()
	res := handler.Execute(ParseCommand("/table loot"))
	if !res.Success || res.Message == "" {
		t.Fatalf("/table loot should return a roll result: %+v", res)
	}
	if session.State.LogLen() <= before {
		t.Error("/table should record the roll on the timeline")
	}

	if res := handler.Execute(ParseCommand("/table missing")); res.Success {
		t.Error("rolling on an unknown table should fail")
	}
}

func TestCommandHandlerBegin(t *testing.T) {
	session := paritySession()
	handler := NewCommandHandler(session)

	// In assistant mode /begin is refused.
	if res := handler.Execute(ParseCommand("/begin")); res.Success {
		t.Error("/begin should be refused outside virtual-DM mode")
	}

	handler.Execute(ParseCommand("/mode dm"))
	res := handler.Execute(ParseCommand("/begin"))
	if !res.Success {
		t.Fatalf("/begin in DM mode failed: %s", res.Message)
	}
	if !res.NeedsUI || res.UIAction != "oracle" || res.UIArg == "" {
		t.Errorf("/begin should request an oracle kickoff, got %+v", res)
	}
	if !session.State.GameStarted() {
		t.Error("/begin should mark the game started")
	}

	// A second /begin is a no-op that reports the game already began.
	res = handler.Execute(ParseCommand("/begin"))
	if !res.Success || res.NeedsUI {
		t.Errorf("second /begin should be a plain no-op message, got %+v", res)
	}
}

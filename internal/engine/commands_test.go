package engine

import (
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		input   string
		cmdType CommandType
		argsLen int
	}{
		{"/help", CmdHelp, 0},
		{"/?", CmdHelp, 0},
		{":help", CmdHelp, 0},
		{"/quit", CmdQuit, 0},
		{"/save mySession", CmdSave, 1},
		{"/load mySession", CmdLoad, 1},
		{"/import /tmp/a.tar.gz", CmdImport, 1},
		{"/goto r1", CmdGoto, 1},
		{"/room", CmdRoom, 0},
		{"/look", CmdRoom, 0},
		{"/npc guard", CmdNPC, 1},
		{"/npcs", CmdNPCs, 0},
		{"/event ambush", CmdEvent, 1},
		{"/item sword", CmdItem, 1},
		{"/map z1", CmdMap, 1},
		{"/art guard", CmdArt, 1},
		{"/note the party rested", CmdNote, 3},
		{"/flag gate=true", CmdFlag, 1},
		{"/roll 2d6+3", CmdRoll, 1},
		{"/search altar", CmdSearch, 1},
		{"/status", CmdStatus, 0},
		{"What should happen here?", CmdOracle, 1},
		{"/unknown", CmdUnknown, 0},
		{"", CommandType(0), 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cmd := ParseCommand(tt.input)
			if tt.input == "" {
				if cmd != nil {
					t.Errorf("ParseCommand(%q) = %v, want nil", tt.input, cmd)
				}
				return
			}
			if cmd == nil {
				t.Fatalf("ParseCommand(%q) = nil", tt.input)
			}
			if cmd.Type != tt.cmdType {
				t.Errorf("Type = %v, want %v", cmd.Type, tt.cmdType)
			}
			if len(cmd.Args) != tt.argsLen {
				t.Errorf("args len = %d, want %d", len(cmd.Args), tt.argsLen)
			}
		})
	}
}

func TestOracleQueryRouting(t *testing.T) {
	cmd := ParseCommand("How does the guard react?")
	if cmd.Type != CmdOracle {
		t.Fatalf("expected CmdOracle, got %v", cmd.Type)
	}
	handler := NewCommandHandler(createTestSession())
	res := handler.Execute(cmd)
	if !res.NeedsUI || res.UIAction != "oracle" {
		t.Errorf("expected NeedsUI oracle action, got %+v", res)
	}
	if res.UIArg == "" {
		t.Error("oracle query should carry the input")
	}
}

func TestCommandHandlerHelp(t *testing.T) {
	handler := NewCommandHandler(createTestSession())
	res := handler.Execute(ParseCommand("/help"))
	if !res.Success || res.Response == "" {
		t.Error("help should succeed and return text")
	}
}

func TestCommandHandlerRoll(t *testing.T) {
	session := createTestSession()
	handler := NewCommandHandler(session)
	before := session.State.Log.Len()
	res := handler.Execute(ParseCommand("/roll 2d6"))
	if !res.Success {
		t.Errorf("roll failed: %s", res.Message)
	}
	if session.State.Log.Len() != before+1 {
		t.Error("roll should append a timeline entry")
	}
}

func TestCommandHandlerGoto(t *testing.T) {
	session := createTestSession()
	handler := NewCommandHandler(session)
	res := handler.Execute(ParseCommand("/goto r2"))
	if !res.Success {
		t.Errorf("goto failed: %s", res.Message)
	}
	if session.State.CurrentRoom != "r2" {
		t.Errorf("CurrentRoom = %q, want r2", session.State.CurrentRoom)
	}
	if !session.State.VisitedRooms["r2"] {
		t.Error("goto should mark the room visited")
	}
}

func TestCommandHandlerNPC(t *testing.T) {
	handler := NewCommandHandler(createTestSession())
	res := handler.Execute(ParseCommand("/npc guard"))
	if !res.Success || res.Response == "" {
		t.Errorf("npc dossier should be returned: %+v", res)
	}
	res = handler.Execute(ParseCommand("/npc ghost"))
	if res.Success {
		t.Error("unknown npc should fail")
	}
}

func TestCommandHandlerFlagAndNote(t *testing.T) {
	session := createTestSession()
	handler := NewCommandHandler(session)

	handler.Execute(ParseCommand("/flag gate=true"))
	if !session.State.Flags["gate"] {
		t.Error("flag gate should be true")
	}

	handler.Execute(ParseCommand("/note the party bribed the guard"))
	found := false
	for _, e := range session.State.Log.Entries {
		if e.Type == domain.LogNote {
			found = true
		}
	}
	if !found {
		t.Error("note should appear in the timeline")
	}
}

func TestCommandHandlerMap(t *testing.T) {
	handler := NewCommandHandler(createTestSession())
	res := handler.Execute(ParseCommand("/map z1"))
	if !res.NeedsUI || res.UIAction != "image" {
		t.Errorf("map should request an image open, got %+v", res)
	}
	if res.UIArg != "assets/map.png" {
		t.Errorf("UIArg = %q, want assets/map.png", res.UIArg)
	}
}

func TestCommandHandlerQuit(t *testing.T) {
	handler := NewCommandHandler(createTestSession())
	res := handler.Execute(ParseCommand("/quit"))
	if !res.ShouldQuit {
		t.Error("quit should set ShouldQuit")
	}
}

func createTestSession() *domain.Session {
	adv := &domain.Adventure{
		SchemaVersion: domain.SchemaVersion,
		ID:            "test",
		Title:         "Test Adventure",
		Zones: []domain.Zone{{
			ID:       "z1",
			Name:     "Zone One",
			MapImage: "assets/map.png",
			Rooms: []domain.Room{
				{ID: "r1", Name: "Entrance", NPCIDs: []string{"guard"}},
				{ID: "r2", Name: "Hall"},
			},
		}},
		NPCs: []domain.NPC{{ID: "guard", Name: "Gate Guard", Role: "sentry"}},
	}
	state := domain.NewSessionState("test_session", adv)
	return domain.NewSession(state, adv, domain.DefaultConfig())
}

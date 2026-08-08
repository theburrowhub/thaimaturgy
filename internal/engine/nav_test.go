package engine

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/types"
)

// navSession builds: entrance --east--> hall (and hall --west--> entrance),
// plus a far, non-adjacent zone "vault".
func navSession() *domain.Session {
	adv := &domain.Adventure{
		SchemaVersion: domain.SchemaVersion, ID: "nav", Title: "Nav",
		Zones: []domain.Zone{
			{ID: "entrance", Name: "Entrance", Rooms: []domain.Room{
				{ID: "e1", Name: "Gate", Exits: []domain.Exit{{Direction: "east", To: "h1"}}},
			}, Exits: []domain.ZoneExit{{Direction: domain.DirEast, To: "hall"}}},
			{ID: "hall", Name: "Great Hall", Rooms: []domain.Room{{ID: "h1", Name: "Hall"}},
				Exits: []domain.ZoneExit{{Direction: domain.DirWest, To: "entrance"}}},
			{ID: "vault", Name: "Vault", Rooms: []domain.Room{{ID: "v1", Name: "Vault"}}},
		},
	}
	state := domain.NewSessionState("nav_session", adv)
	return domain.NewSession(state, adv, domain.DefaultConfig())
}

func call(tr *ToolRouter, name string, args map[string]any) types.ToolResult {
	b, _ := json.Marshal(args)
	return tr.Execute(types.ToolCall{Name: name, Arguments: b})
}

func TestListExitsTool(t *testing.T) {
	tr := NewToolRouter(navSession())
	res := call(tr, "list_exits", nil)
	if res.Error != "" {
		t.Fatalf("list_exits error: %s", res.Error)
	}
	if !strings.Contains(res.Content, "Great Hall") || !strings.Contains(strings.ToLower(res.Content), "east") {
		t.Errorf("list_exits content missing adjacency: %s", res.Content)
	}
}

func TestGoDirectionTool(t *testing.T) {
	s := navSession()
	tr := NewToolRouter(s)
	res := call(tr, "go_direction", map[string]any{"direction": "este"}) // spanish alias -> east
	if res.Error != "" {
		t.Fatalf("go_direction error: %s", res.Error)
	}
	if s.State.CurrentRoom != "h1" {
		t.Errorf("after go east, current room = %q; want h1", s.State.CurrentRoom)
	}
	// No exit north from the hall.
	res = call(tr, "go_direction", map[string]any{"direction": "north"})
	if res.Error == "" {
		t.Errorf("expected error going a direction with no exit, got content=%q", res.Content)
	}
}

func TestFindPathTool(t *testing.T) {
	tr := NewToolRouter(navSession()) // starts at entrance
	res := call(tr, "find_path", map[string]any{"to_zone": "hall"})
	if res.Error != "" || !strings.Contains(res.Content, "Great Hall") {
		t.Errorf("find_path to hall = %q (err %q)", res.Content, res.Error)
	}
	res = call(tr, "find_path", map[string]any{"to_zone": "vault"}) // unreachable
	if !strings.Contains(res.Content, "No known route") {
		t.Errorf("find_path to unreachable vault = %q", res.Content)
	}
}

func TestSetLocationAdjacencyWarning(t *testing.T) {
	s := navSession()
	tr := NewToolRouter(s)
	// Teleport from entrance directly to the non-adjacent vault: allowed, but warned.
	res := call(tr, "set_location", map[string]any{"room_id": "v1"})
	if res.Error != "" {
		t.Fatalf("set_location error: %s", res.Error)
	}
	if !strings.Contains(res.Content, "not directly adjacent") {
		t.Errorf("expected adjacency warning, got: %s", res.Content)
	}
	// Moving to a genuinely adjacent zone must NOT warn.
	s2 := navSession()
	tr2 := NewToolRouter(s2)
	res2 := call(tr2, "set_location", map[string]any{"room_id": "h1"})
	if strings.Contains(res2.Content, "not directly adjacent") {
		t.Errorf("unexpected warning moving to adjacent hall: %s", res2.Content)
	}
}

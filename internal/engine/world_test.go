package engine

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// worldSession builds a one-room adventure whose room describes a suit of armor,
// plus an NPC and an item, so we can exercise the mutable-world overlay (#21).
func worldSession() *domain.Session {
	adv := &domain.Adventure{
		SchemaVersion: domain.SchemaVersion, ID: "world", Title: "World",
		Zones: []domain.Zone{
			{ID: "crypt", Name: "Crypt", Rooms: []domain.Room{
				{ID: "altar", Name: "Altar Room",
					ReadAloud: "A suit of armor stands beside the altar.",
					NPCIDs:    []string{"warden"}},
			}},
		},
		NPCs:  []domain.NPC{{ID: "warden", Name: "The Warden", Appearance: "A silent guardian."}},
		Items: []domain.Item{{ID: "armor", Name: "Ancient Armor", Description: "A dented suit of plate."}},
	}
	state := domain.NewSessionState("world_session", adv)
	return domain.NewSession(state, adv, domain.DefaultConfig())
}

func TestRecordWorldChangeAndRetrieval(t *testing.T) {
	s := worldSession()
	tr := NewToolRouter(s)

	// Record the armor being moved out of the altar room.
	res := call(tr, "record_world_change", map[string]any{
		"kind":   "room",
		"id":     "altar",
		"change": "The suit of armor has been dragged into the hallway and no longer stands by the altar.",
	})
	if res.Error != "" {
		t.Fatalf("record_world_change error: %s", res.Error)
	}
	if !s.IsModified {
		t.Errorf("recording a world change should mark the session modified")
	}

	// A later read of the room must surface the change as current state.
	res = call(tr, "get_room", map[string]any{"room_id": "altar"})
	if res.Error != "" {
		t.Fatalf("get_room error: %s", res.Error)
	}
	if !strings.Contains(res.Content, "CURRENT STATE") || !strings.Contains(res.Content, "dragged into the hallway") {
		t.Errorf("get_room did not layer the recorded change:\n%s", res.Content)
	}

	// list_world_changes reports the same change.
	res = call(tr, "list_world_changes", map[string]any{"kind": "room", "id": "altar"})
	if res.Error != "" || !strings.Contains(res.Content, "dragged into the hallway") {
		t.Errorf("list_world_changes = %q (err %q)", res.Content, res.Error)
	}
}

func TestRecordWorldChangeRejectsUnknownEntity(t *testing.T) {
	tr := NewToolRouter(worldSession())

	// Unknown id.
	if res := call(tr, "record_world_change", map[string]any{"kind": "room", "id": "nope", "change": "x"}); res.Error == "" {
		t.Errorf("expected error for unknown room id, got content=%q", res.Content)
	}
	// Unknown kind (schema enum should also guard, but the router must be safe too).
	if res := call(tr, "record_world_change", map[string]any{"kind": "planet", "id": "altar", "change": "x"}); res.Error == "" {
		t.Errorf("expected error for unknown kind, got content=%q", res.Content)
	}
	// Empty change text.
	if res := call(tr, "record_world_change", map[string]any{"kind": "room", "id": "altar", "change": "   "}); res.Error == "" {
		t.Errorf("expected error for empty change, got content=%q", res.Content)
	}
	// Nothing should have been recorded.
	if got := tr.state().WorldChangesFor(worldTarget("room", "altar")); len(got) != 0 {
		t.Errorf("no change should have been recorded, got %d", len(got))
	}
}

func TestBuildSystemPromptIncludesWorldChanges(t *testing.T) {
	s := worldSession()
	tr := NewToolRouter(s)

	// Change both the current room and the present NPC.
	call(tr, "record_world_change", map[string]any{"kind": "room", "id": "altar", "change": "The armor is gone."})
	call(tr, "record_world_change", map[string]any{"kind": "npc", "id": "warden", "change": "The Warden now lies dead on the floor."})

	prompt := NewOracle(s, nil).buildSystemPrompt()
	if !strings.Contains(prompt, "The armor is gone.") {
		t.Errorf("system prompt missing current-room world change:\n%s", prompt)
	}
	if !strings.Contains(prompt, "The Warden now lies dead") {
		t.Errorf("system prompt missing present-NPC world change:\n%s", prompt)
	}
}

func TestWorldEditsSurviveJSONRoundTrip(t *testing.T) {
	s := worldSession()
	s.State.RecordWorldChange(worldTarget("item", "armor"), "item Ancient Armor", "The armor is now bent and useless.")

	b, err := json.Marshal(s.State)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var loaded domain.SessionState
	if err := json.Unmarshal(b, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := loaded.WorldChangesFor(worldTarget("item", "armor"))
	if len(got) != 1 || !strings.Contains(got[0].Change, "bent and useless") {
		t.Errorf("world edit did not survive round-trip: %+v", got)
	}
}

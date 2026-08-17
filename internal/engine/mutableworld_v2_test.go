package engine

import (
	"strings"
	"testing"
)

// A full current-description override supersedes the authored read-aloud: the
// stale original is suppressed from the (trusted) system prompt, and only the
// current description appears — as untrusted data. Single source of truth. (#96)
func TestWorldDescriptionSupersedesAuthored(t *testing.T) {
	s := worldSession()
	o := NewOracle(s, nil)

	// Baseline: authored read-aloud is in the system prompt.
	if !strings.Contains(o.buildSystemPrompt(), "A suit of armor stands beside the altar.") {
		t.Fatal("baseline system prompt should contain the authored read-aloud")
	}

	const current = "The altar room is scorched black; the armor is gone, dragged into the hallway."
	s.State.SetWorldDescription(worldTarget("room", "altar"), current)

	prompt := o.buildSystemPrompt()
	if strings.Contains(prompt, "A suit of armor stands beside the altar.") {
		t.Errorf("authored read-aloud must be SUPPRESSED once overridden:\n%s", prompt)
	}
	if strings.Contains(prompt, current) {
		t.Errorf("the (untrusted) current description must NOT go in the system prompt:\n%s", prompt)
	}
	ws := o.worldStateContext()
	if !strings.Contains(ws, current) || !strings.Contains(ws, "SUPERSEDES") || !strings.Contains(ws, "untrusted") {
		t.Errorf("current description should be delivered as untrusted world state:\n%s", ws)
	}
	// The authored module must never be mutated in place.
	if room, _ := s.Adventure.Room("altar"); room.ReadAloud != "A suit of armor stands beside the altar." {
		t.Errorf("authored room read-aloud was mutated: %q", room.ReadAloud)
	}
}

// Clearing the override (empty description) reverts to the authored text.
func TestWorldDescriptionClearReverts(t *testing.T) {
	s := worldSession()
	o := NewOracle(s, nil)
	tgt := worldTarget("room", "altar")
	s.State.SetWorldDescription(tgt, "scorched ruin")
	if _, set := s.State.SetWorldDescription(tgt, "   "); set {
		t.Error("a blank description should clear the override")
	}
	if s.State.WorldDescription(tgt) != "" {
		t.Error("override should be cleared")
	}
	if !strings.Contains(o.buildSystemPrompt(), "A suit of armor stands beside the altar.") {
		t.Error("authored read-aloud should reappear after clearing")
	}
}

// An NPC override suppresses the authored appearance the same way.
func TestWorldDescriptionNPCAppearance(t *testing.T) {
	s := worldSession()
	o := NewOracle(s, nil)
	s.State.SetWorldDescription(worldTarget("npc", "warden"), "The Warden lies dead, slumped against the wall.")
	prompt := o.buildSystemPrompt()
	if strings.Contains(prompt, "A silent guardian.") {
		t.Errorf("overridden NPC appearance should be suppressed:\n%s", prompt)
	}
	if !strings.Contains(o.worldStateContext(), "lies dead") {
		t.Error("NPC current description should be in the untrusted world state")
	}
}

// The set_world_description tool sets/clears and validates the entity.
func TestSetWorldDescriptionTool(t *testing.T) {
	s := worldSession()
	tr := NewToolRouter(s)
	if res := call(tr, "set_world_description", map[string]any{"kind": "room", "id": "altar", "description": "burned out"}); res.Error != "" {
		t.Fatalf("set_world_description: %v", res.Error)
	}
	if s.State.WorldDescription(worldTarget("room", "altar")) == "" {
		t.Error("tool should have set the override")
	}
	// Empty description clears.
	call(tr, "set_world_description", map[string]any{"kind": "room", "id": "altar", "description": ""})
	if s.State.WorldDescription(worldTarget("room", "altar")) != "" {
		t.Error("empty description should clear the override")
	}
	// Unknown entity / kind → error.
	if res := call(tr, "set_world_description", map[string]any{"kind": "room", "id": "nope", "description": "x"}); res.Error == "" {
		t.Error("unknown room should error")
	}
	if res := call(tr, "set_world_description", map[string]any{"kind": "zone", "id": "crypt", "description": "x"}); res.Error == "" {
		t.Error("unsupported kind should error")
	}
}

// Retrieval tools must reflect the override too (not just current-room
// grounding): get_room/get_npc suppress the authored text and show the current
// description; list_world_changes reports the override. (#97 review)
func TestRetrievalReflectsOverride(t *testing.T) {
	s := worldSession()
	tr := NewToolRouter(s)
	s.State.SetWorldDescription(worldTarget("room", "altar"), "The altar room is a scorched ruin.")
	s.State.SetWorldDescription(worldTarget("npc", "warden"), "The Warden lies dead.")

	gr := call(tr, "get_room", map[string]any{"room_id": "altar"})
	if strings.Contains(gr.Content, "A suit of armor stands beside the altar.") || !strings.Contains(gr.Content, "scorched ruin") {
		t.Errorf("get_room should show the current description, not the authored:\n%s", gr.Content)
	}
	gn := call(tr, "get_npc", map[string]any{"npc_id": "warden"})
	if strings.Contains(gn.Content, "A silent guardian.") || !strings.Contains(gn.Content, "lies dead") {
		t.Errorf("get_npc should show the current description, not the authored:\n%s", gn.Content)
	}
	lw := call(tr, "list_world_changes", map[string]any{"kind": "room", "id": "altar"})
	if strings.Contains(lw.Content, "as authored") || !strings.Contains(lw.Content, "scorched ruin") {
		t.Errorf("list_world_changes should report the override, not 'as authored':\n%s", lw.Content)
	}
	// The override must keep its untrusted-data framing here too (no bare prose).
	if !strings.Contains(lw.Content, "untrusted") || !strings.Contains(lw.Content, "CURRENT WORLD STATE") {
		t.Errorf("list_world_changes must wrap the override as untrusted data:\n%s", lw.Content)
	}
}

// record_world_change is rejected for a target that has a full override (else the
// note would be silently dropped). (#97 review)
func TestRecordWorldChangeRejectedWhenOverridden(t *testing.T) {
	s := worldSession()
	tr := NewToolRouter(s)
	s.State.SetWorldDescription(worldTarget("room", "altar"), "scorched ruin")
	res := call(tr, "record_world_change", map[string]any{"kind": "room", "id": "altar", "change": "a door is broken"})
	if res.Error == "" {
		t.Error("record_world_change on an overridden target should be rejected with guidance")
	}
}

// A MISSING description must not silently clear an override; an explicit empty
// string still clears. (#97 review)
func TestSetWorldDescriptionRequiresField(t *testing.T) {
	s := worldSession()
	tr := NewToolRouter(s)
	tgt := worldTarget("room", "altar")
	s.State.SetWorldDescription(tgt, "scorched ruin")
	// Field omitted → error, override untouched.
	if res := call(tr, "set_world_description", map[string]any{"kind": "room", "id": "altar"}); res.Error == "" {
		t.Error("a missing description must be rejected, not treated as a clear")
	}
	if s.State.WorldDescription(tgt) == "" {
		t.Error("the override must survive a rejected (missing-description) call")
	}
	// Explicit empty string → clears.
	if res := call(tr, "set_world_description", map[string]any{"kind": "room", "id": "altar", "description": ""}); res.Error != "" {
		t.Errorf("explicit empty description should clear: %v", res.Error)
	}
	if s.State.WorldDescription(tgt) != "" {
		t.Error("explicit empty should have cleared the override")
	}
}

// A world override takes precedence over a scene's read-aloud (#84): with both,
// the prompt shows neither authored nor scene text, only the world state does.
func TestWorldDescriptionBeatsScene(t *testing.T) {
	s := sceneSession() // room "hall": authored + a night-scene read-aloud override
	s.State.SetScene("night", "Private Viewing")
	s.State.SetWorldDescription(worldTarget("room", "hall"), "The hall now lies in rubble.")
	o := NewOracle(s, nil)
	prompt := o.buildSystemPrompt()
	if strings.Contains(prompt, "A grand hall by day.") || strings.Contains(prompt, "The hall is hushed and dim.") {
		t.Errorf("world override should suppress both authored and scene read-aloud:\n%s", prompt)
	}
	if !strings.Contains(o.worldStateContext(), "lies in rubble") {
		t.Error("world override should be the current state")
	}
}

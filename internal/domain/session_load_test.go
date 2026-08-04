package domain

import (
	"encoding/json"
	"testing"
)

// TestUnmarshalInitializesMaps reproduces the Claude-CLI MCP crash: a session
// reloaded from JSON that omitted its (empty) maps must not panic when a tool
// then records an NPC, flag or visited room.
func TestUnmarshalInitializesMaps(t *testing.T) {
	// Minimal JSON with no known_npcs/flags/visited_rooms/etc.
	data := []byte(`{"name":"s","adventure_id":"a","current_room":"r1"}`)
	var st SessionState
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if st.KnownNPCs == nil || st.Flags == nil || st.VisitedRooms == nil ||
		st.TriggeredEvents == nil || st.Variables == nil || st.Log == nil || st.Conversation == nil {
		t.Fatal("maps/pointers should be initialized after unmarshal")
	}

	// These would panic on a nil map before the fix.
	st.MeetNPC("rose", "Rose")
	st.SetFlag("met_rose", true)
	st.SetLocation("z1", "r2", "Hall")
	st.SetVariable("k", "v")

	if s := st.NPCState("rose"); s == nil || !s.Met {
		t.Error("NPC should be recorded as met")
	}
	if !st.Flags["met_rose"] {
		t.Error("flag should be set")
	}
	if !st.VisitedRooms["r2"] {
		t.Error("room should be visited")
	}
}

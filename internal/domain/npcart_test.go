package domain

import "testing"

func TestNPCKnownReflectsMeet(t *testing.T) {
	adv := &Adventure{ID: "m", Title: "M",
		Zones: []Zone{{ID: "z", Rooms: []Room{{ID: "r"}}}},
		NPCs:  []NPC{{ID: "warden", Name: "Warden"}},
	}
	st := NewSessionState("s", adv)
	if st.NPCKnown("warden") {
		t.Fatal("NPC should be unknown before meeting")
	}
	st.MeetNPC("warden", "Warden")
	if !st.NPCKnown("warden") {
		t.Error("NPC should be known after MeetNPC")
	}
	if st.NPCKnown("ghost") {
		t.Error("unmet NPC must not be known")
	}
}

func TestNPCImagesResolve(t *testing.T) {
	adv := &Adventure{ID: "m", Title: "M",
		NPCs: []NPC{{ID: "n", Name: "N", Image: "assets/art/n.png"}},
	}
	if imgs := adv.NPCImages(&adv.NPCs[0]); len(imgs) != 1 || imgs[0] != "assets/art/n.png" {
		t.Errorf("NPCImages = %v; want [assets/art/n.png]", imgs)
	}
}

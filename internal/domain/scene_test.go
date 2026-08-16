package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func sceneAdventure() *Adventure {
	return &Adventure{
		SchemaVersion: SchemaVersion, ID: "heist", Title: "Heist",
		Zones: []Zone{{ID: "z1", Name: "Museum", Rooms: []Room{
			{ID: "hall", Name: "Main Hall", ReadAloud: "A grand hall.", NPCIDs: []string{"curator"}},
		}}},
		NPCs: []NPC{{ID: "curator", Name: "Curator"}, {ID: "guard", Name: "Guard"}},
		Scenes: []Scene{
			{ID: "day", Name: "Opening Hours", Initial: true,
				Rooms: []SceneRoom{{Room: "hall", Present: "crowded; the relic is not on display", NPCIDs: []string{"curator"}}},
				Next:  []SceneTransition{{To: "night", When: "the party returns for the private viewing"}}},
			{ID: "night", Name: "Private Viewing",
				Rooms: []SceneRoom{{Room: "hall", ReadAloud: "The hall is hushed and dim.", Present: "few guests; the relic is displayed; extra guards", NPCIDs: []string{"guard"}}}},
		},
	}
}

func TestSceneHelpers(t *testing.T) {
	a := sceneAdventure()
	if a.InitialSceneID() != "day" {
		t.Errorf("InitialSceneID = %q; want day", a.InitialSceneID())
	}
	if a.Scene("night") == nil || a.Scene("nope") != nil {
		t.Error("Scene lookup broken")
	}
	if sr := a.Scene("night").Room("hall"); sr == nil || sr.ReadAloud == "" {
		t.Errorf("Scene.Room override lookup broken: %+v", sr)
	}
	if a.Scene("day").Room("missing") != nil {
		t.Error("Scene.Room should be nil for an unlisted room")
	}
	// No initial flag → falls back to first scene.
	a.Scenes[0].Initial = false
	if a.InitialSceneID() != "day" {
		t.Errorf("InitialSceneID fallback = %q; want first scene 'day'", a.InitialSceneID())
	}
	// No scenes → empty.
	if (&Adventure{}).InitialSceneID() != "" {
		t.Error("InitialSceneID of a scene-less adventure should be empty")
	}
}

func TestValidateScenes(t *testing.T) {
	a := sceneAdventure()
	if errs := ValidateAdventure(a, nil); len(errs) != 0 {
		t.Fatalf("valid scene adventure reported errors: %v", errs)
	}

	bad := sceneAdventure()
	bad.Scenes = append(bad.Scenes, Scene{ID: "day"}) // duplicate id
	bad.Scenes[0].Initial = true
	bad.Scenes[1].Initial = true // two initials
	bad.Scenes[1].Rooms = []SceneRoom{{Room: "ghostroom", NPCIDs: []string{"ghostnpc"}}}
	bad.Scenes[1].Next = []SceneTransition{{To: "nowhere"}}
	errs := ValidateAdventure(bad, nil)
	joined := ""
	for _, e := range errs {
		joined += e.Error() + "\n"
	}
	for _, want := range []string{"duplicate id", "more than one scene is marked initial", "unknown room", "unknown npc", "unknown scene"} {
		if !strings.Contains(joined, want) {
			t.Errorf("validation missing %q in:\n%s", want, joined)
		}
	}
}

func TestNewSessionStateSeedsInitialScene(t *testing.T) {
	a := sceneAdventure()
	st := NewSessionState("s", a)
	if st.CurrentScene != "day" {
		t.Errorf("session should start in the initial scene, got %q", st.CurrentScene)
	}
	// Round-trip preserves the active scene.
	data, _ := json.Marshal(st)
	var back SessionState
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.CurrentScene != "day" {
		t.Errorf("CurrentScene lost across JSON round-trip: %q", back.CurrentScene)
	}
}

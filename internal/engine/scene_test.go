package engine

import (
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

func sceneSession() *domain.Session {
	adv := &domain.Adventure{
		SchemaVersion: domain.SchemaVersion, ID: "heist", Title: "Heist",
		Zones: []domain.Zone{{ID: "z1", Name: "Museum", Rooms: []domain.Room{
			{ID: "hall", Name: "Main Hall", ReadAloud: "A grand hall by day.", NPCIDs: []string{"curator"}},
		}}},
		NPCs:      []domain.NPC{{ID: "curator", Name: "Curator"}, {ID: "guard", Name: "Guard"}},
		StartRoom: "hall",
		Scenes: []domain.Scene{
			{ID: "day", Name: "Opening Hours", Initial: true,
				Next: []domain.SceneTransition{{To: "night", When: "the party returns at night"}}},
			{ID: "night", Name: "Private Viewing",
				Rooms: []domain.SceneRoom{{
					Room: "hall", ReadAloud: "The hall is hushed and dim.",
					Present: "the relic is displayed; extra guards", NPCIDs: []string{"guard"},
				}}},
		},
	}
	return domain.NewSession(domain.NewSessionState("heist_session", adv), adv, domain.DefaultConfig())
}

// The oracle must render the current room THROUGH the active scene: in the night
// scene the same hall shows the scene's read-aloud, present note and cast — the
// core of "same location, different state". (#84)
func TestBuildSystemPromptAppliesSceneOverride(t *testing.T) {
	s := sceneSession()
	o := NewOracle(s, nil)

	// Day scene (initial): authored defaults, and the transition is advertised.
	day := o.buildSystemPrompt()
	if !strings.Contains(day, "A grand hall by day.") {
		t.Errorf("day scene should show authored read-aloud:\n%s", day)
	}
	if !strings.Contains(day, "Current scene: Opening Hours") || !strings.Contains(day, "Private Viewing") {
		t.Errorf("day scene should name the scene and its transition:\n%s", day)
	}

	// Switch to night: same room, overridden presentation.
	s.State.SetScene("night", "Private Viewing")
	night := o.buildSystemPrompt()
	for _, want := range []string{
		"The hall is hushed and dim.",          // scene read-aloud override
		"the relic is displayed; extra guards", // the "present" note
		"Guard",                                // scene's present cast
	} {
		if !strings.Contains(night, want) {
			t.Errorf("night scene missing %q:\n%s", want, night)
		}
	}
	if strings.Contains(night, "A grand hall by day.") {
		t.Errorf("night scene should NOT show the day read-aloud:\n%s", night)
	}
	if strings.Contains(night, "Curator") {
		t.Errorf("night scene overrides the cast; Curator should not be present:\n%s", night)
	}
}

// A module without scenes is unaffected: the authored room renders as before and
// no scene framing appears.
func TestBuildSystemPromptNoScenes(t *testing.T) {
	adv := &domain.Adventure{
		SchemaVersion: domain.SchemaVersion, ID: "dungeon", Title: "Dungeon",
		Zones:     []domain.Zone{{ID: "z1", Name: "Cave", Rooms: []domain.Room{{ID: "r1", Name: "Mouth", ReadAloud: "A dark cave mouth."}}}},
		StartRoom: "r1",
	}
	s := domain.NewSession(domain.NewSessionState("d", adv), adv, domain.DefaultConfig())
	prompt := NewOracle(s, nil).buildSystemPrompt()
	if !strings.Contains(prompt, "A dark cave mouth.") {
		t.Errorf("scene-less adventure should render the authored room:\n%s", prompt)
	}
	if strings.Contains(prompt, "Current scene:") {
		t.Errorf("scene-less adventure should have no scene framing:\n%s", prompt)
	}
}

func TestSceneCommand(t *testing.T) {
	s := sceneSession()
	h := NewCommandHandler(s)

	// No arg: shows current scene + all scenes.
	res := h.Execute(ParseCommand("/scene"))
	if !res.Success || !strings.Contains(res.Response, "Opening Hours") || !strings.Contains(res.Response, "Private Viewing") {
		t.Fatalf("/scene listing = %+v", res)
	}

	// Switch scene.
	res = h.Execute(ParseCommand("/scene night"))
	if !res.Success || s.State.CurrentScene != "night" {
		t.Fatalf("/scene night should switch, got %+v (scene=%q)", res, s.State.CurrentScene)
	}

	// Unknown scene fails.
	if res := h.Execute(ParseCommand("/scene bogus")); res.Success {
		t.Error("/scene with an unknown id should fail")
	}
}

// set_scene tool switches the active scene and rejects unknown ids.
func TestSetSceneTool(t *testing.T) {
	s := sceneSession()
	tr := NewToolRouter(s)
	res := call(tr, "set_scene", map[string]any{"scene_id": "night"})
	if res.Error != "" || s.State.CurrentScene != "night" {
		t.Fatalf("set_scene should switch to night, got %+v (scene=%q)", res, s.State.CurrentScene)
	}
	if res := call(tr, "set_scene", map[string]any{"scene_id": "ghost"}); res.Error == "" {
		t.Error("set_scene with an unknown id should error")
	}
}

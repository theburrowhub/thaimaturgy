package wailsapp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

func TestAppListsAdventuresAndStartsSession(t *testing.T) {
	store := testStore(t)
	writeAdventure(t, store, sampleAdventure())

	app, err := NewWithStorage(store)
	if err != nil {
		t.Fatalf("NewWithStorage: %v", err)
	}

	library, err := app.GetLibrary()
	if err != nil {
		t.Fatalf("GetLibrary: %v", err)
	}
	if len(library.Adventures) != 1 || library.Adventures[0].ID != "crypt" {
		t.Fatalf("Adventures = %+v, want crypt", library.Adventures)
	}

	session, err := app.StartSession("crypt")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if session.State.CurrentRoom != "entry" || session.CurrentRoom.Name != "Entry Hall" {
		t.Fatalf("session current room = %+v / %+v", session.State, session.CurrentRoom)
	}
	if !store.SessionExists(session.State.Name) {
		t.Fatalf("started session was not persisted")
	}
}

func TestAppMovePartyAndSubmitSlashCommand(t *testing.T) {
	store := testStore(t)
	writeAdventure(t, store, sampleAdventure())
	app, _ := NewWithStorage(store)
	if _, err := app.StartSession("crypt"); err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	moved, err := app.MoveParty("altar")
	if err != nil {
		t.Fatalf("MoveParty: %v", err)
	}
	if moved.State.CurrentRoom != "altar" || !moved.State.VisitedRooms["altar"] {
		t.Fatalf("state after MoveParty = %+v", moved.State)
	}
	if moved.Detail == nil || !strings.Contains(moved.Detail.Markdown, "hidden reliquary") {
		t.Fatalf("detail after MoveParty = %+v", moved.Detail)
	}

	result, err := app.Submit("/goto entry")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if !result.Success || result.Session.State.CurrentRoom != "entry" {
		t.Fatalf("Submit result = %+v", result)
	}
}

func TestAppLibraryManagementAndAdventureDetail(t *testing.T) {
	store := testStore(t)
	writeAdventure(t, store, sampleAdventure())
	app, _ := NewWithStorage(store)

	adv := app.NewAdventureTemplate()
	adv.ID = "new-one"
	adv.Title = "New One"
	if _, err := app.SaveAdventure(adv); err != nil {
		t.Fatalf("SaveAdventure: %v", err)
	}
	lib, err := app.GetLibrary()
	if err != nil {
		t.Fatalf("GetLibrary: %v", err)
	}
	if len(lib.Adventures) != 2 {
		t.Fatalf("adventure count = %d, want 2", len(lib.Adventures))
	}

	if _, err := app.StartSession("crypt"); err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	detail, err := app.GetDetail("about")
	if err != nil {
		t.Fatalf("GetDetail: %v", err)
	}
	if !strings.Contains(detail.Markdown, "compact test crypt") {
		t.Fatalf("about detail = %q", detail.Markdown)
	}
	room, err := app.GetDetail("room:z1::entry")
	if err != nil {
		t.Fatalf("GetDetail room: %v", err)
	}
	if len(room.Groups) == 0 || !strings.Contains(room.Markdown, "Cold air") {
		t.Fatalf("room detail = %+v", room)
	}
}

func testStore(t *testing.T) *storage.Storage {
	t.Helper()
	store, err := storage.NewWithPath(t.TempDir())
	if err != nil {
		t.Fatalf("NewWithPath: %v", err)
	}
	return store
}

func writeAdventure(t *testing.T, store *storage.Storage, adv *domain.Adventure) {
	t.Helper()
	dir := store.AdventureDir(adv.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	data, err := json.MarshalIndent(adv, "", "  ")
	if err != nil {
		t.Fatalf("Marshal adventure: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, storage.AdventureFile), data, 0644); err != nil {
		t.Fatalf("Write adventure: %v", err)
	}
}

func sampleAdventure() *domain.Adventure {
	return &domain.Adventure{
		SchemaVersion: domain.SchemaVersion,
		ID:            "crypt",
		Title:         "The Sunken Crypt",
		Author:        "Test DM",
		System:        "D&D 5e",
		Summary:       "A compact test crypt.",
		Zones: []domain.Zone{{
			ID:       "z1",
			Name:     "Upper Crypt",
			Overview: "Wet stones and old vows.",
			Rooms: []domain.Room{
				{ID: "entry", Name: "Entry Hall", ReadAloud: "Cold air rolls down the stair.", DMNotes: "The floor is slick.", Exits: []domain.Exit{{To: "altar", Direction: "east"}}},
				{ID: "altar", Name: "Drowned Altar", ReadAloud: "Black water pools around a cracked altar.", DMNotes: "A hidden reliquary waits here."},
			},
		}},
	}
}

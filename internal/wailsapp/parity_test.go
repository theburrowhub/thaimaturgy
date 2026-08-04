package wailsapp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

func TestResetPartyInstallsDefaultRoster(t *testing.T) {
	store := testStore(t)
	writeAdventure(t, store, sampleAdventure())
	app, _ := NewWithStorage(store)
	if _, err := app.StartSession("crypt"); err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	p, err := app.ResetParty()
	if err != nil {
		t.Fatalf("ResetParty: %v", err)
	}
	if len(p.State.PartySnapshot()) == 0 {
		t.Fatalf("ResetParty produced an empty party")
	}
}

func TestSetModeEntersVirtualDMAndEnsuresParty(t *testing.T) {
	store := testStore(t)
	writeAdventure(t, store, sampleAdventure())
	app, _ := NewWithStorage(store)
	if _, err := app.StartSession("crypt"); err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	// Force no provider so the kickoff narration is skipped (keeps the test
	// hermetic and offline); the mode switch and party creation must still happen.
	app.prov = nil
	app.oracle = nil
	res, err := app.SetMode()
	if err != nil {
		t.Fatalf("SetMode: %v", err)
	}
	if res.Session.State.EffectiveMode() != domain.ModeVirtualDM {
		t.Fatalf("SetMode did not enter virtual DM: %s", res.Session.State.EffectiveMode())
	}
	if len(res.Session.State.PartySnapshot()) == 0 {
		t.Fatalf("SetMode did not ensure a party")
	}
	// Toggling again returns to oracle mode.
	back, err := app.SetMode()
	if err != nil {
		t.Fatalf("SetMode back: %v", err)
	}
	if back.Session.State.EffectiveMode() == domain.ModeVirtualDM {
		t.Fatalf("SetMode did not leave virtual DM")
	}
}

func TestValidateAdventureReportsProblems(t *testing.T) {
	store := testStore(t)
	app, _ := NewWithStorage(store)

	if problems, err := app.ValidateAdventure(sampleAdventure()); err != nil {
		t.Fatalf("ValidateAdventure(valid): %v", err)
	} else if len(problems) != 0 {
		t.Fatalf("valid adventure reported problems: %v", problems)
	}

	broken := &domain.Adventure{SchemaVersion: domain.SchemaVersion, ID: "", Title: ""}
	problems, err := app.ValidateAdventure(broken)
	if err != nil {
		t.Fatalf("ValidateAdventure(broken): %v", err)
	}
	if len(problems) == 0 {
		t.Fatalf("broken adventure reported no problems")
	}
}

func TestExportAdventureFolderCopiesModule(t *testing.T) {
	store := testStore(t)
	writeAdventure(t, store, sampleAdventure())
	app, _ := NewWithStorage(store)
	dest := filepath.Join(t.TempDir(), "out")
	if _, err := app.ExportAdventureFolder("crypt", dest); err != nil {
		t.Fatalf("ExportAdventureFolder: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, storage.AdventureFile)); err != nil {
		t.Fatalf("adventure.json not written to folder: %v", err)
	}
}

func writeAdventureTo(t *testing.T, dir string, adv *domain.Adventure) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	data, err := json.MarshalIndent(adv, "", "  ")
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, storage.AdventureFile), data, 0644); err != nil {
		t.Fatalf("Write: %v", err)
	}
}

func TestAutosaveDisabledDoesNotPersistMoves(t *testing.T) {
	store := testStore(t)
	writeAdventure(t, store, sampleAdventure())
	app, _ := NewWithStorage(store)
	s, err := app.StartSession("crypt")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	name := s.State.Name
	app.config.AutoSave = false
	if _, err := app.MoveParty("altar"); err != nil {
		t.Fatalf("MoveParty: %v", err)
	}
	// Reload from disk in a fresh app: the move must NOT have been persisted.
	app2, _ := NewWithStorage(store)
	r, err := app2.LoadSession(name)
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if r.State.CurrentRoom == "altar" {
		t.Fatalf("move was persisted despite auto-save being disabled")
	}
}

func TestExportAdventureDMBookWithoutSession(t *testing.T) {
	store := testStore(t)
	writeAdventure(t, store, sampleAdventure())
	app, _ := NewWithStorage(store)
	// No StartSession call — the editor-side DM book export must work session-less.
	out := filepath.Join(t.TempDir(), "crypt-dmbook.md")
	if _, err := app.ExportAdventureDMBook("crypt", out, false); err != nil {
		t.Fatalf("ExportAdventureDMBook: %v", err)
	}
	if b, err := os.ReadFile(out); err != nil || len(b) == 0 {
		t.Fatalf("DM book not written: %v (%d bytes)", err, len(b))
	}
}

func TestOpenExternalModuleInstallsAndReturns(t *testing.T) {
	// Build a .tar.gz module in a scratch dir, then open it into the editor.
	src := t.TempDir()
	writeAdventureTo(t, src, sampleAdventure())
	pkg := filepath.Join(t.TempDir(), "crypt.tar.gz")
	if err := storage.PackageModule(src, pkg); err != nil {
		t.Fatalf("PackageModule: %v", err)
	}

	store := testStore(t)
	app, _ := NewWithStorage(store)
	adv, err := app.OpenExternalModule(pkg)
	if err != nil {
		t.Fatalf("OpenExternalModule: %v", err)
	}
	if adv == nil || adv.ID != "crypt" {
		t.Fatalf("OpenExternalModule returned %+v", adv)
	}
	// It must now be installed in the library (so its assets travel with it).
	if _, err := store.LoadAdventure("crypt"); err != nil {
		t.Fatalf("opened module was not installed: %v", err)
	}
}

func TestSaveConfigPersistsTelegramTokenAndKeeps(t *testing.T) {
	store := testStore(t)
	app, _ := NewWithStorage(store)
	out, err := app.SaveConfig(ConfigPayload{Provider: "anthropic", Model: "claude-sonnet-5", TelegramToken: "secret-token", AnthropicAPIKey: "sk-test"})
	if err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	if !out.TelegramBotTokenSet {
		t.Fatalf("telegram token not marked set")
	}
	if !out.AnthropicAPIKeySet {
		t.Fatalf("anthropic key not marked set")
	}
	if out.AnthropicAPIKey != "" || out.TelegramToken != "" {
		t.Fatalf("configPayload leaked secrets back to the frontend")
	}
	// A subsequent save that omits the secrets must not wipe them.
	out2, err := app.SaveConfig(ConfigPayload{Provider: "anthropic", Model: "claude-sonnet-5"})
	if err != nil {
		t.Fatalf("SaveConfig(2): %v", err)
	}
	if !out2.TelegramBotTokenSet || !out2.AnthropicAPIKeySet {
		t.Fatalf("blank secret fields wiped previously-set credentials")
	}
}

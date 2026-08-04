package wailsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/engine"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

// App is the Go backend exposed to the Wails frontend.
type App struct {
	ctx     context.Context
	store   *storage.Storage
	config  *domain.Config
	session *domain.Session
}

type LibraryPayload struct {
	Adventures []storage.AdventureInfo `json:"adventures"`
	Sessions   []storage.SessionInfo   `json:"sessions"`
	Config     ConfigPayload           `json:"config"`
}

type ConfigPayload struct {
	Provider              string `json:"provider"`
	Model                 string `json:"model"`
	EditModel             string `json:"edit_model"`
	RunModel              string `json:"run_model"`
	Language              string `json:"language"`
	AutoSave              bool   `json:"auto_save"`
	RequestTimeoutSeconds int    `json:"request_timeout_seconds"`
	TelegramBotTokenSet   bool   `json:"telegram_bot_token_set"`
	TelegramChatID        int64  `json:"telegram_chat_id"`
	Configured            bool   `json:"configured"`
	ConfigPath            string `json:"config_path"`
	DataPath              string `json:"data_path"`
}

type SessionPayload struct {
	State       *domain.SessionState `json:"state"`
	Adventure   *domain.Adventure    `json:"adventure"`
	CurrentRoom *domain.Room         `json:"current_room,omitempty"`
	CurrentZone *domain.Zone         `json:"current_zone,omitempty"`
	Tree        []TreeNode           `json:"tree"`
	Detail      *DetailPayload       `json:"detail,omitempty"`
}

type SubmitResult struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Session *SessionPayload `json:"session,omitempty"`
}

type ActionResult struct {
	Message string          `json:"message"`
	Library *LibraryPayload `json:"library,omitempty"`
	Session *SessionPayload `json:"session,omitempty"`
}

func New() (*App, error) {
	store, err := storage.New()
	if err != nil {
		return nil, err
	}
	return NewWithStorage(store)
}

func NewWithStorage(store *storage.Storage) (*App, error) {
	_ = store.LoadEnvFile()
	config, err := store.LoadConfig()
	if err != nil {
		return nil, err
	}
	return &App{store: store, config: config}, nil
}

func (a *App) Startup(ctx context.Context) { a.ctx = ctx }

func (a *App) GetLibrary() (*LibraryPayload, error) {
	adventures, err := a.store.ListAdventures()
	if err != nil {
		return nil, err
	}
	sessions, err := a.store.ListSessions()
	if err != nil {
		return nil, err
	}
	sort.Slice(sessions, func(i, j int) bool { return modTime(sessions[i]).After(modTime(sessions[j])) })
	return &LibraryPayload{Adventures: adventures, Sessions: sessions, Config: a.configPayload()}, nil
}

func (a *App) GetConfig() ConfigPayload { return a.configPayload() }

func (a *App) SaveConfig(in ConfigPayload) (ConfigPayload, error) {
	if strings.TrimSpace(in.Provider) != "" {
		a.config.Provider = domain.ProviderType(strings.ToLower(strings.TrimSpace(in.Provider)))
	}
	if strings.TrimSpace(in.Model) != "" {
		a.config.Model = strings.TrimSpace(in.Model)
	}
	a.config.EditModel = strings.TrimSpace(in.EditModel)
	a.config.RunModel = strings.TrimSpace(in.RunModel)
	if strings.TrimSpace(in.Language) != "" {
		a.config.Language = domain.Language(strings.TrimSpace(in.Language))
	}
	a.config.AutoSave = in.AutoSave
	if in.RequestTimeoutSeconds > 0 {
		a.config.RequestTimeoutSeconds = in.RequestTimeoutSeconds
	}
	a.config.TelegramChatID = in.TelegramChatID
	if err := a.store.SaveConfig(a.config); err != nil {
		return ConfigPayload{}, err
	}
	return a.configPayload(), nil
}

func (a *App) configPayload() ConfigPayload {
	return ConfigPayload{
		Provider:              string(a.config.Provider),
		Model:                 a.config.Model,
		EditModel:             a.config.EditModel,
		RunModel:              a.config.RunModel,
		Language:              string(a.config.Language),
		AutoSave:              a.config.AutoSave,
		RequestTimeoutSeconds: a.config.RequestTimeoutSeconds,
		TelegramBotTokenSet:   strings.TrimSpace(a.config.TelegramToken) != "",
		TelegramChatID:        a.config.TelegramChatID,
		Configured:            a.config.IsConfigured(),
		ConfigPath:            a.store.ConfigPath(),
		DataPath:              a.store.BasePath(),
	}
}

func (a *App) GetAdventure(id string) (*domain.Adventure, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("adventure id is required")
	}
	return a.store.LoadAdventure(id)
}

func (a *App) NewAdventureTemplate() *domain.Adventure {
	return &domain.Adventure{
		SchemaVersion: domain.SchemaVersion,
		ID:            "new-adventure",
		Title:         "New Adventure",
		System:        "D&D 5e",
		Language:      string(a.config.Language),
		Summary:       "A new thAImaturgy module.",
		Zones: []domain.Zone{{
			ID:       "zone-1",
			Name:     "Starting Zone",
			Overview: "Describe the first region of the adventure.",
			Rooms: []domain.Room{{
				ID:        "start",
				Name:      "Opening Scene",
				ReadAloud: "Read this text to the players when the adventure begins.",
				DMNotes:   "Hidden notes for the DM.",
			}},
		}},
	}
}

func (a *App) SaveAdventure(adv *domain.Adventure) (*domain.Adventure, error) {
	if adv == nil {
		return nil, fmt.Errorf("adventure is required")
	}
	adv.ID = strings.TrimSpace(adv.ID)
	if adv.SchemaVersion == "" {
		adv.SchemaVersion = domain.SchemaVersion
	}
	if errs := domain.ValidateAdventure(adv, nil); len(errs) > 0 {
		return nil, fmt.Errorf("adventure validation failed:\n%s", joinErrors(errs))
	}
	dir := a.store.AdventureDir(adv.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(adv, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, storage.AdventureFile), data, 0644); err != nil {
		return nil, err
	}
	return a.store.LoadAdventure(adv.ID)
}

func (a *App) ImportAdventure(path string) (*ActionResult, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("module path is required")
	}
	adv, err := a.store.ImportModule(path)
	if err != nil {
		return nil, err
	}
	lib, _ := a.GetLibrary()
	return &ActionResult{Message: "Imported: " + adv.Title, Library: lib}, nil
}

func (a *App) DeleteAdventure(id string) (*LibraryPayload, error) {
	if err := a.store.DeleteAdventure(strings.TrimSpace(id)); err != nil {
		return nil, err
	}
	return a.GetLibrary()
}

func (a *App) DeleteSession(name string) (*LibraryPayload, error) {
	if err := a.store.DeleteSession(strings.TrimSpace(name)); err != nil {
		return nil, err
	}
	return a.GetLibrary()
}

func (a *App) RenameSession(oldName, newName string) (*LibraryPayload, error) {
	if err := a.store.RenameSession(strings.TrimSpace(oldName), strings.TrimSpace(newName)); err != nil {
		return nil, err
	}
	return a.GetLibrary()
}

func (a *App) StartSession(adventureID string) (*SessionPayload, error) {
	adv, err := a.GetAdventure(adventureID)
	if err != nil {
		return nil, err
	}
	name := adv.ID
	for i := 1; a.store.SessionExists(name); i++ {
		name = fmt.Sprintf("%s-%d", adv.ID, i)
	}
	state := domain.NewSessionState(name, adv)
	if err := a.store.SaveSession(state); err != nil {
		return nil, err
	}
	a.session = domain.NewSession(state, adv, a.config)
	return a.payloadWithDetail("room:" + state.CurrentZone + "::" + state.CurrentRoom)
}

func (a *App) LoadSession(name string) (*SessionPayload, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("session name is required")
	}
	state, err := a.store.LoadSession(name)
	if err != nil {
		return nil, err
	}
	adv, err := a.store.LoadAdventure(state.AdventureID)
	if err != nil {
		return nil, err
	}
	a.session = domain.NewSession(state, adv, a.config)
	uid := "about"
	if state.CurrentRoom != "" {
		uid = "room:" + state.CurrentZone + "::" + state.CurrentRoom
	}
	return a.payloadWithDetail(uid)
}

func (a *App) GetSession() (*SessionPayload, error) { return a.payload() }

func (a *App) GetDetail(uid string) (*DetailPayload, error) {
	if a.session == nil {
		return nil, fmt.Errorf("no active session")
	}
	return detailForUID(a.session.Adventure, a.session.State, uid), nil
}

func (a *App) MoveParty(roomID string) (*SessionPayload, error) {
	if a.session == nil {
		return nil, fmt.Errorf("no active session")
	}
	zone, room := findRoom(a.session.Adventure, roomID)
	if room == nil || zone == nil {
		return nil, fmt.Errorf("room not found: %s", roomID)
	}
	a.session.State.SetLocation(zone.ID, room.ID, room.Name)
	a.session.MarkModified()
	if err := a.store.SaveSession(a.session.State); err != nil {
		return nil, err
	}
	return a.payloadWithDetail("room:" + zone.ID + "::" + room.ID)
}

func (a *App) MarkNPCMet(id string) (*SessionPayload, error) {
	if a.session == nil {
		return nil, fmt.Errorf("no active session")
	}
	if n := a.session.Adventure.NPC(strings.TrimSpace(id)); n != nil {
		a.session.State.MeetNPC(n.ID, n.Name)
		a.session.MarkModified()
		_ = a.store.SaveSession(a.session.State)
		return a.payloadWithDetail("npc:" + n.ID)
	}
	return nil, fmt.Errorf("npc not found: %s", id)
}

func (a *App) TriggerEvent(id string) (*SessionPayload, error) {
	if a.session == nil {
		return nil, fmt.Errorf("no active session")
	}
	if ev := a.session.Adventure.Event(strings.TrimSpace(id)); ev != nil {
		a.session.State.TriggerEvent(ev.ID, ev.Name)
		a.session.MarkModified()
		_ = a.store.SaveSession(a.session.State)
		return a.payloadWithDetail("event:" + ev.ID)
	}
	return nil, fmt.Errorf("event not found: %s", id)
}

func (a *App) RollTable(id string) (*SubmitResult, error) {
	if a.session == nil {
		return nil, fmt.Errorf("no active session")
	}
	t := a.session.Adventure.Table(strings.TrimSpace(id))
	if t == nil {
		return nil, fmt.Errorf("table not found: %s", id)
	}
	roll, row := engine.RollTable(t)
	result := engine.RowText(row)
	if result == "" {
		result = "(no matching row)"
	}
	name := labelOrID(t.Name, t.ID)
	msg := fmt.Sprintf("🎲 %s — rolled %d: %s", name, roll, result)
	a.session.State.AddNote(fmt.Sprintf("Rolled %s (%d): %s", name, roll, result))
	a.session.MarkModified()
	_ = a.store.SaveSession(a.session.State)
	p, _ := a.payloadWithDetail("table:" + t.ID)
	return &SubmitResult{Success: true, Message: msg, Session: p}, nil
}

func (a *App) SaveSession() (*SubmitResult, error) {
	if a.session == nil {
		return nil, fmt.Errorf("no active session")
	}
	if err := a.store.SaveSession(a.session.State); err != nil {
		return nil, err
	}
	p, _ := a.payload()
	return &SubmitResult{Success: true, Message: "Session saved.", Session: p}, nil
}

func (a *App) Submit(input string) (*SubmitResult, error) {
	if a.session == nil {
		return nil, fmt.Errorf("no active session")
	}
	cmd := engine.ParseCommand(input)
	if cmd == nil {
		return &SubmitResult{Success: false, Message: "Enter a command or note."}, nil
	}
	result := engine.NewCommandHandler(a.session).Execute(cmd)
	if result.ShouldQuit {
		p, _ := a.payload()
		return &SubmitResult{Success: true, Message: "Back to library.", Session: p}, nil
	}
	msg := result.Message
	if result.Response != "" {
		msg = result.Response
	}
	if result.NeedsUI && result.UIAction == "oracle" {
		a.session.State.AddNote(strings.TrimSpace(input))
		msg = "Oracle prompt recorded. Full AI oracle answers will be wired in the next Wails slice."
	}
	a.session.MarkModified()
	if err := a.store.SaveSession(a.session.State); err != nil {
		return nil, err
	}
	p, _ := a.payload()
	return &SubmitResult{Success: result.Success, Message: msg, Session: p}, nil
}

func (a *App) payload() (*SessionPayload, error) {
	if a.session == nil {
		return nil, fmt.Errorf("no active session")
	}
	zone, room := findRoom(a.session.Adventure, a.session.State.CurrentRoom)
	return &SessionPayload{State: a.session.State, Adventure: a.session.Adventure, CurrentZone: zone, CurrentRoom: room, Tree: buildAdventureTree(a.session.Adventure, a.session.State)}, nil
}

func (a *App) payloadWithDetail(uid string) (*SessionPayload, error) {
	p, err := a.payload()
	if err != nil {
		return nil, err
	}
	p.Detail = detailForUID(a.session.Adventure, a.session.State, uid)
	return p, nil
}

func findRoom(adv *domain.Adventure, roomID string) (*domain.Zone, *domain.Room) {
	if adv == nil {
		return nil, nil
	}
	for zi := range adv.Zones {
		for ri := range adv.Zones[zi].Rooms {
			room := &adv.Zones[zi].Rooms[ri]
			if room.ID == roomID {
				return &adv.Zones[zi], room
			}
		}
	}
	return nil, nil
}

func modTime(s storage.SessionInfo) time.Time {
	if t, ok := s.ModifiedAt.(time.Time); ok {
		return t
	}
	return time.Time{}
}

func joinErrors(errs []error) string {
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		parts = append(parts, err.Error())
	}
	return strings.Join(parts, "\n")
}

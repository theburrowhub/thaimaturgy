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

	"github.com/theburrowhub/thaimaturgy/internal/bookpdf"
	"github.com/theburrowhub/thaimaturgy/internal/dmbook"
	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/engine"
	"github.com/theburrowhub/thaimaturgy/internal/novel"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

// App is the Go backend exposed to the Wails frontend.
type App struct {
	ctx     context.Context
	store   *storage.Storage
	config  *domain.Config
	prov    providers.Provider
	oracle  *engine.Oracle
	session *domain.Session
}

type LibraryPayload struct {
	Adventures []storage.AdventureInfo `json:"adventures"`
	Sessions   []storage.SessionInfo   `json:"sessions"`
	Config     ConfigPayload           `json:"config"`
}

type ConfigPayload struct {
	Provider                string  `json:"provider"`
	Model                   string  `json:"model"`
	EditModel               string  `json:"edit_model"`
	RunModel                string  `json:"run_model"`
	Language                string  `json:"language"`
	ImportLanguage          string  `json:"import_language"`
	Temperature             float64 `json:"temperature"`
	MaxTokens               int     `json:"max_tokens"`
	ImportMaxOutputTokens   int     `json:"import_max_output_tokens"`
	OracleMaxToolIterations int     `json:"oracle_max_tool_iterations"`
	RequestTimeoutSeconds   int     `json:"request_timeout_seconds"`
	AutoSave                bool    `json:"auto_save"`
	AutoSaveInterval        int     `json:"auto_save_interval"`
	TTSEnabled              bool    `json:"tts_enabled"`
	TTSVoice                string  `json:"tts_voice"`
	TTSModel                string  `json:"tts_model"`
	TTSSpeed                float64 `json:"tts_speed"`
	OpenAIAPIKeySet         bool    `json:"openai_api_key_set"`
	AnthropicAPIKeySet      bool    `json:"anthropic_api_key_set"`
	GeminiAPIKeySet         bool    `json:"gemini_api_key_set"`
	TelegramBotTokenSet     bool    `json:"telegram_bot_token_set"`
	TelegramChatID          int64   `json:"telegram_chat_id"`
	Configured              bool    `json:"configured"`
	ConfigPath              string  `json:"config_path"`
	DataPath                string  `json:"data_path"`
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
	app := &App{store: store, config: config}
	app.rebuildProvider()
	return app, nil
}

func (a *App) rebuildProvider() {
	eff := *a.config
	if eff.RunModel != "" {
		eff.Model = eff.RunModel
	}
	a.config = &eff
	a.prov = providers.New(&eff)
	if a.session != nil {
		a.session.Config = &eff
		a.oracle = engine.NewOracle(a.session, a.prov)
	}
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
	cfg, err := a.store.LoadConfig()
	if err != nil || cfg == nil {
		cfg = domain.DefaultConfig()
	}
	if strings.TrimSpace(in.Provider) != "" {
		cfg.Provider = domain.ProviderType(strings.ToLower(strings.TrimSpace(in.Provider)))
	}
	if strings.TrimSpace(in.Model) != "" {
		cfg.Model = strings.TrimSpace(in.Model)
	}
	cfg.EditModel = strings.TrimSpace(in.EditModel)
	cfg.RunModel = strings.TrimSpace(in.RunModel)
	if strings.TrimSpace(in.Language) != "" {
		cfg.Language = domain.Language(strings.TrimSpace(in.Language))
	}
	cfg.ImportLanguage = strings.TrimSpace(in.ImportLanguage)
	if in.Temperature != 0 {
		cfg.Temperature = in.Temperature
	}
	if in.MaxTokens > 0 {
		cfg.MaxTokens = in.MaxTokens
	}
	if in.ImportMaxOutputTokens > 0 {
		cfg.ImportMaxOutputTokens = in.ImportMaxOutputTokens
	}
	if in.OracleMaxToolIterations > 0 {
		cfg.OracleMaxToolIterations = in.OracleMaxToolIterations
	}
	if in.RequestTimeoutSeconds > 0 {
		cfg.RequestTimeoutSeconds = in.RequestTimeoutSeconds
	}
	cfg.AutoSave = in.AutoSave
	if in.AutoSaveInterval > 0 {
		cfg.AutoSaveInterval = in.AutoSaveInterval
	}
	cfg.TTS.Enabled = in.TTSEnabled
	if strings.TrimSpace(in.TTSVoice) != "" {
		cfg.TTS.Voice = domain.TTSVoice(strings.TrimSpace(in.TTSVoice))
	}
	if strings.TrimSpace(in.TTSModel) != "" {
		cfg.TTS.Model = strings.TrimSpace(in.TTSModel)
	}
	if in.TTSSpeed > 0 {
		cfg.TTS.Speed = in.TTSSpeed
	}
	cfg.TelegramChatID = in.TelegramChatID
	if err := a.store.SaveConfig(cfg); err != nil {
		return ConfigPayload{}, err
	}
	a.config = cfg
	a.rebuildProvider()
	return a.configPayload(), nil
}

func (a *App) configPayload() ConfigPayload {
	return ConfigPayload{
		Provider:                string(a.config.Provider),
		Model:                   a.config.Model,
		EditModel:               a.config.EditModel,
		RunModel:                a.config.RunModel,
		Language:                string(a.config.Language),
		ImportLanguage:          a.config.ImportLanguage,
		Temperature:             a.config.Temperature,
		MaxTokens:               a.config.MaxTokens,
		ImportMaxOutputTokens:   a.config.ImportMaxOutputTokens,
		OracleMaxToolIterations: a.config.OracleMaxToolIterations,
		RequestTimeoutSeconds:   a.config.RequestTimeoutSeconds,
		AutoSave:                a.config.AutoSave,
		AutoSaveInterval:        a.config.AutoSaveInterval,
		TTSEnabled:              a.config.TTS.Enabled,
		TTSVoice:                string(a.config.TTS.Voice),
		TTSModel:                a.config.TTS.Model,
		TTSSpeed:                a.config.TTS.Speed,
		OpenAIAPIKeySet:         strings.TrimSpace(a.config.OpenAIAPIKey) != "",
		AnthropicAPIKeySet:      strings.TrimSpace(a.config.AnthropicAPIKey) != "",
		GeminiAPIKeySet:         strings.TrimSpace(a.config.GeminiAPIKey) != "",
		TelegramBotTokenSet:     strings.TrimSpace(a.config.TelegramToken) != "",
		TelegramChatID:          a.config.TelegramChatID,
		Configured:              a.prov != nil,
		ConfigPath:              a.store.ConfigPath(),
		DataPath:                a.store.BasePath(),
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
	a.oracle = engine.NewOracle(a.session, a.prov)
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
	a.oracle = engine.NewOracle(a.session, a.prov)
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
	d := detailForUID(a.session.Adventure, a.session.State, uid)
	a.resolveDetailImages(d)
	return d, nil
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

func (a *App) ExportDMBook(path string, pdf bool) (*SubmitResult, error) {
	if a.session == nil {
		return nil, fmt.Errorf("no active session")
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("export path is required")
	}
	md := dmbook.Markdown(a.session.Adventure)
	if pdf {
		b, err := bookpdf.FromMarkdown(a.session.Adventure.Title, "Dungeon Master's Sourcebook", md)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, b, 0644); err != nil {
			return nil, err
		}
	} else if err := os.WriteFile(path, []byte(md), 0644); err != nil {
		return nil, err
	}
	p, _ := a.payload()
	return &SubmitResult{Success: true, Message: "DM book exported: " + path, Session: p}, nil
}

func (a *App) ExportNovel(path string, pdf bool) (*SubmitResult, error) {
	if a.session == nil {
		return nil, fmt.Errorf("no active session")
	}
	if a.prov == nil {
		return nil, fmt.Errorf("no AI provider configured")
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("export path is required")
	}
	timeout := time.Duration(a.config.RequestTimeoutSeconds) * time.Second
	if timeout < 15*time.Minute {
		timeout = 15 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	md, err := novel.Generate(ctx, a.prov, a.config.Model, a.session.Adventure, a.session.State)
	if err != nil {
		return nil, err
	}
	if pdf {
		b, err := bookpdf.FromMarkdown(a.session.Adventure.Title, "A novelization of the play session", md)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, b, 0644); err != nil {
			return nil, err
		}
	} else if err := os.WriteFile(path, []byte(md), 0644); err != nil {
		return nil, err
	}
	p, _ := a.payload()
	return &SubmitResult{Success: true, Message: "Novel exported: " + path, Session: p}, nil
}

func (a *App) PackageAdventure(adventureID, path string) (*SubmitResult, error) {
	adventureID = strings.TrimSpace(adventureID)
	path = strings.TrimSpace(path)
	if adventureID == "" || path == "" {
		return nil, fmt.Errorf("adventure id and output path are required")
	}
	if err := storage.PackageModule(a.store.AdventureDir(adventureID), path); err != nil {
		return nil, err
	}
	p, _ := a.payload()
	return &SubmitResult{Success: true, Message: "Adventure package saved: " + path, Session: p}, nil
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
	if result.NeedsUI {
		switch result.UIAction {
		case "oracle":
			if a.oracle == nil {
				a.oracle = engine.NewOracle(a.session, a.prov)
			}
			timeout := time.Duration(a.config.RequestTimeoutSeconds) * time.Second
			if timeout <= 0 {
				timeout = 90 * time.Second
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			resp := a.oracle.Ask(ctx, result.UIArg)
			if resp.Error != nil {
				return &SubmitResult{Success: false, Message: resp.Error.Error()}, nil
			}
			msg = resp.Answer
		case "save":
			return a.SaveSession()
		case "load":
			return &SubmitResult{Success: true, Message: "Use the library session list to load a saved session."}, nil
		case "import":
			return &SubmitResult{Success: true, Message: "Use Library → Import .tar.gz to import a module."}, nil
		case "image":
			msg = "Image selected: " + result.UIArg
		case "mode":
			msg = result.Message
		}
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
	a.resolveDetailImages(p.Detail)
	return p, nil
}

func (a *App) resolveDetailImages(d *DetailPayload) {
	if d == nil || a.session == nil || len(d.Images) == 0 {
		return
	}
	out := make([]string, 0, len(d.Images))
	for _, rel := range d.Images {
		abs, err := a.store.ResolveImagePath(a.session.Adventure.ID, rel)
		if err != nil {
			out = append(out, rel)
			continue
		}
		out = append(out, "file://"+filepath.ToSlash(abs))
	}
	d.Images = out
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

package wailsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/aibuild"
	"github.com/theburrowhub/thaimaturgy/internal/auth"
	"github.com/theburrowhub/thaimaturgy/internal/bookpdf"
	"github.com/theburrowhub/thaimaturgy/internal/dmbook"
	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/engine"
	"github.com/theburrowhub/thaimaturgy/internal/novel"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
	"github.com/theburrowhub/thaimaturgy/internal/tgbot"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Go backend exposed to the Wails frontend.
type App struct {
	ctx      context.Context
	store    *storage.Storage
	config   *domain.Config
	prov     providers.Provider
	oracle   *engine.Oracle
	tg       *tgbot.Bot
	tgCancel context.CancelFunc
	session  *domain.Session
	journal  *storage.SessionJournal

	// authStatus is the human-readable credential message from auth.AutoConfigure
	// (e.g. "Using Claude Code via local login"). Empty when nothing was detected.
	authStatus string
}

// autosave persists the current session only when auto-save is enabled, matching
// the desktop app's autosave semantics (a manual SaveSession always writes).
func (a *App) autosave() {
	if a.session != nil && a.config.AutoSave {
		_ = a.store.SaveSession(a.session.State)
	}
}

// openJournal starts an append-only chronicle for the current session and streams
// every timeline entry to it (mirrors the desktop app's per-session journal).
func (a *App) openJournal() {
	a.closeJournal()
	if a.session == nil {
		return
	}
	j, err := a.store.OpenSessionJournal(a.session.State.Name)
	if err != nil || j == nil {
		return
	}
	a.journal = j
	a.session.State.SetLogHook(func(e domain.LogEntry) { j.Append(e) })
}

func (a *App) closeJournal() {
	if a.journal != nil {
		_ = a.journal.Close()
		a.journal = nil
	}
}

// CloseSession tears down session-scoped resources (journal + Telegram host) when
// the frontend returns to the library.
func (a *App) CloseSession() {
	a.closeJournal()
	a.stopTelegramHost()
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
	// AuthStatus describes how the active credential was obtained (e.g. a local
	// Claude Code login). Output-only; empty when nothing was auto-detected.
	AuthStatus string `json:"auth_status"`

	// Input-only secrets. The frontend sends these when the user edits them; they
	// are never echoed back (configPayload leaves them blank and reports presence
	// via the *_set booleans instead).
	OpenAIAPIKey    string `json:"openai_api_key,omitempty"`
	AnthropicAPIKey string `json:"anthropic_api_key,omitempty"`
	GeminiAPIKey    string `json:"gemini_api_key,omitempty"`
	TelegramToken   string `json:"telegram_bot_token,omitempty"`
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
	Image   string          `json:"image,omitempty"` // data: URI for /map and /art
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
	app.authStatus = auth.AutoConfigure(config)
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
	// Seed secrets from the running config first: API keys are session-only and
	// never written to disk, so reloading from disk above would otherwise drop
	// them on every save. The Telegram token IS persisted, but seeding it too is
	// harmless and keeps the merge uniform.
	if a.config != nil {
		cfg.OpenAIAPIKey = a.config.OpenAIAPIKey
		cfg.AnthropicAPIKey = a.config.AnthropicAPIKey
		cfg.GeminiAPIKey = a.config.GeminiAPIKey
		if strings.TrimSpace(cfg.TelegramToken) == "" {
			cfg.TelegramToken = a.config.TelegramToken
		}
	}
	// Then only overwrite when a value was supplied so a blank field never wipes a
	// previously-set credential.
	if k := strings.TrimSpace(in.OpenAIAPIKey); k != "" {
		cfg.OpenAIAPIKey = k
	}
	if k := strings.TrimSpace(in.AnthropicAPIKey); k != "" {
		cfg.AnthropicAPIKey = k
	}
	if k := strings.TrimSpace(in.GeminiAPIKey); k != "" {
		cfg.GeminiAPIKey = k
	}
	if t := strings.TrimSpace(in.TelegramToken); t != "" {
		cfg.TelegramToken = t
	}
	if err := a.store.SaveConfig(cfg); err != nil {
		return ConfigPayload{}, err
	}
	a.config = cfg
	// Re-detect local logins (Claude Code / Gemini CLI) for the chosen provider,
	// matching the desktop app's Settings save behaviour.
	a.authStatus = auth.AutoConfigure(cfg)
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
		AuthStatus:              a.authStatus,
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
	if errs := domain.ValidateAdventure(adv, a.imageExistsFor(adv.ID)); len(errs) > 0 {
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
	a.stopTelegramHost()
	a.session = domain.NewSession(state, adv, a.config)
	a.oracle = engine.NewOracle(a.session, a.prov)
	a.openJournal()
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
	a.stopTelegramHost()
	a.session = domain.NewSession(state, adv, a.config)
	a.oracle = engine.NewOracle(a.session, a.prov)
	a.openJournal()
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
	a.autosave()
	return a.payloadWithDetail("room:" + zone.ID + "::" + room.ID)
}

func (a *App) MarkNPCMet(id string) (*SessionPayload, error) {
	if a.session == nil {
		return nil, fmt.Errorf("no active session")
	}
	if n := a.session.Adventure.NPC(strings.TrimSpace(id)); n != nil {
		a.session.State.MeetNPC(n.ID, n.Name)
		a.session.MarkModified()
		a.autosave()
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
		a.autosave()
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
	a.autosave()
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

// ExportAdventureDMBook exports a DM sourcebook for an installed adventure
// without needing an active play session — mirrors the desktop editor's "DM
// book…" toolbar button (the session-scoped ExportDMBook requires a session).
func (a *App) ExportAdventureDMBook(adventureID, path string, pdf bool) (*SubmitResult, error) {
	adventureID = strings.TrimSpace(adventureID)
	path = strings.TrimSpace(path)
	if adventureID == "" || path == "" {
		return nil, fmt.Errorf("adventure id and export path are required")
	}
	adv, err := a.store.LoadAdventure(adventureID)
	if err != nil {
		return nil, err
	}
	md := dmbook.Markdown(adv)
	if pdf {
		b, err := bookpdf.FromMarkdown(adv.Title, "Dungeon Master's Sourcebook", md)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, b, 0644); err != nil {
			return nil, err
		}
	} else if err := os.WriteFile(path, []byte(md), 0644); err != nil {
		return nil, err
	}
	return &SubmitResult{Success: true, Message: "DM book exported: " + path}, nil
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

func (a *App) AddImageAsset(adventureID, sourcePath, kind string) (string, error) {
	adventureID = strings.TrimSpace(adventureID)
	sourcePath = strings.TrimSpace(sourcePath)
	kind = safeAssetSegment(kind, "images")
	if adventureID == "" || sourcePath == "" {
		return "", fmt.Errorf("adventure id and image path are required")
	}
	in, err := os.Open(sourcePath)
	if err != nil {
		return "", err
	}
	defer in.Close()
	name := filepath.Base(sourcePath)
	if name == "." || name == string(filepath.Separator) || strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("invalid image filename")
	}
	rel := filepath.ToSlash(filepath.Join("assets", kind, name))
	dst := filepath.Join(a.store.AdventureDir(adventureID), filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return "", err
	}
	out, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return "", err
	}
	if err := out.Close(); err != nil {
		return "", err
	}
	return rel, nil
}

func (a *App) ToggleMode() (*SessionPayload, error) {
	if a.session == nil {
		return nil, fmt.Errorf("no active session")
	}
	a.session.State.ToggleMode()
	if a.session.State.EffectiveMode() == domain.ModeVirtualDM {
		a.session.State.EnsureParty()
		if a.session.State.StartGame() && a.oracle != nil {
			a.session.State.AddNote("Virtual DM mode started.")
		}
	}
	a.session.MarkModified()
	a.autosave()
	return a.payload()
}

func (a *App) SaveParty(party []*domain.Character) (*SessionPayload, error) {
	if a.session == nil {
		return nil, fmt.Errorf("no active session")
	}
	a.session.State.Characters = party
	a.session.State.EnsureParty()
	a.session.MarkModified()
	if err := a.store.SaveSession(a.session.State); err != nil {
		return nil, err
	}
	return a.payload()
}

func (a *App) StartTelegram() (*SubmitResult, error) {
	if a.session == nil {
		return nil, fmt.Errorf("no active session")
	}
	if a.tg != nil {
		p, _ := a.payload()
		return &SubmitResult{Success: true, Message: "Telegram already hosting.", Session: p}, nil
	}
	if strings.TrimSpace(a.config.TelegramToken) == "" {
		return nil, fmt.Errorf("no Telegram bot token configured")
	}
	if a.prov == nil {
		return nil, fmt.Errorf("no AI provider configured")
	}
	if a.oracle == nil {
		a.oracle = engine.NewOracle(a.session, a.prov)
	}
	a.session.State.Mode = domain.ModeVirtualDM
	a.session.State.EnsureParty()
	bot, err := tgbot.New(a.store, a.session, a.oracle, tgbot.Options{
		Token:  a.config.TelegramToken,
		ChatID: a.config.TelegramChatID,
		OnEvent: func(line string) {
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "telegram:event", line)
			}
		},
	})
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.tg, a.tgCancel = bot, cancel
	go bot.Run(ctx)
	p, _ := a.payload()
	return &SubmitResult{Success: true, Message: "Hosting on Telegram as @" + bot.Username(), Session: p}, nil
}

func (a *App) StopTelegram() (*SubmitResult, error) {
	a.stopTelegramHost()
	p, _ := a.payload()
	return &SubmitResult{Success: true, Message: "Telegram bot stopped.", Session: p}, nil
}

// stopTelegramHost tears down any running Telegram bot. Safe to call when none is
// hosting. Invoked when leaving DM mode, starting/loading a different session, or
// returning to the library so a bot never keeps driving a stale session.
func (a *App) stopTelegramHost() {
	if a.tgCancel != nil {
		a.tgCancel()
	}
	if a.tg != nil {
		a.tg.Stop()
	}
	a.tg, a.tgCancel = nil, nil
}

func (a *App) ChooseModuleFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "Import adventure module", Filters: []runtime.FileFilter{{DisplayName: "Adventure module", Pattern: "*.tar.gz;*.tgz;*.gz"}}})
}

func (a *App) ChoosePDFFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "Choose a PDF", Filters: []runtime.FileFilter{{DisplayName: "PDF", Pattern: "*.pdf"}}})
}

func (a *App) ChooseImagesFolder() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "Choose a folder of images"})
}

func (a *App) ChooseImageFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "Choose image", Filters: []runtime.FileFilter{{DisplayName: "Images", Pattern: "*.png;*.jpg;*.jpeg;*.webp;*.gif"}}})
}

func (a *App) ChooseSaveFile(defaultName string) (string, error) {
	return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{Title: "Save file", DefaultFilename: defaultName, CanCreateDirectories: true})
}

func (a *App) ImportAdventureFromPDF(path string, translate bool) (*domain.Adventure, error) {
	if a.prov == nil {
		return nil, fmt.Errorf("AI import needs a configured provider")
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("PDF path is required")
	}
	dir, err := os.MkdirTemp("", "thaim-wails-pdf-*")
	if err != nil {
		return nil, err
	}
	title := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()
	return aibuild.FromPDF(ctx, a.prov, a.importConfig(translate), path, dir, title, a.importProgress(), a.confirmFallback(), a.visionProvider())
}

func (a *App) ImportAdventureFromImages(path string, translate bool) (*domain.Adventure, error) {
	if a.prov == nil {
		return nil, fmt.Errorf("AI import needs a configured provider")
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("images folder is required")
	}
	dir, err := os.MkdirTemp("", "thaim-wails-images-*")
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()
	return aibuild.FromImages(ctx, a.prov, a.importConfig(translate), path, dir, filepath.Base(path), a.importProgress(), a.confirmFallback(), a.visionProvider())
}

// importProgress streams AI-import stage messages to the frontend as the Wails
// event "import:progress" (mirrors the desktop editor's status bar).
func (a *App) importProgress() aibuild.Progress {
	return func(stage string) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "import:progress", stage)
		}
	}
}

// visionProvider returns a separate Anthropic vision provider for image curation
// when the primary backend is text-only (e.g. the Claude CLI). Returns nil when
// the primary provider already supports vision. Mirrors the desktop editor.
func (a *App) visionProvider() providers.Provider {
	if a.prov != nil && a.prov.SupportsVision() {
		return nil
	}
	vcfg := *a.config
	vcfg.Provider = domain.ProviderAnthropic
	vcfg.AnthropicOAuthToken = ""
	vcfg.AnthropicAPIKey = ""
	auth.AutoConfigure(&vcfg)
	return providers.New(&vcfg)
}

// importConfig builds the config for an AI import. When translate is false the
// module is authored in the source document's own language (ImportLanguage is
// cleared); when true it is authored in the configured import language (falling
// back to the UI language). Mirrors the desktop editor's Translate toggle.
func (a *App) importConfig(translate bool) *domain.Config {
	cfg := *a.config
	if !translate {
		cfg.ImportLanguage = ""
	} else if strings.TrimSpace(cfg.ImportLanguage) == "" {
		cfg.ImportLanguage = string(cfg.Language)
	}
	return &cfg
}

// confirmFallback asks the user (via a native dialog) before an AI import
// silently substitutes an unavailable model. Mirrors the desktop editor's
// fallback confirmation; defaults to proceeding if no window context is present.
func (a *App) confirmFallback() aibuild.ConfirmFallback {
	return func(requested, served string) bool {
		if a.ctx == nil {
			return true
		}
		sel, err := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:          runtime.QuestionDialog,
			Title:         "Model unavailable",
			Message:       fmt.Sprintf("The requested model %q is unavailable.\n\nContinue with %q instead?", requested, served),
			Buttons:       []string{"Continue", "Cancel"},
			DefaultButton: "Continue",
			CancelButton:  "Cancel",
		})
		if err != nil {
			return true
		}
		return sel == "Continue"
	}
}

func safeAssetSegment(s, fallback string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, s)
	s = strings.Trim(s, "-")
	if s == "" || s == "." || s == ".." {
		return fallback
	}
	return s
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
	imageURL := ""
	if result.NeedsUI {
		switch result.UIAction {
		case "oracle":
			if a.prov == nil {
				return &SubmitResult{Success: false, Message: "No AI provider configured. Set an API key or provider in Settings to use the oracle."}, nil
			}
			if a.oracle == nil {
				a.oracle = engine.NewOracle(a.session, a.prov)
			}
			timeout := time.Duration(a.config.RequestTimeoutSeconds) * time.Second
			if timeout <= 0 {
				timeout = 90 * time.Second
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			if a.journal != nil {
				a.journal.Note("oracle-q", result.UIArg)
			}
			resp := a.oracle.Ask(ctx, result.UIArg)
			if resp.Error != nil {
				return &SubmitResult{Success: false, Message: resp.Error.Error()}, nil
			}
			msg = resp.Answer
			if a.journal != nil {
				a.journal.Note("oracle-a", resp.Answer)
			}
		case "save":
			return a.SaveSession()
		case "load":
			return &SubmitResult{Success: true, Message: "Use the library session list to load a saved session."}, nil
		case "import":
			return &SubmitResult{Success: true, Message: "Use Library → Import .tar.gz to import a module."}, nil
		case "image":
			if url := a.assetDataURL(a.session.Adventure.ID, result.UIArg); url != "" {
				imageURL = url
				msg = "🖼 " + result.UIArg
			} else {
				msg = "Image not found: " + result.UIArg
			}
		case "mode":
			msg = result.Message
		}
	}
	a.session.MarkModified()
	a.autosave()
	p, _ := a.payload()
	return &SubmitResult{Success: result.Success, Message: msg, Image: imageURL, Session: p}, nil
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
		if url := a.assetDataURL(a.session.Adventure.ID, rel); url != "" {
			out = append(out, url)
		}
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

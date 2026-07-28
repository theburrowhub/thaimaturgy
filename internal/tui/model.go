package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/theburrowhub/thaimaturgy/internal/auth"
	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/engine"
	"github.com/theburrowhub/thaimaturgy/internal/platform"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
	"github.com/theburrowhub/thaimaturgy/internal/tts"
)

// Screen identifies the active top-level view.
type Screen int

const (
	ScreenBoot Screen = iota
	ScreenConfig
	ScreenLibrary
	ScreenImport
	ScreenSession
	ScreenHelp
)

// ConfigStep drives the first-run API key wizard (reused from v1).
type ConfigStep int

const (
	ConfigStepLanguage ConfigStep = iota
	ConfigStepProvider
	ConfigStepAPIKey
	ConfigStepConfirm
)

// FocusPanel identifies which session panel has focus.
type FocusPanel int

const (
	FocusOracle FocusPanel = iota
	FocusLocation
	FocusLog
	FocusInput
)

// libEntryKind classifies a selectable library row.
type libEntryKind int

const (
	libImport libEntryKind = iota
	libAdventure
	libSession
	libSettings
	libHelp
	libQuit
)

type libEntry struct {
	kind  libEntryKind
	id    string
	label string
}

// Model is the root Bubble Tea model.
type Model struct {
	screen         Screen
	previousScreen Screen

	width       int
	height      int
	compactMode bool

	styles    *Styles
	storage   *storage.Storage
	config    *domain.Config
	provider  providers.Provider
	ttsClient *tts.Client

	// Library
	libEntries []libEntry
	libCursor  int

	// Import
	importInput textinput.Model

	// Session
	session       *domain.Session
	oracle        *engine.Oracle
	cmdHandler    *engine.CommandHandler
	input         textinput.Model
	oracleVP      viewport.Model
	locationVP    viewport.Model
	logVP         viewport.Model
	oracleContent string
	focusPanel    FocusPanel

	statusMsg string
	errorMsg  string
	loading   bool

	// Boot
	bootFrame int
	bootDone  bool

	// Config wizard
	configStep     ConfigStep
	configLanguage int
	configProvider int
	apiKeyInput    textinput.Model
	envFileCreated bool
}

var translations = map[string]map[domain.Language]string{
	"configTitle":        {domain.LangEnglish: "API KEY CONFIGURATION", domain.LangSpanish: "CONFIGURACIÓN DE API KEY"},
	"configNoKey":        {domain.LangEnglish: "No API key detected. Select your AI provider:", domain.LangSpanish: "No se detectó API key. Selecciona tu proveedor de IA:"},
	"configEnterKey":     {domain.LangEnglish: "Enter your %s API key:", domain.LangSpanish: "Ingresa tu API key de %s:"},
	"configKeyTemp":      {domain.LangEnglish: "Your key will be stored temporarily and deleted when you exit.", domain.LangSpanish: "Tu clave se almacenará temporalmente y se borrará al salir."},
	"configSuccess":      {domain.LangEnglish: "API key configured successfully!", domain.LangSpanish: "¡API key configurada exitosamente!"},
	"configErrorSaveKey": {domain.LangEnglish: "Failed to save API key: ", domain.LangSpanish: "Error al guardar API key: "},
	"configHintArrows":   {domain.LangEnglish: "Use arrows to select, ENTER to continue, ESC to skip", domain.LangSpanish: "Usa flechas para seleccionar, ENTER para continuar, ESC para omitir"},
	"configHintEnterEsc": {domain.LangEnglish: "Press ENTER to confirm, ESC to go back", domain.LangSpanish: "Presiona ENTER para confirmar, ESC para volver"},
	"configHintEnter":    {domain.LangEnglish: "Press ENTER to continue", domain.LangSpanish: "Presiona ENTER para continuar"},
	"provider":           {domain.LangEnglish: "Provider", domain.LangSpanish: "Proveedor"},
	"model":              {domain.LangEnglish: "Model", domain.LangSpanish: "Modelo"},

	"libraryTitle": {domain.LangEnglish: "ADVENTURE LIBRARY", domain.LangSpanish: "BIBLIOTECA DE AVENTURAS"},
	"libImport":    {domain.LangEnglish: "Import module (.tar.gz)…", domain.LangSpanish: "Importar módulo (.tar.gz)…"},
	"libSettings":  {domain.LangEnglish: "Settings", domain.LangSpanish: "Configuración"},
	"libHelp":      {domain.LangEnglish: "Help", domain.LangSpanish: "Ayuda"},
	"libQuit":      {domain.LangEnglish: "Quit", domain.LangSpanish: "Salir"},
	"libAdvHeader": {domain.LangEnglish: "ADVENTURES", domain.LangSpanish: "AVENTURAS"},
	"libSesHeader": {domain.LangEnglish: "SESSIONS (resume)", domain.LangSpanish: "SESIONES (reanudar)"},
	"libHint":      {domain.LangEnglish: "↑/↓ navigate · ENTER select · ESC quit", domain.LangSpanish: "↑/↓ navegar · ENTER seleccionar · ESC salir"},
	"libEmpty":     {domain.LangEnglish: "No adventures imported yet. Import a module to begin.", domain.LangSpanish: "Aún no hay aventuras. Importa un módulo para empezar."},

	"importTitle":  {domain.LangEnglish: "IMPORT ADVENTURE MODULE", domain.LangSpanish: "IMPORTAR MÓDULO DE AVENTURA"},
	"importPrompt": {domain.LangEnglish: "Path to a .tar.gz module:", domain.LangSpanish: "Ruta a un módulo .tar.gz:"},
	"importHint":   {domain.LangEnglish: "ENTER to import · ESC to cancel", domain.LangSpanish: "ENTER para importar · ESC para cancelar"},

	"panelLocation":    {domain.LangEnglish: "LOCATION", domain.LangSpanish: "LOCALIZACIÓN"},
	"panelOracle":      {domain.LangEnglish: "ORACLE", domain.LangSpanish: "ORÁCULO"},
	"panelLog":         {domain.LangEnglish: "SESSION LOG", domain.LangSpanish: "REGISTRO"},
	"inputPlaceholder": {domain.LangEnglish: "Ask the oracle, or type a /command…", domain.LangSpanish: "Pregunta al oráculo, o escribe un /comando…"},
	"thinking":         {domain.LangEnglish: "Consulting the oracle…", domain.LangSpanish: "Consultando al oráculo…"},
	"sessionSaved":     {domain.LangEnglish: "Session saved", domain.LangSpanish: "Sesión guardada"},
	"noProvider":       {domain.LangEnglish: "No AI provider configured.", domain.LangSpanish: "No hay proveedor de IA configurado."},

	"helpReturn":   {domain.LangEnglish: "Press ENTER or ESC to return", domain.LangSpanish: "Presiona ENTER o ESC para volver"},
	"bootSubtitle": {domain.LangEnglish: "An AI oracle for the Dungeon Master", domain.LangSpanish: "Un oráculo de IA para el Dungeon Master"},

	"ttsEnabled":  {domain.LangEnglish: "Voice narration ON", domain.LangSpanish: "Narración por voz ON"},
	"ttsDisabled": {domain.LangEnglish: "Voice narration OFF", domain.LangSpanish: "Narración por voz OFF"},
	"ttsNoKey":    {domain.LangEnglish: "TTS requires OpenAI API key", domain.LangSpanish: "TTS requiere API key de OpenAI"},
}

func (m *Model) t(key string) string {
	if trans, ok := translations[key]; ok {
		if text, ok := trans[m.config.Language]; ok {
			return text
		}
		return trans[domain.LangEnglish]
	}
	return key
}

// NewModel constructs the root model.
func NewModel(store *storage.Storage, config *domain.Config) *Model {
	input := textinput.New()
	input.Placeholder = "Ask the oracle, or type a /command…"
	input.CharLimit = 1000
	input.Width = 60

	importInput := textinput.New()
	importInput.Placeholder = "/path/to/adventure.tar.gz"
	importInput.CharLimit = 500
	importInput.Width = 60

	apiKeyInput := textinput.New()
	apiKeyInput.Placeholder = "sk-..."
	apiKeyInput.CharLimit = 200
	apiKeyInput.Width = 50
	apiKeyInput.EchoMode = textinput.EchoPassword
	apiKeyInput.EchoCharacter = '*'

	m := &Model{
		screen:      ScreenBoot,
		styles:      NewStyles(),
		storage:     store,
		config:      config,
		input:       input,
		importInput: importInput,
		apiKeyInput: apiKeyInput,
		oracleVP:    viewport.New(60, 15),
		locationVP:  viewport.New(30, 15),
		logVP:       viewport.New(30, 15),
		focusPanel:  FocusInput,
	}
	m.statusMsg = auth.AutoConfigure(m.config)
	m.initProvider()
	return m
}

func (m *Model) initProvider() {
	m.provider = providers.New(m.config)
	m.initTTS()
}

func (m *Model) initTTS() {
	apiKey := m.config.OpenAIAPIKey
	if apiKey == "" {
		return
	}
	if m.ttsClient != nil {
		m.ttsClient.Close()
	}
	if m.config.TTS.Voice == "" {
		m.config.TTS.Voice = domain.TTSVoiceOnyx
	}
	if m.config.TTS.Model == "" {
		m.config.TTS.Model = "tts-1"
	}
	if m.config.TTS.Speed == 0 {
		m.config.TTS.Speed = 1.0
	}
	client, err := tts.NewClient(apiKey, &m.config.TTS)
	if err != nil {
		return
	}
	m.ttsClient = client
}

// Cleanup releases resources and removes a temporary .env file.
func (m *Model) Cleanup() error {
	if m.ttsClient != nil {
		m.ttsClient.Close()
	}
	if m.envFileCreated {
		return m.storage.DeleteEnvFile()
	}
	return nil
}

// Storage exposes the storage layer.
func (m *Model) Storage() *storage.Storage { return m.storage }

// EnvFileCreated reports whether a temporary .env was written this run.
func (m *Model) EnvFileCreated() bool { return m.envFileCreated }

// Init starts the boot animation.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(tea.SetWindowTitle("thAImaturgy"), tickCmd())
}

type tickMsg time.Time
type oracleResponseMsg struct{ resp *engine.Response }
type saveDoneMsg struct{ err error }
type importDoneMsg struct {
	adv *domain.Adventure
	err error
}
type loadSessionMsg struct {
	state *domain.SessionState
	adv   *domain.Adventure
	err   error
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// Update is the Bubble Tea update loop.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		switch m.screen {
		case ScreenBoot:
			if msg.Type == tea.KeyEnter || msg.Type == tea.KeySpace {
				m.leaveBoot()
			}
		case ScreenConfig:
			cmds = append(cmds, m.updateConfig(msg))
		case ScreenLibrary:
			cmds = append(cmds, m.updateLibrary(msg))
		case ScreenImport:
			cmds = append(cmds, m.updateImport(msg))
		case ScreenSession:
			cmds = append(cmds, m.updateSession(msg))
		case ScreenHelp:
			if msg.Type == tea.KeyEsc || msg.Type == tea.KeyEnter {
				m.screen = m.previousScreen
				if m.screen == ScreenSession {
					m.input.Focus()
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.compactMode = m.width < 100

	case tickMsg:
		if m.screen == ScreenBoot && !m.bootDone {
			m.bootFrame++
			if m.bootFrame > 30 {
				m.leaveBoot()
			}
			cmds = append(cmds, tickCmd())
		}

	case oracleResponseMsg:
		m.loading = false
		if msg.resp.Error != nil {
			m.errorMsg = msg.resp.Error.Error()
		} else {
			m.appendOracle(m.styles.Narration.Render(msg.resp.Answer))
			m.statusMsg = fmt.Sprintf("Tokens: %d | %dms", msg.resp.TokensUsed, msg.resp.LatencyMs)
			m.refreshLocationPanel()
			m.refreshLogPanel()
			if m.ttsClient != nil && m.ttsClient.IsEnabled() && msg.resp.Answer != "" {
				go func(text string) { _ = m.ttsClient.Speak(context.Background(), text) }(msg.resp.Answer)
			}
			if m.config.AutoSave && m.session != nil {
				go func() { _ = m.storage.SaveSession(m.session.State) }()
			}
		}

	case saveDoneMsg:
		m.loading = false
		if msg.err != nil {
			m.errorMsg = msg.err.Error()
		} else {
			m.statusMsg = m.t("sessionSaved")
		}

	case importDoneMsg:
		m.loading = false
		if msg.err != nil {
			m.errorMsg = msg.err.Error()
		} else {
			m.statusMsg = "Imported: " + msg.adv.Title
			m.screen = ScreenLibrary
			m.rebuildLibrary()
		}

	case loadSessionMsg:
		m.loading = false
		if msg.err != nil {
			m.errorMsg = msg.err.Error()
		} else {
			m.enterSession(msg.state, msg.adv)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) leaveBoot() {
	m.bootDone = true
	if !m.config.IsConfigured() {
		m.screen = ScreenConfig
	} else {
		m.screen = ScreenLibrary
		m.rebuildLibrary()
	}
}

// --- Library -------------------------------------------------------------

func (m *Model) rebuildLibrary() {
	advs, _ := m.storage.ListAdventures()
	sessions, _ := m.storage.ListSessions()

	entries := []libEntry{{kind: libImport, label: m.t("libImport")}}
	for _, a := range advs {
		entries = append(entries, libEntry{kind: libAdventure, id: a.ID, label: "▶ " + a.Title})
	}
	for _, s := range sessions {
		label := fmt.Sprintf("↻ %s — %s", s.Name, s.AdventureTitle)
		entries = append(entries, libEntry{kind: libSession, id: s.Name, label: label})
	}
	entries = append(entries,
		libEntry{kind: libSettings, label: m.t("libSettings")},
		libEntry{kind: libHelp, label: m.t("libHelp")},
		libEntry{kind: libQuit, label: m.t("libQuit")},
	)
	m.libEntries = entries
	if m.libCursor >= len(entries) {
		m.libCursor = 0
	}
}

func (m *Model) updateLibrary(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyUp, tea.KeyShiftTab:
		m.libCursor = (m.libCursor - 1 + len(m.libEntries)) % len(m.libEntries)
	case tea.KeyDown, tea.KeyTab:
		m.libCursor = (m.libCursor + 1) % len(m.libEntries)
	case tea.KeyEsc:
		return tea.Quit
	case tea.KeyEnter:
		return m.selectLibraryEntry()
	}
	return nil
}

func (m *Model) selectLibraryEntry() tea.Cmd {
	if len(m.libEntries) == 0 {
		return nil
	}
	e := m.libEntries[m.libCursor]
	switch e.kind {
	case libImport:
		m.screen = ScreenImport
		m.importInput.SetValue("")
		m.importInput.Focus()
	case libAdventure:
		return m.startSession(e.id)
	case libSession:
		return m.loadSession(e.id)
	case libSettings:
		m.screen = ScreenConfig
		m.configStep = ConfigStepLanguage
	case libHelp:
		m.previousScreen = ScreenLibrary
		m.screen = ScreenHelp
	case libQuit:
		return tea.Quit
	}
	return nil
}

// startSession creates a fresh session for an imported adventure.
func (m *Model) startSession(advID string) tea.Cmd {
	return func() tea.Msg {
		adv, err := m.storage.LoadAdventure(advID)
		if err != nil {
			return loadSessionMsg{err: err}
		}
		name := uniqueSessionName(m.storage, adv.ID)
		state := domain.NewSessionState(name, adv)
		return loadSessionMsg{state: state, adv: adv}
	}
}

func uniqueSessionName(store *storage.Storage, base string) string {
	name := base
	for i := 1; store.SessionExists(name); i++ {
		name = fmt.Sprintf("%s-%d", base, i)
	}
	return name
}

// loadSession resumes a saved session, reloading its adventure module.
func (m *Model) loadSession(name string) tea.Cmd {
	return func() tea.Msg {
		state, err := m.storage.LoadSession(name)
		if err != nil {
			return loadSessionMsg{err: err}
		}
		adv, err := m.storage.LoadAdventure(state.AdventureID)
		if err != nil {
			return loadSessionMsg{err: fmt.Errorf("adventure %q not found; import it first", state.AdventureID)}
		}
		return loadSessionMsg{state: state, adv: adv}
	}
}

func (m *Model) enterSession(state *domain.SessionState, adv *domain.Adventure) {
	m.session = domain.NewSession(state, adv, m.config)
	m.oracle = engine.NewOracle(m.session, m.provider)
	m.cmdHandler = engine.NewCommandHandler(m.session)
	m.screen = ScreenSession
	m.focusPanel = FocusInput
	m.input.Placeholder = m.t("inputPlaceholder")
	m.input.Focus()
	m.oracleContent = m.styles.Hint.Render(fmt.Sprintf("Running \"%s\". Type a question or /help.", adv.Title))
	m.oracleVP.SetContent(m.oracleContent)
	m.refreshLocationPanel()
	m.refreshLogPanel()
}

// --- Import --------------------------------------------------------------

func (m *Model) updateImport(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEnter:
		path := strings.TrimSpace(m.importInput.Value())
		if path == "" {
			return nil
		}
		return m.doImport(path)
	case tea.KeyEsc:
		m.screen = ScreenLibrary
		m.importInput.Blur()
	default:
		var cmd tea.Cmd
		m.importInput, cmd = m.importInput.Update(msg)
		return cmd
	}
	return nil
}

func (m *Model) doImport(path string) tea.Cmd {
	m.loading = true
	m.statusMsg = "Importing…"
	return func() tea.Msg {
		adv, err := m.storage.ImportModule(expandPath(path))
		return importDoneMsg{adv: adv, err: err}
	}
}

func expandPath(p string) string {
	p = strings.TrimSpace(p)
	if strings.HasPrefix(p, "~/") {
		if home, err := homeDir(); err == nil {
			return home + p[1:]
		}
	}
	return p
}

// --- Session -------------------------------------------------------------

func (m *Model) updateSession(msg tea.KeyMsg) tea.Cmd {
	if m.loading {
		return nil
	}
	switch msg.Type {
	case tea.KeyCtrlS:
		return m.doSave()
	case tea.KeyCtrlH:
		m.previousScreen = ScreenSession
		m.screen = ScreenHelp
		return nil
	case tea.KeyCtrlQ:
		return tea.Quit
	case tea.KeyCtrlN:
		m.toggleTTS()
		return nil
	case tea.KeyEsc:
		m.screen = ScreenLibrary
		m.rebuildLibrary()
		return nil
	case tea.KeyTab:
		m.focusPanel = (m.focusPanel + 1) % 4
		return nil
	case tea.KeyCtrlUp:
		m.oracleVP.LineUp(3)
		return nil
	case tea.KeyCtrlDown:
		m.oracleVP.LineDown(3)
		return nil
	case tea.KeyPgUp:
		m.oracleVP.ViewUp()
		return nil
	case tea.KeyPgDown:
		m.oracleVP.ViewDown()
		return nil
	case tea.KeyEnter:
		if m.focusPanel == FocusInput && strings.TrimSpace(m.input.Value()) != "" {
			return m.handleSessionInput(m.input.Value())
		}
	case tea.KeyUp, tea.KeyDown:
		if m.focusPanel != FocusInput {
			if msg.Type == tea.KeyUp {
				m.oracleVP.LineUp(1)
			} else {
				m.oracleVP.LineDown(1)
			}
			return nil
		}
		fallthrough
	default:
		if m.focusPanel == FocusInput {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return cmd
		}
	}
	return nil
}

func (m *Model) handleSessionInput(raw string) tea.Cmd {
	m.input.SetValue("")
	cmd := engine.ParseCommand(raw)
	if cmd == nil {
		return nil
	}
	result := m.cmdHandler.Execute(cmd)

	if result.ShouldQuit {
		return tea.Quit
	}
	if result.Response != "" {
		m.appendOracle(m.styles.InputPrompt.Render("» ") + raw + "\n" + result.Response)
	}
	if result.Message != "" && !result.NeedsUI {
		m.statusMsg = result.Message
	}
	if !result.Success && result.Message != "" {
		m.errorMsg = result.Message
	}

	if result.NeedsUI {
		switch result.UIAction {
		case "oracle":
			m.appendOracle(m.styles.InputPrompt.Render("» ") + raw)
			return m.askOracle(result.UIArg)
		case "save":
			return m.doSave()
		case "load":
			if result.UIArg != "" {
				return m.loadSession(result.UIArg)
			}
			m.screen = ScreenLibrary
			m.rebuildLibrary()
		case "import":
			if result.UIArg != "" {
				return m.doImport(result.UIArg)
			}
			m.screen = ScreenImport
			m.importInput.SetValue("")
			m.importInput.Focus()
		case "image":
			m.openImage(result.UIArg)
		}
	}

	m.refreshLocationPanel()
	m.refreshLogPanel()
	return nil
}

func (m *Model) openImage(relPath string) {
	if m.session == nil {
		return
	}
	abs, err := m.storage.ResolveImagePath(m.session.Adventure.ID, relPath)
	if err != nil {
		m.errorMsg = err.Error()
		return
	}
	if err := platform.OpenPath(abs); err != nil {
		m.errorMsg = "Could not open image: " + err.Error()
		return
	}
	m.statusMsg = "Opened " + relPath
}

func (m *Model) askOracle(input string) tea.Cmd {
	if m.oracle == nil || m.provider == nil {
		m.errorMsg = m.t("noProvider")
		return nil
	}
	m.loading = true
	m.statusMsg = m.t("thinking")
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		return oracleResponseMsg{resp: m.oracle.Ask(ctx, input)}
	}
}

func (m *Model) doSave() tea.Cmd {
	if m.session == nil {
		return nil
	}
	m.loading = true
	return func() tea.Msg {
		return saveDoneMsg{err: m.storage.SaveSession(m.session.State)}
	}
}

func (m *Model) toggleTTS() {
	if m.config.OpenAIAPIKey == "" {
		m.statusMsg = m.t("ttsNoKey")
		return
	}
	if m.ttsClient == nil {
		m.initTTS()
	}
	if m.ttsClient == nil {
		m.statusMsg = "TTS: failed to initialize"
		return
	}
	if m.ttsClient.Toggle() {
		m.statusMsg = m.t("ttsEnabled")
	} else {
		m.statusMsg = m.t("ttsDisabled")
	}
}

func (m *Model) appendOracle(text string) {
	if m.oracleContent != "" {
		m.oracleContent += "\n\n"
	}
	m.oracleContent += text
	width := m.oracleVP.Width
	if width < 20 {
		width = 60
	}
	m.oracleVP.SetContent(lipgloss.NewStyle().Width(width).Render(m.oracleContent))
	m.oracleVP.GotoBottom()
}

func (m *Model) refreshLocationPanel() {
	if m.session == nil {
		return
	}
	adv := m.session.Adventure
	st := m.session.State
	var sb strings.Builder

	room, zone := adv.Room(st.CurrentRoom)
	if zone != nil {
		sb.WriteString(m.styles.StatLabel.Render("ZONE") + "\n")
		sb.WriteString(zone.Name + "\n")
		if zone.MapImage != "" {
			sb.WriteString(m.styles.Hint.Render("  /map — view map") + "\n")
		}
		sb.WriteString("\n")
	}
	if room != nil {
		sb.WriteString(m.styles.StatLabel.Render("ROOM") + "\n")
		sb.WriteString(m.styles.StatValue.Render(room.Name) + "\n")
		if room.Image != "" {
			sb.WriteString(m.styles.Hint.Render("  /art "+room.ID) + "\n")
		}
		if len(room.Exits) > 0 {
			sb.WriteString("\n" + m.styles.StatLabel.Render("EXITS") + "\n")
			for _, ex := range room.Exits {
				line := "  → " + ex.To
				if ex.Direction != "" {
					line = "  " + ex.Direction + " → " + ex.To
				}
				sb.WriteString(line + "\n")
			}
		}
		if len(room.NPCIDs) > 0 {
			sb.WriteString("\n" + m.styles.StatLabel.Render("NPCs HERE") + "\n")
			for _, nid := range room.NPCIDs {
				if n := adv.NPC(nid); n != nil {
					sb.WriteString("  " + n.Name + m.styles.Hint.Render(" ["+n.ID+"]") + "\n")
				}
			}
		}
	} else {
		sb.WriteString(m.styles.Hint.Render("No current room.\nUse /goto <room_id>."))
	}
	m.locationVP.SetContent(sb.String())
}

func (m *Model) refreshLogPanel() {
	if m.session == nil {
		return
	}
	var sb strings.Builder
	for _, e := range m.session.State.Log.GetLast(60) {
		ts := e.Timestamp.Format("15:04")
		style := m.styles.EventLog
		switch e.Type {
		case domain.LogRoll:
			style = m.styles.StatValue
		case domain.LogNPC, domain.LogEvent:
			style = m.styles.Quest
		case domain.LogNote:
			style = m.styles.Item
		}
		sb.WriteString(style.Render(fmt.Sprintf("[%s] %s", ts, e.Message)) + "\n")
	}
	m.logVP.SetContent(sb.String())
	m.logVP.GotoBottom()
}

// --- Config wizard (reused) ---------------------------------------------

func (m *Model) updateConfig(msg tea.KeyMsg) tea.Cmd {
	switch m.configStep {
	case ConfigStepLanguage:
		switch msg.Type {
		case tea.KeyUp, tea.KeyDown:
			m.configLanguage = (m.configLanguage + 1) % 2
		case tea.KeyEnter:
			if m.configLanguage == 0 {
				m.config.Language = domain.LangEnglish
			} else {
				m.config.Language = domain.LangSpanish
			}
			m.configStep = ConfigStepProvider
		case tea.KeyEsc:
			m.screen = ScreenLibrary
			m.rebuildLibrary()
		}
	case ConfigStepProvider:
		switch msg.Type {
		case tea.KeyUp:
			m.configProvider = (m.configProvider + 2) % 3
		case tea.KeyDown:
			m.configProvider = (m.configProvider + 1) % 3
		case tea.KeyEnter:
			m.configStep = ConfigStepAPIKey
			m.apiKeyInput.Focus()
			m.apiKeyInput.SetValue("")
			switch m.configProvider {
			case 0:
				m.apiKeyInput.Placeholder = "sk-... (OpenAI API Key)"
			case 1:
				m.apiKeyInput.Placeholder = "sk-ant-... (Anthropic API Key)"
			default:
				m.apiKeyInput.Placeholder = "AIza... (Gemini API Key)"
			}
		case tea.KeyEsc:
			m.configStep = ConfigStepLanguage
		}
	case ConfigStepAPIKey:
		switch msg.Type {
		case tea.KeyEnter:
			apiKey := m.apiKeyInput.Value()
			if apiKey == "" {
				return nil
			}
			var provider domain.ProviderType
			switch m.configProvider {
			case 0:
				provider = domain.ProviderOpenAI
				m.config.Provider = domain.ProviderOpenAI
				m.config.OpenAIAPIKey = apiKey
			case 1:
				provider = domain.ProviderAnthropic
				m.config.Provider = domain.ProviderAnthropic
				m.config.AnthropicAPIKey = apiKey
			default:
				provider = domain.ProviderGemini
				m.config.Provider = domain.ProviderGemini
				m.config.GeminiAPIKey = apiKey
			}
			m.config.Model = domain.DefaultModel(provider)
			if err := m.storage.SaveAPIKey(provider, apiKey); err != nil {
				m.errorMsg = m.t("configErrorSaveKey") + err.Error()
				return nil
			}
			m.envFileCreated = true
			m.initProvider()
			m.configStep = ConfigStepConfirm
		case tea.KeyEsc:
			m.configStep = ConfigStepProvider
			m.apiKeyInput.Blur()
		default:
			var cmd tea.Cmd
			m.apiKeyInput, cmd = m.apiKeyInput.Update(msg)
			return cmd
		}
	case ConfigStepConfirm:
		if msg.Type == tea.KeyEnter {
			m.screen = ScreenLibrary
			m.configStep = ConfigStepLanguage
			m.rebuildLibrary()
		}
	}
	return nil
}

func homeDir() (string, error) { return os.UserHomeDir() }

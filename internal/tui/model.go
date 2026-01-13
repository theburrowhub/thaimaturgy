package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/engine"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
	"github.com/theburrowhub/thaimaturgy/internal/tts"
)

type Screen int

const (
	ScreenBoot Screen = iota
	ScreenConfig
	ScreenMenu
	ScreenWizard
	ScreenGame
	ScreenSaves
	ScreenHelp
)

type WizardStep int

const (
	WizardStepName WizardStep = iota
	WizardStepRace
	WizardStepClass
	WizardStepStats
	WizardStepConfirm
)

type ConfigStep int

const (
	ConfigStepLanguage ConfigStep = iota
	ConfigStepProvider
	ConfigStepAPIKey
	ConfigStepConfirm
)

type FocusPanel int

const (
	FocusNarration FocusPanel = iota
	FocusCharacter
	FocusEventLog
	FocusInput
)

type Model struct {
	screen         Screen
	previousScreen Screen // to return to after help/settings
	wizardStep     WizardStep
	focusPanel     FocusPanel

	width        int
	height       int
	compactMode  bool

	styles       *Styles
	storage      *storage.Storage
	config       *domain.Config
	session      *domain.GameSession
	orchestrator *engine.Orchestrator
	cmdHandler   *engine.CommandHandler

	input        textinput.Model
	narration    viewport.Model
	eventLog     viewport.Model
	charSheet    viewport.Model

	narrationContent string
	eventLogContent  string

	bootFrame    int
	bootDone     bool

	wizardName   string
	wizardRace   int
	wizardClass  int
	wizardStats  [6]int
	wizardCursor int

	saves        []storage.SaveInfo
	saveCursor   int

	menuCursor   int

	statusMsg    string
	errorMsg     string
	loading      bool

	provider     providers.Provider
	ttsClient    *tts.Client

	configStep     ConfigStep
	configLanguage int
	configProvider int
	apiKeyInput    textinput.Model
	envFileCreated bool
}

var racesEN = []string{"Human", "Elf", "Dwarf", "Halfling", "Half-Orc", "Tiefling", "Dragonborn", "Gnome"}
var racesES = []string{"Humano", "Elfo", "Enano", "Mediano", "Semiorco", "Tiefling", "Dracónido", "Gnomo"}
var classesEN = []string{"Fighter", "Wizard", "Rogue", "Cleric", "Ranger", "Paladin", "Barbarian", "Bard"}
var classesES = []string{"Guerrero", "Mago", "Pícaro", "Clérigo", "Explorador", "Paladín", "Bárbaro", "Bardo"}

var translations = map[string]map[domain.Language]string{
	// Config screen
	"configTitle":           {domain.LangEnglish: "API KEY CONFIGURATION", domain.LangSpanish: "CONFIGURACIÓN DE API KEY"},
	"configLangTitle":       {domain.LangEnglish: "LANGUAGE / IDIOMA", domain.LangSpanish: "LANGUAGE / IDIOMA"},
	"configLangPrompt":      {domain.LangEnglish: "Select your language:", domain.LangSpanish: "Selecciona tu idioma:"},
	"configNoKey":           {domain.LangEnglish: "No API key detected. Select your AI provider:", domain.LangSpanish: "No se detectó API key. Selecciona tu proveedor de IA:"},
	"configEnterKey":        {domain.LangEnglish: "Enter your %s API key:", domain.LangSpanish: "Ingresa tu API key de %s:"},
	"configKeyTemp":         {domain.LangEnglish: "Your key will be stored temporarily and deleted when you exit.", domain.LangSpanish: "Tu clave se almacenará temporalmente y se borrará al salir."},
	"configSuccess":         {domain.LangEnglish: "API key configured successfully!", domain.LangSpanish: "¡API key configurada exitosamente!"},
	"configErrorSaveKey":    {domain.LangEnglish: "Failed to save API key: ", domain.LangSpanish: "Error al guardar API key: "},
	"configHintArrows":      {domain.LangEnglish: "Use arrows to select, ENTER to continue, ESC to skip", domain.LangSpanish: "Usa flechas para seleccionar, ENTER para continuar, ESC para omitir"},
	"configHintEnterEsc":    {domain.LangEnglish: "Press ENTER to confirm, ESC to go back", domain.LangSpanish: "Presiona ENTER para confirmar, ESC para volver"},
	"configHintEnter":       {domain.LangEnglish: "Press ENTER to continue", domain.LangSpanish: "Presiona ENTER para continuar"},
	"provider":              {domain.LangEnglish: "Provider", domain.LangSpanish: "Proveedor"},
	"model":                 {domain.LangEnglish: "Model", domain.LangSpanish: "Modelo"},

	// Menu
	"menuNewCampaign": {domain.LangEnglish: "New Campaign", domain.LangSpanish: "Nueva Campaña"},
	"menuLoadGame":    {domain.LangEnglish: "Load Game", domain.LangSpanish: "Cargar Partida"},
	"menuSettings":    {domain.LangEnglish: "Settings", domain.LangSpanish: "Configuración"},
	"menuHelp":        {domain.LangEnglish: "Help", domain.LangSpanish: "Ayuda"},
	"menuQuit":        {domain.LangEnglish: "Quit", domain.LangSpanish: "Salir"},
	"menuHint":        {domain.LangEnglish: "Use arrows to navigate, ENTER to select", domain.LangSpanish: "Usa flechas para navegar, ENTER para seleccionar"},
	"menuNoKey":       {domain.LangEnglish: "Warning: No API key configured", domain.LangSpanish: "Advertencia: No hay API key configurada"},

	// Wizard
	"wizardTitle":       {domain.LangEnglish: "CREATE YOUR CHARACTER", domain.LangSpanish: "CREA TU PERSONAJE"},
	"wizardName":        {domain.LangEnglish: "What is your character's name?", domain.LangSpanish: "¿Cuál es el nombre de tu personaje?"},
	"wizardRace":        {domain.LangEnglish: "Choose your race:", domain.LangSpanish: "Elige tu raza:"},
	"wizardClass":       {domain.LangEnglish: "Choose your class:", domain.LangSpanish: "Elige tu clase:"},
	"wizardStats":       {domain.LangEnglish: "Your ability scores (press R to reroll):", domain.LangSpanish: "Tus puntuaciones de habilidad (presiona R para retirar):"},
	"wizardConfirm":     {domain.LangEnglish: "Confirm your character:", domain.LangSpanish: "Confirma tu personaje:"},
	"wizardConfirmHint": {domain.LangEnglish: "Press Y to begin, N to start over", domain.LangSpanish: "Presiona Y para comenzar, N para reiniciar"},
	"wizardBack":        {domain.LangEnglish: "ESC to go back", domain.LangSpanish: "ESC para volver"},
	"name":              {domain.LangEnglish: "Name", domain.LangSpanish: "Nombre"},
	"race":              {domain.LangEnglish: "Race", domain.LangSpanish: "Raza"},
	"class":             {domain.LangEnglish: "Class", domain.LangSpanish: "Clase"},
	"stats":             {domain.LangEnglish: "Stats", domain.LangSpanish: "Estadísticas"},

	// Game
	"inputPlaceholder":  {domain.LangEnglish: "Enter command or action...", domain.LangSpanish: "Ingresa comando o acción..."},
	"thinking":          {domain.LangEnglish: "Thinking...", domain.LangSpanish: "Pensando..."},
	"gameSaved":         {domain.LangEnglish: "Game saved successfully", domain.LangSpanish: "Partida guardada exitosamente"},
	"gameLoaded":        {domain.LangEnglish: "Game loaded", domain.LangSpanish: "Partida cargada"},
	"failedSave":        {domain.LangEnglish: "Failed to save: ", domain.LangSpanish: "Error al guardar: "},
	"failedLoad":        {domain.LangEnglish: "Failed to load: ", domain.LangSpanish: "Error al cargar: "},
	"noProvider":        {domain.LangEnglish: "No AI provider configured. Set API key in environment.", domain.LangSpanish: "No hay proveedor de IA configurado. Configura API key."},
	"beginAdventure":    {domain.LangEnglish: "Begin my adventure!", domain.LangSpanish: "¡Comienza mi aventura!"},

	// Panels
	"panelCharacter": {domain.LangEnglish: "CHARACTER", domain.LangSpanish: "PERSONAJE"},
	"panelNarration": {domain.LangEnglish: "NARRATION", domain.LangSpanish: "NARRACIÓN"},
	"panelEventLog":  {domain.LangEnglish: "EVENT LOG", domain.LangSpanish: "REGISTRO"},

	// Character sheet
	"abilities":  {domain.LangEnglish: "ABILITIES", domain.LangSpanish: "HABILIDADES"},
	"conditions": {domain.LangEnglish: "CONDITIONS", domain.LangSpanish: "CONDICIONES"},
	"inventory":  {domain.LangEnglish: "INVENTORY", domain.LangSpanish: "INVENTARIO"},

	// Saves
	"savesTitle":  {domain.LangEnglish: "LOAD GAME", domain.LangSpanish: "CARGAR PARTIDA"},
	"savesEmpty":  {domain.LangEnglish: "No saved games found.", domain.LangSpanish: "No se encontraron partidas guardadas."},
	"savesHint":   {domain.LangEnglish: "ENTER to load, ESC to cancel", domain.LangSpanish: "ENTER para cargar, ESC para cancelar"},

	// Help
	"helpTitle":      {domain.LangEnglish: "HELP", domain.LangSpanish: "AYUDA"},
	"helpNavigation": {domain.LangEnglish: "NAVIGATION", domain.LangSpanish: "NAVEGACIÓN"},
	"helpCommands":   {domain.LangEnglish: "COMMANDS", domain.LangSpanish: "COMANDOS"},
	"helpGameplay":   {domain.LangEnglish: "GAMEPLAY", domain.LangSpanish: "JUGABILIDAD"},
	"helpGameplayText": {domain.LangEnglish: "Type any text to interact with the DM.\nThe AI will describe scenes and present options.\nUse dice rolls for uncertain actions.",
		domain.LangSpanish: "Escribe cualquier texto para interactuar con el DM.\nLa IA describirá escenas y presentará opciones.\nUsa tiradas de dados para acciones inciertas."},
	"helpReturn": {domain.LangEnglish: "Press ENTER or ESC to return", domain.LangSpanish: "Presiona ENTER o ESC para volver"},

	// Boot
	"bootPress":    {domain.LangEnglish: "[ Press ENTER to continue ]", domain.LangSpanish: "[ Presiona ENTER para continuar ]"},
	"bootSubtitle": {domain.LangEnglish: "A retro AI-powered dungeon master", domain.LangSpanish: "Un dungeon master retro potenciado por IA"},

	// Status hints
	"hintPanels": {domain.LangEnglish: "TAB: switch panels | /help for commands | ESC: menu", domain.LangSpanish: "TAB: cambiar panel | /help para comandos | ESC: menú"},
	"hintDefault": {domain.LangEnglish: "/help for commands | TAB to switch panels | ESC for menu", domain.LangSpanish: "/help para comandos | TAB cambiar panel | ESC menú"},

	// Shortcuts (nano style)
	"shortcutSave":   {domain.LangEnglish: "^S Save", domain.LangSpanish: "^S Guardar"},
	"shortcutHelp":   {domain.LangEnglish: "^H Help", domain.LangSpanish: "^H Ayuda"},
	"shortcutRoll":   {domain.LangEnglish: "^R Roll", domain.LangSpanish: "^R Tirar"},
	"shortcutScroll": {domain.LangEnglish: "^↑↓ Scroll", domain.LangSpanish: "^↑↓ Scroll"},
	"shortcutVoice":  {domain.LangEnglish: "^N Voice", domain.LangSpanish: "^N Voz"},
	"shortcutQuit":   {domain.LangEnglish: "^Q Quit", domain.LangSpanish: "^Q Salir"},

	// TTS
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

func (m *Model) races() []string {
	if m.config.Language == domain.LangSpanish {
		return racesES
	}
	return racesEN
}

func (m *Model) classes() []string {
	if m.config.Language == domain.LangSpanish {
		return classesES
	}
	return classesEN
}

func NewModel(store *storage.Storage, config *domain.Config) *Model {
	ti := textinput.New()
	ti.Placeholder = "Enter command or action..."
	ti.CharLimit = 500
	ti.Width = 60

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
		input:       ti,
		apiKeyInput: apiKeyInput,
		narration:   viewport.New(60, 15),
		eventLog:    viewport.New(30, 10),
		charSheet:   viewport.New(30, 15),
		bootFrame:   0,
		focusPanel:  FocusInput,
	}

	m.initProvider()

	return m
}

func (m *Model) initProvider() {
	switch m.config.Provider {
	case domain.ProviderOpenAI:
		if m.config.OpenAIAPIKey != "" {
			m.provider = providers.NewOpenAIProvider(m.config.OpenAIAPIKey)
		}
	case domain.ProviderAnthropic:
		if m.config.AnthropicAPIKey != "" {
			m.provider = providers.NewAnthropicProvider(m.config.AnthropicAPIKey)
		}
	}

	// Initialize TTS client (requires OpenAI API key)
	m.initTTS()
}

func (m *Model) initTTS() {
	// TTS always uses OpenAI API
	apiKey := m.config.OpenAIAPIKey
	if apiKey == "" {
		return
	}

	// Close existing client if any
	if m.ttsClient != nil {
		m.ttsClient.Close()
	}

	// Ensure TTS config has default values if empty
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

func (m *Model) Cleanup() error {
	if m.ttsClient != nil {
		m.ttsClient.Close()
	}
	if m.envFileCreated {
		return m.storage.DeleteEnvFile()
	}
	return nil
}

func (m *Model) Storage() *storage.Storage {
	return m.storage
}

func (m *Model) EnvFileCreated() bool {
	return m.envFileCreated
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		tea.SetWindowTitle("thAImaturgy"),
		tickCmd(),
	)
}

type tickMsg time.Time
type aiResponseMsg struct {
	response *engine.OrchestratorResponse
}
type saveCompleteMsg struct {
	err error
}
type loadCompleteMsg struct {
	state *domain.GameState
	err   error
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

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
				m.bootDone = true
				if !m.config.IsConfigured() {
					m.screen = ScreenConfig
				} else {
					m.screen = ScreenMenu
				}
			}
		case ScreenConfig:
			cmds = append(cmds, m.updateConfig(msg))
		case ScreenMenu:
			cmds = append(cmds, m.updateMenu(msg))
		case ScreenWizard:
			cmds = append(cmds, m.updateWizard(msg))
		case ScreenGame:
			cmds = append(cmds, m.updateGame(msg))
		case ScreenSaves:
			cmds = append(cmds, m.updateSaves(msg))
		case ScreenHelp:
			if msg.Type == tea.KeyEsc || msg.Type == tea.KeyEnter {
				m.screen = m.previousScreen
				if m.screen == ScreenGame {
					m.input.Focus()
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.compactMode = m.width < 100

		m.updateViewportSizes()

	case tickMsg:
		if m.screen == ScreenBoot && !m.bootDone {
			m.bootFrame++
			if m.bootFrame > 30 {
				m.bootDone = true
				if !m.config.IsConfigured() {
					m.screen = ScreenConfig
				} else {
					m.screen = ScreenMenu
				}
			}
			cmds = append(cmds, tickCmd())
		}

	case aiResponseMsg:
		m.loading = false
		if msg.response.Error != nil {
			m.errorMsg = msg.response.Error.Error()
		} else {
			m.appendNarration("\n" + m.styles.Narration.Render(msg.response.Narrative))
			// Update event log from session state (includes tool call events like dice rolls)
			m.updateEventLogContent()
			m.statusMsg = fmt.Sprintf("Tokens: %d | Latency: %dms", msg.response.TokensUsed, msg.response.LatencyMs)

			// Narrate response with TTS if enabled
			if m.ttsClient != nil && m.ttsClient.IsEnabled() && msg.response.Narrative != "" {
				go func(text string) {
					if err := m.ttsClient.Speak(context.Background(), text); err != nil {
						m.errorMsg = "TTS: " + err.Error()
					}
				}(msg.response.Narrative)
			}

			// Auto-save after each DM response if enabled
			if m.config.AutoSave && m.session != nil {
				go func() {
					_ = m.storage.SaveGame(m.session.State)
				}()
			}
		}
		m.updateCharacterSheet()

	case saveCompleteMsg:
		m.loading = false
		if msg.err != nil {
			m.errorMsg = m.t("failedSave") + msg.err.Error()
		} else {
			m.statusMsg = m.t("gameSaved")
		}

	case loadCompleteMsg:
		m.loading = false
		if msg.err != nil {
			m.errorMsg = m.t("failedLoad") + msg.err.Error()
		} else {
			m.session = domain.NewGameSession(msg.state, m.config)
			m.cmdHandler = engine.NewCommandHandler(m.session)
			m.orchestrator = engine.NewOrchestrator(m.session, m.provider)
			m.screen = ScreenGame
			m.statusMsg = m.t("gameLoaded")
			m.updateCharacterSheet()
			m.updateEventLogContent()
			m.restoreNarrationFromConversation()
			m.input.Placeholder = m.t("inputPlaceholder")
			m.input.Focus()
			m.focusPanel = FocusInput
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) updateViewportSizes() {
	if m.compactMode {
		m.narration.Width = m.width - 4
		m.narration.Height = m.height - 10
		m.eventLog.Width = m.width - 4
		m.eventLog.Height = 5
		m.charSheet.Width = m.width - 4
		m.charSheet.Height = 10
	} else {
		leftWidth := int(float64(m.width) * 0.25)
		centerWidth := int(float64(m.width) * 0.50)
		rightWidth := m.width - leftWidth - centerWidth - 6

		m.charSheet.Width = leftWidth
		m.charSheet.Height = m.height - 8

		m.narration.Width = centerWidth
		m.narration.Height = m.height - 8

		m.eventLog.Width = rightWidth
		m.eventLog.Height = m.height - 8
	}

	m.input.Width = m.width - 6
}

func (m *Model) updateConfig(msg tea.KeyMsg) tea.Cmd {
	switch m.configStep {
	case ConfigStepLanguage:
		switch msg.Type {
		case tea.KeyUp:
			m.configLanguage--
			if m.configLanguage < 0 {
				m.configLanguage = 1
			}
		case tea.KeyDown:
			m.configLanguage++
			if m.configLanguage > 1 {
				m.configLanguage = 0
			}
		case tea.KeyEnter:
			if m.configLanguage == 0 {
				m.config.Language = domain.LangEnglish
			} else {
				m.config.Language = domain.LangSpanish
			}
			m.configStep = ConfigStepProvider
		case tea.KeyEsc:
			m.screen = ScreenMenu
		}

	case ConfigStepProvider:
		switch msg.Type {
		case tea.KeyUp:
			m.configProvider--
			if m.configProvider < 0 {
				m.configProvider = 1
			}
		case tea.KeyDown:
			m.configProvider++
			if m.configProvider > 1 {
				m.configProvider = 0
			}
		case tea.KeyEnter:
			m.configStep = ConfigStepAPIKey
			m.apiKeyInput.Focus()
			m.apiKeyInput.SetValue("")
			if m.configProvider == 0 {
				m.apiKeyInput.Placeholder = "sk-... (OpenAI API Key)"
			} else {
				m.apiKeyInput.Placeholder = "sk-ant-... (Anthropic API Key)"
			}
		case tea.KeyEsc:
			m.configStep = ConfigStepLanguage
		}

	case ConfigStepAPIKey:
		switch msg.Type {
		case tea.KeyEnter:
			apiKey := m.apiKeyInput.Value()
			if apiKey != "" {
				var provider domain.ProviderType
				if m.configProvider == 0 {
					provider = domain.ProviderOpenAI
					m.config.Provider = domain.ProviderOpenAI
					m.config.OpenAIAPIKey = apiKey
					m.config.Model = "gpt-4o-mini"
				} else {
					provider = domain.ProviderAnthropic
					m.config.Provider = domain.ProviderAnthropic
					m.config.AnthropicAPIKey = apiKey
					m.config.Model = "claude-sonnet-4-20250514"
				}

				if err := m.storage.SaveAPIKey(provider, apiKey); err != nil {
					m.errorMsg = m.t("configErrorSaveKey") + err.Error()
					return nil
				}

				m.envFileCreated = true
				m.initProvider()
				m.configStep = ConfigStepConfirm
			}
		case tea.KeyEsc:
			m.configStep = ConfigStepProvider
			m.apiKeyInput.Blur()
		default:
			var cmd tea.Cmd
			m.apiKeyInput, cmd = m.apiKeyInput.Update(msg)
			return cmd
		}

	case ConfigStepConfirm:
		switch msg.Type {
		case tea.KeyEnter:
			m.screen = ScreenMenu
			m.configStep = ConfigStepLanguage
		}
	}

	return nil
}

func (m *Model) updateMenu(msg tea.KeyMsg) tea.Cmd {
	const menuItemCount = 5
	switch msg.Type {
	case tea.KeyUp, tea.KeyShiftTab:
		m.menuCursor--
		if m.menuCursor < 0 {
			m.menuCursor = menuItemCount - 1
		}
	case tea.KeyDown, tea.KeyTab:
		m.menuCursor++
		if m.menuCursor >= menuItemCount {
			m.menuCursor = 0
		}
	case tea.KeyEnter:
		switch m.menuCursor {
		case 0: // New Campaign
			m.screen = ScreenWizard
			m.wizardStep = WizardStepName
			m.wizardStats = engine.RollFullAbilityScores()
			m.input.SetValue("")
			m.input.Placeholder = m.t("inputPlaceholder")
			m.input.Focus()
		case 1: // Load Game
			saves, _ := m.storage.ListSaves()
			m.saves = saves
			m.saveCursor = 0
			m.screen = ScreenSaves
		case 2: // Settings
			m.screen = ScreenConfig
		case 3: // Help
			m.previousScreen = ScreenMenu
			m.screen = ScreenHelp
		case 4: // Quit
			return tea.Quit
		}
	}
	return nil
}

func (m *Model) updateWizard(msg tea.KeyMsg) tea.Cmd {
	switch m.wizardStep {
	case WizardStepName:
		switch msg.Type {
		case tea.KeyEnter:
			if m.input.Value() != "" {
				m.wizardName = m.input.Value()
				m.input.SetValue("")
				m.wizardStep = WizardStepRace
			}
		case tea.KeyEsc:
			m.screen = ScreenMenu
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return cmd
		}

	case WizardStepRace:
		switch msg.Type {
		case tea.KeyUp:
			m.wizardRace--
			if m.wizardRace < 0 {
				m.wizardRace = len(racesEN) - 1
			}
		case tea.KeyDown:
			m.wizardRace++
			if m.wizardRace >= len(racesEN) {
				m.wizardRace = 0
			}
		case tea.KeyEnter:
			m.wizardStep = WizardStepClass
		case tea.KeyEsc:
			m.wizardStep = WizardStepName
		}

	case WizardStepClass:
		switch msg.Type {
		case tea.KeyUp:
			m.wizardClass--
			if m.wizardClass < 0 {
				m.wizardClass = len(classesEN) - 1
			}
		case tea.KeyDown:
			m.wizardClass++
			if m.wizardClass >= len(classesEN) {
				m.wizardClass = 0
			}
		case tea.KeyEnter:
			m.wizardStep = WizardStepStats
		case tea.KeyEsc:
			m.wizardStep = WizardStepRace
		}

	case WizardStepStats:
		switch msg.Type {
		case tea.KeyUp, tea.KeyLeft:
			m.wizardCursor--
			if m.wizardCursor < 0 {
				m.wizardCursor = 5
			}
		case tea.KeyDown, tea.KeyRight:
			m.wizardCursor++
			if m.wizardCursor > 5 {
				m.wizardCursor = 0
			}
		case tea.KeyEnter:
			m.wizardStep = WizardStepConfirm
		case tea.KeyEsc:
			m.wizardStep = WizardStepClass
		case tea.KeyRunes:
			if string(msg.Runes) == "r" || string(msg.Runes) == "R" {
				m.wizardStats = engine.RollFullAbilityScores()
			}
		}

	case WizardStepConfirm:
		switch msg.Type {
		case tea.KeyEnter:
			m.startNewGame()
			return m.sendToAI(m.t("beginAdventure"))
		case tea.KeyEsc:
			m.wizardStep = WizardStepStats
		case tea.KeyRunes:
			r := string(msg.Runes)
			if r == "y" || r == "Y" {
				m.startNewGame()
				return m.sendToAI(m.t("beginAdventure"))
			} else if r == "n" || r == "N" {
				m.wizardStep = WizardStepName
			}
		}
	}

	return nil
}

func (m *Model) updateGame(msg tea.KeyMsg) tea.Cmd {
	if m.loading {
		return nil
	}

	switch msg.Type {
	case tea.KeyCtrlS:
		return m.saveGame()
	case tea.KeyCtrlH:
		m.previousScreen = ScreenGame
		m.screen = ScreenHelp
		return nil
	case tea.KeyCtrlR:
		result := m.cmdHandler.Execute(&engine.Command{Type: engine.CmdRoll, Args: []string{"1d20"}})
		for _, event := range result.Events {
			m.appendEvent(event)
			m.session.State.EventLog.Add(event)
		}
		if result.Response != "" {
			m.appendNarration("\n" + m.styles.Hint.Render(result.Response))
		}
		return nil
	case tea.KeyCtrlT:
		result := m.cmdHandler.Execute(&engine.Command{Type: engine.CmdStatus})
		if result.Response != "" {
			m.appendNarration("\n" + m.styles.Hint.Render(result.Response))
		}
		return nil
	case tea.KeyCtrlQ:
		return tea.Quit
	case tea.KeyCtrlN:
		// Toggle TTS narration
		if m.config.OpenAIAPIKey == "" {
			m.statusMsg = m.t("ttsNoKey")
			return nil
		}
		if m.ttsClient == nil {
			m.initTTS()
		}
		if m.ttsClient != nil {
			enabled := m.ttsClient.Toggle()
			if enabled {
				m.statusMsg = m.t("ttsEnabled") + " (" + m.ttsClient.GetVoiceName() + ")"
			} else {
				m.statusMsg = m.t("ttsDisabled")
			}
		} else {
			m.statusMsg = "TTS: failed to initialize"
		}
		return nil
	case tea.KeyCtrlUp:
		// Scroll narration up (works from any panel)
		m.narration.LineUp(3)
		return nil
	case tea.KeyCtrlDown:
		// Scroll narration down (works from any panel)
		m.narration.LineDown(3)
		return nil
	case tea.KeyHome:
		// Go to top of narration
		m.narration.GotoTop()
		return nil
	case tea.KeyEnd:
		// Go to bottom of narration
		m.narration.GotoBottom()
		return nil
	case tea.KeyEsc:
		m.screen = ScreenMenu
		return nil
	case tea.KeyTab:
		m.focusPanel = (m.focusPanel + 1) % 4
	case tea.KeyEnter:
		if m.focusPanel == FocusInput && m.input.Value() != "" {
			input := m.input.Value()
			m.input.SetValue("")

			cmd := engine.ParseCommand(input)
			if cmd == nil {
				return nil
			}

			result := m.cmdHandler.Execute(cmd)

			for _, event := range result.Events {
				m.appendEvent(event)
				m.session.State.EventLog.Add(event)
			}

			if result.ShouldQuit {
				return tea.Quit
			}

			if result.Response != "" {
				m.appendNarration("\n" + m.styles.Hint.Render(result.Response))
			}

			if result.Message != "" && !result.NeedsUI {
				m.statusMsg = result.Message
			}

			if result.NeedsUI {
				switch result.UIAction {
				case "narration":
					m.appendNarration("\n" + m.styles.InputPrompt.Render("> ") + input)
					return m.sendToAI(result.Message)
				case "save":
					return m.saveGame()
				case "load":
					return m.loadGame(result.Message)
				case "new":
					m.screen = ScreenWizard
					m.wizardStep = WizardStepName
				}
			}

			m.updateCharacterSheet()
		}
	case tea.KeyPgUp:
		// Page scroll (works from any panel for narration)
		m.narration.ViewUp()
		return nil
	case tea.KeyPgDown:
		// Page scroll (works from any panel for narration)
		m.narration.ViewDown()
		return nil
	case tea.KeyUp, tea.KeyDown:
		// Arrow keys scroll when not in input panel
		if m.focusPanel != FocusInput {
			if msg.Type == tea.KeyUp {
				m.narration.LineUp(1)
			} else {
				m.narration.LineDown(1)
			}
			return nil
		}
		// Fall through to default for input handling
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

func (m *Model) updateSaves(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyUp:
		m.saveCursor--
		if m.saveCursor < 0 {
			m.saveCursor = len(m.saves) - 1
		}
	case tea.KeyDown:
		m.saveCursor++
		if m.saveCursor >= len(m.saves) {
			m.saveCursor = 0
		}
	case tea.KeyEnter:
		if len(m.saves) > 0 {
			return m.loadGame(m.saves[m.saveCursor].Name)
		}
	case tea.KeyEsc:
		m.screen = ScreenMenu
	}
	return nil
}

func (m *Model) startNewGame() {
	// Always use English for character data storage
	char := domain.NewCharacter(m.wizardName, racesEN[m.wizardRace], classesEN[m.wizardClass])
	char.Abilities.STR = m.wizardStats[0]
	char.Abilities.DEX = m.wizardStats[1]
	char.Abilities.CON = m.wizardStats[2]
	char.Abilities.INT = m.wizardStats[3]
	char.Abilities.WIS = m.wizardStats[4]
	char.Abilities.CHA = m.wizardStats[5]

	char.MaxHP = 10 + domain.Modifier(char.Abilities.CON)
	char.CurrentHP = char.MaxHP
	char.AC = 10 + domain.Modifier(char.Abilities.DEX)
	char.Initiative = domain.Modifier(char.Abilities.DEX)

	state := domain.NewGameState(m.wizardName, char, m.config.DefaultSetting)
	m.session = domain.NewGameSession(state, m.config)
	m.cmdHandler = engine.NewCommandHandler(m.session)
	m.orchestrator = engine.NewOrchestrator(m.session, m.provider)

	m.screen = ScreenGame
	m.narrationContent = ""
	m.eventLogContent = ""
	m.input.Placeholder = m.t("inputPlaceholder")
	m.input.Focus()
	m.focusPanel = FocusInput

	m.updateCharacterSheet()
}

func (m *Model) sendToAI(input string) tea.Cmd {
	if m.orchestrator == nil || m.provider == nil {
		m.errorMsg = m.t("noProvider")
		return nil
	}

	m.loading = true
	m.statusMsg = m.t("thinking")

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		response := m.orchestrator.ProcessInput(ctx, input)
		return aiResponseMsg{response: response}
	}
}

func (m *Model) saveGame() tea.Cmd {
	m.loading = true
	return func() tea.Msg {
		err := m.storage.SaveGame(m.session.State)
		return saveCompleteMsg{err: err}
	}
}

func (m *Model) loadGame(name string) tea.Cmd {
	m.loading = true
	return func() tea.Msg {
		state, err := m.storage.LoadGame(name)
		return loadCompleteMsg{state: state, err: err}
	}
}

func (m *Model) appendNarration(text string) {
	m.narrationContent += text + "\n"
	// Wrap content to viewport width to ensure proper scrolling
	width := m.narration.Width
	if width < 20 {
		width = 60 // fallback if viewport not sized yet
	}
	wrapped := lipgloss.NewStyle().Width(width).Render(m.narrationContent)
	m.narration.SetContent(wrapped)
	m.narration.GotoBottom()
}

func (m *Model) scrollNarrationToBottom() {
	m.narration.GotoBottom()
}

func (m *Model) appendEvent(event domain.Event) {
	timestamp := event.Timestamp.Format("15:04:05")
	line := fmt.Sprintf("[%s] %s", timestamp, event.Message)

	style := m.styles.EventLog
	switch event.Type {
	case domain.EventTypeError:
		style = m.styles.Error
	case domain.EventTypeDiceRoll:
		style = m.styles.StatValue
	case domain.EventTypeHPChange:
		if delta, ok := event.Data["delta"].(int); ok && delta < 0 {
			style = m.styles.HPCritical
		} else {
			style = m.styles.HPFull
		}
	}

	m.eventLogContent += style.Render(line) + "\n"
	m.eventLog.SetContent(m.eventLogContent)
	m.eventLog.GotoBottom()
}

func (m *Model) updateCharacterSheet() {
	if m.session == nil || m.session.State.Character == nil {
		return
	}

	c := m.session.State.Character
	var sb strings.Builder

	sb.WriteString(m.styles.StatValue.Render(c.Name) + "\n")
	sb.WriteString(fmt.Sprintf("Level %d %s %s\n\n", c.Level, c.Race, c.Class))

	hpPercent := float64(c.CurrentHP) / float64(c.MaxHP)
	hpStyle := m.styles.HPFull
	if hpPercent < 0.25 {
		hpStyle = m.styles.HPCritical
	} else if hpPercent < 0.5 {
		hpStyle = m.styles.HPLow
	}
	sb.WriteString(fmt.Sprintf("HP: %s\n", hpStyle.Render(fmt.Sprintf("%d/%d", c.CurrentHP, c.MaxHP))))
	sb.WriteString(fmt.Sprintf("AC: %d  Init: %+d  Spd: %d\n\n", c.AC, c.Initiative, c.Speed))

	sb.WriteString(m.styles.StatLabel.Render(m.t("abilities")+"\n"))
	sb.WriteString(fmt.Sprintf("STR: %2d %s  INT: %2d %s\n",
		c.Abilities.STR, m.styles.StatModifier.Render(domain.ModifierString(c.Abilities.STR)),
		c.Abilities.INT, m.styles.StatModifier.Render(domain.ModifierString(c.Abilities.INT))))
	sb.WriteString(fmt.Sprintf("DEX: %2d %s  WIS: %2d %s\n",
		c.Abilities.DEX, m.styles.StatModifier.Render(domain.ModifierString(c.Abilities.DEX)),
		c.Abilities.WIS, m.styles.StatModifier.Render(domain.ModifierString(c.Abilities.WIS))))
	sb.WriteString(fmt.Sprintf("CON: %2d %s  CHA: %2d %s\n\n",
		c.Abilities.CON, m.styles.StatModifier.Render(domain.ModifierString(c.Abilities.CON)),
		c.Abilities.CHA, m.styles.StatModifier.Render(domain.ModifierString(c.Abilities.CHA))))

	sb.WriteString(fmt.Sprintf("Gold: %d  XP: %d\n\n", c.Gold, c.XP))

	if len(c.Conditions) > 0 {
		sb.WriteString(m.styles.StatLabel.Render(m.t("conditions")+"\n"))
		for _, cond := range c.Conditions {
			sb.WriteString(m.styles.Condition.Render(string(cond)) + " ")
		}
		sb.WriteString("\n\n")
	}

	if len(c.Inventory) > 0 {
		sb.WriteString(m.styles.StatLabel.Render(m.t("inventory")+"\n"))
		for _, item := range c.Inventory {
			if item.Quantity > 1 {
				sb.WriteString(fmt.Sprintf("- %s (x%d)\n", item.Name, item.Quantity))
			} else {
				sb.WriteString(fmt.Sprintf("- %s\n", item.Name))
			}
		}
	}

	m.charSheet.SetContent(sb.String())
}

func (m *Model) updateEventLogContent() {
	if m.session == nil {
		return
	}

	m.eventLogContent = ""
	for _, event := range m.session.State.EventLog.Events {
		m.appendEvent(event)
	}
}

func (m *Model) restoreNarrationFromConversation() {
	if m.session == nil || m.session.State.Conversation == nil {
		return
	}

	m.narrationContent = ""
	for _, msg := range m.session.State.Conversation.Messages {
		switch msg.Role {
		case domain.RoleUser:
			m.narrationContent += m.styles.InputPrompt.Render("> ") + msg.Content + "\n\n"
		case domain.RoleAssistant:
			m.narrationContent += m.styles.Narration.Render(msg.Content) + "\n\n"
		}
	}
	// Wrap content to viewport width to ensure proper scrolling
	width := m.narration.Width
	if width < 20 {
		width = 60
	}
	wrapped := lipgloss.NewStyle().Width(width).Render(m.narrationContent)
	m.narration.SetContent(wrapped)
	m.narration.GotoBottom()
}

func (m *Model) View() string {
	switch m.screen {
	case ScreenBoot:
		return m.viewBoot()
	case ScreenConfig:
		return m.viewConfig()
	case ScreenMenu:
		return m.viewMenu()
	case ScreenWizard:
		return m.viewWizard()
	case ScreenGame:
		return m.viewGame()
	case ScreenSaves:
		return m.viewSaves()
	case ScreenHelp:
		return m.viewHelp()
	}
	return ""
}

func (m *Model) viewBoot() string {
	var sb strings.Builder

	sb.WriteString("\n\n")
	sb.WriteString(m.styles.BootLogo.Render(Logo))
	sb.WriteString("\n\n")

	if m.bootFrame%6 < 3 {
		sb.WriteString(m.styles.BootText.Render("    [ Press ENTER to continue ]"))
	}

	sb.WriteString("\n\n")
	sb.WriteString(m.styles.Hint.Render("    A retro AI-powered dungeon master"))
	sb.WriteString("\n")
	sb.WriteString(m.styles.Hint.Render("    v0.1.0"))

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, sb.String())
}

func (m *Model) viewConfig() string {
	var sb strings.Builder

	switch m.configStep {
	case ConfigStepLanguage:
		sb.WriteString(m.styles.WizardTitle.Render("LANGUAGE / IDIOMA") + "\n\n")
		sb.WriteString("Select your language / Selecciona tu idioma:\n\n")

		languages := []string{"English", "Español"}
		for i, lang := range languages {
			cursor := "  "
			style := m.styles.WizardOption
			if i == m.configLanguage {
				cursor = "> "
				style = m.styles.WizardSelected
			}
			sb.WriteString(cursor + style.Render(lang) + "\n")
		}

		sb.WriteString("\n")
		sb.WriteString(m.styles.Hint.Render("Use arrows, ENTER to continue"))

	case ConfigStepProvider:
		sb.WriteString(m.styles.WizardTitle.Render(m.t("configTitle")) + "\n\n")
		sb.WriteString(m.t("configNoKey") + "\n\n")

		providers := []string{"OpenAI (GPT-4o)", "Anthropic (Claude)"}
		for i, p := range providers {
			cursor := "  "
			style := m.styles.WizardOption
			if i == m.configProvider {
				cursor = "> "
				style = m.styles.WizardSelected
			}
			sb.WriteString(cursor + style.Render(p) + "\n")
		}

		sb.WriteString("\n")
		sb.WriteString(m.styles.Hint.Render(m.t("configHintArrows")))

	case ConfigStepAPIKey:
		sb.WriteString(m.styles.WizardTitle.Render(m.t("configTitle")) + "\n\n")
		providerName := "OpenAI"
		if m.configProvider == 1 {
			providerName = "Anthropic"
		}
		sb.WriteString(fmt.Sprintf(m.t("configEnterKey")+"\n\n", providerName))
		sb.WriteString(m.apiKeyInput.View())
		sb.WriteString("\n\n")
		sb.WriteString(m.styles.Hint.Render(m.t("configKeyTemp")))
		sb.WriteString("\n")
		sb.WriteString(m.styles.Hint.Render(m.t("configHintEnterEsc")))

	case ConfigStepConfirm:
		sb.WriteString(m.styles.WizardTitle.Render(m.t("configTitle")) + "\n\n")
		sb.WriteString(m.styles.Success.Render(m.t("configSuccess")) + "\n\n")
		providerName := "OpenAI"
		model := "gpt-4o-mini"
		if m.configProvider == 1 {
			providerName = "Anthropic"
			model = "claude-sonnet-4-20250514"
		}
		sb.WriteString(fmt.Sprintf("%s: %s\n", m.t("provider"), m.styles.StatValue.Render(providerName)))
		sb.WriteString(fmt.Sprintf("%s: %s\n\n", m.t("model"), m.styles.StatValue.Render(model)))
		sb.WriteString(m.styles.Hint.Render(m.t("configHintEnter")))
	}

	if m.errorMsg != "" {
		sb.WriteString("\n\n")
		sb.WriteString(m.styles.Error.Render(m.errorMsg))
	}

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, sb.String())
}

func (m *Model) viewMenu() string {
	var sb strings.Builder

	sb.WriteString(m.styles.BootLogo.Render(LogoSmall))
	sb.WriteString("\n\n")

	menuItems := []string{
		m.t("menuNewCampaign"),
		m.t("menuLoadGame"),
		m.t("menuSettings"),
		m.t("menuHelp"),
		m.t("menuQuit"),
	}

	for i, item := range menuItems {
		cursor := "  "
		style := m.styles.WizardOption
		if i == m.menuCursor {
			cursor = "> "
			style = m.styles.WizardSelected
		}
		sb.WriteString(cursor + style.Render(item) + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString(m.styles.Hint.Render(m.t("menuHint")))

	if !m.config.IsConfigured() {
		sb.WriteString("\n\n")
		sb.WriteString(m.styles.Error.Render(m.t("menuNoKey")))
	}

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, sb.String())
}

func (m *Model) viewWizard() string {
	var sb strings.Builder

	sb.WriteString(m.styles.WizardTitle.Render(m.t("wizardTitle")) + "\n\n")

	switch m.wizardStep {
	case WizardStepName:
		sb.WriteString(m.t("wizardName") + "\n\n")
		sb.WriteString(m.input.View())

	case WizardStepRace:
		sb.WriteString(m.t("wizardRace") + "\n\n")
		for i, race := range m.races() {
			cursor := "  "
			style := m.styles.WizardOption
			if i == m.wizardRace {
				cursor = "> "
				style = m.styles.WizardSelected
			}
			sb.WriteString(cursor + style.Render(race) + "\n")
		}

	case WizardStepClass:
		sb.WriteString(m.t("wizardClass") + "\n\n")
		for i, class := range m.classes() {
			cursor := "  "
			style := m.styles.WizardOption
			if i == m.wizardClass {
				cursor = "> "
				style = m.styles.WizardSelected
			}
			sb.WriteString(cursor + style.Render(class) + "\n")
		}

	case WizardStepStats:
		sb.WriteString(m.t("wizardStats") + "\n\n")
		abilities := []string{"STR", "DEX", "CON", "INT", "WIS", "CHA"}
		for i, ab := range abilities {
			cursor := "  "
			style := m.styles.WizardOption
			if i == m.wizardCursor {
				cursor = "> "
				style = m.styles.WizardSelected
			}
			mod := domain.Modifier(m.wizardStats[i])
			modStr := fmt.Sprintf("%+d", mod)
			sb.WriteString(fmt.Sprintf("%s%s: %s (%s)\n", cursor, style.Render(ab), style.Render(fmt.Sprintf("%2d", m.wizardStats[i])), modStr))
		}

	case WizardStepConfirm:
		sb.WriteString(m.t("wizardConfirm") + "\n\n")
		sb.WriteString(fmt.Sprintf("  %s:  %s\n", m.t("name"), m.styles.StatValue.Render(m.wizardName)))
		sb.WriteString(fmt.Sprintf("  %s:  %s\n", m.t("race"), m.styles.StatValue.Render(m.races()[m.wizardRace])))
		sb.WriteString(fmt.Sprintf("  %s: %s\n\n", m.t("class"), m.styles.StatValue.Render(m.classes()[m.wizardClass])))
		sb.WriteString("  " + m.t("stats") + ":\n")
		abilities := []string{"STR", "DEX", "CON", "INT", "WIS", "CHA"}
		for i, ab := range abilities {
			sb.WriteString(fmt.Sprintf("    %s: %d\n", ab, m.wizardStats[i]))
		}
		sb.WriteString("\n")
		sb.WriteString(m.styles.Hint.Render(m.t("wizardConfirmHint")))
	}

	sb.WriteString("\n\n")
	sb.WriteString(m.styles.Hint.Render(m.t("wizardBack")))

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, sb.String())
}

func (m *Model) viewGame() string {
	if m.compactMode {
		return m.viewGameCompact()
	}
	return m.viewGameFull()
}

func (m *Model) viewGameFull() string {
	headerHeight := 2
	inputHeight := 3
	contentHeight := m.height - headerHeight - inputHeight - 4

	leftWidth := int(float64(m.width) * 0.25)
	centerWidth := int(float64(m.width) * 0.50)
	rightWidth := m.width - leftWidth - centerWidth - 2

	header := m.renderHeader()

	m.charSheet.Width = leftWidth - 4
	m.charSheet.Height = contentHeight - 2

	// Update narration viewport size and re-wrap content if dimensions changed
	oldWidth := m.narration.Width
	oldHeight := m.narration.Height
	newWidth := centerWidth - 4
	newHeight := contentHeight - 2
	m.narration.Width = newWidth
	m.narration.Height = newHeight
	if (oldWidth != newWidth || oldHeight != newHeight) && m.narrationContent != "" {
		wrapped := lipgloss.NewStyle().Width(newWidth).Render(m.narrationContent)
		m.narration.SetContent(wrapped)
		m.narration.GotoBottom()
	}

	m.eventLog.Width = rightWidth - 4
	m.eventLog.Height = contentHeight - 2

	charPanel := WrapInPanel(m.charSheet.View(), m.t("panelCharacter"), leftWidth, m.focusPanel == FocusCharacter, m.styles)
	narrPanel := WrapInPanel(m.narration.View(), m.t("panelNarration"), centerWidth, m.focusPanel == FocusNarration, m.styles)
	eventPanel := WrapInPanel(m.eventLog.View(), m.t("panelEventLog"), rightWidth, m.focusPanel == FocusEventLog, m.styles)

	content := lipgloss.JoinHorizontal(lipgloss.Top, charPanel, narrPanel, eventPanel)

	inputStyle := m.styles.Input
	if m.focusPanel == FocusInput {
		inputStyle = m.styles.PanelFocused
	}
	inputBox := inputStyle.Width(m.width - 4).Render(m.styles.InputPrompt.Render("> ") + m.input.View())

	statusBar := m.renderStatusBar()
	shortcutsBar := m.renderShortcutsBar()

	return lipgloss.JoinVertical(lipgloss.Left, header, content, inputBox, statusBar, shortcutsBar)
}

func (m *Model) viewGameCompact() string {
	header := m.renderHeader()

	contentHeight := m.height - 10

	// Update narration viewport size and re-wrap content if dimensions changed
	oldWidth := m.narration.Width
	oldHeight := m.narration.Height
	newWidth := m.width - 4
	newHeight := contentHeight
	m.narration.Width = newWidth
	m.narration.Height = newHeight
	if (oldWidth != newWidth || oldHeight != newHeight) && m.narrationContent != "" {
		wrapped := lipgloss.NewStyle().Width(newWidth).Render(m.narrationContent)
		m.narration.SetContent(wrapped)
		m.narration.GotoBottom()
	}

	narrPanel := WrapInPanel(m.narration.View(), m.t("panelNarration"), m.width-2, true, m.styles)

	inputBox := m.styles.Input.Width(m.width - 4).Render(m.styles.InputPrompt.Render("> ") + m.input.View())

	statusBar := m.renderStatusBar()
	shortcutsBar := m.renderShortcutsBar()

	return lipgloss.JoinVertical(lipgloss.Left, header, narrPanel, inputBox, statusBar, shortcutsBar)
}

func (m *Model) renderHeader() string {
	title := m.styles.HeaderTitle.Render("thAImaturgy")

	status := ""
	if m.session != nil {
		providerName := "none"
		if m.provider != nil {
			providerName = m.provider.Name()
		}
		status = m.styles.HeaderStatus.Render(fmt.Sprintf(" | %s | %s | %s",
			providerName,
			m.config.Model,
			m.session.State.Character.Name))
	}

	if m.loading {
		status += m.styles.Hint.Render(" [loading...]")
	}

	return m.styles.Header.Width(m.width).Render(title + status)
}

func (m *Model) renderStatusBar() string {
	var content string
	if m.errorMsg != "" {
		content = m.styles.Error.Render(m.errorMsg)
		m.errorMsg = ""
	} else if m.statusMsg != "" {
		content = m.styles.Hint.Render(m.statusMsg)
	} else {
		content = m.styles.Hint.Render(m.t("hintDefault"))
	}
	return content
}

func (m *Model) renderShortcutsBar() string {
	shortcuts := []string{
		m.t("shortcutSave"),
		m.t("shortcutHelp"),
		m.t("shortcutRoll"),
		m.t("shortcutVoice"),
		m.t("shortcutQuit"),
	}

	shortcutStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(lipgloss.Color("#AAAAAA")).
		Padding(0, 1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#AAAAAA"))

	var parts []string
	for _, s := range shortcuts {
		// Find where the space is to split key from label
		spaceIdx := strings.Index(s, " ")
		if spaceIdx > 0 {
			parts = append(parts, shortcutStyle.Render(s[:spaceIdx])+labelStyle.Render(s[spaceIdx:]))
		} else {
			parts = append(parts, shortcutStyle.Render(s))
		}
	}

	return strings.Join(parts, "  ")
}

func (m *Model) viewSaves() string {
	var sb strings.Builder

	sb.WriteString(m.styles.WizardTitle.Render(m.t("savesTitle")) + "\n\n")

	if len(m.saves) == 0 {
		sb.WriteString(m.styles.Hint.Render(m.t("savesEmpty") + "\n\n"))
	} else {
		for i, save := range m.saves {
			cursor := "  "
			style := m.styles.WizardOption
			if i == m.saveCursor {
				cursor = "> "
				style = m.styles.WizardSelected
			}
			sb.WriteString(fmt.Sprintf("%s%s - Level %d %s\n",
				cursor, style.Render(save.Name), save.Level, save.Class))
		}
	}

	sb.WriteString("\n")
	sb.WriteString(m.styles.Hint.Render(m.t("savesHint")))

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, sb.String())
}

func (m *Model) viewHelp() string {
	var sb strings.Builder

	sb.WriteString(m.styles.WizardTitle.Render(m.t("helpTitle")) + "\n\n")

	sb.WriteString(m.styles.StatLabel.Render(m.t("helpNavigation")) + "\n")
	sb.WriteString("  TAB        - Switch panels / Cambiar panel\n")
	sb.WriteString("  ↑/↓        - Scroll line / Desplazar línea\n")
	sb.WriteString("  Ctrl+↑/↓   - Scroll fast / Desplazar rápido\n")
	sb.WriteString("  PgUp/PgDn  - Scroll page / Desplazar página\n")
	sb.WriteString("  Home/End   - Top/Bottom / Inicio/Final\n")
	sb.WriteString("  ESC        - Menu / Menú\n\n")

	sb.WriteString(m.styles.StatLabel.Render(m.t("helpCommands")) + "\n")
	sb.WriteString("  /help    - Help / Ayuda\n")
	sb.WriteString("  /save    - Save / Guardar\n")
	sb.WriteString("  /load    - Load / Cargar\n")
	sb.WriteString("  /roll XdY - Roll dice / Tirar dados\n")
	sb.WriteString("  /status  - Status / Estado\n")
	sb.WriteString("  /inv     - Inventory / Inventario\n")
	sb.WriteString("  /quit    - Quit / Salir\n\n")

	sb.WriteString(m.styles.StatLabel.Render("SHORTCUTS / ATAJOS") + "\n")
	sb.WriteString("  ^S Save    ^H Help    ^R Roll d20\n")
	sb.WriteString("  ^N Voice   ^Q Quit    ^↑/↓ Scroll\n\n")

	sb.WriteString(m.styles.StatLabel.Render(m.t("helpGameplay")) + "\n")
	sb.WriteString(m.t("helpGameplayText") + "\n\n")

	sb.WriteString(m.styles.Hint.Render(m.t("helpReturn")))

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, sb.String())
}

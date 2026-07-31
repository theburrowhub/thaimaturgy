package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theburrowhub/thaimaturgy/internal/auth"
	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// sfKind classifies how a settings field is edited.
type sfKind int

const (
	sfSelect sfKind = iota // cycle among opts
	sfText                 // free text edited via settingsInput
	sfInt                  // integer edited via settingsInput
	sfFloat                // float edited via settingsInput
	sfBool                 // toggled
)

type sfield struct {
	key      string
	labelEN  string
	labelES  string
	kind     sfKind
	opts     []string
	editable bool // text/int/float need an input; select/bool are changed in place
}

var settingsFields = []sfield{
	{"provider", "Provider", "Proveedor", sfSelect, []string{"openai", "anthropic", "gemini", "claude-cli"}, false},
	{"model", "Model", "Modelo", sfText, nil, true},
	{"run_model", "Run model", "Modelo del run", sfText, nil, true},
	{"edit_model", "Edit model", "Modelo del editor", sfText, nil, true},
	{"language", "UI language", "Idioma de interfaz", sfSelect, []string{"en", "es"}, false},
	{"import_language", "Import language", "Idioma de importación", sfText, nil, true},
	{"temperature", "Temperature", "Temperatura", sfFloat, nil, true},
	{"max_tokens", "Max tokens", "Máx. tokens", sfInt, nil, true},
	{"import_tokens", "Import max output tokens", "Máx. tokens de importación", sfInt, nil, true},
	{"auto_save", "Auto-save sessions", "Autoguardado", sfBool, nil, false},
	{"tts_enabled", "TTS narration", "Narración TTS", sfBool, nil, false},
}

func (m *Model) settingsLabel(f sfield) string {
	if m.config.Language == domain.LangSpanish {
		return f.labelES
	}
	return f.labelEN
}

// enterSettings loads a fresh copy of the config from disk and opens the editor.
func (m *Model) enterSettings() {
	cfg, err := m.storage.LoadConfig()
	if err != nil || cfg == nil {
		cfg = domain.DefaultConfig()
	}
	m.settingsCfg = cfg
	m.settingsCursor = 0
	m.settingsEditing = false
	m.settingsInput.Blur()
	m.errorMsg = ""
	m.statusMsg = ""
	m.screen = ScreenSettings
}

func (m *Model) updateSettings(msg tea.KeyMsg) tea.Cmd {
	f := settingsFields[m.settingsCursor]

	if m.settingsEditing {
		switch msg.Type {
		case tea.KeyEnter:
			sfSetText(m.settingsCfg, f.key, m.settingsInput.Value())
			m.settingsEditing = false
			m.settingsInput.Blur()
			return nil
		case tea.KeyEsc:
			m.settingsEditing = false
			m.settingsInput.Blur()
			return nil
		default:
			var cmd tea.Cmd
			m.settingsInput, cmd = m.settingsInput.Update(msg)
			return cmd
		}
	}

	switch msg.Type {
	case tea.KeyUp:
		if m.settingsCursor > 0 {
			m.settingsCursor--
		}
	case tea.KeyDown:
		if m.settingsCursor < len(settingsFields)-1 {
			m.settingsCursor++
		}
	case tea.KeyLeft:
		sfCycle(m.settingsCfg, f, -1)
	case tea.KeyRight:
		sfCycle(m.settingsCfg, f, 1)
	case tea.KeyEnter:
		switch f.kind {
		case sfBool:
			sfToggle(m.settingsCfg, f.key)
		case sfSelect:
			sfCycle(m.settingsCfg, f, 1)
		default:
			m.settingsEditing = true
			m.settingsInput.SetValue(sfGet(m.settingsCfg, f.key))
			m.settingsInput.CursorEnd()
			m.settingsInput.Focus()
		}
	case tea.KeySpace:
		if f.kind == sfBool {
			sfToggle(m.settingsCfg, f.key)
		} else if f.kind == sfSelect {
			sfCycle(m.settingsCfg, f, 1)
		}
	case tea.KeyEsc:
		m.screen = ScreenLibrary
	case tea.KeyRunes:
		switch strings.ToLower(string(msg.Runes)) {
		case "s":
			m.saveSettings()
		case "q":
			m.screen = ScreenLibrary
		}
	}
	return nil
}

func (m *Model) saveSettings() {
	if err := m.storage.SaveConfig(m.settingsCfg); err != nil {
		m.errorMsg = err.Error()
		return
	}
	// Re-apply to the running app, mirroring startup: re-detect credentials and
	// honor the run-model override for this session.
	eff := *m.settingsCfg
	msg := auth.AutoConfigure(&eff)
	if eff.RunModel != "" {
		eff.Model = eff.RunModel
	}
	*m.config = eff
	m.initProvider()
	if msg != "" {
		m.statusMsg = msg
	}
	m.screen = ScreenLibrary
}

// --- field accessors -----------------------------------------------------

func sfGet(c *domain.Config, key string) string {
	switch key {
	case "provider":
		return string(c.Provider)
	case "model":
		return c.Model
	case "run_model":
		return c.RunModel
	case "edit_model":
		return c.EditModel
	case "language":
		return string(c.Language)
	case "import_language":
		return c.ImportLanguage
	case "temperature":
		return strconv.FormatFloat(c.Temperature, 'g', -1, 64)
	case "max_tokens":
		return strconv.Itoa(c.MaxTokens)
	case "import_tokens":
		return strconv.Itoa(c.ImportMaxOutputTokens)
	case "auto_save":
		return boolStr(c.AutoSave)
	case "tts_enabled":
		return boolStr(c.TTS.Enabled)
	}
	return ""
}

func sfSetText(c *domain.Config, key, val string) {
	val = strings.TrimSpace(val)
	switch key {
	case "model":
		c.Model = val
	case "run_model":
		c.RunModel = val
	case "edit_model":
		c.EditModel = val
	case "import_language":
		c.ImportLanguage = val
	case "temperature":
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			c.Temperature = f
		}
	case "max_tokens":
		if n, err := strconv.Atoi(val); err == nil {
			c.MaxTokens = n
		}
	case "import_tokens":
		if n, err := strconv.Atoi(val); err == nil {
			c.ImportMaxOutputTokens = n
		}
	}
}

func sfToggle(c *domain.Config, key string) {
	switch key {
	case "auto_save":
		c.AutoSave = !c.AutoSave
	case "tts_enabled":
		c.TTS.Enabled = !c.TTS.Enabled
	}
}

func sfCycle(c *domain.Config, f sfield, dir int) {
	if len(f.opts) == 0 {
		return
	}
	cur := sfGet(c, f.key)
	idx := 0
	for i, o := range f.opts {
		if o == cur {
			idx = i
			break
		}
	}
	idx = (idx + dir + len(f.opts)) % len(f.opts)
	val := f.opts[idx]
	switch f.key {
	case "provider":
		c.Provider = domain.ProviderType(val)
	case "language":
		c.Language = domain.Language(val)
	}
}

func boolStr(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// --- view ----------------------------------------------------------------

func (m *Model) viewSettings() string {
	var b strings.Builder
	title := "SETTINGS"
	if m.config.Language == domain.LangSpanish {
		title = "CONFIGURACIÓN"
	}
	b.WriteString(m.styles.WizardTitle.Render(title) + "\n\n")

	for i, f := range settingsFields {
		label := m.settingsLabel(f)
		value := sfGet(m.settingsCfg, f.key)
		if value == "" {
			value = "—"
		}
		editing := m.settingsEditing && i == m.settingsCursor
		if editing {
			value = m.settingsInput.View()
		}
		line := fmt.Sprintf("%-28s %s", label, value)
		if i == m.settingsCursor {
			b.WriteString(m.styles.WizardSelected.Render("› "+line) + "\n")
		} else {
			b.WriteString(m.styles.WizardOption.Render("  "+line) + "\n")
		}
	}

	b.WriteString("\n")
	if m.errorMsg != "" {
		b.WriteString(m.styles.Error.Render(m.errorMsg) + "\n")
	}
	hint := "↑/↓ move · ←/→ or SPACE change · ENTER edit/toggle · S save · ESC cancel"
	if m.config.Language == domain.LangSpanish {
		hint = "↑/↓ mover · ←/→ o ESPACIO cambiar · ENTER editar/alternar · S guardar · ESC cancelar"
	}
	if m.settingsEditing {
		hint = "ENTER confirm · ESC cancel"
		if m.config.Language == domain.LangSpanish {
			hint = "ENTER confirmar · ESC cancelar"
		}
	}
	b.WriteString(m.styles.Hint.Render(hint))

	return m.styles.App.Render(b.String())
}

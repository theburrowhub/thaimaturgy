package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the active screen.
func (m *Model) View() string {
	switch m.screen {
	case ScreenBoot:
		return m.viewBoot()
	case ScreenConfig:
		return m.viewConfig()
	case ScreenLibrary:
		return m.viewLibrary()
	case ScreenImport:
		return m.viewImport()
	case ScreenSession:
		return m.viewSession()
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
	sb.WriteString(m.styles.Hint.Render("    " + m.t("bootSubtitle")))
	sb.WriteString("\n")
	sb.WriteString(m.styles.Hint.Render("    v2.0.0"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, sb.String())
}

func (m *Model) viewConfig() string {
	var sb strings.Builder
	switch m.configStep {
	case ConfigStepLanguage:
		sb.WriteString(m.styles.WizardTitle.Render("LANGUAGE / IDIOMA") + "\n\n")
		sb.WriteString("Select your language / Selecciona tu idioma:\n\n")
		for i, lang := range []string{"English", "Español"} {
			sb.WriteString(m.renderOption(lang, i == m.configLanguage))
		}
		sb.WriteString("\n" + m.styles.Hint.Render("Use arrows, ENTER to continue"))
	case ConfigStepProvider:
		sb.WriteString(m.styles.WizardTitle.Render(m.t("configTitle")) + "\n\n")
		sb.WriteString(m.t("configNoKey") + "\n\n")
		for i, p := range []string{"OpenAI (GPT-4o)", "Anthropic (Claude)", "Google (Gemini)"} {
			sb.WriteString(m.renderOption(p, i == m.configProvider))
		}
		sb.WriteString("\n" + m.styles.Hint.Render(m.t("configHintArrows")))
	case ConfigStepAPIKey:
		sb.WriteString(m.styles.WizardTitle.Render(m.t("configTitle")) + "\n\n")
		providerName := []string{"OpenAI", "Anthropic", "Gemini"}[m.configProvider]
		sb.WriteString(fmt.Sprintf(m.t("configEnterKey")+"\n\n", providerName))
		sb.WriteString(m.apiKeyInput.View() + "\n\n")
		sb.WriteString(m.styles.Hint.Render(m.t("configKeyTemp")) + "\n")
		sb.WriteString(m.styles.Hint.Render(m.t("configHintEnterEsc")))
	case ConfigStepConfirm:
		sb.WriteString(m.styles.WizardTitle.Render(m.t("configTitle")) + "\n\n")
		sb.WriteString(m.styles.Success.Render(m.t("configSuccess")) + "\n\n")
		providerName := []string{"OpenAI", "Anthropic", "Gemini"}[m.configProvider]
		model := []string{"gpt-4o", "claude-sonnet-4-20250514", "gemini-2.5-flash"}[m.configProvider]
		sb.WriteString(fmt.Sprintf("%s: %s\n", m.t("provider"), m.styles.StatValue.Render(providerName)))
		sb.WriteString(fmt.Sprintf("%s: %s\n\n", m.t("model"), m.styles.StatValue.Render(model)))
		sb.WriteString(m.styles.Hint.Render(m.t("configHintEnter")))
	}
	if m.errorMsg != "" {
		sb.WriteString("\n\n" + m.styles.Error.Render(m.errorMsg))
		m.errorMsg = ""
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, sb.String())
}

func (m *Model) viewLibrary() string {
	var sb strings.Builder
	sb.WriteString(m.styles.BootLogo.Render(LogoSmall) + "\n")
	sb.WriteString(m.styles.WizardTitle.Render(m.t("libraryTitle")) + "\n\n")

	hasAdventure := false
	hasSession := false
	for _, e := range m.libEntries {
		if e.kind == libAdventure {
			hasAdventure = true
		}
		if e.kind == libSession {
			hasSession = true
		}
	}

	printedAdvHeader := false
	printedSesHeader := false
	printedActionsGap := false
	for i, e := range m.libEntries {
		switch e.kind {
		case libAdventure:
			if !printedAdvHeader {
				sb.WriteString(m.styles.StatLabel.Render(m.t("libAdvHeader")) + "\n")
				printedAdvHeader = true
			}
		case libSession:
			if !printedSesHeader {
				sb.WriteString("\n" + m.styles.StatLabel.Render(m.t("libSesHeader")) + "\n")
				printedSesHeader = true
			}
		case libSettings:
			if !printedActionsGap {
				sb.WriteString("\n")
				printedActionsGap = true
			}
		}
		sb.WriteString(m.renderOption(e.label, i == m.libCursor))
	}

	if !hasAdventure && !hasSession {
		sb.WriteString("\n" + m.styles.Hint.Render(m.t("libEmpty")) + "\n")
	}

	sb.WriteString("\n" + m.styles.Hint.Render(m.t("libHint")))
	if !m.config.IsConfigured() {
		sb.WriteString("\n\n" + m.styles.Error.Render("⚠ No API key configured (Settings)"))
	}
	if m.statusMsg != "" {
		sb.WriteString("\n" + m.styles.Success.Render(m.statusMsg))
		m.statusMsg = ""
	}
	if m.errorMsg != "" {
		sb.WriteString("\n" + m.styles.Error.Render(m.errorMsg))
		m.errorMsg = ""
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, sb.String())
}

func (m *Model) viewImport() string {
	var sb strings.Builder
	sb.WriteString(m.styles.WizardTitle.Render(m.t("importTitle")) + "\n\n")
	sb.WriteString(m.t("importPrompt") + "\n\n")
	sb.WriteString(m.importInput.View() + "\n\n")
	sb.WriteString(m.styles.Hint.Render(m.t("importHint")))
	if m.loading {
		sb.WriteString("\n\n" + m.styles.Hint.Render("Importing…"))
	}
	if m.errorMsg != "" {
		sb.WriteString("\n\n" + m.styles.Error.Render(m.errorMsg))
		m.errorMsg = ""
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, sb.String())
}

func (m *Model) viewSession() string {
	header := m.renderHeader()
	statusBar := m.renderStatusBar()
	shortcuts := m.renderShortcutsBar()

	inputStyle := m.styles.Input
	if m.focusPanel == FocusInput {
		inputStyle = m.styles.PanelFocused
	}
	inputBox := inputStyle.Width(m.width - 4).Render(m.styles.InputPrompt.Render("» ") + m.input.View())

	if m.compactMode {
		contentHeight := m.height - 10
		m.oracleVP.Width = m.width - 4
		m.oracleVP.Height = contentHeight
		m.rewrapOracle()
		oraclePanel := WrapInPanel(m.oracleVP.View(), m.t("panelOracle"), m.width-2, true, m.styles)
		return lipgloss.JoinVertical(lipgloss.Left, header, oraclePanel, inputBox, statusBar, shortcuts)
	}

	headerHeight, inputHeight := 2, 3
	contentHeight := m.height - headerHeight - inputHeight - 4
	leftWidth := int(float64(m.width) * 0.25)
	centerWidth := int(float64(m.width) * 0.50)
	rightWidth := m.width - leftWidth - centerWidth - 2

	m.locationVP.Width = leftWidth - 4
	m.locationVP.Height = contentHeight - 2
	m.oracleVP.Width = centerWidth - 4
	m.oracleVP.Height = contentHeight - 2
	m.logVP.Width = rightWidth - 4
	m.logVP.Height = contentHeight - 2
	m.rewrapOracle()

	locPanel := WrapInPanel(m.locationVP.View(), m.t("panelLocation"), leftWidth, m.focusPanel == FocusLocation, m.styles)
	oraclePanel := WrapInPanel(m.oracleVP.View(), m.t("panelOracle"), centerWidth, m.focusPanel == FocusOracle, m.styles)
	logPanel := WrapInPanel(m.logVP.View(), m.t("panelLog"), rightWidth, m.focusPanel == FocusLog, m.styles)

	content := lipgloss.JoinHorizontal(lipgloss.Top, locPanel, oraclePanel, logPanel)
	return lipgloss.JoinVertical(lipgloss.Left, header, content, inputBox, statusBar, shortcuts)
}

// rewrapOracle re-wraps the oracle transcript to the current viewport width.
func (m *Model) rewrapOracle() {
	width := m.oracleVP.Width
	if width < 20 || m.oracleContent == "" {
		return
	}
	m.oracleVP.SetContent(lipgloss.NewStyle().Width(width).Render(m.oracleContent))
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
			providerName, m.config.Model, m.session.Adventure.Title))
	}
	if m.loading {
		status += m.styles.Hint.Render(" [" + m.t("thinking") + "]")
	}
	return m.styles.Header.Width(m.width).Render(title + status)
}

func (m *Model) renderStatusBar() string {
	if m.errorMsg != "" {
		content := m.styles.Error.Render(m.errorMsg)
		m.errorMsg = ""
		return content
	}
	if m.statusMsg != "" {
		content := m.styles.Hint.Render(m.statusMsg)
		return content
	}
	return m.styles.Hint.Render("Type a question · /help for commands · TAB switch panels · ESC library")
}

func (m *Model) renderShortcutsBar() string {
	shortcuts := []string{"^S Save", "^H Help", "^N Voice", "^Q Quit"}
	keyStyle := lipgloss.NewStyle().Foreground(ColorBlack).Background(ColorGray).Padding(0, 1)
	labelStyle := lipgloss.NewStyle().Foreground(ColorGray)
	var parts []string
	for _, s := range shortcuts {
		if idx := strings.Index(s, " "); idx > 0 {
			parts = append(parts, keyStyle.Render(s[:idx])+labelStyle.Render(s[idx:]))
		} else {
			parts = append(parts, keyStyle.Render(s))
		}
	}
	return strings.Join(parts, "  ")
}

func (m *Model) viewHelp() string {
	var sb strings.Builder
	sb.WriteString(m.styles.WizardTitle.Render("HELP / AYUDA") + "\n\n")
	sb.WriteString(m.styles.StatLabel.Render("DM COMMANDS") + "\n")
	sb.WriteString(`  /import <path>   Import a .tar.gz module
  /goto <room_id>  Move the party to a room
  /room  /look     Show the current room
  /zone  /npc <id> /npcs  /event <id>  /item <id>
  /search <query>  Search the whole module
  /map [zone]      Open a zone map (external viewer)
  /art <id|path>   Open NPC/room/item/image art
  /note <text>     Add a note to the timeline
  /flag key=bool   Set a story flag
  /roll <dice>     Roll dice (e.g. /roll 2d6+3)
  /quests  /party  /status  /save  /load  /quit
`)
	sb.WriteString("\n" + m.styles.StatLabel.Render("NAVIGATION") + "\n")
	sb.WriteString("  TAB switch panels · Ctrl+↑/↓ or PgUp/PgDn scroll oracle · ESC library\n")
	sb.WriteString("\n" + m.styles.StatLabel.Render("SHORTCUTS") + "\n")
	sb.WriteString("  ^S Save   ^H Help   ^N Voice   ^Q Quit\n")
	sb.WriteString("\n" + m.styles.StatLabel.Render("GAMEPLAY") + "\n")
	sb.WriteString("  Type any text to ask the oracle about the adventure: what should\n")
	sb.WriteString("  happen, NPC roleplay, read-aloud text, mechanics, and inspiration.\n\n")
	sb.WriteString(m.styles.Hint.Render(m.t("helpReturn")))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, sb.String())
}

func (m *Model) renderOption(label string, selected bool) string {
	cursor := "  "
	style := m.styles.WizardOption
	if selected {
		cursor = "> "
		style = m.styles.WizardSelected
	}
	return cursor + style.Render(label) + "\n"
}

package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	ColorCyan     = lipgloss.Color("#00FFFF")
	ColorMagenta  = lipgloss.Color("#FF00FF")
	ColorYellow   = lipgloss.Color("#FFFF00")
	ColorWhite    = lipgloss.Color("#FFFFFF")
	ColorGreen    = lipgloss.Color("#00FF00")
	ColorRed      = lipgloss.Color("#FF0000")
	ColorDarkGray = lipgloss.Color("#333333")
	ColorGray     = lipgloss.Color("#888888")
	ColorBlack    = lipgloss.Color("#000000")

	ColorPrimary   = ColorCyan
	ColorSecondary = ColorMagenta
	ColorAccent    = ColorYellow
	ColorText      = ColorWhite
	ColorMuted     = ColorGray
	ColorDanger    = ColorRed
	ColorSuccess   = ColorGreen
)

type Styles struct {
	App           lipgloss.Style
	Header        lipgloss.Style
	HeaderTitle   lipgloss.Style
	HeaderStatus  lipgloss.Style

	Panel         lipgloss.Style
	PanelTitle    lipgloss.Style
	PanelContent  lipgloss.Style
	PanelFocused  lipgloss.Style

	Input         lipgloss.Style
	InputPrompt   lipgloss.Style
	InputText     lipgloss.Style

	Narration     lipgloss.Style
	EventLog      lipgloss.Style
	CharSheet     lipgloss.Style

	StatLabel     lipgloss.Style
	StatValue     lipgloss.Style
	StatModifier  lipgloss.Style

	HPFull        lipgloss.Style
	HPLow         lipgloss.Style
	HPCritical    lipgloss.Style

	Condition     lipgloss.Style
	Item          lipgloss.Style
	Quest         lipgloss.Style

	Hint          lipgloss.Style
	Error         lipgloss.Style
	Success       lipgloss.Style

	BootLogo      lipgloss.Style
	BootText      lipgloss.Style

	WizardTitle   lipgloss.Style
	WizardOption  lipgloss.Style
	WizardSelected lipgloss.Style
}

func NewStyles() *Styles {
	s := &Styles{}

	s.App = lipgloss.NewStyle().
		Background(ColorBlack)

	s.Header = lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Background(ColorBlack).
		Bold(true).
		Padding(0, 1)

	s.HeaderTitle = lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)

	s.HeaderStatus = lipgloss.NewStyle().
		Foreground(ColorMuted)

	s.Panel = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(0, 1)

	s.PanelTitle = lipgloss.NewStyle().
		Foreground(ColorAccent).
		Bold(true).
		Padding(0, 1)

	s.PanelContent = lipgloss.NewStyle().
		Foreground(ColorText)

	s.PanelFocused = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorAccent).
		Padding(0, 1)

	s.Input = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorSecondary).
		Padding(0, 1)

	s.InputPrompt = lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true)

	s.InputText = lipgloss.NewStyle().
		Foreground(ColorText)

	s.Narration = lipgloss.NewStyle().
		Foreground(ColorText)

	s.EventLog = lipgloss.NewStyle().
		Foreground(ColorMuted)

	s.CharSheet = lipgloss.NewStyle().
		Foreground(ColorText)

	s.StatLabel = lipgloss.NewStyle().
		Foreground(ColorPrimary)

	s.StatValue = lipgloss.NewStyle().
		Foreground(ColorText).
		Bold(true)

	s.StatModifier = lipgloss.NewStyle().
		Foreground(ColorMuted)

	s.HPFull = lipgloss.NewStyle().
		Foreground(ColorSuccess)

	s.HPLow = lipgloss.NewStyle().
		Foreground(ColorAccent)

	s.HPCritical = lipgloss.NewStyle().
		Foreground(ColorDanger).
		Bold(true)

	s.Condition = lipgloss.NewStyle().
		Foreground(ColorDanger).
		Background(ColorDarkGray).
		Padding(0, 1)

	s.Item = lipgloss.NewStyle().
		Foreground(ColorAccent)

	s.Quest = lipgloss.NewStyle().
		Foreground(ColorSecondary)

	s.Hint = lipgloss.NewStyle().
		Foreground(ColorMuted).
		Italic(true)

	s.Error = lipgloss.NewStyle().
		Foreground(ColorDanger).
		Bold(true)

	s.Success = lipgloss.NewStyle().
		Foreground(ColorSuccess).
		Bold(true)

	s.BootLogo = lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)

	s.BootText = lipgloss.NewStyle().
		Foreground(ColorText)

	s.WizardTitle = lipgloss.NewStyle().
		Foreground(ColorAccent).
		Bold(true)

	s.WizardOption = lipgloss.NewStyle().
		Foreground(ColorText)

	s.WizardSelected = lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)

	return s
}

func (s *Styles) BorderChars() lipgloss.Border {
	return lipgloss.RoundedBorder()
}

func (s *Styles) ASCIIBorderChars() lipgloss.Border {
	return lipgloss.Border{
		Top:         "-",
		Bottom:      "-",
		Left:        "|",
		Right:       "|",
		TopLeft:     "+",
		TopRight:    "+",
		BottomLeft:  "+",
		BottomRight: "+",
	}
}

var Logo = `
 _____ _        _    ___ __  __       _
|_   _| |__    / \  |_ _|  \/  | __ _| |_ _   _ _ __ __ _ _   _
  | | | '_ \  / _ \  | || |\/| |/ _` + "`" + ` | __| | | | '__/ _` + "`" + ` | | | |
  | | | | | |/ ___ \ | || |  | | (_| | |_| |_| | | | (_| | |_| |
  |_| |_| |_/_/   \_\___|_|  |_|\__,_|\__|\__,_|_|  \__, |\__, |
                                                    |___/ |___/
`

var LogoSmall = `
 _   _    _    ___ __  __
| |_| |_ / \  |_ _|  \/  |_ _ _
| __| _ / _ \ | || |\/| | | | | |
|__|_| /_/ \_\___|_|  |_|_,_|_|
`

func WrapInPanel(content string, title string, width int, focused bool, styles *Styles) string {
	style := styles.Panel
	if focused {
		style = styles.PanelFocused
	}

	titleStr := ""
	if title != "" {
		titleStr = styles.PanelTitle.Render(" " + title + " ")
	}

	contentWidth := width - 4
	if contentWidth < 10 {
		contentWidth = 10
	}

	wrappedContent := lipgloss.NewStyle().
		Width(contentWidth).
		Render(content)

	panel := style.
		Width(width).
		Render(wrappedContent)

	if titleStr != "" {
		lines := []rune(panel)
		titleRunes := []rune(titleStr)

		titleStart := 2
		for i, r := range titleRunes {
			if titleStart+i < len(lines) {
				lines[titleStart+i] = r
			}
		}
		panel = string(lines)
	}

	return panel
}

func RenderProgressBar(current, max, width int, styles *Styles) string {
	if width < 5 {
		width = 5
	}

	barWidth := width - 2
	if barWidth < 1 {
		barWidth = 1
	}

	percentage := float64(current) / float64(max)
	if percentage < 0 {
		percentage = 0
	}
	if percentage > 1 {
		percentage = 1
	}

	filled := int(float64(barWidth) * percentage)
	empty := barWidth - filled

	var style lipgloss.Style
	if percentage > 0.5 {
		style = styles.HPFull
	} else if percentage > 0.25 {
		style = styles.HPLow
	} else {
		style = styles.HPCritical
	}

	bar := "[" + style.Render(repeat("=", filled)) + repeat("-", empty) + "]"
	return bar
}

func repeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

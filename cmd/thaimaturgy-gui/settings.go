package main

import (
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/theburrowhub/thaimaturgy/internal/auth"
	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
)

// showSettings renders an in-app editor for config.yaml so the app can be
// configured from the GUI, not only by hand-editing the file. It edits a fresh
// copy loaded from disk; Save persists it and re-applies it to the running app.
func (g *gui) showSettings() {
	cfg, err := g.store.LoadConfig()
	if err != nil || cfg == nil {
		cfg = domain.DefaultConfig()
	}

	provider := widget.NewSelect([]string{
		string(domain.ProviderOpenAI), string(domain.ProviderAnthropic),
		string(domain.ProviderGemini), string(domain.ProviderClaudeCLI),
	}, nil)
	provider.SetSelected(string(cfg.Provider))

	model := entryWith(cfg.Model)
	runModel := entryWith(cfg.RunModel)
	runModel.SetPlaceHolder("(defaults to Model)")
	editModel := entryWith(cfg.EditModel)
	editModel.SetPlaceHolder("(defaults to Model)")

	language := widget.NewSelect([]string{string(domain.LangEnglish), string(domain.LangSpanish)}, nil)
	language.SetSelected(string(cfg.Language))
	importLang := entryWith(cfg.ImportLanguage)
	importLang.SetPlaceHolder("(follows UI language)")

	temperature := entryWith(strconv.FormatFloat(cfg.Temperature, 'g', -1, 64))
	maxTokens := entryWith(strconv.Itoa(cfg.MaxTokens))
	importTokens := entryWith(strconv.Itoa(cfg.ImportMaxOutputTokens))
	toolIter := entryWith(strconv.Itoa(cfg.OracleMaxToolIterations))
	timeout := entryWith(strconv.Itoa(cfg.RequestTimeoutSeconds))

	autoSave := widget.NewCheck("", nil)
	autoSave.SetChecked(cfg.AutoSave)
	autoSaveInterval := entryWith(strconv.Itoa(cfg.AutoSaveInterval))

	ttsEnabled := widget.NewCheck("", nil)
	ttsEnabled.SetChecked(cfg.TTS.Enabled)
	ttsVoice := widget.NewSelect([]string{
		string(domain.TTSVoiceAlloy), string(domain.TTSVoiceEcho), string(domain.TTSVoiceFable),
		string(domain.TTSVoiceOnyx), string(domain.TTSVoiceNova), string(domain.TTSVoiceShimmer),
	}, nil)
	if cfg.TTS.Voice != "" {
		ttsVoice.SetSelected(string(cfg.TTS.Voice))
	}

	openaiKey := widget.NewPasswordEntry()
	anthropicKey := widget.NewPasswordEntry()
	geminiKey := widget.NewPasswordEntry()
	for _, e := range []*widget.Entry{openaiKey, anthropicKey, geminiKey} {
		e.SetPlaceHolder("(applied this session; not written to disk)")
	}

	form := widget.NewForm(
		widget.NewFormItem("Provider", provider),
		widget.NewFormItem("Model", model),
		widget.NewFormItem("Run model", runModel),
		widget.NewFormItem("Edit model", editModel),
		widget.NewFormItem("UI language", language),
		widget.NewFormItem("Import language", importLang),
		widget.NewFormItem("Temperature", temperature),
		widget.NewFormItem("Max tokens", maxTokens),
		widget.NewFormItem("Import max output tokens", importTokens),
		widget.NewFormItem("Oracle max tool iterations", toolIter),
		widget.NewFormItem("Request timeout (s)", timeout),
		widget.NewFormItem("Auto-save sessions", autoSave),
		widget.NewFormItem("Auto-save interval (s)", autoSaveInterval),
		widget.NewFormItem("TTS enabled", ttsEnabled),
		widget.NewFormItem("TTS voice", ttsVoice),
		widget.NewFormItem("OpenAI API key", openaiKey),
		widget.NewFormItem("Anthropic API key", anthropicKey),
		widget.NewFormItem("Gemini API key", geminiKey),
	)

	save := func() {
		cfg.Provider = domain.ProviderType(provider.Selected)
		cfg.Model = strings.TrimSpace(model.Text)
		cfg.RunModel = strings.TrimSpace(runModel.Text)
		cfg.EditModel = strings.TrimSpace(editModel.Text)
		cfg.Language = domain.Language(language.Selected)
		cfg.ImportLanguage = strings.TrimSpace(importLang.Text)
		cfg.Temperature = parseFloat(temperature.Text, cfg.Temperature)
		cfg.MaxTokens = parseInt(maxTokens.Text, cfg.MaxTokens)
		cfg.ImportMaxOutputTokens = parseInt(importTokens.Text, cfg.ImportMaxOutputTokens)
		cfg.OracleMaxToolIterations = parseInt(toolIter.Text, cfg.OracleMaxToolIterations)
		cfg.RequestTimeoutSeconds = parseInt(timeout.Text, cfg.RequestTimeoutSeconds)
		cfg.AutoSave = autoSave.Checked
		cfg.AutoSaveInterval = parseInt(autoSaveInterval.Text, cfg.AutoSaveInterval)
		cfg.TTS.Enabled = ttsEnabled.Checked
		cfg.TTS.Voice = domain.TTSVoice(ttsVoice.Selected)
		if k := strings.TrimSpace(openaiKey.Text); k != "" {
			cfg.OpenAIAPIKey = k
		}
		if k := strings.TrimSpace(anthropicKey.Text); k != "" {
			cfg.AnthropicAPIKey = k
		}
		if k := strings.TrimSpace(geminiKey.Text); k != "" {
			cfg.GeminiAPIKey = k
		}
		if err := g.applySettings(cfg); err != nil {
			g.showErr(err)
			return
		}
		g.showLibrary()
	}

	title := widget.NewLabelWithStyle("⚙  Settings", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	hint := widget.NewLabelWithStyle("Saved to "+g.store.ConfigPath()+". API keys apply this session only (persist them via environment variables or a local login).",
		fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
	hint.Wrapping = fyne.TextWrapWord

	top := container.NewVBox(title, widget.NewSeparator())
	buttons := container.NewHBox(
		widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), g.showLibrary),
		widget.NewButtonWithIcon("Save", theme.ConfirmIcon(), save),
	)
	bottom := container.NewVBox(widget.NewSeparator(), hint, buttons)
	g.win.SetContent(container.NewBorder(top, bottom, nil, nil, container.NewVScroll(form)))
}

// applySettings persists cfg and re-applies it to the running app, mirroring
// startup: re-detect credentials and honor the run-model override.
func (g *gui) applySettings(cfg *domain.Config) error {
	if err := g.store.SaveConfig(cfg); err != nil {
		return err
	}
	eff := *cfg
	g.authMsg = auth.AutoConfigure(&eff)
	if eff.RunModel != "" {
		eff.Model = eff.RunModel
	}
	g.config = &eff
	g.prov = providers.New(&eff)
	return nil
}

func entryWith(text string) *widget.Entry {
	e := widget.NewEntry()
	e.SetText(text)
	return e
}

func parseInt(s string, fallback int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return n
	}
	return fallback
}

func parseFloat(s string, fallback float64) float64 {
	if f, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
		return f
	}
	return fallback
}

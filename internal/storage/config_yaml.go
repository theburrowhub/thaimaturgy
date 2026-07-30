package storage

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// fileConfig is the on-disk YAML shape: a human-organized view of domain.Config
// grouped into sections. It maps to/from the flat runtime Config so the rest of
// the code is unaffected by the file layout.
type fileConfig struct {
	Provider struct {
		Name            string  `yaml:"name"`  // openai | anthropic | gemini
		Model           string  `yaml:"model"` // model id
		Temperature     float64 `yaml:"temperature"`
		MaxTokens       int     `yaml:"max_tokens"`
		OpenAIAPIKey    string  `yaml:"openai_api_key"`    // optional; prefer env / local login
		AnthropicAPIKey string  `yaml:"anthropic_api_key"` // optional
		GeminiAPIKey    string  `yaml:"gemini_api_key"`    // optional
	} `yaml:"provider"`

	UI struct {
		Language      string `yaml:"language"` // en | es
		ShowScanlines bool   `yaml:"show_scanlines"`
		BorderStyle   string `yaml:"border_style"`
	} `yaml:"ui"`

	Session struct {
		AutoSave         bool   `yaml:"auto_save"`
		AutoSaveInterval int    `yaml:"auto_save_interval"`
		DefaultSetting   string `yaml:"default_setting"`
	} `yaml:"session"`

	Oracle struct {
		MaxToolIterations     int `yaml:"max_tool_iterations"`
		RecentTimeline        int `yaml:"recent_timeline"`
		SummarizeAfter        int `yaml:"summarize_after"`
		RequestTimeoutSeconds int `yaml:"request_timeout_seconds"`
	} `yaml:"oracle"`

	Import struct {
		// Language adventures are authored in during AI import, independent of the
		// source document (e.g. import an English PDF as a Spanish module). A name or
		// code ("Spanish", "es", "French"); empty follows ui.language. Game terms —
		// monster/spell/item names — are written translated with the original in
		// parentheses so they stay searchable in official books.
		Language         string `yaml:"language"`
		VisionMaxImages  int    `yaml:"vision_max_images"`
		VisionMaxImageMB int    `yaml:"vision_max_image_mb"`
		MaxDocChars      int    `yaml:"max_doc_chars"`
		MaxOutputTokens  int    `yaml:"max_output_tokens"`
	} `yaml:"import"`

	TTS struct {
		Enabled bool    `yaml:"enabled"`
		Voice   string  `yaml:"voice"`
		Model   string  `yaml:"model"`
		Speed   float64 `yaml:"speed"`
	} `yaml:"tts"`

	SystemPrompt string `yaml:"system_prompt,omitempty"`
}

func fromConfig(c *domain.Config) fileConfig {
	var fc fileConfig
	fc.Provider.Name = string(c.Provider)
	fc.Provider.Model = c.Model
	fc.Provider.Temperature = c.Temperature
	fc.Provider.MaxTokens = c.MaxTokens
	fc.Provider.OpenAIAPIKey = c.OpenAIAPIKey
	fc.Provider.AnthropicAPIKey = c.AnthropicAPIKey
	fc.Provider.GeminiAPIKey = c.GeminiAPIKey

	fc.UI.Language = string(c.Language)
	fc.UI.ShowScanlines = c.ShowScanlines
	fc.UI.BorderStyle = c.BorderStyle

	fc.Session.AutoSave = c.AutoSave
	fc.Session.AutoSaveInterval = c.AutoSaveInterval
	fc.Session.DefaultSetting = c.DefaultSetting

	fc.Oracle.MaxToolIterations = c.OracleMaxToolIterations
	fc.Oracle.RecentTimeline = c.OracleRecentTimeline
	fc.Oracle.SummarizeAfter = c.OracleSummarizeAfter
	fc.Oracle.RequestTimeoutSeconds = c.RequestTimeoutSeconds

	fc.Import.Language = c.ImportLanguage
	fc.Import.VisionMaxImages = c.ImportVisionMaxImages
	fc.Import.VisionMaxImageMB = c.ImportVisionMaxImageMB
	fc.Import.MaxDocChars = c.ImportMaxDocChars
	fc.Import.MaxOutputTokens = c.ImportMaxOutputTokens

	fc.TTS.Enabled = c.TTS.Enabled
	fc.TTS.Voice = string(c.TTS.Voice)
	fc.TTS.Model = c.TTS.Model
	fc.TTS.Speed = c.TTS.Speed

	fc.SystemPrompt = c.SystemPrompt
	return fc
}

func toConfig(fc *fileConfig, c *domain.Config) {
	c.Provider = domain.ProviderType(fc.Provider.Name)
	c.Model = fc.Provider.Model
	c.Temperature = fc.Provider.Temperature
	c.MaxTokens = fc.Provider.MaxTokens
	c.OpenAIAPIKey = fc.Provider.OpenAIAPIKey
	c.AnthropicAPIKey = fc.Provider.AnthropicAPIKey
	c.GeminiAPIKey = fc.Provider.GeminiAPIKey

	c.Language = domain.Language(fc.UI.Language)
	c.ShowScanlines = fc.UI.ShowScanlines
	c.BorderStyle = fc.UI.BorderStyle

	c.AutoSave = fc.Session.AutoSave
	c.AutoSaveInterval = fc.Session.AutoSaveInterval
	c.DefaultSetting = fc.Session.DefaultSetting

	c.OracleMaxToolIterations = fc.Oracle.MaxToolIterations
	c.OracleRecentTimeline = fc.Oracle.RecentTimeline
	c.OracleSummarizeAfter = fc.Oracle.SummarizeAfter
	c.RequestTimeoutSeconds = fc.Oracle.RequestTimeoutSeconds

	c.ImportLanguage = fc.Import.Language
	c.ImportVisionMaxImages = fc.Import.VisionMaxImages
	c.ImportVisionMaxImageMB = fc.Import.VisionMaxImageMB
	c.ImportMaxDocChars = fc.Import.MaxDocChars
	c.ImportMaxOutputTokens = fc.Import.MaxOutputTokens

	c.TTS.Enabled = fc.TTS.Enabled
	c.TTS.Voice = domain.TTSVoice(fc.TTS.Voice)
	c.TTS.Model = fc.TTS.Model
	c.TTS.Speed = fc.TTS.Speed

	c.SystemPrompt = fc.SystemPrompt
}

// applyYAML overlays a YAML document onto an already-populated config, so keys
// absent from the file keep their (default) values.
func applyYAML(data []byte, c *domain.Config) error {
	fc := fromConfig(c)
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return err
	}
	toConfig(&fc, c)
	return nil
}

// marshalYAML renders the config as an organized YAML document with a header,
// stripping secrets (API keys are never written to disk).
func marshalYAML(c *domain.Config) ([]byte, error) {
	fc := fromConfig(c)
	fc.Provider.OpenAIAPIKey = ""
	fc.Provider.AnthropicAPIKey = ""
	fc.Provider.GeminiAPIKey = ""

	body, err := yaml.Marshal(&fc)
	if err != nil {
		return nil, err
	}

	header := "# thAImaturgy configuration\n" +
		"# Shared by the TUI, the GUI and the module editor.\n" +
		"# Edit freely. Secrets (API keys) are best provided via environment\n" +
		"# variables (OPENAI_API_KEY / ANTHROPIC_API_KEY / GEMINI_API_KEY) or a\n" +
		"# local login (Claude Code / Gemini CLI) and are never written here.\n"
	if c.AuthSource != "" {
		header += fmt.Sprintf("# Active credential (auto-detected): %s\n", c.AuthSource)
	}
	header += "\n"

	return append([]byte(header), body...), nil
}

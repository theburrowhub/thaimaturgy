package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

func clearProviderEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"THAIM_PROVIDER", "THAIM_MODEL",
		"THAIM_OPENAI_API_KEY", "OPENAI_API_KEY",
		"THAIM_ANTHROPIC_API_KEY", "ANTHROPIC_API_KEY",
		"THAIM_GEMINI_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY",
	} {
		t.Setenv(k, "")
	}
}

func TestConfigYAMLFormatAndSecrets(t *testing.T) {
	clearProviderEnv(t)
	store, _ := NewWithPath(t.TempDir())

	c := domain.DefaultConfig()
	c.Provider = domain.ProviderAnthropic
	c.Model = "claude-haiku-4-5-20251001"
	c.AnthropicAPIKey = "sk-ant-SECRET"
	c.OracleMaxToolIterations = 9
	c.ImportVisionMaxImages = 3

	if err := store.SaveConfig(c); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	raw, err := os.ReadFile(store.ConfigPath())
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(raw)
	for _, want := range []string{"provider:", "ui:", "session:", "oracle:", "import:", "tts:", "model: claude-haiku-4-5-20251001"} {
		if !strings.Contains(text, want) {
			t.Errorf("config yaml missing %q\n---\n%s", want, text)
		}
	}
	if strings.Contains(text, "SECRET") {
		t.Error("API key was written to the config file (must be stripped)")
	}
	if !strings.HasSuffix(store.ConfigPath(), ".yaml") {
		t.Errorf("config path should be YAML, got %q", store.ConfigPath())
	}

	loaded, err := store.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if loaded.Provider != domain.ProviderAnthropic || loaded.Model != "claude-haiku-4-5-20251001" {
		t.Errorf("provider/model not round-tripped: %+v", loaded)
	}
	if loaded.OracleMaxToolIterations != 9 || loaded.ImportVisionMaxImages != 3 {
		t.Errorf("tunables not round-tripped: oracle=%d import=%d", loaded.OracleMaxToolIterations, loaded.ImportVisionMaxImages)
	}
	if loaded.AnthropicAPIKey != "" {
		t.Error("secret should not be persisted/loaded from the file")
	}
	// A default not present in the file must keep its default value.
	if loaded.OracleRecentTimeline != domain.DefaultConfig().OracleRecentTimeline {
		t.Errorf("absent key should keep default, got %d", loaded.OracleRecentTimeline)
	}
}

func TestConfigMigratesLegacyJSON(t *testing.T) {
	clearProviderEnv(t)
	base := t.TempDir()
	legacy := `{"provider":"gemini","model":"gemini-2.5-flash","temperature":0.5,"max_tokens":1234}`
	if err := os.WriteFile(filepath.Join(base, ConfigFile), []byte(legacy), 0644); err != nil {
		t.Fatal(err)
	}

	store, _ := NewWithPath(base)
	loaded, err := store.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if loaded.Provider != domain.ProviderGemini || loaded.Model != "gemini-2.5-flash" || loaded.MaxTokens != 1234 {
		t.Errorf("legacy JSON not migrated correctly: %+v", loaded)
	}
	if !store.ConfigExists() {
		t.Error("migration should have written the YAML config")
	}
}

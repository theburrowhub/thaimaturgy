package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// isolate clears all provider env vars, points HOME at a temp dir, and disables
// the macOS keychain lookup so detection is deterministic.
func isolate(t *testing.T) string {
	t.Helper()
	for _, k := range []string{
		"THAIM_OPENAI_API_KEY", "OPENAI_API_KEY",
		"THAIM_ANTHROPIC_API_KEY", "ANTHROPIC_API_KEY",
		"THAIM_GEMINI_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY",
	} {
		t.Setenv(k, "")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)

	orig := macKeychainCreds
	macKeychainCreds = func() ([]byte, bool) { return nil, false }
	t.Cleanup(func() { macKeychainCreds = orig })
	return home
}

func TestDetectEnvKeys(t *testing.T) {
	isolate(t)
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-xyz")
	t.Setenv("OPENAI_API_KEY", "sk-oai-xyz")
	t.Setenv("GEMINI_API_KEY", "AIza-xyz")

	creds := Detect()
	got := map[domain.ProviderType]Credential{}
	for _, c := range creds {
		got[c.Provider] = c
	}
	if c, ok := got[domain.ProviderAnthropic]; !ok || c.APIKey != "sk-ant-xyz" || c.Method != MethodAPIKey {
		t.Errorf("anthropic env key not detected: %+v", c)
	}
	if c, ok := got[domain.ProviderOpenAI]; !ok || c.APIKey != "sk-oai-xyz" {
		t.Errorf("openai env key not detected: %+v", c)
	}
	if c, ok := got[domain.ProviderGemini]; !ok || c.APIKey != "AIza-xyz" {
		t.Errorf("gemini env key not detected: %+v", c)
	}
}

func TestDetectClaudeCodeFile(t *testing.T) {
	home := isolate(t)
	dir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	creds := `{"claudeAiOauth":{"accessToken":"sk-ant-oat01-abc","expiresAt":9999999999999}}`
	if err := os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(creds), 0600); err != nil {
		t.Fatal(err)
	}

	found := Detect()
	var oauth *Credential
	for i := range found {
		if found[i].Provider == domain.ProviderAnthropic && found[i].Method == MethodOAuth {
			oauth = &found[i]
		}
	}
	if oauth == nil {
		t.Fatal("expected an Anthropic OAuth credential from the Claude Code file")
	}
	if oauth.Token != "sk-ant-oat01-abc" {
		t.Errorf("token = %q", oauth.Token)
	}
	if oauth.Expired {
		t.Error("token should not be marked expired")
	}
}

func TestAutoConfigureAppliesOAuth(t *testing.T) {
	home := isolate(t)
	dir := filepath.Join(home, ".claude")
	_ = os.MkdirAll(dir, 0755)
	creds := `{"claudeAiOauth":{"accessToken":"sk-ant-oat01-live","expiresAt":9999999999999}}`
	_ = os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(creds), 0600)

	c := domain.DefaultConfig() // openai default, no key → not configured
	msg := AutoConfigure(c)
	if msg == "" {
		t.Fatal("expected an auto-config message")
	}
	if c.Provider != domain.ProviderAnthropic {
		t.Errorf("provider = %q, want anthropic", c.Provider)
	}
	if c.AnthropicOAuthToken != "sk-ant-oat01-live" {
		t.Errorf("oauth token not applied: %q", c.AnthropicOAuthToken)
	}
	if !c.IsConfigured() {
		t.Error("config should be configured after auto-config")
	}
	// A Claude Code subscription login defaults to Haiku (reliably available).
	if c.Model != "claude-haiku-4-5-20251001" {
		t.Errorf("model = %q, want claude-haiku-4-5-20251001 for OAuth login", c.Model)
	}
}

func TestAutoConfigurePrefersConfiguredProvider(t *testing.T) {
	isolate(t)
	t.Setenv("OPENAI_API_KEY", "sk-oai")
	t.Setenv("GEMINI_API_KEY", "AIza")

	c := domain.DefaultConfig() // provider openai, but no key yet
	// Simulate that env keys were merged already: mark openai configured.
	c.OpenAIAPIKey = "sk-oai"
	msg := AutoConfigure(c)
	if msg == "" {
		t.Error("expected a message describing the active source")
	}
	if c.Provider != domain.ProviderOpenAI {
		t.Errorf("should keep configured provider openai, got %q", c.Provider)
	}
}

func TestAutoConfigureNothing(t *testing.T) {
	isolate(t)
	c := domain.DefaultConfig()
	if msg := AutoConfigure(c); msg != "" {
		t.Errorf("expected empty message with no creds, got %q", msg)
	}
}

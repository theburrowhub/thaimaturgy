// Package auth discovers AI-provider credentials already present on the machine
// — provider API keys in the environment, and OAuth logins from local tools
// like Claude Code and the Gemini CLI — so the app can auto-configure itself
// and tell the user which credential it picked up.
package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// Method describes how a credential authenticates.
type Method string

const (
	MethodAPIKey Method = "api_key"
	MethodOAuth  Method = "oauth"
)

// Credential is a discovered way to talk to a provider.
type Credential struct {
	Provider domain.ProviderType
	Method   Method
	APIKey   string
	Token    string
	Source   string // human-readable origin, e.g. "Claude Code login (Keychain)"
	Expired  bool
}

// Detect probes every known credential source and returns what it finds, in a
// stable priority order: for each provider, explicit env API keys first, then
// reused local OAuth logins.
func Detect() []Credential {
	var creds []Credential
	creds = append(creds, detectAnthropic()...)
	creds = append(creds, detectOpenAI()...)
	creds = append(creds, detectGemini()...)
	return creds
}

// AutoConfigure applies a detected credential to the config when it isn't
// already configured, and returns a one-line message describing what happened
// (empty if nothing was detected). It never overrides an explicit configuration
// — it only reports the active source in that case.
func AutoConfigure(c *domain.Config) string {
	creds := Detect()
	if len(creds) == 0 {
		return ""
	}
	if c.IsConfigured() {
		for _, cr := range creds {
			if cr.Provider == c.Provider {
				return fmt.Sprintf("Using %s via %s.", label(cr.Provider), cr.Source)
			}
		}
		return ""
	}
	cr := pick(creds, c.Provider)
	apply(c, cr)
	msg := fmt.Sprintf("Auto-detected %s via %s — configured automatically.", label(cr.Provider), cr.Source)
	if cr.Expired {
		msg += " ⚠ the local token looks expired; re-login if requests fail."
	}
	return msg
}

func pick(creds []Credential, prefer domain.ProviderType) Credential {
	for _, cr := range creds {
		if cr.Provider == prefer {
			return cr
		}
	}
	return creds[0]
}

func apply(c *domain.Config, cr Credential) {
	c.Provider = cr.Provider
	switch cr.Provider {
	case domain.ProviderOpenAI:
		c.OpenAIAPIKey = cr.APIKey
	case domain.ProviderAnthropic:
		if cr.Method == MethodOAuth {
			c.AnthropicOAuthToken = cr.Token
		} else {
			c.AnthropicAPIKey = cr.APIKey
		}
	case domain.ProviderGemini:
		if cr.Method == MethodOAuth {
			c.GeminiOAuthToken = cr.Token
		} else {
			c.GeminiAPIKey = cr.APIKey
		}
	}
	c.Model = domain.DefaultModel(cr.Provider)
	c.AuthSource = cr.Source
}

func label(p domain.ProviderType) string {
	switch p {
	case domain.ProviderOpenAI:
		return "OpenAI"
	case domain.ProviderAnthropic:
		return "Anthropic (Claude)"
	case domain.ProviderGemini:
		return "Gemini"
	}
	return string(p)
}

// --- Anthropic -----------------------------------------------------------

func detectAnthropic() []Credential {
	var out []Credential
	if k := firstEnv("THAIM_ANTHROPIC_API_KEY", "ANTHROPIC_API_KEY"); k != "" {
		out = append(out, Credential{Provider: domain.ProviderAnthropic, Method: MethodAPIKey, APIKey: k, Source: "ANTHROPIC_API_KEY environment variable"})
	}
	if tok, exp, src, ok := claudeCodeToken(); ok {
		out = append(out, Credential{Provider: domain.ProviderAnthropic, Method: MethodOAuth, Token: tok, Source: src, Expired: exp})
	}
	return out
}

// macKeychainCreds returns the raw Claude Code credential blob from the macOS
// Keychain. It's a package var so tests can stub it deterministically.
var macKeychainCreds = func() ([]byte, bool) {
	out, err := exec.Command("security", "find-generic-password", "-s", "Claude Code-credentials", "-w").Output()
	if err != nil {
		return nil, false
	}
	return out, true
}

// claudeCodeToken reads the OAuth access token stored by a local Claude Code
// login — the macOS Keychain first, then ~/.claude/.credentials.json.
func claudeCodeToken() (token string, expired bool, source string, ok bool) {
	if runtime.GOOS == "darwin" {
		if out, ok := macKeychainCreds(); ok {
			if t, e, ok := parseClaudeCreds(out); ok {
				return t, e, "Claude Code login (Keychain)", true
			}
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		p := filepath.Join(home, ".claude", ".credentials.json")
		if data, err := os.ReadFile(p); err == nil {
			if t, e, ok := parseClaudeCreds(data); ok {
				return t, e, "Claude Code login (~/.claude)", true
			}
		}
	}
	return "", false, "", false
}

func parseClaudeCreds(data []byte) (token string, expired bool, ok bool) {
	var v struct {
		ClaudeAiOauth struct {
			AccessToken string `json:"accessToken"`
			ExpiresAt   int64  `json:"expiresAt"` // epoch ms
		} `json:"claudeAiOauth"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return "", false, false
	}
	tok := strings.TrimSpace(v.ClaudeAiOauth.AccessToken)
	if tok == "" {
		return "", false, false
	}
	exp := v.ClaudeAiOauth.ExpiresAt > 0 && time.Now().UnixMilli() > v.ClaudeAiOauth.ExpiresAt
	return tok, exp, true
}

// --- OpenAI --------------------------------------------------------------

func detectOpenAI() []Credential {
	if k := firstEnv("THAIM_OPENAI_API_KEY", "OPENAI_API_KEY"); k != "" {
		return []Credential{{Provider: domain.ProviderOpenAI, Method: MethodAPIKey, APIKey: k, Source: "OPENAI_API_KEY environment variable"}}
	}
	return nil
}

// --- Gemini --------------------------------------------------------------

func detectGemini() []Credential {
	var out []Credential
	if k := firstEnv("THAIM_GEMINI_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY"); k != "" {
		out = append(out, Credential{Provider: domain.ProviderGemini, Method: MethodAPIKey, APIKey: k, Source: "GEMINI_API_KEY environment variable"})
	}
	if tok, exp, src, ok := geminiCLIToken(); ok {
		out = append(out, Credential{Provider: domain.ProviderGemini, Method: MethodOAuth, Token: tok, Source: src, Expired: exp})
	}
	return out
}

// geminiCLIToken reads the OAuth access token stored by the Gemini CLI login.
func geminiCLIToken() (token string, expired bool, source string, ok bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false, "", false
	}
	p := filepath.Join(home, ".gemini", "oauth_creds.json")
	data, err := os.ReadFile(p)
	if err != nil {
		return "", false, "", false
	}
	var v struct {
		AccessToken string `json:"access_token"`
		ExpiryDate  int64  `json:"expiry_date"` // epoch ms
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return "", false, "", false
	}
	tok := strings.TrimSpace(v.AccessToken)
	if tok == "" {
		return "", false, "", false
	}
	exp := v.ExpiryDate > 0 && time.Now().UnixMilli() > v.ExpiryDate
	return tok, exp, "Gemini CLI login (~/.gemini)", true
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

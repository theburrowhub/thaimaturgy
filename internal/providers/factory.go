package providers

import "github.com/theburrowhub/thaimaturgy/internal/domain"

// New builds the Provider for the active configuration, using an API key or a
// reused local OAuth token, whichever is present. Returns nil if the active
// provider has no usable credential.
func New(c *domain.Config) Provider {
	switch c.Provider {
	case domain.ProviderOpenAI:
		if c.OpenAIAPIKey != "" {
			return NewOpenAIProvider(c.OpenAIAPIKey)
		}
	case domain.ProviderAnthropic:
		if c.AnthropicOAuthToken != "" {
			return NewAnthropicOAuthProvider(c.AnthropicOAuthToken)
		}
		if c.AnthropicAPIKey != "" {
			return NewAnthropicProvider(c.AnthropicAPIKey)
		}
	case domain.ProviderGemini:
		if c.GeminiAPIKey != "" {
			return NewGeminiProvider(c.GeminiAPIKey)
		}
		if c.GeminiOAuthToken != "" {
			return NewGeminiOAuthProvider(c.GeminiOAuthToken)
		}
	}
	return nil
}

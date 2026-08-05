package tts

import (
	"context"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

type testSpeechGenerator interface {
	Generate(ctx context.Context, text string) (string, error)
}

func TestNewTelegramSpeechGeneratorDisabledReturnsNilInterface(t *testing.T) {
	cfg := domain.DefaultConfig()
	cfg.TTS.Enabled = false
	cfg.TTS.Provider = domain.TTSProviderOpenAI

	var speech testSpeechGenerator = NewTelegramSpeechGenerator(cfg, t.TempDir())
	if speech != nil {
		t.Fatalf("disabled TTS returned non-nil interface: %T", speech)
	}
}

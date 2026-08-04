package tgbot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
	"github.com/theburrowhub/thaimaturgy/internal/tts"
)

// SpeechGenerator turns DM narration text into an audio file ready to upload to
// Telegram. Implementations should cache where possible and return a local file
// path. The bot treats failures as non-fatal and still sends text.
type SpeechGenerator interface {
	Generate(ctx context.Context, text string) (string, error)
}

type OpenAISpeechGenerator struct {
	APIKey   string
	Config   domain.TTSConfig
	CacheDir string
}

func NewOpenAISpeechGenerator(config *domain.Config, store *storage.Storage) SpeechGenerator {
	if config == nil || !config.TTS.Enabled || strings.TrimSpace(config.OpenAIAPIKey) == "" {
		return nil
	}
	cacheDir := filepath.Join(os.TempDir(), "thaimaturgy-tts-cache")
	if store != nil {
		cacheDir = filepath.Join(store.BasePath(), "tts-cache")
	}
	return &OpenAISpeechGenerator{APIKey: config.OpenAIAPIKey, Config: config.TTS, CacheDir: cacheDir}
}

func (g *OpenAISpeechGenerator) Generate(ctx context.Context, text string) (string, error) {
	if g == nil || strings.TrimSpace(g.APIKey) == "" || !g.Config.Enabled {
		return "", fmt.Errorf("TTS is not configured")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("empty narration")
	}
	if err := os.MkdirAll(g.CacheDir, 0700); err != nil {
		return "", err
	}
	path := filepath.Join(g.CacheDir, g.cacheKey(text)+".mp3")
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		return path, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return path, tts.GenerateSpeechFile(ctx, g.APIKey, &g.Config, text, path)
}

func (g *OpenAISpeechGenerator) cacheKey(text string) string {
	h := sha256.New()
	h.Write([]byte(g.Config.Model))
	h.Write([]byte{0})
	h.Write([]byte(g.Config.Voice))
	h.Write([]byte{0})
	h.Write([]byte(fmt.Sprintf("%.3f", g.Config.Speed)))
	h.Write([]byte{0})
	h.Write([]byte(text))
	return hex.EncodeToString(h.Sum(nil))
}

func (b *Bot) sendNarration(chatID int64, text string) {
	b.send(chatID, text)
	if b.speech == nil {
		return
	}
	ctx, cancel := context.WithTimeout(b.turnBase(), b.turnTimeout()+30*time.Second)
	defer cancel()
	path, err := b.speech.Generate(ctx, text)
	if err != nil {
		log.Printf("telegram tts: %v", err)
		return
	}
	audio := tgbotapi.NewAudio(chatID, tgbotapi.FilePath(path))
	audio.Title = "DM narration"
	if _, err := b.sendChattable(audio); err != nil {
		log.Printf("send audio: %v", err)
	}
}

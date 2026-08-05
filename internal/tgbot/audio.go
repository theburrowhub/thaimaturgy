package tgbot

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// SpeechGenerator produces an audio file for a Telegram DM narration. The file
// must be readable by Telegram's uploader (typically MP3 for sendAudio).
type SpeechGenerator interface {
	Generate(ctx context.Context, text string) (string, error)
}

// sendNarration always posts the DM text first, then best-effort sends audio for
// the same text. TTS/audio errors are logged but never break the Telegram game.
func (b *Bot) sendNarration(chatID int64, text string) {
	b.send(chatID, text)
	if b == nil || b.speech == nil {
		return
	}
	ctx, cancel := context.WithTimeout(b.turnBase(), b.safeTurnTimeout())
	defer cancel()
	path, err := b.speech.Generate(ctx, text)
	if err != nil {
		log.Printf("telegram narration audio: %v", err)
		return
	}
	if path == "" {
		return
	}
	if st, err := os.Stat(path); err != nil || st.IsDir() || st.Size() == 0 {
		if err == nil {
			err = fmt.Errorf("not a non-empty audio file: %s", path)
		}
		log.Printf("telegram narration audio: %v", err)
		return
	}
	audio := tgbotapi.NewAudio(chatID, tgbotapi.FilePath(path))
	audio.Title = "DM narration"
	if _, err := b.sendChattable(audio); err != nil {
		log.Printf("telegram send audio: %v", err)
	}
}

func (b *Bot) sendChattable(ch tgbotapi.Chattable) (tgbotapi.Message, error) {
	if b != nil && b.sendFunc != nil {
		return b.sendFunc(ch)
	}
	return b.api.Send(ch)
}

func (b *Bot) safeTurnTimeout() time.Duration {
	if b != nil && b.session != nil && b.session.Config != nil {
		if t := time.Duration(b.session.Config.RequestTimeoutSeconds) * time.Second; t > 0 {
			return t
		}
	}
	return 90 * time.Second
}

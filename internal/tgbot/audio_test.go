package tgbot

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type fakeSpeech struct {
	path string
	err  error
	text string
}

func (f *fakeSpeech) Generate(ctx context.Context, text string) (string, error) {
	f.text = text
	return f.path, f.err
}

func TestSendNarrationSendsTextThenAudioWhenSpeechConfigured(t *testing.T) {
	audio := filepath.Join(t.TempDir(), "dm.mp3")
	if err := os.WriteFile(audio, []byte("mp3"), 0600); err != nil {
		t.Fatalf("write fake audio: %v", err)
	}
	speech := &fakeSpeech{path: audio}
	var sent []string
	b := &Bot{
		speech: speech,
		sendFunc: func(c tgbotapi.Chattable) (tgbotapi.Message, error) {
			switch c.(type) {
			case tgbotapi.MessageConfig:
				sent = append(sent, "text")
			case tgbotapi.AudioConfig:
				sent = append(sent, "audio")
			default:
				t.Fatalf("unexpected chattable %T", c)
			}
			return tgbotapi.Message{}, nil
		},
	}

	b.sendNarration(99, "The crypt bell rings.")

	if speech.text != "The crypt bell rings." {
		t.Fatalf("speech generated from %q", speech.text)
	}
	if got, want := sent, []string{"text", "audio"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("sent sequence = %#v, want %#v", got, want)
	}
}

func TestSendNarrationFallsBackToTextWhenSpeechFails(t *testing.T) {
	speech := &fakeSpeech{err: errors.New("tts down")}
	var sent []string
	b := &Bot{
		speech: speech,
		sendFunc: func(c tgbotapi.Chattable) (tgbotapi.Message, error) {
			switch c.(type) {
			case tgbotapi.MessageConfig:
				sent = append(sent, "text")
			case tgbotapi.AudioConfig:
				sent = append(sent, "audio")
			}
			return tgbotapi.Message{}, nil
		},
	}

	b.sendNarration(99, "The DM speaks.")

	if got, want := sent, []string{"text"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("sent sequence = %#v, want %#v", got, want)
	}
}

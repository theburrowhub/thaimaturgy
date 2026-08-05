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
	path  string
	err   error
	texts []string
}

func (f *fakeSpeech) Generate(ctx context.Context, text string) (string, error) {
	f.texts = append(f.texts, text)
	return f.path, f.err
}

func TestSendNarrationSendsTextThenAudioWhenSpeechConfigured(t *testing.T) {
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "dm.mp3")
	if err := os.WriteFile(audioPath, []byte("mp3"), 0600); err != nil {
		t.Fatal(err)
	}
	speech := &fakeSpeech{path: audioPath}
	var sent []string
	bot := &Bot{speech: speech, sendFunc: func(ch tgbotapi.Chattable) (tgbotapi.Message, error) {
		switch ch.(type) {
		case tgbotapi.MessageConfig:
			sent = append(sent, "text")
		case tgbotapi.AudioConfig:
			sent = append(sent, "audio")
		default:
			t.Fatalf("unexpected chattable %T", ch)
		}
		return tgbotapi.Message{}, nil
	}}

	bot.sendNarration(123, "The door opens.")

	if got, want := sent, []string{"text", "audio"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("sent %v, want %v", got, want)
	}
	if len(speech.texts) != 1 || speech.texts[0] != "The door opens." {
		t.Fatalf("speech texts = %#v", speech.texts)
	}
}

func TestSendNarrationFallsBackToTextWhenSpeechFails(t *testing.T) {
	speech := &fakeSpeech{err: errors.New("tts unavailable")}
	var sent []string
	bot := &Bot{speech: speech, sendFunc: func(ch tgbotapi.Chattable) (tgbotapi.Message, error) {
		switch ch.(type) {
		case tgbotapi.MessageConfig:
			sent = append(sent, "text")
		case tgbotapi.AudioConfig:
			sent = append(sent, "audio")
		}
		return tgbotapi.Message{}, nil
	}}

	bot.sendNarration(123, "The door opens.")

	if got, want := sent, []string{"text"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("sent %v, want %v", got, want)
	}
}

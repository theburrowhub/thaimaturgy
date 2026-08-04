package tgbot

import (
	"strings"
	"testing"
	"unicode/utf8"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestSplitMessage(t *testing.T) {
	if got := splitMessage("hello", 100); len(got) != 1 || got[0] != "hello" {
		t.Errorf("short text = %v", got)
	}
	if got := splitMessage("   ", 100); len(got) != 1 || got[0] != "(empty)" {
		t.Errorf("empty text = %v", got)
	}

	long := strings.Repeat("word ", 500) + "\n" + strings.Repeat("more ", 500)
	chunks := splitMessage(long, 200)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if len(c) > 200 {
			t.Errorf("chunk %d exceeds limit: %d", i, len(c))
		}
	}

	// Multibyte content with no newline must not be split mid-rune.
	multibyte := strings.TrimSpace(strings.Repeat("acción-ñoño ", 400))
	mchunks := splitMessage(multibyte, 100)
	for i, c := range mchunks {
		if !utf8.ValidString(c) {
			t.Errorf("chunk %d is not valid UTF-8", i)
		}
	}
	if strings.Join(mchunks, "") != multibyte {
		t.Error("multibyte content changed after split/rejoin")
	}
}

func TestDisplayName(t *testing.T) {
	if got := displayName(&tgbotapi.User{FirstName: "Ana"}); got != "Ana" {
		t.Errorf("first name = %q", got)
	}
	if got := displayName(&tgbotapi.User{UserName: "luis"}); got != "luis" {
		t.Errorf("username fallback = %q", got)
	}
	if got := displayName(&tgbotapi.User{ID: 42}); got != "Player42" {
		t.Errorf("id fallback = %q", got)
	}
}

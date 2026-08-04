package main

import (
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestSplitMessage(t *testing.T) {
	// Short text stays as one chunk.
	if got := splitMessage("hello", 100); len(got) != 1 || got[0] != "hello" {
		t.Errorf("short text = %v", got)
	}
	// Empty text yields a placeholder rather than an empty send.
	if got := splitMessage("   ", 100); len(got) != 1 || got[0] != "(empty)" {
		t.Errorf("empty text = %v", got)
	}
	// Long text splits into chunks each within the limit, preserving content.
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
	if !strings.Contains(strings.Join(chunks, " "), "more") {
		t.Error("content lost during split")
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

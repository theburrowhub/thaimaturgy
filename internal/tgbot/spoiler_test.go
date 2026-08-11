package tgbot

import (
	"strings"
	"testing"
)

func TestHTMLEscape(t *testing.T) {
	if got := htmlEscape("a < b & c > d"); got != "a &lt; b &amp; c &gt; d" {
		t.Errorf("htmlEscape = %q", got)
	}
}

func TestSpoilerMessagesSingle(t *testing.T) {
	msgs := spoilerMessages("Possible actions:", "Open the <door>\nLight a torch", 3500)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	m := msgs[0]
	if !strings.HasPrefix(m, "<b>Possible actions:</b>\n") {
		t.Errorf("heading not bold-prefixed: %q", m)
	}
	if !strings.Contains(m, "<tg-spoiler>") || !strings.Contains(m, "</tg-spoiler>") {
		t.Errorf("body not spoiler-wrapped: %q", m)
	}
	if !strings.Contains(m, "Open the &lt;door&gt;") {
		t.Errorf("body not HTML-escaped: %q", m)
	}
}

func TestSpoilerMessagesChunked(t *testing.T) {
	// A long body splits into several messages, each fully spoiler-wrapped, and
	// only the first carries the heading.
	body := strings.Repeat("action line\n", 600) // > limit
	msgs := spoilerMessages("Possible actions:", body, 1000)
	if len(msgs) < 2 {
		t.Fatalf("expected the body to be chunked, got %d message(s)", len(msgs))
	}
	for i, m := range msgs {
		if !strings.HasPrefix(m, "<tg-spoiler>") && !strings.HasPrefix(m, "<b>") {
			t.Errorf("chunk %d doesn't start with heading/spoiler: %q", i, m[:min(20, len(m))])
		}
		if strings.Count(m, "<tg-spoiler>") != 1 || strings.Count(m, "</tg-spoiler>") != 1 {
			t.Errorf("chunk %d must contain exactly one spoiler pair", i)
		}
		if i > 0 && strings.Contains(m, "<b>") {
			t.Errorf("only the first chunk should carry the heading; chunk %d has it", i)
		}
	}
}

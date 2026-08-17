package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSetWorldDescriptionSanitizeAndClear(t *testing.T) {
	st := NewSessionState("s", &Adventure{ID: "a", Title: "A"})

	// Control chars stripped, newlines preserved (paragraphs survive).
	clean, set := st.SetWorldDescription("room:r1", "Line one\x07\nLine two")
	if !set || clean != "Line one\nLine two" {
		t.Fatalf("sanitize = %q set=%v", clean, set)
	}
	if st.WorldDescription("room:r1") != "Line one\nLine two" {
		t.Errorf("stored = %q", st.WorldDescription("room:r1"))
	}

	// Length cap by runes.
	long := strings.Repeat("é", maxWorldDescriptionLen+50)
	clean, _ = st.SetWorldDescription("room:r2", long)
	if r := []rune(clean); len(r) > maxWorldDescriptionLen+1 { // +1 for the ellipsis
		t.Errorf("not capped: %d runes", len(r))
	}

	// Blank clears.
	if _, set := st.SetWorldDescription("room:r1", "  \n "); set {
		t.Error("blank should clear")
	}
	if st.WorldDescription("room:r1") != "" {
		t.Error("override should be gone")
	}
}

func TestWorldDescriptionRoundTrip(t *testing.T) {
	st := NewSessionState("s", &Adventure{ID: "a", Title: "A"})
	st.SetWorldDescription("room:altar", "scorched and empty")
	data, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back SessionState
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.WorldDescription("room:altar") != "scorched and empty" {
		t.Errorf("world description lost across round-trip: %q", back.WorldDescription("room:altar"))
	}
}

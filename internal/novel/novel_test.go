package novel

import (
	"strings"
	"testing"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

func ts(sec int) time.Time {
	return time.Date(2026, 1, 1, 0, 0, sec, 0, time.UTC)
}

// collectBeats must interleave log entries and conversation messages into one
// timeline ordered by timestamp, regardless of which slice they came from.
func TestCollectBeatsInterleavesByTimestamp(t *testing.T) {
	adv := &domain.Adventure{}
	st := &domain.SessionState{
		Log: &domain.SessionLog{Entries: []domain.LogEntry{
			{Type: domain.LogLocation, Message: "the crypt", Timestamp: ts(1)},
			{Type: domain.LogNote, Message: "a trap springs", Timestamp: ts(3)},
		}},
		Conversation: &domain.Conversation{Messages: []domain.Message{
			{Role: domain.RoleUser, Content: "we open the door", Timestamp: ts(2)},
			{Role: domain.RoleAssistant, Content: "the hinges groan", Timestamp: ts(4)},
		}},
	}

	beats := collectBeats(adv, st)
	if len(beats) != 4 {
		t.Fatalf("expected 4 beats, got %d", len(beats))
	}
	// Ordered by timestamp: crypt(1) → open door(2) → trap(3) → hinges(4).
	want := []string{"crypt", "open the door", "trap springs", "hinges groan"}
	for i, w := range want {
		if !strings.Contains(beats[i].text, w) {
			t.Errorf("beat %d = %q, want it to contain %q", i, beats[i].text, w)
		}
	}
	if !beats[0].scene {
		t.Errorf("location beat should be a scene boundary")
	}
}

// Mechanics (rolls, party bookkeeping, flags) must never reach the narrative.
func TestCollectBeatsDropsMechanics(t *testing.T) {
	adv := &domain.Adventure{}
	st := &domain.SessionState{
		Log: &domain.SessionLog{Entries: []domain.LogEntry{
			{Type: domain.LogRoll, Message: "rolled 17", Timestamp: ts(1)},
			{Type: domain.LogParty, Message: "hp -3", Timestamp: ts(2)},
			{Type: domain.LogFlag, Message: "door_open=true", Timestamp: ts(3)},
			{Type: domain.LogNote, Message: "keep this", Timestamp: ts(4)},
		}},
	}
	beats := collectBeats(adv, st)
	if len(beats) != 1 {
		t.Fatalf("expected only the note to survive, got %d beats", len(beats))
	}
	if !strings.Contains(beats[0].text, "keep this") {
		t.Errorf("unexpected surviving beat: %q", beats[0].text)
	}
}

// segmentBeats should split near the budget and prefer scene boundaries.
func TestSegmentBeatsSplitsOnSceneAndBudget(t *testing.T) {
	mk := func(scene bool, n int) beat {
		return beat{text: strings.Repeat("x", n), scene: scene}
	}
	// Budget 100, soft break at 60. Two scenes of ~70 chars each should land in
	// separate segments because the second scene beat arrives past the soft mark.
	beats := []beat{
		{text: "SCENE A", scene: true},
		mk(false, 70),
		{text: "SCENE B", scene: true},
		mk(false, 70),
	}
	segs := segmentBeats(beats, 100)
	if len(segs) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segs))
	}
	if !strings.Contains(segmentDigest(segs[1]), "SCENE B") {
		t.Errorf("second segment should start at SCENE B, got %q", segmentDigest(segs[1]))
	}
}

func TestStripBookTitle(t *testing.T) {
	in := "# The Book\n\n## Chapter Three\nProse continues."
	out := stripBookTitle(in)
	if strings.Contains(out, "# The Book") {
		t.Errorf("title not stripped: %q", out)
	}
	if !strings.HasPrefix(out, "## Chapter Three") {
		t.Errorf("continuation should start at the chapter heading, got %q", out)
	}
	// A segment with no leading title must be left untouched.
	noTitle := "## Chapter Four\nMore prose."
	if got := stripBookTitle(noTitle); got != noTitle {
		t.Errorf("no-title segment changed: %q", got)
	}
}

func TestNormalizeChapters(t *testing.T) {
	in := strings.Join([]string{
		"# La Casa de la Muerte",
		"",
		"## I. El camino",
		"Prosa uno.",
		"## VII",
		"Prosa dos.",
		"## Capítulo IX",
		"Prosa tres.",
		"## La escalera que no debería existir",
		"Prosa cuatro.",
		"## 27. El Salón Comedor — dos amenazas",
		"Prosa cinco.",
		"## ",
		"Prosa seis (sin encabezado).",
		"### Una subsección",
		"Prosa siete.",
	}, "\n")

	got := NormalizeChapters(in)

	// Chapters renumbered 1..5; the empty heading is dropped (so 6 headings → 5).
	want := []string{
		"# La Casa de la Muerte", // book title untouched
		"## 1. El camino",        // roman "I." stripped, title kept
		"## 2",                   // bare roman "VII" → untitled number
		"## 3",                   // "Capítulo IX" → untitled number
		"## 4. La escalera que no debería existir", // plain title kept whole
		"## 5. El Salón Comedor — dos amenazas",    // "27." stripped
		"### Una subsección",                       // subheading untouched
	}
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("normalized output missing %q\n---\n%s", w, got)
		}
	}
	if strings.Contains(got, "## 6") || strings.Contains(got, "## 7") {
		t.Errorf("empty/sub headings should not be numbered as chapters:\n%s", got)
	}
	// The dropped empty heading must not leave a "## " with no title.
	for _, ln := range strings.Split(got, "\n") {
		if strings.TrimSpace(ln) == "##" {
			t.Errorf("found a bare '##' heading after normalization")
		}
	}
	if strings.Contains(got, "\n\n\n") {
		t.Errorf("runs of blank lines were not collapsed")
	}
}

func TestLastCharsRuneSafe(t *testing.T) {
	s := strings.Repeat("é", 10) // 2 bytes each = 20 bytes
	got := lastChars(s, 5)
	if !utf8ValidString(got) {
		t.Errorf("lastChars split a multibyte rune: %q", got)
	}
}

// utf8ValidString is a tiny local check to avoid importing unicode/utf8 twice.
func utf8ValidString(s string) bool {
	for _, r := range s {
		if r == '�' {
			return false
		}
	}
	return true
}

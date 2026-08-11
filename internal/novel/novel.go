// Package novel turns a played session into a prose novelization of the
// adventure — a book to print, bind and read — and renders it to Markdown or PDF.
//
// A whole adventure does not fit in a single model call (neither its play log as
// input nor the finished book as output), so Generate works in a loop: it merges
// the play log and the table narration into one time-ordered timeline, splits it
// into scene-aligned segments, and writes the book one segment at a time. Each
// pass carries a compact rolling synopsis of the story so far plus the tail of the
// previous prose, so the narrative stays continuous while total input and output
// scale with the number of segments — not with a single context window.
package novel

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
)

const (
	// defaultSegmentChars is the target size (digest characters) of one generation
	// segment. Each segment becomes one loop pass, so this trades call count for
	// context size: smaller → more passes, larger → fewer but heavier passes.
	defaultSegmentChars = 40000
	// maxSegmentTokens caps each segment's generated length (a chapter or two).
	maxSegmentTokens = 20000
	// summaryMaxTokens caps the rolling synopsis kept between passes.
	summaryMaxTokens = 1500
	// tailChars is how much of the previous prose is echoed to the next pass so it
	// can pick up mid-scene tone and voice.
	tailChars = 1600
)

// Options tunes the segmented generation. The zero value uses sane defaults.
type Options struct {
	// SegmentChars overrides the per-segment digest budget (0 → default).
	SegmentChars int
	// MaxSegmentTokens overrides the per-segment output cap (0 → default).
	MaxSegmentTokens int
	// Progress, if set, is called once before each segment with (n, total),
	// n being 1-based. Useful for a console progress line.
	Progress func(n, total int)
}

// Generate writes a prose novelization (GitHub-flavored Markdown) of what
// happened in the session, grounded in the adventure's authored text. It uses the
// provider's plain text generation (no tools), so it works on every backend.
func Generate(ctx context.Context, prov providers.Provider, model string, adv *domain.Adventure, st *domain.SessionState) (string, error) {
	return GenerateWithOptions(ctx, prov, model, adv, st, Options{})
}

// GenerateWithOptions is Generate with tunable segmentation and a progress hook.
func GenerateWithOptions(ctx context.Context, prov providers.Provider, model string, adv *domain.Adventure, st *domain.SessionState, opt Options) (string, error) {
	if prov == nil {
		return "", fmt.Errorf("no AI provider configured")
	}
	budget := opt.SegmentChars
	if budget <= 0 {
		budget = defaultSegmentChars
	}
	maxTokens := opt.MaxSegmentTokens
	if maxTokens <= 0 {
		maxTokens = maxSegmentTokens
	}

	lang := languageName(adv.Language)
	context0 := storyContext(adv, st)

	beats := collectBeats(adv, st)
	if len(beats) == 0 {
		return "", fmt.Errorf("session has no narratable content yet")
	}
	segments := segmentBeats(beats, budget)

	var book strings.Builder
	var synopsis string // rolling "story so far"
	var tail string     // last chars of the previous prose

	for i, seg := range segments {
		if opt.Progress != nil {
			opt.Progress(i+1, len(segments))
		}
		first := i == 0
		last := i == len(segments)-1
		prose, err := generateSegment(ctx, prov, model, segParams{
			lang:       lang,
			context0:   context0,
			digest:     segmentDigest(seg),
			synopsis:   synopsis,
			tail:       tail,
			first:      first,
			last:       last,
			chaptersSo: strings.Count(book.String(), "\n## ") + boolToInt(strings.HasPrefix(book.String(), "## ")),
			maxTokens:  maxTokens,
		})
		if err != nil {
			return "", fmt.Errorf("segment %d/%d: %w", i+1, len(segments), err)
		}
		prose = cleanMarkdown(prose)
		if !first {
			prose = stripBookTitle(prose)
		}
		if book.Len() > 0 {
			book.WriteString("\n\n")
		}
		book.WriteString(prose)

		// Prepare continuity for the next pass (skip the extra call after the last).
		if !last {
			tail = lastChars(prose, tailChars)
			if s, err := updateSynopsis(ctx, prov, model, lang, synopsis, prose); err != nil {
				// A synopsis hiccup shouldn't sink the whole book; the prose tail
				// still carries local continuity. But honor cancellation/timeout.
				if ctx.Err() != nil {
					return "", ctx.Err()
				}
			} else {
				synopsis = s
			}
		}
	}

	return NormalizeChapters(cleanMarkdown(book.String())), nil
}

// chapterEnumRe matches a leading chapter enumerator the model may have emitted
// (an optional "Capítulo"/"Chapter" word plus a roman or arabic number and its
// separator), so it can be stripped before we renumber consistently. The number
// must be followed by a boundary — punctuation, whitespace or end — so a title
// that merely starts with roman-looking letters ("La escalera", "Ivan") is kept.
var chapterEnumRe = regexp.MustCompile(`(?i)^(?:cap[íi]tulo|chapter)?\s*([ivxlcdm]+|\d+)(?:\s*[.\-—:]+\s*|\s+|$)`)

// multiBlankRe collapses runs of blank lines left behind when a spurious empty
// heading is dropped.
var multiBlankRe = regexp.MustCompile(`\n{3,}`)

// NormalizeChapters renumbers "## " chapter headings sequentially (1, 2, 3…),
// stripping whatever inconsistent enumeration the model produced across passes
// (roman, arabic, "Capítulo N", or none) while preserving any descriptive title,
// and drops empty "## " headings. The book title ("# ") and subheadings ("### ")
// are left untouched. It is applied automatically at the end of generation and is
// exported so an already-generated Markdown file can be normalized in place.
func NormalizeChapters(md string) string {
	lines := strings.Split(md, "\n")
	out := make([]string, 0, len(lines))
	n := 0
	for _, ln := range lines {
		if strings.HasPrefix(ln, "## ") && !strings.HasPrefix(ln, "### ") {
			body := strings.TrimSpace(ln[3:])
			if body == "" {
				continue // spurious empty heading → drop it
			}
			title := stripChapterEnumerator(body)
			n++
			if title != "" {
				out = append(out, fmt.Sprintf("## %d. %s", n, title))
			} else {
				out = append(out, fmt.Sprintf("## %d", n))
			}
			continue
		}
		out = append(out, ln)
	}
	return multiBlankRe.ReplaceAllString(strings.Join(out, "\n"), "\n\n")
}

// stripChapterEnumerator removes a leading chapter enumerator, returning the
// remaining title (empty if the heading was only a number).
func stripChapterEnumerator(body string) string {
	if loc := chapterEnumRe.FindStringIndex(body); loc != nil {
		return strings.TrimSpace(body[loc[1]:])
	}
	return strings.TrimSpace(body)
}

// segParams bundles the inputs for one segment generation pass.
type segParams struct {
	lang       string
	context0   string
	digest     string
	synopsis   string
	tail       string
	first      bool
	last       bool
	chaptersSo int
	maxTokens  int
}

func generateSegment(ctx context.Context, prov providers.Provider, model string, p segParams) (string, error) {
	sys := fmt.Sprintf(`You are a skilled novelist writing ONE continuous immersive prose NOVEL from a tabletop RPG (D&D) play session — a book a reader could print, bind and enjoy, not a game log.

You write the book in successive passes. Each pass you receive the story so far (a synopsis and the tail of your previous prose) and the NEXT beats to dramatize. Continue the SAME novel: consistent characters, names, places, tone and continuity. Never restart the story and never recap what you already wrote.

Rules:
- Write entirely in %s.
- Third person, past tense, literary but readable. Show, don't tell.
- Follow the ACTUAL sequence of events and the party's real choices from the beats; stay faithful to the adventure's atmosphere and scene descriptions.
- Dramatize scenes: dialogue, sensory detail, tension. Turn DM notes and oracle exchanges into narrative; NEVER include game mechanics, stat blocks, dice rolls, DCs, flags, or DM meta-commentary.
- Use "## " for chapter headings. %s
- Output ONLY Markdown prose (no code fences, no commentary before or after).`,
		p.lang, titleRule(p.first))

	var user strings.Builder
	user.WriteString(p.context0)
	user.WriteString("\n")
	if p.first {
		user.WriteString("\nThis is the OPENING of the novel.\n")
	} else {
		fmt.Fprintf(&user, "\nThis is a CONTINUATION (you have written about %d chapter(s) so far). Do NOT add a title; continue seamlessly.\n", p.chaptersSo)
		if strings.TrimSpace(p.synopsis) != "" {
			user.WriteString("\nSTORY SO FAR (already narrated — do NOT repeat, continue from here):\n")
			user.WriteString(strings.TrimSpace(p.synopsis))
			user.WriteString("\n")
		}
		if strings.TrimSpace(p.tail) != "" {
			user.WriteString("\nLAST LINES OF YOUR PREVIOUS TEXT (continue seamlessly from this moment and tone):\n")
			user.WriteString(strings.TrimSpace(p.tail))
			user.WriteString("\n")
		}
	}
	user.WriteString("\n=== NEXT SESSION BEATS TO DRAMATIZE (in order) ===\n")
	user.WriteString(p.digest)
	if p.last {
		user.WriteString("\nThis is the FINAL part — bring the story to a satisfying close.\n")
	}
	user.WriteString("\nNow write the next part of the novel.")

	resp, err := prov.Chat(ctx, providers.ChatRequest{
		Model:     model,
		MaxTokens: p.maxTokens,
		Messages: []providers.Message{
			{Role: providers.RoleSystem, Content: sys},
			{Role: providers.RoleUser, Content: user.String()},
		},
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// updateSynopsis folds the newly written prose into a compact running synopsis
// that keeps the next pass coherent without re-sending the whole book.
func updateSynopsis(ctx context.Context, prov providers.Provider, model, lang, prev, newProse string) (string, error) {
	sys := fmt.Sprintf(`You maintain a running synopsis of a novel written in %s. Given the previous synopsis and the newly written chapters, output an UPDATED concise synopsis (about 250-400 words) that captures: the plot so far, characters introduced and their current state, unresolved threads, and the party's current location and mood. Write it as plain prose. Output ONLY the synopsis.`, lang)
	prevText := strings.TrimSpace(prev)
	if prevText == "" {
		prevText = "(none yet — this is the beginning)"
	}
	user := "PREVIOUS SYNOPSIS:\n" + prevText + "\n\nNEWLY WRITTEN CHAPTERS:\n" + newProse + "\n\nUpdated synopsis:"
	resp, err := prov.Chat(ctx, providers.ChatRequest{
		Model:     model,
		MaxTokens: summaryMaxTokens,
		Messages: []providers.Message{
			{Role: providers.RoleSystem, Content: sys},
			{Role: providers.RoleUser, Content: user},
		},
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Content), nil
}

func titleRule(first bool) string {
	if first {
		return `Begin the book with a single "# " title line, then chapters.`
	}
	return `Do NOT output any "# " (single-hash) title line; only "## " chapter headings.`
}

// beat is one atomic story moment on the merged timeline.
type beat struct {
	ts    time.Time
	text  string
	scene bool // a location change — the natural place to start a new segment
}

// collectBeats merges the play log and the table narration into a single
// time-ordered list, dropping pure game mechanics (rolls, party bookkeeping, flags).
func collectBeats(adv *domain.Adventure, st *domain.SessionState) []beat {
	var beats []beat
	if st.Log != nil {
		for _, e := range st.Log.Entries {
			if txt, scene, ok := renderLogBeat(adv, e); ok {
				beats = append(beats, beat{ts: e.Timestamp, text: txt, scene: scene})
			}
		}
	}
	if st.Conversation != nil {
		for _, m := range st.Conversation.Messages {
			switch m.Role {
			case domain.RoleUser:
				if c := oneLine(m.Content); c != "" {
					beats = append(beats, beat{ts: m.Timestamp, text: "- DM: " + c})
				}
			case domain.RoleAssistant:
				if c := oneLine(m.Content); c != "" {
					beats = append(beats, beat{ts: m.Timestamp, text: "- Narration: " + c})
				}
			}
		}
	}
	sort.SliceStable(beats, func(i, j int) bool { return beats[i].ts.Before(beats[j].ts) })
	return beats
}

// renderLogBeat turns one authored-content log entry into a digest block. The
// bool return reports whether it's a scene boundary; ok is false for mechanics.
func renderLogBeat(adv *domain.Adventure, e domain.LogEntry) (text string, scene bool, ok bool) {
	switch e.Type {
	case domain.LogLocation:
		id, _ := e.Data["room"].(string)
		if r, _ := adv.Room(id); r != nil {
			var b strings.Builder
			fmt.Fprintf(&b, "• SCENE — %s", r.Name)
			if r.ReadAloud != "" {
				fmt.Fprintf(&b, "\n  Scene text: %s", oneLine(r.ReadAloud))
			}
			if r.DMNotes != "" {
				fmt.Fprintf(&b, "\n  What happens here: %s", oneLine(r.DMNotes))
			}
			return b.String(), true, true
		}
		return "• " + e.Message, true, e.Message != ""
	case domain.LogNPC:
		id, _ := e.Data["npc"].(string)
		if n := adv.NPC(id); n != nil {
			var b strings.Builder
			fmt.Fprintf(&b, "• CHARACTER — %s", n.Name)
			if n.Role != "" {
				fmt.Fprintf(&b, " (%s)", n.Role)
			}
			if n.Appearance != "" {
				fmt.Fprintf(&b, "\n  Appearance: %s", oneLine(n.Appearance))
			}
			if n.Personality != "" {
				fmt.Fprintf(&b, "\n  Personality: %s", oneLine(n.Personality))
			}
			return b.String(), false, true
		}
		return "• " + e.Message, false, e.Message != ""
	case domain.LogEvent:
		id, _ := e.Data["event"].(string)
		if ev := adv.Event(id); ev != nil {
			text := ev.ReadAloud
			if text == "" {
				text = ev.Description
			}
			out := "• EVENT — " + ev.Name
			if text != "" {
				out += "\n  " + oneLine(text)
			}
			return out, false, true
		}
		return "• " + e.Message, false, e.Message != ""
	case domain.LogNote:
		return "• DM NOTE: " + oneLine(e.Message), false, e.Message != ""
	case domain.LogQuest:
		return "• " + e.Message, false, e.Message != ""
	}
	return "", false, false
}

// segmentBeats splits the timeline into passes of roughly budget characters,
// preferring to break at a scene boundary once a segment is past ~60% full so
// chapters align with location changes.
func segmentBeats(beats []beat, budget int) [][]beat {
	soft := budget * 6 / 10
	var segs [][]beat
	var cur []beat
	curLen := 0
	for _, b := range beats {
		bl := len(b.text) + 1
		if len(cur) > 0 && (curLen+bl > budget || (b.scene && curLen >= soft)) {
			segs = append(segs, cur)
			cur = nil
			curLen = 0
		}
		cur = append(cur, b)
		curLen += bl
	}
	if len(cur) > 0 {
		segs = append(segs, cur)
	}
	return segs
}

func segmentDigest(seg []beat) string {
	var sb strings.Builder
	for _, b := range seg {
		sb.WriteString(b.text)
		sb.WriteString("\n")
	}
	return sb.String()
}

// storyContext is the grounding header (title, premise, party) sent with every pass.
func storyContext(adv *domain.Adventure, st *domain.SessionState) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "ADVENTURE: %s\n", adv.Title)
	if adv.Summary != "" {
		fmt.Fprintf(&sb, "PREMISE: %s\n", adv.Summary)
	}
	if len(st.Party) > 0 {
		var names []string
		for _, p := range st.Party {
			label := p.Name
			if p.Class != "" {
				label += " (" + p.Class + ")"
			}
			names = append(names, label)
		}
		fmt.Fprintf(&sb, "PARTY: %s\n", strings.Join(names, ", "))
	}
	return sb.String()
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func lastChars(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	// Cut on a rune boundary so we don't split a multibyte character.
	cut := s[len(s)-n:]
	for i := 0; i < len(cut) && i < 4; i++ {
		if cut[0]&0xC0 != 0x80 { // not a UTF-8 continuation byte
			break
		}
		cut = cut[1:]
	}
	return cut
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func languageName(code string) string {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "es", "spanish", "español", "espanol", "castellano":
		return "Spanish"
	case "", "en", "english":
		return "English"
	}
	return code
}

// cleanMarkdown strips accidental wrapping code fences the model may add.
func cleanMarkdown(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			s = s[i+1:]
		}
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	}
	return strings.TrimSpace(s)
}

// stripBookTitle removes a leading "# " book-title line from a continuation
// segment, so the finished book keeps a single title from the opening pass.
func stripBookTitle(s string) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		if strings.HasPrefix(ln, "# ") {
			return strings.TrimSpace(strings.Join(lines[i+1:], "\n"))
		}
		break // first non-blank line isn't a title → nothing to strip
	}
	return s
}

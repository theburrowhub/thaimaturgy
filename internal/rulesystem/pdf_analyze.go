package rulesystem

import (
	"regexp"
	"strings"
)

// DetectFamily guesses which built-in family best matches extracted PDF text.
func DetectFamily(text string) string {
	lower := strings.ToLower(text)
	scores := map[string]int{
		"dnd5e":         scoreKeywords(lower, dnd5eKeywords),
		"d100":          scoreKeywords(lower, d100Keywords),
		"savage_worlds": scoreKeywords(lower, savageKeywords),
	}
	best, bestScore := "dnd5e", 0
	for id, s := range scores {
		if s > bestScore {
			best, bestScore = id, s
		}
	}
	if bestScore == 0 {
		return "dnd5e"
	}
	return best
}

func scoreKeywords(text string, kws []string) int {
	n := 0
	for _, kw := range kws {
		if strings.Contains(text, kw) {
			n++
		}
	}
	return n
}

var dnd5eKeywords = []string{
	"armor class", "proficiency bonus", "spell slot", "saving throw", "hit dice",
	"dungeons & dragons", "d&d 5", "advantage", "disadvantage", "death saving throw",
}

var d100Keywords = []string{
	"d100", "percentile", "call of cthulhu", "basic roleplaying", "brp",
	"sanity", "magic points", "characteristic", "roll under", "major wound",
}

var savageKeywords = []string{
	"savage worlds", "swade", "wild die", "bennies", "raise", "shaken",
	"power points", "edges", "hindrances", "action cards",
}

var headingRe = regexp.MustCompile(`(?m)^(?:=== Page \d+ ===|[A-Z][A-Za-z0-9 ,'-]{3,})$`)

// ExtractExcerpts pulls likely rules sections from ingest-style PDF text.
func ExtractExcerpts(text string, max int) []SourceExcerpt {
	if max <= 0 {
		max = 12
	}
	lines := strings.Split(text, "\n")
	var out []SourceExcerpt
	var buf strings.Builder
	page := 0
	flush := func(heading string) {
		body := strings.TrimSpace(buf.String())
		buf.Reset()
		if len(body) < 80 {
			return
		}
		if len(body) > 1200 {
			body = body[:1200] + "…"
		}
		out = append(out, SourceExcerpt{Page: page, Heading: heading, Text: body})
	}

	heading := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "=== Page ") {
			if m := regexp.MustCompile(`Page (\d+)`).FindStringSubmatch(line); len(m) == 2 {
				page = atoi(m[1])
			}
			continue
		}
		if line == "" {
			continue
		}
		if isLikelyHeading(line) {
			if buf.Len() > 0 {
				flush(heading)
			}
			heading = line
			if len(out) >= max {
				break
			}
			continue
		}
		if isRulesLine(line) {
			if buf.Len() > 0 {
				buf.WriteByte('\n')
			}
			buf.WriteString(line)
		}
	}
	if buf.Len() > 0 && len(out) < max {
		flush(heading)
	}
	return out
}

func isLikelyHeading(line string) bool {
	if len(line) > 80 {
		return false
	}
	if headingRe.MatchString(line) {
		return true
	}
	lower := strings.ToLower(line)
	for _, h := range []string{"combat", "magic", "skills", "character", "attributes", "spells", "powers", "sanity", "initiative"} {
		if strings.Contains(lower, h) && len(line) < 40 {
			return true
		}
	}
	return false
}

func isRulesLine(line string) bool {
	lower := strings.ToLower(line)
	for _, kw := range []string{"roll", "check", "damage", "attack", "dc", "skill", "spell", "power", "wound", "hp", "ac", "trait", "d100", "d20", "wild"} {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			n = n*10 + int(r-'0')
		}
	}
	return n
}

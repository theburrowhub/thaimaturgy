package rulesystem

import (
	"regexp"
	"strings"
	"unicode"
)

var categoryKeywords = map[string][]string{
	"combat":    {"attack", "damage", "initiative", "armor class", "hit points", "critical", "weapon", "cover"},
	"magic":     {"spell", "casting", "concentration", "slot", "arcane", "divine", "ritual", "components"},
	"skills":    {"skill", "check", "dc", "difficulty", "proficiency", "exploration", "stealth", "investigation"},
	"character": {"ability", "attribute", "level", "experience", "background", "race", "class", "creation"},
}

var wordRe = regexp.MustCompile(`[a-z][a-z0-9_-]*`)

// AnalyzeExcerpt categorizes a text excerpt, extracts keywords, and scores confidence.
func AnalyzeExcerpt(text string) SourceExcerpt {
	text = strings.TrimSpace(text)
	ex := SourceExcerpt{Text: text}
	if text == "" {
		return ex
	}
	lower := strings.ToLower(text)
	scores := map[string]float64{}
	for cat, kws := range categoryKeywords {
		for _, kw := range kws {
			if strings.Contains(lower, kw) {
				scores[cat] += 1
			}
		}
	}
	bestCat := "general"
	bestScore := 0.0
	for cat, score := range scores {
		if score > bestScore {
			bestScore = score
			bestCat = cat
		}
	}
	ex.Category = bestCat
	ex.Keywords = extractKeywords(lower, 12)
	ex.Confidence = scoreConfidence(len(text), bestScore, len(ex.Keywords))
	ex.Heading = inferHeading(text)
	return ex
}

// AnalyzeExcerpts processes multiple raw text blocks.
func AnalyzeExcerpts(blocks []string) []SourceExcerpt {
	out := make([]SourceExcerpt, 0, len(blocks))
	for _, b := range blocks {
		out = append(out, AnalyzeExcerpt(b))
	}
	return out
}

func extractKeywords(text string, limit int) []string {
	words := wordRe.FindAllString(text, -1)
	freq := map[string]int{}
	stop := map[string]struct{}{
		"the": {}, "and": {}, "for": {}, "with": {}, "that": {}, "this": {}, "from": {},
		"are": {}, "was": {}, "you": {}, "your": {}, "can": {}, "may": {}, "when": {},
	}
	for _, w := range words {
		if len(w) < 3 {
			continue
		}
		if _, skip := stop[w]; skip {
			continue
		}
		freq[w]++
	}
	type kv struct {
		k string
		v int
	}
 ranked := make([]kv, 0, len(freq))
	for k, v := range freq {
		ranked = append(ranked, kv{k, v})
	}
	for i := 0; i < len(ranked); i++ {
		for j := i + 1; j < len(ranked); j++ {
			if ranked[j].v > ranked[i].v {
				ranked[i], ranked[j] = ranked[j], ranked[i]
			}
		}
	}
	out := make([]string, 0, limit)
	for _, item := range ranked {
		out = append(out, item.k)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func scoreConfidence(textLen int, categoryHits float64, keywordCount int) float64 {
	base := 0.35
	if textLen > 80 {
		base += 0.15
	}
	if textLen > 300 {
		base += 0.1
	}
	base += categoryHits * 0.08
	base += float64(keywordCount) * 0.02
	if base > 0.95 {
		return 0.95
	}
	return base
}

func inferHeading(text string) string {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len(line) <= 80 && (strings.HasSuffix(line, ":") || isTitleCase(line)) {
			return strings.TrimSuffix(line, ":")
		}
		break
	}
	if len(text) > 60 {
		return text[:57] + "..."
	}
	return text
}

func isTitleCase(s string) bool {
	runes := []rune(s)
	if len(runes) == 0 {
		return false
	}
	if !unicode.IsUpper(runes[0]) {
		return false
	}
	upper := 0
	for _, r := range runes {
		if unicode.IsUpper(r) {
			upper++
		}
	}
	return upper >= 2
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

// DetectFamily guesses which built-in family best matches extracted PDF text.
func DetectFamily(text string) string {
	lower := strings.ToLower(text)
	scores := map[string]int{
		"dnd5e":         scoreFamilyKeywords(lower, dnd5eKeywords),
		"d100":          scoreFamilyKeywords(lower, d100Keywords),
		"savage_worlds": scoreFamilyKeywords(lower, savageKeywords),
	}
	best, bestScore := "dnd5e", 0
	for id, s := range scores {
		if s > bestScore {
			best, bestScore = id, s
		}
	}
	return best
}

func scoreFamilyKeywords(text string, kws []string) int {
	n := 0
	for _, kw := range kws {
		if strings.Contains(text, kw) {
			n++
		}
	}
	return n
}

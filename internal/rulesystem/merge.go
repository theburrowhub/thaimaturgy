package rulesystem

import (
	"fmt"
	"strings"
)
func MergePDFExcerpts(p *Pack, excerpts []SourceExcerpt) (*Pack, error) {
	if p == nil {
		return nil, fmt.Errorf("nil pack")
	}
	out, err := clonePack(p)
	if err != nil {
		return nil, err
	}
	if len(excerpts) == 0 {
		return out, nil
	}
	chapterIndex := map[string]int{}
	for i, ch := range out.Chapters {
		chapterIndex[ch.ID] = i
	}
	categoryToChapter := map[string]string{
		"combat":    "combat",
		"magic":     "magic",
		"skills":    "exploration",
		"character": "character",
		"general":   "general",
	}
	for _, ex := range excerpts {
		cat := strings.ToLower(strings.TrimSpace(ex.Category))
		if cat == "" {
			cat = "general"
		}
		chID := categoryToChapter[cat]
		if chID == "" {
			chID = cat
		}
		idx, ok := chapterIndex[chID]
		if !ok {
			out.Chapters = append(out.Chapters, RuleChapter{
				ID:      chID,
				Title:   titleForCategory(cat),
				Summary: "Imported from source document excerpts.",
				Tags:    []string{cat, "imported"},
			})
			idx = len(out.Chapters) - 1
			chapterIndex[chID] = idx
		}
		secID := toID(ex.Heading)
		if secID == "" {
			secID = fmt.Sprintf("page_%d", ex.Page)
		}
		body := ex.Text
		if ex.Page > 0 {
			body = fmt.Sprintf("[p.%d] %s", ex.Page, body)
		}
		out.Chapters[idx].Sections = append(out.Chapters[idx].Sections, Section{
			ID:    secID,
			Title: ex.Heading,
			Body:  body,
			Bullets: ex.Keywords,
		})
		out.RawExcerpts = append(out.RawExcerpts, ex)
	}
	return out, nil
}

func titleForCategory(cat string) string {
	switch cat {
	case "combat":
		return "Combat (Imported)"
	case "magic":
		return "Magic (Imported)"
	case "skills":
		return "Skills & Exploration (Imported)"
	case "character":
		return "Character (Imported)"
	default:
		if cat == "" {
			return "General"
		}
		return strings.ToUpper(cat[:1]) + strings.ReplaceAll(cat[1:], "_", " ")
	}
}

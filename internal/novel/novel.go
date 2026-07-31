// Package novel turns a played session into a prose novelization of the
// adventure — a book to print, bind and read — and renders it to Markdown or PDF.
package novel

import (
	"context"
	"fmt"
	"strings"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
)

const maxDigestChars = 60000

// Generate writes a prose novelization (GitHub-flavored Markdown) of what
// happened in the session, grounded in the adventure's authored text. It uses the
// provider's plain text generation (no tools), so it works on every backend.
func Generate(ctx context.Context, prov providers.Provider, model string, adv *domain.Adventure, st *domain.SessionState) (string, error) {
	if prov == nil {
		return "", fmt.Errorf("no AI provider configured")
	}
	lang := languageName(adv.Language)
	digest := buildDigest(adv, st)

	sys := fmt.Sprintf(`You are a skilled novelist. Turn the following tabletop RPG (D&D) play session into an immersive prose NOVEL a reader could print, bind and enjoy — not a game log.

Rules:
- Write entirely in %s.
- Third person, past tense, literary but readable. Show, don't tell.
- Follow the ACTUAL sequence of events and the party's real choices from the session beats below; stay faithful to the adventure's atmosphere and the scene descriptions.
- Dramatize scenes: dialogue, sensory detail, tension. Turn the DM's notes and the oracle exchanges into narrative; NEVER include game mechanics, stat blocks, dice rolls, DCs, flags, or DM meta-commentary.
- Structure it as a book: a single "# " title line, then chapters as "## " headings. Aim for coherent chapters that follow the journey.
- Output ONLY Markdown (no code fences, no commentary before or after).`, lang)

	resp, err := prov.Chat(ctx, providers.ChatRequest{
		Model:     model,
		MaxTokens: 64000,
		Messages: []providers.Message{
			{Role: providers.RoleSystem, Content: sys},
			{Role: providers.RoleUser, Content: digest},
		},
	})
	if err != nil {
		return "", err
	}
	return cleanMarkdown(resp.Content), nil
}

// buildDigest reconstructs the session as an ordered list of beats, enriching each
// timeline entry with the relevant authored text, followed by the oracle dialogue.
func buildDigest(adv *domain.Adventure, st *domain.SessionState) string {
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

	sb.WriteString("\n=== SESSION BEATS (in order) ===\n")
	if st.Log != nil {
		for _, e := range st.Log.Entries {
			writeBeat(&sb, adv, e)
		}
	}

	if st.Conversation != nil && len(st.Conversation.Messages) > 0 {
		sb.WriteString("\n=== TABLE NARRATION (DM ↔ oracle; source material, not to quote verbatim) ===\n")
		for _, m := range st.Conversation.Messages {
			switch m.Role {
			case domain.RoleUser:
				fmt.Fprintf(&sb, "- DM: %s\n", oneLine(m.Content))
			case domain.RoleAssistant:
				if strings.TrimSpace(m.Content) != "" {
					fmt.Fprintf(&sb, "- Narration: %s\n", oneLine(m.Content))
				}
			}
		}
	}

	sb.WriteString("\nNow write the novel.")
	return truncate(sb.String(), maxDigestChars)
}

func writeBeat(sb *strings.Builder, adv *domain.Adventure, e domain.LogEntry) {
	switch e.Type {
	case domain.LogLocation:
		id, _ := e.Data["room"].(string)
		if r, _ := adv.Room(id); r != nil {
			fmt.Fprintf(sb, "\n• SCENE — %s\n", r.Name)
			if r.ReadAloud != "" {
				fmt.Fprintf(sb, "  Scene text: %s\n", oneLine(r.ReadAloud))
			}
			if r.DMNotes != "" {
				fmt.Fprintf(sb, "  What happens here: %s\n", oneLine(r.DMNotes))
			}
			return
		}
		fmt.Fprintf(sb, "\n• %s\n", e.Message)
	case domain.LogNPC:
		id, _ := e.Data["npc"].(string)
		if n := adv.NPC(id); n != nil {
			fmt.Fprintf(sb, "• CHARACTER — %s", n.Name)
			if n.Role != "" {
				fmt.Fprintf(sb, " (%s)", n.Role)
			}
			sb.WriteString("\n")
			if n.Appearance != "" {
				fmt.Fprintf(sb, "  Appearance: %s\n", oneLine(n.Appearance))
			}
			if n.Personality != "" {
				fmt.Fprintf(sb, "  Personality: %s\n", oneLine(n.Personality))
			}
			return
		}
		fmt.Fprintf(sb, "• %s\n", e.Message)
	case domain.LogEvent:
		id, _ := e.Data["event"].(string)
		if ev := adv.Event(id); ev != nil {
			fmt.Fprintf(sb, "• EVENT — %s\n", ev.Name)
			text := ev.ReadAloud
			if text == "" {
				text = ev.Description
			}
			if text != "" {
				fmt.Fprintf(sb, "  %s\n", oneLine(text))
			}
			return
		}
		fmt.Fprintf(sb, "• %s\n", e.Message)
	case domain.LogNote:
		fmt.Fprintf(sb, "• DM NOTE: %s\n", oneLine(e.Message))
	case domain.LogQuest:
		fmt.Fprintf(sb, "• %s\n", e.Message)
	}
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
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

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n…(session truncated)…"
}

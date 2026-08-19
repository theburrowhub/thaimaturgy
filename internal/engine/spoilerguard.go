package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
)

const (
	spoilerReviewTimeout   = 60 * time.Second
	spoilerReviewMaxTokens = 2000
)

// spoilerAuditorSystemPrompt instructs the review pass. The opening phrase is a
// stable marker other code/tests can key on to recognize the auditor call.
const spoilerAuditorSystemPrompt = `You are a spoiler auditor for a tabletop RPG session. A Dungeon Master has written narration meant for the PLAYERS. Your job is to make sure it reveals nothing the players are not supposed to know yet.

You are given: (1) what the players already know, (2) HIDDEN or FUTURE material they must NOT learn from this narration, and (3) the proposed narration.

Rules:
- If the narration discloses any hidden/future fact — a name, event, motivation, secret, or plot point the players haven't discovered yet — rewrite it MINIMALLY: remove or obscure only the leaking part, preserving the DM's voice, tense, and everything the players may legitimately perceive or already know.
- Do NOT add new information, foreshadowing, or hints. Do NOT censor what the players can plainly see or already know.
- If the narration leaks nothing, return it EXACTLY unchanged.
- Output ONLY the final player-facing narration — no preamble, no explanation, no quotes, no code fences.`

// reviewSpoilers is the anti-spoiler pass (#89). In virtual-DM mode, when enabled
// in config, it asks the model to check the DM's player-facing narration against
// the hidden/future material and rewrite it minimally to remove any leak. It is
// best-effort and FAIL-OPEN: when disabled, not in virtual-DM mode, or on any
// error/timeout/truncation, it returns the narration unchanged — a reviewer
// hiccup must never drop or truncate the DM's output.
func (o *Oracle) reviewSpoilers(ctx context.Context, narration string) string {
	cfg := o.session.Config
	if cfg == nil || !cfg.SpoilerGuard.Enabled {
		return narration
	}
	if o.session.State.EffectiveMode() != domain.ModeVirtualDM {
		return narration // assistant mode: the human DM is the audience, show everything
	}
	if o.provider == nil || strings.TrimSpace(narration) == "" {
		return narration
	}
	hidden := o.hiddenContext()
	if strings.TrimSpace(hidden) == "" {
		return narration // nothing secret to leak
	}

	// Choose the review engine + model. By default reuse the oracle's provider and
	// model; if a spoiler-guard provider is configured (and differs), build a
	// provider for that engine reusing the same stored credentials, defaulting the
	// model to that provider's default when none is set.
	prov := o.provider
	model := strings.TrimSpace(cfg.SpoilerGuard.Model)
	if p := cfg.SpoilerGuard.Provider; p != "" && p != cfg.Provider {
		if model == "" {
			model = domain.DefaultModel(p)
		}
		sub := *cfg
		sub.Provider = p
		sub.Model = model
		sub.RunModel = ""
		sub.EditModel = ""
		if pr := providers.New(&sub); pr != nil {
			prov = pr
		}
	} else if model == "" {
		model = cfg.Model
	}
	if prov == nil {
		return narration // fail-open: no usable engine for the review
	}

	var user strings.Builder
	user.WriteString("=== WHAT THE PLAYERS ALREADY KNOW ===\n")
	user.WriteString(o.knownContext())
	user.WriteString("\n=== HIDDEN / FUTURE — must NOT be revealed to the players ===\n")
	user.WriteString(hidden)
	user.WriteString("\n=== PROPOSED NARRATION (shown to the players) ===\n")
	user.WriteString(narration)
	user.WriteString("\n\nReturn the player-safe narration.")

	rctx, cancel := context.WithTimeout(ctx, spoilerReviewTimeout)
	defer cancel()
	resp, err := prov.Chat(rctx, providers.ChatRequest{
		Model:       model,
		Temperature: 0.2,
		MaxTokens:   spoilerReviewMaxTokens,
		Messages: []providers.Message{
			{Role: providers.RoleSystem, Content: spoilerAuditorSystemPrompt},
			{Role: providers.RoleUser, Content: user.String()},
		},
	})
	if err != nil || resp == nil {
		return narration // fail-open
	}
	cleaned := strings.TrimSpace(stripFences(resp.Content))
	// Never ship an empty or cut-off narration; keep the original instead.
	if cleaned == "" || resp.FinishReason == "length" || resp.FinishReason == "max_tokens" {
		return narration
	}
	return cleaned
}

// knownContext summarizes what the players have legitimately learned, so the
// auditor doesn't over-censor things they already know.
func (o *Oracle) knownContext() string {
	adv := o.session.Adventure
	st := o.session.State
	var sb strings.Builder
	if s := strings.TrimSpace(st.Summary); s != "" {
		fmt.Fprintf(&sb, "Story so far: %s\n", oneLineTrim(s))
	}
	if sc := adv.Scene(st.Scene()); sc != nil {
		fmt.Fprintf(&sb, "Current scene: %s\n", nameOrID(sc.Name, sc.ID))
	}
	if room, _ := adv.Room(st.CurrentRoom); room != nil {
		fmt.Fprintf(&sb, "Current location: %s\n", room.Name)
	}
	if names := metNPCLabels(adv, st); len(names) > 0 {
		fmt.Fprintf(&sb, "People the party has met: %s\n", strings.Join(names, ", "))
	}
	if sb.Len() == 0 {
		return "(the players have only just begun; they know little beyond the opening)\n"
	}
	return sb.String()
}

// hiddenContext gathers the DM-only / not-yet-revealed material a narration must
// not leak: the adventure's true background, hidden notes and the private
// secrets/motivations of the NPCs present, and the names of characters, events
// and scenes the party hasn't reached yet (those names are themselves spoilers).
func (o *Oracle) hiddenContext() string {
	adv := o.session.Adventure
	st := o.session.State
	var sb strings.Builder

	if b := strings.TrimSpace(adv.Background); b != "" {
		fmt.Fprintf(&sb, "The truth behind the adventure: %s\n", oneLineTrim(b))
	}

	if room, _ := adv.Room(st.CurrentRoom); room != nil {
		eff, _ := effectiveRoom(adv.Scene(st.Scene()), room)
		if dn := strings.TrimSpace(eff.DMNotes); dn != "" {
			fmt.Fprintf(&sb, "Hidden notes for the current location: %s\n", oneLineTrim(dn))
		}
		for _, nid := range eff.NPCIDs {
			if n := adv.NPC(nid); n != nil {
				if s := strings.TrimSpace(n.Secrets); s != "" {
					fmt.Fprintf(&sb, "%s's secret: %s\n", n.Name, oneLineTrim(s))
				}
				if m := strings.TrimSpace(n.Motivations); m != "" {
					fmt.Fprintf(&sb, "%s's private motivation: %s\n", n.Name, oneLineTrim(m))
				}
			}
		}
	}

	if names := unmetNPCLabels(adv, st); len(names) > 0 {
		fmt.Fprintf(&sb, "Characters not yet encountered (their names/roles are spoilers): %s\n", strings.Join(names, "; "))
	}
	if names := untriggeredEventNames(adv, st); len(names) > 0 {
		fmt.Fprintf(&sb, "Events that have not happened yet: %s\n", strings.Join(names, "; "))
	}
	if names := futureSceneLabels(adv, st); len(names) > 0 {
		fmt.Fprintf(&sb, "Later scenes not yet reached: %s\n", strings.Join(names, "; "))
	}
	return sb.String()
}

func metNPCLabels(adv *domain.Adventure, st *domain.SessionState) []string {
	var out []string
	for i := range adv.NPCs {
		n := &adv.NPCs[i]
		if ns := st.KnownNPCs[n.ID]; ns != nil && ns.Met {
			out = append(out, n.Name)
		}
	}
	return out
}

func unmetNPCLabels(adv *domain.Adventure, st *domain.SessionState) []string {
	var out []string
	for i := range adv.NPCs {
		n := &adv.NPCs[i]
		if ns := st.KnownNPCs[n.ID]; ns == nil || !ns.Met {
			label := n.Name
			if n.Role != "" {
				label += " (" + n.Role + ")"
			}
			out = append(out, label)
		}
	}
	return out
}

func untriggeredEventNames(adv *domain.Adventure, st *domain.SessionState) []string {
	var out []string
	for i := range adv.Events {
		e := &adv.Events[i]
		if !st.TriggeredEvents[e.ID] {
			out = append(out, nameOrID(e.Name, e.ID))
		}
	}
	return out
}

func futureSceneLabels(adv *domain.Adventure, st *domain.SessionState) []string {
	cur := st.Scene()
	var out []string
	for i := range adv.Scenes {
		s := &adv.Scenes[i]
		if s.ID == cur {
			continue
		}
		label := nameOrID(s.Name, s.ID)
		if d := strings.TrimSpace(s.Description); d != "" {
			label += " — " + oneLineTrim(d)
		}
		out = append(out, label)
	}
	return out
}

func oneLineTrim(s string) string { return strings.Join(strings.Fields(s), " ") }

// stripFences removes a wrapping markdown code fence a model may add around the
// rewritten narration.
func stripFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			s = s[i+1:]
		}
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	}
	return strings.TrimSpace(s)
}

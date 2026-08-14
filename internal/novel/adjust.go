package novel

import (
	"context"
	"fmt"
	"strings"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
)

const (
	// maxAdjustTokens caps each Adjust generation pass. A whole-novel rewrite can
	// exceed this; the continuation loop resumes until the reply completes.
	maxAdjustTokens = 16000
	// maxAdjustContinuations bounds how many "keep going" round-trips one Adjust
	// call makes when a whole-novel rewrite is cut off at the token limit.
	maxAdjustContinuations = 6
	// adjustContextChars bounds the session-timeline digest sent as grounding so a
	// long campaign can't blow the input budget.
	adjustContextChars = 8000
)

// AdjustOptions describes one AI revision of a novelization.
type AdjustOptions struct {
	// FullText is the current whole novel (Markdown). Required.
	FullText string
	// Selection, if non-empty, is the excerpt to revise; Adjust then returns ONLY
	// the revised excerpt (the caller splices it back in). If empty, the whole
	// FullText is revised and the complete revised novel is returned.
	Selection string
	// Instruction is the natural-language edit request (e.g. "make chapter 2
	// darker", "rename X to Y", "add dialogue for the innkeeper").
	Instruction string
	// Progress, if set, is called once before each generation pass with (n, total),
	// n being 1-based. total is 1 for a selection edit; for a whole-novel rewrite it
	// is an upper bound (the continuation cap) since the real count isn't known yet.
	Progress func(n, total int)
}

// Adjust revises a novelization with the AI, grounded in the adventure and the
// session's play timeline so edits stay faithful to what actually happened. When
// opt.Selection is set it revises just that excerpt; otherwise it revises the
// whole novel (continuing across passes when the output is cut off at the token
// limit). It uses plain text generation (no tools), so it works on every backend.
func Adjust(ctx context.Context, prov providers.Provider, model string, adv *domain.Adventure, st *domain.SessionState, opt AdjustOptions) (string, error) {
	if prov == nil {
		return "", fmt.Errorf("no AI provider configured")
	}
	if strings.TrimSpace(opt.Instruction) == "" {
		return "", fmt.Errorf("an adjustment instruction is required")
	}
	if strings.TrimSpace(opt.FullText) == "" {
		return "", fmt.Errorf("there is no novel to adjust yet")
	}

	lang := languageName(adv.Language)
	sys := fmt.Sprintf(`You are a skilled literary editor revising a prose NOVEL that novelizes a tabletop RPG (D&D) play session, written in %s.

Apply the reader's revision instruction faithfully while preserving the novel's established voice, third-person past tense, character names, places, and continuity. Use the adventure premise and the play timeline below only as grounding — do not contradict what actually happened, and never invent mechanics.

Rules:
- Write entirely in %s.
- NEVER include game mechanics, stat blocks, dice rolls, DCs, flags, or meta-commentary.
- Keep "## " chapter headings where they are, unless the instruction is specifically about structure.
- Output ONLY the revised Markdown prose — no code fences, no commentary before or after.`, lang, lang)

	grounding := strings.TrimSpace(storyContext(adv, st) + "\n" + timelineDigest(adv, st, adjustContextChars))

	if strings.TrimSpace(opt.Selection) != "" {
		return adjustSelection(ctx, prov, model, sys, grounding, opt)
	}
	return adjustWhole(ctx, prov, model, sys, grounding, opt)
}

// adjustSelection revises a single excerpt in one pass and returns only the
// revised excerpt.
func adjustSelection(ctx context.Context, prov providers.Provider, model, sys, grounding string, opt AdjustOptions) (string, error) {
	if opt.Progress != nil {
		opt.Progress(1, 1)
	}
	var user strings.Builder
	user.WriteString(grounding)
	user.WriteString("\n\n=== REVISION INSTRUCTION ===\n")
	user.WriteString(strings.TrimSpace(opt.Instruction))
	user.WriteString("\n\n=== EXCERPT TO REVISE (this is a fragment of the novel; revise ONLY this and return ONLY the revised fragment) ===\n")
	user.WriteString(strings.TrimSpace(opt.Selection))
	user.WriteString("\n\nNow output the revised fragment.")

	resp, err := prov.Chat(ctx, providers.ChatRequest{
		Model:     model,
		MaxTokens: maxAdjustTokens,
		Messages: []providers.Message{
			{Role: providers.RoleSystem, Content: sys},
			{Role: providers.RoleUser, Content: user.String()},
		},
	})
	if err != nil {
		return "", err
	}
	return cleanMarkdown(resp.Content), nil
}

// adjustWhole revises the entire novel, transparently continuing when a reply is
// cut off at the output-token limit and stitching the pieces together.
func adjustWhole(ctx context.Context, prov providers.Provider, model, sys, grounding string, opt AdjustOptions) (string, error) {
	var user strings.Builder
	user.WriteString(grounding)
	user.WriteString("\n\n=== REVISION INSTRUCTION ===\n")
	user.WriteString(strings.TrimSpace(opt.Instruction))
	user.WriteString("\n\n=== CURRENT NOVEL (apply the instruction and return the COMPLETE revised novel) ===\n")
	user.WriteString(strings.TrimSpace(opt.FullText))
	user.WriteString("\n\nNow output the complete revised novel.")

	messages := []providers.Message{
		{Role: providers.RoleSystem, Content: sys},
		{Role: providers.RoleUser, Content: user.String()},
	}

	var full strings.Builder
	lastFinish := ""
	for i := 0; i <= maxAdjustContinuations; i++ {
		if opt.Progress != nil {
			opt.Progress(i+1, maxAdjustContinuations+1)
		}
		resp, err := prov.Chat(ctx, providers.ChatRequest{
			Model:     model,
			MaxTokens: maxAdjustTokens,
			Messages:  messages,
		})
		if err != nil {
			return "", err
		}
		chunk := resp.Content
		if i > 0 {
			chunk = stitchProse(full.String(), chunk)
		}
		full.WriteString(chunk)
		lastFinish = resp.FinishReason
		if !truncatedFinish(lastFinish) {
			break
		}
		// This model family may not support assistant prefill, so the conversation
		// must end on a user turn: hand the model its own partial output back plus
		// an explicit instruction and the exact tail to resume from.
		tail := lastChars(full.String(), 240)
		messages = append(messages,
			providers.Message{Role: providers.RoleAssistant, Content: resp.Content},
			providers.Message{Role: providers.RoleUser, Content: "Your reply was cut off at the length limit. Continue the novel from EXACTLY where it stopped. Output ONLY the remaining Markdown prose: no repetition of what you already sent, no commentary, no code fences. Your output so far ends with:\n" + tail},
		)
	}
	// If the final permitted pass is STILL truncated, the revision is incomplete —
	// returning it would let a partial novel be saved as the finished one. Fail
	// loudly so the caller keeps the previous text.
	if truncatedFinish(lastFinish) {
		return "", fmt.Errorf("the revised novel was too long to finish in %d passes; adjust a smaller selection instead", maxAdjustContinuations+1)
	}
	return cleanMarkdown(full.String()), nil
}

// stitchProse cleans a continuation chunk before appending it: it strips a
// leading code fence and drops any overlap where the model repeated the tail of
// what it already sent, so the concatenation reads seamlessly. Unlike the JSON
// stitcher, it preserves ordinary leading whitespace, since a space or newline
// between two prose fragments can be meaningful.
func stitchProse(prev, chunk string) string {
	if trimmed := strings.TrimLeft(chunk, " \t\r\n"); strings.HasPrefix(trimmed, "```") {
		for _, f := range []string{"```markdown", "```md", "```"} {
			trimmed = strings.TrimPrefix(trimmed, f)
		}
		chunk = strings.TrimLeft(trimmed, " \t\r\n")
	}
	maxOverlap := 400
	if len(prev) < maxOverlap {
		maxOverlap = len(prev)
	}
	if len(chunk) < maxOverlap {
		maxOverlap = len(chunk)
	}
	for k := maxOverlap; k > 0; k-- {
		if prev[len(prev)-k:] == chunk[:k] {
			return chunk[k:]
		}
	}
	return chunk
}

func truncatedFinish(finishReason string) bool {
	return finishReason == "max_tokens" || finishReason == "length"
}

// timelineDigest renders a bounded, chronological digest of the session's play
// so the editor can ground edits ("add the innkeeper's dialogue", "make the
// crypt scene tenser") in what actually happened. It stops at maxChars.
func timelineDigest(adv *domain.Adventure, st *domain.SessionState, maxChars int) string {
	beats := collectBeats(adv, st)
	if len(beats) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("=== SESSION TIMELINE (grounding — what actually happened) ===\n")
	for _, b := range beats {
		if sb.Len()+len(b.text)+1 > maxChars {
			sb.WriteString("… (timeline truncated)\n")
			break
		}
		sb.WriteString(b.text)
		sb.WriteString("\n")
	}
	return sb.String()
}

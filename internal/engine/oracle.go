package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
)

const defaultMaxToolIterations = 6

// Oracle drives the DM's dialogue with the LLM, grounding every reply in the
// loaded adventure module and the running session state.
type Oracle struct {
	session    *domain.Session
	provider   providers.Provider
	toolRouter *ToolRouter
}

// NewOracle builds an oracle for a session and provider.
func NewOracle(session *domain.Session, provider providers.Provider) *Oracle {
	return &Oracle{
		session:    session,
		provider:   provider,
		toolRouter: NewToolRouter(session),
	}
}

// SetProvider swaps the active LLM provider.
func (o *Oracle) SetProvider(provider providers.Provider) { o.provider = provider }

// Response is the result of an oracle query.
type Response struct {
	Answer     string
	TokensUsed int
	LatencyMs  int64
	Error      error
}

// Ask sends a DM query to the oracle and runs the tool-calling loop.
func (o *Oracle) Ask(ctx context.Context, input string) *Response {
	resp := &Response{}
	if o.provider == nil {
		resp.Error = fmt.Errorf("no AI provider configured")
		return resp
	}

	o.session.State.Conversation.AddUserMessage(input)

	req := providers.ChatRequest{
		Messages:    o.buildMessages(),
		Tools:       o.toolRouter.GetToolDefinitions(),
		Model:       o.session.Config.Model,
		Temperature: o.session.Config.Temperature,
		MaxTokens:   o.session.Config.MaxTokens,
	}

	var totalLatency int64
	var totalTokens int

	maxIter := o.session.Config.OracleMaxToolIterations
	if maxIter <= 0 {
		maxIter = defaultMaxToolIterations
	}
	for iteration := 0; iteration < maxIter; iteration++ {
		chat, err := o.provider.Chat(ctx, req)
		if err != nil {
			resp.Error = fmt.Errorf("AI request failed: %w", err)
			return resp
		}
		totalLatency += chat.Latency
		totalTokens += chat.Usage.TotalTokens

		if len(chat.ToolCalls) == 0 {
			resp.Answer = chat.Content
			resp.LatencyMs = totalLatency
			resp.TokensUsed = totalTokens
			o.session.State.Conversation.AddAssistantMessage(chat.Content)
			o.session.MarkModified()
			return resp
		}

		req.Messages = append(req.Messages, providers.Message{
			Role:      providers.RoleAssistant,
			Content:   chat.Content,
			ToolCalls: chat.ToolCalls,
		})

		for _, tc := range chat.ToolCalls {
			result := o.toolRouter.Execute(providers.ConvertToolCallToTypesFormat(tc))
			content := result.Content
			if result.Error != "" {
				content = "Error: " + result.Error
			}
			req.Messages = append(req.Messages, providers.Message{
				Role:       providers.RoleTool,
				Content:    content,
				ToolCallID: tc.ID,
				Name:       tc.Function.Name, // needed by Gemini's functionResponse mapping
			})
		}
	}

	resp.Error = fmt.Errorf("maximum tool iterations reached")
	resp.LatencyMs = totalLatency
	resp.TokensUsed = totalTokens
	return resp
}

func (o *Oracle) buildMessages() []providers.Message {
	msgs := []providers.Message{{
		Role:    providers.RoleSystem,
		Content: o.buildSystemPrompt(),
	}}
	for _, m := range o.session.State.Conversation.Messages {
		role := providers.RoleUser
		switch m.Role {
		case domain.RoleAssistant:
			role = providers.RoleAssistant
		case domain.RoleSystem:
			role = providers.RoleSystem
		case domain.RoleTool:
			role = providers.RoleTool
		}
		msgs = append(msgs, providers.Message{
			Role:       role,
			Content:    m.Content,
			Name:       m.Name,
			ToolCallID: m.ToolCallID,
		})
	}
	return msgs
}

// buildSystemPrompt assembles the base oracle instructions plus the always-on
// context: adventure overview, the current room in full, present NPCs, session
// state, and the recent timeline. Everything else is fetched on demand via the
// retrieval tools.
func (o *Oracle) buildSystemPrompt() string {
	var sb strings.Builder
	adv := o.session.Adventure
	st := o.session.State

	sb.WriteString(o.session.Config.GetSystemPrompt())
	sb.WriteString("\n\n=== ADVENTURE ===\n")
	fmt.Fprintf(&sb, "Title: %s", adv.Title)
	if adv.System != "" {
		fmt.Fprintf(&sb, " (%s)", adv.System)
	}
	sb.WriteString("\n")
	writeSection(&sb, "Summary", adv.Summary)
	writeSection(&sb, "Background (DM eyes only)", adv.Background)

	sb.WriteString("\n=== CURRENT SITUATION ===\n")
	if room, zone := adv.Room(st.CurrentRoom); room != nil {
		if zone != nil {
			fmt.Fprintf(&sb, "Zone: %s [%s]\n", zone.Name, zone.ID)
		}
		sb.WriteString(FormatRoom(adv, room))
		sb.WriteString("\n")
		if len(room.NPCIDs) > 0 {
			sb.WriteString("\n--- Present NPCs ---\n")
			for _, nid := range room.NPCIDs {
				if n := adv.NPC(nid); n != nil {
					sb.WriteString(FormatNPC(adv, n))
					sb.WriteString("\n\n")
				}
			}
		}
	} else {
		sb.WriteString("No current room set. Use set_location or ask the DM where the party is.\n")
	}

	sb.WriteString("\n=== SESSION STATE ===\n")
	fmt.Fprintf(&sb, "Rooms visited: %d\n", len(st.VisitedRooms))
	if len(st.TriggeredEvents) > 0 {
		ids := make([]string, 0, len(st.TriggeredEvents))
		for id := range st.TriggeredEvents {
			ids = append(ids, id)
		}
		sb.WriteString("Triggered events: " + strings.Join(ids, ", ") + "\n")
	}
	if len(st.Flags) > 0 {
		var flags []string
		for k, v := range st.Flags {
			flags = append(flags, fmt.Sprintf("%s=%v", k, v))
		}
		sb.WriteString("Flags: " + strings.Join(flags, ", ") + "\n")
	}
	if len(st.KnownNPCs) > 0 {
		var known []string
		for id, s := range st.KnownNPCs {
			known = append(known, fmt.Sprintf("%s(disp=%s,alive=%v)", id, s.Disposition, s.Alive))
		}
		sb.WriteString("Known NPCs: " + strings.Join(known, ", ") + "\n")
	}
	for _, q := range st.Quests {
		fmt.Fprintf(&sb, "Quest [%s]: %s\n", q.Status, q.Name)
	}
	for _, p := range st.Party {
		fmt.Fprintf(&sb, "PC %s: HP %d/%d AC %d %s\n", p.Name, p.CurrentHP, p.MaxHP, p.AC, p.Notes)
	}

	if st.Summary != "" {
		sb.WriteString("\n=== STORY SO FAR ===\n")
		sb.WriteString(st.Summary)
		sb.WriteString("\n")
	}

	recentN := o.session.Config.OracleRecentTimeline
	if recentN <= 0 {
		recentN = 15
	}
	recent := st.Log.GetLast(recentN)
	if len(recent) > 0 {
		sb.WriteString("\n=== RECENT TIMELINE ===\n")
		for _, e := range recent {
			fmt.Fprintf(&sb, "- [%s] %s\n", e.Type, e.Message)
		}
	}

	return sb.String()
}

func writeSection(sb *strings.Builder, label, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	sb.WriteString(label + ": " + value + "\n")
}

// UpdateSummary asks the model to summarize the session log when it grows long,
// keeping the always-on context bounded.
func (o *Oracle) UpdateSummary(ctx context.Context) error {
	threshold := o.session.Config.OracleSummarizeAfter
	if threshold <= 0 {
		threshold = 20
	}
	if o.session.State.Log.Len() < threshold {
		return nil
	}

	var sb strings.Builder
	sb.WriteString("Summarize the following D&D session timeline into a concise recap (<300 words) of what the party has done, key decisions, and open threads:\n\n")
	for _, e := range o.session.State.Log.Entries {
		fmt.Fprintf(&sb, "- [%s] %s\n", e.Type, e.Message)
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	chat, err := o.provider.Chat(ctx, providers.ChatRequest{
		Messages: []providers.Message{
			{Role: providers.RoleSystem, Content: "You summarize tabletop RPG session logs."},
			{Role: providers.RoleUser, Content: sb.String()},
		},
		Model:       o.session.Config.Model,
		Temperature: 0.3,
		MaxTokens:   600,
	})
	if err != nil {
		return err
	}
	o.session.State.Summary = chat.Content
	o.session.MarkModified()
	return nil
}

// Status returns a quick status map for display.
func (o *Oracle) Status() map[string]any {
	name := "none"
	if o.provider != nil {
		name = o.provider.Name()
	}
	return map[string]any{
		"provider":  name,
		"model":     o.session.Config.Model,
		"adventure": o.session.Adventure.Title,
		"room":      o.session.State.CurrentRoom,
		"log":       o.session.State.Log.Len(),
	}
}

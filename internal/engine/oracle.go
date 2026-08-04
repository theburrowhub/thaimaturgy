package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/mcptools"
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

	// The Claude CLI backend can't drive our tool-calling loop through Chat (it's
	// text-only); instead we let Claude Code run the loop, calling our tools via an
	// MCP server. Everything else uses the direct API tool loop below.
	if cli, ok := o.provider.(*providers.ClaudeCLIProvider); ok {
		return o.askViaCLI(ctx, cli, input)
	}

	o.session.State.AddUserMessage(input)

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
			o.session.State.AddAssistantMessage(chat.Content)
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

// RunGroupTurn resolves a multiplayer round: it aggregates the players' declared
// actions from the round buffer into a single DM prompt, runs the normal GM turn,
// and clears the buffer on success. Returns an error if no actions were declared.
func (o *Oracle) RunGroupTurn(ctx context.Context) *Response {
	actions := o.session.State.RoundActions()
	if len(actions) == 0 {
		return &Response{Error: fmt.Errorf("no actions have been declared this round")}
	}
	resp := o.Ask(ctx, composeRoundInput(actions, o.session.Config.Language))
	if resp.Error == nil {
		// Drop only the actions we actually resolved, so anything submitted while
		// the DM was thinking survives into the next round.
		o.session.State.RemoveResolvedActions(actions)
	}
	return resp
}

// composeRoundInput renders the round's declared actions into the DM prompt.
func composeRoundInput(actions []domain.RoundAction, lang domain.Language) string {
	var sb strings.Builder
	if lang == domain.LangSpanish {
		sb.WriteString("Acciones declaradas por los jugadores esta ronda:\n")
	} else {
		sb.WriteString("The players declared these actions this round:\n")
	}
	for _, a := range actions {
		fmt.Fprintf(&sb, "- %s (%s): %s\n", a.CharacterName, a.DisplayName, a.Text)
	}
	if lang == domain.LangSpanish {
		sb.WriteString("\nResuelve el resultado de todas estas acciones, narra la escena y pregunta qué hacen a continuación.")
	} else {
		sb.WriteString("\nResolve the outcome of all these actions, narrate the scene, and ask what they do next.")
	}
	return sb.String()
}

// conversationContextWindow bounds how many recent conversation messages are
// sent to the model each turn (the full conversation is still persisted).
const conversationContextWindow = 60

// askViaCLI runs the oracle turn through the Claude Code CLI: it exposes the
// session tools over MCP (via this binary's tools subcommand), lets Claude Code
// run the tool-calling loop, then merges the mutations the tools made back into
// the live session state.
func (o *Oracle) askViaCLI(ctx context.Context, cli *providers.ClaudeCLIProvider, input string) *Response {
	resp := &Response{}
	st := o.session.State
	st.AddUserMessage(input)

	// Persist the current state to a temp file the tools subprocess loads/mutates.
	sessPath, err := tempFile("thaim-oracle-session-*.json")
	if err != nil {
		resp.Error = err
		return resp
	}
	defer os.Remove(sessPath)
	if err := writeSessionFile(sessPath, st); err != nil {
		resp.Error = err
		return resp
	}
	oldLogLen := 0
	if st.Log != nil {
		oldLogLen = st.LogLen()
	}

	// MCP config pointing back at this binary's tools subcommand.
	exe, err := os.Executable()
	if err != nil {
		resp.Error = err
		return resp
	}
	cfg := map[string]any{"mcpServers": map[string]any{
		mcptools.ServerName: map[string]any{
			"command": exe,
			"args":    []string{mcptools.SubcommandArg, "--adventure-id", st.AdventureID, "--session", sessPath},
		},
	}}
	cfgPath, err := tempFile("thaim-mcp-*.json")
	if err != nil {
		resp.Error = err
		return resp
	}
	defer os.Remove(cfgPath)
	if b, e := json.Marshal(cfg); e != nil {
		resp.Error = e
		return resp
	} else if e := os.WriteFile(cfgPath, b, 0644); e != nil {
		resp.Error = e
		return resp
	}

	var allowed []string
	for _, d := range o.toolRouter.GetToolDefinitions() {
		allowed = append(allowed, fmt.Sprintf("mcp__%s__%s", mcptools.ServerName, d.Name))
	}

	// The CLI+MCP path runs Claude Code's full agentic tool-calling loop, which is
	// far slower than a single API call — MCP server startup plus several tool
	// iterations. Give it a generous timeout of its own so a normal turn isn't
	// killed by the short per-request deadline ("signal: killed").
	cctx, cancel := cliContext(ctx)
	defer cancel()

	start := time.Now()
	answer, err := cli.RunWithMCP(cctx, o.session.Config.Model, o.buildSystemPrompt(), input, cfgPath, allowed)
	if err != nil {
		resp.Error = fmt.Errorf("AI request failed: %w", err)
		return resp
	}
	resp.LatencyMs = time.Since(start).Milliseconds()

	// Merge tool mutations back into the live state (in place) and record the reply.
	if merged, e := readSessionFile(sessPath); e == nil {
		mergeSessionState(st, merged, oldLogLen)
	}
	st.AddAssistantMessage(answer)
	o.session.MarkModified()
	resp.Answer = answer
	return resp
}

// cliMinTimeout is the floor for a Claude-CLI agentic turn (MCP startup + several
// tool iterations), regardless of the short per-request timeout used for direct
// API calls.
const cliMinTimeout = 5 * time.Minute

// cliContext derives a context for the CLI+MCP turn: it detaches from the
// caller's (short) *deadline* — granting at least cliMinTimeout, or more if the
// caller's deadline was already longer — while still honouring an *explicit*
// cancellation of the parent, so closing the session/app kills the CLI subprocess
// instead of leaving it (and its MCP server) orphaned until the timeout.
func cliContext(parent context.Context) (context.Context, context.CancelFunc) {
	timeout := cliMinTimeout
	if dl, ok := parent.Deadline(); ok {
		if remaining := time.Until(dl); remaining > timeout {
			timeout = remaining
		}
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), timeout)
	// Propagate a real cancel (not the short deadline) from the parent.
	go func() {
		select {
		case <-parent.Done():
			if parent.Err() == context.Canceled {
				cancel()
			}
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

func tempFile(pattern string) (string, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	name := f.Name()
	_ = f.Close()
	return name, nil
}

func writeSessionFile(path string, st *domain.SessionState) error {
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

func readSessionFile(path string) (*domain.SessionState, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var st domain.SessionState
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

// mergeSessionState copies the mutable structured state from src into dst in
// place (so holders of dst see the changes) and replays timeline entries added
// beyond oldLogLen through dst.AppendLog, firing dst's log hook (journal).
func mergeSessionState(dst, src *domain.SessionState, oldLogLen int) {
	dst.ImportStructured(src)
	if src.Log != nil {
		entries := src.Log.Entries
		if oldLogLen < 0 || oldLogLen > len(entries) {
			oldLogLen = len(entries)
		}
		for _, e := range entries[oldLogLen:] {
			dst.AppendLog(e)
		}
	}
}

func (o *Oracle) buildMessages() []providers.Message {
	msgs := []providers.Message{{
		Role:    providers.RoleSystem,
		Content: o.buildSystemPrompt(),
	}}
	// Send only a recent window of the conversation to the model — the full
	// history is persisted for resuming, but the prompt stays bounded. Older
	// context is preserved via the running Summary in the system prompt.
	for _, m := range o.session.State.Conversation.GetLast(conversationContextWindow) {
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

	sb.WriteString(o.systemPromptBase())
	sb.WriteString("\n\n=== ADVENTURE ===\n")
	fmt.Fprintf(&sb, "Title: %s", adv.Title)
	if adv.System != "" {
		fmt.Fprintf(&sb, " (%s)", adv.System)
	}
	sb.WriteString("\n")
	writeSection(&sb, "Summary", adv.Summary)
	writeSection(&sb, "Context", adv.Context)
	writeSection(&sb, "Background (DM eyes only)", adv.Background)
	if len(adv.Tables) > 0 {
		names := make([]string, 0, len(adv.Tables))
		for i := range adv.Tables {
			names = append(names, fmt.Sprintf("%s [%s]", nameOrID(adv.Tables[i].Name, adv.Tables[i].ID), adv.Tables[i].ID))
		}
		fmt.Fprintf(&sb, "Tables (use get_table / roll_table): %s\n", strings.Join(names, ", "))
	}

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

	if st.EffectiveMode() == domain.ModeVirtualDM {
		if party := st.PartySnapshot(); len(party) > 0 {
			sb.WriteString("\n=== PLAYER PARTY (you are the DM; never act for them; target tools by character name) ===\n")
			sb.WriteString(FormatParty(party))
			sb.WriteString("\n")
		}
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
	recent := st.RecentLog(recentN)
	if len(recent) > 0 {
		sb.WriteString("\n=== RECENT TIMELINE ===\n")
		for _, e := range recent {
			fmt.Fprintf(&sb, "- [%s] %s\n", e.Type, e.Message)
		}
	}

	return sb.String()
}

// systemPromptBase returns the base instructions for the session's current mode:
// the virtual-DM (AI-as-DM) prompt in ModeVirtualDM, otherwise the assistant
// oracle prompt (honouring any user override).
func (o *Oracle) systemPromptBase() string {
	if o.session.State.EffectiveMode() == domain.ModeVirtualDM {
		return domain.GMSystemPrompt(o.session.Config.Language)
	}
	return o.session.Config.GetSystemPrompt()
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
	if o.session.State.LogLen() < threshold {
		return nil
	}

	var sb strings.Builder
	sb.WriteString("Summarize the following D&D session timeline into a concise recap (<300 words) of what the party has done, key decisions, and open threads:\n\n")
	for _, e := range o.session.State.RecentLog(0) {
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
		"log":       o.session.State.LogLen(),
	}
}

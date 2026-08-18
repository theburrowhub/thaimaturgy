package engine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/mcptools"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
)

const defaultMaxToolIterations = 6

// Oracle drives the DM's dialogue with the LLM, grounding every reply in the
// loaded adventure module and the running session state.
type Oracle struct {
	session     *domain.Session
	provider    providers.Provider
	toolRouter  *ToolRouter
	askSequence atomic.Uint64
	// executionNamespace distinguishes idempotency keys emitted by different
	// Oracle instances. askSequence alone restarts at one after process/session
	// reload and would otherwise collide with persisted receipts.
	executionNamespace string
}

// NewOracle builds an oracle for a session and provider.
func NewOracle(session *domain.Session, provider providers.Provider) *Oracle {
	return &Oracle{
		session:            session,
		provider:           provider,
		toolRouter:         NewToolRouter(session),
		executionNamespace: newOracleExecutionNamespace(),
	}
}

var oracleNamespaceFallback atomic.Uint64

func newOracleExecutionNamespace() string {
	var entropy [12]byte
	if _, err := rand.Read(entropy[:]); err == nil {
		return hex.EncodeToString(entropy[:])
	}
	// crypto/rand failures are exceptional, but NewOracle has historically been
	// infallible. Preserve that API while retaining process-local uniqueness.
	return fmt.Sprintf("fallback-%d-%d", time.Now().UnixNano(), oracleNamespaceFallback.Add(1))
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
	askID := o.askSequence.Add(1)

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
			answer := o.reviewSpoilers(ctx, chat.Content)
			resp.Answer = answer
			resp.LatencyMs = totalLatency
			resp.TokensUsed = totalTokens
			o.session.State.AddAssistantMessage(answer)
			o.session.MarkModified()
			return resp
		}

		req.Messages = append(req.Messages, providers.Message{
			Role:      providers.RoleAssistant,
			Content:   chat.Content,
			ToolCalls: chat.ToolCalls,
		})

		for callIndex, tc := range chat.ToolCalls {
			call := providers.ConvertToolCallToTypesFormat(tc)
			// Provider IDs correlate the tool response inside that provider's
			// protocol, but they are not guaranteed unique across turns (Gemini's
			// adapter synthesizes name/index IDs). The host owns the idempotency key,
			// so namespace it by this Ask and loop position before execution.
			call.ID = fmt.Sprintf("oracle:%s:%d:%d:%d", o.executionNamespace, askID, iteration, callIndex)
			result := o.toolRouter.Execute(call)
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

// MetaInput frames an out-of-character player message (a rules question, a
// clarification, or a correction) so the DM answers it directly instead of
// treating it as an in-fiction action. Shared by the app and the Telegram bot.
func MetaInput(text string, lang domain.Language) string {
	if lang == domain.LangSpanish {
		return "[METAJUEGO / FUERA DE PERSONAJE — es el jugador hablándote a ti (el DM), no una acción dentro de la ficción. Responde su duda, aclara la regla o aplica su corrección de forma directa y breve, sin narrar una escena]: " + text
	}
	return "[META / OUT-OF-CHARACTER — this is the player talking to you (the DM), not an in-fiction action. Answer their question, clarify the rule, or apply their correction directly and briefly, without narrating a scene]: " + text
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
	askID := o.askSequence.Add(1)
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
	mcpArgs := []string{mcptools.SubcommandArg, "--adventure-id", st.AdventureID, "--session", sessPath}
	mcpArgs = append(mcpArgs, "--request-namespace", fmt.Sprintf("oracle-%s-%d", o.executionNamespace, askID))
	if o.session.DataDirectory != "" {
		mcpArgs = append(mcpArgs, "--data-dir", o.session.DataDirectory)
	}
	cfg := map[string]any{"mcpServers": map[string]any{
		mcptools.ServerName: map[string]any{
			"command": exe,
			"args":    mcpArgs,
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
	} else if e := os.WriteFile(cfgPath, b, 0o600); e != nil {
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

	// Deliver untrusted world state (issue #21) as part of the user input, not the
	// system prompt: for the CLI path the input is the user-role message, so this
	// keeps it at lower priority than the system prompt. Ephemeral — the raw input
	// was already persisted above; this augmented copy is only sent to the model.
	cliInput := input
	if ws := o.worldStateContext(); ws != "" {
		cliInput = ws + "\n\n" + input
	}

	start := time.Now()
	answer, runErr := cli.RunWithMCP(cctx, o.session.Config.Model, o.buildSystemPrompt(), cliInput, cfgPath, allowed)
	resp.LatencyMs = time.Since(start).Milliseconds()

	// Merge even when the CLI failed: the MCP child may already have durably
	// checkpointed a random response, event batch, or external decision. Leaving
	// the live parent stale would let a later autosave roll the canonical file
	// back over that checkpoint.
	merged, err := readSessionFile(sessPath)
	if err != nil {
		resp.Error = fmt.Errorf("read tool-mutated session: %w", err)
		return resp
	}
	if err := mergeSessionState(st, merged, oldLogLen); err != nil {
		resp.Error = fmt.Errorf("merge tool-mutated session: %w", err)
		return resp
	}
	if o.session.PersistRules != nil {
		if err := o.session.PersistRules(st); err != nil {
			resp.Error = fmt.Errorf("persist merged tool session: %w", err)
			return resp
		}
	}
	if runErr != nil {
		resp.Error = fmt.Errorf("AI request failed: %w", runErr)
		return resp
	}
	answer = o.reviewSpoilers(ctx, answer)
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
	return os.WriteFile(path, b, 0o600)
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
func mergeSessionState(dst, src *domain.SessionState, oldLogLen int) error {
	if err := dst.ImportStructuredChecked(src); err != nil {
		return err
	}
	if src.Log != nil {
		entries := src.Log.Entries
		if oldLogLen < 0 || oldLogLen > len(entries) {
			oldLogLen = len(entries)
		}
		for _, e := range entries[oldLogLen:] {
			dst.AppendLog(e)
		}
	}
	return nil
}

// worldStateContext renders the DM-recorded world changes (issue #21) for the
// party's current room and the NPCs present, or "" when there are none. This text
// is model-generated in response to player actions and is therefore UNTRUSTED, so
// it is delivered as a lower-priority data message (never in the system prompt):
// FormatWorldChanges wraps it in a fenced block with a fixed instruction to treat
// the content strictly as factual world state and never as instructions.
func (o *Oracle) worldStateContext() string {
	adv, st := o.session.Adventure, o.session.State
	room, _ := adv.Room(st.CurrentRoom)
	if room == nil {
		return ""
	}
	var blocks []string
	// Per target, a full CURRENT-description override (v2) is the single source of
	// truth and supersedes both the authored text and the bullet log; otherwise
	// fall back to the recorded change bullets (v1).
	roomTgt := worldTarget("room", room.ID)
	if desc := st.WorldDescription(roomTgt); desc != "" {
		blocks = append(blocks, FormatWorldState("the room "+room.Name, desc, nil))
	} else if wc := FormatWorldChanges(st.WorldChangesFor(roomTgt)); wc != "" {
		blocks = append(blocks, "Room "+room.Name+":\n"+wc)
	}
	for _, nid := range room.NPCIDs {
		n := adv.NPC(nid)
		if n == nil {
			continue
		}
		npcTgt := worldTarget("npc", n.ID)
		if desc := st.WorldDescription(npcTgt); desc != "" {
			blocks = append(blocks, FormatWorldState("the character "+n.Name, desc, nil))
		} else if wc := FormatWorldChanges(st.WorldChangesFor(npcTgt)); wc != "" {
			blocks = append(blocks, "NPC "+n.Name+":\n"+wc)
		}
	}
	return strings.Join(blocks, "\n\n")
}

func (o *Oracle) buildMessages() []providers.Message {
	msgs := []providers.Message{{
		Role:    providers.RoleSystem,
		Content: o.buildSystemPrompt(),
	}}
	// Deliver untrusted, player-influenced world state as a lower-priority data
	// message (user role), not in the system prompt (see worldStateContext). It is
	// ephemeral — recomputed each turn from current state, never persisted into the
	// conversation — so it can't grow the history or go stale.
	if ws := o.worldStateContext(); ws != "" {
		msgs = append(msgs, providers.Message{Role: providers.RoleUser, Content: ws})
	}
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
	if snapshot, ok := st.RulesSnapshot(); ok {
		sb.WriteString("\n=== LOADED RULES PACKAGE ===\n")
		fmt.Fprintf(&sb, "Exact identity: %s@%s (%s, protocol %s)\n",
			snapshot.Ruleset.ID, snapshot.Ruleset.Version, snapshot.Ruleset.Digest, snapshot.Ruleset.ProtocolVersion)
		sb.WriteString("This package is the mechanical authority. Use game_list_actions and game_get_action_schema to discover its typed actions, game_submit_intent to resolve them, game_respond for pending input, and game_explain for visible rules. Do not calculate or invent a result outside that interface.\n")
	}
	writeSection(&sb, "Summary", adv.Summary)
	writeSection(&sb, "Context", adv.Context)
	writeSection(&sb, "Background (DM eyes only)", adv.Background)
	// The opening: how the adventure begins and the hooks that draw the party in.
	// Injected so the DM can actually narrate the premise/hook when the game
	// starts — otherwise it drops the party into a location with no framing. This
	// is general: any module that authored an introduction/hooks gets a narrated
	// opening; a module without them is unaffected.
	writeSection(&sb, "How it begins (introduction)", adv.Introduction)
	if len(adv.Hooks) > 0 {
		sb.WriteString("Hooks (how the party is drawn in — deliver at least one when opening):\n")
		for _, h := range adv.Hooks {
			if h = strings.TrimSpace(h); h != "" {
				fmt.Fprintf(&sb, "  - %s\n", h)
			}
		}
	}
	if len(adv.Tables) > 0 {
		names := make([]string, 0, len(adv.Tables))
		for i := range adv.Tables {
			names = append(names, fmt.Sprintf("%s [%s]", nameOrID(adv.Tables[i].Name, adv.Tables[i].ID), adv.Tables[i].ID))
		}
		fmt.Fprintf(&sb, "Tables (use get_table / roll_table): %s\n", strings.Join(names, ", "))
	}

	sb.WriteString("\n=== CURRENT SITUATION ===\n")
	// Resolve the active scene (if any). Scenes let the same location read
	// differently at different points in the story, so the room below is rendered
	// THROUGH the scene's overrides. A module with no scenes → scene is nil and
	// everything behaves exactly as before.
	scene := adv.Scene(st.Scene()) // Scene() reads CurrentScene under the state lock
	if scene != nil {
		fmt.Fprintf(&sb, "Current scene: %s [%s]\n", nameOrID(scene.Name, scene.ID), scene.ID)
		if scene.ReadAloud != "" {
			fmt.Fprintf(&sb, "Scene read-aloud (narrate when the scene opens): %s\n", scene.ReadAloud)
		}
		if scene.Description != "" {
			fmt.Fprintf(&sb, "Scene notes (DM eyes only): %s\n", scene.Description)
		}
		if len(scene.Next) > 0 {
			sb.WriteString("This scene can lead to (advance with set_scene when the story calls for it — you decide when):\n")
			for _, t := range scene.Next {
				label := t.To
				if ns := adv.Scene(t.To); ns != nil {
					label = nameOrID(ns.Name, ns.ID) + " [" + ns.ID + "]"
				}
				if t.When != "" {
					fmt.Fprintf(&sb, "  - %s — when: %s\n", label, t.When)
				} else {
					fmt.Fprintf(&sb, "  - %s\n", label)
				}
			}
		}
		sb.WriteString("Describe locations AS THEY ARE IN THIS SCENE (present cast, mood, what's available) using the scene overrides below; do not treat a change of scene as moving to another place.\n")
	}
	if room, zone := adv.Room(st.CurrentRoom); room != nil {
		if zone != nil {
			fmt.Fprintf(&sb, "Zone: %s [%s]\n", zone.Name, zone.ID)
		}
		effRoom, present := effectiveRoom(scene, room)
		// Mutable-world v2: when the DM has set a full CURRENT description for this
		// room, SUPPRESS the authored/scene read-aloud here so the model never sees
		// the stale original. The current description is delivered instead as
		// untrusted data (worldStateContext). Copy first — effectiveRoom may return
		// the authored room pointer, which must never be mutated.
		if st.WorldDescription(worldTarget("room", room.ID)) != "" {
			cp := *effRoom
			cp.ReadAloud = ""
			effRoom = &cp
		}
		if present != "" {
			fmt.Fprintf(&sb, "In this scene, notably: %s\n", present)
		}
		sb.WriteString(FormatRoom(adv, effRoom))
		sb.WriteString("\n")
		// NB: DM-recorded world changes (issue #21) are NOT injected here. They are
		// model-generated in response to player actions, so they are untrusted and
		// must not sit at system priority; buildMessages/askViaCLI deliver them in a
		// separate, lower-priority data message instead (see worldStateContext).
		if len(effRoom.NPCIDs) > 0 {
			sb.WriteString("\n--- Present NPCs ---\n")
			for _, nid := range effRoom.NPCIDs {
				if n := adv.NPC(nid); n != nil {
					// Same v2 suppression for an NPC whose current appearance was
					// overridden (copy so the authored NPC isn't mutated).
					if st.WorldDescription(worldTarget("npc", nid)) != "" {
						cp := *n
						cp.Appearance = ""
						n = &cp
					}
					sb.WriteString(FormatNPC(adv, n))
					sb.WriteString("\n\n")
				}
			}
		}
		// Adjacency / marching order: tell the model where the party can actually
		// go from here, so it doesn't infer spatial order from the order zones are
		// written in the module (a zone written earlier is NOT automatically
		// "before" a later one).
		if zone != nil {
			if adj := FormatAdjacency(adv, zone.ID); adj != "" {
				sb.WriteString("\n--- WHERE THE PARTY CAN GO (marching order) ---\n")
				sb.WriteString(adj)
				sb.WriteString("\nOnly move the party through the room exits or adjacent zones listed above. Do NOT place them in a zone that is not reachable from here, and do not assume the module's authored order is the travel order. To reach a distant zone, use find_path to get the route.\n")
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

	// The party's CURRENT sheets are authoritative and must reach the DM every
	// turn (in any mode where a party exists), so narration never contradicts a
	// character's real HP/conditions. Tool results within the turn already reflect
	// mutations (e.g. update_hp), so this stays consistent mid-turn.
	if party := st.PartySnapshot(); len(party) > 0 {
		header := "\n=== PLAYER PARTY — CURRENT SHEETS (authoritative: never narrate a state that contradicts these — HP, conditions, etc.) ===\n"
		if st.EffectiveMode() == domain.ModeVirtualDM {
			// The DM-role instruction only applies when the AI is actually the DM;
			// in assistant mode the human is the DM, so don't tell the model it is.
			header = "\n=== PLAYER PARTY — CURRENT SHEETS (authoritative: never narrate a state that contradicts these — HP, conditions, etc.; you are the DM, never act for them; target tools by character name) ===\n"
		}
		sb.WriteString(header)
		sb.WriteString(FormatParty(party))
		sb.WriteString("\n")
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
	// Over-fetch so filtering LogWorld out still leaves a full window.
	recent := st.RecentLog(recentN * 2)
	var timeline []domain.LogEntry
	for _, e := range recent {
		// LogWorld entries quote the raw (untrusted) world-change text; keep them
		// out of the system-priority prompt — current world state is delivered
		// separately as a lower-priority data message (see worldStateContext). The
		// full entries still live in the human-facing log/journal.
		if e.Type == domain.LogWorld {
			continue
		}
		timeline = append(timeline, e)
	}
	if len(timeline) > recentN {
		timeline = timeline[len(timeline)-recentN:]
	}
	if len(timeline) > 0 {
		sb.WriteString("\n=== RECENT TIMELINE ===\n")
		for _, e := range timeline {
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
	sb.WriteString("Summarize the following tabletop RPG session timeline into a concise recap (<300 words) of what the party has done, key decisions, and open threads:\n\n")
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

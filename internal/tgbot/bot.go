// Package tgbot implements the Telegram multiplayer front-end as a reusable
// component: it drives a virtual-DM session over a Telegram chat (players claim
// party members, declare actions with /do, and trigger the AI DM with /dm). It is
// used both by the standalone thaimaturgy-bot binary and, in-process, by the
// desktop app to host the currently-running DM session.
package tgbot

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/engine"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

// Options configures a Bot.
type Options struct {
	Token  string // Telegram bot token (required)
	ChatID int64  // restrict to this chat id (0 = any chat)
	// AllowedUsers optionally restricts who may talk to the bot by IMMUTABLE
	// numeric user id. A message is accepted if it is from ChatID OR from an
	// allowed user id (any chat, including a private DM). Empty = no user filter.
	// @username entries are accepted as input but IGNORED for access control
	// (usernames are reassignable → impersonation); a warning is logged.
	AllowedUsers []string
	// OnEvent, if set, is called with short human-readable activity lines (player
	// joins, actions, narration) so a host UI can mirror what happens in the chat.
	OnEvent func(string)
}

// Bot hosts a multiplayer virtual-DM session over Telegram.
type Bot struct {
	api           *tgbotapi.BotAPI
	store         *storage.Storage
	session       *domain.Session
	oracle        *engine.Oracle
	chatID        int64
	allowedIDs    map[string]bool // immutable numeric user ids allowed to talk to the bot
	userFilterSet bool            // an AllowedUsers list was configured (even if it yielded no valid ids)
	onEvent       func(string)

	mu        sync.Mutex // guards resolving
	resolving bool
	saveMu    sync.Mutex      // serializes session file writes
	runCtx    context.Context // set by Run; parents each /dm turn so Stop cancels it
	turns     sync.WaitGroup  // tracks in-flight /dm turns so Stop can wait them out
}

// New builds a Bot bound to a live session and oracle. The session should already
// be in virtual-DM mode with a party (the caller ensures this).
func New(store *storage.Storage, session *domain.Session, oracle *engine.Oracle, opts Options) (*Bot, error) {
	if strings.TrimSpace(opts.Token) == "" {
		return nil, fmt.Errorf("no Telegram bot token configured")
	}
	api, err := tgbotapi.NewBotAPI(opts.Token)
	if err != nil {
		return nil, err
	}
	// A session already played (in the GUI or a prior run) shouldn't demand /begin
	// or re-narrate an opening — treat it as started when there's evidence.
	session.State.MarkStartedIfInProgress()
	allowedIDs, ignoredUsers := normalizeAllowedUsers(opts.AllowedUsers)
	userFilterSet := len(allowedIDs) > 0 || len(ignoredUsers) > 0
	if len(ignoredUsers) > 0 {
		log.Printf("WARNING: Telegram allowed users %v are @usernames and are IGNORED for access control (usernames are reassignable); list immutable numeric user ids instead.", ignoredUsers)
	}
	if userFilterSet && len(allowedIDs) == 0 && opts.ChatID == 0 {
		log.Printf("WARNING: the Telegram allowed-users list has no valid numeric ids and no chat id is set — NO ONE can talk to the bot (fail-closed). Add a numeric user id and/or a chat id.")
	}
	return &Bot{
		api:           api,
		store:         store,
		session:       session,
		oracle:        oracle,
		chatID:        opts.ChatID,
		allowedIDs:    allowedIDs,
		userFilterSet: userFilterSet,
		onEvent:       opts.OnEvent,
	}, nil
}

// normalizeAllowedUsers turns configured allow-list entries into a set of
// IMMUTABLE numeric Telegram user ids used for access control. @usernames are
// mutable and reassignable, so they must NOT authorize access (impersonation
// risk); any non-numeric entries are returned separately so we can warn that
// they are ignored for authorization. (A user allowed in the configured chat is
// already covered by the chat-id rule; the user allow-list exists for private
// DMs, which is exactly where a spoofable username would be dangerous.)
func normalizeAllowedUsers(list []string) (ids map[string]bool, ignored []string) {
	for _, raw := range list {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		// An explicit @-prefixed entry is a username by intent — never a numeric id
		// (e.g. "@12345" must NOT authorize the account whose id is 12345). Strip the
		// '@' only for the ignored-warning list, and never parse it as an id.
		if strings.HasPrefix(s, "@") {
			ignored = append(ignored, strings.TrimPrefix(s, "@"))
			continue
		}
		if _, err := strconv.ParseInt(s, 10, 64); err == nil {
			if ids == nil {
				ids = map[string]bool{}
			}
			ids[s] = true
		} else {
			ignored = append(ignored, s)
		}
	}
	return ids, ignored
}

// allowMessage decides whether a message is accepted. It is UNRESTRICTED (accept
// everyone) only when no chat id is set AND no user filter was configured at all.
// Once the operator configures either restriction it fails CLOSED: accept only
// messages from the allowed chat or from an allowed immutable numeric user id.
// A configured-but-empty user filter (e.g. only @usernames, which are ignored)
// therefore does NOT re-open access.
func allowMessage(chatID int64, allowedIDs map[string]bool, userFilterSet bool, msgChatID, fromID int64) bool {
	if chatID == 0 && !userFilterSet {
		return true // no restriction configured at all
	}
	if chatID != 0 && msgChatID == chatID {
		return true
	}
	return len(allowedIDs) > 0 && allowedIDs[strconv.FormatInt(fromID, 10)]
}

// Username returns the bot's @username (for display).
func (b *Bot) Username() string { return b.api.Self.UserName }

// Run processes updates until ctx is cancelled. Call it directly (blocking) for
// the standalone binary, or in a goroutine with a cancellable context for the
// in-app host; Stop cancels the receive loop.
func (b *Bot) Run(ctx context.Context) {
	b.runCtx = ctx
	log.Printf("thaimaturgy-bot online as @%s — adventure %q, session %q", b.api.Self.UserName, b.session.Adventure.ID, b.session.State.Name)
	if b.chatID == 0 && !b.userFilterSet {
		log.Printf("WARNING: no chat id or allowed users set — any chat that finds this bot can play and trigger LLM turns; set a chat id and/or allowed users to restrict.")
	}
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30
	updates := b.api.GetUpdatesChan(u)
	for {
		select {
		case <-ctx.Done():
			return
		case update, ok := <-updates:
			if !ok {
				return
			}
			b.onUpdate(update)
		}
	}
}

// Stop ends the receive loop and waits for any in-flight /dm turn to finish, so a
// torn-down host never keeps mutating/saving the (now abandoned) session. Cancel
// the context passed to Run first to abort a slow turn promptly.
func (b *Bot) Stop() {
	b.api.StopReceivingUpdates()
	b.turns.Wait()
}

func (b *Bot) onUpdate(update tgbotapi.Update) {
	m := update.Message
	if m == nil || m.From == nil {
		return
	}
	if !allowMessage(b.chatID, b.allowedIDs, b.userFilterSet, m.Chat.ID, m.From.ID) {
		return // not an allowed chat or user
	}
	// Bind any character reserved for this sender's @username (via /assign) the
	// first time they appear.
	if pc, bound := b.session.State.ResolvePending(strconv.FormatInt(m.From.ID, 10), m.From.UserName, displayName(m.From)); bound {
		b.save()
		b.event(fmt.Sprintf("%s → %s (assigned)", displayName(m.From), pc))
		b.send(m.Chat.ID, fmt.Sprintf("%s is now playing %s.", displayName(m.From), pc))
	}
	if m.IsCommand() {
		b.handleCommand(m)
	}
}

func (b *Bot) handleCommand(m *tgbotapi.Message) {
	playerID := strconv.FormatInt(m.From.ID, 10)
	display := displayName(m.From)
	arg := strings.TrimSpace(m.CommandArguments())

	switch m.Command() {
	case "begin", "beginadventure":
		b.startGame(m)
	case "start", "help":
		// /start is what Telegram auto-sends when a user opens the chat, so it must
		// stay harmless (help) — the game is begun explicitly with /begin.
		b.reply(m, helpText)
	case "chatid":
		b.reply(m, fmt.Sprintf("Chat id: %d\nYour user id: %d\nUse the chat id to restrict the bot to this chat, and your user id in the allowed-users list to talk to it privately.", m.Chat.ID, m.From.ID))
	case "map":
		b.sendZoneMap(m)
	case "portrait", "npcart":
		b.sendNPCArt(m, arg)
	case "party":
		b.reply(m, b.partyText())
	case "roster":
		b.reply(m, b.rosterText())
	case "pick", "play":
		name, err := b.session.State.ClaimCharacter(playerID, display, arg)
		if err != nil {
			b.reply(m, "⚠ "+err.Error())
			return
		}
		b.save()
		b.event(fmt.Sprintf("%s picked %s", display, name))
		controlled := b.session.State.PlayerCharacterNames(playerID)
		if len(controlled) > 1 {
			b.reply(m, fmt.Sprintf("%s now also plays %s (active). You control: %s. Use /as <name> to switch, or “/do <name>: …”.", display, name, strings.Join(controlled, ", ")))
		} else {
			b.reply(m, fmt.Sprintf("%s now plays %s. Declare actions with /do.", display, name))
		}
	case "as", "switch":
		if arg == "" {
			b.reply(m, "Usage: /as <character> — choose which of your characters is active.")
			return
		}
		name, err := b.session.State.SetActiveCharacter(playerID, arg)
		if err != nil {
			b.reply(m, "⚠ "+err.Error())
			return
		}
		b.save()
		b.reply(m, "You are now acting as "+name+".")
	case "chat", "say":
		if !b.session.State.GameStarted() {
			b.reply(m, notStartedMsg)
			return
		}
		if arg == "" {
			b.reply(m, "Usage: /chat [<character>:] <what your character says>")
			return
		}
		actor, text := splitActor(arg, b.session.State.PlayerCharacterNames(playerID))
		char, err := b.session.State.ResolvePlayerCharacter(playerID, actor)
		if err != nil {
			b.reply(m, "⚠ "+err.Error())
			return
		}
		if strings.TrimSpace(text) == "" {
			b.reply(m, "Usage: /chat [<character>:] <what your character says>")
			return
		}
		if b.isResolving() {
			b.reply(m, "The DM is resolving the round — try again in a moment.")
			return
		}
		b.session.State.AddChat(char, text)
		b.save()
		b.event(fmt.Sprintf("%s (in character): %s", char, text))
		b.reply(m, fmt.Sprintf("💬 %s: %s", char, text))
	case "meta", "ooc":
		if arg == "" {
			b.reply(m, "Usage: /meta <question or correction for the DM>")
			return
		}
		b.runMeta(m, arg)
	case "assign":
		b.assign(m)
	case "me":
		b.reply(m, b.sheetText(playerID, arg))
	case "do":
		if !b.session.State.GameStarted() {
			b.reply(m, notStartedMsg)
			return
		}
		if arg == "" {
			b.reply(m, "Usage: /do [<character>:] <what your character does>")
			return
		}
		actor, text := splitActor(arg, b.session.State.PlayerCharacterNames(playerID))
		if strings.TrimSpace(text) == "" {
			b.reply(m, "Usage: /do [<character>:] <what your character does>")
			return
		}
		act, err := b.session.State.SubmitAction(playerID, actor, text)
		if err != nil {
			b.reply(m, "⚠ "+err.Error())
			return
		}
		b.save()
		b.event(fmt.Sprintf("%s: %s", act.CharacterName, text))
		b.reply(m, b.roundStatus())
	case "dm", "narrate":
		if !b.session.State.GameStarted() {
			b.reply(m, notStartedMsg)
			return
		}
		b.runDM(m)
	case "roll":
		b.reply(m, rollText(arg))
	case "hp":
		b.editHP(m, arg)
	case "condition", "cond":
		b.editCondition(m, arg, true)
	case "uncondition", "uncond":
		b.editCondition(m, arg, false)
	case "gold":
		b.editGold(m, arg)
	case "xp":
		b.editXP(m, arg)
	case "item", "inv":
		b.editItem(m, arg)
	case "setnote":
		b.editNote(m, arg)
	case "save":
		b.saveAndReport(m)
	case "log":
		b.reply(m, b.logText(arg))
	default:
		b.delegateToEngine(m)
	}
}

// logText renders the recent session timeline for players (issue #25). It hides
// free-form DM notes (LogNote), which are DM-only, to avoid leaking them.
func (b *Bot) logText(arg string) string {
	n := 15
	if v, err := strconv.Atoi(strings.TrimSpace(arg)); err == nil && v > 0 {
		if v > 50 {
			v = 50
		}
		n = v
	}
	entries := b.session.State.RecentLog(n * 3) // over-fetch: DM notes are filtered out below
	lines := make([]string, 0, n)
	for _, e := range entries {
		if e.Type == domain.LogNote {
			continue
		}
		ts := ""
		if !e.Timestamp.IsZero() {
			ts = e.Timestamp.Format("15:04") + " "
		}
		lines = append(lines, fmt.Sprintf("%s[%s] %s", ts, e.Type, e.Message))
	}
	if len(lines) == 0 {
		return "The session log is empty."
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return fmt.Sprintf("📜 Session log (last %d):\n%s", len(lines), strings.Join(lines, "\n"))
}

// playerSafeCommands is the subset of engine slash commands the Telegram bot may
// run on behalf of *any* player. It deliberately excludes:
//   - DM-facing reads that would leak secrets/DM notes/hidden content to players
//     (room/look, zone, npc, npcs, event, item, search) — see #28;
//   - authoritative mutations players must not trigger (goto, flag);
//   - desktop-only UI actions (load, import, mode, map, art).
//
// These stay available in the desktop DM console; multiplayer commands
// (begin/pick/assign/me/do/dm) are handled explicitly above.
var playerSafeCommands = map[string]bool{
	"status": true,                // where the party is + progress counters (no DM notes)
	"quests": true, "quest": true, // player-facing quest log
	"note":  true, // benign: append a note to the timeline
	"rest":  true, // short/long rest for the party (or a named character)
	"recap": true, // "previously on…" from session state (no DM notes/secrets)
}

// delegateToEngine routes a small, player-safe subset of slash commands through
// the SAME engine.CommandHandler the desktop app uses, so shared commands behave
// identically across both frontends (parity, #20) without exposing DM-only
// content or authoritative mutations to players.
func (b *Bot) delegateToEngine(m *tgbotapi.Message) {
	cmd := m.Command()
	if !playerSafeCommands[cmd] {
		b.reply(m, "Unknown or DM-only command. "+helpText)
		return
	}
	// Resting changes HP/hit dice, so it only makes sense once the game has begun
	// (mirrors the desktop app, where Rest is hidden until then).
	if cmd == "rest" && !b.session.State.GameStarted() {
		b.reply(m, notStartedMsg)
		return
	}
	mutating := cmd == "note" || cmd == "rest"
	// Serialize against an in-flight /dm resolution: a mutation applied while the
	// turn snapshot is open could be lost when that snapshot is merged back.
	if mutating && b.isResolving() {
		b.reply(m, "The DM is resolving the round — try again in a moment.")
		return
	}
	raw := "/" + cmd
	if arg := strings.TrimSpace(m.CommandArguments()); arg != "" {
		raw += " " + arg
	}
	res := engine.NewCommandHandler(b.session).Execute(engine.ParseCommand(raw))
	if mutating && res.Success {
		b.save() // persist the mutation even when it only sets Message
	}
	switch {
	case res.Response != "":
		b.reply(m, res.Response)
	case res.Message != "":
		b.reply(m, res.Message) // keep the command's specific message (e.g. usage errors)
	default:
		b.reply(m, "Done.")
	}
}

// isResolving reports whether a /dm turn is currently being resolved.
func (b *Bot) isResolving() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.resolving
}

// saveAndReport persists the current session on demand (the /save command) and
// reports the outcome to the chat, unlike the internal best-effort save().
func (b *Bot) saveAndReport(m *tgbotapi.Message) {
	b.saveMu.Lock()
	err := b.store.SaveSession(b.session.State)
	b.saveMu.Unlock()
	if err != nil {
		log.Printf("save: %v", err)
		b.reply(m, "⚠ Couldn't save the session: "+err.Error())
		return
	}
	b.event(fmt.Sprintf("%s saved the session", displayName(m.From)))
	b.reply(m, "💾 Session saved as “"+b.session.State.Name+"”.")
}

// assign lets a host give a character to a player who hasn't picked. Two forms:
//   - reply to the target player's message with `/assign <character>` → bound
//     immediately (works for anyone, even without a public @username);
//   - `/assign @username <character>` → reserved for that username and bound when
//     they next send a message (Telegram can't resolve @username → id directly).
func (b *Bot) assign(m *tgbotapi.Message) {
	arg := strings.TrimSpace(m.CommandArguments())

	// Reply form: /assign <character>, replying to the target's message.
	if m.ReplyToMessage != nil && m.ReplyToMessage.From != nil {
		if arg == "" {
			b.reply(m, "Usage: reply to the player's message with /assign <character>")
			return
		}
		target := m.ReplyToMessage.From
		name, err := b.session.State.ClaimCharacter(strconv.FormatInt(target.ID, 10), displayName(target), arg)
		if err != nil {
			b.reply(m, "⚠ "+err.Error())
			return
		}
		b.save()
		b.event(fmt.Sprintf("%s assigned %s to %s", displayName(m.From), name, displayName(target)))
		b.reply(m, fmt.Sprintf("%s now plays %s.", displayName(target), name))
		return
	}

	// Username form: /assign @username <character>.
	fields := strings.Fields(arg)
	if len(fields) < 2 || !strings.HasPrefix(fields[0], "@") {
		b.reply(m, "Usage: /assign @username <character>  (or reply to their message with /assign <character>)")
		return
	}
	username := fields[0]
	pc := strings.TrimSpace(strings.TrimPrefix(arg, fields[0]))
	name, err := b.session.State.AssignByUsername(username, pc)
	if err != nil {
		b.reply(m, "⚠ "+err.Error())
		return
	}
	b.save()
	b.event(fmt.Sprintf("%s assigned %s to %s", displayName(m.From), name, username))
	b.reply(m, fmt.Sprintf("%s reserved for %s — it takes effect when they send a message here.", name, username))
}

// startGame begins the game: the DM sets the opening scene, then hands off to the
// players. Before this, /do and /dm are ignored. Idempotent — a second /start
// once underway just says so.
func (b *Bot) startGame(m *tgbotapi.Message) {
	if b.session.State.GameStarted() {
		b.reply(m, "The game is already underway — declare actions with /do, then /dm.")
		return
	}
	if b.session.State.PlayerCount() == 0 {
		b.reply(m, "Everyone pick a character first (/party then /pick <name>), then /begin.")
		return
	}
	b.mu.Lock()
	if b.resolving {
		b.mu.Unlock()
		b.reply(m, "The DM is busy — hang on…")
		return
	}
	b.resolving = true
	b.mu.Unlock()

	b.reply(m, "🎬 The DM is setting the scene…")
	b.turns.Add(1)
	go func() {
		defer b.turns.Done()
		defer func() {
			b.mu.Lock()
			b.resolving = false
			b.mu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(b.turnBase(), b.turnTimeout())
		defer cancel()

		resp := b.oracle.Ask(ctx, domain.DMKickoffPrompt(b.session.Config.Language))
		if b.turnBase().Err() != nil {
			return
		}
		if resp.Error != nil {
			log.Printf("intro: %v", resp.Error)
			b.send(m.Chat.ID, "⚠ The DM couldn't set the scene right now. Try /start again in a moment.")
			return
		}
		b.session.State.StartGame()
		b.save()
		b.event("DM: " + resp.Answer)
		b.sendNarration(m.Chat.ID, resp.Answer)
	}()
}

// turnBase returns the parent context for a DM turn (the Run ctx, so Stop cancels
// an in-flight turn), defaulting to Background before Run is called.
func (b *Bot) turnBase() context.Context {
	if b.runCtx == nil {
		return context.Background()
	}
	return b.runCtx
}

// turnTimeout is the per-turn timeout from config (90s default).
func (b *Bot) turnTimeout() time.Duration {
	if t := time.Duration(b.session.Config.RequestTimeoutSeconds) * time.Second; t > 0 {
		return t
	}
	return 90 * time.Second
}

// runDM resolves the current round with the AI DM and posts the narration. Only
// one turn runs at a time; other commands stay responsive meanwhile.
// runMeta answers an out-of-character player question/correction immediately via
// the oracle (issue #19). It reuses the same turn guard as runDM so it can't run
// concurrently with a /dm resolution and races the session state.
func (b *Bot) runMeta(m *tgbotapi.Message, text string) {
	b.mu.Lock()
	if b.resolving {
		b.mu.Unlock()
		b.reply(m, "The DM is busy resolving the round — try again in a moment.")
		return
	}
	b.resolving = true
	b.mu.Unlock()

	b.reply(m, "🗨 The DM is considering your note…")
	b.turns.Add(1)
	go func() {
		defer b.turns.Done()
		defer func() {
			b.mu.Lock()
			b.resolving = false
			b.mu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(b.turnBase(), b.turnTimeout())
		defer cancel()

		resp := b.oracle.Ask(ctx, engine.MetaInput(text, b.session.Config.Language))
		if b.turnBase().Err() != nil {
			return
		}
		if resp.Error != nil {
			log.Printf("meta: %v", resp.Error)
			b.send(m.Chat.ID, "⚠ The DM couldn't answer right now. Try /meta again in a moment.")
			return
		}
		b.save()
		b.event("DM (meta): " + resp.Answer)
		b.send(m.Chat.ID, resp.Answer)
	}()
}

func (b *Bot) runDM(m *tgbotapi.Message) {
	b.mu.Lock()
	if b.resolving {
		b.mu.Unlock()
		b.reply(m, "The DM is already narrating — hang on…")
		return
	}
	if len(b.session.State.RoundActions()) == 0 {
		b.mu.Unlock()
		b.reply(m, "No actions declared yet. Players, use /do first.")
		return
	}
	b.resolving = true
	b.mu.Unlock()

	b.reply(m, "🎲 The DM is thinking…")
	b.turns.Add(1)
	go func() {
		defer b.turns.Done()
		defer func() {
			b.mu.Lock()
			b.resolving = false
			b.mu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(b.turnBase(), b.turnTimeout())
		defer cancel()

		resp := b.oracle.RunGroupTurn(ctx)
		// If the host was torn down (Run ctx cancelled) mid-turn, don't save or
		// post to a game that's been abandoned.
		if b.turnBase().Err() != nil {
			return
		}
		if resp.Error != nil {
			log.Printf("group turn: %v", resp.Error) // don't leak details to the chat
			b.send(m.Chat.ID, "⚠ The DM couldn't resolve the turn right now. Try /dm again in a moment.")
			return
		}
		b.save()
		b.event("DM: " + resp.Answer)
		b.sendNarration(m.Chat.ID, resp.Answer)
	}()
}

// rosterText lists the persistent campaign roster (issue #33) for players to
// consult from the chat. Creating/choosing characters is done host-side in the
// app; this is the read-only Telegram view.
func (b *Bot) rosterText() string {
	// ListCharacters may return decoded characters AND an error (some files
	// unreadable); surface the warning but still list what loaded.
	chars, err := b.store.ListCharacters()
	if len(chars) == 0 {
		if err != nil {
			return "⚠ Couldn't read the roster: " + err.Error()
		}
		return "The campaign roster is empty. Save characters from the app (Edit party → Roster…)."
	}
	var sb strings.Builder
	if err != nil {
		sb.WriteString("⚠ Some roster entries could not be read: " + err.Error() + "\n")
	}
	sb.WriteString("🧑‍🤝‍🧑 Campaign roster:\n")
	for _, c := range chars {
		sb.WriteString(fmt.Sprintf("• %s — Lvl %d %s %s\n", c.Name, c.Level, c.Race, c.Class))
	}
	return sb.String()
}

func (b *Bot) partyText() string {
	party := b.session.State.PartySnapshot()
	if len(party) == 0 {
		return "The party is empty."
	}
	controllers := b.session.State.Controllers()
	pending := b.session.State.PendingByCharacter()
	var sb strings.Builder
	sb.WriteString("Party — pick one with /pick <name>:\n")
	for i := range party {
		c := party[i]
		line := fmt.Sprintf("• %s — Lvl %d %s %s", c.Name, c.Level, c.Race, c.Class)
		if who, ok := controllers[c.Name]; ok {
			line += " — played by " + who
		} else if u, ok := pending[c.Name]; ok {
			line += " — reserved for @" + u
		}
		sb.WriteString(line + "\n")
	}
	return sb.String()
}

// splitActor separates an optional "<character>: rest" prefix from an argument,
// but ONLY when the part before the colon names one of the player's controlled
// characters — so a mid-sentence colon ("open the chest: it's locked") isn't
// mistaken for a character selector. Returns ("", arg) when no actor prefix
// applies.
func splitActor(arg string, controlled []string) (actor, rest string) {
	i := strings.Index(arg, ":")
	if i <= 0 {
		return "", arg
	}
	cand := strings.TrimSpace(arg[:i])
	for _, c := range controlled {
		if strings.EqualFold(c, cand) {
			return c, strings.TrimSpace(arg[i+1:])
		}
	}
	return "", arg
}

// sheetText shows a player's character sheet. With no argument it shows the
// active character (and, when the player controls several, lists them all with
// the active one starred); "/me <name>" shows a specific controlled character.
func (b *Bot) sheetText(playerID, arg string) string {
	names := b.session.State.PlayerCharacterNames(playerID)
	if len(names) == 0 {
		return "You haven't picked a character yet. See /party then /pick <name>."
	}
	active := b.session.State.PlayerCharacterName(playerID)
	target := strings.TrimSpace(arg)
	if target == "" {
		target = active
	} else {
		ok := false
		for _, n := range names {
			if strings.EqualFold(n, target) {
				target, ok = n, true
				break
			}
		}
		if !ok {
			return "You don't control “" + target + "”. You play: " + strings.Join(names, ", ")
		}
	}
	header := ""
	if len(names) > 1 {
		labeled := make([]string, len(names))
		for i, n := range names {
			if strings.EqualFold(n, active) {
				n = "⭐ " + n
			}
			labeled[i] = n
		}
		header = "You control: " + strings.Join(labeled, ", ") + "\n(/me <name> for another · /as <name> to switch active)\n\n"
	}
	for _, c := range b.session.State.PartySnapshot() {
		if strings.EqualFold(c.Name, target) {
			return header + engine.FormatCharacter(&c)
		}
	}
	return "Character not found: " + target
}

func (b *Bot) roundStatus() string {
	pending := b.session.State.PendingPlayers()
	if len(pending) == 0 {
		return "Action recorded. Everyone has acted — a player can call /dm to let the DM narrate."
	}
	return fmt.Sprintf("Action recorded. Waiting on: %s (or /dm to resolve now).", strings.Join(pending, ", "))
}

func rollText(notation string) string {
	if notation == "" {
		notation = "1d20"
	}
	roll, err := engine.RollDice(notation)
	if err != nil {
		return "⚠ " + err.Error()
	}
	msg := fmt.Sprintf("🎲 %s: %s", roll.String(), roll.ResultString())
	if roll.IsCriticalHit() {
		msg += " — CRIT!"
	} else if roll.IsCriticalFail() {
		msg += " — FUMBLE!"
	}
	return msg
}

func (b *Bot) save() {
	b.saveMu.Lock()
	defer b.saveMu.Unlock()
	if err := b.store.SaveSession(b.session.State); err != nil {
		log.Printf("save: %v", err)
	}
}

func (b *Bot) event(text string) {
	if b.onEvent != nil {
		b.onEvent(text)
	}
}

func (b *Bot) reply(m *tgbotapi.Message, text string) { b.send(m.Chat.ID, text) }

// send posts text to a chat, splitting messages that exceed Telegram's limit.
// sendZoneMap sends the current zone's map image to the chat (issue #24). Only
// the party's current zone is exposed, to avoid revealing unexplored areas.
func (b *Bot) sendZoneMap(m *tgbotapi.Message) {
	// Read the location under the session mutex — a /dm turn may be writing it in
	// a background goroutine (Adventure itself is immutable, safe to read).
	zoneID, _ := b.session.State.Location()
	zone := b.session.Adventure.Zone(zoneID)
	if zone == nil {
		b.reply(m, "The party isn't in a known zone yet.")
		return
	}
	rel := b.session.Adventure.ZoneMap(zone)
	if rel == "" {
		b.reply(m, "There's no map for "+zone.Name+".")
		return
	}
	abs, err := b.store.ResolveImagePath(b.session.Adventure.ID, rel)
	if err != nil {
		b.reply(m, "The map for "+zone.Name+" is unavailable.")
		return
	}
	photo := tgbotapi.NewPhoto(m.Chat.ID, tgbotapi.FilePath(abs))
	photo.Caption = "🗺 " + zone.Name
	if _, err := b.api.Send(photo); err != nil {
		log.Printf("send map: %v", err)
		b.reply(m, "Couldn't send the map image.")
	}
}

// sendNPCArt sends an NPC's portrait to the chat (issue #27). Only NPCs the
// party has already MET are shown, so unmet NPCs aren't revealed (spoilers).
func (b *Bot) sendNPCArt(m *tgbotapi.Message, arg string) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		b.reply(m, "Usage: /portrait <npc name or id>")
		return
	}
	adv := b.session.Adventure
	npc := adv.NPC(arg)
	if npc == nil {
		for i := range adv.NPCs {
			if strings.EqualFold(adv.NPCs[i].Name, arg) {
				npc = &adv.NPCs[i]
				break
			}
		}
	}
	if npc == nil || !b.session.State.NPCKnown(npc.ID) {
		b.reply(m, "You haven't met anyone by that name yet.")
		return
	}
	imgs := adv.NPCImages(npc)
	if len(imgs) == 0 {
		b.reply(m, "There's no portrait for "+npc.Name+".")
		return
	}
	abs, err := b.store.ResolveImagePath(adv.ID, imgs[0])
	if err != nil {
		b.reply(m, "The portrait for "+npc.Name+" is unavailable.")
		return
	}
	photo := tgbotapi.NewPhoto(m.Chat.ID, tgbotapi.FilePath(abs))
	photo.Caption = npc.Name
	if _, err := b.api.Send(photo); err != nil {
		log.Printf("send portrait: %v", err)
		b.reply(m, "Couldn't send the portrait image.")
	}
}

func (b *Bot) send(chatID int64, text string) {
	const limit = 4000
	for _, chunk := range splitMessage(text, limit) {
		if _, err := b.api.Send(tgbotapi.NewMessage(chatID, chunk)); err != nil {
			log.Printf("send: %v", err)
			return
		}
	}
}

// sendNarration sends a virtual-DM narration, hiding the trailing "suggested
// actions" list behind a Telegram spoiler so players aren't spoiled by options
// they haven't discovered yet (#63). The narrative is sent as plain text (no
// parse mode, so nothing to escape/break); the actions, when present, follow in
// a separate HTML message wrapped in <tg-spoiler>.
func (b *Bot) sendNarration(chatID int64, text string) {
	narr, heading, actions := domain.SplitActions(text)
	if actions == "" {
		b.send(chatID, text) // no actions section detected → send verbatim
		return
	}
	if narr != "" {
		b.send(chatID, narr)
	}
	for _, msg := range spoilerMessages(heading, actions, 3500) {
		m := tgbotapi.NewMessage(chatID, msg)
		m.ParseMode = tgbotapi.ModeHTML
		if _, err := b.api.Send(m); err != nil {
			log.Printf("send spoiler: %v", err)
			return
		}
	}
}

// spoilerMessages builds the HTML message(s) for a heading + spoiler-wrapped
// body: the heading (bold) on the first message only, and each body chunk wrapped
// in its own <tg-spoiler> so a spoiler tag is never split across the message
// limit. Content is HTML-escaped.
func spoilerMessages(heading, body string, limit int) []string {
	label := ""
	if heading != "" {
		label = "<b>" + htmlEscape(heading) + "</b>\n"
	}
	var out []string
	for i, chunk := range splitMessage(body, limit) {
		head := ""
		if i == 0 {
			head = label
		}
		out = append(out, head+"<tg-spoiler>"+htmlEscape(chunk)+"</tg-spoiler>")
	}
	return out
}

// htmlEscape escapes the characters significant to Telegram's HTML parse mode.
func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func splitMessage(text string, limit int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return []string{"(empty)"}
	}
	var out []string
	for len(text) > limit {
		cut := strings.LastIndex(text[:limit], "\n")
		if cut <= 0 {
			// No newline to break on: cut at the limit but back up to a UTF-8 rune
			// boundary so a multibyte character isn't split.
			cut = limit
			for cut > 0 && !utf8.RuneStart(text[cut]) {
				cut--
			}
			if cut == 0 {
				cut = limit
			}
		}
		out = append(out, text[:cut])
		text = strings.TrimLeft(text[cut:], "\n")
	}
	if text != "" {
		out = append(out, text)
	}
	return out
}

func displayName(u *tgbotapi.User) string {
	if u.FirstName != "" {
		return u.FirstName
	}
	if u.UserName != "" {
		return u.UserName
	}
	return "Player" + strconv.FormatInt(u.ID, 10)
}

const notStartedMsg = "The game hasn't started yet. Pick characters with /pick, then a player runs /begin and the DM sets the scene."

const helpText = `thAImaturgy — multiplayer DM bot
/party — list characters and who plays them
/roster — list the persistent campaign roster
/pick <name> — claim a character to play (repeat to control several)
/as <name> — choose which of your characters is active
/assign @user <name> — assign a character to a player (or reply to them with /assign <name>)
/me [name] — show your character sheet (or one of your characters)
/begin — start the game (the DM sets the opening scene)
/chat [name:] <line> — say something in character (context for the DM, not an action)
/meta <text> — ask the DM a question or note a correction (out of character)
/do [name:] <action> — declare an action this round (name: picks which of your characters)
/dm — let the AI Dungeon Master resolve the round and narrate (after /begin)
/roll <dice> — roll dice (e.g. 2d6+3)
/save — save the current session
/status — where the party is and session progress
/map — show the map of the current zone
/portrait <npc> — show a met NPC's portrait
/log [n] — show the last n timeline entries (default 15)
/rest short|long [character] — take a short or long rest
/hp -5 | +3 | =10 — damage, heal, or set your HP
/condition <name> — apply a condition to your character
/uncondition <name> — remove a condition
/gold +50 | -10 | =100 — adjust or set your gold
/xp <n> — award your character experience
/item add|remove <name> [xN] — edit your inventory
/setnote <text> — set your character's notes
/quests — list tracked quests
/recap — a quick "previously on…" recap of the session so far
/note <text> — add a note to the timeline
/chatid — show this chat's id
/help — this help`

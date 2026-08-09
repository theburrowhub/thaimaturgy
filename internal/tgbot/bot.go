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
	// OnEvent, if set, is called with short human-readable activity lines (player
	// joins, actions, narration) so a host UI can mirror what happens in the chat.
	OnEvent func(string)
}

// Bot hosts a multiplayer virtual-DM session over Telegram.
type Bot struct {
	api     *tgbotapi.BotAPI
	store   *storage.Storage
	session *domain.Session
	oracle  *engine.Oracle
	chatID  int64
	onEvent func(string)

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
	return &Bot{
		api:     api,
		store:   store,
		session: session,
		oracle:  oracle,
		chatID:  opts.ChatID,
		onEvent: opts.OnEvent,
	}, nil
}

// Username returns the bot's @username (for display).
func (b *Bot) Username() string { return b.api.Self.UserName }

// Run processes updates until ctx is cancelled. Call it directly (blocking) for
// the standalone binary, or in a goroutine with a cancellable context for the
// in-app host; Stop cancels the receive loop.
func (b *Bot) Run(ctx context.Context) {
	b.runCtx = ctx
	log.Printf("thaimaturgy-bot online as @%s — adventure %q, session %q", b.api.Self.UserName, b.session.Adventure.ID, b.session.State.Name)
	if b.chatID == 0 {
		log.Printf("WARNING: chat id not set — any chat that finds this bot can play and trigger LLM turns; set a chat id to restrict.")
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
	if b.chatID != 0 && m.Chat.ID != b.chatID {
		return // ignore other chats
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
		b.reply(m, fmt.Sprintf("This chat's id is: %d\nUse it as the chat id to restrict the bot to this chat.", m.Chat.ID))
	case "party":
		b.reply(m, b.partyText())
	case "pick", "play":
		name, err := b.session.State.ClaimCharacter(playerID, display, arg)
		if err != nil {
			b.reply(m, "⚠ "+err.Error())
			return
		}
		b.save()
		b.event(fmt.Sprintf("%s picked %s", display, name))
		b.reply(m, fmt.Sprintf("%s now plays %s. Declare actions with /do.", display, name))
	case "assign":
		b.assign(m)
	case "me":
		b.reply(m, b.sheetText(playerID))
	case "do":
		if !b.session.State.GameStarted() {
			b.reply(m, notStartedMsg)
			return
		}
		if arg == "" {
			b.reply(m, "Usage: /do <what your character does>")
			return
		}
		if _, err := b.session.State.SubmitAction(playerID, arg); err != nil {
			b.reply(m, "⚠ "+err.Error())
			return
		}
		b.save()
		b.event(fmt.Sprintf("%s: %s", display, arg))
		b.reply(m, b.roundStatus())
	case "dm", "narrate":
		if !b.session.State.GameStarted() {
			b.reply(m, notStartedMsg)
			return
		}
		b.runDM(m)
	case "roll":
		b.reply(m, rollText(arg))
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
// These stay available in the desktop DM console; multiplayer commands
// (begin/pick/assign/me/do/dm) are handled explicitly above.
var playerSafeCommands = map[string]bool{
	"status": true, // where the party is + progress counters (no DM notes)
	"quests": true, "quest": true, // player-facing quest log
	"note": true, // benign: append a note to the timeline
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
	mutating := cmd == "note"
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
		b.send(m.Chat.ID, resp.Answer)
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
		b.send(m.Chat.ID, resp.Answer)
	}()
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

func (b *Bot) sheetText(playerID string) string {
	name := b.session.State.PlayerCharacterName(playerID)
	if name == "" {
		return "You haven't picked a character yet. See /party then /pick <name>."
	}
	for _, c := range b.session.State.PartySnapshot() {
		if strings.EqualFold(c.Name, name) {
			return engine.FormatCharacter(&c)
		}
	}
	return "Character not found: " + name
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
func (b *Bot) send(chatID int64, text string) {
	const limit = 4000
	for _, chunk := range splitMessage(text, limit) {
		if _, err := b.api.Send(tgbotapi.NewMessage(chatID, chunk)); err != nil {
			log.Printf("send: %v", err)
			return
		}
	}
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
/pick <name> — claim a character to play
/assign @user <name> — assign a character to a player (or reply to them with /assign <name>)
/me — show your character sheet
/begin — start the game (the DM sets the opening scene)
/do <action> — declare your character's action this round (after /begin)
/dm — let the AI Dungeon Master resolve the round and narrate (after /begin)
/roll <dice> — roll dice (e.g. 2d6+3)
/save — save the current session
/status — where the party is and session progress
/log [n] — show the last n timeline entries (default 15)
/quests — list tracked quests
/note <text> — add a note to the timeline
/chatid — show this chat's id
/help — this help`

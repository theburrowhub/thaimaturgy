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
	if m.IsCommand() {
		b.handleCommand(m)
	}
}

func (b *Bot) handleCommand(m *tgbotapi.Message) {
	playerID := strconv.FormatInt(m.From.ID, 10)
	display := displayName(m.From)
	arg := strings.TrimSpace(m.CommandArguments())

	switch m.Command() {
	case "start", "help":
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
	case "me":
		b.reply(m, b.sheetText(playerID))
	case "do":
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
		b.runDM(m)
	case "roll":
		b.reply(m, rollText(arg))
	default:
		b.reply(m, "Unknown command. "+helpText)
	}
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
		timeout := time.Duration(b.session.Config.RequestTimeoutSeconds) * time.Second
		if timeout <= 0 {
			timeout = 90 * time.Second
		}
		base := b.runCtx
		if base == nil {
			base = context.Background()
		}
		ctx, cancel := context.WithTimeout(base, timeout)
		defer cancel()

		resp := b.oracle.RunGroupTurn(ctx)
		// If the host was torn down (Run ctx cancelled) mid-turn, don't save or
		// post to a game that's been abandoned.
		if base.Err() != nil {
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
	var sb strings.Builder
	sb.WriteString("Party — pick one with /pick <name>:\n")
	for i := range party {
		c := party[i]
		line := fmt.Sprintf("• %s — Lvl %d %s %s", c.Name, c.Level, c.Race, c.Class)
		if who, ok := controllers[c.Name]; ok {
			line += " — played by " + who
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

const helpText = `thAImaturgy — multiplayer DM bot
/party — list characters and who plays them
/pick <name> — claim a character to play
/me — show your character sheet
/do <action> — declare your character's action this round
/dm — let the AI Dungeon Master resolve the round and narrate
/roll <dice> — roll dice (e.g. 2d6+3)
/chatid — show this chat's id
/help — this help`

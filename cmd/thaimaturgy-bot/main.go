// Command thaimaturgy-bot runs a Telegram bot that hosts a multiplayer,
// AI-Dungeon-Master session of an adventure. Each player claims a party member
// and declares their action with /do; a player triggers the DM with /dm, and the
// bot posts the AI's narration back to the chat. It reuses the same internal/
// core as the desktop app (domain, storage, engine, providers).
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/theburrowhub/thaimaturgy/internal/auth"
	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/engine"
	"github.com/theburrowhub/thaimaturgy/internal/mcpserve"
	"github.com/theburrowhub/thaimaturgy/internal/mcptools"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

func main() {
	// When invoked as the MCP tools subprocess (by the oracle's Claude-CLI backend),
	// serve the session tools over stdio and exit — never start the bot.
	if len(os.Args) > 1 && os.Args[1] == mcptools.SubcommandArg {
		if err := mcpserve.RunSubcommand(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "mcp-tools:", err)
			os.Exit(1)
		}
		return
	}

	advID := flag.String("adventure", "", "adventure id to run (required)")
	sessionName := flag.String("session", "", "session name (default: <adventure>-telegram)")
	token := flag.String("token", "", "Telegram bot token (or THAIM_TELEGRAM_TOKEN)")
	chatID := flag.Int64("chat", 0, "restrict to this chat id (0 = any chat)")
	flag.Parse()

	if *token == "" {
		*token = os.Getenv("THAIM_TELEGRAM_TOKEN")
	}
	if *token == "" || *advID == "" {
		fmt.Fprintln(os.Stderr, "usage: thaimaturgy-bot -adventure <id> [-session <name>] [-chat <id>] (token via -token or THAIM_TELEGRAM_TOKEN)")
		os.Exit(2)
	}

	b, err := newBot(*token, *advID, *sessionName, *chatID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bot: %v\n", err)
		os.Exit(1)
	}
	b.run()
}

type bot struct {
	api     *tgbotapi.BotAPI
	store   *storage.Storage
	config  *domain.Config
	session *domain.Session
	oracle  *engine.Oracle
	name    string
	chatID  int64 // 0 = any

	mu        sync.Mutex // guards resolving
	resolving bool       // a /dm turn is in flight
	saveMu    sync.Mutex // serializes session file writes across goroutines
}

func newBot(token, advID, sessionName string, chatID int64) (*bot, error) {
	store, err := storage.New()
	if err != nil {
		return nil, err
	}
	_ = store.LoadEnvFile()
	config, err := store.LoadConfig()
	if err != nil {
		return nil, err
	}
	auth.AutoConfigure(config)
	if config.RunModel != "" {
		config.Model = config.RunModel
	}
	if !config.IsConfigured() {
		return nil, fmt.Errorf("no AI credentials found; set an API key or log in with Claude Code / Gemini")
	}

	adv, err := store.LoadAdventure(advID)
	if err != nil {
		return nil, err
	}
	if sessionName == "" {
		sessionName = advID + "-telegram"
	}

	var state *domain.SessionState
	if store.SessionExists(sessionName) {
		if state, err = store.LoadSession(sessionName); err != nil {
			return nil, err
		}
	} else {
		state = domain.NewSessionState(sessionName, adv)
	}
	// Multiplayer runs the AI as DM; ensure the party exists to pick from.
	state.SetMode(domain.ModeVirtualDM)
	state.EnsureParty()

	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	session := domain.NewSession(state, adv, config)
	b := &bot{
		api:     api,
		store:   store,
		config:  config,
		session: session,
		oracle:  engine.NewOracle(session, providers.New(config)),
		name:    sessionName,
		chatID:  chatID,
	}
	_ = store.SaveSession(state)
	return b, nil
}

func (b *bot) run() {
	log.Printf("thaimaturgy-bot online as @%s — adventure %q, session %q", b.api.Self.UserName, b.session.Adventure.ID, b.name)
	if b.chatID == 0 {
		log.Printf("WARNING: -chat not set — any chat that finds this bot can play and trigger LLM turns; set -chat <id> to restrict.")
	}
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30
	for update := range b.api.GetUpdatesChan(u) {
		msg := update.Message
		if msg == nil || msg.From == nil {
			continue
		}
		if b.chatID != 0 && msg.Chat.ID != b.chatID {
			continue // ignore other chats
		}
		if msg.IsCommand() {
			b.handleCommand(msg)
		}
	}
}

func (b *bot) handleCommand(m *tgbotapi.Message) {
	playerID := strconv.FormatInt(m.From.ID, 10)
	display := displayName(m.From)
	arg := strings.TrimSpace(m.CommandArguments())

	switch m.Command() {
	case "start", "help":
		b.reply(m, helpText)
	case "party":
		b.reply(m, b.partyText())
	case "pick", "play":
		name, err := b.session.State.ClaimCharacter(playerID, display, arg)
		if err != nil {
			b.reply(m, "⚠ "+err.Error())
			return
		}
		b.save()
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
func (b *bot) runDM(m *tgbotapi.Message) {
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
	go func() {
		defer func() {
			b.mu.Lock()
			b.resolving = false
			b.mu.Unlock()
		}()
		timeout := time.Duration(b.config.RequestTimeoutSeconds) * time.Second
		if timeout <= 0 {
			timeout = 90 * time.Second
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		resp := b.oracle.RunGroupTurn(ctx)
		if resp.Error != nil {
			// Log the detail; don't leak provider/paths to the chat.
			log.Printf("group turn: %v", resp.Error)
			b.send(m.Chat.ID, "⚠ The DM couldn't resolve the turn right now. Try /dm again in a moment.")
			return
		}
		b.save()
		b.send(m.Chat.ID, resp.Answer)
	}()
}

func (b *bot) partyText() string {
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

func (b *bot) sheetText(playerID string) string {
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

func (b *bot) roundStatus() string {
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

// save persists the session, serialized so the /dm goroutine and the update loop
// can't write the same session file concurrently (which could corrupt it). The
// in-memory marshal itself is already synchronized by SessionState.MarshalJSON.
func (b *bot) save() {
	b.saveMu.Lock()
	defer b.saveMu.Unlock()
	if err := b.store.SaveSession(b.session.State); err != nil {
		log.Printf("save: %v", err)
	}
}

func (b *bot) reply(m *tgbotapi.Message, text string) { b.send(m.Chat.ID, text) }

// send posts text to a chat, splitting messages that exceed Telegram's limit.
func (b *bot) send(chatID int64, text string) {
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
			// boundary so a multibyte character (e.g. ó/ñ in Spanish) isn't split.
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
/help — this help`

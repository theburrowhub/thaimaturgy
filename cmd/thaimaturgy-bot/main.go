// Command thaimaturgy-bot runs the Telegram multiplayer bot standalone. It boots
// the shared internal/ core (config, adventure, session, provider), then hands a
// live virtual-DM session to internal/tgbot. The desktop app hosts the same bot
// in-process; this binary is for running it headless.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/theburrowhub/thaimaturgy/internal/auth"
	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/engine"
	"github.com/theburrowhub/thaimaturgy/internal/mcpserve"
	"github.com/theburrowhub/thaimaturgy/internal/mcptools"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
	"github.com/theburrowhub/thaimaturgy/internal/tgbot"
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
	token := flag.String("token", "", "Telegram bot token (overrides env/config)")
	chatID := flag.Int64("chat", 0, "restrict to this chat id (overrides config; 0 = any)")
	flag.Parse()

	if err := run(*advID, *sessionName, *token, *chatID); err != nil {
		fmt.Fprintf(os.Stderr, "bot: %v\n", err)
		os.Exit(1)
	}
}

func run(advID, sessionName, token string, chatID int64) error {
	store, err := storage.New()
	if err != nil {
		return err
	}
	_ = store.LoadEnvFile()
	config, err := store.LoadConfig()
	if err != nil {
		return err
	}
	auth.AutoConfigure(config)
	if config.RunModel != "" {
		config.Model = config.RunModel
	}
	if !config.IsConfigured() {
		return fmt.Errorf("no AI credentials found; set an API key or log in with Claude Code / Gemini")
	}

	// Token precedence: flag > env > config.
	if token == "" {
		token = os.Getenv("THAIM_TELEGRAM_TOKEN")
	}
	if token == "" {
		token = config.TelegramToken
	}
	if token == "" {
		return fmt.Errorf("no Telegram token: pass -token, set THAIM_TELEGRAM_TOKEN, or configure it")
	}
	if chatID == 0 {
		chatID = config.TelegramChatID
	}
	if advID == "" {
		return fmt.Errorf("-adventure <id> is required")
	}

	adv, err := store.LoadAdventure(advID)
	if err != nil {
		return err
	}
	if sessionName == "" {
		sessionName = advID + "-telegram"
	}

	var state *domain.SessionState
	if store.SessionExists(sessionName) {
		if state, err = store.LoadSession(sessionName); err != nil {
			return err
		}
	} else {
		state = domain.NewSessionState(sessionName, adv)
	}
	state.SetMode(domain.ModeVirtualDM)
	state.EnsureParty()

	session := domain.NewSession(state, adv, config)
	oracle := engine.NewOracle(session, providers.New(config))
	if err := store.SaveSession(state); err != nil {
		return err
	}

	bot, err := tgbot.New(store, session, oracle, tgbot.Options{Token: token, ChatID: chatID, AllowedUsers: config.TelegramAllowedUsers})
	if err != nil {
		return err
	}
	bot.Run(context.Background()) // blocks until interrupted
	return nil
}

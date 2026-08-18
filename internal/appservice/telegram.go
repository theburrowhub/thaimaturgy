package appservice

import (
	"context"
	"errors"
	"fmt"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/tgbot"
)

// ErrSessionHosted is returned by the turn drivers (AskOracle, ExecuteCommand)
// and mutating helpers while a session is hosted on Telegram. The hosted bot
// drives the same Oracle/Session under its OWN lock (not this session's opMu),
// so allowing a second driver would race it; instead the server rejects turns
// from other clients until hosting stops — exactly as the local GUI pauses its
// own input while it hosts the bot in-process.
var ErrSessionHosted = errors.New("session is hosted on Telegram; stop hosting to drive it from here")

// ErrNoTelegramToken is returned by StartTelegramHost when the server config has
// no Telegram bot token. The token is set (write-only) via SaveConfig/Settings.
var ErrNoTelegramToken = errors.New("no Telegram bot token configured (set it in Settings)")

// ErrAlreadyHosting is returned by StartTelegramHost when the session is already
// hosting a Telegram bot.
var ErrAlreadyHosting = errors.New("session is already hosted on Telegram")

// ErrNotVirtualDM is returned by StartTelegramHost when the session is not in
// virtual-DM mode; the Telegram host runs a virtual-DM game for a party.
var ErrNotVirtualDM = errors.New("switch to virtual-DM mode before hosting on Telegram")

// TelegramStatus reports whether a session is currently hosted on Telegram and,
// if so, the bot's @username. An unknown/closed session reports not hosting.
type TelegramStatus struct {
	Hosting  bool   `json:"hosting"`
	Username string `json:"username,omitempty"`
}

// StartTelegramHost binds a Telegram bot to an open session and starts its
// receive loop, so a remote client (GUI or web) can host a multiplayer game the
// same way the local GUI does in-process. The bot token, chat id, and allowed
// users come from the server config. Requires virtual-DM mode. While hosting,
// the server rejects oracle/command turns from other clients (see
// ErrSessionHosted) so the bot is the sole driver. Returns the bot's @username.
func (s *Service) StartTelegramHost(name string) (string, error) {
	os, ok := s.Get(name)
	if !ok {
		return "", fmt.Errorf("session %q is not open", name)
	}
	cfg := s.Config()
	if cfg.TelegramToken == "" {
		return "", ErrNoTelegramToken
	}

	// Build the bot OUTSIDE opMu: tgbot.New performs a network round-trip
	// (getMe) to validate the token, and we must not hold the session lock (which
	// a close/turn contends for) across it. It touches State only through
	// State's own mutex, so it's safe against a concurrent in-flight turn.
	bot, err := tgbot.New(s.store, os.Session, os.Oracle, tgbot.Options{
		Token:        cfg.TelegramToken,
		ChatID:       cfg.TelegramChatID,
		AllowedUsers: cfg.TelegramAllowedUsers,
	})
	if err != nil {
		return "", fmt.Errorf("start Telegram host: %w", err)
	}

	os.opMu.Lock()
	if os.closed {
		os.opMu.Unlock()
		return "", fmt.Errorf("session %q is not open", name)
	}
	if os.tg != nil {
		os.opMu.Unlock()
		return "", ErrAlreadyHosting
	}
	if os.Session.State.EffectiveMode() != domain.ModeVirtualDM {
		os.opMu.Unlock()
		return "", ErrNotVirtualDM
	}
	os.Session.State.EnsureParty()
	ctx, cancel := context.WithCancel(context.Background())
	os.tg = bot
	os.tgCancel = cancel
	os.opMu.Unlock()

	go bot.Run(ctx)
	// Persist EnsureParty / MarkStarted mutations so a viewer's next refresh and
	// the on-disk session reflect that hosting began.
	s.Autosave(name)
	return bot.Username(), nil
}

// StopTelegramHost stops a session's Telegram host if one is running. It clears
// the host under opMu (so a queued turn immediately sees the session as free),
// then cancels the run context and waits for any in-flight bot turn OUTSIDE the
// lock, so a slow turn can't block other operations on the session. It is a
// no-op (nil) for a session that isn't hosting or isn't open.
func (s *Service) StopTelegramHost(name string) error {
	os, ok := s.Get(name)
	if !ok {
		return nil
	}
	os.opMu.Lock()
	bot, cancel := os.tg, os.tgCancel
	os.tg, os.tgCancel = nil, nil
	os.opMu.Unlock()
	if bot == nil {
		return nil
	}
	if cancel != nil {
		cancel() // abort an in-flight turn first…
	}
	bot.Stop() // …then wait for it to unwind.
	s.Autosave(name)
	return nil
}

// TelegramHostStatus reports whether a session is hosted on Telegram.
func (s *Service) TelegramHostStatus(name string) TelegramStatus {
	os, ok := s.Get(name)
	if !ok {
		return TelegramStatus{}
	}
	os.opMu.Lock()
	defer os.opMu.Unlock()
	if os.tg == nil {
		return TelegramStatus{}
	}
	return TelegramStatus{Hosting: true, Username: os.tg.Username()}
}

// stopHostLocked stops a running Telegram host while the caller already holds
// opMu (used by CloseSession). It cancels the in-flight turn and waits for it to
// unwind under the lock — acceptable on the close path, where the session is
// being torn down anyway.
func (o *OpenSession) stopHostLocked() {
	if o.tg == nil {
		return
	}
	bot, cancel := o.tg, o.tgCancel
	o.tg, o.tgCancel = nil, nil
	if cancel != nil {
		cancel()
	}
	bot.Stop()
}

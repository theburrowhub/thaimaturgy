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

// ErrAlreadyHosting is returned by StartTelegramHost when a Telegram bot is
// already hosting a session. Only one host runs at a time server-wide because
// the server has a single bot token (one getUpdates consumer per bot).
var ErrAlreadyHosting = errors.New("a session is already hosted on Telegram")

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
// users come from the server config. Requires virtual-DM mode. Only ONE session
// may host at a time (single server-wide bot token), so a second start is
// rejected with ErrAlreadyHosting. While hosting, the server rejects
// oracle/command turns from other clients (see ErrSessionHosted) so the bot is
// the sole driver. Returns the bot's @username.
func (s *Service) StartTelegramHost(name string) (string, error) {
	os, ok := s.Get(name)
	if !ok {
		return "", fmt.Errorf("session %q is not open", name)
	}
	cfg := s.Config()
	if cfg.TelegramToken == "" {
		return "", ErrNoTelegramToken
	}

	// hostMu serializes the whole lifecycle across sessions, so exactly one host
	// exists server-wide. Holding it across tgbot.New's network round-trip is
	// fine: it blocks only other host start/stop, never oracle/command turns.
	s.hostMu.Lock()
	defer s.hostMu.Unlock()
	if s.hostName == name {
		if h, u := os.hostSnapshot(); h {
			return u, nil // idempotent: already hosting this session
		}
	}
	if s.hostName != "" {
		return "", fmt.Errorf("%w: session %q holds the bot", ErrAlreadyHosting, s.hostName)
	}

	// Cheap early-out (no mutation) so a wrong-mode/closed session doesn't cost a
	// Telegram network round-trip. The authoritative check is repeated below,
	// under the final opMu, right before the bot is installed.
	os.opMu.Lock()
	if os.closed {
		os.opMu.Unlock()
		return "", fmt.Errorf("session %q is not open", name)
	}
	if os.Session.State.EffectiveMode() != domain.ModeVirtualDM {
		os.opMu.Unlock()
		return "", ErrNotVirtualDM
	}
	os.opMu.Unlock()

	// Build the bot OUTSIDE opMu: tgbot.New performs a network round-trip (getMe)
	// to validate the token. It touches State only through State's own mutex.
	bot, err := tgbot.New(s.store, os.Session, os.Oracle, tgbot.Options{
		Token:        cfg.TelegramToken,
		ChatID:       cfg.TelegramChatID,
		AllowedUsers: cfg.TelegramAllowedUsers,
	})
	if err != nil {
		return "", fmt.Errorf("start Telegram host: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Re-validate EVERYTHING under the final opMu before installing the bot: a
	// concurrent turn during the network round-trip above could have closed the
	// session or switched it out of virtual-DM mode (os.tg was still nil then, so
	// nothing blocked it). Checking here, in the same critical section as the
	// assignment, closes that window — no /mode can interleave between the check
	// and os.tg being set.
	os.opMu.Lock()
	if os.closed {
		os.opMu.Unlock()
		cancel()
		return "", fmt.Errorf("session %q is not open", name)
	}
	if os.Session.State.EffectiveMode() != domain.ModeVirtualDM {
		os.opMu.Unlock()
		cancel()
		return "", ErrNotVirtualDM
	}
	os.Session.State.EnsureParty()
	os.tg = bot
	os.tgCancel = cancel
	os.opMu.Unlock()
	s.hostName = name

	go bot.Run(ctx)
	// Persist EnsureParty / MarkStarted mutations so a viewer's next refresh and
	// the on-disk session reflect that hosting began.
	s.Autosave(name)
	return bot.Username(), nil
}

// StopTelegramHost stops the Telegram host if the named session is the current
// host (a no-op otherwise). It cancels the run context and waits for any
// in-flight bot turn to unwind BEFORE clearing the session's host marker, so no
// other driver can slip in while the old bot is still running; hostMu is held
// throughout so a concurrent start/stop/close can't interleave.
func (s *Service) StopTelegramHost(name string) error {
	s.hostMu.Lock()
	defer s.hostMu.Unlock()
	s.stopHostLocked(name)
	return nil
}

// stopHostLocked stops the current Telegram host if it is `name` (or if name is
// "", the host whatever it is). The caller MUST hold s.hostMu. It keeps the
// session's host marker set while it cancels and waits for the bot (so the turn
// drivers still see the session as hosted and reject other clients), then clears
// the marker atomically once the bot has fully stopped.
func (s *Service) stopHostLocked(name string) {
	if s.hostName == "" || (name != "" && s.hostName != name) {
		return
	}
	hosted := s.hostName
	os, ok := s.Get(hosted)
	if !ok {
		s.hostName = ""
		return
	}
	os.opMu.Lock()
	bot, cancel := os.tg, os.tgCancel
	os.opMu.Unlock()
	// os.tg stays set here on purpose: the session still reads as hosted while the
	// bot's in-flight turn unwinds, so AskOracle/ExecuteCommand keep rejecting.
	if cancel != nil {
		cancel() // abort an in-flight turn first…
	}
	if bot != nil {
		bot.Stop() // …then wait for it to unwind, OUTSIDE opMu.
	}
	os.opMu.Lock()
	os.tg, os.tgCancel = nil, nil
	os.opMu.Unlock()
	s.hostName = ""
	s.Autosave(hosted)
}

// TelegramHostStatus reports whether a session is hosted on Telegram.
func (s *Service) TelegramHostStatus(name string) TelegramStatus {
	os, ok := s.Get(name)
	if !ok {
		return TelegramStatus{}
	}
	hosting, username := os.hostSnapshot()
	return TelegramStatus{Hosting: hosting, Username: username}
}

// hostSnapshot returns a consistent (hosting, @username) pair read in a SINGLE
// opMu critical section, so a concurrent StopTelegramHost clearing os.tg can't
// yield a torn {hosting:true, username:""} snapshot.
func (o *OpenSession) hostSnapshot() (bool, string) {
	o.opMu.Lock()
	defer o.opMu.Unlock()
	if o.tg == nil {
		return false, ""
	}
	return true, o.tg.Username()
}

package appservice

import (
	"context"
	"errors"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/tgbot"
)

// TestStartTelegramHostNoToken: hosting is refused when the server config has no
// Telegram bot token, before any bot is constructed.
func TestStartTelegramHostNoToken(t *testing.T) {
	svc, _ := newService(t) // DefaultConfig has no Telegram token
	name, err := svc.NewSession("crypt")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if _, err := svc.StartTelegramHost(name); !errors.Is(err, ErrNoTelegramToken) {
		t.Fatalf("StartTelegramHost without a token = %v; want ErrNoTelegramToken", err)
	}
}

// TestTelegramHostStatusAndStopWhenIdle: status/stop on a session that isn't
// hosting are safe no-ops.
func TestTelegramHostStatusAndStopWhenIdle(t *testing.T) {
	svc, _ := newService(t)
	name, err := svc.NewSession("crypt")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if st := svc.TelegramHostStatus(name); st.Hosting {
		t.Fatalf("a fresh session reports hosting: %+v", st)
	}
	if err := svc.StopTelegramHost(name); err != nil {
		t.Fatalf("StopTelegramHost on an idle session = %v; want nil", err)
	}
	// Unknown session: status is empty, stop is a no-op.
	if st := svc.TelegramHostStatus("nope"); st.Hosting {
		t.Fatalf("unknown session reports hosting: %+v", st)
	}
	if err := svc.StopTelegramHost("nope"); err != nil {
		t.Fatalf("StopTelegramHost on an unknown session = %v; want nil", err)
	}
}

// TestStartTelegramHostSingleHost: because the server has ONE bot token (and
// Telegram allows one getUpdates consumer per bot), a second session's start is
// refused with ErrAlreadyHosting — before any bot is built (no Telegram
// network). The first host is faked at Service level.
func TestStartTelegramHostSingleHost(t *testing.T) {
	svc, _ := newService(t)
	cfg := svc.Config()
	cfg.TelegramToken = "dummy:token" // get past the no-token guard
	if err := svc.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	a, err := svc.NewSession("crypt")
	if err != nil {
		t.Fatalf("NewSession a: %v", err)
	}
	b, err := svc.NewSession("crypt")
	if err != nil {
		t.Fatalf("NewSession b: %v", err)
	}

	// Simulate session a already holding the server-wide host.
	osA, _ := svc.Get(a)
	svc.hostMu.Lock()
	svc.hostName = a
	svc.hostMu.Unlock()
	osA.opMu.Lock()
	osA.tg = &tgbot.Bot{}
	osA.opMu.Unlock()

	if _, err := svc.StartTelegramHost(b); !errors.Is(err, ErrAlreadyHosting) {
		t.Fatalf("second host = %v; want ErrAlreadyHosting", err)
	}

	// Clear the fake so nothing tears it down (a bare bot has no live api).
	svc.hostMu.Lock()
	svc.hostName = ""
	svc.hostMu.Unlock()
	osA.opMu.Lock()
	osA.tg = nil
	osA.opMu.Unlock()
}

// TestStartTelegramHostRequiresVirtualDM: hosting is refused (before any bot is
// built) unless the session is in virtual-DM mode.
func TestStartTelegramHostRequiresVirtualDM(t *testing.T) {
	svc, _ := newService(t)
	cfg := svc.Config()
	cfg.TelegramToken = "dummy:token"
	if err := svc.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	name, err := svc.NewSession("crypt") // fresh session defaults to oracle mode
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if _, err := svc.StartTelegramHost(name); !errors.Is(err, ErrNotVirtualDM) {
		t.Fatalf("start in non-DM mode = %v; want ErrNotVirtualDM", err)
	}
}

// TestHostedSessionRejectsTurns: while a session is hosted, the server's turn
// drivers (ExecuteCommand, AskOracle) and mutating helpers (SetParty via
// withOpenSession) reject other clients with ErrSessionHosted, so the bot is the
// sole driver of the shared Oracle — and turns resume once hosting clears. The
// host is faked (a bare *tgbot.Bot trips the `os.tg != nil` guard) so the test
// needs no Telegram network.
func TestHostedSessionRejectsTurns(t *testing.T) {
	svc, _ := newService(t)
	name, err := svc.NewSession("crypt")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	os, ok := svc.Get(name)
	if !ok {
		t.Fatalf("session %q not open", name)
	}

	// A harmless command succeeds before hosting.
	if res, err := svc.ExecuteCommand(name, "/note the gate is ajar"); err != nil || res == nil || !res.Success {
		t.Fatalf("pre-host command = %+v (%v); want success", res, err)
	}

	// Simulate an active host by installing a bare bot under opMu.
	os.opMu.Lock()
	os.tg = &tgbot.Bot{}
	os.opMu.Unlock()

	if _, err := svc.ExecuteCommand(name, "/note ignored"); !errors.Is(err, ErrSessionHosted) {
		t.Errorf("ExecuteCommand while hosted = %v; want ErrSessionHosted", err)
	}
	if _, err := svc.AskOracle(context.Background(), name, "what happens?"); !errors.Is(err, ErrSessionHosted) {
		t.Errorf("AskOracle while hosted = %v; want ErrSessionHosted", err)
	}
	if err := svc.SetParty(name, []*domain.Character{domain.NewCharacter("Alice", "Elf", "Wizard")}); !errors.Is(err, ErrSessionHosted) {
		t.Errorf("SetParty while hosted = %v; want ErrSessionHosted", err)
	}

	// Clearing the host re-opens the session to server-driven turns.
	os.opMu.Lock()
	os.tg = nil
	os.opMu.Unlock()
	if res, err := svc.ExecuteCommand(name, "/note back in control"); err != nil || res == nil || !res.Success {
		t.Fatalf("post-host command = %+v (%v); want success", res, err)
	}
}

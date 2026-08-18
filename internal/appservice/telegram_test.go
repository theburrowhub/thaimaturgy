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

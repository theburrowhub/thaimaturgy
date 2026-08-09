package engine

import (
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

func TestChatCommandLogsContext(t *testing.T) {
	session := createTestSession()
	h := NewCommandHandler(session)
	res := h.Execute(ParseCommand("/chat I greet the guard warmly"))
	if !res.Success {
		t.Fatalf("chat command failed: %+v", res)
	}
	last := session.State.RecentLog(1)
	if len(last) == 0 || last[0].Type != domain.LogChat {
		t.Errorf("expected a LogChat entry, got %+v", last)
	}
}

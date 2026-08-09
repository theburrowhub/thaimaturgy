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

func TestMetaCommandRoutesToOracleOOC(t *testing.T) {
	session := createTestSession()
	h := NewCommandHandler(session)
	res := h.Execute(ParseCommand("/meta can I use my reaction now?"))
	if !res.NeedsUI || res.UIAction != "oracle" {
		t.Fatalf("meta should route to the oracle UI action: %+v", res)
	}
	if !contains(res.UIArg, "OUT-OF-CHARACTER") || !contains(res.UIArg, "reaction now") {
		t.Errorf("meta UIArg not OOC-framed: %q", res.UIArg)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOfStr(s, sub) >= 0)
}
func indexOfStr(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

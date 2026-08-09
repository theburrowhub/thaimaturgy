package domain

import (
	"strings"
	"testing"
)

func TestAddChatRecordsInCharacterLineWithoutAction(t *testing.T) {
	adv := &Adventure{ID: "m", Title: "M", Zones: []Zone{{ID: "z", Rooms: []Room{{ID: "r"}}}}}
	st := NewSessionState("s", adv)
	before := st.LogLen()
	st.AddChat("Borin", "I don't trust this door.")
	if st.LogLen() != before+1 {
		t.Fatalf("chat not recorded (%d -> %d)", before, st.LogLen())
	}
	last := st.RecentLog(1)
	if len(last) != 1 || last[0].Type != LogChat || !strings.Contains(last[0].Message, "Borin: I don't trust") {
		t.Errorf("chat entry = %+v", last)
	}
	if len(st.RoundActions()) != 0 {
		t.Errorf("/chat must not create a round action")
	}
}

package tgbot

import "testing"

func TestAllowMessage(t *testing.T) {
	allowed := normalizeAllowedUsers([]string{"@Alice", " 12345 ", "bob"})

	cases := []struct {
		name            string
		chatID          int64
		allowed         map[string]bool
		msgChat, fromID int64
		username        string
		want            bool
	}{
		{"unrestricted", 0, nil, 999, 1, "nobody", true},
		{"allowed chat", -100, nil, -100, 5, "x", true},
		{"other chat blocked", -100, nil, -200, 5, "x", false},
		{"user by username other chat", -100, allowed, -200, 5, "alice", true},
		{"username @ and case normalized", -100, allowed, 777, 5, "@Alice", true},
		{"user by id (private)", 0, allowed, 777, 12345, "", true},
		{"non-allowed user blocked", -100, allowed, -200, 999, "eve", false},
		{"allowed chat overrides user filter", -100, allowed, -100, 999, "eve", true},
	}
	for _, c := range cases {
		if got := allowMessage(c.chatID, c.allowed, c.msgChat, c.fromID, c.username); got != c.want {
			t.Errorf("%s: allowMessage = %v, want %v", c.name, got, c.want)
		}
	}
}

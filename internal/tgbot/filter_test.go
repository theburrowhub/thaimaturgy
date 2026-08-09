package tgbot

import "testing"

func TestNormalizeAllowedUsersSplitsIDsFromUsernames(t *testing.T) {
	ids, ignored := normalizeAllowedUsers([]string{"12345", " 678 ", "@Alice", "bob", ""})
	if !ids["12345"] || !ids["678"] || len(ids) != 2 {
		t.Fatalf("numeric ids = %v; want {12345,678}", ids)
	}
	if len(ignored) != 2 { // @Alice, bob are usernames, ignored for auth
		t.Errorf("ignored usernames = %v; want 2", ignored)
	}
}

func TestAllowMessage(t *testing.T) {
	ids, _ := normalizeAllowedUsers([]string{"12345", "@Alice"}) // only 12345 is authoritative
	cases := []struct {
		name            string
		chatID          int64
		allowed         map[string]bool
		msgChat, fromID int64
		want            bool
	}{
		{"unrestricted", 0, nil, 999, 1, true},
		{"allowed chat", -100, nil, -100, 5, true},
		{"other chat blocked", -100, nil, -200, 5, false},
		{"allowed id private dm", 0, ids, 42, 12345, true},
		{"allowed id other chat", -100, ids, -200, 12345, true},
		{"non-allowed id blocked", -100, ids, -200, 999, false},
		{"allowed chat overrides id filter", -100, ids, -100, 999, true},
	}
	for _, c := range cases {
		if got := allowMessage(c.chatID, c.allowed, c.msgChat, c.fromID); got != c.want {
			t.Errorf("%s: allowMessage = %v, want %v", c.name, got, c.want)
		}
	}
}

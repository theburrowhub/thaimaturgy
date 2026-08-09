package tgbot

import "testing"

func TestNormalizeAllowedUsersSplitsIDsFromUsernames(t *testing.T) {
	// "@12345" is a USERNAME form (leading @) and must NOT become numeric id 12345.
	ids, ignored := normalizeAllowedUsers([]string{"12345", " 678 ", "@Alice", "bob", "@12345", ""})
	if !ids["12345"] || !ids["678"] || len(ids) != 2 {
		t.Fatalf("numeric ids = %v; want {12345,678}", ids)
	}
	if len(ignored) != 3 { // @Alice, bob, @12345 → all usernames, ignored for auth
		t.Errorf("ignored usernames = %v; want 3", ignored)
	}
}

func TestAllowMessage(t *testing.T) {
	idsOnly, _ := normalizeAllowedUsers([]string{"12345", "@Alice"}) // only 12345 authoritative
	unameOnly, unameIgnored := normalizeAllowedUsers([]string{"@Alice"})
	// A configured-but-invalid filter must still count as "configured" (fail closed).
	unameFilterSet := len(unameOnly) > 0 || len(unameIgnored) > 0

	cases := []struct {
		name            string
		chatID          int64
		allowed         map[string]bool
		filterSet       bool
		msgChat, fromID int64
		want            bool
	}{
		{"unrestricted", 0, nil, false, 999, 1, true},
		{"allowed chat", -100, nil, false, -100, 5, true},
		{"other chat blocked", -100, nil, false, -200, 5, false},
		{"allowed id private dm", 0, idsOnly, true, 42, 12345, true},
		{"allowed id other chat", -100, idsOnly, true, -200, 12345, true},
		{"non-allowed id blocked", -100, idsOnly, true, -200, 999, false},
		{"allowed chat overrides id filter", -100, idsOnly, true, -100, 999, true},
		// username-only allow-list, no chat id: fail CLOSED, not open.
		{"username-only fails closed", 0, unameOnly, unameFilterSet, 7, 999, false},
	}
	for _, c := range cases {
		if got := allowMessage(c.chatID, c.allowed, c.filterSet, c.msgChat, c.fromID); got != c.want {
			t.Errorf("%s: allowMessage = %v, want %v", c.name, got, c.want)
		}
	}
}

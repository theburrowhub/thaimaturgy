package tgbot

import (
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

func TestParseDelta(t *testing.T) {
	cases := []struct {
		in      string
		wantN   int
		wantSet bool
		wantErr bool
	}{
		{"-5", -5, false, false},
		{"+3", 3, false, false},
		{"7", 7, false, false},
		{"=10", 10, true, false},
		{"= 12", 12, true, false},
		{"abc", 0, false, true},
		{"", 0, false, true},
	}
	for _, c := range cases {
		n, set, err := parseDelta(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("parseDelta(%q) err=%v; wantErr=%v", c.in, err, c.wantErr)
			continue
		}
		if err == nil && (n != c.wantN || set != c.wantSet) {
			t.Errorf("parseDelta(%q) = (%d,%v); want (%d,%v)", c.in, n, set, c.wantN, c.wantSet)
		}
	}
}

func TestCanonicalCondition(t *testing.T) {
	if c, ok := canonicalCondition("poisoned"); !ok || c != domain.ConditionPoisoned {
		t.Errorf("poisoned = (%v,%v); want Poisoned,true", c, ok)
	}
	if c, ok := canonicalCondition("  PRONE "); !ok || c != domain.ConditionProne {
		t.Errorf("case/space-insensitive match failed: (%v,%v)", c, ok)
	}
	if _, ok := canonicalCondition("hangry"); ok {
		t.Error("unknown condition should not match")
	}
}

func TestParseItemArg(t *testing.T) {
	cases := []struct {
		in     string
		action string
		name   string
		qty    int
		ok     bool
	}{
		{"add Rope of Climbing x2", "add", "Rope of Climbing", 2, true},
		{"add Dagger", "add", "Dagger", 1, true},
		{"remove Torch x3", "remove", "Torch", 3, true},
		{"rm Shield", "remove", "Shield", 1, true},
		{"drop Potion", "remove", "Potion", 1, true},
		{"add", "", "", 0, false},        // no name
		{"give Sword", "", "", 0, false}, // unknown action
		{"", "", "", 0, false},
	}
	for _, c := range cases {
		action, name, qty, ok := parseItemArg(c.in)
		if ok != c.ok {
			t.Errorf("parseItemArg(%q) ok=%v; want %v", c.in, ok, c.ok)
			continue
		}
		if ok && (action != c.action || name != c.name || qty != c.qty) {
			t.Errorf("parseItemArg(%q) = (%q,%q,%d); want (%q,%q,%d)", c.in, action, name, qty, c.action, c.name, c.qty)
		}
	}
}

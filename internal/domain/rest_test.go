package domain

import (
	"strings"
	"testing"
)

func TestLongRestRestoresFullAndHalfDice(t *testing.T) {
	c := &Character{Level: 6, MaxHP: 40, CurrentHP: 9, TempHP: 4, HitDiceUsed: 5}
	c.LongRest()
	if c.CurrentHP != 40 || c.TempHP != 0 {
		t.Fatalf("after long rest HP=%d/%d temp=%d; want 40/40 temp 0", c.CurrentHP, c.MaxHP, c.TempHP)
	}
	// half of 6 hit dice = 3 recovered → used 5-3 = 2.
	if c.HitDiceUsed != 2 {
		t.Errorf("HitDiceUsed after long rest = %d; want 2", c.HitDiceUsed)
	}
}

func TestShortRestSpendsHitDice(t *testing.T) {
	c := &Character{Level: 4, MaxHP: 30, CurrentHP: 5, HitDiceUsed: 0,
		Abilities: AbilityScores{CON: 14}} // CON mod +2 → 5+2 = 7 per die
	healed, spent := c.ShortRest(2)
	if spent != 2 || healed != 14 || c.CurrentHP != 19 {
		t.Fatalf("short rest healed=%d spent=%d hp=%d; want 14/2/19", healed, spent, c.CurrentHP)
	}
	if c.HitDiceRemaining() != 2 { // 4 total - 2 used
		t.Errorf("HitDiceRemaining = %d; want 2", c.HitDiceRemaining())
	}
	// Cannot spend more dice than remaining, and never overheals.
	c.ShortRest(0) // spend all remaining
	if c.CurrentHP > c.MaxHP || c.HitDiceRemaining() != 0 {
		t.Errorf("after spending all: hp=%d rem=%d", c.CurrentHP, c.HitDiceRemaining())
	}
}

func TestRestPartyHealsAndLogs(t *testing.T) {
	adv := &Adventure{ID: "m", Title: "M", Zones: []Zone{{ID: "z", Rooms: []Room{{ID: "r"}}}}}
	st := NewSessionState("s", adv)
	st.SetParty([]*Character{
		{Name: "Ana", Level: 3, MaxHP: 24, CurrentHP: 3},
		{Name: "Beto", Level: 3, MaxHP: 20, CurrentHP: 8},
	})
	before := st.LogLen()
	summary := st.RestParty(true, "", 0)
	if !strings.Contains(summary, "long rest") {
		t.Errorf("summary = %q", summary)
	}
	for _, c := range st.PartySnapshot() {
		if c.CurrentHP != c.MaxHP {
			t.Errorf("%s not full after long rest: %d/%d", c.Name, c.CurrentHP, c.MaxHP)
		}
	}
	if st.LogLen() != before+1 {
		t.Errorf("rest did not add a timeline entry (%d -> %d)", before, st.LogLen())
	}

	// Named rest affects only that character.
	st.SetParty([]*Character{
		{Name: "Ana", Level: 3, MaxHP: 24, CurrentHP: 3},
		{Name: "Beto", Level: 3, MaxHP: 20, CurrentHP: 8},
	})
	st.RestParty(true, "Ana", 0)
	snap := st.PartySnapshot()
	if snap[0].CurrentHP != 24 || snap[1].CurrentHP != 8 {
		t.Errorf("named rest affected the wrong members: %d, %d", snap[0].CurrentHP, snap[1].CurrentHP)
	}
}

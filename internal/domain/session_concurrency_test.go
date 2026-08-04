package domain

import (
	"encoding/json"
	"sync"
	"testing"
)

// TestSessionStateConcurrentAccess hammers the mutators, read accessors and JSON
// serialization from many goroutines at once. It exists to make `go test -race`
// actually exercise the concurrency the mutex is meant to protect (mutation vs.
// serialization vs. read) — with the guards removed it fails under the race
// detector.
func TestSessionStateConcurrentAccess(t *testing.T) {
	st := NewSessionState("concurrent", nil)
	// Bound the log/conversation so repeated marshalling stays cheap under -race;
	// the point is to exercise concurrent access, not to grow the state.
	st.Log.MaxSize = 64
	st.Conversation.MaxSize = 64
	st.Characters = []*Character{NewCharacter("Hero", "Human", "Fighter")}
	_, _ = st.ClaimCharacter("p1", "P1", "Hero")

	const iters = 150
	var wg sync.WaitGroup

	// Writers: every mutation path that touches nested pointers/maps.
	for range 4 {
		wg.Go(func() {
			for range iters {
				st.AppendLog(LogEntry{Type: LogNote, Message: "note"})
				st.SetFlag("gate", true)
				st.SetVariable("k", "v")
				st.MutateCharacter("Hero", func(c *Character) { c.AwardXP(1); c.SetHP(5); c.SetGold(3) })
				st.MutateCharacter("", func(c *Character) { c.Heal(1) }) // empty name → sole member
				_, _ = st.SubmitAction("p1", "swing")
				st.ResetRound()
				st.StartGame()
				st.SetNPCDisposition("n", "friendly")
				st.SetNPCAlive("n", true)
				hp := 7
				st.UpsertPartyMember("Bob", &hp, &hp, nil, "note")
				st.AddUserMessage("u")
				st.AddAssistantMessage("a")
				st.ToggleMode()
			}
		})
	}

	// Readers/serializers: what the UI and autosave do.
	for range 3 {
		wg.Go(func() {
			for range iters {
				if _, err := json.Marshal(st); err != nil {
					t.Errorf("marshal: %v", err)
					return
				}
				_ = st.RecentLog(10)
				_ = st.LogLen()
				_ = st.EffectiveMode()
				_ = st.PartySnapshot()
				_ = st.PartyNames()
				_ = st.RoundActions()
				_ = st.PendingPlayers()
				_ = st.GameStarted()
			}
		})
	}

	wg.Wait()
}

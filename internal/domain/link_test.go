package domain

import "testing"

func TestLinkRosterIDs(t *testing.T) {
	st := NewSessionState("s", nil)
	st.Characters = []*Character{
		NewCharacter("Alden", "Human", "Fighter"),
		NewCharacter("Naivara", "Elf", "Wizard"),
		NewCharacter("Thorin", "Dwarf", "Cleric"),
	}

	// Link by position; empty IDs and out-of-range indices are skipped.
	st.LinkRosterIDs([]string{"id-a", "", "id-c", "id-extra"})

	if st.Characters[0].ID != "id-a" {
		t.Errorf("member 0 ID = %q; want id-a", st.Characters[0].ID)
	}
	if st.Characters[1].ID != "" {
		t.Errorf("member 1 ID = %q; want empty (skipped)", st.Characters[1].ID)
	}
	if st.Characters[2].ID != "id-c" {
		t.Errorf("member 2 ID = %q; want id-c", st.Characters[2].ID)
	}
}

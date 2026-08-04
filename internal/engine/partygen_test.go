package engine

import "testing"

func TestParseRoster(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantN   int
		wantErr bool
	}{
		{"plain json", `{"members":[{"name":"A","race":"Elf","class":"Wizard","level":1}]}`, 1, false},
		{"code fenced", "```json\n{\"members\":[{\"race\":\"Dwarf\",\"class\":\"Cleric\"}]}\n```", 1, false},
		{"prose around", "Sure! Here you go:\n{\"members\":[{\"race\":\"Human\",\"class\":\"Fighter\"},{\"race\":\"Elf\",\"class\":\"Rogue\"}]}\nEnjoy.", 2, false},
		{"no json", "I cannot do that.", 0, true},
		{"invalid json", "{members: not valid}", 0, true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			r, err := parseRoster(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got %+v", r)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(r.Members) != tt.wantN {
				t.Errorf("members = %d, want %d", len(r.Members), tt.wantN)
			}
		})
	}
}

// TestGeneratePartyDistinctNames verifies unnamed same-race members get distinct,
// addressable names rather than colliding on one.
func TestGeneratePartyDistinctNames(t *testing.T) {
	party := GeneratePartyFromSpecs([]PartyMemberSpec{
		{Race: "Elf", Class: "Wizard"},
		{Race: "Elf", Class: "Ranger"},
		{Race: "Elf", Class: "Rogue"},
	})
	if len(party) != 3 {
		t.Fatalf("want 3 members, got %d", len(party))
	}
	seen := map[string]bool{}
	for _, c := range party {
		if seen[c.Name] {
			t.Errorf("duplicate party name %q", c.Name)
		}
		seen[c.Name] = true
	}
}

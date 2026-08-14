package rulesystem

import "testing"

func TestBuiltinPacksValidate(t *testing.T) {
	for _, id := range BuiltinIDs {
		t.Run(id, func(t *testing.T) {
			p, err := Builtin(id)
			if err != nil {
				t.Fatal(err)
			}
			if p.ID != id {
				t.Fatalf("id = %q; want %q", p.ID, id)
			}
			if len(p.Tools) == 0 {
				t.Fatal("expected tools")
			}
		})
	}
}

func TestDetectFamily(t *testing.T) {
	cases := []struct {
		text string
		want string
	}{
		{"Armor Class and proficiency bonus on a d20 attack", "dnd5e"},
		{"Roll d100 under your Spot Hidden skill for sanity", "d100"},
		{"Wild die bennies and shaken wounds in Savage Worlds", "savage_worlds"},
	}
	for _, tc := range cases {
		if got := DetectFamily(tc.text); got != tc.want {
			t.Errorf("DetectFamily(%q) = %q; want %q", tc.text, got, tc.want)
		}
	}
}

func TestSaveLoadJSONRoundTrip(t *testing.T) {
	p, err := Builtin("dnd5e")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := dir + "/dnd5e.json"
	if err := Save(p, path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != p.Name || loaded.ID != p.ID {
		t.Fatalf("round trip mismatch: %+v", loaded)
	}
}

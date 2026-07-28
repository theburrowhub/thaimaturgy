package domain

import (
	"encoding/json"
	"testing"
)

func TestStatBlockFlexibleTypes(t *testing.T) {
	// cr as a number, ac/max_hp as strings — all should be accepted.
	in := `{"ac":"15","max_hp":"22","cr":5,"speed":"30 ft",
	        "abilities":{"str":10,"dex":12,"con":14,"int":8,"wis":10,"cha":9}}`
	var sb StatBlock
	if err := json.Unmarshal([]byte(in), &sb); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sb.CR != "5" {
		t.Errorf("CR = %q, want \"5\"", sb.CR)
	}
	if sb.AC != 15 {
		t.Errorf("AC = %d, want 15", sb.AC)
	}
	if sb.MaxHP != 22 {
		t.Errorf("MaxHP = %d, want 22", sb.MaxHP)
	}
	if sb.Speed != "30 ft" || sb.Abilities.DEX != 12 {
		t.Errorf("other fields not parsed: %+v", sb)
	}
}

func TestStatBlockNormalTypes(t *testing.T) {
	// Documented types (cr string, ac/hp numbers) must still work.
	in := `{"ac":13,"max_hp":9,"cr":"1/4"}`
	var sb StatBlock
	if err := json.Unmarshal([]byte(in), &sb); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sb.CR != "1/4" || sb.AC != 13 || sb.MaxHP != 9 {
		t.Errorf("unexpected: %+v", sb)
	}
}

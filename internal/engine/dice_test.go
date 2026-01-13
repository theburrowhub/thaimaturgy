package engine

import (
	"testing"
)

func TestParseDice(t *testing.T) {
	tests := []struct {
		notation  string
		numDice   int
		diceSides int
		modifier  int
		wantErr   bool
	}{
		{"1d20", 1, 20, 0, false},
		{"d20", 1, 20, 0, false},
		{"2d6", 2, 6, 0, false},
		{"4d6", 4, 6, 0, false},
		{"1d8+3", 1, 8, 3, false},
		{"2d6+5", 2, 6, 5, false},
		{"1d20-2", 1, 20, -2, false},
		{"3d8-1", 3, 8, -1, false},
		{"10d10", 10, 10, 0, false},
		{"1d100", 1, 100, 0, false},
		{"", 0, 0, 0, true},
		{"invalid", 0, 0, 0, true},
		{"1d", 0, 0, 0, true},
		{"d", 0, 0, 0, true},
		{"1d0", 0, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.notation, func(t *testing.T) {
			roll, err := ParseDice(tt.notation)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseDice(%q) expected error, got nil", tt.notation)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseDice(%q) unexpected error: %v", tt.notation, err)
				return
			}
			if roll.NumDice != tt.numDice {
				t.Errorf("ParseDice(%q) NumDice = %d, want %d", tt.notation, roll.NumDice, tt.numDice)
			}
			if roll.DiceSides != tt.diceSides {
				t.Errorf("ParseDice(%q) DiceSides = %d, want %d", tt.notation, roll.DiceSides, tt.diceSides)
			}
			if roll.Modifier != tt.modifier {
				t.Errorf("ParseDice(%q) Modifier = %d, want %d", tt.notation, roll.Modifier, tt.modifier)
			}
		})
	}
}

func TestDiceRoll(t *testing.T) {
	roll, err := ParseDice("2d6")
	if err != nil {
		t.Fatalf("ParseDice failed: %v", err)
	}

	for i := 0; i < 100; i++ {
		total := roll.Roll()
		if total < 2 || total > 12 {
			t.Errorf("Roll() = %d, want between 2 and 12", total)
		}
		if len(roll.Rolls) != 2 {
			t.Errorf("Roll() produced %d dice, want 2", len(roll.Rolls))
		}
		for _, r := range roll.Rolls {
			if r < 1 || r > 6 {
				t.Errorf("Individual roll %d out of range [1, 6]", r)
			}
		}
	}
}

func TestDiceRollWithModifier(t *testing.T) {
	roll, err := ParseDice("1d6+5")
	if err != nil {
		t.Fatalf("ParseDice failed: %v", err)
	}

	for i := 0; i < 50; i++ {
		total := roll.Roll()
		if total < 6 || total > 11 {
			t.Errorf("Roll() = %d, want between 6 and 11", total)
		}
	}
}

func TestDiceRollNegativeModifier(t *testing.T) {
	roll, err := ParseDice("1d20-5")
	if err != nil {
		t.Fatalf("ParseDice failed: %v", err)
	}

	for i := 0; i < 50; i++ {
		total := roll.Roll()
		if total < -4 || total > 15 {
			t.Errorf("Roll() = %d, want between -4 and 15", total)
		}
	}
}

func TestRollDice(t *testing.T) {
	roll, err := RollDice("3d6")
	if err != nil {
		t.Fatalf("RollDice failed: %v", err)
	}

	if roll.Total < 3 || roll.Total > 18 {
		t.Errorf("RollDice total = %d, want between 3 and 18", roll.Total)
	}
	if len(roll.Rolls) != 3 {
		t.Errorf("RollDice produced %d rolls, want 3", len(roll.Rolls))
	}
}

func TestRollD20(t *testing.T) {
	for i := 0; i < 100; i++ {
		roll := RollD20()
		if roll.Total < 1 || roll.Total > 20 {
			t.Errorf("RollD20() = %d, want between 1 and 20", roll.Total)
		}
	}
}

func TestCriticals(t *testing.T) {
	roller := NewRoller(42)

	var critHits, critFails int
	for i := 0; i < 1000; i++ {
		roll, _ := roller.Roll("1d20")
		if roll.IsCriticalHit() {
			critHits++
		}
		if roll.IsCriticalFail() {
			critFails++
		}
	}

	if critHits == 0 {
		t.Error("Expected at least one critical hit in 1000 rolls")
	}
	if critFails == 0 {
		t.Error("Expected at least one critical fail in 1000 rolls")
	}
}

func TestRollAbilityScore(t *testing.T) {
	for i := 0; i < 50; i++ {
		roll := RollAbilityScore()
		if roll.Total < 3 || roll.Total > 18 {
			t.Errorf("RollAbilityScore() = %d, want between 3 and 18", roll.Total)
		}
		if len(roll.Rolls) != 3 {
			t.Errorf("RollAbilityScore() kept %d dice, want 3", len(roll.Rolls))
		}
	}
}

func TestDiceRollResultString(t *testing.T) {
	roll := &DiceRoll{
		NumDice:   2,
		DiceSides: 6,
		Modifier:  3,
		Rolls:     []int{4, 5},
		Total:     12,
	}

	result := roll.ResultString()
	if result != "[4+5]+3 = 12" {
		t.Errorf("ResultString() = %q, want %q", result, "[4+5]+3 = 12")
	}
}

func TestDiceRollString(t *testing.T) {
	tests := []struct {
		roll     *DiceRoll
		expected string
	}{
		{&DiceRoll{NumDice: 1, DiceSides: 20, Modifier: 0}, "1d20"},
		{&DiceRoll{NumDice: 2, DiceSides: 6, Modifier: 3}, "2d6+3"},
		{&DiceRoll{NumDice: 1, DiceSides: 8, Modifier: -2}, "1d8-2"},
	}

	for _, tt := range tests {
		result := tt.roll.String()
		if result != tt.expected {
			t.Errorf("String() = %q, want %q", result, tt.expected)
		}
	}
}

func TestSeededRoller(t *testing.T) {
	roller1 := NewRoller(12345)
	roller2 := NewRoller(12345)

	for i := 0; i < 10; i++ {
		roll1, _ := roller1.Roll("1d20")
		roll2, _ := roller2.Roll("1d20")
		if roll1.Total != roll2.Total {
			t.Errorf("Seeded rollers produced different results: %d vs %d", roll1.Total, roll2.Total)
		}
	}
}

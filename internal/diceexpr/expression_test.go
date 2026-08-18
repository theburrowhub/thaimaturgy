package diceexpr

import (
	"strings"
	"testing"
)

func TestParseMatchesLegacyGrammarAndCanonicalization(t *testing.T) {
	tests := []struct {
		input               string
		notation, canonical string
		num, sides, mod     int
	}{
		{"1d20", "1d20", "1d20", 1, 20, 0},
		{" D20 ", "d20", "1d20", 1, 20, 0},
		{"2D6+5", "2d6+5", "2d6+5", 2, 6, 5},
		{"3d8-1", "3d8-1", "3d8-1", 3, 8, -1},
		{"01d020+000", "01d020+000", "1d20", 1, 20, 0},
		{"1d6-0", "1d6-0", "1d6", 1, 6, 0},
		{"100d1000", "100d1000", "100d1000", 100, 1000, 0},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			expr, err := Parse(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if expr.Notation != test.notation || expr.String() != test.canonical ||
				expr.NumDice != test.num || expr.DiceSides != test.sides || expr.Modifier != test.mod {
				t.Fatalf("Parse(%q) = %+v canonical=%q", test.input, expr, expr.String())
			}
		})
	}
}

func TestParseMatchesLegacyErrors(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "invalid dice notation:  (expected format: NdM or NdM+K)"},
		{"invalid", "invalid dice notation: invalid (expected format: NdM or NdM+K)"},
		{"1d", "invalid dice notation: 1d (expected format: NdM or NdM+K)"},
		{"0d6", "number of dice must be between 1 and 100"},
		{"101d6", "number of dice must be between 1 and 100"},
		{"1d0", "dice sides must be between 1 and 1000"},
		{"1d1001", "dice sides must be between 1 and 1000"},
		{"+1d6", "invalid dice notation: +1d6 (expected format: NdM or NdM+K)"},
		{"1 d6", "invalid dice notation: 1 d6 (expected format: NdM or NdM+K)"},
		{strings.Repeat("9", 100) + "d6", "invalid number of dice: " + strings.Repeat("9", 100)},
		{"1d" + strings.Repeat("9", 100), "invalid dice sides: " + strings.Repeat("9", 100)},
		{"1d6+" + strings.Repeat("9", 100), "invalid modifier: +" + strings.Repeat("9", 100)},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			_, err := Parse(test.input)
			if err == nil || err.Error() != test.want {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestExternalRollEvaluationAndFormatting(t *testing.T) {
	expr, err := Parse("2d6+3")
	if err != nil {
		t.Fatal(err)
	}
	total, err := expr.Total([]int{4, 5})
	if err != nil || total != 12 {
		t.Fatalf("Total = %d, %v", total, err)
	}
	result, err := expr.ResultString([]int{4, 5})
	if err != nil || result != "[4+5]+3 = 12" {
		t.Fatalf("ResultString = %q, %v", result, err)
	}
	if _, err := expr.Total([]int{1}); err == nil {
		t.Fatal("wrong roll count was accepted")
	}
	if _, err := expr.Total([]int{1, 7}); err == nil {
		t.Fatal("out-of-range roll was accepted")
	}
}

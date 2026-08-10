package main

import "testing"

func TestRollAbility4d6DropLowest(t *testing.T) {
	// 4d6-drop-lowest always yields a value in [3, 18]. Sample many rolls.
	sawHigh, sawLow := false, false
	for i := 0; i < 2000; i++ {
		v := rollAbility4d6DropLowest()
		if v < 3 || v > 18 {
			t.Fatalf("roll out of range: %d", v)
		}
		if v >= 15 {
			sawHigh = true
		}
		if v <= 8 {
			sawLow = true
		}
	}
	if !sawHigh || !sawLow {
		t.Error("expected a spread of rolls across the range")
	}
}

func TestClampScore(t *testing.T) {
	for _, c := range []struct{ in, want int }{{0, 1}, {-5, 1}, {10, 10}, {30, 30}, {99, 30}} {
		if got := clampScore(c.in); got != c.want {
			t.Errorf("clampScore(%d) = %d; want %d", c.in, got, c.want)
		}
	}
}

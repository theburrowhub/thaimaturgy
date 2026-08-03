package main

import "testing"

func TestComposeNotation(t *testing.T) {
	tests := []struct {
		name  string
		qty   string
		sides int
		mod   string
		want  string
	}{
		{"basic", "1", 20, "0", "1d20"},
		{"count and positive mod", "2", 6, "3", "2d6+3"},
		{"negative mod", "1", 20, "-1", "1d20-1"},
		{"zero mod omitted", "4", 6, "0", "4d6"},
		{"blank count defaults to 1", "", 8, "0", "1d8"},
		{"non-numeric count defaults to 1", "abc", 10, "0", "1d10"},
		{"count below 1 defaults to 1", "0", 12, "0", "1d12"},
		{"negative count defaults to 1", "-3", 6, "2", "1d6+2"},
		{"blank mod treated as zero", "3", 6, "", "3d6"},
		{"non-numeric mod treated as zero", "3", 6, "xx", "3d6"},
		{"whitespace trimmed", " 2 ", 6, " 3 ", "2d6+3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := composeNotation(tt.qty, tt.sides, tt.mod)
			if got != tt.want {
				t.Errorf("composeNotation(%q, %d, %q) = %q, want %q", tt.qty, tt.sides, tt.mod, got, tt.want)
			}
		})
	}
}

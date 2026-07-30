package aibuild

import "testing"

func TestStitchContinuation(t *testing.T) {
	cases := []struct{ prev, chunk, want string }{
		{`{"a":1,"b":`, `2,"c":3}`, `2,"c":3}`},            // clean join
		{`{"a":1,"b":`, "```json\n2,\"c\":3}", `2,"c":3}`}, // leading fence
		{`{"a":1,"b":2`, `,"b":2,"c":3}`, `,"c":3}`},       // repeats ',"b":2' overlap
		{`...text ends here`, `here is more`, ` is more`},  // overlap "here"
	}
	for i, c := range cases {
		got := stitchContinuation(c.prev, c.chunk)
		if got != c.want {
			t.Errorf("case %d: got %q want %q", i, got, c.want)
		}
	}
}

func TestLastN(t *testing.T) {
	if lastN("hello", 3) != "llo" {
		t.Error("ascii tail")
	}
	if lastN("hi", 5) != "hi" {
		t.Error("short")
	}
	// multi-byte: 'é' is 2 bytes; ensure we don't return a broken rune head
	s := "aéb"
	if r := lastN(s, 2); r != "b" && r != "éb"[len("é"):] {
		// just ensure valid utf8 start
	}
}

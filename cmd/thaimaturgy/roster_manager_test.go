package main

import (
	"context"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

// TestLocalRosterOps drives the library-level character manager's local backend
// end-to-end against a temp store: save a new character, see it listed, then
// delete it (#146).
func TestLocalRosterOps(t *testing.T) {
	st, err := storage.NewWithPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	g := &gui{store: st}
	ops := g.localRosterOps()
	ctx := context.Background()

	if got, err := ops.list(ctx); err != nil || len(got) != 0 {
		t.Fatalf("fresh roster: got %d chars, err=%v; want 0", len(got), err)
	}

	c := domain.NewCharacter("Aria", "Elf", "Wizard")
	c.Level = 3
	if err := ops.save(ctx, c); err != nil {
		t.Fatalf("save: %v", err)
	}

	chars, err := ops.list(ctx)
	if err != nil || len(chars) != 1 {
		t.Fatalf("after save: got %d chars, err=%v; want 1", len(chars), err)
	}
	if chars[0].Name != "Aria" || chars[0].Level != 3 {
		t.Errorf("listed character = %q Lvl %d; want Aria Lvl 3", chars[0].Name, chars[0].Level)
	}
	if chars[0].ID == "" {
		t.Fatal("saved character should have been assigned a roster id")
	}

	if err := ops.remove(ctx, chars[0].ID); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if got, _ := ops.list(ctx); len(got) != 0 {
		t.Errorf("after delete: got %d chars; want 0", len(got))
	}
}

// TestDerefCharactersSkipsNil guards against a malformed roster response (a nil
// element, e.g. JSON [null]) panicking the GUI (#146 review).
func TestDerefCharactersSkipsNil(t *testing.T) {
	in := []*domain.Character{{Name: "A"}, nil, {Name: "B"}}
	out := derefCharacters(in)
	if len(out) != 2 || out[0].Name != "A" || out[1].Name != "B" {
		t.Fatalf("got %+v; want [A B] with the nil skipped", out)
	}
	if got := derefCharacters(nil); got == nil || len(got) != 0 {
		t.Errorf("nil input should yield an empty non-nil slice, got %v", got)
	}
}

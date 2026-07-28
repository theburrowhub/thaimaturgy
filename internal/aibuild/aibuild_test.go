package aibuild

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
)

// stubProvider returns a canned response and records the request it received.
type stubProvider struct {
	content string
	lastReq providers.ChatRequest
}

func (s *stubProvider) Name() string        { return "stub" }
func (s *stubProvider) SupportsTools() bool { return false }
func (s *stubProvider) Chat(_ context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	s.lastReq = req
	return &providers.ChatResponse{Content: s.content}, nil
}

func writePNG(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	img.Set(0, 0, color.RGBA{1, 2, 3, 255})
	f, _ := os.Create(path)
	defer f.Close()
	_ = png.Encode(f, img)
}

func TestFromImagesBuildsAndSanitizes(t *testing.T) {
	src := t.TempDir()
	writePNG(t, filepath.Join(src, "hero.png"))
	work := t.TempDir()

	// The model references one existing image and one missing one, plus a
	// dangling npc reference and a bad exit — all must be cleaned up.
	stub := &stubProvider{content: "```json\n" + `{
	  "id":"My Cool Module","title":"My Cool Module",
	  "zones":[{"id":"z1","name":"Zone","map_image":"assets/art/hero.png",
	    "rooms":[{"id":"r1","name":"Room","image":"assets/art/missing.png",
	      "npc_ids":["villain","ghost"],"exits":[{"to":"r1"},{"to":"nowhere"}]}]}],
	  "npcs":[{"id":"villain","name":"Villain","default_location":"badroom"}]
	}` + "\n```"}

	adv, err := FromImages(context.Background(), stub, &domain.Config{Model: "gpt-4o"}, src, work, "Fallback Title")
	if err != nil {
		t.Fatalf("FromImages: %v", err)
	}

	if adv.ID != "my-cool-module" {
		t.Errorf("ID = %q, want my-cool-module (slugified)", adv.ID)
	}
	z := adv.Zones[0]
	if z.MapImage != "assets/art/hero.png" {
		t.Errorf("existing map image should be kept, got %q", z.MapImage)
	}
	r := z.Rooms[0]
	if r.Image != "" {
		t.Errorf("missing room image should be cleared, got %q", r.Image)
	}
	if len(r.NPCIDs) != 1 || r.NPCIDs[0] != "villain" {
		t.Errorf("dangling npc ref should be dropped, got %v", r.NPCIDs)
	}
	if len(r.Exits) != 1 || r.Exits[0].To != "r1" {
		t.Errorf("exit to unknown target should be dropped, got %v", r.Exits)
	}
	if adv.NPCs[0].DefaultLocation != "" {
		t.Errorf("bad default_location should be cleared, got %q", adv.NPCs[0].DefaultLocation)
	}

	// The hero image must have been attached for visual interpretation.
	if len(stub.lastReq.Messages) < 2 || len(stub.lastReq.Messages[1].Images) == 0 {
		t.Error("expected the source image to be attached to the AI request")
	}
	if stub.lastReq.Model != "gpt-4o" {
		t.Errorf("model not passed through, got %q", stub.lastReq.Model)
	}
}

func TestFromImagesRequiresProvider(t *testing.T) {
	if _, err := FromImages(context.Background(), nil, nil, t.TempDir(), t.TempDir(), "x"); err == nil {
		t.Fatal("expected an error when no provider is configured")
	}
}

// seqProvider returns a queued sequence of responses (last one repeats).
type seqProvider struct {
	resps []*providers.ChatResponse
	calls int
}

func (s *seqProvider) Name() string        { return "seq" }
func (s *seqProvider) SupportsTools() bool { return false }
func (s *seqProvider) Chat(_ context.Context, _ providers.ChatRequest) (*providers.ChatResponse, error) {
	i := s.calls
	if i >= len(s.resps) {
		i = len(s.resps) - 1
	}
	s.calls++
	return s.resps[i], nil
}

func TestBuildRepairsInvalidJSON(t *testing.T) {
	stub := &seqProvider{resps: []*providers.ChatResponse{
		{Content: "Here is your module: {\"id\":\"x\"", FinishReason: "stop"}, // unparseable
		{Content: `{"id":"x","title":"X","zones":[{"id":"z","name":"Z"}]}`, FinishReason: "stop"},
	}}
	adv, err := build(context.Background(), stub, &domain.Config{Model: "m"}, "T", "doc", nil, t.TempDir())
	if err != nil {
		t.Fatalf("build should recover via repair: %v", err)
	}
	if adv.ID != "x" {
		t.Errorf("ID = %q, want x", adv.ID)
	}
	if stub.calls != 2 {
		t.Errorf("expected a repair call (2 total), got %d", stub.calls)
	}
}

func TestBuildReportsTruncation(t *testing.T) {
	// The model keeps hitting the output limit; every reply is truncated.
	stub := &seqProvider{resps: []*providers.ChatResponse{
		{Content: "{\"id\":\"x\",", FinishReason: "max_tokens"},
	}}
	_, err := build(context.Background(), stub, &domain.Config{Model: "m"}, "T", "doc", nil, t.TempDir())
	if err == nil {
		t.Fatal("expected an error for persistently truncated output")
	}
	if !strings.Contains(err.Error(), "output limit") {
		t.Errorf("error should mention the output limit, got: %v", err)
	}
	// It must have attempted continuations rather than giving up after one call.
	if stub.calls < 2 {
		t.Errorf("expected continuation attempts, got %d calls", stub.calls)
	}
}

func TestBuildContinuesTruncatedJSON(t *testing.T) {
	// First reply is cut off; the continuation completes the JSON.
	stub := &seqProvider{resps: []*providers.ChatResponse{
		{Content: `{"id":"x","title":"X","zones":[`, FinishReason: "max_tokens"},
		{Content: `{"id":"z","name":"Z"}]}`, FinishReason: "stop"},
	}}
	adv, err := build(context.Background(), stub, &domain.Config{Model: "m"}, "T", "doc", nil, t.TempDir())
	if err != nil {
		t.Fatalf("build should stitch continuations: %v", err)
	}
	if adv.ID != "x" || len(adv.Zones) != 1 {
		t.Errorf("unexpected stitched result: %+v", adv)
	}
	if stub.calls != 2 {
		t.Errorf("expected 2 calls (initial + continuation), got %d", stub.calls)
	}
}

func TestParseAdventureToleratesTrailingCommas(t *testing.T) {
	in := `{"id":"a","title":"A","zones":[{"id":"z","name":"Z"},],}`
	adv, err := parseAdventure(in)
	if err != nil {
		t.Fatalf("trailing commas should be tolerated: %v", err)
	}
	if adv.ID != "a" || len(adv.Zones) != 1 {
		t.Errorf("unexpected parse result: %+v", adv)
	}
}

func TestParseAdventurePlainAndFenced(t *testing.T) {
	for _, in := range []string{
		`{"id":"a","title":"A","zones":[{"id":"z","name":"Z"}]}`,
		"prefix ```json\n{\"id\":\"a\",\"title\":\"A\",\"zones\":[{\"id\":\"z\",\"name\":\"Z\"}]}\n``` suffix",
	} {
		adv, err := parseAdventure(in)
		if err != nil {
			t.Fatalf("parseAdventure(%q): %v", in, err)
		}
		if adv.ID != "a" {
			t.Errorf("ID = %q, want a", adv.ID)
		}
	}
	if _, err := parseAdventure("no json here"); err == nil {
		t.Error("expected error when there is no JSON object")
	}
}

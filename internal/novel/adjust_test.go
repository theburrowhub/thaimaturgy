package novel

import (
	"context"
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
)

// scriptProvider returns a scripted sequence of replies, one per Chat call, so a
// test can simulate a reply cut off at the token limit followed by a completion.
type scriptProvider struct {
	replies []providers.ChatResponse
	n       int
	calls   int      // number of Chat invocations
	prompts []string // the concatenated user turns of each call, for assertions
}

func (p *scriptProvider) Name() string         { return "script" }
func (p *scriptProvider) SupportsTools() bool  { return false }
func (p *scriptProvider) SupportsVision() bool { return false }
func (p *scriptProvider) Chat(_ context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	p.calls++
	var users []string
	for _, m := range req.Messages {
		if m.Role == providers.RoleUser {
			users = append(users, m.Content)
		}
	}
	p.prompts = append(p.prompts, strings.Join(users, "\n"))
	r := p.replies[p.n]
	if p.n < len(p.replies)-1 {
		p.n++
	}
	return &r, nil
}

func advWithLang(lang string) *domain.Adventure {
	return &domain.Adventure{Title: "The Crypt", Summary: "A tomb.", Language: lang}
}

// A selection edit revises exactly one excerpt in one pass and returns only the
// revised excerpt (code fences the model may add are stripped).
func TestAdjustSelectionSinglePass(t *testing.T) {
	prov := &scriptProvider{replies: []providers.ChatResponse{
		{Content: "```markdown\nThe crypt was darker now.\n```", FinishReason: "stop"},
	}}
	got, err := Adjust(context.Background(), prov, "m", advWithLang("en"), &domain.SessionState{}, AdjustOptions{
		FullText:    "# Book\n\nThe crypt was cold.",
		Selection:   "The crypt was cold.",
		Instruction: "make it darker",
	})
	if err != nil {
		t.Fatalf("Adjust: %v", err)
	}
	if got != "The crypt was darker now." {
		t.Errorf("selection result = %q; want the stripped revised excerpt", got)
	}
	if prov.calls != 1 {
		t.Errorf("selection edit should make exactly one call, made %d", prov.calls)
	}
	// The excerpt (not the whole book) is what gets sent for revision.
	if !strings.Contains(prov.prompts[0], "EXCERPT TO REVISE") {
		t.Errorf("selection prompt missing the excerpt framing: %q", prov.prompts[0])
	}
}

// A whole-novel rewrite continues across passes when the first reply is cut off
// at the token limit, stitching the pieces into one seamless result.
func TestAdjustWholeContinues(t *testing.T) {
	prov := &scriptProvider{replies: []providers.ChatResponse{
		{Content: "The story began well", FinishReason: "max_tokens"},
		{Content: " and ended better.", FinishReason: "stop"},
	}}
	got, err := Adjust(context.Background(), prov, "m", advWithLang("en"), &domain.SessionState{}, AdjustOptions{
		FullText:    "The story began.",
		Instruction: "polish it",
	})
	if err != nil {
		t.Fatalf("Adjust: %v", err)
	}
	if got != "The story began well and ended better." {
		t.Errorf("continuation not stitched: %q", got)
	}
	if prov.calls != 2 {
		t.Fatalf("expected 2 passes (truncation + resume), got %d", prov.calls)
	}
	// The resume turn must feed the model its own tail so it can continue.
	if !strings.Contains(prov.prompts[1], "cut off") {
		t.Errorf("resume prompt should explain the cut-off: %q", prov.prompts[1])
	}
}

// stitchProse must drop an overlap the model repeats and strip a leading fence.
func TestStitchProseDropsOverlapAndFence(t *testing.T) {
	prev := "the hinges groaned"
	chunk := "```\nhinges groaned as the door swung wide."
	got := stitchProse(prev, chunk)
	if strings.Contains(got, "```") {
		t.Errorf("fence not stripped: %q", got)
	}
	if !strings.HasPrefix(got, " as the door") {
		t.Errorf("overlap not dropped: %q", got)
	}
}

// If every allowed pass is still cut off at the token limit, Adjust must fail
// rather than return a partial novel that could be saved as the finished one.
func TestAdjustWholeTruncationExhaustedErrors(t *testing.T) {
	// A provider whose replies are ALWAYS truncated.
	always := []providers.ChatResponse{}
	for i := 0; i < maxAdjustContinuations+2; i++ {
		always = append(always, providers.ChatResponse{Content: "more", FinishReason: "max_tokens"})
	}
	prov := &scriptProvider{replies: always}
	if _, err := Adjust(context.Background(), prov, "m", advWithLang("en"), &domain.SessionState{}, AdjustOptions{
		FullText: "The story began.", Instruction: "expand it a lot",
	}); err == nil {
		t.Error("a never-completing whole-novel rewrite should return an error, not a partial result")
	}
}

func TestAdjustValidatesInput(t *testing.T) {
	prov := &scriptProvider{replies: []providers.ChatResponse{{Content: "x", FinishReason: "stop"}}}
	if _, err := Adjust(context.Background(), prov, "m", advWithLang("en"), &domain.SessionState{}, AdjustOptions{FullText: "text", Instruction: "  "}); err == nil {
		t.Error("empty instruction should error")
	}
	if _, err := Adjust(context.Background(), prov, "m", advWithLang("en"), &domain.SessionState{}, AdjustOptions{FullText: "  ", Instruction: "do"}); err == nil {
		t.Error("empty novel text should error")
	}
	if _, err := Adjust(context.Background(), nil, "m", advWithLang("en"), &domain.SessionState{}, AdjustOptions{FullText: "t", Instruction: "d"}); err == nil {
		t.Error("nil provider should error")
	}
}

// The grounding block carries the actual play timeline so edits stay faithful.
func TestAdjustGroundsInTimeline(t *testing.T) {
	prov := &scriptProvider{replies: []providers.ChatResponse{{Content: "ok", FinishReason: "stop"}}}
	st := &domain.SessionState{
		Log: &domain.SessionLog{Entries: []domain.LogEntry{
			{Type: domain.LogNote, Message: "the innkeeper winked", Timestamp: ts(1)},
		}},
	}
	if _, err := Adjust(context.Background(), prov, "m", advWithLang("en"), st, AdjustOptions{
		FullText: "text", Instruction: "add the innkeeper",
	}); err != nil {
		t.Fatalf("Adjust: %v", err)
	}
	if !strings.Contains(prov.prompts[0], "innkeeper winked") {
		t.Errorf("prompt should ground on the session timeline: %q", prov.prompts[0])
	}
}

package appservice

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/providers"
)

// TestNovelJobGlobalCap verifies the service-wide concurrency cap: even across
// distinct sessions, no more than maxConcurrentNovelJobs run at once.
func TestNovelJobGlobalCap(t *testing.T) {
	svc, _ := newService(t)
	svc.SetProvider(&planProvider{})

	name, err := svc.NewSession("crypt")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer func() { _ = svc.CloseSession(name) }()
	os, _ := svc.Get(name)
	os.Session.State.AddAssistantMessage("A beat, so Generate has content.")

	// Pre-fill the global cap with running jobs for OTHER sessions.
	svc.jobMu.Lock()
	svc.novelJobs = map[string]*NovelJob{}
	for i := 0; i < maxConcurrentNovelJobs; i++ {
		svc.novelJobs["r"+strconv.Itoa(i)] = &NovelJob{ID: "r" + strconv.Itoa(i), Session: "other-" + strconv.Itoa(i), status: ImportRunning}
	}
	svc.jobMu.Unlock()

	// A new export for a different session is rejected by the global cap.
	if _, err := svc.StartNovelJob(name); !errors.Is(err, ErrNovelCapacity) {
		t.Fatalf("StartNovelJob at global cap = %v; want ErrNovelCapacity", err)
	}
}

// blockProvider blocks in Chat until release is closed, so a job stays "running"
// deterministically for the single-flight test.
type blockProvider struct{ release chan struct{} }

func (p *blockProvider) Name() string         { return "block" }
func (p *blockProvider) SupportsTools() bool  { return false }
func (p *blockProvider) SupportsVision() bool { return false }
func (p *blockProvider) Chat(ctx context.Context, _ providers.ChatRequest) (*providers.ChatResponse, error) {
	select {
	case <-p.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return &providers.ChatResponse{Content: "The end.", FinishReason: "stop"}, nil
}

func TestNovelJobSingleFlight(t *testing.T) {
	svc, _ := newService(t)
	rel := make(chan struct{})
	svc.SetProvider(&blockProvider{release: rel})

	name, err := svc.NewSession("crypt")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer func() { _ = svc.CloseSession(name) }()

	// Give the session narratable content so novel.Generate proceeds to the
	// (blocked) AI call and the job stays reliably "running" during the test.
	os, _ := svc.Get(name)
	os.Session.State.AddUserMessage("The party descends into the crypt.")
	os.Session.State.AddAssistantMessage("Cold, still air rises from the dark stair.")

	j1, err := svc.StartNovelJob(name)
	if err != nil {
		t.Fatalf("first StartNovelJob: %v", err)
	}
	// A second export for the same session while the first runs is rejected.
	if _, err := svc.StartNovelJob(name); err == nil {
		t.Error("a second concurrent novel export should be rejected (single-flight)")
	}

	close(rel) // unblock the generator so the job reaches a terminal state
	var status string
	for i := 0; i < 200; i++ {
		status, _ = j1.Snapshot()["status"].(string)
		if status != "running" {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	// It must settle (not hang); the exact terminal state depends on the stub's
	// output, which isn't what this test is about.
	if status == "running" || status == "" {
		t.Fatalf("job did not settle to a terminal state, status=%q", status)
	}

	// Once it has settled, a new export for the session is allowed again.
	if _, err := svc.StartNovelJob(name); err != nil {
		t.Errorf("a new export after the previous finished should be allowed: %v", err)
	}
}

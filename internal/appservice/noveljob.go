package appservice

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/novel"
)

// novelJobTimeout bounds a single novelization (a long, single-shot AI call).
const novelJobTimeout = 15 * time.Minute

// NovelJob tracks an asynchronous session-novelization and holds its result.
type NovelJob struct {
	ID       string
	Title    string
	Subtitle string

	mu       sync.Mutex
	status   ImportJobStatus // reuses running|done|error
	stage    string
	errMsg   string
	markdown string
}

// Snapshot returns a JSON-friendly status view (without the full markdown).
func (j *NovelJob) Snapshot() map[string]any {
	j.mu.Lock()
	defer j.mu.Unlock()
	m := map[string]any{"id": j.ID, "status": string(j.status), "stage": j.stage}
	if j.errMsg != "" {
		m["error"] = j.errMsg
	}
	return m
}

// Markdown returns the generated novel and whether it is ready.
func (j *NovelJob) Markdown() (string, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.markdown, j.status == ImportDone
}

// StartNovelJob begins novelizing an open session. It passes a JSON-deep-copied
// snapshot of the session state to the generator, so the long (~15 min) read
// can't race concurrent mutations of the live session.
func (s *Service) StartNovelJob(sessionName string) (*NovelJob, error) {
	os, ok := s.Get(sessionName)
	if !ok {
		return nil, fmt.Errorf("session %q is not open", sessionName)
	}
	s.mu.Lock()
	prov, model := s.provider, s.config.Model
	s.mu.Unlock()
	if prov == nil {
		return nil, fmt.Errorf("no AI provider configured")
	}
	adv := os.Session.Adventure

	// Deep-copy the state via its JSON round-trip for a stable, race-free snapshot.
	raw, err := json.Marshal(os.Session.State)
	if err != nil {
		return nil, err
	}
	stCopy := &domain.SessionState{}
	if err := json.Unmarshal(raw, stCopy); err != nil {
		return nil, err
	}

	subtitle := "A novelization of the play session"
	if len(adv.Language) >= 2 && adv.Language[:2] == "es" {
		subtitle = "Una novelización de la partida"
	}
	title := adv.Title
	if title == "" {
		title = sessionName
	}

	s.jobMu.Lock()
	s.jobSeq++
	id := "novel-" + strconv.Itoa(s.jobSeq)
	job := &NovelJob{ID: id, Title: title, Subtitle: subtitle, status: ImportRunning, stage: "writing"}
	if s.novelJobs == nil {
		s.novelJobs = make(map[string]*NovelJob)
	}
	s.novelJobs[id] = job
	s.jobMu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), novelJobTimeout)
		defer cancel()
		md, err := novel.Generate(ctx, prov, model, adv, stCopy)
		job.mu.Lock()
		if err != nil {
			job.status, job.errMsg = ImportError, err.Error()
		} else {
			job.status, job.markdown = ImportDone, md
		}
		job.mu.Unlock()
	}()
	return job, nil
}

// NovelJobByID returns a novel job by id.
func (s *Service) NovelJobByID(id string) (*NovelJob, bool) {
	s.jobMu.Lock()
	defer s.jobMu.Unlock()
	j, ok := s.novelJobs[id]
	return j, ok
}

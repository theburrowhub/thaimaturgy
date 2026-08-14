package appservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/novel"
)

// novelJobTimeout bounds a single novelization (a long, single-shot AI call).
const novelJobTimeout = 15 * time.Minute

// novelJobRetention is how long a finished job is kept for its result to be
// downloaded before it is evicted, so the jobs map can't grow without bound.
const novelJobRetention = 30 * time.Minute

// maxConcurrentNovelJobs caps simultaneously-running novelizations across ALL
// sessions, so a client can't spawn unbounded 15-minute AI calls by opening many
// sessions (the per-session duplicate guard alone wouldn't stop that).
const maxConcurrentNovelJobs = 2

// ErrNovelCapacity is returned when too many novelizations are already running.
var ErrNovelCapacity = errors.New("too many novel exports are already running; try again later")

// NovelJob tracks an asynchronous session-novelization and holds its result.
type NovelJob struct {
	ID       string
	Title    string
	Subtitle string
	Session  string // the session it was started for (single-flight key)

	mu        sync.Mutex
	status    ImportJobStatus // reuses running|done|error
	stage     string
	errMsg    string
	markdown  string
	createdAt time.Time
	endedAt   time.Time // when it reached a terminal state (for eviction)
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

func (j *NovelJob) finish(status ImportJobStatus, md, errMsg string, now time.Time) {
	j.mu.Lock()
	j.status, j.markdown, j.errMsg, j.endedAt = status, md, errMsg, now
	j.mu.Unlock()
}

// StartNovelJob begins novelizing an open session. It snapshots the session
// state under the session's operation lock (so the long, race-free read below
// can't clash with concurrent gameplay mutations), then hands a JSON deep copy
// to the generator. Only one novel job may run per session at a time, and
// finished jobs are evicted after novelJobRetention.
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

	// Deep-copy the state via its JSON round-trip for a stable, race-free snapshot,
	// holding the session's op-lock so no mutation runs during the marshal.
	var raw []byte
	if err := s.withOpenSession(sessionName, func(o *OpenSession) (bool, error) {
		b, e := json.Marshal(o.Session.State)
		raw = b
		return false, e
	}); err != nil {
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

	now := time.Now()
	s.jobMu.Lock()
	if s.novelJobs == nil {
		s.novelJobs = make(map[string]*NovelJob)
	}
	// Evict finished jobs past their retention, count the running ones, and note a
	// same-session duplicate — computed over a full pass so counts are accurate.
	running, sameSession := 0, false
	for id, j := range s.novelJobs {
		j.mu.Lock()
		isRunning := j.status == ImportRunning
		expired := !isRunning && !j.endedAt.IsZero() && now.Sub(j.endedAt) > novelJobRetention
		sess := j.Session
		j.mu.Unlock()
		if expired {
			delete(s.novelJobs, id)
			continue
		}
		if isRunning {
			running++
			if sess == sessionName {
				sameSession = true
			}
		}
	}
	// One export per session (duplicate guard) AND a service-wide cap, so a client
	// can't spawn unbounded 15-minute jobs by opening many distinct sessions.
	if sameSession {
		s.jobMu.Unlock()
		return nil, fmt.Errorf("a novel export is already running for this session")
	}
	if running >= maxConcurrentNovelJobs {
		s.jobMu.Unlock()
		return nil, ErrNovelCapacity
	}
	s.jobSeq++
	id := "novel-" + strconv.Itoa(s.jobSeq)
	job := &NovelJob{ID: id, Title: title, Subtitle: subtitle, Session: sessionName, status: ImportRunning, stage: "writing", createdAt: now}
	s.novelJobs[id] = job
	s.jobMu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), novelJobTimeout)
		defer cancel()
		md, err := novel.Generate(ctx, prov, model, adv, stCopy)
		if err != nil {
			job.finish(ImportError, "", err.Error(), time.Now())
			return
		}
		job.finish(ImportDone, md, "", time.Now())
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

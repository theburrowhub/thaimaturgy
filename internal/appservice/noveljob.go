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
	"github.com/theburrowhub/thaimaturgy/internal/providers"
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

// NovelJob tracks an asynchronous session-novelization or novel adjustment and
// holds its result.
type NovelJob struct {
	ID       string
	Title    string
	Subtitle string
	Session  string // the session it was started for (single-flight key)
	Kind     string // "generate" or "adjust"

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
	m := map[string]any{"id": j.ID, "status": string(j.status), "stage": j.stage, "kind": j.Kind}
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
	adv, stCopy, prov, model, err := s.novelSnapshot(sessionName)
	if err != nil {
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

	job, err := s.registerNovelJob(sessionName, title, subtitle, "generate", "writing")
	if err != nil {
		return nil, err
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), novelJobTimeout)
		defer cancel()
		md, err := novel.Generate(ctx, prov, model, adv, stCopy)
		if err != nil {
			job.finish(ImportError, "", err.Error(), time.Now())
			return
		}
		// Associate the result with the session so it can be re-opened and edited.
		// A persistence failure shouldn't lose the generated prose — the client can
		// still download it from the job — so surface it in the stage, not as a
		// hard error.
		if perr := s.saveNovelRaw(sessionName, md); perr != nil {
			job.setStage("generated (warning: could not save to session: " + perr.Error() + ")")
		}
		job.finish(ImportDone, md, "", time.Now())
	}()
	return job, nil
}

// setStage updates the job's human-readable progress line.
func (j *NovelJob) setStage(stage string) {
	j.mu.Lock()
	j.stage = stage
	j.mu.Unlock()
}

// registerNovelJob applies the shared admission policy (evict expired, one job
// per session, service-wide concurrency cap) and registers a new running job.
// It returns ErrNovelCapacity or a duplicate-session error when admission fails.
func (s *Service) registerNovelJob(sessionName, title, subtitle, kind, stage string) (*NovelJob, error) {
	now := time.Now()
	s.jobMu.Lock()
	defer s.jobMu.Unlock()
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
	// One job per session (duplicate guard) AND a service-wide cap, so a client
	// can't spawn unbounded 15-minute jobs by opening many distinct sessions.
	if sameSession {
		return nil, fmt.Errorf("a novel job is already running for this session")
	}
	if running >= maxConcurrentNovelJobs {
		return nil, ErrNovelCapacity
	}
	s.jobSeq++
	id := "novel-" + strconv.Itoa(s.jobSeq)
	job := &NovelJob{ID: id, Title: title, Subtitle: subtitle, Session: sessionName, Kind: kind, status: ImportRunning, stage: stage, createdAt: now}
	s.novelJobs[id] = job
	return job, nil
}

// NovelJobByID returns a novel job by id.
func (s *Service) NovelJobByID(id string) (*NovelJob, bool) {
	s.jobMu.Lock()
	defer s.jobMu.Unlock()
	j, ok := s.novelJobs[id]
	return j, ok
}

// novelSnapshot resolves the provider/model and takes a stable, race-free deep
// copy of the open session's state (via its JSON round-trip, under the session's
// op-lock so no gameplay mutation runs during the marshal). Both generate and
// adjust need this before handing work to a background goroutine.
func (s *Service) novelSnapshot(sessionName string) (*domain.Adventure, *domain.SessionState, providers.Provider, string, error) {
	os, ok := s.Get(sessionName)
	if !ok {
		return nil, nil, nil, "", fmt.Errorf("session %q is not open", sessionName)
	}
	s.mu.Lock()
	prov, model := s.provider, s.config.Model
	s.mu.Unlock()
	if prov == nil {
		return nil, nil, nil, "", fmt.Errorf("no AI provider configured")
	}
	adv := os.Session.Adventure

	var raw []byte
	if err := s.withOpenSession(sessionName, func(o *OpenSession) (bool, error) {
		b, e := json.Marshal(o.Session.State)
		raw = b
		return false, e
	}); err != nil {
		return nil, nil, nil, "", err
	}
	stCopy := &domain.SessionState{}
	if err := json.Unmarshal(raw, stCopy); err != nil {
		return nil, nil, nil, "", err
	}
	return adv, stCopy, prov, model, nil
}

// StartNovelAdjustJob begins an AI revision of a session's novel. fullText is
// the current whole novel; if selection is non-empty only that excerpt is
// revised and the job's result is the revised excerpt (the caller splices it
// back). The result is NOT persisted — the caller reviews it and saves via
// SaveNovelText — so an adjustment stays undoable. It shares the same admission
// policy and concurrency caps as StartNovelJob (one job per session).
func (s *Service) StartNovelAdjustJob(sessionName, fullText, selection, instruction string) (*NovelJob, error) {
	adv, stCopy, prov, model, err := s.novelSnapshot(sessionName)
	if err != nil {
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

	job, err := s.registerNovelJob(sessionName, title, subtitle, "adjust", "revising")
	if err != nil {
		return nil, err
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), novelJobTimeout)
		defer cancel()
		md, err := novel.Adjust(ctx, prov, model, adv, stCopy, novel.AdjustOptions{
			FullText:    fullText,
			Selection:   selection,
			Instruction: instruction,
		})
		if err != nil {
			job.finish(ImportError, "", err.Error(), time.Now())
			return
		}
		job.finish(ImportDone, md, "", time.Now())
	}()
	return job, nil
}

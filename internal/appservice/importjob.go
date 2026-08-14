package appservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/aibuild"
	"github.com/theburrowhub/thaimaturgy/internal/auth"
	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

// importJobTimeout bounds a single AI-import job. AI authoring of a large module
// (with continuation) can take many minutes; this is generous but not unbounded.
const importJobTimeout = 45 * time.Minute

// maxConcurrentImportJobs caps simultaneously-running AI imports, so repeated or
// concurrent requests can't spawn unbounded 45-minute jobs / provider spend.
const maxConcurrentImportJobs = 2

// importJobRetention is how long a finished job is kept (for its status/result to
// be polled) before eviction, so the jobs map can't grow for the process lifetime.
const importJobRetention = 30 * time.Minute

// ErrImportCapacity is returned when too many AI imports are already running.
var ErrImportCapacity = errors.New("too many imports are already running; try again later")

// ImportJobStatus is the lifecycle state of an AI-import job.
type ImportJobStatus string

const (
	ImportRunning ImportJobStatus = "running"
	ImportDone    ImportJobStatus = "done"
	ImportError   ImportJobStatus = "error"
)

// ImportJob tracks one asynchronous AI-import (PDF or images → module). It is
// safe for concurrent access.
type ImportJob struct {
	ID string

	mu          sync.Mutex
	status      ImportJobStatus
	stage       string // latest human-readable progress line
	errMsg      string
	adventureID string // set when done
	adventure   string // resulting title, for display
	createdAt   time.Time
	endedAt     time.Time // when it reached a terminal state (for eviction)
}

// Snapshot returns a JSON-friendly view of the job under its lock.
func (j *ImportJob) Snapshot() map[string]any {
	j.mu.Lock()
	defer j.mu.Unlock()
	m := map[string]any{"id": j.ID, "status": string(j.status), "stage": j.stage}
	if j.errMsg != "" {
		m["error"] = j.errMsg
	}
	if j.adventureID != "" {
		m["adventure_id"] = j.adventureID
		m["adventure_title"] = j.adventure
	}
	return m
}

func (j *ImportJob) setStage(s string) { j.mu.Lock(); j.stage = s; j.mu.Unlock() }
func (j *ImportJob) fail(err error) {
	j.mu.Lock()
	j.status, j.errMsg, j.endedAt = ImportError, err.Error(), time.Now()
	j.mu.Unlock()
}
func (j *ImportJob) finish(id, title string) {
	j.mu.Lock()
	j.status, j.adventureID, j.adventure, j.endedAt = ImportDone, id, title, time.Now()
	j.mu.Unlock()
}

// StartImportJob kicks off an asynchronous AI import from a PDF file (kind
// "pdf") or a directory of images (kind "images") and returns the job id. The
// caller owns srcPath/srcDir (an uploaded temp file/dir); the job removes it and
// any working directory when it finishes.
func (s *Service) StartImportJob(kind, src, title string) (*ImportJob, error) {
	if kind != "pdf" && kind != "images" {
		return nil, fmt.Errorf("unknown import kind %q (want pdf|images)", kind)
	}
	s.mu.Lock()
	prov := s.provider
	cfgCopy := *s.config
	s.mu.Unlock()
	if prov == nil {
		return nil, fmt.Errorf("no AI provider configured")
	}

	now := time.Now()
	s.jobMu.Lock()
	if s.importJobs == nil {
		s.importJobs = make(map[string]*ImportJob)
	}
	// Evict finished jobs past their retention and count the running ones, so
	// admission is bounded (no unbounded goroutines / provider spend / memory).
	running := 0
	for id, j := range s.importJobs {
		j.mu.Lock()
		isRunning := j.status == ImportRunning
		expired := !isRunning && !j.endedAt.IsZero() && now.Sub(j.endedAt) > importJobRetention
		j.mu.Unlock()
		if expired {
			delete(s.importJobs, id)
		} else if isRunning {
			running++
		}
	}
	if running >= maxConcurrentImportJobs {
		s.jobMu.Unlock()
		return nil, ErrImportCapacity
	}
	s.jobSeq++
	id := "imp-" + strconv.Itoa(s.jobSeq)
	job := &ImportJob{ID: id, status: ImportRunning, stage: "starting", createdAt: now}
	s.importJobs[id] = job
	s.jobMu.Unlock()

	go s.runImportJob(job, kind, src, title, &cfgCopy, prov)
	return job, nil
}

// ImportJobByID returns a running/finished job by id.
func (s *Service) ImportJobByID(id string) (*ImportJob, bool) {
	s.jobMu.Lock()
	defer s.jobMu.Unlock()
	j, ok := s.importJobs[id]
	return j, ok
}

func (s *Service) runImportJob(job *ImportJob, kind, src, title string, cfg *domain.Config, prov providers.Provider) {
	defer os.RemoveAll(src) // uploaded PDF file or images dir

	workingDir, err := os.MkdirTemp("", "thaim-webimport-*")
	if err != nil {
		job.fail(err)
		return
	}
	defer os.RemoveAll(workingDir)

	ctx, cancel := context.WithTimeout(context.Background(), importJobTimeout)
	defer cancel()

	progress := aibuild.Progress(func(stage string) { job.setStage(stage) })
	confirm := aibuild.ConfirmFallback(func(_, _ string) bool { return true }) // headless: accept model fallback
	vis := buildVisionProvider(cfg)

	var adv *domain.Adventure
	if kind == "pdf" {
		adv, err = aibuild.FromPDF(ctx, prov, cfg, src, workingDir, title, progress, confirm, vis)
	} else {
		adv, err = aibuild.FromImages(ctx, prov, cfg, src, workingDir, title, progress, confirm, vis)
	}
	if err != nil {
		job.fail(err)
		return
	}

	// Write adventure.json into the working dir, package it, and import it into
	// the library — the same install path the desktop editor uses.
	job.setStage("installing module")
	data, err := json.MarshalIndent(adv, "", "  ")
	if err != nil {
		job.fail(err)
		return
	}
	if err := os.WriteFile(filepath.Join(workingDir, storage.AdventureFile), data, 0o644); err != nil {
		job.fail(err)
		return
	}
	tgz, err := os.CreateTemp("", "thaim-webimport-*.tar.gz")
	if err != nil {
		job.fail(err)
		return
	}
	tgzPath := tgz.Name()
	_ = tgz.Close()
	_ = os.Remove(tgzPath) // PackageModule recreates it
	defer os.Remove(tgzPath)
	if err := storage.PackageModule(workingDir, tgzPath); err != nil {
		job.fail(err)
		return
	}
	imported, err := s.store.ImportModule(tgzPath)
	if err != nil {
		job.fail(err)
		return
	}
	job.finish(imported.ID, imported.Title)
}

// buildVisionProvider returns a vision-capable provider for image curation when
// the primary backend is text-only (e.g. the Claude CLI); nil means the primary
// backend already handles vision. Mirrors the desktop editor.
func buildVisionProvider(cfg *domain.Config) providers.Provider {
	if p := providers.New(cfg); p != nil && p.SupportsVision() {
		return nil
	}
	vcfg := *cfg
	vcfg.Provider = domain.ProviderAnthropic
	vcfg.AnthropicOAuthToken = ""
	vcfg.AnthropicAPIKey = ""
	auth.AutoConfigure(&vcfg)
	return providers.New(&vcfg)
}

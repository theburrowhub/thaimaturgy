package appservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

// maxTotalImportJobs caps the number of retained jobs (running + terminal). When
// full, the oldest terminal jobs are evicted on insertion, so a flood of quickly-
// failing imports can't grow the map with request volume before the TTL elapses.
const maxTotalImportJobs = 20

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
	// Evict finished jobs past their retention, count the running ones, and track
	// terminal jobs (with their end time) so we can also cap the TOTAL retained —
	// otherwise a flood of quickly-failing imports would grow the map within the
	// retention window even though few are running.
	running := 0
	type term struct {
		id    string
		ended time.Time
	}
	var terminal []term
	for id, j := range s.importJobs {
		j.mu.Lock()
		isRunning := j.status == ImportRunning
		ended := j.endedAt
		j.mu.Unlock()
		if !isRunning && !ended.IsZero() && now.Sub(ended) > importJobRetention {
			delete(s.importJobs, id)
			continue
		}
		if isRunning {
			running++
		} else {
			terminal = append(terminal, term{id, ended})
		}
	}
	if running >= maxConcurrentImportJobs {
		s.jobMu.Unlock()
		return nil, ErrImportCapacity
	}
	// Enforce the total cap by evicting the oldest terminal jobs to make room for
	// the new one (running jobs are never evicted).
	sort.Slice(terminal, func(i, j int) bool { return terminal[i].ended.Before(terminal[j].ended) })
	for len(s.importJobs) >= maxTotalImportJobs && len(terminal) > 0 {
		delete(s.importJobs, terminal[0].id)
		terminal = terminal[1:]
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

// runImportJob does the import, ensures ALL temp files (the uploaded source, the
// working dir, the packaged archive) are removed, and only THEN publishes the
// terminal status — so a poller can never observe "done"/"error" while the temp
// artifacts still exist.
func (s *Service) runImportJob(job *ImportJob, kind, src, title string, cfg *domain.Config, prov providers.Provider) {
	id, ttl, err := s.buildImport(job, kind, src, title, cfg, prov)
	_ = os.RemoveAll(src) // uploaded PDF file or images dir
	if err != nil {
		job.fail(err)
		return
	}
	job.finish(id, ttl)
}

// buildImport runs the AI import and installs the module, cleaning up its own
// working dir and packaged archive before it returns (so all cleanup precedes the
// caller publishing a terminal status).
func (s *Service) buildImport(job *ImportJob, kind, src, title string, cfg *domain.Config, prov providers.Provider) (string, string, error) {
	workingDir, err := os.MkdirTemp("", "thaim-webimport-*")
	if err != nil {
		return "", "", err
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
		return "", "", err
	}

	// Write adventure.json into the working dir, package it, and import it into
	// the library — the same install path the desktop editor uses.
	job.setStage("installing module")
	data, err := json.MarshalIndent(adv, "", "  ")
	if err != nil {
		return "", "", err
	}
	if err := os.WriteFile(filepath.Join(workingDir, storage.AdventureFile), data, 0o644); err != nil {
		return "", "", err
	}
	tgz, err := os.CreateTemp("", "thaim-webimport-*.tar.gz")
	if err != nil {
		return "", "", err
	}
	tgzPath := tgz.Name()
	_ = tgz.Close()
	_ = os.Remove(tgzPath) // PackageModule recreates it
	defer os.Remove(tgzPath)
	if err := storage.PackageModule(workingDir, tgzPath); err != nil {
		return "", "", err
	}
	imported, err := s.store.ImportModule(tgzPath)
	if err != nil {
		return "", "", err
	}
	return imported.ID, imported.Title, nil
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

package appservice

import (
	"errors"
	"os"
	"strconv"
	"testing"
	"time"
)

// TestImportJobAdmission verifies the concurrency cap (reject when too many are
// running) and that expired terminal jobs are evicted on admission — both
// deterministically, without launching real AI goroutines.
func TestImportJobAdmission(t *testing.T) {
	svc, _ := newService(t)
	svc.SetProvider(&planProvider{}) // non-nil so admission is reached

	svc.jobMu.Lock()
	svc.importJobs = map[string]*ImportJob{}
	for i := 0; i < maxConcurrentImportJobs; i++ {
		svc.importJobs["run"+strconv.Itoa(i)] = &ImportJob{ID: "run" + strconv.Itoa(i), status: ImportRunning}
	}
	// An old, finished job that should be evicted on the next admission pass.
	svc.importJobs["old"] = &ImportJob{ID: "old", status: ImportDone, endedAt: time.Now().Add(-2 * importJobRetention)}
	svc.jobMu.Unlock()

	// At capacity → rejected with ErrImportCapacity (no goroutine launched).
	if _, err := svc.StartImportJob("images", t.TempDir(), "X"); !errors.Is(err, ErrImportCapacity) {
		t.Fatalf("StartImportJob at capacity = %v; want ErrImportCapacity", err)
	}
	// The expired job was evicted during the admission pass.
	svc.jobMu.Lock()
	_, stillThere := svc.importJobs["old"]
	svc.jobMu.Unlock()
	if stillThere {
		t.Error("an expired terminal job should be evicted on admission")
	}
}

func TestImportJobLifecycle(t *testing.T) {
	svc, _ := newService(t)
	svc.SetProvider(&planProvider{resp: "{}"})

	// An empty images dir makes the AI import fail fast (no images), which is
	// enough to exercise the async job lifecycle and cleanup without a real model.
	src, err := os.MkdirTemp("", "thaim-jobtest-*")
	if err != nil {
		t.Fatalf("tempdir: %v", err)
	}
	job, err := svc.StartImportJob("images", src, "Test")
	if err != nil {
		t.Fatalf("StartImportJob: %v", err)
	}
	if _, ok := svc.ImportJobByID(job.ID); !ok {
		t.Error("job should be retrievable by id")
	}

	// Wait for it to settle (it should end in error, not run forever).
	var status string
	for i := 0; i < 200; i++ {
		status, _ = job.Snapshot()["status"].(string)
		if status != "running" {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if status != "error" {
		t.Fatalf("job status = %q; want error", status)
	}
	// The job removed its uploaded source dir.
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("import job should remove its source dir, stat err = %v", err)
	}

	// Unknown kind is rejected synchronously.
	if _, err := svc.StartImportJob("bogus", "/nope", ""); err == nil {
		t.Error("unknown kind should error")
	}
}

func TestStartImportJobNoProvider(t *testing.T) {
	svc, _ := newService(t) // provider is nil
	if _, err := svc.StartImportJob("pdf", "/nope", ""); err == nil {
		t.Error("import without a provider should error")
	}
}

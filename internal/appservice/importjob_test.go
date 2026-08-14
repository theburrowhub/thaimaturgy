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

// TestImportJobTotalCap verifies that when the registry is full of terminal jobs,
// admitting a new one evicts the oldest terminal job(s) rather than growing
// unboundedly — even though terminal jobs don't count toward the running cap.
func TestImportJobTotalCap(t *testing.T) {
	svc, _ := newService(t)
	svc.SetProvider(&planProvider{})

	base := time.Now().Add(-time.Minute) // recent (not past retention) terminal jobs
	svc.jobMu.Lock()
	svc.importJobs = map[string]*ImportJob{}
	for i := 0; i < maxTotalImportJobs; i++ {
		svc.importJobs["done"+strconv.Itoa(i)] = &ImportJob{
			ID: "done" + strconv.Itoa(i), status: ImportDone, endedAt: base.Add(time.Duration(i) * time.Second),
		}
	}
	svc.jobMu.Unlock()

	// Admitting a new job (no running jobs → under the concurrency cap) must evict
	// the oldest terminal one and keep the map at the cap.
	src, _ := os.MkdirTemp("", "thaim-capbtest-*")
	job, err := svc.StartImportJob("images", src, "X")
	if err != nil {
		t.Fatalf("StartImportJob should be admitted (running under cap): %v", err)
	}
	svc.jobMu.Lock()
	_, oldestGone := svc.importJobs["done0"]
	total := len(svc.importJobs)
	svc.jobMu.Unlock()
	if oldestGone {
		t.Error("the oldest terminal job (done0) should have been evicted")
	}
	if total > maxTotalImportJobs {
		t.Errorf("retained jobs = %d; must not exceed cap %d", total, maxTotalImportJobs)
	}
	_ = job
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

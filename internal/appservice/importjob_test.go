package appservice

import (
	"os"
	"testing"
	"time"
)

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

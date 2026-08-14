package appservice

import (
	"errors"
	"testing"
	"time"
)

// waitNovelJob polls a novel job until it leaves "running" (or times out).
func waitNovelJob(t *testing.T, svc *Service, id string) map[string]any {
	t.Helper()
	for i := 0; i < 200; i++ {
		j, ok := svc.NovelJobByID(id)
		if !ok {
			t.Fatalf("job %s vanished", id)
		}
		snap := j.Snapshot()
		if snap["status"] != "running" {
			return snap
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("job %s never finished", id)
	return nil
}

func TestNovelTextOptimisticConcurrency(t *testing.T) {
	svc, _ := newService(t)
	defer svc.CloseSession("crypt")
	name, _ := svc.NewSession("crypt")

	// No novel yet.
	md, ver, exists, err := svc.NovelText(name)
	if err != nil || exists || md != "" || ver != "" {
		t.Fatalf("NovelText of a novel-less session = (%q,%q,%v,%v)", md, ver, exists, err)
	}

	// Saving over "nothing" (baseVersion "") succeeds and yields a version.
	v1, err := svc.SaveNovelText(name, "first draft", "")
	if err != nil || v1 == "" {
		t.Fatalf("SaveNovelText initial = (%q,%v)", v1, err)
	}
	md, ver, exists, _ = svc.NovelText(name)
	if !exists || md != "first draft" || ver != v1 {
		t.Fatalf("NovelText after save = (%q,%q,%v)", md, ver, exists)
	}

	// A stale base version is rejected (someone else changed it).
	if _, err := svc.SaveNovelText(name, "clobber", ""); !errors.Is(err, ErrNovelConflict) {
		t.Errorf("stale save = %v; want ErrNovelConflict", err)
	}
	// The current version still saves.
	v2, err := svc.SaveNovelText(name, "second draft", v1)
	if err != nil || v2 == v1 {
		t.Fatalf("SaveNovelText with current version = (%q,%v)", v2, err)
	}
	if md, _, _, _ := svc.NovelText(name); md != "second draft" {
		t.Errorf("final novel = %q; want second draft", md)
	}
}

// A finished generate job associates its result with the session (persists it).
func TestGenerateJobPersistsNovel(t *testing.T) {
	svc, _ := newService(t)
	defer svc.CloseSession("crypt")
	svc.SetProvider(&planProvider{resp: "# The Tale\n\n## One\nIt happened."})
	name, _ := svc.NewSession("crypt")
	// Give the timeline a narratable beat so Generate produces content.
	if _, err := svc.ExecuteCommand(name, "/note the door creaked open"); err != nil {
		t.Fatalf("note: %v", err)
	}

	job, err := svc.StartNovelJob(name)
	if err != nil {
		t.Fatalf("StartNovelJob: %v", err)
	}
	snap := waitNovelJob(t, svc, job.ID)
	if snap["status"] != "done" {
		t.Fatalf("generate job status = %v (%v)", snap["status"], snap["error"])
	}
	if snap["kind"] != "generate" {
		t.Errorf("kind = %v; want generate", snap["kind"])
	}
	md, _, exists, err := svc.NovelText(name)
	if err != nil || !exists {
		t.Fatalf("generated novel not associated with session: exists=%v err=%v", exists, err)
	}
	if md == "" {
		t.Error("persisted novel is empty")
	}
}

// An adjust job returns a revised text WITHOUT persisting it — the caller saves
// explicitly, so an adjustment stays undoable.
func TestAdjustJobDoesNotPersist(t *testing.T) {
	svc, _ := newService(t)
	defer svc.CloseSession("crypt")
	svc.SetProvider(&planProvider{resp: "The revised prose."})
	name, _ := svc.NewSession("crypt")

	job, err := svc.StartNovelAdjustJob(name, "The original prose.", "", "make it better")
	if err != nil {
		t.Fatalf("StartNovelAdjustJob: %v", err)
	}
	snap := waitNovelJob(t, svc, job.ID)
	if snap["status"] != "done" {
		t.Fatalf("adjust job status = %v (%v)", snap["status"], snap["error"])
	}
	if snap["kind"] != "adjust" {
		t.Errorf("kind = %v; want adjust", snap["kind"])
	}
	if md, ready := job.Markdown(); !ready || md != "The revised prose." {
		t.Errorf("adjust result = (%q,%v); want the revised prose", md, ready)
	}
	// Nothing was persisted to the session.
	if _, _, exists, _ := svc.NovelText(name); exists {
		t.Error("adjust must not persist; the session should still have no saved novel")
	}
}

func TestNovelJobRequiresProviderAndSession(t *testing.T) {
	svc, _ := newService(t) // provider nil
	if _, err := svc.StartNovelJob("nope"); err == nil {
		t.Error("novel job on an unopened session should error")
	}
	if _, err := svc.StartNovelAdjustJob("nope", "t", "", "x"); err == nil {
		t.Error("adjust job on an unopened session should error")
	}
}

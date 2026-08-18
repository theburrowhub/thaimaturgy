package bundlepack

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/rules"
	"github.com/theburrowhub/thaimaturgy/internal/rules/starlarkruntime"
)

func exampleSource(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate pack test")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "../../../examples/rules/simple-d6"))
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		t.Fatalf("example source %q: %v", path, err)
	}
	return path
}

func mustPayload(t *testing.T, value any) rules.Payload {
	t.Helper()
	payload, err := rules.PayloadFrom(value)
	if err != nil {
		t.Fatalf("PayloadFrom: %v", err)
	}
	return payload
}

func TestPackIsDeterministicAndIdempotent(t *testing.T) {
	ctx := context.Background()
	outputDirectory := t.TempDir()
	firstPath := filepath.Join(outputDirectory, "first.rules.zip")
	secondPath := filepath.Join(outputDirectory, "second.rules.zip")

	first, err := Pack(ctx, exampleSource(t), firstPath, nil)
	if err != nil {
		t.Fatalf("first Pack: %v", err)
	}
	second, err := Pack(ctx, exampleSource(t), secondPath, nil)
	if err != nil {
		t.Fatalf("second Pack: %v", err)
	}
	firstBytes, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("identical source trees produced different bundle bytes")
	}
	if first.Loaded.Artifact.Lock() != second.Loaded.Artifact.Lock() {
		t.Fatalf("deterministic bytes produced different locks: %+v / %+v", first.Loaded.Artifact.Lock(), second.Loaded.Artifact.Lock())
	}
	lock := first.Loaded.Artifact.Lock()
	if lock.ID != "simple-d6" || lock.Version != "0.1.0" || lock.ProtocolVersion != rules.ProtocolVersion {
		t.Fatalf("unexpected example lock: %+v", lock)
	}

	idempotent, err := Pack(ctx, exampleSource(t), firstPath, nil)
	if err != nil {
		t.Fatalf("idempotent Pack: %v", err)
	}
	if idempotent.Loaded.Artifact.Lock() != lock {
		t.Fatalf("idempotent Pack lock = %+v, want %+v", idempotent.Loaded.Artifact.Lock(), lock)
	}
	assertCanonicalArchive(t, firstPath)
}

func assertCanonicalArchive(t *testing.T, bundlePath string) {
	t.Helper()
	archive, err := zip.OpenReader(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	wantNames := []string{"README.md", "main.star", "ruleset.json"}
	if len(archive.File) != len(wantNames) {
		t.Fatalf("ZIP entries = %d, want %d", len(archive.File), len(wantNames))
	}
	for index, file := range archive.File {
		if file.Name != wantNames[index] {
			t.Fatalf("ZIP entry %d = %q, want %q", index, file.Name, wantNames[index])
		}
		if !file.Modified.Equal(deterministicZIPTime) {
			t.Fatalf("ZIP entry %q timestamp = %s", file.Name, file.Modified)
		}
		if file.Method != zip.Store || !file.Mode().IsRegular() || file.Mode().Perm() != archiveFileMode {
			t.Fatalf("ZIP entry %q metadata: method=%d mode=%s", file.Name, file.Method, file.Mode())
		}
	}
}

func TestPackRejectsUnsafeOrInvalidSourcesWithoutPublishing(t *testing.T) {
	t.Run("source root symlink", func(t *testing.T) {
		realSource := t.TempDir()
		link := filepath.Join(t.TempDir(), "source")
		if err := os.Symlink(realSource, link); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		output := filepath.Join(t.TempDir(), "output.rules.zip")
		if _, err := Pack(context.Background(), link, output, nil); !errors.Is(err, ErrInvalidSource) {
			t.Fatalf("Pack error = %v, want ErrInvalidSource", err)
		}
		assertAbsent(t, output)
	})

	t.Run("nested symlink", func(t *testing.T) {
		source := t.TempDir()
		outside := filepath.Join(t.TempDir(), "outside.star")
		if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(source, "main.star")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		output := filepath.Join(t.TempDir(), "output.rules.zip")
		if _, err := Pack(context.Background(), source, output, nil); !errors.Is(err, ErrInvalidSource) {
			t.Fatalf("Pack error = %v, want ErrInvalidSource", err)
		}
		assertAbsent(t, output)
	})

	t.Run("non-portable path", func(t *testing.T) {
		source := t.TempDir()
		if err := os.WriteFile(filepath.Join(source, "not portable.star"), []byte("pass"), 0o600); err != nil {
			t.Fatal(err)
		}
		output := filepath.Join(t.TempDir(), "output.rules.zip")
		if _, err := Pack(context.Background(), source, output, nil); !errors.Is(err, ErrInvalidSource) {
			t.Fatalf("Pack error = %v, want ErrInvalidSource", err)
		}
		assertAbsent(t, output)
	})

	t.Run("output inside source", func(t *testing.T) {
		source := t.TempDir()
		output := filepath.Join(source, "output.rules.zip")
		if _, err := Pack(context.Background(), source, output, nil); !errors.Is(err, ErrInvalidSource) {
			t.Fatalf("Pack error = %v, want ErrInvalidSource", err)
		}
		assertAbsent(t, output)
	})

	t.Run("output resolves inside source through ancestor symlink", func(t *testing.T) {
		source := t.TempDir()
		if err := os.Mkdir(filepath.Join(source, "generated"), 0o700); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(t.TempDir(), "source-link")
		if err := os.Symlink(source, link); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		output := filepath.Join(link, "generated", "output.rules.zip")
		if _, err := Pack(context.Background(), source, output, nil); !errors.Is(err, ErrInvalidSource) {
			t.Fatalf("Pack error = %v, want ErrInvalidSource", err)
		}
		assertAbsent(t, filepath.Join(source, "generated", "output.rules.zip"))
	})

	t.Run("oversized source", func(t *testing.T) {
		source := t.TempDir()
		tooLarge := make([]byte, starlarkruntime.DefaultLimits().MaxSourceFileBytes+1)
		if err := os.WriteFile(filepath.Join(source, "main.star"), tooLarge, 0o600); err != nil {
			t.Fatal(err)
		}
		output := filepath.Join(t.TempDir(), "output.rules.zip")
		if _, err := Pack(context.Background(), source, output, nil); !errors.Is(err, starlarkruntime.ErrBundleTooLarge) {
			t.Fatalf("Pack error = %v, want ErrBundleTooLarge", err)
		}
		assertAbsent(t, output)
	})

	t.Run("too many source entries", func(t *testing.T) {
		source := t.TempDir()
		for index := 0; index <= starlarkruntime.DefaultLimits().MaxFiles; index++ {
			if err := os.Mkdir(filepath.Join(source, fmt.Sprintf("entry-%03d", index)), 0o700); err != nil {
				t.Fatal(err)
			}
		}
		output := filepath.Join(t.TempDir(), "output.rules.zip")
		if _, err := Pack(context.Background(), source, output, nil); !errors.Is(err, starlarkruntime.ErrBundleTooLarge) {
			t.Fatalf("Pack error = %v, want ErrBundleTooLarge", err)
		}
		assertAbsent(t, output)
	})

	t.Run("invalid executable contract", func(t *testing.T) {
		source := t.TempDir()
		if err := os.WriteFile(filepath.Join(source, "ruleset.json"), []byte(`{"not":"a manifest"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		output := filepath.Join(t.TempDir(), "output.rules.zip")
		if _, err := Pack(context.Background(), source, output, nil); err == nil || !strings.Contains(err.Error(), "validate output") {
			t.Fatalf("Pack error = %v, want loader validation failure", err)
		}
		assertAbsent(t, output)
	})

	t.Run("ruleset rejects initial state", func(t *testing.T) {
		source := t.TempDir()
		manifest, err := os.ReadFile(filepath.Join(exampleSource(t), "ruleset.json"))
		if err != nil {
			t.Fatal(err)
		}
		program, err := os.ReadFile(filepath.Join(exampleSource(t), "main.star"))
		if err != nil {
			t.Fatal(err)
		}
		program = bytes.Replace(program, []byte(`"attempts": 0,`), []byte(`"attempts": 1,`), 1)
		if err := os.WriteFile(filepath.Join(source, "ruleset.json"), manifest, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(source, "main.star"), program, 0o600); err != nil {
			t.Fatal(err)
		}
		output := filepath.Join(t.TempDir(), "output.rules.zip")
		if _, err := Pack(context.Background(), source, output, nil); err == nil || !strings.Contains(err.Error(), "rejected initial state") {
			t.Fatalf("Pack error = %v, want initial-state validation failure", err)
		}
		assertAbsent(t, output)
	})

	t.Run("canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		output := filepath.Join(t.TempDir(), "output.rules.zip")
		if _, err := Pack(ctx, exampleSource(t), output, nil); !errors.Is(err, context.Canceled) {
			t.Fatalf("Pack error = %v, want context.Canceled", err)
		}
		assertAbsent(t, output)
	})
}

func TestPackPreservesConflictingOrSymlinkDestinations(t *testing.T) {
	t.Run("different regular file", func(t *testing.T) {
		output := filepath.Join(t.TempDir(), "output.rules.zip")
		want := []byte("do not overwrite")
		if err := os.WriteFile(output, want, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Pack(context.Background(), exampleSource(t), output, nil); !errors.Is(err, ErrDestinationConflict) {
			t.Fatalf("Pack error = %v, want ErrDestinationConflict", err)
		}
		got, err := os.ReadFile(output)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("conflicting destination changed to %q", got)
		}
	})

	t.Run("symlink", func(t *testing.T) {
		outside := filepath.Join(t.TempDir(), "outside")
		want := []byte("outside remains unchanged")
		if err := os.WriteFile(outside, want, 0o600); err != nil {
			t.Fatal(err)
		}
		output := filepath.Join(t.TempDir(), "output.rules.zip")
		if err := os.Symlink(outside, output); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		if _, err := Pack(context.Background(), exampleSource(t), output, nil); err == nil || !strings.Contains(err.Error(), "not a symlink") {
			t.Fatalf("Pack error = %v, want symlink rejection", err)
		}
		got, err := os.ReadFile(outside)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("symlink target changed to %q", got)
		}
	})
}

func TestSimpleD6ExampleRunsCompleteStatefulProtocolAndReplays(t *testing.T) {
	ctx := context.Background()
	packed, err := Pack(ctx, exampleSource(t), filepath.Join(t.TempDir(), "simple-d6.rules.zip"), nil)
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	implementation := packed.Loaded.Ruleset
	lock := packed.Loaded.Artifact.Lock()
	initial := packed.Loaded.InitialState
	principal := rules.Principal{ID: "player-1", Kind: "human", Roles: []string{"participant"}}
	initialSnapshot := rules.Snapshot{Ruleset: lock, State: initial}

	if err := implementation.ValidateState(ctx, rules.ValidateStateRequest{Snapshot: initialSnapshot}); err != nil {
		t.Fatalf("ValidateState(initial): %v", err)
	}
	actions, err := implementation.ListActions(ctx, rules.CatalogRequest{Snapshot: initialSnapshot, Principal: principal})
	if err != nil || len(actions) != 1 || actions[0].ID != "simple_d6.check" {
		t.Fatalf("ListActions = %#v, %v", actions, err)
	}
	startRequest := rules.StartRequest{
		Snapshot:  initialSnapshot,
		Principal: principal,
		Intent: rules.Intent{
			ID:        "intent-1",
			ActionID:  "simple_d6.check",
			ActorID:   "hero-1",
			Arguments: mustPayload(t, map[string]any{"modifier": 2, "target": 6}),
		},
	}
	first, err := implementation.Start(ctx, startRequest)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if first.Kind != rules.StepKindNeedRandom || first.NeedRandom == nil || first.NeedRandom.Method != "dice.roll" {
		t.Fatalf("first step = %#v, want dice.roll", first)
	}
	var specification struct {
		Count int `json:"count"`
		Sides int `json:"sides"`
	}
	if err := json.Unmarshal(first.NeedRandom.Specification.Bytes(), &specification); err != nil || specification.Count != 1 || specification.Sides != 6 {
		t.Fatalf("random specification = %+v, %v", specification, err)
	}
	randomPending, err := first.Pending()
	if err != nil {
		t.Fatal(err)
	}
	emission, err := implementation.Resume(ctx, rules.ResumeRequest{
		Snapshot: initialSnapshot, Principal: principal, Pending: randomPending,
		Response: rules.HostResponse{StepID: randomPending.StepID, Kind: randomPending.Kind, Data: mustPayload(t, map[string]any{"rolls": []int{4}})},
	})
	if err != nil {
		t.Fatalf("Resume(random): %v", err)
	}
	if emission.Kind != rules.StepKindEmit || emission.Emit == nil || len(emission.Emit.Events) != 1 {
		t.Fatalf("random resume = %#v, want one emitted event", emission)
	}

	reduced, err := implementation.Reduce(ctx, rules.ReduceRequest{Snapshot: initialSnapshot, Events: emission.Emit.Events})
	if err != nil {
		t.Fatalf("Reduce: %v", err)
	}
	committed := rules.Snapshot{Ruleset: lock, Revision: 1, State: reduced.State}
	if err := implementation.ValidateState(ctx, rules.ValidateStateRequest{Snapshot: committed}); err != nil {
		t.Fatalf("ValidateState(committed): %v", err)
	}
	replayed, err := implementation.Reduce(ctx, rules.ReduceRequest{Snapshot: initialSnapshot, Events: emission.Emit.Events})
	if err != nil {
		t.Fatalf("replay Reduce: %v", err)
	}
	if replayed.State != reduced.State {
		t.Fatalf("replayed state = %s, want %s", replayed.State.String(), reduced.State.String())
	}

	emitPending, err := emission.Pending()
	if err != nil {
		t.Fatal(err)
	}
	complete, err := implementation.Resume(ctx, rules.ResumeRequest{
		Snapshot: committed, Principal: principal, Pending: emitPending,
		Response: rules.HostResponse{StepID: emitPending.StepID, Kind: emitPending.Kind, Data: mustPayload(t, map[string]any{"base_revision": 0, "revision": 1})},
	})
	if err != nil {
		t.Fatalf("Resume(emit): %v", err)
	}
	if complete.Kind != rules.StepKindComplete || complete.Complete == nil || complete.Complete.Outcome != "simple_d6.check.success" {
		t.Fatalf("completion = %#v", complete)
	}
	var result struct {
		Attempts int  `json:"attempts"`
		Roll     int  `json:"roll"`
		Total    int  `json:"total"`
		Success  bool `json:"success"`
	}
	if err := json.Unmarshal(complete.Complete.Result.Bytes(), &result); err != nil || result.Attempts != 1 || result.Roll != 4 || result.Total != 6 || !result.Success {
		t.Fatalf("completion result = %+v, %v", result, err)
	}

	projection, err := implementation.Project(ctx, rules.ProjectRequest{Snapshot: committed, Principal: principal})
	if err != nil || projection.View != committed.State {
		t.Fatalf("Project = %#v, %v", projection, err)
	}
	explanation, err := implementation.Explain(ctx, rules.ExplainRequest{Snapshot: committed, Principal: principal, Reference: "simple_d6.check", Locale: "en"})
	if err != nil || !strings.Contains(explanation.Text, "six-sided") {
		t.Fatalf("Explain = %#v, %v", explanation, err)
	}
	migrated, err := implementation.Migrate(ctx, rules.MigrateRequest{From: lock, State: committed.State})
	if err != nil || migrated.State != committed.State {
		t.Fatalf("Migrate = %#v, %v", migrated, err)
	}

	repeated, err := implementation.Start(ctx, startRequest)
	if err != nil || !reflect.DeepEqual(repeated, first) {
		t.Fatalf("Start is not deterministic:\nfirst=%#v\nagain=%#v\nerror=%v", first, repeated, err)
	}
}

func assertAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("path %q exists after rejected pack: %v", path, err)
	}
}

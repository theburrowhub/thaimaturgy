package bundlestore

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/rules"
	"github.com/theburrowhub/thaimaturgy/internal/rules/catalog"
	"github.com/theburrowhub/thaimaturgy/internal/rules/starlarkruntime"
)

func testBundle(t *testing.T, id, version string) []byte {
	return testBundleVariant(t, id, version, `{"counter": 0}`, "primary")
}

func testBundleWithInitial(t *testing.T, id, version, initial string) []byte {
	return testBundleVariant(t, id, version, initial, "primary")
}

func testBundleVariant(t *testing.T, id, version, initial, marker string) []byte {
	t.Helper()
	manifest := rules.Manifest{
		ID: id, Name: "Store test", Version: version, ProtocolVersion: rules.ProtocolVersion,
		Runtime:      rules.Runtime{Kind: rules.RuntimeStarlark, Entrypoint: "main.star"},
		Capabilities: []string{"test.echo"},
	}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	source := fmt.Sprintf(`def manifest():
    return %s
def initial_state():
    return %s
def list_actions(request):
    return [{"id":"test.echo","label":"Echo","input_schema":{"type":"object"}}]
def start(request):
    return {"id":request["intent"]["id"],"kind":"complete","complete":{"outcome":"test.echoed","result":request["intent"]["arguments"]}}
def resume(request):
    return {"id":request["pending"]["step_id"],"kind":"complete","complete":{"outcome":"test.resumed","result":request["response"]["data"]}}
def project(request):
    return {"view":request["snapshot"]["state"]}
def explain(request):
    return {"text":"Store test"}
def validate_state(request):
    return None if request["snapshot"]["state"].get("counter") != None else "counter is required"
def reduce(request):
    return {"state":request["snapshot"]["state"]}
def migrate(request):
    return {"state":request["state"]}
# %s
`, manifestRaw, initial, marker)
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for name, contents := range map[string][]byte{
		starlarkruntime.ManifestPath: manifestRaw,
		"main.star":                  []byte(source),
	} {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(contents); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestStoreRejectsReleaseEquivocation(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), DirectoryName), nil)
	if err != nil {
		t.Fatal(err)
	}
	first := testBundleVariant(t, "test.release", "1.0.0", `{"counter": 0}`, "first")
	second := testBundleVariant(t, "test.release", "1.0.0", `{"counter": 0}`, "second")
	installed, err := store.Install(context.Background(), bytes.NewReader(first))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Install(context.Background(), bytes.NewReader(second)); !errors.Is(err, rules.ErrArtifactConflict) {
		t.Fatalf("second release digest error = %v", err)
	}

	// Manual store tampering also fails closed: discovery returns neither digest,
	// so lexical path order cannot decide which release wins.
	loadedSecond, err := store.loader.Load(context.Background(), bytes.NewReader(second))
	if err != nil {
		t.Fatal(err)
	}
	secondPath := filepath.Join(store.Root(), canonicalRelativePath(loadedSecond.Artifact.Lock()))
	if err := os.WriteFile(secondPath, second, 0o600); err != nil {
		t.Fatal(err)
	}
	report := store.Discover(context.Background())
	if len(report.Bundles) != 0 || len(report.Failures) != 2 || !errors.Is(report.Err(), rules.ErrArtifactConflict) {
		t.Fatalf("equivocating discover = %#v, %v; first=%s", report, report.Err(), installed.Path)
	}
}

func TestConcurrentStoreInstancesCannotEquivocateRelease(t *testing.T) {
	root := filepath.Join(t.TempDir(), DirectoryName)
	firstStore, err := New(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	secondStore, err := New(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	first := testBundleVariant(t, "test.concurrent", "1.0.0", `{"counter": 0}`, "first")
	second := testBundleVariant(t, "test.concurrent", "1.0.0", `{"counter": 0}`, "second")

	start := make(chan struct{})
	results := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	install := func(store *Store, bundle []byte) {
		ready.Done()
		<-start
		_, err := store.Install(context.Background(), bytes.NewReader(bundle))
		results <- err
	}
	go install(firstStore, first)
	go install(secondStore, second)
	ready.Wait()
	close(start)

	successes, conflicts := 0, 0
	for range 2 {
		err := <-results
		switch {
		case err == nil:
			successes++
		case errors.Is(err, rules.ErrArtifactConflict):
			conflicts++
		default:
			t.Fatalf("concurrent Install error = %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent installs: successes=%d conflicts=%d", successes, conflicts)
	}
	report := firstStore.Discover(context.Background())
	if len(report.Bundles) != 1 || report.Err() != nil {
		t.Fatalf("concurrent install left an equivocated store: %#v, %v", report, report.Err())
	}
}

func TestInstallDiscoverAndRegisterExactBundle(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), DirectoryName), nil)
	if err != nil {
		t.Fatal(err)
	}
	bundle := testBundle(t, "test.store", "1.2.3")
	installed, err := store.Install(context.Background(), bytes.NewReader(bundle))
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(store.Root(), canonicalRelativePath(installed.Loaded.Artifact.Lock()))
	if installed.Path != wantPath {
		t.Fatalf("installed path = %q, want %q", installed.Path, wantPath)
	}
	stored, err := os.ReadFile(installed.Path)
	if err != nil || !bytes.Equal(stored, bundle) {
		t.Fatalf("stored exact bytes = %v, %v", bytes.Equal(stored, bundle), err)
	}
	// Installation of the same immutable artifact is idempotent.
	second, err := store.Install(context.Background(), bytes.NewReader(bundle))
	if err != nil || second.Path != installed.Path {
		t.Fatalf("second install = %#v, %v", second, err)
	}

	report := store.Discover(context.Background())
	if len(report.Bundles) != 1 || report.Err() != nil {
		t.Fatalf("discover = %#v, %v", report, report.Err())
	}
	registry := catalog.New()
	locks, err := store.RegisterAll(context.Background(), registry)
	if err != nil || len(locks) != 1 || locks[0] != installed.Loaded.Artifact.Lock() {
		t.Fatalf("RegisterAll = %#v, %v", locks, err)
	}
	initial, err := registry.InitialState(locks[0])
	if err != nil || initial.String() != `{"counter":0}` {
		t.Fatalf("initial state = %s, %v", initial.String(), err)
	}
}

func TestInvalidInstallPublishesNothing(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), DirectoryName), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Install(context.Background(), strings.NewReader("not a ZIP")); err == nil {
		t.Fatal("invalid bundle installed")
	}
	if _, err := store.Install(context.Background(), bytes.NewReader(
		testBundleWithInitial(t, "test.bad-state", "1.0.0", `{}`),
	)); err == nil || !strings.Contains(err.Error(), "rejected initial state") {
		t.Fatalf("semantically invalid initial state error = %v", err)
	}
	var files []string
	if err := filepath.WalkDir(store.Root(), func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			files = append(files, path)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("invalid install left files: %v", files)
	}
}

func TestDiscoveryRejectsMislabeledAndSymlinkedArtifactsButKeepsHealthyOnes(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), DirectoryName), nil)
	if err != nil {
		t.Fatal(err)
	}
	healthy, err := store.Install(context.Background(), bytes.NewReader(testBundle(t, "test.healthy", "1.0.0")))
	if err != nil {
		t.Fatal(err)
	}
	mislabeled := filepath.Join(store.Root(), "wrong", "1.0.0", "bad"+BundleExtension)
	if err := os.MkdirAll(filepath.Dir(mislabeled), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mislabeled, testBundle(t, "test.other", "1.0.0"), 0o600); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(store.Root(), "linked"+BundleExtension)
	if err := os.Symlink(healthy.Path, symlink); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	report := store.Discover(context.Background())
	if len(report.Bundles) != 1 || report.Bundles[0].Path != healthy.Path || len(report.Failures) != 2 {
		t.Fatalf("discover bundles=%v failures=%v", report.Bundles, report.Failures)
	}
	if !errors.Is(report.Err(), ErrStoredArtifactMismatch) || !strings.Contains(report.Err().Error(), "symlinks are forbidden") {
		t.Fatalf("discover error = %v", report.Err())
	}
}

func TestStoreRejectsSymlinkRootAndSource(t *testing.T) {
	base := t.TempDir()
	realRoot := filepath.Join(base, "real")
	if err := os.Mkdir(realRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	linkedRoot := filepath.Join(base, "linked")
	if err := os.Symlink(realRoot, linkedRoot); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := New(linkedRoot, nil); err == nil {
		t.Fatal("symlink root accepted")
	}
	store, err := New(filepath.Join(base, "store"), nil)
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(base, "source"+BundleExtension)
	if err := os.WriteFile(source, testBundle(t, "test.source", "1.0.0"), 0o600); err != nil {
		t.Fatal(err)
	}
	linkedSource := filepath.Join(base, "source-link"+BundleExtension)
	if err := os.Symlink(source, linkedSource); err != nil {
		t.Fatal(err)
	}
	if _, err := store.InstallFile(context.Background(), linkedSource); err == nil {
		t.Fatal("symlink source accepted")
	}
}

func TestCanceledStoreOperationsFailEvenWhenStoreIsEmpty(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), DirectoryName), nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if report := store.Discover(ctx); !errors.Is(report.Err(), context.Canceled) {
		t.Fatalf("Discover error = %v, want context.Canceled", report.Err())
	}
	if _, err := store.RegisterAll(ctx, catalog.New()); !errors.Is(err, context.Canceled) {
		t.Fatalf("RegisterAll error = %v, want context.Canceled", err)
	}
	if _, err := store.Install(ctx, strings.NewReader("ignored")); !errors.Is(err, context.Canceled) {
		t.Fatalf("Install error = %v, want context.Canceled", err)
	}
}

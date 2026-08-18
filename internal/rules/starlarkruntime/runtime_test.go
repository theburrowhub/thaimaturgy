package starlarkruntime

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	core "github.com/theburrowhub/thaimaturgy/internal/rules"
)

type archiveEntry struct {
	name     string
	contents string
	mode     os.FileMode
}

func testManifest() core.Manifest {
	return core.Manifest{
		ID:              "test.rules",
		Name:            "Test Rules",
		Description:     "Deterministic test ruleset",
		Version:         "1.2.3",
		ProtocolVersion: core.ProtocolVersion,
		Runtime: core.Runtime{
			Kind:       core.RuntimeStarlark,
			Entrypoint: "main.star",
		},
		Capabilities: []string{"test.echo"},
	}
}

func manifestJSON(t *testing.T, manifest core.Manifest) string {
	t.Helper()
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	return string(raw)
}

func validSource(t *testing.T, declared core.Manifest) string {
	t.Helper()
	return fmt.Sprintf(`def manifest():
    return %s

def initial_state():
    return {"counter": 0}

def list_actions(request):
    return [{
        "id": "test.echo",
        "label": "Echo",
        "description": "Return the supplied arguments",
        "input_schema": {"type": "object"},
        "tags": ["test"],
    }]

def start(request):
    return {
        "id": request["intent"]["id"],
        "kind": "complete",
        "complete": {
            "outcome": "test.echoed",
            "result": {"echo": request["intent"]["arguments"]},
        },
    }

def resume(request):
    return {
        "id": request["pending"]["step_id"],
        "kind": "complete",
        "complete": {
            "outcome": "test.resumed",
            "result": request["response"]["data"],
        },
    }

def project(request):
    return {"view": request["snapshot"]["state"]}

def explain(request):
    return {"text": "Explanation for " + request["reference"]}

def validate_state(request):
    return None

def reduce(request):
    return {"state": request["snapshot"]["state"]}

def migrate(request):
    return {"state": request["state"]}
`, manifestJSON(t, declared))
}

func makeBundle(t *testing.T, manifest core.Manifest, source string, extras ...archiveEntry) []byte {
	t.Helper()
	entries := []archiveEntry{
		{name: ManifestPath, contents: manifestJSON(t, manifest)},
		{name: manifest.Runtime.Entrypoint, contents: source},
	}
	entries = append(entries, extras...)
	return makeArchive(t, entries)
}

func makeArchive(t *testing.T, entries []archiveEntry) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		mode := entry.mode
		if mode == 0 {
			mode = 0o600
		}
		header.SetMode(mode)
		file, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatalf("create ZIP entry %q: %v", entry.name, err)
		}
		if _, err := file.Write([]byte(entry.contents)); err != nil {
			t.Fatalf("write ZIP entry %q: %v", entry.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close ZIP: %v", err)
	}
	return output.Bytes()
}

func newTestLoader(t *testing.T, limits Limits) *Loader {
	t.Helper()
	loader, err := NewLoader(limits)
	if err != nil {
		t.Fatalf("NewLoader: %v", err)
	}
	return loader
}

func loadValid(t *testing.T, loader *Loader) LoadedBundle {
	t.Helper()
	manifest := testManifest()
	bundle := makeBundle(t, manifest, validSource(t, manifest))
	loaded, err := loader.Load(context.Background(), bytes.NewReader(bundle))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return loaded
}

func testPayload(t *testing.T, value any) core.Payload {
	t.Helper()
	payload, err := core.PayloadFrom(value)
	if err != nil {
		t.Fatalf("PayloadFrom: %v", err)
	}
	return payload
}

func testSnapshot(t *testing.T, loaded LoadedBundle) core.Snapshot {
	t.Helper()
	return core.Snapshot{
		Ruleset:  loaded.Artifact.Lock(),
		Revision: 7,
		State:    testPayload(t, map[string]any{"counter": 2}),
	}
}

func testPrincipal() core.Principal {
	return core.Principal{ID: "user-1", Kind: "user", Roles: []string{"player"}}
}

func TestLoaderAdaptsCompleteRulesetContract(t *testing.T) {
	loader := newTestLoader(t, Limits{})
	loaded := loadValid(t, loader)
	ctx := context.Background()

	registry := core.NewRegistry()
	if err := registry.Register(ctx, loaded.Artifact, loaded.Ruleset); err != nil {
		t.Fatalf("Register: %v", err)
	}
	manifest, err := loaded.Ruleset.Manifest(ctx)
	if err != nil {
		t.Fatalf("Manifest: %v", err)
	}
	if !reflect.DeepEqual(manifest, testManifest()) {
		t.Fatalf("manifest = %#v", manifest)
	}
	if loaded.InitialState.String() != `{"counter":0}` || loaded.Ruleset.InitialState() != loaded.InitialState {
		t.Fatalf("initial state = %s", loaded.InitialState.String())
	}

	snapshot := testSnapshot(t, loaded)
	principal := testPrincipal()
	actions, err := loaded.Ruleset.ListActions(ctx, core.CatalogRequest{Snapshot: snapshot, Principal: principal})
	if err != nil {
		t.Fatalf("ListActions: %v", err)
	}
	if len(actions) != 1 || actions[0].ID != "test.echo" {
		t.Fatalf("actions = %#v", actions)
	}

	arguments, err := core.NewPayload([]byte(`{"b":2,"a":1}`))
	if err != nil {
		t.Fatalf("NewPayload: %v", err)
	}
	step, err := loaded.Ruleset.Start(ctx, core.StartRequest{
		Snapshot:  snapshot,
		Principal: principal,
		Intent: core.Intent{
			ID:        "intent-1",
			ActionID:  "test.echo",
			ActorID:   "actor-1",
			Arguments: arguments,
		},
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if step.Complete == nil || step.Complete.Result.String() != `{"echo":{"a":1,"b":2}}` {
		t.Fatalf("step = %#v, result = %s", step, step.Complete.Result.String())
	}

	pending := core.PendingStep{StepID: "step-1", Kind: core.StepKindNeedDecision, State: testPayload(t, map[string]any{})}
	response := core.HostResponse{StepID: "step-1", Kind: core.StepKindNeedDecision, Data: testPayload(t, map[string]any{"answer": "yes"})}
	resumed, err := loaded.Ruleset.Resume(ctx, core.ResumeRequest{
		Snapshot: snapshot, Principal: principal, Pending: pending, Response: response,
	})
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if resumed.Complete == nil || resumed.Complete.Outcome != "test.resumed" {
		t.Fatalf("resumed = %#v", resumed)
	}

	projection, err := loaded.Ruleset.Project(ctx, core.ProjectRequest{Snapshot: snapshot, Principal: principal})
	if err != nil || projection.View.String() != `{"counter":2}` {
		t.Fatalf("Project = %#v, %v", projection, err)
	}
	explanation, err := loaded.Ruleset.Explain(ctx, core.ExplainRequest{
		Snapshot: snapshot, Principal: principal, Reference: "test.echo", Locale: "en-US",
	})
	if err != nil || explanation.Text != "Explanation for test.echo" {
		t.Fatalf("Explain = %#v, %v", explanation, err)
	}
	if err := loaded.Ruleset.ValidateState(ctx, core.ValidateStateRequest{Snapshot: snapshot}); err != nil {
		t.Fatalf("ValidateState: %v", err)
	}
	event := core.Event{Type: "test.changed", SchemaVersion: 1, Data: testPayload(t, map[string]any{"value": 3})}
	reduced, err := loaded.Ruleset.Reduce(ctx, core.ReduceRequest{Snapshot: snapshot, Events: []core.Event{event}})
	if err != nil || reduced.State.String() != `{"counter":2}` {
		t.Fatalf("Reduce = %#v, %v", reduced, err)
	}
	migrated, err := loaded.Ruleset.Migrate(ctx, core.MigrateRequest{From: loaded.Artifact.Lock(), State: snapshot.State})
	if err != nil || migrated.State.String() != `{"counter":2}` {
		t.Fatalf("Migrate = %#v, %v", migrated, err)
	}
}

func TestArtifactDigestCoversExactBundleBytesAndProgramsAreCached(t *testing.T) {
	loader := newTestLoader(t, Limits{})
	manifest := testManifest()
	source := validSource(t, manifest)
	bundle := makeBundle(t, manifest, source)

	first, err := loader.Load(context.Background(), bytes.NewReader(bundle))
	if err != nil {
		t.Fatalf("first Load: %v", err)
	}
	digest := sha256.Sum256(bundle)
	want := "sha256:" + hex.EncodeToString(digest[:])
	if first.Artifact.Digest() != want {
		t.Fatalf("digest = %q, want %q", first.Artifact.Digest(), want)
	}
	second, err := loader.Load(context.Background(), bytes.NewReader(bundle))
	if err != nil {
		t.Fatalf("cached Load: %v", err)
	}
	if first.Ruleset != second.Ruleset || loader.CacheEntries() != 1 {
		t.Fatalf("cache did not reuse program: first=%p second=%p entries=%d", first.Ruleset, second.Ruleset, loader.CacheEntries())
	}

	changed := makeBundle(t, manifest, source+"\n# exact bytes changed\n")
	third, err := loader.Load(context.Background(), bytes.NewReader(changed))
	if err != nil {
		t.Fatalf("changed Load: %v", err)
	}
	if third.Artifact.Digest() == first.Artifact.Digest() || loader.CacheEntries() != 2 {
		t.Fatalf("changed bundle was not independently attested")
	}
}

func TestImportsAreBundleRootedAndFrozen(t *testing.T) {
	loader := newTestLoader(t, Limits{})
	manifest := testManifest()
	source := "load(\"lib.star\", \"VALUE\")\n" + validSource(t, manifest)
	bundle := makeBundle(t, manifest, source, archiveEntry{name: "lib.star", contents: `VALUE = "loaded"`})
	if _, err := loader.Load(context.Background(), bytes.NewReader(bundle)); err != nil {
		t.Fatalf("Load with confined module: %v", err)
	}
}

func TestManifestFunctionMustMatchBundleManifest(t *testing.T) {
	loader := newTestLoader(t, Limits{})
	embedded := testManifest()
	declared := embedded
	declared.Name = "Different Rules"
	bundle := makeBundle(t, embedded, validSource(t, declared))
	_, err := loader.Load(context.Background(), bytes.NewReader(bundle))
	if !errors.Is(err, core.ErrManifestMismatch) {
		t.Fatalf("Load error = %v, want ErrManifestMismatch", err)
	}
}

func TestBundleRejectsTraversalSymlinksAndEscapingLoads(t *testing.T) {
	manifest := testManifest()
	source := validSource(t, manifest)
	tests := []struct {
		name   string
		bundle []byte
	}{
		{
			name: "archive traversal",
			bundle: makeArchive(t, []archiveEntry{
				{name: ManifestPath, contents: manifestJSON(t, manifest)},
				{name: "main.star", contents: source},
				{name: "../outside.star", contents: "VALUE = 1"},
			}),
		},
		{
			name: "symlink",
			bundle: makeBundle(t, manifest, source,
				archiveEntry{name: "alias.star", contents: "main.star", mode: os.ModeSymlink | 0o777}),
		},
		{
			name:   "load traversal",
			bundle: makeBundle(t, manifest, "load(\"../outside.star\", \"VALUE\")\n"+source),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			loader := newTestLoader(t, Limits{})
			_, err := loader.Load(context.Background(), bytes.NewReader(test.bundle))
			if !errors.Is(err, ErrInvalidBundle) {
				t.Fatalf("Load error = %v, want ErrInvalidBundle", err)
			}
		})
	}
}

func TestSandboxHasNoHostClockRandomFilesystemOrNetworkNames(t *testing.T) {
	manifest := testManifest()
	for _, name := range []string{"os", "time", "random", "open", "http"} {
		t.Run(name, func(t *testing.T) {
			loader := newTestLoader(t, Limits{})
			source := validSource(t, manifest) + "\ndef forbidden():\n    return " + name + "\n"
			bundle := makeBundle(t, manifest, source)
			_, err := loader.Load(context.Background(), bytes.NewReader(bundle))
			if !errors.Is(err, ErrInvalidBundle) || !strings.Contains(err.Error(), "undefined") {
				t.Fatalf("Load error = %v, want undefined-name bundle error", err)
			}
		})
	}
}

func TestExecutionStepQuotaStopsProgram(t *testing.T) {
	limits := Limits{MaxExecutionSteps: 500}
	loader := newTestLoader(t, limits)
	manifest := testManifest()
	source := strings.Replace(validSource(t, manifest), `def list_actions(request):
    return [{
        "id": "test.echo",
        "label": "Echo",
        "description": "Return the supplied arguments",
        "input_schema": {"type": "object"},
        "tags": ["test"],
    }]`, `def list_actions(request):
    total = 0
    for number in range(1000000):
        total += number
    return []`, 1)
	loaded, err := loader.Load(context.Background(), bytes.NewReader(makeBundle(t, manifest, source)))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	_, err = loaded.Ruleset.ListActions(context.Background(), core.CatalogRequest{
		Snapshot: testSnapshot(t, loaded), Principal: testPrincipal(),
	})
	if !errors.Is(err, ErrExecutionLimit) {
		t.Fatalf("ListActions error = %v, want ErrExecutionLimit", err)
	}
}

func TestExecutionStepQuotaAlsoBoundsModuleInitialization(t *testing.T) {
	loader := newTestLoader(t, Limits{MaxExecutionSteps: 500})
	manifest := testManifest()
	source := "EXPENSIVE = [number for number in range(1000000)]\n" + validSource(t, manifest)
	_, err := loader.Load(context.Background(), bytes.NewReader(makeBundle(t, manifest, source)))
	if !errors.Is(err, ErrExecutionLimit) {
		t.Fatalf("Load error = %v, want ErrExecutionLimit", err)
	}
}

func TestInitialStateMustBeBoundedNonNullJSON(t *testing.T) {
	manifest := testManifest()
	tests := []struct {
		name        string
		replacement string
	}{
		{name: "null", replacement: "    return None"},
		{name: "collection", replacement: "    return [number for number in range(257)]"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			loader := newTestLoader(t, Limits{})
			source := strings.Replace(validSource(t, manifest), "    return {\"counter\": 0}", test.replacement, 1)
			_, err := loader.Load(context.Background(), bytes.NewReader(makeBundle(t, manifest, source)))
			if !errors.Is(err, ErrContract) {
				t.Fatalf("Load error = %v, want ErrContract", err)
			}
		})
	}
}

func TestContextCancellationInterruptsActiveProgram(t *testing.T) {
	loader := newTestLoader(t, Limits{})
	manifest := testManifest()
	source := strings.Replace(validSource(t, manifest), `def list_actions(request):
    return [{
        "id": "test.echo",
        "label": "Echo",
        "description": "Return the supplied arguments",
        "input_schema": {"type": "object"},
        "tags": ["test"],
    }]`, `def list_actions(request):
    total = 0
    for number in range(1000000000):
        total += number
    return []`, 1)
	loaded, err := loader.Load(context.Background(), bytes.NewReader(makeBundle(t, manifest, source)))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	timer := time.AfterFunc(50*time.Microsecond, cancel)
	defer timer.Stop()
	_, err = loaded.Ruleset.ListActions(ctx, core.CatalogRequest{
		Snapshot: testSnapshot(t, loaded), Principal: testPrincipal(),
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ListActions error = %v, want context cancellation", err)
	}
}

func TestJSONBoundaryIsDeterministicAcrossObjectMemberOrder(t *testing.T) {
	loaded := loadValid(t, newTestLoader(t, Limits{}))
	makeRequest := func(raw string) core.StartRequest {
		arguments, err := core.NewPayload([]byte(raw))
		if err != nil {
			t.Fatalf("NewPayload: %v", err)
		}
		return core.StartRequest{
			Snapshot: testSnapshot(t, loaded), Principal: testPrincipal(),
			Intent: core.Intent{ID: "intent-1", ActionID: "test.echo", Arguments: arguments},
		}
	}
	first, err := loaded.Ruleset.Start(context.Background(), makeRequest(`{"z":1,"a":2}`))
	if err != nil {
		t.Fatalf("first Start: %v", err)
	}
	second, err := loaded.Ruleset.Start(context.Background(), makeRequest(`{"a":2,"z":1}`))
	if err != nil {
		t.Fatalf("second Start: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("results differ:\nfirst:  %#v\nsecond: %#v", first, second)
	}
}

func TestJSONBoundaryRejectsOversizedInputAndOutputCollections(t *testing.T) {
	manifest := testManifest()
	t.Run("input", func(t *testing.T) {
		loaded := loadValid(t, newTestLoader(t, Limits{}))
		values := make([]int, core.MaxCollectionItems+1)
		request := core.StartRequest{
			Snapshot: testSnapshot(t, loaded), Principal: testPrincipal(),
			Intent: core.Intent{ID: "intent-1", ActionID: "test.echo", Arguments: testPayload(t, values)},
		}
		_, err := loaded.Ruleset.Start(context.Background(), request)
		if !errors.Is(err, ErrContract) {
			t.Fatalf("Start error = %v, want ErrContract", err)
		}
	})
	t.Run("output collection", func(t *testing.T) {
		loader := newTestLoader(t, Limits{})
		source := strings.Replace(validSource(t, manifest), `def list_actions(request):
    return [{
        "id": "test.echo",
        "label": "Echo",
        "description": "Return the supplied arguments",
        "input_schema": {"type": "object"},
        "tags": ["test"],
    }]`, `def list_actions(request):
    result = []
    for number in range(257):
        result.append({
            "id": "test.action_" + str(number),
            "label": "Action",
            "input_schema": {"type": "object"},
        })
    return result`, 1)
		loaded, err := loader.Load(context.Background(), bytes.NewReader(makeBundle(t, manifest, source)))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		_, err = loaded.Ruleset.ListActions(context.Background(), core.CatalogRequest{
			Snapshot: testSnapshot(t, loaded), Principal: testPrincipal(),
		})
		if !errors.Is(err, ErrContract) {
			t.Fatalf("ListActions error = %v, want ErrContract", err)
		}
	})
	t.Run("output bytes", func(t *testing.T) {
		loader := newTestLoader(t, Limits{MaxCallBytes: 1024})
		source := strings.Replace(validSource(t, manifest),
			`def project(request):
    return {"view": request["snapshot"]["state"]}`,
			`def project(request):
    return {"view": "x" * 2048}`, 1)
		loaded, err := loader.Load(context.Background(), bytes.NewReader(makeBundle(t, manifest, source)))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		_, err = loaded.Ruleset.Project(context.Background(), core.ProjectRequest{
			Snapshot: testSnapshot(t, loaded), Principal: testPrincipal(),
		})
		if !errors.Is(err, ErrContract) {
			t.Fatalf("Project error = %v, want ErrContract", err)
		}
	})
}

func TestLoaderRejectsBundleAndSourceLimits(t *testing.T) {
	manifest := testManifest()
	source := validSource(t, manifest)
	bundle := makeBundle(t, manifest, source)
	loader := newTestLoader(t, Limits{MaxBundleBytes: int64(len(bundle) - 1)})
	if _, err := loader.Load(context.Background(), bytes.NewReader(bundle)); !errors.Is(err, ErrBundleTooLarge) {
		t.Fatalf("compressed limit error = %v", err)
	}

	loader = newTestLoader(t, Limits{MaxSourceFileBytes: int64(len(source) - 1)})
	if _, err := loader.Load(context.Background(), bytes.NewReader(bundle)); !errors.Is(err, ErrBundleTooLarge) {
		t.Fatalf("source limit error = %v", err)
	}
}

func TestCustomLimitsCanOnlyReduceFailClosedDefaults(t *testing.T) {
	tests := []struct {
		name  string
		raise func(*Limits)
	}{
		{name: "bundle", raise: func(limits *Limits) { limits.MaxBundleBytes++ }},
		{name: "expanded", raise: func(limits *Limits) { limits.MaxExpandedBytes++ }},
		{name: "source", raise: func(limits *Limits) { limits.MaxSourceFileBytes++ }},
		{name: "files", raise: func(limits *Limits) { limits.MaxFiles++ }},
		{name: "steps", raise: func(limits *Limits) { limits.MaxExecutionSteps++ }},
		{name: "depth", raise: func(limits *Limits) { limits.MaxValueDepth++ }},
		{name: "nodes", raise: func(limits *Limits) { limits.MaxValueNodes++ }},
		{name: "collection", raise: func(limits *Limits) { limits.MaxCollectionItems++ }},
		{name: "call bytes", raise: func(limits *Limits) { limits.MaxCallBytes++ }},
		{name: "cache", raise: func(limits *Limits) { limits.MaxCachedBundles++ }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			limits := DefaultLimits()
			test.raise(&limits)
			if _, err := NewLoader(limits); !errors.Is(err, ErrInvalidBundle) {
				t.Fatalf("NewLoader error = %v, want fail-closed ErrInvalidBundle", err)
			}
		})
	}
}

func TestSnapshotMustUseExactLoadedArtifact(t *testing.T) {
	loaded := loadValid(t, newTestLoader(t, Limits{}))
	snapshot := testSnapshot(t, loaded)
	snapshot.Ruleset.Digest = "sha256:" + strings.Repeat("0", 64)
	_, err := loaded.Ruleset.ListActions(context.Background(), core.CatalogRequest{
		Snapshot: snapshot, Principal: testPrincipal(),
	})
	if err == nil || !strings.Contains(err.Error(), "snapshot lock") {
		t.Fatalf("ListActions error = %v", err)
	}
}

func TestLoaderAndFrozenRulesetAreConcurrent(t *testing.T) {
	loader := newTestLoader(t, Limits{})
	manifest := testManifest()
	bundle := makeBundle(t, manifest, validSource(t, manifest))
	const workers = 16
	errorsFound := make(chan error, workers)
	var wait sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			loaded, err := loader.Load(context.Background(), bytes.NewReader(bundle))
			if err != nil {
				errorsFound <- err
				return
			}
			arguments, err := core.PayloadFrom(map[string]any{"worker": worker})
			if err != nil {
				errorsFound <- err
				return
			}
			_, err = loaded.Ruleset.Start(context.Background(), core.StartRequest{
				Snapshot: core.Snapshot{
					Ruleset: loaded.Artifact.Lock(),
					State:   loaded.InitialState,
				},
				Principal: testPrincipal(),
				Intent: core.Intent{
					ID:        fmt.Sprintf("intent-%d", worker),
					ActionID:  "test.echo",
					Arguments: arguments,
				},
			})
			if err != nil {
				errorsFound <- err
			}
		}(worker)
	}
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Errorf("concurrent operation: %v", err)
	}
	if loader.CacheEntries() != 1 {
		t.Fatalf("cache entries = %d, want 1", loader.CacheEntries())
	}
}

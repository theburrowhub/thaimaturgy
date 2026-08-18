// Package bundlestore installs and discovers external rules bundles in a
// dedicated store. Adventure archives use a different import path and can never
// reach this package.
package bundlestore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/theburrowhub/thaimaturgy/internal/rules"
	"github.com/theburrowhub/thaimaturgy/internal/rules/catalog"
	"github.com/theburrowhub/thaimaturgy/internal/rules/starlarkruntime"
)

const (
	// DirectoryName is the data-directory child reserved for executable rules.
	// It is intentionally distinct from the adventures directory.
	DirectoryName = "rulesets"
	// BundleExtension identifies immutable Starlark rules ZIPs in the store.
	BundleExtension = ".rules.zip"
)

var ErrStoredArtifactMismatch = errors.New("rules bundle store: stored artifact does not match its identity path")

// InstalledBundle is a verified bundle and its canonical on-disk location.
type InstalledBundle struct {
	Loaded starlarkruntime.LoadedBundle
	Path   string
}

// Failure identifies one bundle that could not be discovered or registered.
// Discovery continues so one broken optional package does not hide healthy
// packages; a session pinned to the broken artifact remains unavailable.
type Failure struct {
	Path string
	Err  error
}

func (f Failure) Error() string { return f.Path + ": " + f.Err.Error() }
func (f Failure) Unwrap() error { return f.Err }

// Report is a deterministic discovery result. Err joins every failure.
type Report struct {
	Bundles  []InstalledBundle
	Failures []Failure
}

func (r Report) Err() error {
	joined := make([]error, len(r.Failures))
	for index := range r.Failures {
		joined[index] = r.Failures[index]
	}
	return errors.Join(joined...)
}

// Store owns one local directory and one bounded Starlark loader.
type Store struct {
	root   string
	loader *starlarkruntime.Loader
	mu     sync.Mutex
}

// New creates or opens a dedicated rules store. A nil loader selects the
// runtime's fail-closed production limits.
func New(root string, loader *starlarkruntime.Loader) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("rules bundle store: root is required")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("rules bundle store: resolve root: %w", err)
	}
	if info, err := os.Lstat(absolute); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return nil, errors.New("rules bundle store: root must be a real directory, not a symlink")
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("rules bundle store: inspect root: %w", err)
	} else if err := os.MkdirAll(absolute, 0o700); err != nil {
		return nil, fmt.Errorf("rules bundle store: create root: %w", err)
	}
	if loader == nil {
		loader, err = starlarkruntime.NewLoader(starlarkruntime.Limits{})
		if err != nil {
			return nil, err
		}
	}
	return &Store{root: absolute, loader: loader}, nil
}

func (s *Store) Root() string {
	if s == nil {
		return ""
	}
	return s.root
}

// InstallFile verifies a regular file before copying its exact bytes into the
// canonical content-addressed location.
func (s *Store) InstallFile(ctx context.Context, sourcePath string) (InstalledBundle, error) {
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return InstalledBundle{}, fmt.Errorf("rules bundle store: inspect source: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return InstalledBundle{}, errors.New("rules bundle store: source must be a regular file, not a symlink")
	}
	file, err := os.Open(sourcePath)
	if err != nil {
		return InstalledBundle{}, fmt.Errorf("rules bundle store: open source: %w", err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil || !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		if err == nil {
			err = errors.New("source changed while it was being opened")
		}
		return InstalledBundle{}, fmt.Errorf("rules bundle store: unsafe source: %w", err)
	}
	return s.Install(ctx, file)
}

// Install verifies the complete bundle before publishing it. Invalid input
// leaves no visible artifact in the store.
func (s *Store) Install(ctx context.Context, source io.Reader) (InstalledBundle, error) {
	if s == nil || s.loader == nil {
		return InstalledBundle{}, errors.New("rules bundle store: nil store")
	}
	if ctx == nil {
		return InstalledBundle{}, errors.New("rules bundle store: nil context")
	}
	if source == nil {
		return InstalledBundle{}, errors.New("rules bundle store: nil source")
	}
	maximum := starlarkruntime.DefaultLimits().MaxBundleBytes
	raw, err := io.ReadAll(io.LimitReader(source, maximum+1))
	if err != nil {
		return InstalledBundle{}, fmt.Errorf("rules bundle store: read source: %w", err)
	}
	if int64(len(raw)) > maximum {
		return InstalledBundle{}, fmt.Errorf("%w: compressed bytes exceed %d", starlarkruntime.ErrBundleTooLarge, maximum)
	}
	loaded, err := s.loader.Load(ctx, bytes.NewReader(raw))
	if err != nil {
		return InstalledBundle{}, err
	}
	lock := loaded.Artifact.Lock()
	initialSnapshot := rules.Snapshot{Ruleset: lock, State: loaded.InitialState}
	if err := loaded.Ruleset.ValidateState(ctx, rules.ValidateStateRequest{Snapshot: initialSnapshot}); err != nil {
		return InstalledBundle{}, fmt.Errorf("rules bundle store: ruleset rejected initial state: %w", err)
	}
	directory, err := ensurePackageDirectory(s.root, lock.ID, lock.Version)
	if err != nil {
		return InstalledBundle{}, err
	}
	destination := filepath.Join(directory, strings.TrimPrefix(lock.Digest, "sha256:")+BundleExtension)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ensureSingleReleaseArtifact(directory, destination, lock); err != nil {
		return InstalledBundle{}, err
	}
	if err := publishExact(destination, raw); err != nil {
		return InstalledBundle{}, err
	}
	return InstalledBundle{Loaded: loaded, Path: destination}, nil
}

// Discover loads every canonical bundle without following symlinks. Results
// and failures are ordered by path because filepath.WalkDir is lexical.
func (s *Store) Discover(ctx context.Context) Report {
	var report Report
	if s == nil || s.loader == nil {
		report.Failures = append(report.Failures, Failure{Path: "<store>", Err: errors.New("nil rules bundle store")})
		return report
	}
	if ctx == nil {
		report.Failures = append(report.Failures, Failure{Path: s.root, Err: errors.New("nil context")})
		return report
	}
	walkErr := filepath.WalkDir(s.root, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			report.Failures = append(report.Failures, Failure{Path: filePath, Err: walkErr})
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if filePath == s.root {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			report.Failures = append(report.Failures, Failure{Path: filePath, Err: errors.New("symlinks are forbidden in the rules store")})
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), BundleExtension) {
			return nil
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			if err == nil {
				err = errors.New("bundle entry is not a regular file")
			}
			report.Failures = append(report.Failures, Failure{Path: filePath, Err: err})
			return nil
		}
		file, err := os.Open(filePath)
		if err != nil {
			report.Failures = append(report.Failures, Failure{Path: filePath, Err: err})
			return nil
		}
		loaded, loadErr := s.loader.Load(ctx, file)
		closeErr := file.Close()
		if loadErr != nil {
			report.Failures = append(report.Failures, Failure{Path: filePath, Err: loadErr})
			return nil
		}
		if closeErr != nil {
			report.Failures = append(report.Failures, Failure{Path: filePath, Err: closeErr})
			return nil
		}
		expected := filepath.Join(s.root, canonicalRelativePath(loaded.Artifact.Lock()))
		if filepath.Clean(filePath) != expected {
			report.Failures = append(report.Failures, Failure{Path: filePath, Err: ErrStoredArtifactMismatch})
			return nil
		}
		report.Bundles = append(report.Bundles, InstalledBundle{Loaded: loaded, Path: filePath})
		return nil
	})
	if walkErr != nil {
		report.Failures = append(report.Failures, Failure{Path: s.root, Err: walkErr})
	}
	report.rejectReleaseEquivocation()
	return report
}

func (r *Report) rejectReleaseEquivocation() {
	type release struct{ id, version string }
	counts := make(map[release]int)
	for _, bundle := range r.Bundles {
		lock := bundle.Loaded.Artifact.Lock()
		counts[release{id: lock.ID, version: lock.Version}]++
	}
	rejected := make(map[int]struct{})
	for index, bundle := range r.Bundles {
		lock := bundle.Loaded.Artifact.Lock()
		key := release{id: lock.ID, version: lock.Version}
		if counts[key] < 2 {
			continue
		}
		rejected[index] = struct{}{}
		r.Failures = append(r.Failures, Failure{
			Path: bundle.Path,
			Err:  fmt.Errorf("%w: %s@%s has multiple digests in the store", rules.ErrArtifactConflict, key.id, key.version),
		})
	}
	if len(rejected) == 0 {
		return
	}
	accepted := r.Bundles[:0]
	for index, bundle := range r.Bundles {
		if _, exists := rejected[index]; !exists {
			accepted = append(accepted, bundle)
		}
	}
	r.Bundles = accepted
}

// RegisterAll discovers and registers every healthy external bundle. It keeps
// successful registrations even when another bundle fails and returns their
// exact locks plus a joined diagnostic.
func (s *Store) RegisterAll(ctx context.Context, destination *catalog.Catalog) ([]rules.Lock, error) {
	if destination == nil {
		return nil, errors.New("rules bundle store: nil catalog")
	}
	report := s.Discover(ctx)
	locks := make([]rules.Lock, 0, len(report.Bundles))
	failures := append([]Failure(nil), report.Failures...)
	for _, bundle := range report.Bundles {
		loaded := bundle.Loaded
		if err := destination.Register(ctx, loaded.Artifact, loaded.Ruleset, loaded.InitialState); err != nil {
			failures = append(failures, Failure{Path: bundle.Path, Err: err})
			continue
		}
		locks = append(locks, loaded.Artifact.Lock())
	}
	report.Failures = failures
	return locks, report.Err()
}

func canonicalRelativePath(lock rules.Lock) string {
	digest := strings.TrimPrefix(lock.Digest, "sha256:")
	return filepath.Join(lock.ID, lock.Version, digest+BundleExtension)
}

func ensurePackageDirectory(root, id, version string) (string, error) {
	current := root
	for _, segment := range []string{id, version} {
		current = filepath.Join(current, segment)
		if err := os.Mkdir(current, 0o700); err != nil && !os.IsExist(err) {
			return "", fmt.Errorf("rules bundle store: create package directory: %w", err)
		}
		info, err := os.Lstat(current)
		if err != nil {
			return "", fmt.Errorf("rules bundle store: inspect package directory: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", errors.New("rules bundle store: package path must contain only real directories")
		}
	}
	return current, nil
}

func ensureSingleReleaseArtifact(directory, destination string, lock rules.Lock) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("rules bundle store: inspect release directory: %w", err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), BundleExtension) {
			continue
		}
		candidate := filepath.Join(directory, entry.Name())
		if candidate != destination {
			return fmt.Errorf("%w: %s@%s is already installed with another digest", rules.ErrArtifactConflict, lock.ID, lock.Version)
		}
	}
	return nil
}

func publishExact(destination string, raw []byte) error {
	directory := filepath.Dir(destination)
	if info, err := os.Lstat(destination); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return ErrStoredArtifactMismatch
		}
		if info.Size() != int64(len(raw)) {
			return ErrStoredArtifactMismatch
		}
		existing, err := os.ReadFile(destination)
		if err != nil {
			return fmt.Errorf("rules bundle store: read existing artifact: %w", err)
		}
		if !bytes.Equal(existing, raw) {
			return ErrStoredArtifactMismatch
		}
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("rules bundle store: inspect destination: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".install-*")
	if err != nil {
		return fmt.Errorf("rules bundle store: create temporary artifact: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(raw); err != nil {
		temporary.Close()
		return fmt.Errorf("rules bundle store: write temporary artifact: %w", err)
	}
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return fmt.Errorf("rules bundle store: publish artifact: %w", err)
	}
	if directoryHandle, err := os.Open(directory); err == nil {
		_ = directoryHandle.Sync()
		_ = directoryHandle.Close()
	}
	return nil
}

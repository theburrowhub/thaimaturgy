// Package bundlepack creates deterministic Starlark rules bundles from trusted
// source directories. It never follows symlinks and validates the completed
// archive with the production loader before publishing it.
package bundlepack

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/rules"
	"github.com/theburrowhub/thaimaturgy/internal/rules/starlarkruntime"
)

const (
	// BundleExtension is the filename suffix emitted by the authoring command.
	BundleExtension = ".rules.zip"

	outputFileMode  = 0o600
	archiveFileMode = 0o600
)

var (
	// ErrInvalidSource indicates that a source tree cannot be represented as a
	// confined rules bundle.
	ErrInvalidSource = errors.New("rules bundle pack: invalid source")
	// ErrDestinationConflict preserves an existing output whose bytes differ
	// from the bundle being packed.
	ErrDestinationConflict = errors.New("rules bundle pack: destination already contains different bytes")
)

// Result describes the loader-attested bundle that was published.
type Result struct {
	Loaded starlarkruntime.LoadedBundle
	Path   string
}

type sourceFile struct {
	name     string
	contents []byte
}

var deterministicZIPTime = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)

// Pack snapshots sourceDirectory, creates a byte-for-byte deterministic ZIP,
// validates its executable contract and initial state, and atomically publishes
// it at outputPath. A nil loader selects the production limits.
//
// Existing output is accepted only when it already contains the exact bytes.
// A different existing file is never overwritten.
func Pack(ctx context.Context, sourceDirectory, outputPath string, loader *starlarkruntime.Loader) (Result, error) {
	if ctx == nil {
		return Result{}, errors.New("rules bundle pack: nil context")
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if !strings.HasSuffix(outputPath, BundleExtension) {
		return Result{}, fmt.Errorf("rules bundle pack: output must end in %s", BundleExtension)
	}

	sourceRoot, err := checkedSourceRoot(sourceDirectory)
	if err != nil {
		return Result{}, err
	}
	destination, err := filepath.Abs(outputPath)
	if err != nil {
		return Result{}, fmt.Errorf("rules bundle pack: resolve output: %w", err)
	}
	resolvedSourceRoot, err := filepath.EvalSymlinks(sourceRoot)
	if err != nil {
		return Result{}, fmt.Errorf("%w: resolve source directory: %v", ErrInvalidSource, err)
	}
	resolvedDestination, err := resolveOutputLocation(destination)
	if err != nil {
		return Result{}, err
	}
	if pathWithin(sourceRoot, destination) || pathWithin(resolvedSourceRoot, resolvedDestination) {
		return Result{}, fmt.Errorf("%w: output must be outside the source directory", ErrInvalidSource)
	}

	files, err := snapshotSource(ctx, sourceRoot)
	if err != nil {
		return Result{}, err
	}
	if loader == nil {
		loader, err = starlarkruntime.NewLoader(starlarkruntime.Limits{})
		if err != nil {
			return Result{}, err
		}
	}

	if err := ensureOutputDirectory(destination); err != nil {
		return Result{}, err
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".rules-pack-*")
	if err != nil {
		return Result{}, fmt.Errorf("rules bundle pack: create temporary output: %w", err)
	}
	temporaryPath := temporary.Name()
	temporaryOpen := true
	defer func() {
		if temporaryOpen {
			_ = temporary.Close()
		}
		_ = os.Remove(temporaryPath)
	}()

	if err := writeArchive(ctx, temporary, files); err != nil {
		return Result{}, err
	}
	if err := temporary.Chmod(outputFileMode); err != nil {
		return Result{}, fmt.Errorf("rules bundle pack: secure temporary output: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return Result{}, fmt.Errorf("rules bundle pack: sync temporary output: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return Result{}, fmt.Errorf("rules bundle pack: close temporary output: %w", err)
	}
	temporaryOpen = false

	loaded, err := validateTemporary(ctx, temporaryPath, loader)
	if err != nil {
		return Result{}, err
	}
	publishLock, err := acquireOutputLock(ctx, destination)
	if err != nil {
		return Result{}, err
	}
	defer publishLock.Close()
	identical, err := destinationMatches(destination, temporaryPath)
	if err != nil {
		return Result{}, err
	}
	if identical {
		return Result{Loaded: loaded, Path: destination}, nil
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return Result{}, fmt.Errorf("rules bundle pack: publish output: %w", err)
	}
	syncDirectory(filepath.Dir(destination))
	return Result{Loaded: loaded, Path: destination}, nil
}

func checkedSourceRoot(sourceDirectory string) (string, error) {
	if strings.TrimSpace(sourceDirectory) == "" {
		return "", fmt.Errorf("%w: source directory is required", ErrInvalidSource)
	}
	absolute, err := filepath.Abs(sourceDirectory)
	if err != nil {
		return "", fmt.Errorf("%w: resolve source directory: %v", ErrInvalidSource, err)
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		return "", fmt.Errorf("%w: inspect source directory: %v", ErrInvalidSource, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", fmt.Errorf("%w: source must be a real directory, not a symlink", ErrInvalidSource)
	}
	return absolute, nil
}

func pathWithin(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// resolveOutputLocation maps a not-yet-created destination through the nearest
// existing directory. This closes the lexical-path gap where an ancestor
// symlink could otherwise place output inside the source tree.
func resolveOutputLocation(destination string) (string, error) {
	current := filepath.Dir(destination)
	missing := make([]string, 0)
	for {
		info, err := os.Lstat(current)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
				return "", errors.New("rules bundle pack: output path must descend from a real directory, not a symlink")
			}
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", fmt.Errorf("rules bundle pack: resolve output directory: %w", err)
			}
			for index := len(missing) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, missing[index])
			}
			return filepath.Join(resolved, filepath.Base(destination)), nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("rules bundle pack: inspect output directory: %w", err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", errors.New("rules bundle pack: no usable output directory")
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

func snapshotSource(ctx context.Context, root string) ([]sourceFile, error) {
	limits := starlarkruntime.DefaultLimits()
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("%w: resolve source directory: %v", ErrInvalidSource, err)
	}
	files := make([]sourceFile, 0)
	var expanded int64
	entryCount := 0
	err = filepath.WalkDir(root, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("%w: walk %s: %v", ErrInvalidSource, filePath, walkErr)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if filePath == root {
			return nil
		}
		entryCount++
		if entryCount > limits.MaxFiles {
			return fmt.Errorf("%w: more than %d source entries", starlarkruntime.ErrBundleTooLarge, limits.MaxFiles)
		}
		relative, err := filepath.Rel(root, filePath)
		if err != nil {
			return fmt.Errorf("%w: resolve %s: %v", ErrInvalidSource, filePath, err)
		}
		name := filepath.ToSlash(relative)
		if err := starlarkruntime.ValidateBundlePath(name); err != nil {
			return fmt.Errorf("%w: path %q: %v", ErrInvalidSource, name, err)
		}
		resolvedPath, err := filepath.EvalSymlinks(filePath)
		if err != nil {
			return fmt.Errorf("%w: resolve %q: %v", ErrInvalidSource, name, err)
		}
		if !pathWithin(resolvedRoot, resolvedPath) {
			return fmt.Errorf("%w: path %q resolves outside the source directory", ErrInvalidSource, name)
		}
		info, err := os.Lstat(filePath)
		if err != nil {
			return fmt.Errorf("%w: inspect %q: %v", ErrInvalidSource, name, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: symlink %q is forbidden", ErrInvalidSource, name)
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%w: non-regular file %q is forbidden", ErrInvalidSource, name)
		}
		if info.Size() < 0 || info.Size() > limits.MaxExpandedBytes-expanded {
			return fmt.Errorf("%w: expanded bytes exceed %d", starlarkruntime.ErrBundleTooLarge, limits.MaxExpandedBytes)
		}
		if filepath.Ext(name) == ".star" && info.Size() > limits.MaxSourceFileBytes {
			return fmt.Errorf("%w: source %q exceeds %d bytes", starlarkruntime.ErrBundleTooLarge, name, limits.MaxSourceFileBytes)
		}
		contents, err := readUnchangedRegularFile(filePath, info)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrInvalidSource, err)
		}
		expanded += int64(len(contents))
		files = append(files, sourceFile{name: name, contents: contents})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(left, right int) bool { return files[left].name < files[right].name })
	return files, nil
}

func readUnchangedRegularFile(filePath string, before os.FileInfo) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open %q: %v", filePath, err)
	}
	defer file.Close()
	afterOpen, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect opened %q: %v", filePath, err)
	}
	if !afterOpen.Mode().IsRegular() || !os.SameFile(before, afterOpen) || before.Size() != afterOpen.Size() {
		return nil, fmt.Errorf("source file %q changed while it was opened", filePath)
	}
	contents, err := io.ReadAll(io.LimitReader(file, before.Size()+1))
	if err != nil {
		return nil, fmt.Errorf("read %q: %v", filePath, err)
	}
	if int64(len(contents)) != before.Size() {
		return nil, fmt.Errorf("source file %q changed while it was read", filePath)
	}
	afterRead, err := file.Stat()
	if err != nil || !os.SameFile(before, afterRead) || afterRead.Size() != before.Size() {
		if err != nil {
			return nil, fmt.Errorf("inspect read %q: %v", filePath, err)
		}
		return nil, fmt.Errorf("source file %q changed while it was read", filePath)
	}
	return contents, nil
}

func ensureOutputDirectory(destination string) error {
	directory := filepath.Dir(destination)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("rules bundle pack: create output directory: %w", err)
	}
	info, err := os.Lstat(directory)
	if err != nil {
		return fmt.Errorf("rules bundle pack: inspect output directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("rules bundle pack: output directory must be a real directory, not a symlink")
	}
	return nil
}

func writeArchive(ctx context.Context, destination io.Writer, files []sourceFile) error {
	archive := zip.NewWriter(destination)
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			_ = archive.Close()
			return err
		}
		header := &zip.FileHeader{Name: file.name, Method: zip.Store}
		header.SetMode(archiveFileMode)
		header.SetModTime(deterministicZIPTime)
		entry, err := archive.CreateHeader(header)
		if err != nil {
			_ = archive.Close()
			return fmt.Errorf("rules bundle pack: create ZIP entry %q: %w", file.name, err)
		}
		if _, err := entry.Write(file.contents); err != nil {
			_ = archive.Close()
			return fmt.Errorf("rules bundle pack: write ZIP entry %q: %w", file.name, err)
		}
	}
	if err := archive.Close(); err != nil {
		return fmt.Errorf("rules bundle pack: close ZIP: %w", err)
	}
	return nil
}

func validateTemporary(ctx context.Context, temporaryPath string, loader *starlarkruntime.Loader) (starlarkruntime.LoadedBundle, error) {
	file, err := os.Open(temporaryPath)
	if err != nil {
		return starlarkruntime.LoadedBundle{}, fmt.Errorf("rules bundle pack: reopen temporary output: %w", err)
	}
	loaded, loadErr := loader.Load(ctx, file)
	closeErr := file.Close()
	if loadErr != nil {
		return starlarkruntime.LoadedBundle{}, fmt.Errorf("rules bundle pack: validate output: %w", loadErr)
	}
	if closeErr != nil {
		return starlarkruntime.LoadedBundle{}, fmt.Errorf("rules bundle pack: close validated output: %w", closeErr)
	}
	snapshot := rules.Snapshot{Ruleset: loaded.Artifact.Lock(), State: loaded.InitialState}
	if err := loaded.Ruleset.ValidateState(ctx, rules.ValidateStateRequest{Snapshot: snapshot}); err != nil {
		return starlarkruntime.LoadedBundle{}, fmt.Errorf("rules bundle pack: ruleset rejected initial state: %w", err)
	}
	return loaded, nil
}

func destinationMatches(destination, temporaryPath string) (bool, error) {
	destinationInfo, err := os.Lstat(destination)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("rules bundle pack: inspect destination: %w", err)
	}
	if destinationInfo.Mode()&os.ModeSymlink != 0 || !destinationInfo.Mode().IsRegular() {
		return false, errors.New("rules bundle pack: destination must be a regular file, not a symlink")
	}
	temporaryInfo, err := os.Stat(temporaryPath)
	if err != nil {
		return false, fmt.Errorf("rules bundle pack: inspect temporary output: %w", err)
	}
	if destinationInfo.Size() != temporaryInfo.Size() {
		return false, ErrDestinationConflict
	}
	destinationBytes, err := os.ReadFile(destination)
	if err != nil {
		return false, fmt.Errorf("rules bundle pack: read destination: %w", err)
	}
	temporaryBytes, err := os.ReadFile(temporaryPath)
	if err != nil {
		return false, fmt.Errorf("rules bundle pack: read temporary output: %w", err)
	}
	if !bytes.Equal(destinationBytes, temporaryBytes) {
		return false, ErrDestinationConflict
	}
	return true, nil
}

func syncDirectory(directory string) {
	handle, err := os.Open(directory)
	if err != nil {
		return
	}
	_ = handle.Sync()
	_ = handle.Close()
}

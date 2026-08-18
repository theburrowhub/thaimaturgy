package starlarkruntime

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/theburrowhub/thaimaturgy/internal/jsonstrict"
	core "github.com/theburrowhub/thaimaturgy/internal/rules"
	star "go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// LoadedBundle is an exact host-attested artifact and its executable adapter.
type LoadedBundle struct {
	Artifact     core.Artifact
	Ruleset      *Ruleset
	InitialState core.Payload
}

type cachedBundle struct {
	loaded LoadedBundle
}

// Loader validates and compiles immutable Starlark ZIP bundles. Compiled
// programs and frozen module globals are cached by the digest of the exact ZIP
// bytes. A Loader is safe for concurrent use.
type Loader struct {
	limits Limits

	mu        sync.Mutex
	cache     map[string]cachedBundle
	cacheFIFO []string
}

// NewLoader creates a loader with bounded resource limits. Passing Limits{}
// selects DefaultLimits.
func NewLoader(limits Limits) (*Loader, error) {
	normalized, err := normalizeLimits(limits)
	if err != nil {
		return nil, err
	}
	return &Loader{
		limits: normalized,
		cache:  make(map[string]cachedBundle),
	}, nil
}

// Load consumes one ZIP bundle, verifies its embedded manifest against its
// Starlark manifest() function, and returns an adapter bound to the SHA-256 of
// the exact bytes consumed.
func (l *Loader) Load(ctx context.Context, bundle io.Reader) (LoadedBundle, error) {
	if l == nil {
		return LoadedBundle{}, fmt.Errorf("%w: nil loader", ErrInvalidBundle)
	}
	if ctx == nil {
		return LoadedBundle{}, fmt.Errorf("%w: nil context", ErrInvalidBundle)
	}
	if bundle == nil {
		return LoadedBundle{}, fmt.Errorf("%w: nil reader", ErrInvalidBundle)
	}
	if err := ctx.Err(); err != nil {
		return LoadedBundle{}, err
	}

	raw, err := readBounded(bundle, l.limits.MaxBundleBytes)
	if err != nil {
		return LoadedBundle{}, err
	}
	digest := digestBytes(raw)
	if cached, ok := l.cached(digest); ok {
		if err := ctx.Err(); err != nil {
			return LoadedBundle{}, err
		}
		return cached.loaded, nil
	}

	files, err := l.readArchive(raw)
	if err != nil {
		return LoadedBundle{}, err
	}
	manifestRaw, ok := files[ManifestPath]
	if !ok {
		return LoadedBundle{}, fmt.Errorf("%w: missing %s", ErrInvalidBundle, ManifestPath)
	}
	var manifest core.Manifest
	if err := jsonstrict.Decode(manifestRaw, &manifest); err != nil {
		return LoadedBundle{}, fmt.Errorf("%w: decode %s: %v", ErrInvalidBundle, ManifestPath, err)
	}
	if err := manifest.Validate(); err != nil {
		return LoadedBundle{}, fmt.Errorf("%w: %v", ErrInvalidBundle, err)
	}
	if manifest.Runtime.Kind != core.RuntimeStarlark {
		return LoadedBundle{}, fmt.Errorf("%w: runtime kind is %q, want %q", ErrInvalidBundle, manifest.Runtime.Kind, core.RuntimeStarlark)
	}
	if path.Ext(manifest.Runtime.Entrypoint) != ".star" {
		return LoadedBundle{}, fmt.Errorf("%w: entrypoint must have .star extension", ErrInvalidBundle)
	}
	if manifest.ProtocolVersion != core.ProtocolVersion {
		return LoadedBundle{}, fmt.Errorf("%w: package uses %s, host uses %s", core.ErrIncompatibleProtocol, manifest.ProtocolVersion, core.ProtocolVersion)
	}

	artifact, err := core.NewArtifact(manifest, bytes.NewReader(raw))
	if err != nil {
		return LoadedBundle{}, fmt.Errorf("%w: attest artifact: %v", ErrInvalidBundle, err)
	}
	if artifact.Digest() != digest {
		return LoadedBundle{}, fmt.Errorf("%w: inconsistent artifact digest", ErrInvalidBundle)
	}

	programs, err := l.compilePrograms(files, manifest.Runtime.Entrypoint)
	if err != nil {
		return LoadedBundle{}, err
	}
	globals, err := initializePrograms(ctx, programs, manifest.Runtime.Entrypoint, l.limits)
	if err != nil {
		return LoadedBundle{}, err
	}
	ruleset, err := newRuleset(ctx, manifest, artifact.Lock(), globals, l.limits)
	if err != nil {
		return LoadedBundle{}, err
	}

	loaded := LoadedBundle{Artifact: artifact, Ruleset: ruleset, InitialState: ruleset.InitialState()}
	return l.storeCached(digest, loaded), nil
}

// CacheEntries reports the current number of compiled bundle entries.
func (l *Loader) CacheEntries() int {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.cache)
}

func (l *Loader) cached(digest string) (cachedBundle, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	cached, ok := l.cache[digest]
	return cached, ok
}

func (l *Loader) storeCached(digest string, loaded LoadedBundle) LoadedBundle {
	l.mu.Lock()
	defer l.mu.Unlock()
	if existing, ok := l.cache[digest]; ok {
		return existing.loaded
	}
	for len(l.cacheFIFO) >= l.limits.MaxCachedBundles {
		oldest := l.cacheFIFO[0]
		l.cacheFIFO = l.cacheFIFO[1:]
		delete(l.cache, oldest)
	}
	l.cache[digest] = cachedBundle{loaded: loaded}
	l.cacheFIFO = append(l.cacheFIFO, digest)
	return loaded
}

func readBounded(reader io.Reader, maximum int64) ([]byte, error) {
	limited := io.LimitReader(reader, maximum+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("%w: read bundle: %v", ErrInvalidBundle, err)
	}
	if int64(len(raw)) > maximum {
		return nil, fmt.Errorf("%w: compressed bytes exceed %d", ErrBundleTooLarge, maximum)
	}
	return raw, nil
}

func digestBytes(raw []byte) string {
	digest := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func (l *Loader) readArchive(raw []byte) (map[string][]byte, error) {
	archive, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, fmt.Errorf("%w: open ZIP: %v", ErrInvalidBundle, err)
	}
	files := make(map[string][]byte)
	seen := make(map[string]struct{})
	var expanded int64
	entryCount := 0
	for _, item := range archive.File {
		entryCount++
		if entryCount > l.limits.MaxFiles {
			return nil, fmt.Errorf("%w: more than %d ZIP entries", ErrBundleTooLarge, l.limits.MaxFiles)
		}
		name := item.Name
		isDirectory := item.FileInfo().IsDir() || strings.HasSuffix(name, "/")
		cleanName := strings.TrimSuffix(name, "/")
		if err := validateBundlePath(cleanName); err != nil {
			return nil, fmt.Errorf("%w: ZIP entry %q: %v", ErrInvalidBundle, name, err)
		}
		if _, exists := seen[cleanName]; exists {
			return nil, fmt.Errorf("%w: duplicate ZIP entry %q", ErrInvalidBundle, name)
		}
		seen[cleanName] = struct{}{}
		mode := item.Mode()
		if mode&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("%w: symlink entry %q is forbidden", ErrInvalidBundle, name)
		}
		if isDirectory {
			if item.UncompressedSize64 != 0 {
				return nil, fmt.Errorf("%w: malformed directory entry %q", ErrInvalidBundle, name)
			}
			continue
		}
		if !mode.IsRegular() {
			return nil, fmt.Errorf("%w: non-regular entry %q is forbidden", ErrInvalidBundle, name)
		}
		if item.UncompressedSize64 > uint64(l.limits.MaxExpandedBytes-expanded) {
			return nil, fmt.Errorf("%w: expanded bytes exceed %d", ErrBundleTooLarge, l.limits.MaxExpandedBytes)
		}
		if path.Ext(name) == ".star" && item.UncompressedSize64 > uint64(l.limits.MaxSourceFileBytes) {
			return nil, fmt.Errorf("%w: source %q exceeds %d bytes", ErrBundleTooLarge, name, l.limits.MaxSourceFileBytes)
		}
		contents, err := readZIPFile(item, l.limits.MaxExpandedBytes-expanded)
		if err != nil {
			return nil, err
		}
		expanded += int64(len(contents))
		if name == ManifestPath && len(contents) > core.MaxPayloadBytes {
			return nil, fmt.Errorf("%w: %s exceeds %d bytes", ErrBundleTooLarge, ManifestPath, core.MaxPayloadBytes)
		}
		if path.Ext(name) == ".star" && !utf8.Valid(contents) {
			return nil, fmt.Errorf("%w: source %q is not valid UTF-8", ErrInvalidBundle, name)
		}
		files[name] = contents
	}
	return files, nil
}

func readZIPFile(file *zip.File, maximum int64) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("%w: open ZIP entry %q: %v", ErrInvalidBundle, file.Name, err)
	}
	contents, readErr := io.ReadAll(io.LimitReader(reader, maximum+1))
	closeErr := reader.Close()
	if readErr != nil {
		return nil, fmt.Errorf("%w: read ZIP entry %q: %v", ErrInvalidBundle, file.Name, readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("%w: close ZIP entry %q: %v", ErrInvalidBundle, file.Name, closeErr)
	}
	if int64(len(contents)) > maximum {
		return nil, fmt.Errorf("%w: expanded bytes exceed limit", ErrBundleTooLarge)
	}
	if uint64(len(contents)) != file.UncompressedSize64 {
		return nil, fmt.Errorf("%w: ZIP entry %q size mismatch", ErrInvalidBundle, file.Name)
	}
	return contents, nil
}

func validateBundlePath(name string) error {
	if name == "" {
		return errors.New("path must not be empty")
	}
	if len(name) > core.MaxTextBytes {
		return fmt.Errorf("path exceeds %d bytes", core.MaxTextBytes)
	}
	if !utf8.ValidString(name) {
		return errors.New("path is not valid UTF-8")
	}
	if strings.Contains(name, "\\") || path.IsAbs(name) || path.Clean(name) != name {
		return errors.New("path must be clean, relative, and slash-separated")
	}
	for _, segment := range strings.Split(name, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return errors.New("path contains an empty, dot, or parent segment")
		}
		for index := 0; index < len(segment); index++ {
			character := segment[index]
			if !isPortablePathCharacter(character) {
				return fmt.Errorf("path contains unsupported character %q", character)
			}
		}
	}
	return nil
}

func isPortablePathCharacter(character byte) bool {
	return character >= 'a' && character <= 'z' ||
		character >= 'A' && character <= 'Z' ||
		character >= '0' && character <= '9' ||
		character == '.' || character == '-' || character == '_'
}

func (l *Loader) compilePrograms(files map[string][]byte, entrypoint string) (map[string]*star.Program, error) {
	if _, ok := files[entrypoint]; !ok {
		return nil, fmt.Errorf("%w: entrypoint %q does not exist", ErrInvalidBundle, entrypoint)
	}
	programs := make(map[string]*star.Program)
	options := &syntax.FileOptions{}
	noPredeclared := func(string) bool { return false }
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		source := files[name]
		if path.Ext(name) != ".star" {
			continue
		}
		_, program, err := star.SourceProgramOptions(options, name, source, noPredeclared)
		if err != nil {
			return nil, fmt.Errorf("%w: compile %q: %v", ErrInvalidBundle, name, err)
		}
		programs[name] = program
	}
	programNames := make([]string, 0, len(programs))
	for name := range programs {
		programNames = append(programNames, name)
	}
	sort.Strings(programNames)
	for _, name := range programNames {
		program := programs[name]
		for index := 0; index < program.NumLoads(); index++ {
			module, _ := program.Load(index)
			if err := validateBundlePath(module); err != nil {
				return nil, fmt.Errorf("%w: module %q loaded by %q: %v", ErrInvalidBundle, module, name, err)
			}
			if path.Ext(module) != ".star" {
				return nil, fmt.Errorf("%w: module %q loaded by %q must have .star extension", ErrInvalidBundle, module, name)
			}
			if _, ok := programs[module]; !ok {
				return nil, fmt.Errorf("%w: module %q loaded by %q does not exist", ErrInvalidBundle, module, name)
			}
		}
	}
	if err := rejectLoadCycles(programs); err != nil {
		return nil, err
	}
	return programs, nil
}

func rejectLoadCycles(programs map[string]*star.Program) error {
	const (
		unvisited = iota
		visiting
		visited
	)
	states := make(map[string]int, len(programs))
	var visit func(string) error
	visit = func(name string) error {
		switch states[name] {
		case visiting:
			return fmt.Errorf("%w: load cycle includes %q", ErrInvalidBundle, name)
		case visited:
			return nil
		}
		states[name] = visiting
		program := programs[name]
		for index := 0; index < program.NumLoads(); index++ {
			module, _ := program.Load(index)
			if err := visit(module); err != nil {
				return err
			}
		}
		states[name] = visited
		return nil
	}
	names := make([]string, 0, len(programs))
	for name := range programs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := visit(name); err != nil {
			return err
		}
	}
	return nil
}

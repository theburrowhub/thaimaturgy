package storage

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/ingest"
)

const (
	// AdventuresDir holds extracted adventure modules under the app dir.
	AdventuresDir = "adventures"
	// AdventureFile is the canonical JSON entry inside a module.
	AdventureFile = "adventure.json"

	// maxDecompressedFileSize caps a single extracted file to guard against
	// decompression bombs (100 MiB).
	maxDecompressedFileSize = 100 << 20
)

// AdventuresPath returns the directory that stores extracted modules.
func (s *Storage) AdventuresPath() string {
	return filepath.Join(s.basePath, AdventuresDir)
}

// AdventureDir returns the on-disk directory for an imported adventure ID.
func (s *Storage) AdventureDir(id string) string {
	return filepath.Join(s.AdventuresPath(), sanitizeID(id))
}

// AdventureInfo is a lightweight summary of an imported adventure for listing.
type AdventureInfo struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Author string `json:"author"`
	System string `json:"system"`
	Dir    string `json:"dir"`
}

// ImportModule extracts a .tar.gz adventure module, validates its
// adventure.json, and stores it under AdventuresPath()/<id>/. It returns the
// parsed, validated Adventure. Extraction is hardened against path-traversal
// (zip-slip) and oversized entries.
func (s *Storage) ImportModule(srcPath string) (*domain.Adventure, error) {
	if err := os.MkdirAll(s.AdventuresPath(), 0755); err != nil {
		return nil, fmt.Errorf("failed to create adventures directory: %w", err)
	}

	staging, err := os.MkdirTemp(s.AdventuresPath(), ".staging-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create staging directory: %w", err)
	}
	cleanupStaging := true
	defer func() {
		if cleanupStaging {
			_ = os.RemoveAll(staging)
		}
	}()

	if err := extractTarGz(srcPath, staging); err != nil {
		return nil, err
	}

	advPath := filepath.Join(staging, AdventureFile)
	data, err := os.ReadFile(advPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("module is missing %s at its root", AdventureFile)
		}
		return nil, fmt.Errorf("failed to read %s: %w", AdventureFile, err)
	}

	var adv domain.Adventure
	if err := json.Unmarshal(data, &adv); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", AdventureFile, err)
	}
	if strings.TrimSpace(adv.ID) == "" {
		return nil, fmt.Errorf("adventure.json is missing required field 'id'")
	}

	// Normalize images (transcode TIFF→PNG incl. CMYK, downscale huge scans, and
	// drop near-blank paper/texture layers), rewriting adv's references. Pre-built
	// modules bypass the PDF/folder ingest pipeline, so without this they can carry
	// undisplayable TIFFs and blank layers. Persist the updated references so the
	// stored adventure.json matches the normalized files on disk.
	if t, d := ingest.NormalizeModuleImages(staging, &adv); t > 0 || d > 0 {
		if out, merr := json.MarshalIndent(&adv, "", "  "); merr == nil {
			_ = os.WriteFile(advPath, out, 0644)
		}
	}

	imageExists := func(rel string) bool {
		p, ok := safeJoin(staging, rel)
		if !ok {
			return false
		}
		info, statErr := os.Stat(p)
		return statErr == nil && !info.IsDir()
	}
	if verrs := domain.ValidateAdventure(&adv, imageExists); len(verrs) > 0 {
		return nil, fmt.Errorf("adventure validation failed:\n%s", joinErrs(verrs))
	}

	dest := s.AdventureDir(adv.ID)
	if err := os.RemoveAll(dest); err != nil {
		return nil, fmt.Errorf("failed to replace existing adventure %q: %w", adv.ID, err)
	}
	if err := os.Rename(staging, dest); err != nil {
		return nil, fmt.Errorf("failed to store adventure: %w", err)
	}
	cleanupStaging = false

	return &adv, nil
}

// LoadAdventure reads a previously imported adventure by ID.
func (s *Storage) LoadAdventure(id string) (*domain.Adventure, error) {
	advPath := filepath.Join(s.AdventureDir(id), AdventureFile)
	data, err := os.ReadFile(advPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read adventure %q: %w", id, err)
	}
	var adv domain.Adventure
	if err := json.Unmarshal(data, &adv); err != nil {
		return nil, fmt.Errorf("failed to parse adventure %q: %w", id, err)
	}
	adv.Migrate() // normalize directions + backfill the zone graph for older modules
	return &adv, nil
}

// ListAdventures enumerates imported adventures.
func (s *Storage) ListAdventures() ([]AdventureInfo, error) {
	entries, err := os.ReadDir(s.AdventuresPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read adventures directory: %w", err)
	}

	var out []AdventureInfo
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		adv, err := s.LoadAdventure(entry.Name())
		if err != nil {
			continue
		}
		out = append(out, AdventureInfo{
			ID:     adv.ID,
			Title:  adv.Title,
			Author: adv.Author,
			System: adv.System,
			Dir:    s.AdventureDir(adv.ID),
		})
	}
	return out, nil
}

// ResolveImagePath returns the absolute on-disk path of a relative image asset
// for an imported adventure, verifying it stays within the module directory.
func (s *Storage) ResolveImagePath(adventureID, relPath string) (string, error) {
	base := s.AdventureDir(adventureID)
	p, ok := safeJoin(base, relPath)
	if !ok {
		return "", fmt.Errorf("unsafe image path: %q", relPath)
	}
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("image not found: %q", relPath)
	}
	return p, nil
}

// AdventureExists reports whether an adventure with the given ID is imported.
func (s *Storage) AdventureExists(id string) bool {
	_, err := os.Stat(filepath.Join(s.AdventureDir(id), AdventureFile))
	return err == nil
}

// DeleteAdventure removes an imported adventure and its assets.
func (s *Storage) DeleteAdventure(id string) error {
	dir := s.AdventureDir(id)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("adventure not found: %s", id)
	}
	return os.RemoveAll(dir)
}

// --- helpers -------------------------------------------------------------

// extractTarGz safely extracts a gzip-compressed tar archive into destDir.
// It rejects absolute paths, path-traversal, and symlinks, and caps per-file
// size to prevent decompression bombs.
func extractTarGz(srcPath, destDir string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open module: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("module is not a valid gzip archive: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read module archive: %w", err)
		}

		target, ok := safeJoin(destDir, hdr.Name)
		if !ok {
			return fmt.Errorf("unsafe path in module archive: %q", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}
			// LimitReader guards against decompression bombs; +1 detects overflow.
			n, err := io.Copy(out, io.LimitReader(tr, maxDecompressedFileSize+1))
			closeErr := out.Close()
			if err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}
			if closeErr != nil {
				return fmt.Errorf("failed to close file: %w", closeErr)
			}
			if n > maxDecompressedFileSize {
				return fmt.Errorf("%q is larger than the %d MB per-file limit — this .tar.gz does not look like an adventure module (a module holds adventure.json and small images); did you pick the wrong file?", hdr.Name, maxDecompressedFileSize>>20)
			}
		default:
			// Skip symlinks, devices, etc. for safety.
			continue
		}
	}
	return nil
}

// safeJoin joins base and a relative (slash-separated) path from an archive or
// adventure reference, returning false if the result would escape base.
func safeJoin(base, name string) (string, bool) {
	if name == "" {
		return "", false
	}
	name = filepath.FromSlash(name)
	if filepath.IsAbs(name) {
		return "", false
	}
	target := filepath.Join(base, name)
	rel, err := filepath.Rel(base, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", false
	}
	return target, true
}

// sanitizeID makes an adventure ID safe to use as a single directory name.
func sanitizeID(id string) string {
	repl := func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '-', r == '_', r == '.':
			return r
		default:
			return '_'
		}
	}
	s := strings.Map(repl, id)
	s = strings.TrimLeft(s, ".") // never allow a hidden/staging-looking name
	if s == "" {
		s = "adventure"
	}
	return s
}

func joinErrs(errs []error) string {
	var sb strings.Builder
	for _, e := range errs {
		sb.WriteString("  - ")
		sb.WriteString(e.Error())
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

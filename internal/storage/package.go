package storage

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// PackageModule writes the contents of srcDir (which must contain an
// adventure.json at its root, plus any referenced assets) into a gzip-compressed
// tar archive at destPath. Entries are stored with paths relative to srcDir, so
// the resulting archive is importable via ImportModule. Hidden files/dirs (those
// whose name starts with ".") are skipped.
func PackageModule(srcDir, destPath string) error {
	if _, err := os.Stat(filepath.Join(srcDir, AdventureFile)); err != nil {
		return fmt.Errorf("%s not found in %s", AdventureFile, srcDir)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
	}
	defer out.Close()

	gz := gzip.NewWriter(out)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		slash := filepath.ToSlash(rel)
		// Package ONLY the canonical module layout: adventure.json at the root and
		// the assets/ tree. Anything else in the working directory is ignored, so a
		// polluted working dir (e.g. one that ended up pointing at a folder full of
		// unrelated files) never bloats the archive.
		top := slash
		if i := strings.IndexByte(slash, '/'); i >= 0 {
			top = slash[:i]
		}
		if top != AdventureFile && top != "assets" {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// Skip hidden files/dirs anywhere in the path.
		for _, part := range strings.Split(slash, "/") {
			if strings.HasPrefix(part, ".") {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if info.IsDir() {
			hdr.Name += "/"
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
}

// ExtractModule safely extracts a .tar.gz module into destDir (without importing
// it into the app's adventure store). It applies the same path-traversal and
// size protections as ImportModule. Useful for opening a module for editing.
func ExtractModule(srcPath, destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}
	return extractTarGz(srcPath, destDir)
}

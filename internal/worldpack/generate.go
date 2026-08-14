package worldpack

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GenerateOptions controls world pack generation from built-in templates.
type GenerateOptions struct {
	TemplateID string
	OutDir     string
	Name       string
	Language   string
	Format     string // json | yaml
}

// Generate builds a world pack from a built-in template.
func Generate(opts GenerateOptions) (*Pack, error) {
	id := strings.TrimSpace(opts.TemplateID)
	if id == "" {
		id = "shattered_vale"
	}
	p, err := Builtin(id)
	if err != nil {
		return nil, err
	}
	p, err = clonePack(p)
	if err != nil {
		return nil, err
	}

	if opts.Name != "" {
		p.Name = opts.Name
	}
	if opts.Language != "" {
		p.Language = opts.Language
	}

	BuildIndexes(p)

	p.Metadata = mergeMeta(p.Metadata, map[string]string{
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"generator":    "worldpack-gen",
	})

	if err := ValidatePackStrict(p); err != nil {
		return nil, err
	}
	return p, nil
}

// GenerateToFile writes a pack to OutDir/<id>.<ext>.
func GenerateToFile(opts GenerateOptions) (string, error) {
	pack, err := Generate(opts)
	if err != nil {
		return "", err
	}
	ext := "json"
	if opts.Format == "yaml" {
		ext = "yaml"
	}
	outDir := opts.OutDir
	if outDir == "" {
		outDir = "."
	}
	worldDir := filepath.Join(outDir, pack.ID)
	if err := os.MkdirAll(worldDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(worldDir, "world."+ext)
	if err := SavePack(path, pack); err != nil {
		return "", err
	}
	return path, nil
}

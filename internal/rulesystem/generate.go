package rulesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/ingest"
)

// GenerateOptions controls pack generation from templates and optional PDF input.
type GenerateOptions struct {
	TemplateID string
	PDFPath    string
	OutDir     string
	Name       string
	Language   string
	Format     string // json | yaml
	Enrich     bool
}

// Generate builds a rulesystem pack from a built-in template, optionally enriched
// with mechanically extracted PDF excerpts and an LLM-ready enrichment spec.
func Generate(opts GenerateOptions) (*Pack, error) {
	id, err := resolveTemplateID(opts)
	if err != nil {
		return nil, err
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

	if opts.PDFPath != "" {
		if err := enrichFromPDF(p, opts.PDFPath); err != nil {
			return nil, err
		}
	}

	if opts.Enrich {
		p.Enrichment = DefaultEnrichmentSpec(p)
	}

	p.Metadata = mergeMeta(p.Metadata, map[string]string{
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"generator":    "rulesystem-gen",
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
	path := filepath.Join(outDir, pack.ID+"."+ext)
	if err := SavePack(path, pack); err != nil {
		return "", err
	}
	return path, nil
}

func resolveTemplateID(opts GenerateOptions) (string, error) {
	id := strings.TrimSpace(opts.TemplateID)
	if id == "" && opts.PDFPath != "" {
		dir, err := os.MkdirTemp("", "thaim-rulesystem-detect-*")
		if err != nil {
			return "", err
		}
		defer os.RemoveAll(dir)
		text, _, err := ingest.ExtractPDF(opts.PDFPath, dir)
		if err != nil {
			return "", fmt.Errorf("extract pdf: %w", err)
		}
		id = DetectFamily(text)
	}
	if id == "" {
		id = "dnd5e"
	}
	return id, nil
}

func enrichFromPDF(p *Pack, pdfPath string) error {
	dir, err := os.MkdirTemp("", "thaim-rulesystem-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	text, _, err := ingest.ExtractPDF(pdfPath, dir)
	if err != nil {
		return fmt.Errorf("extract pdf: %w", err)
	}

	p.Source.Type = "pdf"
	if p.Source.Document == "" {
		p.Source.Document = filepath.Base(pdfPath)
	}
	p.Source.Notes = "Built-in scaffold merged with mechanically extracted PDF excerpts."
	p.Source.Confidence = 0.65

	lines := splitPages(text)
	excerpts := AnalyzeExcerpts(lines)
	p.RawExcerpts = append(p.RawExcerpts, excerpts...)
	if len(excerpts) > 0 {
		merged, err := MergePDFExcerpts(p, excerpts)
		if err != nil {
			return err
		}
		*p = *merged
		p.RulesSummary = append(p.RulesSummary, summarizeExcerpts(excerpts)...)
	}
	if p.Metadata == nil {
		p.Metadata = map[string]string{}
	}
	p.Metadata["detected_family"] = DetectFamily(text)
	return nil
}

func splitPages(text string) []string {
	var out []string
	var buf strings.Builder
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "=== Page ") {
			if buf.Len() > 0 {
				out = append(out, buf.String())
				buf.Reset()
			}
			continue
		}
		if buf.Len() > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(line)
	}
	if buf.Len() > 0 {
		out = append(out, buf.String())
	}
	if len(out) == 0 && strings.TrimSpace(text) != "" {
		out = append(out, text)
	}
	return out
}

func summarizeExcerpts(excerpts []SourceExcerpt) []string {
	out := make([]string, 0, len(excerpts))
	for _, e := range excerpts {
		line := strings.TrimSpace(e.Text)
		if line == "" {
			continue
		}
		if idx := strings.Index(line, "\n"); idx > 0 && idx < 160 {
			line = line[:idx]
		}
		if len(line) > 160 {
			line = line[:160] + "…"
		}
		if e.Heading != "" {
			line = e.Heading + ": " + line
		}
		if e.Category != "" {
			line = "[" + e.Category + "] " + line
		}
		out = append(out, line)
	}
	return out
}

func mergeMeta(base map[string]string, extra map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

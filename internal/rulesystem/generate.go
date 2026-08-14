package rulesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/ingest"
)

// GenerateOptions controls pack generation.
type GenerateOptions struct {
	TemplateID string // dnd5e | d100 | savage_worlds | "" (auto from PDF)
	PDFPath    string
	OutDir     string
	Name       string
	Language   string
	Format     string // json | yaml
}

// Generate builds a rulesystem pack. When PDFPath is set, text is extracted and
// used to refine metadata; the template (explicit or auto-detected) provides the
// structural scaffold. Generation does not call an LLM — AI enrichment is a future
// step documented in docs/rulesystem-generator.md.
func Generate(opts GenerateOptions) (*Pack, error) {
	base, err := pickTemplate(opts)
	if err != nil {
		return nil, err
	}
	pack := clonePack(base)

	if opts.Name != "" {
		pack.Name = opts.Name
	}
	if opts.Language != "" {
		pack.Language = opts.Language
	}

	if opts.PDFPath != "" {
		if err := enrichFromPDF(pack, opts.PDFPath); err != nil {
			return nil, err
		}
	}

	pack.Metadata = mergeMeta(pack.Metadata, map[string]string{
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"generator":    "rulesystem-gen",
	})
	if err := Validate(pack); err != nil {
		return nil, err
	}
	return pack, nil
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
	if err := Save(pack, path); err != nil {
		return "", err
	}
	return path, nil
}

func pickTemplate(opts GenerateOptions) (*Pack, error) {
	id := strings.TrimSpace(opts.TemplateID)
	if id == "" && opts.PDFPath != "" {
		text, _, err := ingest.ExtractPDF(opts.PDFPath, os.TempDir())
		if err != nil {
			return nil, fmt.Errorf("extract pdf: %w", err)
		}
		id = DetectFamily(text)
	}
	if id == "" {
		id = "dnd5e"
	}
	return Builtin(id)
}

func enrichFromPDF(pack *Pack, pdfPath string) error {
	dir, err := os.MkdirTemp("", "thaim-rulesystem-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	text, _, err := ingest.ExtractPDF(pdfPath, dir)
	if err != nil {
		return fmt.Errorf("extract pdf: %w", err)
	}
	pack.Source.Type = "pdf"
	pack.Source.Document = filepath.Base(pdfPath)
	pack.Source.Notes = "Scaffold from built-in template; excerpts extracted mechanically from PDF text."

	excerpts := ExtractExcerpts(text, 16)
	pack.RawExcerpts = excerpts
	if len(excerpts) > 0 {
		pack.RulesSummary = append(pack.RulesSummary, summarizeExcerpts(excerpts)...)
	}
	if pack.Metadata == nil {
		pack.Metadata = map[string]string{}
	}
	pack.Metadata["detected_family"] = DetectFamily(text)
	return nil
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
		out = append(out, line)
	}
	return out
}

func clonePack(p *Pack) *Pack {
	if p == nil {
		return nil
	}
	c := *p
	c.Attributes = append([]AttributeDef(nil), p.Attributes...)
	c.Skills = append([]SkillDef(nil), p.Skills...)
	c.Resources = append([]ResourceDef(nil), p.Resources...)
	c.Conditions = append([]ConditionDef(nil), p.Conditions...)
	c.Tools = append([]ToolBinding(nil), p.Tools...)
	c.Character.Fields = append([]CharacterField(nil), p.Character.Fields...)
	c.RulesSummary = append([]string(nil), p.RulesSummary...)
	c.RawExcerpts = append([]SourceExcerpt(nil), p.RawExcerpts...)
	if p.Metadata != nil {
		c.Metadata = map[string]string{}
		for k, v := range p.Metadata {
			c.Metadata[k] = v
		}
	}
	return &c
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

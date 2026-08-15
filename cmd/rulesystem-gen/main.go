// Command rulesystem-gen generates portable RPG rulesystem packs for thAImaturgy.
// Packs are standalone artifacts — the main oracle/engine does not load them yet.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/theburrowhub/thaimaturgy/internal/rulesystem"
	_ "github.com/theburrowhub/thaimaturgy/internal/rulesystem/profiles"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		list     = flag.Bool("list", false, "list built-in templates and exit")
		template = flag.String("template", "", "built-in template: dnd5e | d100 | savage_worlds")
		builtin  = flag.String("builtin", "", "alias for -template")
		pdf      = flag.String("pdf", "", "optional PDF to extract and merge rule excerpts")
		out      = flag.String("out", "examples/rulesystems", "output directory or file path")
		name     = flag.String("name", "", "override pack display name")
		lang     = flag.String("lang", "en", "language code for prompts")
		format   = flag.String("format", "json", "output format: json | yaml")
		all      = flag.Bool("all", false, "generate all built-in templates")
		inspect  = flag.Bool("inspect", false, "print inspection report")
		validate = flag.Bool("validate", false, "validate pack (non-zero exit on failure)")
		enrich   = flag.Bool("enrich", false, "attach LLM-ready enrichment spec")
		diff     = flag.String("diff", "", "compare two pack files and print summary")
	)
	flag.Parse()

	if *diff != "" {
		parts := strings.Split(*diff, ",")
		if len(parts) != 2 {
			return fmt.Errorf("-diff expects two comma-separated paths")
		}
		a, err := rulesystem.LoadPack(strings.TrimSpace(parts[0]))
		if err != nil {
			return err
		}
		b, err := rulesystem.LoadPack(strings.TrimSpace(parts[1]))
		if err != nil {
			return err
		}
		fmt.Println(rulesystem.DiffSummary(a, b))
		return nil
	}

	if *list {
		return printList()
	}

	tmpl := *template
	if tmpl == "" {
		tmpl = *builtin
	}

	if *all {
		for _, id := range rulesystem.BuiltinIDs() {
			if err := generateOne(id, *out, *name, *lang, *format, *pdf, *enrich, *inspect, *validate); err != nil {
				return err
			}
		}
		return nil
	}

	if tmpl == "" && *pdf == "" && !*inspect && !*validate {
		flag.Usage()
		return fmt.Errorf("provide -template, -pdf, -all, or -diff")
	}

	if tmpl == "" {
		tmpl = "dnd5e"
	}
	return generateOne(tmpl, *out, *name, *lang, *format, *pdf, *enrich, *inspect, *validate)
}

func generateOne(tmpl, out, name, lang, format, pdfPath string, enrich, inspect, validate bool) error {
	opts := rulesystem.GenerateOptions{
		TemplateID: tmpl,
		PDFPath:    pdfPath,
		Name:       name,
		Language:   lang,
		Format:     format,
		Enrich:     enrich,
	}

	pack, err := rulesystem.Generate(opts)
	if err != nil {
		return err
	}

	if validate {
		if err := rulesystem.ValidatePackStrict(pack); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
		fmt.Fprintln(os.Stderr, "validation: OK")
	}

	if inspect {
		fmt.Println(rulesystem.InspectReport(pack))
	}

	outPath := out
	if strings.HasSuffix(outPath, ".json") || strings.HasSuffix(outPath, ".yaml") || strings.HasSuffix(outPath, ".yml") {
		if err := rulesystem.SavePack(outPath, pack); err != nil {
			return err
		}
		fmt.Println("wrote", outPath)
		return nil
	}

	opts.OutDir = outPath
	path, err := rulesystem.GenerateToFile(opts)
	if err != nil {
		return err
	}
	if !inspect {
		fmt.Println("wrote", path)
	}
	return nil
}

func printList() error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tTOOLS\tWORKFLOWS\tCHAPTERS")
	for _, id := range rulesystem.BuiltinIDs() {
		p, err := rulesystem.Builtin(id)
		if err != nil {
			return err
		}
		enabled := 0
		for _, t := range p.Tools {
			if t.Enabled {
				enabled++
			}
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%d\n", p.ID, p.Name, enabled, len(p.Workflows), len(p.Chapters))
	}
	return w.Flush()
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nTemplates:", strings.Join(rulesystem.BuiltinIDs(), ", "))
	}
}

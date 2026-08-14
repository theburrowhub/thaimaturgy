// Command rulesystem-gen generates portable RPG rulesystem packs for thAImaturgy.
// Packs are standalone artifacts — the main oracle/engine does not load them yet.
//
// Usage:
//
//	rulesystem-gen -list
//	rulesystem-gen -template dnd5e -out examples/rulesystems/
//	rulesystem-gen -template d100 -format yaml -out dist/rulesystems/
//	rulesystem-gen -pdf my-rules.pdf -out dist/rulesystems/   # auto-detect family
//	rulesystem-gen -template savage_worlds -pdf swade.pdf -out dist/
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/theburrowhub/thaimaturgy/internal/rulesystem"
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
		pdf      = flag.String("pdf", "", "optional PDF to extract rules excerpts from")
		out      = flag.String("out", "examples/rulesystems", "output directory")
		name     = flag.String("name", "", "override pack display name")
		lang     = flag.String("lang", "en", "language code for prompts")
		format   = flag.String("format", "json", "output format: json | yaml")
		all      = flag.Bool("all", false, "generate all built-in templates to -out")
	)
	flag.Parse()

	if *list {
		return printList()
	}

	if *all {
		for _, id := range rulesystem.BuiltinIDs {
			path, err := rulesystem.GenerateToFile(rulesystem.GenerateOptions{
				TemplateID: id,
				OutDir:     *out,
				Language:   *lang,
				Format:     *format,
			})
			if err != nil {
				return err
			}
			fmt.Println("wrote", path)
		}
		return nil
	}

	if *template == "" && *pdf == "" {
		flag.Usage()
		return fmt.Errorf("provide -template, -pdf, or -all")
	}

	path, err := rulesystem.GenerateToFile(rulesystem.GenerateOptions{
		TemplateID: *template,
		PDFPath:    *pdf,
		OutDir:     *out,
		Name:       *name,
		Language:   *lang,
		Format:     *format,
	})
	if err != nil {
		return err
	}
	fmt.Println("wrote", path)
	return nil
}

func printList() error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tTOOLS")
	for _, id := range rulesystem.BuiltinIDs {
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
		fmt.Fprintf(w, "%s\t%s\t%d\n", p.ID, p.Name, enabled)
	}
	return w.Flush()
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nTemplates:", strings.Join(rulesystem.BuiltinIDs, ", "))
	}
}

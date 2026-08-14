// Command worldpack-gen generates portable world content packs for thAImaturgy.
// Packs are standalone artifacts — the main oracle/engine does not load them yet.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/theburrowhub/thaimaturgy/internal/worldpack"
	_ "github.com/theburrowhub/thaimaturgy/internal/worldpack/worlds"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		list         = flag.Bool("list", false, "list built-in templates and exit")
		template     = flag.String("template", "", "built-in template: dnd5e_shattered_vale | dnd5e")
		out          = flag.String("out", "examples/worlds", "output directory or file path")
		name         = flag.String("name", "", "override pack display name")
		lang         = flag.String("lang", "en", "language code")
		format       = flag.String("format", "json", "output format: json | yaml")
		all          = flag.Bool("all", false, "generate all built-in templates")
		inspect      = flag.Bool("inspect", false, "print inspection report")
		validate     = flag.Bool("validate", false, "validate pack (non-zero exit on failure)")
		buildIndexes = flag.Bool("build-indexes", false, "rebuild indexes before save (always on by default)")
	)
	flag.Parse()

	if *list {
		return printList()
	}

	if *all {
		for _, id := range worldpack.BuiltinIDs() {
			if err := generateOne(id, *out, *name, *lang, *format, *inspect, *validate, *buildIndexes); err != nil {
				return err
			}
		}
		return nil
	}

	tmpl := strings.TrimSpace(*template)
	if tmpl == "" && !*inspect && !*validate {
		flag.Usage()
		return fmt.Errorf("provide -template, -all, or load a pack with -validate/-inspect via generation")
	}
	if tmpl == "" {
		tmpl = "shattered_vale"
	}
	return generateOne(tmpl, *out, *name, *lang, *format, *inspect, *validate, *buildIndexes)
}

func generateOne(tmpl, out, name, lang, format string, inspect, validate, buildIndexes bool) error {
	opts := worldpack.GenerateOptions{
		TemplateID: tmpl,
		Name:       name,
		Language:   lang,
		Format:     format,
	}
	pack, err := worldpack.Generate(opts)
	if err != nil {
		return err
	}
	if buildIndexes {
		worldpack.BuildIndexes(pack)
	}

	if validate {
		if err := worldpack.ValidatePackStrict(pack); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
		fmt.Fprintln(os.Stderr, "validation: OK")
	}

	if inspect {
		fmt.Println(worldpack.InspectReport(pack))
	}

	outPath := out
	if strings.HasSuffix(outPath, ".json") || strings.HasSuffix(outPath, ".yaml") || strings.HasSuffix(outPath, ".yml") {
		if err := worldpack.SavePack(outPath, pack); err != nil {
			return err
		}
		fmt.Println("wrote", outPath)
		return nil
	}

	opts.OutDir = outPath
	path, err := worldpack.GenerateToFile(opts)
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
	fmt.Fprintln(w, "ID\tNAME\tREGIONS\tCITIES\tLOCATIONS\tNPCS\tCREATURES")
	for _, id := range worldpack.BuiltinIDs() {
		p, err := worldpack.Builtin(id)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%d\t%d\t%d\n",
			p.ID, p.Name, len(p.Regions), len(p.Cities), len(p.Locations), len(p.NPCs), len(p.Creatures))
	}
	return w.Flush()
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nTemplates:", strings.Join(worldpack.BuiltinIDs(), ", "))
		fmt.Fprintln(os.Stderr, "Worlds: shattered_vale, caribdus (aliases: dnd5e, 50_brazas)")
	}
}

// Command thaimaturgy-novel is a headless console tool that turns a played
// session into a prose novelization of the adventure — a book to read, in
// Markdown or a print-ready PDF. It reuses the same storage/config/provider
// setup as the desktop app, so it operates on the same ~/.thaimaturgy data (or
// a THAIM_DATA_DIR volume), and the shared internal/novel + internal/bookpdf
// packages that back the GUI's "Export novel" action.
//
// Usage:
//
//	thaimaturgy-novel -list                       # list resumable sessions
//	thaimaturgy-novel -session "My Session"       # → <id>-novel.md
//	thaimaturgy-novel -session "My Session" -format pdf -out book.pdf
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/auth"
	"github.com/theburrowhub/thaimaturgy/internal/bookpdf"
	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/novel"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		session = flag.String("session", "", "name of the persisted session to novelize (see -list)")
		out     = flag.String("out", "", "output file path (default: <adventure-id>-novel.<ext>)")
		format  = flag.String("format", "md", "output format: md | pdf")
		model   = flag.String("model", "", "override the model id (default: from config)")
		list    = flag.Bool("list", false, "list resumable sessions and exit")
		timeout = flag.Duration("timeout", 30*time.Minute, "max time to wait for the whole (multi-pass) generation")
		segment = flag.Int("segment-chars", 0, "characters of play log per generation pass (0 = default); smaller = more passes")
	)
	flag.Parse()

	// Same data-dir resolution as the server: a pinned THAIM_DATA_DIR (for a
	// mounted volume) or the default ~/.thaimaturgy.
	var store *storage.Storage
	var err error
	if dataDir := strings.TrimSpace(os.Getenv("THAIM_DATA_DIR")); dataDir != "" {
		store, err = storage.NewWithPath(dataDir)
	} else {
		store, err = storage.New()
	}
	if err != nil {
		return fmt.Errorf("storage: %w", err)
	}

	if *list {
		return listSessions(store)
	}
	if strings.TrimSpace(*session) == "" {
		flag.Usage()
		fmt.Fprintln(os.Stderr, "\nno -session given; here are the available sessions:")
		return listSessions(store)
	}

	_ = store.LoadEnvFile()
	config, err := store.LoadConfig()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	msg := auth.AutoConfigure(config)
	if config.RunModel != "" {
		config.Model = config.RunModel
	}
	if strings.TrimSpace(*model) != "" {
		config.Model = *model
	}

	prov := providers.New(config)
	if prov == nil {
		return fmt.Errorf("no AI provider configured; set an API key or use the Claude CLI backend")
	}

	st, err := store.LoadSession(*session)
	if err != nil {
		return err
	}
	adv, err := store.LoadAdventure(st.AdventureID)
	if err != nil {
		return fmt.Errorf("load adventure %q for session %q: %w", st.AdventureID, *session, err)
	}

	fmt.Fprintf(os.Stderr, "provider: %s\n", msg)
	fmt.Fprintf(os.Stderr, "novelizing %q (%s) — this can take a minute…\n", adv.Title, config.Model)

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	md, err := novel.GenerateWithOptions(ctx, prov, config.Model, adv, st, novel.Options{
		SegmentChars: *segment,
		Progress: func(n, total int) {
			fmt.Fprintf(os.Stderr, "  pass %d/%d…\n", n, total)
		},
	})
	if err != nil {
		return fmt.Errorf("novel generation failed: %w", err)
	}

	dest := strings.TrimSpace(*out)
	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "md", "markdown":
		if dest == "" {
			dest = adv.ID + "-novel.md"
		}
		if err := os.WriteFile(dest, []byte(md), 0644); err != nil {
			return err
		}
	case "pdf":
		if dest == "" {
			dest = adv.ID + "-novel.pdf"
		}
		pdfBytes, err := bookpdf.FromMarkdown(adv.Title, subtitle(adv), md)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dest, pdfBytes, 0644); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown -format %q (use md or pdf)", *format)
	}

	fmt.Fprintf(os.Stderr, "novel written to %s\n", dest)
	return nil
}

// subtitle matches the GUI's localized book subtitle.
func subtitle(adv *domain.Adventure) string {
	if strings.HasPrefix(strings.ToLower(adv.Language), "es") {
		return "Una novelización de la partida"
	}
	return "A novelization of the play session"
}

func listSessions(store *storage.Storage) error {
	sessions, err := store.ListSessions()
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		fmt.Println("no sessions found")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "SESSION\tADVENTURE\tMODIFIED")
	for _, s := range sessions {
		mod := ""
		if t, ok := s.ModifiedAt.(time.Time); ok {
			mod = t.Format("2006-01-02 15:04")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", s.Name, s.AdventureTitle, mod)
	}
	return w.Flush()
}

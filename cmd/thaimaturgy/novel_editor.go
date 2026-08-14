package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/theburrowhub/thaimaturgy/internal/bookpdf"
	"github.com/theburrowhub/thaimaturgy/internal/nativeui"
	"github.com/theburrowhub/thaimaturgy/internal/novel"
)

// novelOps abstracts the novel-editor's backend so the same modal drives both the
// in-process (local) GUI and the remote (apiclient) GUI. Every method blocks and
// must be called from a goroutine; the modal marshals UI updates via fyne.Do.
type novelOps interface {
	// load returns the saved novel text and an opaque version token (threaded back
	// to save for optimistic concurrency), or "" text when none exists yet.
	load(ctx context.Context) (text, version string, err error)
	// save persists edited text; base is the version from load/generate. Returns
	// the new version.
	save(ctx context.Context, text, base string) (version string, err error)
	// generate (re)writes the novel from the session and persists it, returning the
	// new text and version.
	generate(ctx context.Context) (text, version string, err error)
	// adjust revises text with the AI (only the selection when non-empty) and
	// returns the revised prose (whole text, or the revised excerpt).
	adjust(ctx context.Context, text, selection, instruction string) (string, error)
	// exportBytes renders the given text to md/pdf bytes for saving to a file.
	exportBytes(ctx context.Context, format, text string) ([]byte, error)
	// titles returns the book title and a default export filename stem.
	titles() (title, fileStem string)
}

// openNovelEditor opens the novel editor for the current session, picking the
// local or remote backend.
func (g *gui) openNovelEditor() {
	var ops novelOps
	if g.remote != nil {
		if g.remoteName == "" {
			return
		}
		ops = &remoteNovelOps{g: g, name: g.remoteName}
	} else {
		if g.session == nil {
			return
		}
		ops = &localNovelOps{g: g, name: g.session.State.Name}
	}
	showNovelEditor(g, ops)
}

// showNovelEditor builds the modal editor: generate, hand-edit, AI-adjust (whole
// text or the selection), save, and export to md/pdf.
func showNovelEditor(g *gui, ops novelOps) {
	text := widget.NewMultiLineEntry()
	text.Wrapping = fyne.TextWrapWord
	text.SetPlaceHolder("No novel yet. Click Generate to write one from this session, then edit it here or adjust it with AI.")

	instruction := widget.NewEntry()
	instruction.SetPlaceHolder("Adjust with AI, e.g. \"make chapter 2 darker\" — select text first to revise only that")

	statusLbl := widget.NewLabel("")
	version := ""  // opaque concurrency token for the loaded/saved text
	dirty := false // unsaved manual edits

	var pop *widget.PopUp
	var genBtn, adjustBtn, saveBtn, exportMDBtn, exportPDFBtn, closeBtn *widget.Button

	setBusy := func(b bool) {
		for _, btn := range []*widget.Button{genBtn, adjustBtn, saveBtn, exportMDBtn, exportPDFBtn} {
			if btn == nil {
				continue
			}
			if b {
				btn.Disable()
			} else {
				btn.Enable()
			}
		}
		if b {
			instruction.Disable()
			text.Disable()
		} else {
			instruction.Enable()
			text.Enable()
		}
	}
	setStatus := func(s string) { fyne.Do(func() { statusLbl.SetText(s) }) }

	text.OnChanged = func(string) {
		if !dirty {
			dirty = true
			fyne.Do(func() { statusLbl.SetText("unsaved changes") })
		}
	}

	// run executes a blocking op in a goroutine with busy-gating, then calls done
	// on the UI thread.
	run := func(fn func()) {
		setBusy(true)
		go func() {
			fn()
			fyne.Do(func() { setBusy(false) })
		}()
	}

	genBtn = widget.NewButtonWithIcon("Generate", theme.DocumentCreateIcon(), func() {
		if text.Text != "" && nativeui.Choice("Generate novel",
			"Generate a new novel from the session? This replaces the current text (unsaved edits are lost).",
			"Generate", "Cancel") != 1 {
			return
		}
		setStatus("writing the novel… this can take a minute")
		run(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
			defer cancel()
			md, ver, err := ops.generate(ctx)
			if err != nil {
				setStatus("generation failed")
				g.showErr(err)
				return
			}
			fyne.Do(func() {
				text.SetText(md)
				version = ver
				dirty = false
				statusLbl.SetText("generated & saved")
			})
		})
	})

	adjustBtn = widget.NewButtonWithIcon("Adjust ✨", theme.MediaPlayIcon(), func() {
		instr := strings.TrimSpace(instruction.Text)
		if instr == "" {
			g.showErr(fmt.Errorf("type an adjustment instruction first"))
			return
		}
		if strings.TrimSpace(text.Text) == "" {
			g.showErr(fmt.Errorf("generate or write a novel first"))
			return
		}
		sel := text.SelectedText()
		full := text.Text
		setStatus(func() string {
			if sel != "" {
				return "revising the selection…"
			}
			return "revising the whole novel…"
		}())
		run(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
			defer cancel()
			revised, err := ops.adjust(ctx, full, sel, instr)
			if err != nil {
				setStatus("adjustment failed")
				g.showErr(err)
				return
			}
			newText := revised
			if sel != "" {
				// Splice the revised excerpt back in place of the (first) selection.
				newText = strings.Replace(full, sel, revised, 1)
			}
			fyne.Do(func() {
				text.SetText(newText)
				instruction.SetText("")
				dirty = true
				statusLbl.SetText("adjusted — review and Save")
			})
		})
	})

	saveNovel := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		ver, err := ops.save(ctx, text.Text, version)
		if err != nil {
			return err
		}
		fyne.Do(func() {
			version = ver
			dirty = false
		})
		return nil
	}
	saveBtn = widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), func() {
		setStatus("saving…")
		run(func() {
			if err := saveNovel(); err != nil {
				setStatus("save failed")
				g.showErr(err)
				return
			}
			setStatus("saved")
		})
	})

	doExport := func(format, ext string) {
		run(func() {
			// Persist pending edits first so the export reflects them.
			if dirty {
				if err := saveNovel(); err != nil {
					g.showErr(err)
					return
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			data, err := ops.exportBytes(ctx, format, text.Text)
			if err != nil {
				g.showErr(err)
				return
			}
			_, stem := ops.titles()
			dest, ok := nativeui.SaveFile("Save novel", stem+"-novel."+ext,
				nativeui.Filter{Name: strings.ToUpper(ext), Patterns: []string{"*." + ext}})
			if !ok {
				return
			}
			if err := os.WriteFile(dest, data, 0644); err != nil {
				g.showErr(err)
				return
			}
			nativeui.Info("Novel exported", "Saved:\n"+dest)
			setStatus("exported")
		})
	}
	exportMDBtn = widget.NewButton("Export .md", func() { doExport("md", "md") })
	exportPDFBtn = widget.NewButton("Export .pdf", func() { doExport("pdf", "pdf") })

	closeBtn = widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
		if dirty && nativeui.Choice("Novel editor",
			"Discard unsaved changes to the novel?", "Discard", "Keep editing") != 1 {
			return
		}
		pop.Hide()
	})

	header := container.NewHBox(
		widget.NewLabelWithStyle("Novel editor", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		statusLbl, layoutSpacer(), closeBtn,
	)
	toolbar := container.NewHBox(genBtn, layoutSpacer(), exportMDBtn, exportPDFBtn, saveBtn)
	adjustRow := container.NewBorder(nil, nil, nil, adjustBtn, instruction)
	top := container.NewVBox(header, toolbar, adjustRow)
	content := container.NewBorder(top, nil, nil, nil, text)

	pop = widget.NewModalPopUp(container.NewPadded(content), g.win.Canvas())
	pop.Resize(fyne.NewSize(880, 620))
	pop.Show()

	// Load any saved novel into the editor.
	setBusy(true)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		md, ver, err := ops.load(ctx)
		fyne.Do(func() {
			setBusy(false)
			if err != nil {
				g.showErr(err)
				return
			}
			text.SetText(md)
			version = ver
			dirty = false
			if md == "" {
				statusLbl.SetText("no novel yet — Generate to start")
			} else {
				statusLbl.SetText("loaded")
			}
		})
	}()
}

// --- local backend (in-process core) ------------------------------------

type localNovelOps struct {
	g    *gui
	name string
}

func (o *localNovelOps) titles() (string, string) {
	adv := o.g.session.Adventure
	return adv.Title, adv.ID
}

func (o *localNovelOps) subtitle() string {
	if strings.HasPrefix(strings.ToLower(o.g.session.Adventure.Language), "es") {
		return "Una novelización de la partida"
	}
	return "A novelization of the play session"
}

func (o *localNovelOps) load(context.Context) (string, string, error) {
	md, err := o.g.store.LoadNovel(o.name)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", nil
		}
		return "", "", err
	}
	return md, "", nil
}

func (o *localNovelOps) save(_ context.Context, text, _ string) (string, error) {
	return "", o.g.store.SaveNovel(o.name, text)
}

func (o *localNovelOps) generate(ctx context.Context) (string, string, error) {
	if o.g.prov == nil {
		return "", "", fmt.Errorf("no AI provider configured; set an API key or use the Claude CLI backend")
	}
	md, err := novel.Generate(ctx, o.g.prov, o.g.config.Model, o.g.session.Adventure, o.g.session.State)
	if err != nil {
		return "", "", err
	}
	if err := o.g.store.SaveNovel(o.name, md); err != nil {
		return "", "", err
	}
	return md, "", nil
}

func (o *localNovelOps) adjust(ctx context.Context, text, selection, instruction string) (string, error) {
	if o.g.prov == nil {
		return "", fmt.Errorf("no AI provider configured; set an API key or use the Claude CLI backend")
	}
	return novel.Adjust(ctx, o.g.prov, o.g.config.Model, o.g.session.Adventure, o.g.session.State, novel.AdjustOptions{
		FullText: text, Selection: selection, Instruction: instruction,
	})
}

func (o *localNovelOps) exportBytes(_ context.Context, format, text string) ([]byte, error) {
	if format == "pdf" {
		return bookpdf.FromMarkdown(o.g.session.Adventure.Title, o.subtitle(), text)
	}
	return []byte(text), nil
}

// --- remote backend (apiclient) -----------------------------------------

type remoteNovelOps struct {
	g    *gui
	name string
}

func (o *remoteNovelOps) titles() (string, string) { return o.name, o.name }

func (o *remoteNovelOps) load(ctx context.Context) (string, string, error) {
	text, version, _, err := o.g.remote.NovelText(ctx, o.name)
	return text, version, err
}

func (o *remoteNovelOps) save(ctx context.Context, text, base string) (string, error) {
	return o.g.remote.SaveNovelText(ctx, o.name, text, base)
}

func (o *remoteNovelOps) generate(ctx context.Context) (string, string, error) {
	job, err := o.g.remote.StartNovelJob(ctx, o.name)
	if err != nil {
		return "", "", err
	}
	if err := o.pollJob(ctx, job.ID); err != nil {
		return "", "", err
	}
	// Generate persisted server-side; reload the saved text + version.
	text, version, _, err := o.g.remote.NovelText(ctx, o.name)
	return text, version, err
}

func (o *remoteNovelOps) adjust(ctx context.Context, text, selection, instruction string) (string, error) {
	job, err := o.g.remote.StartNovelAdjustJob(ctx, o.name, text, selection, instruction)
	if err != nil {
		return "", err
	}
	if err := o.pollJob(ctx, job.ID); err != nil {
		return "", err
	}
	return o.g.remote.NovelJobResult(ctx, job.ID)
}

func (o *remoteNovelOps) exportBytes(ctx context.Context, format, _ string) ([]byte, error) {
	return o.g.remote.DownloadSessionNovel(ctx, o.name, format)
}

// pollJob waits for a remote novel job to reach a terminal state.
func (o *remoteNovelOps) pollJob(ctx context.Context, id string) error {
	for {
		st, err := o.g.remote.NovelJob(ctx, id)
		if err != nil {
			return err
		}
		switch st.Status {
		case "done":
			return nil
		case "error":
			if st.Error != "" {
				return fmt.Errorf("%s", st.Error)
			}
			return fmt.Errorf("the AI job failed")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
}

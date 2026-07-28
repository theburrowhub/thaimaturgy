// Command thaimaturgy-gui is a desktop GUI frontend for the thAImaturgy DM
// oracle. It reuses the same internal/ core as the TUI (domain, storage,
// engine, providers) and renders maps and art inline so the DM never needs an
// external image viewer.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	fynestorage "fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"github.com/theburrowhub/thaimaturgy/internal/auth"
	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/engine"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

type gui struct {
	app     fyne.App
	win     fyne.Window
	store   *storage.Storage
	config  *domain.Config
	prov    providers.Provider
	authMsg string

	// Active session (nil on the library screen).
	session *domain.Session
	oracle  *engine.Oracle
	cmd     *engine.CommandHandler

	// Session widgets.
	transcript   *widget.RichText
	transScroll  *container.Scroll
	logText      *widget.RichText
	logScroll    *container.Scroll
	location     *widget.RichText
	imageBox     *fyne.Container
	entry        *widget.Entry
	transcriptMD string
}

func main() {
	store, err := storage.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "storage: %v\n", err)
		os.Exit(1)
	}
	_ = store.LoadEnvFile()
	config, err := store.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	g := &gui{
		app:    app.New(),
		store:  store,
		config: config,
	}
	g.authMsg = auth.AutoConfigure(config)
	if !store.ConfigExists() {
		_ = store.SaveConfig(config) // generate config.yaml on first run
	}
	g.prov = providers.New(config)
	g.win = g.app.NewWindow("thAImaturgy — DM Oracle")
	g.win.Resize(fyne.NewSize(1200, 780))
	g.showLibrary()
	g.win.ShowAndRun()
}

// --- Library screen ------------------------------------------------------

func (g *gui) showLibrary() {
	g.session, g.oracle, g.cmd = nil, nil, nil

	title := widget.NewLabelWithStyle("thAImaturgy — Adventure Library",
		fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	importBtn := widget.NewButton("Import module (.tar.gz)…", g.importDialog)

	list := container.NewVBox()

	advs, _ := g.store.ListAdventures()
	if len(advs) > 0 {
		list.Add(widget.NewLabelWithStyle("Adventures", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		for _, a := range advs {
			id := a.ID
			list.Add(widget.NewButton("▶  "+a.Title, func() { g.startSession(id) }))
		}
	}

	sessions, _ := g.store.ListSessions()
	if len(sessions) > 0 {
		list.Add(widget.NewLabelWithStyle("Resume session", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		for _, s := range sessions {
			name := s.Name
			label := fmt.Sprintf("↻  %s — %s", s.Name, s.AdventureTitle)
			list.Add(widget.NewButton(label, func() { g.resumeSession(name) }))
		}
	}

	if len(advs) == 0 && len(sessions) == 0 {
		list.Add(widget.NewLabel("No adventures yet. Import a module to begin."))
	}

	bottom := container.NewVBox()
	if g.authMsg != "" {
		bottom.Add(widget.NewLabelWithStyle("✓ "+g.authMsg, fyne.TextAlignCenter, fyne.TextStyle{Italic: true}))
	}
	if !g.config.IsConfigured() {
		bottom.Add(widget.NewLabelWithStyle("⚠ No credentials found — the oracle is disabled (you can still browse). Set an API key or log in with Claude Code / Gemini.",
			fyne.TextAlignCenter, fyne.TextStyle{Italic: true}))
	}

	top := container.NewVBox(title, importBtn, widget.NewSeparator())
	content := container.NewBorder(top, bottom, nil, nil, container.NewVScroll(list))
	g.win.SetContent(content)
}

func (g *gui) importDialog() {
	d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			return
		}
		path := r.URI().Path()
		_ = r.Close()
		adv, err := g.store.ImportModule(path)
		if err != nil {
			dialog.ShowError(err, g.win)
			return
		}
		dialog.ShowInformation("Imported", "Imported: "+adv.Title, g.win)
		g.showLibrary()
	}, g.win)
	d.SetFilter(fynestorage.NewExtensionFileFilter([]string{".gz", ".tgz", ".tar.gz"}))
	d.Show()
}

// --- Session screen ------------------------------------------------------

func (g *gui) startSession(advID string) {
	adv, err := g.store.LoadAdventure(advID)
	if err != nil {
		dialog.ShowError(err, g.win)
		return
	}
	name := advID
	for i := 1; g.store.SessionExists(name); i++ {
		name = fmt.Sprintf("%s-%d", advID, i)
	}
	g.openSession(domain.NewSessionState(name, adv), adv)
}

func (g *gui) resumeSession(name string) {
	state, err := g.store.LoadSession(name)
	if err != nil {
		dialog.ShowError(err, g.win)
		return
	}
	adv, err := g.store.LoadAdventure(state.AdventureID)
	if err != nil {
		dialog.ShowError(fmt.Errorf("adventure %q not found; import it first", state.AdventureID), g.win)
		return
	}
	g.openSession(state, adv)
}

func (g *gui) openSession(state *domain.SessionState, adv *domain.Adventure) {
	g.session = domain.NewSession(state, adv, g.config)
	g.oracle = engine.NewOracle(g.session, g.prov)
	g.cmd = engine.NewCommandHandler(g.session)

	g.transcript = widget.NewRichTextFromMarkdown("")
	g.transcript.Wrapping = fyne.TextWrapWord
	g.transScroll = container.NewVScroll(g.transcript)
	g.transcriptMD = fmt.Sprintf("_Running **%s**. Ask a question or type a /command._\n\n", adv.Title)
	g.transcript.ParseMarkdown(g.transcriptMD)

	g.logText = widget.NewRichTextFromMarkdown("")
	g.logText.Wrapping = fyne.TextWrapWord
	g.logScroll = container.NewVScroll(g.logText)

	g.location = widget.NewRichTextFromMarkdown("")
	g.location.Wrapping = fyne.TextWrapWord
	g.imageBox = container.NewStack()

	g.entry = widget.NewEntry()
	g.entry.SetPlaceHolder("Ask the oracle, or type a /command…")
	g.entry.OnSubmitted = func(s string) { g.submit(s) }
	sendBtn := widget.NewButton("Send", func() { g.submit(g.entry.Text) })

	// Left column: location + inline image.
	leftTop := container.NewVScroll(g.location)
	left := container.NewVSplit(leftTop, container.NewVScroll(g.imageBox))
	left.SetOffset(0.45)

	// Center: transcript + input.
	inputRow := container.NewBorder(nil, nil, nil, sendBtn, g.entry)
	center := container.NewBorder(nil, inputRow, nil, nil, g.transScroll)

	// Right: session log.
	right := container.NewBorder(
		widget.NewLabelWithStyle("Session Log", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		nil, nil, nil, g.logScroll)

	body := container.NewHSplit(left, container.NewHSplit(center, right))
	body.SetOffset(0.24)

	toolbar := container.NewHBox(
		widget.NewButton("← Library", g.showLibrary),
		widget.NewButton("Save", g.save),
		widget.NewButton("Map", func() { g.openZoneMap() }),
		widget.NewLabel(adv.Title),
	)

	g.win.SetContent(container.NewBorder(toolbar, nil, nil, nil, body))
	g.refreshLocation()
	g.refreshLog()
	g.win.Canvas().Focus(g.entry)
}

func (g *gui) submit(raw string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}
	g.entry.SetText("")

	cmd := engine.ParseCommand(raw)
	if cmd == nil {
		return
	}
	result := g.cmd.Execute(cmd)

	if result.ShouldQuit {
		g.showLibrary()
		return
	}
	if result.Response != "" {
		g.appendTranscript("**» " + raw + "**\n\n" + "```\n" + result.Response + "\n```")
	}

	if result.NeedsUI {
		switch result.UIAction {
		case "oracle":
			g.appendTranscript("**» " + raw + "**")
			g.ask(result.UIArg)
		case "save":
			g.save()
		case "image":
			g.showImage(result.UIArg)
		case "import":
			g.importDialog()
		case "load":
			g.showLibrary()
		}
	} else if result.Message != "" {
		if !result.Success {
			dialog.ShowError(fmt.Errorf("%s", result.Message), g.win)
		} else {
			g.appendTranscript("_" + result.Message + "_")
		}
	}

	g.refreshLocation()
	g.refreshLog()
}

func (g *gui) ask(input string) {
	if g.oracle == nil || g.prov == nil {
		dialog.ShowError(fmt.Errorf("no AI provider configured; set an API key"), g.win)
		return
	}
	g.appendTranscript("_Consulting the oracle…_")
	timeout := time.Duration(g.config.RequestTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		resp := g.oracle.Ask(ctx, input)
		fyne.Do(func() {
			if resp.Error != nil {
				dialog.ShowError(resp.Error, g.win)
				return
			}
			g.appendTranscript(resp.Answer)
			g.refreshLocation()
			g.refreshLog()
			if g.config.AutoSave {
				go func() { _ = g.store.SaveSession(g.session.State) }()
			}
		})
	}()
}

func (g *gui) save() {
	if g.session == nil {
		return
	}
	if err := g.store.SaveSession(g.session.State); err != nil {
		dialog.ShowError(err, g.win)
		return
	}
	g.appendTranscript("_Session saved._")
}

func (g *gui) appendTranscript(md string) {
	g.transcriptMD += md + "\n\n"
	g.transcript.ParseMarkdown(g.transcriptMD)
	g.transScroll.ScrollToBottom()
}

func (g *gui) refreshLocation() {
	if g.session == nil {
		return
	}
	adv, st := g.session.Adventure, g.session.State
	var sb strings.Builder
	room, zone := adv.Room(st.CurrentRoom)
	if zone != nil {
		fmt.Fprintf(&sb, "## %s\n\n", zone.Name)
	}
	if room != nil {
		fmt.Fprintf(&sb, "### %s\n\n", room.Name)
		if room.ReadAloud != "" {
			fmt.Fprintf(&sb, "> %s\n\n", room.ReadAloud)
		}
		if len(room.Exits) > 0 {
			sb.WriteString("**Exits:** ")
			outs := make([]string, 0, len(room.Exits))
			for _, e := range room.Exits {
				outs = append(outs, e.To)
			}
			sb.WriteString(strings.Join(outs, ", ") + "\n\n")
		}
		if len(room.NPCIDs) > 0 {
			sb.WriteString("**NPCs here:**\n\n")
			for _, nid := range room.NPCIDs {
				if n := adv.NPC(nid); n != nil {
					fmt.Fprintf(&sb, "- %s `%s`\n", n.Name, n.ID)
				}
			}
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString("_No current room. Use /goto <room_id>._")
	}
	g.location.ParseMarkdown(sb.String())
}

func (g *gui) refreshLog() {
	if g.session == nil {
		return
	}
	var sb strings.Builder
	for _, e := range g.session.State.Log.GetLast(80) {
		fmt.Fprintf(&sb, "- `%s` %s\n", e.Timestamp.Format("15:04"), e.Message)
	}
	g.logText.ParseMarkdown(sb.String())
	g.logScroll.ScrollToBottom()
}

// showImage loads a module-relative image inline in the left image pane.
func (g *gui) showImage(relPath string) {
	if g.session == nil {
		return
	}
	abs, err := g.store.ResolveImagePath(g.session.Adventure.ID, relPath)
	if err != nil {
		dialog.ShowError(err, g.win)
		return
	}
	img := canvas.NewImageFromFile(abs)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(240, 240))
	g.imageBox.Objects = []fyne.CanvasObject{img}
	g.imageBox.Refresh()
}

func (g *gui) openZoneMap() {
	if g.session == nil {
		return
	}
	z := g.session.Adventure.Zone(g.session.State.CurrentZone)
	if z == nil || z.MapImage == "" {
		dialog.ShowInformation("No map", "This zone has no map image.", g.win)
		return
	}
	g.showImage(z.MapImage)
}

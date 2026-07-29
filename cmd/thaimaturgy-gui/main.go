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
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/theburrowhub/thaimaturgy/internal/auth"
	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/engine"
	"github.com/theburrowhub/thaimaturgy/internal/guitheme"
	"github.com/theburrowhub/thaimaturgy/internal/nativeui"
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
	entry        *widget.Entry
	transcriptMD string

	// Adventure browser + detail pane.
	navTree       *widget.Tree
	detailText    *widget.RichText
	detailImage   *fyne.Container
	detailActions *fyne.Container
	locLabel      *widget.Label
	currentUID    string
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
	g.app.Settings().SetTheme(guitheme.New())
	g.win = g.app.NewWindow("thAImaturgy — DM Oracle")
	g.win.Resize(fyne.NewSize(1200, 780))
	g.showLibrary()
	g.win.ShowAndRun()
}

// --- Library screen ------------------------------------------------------

func (g *gui) showLibrary() {
	g.session, g.oracle, g.cmd = nil, nil, nil

	title := widget.NewLabelWithStyle("🐉  thAImaturgy", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabelWithStyle("An AI oracle for the Dungeon Master", fyne.TextAlignCenter, fyne.TextStyle{Italic: true})

	importBtn := widget.NewButtonWithIcon("Import module (.tar.gz)…", theme.FolderOpenIcon(), g.importDialog)

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

	top := container.NewVBox(title, subtitle, widget.NewSeparator(), importBtn, widget.NewSeparator())
	content := container.NewBorder(top, bottom, nil, nil, widget.NewCard("Library", "", container.NewVScroll(list)))
	g.win.SetContent(content)
}

func (g *gui) importDialog() {
	go func() {
		path, ok := nativeui.OpenFile("Import adventure module",
			nativeui.Filter{Name: "Adventure module", Patterns: []string{"*.tar.gz", "*.tgz", "*.gz"}})
		if !ok {
			return
		}
		adv, err := g.store.ImportModule(path)
		if err != nil {
			nativeui.Error("Import failed", err.Error())
			return
		}
		nativeui.Info("Imported", "Imported: "+adv.Title)
		fyne.Do(func() { g.showLibrary() })
	}()
}

func (g *gui) showErr(err error) { go nativeui.Error("thAImaturgy", err.Error()) }

// --- Session screen ------------------------------------------------------

func (g *gui) startSession(advID string) {
	adv, err := g.store.LoadAdventure(advID)
	if err != nil {
		g.showErr(err)
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
		g.showErr(err)
		return
	}
	adv, err := g.store.LoadAdventure(state.AdventureID)
	if err != nil {
		g.showErr(fmt.Errorf("adventure %q not found; import it first", state.AdventureID))
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

	g.detailText = widget.NewRichTextFromMarkdown("")
	g.detailText.Wrapping = fyne.TextWrapWord
	g.detailImage = container.NewStack()
	g.detailActions = container.NewHBox()
	g.navTree = g.buildAdvTree()

	g.entry = widget.NewEntry()
	g.entry.SetPlaceHolder("Ask the oracle, or type a /command…")
	g.entry.OnSubmitted = func(s string) { g.submit(s) }
	sendBtn := widget.NewButton("Send", func() { g.submit(g.entry.Text) })

	// Left column: adventure browser (top) + session log (bottom).
	left := container.NewVSplit(
		widget.NewCard("Adventure", "", g.navTree),
		widget.NewCard("Session Log", "", g.logScroll),
	)
	left.SetOffset(0.62)

	// Center: oracle transcript + input.
	inputRow := container.NewBorder(nil, nil, nil, sendBtn, g.entry)
	center := widget.NewCard("Oracle", "", container.NewBorder(nil, inputRow, nil, nil, g.transScroll))

	// Right: detail of the selected zone/room/NPC/event/item + inline image.
	detailBody := container.NewVSplit(container.NewVScroll(g.detailText), container.NewVScroll(g.detailImage))
	detailBody.SetOffset(0.6)
	right := widget.NewCard("Detail", "", container.NewBorder(g.detailActions, nil, nil, nil, detailBody))

	body := container.NewHSplit(left, container.NewHSplit(center, right))
	body.SetOffset(0.24)

	g.locLabel = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title := widget.NewLabelWithStyle("🐉  "+adv.Title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	toolbar := container.NewVBox(
		container.NewHBox(
			widget.NewButtonWithIcon("Library", theme.NavigateBackIcon(), g.showLibrary),
			widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), g.save),
			title,
			g.locLabel,
		),
		widget.NewSeparator(),
	)

	g.win.SetContent(container.NewBorder(toolbar, nil, nil, nil, body))
	g.refreshState()
	// Show the current room in the detail pane to start.
	if g.session.State.CurrentRoom != "" {
		g.showDetail("room:" + g.session.State.CurrentZone + "::" + g.session.State.CurrentRoom)
	}
	g.win.Canvas().Focus(g.entry)
}

// --- Adventure browser ---------------------------------------------------

func (g *gui) buildAdvTree() *widget.Tree {
	t := widget.NewTree(
		func(uid widget.TreeNodeID) []widget.TreeNodeID { return g.treeChildren(uid) },
		func(uid widget.TreeNodeID) bool { return g.treeIsBranch(uid) },
		func(bool) fyne.CanvasObject { return widget.NewLabel("template") },
		func(uid widget.TreeNodeID, _ bool, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(g.treeLabel(uid))
		},
	)
	t.OnSelected = func(uid widget.TreeNodeID) { g.showDetail(uid) }
	t.OpenAllBranches()
	return t
}

func (g *gui) treeChildren(uid widget.TreeNodeID) []widget.TreeNodeID {
	adv := g.session.Adventure
	switch {
	case uid == "":
		return []widget.TreeNodeID{"zones", "npcs", "events", "items"}
	case uid == "zones":
		out := []widget.TreeNodeID{}
		for _, z := range adv.Zones {
			out = append(out, "zone:"+z.ID)
		}
		return out
	case uid == "npcs":
		out := []widget.TreeNodeID{}
		for _, n := range adv.NPCs {
			out = append(out, "npc:"+n.ID)
		}
		return out
	case uid == "events":
		out := []widget.TreeNodeID{}
		for _, e := range adv.Events {
			out = append(out, "event:"+e.ID)
		}
		return out
	case uid == "items":
		out := []widget.TreeNodeID{}
		for _, it := range adv.Items {
			out = append(out, "item:"+it.ID)
		}
		return out
	case strings.HasPrefix(uid, "zone:"):
		z := adv.Zone(strings.TrimPrefix(uid, "zone:"))
		out := []widget.TreeNodeID{}
		if z != nil {
			for _, r := range z.Rooms {
				out = append(out, "room:"+z.ID+"::"+r.ID)
			}
		}
		return out
	}
	return nil
}

func (g *gui) treeIsBranch(uid widget.TreeNodeID) bool {
	switch uid {
	case "", "zones", "npcs", "events", "items":
		return true
	}
	return strings.HasPrefix(uid, "zone:")
}

func (g *gui) treeLabel(uid widget.TreeNodeID) string {
	adv, st := g.session.Adventure, g.session.State
	switch uid {
	case "zones":
		return "🗺 Zones"
	case "npcs":
		return "🧑 NPCs"
	case "events":
		return "⚡ Events"
	case "items":
		return "💎 Items"
	}
	switch {
	case strings.HasPrefix(uid, "zone:"):
		if z := adv.Zone(strings.TrimPrefix(uid, "zone:")); z != nil {
			return "▸ " + labelOrID(z.Name, z.ID)
		}
	case strings.HasPrefix(uid, "room:"):
		_, rid := splitRoomUID(uid)
		if r, _ := adv.Room(rid); r != nil {
			marker := "·"
			if st.CurrentRoom == rid {
				marker = "▶"
			} else if st.VisitedRooms[rid] {
				marker = "✓"
			}
			return marker + " " + labelOrID(r.Name, r.ID)
		}
	case strings.HasPrefix(uid, "npc:"):
		if n := adv.NPC(strings.TrimPrefix(uid, "npc:")); n != nil {
			name := labelOrID(n.Name, n.ID)
			if s := st.KnownNPCs[n.ID]; s != nil && s.Met {
				name = "✓ " + name
			}
			return name
		}
	case strings.HasPrefix(uid, "event:"):
		if e := adv.Event(strings.TrimPrefix(uid, "event:")); e != nil {
			name := labelOrID(e.Name, e.ID)
			if st.TriggeredEvents[e.ID] {
				name = "✓ " + name
			}
			return name
		}
	case strings.HasPrefix(uid, "item:"):
		if it := adv.Item(strings.TrimPrefix(uid, "item:")); it != nil {
			return labelOrID(it.Name, it.ID)
		}
	}
	return uid
}

// --- Detail pane ---------------------------------------------------------

func (g *gui) showDetail(uid widget.TreeNodeID) {
	g.currentUID = uid
	adv := g.session.Adventure
	var text string
	var actions []fyne.CanvasObject
	image := ""

	switch {
	case strings.HasPrefix(uid, "zone:"):
		if z := adv.Zone(strings.TrimPrefix(uid, "zone:")); z != nil {
			text = engine.FormatZone(z)
			image = z.MapImage
			if z.MapImage != "" {
				actions = append(actions, widget.NewButton("Show map", func() { g.showInline(z.MapImage) }))
			}
		}
	case strings.HasPrefix(uid, "room:"):
		_, rid := splitRoomUID(uid)
		if r, _ := adv.Room(rid); r != nil {
			text = engine.FormatRoom(adv, r)
			image = r.Image
			id := r.ID
			actions = append(actions, widget.NewButton("▶ Move party here", func() { g.movePartyHere(id) }))
			if r.Image != "" {
				actions = append(actions, widget.NewButton("Show art", func() { g.showInline(r.Image) }))
			}
		}
	case strings.HasPrefix(uid, "npc:"):
		if n := adv.NPC(strings.TrimPrefix(uid, "npc:")); n != nil {
			text = engine.FormatNPC(n)
			image = n.Image
			id, name := n.ID, n.Name
			actions = append(actions, widget.NewButton("Mark as met", func() { g.markNPCMet(id, name) }))
			if n.Image != "" {
				actions = append(actions, widget.NewButton("Show art", func() { g.showInline(n.Image) }))
			}
		}
	case strings.HasPrefix(uid, "event:"):
		if e := adv.Event(strings.TrimPrefix(uid, "event:")); e != nil {
			text = engine.FormatEvent(e)
			id, name := e.ID, e.Name
			actions = append(actions, widget.NewButton("Mark triggered", func() { g.triggerEvent(id, name) }))
		}
	case strings.HasPrefix(uid, "item:"):
		if it := adv.Item(strings.TrimPrefix(uid, "item:")); it != nil {
			text = engine.FormatItem(it)
			image = it.Image
			if it.Image != "" {
				actions = append(actions, widget.NewButton("Show art", func() { g.showInline(it.Image) }))
			}
		}
	default:
		text = "Select a zone, room, NPC, event, or item on the left to view it."
	}

	g.detailText.ParseMarkdown("```\n" + text + "\n```")
	g.detailActions.Objects = actions
	g.detailActions.Refresh()
	if image != "" {
		g.showInline(image)
	} else {
		g.detailImage.Objects = nil
		g.detailImage.Refresh()
	}
}

func (g *gui) movePartyHere(roomID string) {
	res := g.cmd.Execute(engine.ParseCommand("/goto " + roomID))
	if !res.Success && res.Message != "" {
		g.showErr(fmt.Errorf("%s", res.Message))
		return
	}
	g.appendTranscript("_" + res.Message + "_")
	g.refreshState()
	g.autosave()
}

func (g *gui) markNPCMet(id, name string) {
	g.session.State.MeetNPC(id, name)
	g.session.MarkModified()
	g.refreshState()
	g.autosave()
}

func (g *gui) triggerEvent(id, name string) {
	g.session.State.TriggerEvent(id, name)
	g.session.MarkModified()
	g.refreshState()
	g.autosave()
}

func (g *gui) autosave() {
	if g.config.AutoSave && g.session != nil {
		go func() { _ = g.store.SaveSession(g.session.State) }()
	}
}

func splitRoomUID(uid string) (zoneID, roomID string) {
	body := strings.TrimPrefix(uid, "room:")
	if i := strings.Index(body, "::"); i >= 0 {
		return body[:i], body[i+2:]
	}
	return "", body
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
			g.showInline(result.UIArg)
		case "import":
			g.importDialog()
		case "load":
			g.showLibrary()
		}
	} else if result.Message != "" {
		if !result.Success {
			g.showErr(fmt.Errorf("%s", result.Message))
		} else {
			g.appendTranscript("_" + result.Message + "_")
		}
	}

	g.refreshState()
	g.refreshLog()
}

func (g *gui) ask(input string) {
	if g.oracle == nil || g.prov == nil {
		g.showErr(fmt.Errorf("no AI provider configured; set an API key"))
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
				g.showErr(resp.Error)
				return
			}
			g.appendTranscript(resp.Answer)
			g.refreshState()
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
		g.showErr(err)
		return
	}
	g.appendTranscript("_Session saved._")
}

func (g *gui) appendTranscript(md string) {
	g.transcriptMD += md + "\n\n"
	g.transcript.ParseMarkdown(g.transcriptMD)
	g.transScroll.ScrollToBottom()
}

// refreshState re-renders the browser tree, the current-location label, the log
// and (if something is selected) the detail pane after state changes.
func (g *gui) refreshState() {
	if g.session == nil {
		return
	}
	if g.navTree != nil {
		g.navTree.Refresh()
	}
	g.refreshCurrentLabel()
	g.refreshLog()
	if g.currentUID != "" {
		g.showDetail(g.currentUID)
	}
}

func (g *gui) refreshCurrentLabel() {
	if g.locLabel == nil {
		return
	}
	adv, st := g.session.Adventure, g.session.State
	room, zone := adv.Room(st.CurrentRoom)
	loc := "no room set"
	if room != nil {
		loc = room.Name
		if zone != nil {
			loc = zone.Name + " / " + room.Name
		}
	}
	g.locLabel.SetText("📍 " + loc)
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

// showInline renders a module-relative image inline in the detail image pane.
func (g *gui) showInline(relPath string) {
	if g.session == nil {
		return
	}
	abs, err := g.store.ResolveImagePath(g.session.Adventure.ID, relPath)
	if err != nil {
		g.showErr(err)
		return
	}
	img := canvas.NewImageFromFile(abs)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(280, 280))
	g.detailImage.Objects = []fyne.CanvasObject{img}
	g.detailImage.Refresh()
}

func labelOrID(name, id string) string {
	if strings.TrimSpace(name) != "" {
		return name
	}
	return id
}

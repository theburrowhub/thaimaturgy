// Command thaimaturgy is the thAImaturgy desktop app: a DM oracle and virtual-DM
// player built on the internal/ core (domain, storage, engine, providers). It
// renders maps and art inline and can host a multiplayer game over Telegram.
package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/theburrowhub/thaimaturgy/internal/auth"
	"github.com/theburrowhub/thaimaturgy/internal/bookpdf"
	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/engine"
	"github.com/theburrowhub/thaimaturgy/internal/guitheme"
	_ "github.com/theburrowhub/thaimaturgy/internal/imagefmt" // register TIFF/WebP/BMP decoders
	"github.com/theburrowhub/thaimaturgy/internal/mcpserve"
	"github.com/theburrowhub/thaimaturgy/internal/mcptools"
	"github.com/theburrowhub/thaimaturgy/internal/nativeui"
	"github.com/theburrowhub/thaimaturgy/internal/novel"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
	"github.com/theburrowhub/thaimaturgy/internal/tgbot"
)

type gui struct {
	app     fyne.App
	win     fyne.Window
	store   *storage.Storage
	config  *domain.Config
	prov    providers.Provider
	authMsg string

	// Editor view (nil until first opened); shares this window.
	editor *editor

	// Active session (nil on the library screen).
	session *domain.Session
	oracle  *engine.Oracle
	cmd     *engine.CommandHandler
	journal *storage.SessionJournal // append-only chronicle for the active session

	// Session widgets. The chat log is a column of per-message selectable Labels
	// (Fyne's only selectable/copyable text widget), each styled by role, so the
	// formatting is preserved while text can be selected and copied.
	transcriptBox *fyne.Container
	transScroll   *container.Scroll
	logText       *widget.RichText
	logScroll     *container.Scroll
	entry         *chatEntry

	// Adventure browser + detail pane.
	navTree       *widget.Tree
	detailText    *widget.RichText
	detailLinks   *fyne.Container
	detailImage   *fyne.Container
	detailActions *fyne.Container
	locLabel      *widget.Label
	currentUID    string

	// Mode toggle (Oracle ↔ Virtual DM) and the player-character panel shown in
	// virtual-DM mode in place of the adventure browser.
	modeBtn     *widget.Button
	beginBtn    *widget.Button // "Begin" — starts the game in virtual-DM mode (hidden once started)
	sendBtn     *widget.Button
	diceBtn     *widget.Button
	saveBtn     *widget.Button
	exportBtn   *widget.Button
	libraryBtn  *widget.Button
	telegramBtn *widget.Button
	busy        bool // an oracle request is in flight; block state reads/mutations from the UI

	// In-process Telegram bot hosting the current DM session (nil when stopped).
	tg        *tgbot.Bot
	tgCancel  context.CancelFunc
	hosting   bool // the Telegram bot is driving the session; GUI inputs are paused
	leftSplit *container.Split
	navCard   fyne.CanvasObject
	pcCard    fyne.CanvasObject
	pcSheet   *fyne.Container // tabletop-style character sheet (rebuilt on refresh)
	pcScroll  *container.Scroll

	// Body layout, so virtual-DM mode can hide the detail pane (spoilers): in DM
	// mode the body's trailing side is just centerCard; in oracle mode it is the
	// centerRight split (transcript + detail).
	body        *container.Split
	centerRight *container.Split
	centerCard  fyne.CanvasObject
	rightCard   fyne.CanvasObject
}

func main() {
	// When invoked as the MCP tools subprocess (by the oracle's CLI backend), serve
	// the session tools over stdio and exit — never launch the GUI.
	if len(os.Args) > 1 && os.Args[1] == mcptools.SubcommandArg {
		if err := mcpserve.RunSubcommand(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "mcp-tools:", err)
			os.Exit(1)
		}
		return
	}

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
		app:    app.NewWithID("dev.theburrowhub.thaimaturgy"),
		store:  store,
		config: config,
	}
	g.authMsg = auth.AutoConfigure(config)
	if config.RunModel != "" {
		config.Model = config.RunModel // the player/oracle may use its own model
	}
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
	g.shutdownTelegram() // stop any running host bot when leaving the session
	if g.journal != nil {
		_ = g.journal.Close()
		g.journal = nil
	}
	g.session, g.oracle, g.cmd = nil, nil, nil

	newBtn := widget.NewButtonWithIcon("New / author…", theme.DocumentCreateIcon(), g.newAuthoring)
	importBtn := widget.NewButtonWithIcon("Import (.tar.gz)…", theme.FolderOpenIcon(), g.importDialog)
	settingsBtn := widget.NewButtonWithIcon("Settings", theme.SettingsIcon(), g.showSettings)
	actions := container.NewHBox(newBtn, importBtn, settingsBtn)
	hero := modernLibraryHero("thAImaturgy", "A starlit oracle for running tabletop adventures", actions)

	list := container.NewVBox()

	advs, _ := g.store.ListAdventures()
	if len(advs) > 0 {
		list.Add(widget.NewLabelWithStyle("Adventures", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		for _, a := range advs {
			id, titleTxt := a.ID, a.Title
			play := widget.NewButton("▶  "+titleTxt, func() { g.startSession(id) })
			edit := widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), func() { g.editAdventure(id) })
			edit.Importance = widget.LowImportance
			del := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() { g.deleteAdventure(id, titleTxt) })
			del.Importance = widget.LowImportance
			list.Add(container.NewBorder(nil, nil, nil, container.NewHBox(edit, del), play))
		}
	}

	sessions, _ := g.store.ListSessions()
	if len(sessions) > 0 {
		// Most recently saved first, so the latest session is at the top.
		sort.Slice(sessions, func(i, j int) bool {
			return sessionModTime(sessions[i]).After(sessionModTime(sessions[j]))
		})
		list.Add(widget.NewLabelWithStyle("Resume session", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		for _, s := range sessions {
			name := s.Name
			label := fmt.Sprintf("↻  %s — %s   (%s)", s.Name, s.AdventureTitle, formatSessionTime(sessionModTime(s)))
			resume := widget.NewButton(label, func() { g.resumeSession(name) })
			resume.Alignment = widget.ButtonAlignLeading
			rename := widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), func() { g.renameSession(name) })
			rename.Importance = widget.LowImportance
			del := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() { g.deleteSession(name) })
			del.Importance = widget.LowImportance
			list.Add(container.NewBorder(nil, nil, nil, container.NewHBox(rename, del), resume))
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

	content := container.NewBorder(hero, bottom, nil, nil, modernPanel("Library", "Imported modules and saved sessions", container.NewVScroll(list)))
	g.win.SetContent(appShell(content))
}

// showEditor renders the editor view on the shared window.
func (g *gui) showEditor() {
	if g.editor == nil {
		g.editor = newEditor(g)
	}
	g.win.SetContent(g.editor.buildUI())
	g.editor.reload()
}

// editAdventure opens an installed adventure in the editor, editing it in place
// (saves persist to ~/.thaimaturgy/adventures/<id>, so playing reflects them).
func (g *gui) editAdventure(id string) {
	adv, err := g.store.LoadAdventure(id)
	if err != nil {
		g.showErr(err)
		return
	}
	if g.editor == nil {
		g.editor = newEditor(g)
	}
	g.editor.adv = adv
	g.editor.workingDir = g.store.AdventureDir(id)
	g.editor.dirty = false
	g.showEditor()
}

// newAuthoring opens the editor on a fresh template to author or AI-import.
func (g *gui) newAuthoring() {
	if g.editor == nil {
		g.editor = newEditor(g)
	}
	g.editor.newAdventure()
	g.showEditor()
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

// deleteAdventure removes an imported adventure (and its assets) after a native
// confirmation, then refreshes the library.
func (g *gui) deleteAdventure(id, title string) {
	go func() {
		if !nativeui.Confirm("Delete adventure", fmt.Sprintf("Delete %q and all its files?\nThis cannot be undone.", title)) {
			return
		}
		if err := g.store.DeleteAdventure(id); err != nil {
			g.showErr(err)
			return
		}
		fyne.Do(func() { g.showLibrary() })
	}()
}

// deleteSession removes a saved session after a native confirmation. Its
// append-only journal file (if any) is left in place as a record.
func (g *gui) deleteSession(name string) {
	go func() {
		if !nativeui.Confirm("Delete session", fmt.Sprintf("Delete saved session %q?\nThis cannot be undone.", name)) {
			return
		}
		if err := g.store.DeleteSession(name); err != nil {
			g.showErr(err)
			return
		}
		fyne.Do(func() { g.showLibrary() })
	}()
}

// sessionModTime extracts the save time from a SessionInfo (ModifiedAt is an
// interface value holding a time.Time).
func sessionModTime(s storage.SessionInfo) time.Time {
	if t, ok := s.ModifiedAt.(time.Time); ok {
		return t
	}
	return time.Time{}
}

// formatSessionTime renders a save time in the local zone for the library list.
func formatSessionTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Local().Format("2006-01-02 15:04")
}

// renameSession prompts for a new name and renames the saved session in place.
func (g *gui) renameSession(name string) {
	entry := widget.NewEntry()
	entry.SetText(name)

	var pop *widget.PopUp
	doRename := func() {
		newName := strings.TrimSpace(entry.Text)
		if newName == "" || newName == name {
			pop.Hide()
			return
		}
		if err := g.store.RenameSession(name, newName); err != nil {
			g.showErr(err)
			return
		}
		pop.Hide()
		g.showLibrary()
	}
	entry.OnSubmitted = func(string) { doRename() }

	save := widget.NewButton("Rename", doRename)
	save.Importance = widget.HighImportance
	cancel := widget.NewButton("Cancel", func() { pop.Hide() })

	content := container.NewVBox(
		widget.NewLabelWithStyle("Rename session", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		entry,
		container.NewHBox(save, cancel),
	)
	pop = widget.NewModalPopUp(container.NewPadded(content), g.win.Canvas())
	pop.Resize(fyne.NewSize(380, 160))
	pop.Show()
	g.win.Canvas().Focus(entry)
}

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
	// A fresh session must not inherit stale busy/hosting flags from a previous one.
	g.busy, g.hosting = false, false
	// Open an append-only journal and stream every timeline entry to it as it
	// happens, so the game is recorded continuously (not just on autosave).
	if g.journal != nil {
		_ = g.journal.Close()
	}
	g.journal, _ = g.store.OpenSessionJournal(state.Name)
	if j := g.journal; j != nil {
		state.SetLogHook(func(e domain.LogEntry) { j.Append(e) })
	}

	g.session = domain.NewSession(state, adv, g.config)
	g.oracle = engine.NewOracle(g.session, g.prov)
	g.cmd = engine.NewCommandHandler(g.session)

	// Chat log: a column of selectable Labels (one per message) inside a scroll.
	// Fyne's RichText isn't selectable, but Label is (Selectable), so this keeps
	// per-message formatting while allowing selection + copy.
	g.transcriptBox = container.NewVBox()
	g.transScroll = container.NewVScroll(g.transcriptBox)
	g.appendTranscript(fmt.Sprintf("_Running **%s**. Ask a question or type a /command._", adv.Title))
	// Restore the saved oracle chat when resuming a session so it isn't empty.
	if state.Conversation != nil {
		for _, m := range state.Conversation.Messages {
			switch m.Role {
			case domain.RoleUser:
				g.appendTranscript("**» " + m.Content + "**")
			case domain.RoleAssistant:
				if strings.TrimSpace(m.Content) != "" {
					g.appendTranscript(m.Content)
				}
			}
		}
	}

	g.logText = widget.NewRichTextFromMarkdown("")
	g.logText.Wrapping = fyne.TextWrapWord
	g.logScroll = container.NewVScroll(g.logText)

	g.detailText = widget.NewRichTextFromMarkdown("")
	g.detailText.Wrapping = fyne.TextWrapWord
	g.detailLinks = container.NewVBox()
	g.detailImage = container.NewStack()
	g.detailActions = container.NewHBox()
	g.navTree = g.buildAdvTree()

	// Multi-line input with chat-style keys: Enter submits, Ctrl/Cmd+Enter inserts
	// a newline. The Send button also submits.
	g.entry = newChatEntry(func(s string) { g.submit(s) })
	g.entry.SetMinRowsVisible(3)
	g.entry.SetPlaceHolder("Ask the oracle, or type a /command… (Enter sends, ⌘/Ctrl+Enter = newline)")
	g.sendBtn = widget.NewButton("Send", func() { g.submit(g.entry.Text) })
	g.sendBtn.Importance = widget.HighImportance
	sendBtn := g.sendBtn

	// Player-character panel, shown in virtual-DM mode in place of the adventure
	// browser (which is hidden to avoid spoilers). Rebuilt as a tabletop-style
	// sheet by refreshPCPanel.
	g.pcSheet = container.NewVBox()
	g.pcScroll = container.NewVScroll(g.pcSheet)
	g.navCard = modernPanel("Adventure", "Module browser", g.navTree)
	g.pcCard = modernPanel("Character", "Virtual DM sheet", g.pcScroll)

	// Left column: adventure browser / character sheet (top) + session log (bottom).
	g.leftSplit = container.NewVSplit(
		g.navCard,
		modernPanel("Session Log", "Timeline and table state", g.logScroll),
	)
	g.leftSplit.SetOffset(0.62)
	left := g.leftSplit

	// Center: oracle transcript + input.
	inputRow := container.NewBorder(nil, nil, nil, sendBtn, g.entry)
	g.centerCard = modernPanel("Oracle", "Ask questions, roll forward, keep the session moving", container.NewBorder(nil, inputRow, nil, nil, g.transScroll))

	// Right: detail of the selected zone/room/NPC/event/item — prose + navigable
	// links (top, scrolls together) and the inline image (bottom).
	detailContent := container.NewVScroll(container.NewVBox(g.detailText, g.detailLinks))
	detailBody := container.NewVSplit(detailContent, container.NewVScroll(g.detailImage))
	detailBody.SetOffset(0.6)
	g.rightCard = modernPanel("Detail", "Scene notes, read-aloud text and artwork", container.NewBorder(g.detailActions, nil, nil, nil, detailBody))

	g.centerRight = container.NewHSplit(g.centerCard, g.rightCard)
	g.centerRight.SetOffset(0.64)
	g.body = container.NewHSplit(left, g.centerRight)
	g.body.SetOffset(0.22)
	body := g.body

	g.locLabel = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	g.modeBtn = widget.NewButtonWithIcon("", theme.MediaPlayIcon(), g.toggleMode)
	g.beginBtn = widget.NewButtonWithIcon("Begin", theme.MediaSkipNextIcon(), g.beginGame)
	g.beginBtn.Importance = widget.HighImportance
	g.beginBtn.Hide() // shown only in virtual-DM mode before the game has begun (applyMode)
	g.diceBtn = widget.NewButton("Dice", g.showDiceRoller)
	g.telegramBtn = widget.NewButton("Telegram", g.toggleTelegram)
	g.telegramBtn.Hide() // shown only in virtual-DM mode (applyMode)
	g.libraryBtn = widget.NewButtonWithIcon("Library", theme.NavigateBackIcon(), g.showLibrary)
	g.saveBtn = widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), g.save)
	g.exportBtn = widget.NewButtonWithIcon("Export novel", theme.DocumentCreateIcon(), g.exportNovel)
	toolbar := modernSessionToolbar(adv.Title,
		g.locLabel,
		g.libraryBtn,
		g.saveBtn,
		g.exportBtn,
		g.diceBtn,
		g.modeBtn,
		g.beginBtn,
		g.telegramBtn,
	)

	g.win.SetContent(appShell(container.NewBorder(toolbar, nil, nil, nil, body)))
	g.applyMode() // reflect the (possibly restored) session mode in the UI
	g.refreshState()
	// Show the current room in the detail pane to start.
	if g.session.State.CurrentRoom != "" {
		g.showDetail("room:" + g.session.State.CurrentZone + "::" + g.session.State.CurrentRoom)
	}
	g.win.Canvas().Focus(g.entry)
	// Jump to the end of the history so a resumed session opens on the newest line.
	g.scrollTranscriptToBottom()
}

// scrollTranscriptToBottom scrolls the chat log to the newest message. It scrolls
// immediately and once more after a short delay, because at session-open time the
// content size isn't laid out yet, so the first call alone would be a no-op.
func (g *gui) scrollTranscriptToBottom() {
	s := g.transScroll
	if s == nil {
		return
	}
	s.ScrollToBottom()
	go func() {
		time.Sleep(100 * time.Millisecond)
		fyne.Do(func() { s.ScrollToBottom() })
	}()
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
	t.OnSelected = func(uid widget.TreeNodeID) {
		if g.busy || g.hosting {
			return
		}
		g.showDetail(uid)
	}
	t.OpenAllBranches()
	return t
}

func (g *gui) treeChildren(uid widget.TreeNodeID) []widget.TreeNodeID {
	adv := g.session.Adventure
	switch {
	case uid == "":
		return []widget.TreeNodeID{"about", "zones", "npcs", "events", "items", "tables"}
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
	case uid == "tables":
		out := []widget.TreeNodeID{}
		for _, t := range adv.Tables {
			out = append(out, "table:"+t.ID)
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
	case "", "zones", "npcs", "events", "items", "tables":
		return true
	}
	return strings.HasPrefix(uid, "zone:")
}

func (g *gui) treeLabel(uid widget.TreeNodeID) string {
	adv, st := g.session.Adventure, g.session.State
	switch uid {
	case "about":
		return "Adventure"
	case "zones":
		return "Zones"
	case "npcs":
		return "NPCs"
	case "events":
		return "Events"
	case "items":
		return "Items"
	case "tables":
		return "Random tables"
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
				marker = "● Party"
			} else if st.VisitedRooms[rid] {
				marker = "✓"
			}
			return marker + "  " + labelOrID(r.Name, r.ID)
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
	case strings.HasPrefix(uid, "table:"):
		if t := adv.Table(strings.TrimPrefix(uid, "table:")); t != nil {
			return labelOrID(t.Name, t.ID)
		}
	}
	return uid
}

// --- Detail pane ---------------------------------------------------------

func (g *gui) showDetail(uid widget.TreeNodeID) {
	g.currentUID = uid
	adv := g.session.Adventure
	var md string
	var groups []navGroup
	var actions []fyne.CanvasObject
	var images []string // resolved relative paths (image_ids + legacy paths)

	switch {
	case uid == "about":
		md = adventureMarkdown(adv)
	case strings.HasPrefix(uid, "zone:"):
		if z := adv.Zone(strings.TrimPrefix(uid, "zone:")); z != nil {
			md = zoneMarkdown(z)
			groups = zoneGroups(adv, z)
			images = adv.ZoneImages(z)
		}
	case strings.HasPrefix(uid, "room:"):
		_, rid := splitRoomUID(uid)
		if r, _ := adv.Room(rid); r != nil {
			md = roomMarkdown(r)
			groups = roomGroups(adv, r)
			images = adv.RoomImages(r)
			id := r.ID
			move := widget.NewButton("Move party here", func() { g.movePartyHere(id) })
			move.Importance = widget.HighImportance
			actions = append(actions, move)
		}
	case strings.HasPrefix(uid, "npc:"):
		if n := adv.NPC(strings.TrimPrefix(uid, "npc:")); n != nil {
			md = npcMarkdown(n)
			groups = npcGroups(adv, n)
			images = adv.NPCImages(n)
			id, name := n.ID, n.Name
			actions = append(actions, widget.NewButton("Mark as met", func() { g.markNPCMet(id, name) }))
		}
	case strings.HasPrefix(uid, "event:"):
		if e := adv.Event(strings.TrimPrefix(uid, "event:")); e != nil {
			md = eventMarkdown(e)
			id, name := e.ID, e.Name
			actions = append(actions, widget.NewButton("Mark triggered", func() { g.triggerEvent(id, name) }))
		}
	case strings.HasPrefix(uid, "item:"):
		if it := adv.Item(strings.TrimPrefix(uid, "item:")); it != nil {
			md = itemMarkdown(it)
			images = adv.ItemImages(it)
		}
	case strings.HasPrefix(uid, "table:"):
		if t := adv.Table(strings.TrimPrefix(uid, "table:")); t != nil {
			md = "## " + labelOrID(t.Name, t.ID) + "\n\n" + engine.TableMarkdown(t)
			if len(t.Rows) > 0 {
				id := t.ID
				actions = append(actions, widget.NewButton("🎲 Roll on table", func() { g.rollTable(id) }))
			}
		}
	default:
		md = "_Select a zone, room, NPC, event, item or table on the left to view it._"
	}

	// One "Show" button per referenced image; show the first inline by default.
	for i, p := range images {
		p := p
		label := "Show image"
		if len(images) > 1 {
			label = fmt.Sprintf("Image %d", i+1)
		}
		actions = append(actions, widget.NewButton(label, func() { g.showInline(p) }))
	}

	g.detailText.ParseMarkdown(md)
	g.setDetailLinks(groups)
	g.detailActions.Objects = actions
	g.detailActions.Refresh()
	if len(images) > 0 {
		g.showInline(images[0])
	} else {
		g.detailImage.Objects = nil
		g.detailImage.Refresh()
	}
}

// setDetailLinks renders the navigable "Go to" reference groups as tappable
// hyperlinks that jump to the target node in the browser and detail pane.
func (g *gui) setDetailLinks(groups []navGroup) {
	g.detailLinks.Objects = nil
	for _, grp := range groups {
		if len(grp.refs) == 0 {
			continue
		}
		g.detailLinks.Add(widget.NewLabelWithStyle(grp.title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		for _, ref := range grp.refs {
			target := ref.uid
			link := widget.NewHyperlink("› "+ref.label, nil)
			link.OnTapped = func() { g.selectNode(target) }
			g.detailLinks.Add(link)
		}
	}
	g.detailLinks.Refresh()
}

// selectNode navigates the browser to uid and shows its detail, revealing the
// containing zone branch for rooms.
func (g *gui) selectNode(uid widget.TreeNodeID) {
	if strings.HasPrefix(uid, "room:") {
		if zid, _ := splitRoomUID(uid); zid != "" {
			g.navTree.OpenBranch("zones")
			g.navTree.OpenBranch("zone:" + zid)
		}
	}
	g.navTree.Select(uid)
	g.navTree.ScrollTo(uid)
	g.showDetail(uid)
}

func (g *gui) movePartyHere(roomID string) {
	if g.busy || g.hosting {
		return
	}
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
	if g.busy || g.hosting {
		return
	}
	g.session.State.MeetNPC(id, name)
	g.session.MarkModified()
	g.refreshState()
	g.autosave()
}

func (g *gui) triggerEvent(id, name string) {
	if g.busy || g.hosting {
		return
	}
	g.session.State.TriggerEvent(id, name)
	g.session.MarkModified()
	g.refreshState()
	g.autosave()
}

// rollTable rolls on a table, shows the result in the oracle transcript, and
// records it in the session log.
func (g *gui) rollTable(id string) {
	if g.busy || g.hosting {
		return
	}
	t := g.session.Adventure.Table(id)
	if t == nil {
		return
	}
	roll, row := engine.RollTable(t)
	result := engine.RowText(row)
	if result == "" {
		result = "(no matching row)"
	}
	name := labelOrID(t.Name, t.ID)
	g.appendTranscript(fmt.Sprintf("**🎲 %s — rolled %d:** %s", name, roll, result))
	g.session.State.AddNote(fmt.Sprintf("Rolled %s (%d): %s", name, roll, result))
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
	if g.busy || g.hosting { // an oracle request is in flight; ignore further submissions
		return
	}
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
		// Two messages so only the command echo is bold, not the whole output.
		g.appendTranscript("**» " + raw + "**")
		g.appendTranscript(result.Response)
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
		case "mode":
			g.onModeChanged()
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

// setBusy enables/disables the session input controls. While an oracle request
// is in flight the controls are disabled so the user can't mutate the session
// state (dice roll, mode toggle, another query) concurrently with the oracle
// goroutine that is mutating it.
func (g *gui) setBusy(busy bool) {
	g.busy = busy
	toggle := func(b *widget.Button) {
		if b == nil {
			return
		}
		if busy {
			b.Disable()
		} else {
			b.Enable()
		}
	}
	// Disable every control that reads or mutates session state, including Save /
	// Export (which serialize it) and Library (which tears it down). Tree selection
	// and detail-action buttons are gated separately via the g.busy flag.
	toggle(g.sendBtn)
	toggle(g.diceBtn)
	toggle(g.modeBtn)
	toggle(g.saveBtn)
	toggle(g.exportBtn)
	toggle(g.libraryBtn)
	toggle(g.telegramBtn)
	if g.entry != nil {
		if busy {
			g.entry.Disable()
		} else {
			g.entry.Enable()
		}
	}
}

func (g *gui) ask(input string) {
	if g.oracle == nil || g.prov == nil {
		g.showErr(fmt.Errorf("no AI provider configured; set an API key"))
		return
	}
	g.appendTranscript("_Consulting the oracle…_")
	if g.journal != nil {
		g.journal.Note("oracle-q", input)
	}
	timeout := time.Duration(g.config.RequestTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	g.setBusy(true)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		resp := g.oracle.Ask(ctx, input)
		fyne.Do(func() {
			g.setBusy(false)
			if resp.Error != nil {
				g.showErr(resp.Error)
				return
			}
			g.appendTranscript(resp.Answer)
			if g.journal != nil {
				g.journal.Note("oracle-a", resp.Answer)
			}
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

// exportNovel novelizes the session (via the LLM) and saves it as Markdown or a
// print-ready PDF, chosen from a native dialog.
func (g *gui) exportNovel() {
	if g.session == nil {
		return
	}
	if g.prov == nil {
		g.showErr(fmt.Errorf("no AI provider configured; set an API key or use the Claude CLI backend"))
		return
	}
	adv, st, model := g.session.Adventure, g.session.State, g.config.Model
	subtitle := "A novelization of the play session"
	if strings.HasPrefix(strings.ToLower(adv.Language), "es") {
		subtitle = "Una novelización de la partida"
	}
	go func() {
		choice := nativeui.Choice("Export novel", "Export the session as a novel — which format?", "Markdown (.md)", "PDF (.pdf)")
		if choice == 0 {
			return
		}
		fyne.Do(func() { g.appendTranscript("_Writing the novel from the session… this can take a minute._") })

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		md, err := novel.Generate(ctx, g.prov, model, adv, st)
		if err != nil {
			g.showErr(fmt.Errorf("novel generation failed: %w", err))
			return
		}

		switch choice {
		case 1: // Markdown
			dest, ok := nativeui.SaveFile("Save novel", adv.ID+"-novel.md",
				nativeui.Filter{Name: "Markdown", Patterns: []string{"*.md"}})
			if !ok {
				return
			}
			if err := os.WriteFile(dest, []byte(md), 0644); err != nil {
				g.showErr(err)
				return
			}
			nativeui.Info("Novel exported", "Saved:\n"+dest)
			fyne.Do(func() { g.appendTranscript("_Novel exported to " + dest + "_") })
		case 2: // PDF
			dest, ok := nativeui.SaveFile("Save novel", adv.ID+"-novel.pdf",
				nativeui.Filter{Name: "PDF", Patterns: []string{"*.pdf"}})
			if !ok {
				return
			}
			pdfBytes, err := bookpdf.FromMarkdown(adv.Title, subtitle, md)
			if err != nil {
				g.showErr(err)
				return
			}
			if err := os.WriteFile(dest, pdfBytes, 0644); err != nil {
				g.showErr(err)
				return
			}
			nativeui.Info("Novel exported", "Saved:\n"+dest)
			fyne.Do(func() { g.appendTranscript("_Novel exported to " + dest + "_") })
		}
	}()
}

// appendTranscript adds one chat message to the log as a selectable Label, styled
// by role (bold question, italic status, plain narration), and scrolls to it.
func (g *gui) appendTranscript(md string) {
	if g.transcriptBox == nil {
		return
	}
	style := fyne.TextStyle{}
	t := strings.TrimSpace(md)
	switch {
	case strings.HasPrefix(t, "**» "): // the DM/player's own line
		style.Bold = true
	case len(t) >= 2 && strings.HasPrefix(t, "_") && strings.HasSuffix(t, "_"): // status line
		style.Italic = true
	}
	lbl := widget.NewLabelWithStyle(cleanMarkdown(md), fyne.TextAlignLeading, style)
	lbl.Wrapping = fyne.TextWrapWord
	lbl.Selectable = true // enables mouse selection + copy (Cmd/Ctrl+C)
	g.transcriptBox.Add(lbl)
	g.transScroll.ScrollToBottom()
}

// modeIsDM reports whether the active session is running in virtual-DM mode.
func (g *gui) modeIsDM() bool {
	return g.session != nil && g.session.State.EffectiveMode() == domain.ModeVirtualDM
}

// applyMode reflects the session's current mode in the UI: the toggle button
// label, the input placeholder, and whether the left panel shows the adventure
// browser (oracle) or the player-character sheet (virtual DM, tree hidden to
// avoid spoilers).
func (g *gui) applyMode() {
	if g.session == nil {
		return
	}
	dm := g.modeIsDM()
	if g.modeBtn != nil {
		if dm {
			g.modeBtn.SetText("Mode: Virtual DM")
			g.modeBtn.SetIcon(theme.MediaReplayIcon())
		} else {
			g.modeBtn.SetText("Mode: Oracle")
			g.modeBtn.SetIcon(theme.MediaPlayIcon())
		}
	}
	if g.entry != nil {
		if dm {
			g.entry.SetPlaceHolder("What do you do?  (Enter sends · ⌘/Ctrl+Enter = newline)")
		} else {
			g.entry.SetPlaceHolder("Ask the oracle, or type a /command…  (Enter sends · ⌘/Ctrl+Enter = newline)")
		}
	}
	if g.leftSplit != nil {
		if dm {
			g.leftSplit.Leading = g.pcCard
		} else {
			g.leftSplit.Leading = g.navCard
		}
		g.leftSplit.Refresh()
	}
	// In virtual-DM mode hide the detail pane (spoilers) so only the character
	// sheet + log (left) and the DM narration (center) remain.
	if g.body != nil {
		if dm {
			g.body.Trailing = g.centerCard
		} else {
			g.body.Trailing = g.centerRight
		}
		g.body.Refresh()
	}
	// modernPanel titles are static; mode is reflected by the toolbar button and placeholder.
	// The Telegram host button is only meaningful in virtual-DM mode; leaving DM
	// mode stops any running bot.
	if g.telegramBtn != nil {
		if dm {
			g.telegramBtn.Show()
		} else {
			g.stopTelegram("_📴 Telegram bot stopped (left DM mode)._")
			g.telegramBtn.Hide()
		}
	}
	// The Begin button appears only in virtual-DM mode while the game hasn't begun.
	if g.beginBtn != nil {
		if dm && !g.session.State.GameStarted() {
			g.beginBtn.Show()
		} else {
			g.beginBtn.Hide()
		}
	}
	g.refreshPCPanel()
}

// refreshPCPanel rebuilds the tabletop-style party sheet shown in virtual-DM
// mode: an "Edit party…" button (create/adjust with AI) followed by one sheet per
// party member.
func (g *gui) refreshPCPanel() {
	if g.pcSheet == nil || g.session == nil {
		return
	}
	objs := []fyne.CanvasObject{
		widget.NewButtonWithIcon("Edit party…", theme.DocumentCreateIcon(), g.showPartyEditor),
		widget.NewSeparator(),
	}
	party := g.session.State.PartySnapshot()
	if len(party) == 0 {
		objs = append(objs,
			widget.NewLabelWithStyle("No party yet.", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}),
			wrapLabel("Switch to Virtual DM (a default party is created) or use “Edit party…”."))
	}
	for i := range party {
		if i > 0 {
			objs = append(objs, widget.NewSeparator())
		}
		objs = append(objs, buildPCSheet(&party[i])...)
	}
	g.pcSheet.Objects = objs
	g.pcSheet.Refresh()
	if g.pcScroll != nil {
		g.pcScroll.Refresh()
	}
}

// toggleMode flips the session mode (toolbar button); onModeChanged then syncs
// the UI. The /mode command mutates the mode in the handler and reaches the same
// onModeChanged via the "mode" UI action, so both paths behave identically.
func (g *gui) toggleMode() {
	if g.session == nil {
		return
	}
	g.session.State.ToggleMode()
	g.onModeChanged()
}

// onModeChanged reconciles the UI with the session's (already updated) mode:
// swaps panels, ensures a player character exists in virtual-DM mode, narrates
// the opening scene the first time, and posts a status line.
func (g *gui) onModeChanged() {
	if g.session == nil {
		return
	}
	dm := g.modeIsDM()
	if dm {
		g.session.State.EnsureParty()
	}
	g.applyMode()
	g.refreshLog()
	if dm {
		g.appendTranscript("_🎲 **Virtual DM mode** — the AI runs the game for your party._")
		if !g.session.State.GameStarted() {
			// The game does NOT start automatically: the DM presses **Begin** (or a
			// player runs /begin over Telegram) to have the AI narrate the opening
			// scene. This keeps a session un-started so a Telegram host can begin it.
			g.appendTranscript("_Press **Begin** to start the game and have the DM set the opening scene — or host on Telegram and let a player run /begin._")
		}
	} else {
		g.appendTranscript("_📖 **Oracle mode** — the AI assists you as the human DM again._")
	}
	g.autosave()
}

// beginGame starts a virtual-DM game on demand (the Begin toolbar button): the AI
// narrates the opening scene, exactly once. Mirrors Telegram's /begin. Before this
// the game stays un-started so a Telegram host can be the one to begin it.
func (g *gui) beginGame() {
	if g.session == nil || !g.modeIsDM() {
		return
	}
	if g.busy {
		g.showErr(fmt.Errorf("wait for the current oracle turn to finish"))
		return
	}
	if g.session.State.GameStarted() {
		return
	}
	if g.oracle == nil {
		g.showErr(fmt.Errorf("no AI provider configured; set an API key in Settings"))
		return
	}
	g.session.State.StartGame()
	g.applyMode() // hides the Begin button now that the game has started
	g.ask(domain.DMKickoffPrompt(g.config.Language))
	g.autosave()
}

// toggleTelegram starts or stops the in-process Telegram bot hosting the current
// virtual-DM session. Available only in virtual-DM mode.
func (g *gui) toggleTelegram() {
	if g.session == nil {
		return
	}
	if g.tg != nil {
		g.stopTelegram("_📴 Telegram bot stopped._")
		return
	}
	if g.busy {
		g.showErr(fmt.Errorf("wait for the current oracle turn to finish before hosting"))
		return
	}
	if !g.modeIsDM() {
		g.showErr(fmt.Errorf("switch to Virtual DM mode to host a Telegram game"))
		return
	}
	token := strings.TrimSpace(g.config.TelegramToken)
	if token == "" {
		g.showErr(fmt.Errorf("no Telegram bot token configured — set it in Settings"))
		return
	}
	if g.oracle == nil {
		g.showErr(fmt.Errorf("no AI provider configured; set an API key"))
		return
	}
	g.session.State.EnsureParty()
	bot, err := tgbot.New(g.store, g.session, g.oracle, tgbot.Options{
		Token:   token,
		ChatID:  g.config.TelegramChatID,
		OnEvent: func(line string) { fyne.Do(func() { g.onTelegramEvent(line) }) },
	})
	if err != nil {
		g.showErr(err)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	g.tg, g.tgCancel = bot, cancel
	go bot.Run(ctx)
	g.setHosting(true)
	g.telegramBtn.SetText("Telegram: on")
	g.appendTranscript(fmt.Sprintf("_📡 Hosting on Telegram as @%s. Players: /party, /pick, /do, then /dm. GUI input is paused while hosting._", bot.Username()))
}

// shutdownTelegram stops the bot without touching the session UI (used when the
// session is being torn down).
func (g *gui) shutdownTelegram() {
	g.hosting = false // always clear the flag, even if the bot wasn't running
	if g.tg == nil {
		return
	}
	if g.tgCancel != nil {
		g.tgCancel()
	}
	g.tg.Stop()
	g.tg, g.tgCancel = nil, nil
}

// stopTelegram stops the bot and restores the GUI, posting a note.
func (g *gui) stopTelegram(note string) {
	if g.tg == nil {
		return
	}
	g.shutdownTelegram()
	g.setHosting(false)
	if g.telegramBtn != nil {
		g.telegramBtn.SetText("Telegram")
	}
	if note != "" {
		g.appendTranscript(note)
	}
}

// setHosting pauses every GUI path that mutates the session while the Telegram
// bot drives the game, so the two don't run the oracle on the same session
// concurrently. Library and the Telegram button stay enabled (to stop/leave);
// tree and detail actions are gated via the hosting flag (see their guards).
func (g *gui) setHosting(on bool) {
	g.hosting = on
	for _, b := range []*widget.Button{g.sendBtn, g.diceBtn, g.modeBtn, g.saveBtn, g.exportBtn} {
		if b == nil {
			continue
		}
		if on {
			b.Disable()
		} else {
			b.Enable()
		}
	}
	if g.entry != nil {
		if on {
			g.entry.Disable()
		} else {
			g.entry.Enable()
		}
	}
}

// onTelegramEvent mirrors bot activity into the GUI transcript and refreshes the
// panels (the bot mutates the shared session from its own goroutine).
func (g *gui) onTelegramEvent(line string) {
	g.appendTranscript("· " + line)
	g.refreshState()
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
	if g.modeIsDM() {
		g.refreshPCPanel()
	}
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
	g.locLabel.SetText("● " + loc)
}

func (g *gui) refreshLog() {
	if g.session == nil {
		return
	}
	var sb strings.Builder
	for _, e := range g.session.State.RecentLog(80) {
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

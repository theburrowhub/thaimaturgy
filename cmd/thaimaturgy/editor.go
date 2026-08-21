// Command thaimaturgy-edit is a desktop editor for authoring thAImaturgy
// adventure modules. It edits an in-memory domain.Adventure through forms,
// imports images into the module's assets/, validates with the same rules the
// player uses, and packages the result into an importable .tar.gz.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/theburrowhub/thaimaturgy/internal/aibuild"
	"github.com/theburrowhub/thaimaturgy/internal/auth"
	"github.com/theburrowhub/thaimaturgy/internal/bookpdf"
	"github.com/theburrowhub/thaimaturgy/internal/dmbook"
	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/ingest"
	"github.com/theburrowhub/thaimaturgy/internal/nativeui"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

type editor struct {
	app   fyne.App
	win   fyne.Window
	store *storage.Storage

	// Navigation callbacks into the unified app.
	onBack func()       // return to the library
	onPlay func(string) // start a play session for the given adventure id

	config *domain.Config
	prov   providers.Provider
	// visionProv handles image curation during import when the primary backend is
	// text-only (e.g. the Claude CLI). nil when the primary backend already sees
	// images or no image-capable credential is available.
	visionProv providers.Provider
	model      string
	authMsg    string

	adv        *domain.Adventure
	workingDir string // holds adventure.json + assets/
	dirty      bool

	// editRev bumps on every content edit (via markDirty); a save snapshots it and
	// only clears the dirty flag if it is unchanged on completion, so edits made
	// after the snapshot aren't marked as saved. saving serializes remote saves so
	// an older async PUT can't land after (and overwrite) a newer one.
	editRev int
	saving  bool

	// Remote editing (#144): when saveHook is set the editor persists over the HTTP
	// API (a server adventure) instead of to workingDir on disk, and remoteMode
	// relaxes on-disk image validation (assets live on the server, not locally).
	// saveHook runs its network I/O OFF the UI thread and reports the result by
	// calling done on the UI thread (via fyne.Do), so a stalled server never
	// freezes the GUI. It is invoked only from doSave.
	saveHook   func(done func(error))
	remoteMode bool

	// translate toggles AI translation of the imported module into the configured
	// import language. Off by default: import in the source document's language.
	translate bool

	nav        *widget.Tree
	formHost   *fyne.Container
	status     *widget.Label
	currentUID string
}

// useLocalBackend resets the (shared, reused) editor to persist to the on-disk
// store, clearing any remote-editing state left over from a previous remote
// session. Without this, reusing the editor for a local adventure would still see
// saveHook set and PUT the local adventure's contents to the last remote one.
func (e *editor) useLocalBackend(onBack func(), onPlay func(string)) {
	e.remoteMode = false
	e.saveHook = nil
	// NOTE: do NOT clear e.saving here. A remote save may still be in flight; the
	// serialization guard must stay set until that save's own completion clears it,
	// so navigating through a local editor can't let a second remote save overlap
	// the first. The guard is bounded by the save's context timeout.
	e.onBack = onBack
	e.onPlay = onPlay
}

// errSaveSuperseded marks a save whose result no longer applies to what's on
// screen (the editor navigated away, or newer edits arrived after the snapshot),
// so continuations like playCurrent don't act on stale server state.
var errSaveSuperseded = fmt.Errorf("save superseded by a newer edit or navigation")

// markDirty flags unsaved changes and advances the edit revision, so an in-flight
// save that snapshotted an earlier revision won't clear the dirty flag on completion.
func (e *editor) markDirty() {
	e.dirty = true
	e.editRev++
}

// playCurrent saves the current adventure, ensures it's installed in the library,
// and switches to a play session for it.
func (e *editor) playCurrent() {
	if e.adv == nil {
		return
	}
	e.doSave(func(err error) {
		if err != nil {
			return // doSave already surfaced it
		}
		// Remote mode already persisted to the server; there is no local working
		// copy to install into the on-disk library.
		if !e.remoteMode {
			if err := e.installToLibrary(); err != nil {
				e.showErr(err)
				return
			}
		}
		if e.onPlay != nil {
			e.onPlay(e.adv.ID)
		}
	})
}

// installToLibrary makes the current working module available in the player's
// library (~/.thaimaturgy/adventures/<id>). If the editor is already working in
// place on the installed copy, this is a no-op; otherwise it packages the working
// dir and imports it, then points the editor at the installed copy so subsequent
// saves are in place.
func (e *editor) installToLibrary() error {
	if e.adv == nil || e.store == nil {
		return nil
	}
	dest := e.store.AdventureDir(e.adv.ID)
	if pathsEqual(e.workingDir, dest) {
		return nil // already editing the installed copy
	}
	tgz, err := os.CreateTemp("", "thaim-install-*.tar.gz")
	if err != nil {
		return err
	}
	tgzPath := tgz.Name()
	_ = tgz.Close()
	defer os.Remove(tgzPath)
	_ = os.Remove(tgzPath) // PackageModule recreates it
	if err := storage.PackageModule(e.workingDir, tgzPath); err != nil {
		return err
	}
	if _, err := e.store.ImportModule(tgzPath); err != nil {
		return err
	}
	e.workingDir = dest // future saves go straight to the installed copy
	e.dirty = false
	return nil
}

func pathsEqual(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	ca, _ := filepath.Abs(a)
	cb, _ := filepath.Abs(b)
	return filepath.Clean(ca) == filepath.Clean(cb)
}

// newEditor builds the editor as a view of the unified app, sharing the window
// and store. It uses its own config copy with the edit-model override (which may
// differ from the play/oracle model), and its own provider + vision provider.
func newEditor(g *gui) *editor {
	cfg := domain.DefaultConfig()
	if loaded, err := g.store.LoadConfig(); err == nil && loaded != nil {
		cfg = loaded
	}
	auth.AutoConfigure(cfg)
	if cfg.EditModel != "" {
		cfg.Model = cfg.EditModel
	}
	e := &editor{
		app:        g.app,
		win:        g.win,
		store:      g.store,
		config:     cfg,
		prov:       providers.New(cfg),
		model:      cfg.Model,
		visionProv: buildVisionProvider(cfg),
		onBack:     g.showLibrary,
		onPlay:     func(id string) { g.startSession(id) },
	}
	return e
}

// buildVisionProvider returns an image-capable provider used for import vision
// curation when the primary backend can't see images (e.g. the Claude CLI). It
// reuses a detected Anthropic login/key. Returns nil when the primary backend
// already handles vision or no image-capable credential is available.
func buildVisionProvider(cfg *domain.Config) providers.Provider {
	if p := providers.New(cfg); p != nil && p.SupportsVision() {
		return nil // primary backend already handles vision
	}
	vcfg := *cfg
	vcfg.Provider = domain.ProviderAnthropic
	vcfg.AnthropicOAuthToken = ""
	vcfg.AnthropicAPIKey = ""
	auth.AutoConfigure(&vcfg) // populate an Anthropic credential if one is detected locally
	return providers.New(&vcfg)
}

// --- UI scaffold ---------------------------------------------------------

func (e *editor) buildUI() fyne.CanvasObject {
	translateCheck := widget.NewCheck(e.translateLabel(), func(v bool) { e.translate = v })
	translateCheck.SetChecked(e.translate)

	toolbar := container.NewHBox(
		widget.NewButton("← Library", func() {
			if e.onBack != nil {
				e.onBack()
			}
		}),
		widget.NewButton("Save", func() { e.doSave(nil) }),
		widget.NewButton("▶ Play", e.playCurrent),
		widget.NewButton("Import…", e.importDialog),
		translateCheck,
		widget.NewButton("Validate", e.validate),
		widget.NewButton("Export .tar.gz…", e.saveDialog),
		widget.NewButton("DM book…", e.exportDMBook),
	)

	e.status = widget.NewLabel("")
	e.formHost = container.NewStack()

	navTools := container.NewHBox(
		widget.NewButton("+Zone", e.addZone),
		widget.NewButton("+Room", e.addRoom),
		widget.NewButton("+NPC", e.addNPC),
		widget.NewButton("+Event", e.addEvent),
		widget.NewButton("+Item", e.addItem),
		widget.NewButton("+Table", e.addTable),
		widget.NewButton("+Image", e.addImage),
		widget.NewButton("Delete", e.deleteSelected),
	)
	e.nav = e.buildTree()
	left := widget.NewCard("Adventure", "", container.NewBorder(navTools, nil, nil, nil, e.nav))
	form := widget.NewCard("Editor", "", container.NewVScroll(e.formHost))

	split := container.NewHSplit(left, form)
	split.SetOffset(0.28)

	title := widget.NewLabelWithStyle("⚔  thAImaturgy — Module Editor", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	header := container.NewVBox(title, toolbar, widget.NewSeparator())

	return container.NewBorder(header, e.status, nil, nil, split)
}

func (e *editor) buildTree() *widget.Tree {
	t := widget.NewTree(
		func(uid widget.TreeNodeID) []widget.TreeNodeID { return e.childUIDs(uid) },
		func(uid widget.TreeNodeID) bool { return e.isBranch(uid) },
		func(bool) fyne.CanvasObject { return widget.NewLabel("template") },
		func(uid widget.TreeNodeID, _ bool, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(e.nodeLabel(uid))
		},
	)
	t.OnSelected = func(uid widget.TreeNodeID) { e.showForm(uid) }
	t.OpenAllBranches()
	return t
}

func (e *editor) childUIDs(uid widget.TreeNodeID) []widget.TreeNodeID {
	switch {
	case uid == "":
		return []widget.TreeNodeID{"meta", "zones", "npcs", "events", "items", "tables", "images"}
	case uid == "images":
		var out []widget.TreeNodeID
		for _, img := range e.adv.Images {
			out = append(out, "img:"+img.ID)
		}
		return out
	case uid == "zones":
		var out []widget.TreeNodeID
		for _, z := range e.adv.Zones {
			out = append(out, "zone:"+z.ID)
		}
		return out
	case uid == "npcs":
		var out []widget.TreeNodeID
		for _, n := range e.adv.NPCs {
			out = append(out, "npc:"+n.ID)
		}
		return out
	case uid == "events":
		var out []widget.TreeNodeID
		for _, ev := range e.adv.Events {
			out = append(out, "event:"+ev.ID)
		}
		return out
	case uid == "items":
		var out []widget.TreeNodeID
		for _, it := range e.adv.Items {
			out = append(out, "item:"+it.ID)
		}
		return out
	case uid == "tables":
		var out []widget.TreeNodeID
		for _, t := range e.adv.Tables {
			out = append(out, "table:"+t.ID)
		}
		return out
	case strings.HasPrefix(uid, "zone:"):
		zid := strings.TrimPrefix(uid, "zone:")
		z := e.adv.Zone(zid)
		var out []widget.TreeNodeID
		if z != nil {
			for _, r := range z.Rooms {
				out = append(out, "room:"+zid+"::"+r.ID)
			}
		}
		return out
	}
	return nil
}

func (e *editor) isBranch(uid widget.TreeNodeID) bool {
	switch uid {
	case "", "zones", "npcs", "events", "items", "tables", "images":
		return true
	}
	return strings.HasPrefix(uid, "zone:")
}

func (e *editor) nodeLabel(uid widget.TreeNodeID) string {
	switch uid {
	case "meta":
		return "📖 Adventure"
	case "zones":
		return "🗺 Zones"
	case "npcs":
		return "🧑 NPCs"
	case "events":
		return "⚡ Events"
	case "items":
		return "💎 Items"
	case "tables":
		return "🎲 Tables"
	case "images":
		return "🖼 Images"
	}
	switch {
	case strings.HasPrefix(uid, "zone:"):
		if z := e.adv.Zone(strings.TrimPrefix(uid, "zone:")); z != nil {
			return "▸ " + labelOrID(z.Name, z.ID)
		}
	case strings.HasPrefix(uid, "room:"):
		_, rid := parseRoomUID(uid)
		if r, _ := e.adv.Room(rid); r != nil {
			return "· " + labelOrID(r.Name, r.ID)
		}
	case strings.HasPrefix(uid, "npc:"):
		if n := e.adv.NPC(strings.TrimPrefix(uid, "npc:")); n != nil {
			return labelOrID(n.Name, n.ID)
		}
	case strings.HasPrefix(uid, "event:"):
		if ev := e.adv.Event(strings.TrimPrefix(uid, "event:")); ev != nil {
			return labelOrID(ev.Name, ev.ID)
		}
	case strings.HasPrefix(uid, "item:"):
		if it := e.adv.Item(strings.TrimPrefix(uid, "item:")); it != nil {
			return labelOrID(it.Name, it.ID)
		}
	case strings.HasPrefix(uid, "table:"):
		if t := e.adv.Table(strings.TrimPrefix(uid, "table:")); t != nil {
			return labelOrID(t.Name, t.ID)
		}
	case strings.HasPrefix(uid, "img:"):
		if img := e.adv.ImageByID(strings.TrimPrefix(uid, "img:")); img != nil {
			label := img.ID
			if img.Kind != "" {
				label += " (" + img.Kind + ")"
			}
			return label
		}
	}
	return uid
}

// showForm renders the form for the selected node into the form host.
func (e *editor) showForm(uid widget.TreeNodeID) {
	e.currentUID = uid
	var form fyne.CanvasObject
	switch {
	case uid == "meta":
		form = e.metaForm()
	case strings.HasPrefix(uid, "zone:"):
		if z := e.adv.Zone(strings.TrimPrefix(uid, "zone:")); z != nil {
			form = e.zoneForm(z)
		}
	case strings.HasPrefix(uid, "room:"):
		_, rid := parseRoomUID(uid)
		if r, _ := e.adv.Room(rid); r != nil {
			form = e.roomForm(r)
		}
	case strings.HasPrefix(uid, "npc:"):
		if n := e.adv.NPC(strings.TrimPrefix(uid, "npc:")); n != nil {
			form = e.npcForm(n)
		}
	case strings.HasPrefix(uid, "event:"):
		if ev := e.adv.Event(strings.TrimPrefix(uid, "event:")); ev != nil {
			form = e.eventForm(ev)
		}
	case strings.HasPrefix(uid, "item:"):
		if it := e.adv.Item(strings.TrimPrefix(uid, "item:")); it != nil {
			form = e.itemForm(it)
		}
	case strings.HasPrefix(uid, "table:"):
		if t := e.adv.Table(strings.TrimPrefix(uid, "table:")); t != nil {
			form = e.tableForm(t)
		}
	case strings.HasPrefix(uid, "img:"):
		if img := e.adv.ImageByID(strings.TrimPrefix(uid, "img:")); img != nil {
			form = e.imageForm(img)
		}
	default:
		form = widget.NewLabel("Select an item to edit, or use the + buttons to add content.")
	}
	if form == nil {
		form = widget.NewLabel("Nothing to edit here.")
	}
	e.formHost.Objects = []fyne.CanvasObject{form}
	e.formHost.Refresh()
}

// refreshForm rebuilds the current form (after a sub-list add/remove).
func (e *editor) refreshForm() { e.showForm(e.currentUID) }

func (e *editor) refreshTree() {
	if e.nav != nil {
		e.nav.Refresh()
		e.nav.OpenAllBranches()
	}
}

// --- Add / delete --------------------------------------------------------

func (e *editor) addZone() {
	id := uniqueID("zone", func(s string) bool { return e.adv.Zone(s) != nil })
	e.adv.Zones = append(e.adv.Zones, domain.Zone{ID: id, Name: "New Zone"})
	e.markDirty()
	e.refreshTree()
	e.showForm("zone:" + id)
}

func (e *editor) addRoom() {
	zid := e.selectedZoneID()
	if zid == "" {
		e.info("Select a zone (or a room in it) first, then +Room.")
		return
	}
	z := e.adv.Zone(zid)
	rid := uniqueID("room", func(s string) bool { r, _ := e.adv.Room(s); return r != nil })
	z.Rooms = append(z.Rooms, domain.Room{ID: rid, Name: "New Room"})
	e.markDirty()
	e.refreshTree()
	e.showForm("room:" + zid + "::" + rid)
}

func (e *editor) addNPC() {
	id := uniqueID("npc", func(s string) bool { return e.adv.NPC(s) != nil })
	e.adv.NPCs = append(e.adv.NPCs, domain.NPC{ID: id, Name: "New NPC"})
	e.markDirty()
	e.refreshTree()
	e.showForm("npc:" + id)
}

func (e *editor) addEvent() {
	id := uniqueID("event", func(s string) bool { return e.adv.Event(s) != nil })
	e.adv.Events = append(e.adv.Events, domain.Event{ID: id, Name: "New Event"})
	e.markDirty()
	e.refreshTree()
	e.showForm("event:" + id)
}

func (e *editor) addItem() {
	id := uniqueID("item", func(s string) bool { return e.adv.Item(s) != nil })
	e.adv.Items = append(e.adv.Items, domain.Item{ID: id, Name: "New Item"})
	e.markDirty()
	e.refreshTree()
	e.showForm("item:" + id)
}

func (e *editor) addImage() {
	// Remote editing persists only adventure JSON (there is no asset-upload API),
	// so a new image reference would point at a file the server never receives.
	// Block it and steer the user to full-module import, which carries assets.
	if e.remoteMode {
		go nativeui.Info("Images not editable remotely",
			"Adding or replacing image assets isn't available when editing on a server. Package the module and use Import (.tar.gz) to change assets.")
		return
	}
	id := uniqueID("image", func(s string) bool { return e.adv.ImageByID(s) != nil })
	e.adv.Images = append(e.adv.Images, domain.ImageRef{ID: id, Kind: "art"})
	e.markDirty()
	e.refreshTree()
	e.showForm("img:" + id)
}

func (e *editor) addTable() {
	id := uniqueID("table", func(s string) bool { return e.adv.Table(s) != nil })
	e.adv.Tables = append(e.adv.Tables, domain.Table{ID: id, Name: "New Table", Dice: "d20"})
	e.markDirty()
	e.refreshTree()
	e.showForm("table:" + id)
}

func (e *editor) deleteSelected() {
	uid := e.currentUID
	if uid == "" || e.isBranch(uid) && !strings.HasPrefix(uid, "zone:") {
		e.info("Select a zone, room, NPC, event, or item to delete.")
		return
	}
	label := e.nodeLabel(uid)
	go func() {
		if !nativeui.Confirm("Delete", "Delete "+label+"?") {
			return
		}
		fyne.Do(func() {
			e.removeByUID(uid)
			e.markDirty()
			e.currentUID = ""
			e.refreshTree()
			e.formHost.Objects = []fyne.CanvasObject{widget.NewLabel("Deleted.")}
			e.formHost.Refresh()
		})
	}()
}

func (e *editor) removeByUID(uid string) {
	switch {
	case strings.HasPrefix(uid, "zone:"):
		zid := strings.TrimPrefix(uid, "zone:")
		e.adv.Zones = filterZones(e.adv.Zones, func(z domain.Zone) bool { return z.ID != zid })
	case strings.HasPrefix(uid, "room:"):
		zid, rid := parseRoomUID(uid)
		if z := e.adv.Zone(zid); z != nil {
			z.Rooms = filterRooms(z.Rooms, func(r domain.Room) bool { return r.ID != rid })
		}
	case strings.HasPrefix(uid, "npc:"):
		id := strings.TrimPrefix(uid, "npc:")
		e.adv.NPCs = filterNPCs(e.adv.NPCs, func(n domain.NPC) bool { return n.ID != id })
	case strings.HasPrefix(uid, "event:"):
		id := strings.TrimPrefix(uid, "event:")
		e.adv.Events = filterEvents(e.adv.Events, func(ev domain.Event) bool { return ev.ID != id })
	case strings.HasPrefix(uid, "table:"):
		id := strings.TrimPrefix(uid, "table:")
		e.adv.Tables = filterTables(e.adv.Tables, func(t domain.Table) bool { return t.ID != id })
	case strings.HasPrefix(uid, "item:"):
		id := strings.TrimPrefix(uid, "item:")
		e.adv.Items = filterItems(e.adv.Items, func(it domain.Item) bool { return it.ID != id })
	case strings.HasPrefix(uid, "img:"):
		id := strings.TrimPrefix(uid, "img:")
		e.adv.Images = filterImages(e.adv.Images, func(im domain.ImageRef) bool { return im.ID != id })
	}
}

func filterImages(s []domain.ImageRef, keep func(domain.ImageRef) bool) []domain.ImageRef {
	out := s[:0]
	for _, v := range s {
		if keep(v) {
			out = append(out, v)
		}
	}
	return out
}

func (e *editor) selectedZoneID() string {
	switch {
	case strings.HasPrefix(e.currentUID, "zone:"):
		return strings.TrimPrefix(e.currentUID, "zone:")
	case strings.HasPrefix(e.currentUID, "room:"):
		zid, _ := parseRoomUID(e.currentUID)
		return zid
	}
	if len(e.adv.Zones) > 0 {
		return e.adv.Zones[0].ID
	}
	return ""
}

// --- File operations -----------------------------------------------------

func (e *editor) confirmNew() {
	if !e.dirty {
		e.newAdventure()
		e.reload()
		return
	}
	go func() {
		if nativeui.Confirm("New adventure", "Discard unsaved changes?") {
			fyne.Do(func() {
				e.newAdventure()
				e.reload()
			})
		}
	}()
}

func (e *editor) newAdventure() {
	dir, err := os.MkdirTemp("", "thaim-edit-*")
	if err != nil {
		e.showErr(err)
		return
	}
	_ = os.MkdirAll(filepath.Join(dir, "assets"), 0755)
	e.workingDir = dir
	e.adv = &domain.Adventure{
		SchemaVersion: domain.SchemaVersion,
		ID:            "new-adventure",
		Title:         "New Adventure",
		System:        "D&D 5e",
		Zones: []domain.Zone{{
			ID:    "zone1",
			Name:  "Zone 1",
			Rooms: []domain.Room{{ID: "room1", Name: "Starting Room"}},
		}},
	}
	e.dirty = false
}

// openDialog opens an existing module — either an unpacked adventure folder or a
// packaged .tar.gz — chosen from a single dialog.
func (e *editor) openDialog() {
	go func() {
		if !e.confirmReplaceNative() {
			return
		}
		switch nativeui.Choice("Open adventure", "Open an adventure folder, or a .tar.gz module?", "Adventure folder…", ".tar.gz module…") {
		case 1:
			if dir, ok := nativeui.OpenFolder("Open adventure folder"); ok {
				e.loadFromFolder(dir)
			}
		case 2:
			if src, ok := nativeui.OpenFile("Open .tar.gz module",
				nativeui.Filter{Name: "Adventure module", Patterns: []string{"*.tar.gz", "*.tgz", "*.gz"}}); ok {
				e.loadFromArchive(src)
			}
		}
	}()
}

// loadFromFolder opens an unpacked module directory in place. It runs on the
// caller's goroutine (not the UI thread) and marshals UI updates via fyne.Do.
func (e *editor) loadFromFolder(dir string) {
	data, rerr := os.ReadFile(filepath.Join(dir, storage.AdventureFile))
	if rerr != nil {
		nativeui.Error("Open folder", fmt.Sprintf("No %s in that folder.", storage.AdventureFile))
		return
	}
	var adv domain.Adventure
	if jerr := json.Unmarshal(data, &adv); jerr != nil {
		nativeui.Error("Open folder", "Invalid adventure.json: "+jerr.Error())
		return
	}
	fyne.Do(func() {
		e.adv = &adv
		e.workingDir = dir
		e.dirty = false
		e.reload()
		e.setStatus("Opened folder: " + dir)
	})
}

// loadFromArchive extracts a .tar.gz into a temp working dir and opens it,
// normalizing images (TIFF→PNG, drop near-blank layers) so previews render.
func (e *editor) loadFromArchive(src string) {
	dir, merr := os.MkdirTemp("", "thaim-edit-*")
	if merr != nil {
		nativeui.Error("Open module", merr.Error())
		return
	}
	if xerr := storage.ExtractModule(src, dir); xerr != nil {
		nativeui.Error("Open module", xerr.Error())
		return
	}
	data, rerr := os.ReadFile(filepath.Join(dir, storage.AdventureFile))
	if rerr != nil {
		nativeui.Error("Open module", fmt.Sprintf("Archive has no %s.", storage.AdventureFile))
		return
	}
	var adv domain.Adventure
	if jerr := json.Unmarshal(data, &adv); jerr != nil {
		nativeui.Error("Open module", "Invalid adventure.json: "+jerr.Error())
		return
	}
	transcoded, dropped := ingest.NormalizeModuleImages(dir, &adv)
	status := "Opened archive: " + src
	if transcoded > 0 || dropped > 0 {
		status = fmt.Sprintf("%s (normalized %d image(s), dropped %d blank layer(s))", status, transcoded, dropped)
	}
	fyne.Do(func() {
		e.adv = &adv
		e.workingDir = dir
		e.dirty = transcoded > 0 || dropped > 0 // prompt to save the cleaned-up module
		e.reload()
		e.setStatus(status)
	})
}

// importDialog builds a new module with AI from either a PDF file or a folder of
// images, chosen from a single dialog.
func (e *editor) importDialog() {
	// AI import scaffolds a fresh working dir with locally-extracted images. In
	// remote mode those assets are never uploaded (saves send JSON only), so the
	// server would end up with references to files it doesn't have. Block it and
	// steer the user to full-module import from the library, which carries assets.
	if e.remoteMode {
		go nativeui.Info("Import not available remotely",
			"Building a module from a PDF/images isn't available when editing on a server (its images wouldn't be uploaded). Author locally and Import the .tar.gz, or use Import (.tar.gz) from the library.")
		return
	}
	if !e.requireProvider() {
		return
	}
	go func() {
		if !e.confirmReplaceNative() {
			return
		}
		switch nativeui.Choice("Import adventure", "Import from a PDF file, or a folder of images?", "PDF file…", "Images folder…") {
		case 1:
			src, ok := nativeui.OpenFile("Choose a PDF", nativeui.Filter{Name: "PDF", Patterns: []string{"*.pdf"}})
			if !ok {
				return
			}
			title := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
			e.runIngest("Interpreting PDF with AI…", func(dir string) (*domain.Adventure, error) {
				ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
				defer cancel()
				return aibuild.FromPDF(ctx, e.prov, e.importConfig(), src, dir, title, e.progress(), e.fallbackConfirm(), e.visionProv)
			})
		case 2:
			src, ok := nativeui.OpenFolder("Choose a folder of images")
			if !ok {
				return
			}
			e.runIngest("Interpreting images with AI…", func(dir string) (*domain.Adventure, error) {
				ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
				defer cancel()
				return aibuild.FromImages(ctx, e.prov, e.importConfig(), src, dir, filepath.Base(src), e.progress(), e.fallbackConfirm(), e.visionProv)
			})
		}
	}()
}

// progress returns a callback that mirrors import progress into the status bar.
func (e *editor) progress() aibuild.Progress {
	return func(s string) { fyne.Do(func() { e.setStatus(s) }) }
}

// fallbackConfirm returns a callback the importer consults when the chosen model
// is unavailable and would be substituted (e.g. a rate-limited model falling back
// to a smaller one). It asks natively whether to proceed with the substitute.
func (e *editor) fallbackConfirm() aibuild.ConfirmFallback {
	return func(requested, served string) bool {
		return nativeui.Confirm("Model unavailable",
			fmt.Sprintf("%q is currently unavailable (likely rate-limited).\n\nContinue this import with %q instead?\n\nChoose Cancel to abort and retry later with %q.",
				requested, served, requested))
	}
}

// importLang is the language used when the Translate toggle is on: the configured
// import language, falling back to the UI language.
func (e *editor) importLang() string {
	if e.config == nil {
		return "Spanish"
	}
	if l := strings.TrimSpace(e.config.ImportLanguage); l != "" {
		return l
	}
	return string(e.config.Language)
}

// translateLabel is the checkbox caption, naming the target language.
func (e *editor) translateLabel() string {
	return "Translate import → " + e.importConfigWith(true).ImportLanguageName()
}

// importConfig returns the config to use for an import, honoring the Translate
// toggle: when off, translation is disabled (import in the source language);
// when on, the configured/UI import language is applied. It never mutates the
// shared config — it returns a copy.
func (e *editor) importConfig() *domain.Config {
	return e.importConfigWith(e.translate)
}

func (e *editor) importConfigWith(translate bool) *domain.Config {
	if e.config == nil {
		e.config = domain.DefaultConfig()
	}
	cfg := *e.config // shallow copy
	if translate {
		cfg.ImportLanguage = e.importLang()
	} else {
		cfg.ImportLanguage = ""
	}
	return &cfg
}

// requireProvider ensures an AI provider is configured before an AI import.
func (e *editor) requireProvider() bool {
	if e.prov != nil {
		return true
	}
	e.showErr(fmt.Errorf("AI import needs an API key. Set THAIM_OPENAI_API_KEY or THAIM_ANTHROPIC_API_KEY (or configure it in the player) and restart the editor"))
	return false
}

// runIngest creates a fresh working dir, runs build off the UI goroutine, and
// swaps in the resulting adventure on success.
func (e *editor) runIngest(msg string, build func(workingDir string) (*domain.Adventure, error)) {
	dir, err := os.MkdirTemp("", "thaim-edit-*")
	if err != nil {
		e.showErr(err)
		return
	}
	fyne.Do(func() { e.setStatus(msg) })
	go func() {
		adv, err := build(dir)
		fyne.Do(func() {
			if err != nil {
				_ = os.RemoveAll(dir)
				e.showErr(err)
				e.setStatus("")
				return
			}
			e.adv = adv
			e.workingDir = dir
			e.markDirty()
			e.reload()
			e.setStatus(fmt.Sprintf("Imported scaffold: %d zone(s), %d room(s), %d image(s). Review, then Save/Package.",
				len(adv.Zones), countRooms(adv), len(adv.ImageRefs())))
		})
	}()
}

// confirmReplaceNative asks (natively) to discard unsaved changes. Must be
// called off the UI goroutine. Returns true to proceed.
func (e *editor) confirmReplaceNative() bool {
	if !e.dirty {
		return true
	}
	return nativeui.Confirm("Discard changes?", "This will replace the current module. Continue?")
}

func countRooms(adv *domain.Adventure) int {
	n := 0
	for _, z := range adv.Zones {
		n += len(z.Rooms)
	}
	return n
}

func (e *editor) reload() {
	e.currentUID = ""
	e.refreshTree()
	e.formHost.Objects = []fyne.CanvasObject{widget.NewLabel("Select a node on the left to edit.")}
	e.formHost.Refresh()
}

// doSave persists the adventure and invokes onDone (on the UI thread) with the
// result. In remote mode it delegates to saveHook, which runs its HTTP off the UI
// thread and reports back via fyne.Do — so a slow/stalled server never freezes the
// UI. In local mode it writes synchronously to the working dir. onDone may be nil.
func (e *editor) doSave(onDone func(error)) {
	if e.saveHook == nil {
		err := e.save()
		if onDone != nil {
			onDone(err)
		}
		return
	}
	// Serialize remote saves: one in flight at a time, so an older PUT can't land
	// after (and overwrite) a newer one.
	if e.saving {
		e.setStatus("A save is already in progress…")
		return
	}
	e.saving = true
	e.setStatus("Saving…")
	advAtSnap := e.adv     // the adventure this save is for
	revAtSnap := e.editRev // the content revision it captured
	e.saveHook(func(err error) {
		e.saving = false
		// Superseded: the editor moved to a different adventure since this save
		// started (navigation reused the shared editor). The result is irrelevant to
		// what's on screen now — don't touch its dirty flag or status, and report it
		// as unsuccessful so a continuation (playCurrent) doesn't act on stale state.
		if e.adv != advAtSnap {
			if onDone != nil {
				onDone(errSaveSuperseded)
			}
			return
		}
		if err != nil {
			e.showErr(err)
			if onDone != nil {
				onDone(err)
			}
			return
		}
		if e.editRev != revAtSnap {
			// Newer edits arrived after the snapshot and weren't included in this
			// save. Keep dirty set, and treat it as non-success so playCurrent won't
			// launch a session on the older (just-saved) server state.
			e.setStatus("Saved an earlier revision — you have newer unsaved changes.")
			if onDone != nil {
				onDone(errSaveSuperseded)
			}
			return
		}
		e.dirty = false
		e.setStatus("Saved to server")
		if onDone != nil {
			onDone(nil)
		}
	})
}

// save writes the adventure.json to the local working dir. It is the on-disk
// (local-mode) path; remote persistence goes through doSave/saveHook instead.
func (e *editor) save() error {
	if e.workingDir == "" {
		e.showErr(fmt.Errorf("no working directory"))
		return nil
	}
	data, err := json.MarshalIndent(e.adv, "", "  ")
	if err != nil {
		e.showErr(err)
		return err
	}
	path := filepath.Join(e.workingDir, storage.AdventureFile)
	if err := os.WriteFile(path, data, 0644); err != nil {
		e.showErr(err)
		return err
	}
	e.dirty = false
	e.setStatus("Saved " + path)
	return nil
}

func (e *editor) validate() {
	imageExists := func(rel string) bool {
		// In remote mode the assets live on the server, not in a local working dir,
		// so we can't stat them here — don't flag referenced images as missing.
		if e.remoteMode {
			return true
		}
		if e.workingDir == "" {
			return false
		}
		info, err := os.Stat(filepath.Join(e.workingDir, filepath.FromSlash(rel)))
		return err == nil && !info.IsDir()
	}
	errs := domain.ValidateAdventure(e.adv, imageExists)
	if len(errs) == 0 {
		go nativeui.Info("Validation", "✓ The adventure is valid.")
		return
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d problem(s):\n\n", len(errs)))
	for _, er := range errs {
		sb.WriteString("• " + er.Error() + "\n")
	}
	go nativeui.Info("Validation", sb.String())
}

// exportDMBook renders the current adventure as a complete DM sourcebook and
// saves it as Markdown or a print-ready PDF, chosen from a native dialog. It is
// deterministic (no AI): a faithful, organized rendering of the module content.
func (e *editor) exportDMBook() {
	if e.adv == nil {
		e.showErr(fmt.Errorf("open or create a module first"))
		return
	}
	adv := e.adv
	md := dmbook.Markdown(adv)
	go func() {
		switch nativeui.Choice("Export DM book", "Export the adventure sourcebook — which format?", "Markdown (.md)", "PDF (.pdf)") {
		case 1:
			dest, ok := nativeui.SaveFile("Save DM book", adv.ID+"-dmbook.md",
				nativeui.Filter{Name: "Markdown", Patterns: []string{"*.md"}})
			if !ok {
				return
			}
			if err := os.WriteFile(dest, []byte(md), 0644); err != nil {
				nativeui.Error("Export failed", err.Error())
				return
			}
			nativeui.Info("DM book exported", "Saved:\n"+dest)
			fyne.Do(func() { e.setStatus("Exported DM book: " + dest) })
		case 2:
			dest, ok := nativeui.SaveFile("Save DM book", adv.ID+"-dmbook.pdf",
				nativeui.Filter{Name: "PDF", Patterns: []string{"*.pdf"}})
			if !ok {
				return
			}
			pdfBytes, err := bookpdf.FromMarkdown(adv.Title, "Dungeon Master's Sourcebook", md)
			if err != nil {
				nativeui.Error("Export failed", err.Error())
				return
			}
			if err := os.WriteFile(dest, pdfBytes, 0644); err != nil {
				nativeui.Error("Export failed", err.Error())
				return
			}
			nativeui.Info("DM book exported", "Saved:\n"+dest)
			fyne.Do(func() { e.setStatus("Exported DM book: " + dest) })
		}
	}()
}

// saveDialog flushes the current edits and saves the module either as an unpacked
// adventure folder or as a packaged .tar.gz, chosen from a single dialog.
func (e *editor) saveDialog() {
	if e.workingDir == "" || e.adv == nil {
		e.showErr(fmt.Errorf("nothing to save yet — create or import a module first"))
		return
	}
	if err := e.save(); err != nil { // flush adventure.json into the working dir first
		return
	}
	work, advID := e.workingDir, e.adv.ID
	go func() {
		switch nativeui.Choice("Save adventure", "Save as an adventure folder, or a .tar.gz module?", "Adventure folder…", ".tar.gz module…") {
		case 1:
			dest, ok := nativeui.OpenFolder("Choose a destination folder")
			if !ok {
				return
			}
			if filepath.Clean(dest) != filepath.Clean(work) {
				if cerr := copyTree(work, dest); cerr != nil {
					nativeui.Error("Save failed", cerr.Error())
					return
				}
			}
			fyne.Do(func() {
				e.workingDir = dest // subsequent saves target the chosen folder
				e.setStatus("Saved folder: " + dest)
			})
		case 2:
			dest, ok := nativeui.SaveFile("Save .tar.gz module", advID+".tar.gz",
				nativeui.Filter{Name: "Adventure module", Patterns: []string{"*.tar.gz"}})
			if !ok {
				return
			}
			_ = os.Remove(dest) // PackageModule recreates it
			if perr := storage.PackageModule(work, dest); perr != nil {
				nativeui.Error("Save failed", perr.Error())
				return
			}
			nativeui.Info("Saved", "Module written to:\n"+dest)
			fyne.Do(func() { e.setStatus("Saved archive: " + dest) })
		}
	}()
}

// copyTree recursively copies the module at src into dst, skipping hidden files
// and the diagnostic import dump so a saved folder stays clean.
func copyTree(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(src, p)
		if rerr != nil {
			return rerr
		}
		if rel == "." {
			return os.MkdirAll(dst, 0755)
		}
		// Copy ONLY the canonical module layout: adventure.json + the assets/ tree.
		// Never copy anything else in the working dir — this both keeps the saved
		// folder clean and prevents runaway/recursive copies when the working dir
		// points at a folder full of unrelated files.
		slash := filepath.ToSlash(rel)
		top := slash
		if i := strings.IndexByte(slash, '/'); i >= 0 {
			top = slash[:i]
		}
		if top != storage.AdventureFile && top != "assets" {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		base := filepath.Base(p)
		if strings.HasPrefix(base, ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyFileContents(p, target)
	})
}

func copyFileContents(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// importImage copies a chosen file into workingDir/assets/<kind>/ and calls set
// with the new relative path.
func (e *editor) importImage(kind string, set func(string)) {
	if e.workingDir == "" {
		e.showErr(fmt.Errorf("save or create a module first"))
		return
	}
	work := e.workingDir
	go func() {
		src, ok := nativeui.OpenFile("Import image",
			nativeui.Filter{Name: "Images", Patterns: []string{"*.png", "*.jpg", "*.jpeg", "*.gif", "*.webp"}})
		if !ok {
			return
		}
		name := filepath.Base(src)
		relDir := filepath.Join("assets", kind)
		destDir := filepath.Join(work, relDir)
		if mkErr := os.MkdirAll(destDir, 0755); mkErr != nil {
			nativeui.Error("Import image", mkErr.Error())
			return
		}
		in, oerr := os.Open(src)
		if oerr != nil {
			nativeui.Error("Import image", oerr.Error())
			return
		}
		defer in.Close()
		out, cerr := os.Create(filepath.Join(destDir, name))
		if cerr != nil {
			nativeui.Error("Import image", cerr.Error())
			return
		}
		defer out.Close()
		if _, werr := io.Copy(out, in); werr != nil {
			nativeui.Error("Import image", werr.Error())
			return
		}
		rel := filepath.ToSlash(filepath.Join(relDir, name))
		fyne.Do(func() {
			set(rel)
			e.markDirty()
			e.setStatus("Imported image: " + rel)
		})
	}()
}

// --- helpers -------------------------------------------------------------

func (e *editor) setStatus(s string) {
	if e.status != nil {
		e.status.SetText(s)
	}
}
func (e *editor) info(s string)     { go nativeui.Info("Editor", s) }
func (e *editor) showErr(err error) { go nativeui.Error("Editor", err.Error()) }

func parseRoomUID(uid string) (zoneID, roomID string) {
	body := strings.TrimPrefix(uid, "room:")
	if i := strings.Index(body, "::"); i >= 0 {
		return body[:i], body[i+2:]
	}
	return "", body
}

func uniqueID(prefix string, exists func(string) bool) string {
	for i := 1; ; i++ {
		id := fmt.Sprintf("%s%d", prefix, i)
		if !exists(id) {
			return id
		}
	}
}

func filterZones(s []domain.Zone, keep func(domain.Zone) bool) []domain.Zone {
	out := s[:0]
	for _, v := range s {
		if keep(v) {
			out = append(out, v)
		}
	}
	return out
}
func filterRooms(s []domain.Room, keep func(domain.Room) bool) []domain.Room {
	out := s[:0]
	for _, v := range s {
		if keep(v) {
			out = append(out, v)
		}
	}
	return out
}
func filterNPCs(s []domain.NPC, keep func(domain.NPC) bool) []domain.NPC {
	out := s[:0]
	for _, v := range s {
		if keep(v) {
			out = append(out, v)
		}
	}
	return out
}
func filterEvents(s []domain.Event, keep func(domain.Event) bool) []domain.Event {
	out := s[:0]
	for _, v := range s {
		if keep(v) {
			out = append(out, v)
		}
	}
	return out
}
func filterItems(s []domain.Item, keep func(domain.Item) bool) []domain.Item {
	out := s[:0]
	for _, v := range s {
		if keep(v) {
			out = append(out, v)
		}
	}
	return out
}

func filterTables(s []domain.Table, keep func(domain.Table) bool) []domain.Table {
	out := s[:0]
	for _, v := range s {
		if keep(v) {
			out = append(out, v)
		}
	}
	return out
}

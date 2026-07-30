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
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/theburrowhub/thaimaturgy/internal/aibuild"
	"github.com/theburrowhub/thaimaturgy/internal/auth"
	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/guitheme"
	_ "github.com/theburrowhub/thaimaturgy/internal/imagefmt" // register TIFF/WebP/BMP decoders
	"github.com/theburrowhub/thaimaturgy/internal/ingest"
	"github.com/theburrowhub/thaimaturgy/internal/nativeui"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

type editor struct {
	app fyne.App
	win fyne.Window

	config  *domain.Config
	prov    providers.Provider
	model   string
	authMsg string

	adv        *domain.Adventure
	workingDir string // holds adventure.json + assets/
	dirty      bool

	nav        *widget.Tree
	formHost   *fyne.Container
	status     *widget.Label
	currentUID string
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

	authMsg := auth.AutoConfigure(config)
	if !store.ConfigExists() {
		_ = store.SaveConfig(config) // generate config.yaml on first run
	}

	e := &editor{app: app.NewWithID("dev.theburrowhub.thaimaturgy.editor"), config: config, prov: providers.New(config), model: config.Model, authMsg: authMsg}
	e.app.Settings().SetTheme(guitheme.New())
	e.win = e.app.NewWindow("thAImaturgy — Module Editor")
	e.win.Resize(fyne.NewSize(1180, 820))
	e.newAdventure() // start with a template
	e.win.SetContent(e.buildUI())
	if authMsg != "" {
		e.setStatus(authMsg)
	}
	e.win.ShowAndRun()
}

// --- UI scaffold ---------------------------------------------------------

func (e *editor) buildUI() fyne.CanvasObject {
	toolbar := container.NewHBox(
		widget.NewButton("New", e.confirmNew),
		widget.NewButton("Open folder…", e.openFolder),
		widget.NewButton("Open .tar.gz…", e.openArchive),
		widget.NewButton("Import images…", e.ingestFolder),
		widget.NewButton("Import PDF…", e.ingestPDF),
		widget.NewButton("Save", func() { _ = e.save() }),
		widget.NewButton("Validate", e.validate),
		widget.NewButton("Package .tar.gz…", e.packageModule),
	)

	e.status = widget.NewLabel("")
	e.formHost = container.NewStack()

	navTools := container.NewHBox(
		widget.NewButton("+Zone", e.addZone),
		widget.NewButton("+Room", e.addRoom),
		widget.NewButton("+NPC", e.addNPC),
		widget.NewButton("+Event", e.addEvent),
		widget.NewButton("+Item", e.addItem),
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
		return []widget.TreeNodeID{"meta", "zones", "npcs", "events", "items", "images"}
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
	case "", "zones", "npcs", "events", "items", "images":
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
	e.dirty = true
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
	e.dirty = true
	e.refreshTree()
	e.showForm("room:" + zid + "::" + rid)
}

func (e *editor) addNPC() {
	id := uniqueID("npc", func(s string) bool { return e.adv.NPC(s) != nil })
	e.adv.NPCs = append(e.adv.NPCs, domain.NPC{ID: id, Name: "New NPC"})
	e.dirty = true
	e.refreshTree()
	e.showForm("npc:" + id)
}

func (e *editor) addEvent() {
	id := uniqueID("event", func(s string) bool { return e.adv.Event(s) != nil })
	e.adv.Events = append(e.adv.Events, domain.Event{ID: id, Name: "New Event"})
	e.dirty = true
	e.refreshTree()
	e.showForm("event:" + id)
}

func (e *editor) addItem() {
	id := uniqueID("item", func(s string) bool { return e.adv.Item(s) != nil })
	e.adv.Items = append(e.adv.Items, domain.Item{ID: id, Name: "New Item"})
	e.dirty = true
	e.refreshTree()
	e.showForm("item:" + id)
}

func (e *editor) addImage() {
	id := uniqueID("image", func(s string) bool { return e.adv.ImageByID(s) != nil })
	e.adv.Images = append(e.adv.Images, domain.ImageRef{ID: id, Kind: "art"})
	e.dirty = true
	e.refreshTree()
	e.showForm("img:" + id)
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
			e.dirty = true
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

func (e *editor) openFolder() {
	go func() {
		dir, ok := nativeui.OpenFolder("Open module folder")
		if !ok {
			return
		}
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
	}()
}

func (e *editor) openArchive() {
	go func() {
		src, ok := nativeui.OpenFile("Open .tar.gz module",
			nativeui.Filter{Name: "Adventure module", Patterns: []string{"*.tar.gz", "*.tgz", "*.gz"}})
		if !ok {
			return
		}
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
		// Transcode TIFF→PNG (incl. CMYK), downscale, and drop near-blank layers so
		// previews render and junk paper-texture layers are purged on open.
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
	}()
}

// ingestFolder builds a module from a folder of images, interpreted by the AI.
func (e *editor) ingestFolder() {
	if !e.requireProvider() {
		return
	}
	go func() {
		if !e.confirmReplaceNative() {
			return
		}
		src, ok := nativeui.OpenFolder("Choose a folder of images")
		if !ok {
			return
		}
		e.runIngest("Interpreting images with AI…", func(dir string) (*domain.Adventure, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer cancel()
			return aibuild.FromImages(ctx, e.prov, e.config, src, dir, filepath.Base(src), e.progress())
		})
	}()
}

// ingestPDF builds a module from a PDF: text and images are interpreted by the AI.
func (e *editor) ingestPDF() {
	if !e.requireProvider() {
		return
	}
	go func() {
		if !e.confirmReplaceNative() {
			return
		}
		src, ok := nativeui.OpenFile("Choose a PDF", nativeui.Filter{Name: "PDF", Patterns: []string{"*.pdf"}})
		if !ok {
			return
		}
		title := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
		e.runIngest("Interpreting PDF with AI…", func(dir string) (*domain.Adventure, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer cancel()
			return aibuild.FromPDF(ctx, e.prov, e.config, src, dir, title, e.progress())
		})
	}()
}

// progress returns a callback that mirrors import progress into the status bar.
func (e *editor) progress() aibuild.Progress {
	return func(s string) { fyne.Do(func() { e.setStatus(s) }) }
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
			e.dirty = true
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

func (e *editor) packageModule() {
	if err := e.save(); err != nil {
		return
	}
	work, advID := e.workingDir, e.adv.ID
	go func() {
		dest, ok := nativeui.SaveFile("Package module", advID+".tar.gz",
			nativeui.Filter{Name: "Adventure module", Patterns: []string{"*.tar.gz"}})
		if !ok {
			return
		}
		_ = os.Remove(dest) // PackageModule recreates it
		if perr := storage.PackageModule(work, dest); perr != nil {
			nativeui.Error("Package failed", perr.Error())
			return
		}
		nativeui.Info("Packaged", "Module written to:\n"+dest)
		fyne.Do(func() { e.setStatus("Packaged: " + dest) })
	}()
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
			e.dirty = true
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

func labelOrID(name, id string) string {
	if strings.TrimSpace(name) != "" {
		return name
	}
	return id
}

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

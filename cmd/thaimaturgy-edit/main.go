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
	"fyne.io/fyne/v2/dialog"
	fynestorage "fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"github.com/theburrowhub/thaimaturgy/internal/aibuild"
	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

type editor struct {
	app fyne.App
	win fyne.Window

	config *domain.Config
	prov   providers.Provider
	model  string

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

	e := &editor{app: app.New(), config: config, prov: newProvider(config), model: config.Model}
	e.win = e.app.NewWindow("thAImaturgy — Module Editor")
	e.win.Resize(fyne.NewSize(1180, 820))
	e.newAdventure() // start with a template
	e.win.SetContent(e.buildUI())
	e.win.ShowAndRun()
}

func newProvider(c *domain.Config) providers.Provider {
	switch c.Provider {
	case domain.ProviderOpenAI:
		if c.OpenAIAPIKey != "" {
			return providers.NewOpenAIProvider(c.OpenAIAPIKey)
		}
	case domain.ProviderAnthropic:
		if c.AnthropicAPIKey != "" {
			return providers.NewAnthropicProvider(c.AnthropicAPIKey)
		}
	}
	return nil
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
		widget.NewButton("Delete", e.deleteSelected),
	)
	e.nav = e.buildTree()
	left := container.NewBorder(navTools, nil, nil, nil, e.nav)

	split := container.NewHSplit(left, container.NewVScroll(e.formHost))
	split.SetOffset(0.28)

	return container.NewBorder(toolbar, e.status, nil, nil, split)
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
		return []widget.TreeNodeID{"meta", "zones", "npcs", "events", "items"}
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
	case "", "zones", "npcs", "events", "items":
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

func (e *editor) deleteSelected() {
	uid := e.currentUID
	if uid == "" || e.isBranch(uid) && !strings.HasPrefix(uid, "zone:") {
		e.info("Select a zone, room, NPC, event, or item to delete.")
		return
	}
	dialog.ShowConfirm("Delete", "Delete "+e.nodeLabel(uid)+"?", func(ok bool) {
		if !ok {
			return
		}
		e.removeByUID(uid)
		e.dirty = true
		e.currentUID = ""
		e.refreshTree()
		e.formHost.Objects = []fyne.CanvasObject{widget.NewLabel("Deleted.")}
		e.formHost.Refresh()
	}, e.win)
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
	}
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
	dialog.ShowConfirm("New adventure", "Discard unsaved changes?", func(ok bool) {
		if ok {
			e.newAdventure()
			e.reload()
		}
	}, e.win)
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
	dialog.ShowFolderOpen(func(lu fyne.ListableURI, err error) {
		if err != nil || lu == nil {
			return
		}
		dir := lu.Path()
		data, rerr := os.ReadFile(filepath.Join(dir, storage.AdventureFile))
		if rerr != nil {
			e.showErr(fmt.Errorf("no %s in that folder: %w", storage.AdventureFile, rerr))
			return
		}
		var adv domain.Adventure
		if jerr := json.Unmarshal(data, &adv); jerr != nil {
			e.showErr(fmt.Errorf("invalid adventure.json: %w", jerr))
			return
		}
		e.adv = &adv
		e.workingDir = dir
		e.dirty = false
		e.reload()
		e.setStatus("Opened folder: " + dir)
	}, e.win)
}

func (e *editor) openArchive() {
	d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			return
		}
		src := r.URI().Path()
		_ = r.Close()
		dir, merr := os.MkdirTemp("", "thaim-edit-*")
		if merr != nil {
			e.showErr(merr)
			return
		}
		if xerr := storage.ExtractModule(src, dir); xerr != nil {
			e.showErr(xerr)
			return
		}
		data, rerr := os.ReadFile(filepath.Join(dir, storage.AdventureFile))
		if rerr != nil {
			e.showErr(fmt.Errorf("archive has no %s", storage.AdventureFile))
			return
		}
		var adv domain.Adventure
		if jerr := json.Unmarshal(data, &adv); jerr != nil {
			e.showErr(fmt.Errorf("invalid adventure.json: %w", jerr))
			return
		}
		e.adv = &adv
		e.workingDir = dir
		e.dirty = false
		e.reload()
		e.setStatus("Opened archive: " + src)
	}, e.win)
	d.SetFilter(fynestorage.NewExtensionFileFilter([]string{".gz", ".tgz", ".tar.gz"}))
	d.Show()
}

// ingestFolder builds a module from a folder of images, interpreted by the AI.
func (e *editor) ingestFolder() {
	if !e.requireProvider() {
		return
	}
	e.confirmReplace(func() {
		dialog.ShowFolderOpen(func(lu fyne.ListableURI, err error) {
			if err != nil || lu == nil {
				return
			}
			src := lu.Path()
			e.runIngest("Interpreting images with AI…", func(dir string) (*domain.Adventure, error) {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()
				return aibuild.FromImages(ctx, e.prov, e.model, src, dir, filepath.Base(src))
			})
		}, e.win)
	})
}

// ingestPDF builds a module from a PDF: text and images are interpreted by the AI.
func (e *editor) ingestPDF() {
	if !e.requireProvider() {
		return
	}
	e.confirmReplace(func() {
		d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil || r == nil {
				return
			}
			src := r.URI().Path()
			_ = r.Close()
			title := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
			e.runIngest("Interpreting PDF with AI…", func(dir string) (*domain.Adventure, error) {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()
				return aibuild.FromPDF(ctx, e.prov, e.model, src, dir, title)
			})
		}, e.win)
		d.SetFilter(fynestorage.NewExtensionFileFilter([]string{".pdf"}))
		d.Show()
	})
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
	e.setStatus(msg)
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

// confirmReplace runs action, first confirming if there are unsaved changes.
func (e *editor) confirmReplace(action func()) {
	if !e.dirty {
		action()
		return
	}
	dialog.ShowConfirm("Discard changes?", "This will replace the current module. Continue?", func(ok bool) {
		if ok {
			action()
		}
	}, e.win)
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
		dialog.ShowInformation("Validation", "✓ The adventure is valid.", e.win)
		return
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d problem(s):\n\n", len(errs)))
	for _, er := range errs {
		sb.WriteString("• " + er.Error() + "\n")
	}
	dialog.ShowInformation("Validation", sb.String(), e.win)
}

func (e *editor) packageModule() {
	if err := e.save(); err != nil {
		return
	}
	d := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		if err != nil || w == nil {
			return
		}
		dest := w.URI().Path()
		_ = w.Close()
		_ = os.Remove(dest) // PackageModule recreates it
		if perr := storage.PackageModule(e.workingDir, dest); perr != nil {
			e.showErr(perr)
			return
		}
		e.setStatus("Packaged: " + dest)
		dialog.ShowInformation("Packaged", "Module written to:\n"+dest, e.win)
	}, e.win)
	d.SetFileName(e.adv.ID + ".tar.gz")
	d.Show()
}

// importImage copies a chosen file into workingDir/assets/<kind>/ and calls set
// with the new relative path.
func (e *editor) importImage(kind string, set func(string)) {
	if e.workingDir == "" {
		e.showErr(fmt.Errorf("save or create a module first"))
		return
	}
	dialog.ShowFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			return
		}
		defer r.Close()
		name := filepath.Base(r.URI().Path())
		relDir := filepath.Join("assets", kind)
		destDir := filepath.Join(e.workingDir, relDir)
		if mkErr := os.MkdirAll(destDir, 0755); mkErr != nil {
			e.showErr(mkErr)
			return
		}
		out, cerr := os.Create(filepath.Join(destDir, name))
		if cerr != nil {
			e.showErr(cerr)
			return
		}
		defer out.Close()
		if _, werr := io.Copy(out, r); werr != nil {
			e.showErr(werr)
			return
		}
		rel := filepath.ToSlash(filepath.Join(relDir, name))
		set(rel)
		e.dirty = true
		e.setStatus("Imported image: " + rel)
	}, e.win)
}

// --- helpers -------------------------------------------------------------

func (e *editor) setStatus(s string) {
	if e.status != nil {
		e.status.SetText(s)
	}
}
func (e *editor) info(s string)     { dialog.ShowInformation("Editor", s, e.win) }
func (e *editor) showErr(err error) { dialog.ShowError(err, e.win) }

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

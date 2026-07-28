// Package ingest builds a scaffold adventure module from raw source material —
// a directory of images or a PDF — copying/extracting assets into a working
// directory and returning a *domain.Adventure the editor can refine. It is
// UI-agnostic and uses pure-Go PDF libraries, so no external tools are required.
package ingest

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	pdfread "github.com/ledongthuc/pdf"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	pdfmodel "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// maxImages caps how many images a single ingest will scaffold, so a huge PDF
// or folder can't produce an unwieldy module.
const maxImages = 300

// Asset is an extracted image copied into a module working directory.
type Asset struct {
	RelPath string // module-relative slash path, e.g. "assets/pdf/img_1_0.png"
	Page    int    // source PDF page (0 when unknown / from a folder)
	IsMap   bool   // filename heuristic suggests a map
}

// ExtractPDF extracts the PDF's text (with page markers) and its embedded images
// into workingDir/assets/, returning the combined text and the image assets.
// Used by the AI builder to interpret a document. Errors only if nothing at all
// could be read.
func ExtractPDF(pdfPath, workingDir string) (string, []Asset, error) {
	pageText := extractPDFText(pdfPath)
	byPage, catalog := extractPDFImages(pdfPath, workingDir)

	maxPage := 0
	for p := range pageText {
		if p > maxPage {
			maxPage = p
		}
	}
	for p := range byPage {
		if p > maxPage {
			maxPage = p
		}
	}
	if n, err := api.PageCountFile(pdfPath); err == nil && n > maxPage {
		maxPage = n
	}

	var sb strings.Builder
	for p := 1; p <= maxPage; p++ {
		t := strings.TrimSpace(pageText[p])
		if t == "" {
			continue
		}
		fmt.Fprintf(&sb, "\n=== Page %d ===\n%s\n", p, t)
	}

	var assets []Asset
	for p := 1; p <= maxPage; p++ {
		for _, rel := range byPage[p] {
			assets = append(assets, Asset{RelPath: rel, Page: p})
		}
	}
	for _, c := range catalog {
		assets = append(assets, Asset{RelPath: c.Path})
	}

	if strings.TrimSpace(sb.String()) == "" && len(assets) == 0 {
		return "", nil, fmt.Errorf("could not extract text or images from the PDF")
	}
	return sb.String(), assets, nil
}

// CollectDirImages copies every image in srcDir into workingDir/assets/ (maps vs
// art by filename heuristic) and returns them as assets, for the AI builder.
func CollectDirImages(srcDir, workingDir string) ([]Asset, error) {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && isImage(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil, fmt.Errorf("no image files found in %s", srcDir)
	}
	if len(names) > maxImages {
		names = names[:maxImages]
	}

	var assets []Asset
	for _, name := range names {
		isMap := looksLikeMap(name)
		kind := "art"
		if isMap {
			kind = "maps"
		}
		rel, err := copyImage(filepath.Join(srcDir, name), workingDir, kind)
		if err != nil {
			return nil, err
		}
		assets = append(assets, Asset{RelPath: rel, IsMap: isMap})
	}
	return assets, nil
}

var imageExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".bmp": true, ".tiff": true,
}

func isImage(name string) bool { return imageExts[strings.ToLower(filepath.Ext(name))] }

func looksLikeMap(name string) bool {
	n := strings.ToLower(name)
	for _, kw := range []string{"map", "mapa", "plan", "plano", "level", "nivel", "floor", "dungeon"} {
		if strings.Contains(n, kw) {
			return true
		}
	}
	return false
}

// slug turns a title into a filesystem/id-safe slug.
func slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "imported-adventure"
	}
	return out
}

// FromDirectory scaffolds a module from a directory of images. Every image is
// copied into workingDir/assets/ (under maps/ or art/ by a filename heuristic),
// cataloged, and turned into either the zone map or a room so it is reachable in
// the editor and the player. title seeds the adventure id/title.
func FromDirectory(srcDir, workingDir, title string) (*domain.Adventure, error) {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}
	var images []string
	for _, e := range entries {
		if e.IsDir() || !isImage(e.Name()) {
			continue
		}
		images = append(images, e.Name())
	}
	sort.Strings(images)
	if len(images) == 0 {
		return nil, fmt.Errorf("no image files found in %s", srcDir)
	}
	if len(images) > maxImages {
		images = images[:maxImages]
	}

	if title == "" {
		title = filepath.Base(srcDir)
	}
	adv := newScaffold(title)
	zone := &adv.Zones[0]
	zone.Rooms = nil // replace the placeholder room with imported content
	zone.Overview = "Auto-imported from a folder of images. Edit rooms and add detail."

	for _, name := range images {
		kind := "art"
		if looksLikeMap(name) {
			kind = "maps"
		}
		rel, err := copyImage(filepath.Join(srcDir, name), workingDir, kind)
		if err != nil {
			return nil, err
		}
		adv.Images = append(adv.Images, domain.ImageRef{
			ID:   slug(strings.TrimSuffix(name, filepath.Ext(name))),
			Path: rel, Kind: strings.TrimSuffix(kind, "s"),
		})
		if kind == "maps" && zone.MapImage == "" {
			zone.MapImage = rel
			continue
		}
		roomName := prettyName(name)
		zone.Rooms = append(zone.Rooms, domain.Room{
			ID:      uniqueRoomID(adv, slug(roomName)),
			Name:    roomName,
			Image:   rel,
			DMNotes: "Imported image: " + name,
		})
	}
	return adv, nil
}

var pageImgRe = regexp.MustCompile(`_(\d+)_`)

// FromPDF scaffolds a module from a PDF: text is extracted per page into rooms,
// and embedded images are extracted into workingDir/assets/ and attached to the
// room for their page when possible. Text and image extraction degrade
// independently — a failure in one still yields a useful scaffold.
func FromPDF(pdfPath, workingDir, title string) (*domain.Adventure, error) {
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(pdfPath), filepath.Ext(pdfPath))
	}
	adv := newScaffold(title)
	// Replace the default single room; we build one room per page.
	adv.Zones[0].Rooms = nil
	adv.Zones[0].Overview = "Auto-imported from a PDF. Each room is a page; edit and reorganize."
	zone := &adv.Zones[0]

	pageText := extractPDFText(pdfPath) // map[pageNo]text (best-effort)
	imagesByPage, catalog := extractPDFImages(pdfPath, workingDir)

	pageCount := len(pageText)
	if n, err := api.PageCountFile(pdfPath); err == nil && n > pageCount {
		pageCount = n
	}
	if pageCount == 0 {
		pageCount = len(imagesByPage)
	}
	if pageCount == 0 {
		return nil, fmt.Errorf("could not read any text or images from the PDF")
	}

	for p := 1; p <= pageCount; p++ {
		text := strings.TrimSpace(pageText[p])
		room := domain.Room{
			ID:        fmt.Sprintf("page-%d", p),
			Name:      fmt.Sprintf("Page %d", p),
			ReadAloud: text,
			DMNotes:   "Imported from PDF page " + strconv.Itoa(p),
		}
		if imgs := imagesByPage[p]; len(imgs) > 0 {
			room.Image = imgs[0]
			for _, extra := range imgs {
				adv.Images = append(adv.Images, domain.ImageRef{
					ID: slug(strings.TrimSuffix(filepath.Base(extra), filepath.Ext(extra))), Path: extra, Kind: "art",
				})
			}
		}
		zone.Rooms = append(zone.Rooms, room)
	}
	// Images we could not attribute to a page still go in the catalog.
	adv.Images = append(adv.Images, catalog...)

	// Seed the summary from the first page's text.
	if first := strings.TrimSpace(pageText[1]); first != "" {
		adv.Summary = truncate(first, 400)
	}
	return adv, nil
}

// extractPDFText returns per-page plain text, recovering from parser panics.
func extractPDFText(pdfPath string) map[int]string {
	out := map[int]string{}
	func() {
		defer func() { _ = recover() }()
		f, r, err := pdfread.Open(pdfPath)
		if err != nil {
			return
		}
		defer f.Close()
		n := r.NumPage()
		for i := 1; i <= n; i++ {
			func() {
				defer func() { _ = recover() }()
				p := r.Page(i)
				if p.V.IsNull() {
					return
				}
				if txt, err := p.GetPlainText(nil); err == nil {
					out[i] = txt
				}
			}()
		}
	}()
	return out
}

// extractPDFImages extracts embedded images to workingDir/assets/pdf/ and maps
// them to page numbers (parsed from pdfcpu's output filenames). Images whose
// page can't be determined are returned in the catalog slice.
func extractPDFImages(pdfPath, workingDir string) (map[int][]string, []domain.ImageRef) {
	byPage := map[int][]string{}
	var catalog []domain.ImageRef

	relDir := filepath.Join("assets", "pdf")
	outDir := filepath.Join(workingDir, relDir)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return byPage, catalog
	}
	conf := pdfmodel.NewDefaultConfiguration()
	if err := api.ExtractImagesFile(pdfPath, outDir, nil, conf); err != nil {
		return byPage, catalog
	}

	entries, _ := os.ReadDir(outDir)
	count := 0
	for _, e := range entries {
		if e.IsDir() || !isImage(e.Name()) {
			continue
		}
		count++
		if count > maxImages {
			break
		}
		rel := filepath.ToSlash(filepath.Join(relDir, e.Name()))
		if m := pageImgRe.FindStringSubmatch(e.Name()); m != nil {
			if pg, err := strconv.Atoi(m[1]); err == nil {
				byPage[pg] = append(byPage[pg], rel)
				continue
			}
		}
		catalog = append(catalog, domain.ImageRef{
			ID: slug(strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))), Path: rel, Kind: "art",
		})
	}
	return byPage, catalog
}

// --- helpers -------------------------------------------------------------

func newScaffold(title string) *domain.Adventure {
	return &domain.Adventure{
		SchemaVersion: domain.SchemaVersion,
		ID:            slug(title),
		Title:         title,
		System:        "D&D 5e",
		Zones: []domain.Zone{{
			ID:    "imported",
			Name:  "Imported",
			Rooms: []domain.Room{{ID: "room1", Name: "Room 1"}},
		}},
	}
}

// copyImage copies src into workingDir/assets/<kind>/ (avoiding name clashes)
// and returns the module-relative slash path.
func copyImage(src, workingDir, kind string) (string, error) {
	relDir := filepath.Join("assets", kind)
	destDir := filepath.Join(workingDir, relDir)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", err
	}
	base := filepath.Base(src)
	dest := filepath.Join(destDir, base)
	for i := 1; fileExists(dest); i++ {
		ext := filepath.Ext(base)
		dest = filepath.Join(destDir, fmt.Sprintf("%s-%d%s", strings.TrimSuffix(base, ext), i, ext))
	}
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return "", err
	}
	return filepath.ToSlash(filepath.Join(relDir, filepath.Base(dest))), nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func uniqueRoomID(adv *domain.Adventure, base string) string {
	if base == "" {
		base = "room"
	}
	id := base
	for i := 1; ; i++ {
		if r, _ := adv.Room(id); r == nil {
			return id
		}
		id = fmt.Sprintf("%s-%d", base, i)
	}
}

func prettyName(filename string) string {
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	name = strings.NewReplacer("_", " ", "-", " ").Replace(name)
	name = strings.TrimSpace(name)
	if name == "" {
		return filename
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

func truncate(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

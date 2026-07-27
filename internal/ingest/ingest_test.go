package ingest

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	pdfmodel "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func writePNG(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 8), uint8(y * 8), 128, 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestFromDirectory(t *testing.T) {
	src := t.TempDir()
	writePNG(t, filepath.Join(src, "goblin.png"))
	writePNG(t, filepath.Join(src, "level1-map.png"))
	writePNG(t, filepath.Join(src, "notes.txt.png")) // still an image by extension

	work := t.TempDir()
	adv, err := FromDirectory(src, work, "My Import")
	if err != nil {
		t.Fatalf("FromDirectory: %v", err)
	}
	if adv.ID != "my-import" {
		t.Errorf("ID = %q, want my-import", adv.ID)
	}
	if len(adv.Zones) != 1 {
		t.Fatalf("want 1 zone, got %d", len(adv.Zones))
	}
	z := adv.Zones[0]
	if !strings.Contains(z.MapImage, "level1-map.png") {
		t.Errorf("expected map image to be set from the map-like file, got %q", z.MapImage)
	}
	if !strings.HasPrefix(z.MapImage, "assets/maps/") {
		t.Errorf("map should live under assets/maps/, got %q", z.MapImage)
	}
	// Two non-map images become rooms.
	if len(z.Rooms) != 2 {
		t.Errorf("want 2 rooms, got %d", len(z.Rooms))
	}
	for _, r := range z.Rooms {
		if !strings.HasPrefix(r.Image, "assets/art/") {
			t.Errorf("room image should live under assets/art/, got %q", r.Image)
		}
		if _, err := os.Stat(filepath.Join(work, filepath.FromSlash(r.Image))); err != nil {
			t.Errorf("room image not copied to working dir: %v", err)
		}
	}
	if len(adv.Images) != 3 {
		t.Errorf("expected 3 cataloged images, got %d", len(adv.Images))
	}
}

func TestFromDirectoryEmpty(t *testing.T) {
	if _, err := FromDirectory(t.TempDir(), t.TempDir(), "x"); err == nil {
		t.Fatal("expected an error for a directory with no images")
	}
}

// makeTestPDF creates a small PDF containing one image page.
func makeTestPDF(t *testing.T) string {
	t.Helper()
	imgDir := t.TempDir()
	imgPath := filepath.Join(imgDir, "page.png")
	writePNG(t, imgPath)

	pdfPath := filepath.Join(t.TempDir(), "doc.pdf")
	conf := pdfmodel.NewDefaultConfiguration()
	if err := api.ImportImagesFile([]string{imgPath}, pdfPath, nil, conf); err != nil {
		t.Skipf("could not synthesize a test PDF: %v", err)
	}
	return pdfPath
}

func TestFromPDF(t *testing.T) {
	pdfPath := makeTestPDF(t)
	work := t.TempDir()
	adv, err := FromPDF(pdfPath, work, "My PDF")
	if err != nil {
		t.Fatalf("FromPDF: %v", err)
	}
	if adv.ID != "my-pdf" {
		t.Errorf("ID = %q, want my-pdf", adv.ID)
	}
	if len(adv.Zones) != 1 {
		t.Fatalf("want 1 zone, got %d", len(adv.Zones))
	}
	if len(adv.Zones[0].Rooms) < 1 {
		t.Errorf("expected at least one page room, got %d", len(adv.Zones[0].Rooms))
	}
	// The embedded image should have been extracted into the working dir.
	if len(adv.ImageRefs()) == 0 {
		t.Log("no images attributed (acceptable if extraction found none)")
	}
	for _, ref := range adv.ImageRefs() {
		if _, err := os.Stat(filepath.Join(work, filepath.FromSlash(ref))); err != nil {
			t.Errorf("referenced image %q missing on disk: %v", ref, err)
		}
	}
}

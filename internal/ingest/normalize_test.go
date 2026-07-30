package ingest

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	hhtiff "github.com/hhrutter/tiff"
)

// solid returns an image filled with c.
func solid(w, h int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func TestIsBlank(t *testing.T) {
	// A near-white page (paper/texture layer) is blank.
	white := solid(400, 400, color.White)
	if !isBlank(white) {
		t.Error("all-white image should be blank")
	}

	// An off-white texture with faint grey mottling is still blank.
	texture := solid(400, 400, color.Gray{Y: 243})
	for i := 0; i < 400*400/20; i++ { // sprinkle light-grey (not dark) noise
		texture.Set(i%400, (i*7)%400, color.Gray{Y: 225})
	}
	if !isBlank(texture) {
		t.Error("faint light-grey texture should be blank")
	}

	// An image with substantial dark content is not blank.
	content := solid(400, 400, color.White)
	for y := 100; y < 300; y++ { // a dark block covering 25% of the image
		for x := 100; x < 300; x++ {
			content.Set(x, y, color.Black)
		}
	}
	if isBlank(content) {
		t.Error("image with a large dark region should not be blank")
	}

	// A subject on a transparent background is judged on the subject.
	subject := image.NewRGBA(image.Rect(0, 0, 400, 400)) // all transparent
	for y := 150; y < 250; y++ {
		for x := 150; x < 250; x++ {
			subject.Set(x, y, color.Black)
		}
	}
	if isBlank(subject) {
		t.Error("dark subject on transparent background should not be blank")
	}
}

// TestNormalizeImageFileTIFF verifies a TIFF is transcoded to a downscaled PNG,
// the original removed, and blankness reported.
func TestNormalizeImageFileTIFF(t *testing.T) {
	dir := t.TempDir()

	// A large content TIFF → should be kept, transcoded to PNG, downscaled.
	big := solid(3000, 2000, color.White)
	for y := 0; y < 2000; y++ {
		for x := 0; x < 1500; x++ { // dark half
			big.Set(x, y, color.Black)
		}
	}
	tifPath := filepath.Join(dir, "content.tif")
	writeTIFF(t, tifPath, big)

	name, keep := normalizeImageFile(tifPath)
	if !keep {
		t.Error("content TIFF should be kept")
	}
	if filepath.Ext(name) != ".png" {
		t.Errorf("expected .png output, got %q", name)
	}
	if _, err := os.Stat(tifPath); !os.IsNotExist(err) {
		t.Error("original .tif should be removed after transcode")
	}
	out := filepath.Join(dir, name)
	f, err := os.Open(out)
	if err != nil {
		t.Fatalf("open transcoded png: %v", err)
	}
	defer f.Close()
	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatalf("decode transcoded png: %v", err)
	}
	if format != "png" {
		t.Errorf("expected png, got %s", format)
	}
	if cfg.Width > maxImageDim || cfg.Height > maxImageDim {
		t.Errorf("expected downscale to <= %d, got %dx%d", maxImageDim, cfg.Width, cfg.Height)
	}

	// A blank TIFF → transcoded (file valid) but reported not-keep.
	blankPath := filepath.Join(dir, "blank.tif")
	writeTIFF(t, blankPath, solid(800, 800, color.White))
	if _, keep := normalizeImageFile(blankPath); keep {
		t.Error("blank TIFF should be reported not-keep")
	}
}

func writeTIFF(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := hhtiff.Encode(f, img, nil); err != nil {
		t.Fatal(err)
	}
}

func writePNGFile(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

// TestNormalizeImageFilePNGSmallUnchanged verifies a small web-friendly image is
// left untouched (no needless re-encode) but still assessed for blankness.
func TestNormalizeImageFilePNGSmallUnchanged(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "small.png")
	img := solid(300, 300, color.White)
	for y := 0; y < 150; y++ {
		for x := 0; x < 300; x++ {
			img.Set(x, y, color.Black)
		}
	}
	writePNGFile(t, p, img)
	before, _ := os.Stat(p)
	name, keep := normalizeImageFile(p)
	if name != "small.png" {
		t.Errorf("expected unchanged name, got %q", name)
	}
	if !keep {
		t.Error("half-dark image should be kept")
	}
	after, _ := os.Stat(p)
	if before.ModTime() != after.ModTime() {
		t.Error("small web-friendly image should not be re-encoded")
	}
}

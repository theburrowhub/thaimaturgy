package ingest

import (
	"image"
	"image/png"
	"os"
	"path"
	"path/filepath"
	"strings"

	hhtiff "github.com/hhrutter/tiff"
	"github.com/theburrowhub/thaimaturgy/internal/domain"
	_ "github.com/theburrowhub/thaimaturgy/internal/imagefmt" // decoders: TIFF (incl. CMYK), WebP, BMP

	"golang.org/x/image/draw"
)

// maxImageDim is the longest-side pixel budget for normalized images. PDF page
// scans are frequently 2500×3300+; anything above this is downscaled. It keeps
// maps/art crisp enough to read while staying small enough to render fast and
// fit vision-model size limits.
const maxImageDim = 1600

// Thresholds for judging an image "blank" — a paper/parchment texture or overlay
// layer that PDFs composite behind real art. pdfcpu extracts every image XObject
// separately, so these near-empty layers show up as their own files. A real map,
// portrait, or scene always carries dark ink (lines, text, shadows); a blank layer
// has essentially none and is overwhelmingly light.
const (
	blankDarkMaxFrac  = 0.005 // ≤0.5% of opaque pixels are genuinely dark
	blankLightMinFrac = 0.75  // ≥75% of opaque pixels are near-white
)

// webFriendly are formats that render everywhere and need no transcoding when
// already reasonably sized. TIFF/BMP/WebP are not: Fyne (and browsers) can't
// display TIFF at all, so those are always converted to PNG.
var webFriendly = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true}

// normalizeImageFile makes an extracted image portable and light: it decodes the
// source (handling CMYK TIFF via the registered hhrutter decoder), downscales it
// to maxImageDim on the longest side when oversized, and — for any non
// web-friendly or oversized image — re-encodes it as PNG next to the original,
// removing the original when the extension changed. The file at the returned name
// always exists and is renderable.
//
// The second return value reports whether the image looks like usable content
// (keep=true) or a near-blank texture/overlay layer (keep=false). It never deletes
// based on that judgement — the caller decides — so callers importing user-curated
// images can ignore it while the PDF extractor drops the junk layers.
//
// It degrades gracefully: on any decode/encode failure the original file is left
// untouched, its basename is returned, and keep=true (never lose an image we
// simply couldn't assess).
func normalizeImageFile(absPath string) (string, bool) {
	ext := strings.ToLower(filepath.Ext(absPath))
	orig := filepath.Base(absPath)

	img, err := decodeImage(absPath, ext)
	if err != nil {
		ingestLog.Printf("normalize: cannot decode %s (%v); leaving as-is", orig, err)
		return orig, true
	}

	keep := !isBlank(img)

	b := img.Bounds()
	oversized := b.Dx() > maxImageDim || b.Dy() > maxImageDim
	if webFriendly[ext] && !oversized {
		return orig, keep // already fine; no re-encode needed
	}

	img = downscale(img, maxImageDim)
	dest := absPath
	if !webFriendly[ext] {
		dest = strings.TrimSuffix(absPath, filepath.Ext(absPath)) + ".png"
	}
	out, err := os.Create(dest)
	if err != nil {
		ingestLog.Printf("normalize: cannot create %s (%v); leaving as-is", filepath.Base(dest), err)
		return orig, keep
	}
	if err := png.Encode(out, img); err != nil {
		out.Close()
		_ = os.Remove(dest)
		ingestLog.Printf("normalize: cannot encode %s (%v); leaving as-is", filepath.Base(dest), err)
		return orig, keep
	}
	if err := out.Close(); err != nil {
		return orig, keep
	}
	if dest != absPath {
		_ = os.Remove(absPath) // drop the un-renderable original (e.g. TIFF)
	}
	return filepath.Base(dest), keep
}

// NormalizeModuleImages makes every image referenced by adv portable and light,
// in place under workingDir, and updates adv's references accordingly:
//
//   - TIFF (incl. CMYK page scans) → PNG; oversized images downscaled.
//   - Near-blank paper/texture/overlay layers are deleted, dropped from the image
//     catalog, and their ids pruned from every entity that referenced them.
//
// It is idempotent: already-normalized PNGs are left untouched. Returns how many
// images were transcoded and how many blank layers were dropped. Use it when a
// module is imported/opened from a pre-built .tar.gz, which otherwise bypasses the
// PDF/folder ingest pipeline where this normalization already happens.
func NormalizeModuleImages(workingDir string, adv *domain.Adventure) (transcoded, dropped int) {
	if adv == nil || workingDir == "" {
		return 0, 0
	}
	rename := map[string]string{} // original rel → normalized rel
	drop := map[string]bool{}     // original rel → should be removed

	for _, rel := range adv.ImageRefs() {
		abs := filepath.Join(workingDir, filepath.FromSlash(rel))
		if info, err := os.Stat(abs); err != nil || info.IsDir() {
			continue
		}
		newBase, keep := normalizeImageFile(abs)
		newRel := path.Join(path.Dir(rel), newBase)
		if newRel != rel {
			rename[rel] = newRel
			transcoded++
		}
		if !keep {
			drop[rel] = true
			_ = os.Remove(filepath.Join(workingDir, filepath.FromSlash(newRel)))
			dropped++
		}
	}
	applyImageEdits(adv, rename, drop)
	return transcoded, dropped
}

// applyImageEdits rewrites adv's image references: renamed files get their new
// path, dropped files are removed from the catalog and cleared from direct fields,
// and any image ids whose catalog entry was dropped are pruned from every entity.
func applyImageEdits(adv *domain.Adventure, rename map[string]string, drop map[string]bool) {
	if len(rename) == 0 && len(drop) == 0 {
		return
	}
	removedID := map[string]bool{}
	kept := adv.Images[:0]
	for _, img := range adv.Images {
		if drop[img.Path] {
			if img.ID != "" {
				removedID[img.ID] = true
			}
			continue
		}
		if nr, ok := rename[img.Path]; ok {
			img.Path = nr
		}
		kept = append(kept, img)
	}
	adv.Images = kept

	fixPath := func(p string) string {
		if p == "" {
			return ""
		}
		if drop[p] {
			return ""
		}
		if nr, ok := rename[p]; ok {
			return nr
		}
		return p
	}
	pruneIDs := func(ids []string) []string {
		if len(ids) == 0 {
			return ids
		}
		out := ids[:0]
		for _, id := range ids {
			if !removedID[id] {
				out = append(out, id)
			}
		}
		return out
	}

	for zi := range adv.Zones {
		adv.Zones[zi].MapImage = fixPath(adv.Zones[zi].MapImage)
		adv.Zones[zi].ImageIDs = pruneIDs(adv.Zones[zi].ImageIDs)
		for ri := range adv.Zones[zi].Rooms {
			adv.Zones[zi].Rooms[ri].Image = fixPath(adv.Zones[zi].Rooms[ri].Image)
			adv.Zones[zi].Rooms[ri].ImageIDs = pruneIDs(adv.Zones[zi].Rooms[ri].ImageIDs)
		}
	}
	for ni := range adv.NPCs {
		adv.NPCs[ni].Image = fixPath(adv.NPCs[ni].Image)
		adv.NPCs[ni].ImageIDs = pruneIDs(adv.NPCs[ni].ImageIDs)
	}
	for ii := range adv.Items {
		adv.Items[ii].Image = fixPath(adv.Items[ii].Image)
		adv.Items[ii].ImageIDs = pruneIDs(adv.Items[ii].ImageIDs)
	}
}

// decodeImage decodes an image file. TIFF is decoded explicitly with hhrutter
// (which handles CMYK, unlike golang.org/x/image/tiff) rather than through the
// image.Decode registry, so the result never depends on which TIFF decoder some
// other package happened to register first. Other formats go through image.Decode
// (imagefmt registers WebP/BMP/PNG/JPEG/GIF).
func decodeImage(absPath, ext string) (image.Image, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if ext == ".tif" || ext == ".tiff" {
		return hhtiff.Decode(f)
	}
	img, _, err := image.Decode(f)
	return img, err
}

// isBlank reports whether img is a near-empty paper/texture/overlay layer: almost
// no genuinely-dark pixels and overwhelmingly light. Fully-transparent pixels are
// ignored so a subject on a transparent background is judged on the subject alone.
func isBlank(img image.Image) bool {
	b := img.Bounds()
	if b.Empty() {
		return true
	}
	sx := b.Dx()/200 + 1
	sy := b.Dy()/200 + 1
	var dark, light, opaque float64
	for y := b.Min.Y; y < b.Max.Y; y += sy {
		for x := b.Min.X; x < b.Max.X; x += sx {
			r, g, bl, a := img.At(x, y).RGBA()
			if a>>8 < 16 {
				continue // transparent — not part of the visible content
			}
			opaque++
			lum := (299*uint64(r>>8) + 587*uint64(g>>8) + 114*uint64(bl>>8)) / 1000
			switch {
			case lum < 110:
				dark++
			case lum >= 220:
				light++
			}
		}
	}
	if opaque == 0 {
		return true // fully transparent
	}
	return dark/opaque < blankDarkMaxFrac && light/opaque > blankLightMinFrac
}

// downscale returns img shrunk so its longest side is at most maxDim, preserving
// aspect ratio. Images already within budget are returned unchanged.
func downscale(img image.Image, maxDim int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxDim && h <= maxDim {
		return img
	}
	nw, nh := w, h
	if w >= h {
		nw = maxDim
		nh = h * maxDim / w
	} else {
		nh = maxDim
		nw = w * maxDim / h
	}
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
	return dst
}

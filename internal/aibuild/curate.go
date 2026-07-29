package aibuild

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/ingest"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
)

const (
	curationBatch     = 6  // images classified per vision call
	maxCurationImages = 30 // overall cap on images to classify
)

// asset is an extracted image plus the AI's classification of it. id is a stable
// catalog id (referenced from entities via image_ids).
type asset struct {
	id    string
	rel   string
	page  int
	kind  string // map | portrait | scene | item | decorative
	title string
	desc  string
}

func (a asset) isMap() bool      { return a.kind == "map" }
func (a asset) decorative() bool { return a.kind == "decorative" }

func toAssets(in []ingest.Asset) []asset {
	out := make([]asset, len(in))
	seen := make(map[string]bool)
	for i, a := range in {
		kind := "scene"
		if a.IsMap {
			kind = "map"
		}
		base := slug(strings.TrimSuffix(filepath.Base(a.RelPath), filepath.Ext(a.RelPath)))
		if base == "" {
			base = "image"
		}
		id := base
		for n := 2; seen[id]; n++ {
			id = fmt.Sprintf("%s-%d", base, n)
		}
		seen[id] = true
		out[i] = asset{id: id, rel: a.RelPath, page: a.Page, kind: kind}
	}
	return out
}

// curateAssets asks a vision model to classify every extracted image (map /
// portrait / scene / item / decorative) and caption it, then drops decorative
// junk (and deletes those files). It degrades gracefully: any failure leaves the
// assets with their filename-based defaults and drops nothing.
func curateAssets(ctx context.Context, prov providers.Provider, model, workingDir string, assets []asset, maxBytes int, progress Progress) []asset {
	if prov == nil || len(assets) == 0 {
		return assets
	}

	classified := 0
	for start := 0; start < len(assets) && classified < maxCurationImages; start += curationBatch {
		end := min(start+curationBatch, len(assets))
		var imgs []providers.ImageData
		var targets []*asset
		for i := start; i < end && classified < maxCurationImages; i++ {
			mt := mediaType(assets[i].rel)
			if mt == "" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(workingDir, filepath.FromSlash(assets[i].rel)))
			if err != nil || len(data) == 0 || len(data) > maxBytes {
				continue
			}
			imgs = append(imgs, providers.ImageData{MediaType: mt, Data: data})
			targets = append(targets, &assets[i])
			classified++
		}
		if len(imgs) > 0 {
			report(progress, "Classifying images %d–%d of %d…", start+1, start+len(imgs), len(assets))
			classifyBatch(ctx, prov, model, imgs, targets)
		}
	}

	// Drop decorative junk and remove those files from the module.
	kept := assets[:0]
	for _, a := range assets {
		if a.decorative() {
			_ = os.Remove(filepath.Join(workingDir, filepath.FromSlash(a.rel)))
			continue
		}
		kept = append(kept, a)
	}
	return kept
}

type classification struct {
	Index       int    `json:"index"`
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

func classifyBatch(ctx context.Context, prov providers.Provider, model string, imgs []providers.ImageData, targets []*asset) {
	prompt := fmt.Sprintf(`These are %d images extracted from a tabletop RPG (D&D) source document, given in order (Image 1..%d).
Classify EACH image. Reply with ONLY a JSON array, one object per image, in order:
[{"index":1,"kind":"map|portrait|scene|item|decorative","title":"short name","description":"one concise line"}]

kind meanings:
- map: a battle map, region/dungeon map, or floor plan.
- portrait: a character/creature/NPC illustration.
- scene: an environment or scene illustration.
- item: an object, artifact, or handout.
- decorative: borders, logos, textures, page ornaments, or anything not usable as adventure content.`, len(imgs), len(imgs))

	resp, err := prov.Chat(ctx, providers.ChatRequest{
		Model:     model,
		MaxTokens: 1200,
		Messages: []providers.Message{
			{Role: providers.RoleSystem, Content: "You are a precise visual classifier for tabletop RPG art and maps."},
			{Role: providers.RoleUser, Content: prompt, Images: imgs},
		},
	})
	if err != nil {
		return
	}
	for _, c := range parseClassifications(resp.Content) {
		i := c.Index - 1
		if i < 0 || i >= len(targets) {
			continue
		}
		if k := normalizeKind(c.Kind); k != "" {
			targets[i].kind = k
		}
		targets[i].title = strings.TrimSpace(c.Title)
		targets[i].desc = strings.TrimSpace(c.Description)
	}
}

func parseClassifications(content string) []classification {
	s := strings.TrimSpace(content)
	if i := strings.Index(s, "```"); i >= 0 {
		s = strings.TrimPrefix(s[i+3:], "json")
		if j := strings.LastIndex(s, "```"); j >= 0 {
			s = s[:j]
		}
	}
	start := strings.Index(s, "[")
	end := strings.LastIndex(s, "]")
	if start < 0 || end <= start {
		return nil
	}
	var out []classification
	if err := json.Unmarshal([]byte(s[start:end+1]), &out); err != nil {
		return nil
	}
	return out
}

func normalizeKind(k string) string {
	switch strings.ToLower(strings.TrimSpace(k)) {
	case "map", "maps", "battlemap", "floorplan", "floor plan":
		return "map"
	case "portrait", "character", "npc", "creature", "monster":
		return "portrait"
	case "scene", "environment", "location", "landscape":
		return "scene"
	case "item", "object", "artifact", "handout":
		return "item"
	case "decorative", "decoration", "border", "logo", "texture", "ornament", "junk":
		return "decorative"
	}
	return ""
}

// enrichCatalog updates the adventure's image catalog with the AI's captions and
// kinds for the assets that survived curation, so the module carries useful
// descriptions rather than bare paths.
func enrichCatalog(adv *domain.Adventure, assets []asset) {
	byPath := make(map[string]asset, len(assets))
	for _, a := range assets {
		byPath[a.rel] = a
	}

	// Update existing catalog entries.
	seen := make(map[string]bool)
	for i := range adv.Images {
		if a, ok := byPath[adv.Images[i].Path]; ok {
			if adv.Images[i].ID == "" {
				adv.Images[i].ID = a.id
			}
			adv.Images[i].Kind = catalogKind(a)
			if d := caption(a); d != "" {
				adv.Images[i].Description = d
			}
			seen[a.rel] = true
		}
	}
	// Append any curated asset the model didn't already catalog.
	for _, a := range assets {
		if seen[a.rel] {
			continue
		}
		adv.Images = append(adv.Images, domain.ImageRef{
			ID:          a.id,
			Path:        a.rel,
			Kind:        catalogKind(a),
			Description: caption(a),
		})
	}
}

// dropUnknownImageIDs removes any image_ids the model invented that don't match
// a catalog image, keeping referential integrity.
func dropUnknownImageIDs(adv *domain.Adventure) {
	valid := make(map[string]bool, len(adv.Images))
	for _, img := range adv.Images {
		valid[img.ID] = true
	}
	keep := func(ids []string) []string {
		var out []string
		for _, id := range ids {
			if valid[id] {
				out = append(out, id)
			}
		}
		return out
	}
	for zi := range adv.Zones {
		adv.Zones[zi].ImageIDs = keep(adv.Zones[zi].ImageIDs)
		for ri := range adv.Zones[zi].Rooms {
			adv.Zones[zi].Rooms[ri].ImageIDs = keep(adv.Zones[zi].Rooms[ri].ImageIDs)
		}
	}
	for ni := range adv.NPCs {
		adv.NPCs[ni].ImageIDs = keep(adv.NPCs[ni].ImageIDs)
	}
	for ii := range adv.Items {
		adv.Items[ii].ImageIDs = keep(adv.Items[ii].ImageIDs)
	}
}

func catalogKind(a asset) string {
	if a.isMap() {
		return "map"
	}
	return "art"
}

func caption(a asset) string {
	switch {
	case a.title != "" && a.desc != "":
		return a.title + " — " + a.desc
	case a.title != "":
		return a.title
	default:
		return a.desc
	}
}

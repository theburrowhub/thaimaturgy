// Package aibuild turns raw source material (a PDF, or a folder of images) into
// a structured adventure module by having an AI model interpret the document's
// text and images. Extracted art and maps are referenced back into the
// generated zones, rooms and NPCs. It is UI-agnostic and works with any
// providers.Provider (using vision when the model supports it).
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

// Default import caps, used when the config leaves them unset.
const (
	defVisionMaxImages  = 10
	defVisionMaxImageMB = 4
	defMaxDocChars      = 90_000
	defMaxOutputTokens  = 8000
)

// limits resolves the effective import caps from config, applying defaults.
type limits struct {
	visionMaxImages int
	maxImageBytes   int
	maxDocChars     int
	maxOutputTokens int
}

func limitsFrom(cfg *domain.Config) limits {
	l := limits{defVisionMaxImages, defVisionMaxImageMB << 20, defMaxDocChars, defMaxOutputTokens}
	if cfg == nil {
		return l
	}
	if cfg.ImportVisionMaxImages > 0 {
		l.visionMaxImages = cfg.ImportVisionMaxImages
	}
	if cfg.ImportVisionMaxImageMB > 0 {
		l.maxImageBytes = cfg.ImportVisionMaxImageMB << 20
	}
	if cfg.ImportMaxDocChars > 0 {
		l.maxDocChars = cfg.ImportMaxDocChars
	}
	if cfg.ImportMaxOutputTokens > 0 {
		l.maxOutputTokens = cfg.ImportMaxOutputTokens
	}
	return l
}

// FromPDF extracts a PDF and asks the model to build an adventure from it.
func FromPDF(ctx context.Context, prov providers.Provider, cfg *domain.Config, pdfPath, workingDir, title string) (*domain.Adventure, error) {
	if prov == nil {
		return nil, fmt.Errorf("no AI provider configured; set an API key first")
	}
	text, assets, err := ingest.ExtractPDF(pdfPath, workingDir)
	if err != nil {
		return nil, err
	}
	return build(ctx, prov, cfg, title, text, assets, workingDir)
}

// FromImages copies a folder of images and asks the model to build an adventure
// by interpreting them visually.
func FromImages(ctx context.Context, prov providers.Provider, cfg *domain.Config, srcDir, workingDir, title string) (*domain.Adventure, error) {
	if prov == nil {
		return nil, fmt.Errorf("no AI provider configured; set an API key first")
	}
	assets, err := ingest.CollectDirImages(srcDir, workingDir)
	if err != nil {
		return nil, err
	}
	return build(ctx, prov, cfg, title, "", assets, workingDir)
}

func build(ctx context.Context, prov providers.Provider, cfg *domain.Config, title, docText string, assets []ingest.Asset, workingDir string) (*domain.Adventure, error) {
	lim := limitsFrom(cfg)
	model := ""
	if cfg != nil {
		model = cfg.Model
	}
	user := buildUserPrompt(title, docText, assets, lim.maxDocChars)
	images := loadVisionImages(workingDir, assets, lim.visionMaxImages, lim.maxImageBytes)

	req := providers.ChatRequest{
		Model:       model,
		Temperature: 0.4,
		MaxTokens:   lim.maxOutputTokens,
		Messages: []providers.Message{
			{Role: providers.RoleSystem, Content: systemPrompt},
			{Role: providers.RoleUser, Content: user, Images: images},
		},
	}

	resp, err := prov.Chat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("AI request failed: %w", err)
	}
	adv, err := parseAdventure(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("the model did not return a usable adventure: %w", err)
	}
	sanitize(adv, title, workingDir)
	return adv, nil
}

const systemPrompt = `You are an expert tabletop RPG (D&D 5e) module designer. You receive the raw text and images extracted from a source document (an adventure PDF or a set of images). Interpret ALL of it and produce a single, complete adventure module as JSON.

Output ONLY a JSON object (no prose, no markdown fences) with this shape:
{
  "schema_version":"1.0","id":"kebab-case-id","title":"...","author":"","system":"D&D 5e","language":"en",
  "summary":"...","background":"hidden DM lore","introduction":"how it starts","conclusion":"possible endings","hooks":["..."],
  "zones":[{"id":"...","name":"...","overview":"DM summary","description":"...","map_image":"assets/....png","connections":["zoneId"],
    "rooms":[{"id":"...","name":"...","read_aloud":"boxed text for players","dm_notes":"secrets/what happens","image":"assets/....png",
      "npc_ids":["..."],"event_ids":["..."],"treasure":["..."],
      "exits":[{"to":"roomOrZoneId","direction":"north","description":"...","locked":false}],
      "features":[{"name":"...","description":"...","skill":"Perception","dc":13,"success":"...","failure":"..."}],
      "encounters":[{"name":"...","description":"...","creatures":["..."],"difficulty":"medium","tactics":"..."}]}]}],
  "npcs":[{"id":"...","name":"...","role":"...","appearance":"...","personality":"...","motivations":"...","secrets":"...","voice":"...",
    "disposition":"...","default_location":"roomId","image":"assets/....png","knowledge":["..."],"sample_dialogue":["..."],
    "stat_block":{"ac":13,"max_hp":22,"speed":"30 ft","cr":"1","abilities":{"str":10,"dex":10,"con":10,"int":10,"wis":10,"cha":10},
      "skills":["..."],"traits":["..."],"actions":[{"name":"...","to_hit":"+4","damage":"1d8+2 slashing","description":"..."}]}}],
  "events":[{"id":"...","name":"...","trigger":"...","description":"...","read_aloud":"...","dm_notes":"...","consequences":"...",
    "outcomes":[{"condition":"...","result":"..."}]}],
  "items":[{"id":"...","name":"...","description":"...","rarity":"...","mechanics":"...","image":"assets/....png"}]
}

RULES:
- Ground everything in the provided material; do not invent a different adventure. Preserve names, places, NPCs, and plot.
- Split the content into coherent zones and rooms. Give NPCs motivations, secrets and voice for roleplay, plus a stat block when the source implies combat.
- IMAGE REFERENCES: you are given a list of extracted image files with their relative paths and source page. Reference them EXACTLY by those relative paths — use map-like images as a zone "map_image" and character/scene art as a room, NPC, or item "image". Only reference paths from the provided list; never invent paths.
- Every id must be unique and kebab-case. Every reference (npc_ids, event_ids, exit "to", default_location) must point to an id you actually define.
- Return valid JSON only.`

func buildUserPrompt(title, docText string, assets []ingest.Asset, maxDocChars int) string {
	var sb strings.Builder
	if title != "" {
		fmt.Fprintf(&sb, "Suggested title: %s\n\n", title)
	}
	sb.WriteString("EXTRACTED IMAGE FILES (reference these relative paths exactly):\n")
	if len(assets) == 0 {
		sb.WriteString("(none)\n")
	}
	for _, a := range assets {
		kind := "art"
		if a.IsMap {
			kind = "map (likely)"
		}
		if a.Page > 0 {
			fmt.Fprintf(&sb, "- %s (page %d, %s)\n", a.RelPath, a.Page, kind)
		} else {
			fmt.Fprintf(&sb, "- %s (%s)\n", a.RelPath, kind)
		}
	}
	if strings.TrimSpace(docText) != "" {
		sb.WriteString("\nDOCUMENT TEXT:\n")
		sb.WriteString(truncate(docText, maxDocChars))
	} else {
		sb.WriteString("\n(There is no extracted text; interpret the attached images to design the adventure.)\n")
	}
	sb.WriteString("\n\nProduce the adventure JSON now.")
	return sb.String()
}

// loadVisionImages reads up to maxVisionImages image files for the model to see,
// preferring maps first, skipping oversized files and formats vision APIs reject.
func loadVisionImages(workingDir string, assets []ingest.Asset, maxImages, maxBytes int) []providers.ImageData {
	ordered := append([]ingest.Asset{}, assets...)
	// Maps first — they carry the most structural information.
	maps, rest := []ingest.Asset{}, []ingest.Asset{}
	for _, a := range ordered {
		if a.IsMap {
			maps = append(maps, a)
		} else {
			rest = append(rest, a)
		}
	}
	ordered = append(maps, rest...)

	var out []providers.ImageData
	for _, a := range ordered {
		if len(out) >= maxImages {
			break
		}
		mt := mediaType(a.RelPath)
		if mt == "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(workingDir, filepath.FromSlash(a.RelPath)))
		if err != nil || len(data) == 0 || len(data) > maxBytes {
			continue
		}
		out = append(out, providers.ImageData{MediaType: mt, Data: data})
	}
	return out
}

func mediaType(rel string) string {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	}
	return "" // tiff/bmp/etc. not sent to vision (still referenced as files)
}

// parseAdventure extracts a JSON object from the model's reply and unmarshals it.
func parseAdventure(content string) (*domain.Adventure, error) {
	s := strings.TrimSpace(content)
	// Strip ```json ... ``` fences if present.
	if i := strings.Index(s, "```"); i >= 0 {
		s = s[i+3:]
		s = strings.TrimPrefix(s, "json")
		if j := strings.LastIndex(s, "```"); j >= 0 {
			s = s[:j]
		}
	}
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("no JSON object found in the response")
	}
	var adv domain.Adventure
	if err := json.Unmarshal([]byte(s[start:end+1]), &adv); err != nil {
		return nil, err
	}
	return &adv, nil
}

// sanitize fixes up the model's output so it validates: fills required scalars,
// drops dangling references, and removes image paths that aren't on disk.
func sanitize(adv *domain.Adventure, title, workingDir string) {
	if adv.SchemaVersion == "" {
		adv.SchemaVersion = domain.SchemaVersion
	}
	if strings.TrimSpace(adv.Title) == "" {
		adv.Title = title
	}
	if strings.TrimSpace(adv.ID) == "" {
		adv.ID = slug(adv.Title)
	} else {
		adv.ID = slug(adv.ID)
	}
	if len(adv.Zones) == 0 {
		adv.Zones = []domain.Zone{{ID: "imported", Name: "Imported"}}
	}

	zoneIDs := map[string]bool{}
	roomIDs := map[string]bool{}
	npcIDs := map[string]bool{}
	eventIDs := map[string]bool{}
	for _, z := range adv.Zones {
		zoneIDs[z.ID] = true
		for _, r := range z.Rooms {
			roomIDs[r.ID] = true
		}
	}
	for _, n := range adv.NPCs {
		npcIDs[n.ID] = true
	}
	for _, ev := range adv.Events {
		eventIDs[ev.ID] = true
	}

	imgOK := func(rel string) bool {
		if rel == "" {
			return false
		}
		info, err := os.Stat(filepath.Join(workingDir, filepath.FromSlash(rel)))
		return err == nil && !info.IsDir()
	}

	for zi := range adv.Zones {
		z := &adv.Zones[zi]
		if !imgOK(z.MapImage) {
			z.MapImage = ""
		}
		z.Connections = keepStrings(z.Connections, func(s string) bool { return zoneIDs[s] })
		for ri := range z.Rooms {
			r := &z.Rooms[ri]
			if !imgOK(r.Image) {
				r.Image = ""
			}
			r.NPCIDs = keepStrings(r.NPCIDs, func(s string) bool { return npcIDs[s] })
			r.EventIDs = keepStrings(r.EventIDs, func(s string) bool { return eventIDs[s] })
			r.Exits = keepExits(r.Exits, func(to string) bool { return roomIDs[to] || zoneIDs[to] })
		}
	}
	for ni := range adv.NPCs {
		if !imgOK(adv.NPCs[ni].Image) {
			adv.NPCs[ni].Image = ""
		}
		if dl := adv.NPCs[ni].DefaultLocation; dl != "" && !roomIDs[dl] {
			adv.NPCs[ni].DefaultLocation = ""
		}
	}
	for ii := range adv.Items {
		if !imgOK(adv.Items[ii].Image) {
			adv.Items[ii].Image = ""
		}
	}
	// Keep only catalog entries that exist on disk.
	adv.Images = keepImageRefs(adv.Images, imgOK)
}

// --- helpers -------------------------------------------------------------

func keepStrings(in []string, keep func(string) bool) []string {
	var out []string
	for _, s := range in {
		if keep(s) {
			out = append(out, s)
		}
	}
	return out
}

func keepExits(in []domain.Exit, keep func(string) bool) []domain.Exit {
	var out []domain.Exit
	for _, e := range in {
		if e.To == "" || keep(e.To) {
			out = append(out, e)
		}
	}
	return out
}

func keepImageRefs(in []domain.ImageRef, keep func(string) bool) []domain.ImageRef {
	var out []domain.ImageRef
	for _, r := range in {
		if keep(r.Path) {
			out = append(out, r)
		}
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n…[truncated]"
}

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

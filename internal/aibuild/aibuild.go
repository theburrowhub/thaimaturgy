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
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/ingest"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
)

// Progress receives human-readable status lines as the import proceeds. It may
// be nil.
type Progress func(stage string)

var importLog = log.New(os.Stderr, "[import] ", log.LstdFlags)

// report logs a stage to stderr and forwards it to the progress callback.
func report(p Progress, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	importLog.Println(msg)
	if p != nil {
		p(msg)
	}
}

// Default import caps, used when the config leaves them unset.
const (
	defVisionMaxImages  = 10
	defVisionMaxImageMB = 4
	defMaxDocChars      = 200_000
	defMaxOutputTokens  = 32000
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
func FromPDF(ctx context.Context, prov providers.Provider, cfg *domain.Config, pdfPath, workingDir, title string, progress Progress, confirm ConfirmFallback, visionProv providers.Provider) (*domain.Adventure, error) {
	if prov == nil {
		return nil, fmt.Errorf("no AI provider configured; set an API key first")
	}
	report(progress, "Extracting text and images from the PDF…")
	text, assets, err := ingest.ExtractPDF(pdfPath, workingDir)
	if err != nil {
		return nil, err
	}
	report(progress, "Extracted %d image(s) and %d characters of text.", len(assets), len(text))
	if len(assets) == 0 {
		report(progress, "No embedded images could be extracted (the PDF may use vector art or full-page scans). Proceeding with text only.")
	}
	return build(ctx, prov, cfg, title, text, assets, workingDir, progress, confirm, visionProv)
}

// FromImages copies a folder of images and asks the model to build an adventure
// by interpreting them visually.
func FromImages(ctx context.Context, prov providers.Provider, cfg *domain.Config, srcDir, workingDir, title string, progress Progress, confirm ConfirmFallback, visionProv providers.Provider) (*domain.Adventure, error) {
	if prov == nil {
		return nil, fmt.Errorf("no AI provider configured; set an API key first")
	}
	report(progress, "Reading images from the folder…")
	assets, err := ingest.CollectDirImages(srcDir, workingDir)
	if err != nil {
		return nil, err
	}
	report(progress, "Found %d page image(s).", len(assets))

	// The images are scanned pages of a physical adventure book: transcribe each
	// page's full text with vision (OCR) so authoring has the complete text, just
	// as it would from a PDF's text layer.
	model := ""
	if cfg != nil {
		model = cfg.Model
	}
	docText := transcribePages(ctx, visionProviderFor(prov, visionProv), model, workingDir, assets, curationMaxBytes, progress)

	return build(ctx, prov, cfg, title, docText, assets, workingDir, progress, confirm, visionProv)
}

// ConfirmFallback is consulted when the configured model is unavailable and the
// provider would serve the request with a different (usually smaller) model. It
// receives the requested and the substitute model ids; returning true proceeds
// with the substitute, false aborts the import. A nil ConfirmFallback proceeds
// silently (the provider's own fallback applies).
type ConfirmFallback func(requested, served string) bool

func build(ctx context.Context, prov providers.Provider, cfg *domain.Config, title, docText string, assets []ingest.Asset, workingDir string, progress Progress, confirm ConfirmFallback, visionProv providers.Provider) (*domain.Adventure, error) {
	lim := limitsFrom(cfg)
	model := ""
	if cfg != nil {
		model = cfg.Model
	}

	// Before the expensive authoring, check whether the chosen model is actually
	// available: a cheap ping reveals if the provider would substitute a different
	// (usually smaller) model — e.g. a rate-limited Sonnet falling back to Haiku.
	// If so, let the caller decide whether to proceed or abort. Only Anthropic does
	// this family-level downgrade; OpenAI/Gemini return dated ids for the same model
	// (which would falsely look like a substitution), so restrict the check to it.
	if confirm != nil && model != "" && cfg != nil && cfg.Provider == domain.ProviderAnthropic {
		if served := servedModel(ctx, prov, model); served != "" && served != model {
			if !confirm(model, served) {
				return nil, fmt.Errorf("import cancelled: %q is unavailable and %q was declined", model, served)
			}
			report(progress, "Continuing with %q (the chosen model %q is unavailable).", served, model)
		}
	}

	// Curate the extracted images with vision: classify (map/portrait/scene/
	// item/decorative), caption, and drop decorative junk. If the authoring model
	// can't see images (e.g. the Claude CLI backend), use the separate vision
	// provider for this step only; if none is available, skip curation.
	visP := prov
	if !prov.SupportsVision() {
		visP = visionProv
	}
	curated := toAssets(assets)
	switch {
	case len(assets) == 0:
		// nothing to curate
	case visP != nil && visP.SupportsVision():
		report(progress, "Curating %d image(s) with vision…", len(assets))
		curated = curateAssets(ctx, visP, model, workingDir, curated, curationMaxBytes, progress)
		report(progress, "Kept %d image(s) after curation.", len(curated))
	default:
		report(progress, "Vision unavailable on this backend; skipping image curation (%d image(s) kept as-is).", len(assets))
	}

	user := buildUserPrompt(title, docText, curated, lim.maxDocChars)

	// Inline images go to the authoring model only if IT can see them. With a
	// text-only backend we rely on the curated captions carried in the prompt.
	var images []providers.ImageData
	if prov.SupportsVision() {
		images = loadVisionImages(workingDir, curated, lim.visionMaxImages, lim.maxImageBytes)
	}

	sys := systemPrompt
	if dir := importLanguageDirective(cfg); dir != "" {
		sys += "\n\n" + dir
		report(progress, "Authoring the module in %s.", cfg.ImportLanguageName())
	}

	// Generate the JSON, continuing across responses when the model runs into
	// the per-reply output-token limit (adventures easily exceed it).
	report(progress, "Generating the adventure…")
	raw, finish, err := generate(ctx, prov, model, sys, user, images, lim.maxOutputTokens, progress)
	if err != nil {
		// Surface the reason in the log and keep whatever partial output we got,
		// so a mid-continuation API failure is diagnosable and recoverable.
		report(progress, "AI request failed: %v", err)
		if workingDir != "" && strings.TrimSpace(raw) != "" {
			rawPath := filepath.Join(workingDir, "_import_raw.txt")
			if werr := os.WriteFile(rawPath, []byte(raw), 0644); werr == nil {
				report(progress, "Partial output saved to %s (%d chars).", rawPath, len(raw))
			}
		}
		return nil, fmt.Errorf("AI request failed: %w", err)
	}

	report(progress, "Parsing the generated JSON…")
	adv, perr := parseAdventure(raw)
	if perr != nil && !truncated(finish) {
		// Not a truncation — try one repair round for prose/JSON quirks.
		report(progress, "Output wasn't valid JSON; asking the model to repair it…")
		if fixed, ok := tryRepair(ctx, prov, model, raw, lim.maxOutputTokens); ok {
			if a2, e2 := parseAdventure(fixed); e2 == nil {
				adv, perr = a2, nil
			}
		}
	}
	if perr != nil {
		hint := "Try the import again, or raise import.max_output_tokens in config.yaml."
		if truncated(finish) {
			hint = "the reply exceeded the output limit even after continuing — raise import.max_output_tokens in config.yaml, or import a smaller source."
		}
		// Persist the raw reply so the work isn't lost and the failure is diagnosable.
		if workingDir != "" && strings.TrimSpace(raw) != "" {
			rawPath := filepath.Join(workingDir, "_import_raw.txt")
			if werr := os.WriteFile(rawPath, []byte(raw), 0644); werr == nil {
				hint += " The raw model output was saved to " + rawPath + "."
			}
		}
		return nil, fmt.Errorf("the model did not return usable adventure JSON (%v); %s", perr, hint)
	}

	sanitize(adv, title, workingDir)
	adv.Migrate() // normalize exit directions + backfill the directional zone graph
	if code := importLanguageCode(cfg); code != "" {
		adv.Language = code // keep the module's language tag consistent with the import target
	}
	enrichCatalog(adv, curated)
	dropUnknownImageIDs(adv)
	report(progress, "Done: %d zone(s), %d room(s), %d NPC(s), %d event(s), %d image(s).",
		len(adv.Zones), countRooms(adv), len(adv.NPCs), len(adv.Events), len(adv.ImageRefs()))
	return adv, nil
}

func countRooms(adv *domain.Adventure) int {
	n := 0
	for _, z := range adv.Zones {
		n += len(z.Rooms)
	}
	return n
}

// servedModel pings the provider with a tiny request and reports which model id
// actually served it, which may differ from the requested one when the provider
// substitutes a fallback. Returns "" if the ping fails (then we don't second-guess
// the real call).
func servedModel(ctx context.Context, prov providers.Provider, model string) string {
	resp, err := prov.Chat(ctx, providers.ChatRequest{
		Model:     model,
		MaxTokens: 8,
		Messages:  []providers.Message{{Role: providers.RoleUser, Content: "ping"}},
	})
	if err != nil || resp == nil {
		return ""
	}
	return resp.Model
}

// maxContinuations bounds how many "keep going" round-trips generate() will make.
const maxContinuations = 6

// generate produces the model's JSON, transparently continuing when a reply is
// cut off at the output-token limit and concatenating the pieces. It returns
// the combined text and the finish reason of the last reply.
func generate(ctx context.Context, prov providers.Provider, model, sys, user string, images []providers.ImageData, maxTokens int, progress Progress) (string, string, error) {
	messages := []providers.Message{
		{Role: providers.RoleSystem, Content: sys},
		{Role: providers.RoleUser, Content: user, Images: images},
	}

	var full strings.Builder
	finish := ""
	for i := 0; i <= maxContinuations; i++ {
		if i > 0 {
			report(progress, "Reply reached the token limit; continuing (%d/%d)…", i, maxContinuations)
		}
		resp, err := prov.Chat(ctx, providers.ChatRequest{
			Model:     model,
			MaxTokens: maxTokens,
			Messages:  messages,
		})
		if err != nil {
			return full.String(), finish, err
		}
		if i == 0 && resp.Model != "" && resp.Model != model {
			// The provider served a different model than requested (e.g. fell back
			// to a smaller model because the chosen one was rate-limited). Surface
			// it: a downgrade materially affects output quality on large modules.
			report(progress, "Note: request served by %q instead of %q (the chosen model was unavailable).", resp.Model, model)
		}

		// Stitch the reply onto what we have. Continuations (i>0) can arrive wrapped
		// in a code fence or repeating a few trailing characters despite the
		// instruction; clean that so the concatenation stays valid JSON.
		chunk := resp.Content
		if i > 0 {
			chunk = stitchContinuation(full.String(), chunk)
		}
		full.WriteString(chunk)
		finish = resp.FinishReason
		if !truncated(finish) {
			break
		}
		// Resume. This model family may not support assistant prefill, so the
		// conversation must end with a user turn; give the model its own partial
		// output back plus an explicit instruction and the exact tail to continue.
		tail := lastN(full.String(), 240)
		messages = append(messages,
			providers.Message{Role: providers.RoleAssistant, Content: resp.Content},
			providers.Message{Role: providers.RoleUser, Content: "Your JSON object is incomplete — it was cut off. Continue it from EXACTLY where it stopped. Output ONLY the remaining raw JSON characters: no repetition of what you already sent, no explanation, no markdown fences. For reference, your output so far ends with:\n" + tail},
		)
	}
	return full.String(), finish, nil
}

// stitchContinuation cleans a continuation chunk before appending it to prev: it
// strips a leading code fence and removes any overlap where the model repeated
// the tail of what it already sent, so the concatenation stays valid JSON.
func stitchContinuation(prev, chunk string) string {
	chunk = strings.TrimLeft(chunk, " \t\r\n")
	for _, f := range []string{"```json", "```JSON", "```Json", "```"} {
		chunk = strings.TrimPrefix(chunk, f)
	}
	chunk = strings.TrimLeft(chunk, " \t\r\n")
	// Drop the largest prefix of chunk that duplicates the suffix of prev.
	max := 400
	if len(prev) < max {
		max = len(prev)
	}
	if len(chunk) < max {
		max = len(chunk)
	}
	for k := max; k > 0; k-- {
		if prev[len(prev)-k:] == chunk[:k] {
			return chunk[k:]
		}
	}
	return chunk
}

// lastN returns the last n bytes of s (rune-safe at the cut point).
func lastN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[len(s)-n:]
	for len(s) > 0 && !utf8.RuneStart(s[0]) {
		s = s[1:]
	}
	return s
}

func truncated(finishReason string) bool {
	return finishReason == "max_tokens" || finishReason == "length"
}

// tryRepair asks the model to turn a malformed/truncated reply into a single
// valid JSON object. Returns the repaired text and whether the call succeeded.
func tryRepair(ctx context.Context, prov providers.Provider, model, raw string, maxTokens int) (string, bool) {
	resp, err := prov.Chat(ctx, providers.ChatRequest{
		Model:     model,
		MaxTokens: maxTokens,
		Messages: []providers.Message{
			{Role: providers.RoleSystem, Content: "You repair malformed or truncated JSON. Output ONLY one complete, valid JSON object — no prose, no markdown fences."},
			{Role: providers.RoleUser, Content: "Repair and complete this into a single valid JSON object for a D&D adventure module, preserving all of its content:\n\n" + raw},
		},
	})
	if err != nil {
		return "", false
	}
	return resp.Content, true
}

const systemPrompt = `You are an expert tabletop RPG (D&D 5e) module designer. You receive the raw text and images extracted from a source document (an adventure PDF or a set of images). Interpret ALL of it and produce a single, complete adventure module as JSON.

Output ONLY a JSON object (no prose, no markdown fences) with this shape:
{
  "schema_version":"1.1","id":"kebab-case-id","title":"...","author":"","system":"D&D 5e","language":"en","start_room":"id of the room where the party begins",
  "summary":"...","context":"how to position/run it: setting, tone, recommended level & party, campaign fit, prerequisites","background":"the FULL in-world history/backstory for the DM (keep every paragraph)","introduction":"how it starts","conclusion":"possible endings","hooks":["..."],
  "images":[{"id":"<image_id>","kind":"map|art","description":"..."}],
  "zones":[{"id":"...","name":"...","overview":"DM summary","description":"...","image_ids":["<image_id>"],
    "exits":[{"direction":"north|south|east|west|ne|nw|se|sw|up|down|in|out","to":"adjacent zoneId","locked":false,"description":"the passage"}],
    "rooms":[{"id":"...","name":"...","read_aloud":"boxed text for players","dm_notes":"secrets/what happens","image_ids":["<image_id>"],
      "npc_ids":["..."],"event_ids":["..."],"treasure":["..."],
      "exits":[{"to":"roomOrZoneId","direction":"north","description":"...","locked":false}],
      "features":[{"name":"...","description":"...","skill":"Perception","dc":13,"success":"...","failure":"..."}],
      "encounters":[{"name":"...","description":"...","creatures":["..."],"difficulty":"medium","tactics":"..."}]}]}],
  "npcs":[{"id":"...","name":"...","role":"...","appearance":"...","personality":"...","motivations":"...","secrets":"...","voice":"...",
    "disposition":"...","default_location":"roomId","image_ids":["<image_id>"],"knowledge":["..."],"sample_dialogue":["..."],
    "stat_block":{"ac":13,"max_hp":22,"speed":"30 ft","cr":"1","abilities":{"str":10,"dex":10,"con":10,"int":10,"wis":10,"cha":10},
      "skills":["..."],"traits":["..."],"actions":[{"name":"...","to_hit":"+4","damage":"1d8+2 slashing","description":"..."}]}}],
  "events":[{"id":"...","name":"...","trigger":"...","description":"...","read_aloud":"...","dm_notes":"...","consequences":"...",
    "outcomes":[{"condition":"...","result":"..."}]}],
  "items":[{"id":"...","name":"...","description":"...","rarity":"...","mechanics":"...","image_ids":["<image_id>"]}],
  "tables":[{"id":"...","name":"...","description":"what it is for / when to roll it","dice":"d20","headers":["Result"],"rows":[{"roll":"1-3","cells":["outcome text"]}]}]
}

RULES:
- Ground everything in the provided material; do not invent a different adventure. Preserve names, places, NPCs, and plot.
- Capture the adventure's framing COMPLETELY and faithfully — never drop the front-matter. Put the in-world history/backstory in "background" and keep it FULL (multiple paragraphs if the source has them; do NOT compress it to a sentence). Put the positioning/running context — setting and tone, recommended character level and party size, how to fit it into a larger campaign, prerequisites, and running advice — in "context". If the source separates these, keep them separate; if it only has one, fill that one.
- Split the content into coherent zones and rooms. Give NPCs motivations, secrets and voice for roleplay, plus a stat block when the source implies combat.
- TABLES: whenever the source has a table — random encounters, treasure, roll-a-d20 result lists, name lists, price/reference tables — reproduce it in "tables". Put the die in "dice" (e.g. "d20", "2d6", "d100") when it is a roll table, the column titles in "headers", and one entry per row in "rows" with its "roll" range (e.g. "1", "1-3", "18-20") and "cells". Transcribe every row faithfully; do not summarize or drop rows.
- IMAGES: you are given a list of extracted images, each with an image_id, a kind, and a caption. Reference them by id in the "image_ids" arrays: put kind=map images in the matching zone's image_ids; put kind=portrait/scene/item in the matching NPC, room, or item's image_ids, guided by the caption. Use ONLY image_ids from the provided list — never invent ids or file paths. You do not need to output the top-level "images" catalog; it is filled in automatically.
- Every id must be unique and kebab-case. Every reference (npc_ids, event_ids, exit "to", default_location, image_ids) must point to an id that exists.
- Return valid JSON only.`

// importLanguageDirective returns extra system-prompt rules pinning the language
// the module is authored in, or "" when import.language is unset — in which case
// the model follows the source document, preserving prior behavior. When set, all
// prose is written in the target language while rules terms (monsters, spells,
// items) are given translated with the original in parentheses, so the DM can
// still cross-reference official books and the internet.
func importLanguageDirective(cfg *domain.Config) string {
	if cfg == nil || strings.TrimSpace(cfg.ImportLanguage) == "" {
		return ""
	}
	lang := cfg.ImportLanguageName()
	var sb strings.Builder
	sb.WriteString("LANGUAGE:\n")
	fmt.Fprintf(&sb, "- Author ALL prose in %s, regardless of the source document's language: summary, background, introduction, conclusion, hooks, zone/room overviews and descriptions, read_aloud (boxed) text, dm_notes, every NPC field (appearance, personality, motivations, secrets, voice, knowledge, sample_dialogue), event text, and item descriptions/mechanics.\n", lang)
	fmt.Fprintf(&sb, "- Set the top-level \"language\" field to %q.\n", cfg.ImportLanguageCode())
	sb.WriteString("- Keep every \"id\" value in ASCII kebab-case; never translate ids.\n")
	if !strings.EqualFold(lang, "English") {
		fmt.Fprintf(&sb, "- For rules terminology a DM may need to look up in official books — monster/creature names, spells, magic items, equipment, conditions, and other game keywords — write the %s term followed by the original source-language term in parentheses on first use in each field, e.g. \"lobo huargo (dire wolf)\", \"espada larga (longsword)\". Afterwards the %s term alone is fine.\n", lang, lang)
		sb.WriteString("- Preserve unique proper nouns (character and place names) in their original form; you may add a parenthetical translation when the source itself localizes them.\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

// importLanguageCode returns the module "language" tag for the import target, or
// "" when import.language is unset.
func importLanguageCode(cfg *domain.Config) string {
	if cfg == nil || strings.TrimSpace(cfg.ImportLanguage) == "" {
		return ""
	}
	return cfg.ImportLanguageCode()
}

func buildUserPrompt(title, docText string, assets []asset, maxDocChars int) string {
	var sb strings.Builder
	if title != "" {
		fmt.Fprintf(&sb, "Suggested title: %s\n\n", title)
	}
	sb.WriteString("EXTRACTED IMAGES (classified & captioned — reference them by image_id):\n")
	if len(assets) == 0 {
		sb.WriteString("(none)\n")
	}
	for _, a := range assets {
		line := fmt.Sprintf("- image_id %q [kind: %s]", a.id, a.kind)
		if a.page > 0 {
			line += fmt.Sprintf(" (page %d)", a.page)
		}
		if cap := caption(a); cap != "" {
			line += " — " + cap
		}
		sb.WriteString(line + "\n")
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
func loadVisionImages(workingDir string, assets []asset, maxImages, maxBytes int) []providers.ImageData {
	// Maps first — they carry the most structural information.
	maps, rest := []asset{}, []asset{}
	for _, a := range assets {
		if a.decorative() {
			continue
		}
		if a.isMap() {
			maps = append(maps, a)
		} else {
			rest = append(rest, a)
		}
	}
	ordered := append(maps, rest...)

	var out []providers.ImageData
	for _, a := range ordered {
		if len(out) >= maxImages {
			break
		}
		mt := mediaType(a.rel)
		if mt == "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(workingDir, filepath.FromSlash(a.rel)))
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
	// Remove ALL markdown code fences, not just the outer pair: when the reply
	// was stitched from several continuation chunks, each chunk may have added
	// its own ```json fence in the middle of the JSON.
	s := content
	for _, f := range []string{"```json", "```JSON", "```Json", "```"} {
		s = strings.ReplaceAll(s, f, "")
	}
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("no JSON object found in the response")
	}
	candidate := trailingCommaRe.ReplaceAllString(s[start:end+1], "$1")

	var adv domain.Adventure
	if err := json.Unmarshal([]byte(candidate), &adv); err != nil {
		return nil, err
	}
	return &adv, nil
}

// trailingCommaRe matches a comma before a closing brace/bracket — a common
// LLM JSON slip that strict parsers reject.
var trailingCommaRe = regexp.MustCompile(`,(\s*[}\]])`)

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
		z.Exits = keepZoneExits(z.Exits, func(to string) bool { return zoneIDs[to] })
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
	// Drop an entry point that doesn't resolve to a real room.
	if adv.StartRoom != "" && !roomIDs[adv.StartRoom] {
		adv.StartRoom = ""
	}
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

func keepZoneExits(in []domain.ZoneExit, keep func(string) bool) []domain.ZoneExit {
	var out []domain.ZoneExit
	for _, e := range in {
		if e.To != "" && keep(e.To) {
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

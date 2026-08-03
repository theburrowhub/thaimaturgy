package aibuild

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/theburrowhub/thaimaturgy/internal/ingest"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
)

// transcribePageMaxTokens bounds a single page's transcription. A dense page is
// well under this; the generous ceiling avoids cutting off text.
const transcribePageMaxTokens = 16000

const transcribeSystemPrompt = `You are a meticulous OCR and layout transcriber for scanned pages of a tabletop RPG (D&D) sourcebook. Transcribe EVERYTHING on the page, VERBATIM and COMPLETELY — lose no text.

Include, in natural reading order: titles and headers, body text, boxed/read-aloud text, sidebars, captions, tables, monster/NPC stat blocks, map labels and legends, item and spell descriptions, footnotes, and page numbers. Preserve structure with Markdown (headings, lists, tables, and blockquotes for boxed/read-aloud text).

Where an illustration or map appears, insert a line: [ART: short description] or [MAP: short description] at its position.

Do NOT summarize, paraphrase, translate, or omit anything. Do NOT add commentary of your own. Output only the transcription.`

// visionProviderFor returns the image-capable provider to use: the primary one if
// it accepts images, otherwise the dedicated vision provider, otherwise nil.
func visionProviderFor(prov, visionProv providers.Provider) providers.Provider {
	if prov != nil && prov.SupportsVision() {
		return prov
	}
	if visionProv != nil && visionProv.SupportsVision() {
		return visionProv
	}
	return nil
}

// transcribePages treats the given images as scanned pages of a physical book and
// transcribes each page's full text (marking art/maps) with a vision model,
// returning the combined document text with page markers — the equivalent of a
// PDF's text layer, recovered by OCR. Pages are processed one at a time so nothing
// is dropped to truncation, in filename order.
func transcribePages(ctx context.Context, vp providers.Provider, model, workingDir string, assets []ingest.Asset, maxBytes int, progress Progress) string {
	if vp == nil || len(assets) == 0 {
		report(progress, "No vision backend available; importing images without page-text transcription.")
		return ""
	}

	ordered := append([]ingest.Asset(nil), assets...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].RelPath < ordered[j].RelPath })

	var sb strings.Builder
	page := 0
	for _, a := range ordered {
		mt := mediaType(a.RelPath)
		if mt == "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(workingDir, filepath.FromSlash(a.RelPath)))
		if err != nil || len(data) == 0 || len(data) > maxBytes {
			continue
		}
		page++
		report(progress, "Transcribing page %d of %d…", page, len(ordered))
		resp, err := vp.Chat(ctx, providers.ChatRequest{
			Model:     model,
			MaxTokens: transcribePageMaxTokens,
			Messages: []providers.Message{
				{Role: providers.RoleSystem, Content: transcribeSystemPrompt},
				{Role: providers.RoleUser, Content: "Transcribe this page completely and verbatim.",
					Images: []providers.ImageData{{MediaType: mt, Data: data}}},
			},
		})
		if err != nil {
			report(progress, "Page %d transcription failed: %v", page, err)
			continue
		}
		fmt.Fprintf(&sb, "\n\n=== PAGE %d (%s) ===\n%s", page, filepath.Base(a.RelPath), strings.TrimSpace(resp.Content))
	}
	report(progress, "Transcribed %d page(s) (%d characters).", page, sb.Len())
	return strings.TrimSpace(sb.String())
}

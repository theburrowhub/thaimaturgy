package aibuild

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/theburrowhub/thaimaturgy/internal/ingest"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
)

// transcribePageMaxTokens bounds a single page's transcription. A dense page is
// well under this; the generous ceiling avoids cutting off text.
const transcribePageMaxTokens = 16000

// transcribeConcurrency bounds how many pages are OCR'd at once. Whole books have
// many pages, so transcribing them one-by-one is slow; a small pool cuts the
// wall-clock without hammering the provider's rate limits.
const transcribeConcurrency = 4

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

	// Order the pages (by filename), keeping only vision-eligible images.
	ordered := append([]ingest.Asset(nil), assets...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].RelPath < ordered[j].RelPath })
	var pages []ingest.Asset
	for _, a := range ordered {
		if mediaType(a.RelPath) != "" {
			pages = append(pages, a)
		}
	}
	if len(pages) == 0 {
		return ""
	}

	results := make([]string, len(pages))
	sem := make(chan struct{}, transcribeConcurrency)
	var wg sync.WaitGroup
	var done int64
	for i := range pages {
		i, a := i, pages[i]
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			data, err := os.ReadFile(filepath.Join(workingDir, filepath.FromSlash(a.RelPath)))
			if err != nil || len(data) == 0 || len(data) > maxBytes {
				atomic.AddInt64(&done, 1)
				return
			}
			resp, cerr := vp.Chat(ctx, providers.ChatRequest{
				Model:     model,
				MaxTokens: transcribePageMaxTokens,
				Messages: []providers.Message{
					{Role: providers.RoleSystem, Content: transcribeSystemPrompt},
					{Role: providers.RoleUser, Content: "Transcribe this page completely and verbatim.",
						Images: []providers.ImageData{{MediaType: mediaType(a.RelPath), Data: data}}},
				},
			})
			n := atomic.AddInt64(&done, 1)
			report(progress, "Transcribing pages… %d/%d", n, len(pages))
			if cerr != nil {
				report(progress, "Page %d transcription failed: %v", i+1, cerr)
				return
			}
			results[i] = strings.TrimSpace(resp.Content)
		}()
	}
	wg.Wait()

	var sb strings.Builder
	page := 0
	for i, a := range pages {
		if results[i] == "" {
			continue
		}
		page++
		fmt.Fprintf(&sb, "\n\n=== PAGE %d (%s) ===\n%s", page, filepath.Base(a.RelPath), results[i])
	}
	report(progress, "Transcribed %d of %d page(s) (%d characters).", page, len(pages), sb.Len())
	return strings.TrimSpace(sb.String())
}

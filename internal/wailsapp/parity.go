package wailsapp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/engine"
	"github.com/theburrowhub/thaimaturgy/internal/ingest"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

// mimeByExt maps a file extension to an image MIME type for data: URIs.
func mimeByExt(p string) string {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}

// assetDataURL resolves a relative module asset path to a self-contained data:
// URI. The Wails webview refuses cross-origin file:// image loads, so detail
// art, map/art commands and editor previews must be inlined as data URIs.
func (a *App) assetDataURL(adventureID, rel string) string {
	abs, err := a.store.ResolveImagePath(adventureID, rel)
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return ""
	}
	return "data:" + mimeByExt(abs) + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// AssetURL returns a data: URI for a module asset (used by editor image previews
// and the /map, /art commands). Returns an error if the asset can't be read.
func (a *App) AssetURL(adventureID, rel string) (string, error) {
	adventureID, rel = strings.TrimSpace(adventureID), strings.TrimSpace(rel)
	if adventureID == "" || rel == "" {
		return "", fmt.Errorf("adventure id and asset path are required")
	}
	url := a.assetDataURL(adventureID, rel)
	if url == "" {
		return "", fmt.Errorf("asset not found: %s", rel)
	}
	return url, nil
}

// PlanParty turns a natural-language request into a full party via the LLM
// (roster only; stat blocks are generated deterministically by the rules) and
// installs it into the current session. Mirrors the desktop party editor's
// "Generate with AI" action.
func (a *App) PlanParty(prompt string) (*SessionPayload, error) {
	if a.session == nil {
		return nil, fmt.Errorf("no active session")
	}
	if a.prov == nil {
		return nil, fmt.Errorf("no AI provider configured")
	}
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("describe the party you want")
	}
	timeout := time.Duration(a.config.RequestTimeoutSeconds) * time.Second
	if timeout < 2*time.Minute {
		timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	party, err := engine.PlanParty(ctx, a.prov, a.config.Model, prompt, a.session.State.PartySnapshot())
	if err != nil {
		return nil, err
	}
	a.session.State.SetParty(party)
	a.session.MarkModified()
	a.autosave()
	return a.payload()
}

// SetMode toggles between Oracle and Virtual-DM mode. Entering Virtual DM
// ensures a party exists and, on the first entry, starts the game and narrates
// the opening scene through the oracle — matching the desktop app's
// onModeChanged behaviour (which the web UI previously only half-replicated via
// a bare "/mode" command). The kickoff narration is returned as the message and
// also stored in the conversation.
func (a *App) SetMode() (*SubmitResult, error) {
	if a.session == nil {
		return nil, fmt.Errorf("no active session")
	}
	a.session.State.ToggleMode()
	msg := "Oracle mode."
	if a.session.State.EffectiveMode() != domain.ModeVirtualDM {
		// Leaving Virtual DM: stop any Telegram host (it only makes sense in DM
		// mode), mirroring the desktop app's applyMode teardown.
		a.stopTelegramHost()
	}
	if a.session.State.EffectiveMode() == domain.ModeVirtualDM {
		a.session.State.EnsureParty()
		msg = "Virtual DM mode."
		if a.session.State.StartGame() {
			a.session.State.AddNote("Virtual DM mode started.")
			if a.oracle == nil {
				a.oracle = engine.NewOracle(a.session, a.prov)
			}
			if a.prov != nil {
				timeout := time.Duration(a.config.RequestTimeoutSeconds) * time.Second
				if timeout <= 0 {
					timeout = 90 * time.Second
				}
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				defer cancel()
				if resp := a.oracle.Ask(ctx, domain.DMKickoffPrompt(a.config.Language)); resp.Error == nil && strings.TrimSpace(resp.Answer) != "" {
					msg = resp.Answer
				}
			}
		}
	}
	a.session.MarkModified()
	a.autosave()
	p, _ := a.payload()
	return &SubmitResult{Success: true, Message: msg, Session: p}, nil
}

// ResetParty replaces the current party with the default pre-generated roster.
// Mirrors the desktop party editor's "Default party" action.
func (a *App) ResetParty() (*SessionPayload, error) {
	if a.session == nil {
		return nil, fmt.Errorf("no active session")
	}
	a.session.State.SetParty(domain.DefaultParty())
	a.session.MarkModified()
	a.autosave()
	return a.payload()
}

// ValidateAdventure runs the shared module validator and returns the list of
// problems as plain strings (empty when the module is valid). Mirrors the
// desktop editor's Validate button, which the web editor previously only
// approximated with a local JSON parse.
func (a *App) ValidateAdventure(adv *domain.Adventure) ([]string, error) {
	if adv == nil {
		return nil, fmt.Errorf("adventure is required")
	}
	errs := domain.ValidateAdventure(adv, a.imageExistsFor(adv.ID))
	out := make([]string, 0, len(errs))
	for _, e := range errs {
		out = append(out, e.Error())
	}
	return out, nil
}

// imageExistsFor reports whether a referenced relative asset exists under the
// adventure's installed directory, so validation can flag missing/misspelled
// image paths (matching the desktop editor's working-dir check). Empty paths are
// treated as present (nothing to check).
func (a *App) imageExistsFor(adventureID string) func(string) bool {
	base := a.store.AdventureDir(strings.TrimSpace(adventureID))
	return func(rel string) bool {
		rel = strings.TrimSpace(rel)
		if rel == "" {
			return true
		}
		info, err := os.Stat(filepath.Join(base, filepath.FromSlash(rel)))
		return err == nil && !info.IsDir()
	}
}

// OpenExternalModule opens an adventure from an external .tar.gz archive or an
// unpacked folder, normalizes its images (TIFF→PNG, drops near-blank layers) and
// installs it into the library so its assets travel with it, then returns the
// installed adventure for editing. Mirrors the desktop editor's "open folder /
// .tar.gz" flow (which the web editor previously lacked entirely).
func (a *App) OpenExternalModule(path string) (*domain.Adventure, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	srcDir := path
	if !info.IsDir() {
		tmp, err := os.MkdirTemp("", "thaim-openmod-*")
		if err != nil {
			return nil, err
		}
		if err := storage.ExtractModule(path, tmp); err != nil {
			return nil, err
		}
		srcDir = tmp
	}
	data, err := os.ReadFile(filepath.Join(srcDir, storage.AdventureFile))
	if err != nil {
		return nil, fmt.Errorf("no %s found in module: %w", storage.AdventureFile, err)
	}
	var adv domain.Adventure
	if err := json.Unmarshal(data, &adv); err != nil {
		return nil, err
	}
	ingest.NormalizeModuleImages(srcDir, &adv)
	if strings.TrimSpace(adv.ID) == "" {
		return nil, fmt.Errorf("module has no id")
	}
	// Persist the normalized adventure.json, package the folder and import it so
	// assets are copied into the library under the module's id.
	nb, err := json.MarshalIndent(&adv, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(srcDir, storage.AdventureFile), nb, 0644); err != nil {
		return nil, err
	}
	pkg := filepath.Join(os.TempDir(), "thaim-openmod-"+adv.ID+".tar.gz")
	if err := storage.PackageModule(srcDir, pkg); err != nil {
		return nil, err
	}
	defer os.Remove(pkg)
	if _, err := a.store.ImportModule(pkg); err != nil {
		return nil, err
	}
	return a.store.LoadAdventure(adv.ID)
}

// ExportAdventureFolder writes the adventure's canonical layout (adventure.json
// plus the assets/ tree) into destDir. Mirrors the desktop editor's "Adventure
// folder…" export option, complementing the .tar.gz PackageAdventure.
func (a *App) ExportAdventureFolder(adventureID, destDir string) (*SubmitResult, error) {
	adventureID = strings.TrimSpace(adventureID)
	destDir = strings.TrimSpace(destDir)
	if adventureID == "" || destDir == "" {
		return nil, fmt.Errorf("adventure id and destination folder are required")
	}
	srcDir := a.store.AdventureDir(adventureID)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, err
	}
	if err := copyFile(filepath.Join(srcDir, storage.AdventureFile), filepath.Join(destDir, storage.AdventureFile)); err != nil {
		return nil, err
	}
	if assets := filepath.Join(srcDir, "assets"); dirExists(assets) {
		if err := copyDir(assets, filepath.Join(destDir, "assets")); err != nil {
			return nil, err
		}
	}
	p, _ := a.payload()
	return &SubmitResult{Success: true, Message: "Adventure folder saved: " + destDir, Session: p}, nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyFile(path, target)
	})
}

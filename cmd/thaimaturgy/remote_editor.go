package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"

	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

// This file wires the module editor to a remote server (#144), so an adventure
// in the server library can be edited in place, and a new one authored from
// scratch, from the remote GUI's main window. It reuses the exact editor UI
// (editor.go); only load and save are redirected to the HTTP API.

// editRemoteAdventure loads an adventure from the server and opens it in the
// editor, saving edits back with PUT /api/adventures/{id}.
func (g *gui) editRemoteAdventure(id string) {
	g.editorGen++ // this navigation supersedes any earlier in-flight editor load
	myGen := g.editorGen
	go func() {
		ctx, cancel := bg(20)
		adv, err := g.remote.GetAdventure(ctx, id)
		cancel()
		fyne.Do(func() {
			// Drop a late load if the user has since opened another adventure (or a
			// new one) — otherwise this stale result would replace the newer editor
			// session and its persistence hooks, discarding an unsaved draft.
			if myGen != g.editorGen {
				return
			}
			if err != nil {
				g.showErr(err)
				return
			}
			if g.editor == nil {
				g.editor = newEditor(g)
			}
			e := g.editor
			e.adv = adv
			e.workingDir = ""
			e.dirty = false
			e.remoteMode = true
			e.onBack = g.showRemoteLibrary
			e.onPlay = func(string) { g.remoteNewSession(id) }
			e.saveHook = func() error {
				sctx, scancel := bg(30)
				defer scancel()
				return g.remote.SaveAdventure(sctx, id, e.adv)
			}
			g.showEditor()
		})
	}()
}

// newRemoteAdventure opens the editor on a fresh adventure whose first save
// PACKAGES the working dir and imports it to the server (create); subsequent
// saves update the now-existing server adventure by id.
func (g *gui) newRemoteAdventure() {
	g.editorGen++ // supersede any in-flight remote editor load
	if g.editor == nil {
		g.editor = newEditor(g)
	}
	e := g.editor
	e.newAdventure() // fresh in-memory adventure + a temp working dir used for packaging
	e.remoteMode = true
	e.onBack = g.showRemoteLibrary

	created := false
	var savedID string
	// markCreated records that the adventure now exists on the server, switching
	// future saves to an in-place PUT and Play to a session for it.
	markCreated := func(id string) {
		savedID, created = id, true
		e.onPlay = func(string) { g.remoteNewSession(savedID) }
	}
	e.saveHook = func() error {
		if created {
			sctx, scancel := bg(30)
			defer scancel()
			return g.remote.SaveAdventure(sctx, savedID, e.adv)
		}
		// Reconcile first: if an adventure with this id already exists on the server
		// (e.g. a prior import whose response we lost), switch to update rather than
		// importing again — so a retry is idempotent and never duplicates.
		rctx, rcancel := bg(20)
		if _, err := g.remote.GetAdventure(rctx, e.adv.ID); err == nil {
			rcancel()
			markCreated(e.adv.ID)
			sctx, scancel := bg(30)
			defer scancel()
			return g.remote.SaveAdventure(sctx, savedID, e.adv)
		}
		rcancel()
		// Not present: write adventure.json into the working dir, package it, and
		// import it to create the adventure server-side.
		data, err := json.MarshalIndent(e.adv, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(e.workingDir, storage.AdventureFile), data, 0o644); err != nil {
			return err
		}
		tgz, err := os.CreateTemp("", "thaim-newremote-*.tar.gz")
		if err != nil {
			return err
		}
		tgzPath := tgz.Name()
		_ = tgz.Close()
		defer os.Remove(tgzPath)
		_ = os.Remove(tgzPath) // PackageModule recreates it
		if err := storage.PackageModule(e.workingDir, tgzPath); err != nil {
			return err
		}
		ictx, icancel := bg(120)
		defer icancel()
		newID, _, err := g.remote.ImportAdventure(ictx, tgzPath)
		if err != nil {
			// Ambiguous failure (e.g. lost/undecodable response): if the adventure
			// got created anyway, reconcile so a retry updates instead of duplicating.
			cctx, ccancel := bg(20)
			_, gerr := g.remote.GetAdventure(cctx, e.adv.ID)
			ccancel()
			if gerr == nil {
				markCreated(e.adv.ID)
				return nil
			}
			return err
		}
		markCreated(newID)
		return nil
	}
	// Before the first save the adventure isn't on the server yet; playCurrent runs
	// save() first (creating it) and then reads e.onPlay, so this is only a
	// pre-create fallback and is replaced once creation succeeds.
	e.onPlay = func(string) { g.remoteNewSession(e.adv.ID) }
	g.showEditor()
}

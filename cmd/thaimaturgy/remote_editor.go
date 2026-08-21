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
	go func() {
		ctx, cancel := bg(20)
		adv, err := g.remote.GetAdventure(ctx, id)
		cancel()
		fyne.Do(func() {
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
	if g.editor == nil {
		g.editor = newEditor(g)
	}
	e := g.editor
	e.newAdventure() // fresh in-memory adventure + a temp working dir used for packaging
	e.remoteMode = true
	e.onBack = g.showRemoteLibrary

	created := false
	var savedID string
	e.saveHook = func() error {
		if created {
			sctx, scancel := bg(30)
			defer scancel()
			return g.remote.SaveAdventure(sctx, savedID, e.adv)
		}
		// First save: write adventure.json into the working dir, package it, and
		// import it to the server. Guard against a double-create by only switching
		// to PUT once the import succeeds.
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
			return err
		}
		savedID = newID
		created = true
		// Future saves update the server copy; Play starts a session for it.
		e.onPlay = func(string) { g.remoteNewSession(savedID) }
		return nil
	}
	// Before the first save the adventure isn't on the server yet; playCurrent runs
	// save() first (creating it) and then reads e.onPlay, so this is only a
	// pre-create fallback and is replaced once the import succeeds.
	e.onPlay = func(string) { g.remoteNewSession(e.adv.ID) }
	g.showEditor()
}

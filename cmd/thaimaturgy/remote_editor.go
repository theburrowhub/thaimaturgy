package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

// This file wires the module editor to a remote server (#144), so an adventure
// in the server library can be edited in place, and a new one authored from
// scratch, from the remote GUI's main window. It reuses the exact editor UI
// (editor.go); only load and save are redirected to the HTTP API. All network
// I/O runs off the UI thread; save hooks report completion via fyne.Do.

// snapshotAdventure marshals the editor's adventure on the UI thread (the only
// safe place to read it), so the background save can't race a concurrent form
// edit. The bytes are decoded back into a detached value for PUTs.
func snapshotAdventure(adv *domain.Adventure) ([]byte, error) {
	return json.MarshalIndent(adv, "", "  ")
}

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
			// Drop a late load if any newer navigation happened since (opening another
			// adventure, starting a session, Settings, Characters…) — otherwise this
			// stale result would replace the newer screen and discard an unsaved draft.
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
			e.saveHook = func(done func(error)) {
				data, merr := snapshotAdventure(e.adv) // on the UI thread
				if merr != nil {
					done(merr)
					return
				}
				go func() {
					var snap domain.Adventure
					if err := json.Unmarshal(data, &snap); err != nil {
						fyne.Do(func() { done(err) })
						return
					}
					sctx, scancel := bg(30)
					err := g.remote.SaveAdventure(sctx, id, &snap)
					scancel()
					fyne.Do(func() { done(err) })
				}()
			}
			g.showEditor()
		})
	}()
}

// newRemoteAdventure opens the editor on a fresh adventure. The first save
// packages the working dir and imports it to the server (create); afterwards
// saves update the created adventure in place by id.
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
	// markCreated (called on the UI thread) records that the adventure now exists
	// on the server, switching future saves to an in-place PUT and Play to it.
	markCreated := func(id string) {
		savedID, created = id, true
		e.onPlay = func(string) { g.remoteNewSession(savedID) }
	}

	e.saveHook = func(done func(error)) {
		id := e.adv.ID
		data, merr := snapshotAdventure(e.adv) // on the UI thread
		if merr != nil {
			done(merr)
			return
		}
		alreadyCreated := created
		workingDir := e.workingDir
		go func() {
			// Update path: the adventure was created by THIS draft already.
			if alreadyCreated {
				var snap domain.Adventure
				if err := json.Unmarshal(data, &snap); err != nil {
					fyne.Do(func() { done(err) })
					return
				}
				sctx, scancel := bg(30)
				err := g.remote.SaveAdventure(sctx, savedID, &snap)
				scancel()
				fyne.Do(func() { done(err) })
				return
			}
			// First save. A pre-existing id we never imported is a COLLISION — refuse
			// rather than PUT over (and destroy) an unrelated adventure.
			cctx, ccancel := bg(20)
			_, gerr := g.remote.GetAdventure(cctx, id)
			ccancel()
			if gerr == nil {
				fyne.Do(func() {
					done(fmt.Errorf("an adventure with id %q already exists on the server — choose a different id", id))
				})
				return
			}
			// Create it: write adventure.json into the working dir, package, import.
			if err := os.WriteFile(filepath.Join(workingDir, storage.AdventureFile), data, 0o644); err != nil {
				fyne.Do(func() { done(err) })
				return
			}
			tgz, err := os.CreateTemp("", "thaim-newremote-*.tar.gz")
			if err != nil {
				fyne.Do(func() { done(err) })
				return
			}
			tgzPath := tgz.Name()
			_ = tgz.Close()
			defer os.Remove(tgzPath)
			_ = os.Remove(tgzPath) // PackageModule recreates it
			if err := storage.PackageModule(workingDir, tgzPath); err != nil {
				fyne.Do(func() { done(err) })
				return
			}
			ictx, icancel := bg(120)
			newID, _, ierr := g.remote.ImportAdventure(ictx, tgzPath)
			icancel()
			if ierr != nil {
				// Report the error and do NOT infer ownership from a later ID lookup:
				// with concurrent clients, an adventure appearing under this id may have
				// been created by someone else, and treating it as ours would let a
				// later save PUT over their content. If our import actually succeeded
				// but the response was lost, a retry hits the collision check above and
				// the user opens the now-listed adventure via "Edit".
				fyne.Do(func() { done(ierr) })
				return
			}
			fyne.Do(func() { markCreated(newID); done(nil) })
		}()
	}
	// Before the first save the adventure isn't on the server yet; playCurrent runs
	// save first (creating it) and then, on success, reads e.onPlay — which
	// markCreated has by then repointed at the created id. This is the pre-create
	// fallback.
	e.onPlay = func(string) { g.remoteNewSession(e.adv.ID) }
	g.showEditor()
}

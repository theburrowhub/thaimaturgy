package httpapi

import (
	"errors"
	"log"
	"net/http"

	"github.com/theburrowhub/thaimaturgy/internal/appservice"
	"github.com/theburrowhub/thaimaturgy/internal/bookpdf"
)

// maxNovelBytes bounds the novel text a client may save or send for adjustment.
// Generous for a full book, but not unbounded.
const maxNovelBytes = 8 << 20 // 8 MiB

// getNovelText returns the session's saved novelization plus a version tag for
// optimistic-concurrency saves, and whether one exists yet.
func (s *Server) getNovelText(w http.ResponseWriter, r *http.Request) {
	md, version, exists, err := s.svc.NovelText(r.PathValue("name"))
	if errors.Is(err, appservice.ErrSessionUnknown) {
		httpError(w, http.StatusNotFound, "session not found")
		return
	}
	if err != nil {
		log.Printf("httpapi: load novel %q: %v", r.PathValue("name"), err)
		httpError(w, http.StatusInternalServerError, "could not load the novel")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"text": md, "version": version, "exists": exists})
}

// putNovelText saves an edited novelization, honoring optimistic concurrency:
// base_version must match the stored novel's version (as returned by
// getNovelText), else 409 so the client can reload and re-apply.
func (s *Server) putNovelText(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text        *string `json:"text"` // pointer: distinguishes an omitted field from ""
		BaseVersion string  `json:"base_version"`
	}
	if !readJSONLimited(w, r, &body, maxNovelBytes) {
		return
	}
	if body.Text == nil {
		// A missing "text" would otherwise decode as "" and silently erase the saved
		// novel; an explicit empty string is still allowed (clearing on purpose).
		httpError(w, http.StatusBadRequest, "the 'text' field is required")
		return
	}
	version, err := s.svc.SaveNovelText(r.PathValue("name"), *body.Text, body.BaseVersion)
	switch {
	case errors.Is(err, appservice.ErrSessionUnknown):
		httpError(w, http.StatusNotFound, "session not found")
	case errors.Is(err, appservice.ErrNovelConflict):
		httpError(w, http.StatusConflict, "the novel changed since you loaded it; reload and re-apply")
	case err != nil:
		log.Printf("httpapi: save novel %q: %v", r.PathValue("name"), err)
		httpError(w, http.StatusInternalServerError, "could not save the novel")
	default:
		writeJSON(w, http.StatusOK, map[string]any{"version": version})
	}
}

// startNovelAdjust begins an AI revision of the session's novel. The body
// carries the current text, an optional selection to revise, and the
// natural-language instruction. Returns the job id to poll; the result is
// fetched from GET /api/novel-jobs/{id}/result and NOT auto-persisted.
func (s *Server) startNovelAdjust(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Instruction string `json:"instruction"`
		Selection   string `json:"selection"`
		Text        string `json:"text"`
	}
	if !readJSONLimited(w, r, &body, maxNovelBytes) {
		return
	}
	job, err := s.svc.StartNovelAdjustJob(r.PathValue("name"), body.Text, body.Selection, body.Instruction)
	if err != nil {
		if errors.Is(err, appservice.ErrNovelCapacity) {
			httpError(w, http.StatusServiceUnavailable, err.Error())
		} else {
			httpError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

// novelJobResult returns a finished novel job's text as JSON (used by the editor
// to read an adjustment's result), 409 while it is still running.
func (s *Server) novelJobResult(w http.ResponseWriter, r *http.Request) {
	job, ok := s.svc.NovelJobByID(r.PathValue("id"))
	if !ok {
		httpError(w, http.StatusNotFound, "no such novel job")
		return
	}
	md, ready := job.Markdown()
	if !ready {
		httpError(w, http.StatusConflict, "the result is not ready yet")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"text": md, "kind": job.Kind})
}

// downloadSessionNovel streams the session's SAVED (edited) novel as Markdown
// (default) or PDF (?format=pdf), so exports reflect manual edits — unlike the
// job download, which serves the just-generated text.
func (s *Server) downloadSessionNovel(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	md, _, exists, err := s.svc.NovelText(name)
	if errors.Is(err, appservice.ErrSessionUnknown) {
		httpError(w, http.StatusNotFound, "session not found")
		return
	}
	if err != nil {
		log.Printf("httpapi: load novel for download %q: %v", name, err)
		httpError(w, http.StatusInternalServerError, "could not load the novel")
		return
	}
	if !exists {
		httpError(w, http.StatusNotFound, "this session has no saved novel yet")
		return
	}

	title, subtitle := s.novelTitleSubtitle(name)
	if r.URL.Query().Get("format") == "pdf" {
		pdf, perr := bookpdf.FromMarkdown(title, subtitle, md)
		if perr != nil {
			log.Printf("httpapi: novel pdf for %q: %v", name, perr)
			httpError(w, http.StatusInternalServerError, "could not render the PDF")
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+safeFilename(title)+"-novel.pdf\"")
		_, _ = w.Write(pdf)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+safeFilename(title)+"-novel.md\"")
	_, _ = w.Write([]byte(md))
}

// novelTitleSubtitle derives the book title and subtitle for an export from the
// session's adventure when open, falling back to the session name.
func (s *Server) novelTitleSubtitle(name string) (title, subtitle string) {
	title, subtitle = name, "A novelization of the play session"
	if os, ok := s.svc.Get(name); ok && os.Session.Adventure != nil {
		adv := os.Session.Adventure
		if adv.Title != "" {
			title = adv.Title
		}
		if len(adv.Language) >= 2 && adv.Language[:2] == "es" {
			subtitle = "Una novelización de la partida"
		}
	}
	return title, subtitle
}

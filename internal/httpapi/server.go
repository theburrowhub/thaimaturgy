// Package httpapi is the HTTP/SSE transport for thAImaturgy (issue #36, Phase B).
// It exposes the appservice facade over a JSON REST API plus a Server-Sent-Events
// stream for live session updates, and serves a placeholder page (the full web UI
// lands in Phase C). It holds no business logic — every request delegates to
// internal/appservice.
//
// SSE (not WebSocket) is used for server→client push: it needs no third-party
// dependency, and client→server actions travel over REST, which covers the
// current one-directional streaming need. It can be upgraded to WebSocket later
// if bidirectional streaming (e.g. token-level oracle output) is added.
package httpapi

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/appservice"
	"github.com/theburrowhub/thaimaturgy/internal/bookpdf"
	"github.com/theburrowhub/thaimaturgy/internal/buildinfo"
	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// webFS holds the embedded single-page web UI (issue #36, Phase C), so the server
// ships as one self-contained binary with no external assets.
//
//go:embed web
var webFS embed.FS

// maxBodyBytes bounds a JSON request body so a client can't exhaust memory.
const maxBodyBytes = 1 << 20 // 1 MiB

// maxImportBytes bounds an uploaded adventure module (.tar.gz). Modules ship
// their images, so this is generous, but still caps a runaway upload.
const maxImportBytes = 256 << 20 // 256 MiB

// maxAdventureBytes bounds an adventure.json payload (editor save/validate),
// which is text-heavy and can exceed the default 1 MiB JSON cap.
const maxAdventureBytes = 16 << 20 // 16 MiB

// sseTicketTTL bounds how long an issued SSE ticket stays valid. It is a
// reconnection WINDOW (not single-use), so native EventSource can reconnect with
// the same ticket after a transient drop without a 401.
const sseTicketTTL = 2 * time.Minute

// Server adapts an appservice.Service to HTTP. token, when non-empty, is required
// as an Authorization: Bearer header on every /api/ request (except the SSE
// stream, which EventSource can't add headers to — it uses a short-lived,
// single-use ticket instead, so the long-lived master token never travels in a
// URL where it could be logged).
type Server struct {
	svc   *appservice.Service
	token string

	// onConfigSaved, when set, is invoked with the freshly-saved config after a
	// successful PUT /api/config, so the host can rebuild the LLM provider (e.g.
	// after new API keys) without a restart.
	onConfigSaved func(*domain.Config)

	ticketMu sync.Mutex
	tickets  map[string]time.Time // single-use SSE tickets → expiry

	authMu       sync.Mutex             // guards authSessions
	authSessions map[string]authSession // per-user session tokens → user id + expiry (#151)
	// Sessions are held in memory: they DO NOT survive a server restart, so users
	// re-login after one. This is intentional (simplicity; no session store to
	// persist/rotate) — the master token still works across restarts for the GUI.

	loginAttempts *loginTracker // per-IP/per-account login throttle (#151)
	verifySem     chan struct{} // caps concurrent password-hash verifications
	authSeq       uint64        // monotonic session-creation counter (guarded by authMu)
}

// OnConfigSaved registers a callback run after a successful config save (see the
// field doc). Returns the server for chaining.
func (s *Server) OnConfigSaved(fn func(*domain.Config)) *Server {
	s.onConfigSaved = fn
	return s
}

// New builds an HTTP server over the service. token may be empty (no auth — only
// safe when bound to loopback).
func New(svc *appservice.Service, token string) *Server {
	return &Server{
		svc:           svc,
		token:         token,
		tickets:       make(map[string]time.Time),
		authSessions:  make(map[string]authSession),
		loginAttempts: newLoginTracker(),
		verifySem:     make(chan struct{}, verifyConcurrency),
	}
}

// Handler returns the routed, auth-wrapped http.Handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/version", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"version":    buildinfo.String(),
			"commit":     buildinfo.Commit,
			"build_time": buildinfo.Date,
		})
	})
	// Authentication (#151): login/logout/whoami. /api/login is exempt from the auth
	// gate in withAuth so a user can obtain a session token.
	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("POST /api/logout", s.handleLogout)
	mux.HandleFunc("GET /api/whoami", s.handleWhoami)

	// Admin user management (#151): all require an admin caller (enforced in-handler).
	mux.HandleFunc("GET /api/users", s.listUsers)
	mux.HandleFunc("POST /api/users", s.createUser)
	mux.HandleFunc("GET /api/users/{id}", s.getUser)
	mux.HandleFunc("PUT /api/users/{id}", s.updateUser)
	mux.HandleFunc("DELETE /api/users/{id}", s.deleteUser)

	mux.HandleFunc("GET /api/adventures", s.listAdventures)
	mux.HandleFunc("POST /api/adventures/import", s.importAdventure)
	mux.HandleFunc("POST /api/import-jobs", s.startImportJob)
	mux.HandleFunc("GET /api/import-jobs/{id}", s.getImportJob)
	mux.HandleFunc("GET /api/adventures/{id}", s.getAdventure)
	mux.HandleFunc("PUT /api/adventures/{id}", s.saveAdventure)
	mux.HandleFunc("DELETE /api/adventures/{id}", s.deleteAdventure)
	mux.HandleFunc("POST /api/adventures/{id}/validate", s.validateAdventure)
	mux.HandleFunc("GET /api/adventures/{id}/export", s.exportAdventure)
	mux.HandleFunc("GET /api/adventures/{id}/dmbook", s.dmbookAdventure)
	mux.HandleFunc("GET /api/adventures/{id}/asset", s.adventureAsset)
	mux.HandleFunc("GET /api/sessions", s.listSessions)
	mux.HandleFunc("POST /api/sessions", s.newSession)
	mux.HandleFunc("GET /api/sessions/{name}", s.getSession)
	mux.HandleFunc("POST /api/sessions/{name}/save", s.saveSession)
	mux.HandleFunc("POST /api/sessions/{name}/close", s.closeSession)
	mux.HandleFunc("POST /api/sessions/{name}/rename", s.renameSession)
	mux.HandleFunc("DELETE /api/sessions/{name}", s.deleteSession)
	mux.HandleFunc("POST /api/sessions/{name}/command", s.command)
	mux.HandleFunc("POST /api/sessions/{name}/oracle", s.oracle)
	mux.HandleFunc("GET /api/sessions/{name}/telegram", s.telegramStatus)
	mux.HandleFunc("POST /api/sessions/{name}/telegram/start", s.startTelegramHost)
	mux.HandleFunc("POST /api/sessions/{name}/telegram/stop", s.stopTelegramHost)
	mux.HandleFunc("GET /api/sessions/{name}/party", s.getParty)
	mux.HandleFunc("PUT /api/sessions/{name}/party", s.setParty)
	mux.HandleFunc("POST /api/sessions/{name}/party/default", s.defaultParty)
	mux.HandleFunc("POST /api/sessions/{name}/party/plan", s.planParty)
	mux.HandleFunc("POST /api/sessions/{name}/party/save-to-roster", s.savePartyToRoster)
	mux.HandleFunc("PUT /api/sessions/{name}/characters/{char}", s.updateCharacter)
	mux.HandleFunc("POST /api/sessions/{name}/novel", s.startNovelJob)
	mux.HandleFunc("GET /api/sessions/{name}/novel", s.getNovelText)
	mux.HandleFunc("PUT /api/sessions/{name}/novel", s.putNovelText)
	mux.HandleFunc("POST /api/sessions/{name}/novel/adjust", s.startNovelAdjust)
	mux.HandleFunc("GET /api/sessions/{name}/novel/download", s.downloadSessionNovel)
	mux.HandleFunc("GET /api/novel-jobs/{id}", s.getNovelJob)
	mux.HandleFunc("GET /api/novel-jobs/{id}/download", s.downloadNovel)
	mux.HandleFunc("GET /api/novel-jobs/{id}/result", s.novelJobResult)
	mux.HandleFunc("GET /api/sessions/{name}/events", s.sessionEvents)
	mux.HandleFunc("POST /api/sse-ticket", s.sseTicket)
	mux.HandleFunc("GET /api/chargen/options", s.chargenOptions)
	mux.HandleFunc("POST /api/chargen", s.chargen)
	mux.HandleFunc("GET /api/roster", s.listRoster)
	mux.HandleFunc("POST /api/roster", s.saveRosterCharacter)
	mux.HandleFunc("DELETE /api/roster/{id}", s.deleteRosterCharacter)
	mux.HandleFunc("GET /api/config", s.getConfig)
	mux.HandleFunc("PUT /api/config", s.putConfig)
	mux.HandleFunc("GET /", s.static)

	return s.withAuth(mux)
}

// withAuth protects /api/ routes. When a token is configured, the Authorization:
// Bearer header is required (a cross-origin "simple" request can't set it, so
// this is also the CSRF defense); the SSE stream is exempt and uses a ticket
// instead. When NO token is configured the server is loopback-only, so instead we
// apply an anti-CSRF / anti-DNS-rebinding guard: the Host must be a loopback name
// and any Origin must be same-origin, blocking a malicious web page from driving
// the local API.
func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api") {
			next.ServeHTTP(w, r)
			return
		}
		// Login must be reachable without prior auth (it's how you get a token). The
		// SSE stream authenticates with a single-use ticket in the query string, not
		// a header, so it's exempt from the bearer gate here.
		if r.URL.Path == "/api/login" || strings.HasSuffix(r.URL.Path, "/events") {
			next.ServeHTTP(w, r)
			return
		}
		user, loopback, status := s.resolveUser(r)
		if status != 0 {
			if status == http.StatusInternalServerError {
				httpError(w, status, "the user store is temporarily unavailable")
			} else {
				httpError(w, status, "missing or invalid token")
			}
			return
		}
		// Token-less loopback still needs the anti-CSRF / anti-DNS-rebinding guard.
		if loopback {
			if code, msg := csrfGuard(r); code != 0 {
				httpError(w, code, msg)
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userCtxKey, user)))
	})
}

// csrfGuard rejects requests that a browser on another origin could use to drive
// the token-less loopback API: the Host header must be a loopback name (defeats
// DNS-rebinding, where attacker.com resolves to 127.0.0.1), and any Origin must
// itself be loopback (defeats cross-origin fetch/EventSource). Returns (0,"") to
// allow.
func csrfGuard(r *http.Request) (int, string) {
	if !isLoopbackHost(r.Host) {
		return http.StatusForbidden, "unexpected Host header"
	}
	if o := r.Header.Get("Origin"); o != "" && !isLoopbackOrigin(o) {
		return http.StatusForbidden, "cross-origin request blocked"
	}
	return 0, ""
}

func hostOnly(hostport string) string {
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		return h
	}
	return hostport
}

func isLoopbackHost(hostport string) bool {
	h := hostOnly(hostport)
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

func isLoopbackOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return isLoopbackHost(u.Host)
}

// sseTicket issues a short-lived, single-use ticket for opening the SSE stream,
// so the long-lived master token stays out of URLs. Requires header auth (via
// withAuth). When no token is configured it still returns a (harmless) ticket.
func (s *Server) sseTicket(w http.ResponseWriter, r *http.Request) {
	var b [18]byte
	if _, err := rand.Read(b[:]); err != nil {
		httpError(w, http.StatusInternalServerError, "could not mint ticket")
		return
	}
	t := base64.RawURLEncoding.EncodeToString(b[:])
	s.ticketMu.Lock()
	// Opportunistically drop expired tickets, then record the new one.
	now := time.Now()
	for k, exp := range s.tickets {
		if now.After(exp) {
			delete(s.tickets, k)
		}
	}
	s.tickets[t] = now.Add(sseTicketTTL)
	s.ticketMu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"ticket": t})
}

// validTicket reports whether an SSE ticket is still valid. It is NOT single-use:
// the ticket stays valid for its TTL so EventSource can reconnect after a
// transient drop; expired tickets are dropped on read.
func (s *Server) validTicket(t string) bool {
	if t == "" {
		return false
	}
	s.ticketMu.Lock()
	defer s.ticketMu.Unlock()
	exp, ok := s.tickets[t]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(s.tickets, t)
		return false
	}
	return true
}

// sseAuthorized reports whether an SSE request may proceed: always when no token
// is configured (the loopback CSRF guard already ran in withAuth), otherwise it
// must present a valid, unexpired ticket.
func (s *Server) sseAuthorized(r *http.Request) bool {
	if s.token == "" {
		return true
	}
	return s.validTicket(r.URL.Query().Get("ticket"))
}

// --- handlers ------------------------------------------------------------

func (s *Server) listAdventures(w http.ResponseWriter, r *http.Request) {
	advs, err := s.svc.ListAdventures()
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, advs)
}

// importAdventure accepts a multipart upload of an adventure module (.tar.gz)
// under the form field "module", stores it to a temp file, and imports it. The
// upload is size-capped; extraction is path-traversal/zip-slip safe in storage.
func (s *Server) importAdventure(w http.ResponseWriter, r *http.Request) {
	// CSRF hardening for a browser-safelisted content type: multipart/form-data
	// triggers no CORS preflight, so (unlike the JSON endpoints, whose
	// application/json body forces one) a token-less loopback server could be
	// driven cross-origin by a hostile page. Requiring a non-safelisted custom
	// header forces a preflight the attacker's origin can't satisfy; our own
	// same-origin frontend sends it freely.
	if r.Header.Get("X-Thaim-CSRF") == "" {
		httpError(w, http.StatusForbidden, "missing X-Thaim-CSRF header")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxImportBytes)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		httpError(w, http.StatusBadRequest, "invalid or too-large upload")
		return
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()
	file, _, err := r.FormFile("module")
	if err != nil {
		httpError(w, http.StatusBadRequest, "missing 'module' file field")
		return
	}
	defer file.Close()

	tmp, err := os.CreateTemp("", "thaim-import-*.tar.gz")
	if err != nil {
		log.Printf("httpapi: stage import upload: %v", err)
		httpError(w, http.StatusInternalServerError, "could not stage the upload")
		return
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := io.Copy(tmp, file); err != nil {
		tmp.Close()
		httpError(w, http.StatusBadRequest, "could not read the upload")
		return
	}
	tmp.Close()

	adv, err := s.svc.ImportAdventure(tmpPath)
	if err != nil {
		// Import failures are dominated by a bad/unreadable archive (client error);
		// return a generic message and keep the detail (which may include internal
		// paths) in the server log rather than the response body.
		log.Printf("httpapi: import module: %v", err)
		httpError(w, http.StatusBadRequest, "the uploaded file is not a valid adventure module")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": adv.ID, "title": adv.Title})
}

// saveAdventure persists an edited adventure.json for the web editor. The id is
// pinned to the module folder by appservice, so the body can't move the module.
func (s *Server) saveAdventure(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.svc.AdventureExists(id) {
		httpError(w, http.StatusNotFound, "adventure not found")
		return
	}
	var adv domain.Adventure
	if !readJSONLimited(w, r, &adv, maxAdventureBytes) {
		return
	}
	if strings.TrimSpace(adv.Title) == "" {
		httpError(w, http.StatusBadRequest, "the adventure needs a title")
		return
	}
	if err := s.svc.SaveAdventure(id, &adv); err != nil {
		log.Printf("httpapi: save adventure %q: %v", id, err)
		httpError(w, http.StatusInternalServerError, "could not save the adventure")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// validateAdventure runs full validation over a candidate adventure and returns
// the list of problems (empty when valid), so the editor can surface them.
func (s *Server) validateAdventure(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.svc.AdventureExists(id) {
		httpError(w, http.StatusNotFound, "adventure not found")
		return
	}
	var adv domain.Adventure
	if !readJSONLimited(w, r, &adv, maxAdventureBytes) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"errors": s.svc.ValidateAdventure(id, &adv)})
}

// exportAdventure streams the adventure packaged as a .tar.gz download. A missing
// adventure is a 404; a packaging/temp-file failure is an operational 500 (with
// the detail logged, not returned, so internal paths aren't exposed).
func (s *Server) exportAdventure(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.svc.AdventureExists(id) {
		httpError(w, http.StatusNotFound, "adventure not found")
		return
	}
	path, err := s.svc.ExportModule(id)
	if err != nil {
		log.Printf("httpapi: export adventure %q: %v", id, err)
		httpError(w, http.StatusInternalServerError, "could not export the adventure")
		return
	}
	defer os.Remove(path)
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+safeFilename(id)+".tar.gz\"")
	http.ServeFile(w, r, path)
}

// dmbookAdventure renders the deterministic DM book as Markdown (default) or PDF
// (?format=pdf) and streams it as a download.
func (s *Server) dmbookAdventure(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	md, adv, err := s.svc.DMBookMarkdown(id)
	if err != nil {
		httpError(w, http.StatusNotFound, err.Error())
		return
	}
	if r.URL.Query().Get("format") == "pdf" {
		pdf, err := bookpdf.FromMarkdown(adv.Title, "DM book", md)
		if err != nil {
			log.Printf("httpapi: dmbook pdf %q: %v", id, err)
			httpError(w, http.StatusInternalServerError, "could not render the PDF")
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+safeFilename(id)+"-dmbook.pdf\"")
		_, _ = w.Write(pdf)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+safeFilename(id)+"-dmbook.md\"")
	_, _ = w.Write([]byte(md))
}

// safeFilename reduces an id to a filename-safe token for Content-Disposition.
func safeFilename(id string) string {
	id = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, id)
	if id == "" {
		return "adventure"
	}
	return id
}

// startImportJob accepts a multipart upload (a PDF under "file", or images under
// "files") and starts an asynchronous AI import, returning the job id. Like the
// module upload it requires the non-safelisted X-Thaim-CSRF header.
func (s *Server) startImportJob(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Thaim-CSRF") == "" {
		httpError(w, http.StatusForbidden, "missing X-Thaim-CSRF header")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxImportBytes)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		httpError(w, http.StatusBadRequest, "invalid or too-large upload")
		return
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()
	kind := r.FormValue("kind")
	title := r.FormValue("title")

	var src string
	switch kind {
	case "pdf":
		f, _, err := r.FormFile("file")
		if err != nil {
			httpError(w, http.StatusBadRequest, "missing 'file' (a PDF) field")
			return
		}
		defer f.Close()
		tmp, err := os.CreateTemp("", "thaim-import-*.pdf")
		if err != nil {
			httpError(w, http.StatusInternalServerError, "could not stage the upload")
			return
		}
		if _, err := io.Copy(tmp, f); err != nil {
			tmp.Close()
			_ = os.Remove(tmp.Name())
			httpError(w, http.StatusBadRequest, "could not read the upload")
			return
		}
		tmp.Close()
		src = tmp.Name()
	case "images":
		files := r.MultipartForm.File["files"]
		if len(files) == 0 {
			httpError(w, http.StatusBadRequest, "no images uploaded under 'files'")
			return
		}
		dir, err := os.MkdirTemp("", "thaim-import-imgs-*")
		if err != nil {
			httpError(w, http.StatusInternalServerError, "could not stage the upload")
			return
		}
		for i, fh := range files {
			if err := saveUpload(fh, filepath.Join(dir, fmt.Sprintf("%03d-%s", i, filepath.Base(fh.Filename)))); err != nil {
				_ = os.RemoveAll(dir)
				httpError(w, http.StatusBadRequest, "could not read an uploaded image")
				return
			}
		}
		src = dir
	default:
		httpError(w, http.StatusBadRequest, "kind must be 'pdf' or 'images'")
		return
	}

	job, err := s.svc.StartImportJob(kind, src, title)
	if err != nil {
		_ = os.RemoveAll(src)
		if errors.Is(err, appservice.ErrImportCapacity) {
			httpError(w, http.StatusServiceUnavailable, err.Error())
		} else {
			httpError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

// getImportJob returns the status/progress of an AI-import job.
func (s *Server) getImportJob(w http.ResponseWriter, r *http.Request) {
	job, ok := s.svc.ImportJobByID(r.PathValue("id"))
	if !ok {
		httpError(w, http.StatusNotFound, "no such import job")
		return
	}
	writeJSON(w, http.StatusOK, job.Snapshot())
}

// saveUpload copies a multipart file to dst.
func saveUpload(fh *multipart.FileHeader, dst string) error {
	f, err := fh.Open()
	if err != nil {
		return err
	}
	defer f.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, f)
	return err
}

// deleteAdventure removes an imported adventure and its assets. A missing
// adventure is a 404; an actual removal failure is an operational 5xx (with the
// detail logged, not returned).
func (s *Server) deleteAdventure(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.svc.AdventureExists(id) {
		httpError(w, http.StatusNotFound, "adventure not found")
		return
	}
	if err := s.svc.DeleteAdventure(id); err != nil {
		log.Printf("httpapi: delete adventure %q: %v", id, err)
		httpError(w, http.StatusInternalServerError, "could not delete the adventure")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// getAdventure returns a single imported adventure's full content, so the web UI
// can build the module browser and detail panes client-side.
func (s *Server) getAdventure(w http.ResponseWriter, r *http.Request) {
	adv, err := s.svc.LoadAdventure(r.PathValue("id"))
	if err != nil {
		httpError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, adv)
}

// adventureAsset streams a module image (map/art) by its module-relative path.
// The path is resolved and bounds-checked inside the adventure directory by
// AdventureAsset, so path traversal is rejected. It stays under the normal auth
// wrapper; the web UI fetches it with the bearer header (as a blob), since an
// <img> tag can't set Authorization when a token is configured.
func (s *Server) adventureAsset(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	if rel == "" {
		httpError(w, http.StatusBadRequest, "missing image path")
		return
	}
	abs, err := s.svc.AdventureAsset(r.PathValue("id"), rel)
	if err != nil {
		httpError(w, http.StatusNotFound, err.Error())
		return
	}
	http.ServeFile(w, r, abs)
}

func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	sess, err := s.svc.ListSessions()
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

func (s *Server) newSession(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AdventureID string `json:"adventure_id"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	name, err := s.svc.NewSession(body.AdventureID)
	if err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"name": name})
}

// getSession returns the live session state, resuming it from disk if not open.
func (s *Server) getSession(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	os, ok := s.svc.Get(name)
	if !ok {
		var err error
		if os, err = s.svc.ResumeSession(name); err != nil {
			httpError(w, http.StatusNotFound, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, os.Session.State)
}

func (s *Server) saveSession(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.SaveSession(r.PathValue("name")); err != nil {
		httpError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (s *Server) closeSession(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.CloseSession(r.PathValue("name")); err != nil {
		httpError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "closed"})
}

func (s *Server) renameSession(w http.ResponseWriter, r *http.Request) {
	var body struct {
		NewName string `json:"new_name"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if err := s.svc.RenameSession(r.PathValue("name"), body.NewName); err != nil {
		httpError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "renamed"})
}

func (s *Server) deleteSession(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.DeleteSession(r.PathValue("name")); err != nil {
		httpError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) command(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Input string `json:"input"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	res, err := s.svc.ExecuteCommand(r.PathValue("name"), body.Input)
	if err != nil {
		httpError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success":   res.Success,
		"message":   res.Message,
		"response":  res.Response,
		"ui_action": res.UIAction,
		"ui_arg":    res.UIArg,
	})
}

func (s *Server) oracle(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Input string `json:"input"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	resp, err := s.svc.AskOracle(r.Context(), r.PathValue("name"), body.Input)
	if err != nil {
		httpError(w, http.StatusConflict, err.Error())
		return
	}
	out := map[string]any{
		"answer":      resp.Answer,
		"tokens_used": resp.TokensUsed,
		"latency_ms":  resp.LatencyMs,
	}
	if resp.Error != nil {
		out["error"] = resp.Error.Error()
	}
	writeJSON(w, http.StatusOK, out)
}

// telegramStatus reports whether a session is currently hosted on Telegram.
func (s *Server) telegramStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.svc.TelegramHostStatus(r.PathValue("name")))
}

// startTelegramHost binds the server-configured Telegram bot to an open session
// and starts hosting a multiplayer game on it. The token comes from the server
// config (write-only); the request carries no secret.
func (s *Server) startTelegramHost(w http.ResponseWriter, r *http.Request) {
	username, err := s.svc.StartTelegramHost(r.PathValue("name"))
	if err != nil {
		httpError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, appservice.TelegramStatus{Hosting: true, Username: username})
}

// stopTelegramHost stops a session's Telegram host (no-op if it isn't hosting).
func (s *Server) stopTelegramHost(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.StopTelegramHost(r.PathValue("name")); err != nil {
		httpError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, appservice.TelegramStatus{})
}

func (s *Server) getParty(w http.ResponseWriter, r *http.Request) {
	party, err := s.svc.Party(r.PathValue("name"))
	if err != nil {
		httpError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, party)
}

func (s *Server) setParty(w http.ResponseWriter, r *http.Request) {
	var party []*domain.Character
	if !readJSON(w, r, &party) {
		return
	}
	if err := s.svc.SetParty(r.PathValue("name"), party); err != nil {
		httpError(w, http.StatusConflict, err.Error())
		return
	}
	s.writeParty(w, r.PathValue("name"))
}

func (s *Server) defaultParty(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.DefaultParty(r.PathValue("name")); err != nil {
		httpError(w, http.StatusConflict, err.Error())
		return
	}
	s.writeParty(w, r.PathValue("name"))
}

func (s *Server) planParty(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Prompt string `json:"prompt"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	party, err := s.svc.PlanParty(r.Context(), r.PathValue("name"), body.Prompt)
	switch {
	case errors.Is(err, appservice.ErrPartyConflict):
		httpError(w, http.StatusConflict, "the party changed while planning; reload and try again")
	case err != nil:
		httpError(w, http.StatusBadRequest, err.Error())
	default:
		writeJSON(w, http.StatusOK, party)
	}
}

func (s *Server) savePartyToRoster(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.SavePartyToRoster(r.PathValue("name")); err != nil {
		httpError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// updateCharacter applies an edited character over the named session character,
// guarded by optimistic concurrency: the body carries the baseline the client
// loaded plus the edited version, and a concurrent change is rejected with 409.
func (s *Server) updateCharacter(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Base   *domain.Character `json:"base"`
		Edited *domain.Character `json:"edited"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if body.Base == nil || body.Edited == nil {
		httpError(w, http.StatusBadRequest, "both base and edited characters are required")
		return
	}
	err := s.svc.UpdateCharacter(r.PathValue("name"), r.PathValue("char"), body.Base, body.Edited)
	switch {
	case errors.Is(err, appservice.ErrCharacterConflict):
		httpError(w, http.StatusConflict, "this character changed since you loaded it; reload and re-apply")
	case errors.Is(err, appservice.ErrNameConflict):
		httpError(w, http.StatusConflict, "another party member already uses that name; pick a different name")
	case err != nil:
		httpError(w, http.StatusBadRequest, err.Error())
	default:
		s.writeParty(w, r.PathValue("name"))
	}
}

// writeParty responds with the session's current party snapshot.
func (s *Server) writeParty(w http.ResponseWriter, name string) {
	party, err := s.svc.Party(name)
	if err != nil {
		httpError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, party)
}

// chargenOptions returns the allowed races/classes and the standard ability
// array, so the web character creator can populate its controls.
func (s *Server) chargenOptions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"races":          domain.Races,
		"classes":        domain.Classes,
		"standard_array": domain.StandardArray(),
	})
}

// chargen generates a single 5e character by the rules and returns it (it is not
// added to any session — the caller decides where it goes).
func (s *Server) chargen(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name      string                `json:"name"`
		Race      string                `json:"race"`
		Class     string                `json:"class"`
		Level     int                   `json:"level"`
		Abilities *domain.AbilityScores `json:"abilities"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		httpError(w, http.StatusBadRequest, "name is required")
		return
	}
	if body.Level < 1 {
		body.Level = 1
	}
	var c *domain.Character
	if body.Abilities != nil {
		c = domain.GenerateCharacterWithAbilities(body.Name, body.Race, body.Class, body.Level, *body.Abilities)
	} else {
		c = domain.GenerateCharacter(body.Name, body.Race, body.Class, body.Level)
	}
	writeJSON(w, http.StatusOK, c)
}

// startNovelJob begins novelizing an open session (a long AI job) and returns the
// job id to poll.
func (s *Server) startNovelJob(w http.ResponseWriter, r *http.Request) {
	job, err := s.svc.StartNovelJob(r.PathValue("name"))
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

// getNovelJob returns a novel job's status/progress.
func (s *Server) getNovelJob(w http.ResponseWriter, r *http.Request) {
	job, ok := s.svc.NovelJobByID(r.PathValue("id"))
	if !ok {
		httpError(w, http.StatusNotFound, "no such novel job")
		return
	}
	writeJSON(w, http.StatusOK, job.Snapshot())
}

// downloadNovel streams a finished novel as Markdown (default) or PDF
// (?format=pdf). 409 while it is still running.
func (s *Server) downloadNovel(w http.ResponseWriter, r *http.Request) {
	job, ok := s.svc.NovelJobByID(r.PathValue("id"))
	if !ok {
		httpError(w, http.StatusNotFound, "no such novel job")
		return
	}
	md, ready := job.Markdown()
	if !ready {
		httpError(w, http.StatusConflict, "the novel is not ready yet")
		return
	}
	if r.URL.Query().Get("format") == "pdf" {
		pdf, err := bookpdf.FromMarkdown(job.Title, job.Subtitle, md)
		if err != nil {
			log.Printf("httpapi: novel pdf %q: %v", job.ID, err)
			httpError(w, http.StatusInternalServerError, "could not render the PDF")
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+safeFilename(job.Title)+"-novel.pdf\"")
		_, _ = w.Write(pdf)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+safeFilename(job.Title)+"-novel.md\"")
	_, _ = w.Write([]byte(md))
}

// sessionEvents streams new timeline entries for a session as Server-Sent Events,
// resuming the session if needed. It sends the current log first, then tails for
// new entries until the client disconnects.
func (s *Server) sessionEvents(w http.ResponseWriter, r *http.Request) {
	if !s.sseAuthorized(r) {
		httpError(w, http.StatusUnauthorized, "missing or invalid SSE ticket (POST /api/sse-ticket)")
		return
	}
	name := r.PathValue("name")
	os, ok := s.svc.Get(name)
	if !ok {
		var err error
		if os, err = s.svc.ResumeSession(name); err != nil {
			httpError(w, http.StatusNotFound, err.Error())
			return
		}
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		httpError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	st := os.Session.State
	sent := 0
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		entries := st.RecentLog(0) // full timeline (copy, under lock)
		if len(entries) < sent {
			sent = 0 // log was trimmed/reset; resync
		}
		for _, e := range entries[sent:] {
			if b, err := json.Marshal(e); err == nil {
				fmt.Fprintf(w, "event: log\ndata: %s\n\n", b)
			}
		}
		if len(entries) > sent {
			sent = len(entries)
		} else {
			fmt.Fprint(w, ": heartbeat\n\n") // keep the connection alive
		}
		flusher.Flush()
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Server) listRoster(w http.ResponseWriter, r *http.Request) {
	chars, err := s.svc.ListCharacters()
	// Return decoded characters even if some entries were unreadable (#33), with
	// the warning in a header so a partial roster is surfaced, not hidden.
	if err != nil {
		w.Header().Set("X-Roster-Warning", err.Error())
	}
	writeJSON(w, http.StatusOK, chars)
}

func (s *Server) saveRosterCharacter(w http.ResponseWriter, r *http.Request) {
	var c domain.Character
	if !readJSON(w, r, &c) {
		return
	}
	id, err := s.svc.SaveCharacter(&c)
	if err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": id})
}

func (s *Server) deleteRosterCharacter(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.DeleteCharacter(r.PathValue("id")); err != nil {
		httpError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// getConfig returns the active configuration with secrets redacted. Secrets are
// write-only over the API: the UI shows blank fields and PUT treats an empty
// value as "leave unchanged" (see putConfig). The response embeds the plain
// domain.Config shape (so existing clients keep working) plus a read-only
// auth_source describing the credential the server auto-detected (e.g. "Claude
// Code login (Keychain)", "Gemini CLI"); AuthSource is json:"-" on the config so
// it is surfaced only through this extra field and never round-trips on save.
func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	c := s.svc.Config()
	authSource := c.AuthSource
	c.OpenAIAPIKey, c.AnthropicAPIKey, c.GeminiAPIKey, c.TelegramToken = "", "", "", ""
	writeJSON(w, http.StatusOK, struct {
		*domain.Config
		AuthSource string `json:"auth_source,omitempty"`
	}{Config: c, AuthSource: authSource})
}

// putConfig saves configuration. It starts from the CURRENT config and decodes
// the request over it, so fields the client omits — including the never-serialized
// OAuth tokens / auth source — are preserved. Secret fields are write-only: an
// empty value keeps the stored secret (the client never receives it to echo back),
// a non-empty value replaces it. On success it rebuilds the provider (if a hook
// is set) so new credentials take effect without a restart.
func (s *Server) putConfig(w http.ResponseWriter, r *http.Request) {
	cfg := s.svc.Config() // *domain.Config; decode overlays present fields
	oldOpenAI, oldAnthropic := cfg.OpenAIAPIKey, cfg.AnthropicAPIKey
	oldGemini, oldTelegram := cfg.GeminiAPIKey, cfg.TelegramToken
	if !readJSON(w, r, cfg) {
		return
	}
	if cfg.OpenAIAPIKey == "" {
		cfg.OpenAIAPIKey = oldOpenAI
	}
	if cfg.AnthropicAPIKey == "" {
		cfg.AnthropicAPIKey = oldAnthropic
	}
	if cfg.GeminiAPIKey == "" {
		cfg.GeminiAPIKey = oldGemini
	}
	if cfg.TelegramToken == "" {
		cfg.TelegramToken = oldTelegram
	}
	if err := s.svc.SaveConfig(cfg); err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if s.onConfigSaved != nil {
		s.onConfigSaved(cfg)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// static serves the embedded web UI, falling back to index.html for unknown
// paths so client-side routing works. Unsafe paths resolve to index.html.
func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	// Never SPA-fallback an /api path: an unknown/misspelled API route must 404 as
	// JSON, not return the HTML shell with 200 (which clients would misread as
	// success).
	if strings.HasPrefix(r.URL.Path, "/api") {
		httpError(w, http.StatusNotFound, "no such API endpoint")
		return
	}
	p := strings.TrimPrefix(r.URL.Path, "/")
	if p == "" || strings.Contains(p, "..") {
		p = "index.html"
	}
	b, err := webFS.ReadFile("web/" + p)
	if err != nil {
		b, err = webFS.ReadFile("web/index.html")
		p = "index.html"
		if err != nil {
			http.Error(w, "web UI not built into this binary", http.StatusInternalServerError)
			return
		}
	}
	ct := mime.TypeByExtension(path.Ext(p))
	if ct == "" {
		ct = "text/html; charset=utf-8"
	}
	w.Header().Set("Content-Type", ct)
	_, _ = w.Write(b)
}

// --- helpers -------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func httpError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// readJSON decodes a single JSON value from the request body into v under a size
// cap. It requires an application/json Content-Type (so a cross-origin "simple"
// request with a text/plain body can't reach a mutating endpoint), rejects a
// body larger than maxBodyBytes (413) or with trailing data, and 400s other
// decode failures. Returns false (having written the response) on any failure.
func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	return readJSONLimited(w, r, v, maxBodyBytes)
}

// readJSONLimited is readJSON with a caller-chosen size cap, for endpoints whose
// payload (a whole adventure) legitimately exceeds the default 1 MiB JSON limit.
func readJSONLimited(w http.ResponseWriter, r *http.Request, v any, max int64) bool {
	if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" {
		httpError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, max)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			httpError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return false
		}
		httpError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return false
	}
	if dec.More() {
		httpError(w, http.StatusBadRequest, "unexpected trailing data after JSON body")
		return false
	}
	return true
}

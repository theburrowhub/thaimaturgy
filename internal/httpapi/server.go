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
	"crypto/rand"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/appservice"
	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// webFS holds the embedded single-page web UI (issue #36, Phase C), so the server
// ships as one self-contained binary with no external assets.
//
//go:embed web
var webFS embed.FS

// maxBodyBytes bounds a JSON request body so a client can't exhaust memory.
const maxBodyBytes = 1 << 20 // 1 MiB

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

	ticketMu sync.Mutex
	tickets  map[string]time.Time // single-use SSE tickets → expiry
}

// New builds an HTTP server over the service. token may be empty (no auth — only
// safe when bound to loopback).
func New(svc *appservice.Service, token string) *Server {
	return &Server{svc: svc, token: token, tickets: make(map[string]time.Time)}
}

// Handler returns the routed, auth-wrapped http.Handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/adventures", s.listAdventures)
	mux.HandleFunc("GET /api/sessions", s.listSessions)
	mux.HandleFunc("POST /api/sessions", s.newSession)
	mux.HandleFunc("GET /api/sessions/{name}", s.getSession)
	mux.HandleFunc("POST /api/sessions/{name}/save", s.saveSession)
	mux.HandleFunc("POST /api/sessions/{name}/close", s.closeSession)
	mux.HandleFunc("POST /api/sessions/{name}/rename", s.renameSession)
	mux.HandleFunc("DELETE /api/sessions/{name}", s.deleteSession)
	mux.HandleFunc("POST /api/sessions/{name}/command", s.command)
	mux.HandleFunc("POST /api/sessions/{name}/oracle", s.oracle)
	mux.HandleFunc("GET /api/sessions/{name}/events", s.sessionEvents)
	mux.HandleFunc("POST /api/sse-ticket", s.sseTicket)
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
		if strings.HasPrefix(r.URL.Path, "/api") {
			if s.token != "" {
				if !strings.HasSuffix(r.URL.Path, "/events") && r.Header.Get("Authorization") != "Bearer "+s.token {
					httpError(w, http.StatusUnauthorized, "missing or invalid token")
					return
				}
			} else if code, msg := csrfGuard(r); code != 0 {
				httpError(w, code, msg)
				return
			}
		}
		next.ServeHTTP(w, r)
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

func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.svc.Config())
}

func (s *Server) putConfig(w http.ResponseWriter, r *http.Request) {
	var cfg domain.Config
	if !readJSON(w, r, &cfg) {
		return
	}
	if err := s.svc.SaveConfig(&cfg); err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// static serves the embedded web UI, falling back to index.html for unknown
// paths so client-side routing works. Unsafe paths resolve to index.html.
func (s *Server) static(w http.ResponseWriter, r *http.Request) {
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
	if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" {
		httpError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
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

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
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/appservice"
	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// Server adapts an appservice.Service to HTTP. token, when non-empty, is required
// as a Bearer header (or ?token= for SSE) on every /api/ request.
type Server struct {
	svc   *appservice.Service
	token string
}

// New builds an HTTP server over the service. token may be empty (no auth — only
// safe when bound to loopback).
func New(svc *appservice.Service, token string) *Server {
	return &Server{svc: svc, token: token}
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
	mux.HandleFunc("GET /api/roster", s.listRoster)
	mux.HandleFunc("POST /api/roster", s.saveRosterCharacter)
	mux.HandleFunc("DELETE /api/roster/{id}", s.deleteRosterCharacter)
	mux.HandleFunc("GET /api/config", s.getConfig)
	mux.HandleFunc("PUT /api/config", s.putConfig)
	mux.HandleFunc("GET /", s.index)

	return s.withAuth(mux)
}

// withAuth enforces the bearer token on /api/ routes when one is configured.
func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.token != "" && len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/api" {
			if !s.authorized(r) {
				httpError(w, http.StatusUnauthorized, "missing or invalid token")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) authorized(r *http.Request) bool {
	if h := r.Header.Get("Authorization"); h == "Bearer "+s.token {
		return true
	}
	// EventSource can't set headers, so allow the token as a query param for SSE.
	return r.URL.Query().Get("token") == s.token
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

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, indexHTML)
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

// readJSON decodes the request body into v, writing a 400 and returning false on
// failure.
func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		httpError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return false
	}
	return true
}

const indexHTML = `<!doctype html><html><head><meta charset="utf-8">
<title>thAImaturgy server</title></head><body>
<h1>thAImaturgy server</h1>
<p>The JSON API is under <code>/api/</code>. The full web UI ships in Phase C (#36).</p>
<p>Try <a href="/api/health">/api/health</a>, <a href="/api/adventures">/api/adventures</a>,
<a href="/api/sessions">/api/sessions</a>.</p>
</body></html>`

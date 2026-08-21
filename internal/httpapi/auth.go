package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// User authentication for remote access (#151). The server supports three ways to
// be authorized, resolved by resolveUser:
//
//  1. A per-user SESSION token, obtained from POST /api/login and sent as
//     "Authorization: Bearer <token>". It maps to a registered user with their role.
//  2. The MASTER token (THAIM_SERVER_TOKEN), which acts as a break-glass admin —
//     this keeps existing single-token deployments and the desktop GUI working, and
//     bootstraps the first admin before any user is created.
//  3. TOKEN-LESS LOOPBACK (no master token configured): the loopback CSRF guard
//     applies and the caller is treated as a local admin (unchanged local-dev
//     behavior).
//
// The resolved user is attached to the request context; authorization by role
// (restricting a player to their own characters) is layered on top separately.

const sessionTTL = 30 * 24 * time.Hour

type authSession struct {
	userID string
	expiry time.Time
}

// masterAdmin is the synthetic identity for the master token / token-less loopback:
// a non-persisted admin with an empty id. IsAdmin() is true; it owns no specific
// roster characters (admins aren't restricted anyway).
func masterAdmin() *domain.User {
	return &domain.User{Username: "(master)", Role: domain.RoleAdmin}
}

type ctxKey int

const userCtxKey ctxKey = 0

// UserFromContext returns the authenticated user attached by withAuth, if any.
func UserFromContext(ctx context.Context) (*domain.User, bool) {
	u, ok := ctx.Value(userCtxKey).(*domain.User)
	return u, ok
}

func newToken() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

// createSession mints a session token for a user and stores it with an expiry.
func (s *Server) createSession(userID string) (string, error) {
	tok, err := newToken()
	if err != nil {
		return "", err
	}
	s.authMu.Lock()
	s.authSessions[tok] = authSession{userID: userID, expiry: time.Now().Add(sessionTTL)}
	s.authMu.Unlock()
	return tok, nil
}

// sessionUserID returns the (unexpired) user id for a token, purging it if expired.
func (s *Server) sessionUserID(tok string) (string, bool) {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	sess, ok := s.authSessions[tok]
	if !ok {
		return "", false
	}
	if time.Now().After(sess.expiry) {
		delete(s.authSessions, tok)
		return "", false
	}
	return sess.userID, true
}

func (s *Server) revokeSession(tok string) {
	s.authMu.Lock()
	delete(s.authSessions, tok)
	s.authMu.Unlock()
}

// bearerToken extracts the token from an Authorization: Bearer header.
func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if after, ok := strings.CutPrefix(h, "Bearer "); ok {
		return strings.TrimSpace(after)
	}
	return ""
}

// resolveUser determines the caller's identity for an /api request. It returns
// the user and true when authorized, or (nil,false) when the request should be
// rejected. It also returns whether the loopback CSRF guard still needs to run
// (only for the token-less loopback case).
func (s *Server) resolveUser(r *http.Request) (user *domain.User, loopback bool, ok bool) {
	tok := bearerToken(r)

	// 1) A live per-user session token.
	if tok != "" {
		if uid, live := s.sessionUserID(tok); live {
			if u, err := s.svc.LoadUser(uid); err == nil {
				return u, false, true
			}
			// The user was deleted since login — drop the stale session.
			s.revokeSession(tok)
		}
	}

	// 2) The master token → break-glass admin.
	if s.token != "" {
		if tok == s.token {
			return masterAdmin(), false, true
		}
		return nil, false, false // a token is configured but this one is neither
	}

	// 3) No master token: token-less loopback = local admin (guard applied by caller).
	return masterAdmin(), true, true
}

// handleLogin authenticates a username/password and returns a session token.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !readJSONLimited(w, r, &body, 1<<20) {
		return
	}
	u, ok := s.svc.Authenticate(body.Username, body.Password)
	if !ok {
		httpError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	tok, err := s.createSession(u.ID)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "could not start a session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": tok, "user": u.Sanitized()})
}

// handleLogout invalidates the caller's session token (no-op for the master token).
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if tok := bearerToken(r); tok != "" {
		s.revokeSession(tok)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleWhoami returns the authenticated user (sanitized).
func (s *Server) handleWhoami(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	writeJSON(w, http.StatusOK, u.Sanitized())
}

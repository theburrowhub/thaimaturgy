package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/appservice"
	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
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

const (
	sessionTTL = 30 * 24 * time.Hour
	// Session-store bounds so abandoned tokens can't grow authSessions without
	// limit; oldest sessions are evicted past these caps.
	maxSessionsPerUser = 10
	maxSessionsGlobal  = 10_000
	// Login throttling: a key (client IP or account) that fails loginMaxFails times
	// within loginFailWindow is locked out for loginLockout.
	loginMaxFails   = 5
	loginFailWindow = 15 * time.Minute
	loginLockout    = 15 * time.Minute
	// verifyConcurrency caps simultaneous password-hash verifications so a burst of
	// login attempts can't exhaust CPU with bcrypt work.
	verifyConcurrency = 16
)

type authSession struct {
	userID string
	expiry time.Time
	seq    uint64 // monotonic creation order, for deterministic oldest-first eviction
}

// loginTracker throttles failed logins per key (client IP and per account) to
// blunt online password guessing. It is safe for concurrent use.
type loginTracker struct {
	mu   sync.Mutex
	fail map[string]*failState
}

type failState struct {
	count     int
	firstFail time.Time
	lockUntil time.Time
}

func newLoginTracker() *loginTracker { return &loginTracker{fail: map[string]*failState{}} }

// locked reports whether key is currently locked out and for how much longer.
func (t *loginTracker) locked(key string, now time.Time) (bool, time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	fs := t.fail[key]
	if fs != nil && now.Before(fs.lockUntil) {
		return true, fs.lockUntil.Sub(now)
	}
	return false, 0
}

// recordFail counts a failed attempt, starting a lockout once the threshold is
// reached within the window. The counter resets if the window has elapsed.
func (t *loginTracker) recordFail(key string, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	fs := t.fail[key]
	if fs == nil || now.Sub(fs.firstFail) > loginFailWindow {
		fs = &failState{firstFail: now}
		t.fail[key] = fs
	}
	fs.count++
	if fs.count >= loginMaxFails {
		fs.lockUntil = now.Add(loginLockout)
	}
	t.sweepLocked(now)
}

// reset clears a key's failure state (called on a successful login).
func (t *loginTracker) reset(key string) {
	t.mu.Lock()
	delete(t.fail, key)
	t.mu.Unlock()
}

// sweepLocked drops entries whose window has elapsed and whose lockout is over,
// so the map can't grow without bound from one-off IPs/usernames. Caller holds mu.
func (t *loginTracker) sweepLocked(now time.Time) {
	for k, fs := range t.fail {
		if now.After(fs.lockUntil) && now.Sub(fs.firstFail) > loginFailWindow {
			delete(t.fail, k)
		}
	}
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
// It first purges expired sessions and enforces the per-user and global caps
// (evicting the oldest), so abandoned tokens can't grow the store without bound.
func (s *Server) createSession(userID string) (string, error) {
	tok, err := newToken()
	if err != nil {
		return "", err
	}
	now := time.Now()
	s.authMu.Lock()
	defer s.authMu.Unlock()
	for t, sess := range s.authSessions { // purge expired
		if now.After(sess.expiry) {
			delete(s.authSessions, t)
		}
	}
	for s.countUserSessionsLocked(userID) >= maxSessionsPerUser { // per-user cap (leave room for the new one)
		s.evictOldestLocked(userID)
	}
	for len(s.authSessions) >= maxSessionsGlobal { // global cap
		s.evictOldestLocked("")
	}
	s.authSeq++
	s.authSessions[tok] = authSession{userID: userID, expiry: now.Add(sessionTTL), seq: s.authSeq}
	return tok, nil
}

// countUserSessionsLocked counts a user's live sessions. Caller holds authMu.
func (s *Server) countUserSessionsLocked(userID string) int {
	n := 0
	for _, sess := range s.authSessions {
		if sess.userID == userID {
			n++
		}
	}
	return n
}

// evictOldestLocked deletes the oldest session (lowest creation seq), optionally
// restricted to a user id (empty = any). Caller holds authMu.
func (s *Server) evictOldestLocked(userID string) {
	var oldestTok string
	var oldestSeq uint64
	for t, sess := range s.authSessions {
		if userID != "" && sess.userID != userID {
			continue
		}
		if oldestTok == "" || sess.seq < oldestSeq {
			oldestTok, oldestSeq = t, sess.seq
		}
	}
	if oldestTok != "" {
		delete(s.authSessions, oldestTok)
	}
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

// resolveUser determines the caller's identity for an /api request. status is 0
// when authorized; otherwise it is the HTTP status to return (401 unauthorized,
// 500 on a transient store failure). loopback is true only for the token-less
// loopback case, where the caller must still run the CSRF guard.
func (s *Server) resolveUser(r *http.Request) (user *domain.User, loopback bool, status int) {
	tok := bearerToken(r)

	// 1) A live per-user session token.
	if tok != "" {
		if uid, live := s.sessionUserID(tok); live {
			u, err := s.svc.LoadUser(uid)
			if err == nil {
				return u, false, 0
			}
			if errors.Is(err, storage.ErrUserNotFound) {
				s.revokeSession(tok) // the user was really deleted — drop the stale session
			} else {
				// Transient store failure: KEEP the session (don't lock the user out
				// permanently) and surface a server error so a retry can succeed.
				return nil, false, http.StatusInternalServerError
			}
		}
	}

	// 2) The master token → break-glass admin.
	if s.token != "" {
		if tok == s.token {
			return masterAdmin(), false, 0
		}
		return nil, false, http.StatusUnauthorized // a token is configured but this one is neither
	}

	// 3) No master token: token-less loopback = local admin (guard applied by caller).
	return masterAdmin(), true, 0
}

// clientIP is the direct peer's IP (RemoteAddr). It intentionally ignores
// X-Forwarded-For, which a client can spoof to evade per-IP throttling.
func clientIP(r *http.Request) string {
	if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return h
	}
	return r.RemoteAddr
}

// handleLogin authenticates a username/password and returns a session token. It
// throttles by client IP and by account (lockout after repeated failures), and
// bounds concurrent password-hash verifications, so it can't be used for online
// guessing or CPU exhaustion.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !readJSONLimited(w, r, &body, 1<<20) {
		return
	}
	now := time.Now()
	ipKey := "ip:" + clientIP(r)
	acctKey := "user:" + domain.NormalizeUsername(body.Username)
	lockedNow := func() bool {
		for _, key := range []string{ipKey, acctKey} {
			if locked, retry := s.loginAttempts.locked(key, time.Now()); locked {
				w.Header().Set("Retry-After", retryAfterSeconds(retry))
				httpError(w, http.StatusTooManyRequests, "too many login attempts — try again later")
				return true
			}
		}
		return false
	}
	// Fast pre-check: reject a locked key before doing any work.
	if lockedNow() {
		return
	}

	// Admit to the verification stage without blocking: when bcrypt capacity is
	// full, shed load with 429 rather than queuing unbounded goroutines that would
	// each still run a hash check — the concurrent-burst bypass. The slot is held
	// through Authenticate + createSession (both cheap after the hash) and released
	// on any return.
	select {
	case s.verifySem <- struct{}{}:
		defer func() { <-s.verifySem }()
	default:
		w.Header().Set("Retry-After", "1")
		httpError(w, http.StatusTooManyRequests, "the server is busy — try again shortly")
		return
	}
	// Re-check lockout AFTER admission: a burst that all passed the pre-check must
	// not still run bcrypt once the key locked while they were queued/among the
	// first wave.
	if lockedNow() {
		return
	}

	u, err := s.svc.Authenticate(body.Username, body.Password)
	if err != nil {
		if errors.Is(err, appservice.ErrInvalidCredentials) {
			s.loginAttempts.recordFail(ipKey, now)
			s.loginAttempts.recordFail(acctKey, now)
			httpError(w, http.StatusUnauthorized, "invalid username or password")
			return
		}
		// A storage/internal failure is NOT a credential failure: don't count it
		// against the user and don't report it as a 401.
		httpError(w, http.StatusInternalServerError, "login is temporarily unavailable")
		return
	}
	s.loginAttempts.reset(ipKey)
	s.loginAttempts.reset(acctKey)

	tok, err := s.createSession(u.ID)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "could not start a session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": tok, "user": u.Sanitized()})
}

func retryAfterSeconds(d time.Duration) string {
	secs := int(d.Seconds())
	if secs < 1 {
		secs = 1
	}
	return strconv.Itoa(secs)
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

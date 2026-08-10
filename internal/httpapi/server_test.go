package httpapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/appservice"
	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

func newTestServer(t *testing.T, token string) *httptest.Server {
	t.Helper()
	store, err := storage.NewWithPath(t.TempDir())
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	adv := &domain.Adventure{
		SchemaVersion: domain.SchemaVersion, ID: "crypt", Title: "The Crypt",
		Zones: []domain.Zone{{ID: "z1", Name: "Entrance", Rooms: []domain.Room{{ID: "r1", Name: "Gate"}}}},
	}
	dir := store.AdventureDir("crypt")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data, _ := json.MarshalIndent(adv, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, storage.AdventureFile), data, 0o644); err != nil {
		t.Fatalf("write adventure: %v", err)
	}
	svc := appservice.New(store, domain.DefaultConfig(), nil)
	ts := httptest.NewServer(New(svc, token).Handler())
	t.Cleanup(ts.Close)
	return ts
}

func doJSON(t *testing.T, method, url, body string) (*http.Response, map[string]any) {
	t.Helper()
	var rdr *bytes.Reader
	if body != "" {
		rdr = bytes.NewReader([]byte(body))
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, url, rdr)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	resp.Body.Close()
	return resp, out
}

func TestRESTFlow(t *testing.T) {
	ts := newTestServer(t, "")

	if resp, _ := doJSON(t, "GET", ts.URL+"/api/health", ""); resp.StatusCode != 200 {
		t.Fatalf("health = %d", resp.StatusCode)
	}

	// New session.
	resp, out := doJSON(t, "POST", ts.URL+"/api/sessions", `{"adventure_id":"crypt"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("new session = %d (%v)", resp.StatusCode, out)
	}
	name, _ := out["name"].(string)
	if name == "" {
		t.Fatal("no session name returned")
	}

	// Get session state.
	resp, state := doJSON(t, "GET", ts.URL+"/api/sessions/"+name, "")
	if resp.StatusCode != 200 || state["adventure_id"] != "crypt" {
		t.Fatalf("get session = %d (%v)", resp.StatusCode, state)
	}

	// Run a command.
	resp, cmd := doJSON(t, "POST", ts.URL+"/api/sessions/"+name+"/command", `{"input":"/note a torch gutters"}`)
	if resp.StatusCode != 200 || cmd["success"] != true {
		t.Fatalf("command = %d (%v)", resp.StatusCode, cmd)
	}

	// Save + close.
	if resp, _ := doJSON(t, "POST", ts.URL+"/api/sessions/"+name+"/save", ""); resp.StatusCode != 200 {
		t.Fatalf("save = %d", resp.StatusCode)
	}
	if resp, _ := doJSON(t, "POST", ts.URL+"/api/sessions/"+name+"/close", ""); resp.StatusCode != 200 {
		t.Fatalf("close = %d", resp.StatusCode)
	}

	// Roster round-trip.
	resp, rc := doJSON(t, "POST", ts.URL+"/api/roster", `{"name":"Alice","race":"Elf","class":"Wizard","max_hp":6,"current_hp":6}`)
	if resp.StatusCode != 200 || rc["id"] == "" {
		t.Fatalf("save roster = %d (%v)", resp.StatusCode, rc)
	}
	req, _ := http.NewRequest("GET", ts.URL+"/api/roster", nil)
	lr, _ := http.DefaultClient.Do(req)
	var chars []map[string]any
	_ = json.NewDecoder(lr.Body).Decode(&chars)
	lr.Body.Close()
	if len(chars) != 1 {
		t.Fatalf("roster list = %d; want 1", len(chars))
	}

	// Config get/put.
	if resp, _ := doJSON(t, "PUT", ts.URL+"/api/config", `{"language":"es"}`); resp.StatusCode != 200 {
		t.Fatalf("put config = %d", resp.StatusCode)
	}
	if _, cfg := doJSON(t, "GET", ts.URL+"/api/config", ""); cfg["language"] != "es" {
		t.Errorf("config not adopted: %v", cfg["language"])
	}
}

func TestAuthToken(t *testing.T) {
	ts := newTestServer(t, "s3cret")
	// No token → 401.
	if resp, _ := doJSON(t, "GET", ts.URL+"/api/adventures", ""); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no token = %d; want 401", resp.StatusCode)
	}
	// Correct bearer → 200.
	req, _ := http.NewRequest("GET", ts.URL+"/api/adventures", nil)
	req.Header.Set("Authorization", "Bearer s3cret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("with token = %v / %d", err, resp.StatusCode)
	}
	resp.Body.Close()
	// The index page stays public.
	r2, _ := http.Get(ts.URL + "/")
	if r2.StatusCode != 200 {
		t.Errorf("index should be public, got %d", r2.StatusCode)
	}
	r2.Body.Close()
}

func TestSSETicketAuth(t *testing.T) {
	ts := newTestServer(t, "s3cret")
	_, out := func() (*http.Response, map[string]any) {
		req, _ := http.NewRequest("POST", ts.URL+"/api/sessions", bytes.NewReader([]byte(`{"adventure_id":"crypt"}`)))
		req.Header.Set("Authorization", "Bearer s3cret")
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("new session: %v", err)
		}
		var m map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&m)
		resp.Body.Close()
		return resp, m
	}()
	name := out["name"].(string)

	// SSE without a ticket → 401.
	r0, _ := http.Get(ts.URL + "/api/sessions/" + name + "/events")
	if r0.StatusCode != http.StatusUnauthorized {
		t.Fatalf("SSE without ticket = %d; want 401", r0.StatusCode)
	}
	r0.Body.Close()

	// A ticket can't be minted without the bearer header.
	rNo, _ := http.Post(ts.URL+"/api/sse-ticket", "application/json", nil)
	if rNo.StatusCode != http.StatusUnauthorized {
		t.Fatalf("sse-ticket without auth = %d; want 401", rNo.StatusCode)
	}
	rNo.Body.Close()

	// Mint a ticket with the header, then open SSE with it.
	treq, _ := http.NewRequest("POST", ts.URL+"/api/sse-ticket", nil)
	treq.Header.Set("Authorization", "Bearer s3cret")
	tr, err := http.DefaultClient.Do(treq)
	if err != nil {
		t.Fatalf("sse-ticket: %v", err)
	}
	var tk map[string]string
	_ = json.NewDecoder(tr.Body).Decode(&tk)
	tr.Body.Close()
	if tk["ticket"] == "" {
		t.Fatal("no ticket issued")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/api/sessions/"+name+"/events?ticket="+tk["ticket"], nil)
	r1, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("SSE with ticket: %v", err)
	}
	defer r1.Body.Close()
	if r1.StatusCode != 200 || !strings.HasPrefix(r1.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("SSE with ticket = %d / %q", r1.StatusCode, r1.Header.Get("Content-Type"))
	}
	// The ticket stays valid within its window so EventSource can reconnect.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	req2, _ := http.NewRequestWithContext(ctx2, "GET", ts.URL+"/api/sessions/"+name+"/events?ticket="+tk["ticket"], nil)
	r2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("SSE reconnect: %v", err)
	}
	if r2.StatusCode != 200 {
		t.Errorf("reconnect with same ticket = %d; want 200 (bounded window)", r2.StatusCode)
	}
	r2.Body.Close()
	// A bogus ticket is rejected.
	r3, _ := http.Get(ts.URL + "/api/sessions/" + name + "/events?ticket=nope")
	if r3.StatusCode != http.StatusUnauthorized {
		t.Errorf("bogus ticket = %d; want 401", r3.StatusCode)
	}
	r3.Body.Close()
}

func TestCSRFGuardOnLoopbackNoToken(t *testing.T) {
	ts := newTestServer(t, "") // no token → loopback CSRF guard active
	base := ts.URL + "/api/adventures"

	// A cross-origin Origin is blocked.
	req, _ := http.NewRequest("GET", base, nil)
	req.Header.Set("Origin", "http://evil.example.com")
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("cross-origin request = %d; want 403", resp.StatusCode)
	}
	resp.Body.Close()

	// A spoofed (rebinding) Host is blocked.
	req2, _ := http.NewRequest("GET", base, nil)
	req2.Host = "attacker.com"
	resp2, _ := http.DefaultClient.Do(req2)
	if resp2.StatusCode != http.StatusForbidden {
		t.Errorf("rebinding Host = %d; want 403", resp2.StatusCode)
	}
	resp2.Body.Close()

	// A text/plain body to a mutating endpoint is rejected (415), so a simple
	// cross-origin POST can't reach the JSON handler.
	tp, _ := http.Post(ts.URL+"/api/sessions", "text/plain", bytes.NewReader([]byte(`{"adventure_id":"crypt"}`)))
	if tp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("text/plain body = %d; want 415", tp.StatusCode)
	}
	tp.Body.Close()
}

func TestServesEmbeddedWebUI(t *testing.T) {
	ts := newTestServer(t, "")
	// Index at /.
	r, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	body, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if r.StatusCode != 200 || !strings.Contains(string(body), "thAImaturgy") {
		t.Fatalf("index = %d, body has app marker? %v", r.StatusCode, strings.Contains(string(body), "thAImaturgy"))
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "text/html") {
		t.Errorf("index content-type = %q", r.Header.Get("Content-Type"))
	}
	// The JS asset is served with a JS content-type.
	rj, err := http.Get(ts.URL + "/app.js")
	if err != nil {
		t.Fatalf("GET /app.js: %v", err)
	}
	defer rj.Body.Close()
	if rj.StatusCode != 200 || !strings.Contains(rj.Header.Get("Content-Type"), "javascript") {
		t.Errorf("app.js = %d / %q", rj.StatusCode, rj.Header.Get("Content-Type"))
	}
	// An unknown NON-API path falls back to index.html (SPA routing).
	rf, _ := http.Get(ts.URL + "/some/spa/route")
	fb, _ := io.ReadAll(rf.Body)
	rf.Body.Close()
	if !strings.Contains(string(fb), "thAImaturgy") {
		t.Error("unknown non-API path should fall back to index.html")
	}
	// An unknown API path must 404 (JSON), NOT return the HTML shell with 200.
	ra, _ := http.Get(ts.URL + "/api/nope")
	ab, _ := io.ReadAll(ra.Body)
	ra.Body.Close()
	if ra.StatusCode != http.StatusNotFound || strings.Contains(string(ab), "<!doctype") {
		t.Errorf("unknown API path = %d, body=%q; want 404 JSON", ra.StatusCode, string(ab))
	}
}

func TestWebAssetsPublicUnderToken(t *testing.T) {
	ts := newTestServer(t, "s3cret")
	// The UI shell and assets must load WITHOUT a token, so the page (which holds
	// the token input) can bootstrap; only /api needs the token.
	for _, p := range []string{"/", "/app.js", "/style.css"} {
		r, err := http.Get(ts.URL + p)
		if err != nil {
			t.Fatalf("GET %s: %v", p, err)
		}
		r.Body.Close()
		if r.StatusCode != 200 {
			t.Errorf("asset %s under token = %d; want 200 (public shell)", p, r.StatusCode)
		}
	}
}

func TestBodyTooLarge(t *testing.T) {
	ts := newTestServer(t, "")
	big := `{"adventure_id":"` + strings.Repeat("a", (1<<20)+16) + `"}`
	resp, _ := doJSON(t, "POST", ts.URL+"/api/sessions", big)
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized body = %d; want 413", resp.StatusCode)
	}
}

func TestSSEStreamsLog(t *testing.T) {
	ts := newTestServer(t, "")
	_, out := doJSON(t, "POST", ts.URL+"/api/sessions", `{"adventure_id":"crypt"}`)
	name := out["name"].(string)
	doJSON(t, "POST", ts.URL+"/api/sessions/"+name+"/command", `{"input":"/note something happened"}`)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/api/sessions/"+name+"/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("sse: %v", err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content-type = %q; want text/event-stream", ct)
	}
	// Read until we see a log data line carrying the note (or time out).
	sc := bufio.NewScanner(resp.Body)
	found := false
	for sc.Scan() {
		if strings.Contains(sc.Text(), "something happened") {
			found = true
			break
		}
	}
	if !found {
		t.Error("SSE did not stream the session's log entry")
	}
}

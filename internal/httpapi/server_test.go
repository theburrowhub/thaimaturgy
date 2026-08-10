package httpapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
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

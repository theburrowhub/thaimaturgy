package httpapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
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
		Zones: []domain.Zone{{ID: "z1", Name: "Entrance", MapImage: "assets/map.png",
			Rooms: []domain.Room{{ID: "r1", Name: "Gate", Image: "assets/map.png"}}}},
	}
	dir := store.AdventureDir("crypt")
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data, _ := json.MarshalIndent(adv, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, storage.AdventureFile), data, 0o644); err != nil {
		t.Fatalf("write adventure: %v", err)
	}
	// A tiny image asset so the asset route has something to serve.
	if err := os.WriteFile(filepath.Join(dir, "assets", "map.png"), []byte("\x89PNG\r\n\x1a\nfake"), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
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

func TestAdventureContentAndAsset(t *testing.T) {
	ts := newTestServer(t, "")

	// Full adventure content for the module browser.
	resp, out := doJSON(t, "GET", ts.URL+"/api/adventures/crypt", "")
	if resp.StatusCode != 200 || out["title"] != "The Crypt" {
		t.Fatalf("get adventure = %d (%v)", resp.StatusCode, out)
	}
	// Unknown adventure → 404.
	if resp, _ := doJSON(t, "GET", ts.URL+"/api/adventures/nope", ""); resp.StatusCode != 404 {
		t.Errorf("unknown adventure = %d; want 404", resp.StatusCode)
	}

	// Asset streams the image bytes.
	ar, err := http.Get(ts.URL + "/api/adventures/crypt/asset?path=assets/map.png")
	if err != nil || ar.StatusCode != 200 {
		t.Fatalf("asset = %v / %d", err, ar.StatusCode)
	}
	b, _ := io.ReadAll(ar.Body)
	ar.Body.Close()
	if len(b) == 0 {
		t.Error("asset body should not be empty")
	}

	// Path traversal is rejected (resolved inside the module dir).
	tr, _ := http.Get(ts.URL + "/api/adventures/crypt/asset?path=../../../../etc/passwd")
	if tr.StatusCode == 200 {
		t.Error("path traversal should be rejected")
	}
	tr.Body.Close()

	// Missing path parameter → 400.
	mr, _ := http.Get(ts.URL + "/api/adventures/crypt/asset")
	if mr.StatusCode != 400 {
		t.Errorf("missing path = %d; want 400", mr.StatusCode)
	}
	mr.Body.Close()
}

func getArray(t *testing.T, url string) []map[string]any {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	var out []map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out
}

func TestPartyAndCharacterEndpoints(t *testing.T) {
	ts := newTestServer(t, "")
	_, out := doJSON(t, "POST", ts.URL+"/api/sessions", `{"adventure_id":"crypt"}`)
	name := out["name"].(string)
	base := ts.URL + "/api/sessions/" + name

	// Set a two-member party.
	party := `[{"name":"Alden","race":"Human","class":"Fighter","level":1,"max_hp":10,"current_hp":10,"ac":10},` +
		`{"name":"Naivara","race":"Elf","class":"Wizard","level":1,"max_hp":6,"current_hp":6,"ac":12}]`
	if resp, _ := doJSON(t, "PUT", base+"/party", party); resp.StatusCode != 200 {
		t.Fatalf("set party = %d", resp.StatusCode)
	}
	if got := getArray(t, base+"/party"); len(got) != 2 {
		t.Fatalf("party = %d; want 2", len(got))
	}

	// Chargen options + generate a character.
	_, opts := doJSON(t, "GET", ts.URL+"/api/chargen/options", "")
	if races, _ := opts["races"].([]any); len(races) == 0 {
		t.Error("chargen options should list races")
	}
	_, gen := doJSON(t, "POST", ts.URL+"/api/chargen", `{"name":"Borin","race":"Dwarf","class":"Cleric","level":2}`)
	if gen["name"] != "Borin" || gen["max_hp"] == nil {
		t.Errorf("chargen should return a built character: %v", gen)
	}

	// Optimistic concurrency: first edit with the correct baseline succeeds…
	cur := getArray(t, base+"/party")
	baseChar := cur[0] // Alden
	baseJSON, _ := json.Marshal(baseChar)
	edited := map[string]any{}
	for k, v := range baseChar {
		edited[k] = v
	}
	edited["current_hp"] = 5
	editedJSON, _ := json.Marshal(edited)
	body1 := `{"base":` + string(baseJSON) + `,"edited":` + string(editedJSON) + `}`
	if resp, _ := doJSON(t, "PUT", base+"/characters/Alden", body1); resp.StatusCode != 200 {
		t.Fatalf("update character = %d", resp.StatusCode)
	}
	// …but re-using the now-stale baseline is rejected with 409.
	if resp, _ := doJSON(t, "PUT", base+"/characters/Alden", body1); resp.StatusCode != http.StatusConflict {
		t.Errorf("stale update = %d; want 409", resp.StatusCode)
	}

	// Renaming a member onto another member's name is rejected (409), so
	// name-based addressing stays unambiguous.
	cur2 := getArray(t, base+"/party")
	fresh, _ := json.Marshal(cur2[0]) // current Alden
	rename := map[string]any{}
	for k, v := range cur2[0] {
		rename[k] = v
	}
	rename["name"] = "Naivara" // collides with the other member
	renameJSON, _ := json.Marshal(rename)
	collide := `{"base":` + string(fresh) + `,"edited":` + string(renameJSON) + `}`
	if resp, _ := doJSON(t, "PUT", base+"/characters/Alden", collide); resp.StatusCode != http.StatusConflict {
		t.Errorf("colliding rename = %d; want 409", resp.StatusCode)
	}

	// Default party replaces the roster with the sample party.
	if resp, _ := doJSON(t, "POST", base+"/party/default", ""); resp.StatusCode != 200 {
		t.Fatalf("default party = %d", resp.StatusCode)
	}
	if got := getArray(t, base+"/party"); len(got) == 0 {
		t.Error("default party should not be empty")
	}

	// Save party to roster, then it should appear in the roster listing.
	if resp, _ := doJSON(t, "POST", base+"/party/save-to-roster", ""); resp.StatusCode != 200 {
		t.Fatalf("save to roster = %d", resp.StatusCode)
	}
	if got := getArray(t, ts.URL+"/api/roster"); len(got) == 0 {
		t.Error("roster should contain the saved party")
	}

	// PlanParty needs an AI provider (nil in tests) → 400, not a crash.
	if resp, _ := doJSON(t, "POST", base+"/party/plan", `{"prompt":"a balanced trio"}`); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("plan without provider = %d; want 400", resp.StatusCode)
	}

	// Close the session so background autosaves stop before temp-dir teardown.
	doJSON(t, "POST", base+"/close", "")
}

func TestImportAndDeleteAdventure(t *testing.T) {
	ts := newTestServer(t, "")

	// Build a minimal module .tar.gz to upload.
	src := t.TempDir()
	adv := &domain.Adventure{
		SchemaVersion: domain.SchemaVersion, ID: "imported", Title: "Imported Module",
		Zones: []domain.Zone{{ID: "z1", Name: "Start", Rooms: []domain.Room{{ID: "r1", Name: "Door"}}}},
	}
	data, _ := json.MarshalIndent(adv, "", "  ")
	if err := os.WriteFile(filepath.Join(src, storage.AdventureFile), data, 0o644); err != nil {
		t.Fatalf("write adventure: %v", err)
	}
	pkg := filepath.Join(t.TempDir(), "module.tar.gz")
	if err := storage.PackageModule(src, pkg); err != nil {
		t.Fatalf("package: %v", err)
	}

	// Serialize the multipart body once, then reuse the bytes for each request.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("module", "module.tar.gz")
	pkgBytes, _ := os.ReadFile(pkg)
	_, _ = fw.Write(pkgBytes)
	mw.Close()
	ctype := mw.FormDataContentType()
	body := buf.Bytes()

	doImport := func(csrf bool) *http.Response {
		req, _ := http.NewRequest("POST", ts.URL+"/api/adventures/import", bytes.NewReader(body))
		req.Header.Set("Content-Type", ctype)
		if csrf {
			req.Header.Set("X-Thaim-CSRF", "1")
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("import request: %v", err)
		}
		return resp
	}

	// Without the CSRF header the safelisted upload is rejected (403).
	if resp := doImport(false); resp.StatusCode != http.StatusForbidden {
		resp.Body.Close()
		t.Fatalf("import without CSRF header = %d; want 403", resp.StatusCode)
	}
	// With it, the import succeeds.
	if resp := doImport(true); resp.StatusCode != http.StatusCreated {
		resp.Body.Close()
		t.Fatalf("import = %d; want 201", resp.StatusCode)
	} else {
		resp.Body.Close()
	}

	// It now appears in the library.
	if got := getArray(t, ts.URL+"/api/adventures"); len(got) < 2 {
		t.Fatalf("library should list the imported adventure, got %d", len(got))
	}

	// Delete it.
	if resp, _ := doJSON(t, "DELETE", ts.URL+"/api/adventures/imported", ""); resp.StatusCode != 200 {
		t.Fatalf("delete = %d", resp.StatusCode)
	}
	if resp, _ := doJSON(t, "GET", ts.URL+"/api/adventures/imported", ""); resp.StatusCode != 404 {
		t.Errorf("deleted adventure should be gone, got %d", resp.StatusCode)
	}
	// Deleting a missing adventure → 404.
	if resp, _ := doJSON(t, "DELETE", ts.URL+"/api/adventures/nope", ""); resp.StatusCode != 404 {
		t.Errorf("delete missing = %d; want 404", resp.StatusCode)
	}
}

func TestConfigSecretsWriteOnly(t *testing.T) {
	store, err := storage.NewWithPath(t.TempDir())
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	svc := appservice.New(store, domain.DefaultConfig(), nil)
	var rebuilt int
	ts := httptest.NewServer(New(svc, "").OnConfigSaved(func(*domain.Config) { rebuilt++ }).Handler())
	t.Cleanup(ts.Close)

	// Set a provider + a secret.
	if resp, _ := doJSON(t, "PUT", ts.URL+"/api/config", `{"provider":"openai","telegram_token":"tok-123"}`); resp.StatusCode != 200 {
		t.Fatalf("put config = %d", resp.StatusCode)
	}
	if svc.Config().TelegramToken != "tok-123" {
		t.Fatalf("token not saved: %q", svc.Config().TelegramToken)
	}
	// GET must NOT leak the secret.
	if _, got := doJSON(t, "GET", ts.URL+"/api/config", ""); got["telegram_token"] != nil && got["telegram_token"] != "" {
		t.Errorf("GET leaked telegram_token: %v", got["telegram_token"])
	}
	// PUT with an empty secret keeps the stored one (write-only), while other
	// fields still update.
	if resp, _ := doJSON(t, "PUT", ts.URL+"/api/config", `{"model":"m2","telegram_token":""}`); resp.StatusCode != 200 {
		t.Fatalf("put config 2 = %d", resp.StatusCode)
	}
	if svc.Config().TelegramToken != "tok-123" {
		t.Errorf("empty secret should preserve the stored token, got %q", svc.Config().TelegramToken)
	}
	if svc.Config().Model != "m2" {
		t.Errorf("model not updated: %q", svc.Config().Model)
	}
	// A non-empty secret replaces it.
	if resp, _ := doJSON(t, "PUT", ts.URL+"/api/config", `{"telegram_token":"tok-456"}`); resp.StatusCode != 200 {
		t.Fatalf("put config 3 = %d", resp.StatusCode)
	}
	if svc.Config().TelegramToken != "tok-456" {
		t.Errorf("secret not replaced: %q", svc.Config().TelegramToken)
	}
	if rebuilt < 3 {
		t.Errorf("onConfigSaved should fire on each save, got %d", rebuilt)
	}
}

func TestAdventureEditorEndpoints(t *testing.T) {
	ts := newTestServer(t, "")

	// Load, rename, save, reload.
	_, adv := doJSON(t, "GET", ts.URL+"/api/adventures/crypt", "")
	adv["title"] = "Renamed Crypt"
	b, _ := json.Marshal(adv)
	if resp, _ := doJSON(t, "PUT", ts.URL+"/api/adventures/crypt", string(b)); resp.StatusCode != 200 {
		t.Fatalf("save adventure = %d", resp.StatusCode)
	}
	if _, got := doJSON(t, "GET", ts.URL+"/api/adventures/crypt", ""); got["title"] != "Renamed Crypt" {
		t.Errorf("title not saved: %v", got["title"])
	}

	// Empty title is rejected.
	if resp, _ := doJSON(t, "PUT", ts.URL+"/api/adventures/crypt", `{"id":"crypt","title":""}`); resp.StatusCode != 400 {
		t.Errorf("empty title = %d; want 400", resp.StatusCode)
	}

	// Validate: the round-tripped module is valid; a dangling npc reference is not.
	if _, v := doJSON(t, "POST", ts.URL+"/api/adventures/crypt/validate", string(b)); v["errors"] == nil {
		t.Error("validate should return an errors array")
	} else if errs, _ := v["errors"].([]any); len(errs) != 0 {
		t.Errorf("valid module should have no errors, got %v", errs)
	}
	broken := `{"schema_version":"1.0","id":"crypt","title":"X","zones":[{"id":"z1","name":"Z","rooms":[{"id":"r1","name":"R","npc_ids":["ghost"]}]}]}`
	if _, v := doJSON(t, "POST", ts.URL+"/api/adventures/crypt/validate", broken); func() bool {
		errs, _ := v["errors"].([]any)
		return len(errs) == 0
	}() {
		t.Error("a dangling npc reference should produce a validation error")
	}

	// Export streams a gzip module.
	er, _ := http.Get(ts.URL + "/api/adventures/crypt/export")
	if er.StatusCode != 200 || er.Header.Get("Content-Type") != "application/gzip" {
		t.Fatalf("export = %d / %q", er.StatusCode, er.Header.Get("Content-Type"))
	}
	eb, _ := io.ReadAll(er.Body)
	er.Body.Close()
	if len(eb) == 0 {
		t.Error("export body should not be empty")
	}
	// Exporting a missing adventure is a 404 (not a misclassified error).
	if nr, _ := http.Get(ts.URL + "/api/adventures/nope/export"); nr.StatusCode != 404 {
		t.Errorf("export missing = %d; want 404", nr.StatusCode)
	}

	// DM book: markdown and PDF.
	dr, _ := http.Get(ts.URL + "/api/adventures/crypt/dmbook")
	db, _ := io.ReadAll(dr.Body)
	dr.Body.Close()
	if dr.StatusCode != 200 || len(db) == 0 {
		t.Fatalf("dmbook md = %d / %d bytes", dr.StatusCode, len(db))
	}
	pr, _ := http.Get(ts.URL + "/api/adventures/crypt/dmbook?format=pdf")
	pb, _ := io.ReadAll(pr.Body)
	pr.Body.Close()
	if pr.StatusCode != 200 || pr.Header.Get("Content-Type") != "application/pdf" || len(pb) == 0 {
		t.Errorf("dmbook pdf = %d / %q / %d bytes", pr.StatusCode, pr.Header.Get("Content-Type"), len(pb))
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

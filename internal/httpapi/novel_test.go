package httpapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The novel-text endpoints: 404 for an unknown session, load/save round-trip
// with optimistic concurrency (409 on a stale base version), and download of the
// saved prose.
func TestNovelTextEndpoints(t *testing.T) {
	ts := newTestServer(t, "")

	// Unknown session → 404 (and no orphan file is created).
	if resp, _ := doJSON(t, "GET", ts.URL+"/api/sessions/ghost/novel", ""); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET novel of unknown session = %d; want 404", resp.StatusCode)
	}

	// Open a session.
	_, out := doJSON(t, "POST", ts.URL+"/api/sessions", `{"adventure_id":"crypt"}`)
	name, _ := out["name"].(string)
	if name == "" {
		t.Fatalf("no session name in %v", out)
	}
	base := ts.URL + "/api/sessions/" + name + "/novel"

	// No novel yet.
	resp, got := doJSON(t, "GET", base, "")
	if resp.StatusCode != 200 || got["exists"] != false || got["text"] != "" {
		t.Fatalf("initial GET novel = %d %v", resp.StatusCode, got)
	}

	// Save over nothing (base_version "") → 200 + a version.
	resp, saved := doJSON(t, "PUT", base, `{"text":"# Book\n\nChapter one.","base_version":""}`)
	if resp.StatusCode != 200 {
		t.Fatalf("PUT novel = %d %v", resp.StatusCode, saved)
	}
	v1, _ := saved["version"].(string)
	if v1 == "" {
		t.Fatal("PUT should return a non-empty version")
	}

	// GET returns the saved text + matching version.
	_, got = doJSON(t, "GET", base, "")
	if got["text"] != "# Book\n\nChapter one." || got["version"] != v1 || got["exists"] != true {
		t.Fatalf("GET after save = %v", got)
	}

	// A stale base version is rejected with 409.
	if resp, _ := doJSON(t, "PUT", base, `{"text":"clobber","base_version":"stale"}`); resp.StatusCode != http.StatusConflict {
		t.Fatalf("stale PUT = %d; want 409", resp.StatusCode)
	}

	// The current version saves.
	if resp, _ := doJSON(t, "PUT", base, `{"text":"# Book\n\nEdited.","base_version":"`+v1+`"}`); resp.StatusCode != 200 {
		t.Fatalf("PUT with current version failed = %d", resp.StatusCode)
	}

	// Download the saved Markdown.
	body := getRaw(t, ts.URL+"/api/sessions/"+name+"/novel/download")
	if !strings.Contains(body, "Edited.") {
		t.Errorf("downloaded novel missing edit: %q", body)
	}
}

// A PUT that omits "text" must be rejected (not silently erase the novel), while
// an explicit empty string is a legitimate clear.
func TestNovelPutRequiresTextField(t *testing.T) {
	ts := newTestServer(t, "")
	_, out := doJSON(t, "POST", ts.URL+"/api/sessions", `{"adventure_id":"crypt"}`)
	name, _ := out["name"].(string)
	base := ts.URL + "/api/sessions/" + name + "/novel"

	// Seed a novel so we can detect an accidental erase.
	if resp, _ := doJSON(t, "PUT", base, `{"text":"important prose","base_version":""}`); resp.StatusCode != 200 {
		t.Fatalf("seed PUT = %d", resp.StatusCode)
	}
	v, _, _, _ := serverNovel(t, ts, name)

	// A body with only base_version (text omitted) → 400, and the novel is intact.
	if resp, _ := doJSON(t, "PUT", base, `{"base_version":"`+v+`"}`); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("PUT without text = %d; want 400", resp.StatusCode)
	}
	if _, got, _, _ := serverNovel(t, ts, name); got != "important prose" {
		t.Errorf("novel was erased by a text-less PUT: %q", got)
	}

	// An explicit empty string is allowed (deliberate clear).
	if resp, _ := doJSON(t, "PUT", base, `{"text":"","base_version":"`+v+`"}`); resp.StatusCode != 200 {
		t.Errorf("PUT with explicit empty text = %d; want 200", resp.StatusCode)
	}
}

// serverNovel fetches (version, text, exists) for a session's novel.
func serverNovel(t *testing.T, ts *httptest.Server, name string) (version, text string, exists bool, ok bool) {
	t.Helper()
	_, got := doJSON(t, "GET", ts.URL+"/api/sessions/"+name+"/novel", "")
	v, _ := got["version"].(string)
	tx, _ := got["text"].(string)
	ex, _ := got["exists"].(bool)
	return v, tx, ex, true
}

// The saved novel exports as a PDF (magic header), not only Markdown.
func TestNovelDownloadPDF(t *testing.T) {
	ts := newTestServer(t, "")
	_, out := doJSON(t, "POST", ts.URL+"/api/sessions", `{"adventure_id":"crypt"}`)
	name, _ := out["name"].(string)
	if resp, _ := doJSON(t, "PUT", ts.URL+"/api/sessions/"+name+"/novel", `{"text":"# Book\n\nChapter one.","base_version":""}`); resp.StatusCode != 200 {
		t.Fatalf("seed PUT = %d", resp.StatusCode)
	}
	body := getRaw(t, ts.URL+"/api/sessions/"+name+"/novel/download?format=pdf")
	if !strings.HasPrefix(body, "%PDF") {
		t.Errorf("PDF download missing %%PDF header, got %q", body[:min(8, len(body))])
	}
}

// Downloading a novel for a session that has none is a 404, not an empty file.
func TestNovelDownloadWithoutNovel(t *testing.T) {
	ts := newTestServer(t, "")
	_, out := doJSON(t, "POST", ts.URL+"/api/sessions", `{"adventure_id":"crypt"}`)
	name, _ := out["name"].(string)
	resp, err := http.Get(ts.URL + "/api/sessions/" + name + "/novel/download")
	if err != nil {
		t.Fatalf("GET download: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("download with no saved novel = %d; want 404", resp.StatusCode)
	}
}

func getRaw(t *testing.T, url string) string {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		t.Fatalf("GET %s = %d", url, resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

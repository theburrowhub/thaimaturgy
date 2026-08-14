package apiclient

import (
	"context"
	"strings"
	"testing"
)

// The novel text methods round-trip against a live server: load (empty), save
// with optimistic concurrency, reload, a stale-version save fails, and download
// returns the saved prose. Generate/adjust need a provider and are covered in
// appservice; here we exercise the client wiring for the non-AI endpoints.
func TestClientNovelTextRoundTrip(t *testing.T) {
	ctx := context.Background()
	c := liveServer(t, "")
	name, err := c.NewSession(ctx, "crypt")
	if err != nil {
		t.Fatalf("new session: %v", err)
	}

	text, version, exists, err := c.NovelText(ctx, name)
	if err != nil || exists || text != "" || version != "" {
		t.Fatalf("initial NovelText = (%q,%q,%v,%v)", text, version, exists, err)
	}

	v1, err := c.SaveNovelText(ctx, name, "# Book\n\nChapter one.", "")
	if err != nil || v1 == "" {
		t.Fatalf("SaveNovelText = (%q,%v)", v1, err)
	}

	text, version, exists, err = c.NovelText(ctx, name)
	if err != nil || !exists || text != "# Book\n\nChapter one." || version != v1 {
		t.Fatalf("NovelText after save = (%q,%q,%v,%v)", text, version, exists, err)
	}

	// A stale base version is rejected (409 → error).
	if _, err := c.SaveNovelText(ctx, name, "clobber", "stale"); err == nil {
		t.Error("SaveNovelText with a stale version should error")
	}

	// The download returns the saved Markdown.
	md, err := c.DownloadSessionNovel(ctx, name, "md")
	if err != nil {
		t.Fatalf("DownloadSessionNovel: %v", err)
	}
	if !strings.Contains(string(md), "Chapter one.") {
		t.Errorf("downloaded novel = %q", string(md))
	}
}

// An unknown session is a clean error, not a panic or empty success.
func TestClientNovelUnknownSession(t *testing.T) {
	ctx := context.Background()
	c := liveServer(t, "")
	if _, _, _, err := c.NovelText(ctx, "ghost"); err == nil {
		t.Error("NovelText of an unknown session should error")
	}
}

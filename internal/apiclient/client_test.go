package apiclient

import (
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
	"github.com/theburrowhub/thaimaturgy/internal/httpapi"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

// liveServer spins up a real httpapi server over a temp storage with one
// adventure, and returns a client pointed at it.
func liveServer(t *testing.T, token string) *Client {
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
	ts := httptest.NewServer(httpapi.New(svc, token).Handler())
	t.Cleanup(ts.Close)
	return New(ts.URL, token)
}

func TestClientRoundTrip(t *testing.T) {
	ctx := context.Background()
	c := liveServer(t, "")

	if err := c.Health(ctx); err != nil {
		t.Fatalf("health: %v", err)
	}
	advs, err := c.ListAdventures(ctx)
	if err != nil || len(advs) != 1 || advs[0].ID != "crypt" {
		t.Fatalf("adventures = %+v (%v)", advs, err)
	}
	name, err := c.NewSession(ctx, "crypt")
	if err != nil || name == "" {
		t.Fatalf("new session = %q (%v)", name, err)
	}
	st, err := c.Session(ctx, name)
	if err != nil || st.AdventureID != "crypt" {
		t.Fatalf("session = %+v (%v)", st, err)
	}
	res, err := c.Command(ctx, name, "/note the door is ajar")
	if err != nil || !res.Success {
		t.Fatalf("command = %+v (%v)", res, err)
	}
	if err := c.SaveSession(ctx, name); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := c.CloseSession(ctx, name); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Roster.
	id, err := c.SaveCharacter(ctx, domain.NewCharacter("Alice", "Elf", "Wizard"))
	if err != nil || id == "" {
		t.Fatalf("save char = %q (%v)", id, err)
	}
	chars, err := c.ListCharacters(ctx)
	if err != nil || len(chars) != 1 {
		t.Fatalf("list chars = %d (%v)", len(chars), err)
	}
	if err := c.DeleteCharacter(ctx, id); err != nil {
		t.Fatalf("delete char: %v", err)
	}

	// Config.
	cfg, err := c.Config(ctx)
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	cfg.Language = "es"
	if err := c.SaveConfig(ctx, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	if got, _ := c.Config(ctx); got.Language != "es" {
		t.Errorf("config not adopted: %q", got.Language)
	}
}

// TestClientConfigWithAuth: the server exposes the auto-detected credential as a
// read-only auth_source, ConfigWithAuth returns it alongside the config, and the
// plain Config still decodes (ignoring auth_source).
func TestClientConfigWithAuth(t *testing.T) {
	store, err := storage.NewWithPath(t.TempDir())
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	cfg := domain.DefaultConfig()
	cfg.AuthSource = "Claude Code login (Keychain)"
	svc := appservice.New(store, cfg, nil)
	ts := httptest.NewServer(httpapi.New(svc, "").Handler())
	t.Cleanup(ts.Close)
	c := New(ts.URL, "")

	got, src, err := c.ConfigWithAuth(context.Background())
	if err != nil {
		t.Fatalf("ConfigWithAuth: %v", err)
	}
	if src != "Claude Code login (Keychain)" {
		t.Errorf("auth source = %q; want the detected credential", src)
	}
	if got == nil || got.Provider != cfg.Provider {
		t.Errorf("config = %+v; want provider %q", got, cfg.Provider)
	}
	// The plain Config() path stays compatible (auth_source is ignored).
	if _, err := c.Config(context.Background()); err != nil {
		t.Errorf("Config: %v", err)
	}
}

func TestClientErrorSurface(t *testing.T) {
	ctx := context.Background()
	c := liveServer(t, "")
	// An unknown session → the server's error message should surface.
	if _, err := c.Session(ctx, "does-not-exist"); err == nil {
		t.Error("expected an error for an unknown session")
	}
}

// TestClientHandlesLargeResponse ensures a valid response bigger than the old
// 8 MiB read cap decodes fully (Heimdallm review) rather than being truncated.
func TestClientHandlesLargeResponse(t *testing.T) {
	// A bare server that returns a large roster payload (~10 MiB).
	big := make([]*domain.Character, 0, 100)
	pad := strings.Repeat("x", 100*1024) // 100 KiB notes each → ~10 MiB total
	for i := 0; i < 100; i++ {
		c := domain.NewCharacter("Char", "Human", "Fighter")
		c.Notes = pad
		big = append(big, c)
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/roster" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(big)
			return
		}
		w.WriteHeader(200)
	}))
	defer ts.Close()

	c := New(ts.URL, "")
	got, err := c.ListCharacters(context.Background())
	if err != nil {
		t.Fatalf("large ListCharacters: %v", err)
	}
	if len(got) != len(big) {
		t.Fatalf("got %d characters; want %d (response was truncated)", len(got), len(big))
	}
	if len(got[len(got)-1].Notes) != len(pad) {
		t.Error("last character's notes were truncated")
	}
}

func TestClientStreamEvents(t *testing.T) {
	c := liveServer(t, "")
	ctx := context.Background()
	name, err := c.NewSession(ctx, "crypt")
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	// Produce a timeline entry the stream should deliver.
	if _, err := c.Command(ctx, name, "/note a bell tolls"); err != nil {
		t.Fatalf("command: %v", err)
	}

	sctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	got := make(chan string, 8)
	go func() {
		_ = c.StreamEvents(sctx, name, func(e domain.LogEntry) {
			if strings.Contains(e.Message, "a bell tolls") {
				select {
				case got <- e.Message:
				default:
				}
			}
		})
	}()
	select {
	case <-got:
		// delivered
	case <-time.After(3 * time.Second):
		t.Error("StreamEvents did not deliver the log entry")
	}
}

func TestClientTokenAuth(t *testing.T) {
	ctx := context.Background()
	// Correct token works.
	c := liveServer(t, "s3cret")
	if err := c.Health(ctx); err != nil {
		t.Fatalf("health with token: %v", err)
	}
	if _, err := c.ListAdventures(ctx); err != nil {
		t.Fatalf("adventures with token: %v", err)
	}
	// Wrong token is rejected.
	bad := New(c.base, "wrong")
	if _, err := bad.ListAdventures(ctx); err == nil {
		t.Error("a wrong token should be rejected")
	}
	// A blank token is rejected by a token-protected server, but connects to one
	// that requires none — the contract the desktop Settings rely on (leave the
	// token field blank when the server has no token).
	if err := New(c.base, "").Health(ctx); err == nil {
		t.Error("a blank token should be rejected by a token-protected server")
	}
	if err := liveServer(t, "").Health(ctx); err != nil {
		t.Errorf("a blank token should connect to a token-less server: %v", err)
	}
	// An SSE ticket can be minted with the right token.
	if tk, err := c.SSETicket(ctx); err != nil || tk == "" {
		t.Errorf("sse ticket = %q (%v)", tk, err)
	}
}

// TestClientParty exercises the party endpoints the remote GUI uses to assemble
// and edit a party (#130): SetParty, Party (snapshot), and DefaultParty.
func TestClientParty(t *testing.T) {
	c := liveServer(t, "")
	ctx := context.Background()
	name, err := c.NewSession(ctx, "crypt")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if err := c.SetParty(ctx, name, []*domain.Character{domain.NewCharacter("Alice", "Elf", "Wizard")}); err != nil {
		t.Fatalf("SetParty: %v", err)
	}
	p, err := c.Party(ctx, name)
	if err != nil || len(p) != 1 || p[0].Name != "Alice" {
		t.Fatalf("Party = %+v (%v); want [Alice]", p, err)
	}
	if err := c.DefaultParty(ctx, name); err != nil {
		t.Fatalf("DefaultParty: %v", err)
	}
	if p, _ := c.Party(ctx, name); len(p) == 0 {
		t.Error("DefaultParty left the party empty")
	}
}

// TestClientUpdateCharacter exercises the sheet-edit path the remote GUI uses
// (#130): UpdateCharacter applies an edit and rejects a stale base (optimistic
// concurrency).
func TestClientUpdateCharacter(t *testing.T) {
	c := liveServer(t, "")
	ctx := context.Background()
	name, err := c.NewSession(ctx, "crypt")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if err := c.SetParty(ctx, name, []*domain.Character{domain.NewCharacter("Alice", "Elf", "Wizard")}); err != nil {
		t.Fatalf("SetParty: %v", err)
	}
	p, err := c.Party(ctx, name)
	if err != nil || len(p) != 1 {
		t.Fatalf("Party = %+v (%v)", p, err)
	}
	base := p[0] // the live (normalized) character = the correct base
	edited := base
	edited.Level = 3
	if err := c.UpdateCharacter(ctx, name, base.Name, &base, &edited); err != nil {
		t.Fatalf("UpdateCharacter: %v", err)
	}
	if p2, _ := c.Party(ctx, name); len(p2) != 1 || p2[0].Level != 3 {
		t.Fatalf("edit not applied: %+v", p2)
	}
	// A stale base (level 1) must be rejected now that the live level is 3.
	edited2 := edited
	edited2.Level = 5
	if err := c.UpdateCharacter(ctx, name, base.Name, &base, &edited2); err == nil {
		t.Error("expected a conflict when updating from a stale base")
	}
}

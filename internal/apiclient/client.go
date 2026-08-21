// Package apiclient is a typed Go client for the thAImaturgy HTTP API (issue #36,
// Phase B/D). It's the reusable "client" half of the client/server split: a
// remote desktop GUI, a CLI, or tests talk to a running server through this
// instead of the core, mirroring the operations of internal/appservice.
package apiclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

// Client talks to a thAImaturgy server. It is safe for concurrent use.
type Client struct {
	base  string
	token string
	hc    *http.Client
}

// New builds a client for a base URL (e.g. "http://127.0.0.1:8765"); token may be
// empty when the server requires none.
func New(baseURL, token string) *Client {
	return &Client{
		base:  strings.TrimRight(baseURL, "/"),
		token: strings.TrimSpace(token),
		hc:    &http.Client{Timeout: 0}, // no global timeout: an oracle turn is long
	}
}

// BaseURL returns the server's base URL (for display).
func (c *Client) BaseURL() string { return c.base }

// CommandResult mirrors the server's /command response.
type CommandResult struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	Response string `json:"response"`
	UIAction string `json:"ui_action"`
	UIArg    string `json:"ui_arg"`
}

// OracleResult mirrors the server's /oracle response.
type OracleResult struct {
	Answer     string `json:"answer"`
	TokensUsed int    `json:"tokens_used"`
	LatencyMs  int64  `json:"latency_ms"`
	Error      string `json:"error"`
}

// do performs a request, sending/expecting JSON, and decodes the 2xx body into
// out (when non-nil). A non-2xx response becomes an error carrying the server's
// {"error":...} message.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rdr)
	if err != nil {
		return err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		// Read a bounded slice of the error body just to surface the message.
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &e) == nil && e.Error != "" {
			return fmt.Errorf("%s (HTTP %d)", e.Error, resp.StatusCode)
		}
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if out != nil {
		// Decode the success body as a stream so a large-but-valid response (a big
		// session state or collection) is never truncated.
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}
	return nil
}

func enc(name string) string { return url.PathEscape(name) }

// --- Adventures & sessions ----------------------------------------------

func (c *Client) ListAdventures(ctx context.Context) ([]storage.AdventureInfo, error) {
	var out []storage.AdventureInfo
	err := c.do(ctx, "GET", "/api/adventures", nil, &out)
	return out, err
}

// ImportAdventure uploads a .tar.gz module to the server (multipart/form-data,
// file field "module") and returns the imported adventure's id and title. It
// sends the X-Thaim-CSRF header the server's multipart endpoint requires (that
// endpoint triggers no CORS preflight, so the header is its CSRF guard).
func (c *Client) ImportAdventure(ctx context.Context, path string) (id, title string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	// Stream the multipart body through a pipe so the archive is never fully
	// buffered in memory — a huge file can't exhaust the heap and crash the GUI
	// before the server applies its own upload limit. A write error (read failure,
	// etc.) is propagated to the reader side via CloseWithError so the request fails.
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		part, werr := mw.CreateFormFile("module", filepath.Base(path))
		if werr == nil {
			_, werr = io.Copy(part, f)
		}
		if werr == nil {
			werr = mw.Close()
		}
		_ = pw.CloseWithError(werr)
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/api/adventures/import", pr)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Thaim-CSRF", "1")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &e) == nil && e.Error != "" {
			return "", "", fmt.Errorf("%s (HTTP %d)", e.Error, resp.StatusCode)
		}
		return "", "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var out struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", fmt.Errorf("decoding response: %w", err)
	}
	return out.ID, out.Title, nil
}

func (c *Client) ListSessions(ctx context.Context) ([]storage.SessionInfo, error) {
	var out []storage.SessionInfo
	err := c.do(ctx, "GET", "/api/sessions", nil, &out)
	return out, err
}

// NewSession creates a session for an adventure and returns its name.
func (c *Client) NewSession(ctx context.Context, adventureID string) (string, error) {
	var out struct {
		Name string `json:"name"`
	}
	err := c.do(ctx, "POST", "/api/sessions", map[string]string{"adventure_id": adventureID}, &out)
	return out.Name, err
}

// Session fetches (resuming if needed) a session's full state.
func (c *Client) Session(ctx context.Context, name string) (*domain.SessionState, error) {
	var st domain.SessionState
	if err := c.do(ctx, "GET", "/api/sessions/"+enc(name), nil, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

func (c *Client) SaveSession(ctx context.Context, name string) error {
	return c.do(ctx, "POST", "/api/sessions/"+enc(name)+"/save", nil, nil)
}
func (c *Client) CloseSession(ctx context.Context, name string) error {
	return c.do(ctx, "POST", "/api/sessions/"+enc(name)+"/close", nil, nil)
}
func (c *Client) DeleteSession(ctx context.Context, name string) error {
	return c.do(ctx, "DELETE", "/api/sessions/"+enc(name), nil, nil)
}
func (c *Client) RenameSession(ctx context.Context, name, newName string) error {
	return c.do(ctx, "POST", "/api/sessions/"+enc(name)+"/rename", map[string]string{"new_name": newName}, nil)
}

// Command runs a shared engine command (the parity path) against a session.
func (c *Client) Command(ctx context.Context, name, input string) (CommandResult, error) {
	var out CommandResult
	err := c.do(ctx, "POST", "/api/sessions/"+enc(name)+"/command", map[string]string{"input": input}, &out)
	return out, err
}

// Oracle runs one oracle/DM turn against a session.
func (c *Client) Oracle(ctx context.Context, name, input string) (OracleResult, error) {
	var out OracleResult
	err := c.do(ctx, "POST", "/api/sessions/"+enc(name)+"/oracle", map[string]string{"input": input}, &out)
	return out, err
}

// --- Party ---------------------------------------------------------------

// Party returns a snapshot of an open session's party.
func (c *Client) Party(ctx context.Context, name string) ([]domain.Character, error) {
	var out []domain.Character
	err := c.do(ctx, "GET", "/api/sessions/"+enc(name)+"/party", nil, &out)
	return out, err
}

// SetParty replaces an open session's party.
func (c *Client) SetParty(ctx context.Context, name string, party []*domain.Character) error {
	return c.do(ctx, "PUT", "/api/sessions/"+enc(name)+"/party", party, nil)
}

// DefaultParty sets an open session's party to the built-in sample party.
func (c *Client) DefaultParty(ctx context.Context, name string) error {
	return c.do(ctx, "POST", "/api/sessions/"+enc(name)+"/party/default", nil, nil)
}

// --- Roster & config -----------------------------------------------------

func (c *Client) ListCharacters(ctx context.Context) ([]*domain.Character, error) {
	var out []*domain.Character
	err := c.do(ctx, "GET", "/api/roster", nil, &out)
	return out, err
}

// SaveCharacter persists a roster character and returns its id.
func (c *Client) SaveCharacter(ctx context.Context, ch *domain.Character) (string, error) {
	var out struct {
		ID string `json:"id"`
	}
	err := c.do(ctx, "POST", "/api/roster", ch, &out)
	return out.ID, err
}

func (c *Client) DeleteCharacter(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/api/roster/"+enc(id), nil, nil)
}

// UpdateCharacter applies an edited party character over the one named charName,
// with optimistic concurrency: base is the snapshot the edit started from, and
// the server rejects the write (surfaced as an error) if the live character has
// changed since. edited should carry every field (start from a copy of base) so
// untouched fields are preserved.
func (c *Client) UpdateCharacter(ctx context.Context, name, charName string, base, edited *domain.Character) error {
	body := map[string]*domain.Character{"base": base, "edited": edited}
	return c.do(ctx, "PUT", "/api/sessions/"+enc(name)+"/characters/"+enc(charName), body, nil)
}

func (c *Client) Config(ctx context.Context) (*domain.Config, error) {
	var cfg domain.Config
	if err := c.do(ctx, "GET", "/api/config", nil, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ConfigWithAuth is like Config but also returns the server's read-only
// auth_source (the auto-detected credential description, e.g. "Claude Code login
// (Keychain)" or "Gemini CLI"), which the plain domain.Config drops (json:"-").
func (c *Client) ConfigWithAuth(ctx context.Context) (*domain.Config, string, error) {
	var out struct {
		domain.Config
		AuthSource string `json:"auth_source"`
	}
	if err := c.do(ctx, "GET", "/api/config", nil, &out); err != nil {
		return nil, "", err
	}
	cfg := out.Config
	return &cfg, out.AuthSource, nil
}
func (c *Client) SaveConfig(ctx context.Context, cfg *domain.Config) error {
	return c.do(ctx, "PUT", "/api/config", cfg, nil)
}

// SSETicket mints a short-lived ticket for opening the events stream (needed only
// when the server requires a token).
func (c *Client) SSETicket(ctx context.Context) (string, error) {
	var out struct {
		Ticket string `json:"ticket"`
	}
	err := c.do(ctx, "POST", "/api/sse-ticket", nil, &out)
	return out.Ticket, err
}

// StreamEvents opens the session's SSE stream and calls onLog for each timeline
// entry until ctx is cancelled or the stream ends. When the server requires a
// token it first mints an SSE ticket. Blocks; run it in a goroutine.
func (c *Client) StreamEvents(ctx context.Context, name string, onLog func(domain.LogEntry)) error {
	u := c.base + "/api/sessions/" + enc(name) + "/events"
	if c.token != "" {
		ticket, err := c.SSETicket(ctx)
		if err != nil {
			return err
		}
		u += "?ticket=" + url.QueryEscape(ticket)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return err
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		return fmt.Errorf("events stream: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	var event string
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(line[len("event:"):])
		case strings.HasPrefix(line, "data:"):
			if event == "log" {
				var e domain.LogEntry
				if json.Unmarshal([]byte(strings.TrimSpace(line[len("data:"):])), &e) == nil {
					onLog(e)
				}
			}
		case line == "":
			event = "" // end of one SSE event
		}
	}
	return sc.Err()
}

// Health pings the server, returning an error if it isn't reachable/healthy.
func (c *Client) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.do(ctx, "GET", "/api/health", nil, nil)
}

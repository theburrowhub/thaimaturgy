package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/appservice"
	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

// authServer builds a server over a temp store with one player user, returning
// the httptest server and the store (so tests can seed/inspect users).
func authServer(t *testing.T, masterToken string) (*httptest.Server, *storage.Storage) {
	t.Helper()
	store, err := storage.NewWithPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateUser("aria", domain.RolePlayer, "pw123"); err != nil {
		t.Fatal(err)
	}
	svc := appservice.New(store, domain.DefaultConfig(), nil)
	ts := httptest.NewServer(New(svc, masterToken).Handler())
	t.Cleanup(ts.Close)
	return ts, store
}

func req(t *testing.T, method, url, bearer, body string) (*http.Response, map[string]any) {
	t.Helper()
	r, _ := http.NewRequest(method, url, bytes.NewReader([]byte(body)))
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		r.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	resp.Body.Close()
	return resp, out
}

func TestLoginWhoamiLogout(t *testing.T) {
	ts, _ := authServer(t, "master-tok")

	// Wrong password is rejected.
	if resp, _ := req(t, "POST", ts.URL+"/api/login", "", `{"username":"aria","password":"nope"}`); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad-password login = %d; want 401", resp.StatusCode)
	}
	// Unknown user is rejected the same way (no user enumeration).
	if resp, _ := req(t, "POST", ts.URL+"/api/login", "", `{"username":"ghost","password":"x"}`); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unknown-user login = %d; want 401", resp.StatusCode)
	}

	// Correct login yields a session token.
	resp, out := req(t, "POST", ts.URL+"/api/login", "", `{"username":"aria","password":"pw123"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login = %d; want 200", resp.StatusCode)
	}
	tok, _ := out["token"].(string)
	if tok == "" {
		t.Fatal("login returned no token")
	}
	if u, _ := out["user"].(map[string]any); u["role"] != "player" || u["password_hash"] != nil {
		t.Errorf("login user payload wrong or leaked hash: %v", u)
	}

	// whoami with the session token resolves to aria.
	resp, out = req(t, "GET", ts.URL+"/api/whoami", tok, "")
	if resp.StatusCode != http.StatusOK || out["username"] != "aria" || out["role"] != "player" {
		t.Fatalf("whoami(session) = %d %v", resp.StatusCode, out)
	}

	// whoami with the master token resolves to the break-glass admin.
	_, out = req(t, "GET", ts.URL+"/api/whoami", "master-tok", "")
	if out["role"] != "admin" {
		t.Errorf("whoami(master) role = %v; want admin", out["role"])
	}

	// whoami with no/invalid token is rejected.
	if resp, _ := req(t, "GET", ts.URL+"/api/whoami", "", ""); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("whoami(anon) = %d; want 401", resp.StatusCode)
	}
	if resp, _ := req(t, "GET", ts.URL+"/api/whoami", "wrong", ""); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("whoami(bad token) = %d; want 401", resp.StatusCode)
	}

	// Logout invalidates the session token.
	if resp, _ := req(t, "POST", ts.URL+"/api/logout", tok, ""); resp.StatusCode != http.StatusOK {
		t.Errorf("logout = %d; want 200", resp.StatusCode)
	}
	if resp, _ := req(t, "GET", ts.URL+"/api/whoami", tok, ""); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("whoami after logout = %d; want 401", resp.StatusCode)
	}
}

func TestTokenlessLoopbackIsAdmin(t *testing.T) {
	// No master token configured → loopback caller is treated as admin (unchanged
	// local-dev behavior), so whoami succeeds as admin over the loopback httptest host.
	ts, _ := authServer(t, "")
	resp, out := req(t, "GET", ts.URL+"/api/whoami", "", "")
	if resp.StatusCode != http.StatusOK || out["role"] != "admin" {
		t.Fatalf("token-less loopback whoami = %d %v; want 200 admin", resp.StatusCode, out)
	}
}

func TestLoginThrottling(t *testing.T) {
	ts, _ := authServer(t, "")

	// loginMaxFails wrong attempts each get 401; the next is locked out with 429.
	for i := 0; i < loginMaxFails; i++ {
		if resp, _ := req(t, "POST", ts.URL+"/api/login", "", `{"username":"aria","password":"nope"}`); resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d = %d; want 401", i+1, resp.StatusCode)
		}
	}
	// Even the CORRECT password is refused while locked out (429).
	resp, _ := req(t, "POST", ts.URL+"/api/login", "", `{"username":"aria","password":"pw123"}`)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("after %d fails, login = %d; want 429", loginMaxFails, resp.StatusCode)
	}
	if resp.Header.Get("Retry-After") == "" {
		t.Error("a 429 should carry a Retry-After header")
	}
}

func TestSessionPerUserCap(t *testing.T) {
	// A master token is set so an evicted/invalid session token is rejected (401)
	// rather than falling through to token-less loopback admin.
	ts, _ := authServer(t, "master-tok")

	var tokens []string
	for i := 0; i < maxSessionsPerUser+2; i++ {
		_, out := req(t, "POST", ts.URL+"/api/login", "", `{"username":"aria","password":"pw123"}`)
		tok, _ := out["token"].(string)
		if tok == "" {
			t.Fatalf("login %d returned no token", i)
		}
		tokens = append(tokens, tok)
	}
	// The two oldest sessions were evicted past the per-user cap.
	for i := 0; i < 2; i++ {
		if resp, _ := req(t, "GET", ts.URL+"/api/whoami", tokens[i], ""); resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("evicted token %d whoami = %d; want 401", i, resp.StatusCode)
		}
	}
	// The newest is still valid.
	if resp, _ := req(t, "GET", ts.URL+"/api/whoami", tokens[len(tokens)-1], ""); resp.StatusCode != http.StatusOK {
		t.Errorf("newest token whoami = %d; want 200", resp.StatusCode)
	}
}

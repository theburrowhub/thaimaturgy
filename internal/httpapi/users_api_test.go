package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"
)

func listUsersAs(t *testing.T, url, bearer string) (int, []map[string]any) {
	t.Helper()
	r, _ := http.NewRequest("GET", url, nil)
	if bearer != "" {
		r.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var arr []map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&arr)
	return resp.StatusCode, arr
}

func TestAdminUserAPI(t *testing.T) {
	// authServer seeds player "aria"; the master token is the break-glass admin.
	ts, _ := authServer(t, "master-tok")
	const admin = "master-tok"

	// A player session token is forbidden from the admin endpoints.
	_, out := req(t, "POST", ts.URL+"/api/login", "", `{"username":"aria","password":"pw123"}`)
	ariaTok, _ := out["token"].(string)
	if code, _ := listUsersAs(t, ts.URL+"/api/users", ariaTok); code != http.StatusForbidden {
		t.Errorf("player GET /api/users = %d; want 403", code)
	}
	if resp, _ := req(t, "POST", ts.URL+"/api/users", ariaTok, `{"username":"x","role":"player"}`); resp.StatusCode != http.StatusForbidden {
		t.Errorf("player POST /api/users = %d; want 403", resp.StatusCode)
	}
	// Anonymous is rejected before reaching the handler.
	if code, _ := listUsersAs(t, ts.URL+"/api/users", ""); code != http.StatusUnauthorized {
		t.Errorf("anon GET /api/users = %d; want 401", code)
	}

	// Admin creates an admin user.
	resp, borin := req(t, "POST", ts.URL+"/api/users", admin, `{"username":"borin","role":"admin","password":"pw"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create user = %d; want 201", resp.StatusCode)
	}
	borinID, _ := borin["id"].(string)
	if borinID == "" || borin["password_hash"] != nil {
		t.Fatalf("create returned bad payload: %v", borin)
	}
	// Duplicate username → 409.
	if resp, _ := req(t, "POST", ts.URL+"/api/users", admin, `{"username":"Borin","role":"player","password":"y"}`); resp.StatusCode != http.StatusConflict {
		t.Errorf("duplicate create = %d; want 409", resp.StatusCode)
	}
	// List (admin) includes aria + borin.
	if code, arr := listUsersAs(t, ts.URL+"/api/users", admin); code != 200 || len(arr) != 2 {
		t.Errorf("admin list = %d, %d users; want 200, 2", code, len(arr))
	}

	// Assign characters via PUT.
	resp, upd := req(t, "PUT", ts.URL+"/api/users/"+borinID, admin, `{"character_ids":["borin-abc123"]}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update = %d; want 200", resp.StatusCode)
	}
	if ids, _ := upd["character_ids"].([]any); len(ids) != 1 || ids[0] != "borin-abc123" {
		t.Errorf("character_ids not applied: %v", upd["character_ids"])
	}

	// Not found.
	if resp, _ := req(t, "GET", ts.URL+"/api/users/nope-000000", admin, ""); resp.StatusCode != http.StatusNotFound {
		t.Errorf("get missing = %d; want 404", resp.StatusCode)
	}

	// Last-admin guard: borin is the only STORED admin, so demote/delete are blocked.
	if resp, _ := req(t, "PUT", ts.URL+"/api/users/"+borinID, admin, `{"role":"player"}`); resp.StatusCode != http.StatusConflict {
		t.Errorf("demote last admin = %d; want 409", resp.StatusCode)
	}
	if resp, _ := req(t, "DELETE", ts.URL+"/api/users/"+borinID, admin, ""); resp.StatusCode != http.StatusConflict {
		t.Errorf("delete last admin = %d; want 409", resp.StatusCode)
	}

	// With a second admin present, deleting borin succeeds.
	req(t, "POST", ts.URL+"/api/users", admin, `{"username":"gandalf","role":"admin","password":"pw"}`)
	if resp, _ := req(t, "DELETE", ts.URL+"/api/users/"+borinID, admin, ""); resp.StatusCode != http.StatusOK {
		t.Errorf("delete admin (with another present) = %d; want 200", resp.StatusCode)
	}
}

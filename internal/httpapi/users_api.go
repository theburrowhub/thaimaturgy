package httpapi

import (
	"errors"
	"net/http"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

// Admin user-management API (#151). Every handler here requires an ADMIN caller
// (resolved by withAuth into the request context). A last-admin guard prevents an
// admin from deleting or demoting the only remaining admin and locking everyone
// out.

// requireAdmin returns the authenticated user if it is an admin; otherwise it
// writes 401/403 and returns ok=false.
func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) (*domain.User, bool) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpError(w, http.StatusUnauthorized, "not authenticated")
		return nil, false
	}
	if !u.IsAdmin() {
		httpError(w, http.StatusForbidden, "admin access required")
		return nil, false
	}
	return u, true
}

// countAdmins returns how many registered users have the admin role.
func (s *Server) countAdmins() (int, error) {
	users, err := s.svc.ListUsers()
	if err != nil {
		return 0, err
	}
	n := 0
	for _, u := range users {
		if u.IsAdmin() {
			n++
		}
	}
	return n, nil
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	users, err := s.svc.ListUsers()
	if err != nil {
		httpError(w, http.StatusInternalServerError, "could not list users")
		return
	}
	out := make([]*domain.User, len(users))
	for i, u := range users {
		out[i] = u.Sanitized()
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var body struct {
		Username string          `json:"username"`
		Password string          `json:"password"`
		Role     domain.UserRole `json:"role"`
	}
	if !readJSONLimited(w, r, &body, maxBodyBytes) {
		return
	}
	if body.Role == "" {
		body.Role = domain.RolePlayer
	}
	u, err := s.svc.CreateUser(body.Username, body.Role, body.Password)
	if err != nil {
		writeUserErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, u.Sanitized())
}

func (s *Server) getUser(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	u, err := s.svc.LoadUser(r.PathValue("id"))
	if err != nil {
		writeUserErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u.Sanitized())
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id := r.PathValue("id")
	u, err := s.svc.LoadUser(id)
	if err != nil {
		writeUserErr(w, err)
		return
	}
	// All fields optional: only those present in the body are changed.
	var body struct {
		Username     *string          `json:"username"`
		Role         *domain.UserRole `json:"role"`
		Password     *string          `json:"password"`
		CharacterIDs *[]string        `json:"character_ids"`
	}
	if !readJSONLimited(w, r, &body, maxBodyBytes) {
		return
	}
	// Guard against demoting the last admin (which would lock everyone out).
	if body.Role != nil && u.IsAdmin() && *body.Role != domain.RoleAdmin {
		if n, err := s.countAdmins(); err != nil {
			httpError(w, http.StatusInternalServerError, "could not verify admins")
			return
		} else if n <= 1 {
			httpError(w, http.StatusConflict, "cannot demote the last admin")
			return
		}
	}
	if body.Username != nil {
		u.Username = *body.Username
	}
	if body.Role != nil {
		u.Role = *body.Role
	}
	if body.CharacterIDs != nil {
		u.CharacterIDs = *body.CharacterIDs
	}
	if body.Password != nil && *body.Password != "" {
		if err := u.SetPassword(*body.Password); err != nil {
			httpError(w, http.StatusInternalServerError, "could not set password")
			return
		}
	}
	if err := s.svc.SaveUser(u); err != nil {
		writeUserErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u.Sanitized())
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id := r.PathValue("id")
	u, err := s.svc.LoadUser(id)
	if err != nil {
		writeUserErr(w, err)
		return
	}
	// Guard against deleting the last admin.
	if u.IsAdmin() {
		if n, err := s.countAdmins(); err != nil {
			httpError(w, http.StatusInternalServerError, "could not verify admins")
			return
		} else if n <= 1 {
			httpError(w, http.StatusConflict, "cannot delete the last admin")
			return
		}
	}
	if err := s.svc.DeleteUser(id); err != nil {
		httpError(w, http.StatusInternalServerError, "could not delete user")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// writeUserErr maps user-store errors to HTTP status codes.
func writeUserErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, storage.ErrUsernameTaken):
		httpError(w, http.StatusConflict, "username already taken")
	case errors.Is(err, storage.ErrUserNotFound):
		httpError(w, http.StatusNotFound, "user not found")
	default:
		httpError(w, http.StatusBadRequest, err.Error())
	}
}

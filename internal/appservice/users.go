package appservice

import "github.com/theburrowhub/thaimaturgy/internal/domain"

// User management (#151). These are thin pass-throughs to the storage user store;
// the HTTP layer adds authentication (sessions) and authorization on top.

func (s *Service) CreateUser(username string, role domain.UserRole, password string) (*domain.User, error) {
	return s.store.CreateUser(username, role, password)
}

func (s *Service) ListUsers() ([]*domain.User, error) { return s.store.ListUsers() }

func (s *Service) LoadUser(id string) (*domain.User, error) { return s.store.LoadUser(id) }

func (s *Service) SaveUser(u *domain.User) error { return s.store.SaveUser(u) }

func (s *Service) DeleteUser(id string) error { return s.store.DeleteUser(id) }

func (s *Service) UserCount() (int, error) { return s.store.UserCount() }

// Authenticate verifies a username/password pair, returning the user on success.
// It is deliberately coarse (no distinction between unknown user and bad
// password) so it doesn't reveal which usernames exist.
func (s *Service) Authenticate(username, password string) (*domain.User, bool) {
	u, err := s.store.UserByUsername(username)
	if err != nil {
		return nil, false
	}
	if !u.CheckPassword(password) {
		return nil, false
	}
	return u, true
}

package appservice

import (
	"errors"

	"golang.org/x/crypto/bcrypt"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

// ErrInvalidCredentials is returned by Authenticate for either an unknown user or
// a wrong password — the two are deliberately indistinguishable to the caller so
// the API can't be used to enumerate usernames. Any OTHER error from Authenticate
// is a storage/internal failure and must NOT be treated as an auth failure.
var ErrInvalidCredentials = errors.New("invalid username or password")

// dummyHash is a valid bcrypt hash (of a fixed string) compared against on the
// unknown-user path so a missing account costs roughly the same as a wrong
// password, closing the timing side-channel that would reveal which usernames
// exist. Generated once at load with the same cost as real passwords.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("thaimaturgy-timing-equalizer"), bcrypt.DefaultCost)

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

// Authenticate verifies a username/password pair. It returns the user on success;
// ErrInvalidCredentials for an unknown user OR a wrong password (indistinguishable,
// and equal-time — see dummyHash); or the underlying error for a storage/internal
// failure (which the caller must surface as a server error, NOT a 401, so a valid
// user isn't locked out by an outage).
func (s *Service) Authenticate(username, password string) (*domain.User, error) {
	u, err := s.store.UserByUsername(username)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			// Spend comparable time to a real check so timing can't reveal the miss.
			_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
			return nil, ErrInvalidCredentials
		}
		return nil, err // storage failure — not an authentication failure
	}
	if !u.CheckPassword(password) {
		return nil, ErrInvalidCredentials
	}
	return u, nil
}

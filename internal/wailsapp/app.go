package wailsapp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/engine"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

// App is the Go backend exposed to the Wails frontend.
type App struct {
	ctx     context.Context
	store   *storage.Storage
	config  *domain.Config
	session *domain.Session
}

type LibraryPayload struct {
	Adventures []storage.AdventureInfo `json:"adventures"`
	Sessions   []storage.SessionInfo   `json:"sessions"`
}

type SessionPayload struct {
	State       *domain.SessionState `json:"state"`
	Adventure   *domain.Adventure    `json:"adventure"`
	CurrentRoom *domain.Room         `json:"current_room,omitempty"`
	CurrentZone *domain.Zone         `json:"current_zone,omitempty"`
}

type SubmitResult struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Session *SessionPayload `json:"session,omitempty"`
}

func New() (*App, error) {
	store, err := storage.New()
	if err != nil {
		return nil, err
	}
	return NewWithStorage(store)
}

func NewWithStorage(store *storage.Storage) (*App, error) {
	config, err := store.LoadConfig()
	if err != nil {
		return nil, err
	}
	return &App{store: store, config: config}, nil
}

func (a *App) Startup(ctx context.Context) { a.ctx = ctx }

func (a *App) GetLibrary() (*LibraryPayload, error) {
	adventures, err := a.store.ListAdventures()
	if err != nil {
		return nil, err
	}
	sessions, err := a.store.ListSessions()
	if err != nil {
		return nil, err
	}
	return &LibraryPayload{Adventures: adventures, Sessions: sessions}, nil
}

func (a *App) GetAdventure(id string) (*domain.Adventure, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("adventure id is required")
	}
	return a.store.LoadAdventure(id)
}

func (a *App) StartSession(adventureID string) (*SessionPayload, error) {
	adv, err := a.GetAdventure(adventureID)
	if err != nil {
		return nil, err
	}
	name := fmt.Sprintf("%s-%s", adv.ID, time.Now().Format("20060102-150405"))
	state := domain.NewSessionState(name, adv)
	if err := a.store.SaveSession(state); err != nil {
		return nil, err
	}
	a.session = domain.NewSession(state, adv, a.config)
	return a.payload()
}

func (a *App) LoadSession(name string) (*SessionPayload, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("session name is required")
	}
	state, err := a.store.LoadSession(name)
	if err != nil {
		return nil, err
	}
	adv, err := a.store.LoadAdventure(state.AdventureID)
	if err != nil {
		return nil, err
	}
	a.session = domain.NewSession(state, adv, a.config)
	return a.payload()
}

func (a *App) MoveParty(roomID string) (*SessionPayload, error) {
	if a.session == nil {
		return nil, fmt.Errorf("no active session")
	}
	zone, room := findRoom(a.session.Adventure, roomID)
	if room == nil || zone == nil {
		return nil, fmt.Errorf("room not found: %s", roomID)
	}
	a.session.State.SetLocation(zone.ID, room.ID, room.Name)
	a.session.MarkModified()
	if err := a.store.SaveSession(a.session.State); err != nil {
		return nil, err
	}
	return a.payload()
}

func (a *App) Submit(input string) (*SubmitResult, error) {
	if a.session == nil {
		return nil, fmt.Errorf("no active session")
	}
	cmd := engine.ParseCommand(input)
	if cmd == nil {
		return &SubmitResult{Success: false, Message: "Enter a command or note."}, nil
	}
	if cmd.Type == engine.CmdOracle {
		a.session.State.AddNote(strings.TrimSpace(input))
		a.session.MarkModified()
		if err := a.store.SaveSession(a.session.State); err != nil {
			return nil, err
		}
		p, _ := a.payload()
		return &SubmitResult{Success: true, Message: "Noted. Full AI oracle answers will be wired in the next Wails slice.", Session: p}, nil
	}
	result := engine.NewCommandHandler(a.session).Execute(cmd)
	a.session.MarkModified()
	if err := a.store.SaveSession(a.session.State); err != nil {
		return nil, err
	}
	p, _ := a.payload()
	return &SubmitResult{Success: result.Success, Message: result.Message, Session: p}, nil
}

func (a *App) payload() (*SessionPayload, error) {
	if a.session == nil {
		return nil, fmt.Errorf("no active session")
	}
	zone, room := findRoom(a.session.Adventure, a.session.State.CurrentRoom)
	return &SessionPayload{State: a.session.State, Adventure: a.session.Adventure, CurrentZone: zone, CurrentRoom: room}, nil
}

func findRoom(adv *domain.Adventure, roomID string) (*domain.Zone, *domain.Room) {
	if adv == nil {
		return nil, nil
	}
	for zi := range adv.Zones {
		for ri := range adv.Zones[zi].Rooms {
			room := &adv.Zones[zi].Rooms[ri]
			if room.ID == roomID {
				return &adv.Zones[zi], room
			}
		}
	}
	return nil, nil
}

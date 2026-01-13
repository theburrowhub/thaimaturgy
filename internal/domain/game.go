package domain

import (
	"time"
)

type GameState struct {
	SaveName     string        `json:"save_name"`
	Character    *Character    `json:"character"`
	World        *WorldState   `json:"world"`
	Conversation *Conversation `json:"conversation"`
	EventLog     *EventLog     `json:"event_log"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	PlayTime     time.Duration `json:"play_time"`
}

func NewGameState(saveName string, character *Character, setting string) *GameState {
	now := time.Now()
	return &GameState{
		SaveName:     saveName,
		Character:    character,
		World:        NewWorldState(setting),
		Conversation: NewConversation(50),
		EventLog:     NewEventLog(100),
		CreatedAt:    now,
		UpdatedAt:    now,
		PlayTime:     0,
	}
}

func (gs *GameState) Update() {
	gs.UpdatedAt = time.Now()
}

func (gs *GameState) Summary() string {
	return gs.Character.Summary() + " | " + gs.World.CurrentLocation.Name
}

type GameSession struct {
	State       *GameState
	Config      *Config
	StartedAt   time.Time
	IsModified  bool
}

func NewGameSession(state *GameState, config *Config) *GameSession {
	return &GameSession{
		State:      state,
		Config:     config,
		StartedAt:  time.Now(),
		IsModified: false,
	}
}

func (gs *GameSession) MarkModified() {
	gs.IsModified = true
	gs.State.Update()
}

func (gs *GameSession) AddPlayTime() {
	gs.State.PlayTime += time.Since(gs.StartedAt)
	gs.StartedAt = time.Now()
}

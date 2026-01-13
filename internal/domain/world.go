package domain

import (
	"time"
)

type Location struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Exits       []string `json:"exits,omitempty"`
}

type Quest struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Giver       string `json:"giver,omitempty"`
}

type NPC struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Disposition string `json:"disposition"`
	IsAlive     bool   `json:"is_alive"`
}

type WorldState struct {
	Setting         string              `json:"setting"`
	CurrentLocation Location            `json:"current_location"`
	TimeOfDay       string              `json:"time_of_day"`
	Weather         string              `json:"weather,omitempty"`
	DayNumber       int                 `json:"day_number"`
	Quests          []Quest             `json:"quests,omitempty"`
	NPCs            map[string]*NPC     `json:"npcs,omitempty"`
	Flags           map[string]bool     `json:"flags,omitempty"`
	Variables       map[string]string   `json:"variables,omitempty"`
	MemorySummary   string              `json:"memory_summary,omitempty"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
}

func NewWorldState(setting string) *WorldState {
	now := time.Now()
	return &WorldState{
		Setting: setting,
		CurrentLocation: Location{
			Name:        "Unknown",
			Description: "You find yourself in an unfamiliar place...",
			Exits:       []string{},
		},
		TimeOfDay: "morning",
		DayNumber: 1,
		Quests:    []Quest{},
		NPCs:      make(map[string]*NPC),
		Flags:     make(map[string]bool),
		Variables: make(map[string]string),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (w *WorldState) AddQuest(quest Quest) {
	w.Quests = append(w.Quests, quest)
	w.UpdatedAt = time.Now()
}

func (w *WorldState) UpdateQuestStatus(questID, status string) bool {
	for i, q := range w.Quests {
		if q.ID == questID {
			w.Quests[i].Status = status
			w.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

func (w *WorldState) GetActiveQuests() []Quest {
	var active []Quest
	for _, q := range w.Quests {
		if q.Status == "active" || q.Status == "in_progress" {
			active = append(active, q)
		}
	}
	return active
}

func (w *WorldState) SetLocation(loc Location) {
	w.CurrentLocation = loc
	w.UpdatedAt = time.Now()
}

func (w *WorldState) SetFlag(key string, value bool) {
	w.Flags[key] = value
	w.UpdatedAt = time.Now()
}

func (w *WorldState) GetFlag(key string) bool {
	return w.Flags[key]
}

func (w *WorldState) SetVariable(key, value string) {
	w.Variables[key] = value
	w.UpdatedAt = time.Now()
}

func (w *WorldState) GetVariable(key string) string {
	return w.Variables[key]
}

func (w *WorldState) AddNPC(npc *NPC) {
	w.NPCs[npc.Name] = npc
	w.UpdatedAt = time.Now()
}

func (w *WorldState) Summary() string {
	return w.Setting + " - Day " + string(rune(w.DayNumber+'0')) + ", " + w.TimeOfDay + " at " + w.CurrentLocation.Name
}

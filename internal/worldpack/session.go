package worldpack

import (
	"fmt"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// SessionConfig describes how a play session combines a world catalog with a
// rulesystem pack. Not wired into the live engine yet.
type SessionConfig struct {
	WorldID      string `json:"world_id" yaml:"world_id"`
	RulesystemID string `json:"rulesystem_id" yaml:"rulesystem_id"`
	Language     string `json:"language,omitempty" yaml:"language,omitempty"`
	Notes        string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

// Validate checks required fields for a session config (no I/O).
func (s SessionConfig) Validate() error {
	if s.WorldID == "" {
		return fmt.Errorf("session: world_id is required")
	}
	if s.RulesystemID == "" {
		return fmt.Errorf("session: rulesystem_id is required")
	}
	return nil
}

// CreatureStatBlock returns the stat block for a creature using the session rulesystem.
func CreatureStatBlock(entry CreatureEntry, rulesystemID string) domain.StatBlock {
	return entry.StatBlockFor(rulesystemID)
}

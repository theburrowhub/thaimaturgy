package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
)

// PartyMemberSpec is one roster entry the LLM produces from a natural-language
// request; the full stat block is generated deterministically from it.
type PartyMemberSpec struct {
	Name  string `json:"name"`
	Race  string `json:"race"`
	Class string `json:"class"`
	Level int    `json:"level"`
}

type partyRoster struct {
	Members []PartyMemberSpec `json:"members"`
}

// GeneratePartyFromSpecs builds full D&D character sheets from a roster, applying
// the rules in domain.GenerateCharacter. Empty race/class/level fall back to
// sensible defaults (Human/Fighter/level 1).
func GeneratePartyFromSpecs(specs []PartyMemberSpec) []*domain.Character {
	party := make([]*domain.Character, 0, len(specs))
	perRace := map[string]int{} // distinct sample names per race for unnamed entries
	for _, s := range specs {
		name := strings.TrimSpace(s.Name)
		if name == "" {
			key := strings.ToLower(domain.NormalizeRace(s.Race))
			name = domain.SampleName(s.Race, perRace[key])
			perRace[key]++
		}
		party = append(party, domain.GenerateCharacter(name, s.Race, s.Class, s.Level))
	}
	domain.EnsureUniqueNames(party) // safety net against any remaining duplicates
	return party
}

// PlanParty turns a natural-language request into a party: it asks the LLM for a
// roster (names/races/classes/levels), optionally adjusting the current party,
// then generates full stat blocks deterministically by D&D rules. The LLM only
// decides the roster; stats come from the rules, never from the model.
func PlanParty(ctx context.Context, provider providers.Provider, model, prompt string, current []domain.Character) ([]*domain.Character, error) {
	if provider == nil {
		return nil, fmt.Errorf("no AI provider configured")
	}
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("empty request")
	}

	sys := "You build a Dungeons & Dragons 5e adventuring party from the user's request.\n" +
		"Output ONLY JSON, no prose, of the form: {\"members\":[{\"name\":\"\",\"race\":\"\",\"class\":\"\",\"level\":1}]}.\n" +
		"Allowed races: " + strings.Join(domain.Races, ", ") + ".\n" +
		"Allowed classes: " + strings.Join(domain.Classes, ", ") + ".\n" +
		"Rules: default level is 1; give each member a fitting fantasy name; if the user doesn't specify how many, make a balanced heterogeneous party of 3-4 members with distinct races and classes. " +
		"If a CURRENT PARTY is provided, apply the user's requested changes to it and return the FULL updated roster. Do not include stats — they are computed from the roster by rules."

	var userMsg strings.Builder
	if len(current) > 0 {
		cur := make([]PartyMemberSpec, 0, len(current))
		for _, c := range current {
			cur = append(cur, PartyMemberSpec{Name: c.Name, Race: c.Race, Class: c.Class, Level: c.Level})
		}
		if b, err := json.Marshal(partyRoster{Members: cur}); err == nil {
			userMsg.WriteString("CURRENT PARTY:\n")
			userMsg.Write(b)
			userMsg.WriteString("\n\n")
		}
	}
	userMsg.WriteString("REQUEST:\n")
	userMsg.WriteString(prompt)

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	chat, err := provider.Chat(ctx, providers.ChatRequest{
		Messages: []providers.Message{
			{Role: providers.RoleSystem, Content: sys},
			{Role: providers.RoleUser, Content: userMsg.String()},
		},
		Model:       model,
		Temperature: 0.4,
		MaxTokens:   1200,
	})
	if err != nil {
		return nil, fmt.Errorf("party planning failed: %w", err)
	}

	roster, err := parseRoster(chat.Content)
	if err != nil {
		return nil, err
	}
	if len(roster.Members) == 0 {
		return nil, fmt.Errorf("the AI returned no party members")
	}
	return GeneratePartyFromSpecs(roster.Members), nil
}

// parseRoster extracts the JSON roster from a model reply, tolerating code fences
// or surrounding prose by taking the outermost {...} span.
func parseRoster(content string) (*partyRoster, error) {
	s := strings.TrimSpace(content)
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("could not find a JSON roster in the AI reply")
	}
	var r partyRoster
	if err := json.Unmarshal([]byte(s[start:end+1]), &r); err != nil {
		return nil, fmt.Errorf("could not parse the AI roster: %w", err)
	}
	return &r, nil
}

package engine

import (
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

func openingAdventure(intro string, hooks []string) *domain.Adventure {
	return &domain.Adventure{
		SchemaVersion: domain.SchemaVersion,
		ID:            "op", Title: "Opening Test",
		Introduction: intro,
		Hooks:        hooks,
		Zones: []domain.Zone{{ID: "z1", Name: "Start Zone", Rooms: []domain.Room{
			{ID: "r1", Name: "Doorstep"},
		}}},
		StartRoom: "r1",
	}
}

// The oracle must inject the authored introduction and hooks so the DM can
// narrate the opening — this is what lets ANY adventure open with its premise
// instead of dropping the party into a bare location. (#84)
func TestBuildSystemPromptIncludesOpening(t *testing.T) {
	adv := openingAdventure(
		"A patron summons the party to recover a stolen relic.",
		[]string{"The party owes the patron a debt.", "A rival crew is also after it."},
	)
	s := domain.NewSession(domain.NewSessionState("op_session", adv), adv, domain.DefaultConfig())
	prompt := NewOracle(s, nil).buildSystemPrompt()

	for _, want := range []string{
		"introduction", // section header
		"A patron summons the party to recover a stolen relic.",
		"Hooks",
		"The party owes the patron a debt.",
		"A rival crew is also after it.",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("system prompt missing %q", want)
		}
	}
}

// A module without an introduction/hooks must not grow empty opening sections.
func TestBuildSystemPromptOmitsAbsentOpening(t *testing.T) {
	adv := openingAdventure("", nil)
	s := domain.NewSession(domain.NewSessionState("op_session", adv), adv, domain.DefaultConfig())
	prompt := NewOracle(s, nil).buildSystemPrompt()
	if strings.Contains(prompt, "How it begins") || strings.Contains(prompt, "Hooks (") {
		t.Errorf("opening sections should be omitted when unauthored:\n%s", prompt)
	}
}

// The kickoff instruction must tell the DM to narrate the premise/hook first,
// in both languages, so the opening isn't skipped.
func TestDMKickoffNarratesHook(t *testing.T) {
	en := strings.ToLower(domain.DMKickoffPrompt(domain.LangEnglish))
	if !strings.Contains(en, "hook") || !strings.Contains(en, "premise") {
		t.Errorf("EN kickoff should mention premise+hook: %q", en)
	}
	es := strings.ToLower(domain.DMKickoffPrompt(domain.LangSpanish))
	if !strings.Contains(es, "gancho") || !strings.Contains(es, "premisa") {
		t.Errorf("ES kickoff should mention premise+hook: %q", es)
	}
}

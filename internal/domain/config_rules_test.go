package domain

import (
	"strings"
	"testing"
)

func TestDefaultPromptsDelegateMechanicsToLoadedRulesPackage(t *testing.T) {
	for name, test := range map[string]struct {
		prompt, conditionalSheetTools string
	}{
		"assistant-en": {DefaultSystemPromptEN, "only when they are available"},
		"assistant-es": {DefaultSystemPromptES, "solo cuando estén disponibles"},
		"gm-en":        {DefaultGMPromptEN, "when compatible d&d sheet tools are advertised"},
		"gm-es":        {DefaultGMPromptES, "cuando se anuncien herramientas compatibles de ficha de d&d"},
	} {
		t.Run(name, func(t *testing.T) {
			prompt := test.prompt
			for _, required := range []string{"game_list_actions", "game_submit_intent", "game_respond", "game_explain"} {
				if !strings.Contains(prompt, required) {
					t.Fatalf("prompt does not teach stable rules tool %q", required)
				}
			}
			for _, legacy := range []string{"roll_dice", "ability_check"} {
				if strings.Contains(prompt, legacy) {
					t.Fatalf("prompt still teaches dnd5e-only alias %q", legacy)
				}
			}
			if !strings.Contains(strings.ToLower(prompt), test.conditionalSheetTools) {
				t.Fatalf("prompt does not condition D&D-only sheet tools on availability")
			}
		})
	}
}

package domain

import (
	"strings"
	"testing"
)

func TestDefaultPromptsDelegateMechanicsToLoadedRulesPackage(t *testing.T) {
	for name, prompt := range map[string]string{
		"assistant-en": DefaultSystemPromptEN,
		"assistant-es": DefaultSystemPromptES,
		"gm-en":        DefaultGMPromptEN,
		"gm-es":        DefaultGMPromptES,
	} {
		t.Run(name, func(t *testing.T) {
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
		})
	}
}

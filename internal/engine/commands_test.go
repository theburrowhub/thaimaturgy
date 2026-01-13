package engine

import (
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		input    string
		cmdType  CommandType
		argsLen  int
		paramsLen int
	}{
		{"/help", CmdHelp, 0, 0},
		{"/h", CmdHelp, 0, 0},
		{"/?", CmdHelp, 0, 0},
		{":help", CmdHelp, 0, 0},
		{"/quit", CmdQuit, 0, 0},
		{"/q", CmdQuit, 0, 0},
		{"/exit", CmdQuit, 0, 0},
		{"/save myGame", CmdSave, 1, 0},
		{"/load myGame", CmdLoad, 1, 0},
		{"/new", CmdNew, 0, 0},
		{"/roll 2d6+3", CmdRoll, 1, 0},
		{"/roll", CmdRoll, 0, 0},
		{"/r 1d20", CmdRoll, 1, 0},
		{"/status", CmdStatus, 0, 0},
		{"/st", CmdStatus, 0, 0},
		{"/inv", CmdInventory, 0, 0},
		{"/inv add sword", CmdInvAdd, 1, 0},
		{"/inv rm sword", CmdInvRemove, 1, 0},
		{"/cond add Poisoned", CmdCondAdd, 1, 0},
		{"/cond rm Poisoned", CmdCondRemove, 1, 0},
		{"/provider openai", CmdProvider, 1, 0},
		{"/model gpt-4", CmdModel, 1, 0},
		{"/temp 0.7", CmdTemp, 1, 0},
		{"/char set name=Bob", CmdCharSet, 0, 1},
		{"/char set str=18 dex=14", CmdCharSet, 0, 2},
		{"I attack the goblin", CmdNarration, 1, 0},
		{"Look around the room", CmdNarration, 1, 0},
		{"/unknown", CmdUnknown, 1, 0},
		{"", CommandType(0), 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cmd := ParseCommand(tt.input)

			if tt.input == "" {
				if cmd != nil {
					t.Errorf("ParseCommand(%q) = %v, want nil", tt.input, cmd)
				}
				return
			}

			if cmd == nil {
				t.Fatalf("ParseCommand(%q) = nil, want non-nil", tt.input)
			}

			if cmd.Type != tt.cmdType {
				t.Errorf("ParseCommand(%q).Type = %v, want %v", tt.input, cmd.Type, tt.cmdType)
			}

			if len(cmd.Args) != tt.argsLen {
				t.Errorf("ParseCommand(%q) args len = %d, want %d", tt.input, len(cmd.Args), tt.argsLen)
			}

			if len(cmd.Params) != tt.paramsLen {
				t.Errorf("ParseCommand(%q) params len = %d, want %d", tt.input, len(cmd.Params), tt.paramsLen)
			}
		})
	}
}

func TestParseCommandKeyValues(t *testing.T) {
	cmd := ParseCommand("/char set name=Alice str=16 dex=14")

	if cmd.Type != CmdCharSet {
		t.Fatalf("Expected CmdCharSet, got %v", cmd.Type)
	}

	expectedParams := map[string]string{
		"name": "Alice",
		"str":  "16",
		"dex":  "14",
	}

	for key, expected := range expectedParams {
		if got := cmd.Params[key]; got != expected {
			t.Errorf("Params[%q] = %q, want %q", key, got, expected)
		}
	}
}

func TestCommandHandlerHelp(t *testing.T) {
	session := createTestSession()
	handler := NewCommandHandler(session)

	cmd := ParseCommand("/help")
	result := handler.Execute(cmd)

	if !result.Success {
		t.Error("Help command should succeed")
	}
	if result.Response == "" {
		t.Error("Help command should return response text")
	}
}

func TestCommandHandlerRoll(t *testing.T) {
	session := createTestSession()
	handler := NewCommandHandler(session)

	cmd := ParseCommand("/roll 2d6")
	result := handler.Execute(cmd)

	if !result.Success {
		t.Errorf("Roll command failed: %s", result.Message)
	}
	if len(result.Events) == 0 {
		t.Error("Roll command should produce events")
	}
	if result.Events[0].Type != domain.EventTypeDiceRoll {
		t.Errorf("Expected dice roll event, got %v", result.Events[0].Type)
	}
}

func TestCommandHandlerInventory(t *testing.T) {
	session := createTestSession()
	handler := NewCommandHandler(session)

	cmd := ParseCommand("/inv add sword")
	result := handler.Execute(cmd)

	if !result.Success {
		t.Errorf("Add item failed: %s", result.Message)
	}

	if len(session.State.Character.Inventory) == 0 {
		t.Error("Item should be added to inventory")
	}
	if session.State.Character.Inventory[0].Name != "sword" {
		t.Errorf("Item name = %q, want %q", session.State.Character.Inventory[0].Name, "sword")
	}

	cmd = ParseCommand("/inv rm sword")
	result = handler.Execute(cmd)

	if !result.Success {
		t.Errorf("Remove item failed: %s", result.Message)
	}

	if len(session.State.Character.Inventory) != 0 {
		t.Error("Item should be removed from inventory")
	}
}

func TestCommandHandlerConditions(t *testing.T) {
	session := createTestSession()
	handler := NewCommandHandler(session)

	cmd := ParseCommand("/cond add Poisoned")
	result := handler.Execute(cmd)

	if !result.Success {
		t.Errorf("Add condition failed: %s", result.Message)
	}

	if !session.State.Character.HasCondition("Poisoned") {
		t.Error("Condition should be added")
	}

	cmd = ParseCommand("/cond rm Poisoned")
	result = handler.Execute(cmd)

	if !result.Success {
		t.Errorf("Remove condition failed: %s", result.Message)
	}

	if session.State.Character.HasCondition("Poisoned") {
		t.Error("Condition should be removed")
	}
}

func TestCommandHandlerCharSet(t *testing.T) {
	session := createTestSession()
	handler := NewCommandHandler(session)

	cmd := ParseCommand("/char set name=TestHero str=18 ac=16")
	result := handler.Execute(cmd)

	if !result.Success {
		t.Errorf("Char set failed: %s", result.Message)
	}

	char := session.State.Character
	if char.Name != "TestHero" {
		t.Errorf("Name = %q, want %q", char.Name, "TestHero")
	}
	if char.Abilities.STR != 18 {
		t.Errorf("STR = %d, want %d", char.Abilities.STR, 18)
	}
	if char.AC != 16 {
		t.Errorf("AC = %d, want %d", char.AC, 16)
	}
}

func TestCommandHandlerProvider(t *testing.T) {
	session := createTestSession()
	handler := NewCommandHandler(session)

	cmd := ParseCommand("/provider anthropic")
	result := handler.Execute(cmd)

	if !result.Success {
		t.Errorf("Provider command failed: %s", result.Message)
	}
	if session.Config.Provider != domain.ProviderAnthropic {
		t.Errorf("Provider = %v, want %v", session.Config.Provider, domain.ProviderAnthropic)
	}

	cmd = ParseCommand("/provider invalid")
	result = handler.Execute(cmd)

	if result.Success {
		t.Error("Invalid provider should fail")
	}
}

func TestCommandHandlerTemperature(t *testing.T) {
	session := createTestSession()
	handler := NewCommandHandler(session)

	cmd := ParseCommand("/temp 0.5")
	result := handler.Execute(cmd)

	if !result.Success {
		t.Errorf("Temperature command failed: %s", result.Message)
	}
	if session.Config.Temperature != 0.5 {
		t.Errorf("Temperature = %f, want %f", session.Config.Temperature, 0.5)
	}

	cmd = ParseCommand("/temp 3.0")
	result = handler.Execute(cmd)

	if result.Success {
		t.Error("Invalid temperature should fail")
	}
}

func TestCommandHandlerNarration(t *testing.T) {
	session := createTestSession()
	handler := NewCommandHandler(session)

	cmd := ParseCommand("I search the room carefully")
	result := handler.Execute(cmd)

	if cmd.Type != CmdNarration {
		t.Errorf("Expected CmdNarration, got %v", cmd.Type)
	}
	if !result.NeedsUI {
		t.Error("Narration should need UI handling")
	}
	if result.UIAction != "narration" {
		t.Errorf("UIAction = %q, want %q", result.UIAction, "narration")
	}
}

func TestCommandHandlerQuit(t *testing.T) {
	session := createTestSession()
	handler := NewCommandHandler(session)

	cmd := ParseCommand("/quit")
	result := handler.Execute(cmd)

	if !result.ShouldQuit {
		t.Error("Quit command should set ShouldQuit")
	}
}

func TestCommandHandlerStatus(t *testing.T) {
	session := createTestSession()
	handler := NewCommandHandler(session)

	cmd := ParseCommand("/status")
	result := handler.Execute(cmd)

	if !result.Success {
		t.Errorf("Status command failed: %s", result.Message)
	}
	if result.Response == "" {
		t.Error("Status should return character info")
	}
}

func createTestSession() *domain.GameSession {
	char := domain.NewCharacter("TestChar", "Human", "Fighter")
	state := domain.NewGameState("test_save", char, "fantasy")
	config := domain.DefaultConfig()
	return domain.NewGameSession(state, config)
}

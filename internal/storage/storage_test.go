package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

func TestNewStorage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "thaimaturgy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewWithPath(tmpDir)
	if err != nil {
		t.Fatalf("NewWithPath failed: %v", err)
	}

	savesDir := filepath.Join(tmpDir, SavesDir)
	if _, err := os.Stat(savesDir); os.IsNotExist(err) {
		t.Error("Saves directory should be created")
	}

	if store.BasePath() != tmpDir {
		t.Errorf("BasePath() = %q, want %q", store.BasePath(), tmpDir)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "thaimaturgy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewWithPath(tmpDir)

	config := &domain.Config{
		Provider:    domain.ProviderAnthropic,
		Model:       "claude-3-opus-20240229",
		Temperature: 0.7,
		MaxTokens:   4096,
	}

	err = store.SaveConfig(config)
	if err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	loaded, err := store.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if loaded.Provider != config.Provider {
		t.Errorf("Provider = %v, want %v", loaded.Provider, config.Provider)
	}
	if loaded.Model != config.Model {
		t.Errorf("Model = %q, want %q", loaded.Model, config.Model)
	}
	if loaded.Temperature != config.Temperature {
		t.Errorf("Temperature = %f, want %f", loaded.Temperature, config.Temperature)
	}
}

func TestLoadConfigDefault(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "thaimaturgy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewWithPath(tmpDir)

	config, err := store.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if config.Provider != domain.ProviderOpenAI {
		t.Errorf("Default provider should be OpenAI, got %v", config.Provider)
	}
}

func TestSaveAndLoadGame(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "thaimaturgy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewWithPath(tmpDir)

	char := domain.NewCharacter("TestHero", "Elf", "Wizard")
	char.Level = 5
	char.CurrentHP = 25
	char.MaxHP = 30
	char.Abilities.INT = 18
	char.AddItem(domain.InventoryItem{Name: "Staff", Quantity: 1})
	char.AddCondition(domain.ConditionPoisoned)
	char.Gold = 100

	state := domain.NewGameState("test_save", char, "fantasy")
	state.World.CurrentLocation = domain.Location{
		Name:        "Dark Forest",
		Description: "A spooky forest",
	}
	state.Conversation.AddUserMessage("Hello")
	state.Conversation.AddAssistantMessage("Welcome, adventurer!")

	err = store.SaveGame(state)
	if err != nil {
		t.Fatalf("SaveGame failed: %v", err)
	}

	loaded, err := store.LoadGame("test_save")
	if err != nil {
		t.Fatalf("LoadGame failed: %v", err)
	}

	if loaded.Character.Name != "TestHero" {
		t.Errorf("Character name = %q, want %q", loaded.Character.Name, "TestHero")
	}
	if loaded.Character.Race != "Elf" {
		t.Errorf("Character race = %q, want %q", loaded.Character.Race, "Elf")
	}
	if loaded.Character.Class != "Wizard" {
		t.Errorf("Character class = %q, want %q", loaded.Character.Class, "Wizard")
	}
	if loaded.Character.Level != 5 {
		t.Errorf("Character level = %d, want %d", loaded.Character.Level, 5)
	}
	if loaded.Character.CurrentHP != 25 {
		t.Errorf("Character HP = %d, want %d", loaded.Character.CurrentHP, 25)
	}
	if loaded.Character.Abilities.INT != 18 {
		t.Errorf("Character INT = %d, want %d", loaded.Character.Abilities.INT, 18)
	}
	if len(loaded.Character.Inventory) != 1 {
		t.Errorf("Inventory length = %d, want %d", len(loaded.Character.Inventory), 1)
	}
	if len(loaded.Character.Conditions) != 1 {
		t.Errorf("Conditions length = %d, want %d", len(loaded.Character.Conditions), 1)
	}
	if loaded.Character.Gold != 100 {
		t.Errorf("Gold = %d, want %d", loaded.Character.Gold, 100)
	}
	if loaded.World.CurrentLocation.Name != "Dark Forest" {
		t.Errorf("Location = %q, want %q", loaded.World.CurrentLocation.Name, "Dark Forest")
	}
	if loaded.Conversation.Len() != 2 {
		t.Errorf("Conversation length = %d, want %d", loaded.Conversation.Len(), 2)
	}
}

func TestSaveExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "thaimaturgy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewWithPath(tmpDir)

	if store.SaveExists("nonexistent") {
		t.Error("SaveExists should return false for nonexistent save")
	}

	char := domain.NewCharacter("Test", "Human", "Fighter")
	state := domain.NewGameState("existing", char, "fantasy")
	store.SaveGame(state)

	if !store.SaveExists("existing") {
		t.Error("SaveExists should return true for existing save")
	}
}

func TestDeleteGame(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "thaimaturgy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewWithPath(tmpDir)

	char := domain.NewCharacter("Test", "Human", "Fighter")
	state := domain.NewGameState("to_delete", char, "fantasy")
	store.SaveGame(state)

	if !store.SaveExists("to_delete") {
		t.Fatal("Save should exist before deletion")
	}

	err = store.DeleteGame("to_delete")
	if err != nil {
		t.Fatalf("DeleteGame failed: %v", err)
	}

	if store.SaveExists("to_delete") {
		t.Error("Save should not exist after deletion")
	}
}

func TestListSaves(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "thaimaturgy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewWithPath(tmpDir)

	saves, err := store.ListSaves()
	if err != nil {
		t.Fatalf("ListSaves failed: %v", err)
	}
	if len(saves) != 0 {
		t.Errorf("Expected 0 saves, got %d", len(saves))
	}

	for i := 0; i < 3; i++ {
		char := domain.NewCharacter("Hero"+string(rune('1'+i)), "Human", "Fighter")
		char.Level = i + 1
		state := domain.NewGameState("save_"+string(rune('1'+i)), char, "fantasy")
		store.SaveGame(state)
	}

	saves, err = store.ListSaves()
	if err != nil {
		t.Fatalf("ListSaves failed: %v", err)
	}
	if len(saves) != 3 {
		t.Errorf("Expected 3 saves, got %d", len(saves))
	}
}

func TestLoadGameNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "thaimaturgy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewWithPath(tmpDir)

	_, err = store.LoadGame("nonexistent")
	if err == nil {
		t.Error("LoadGame should fail for nonexistent save")
	}
}

func TestSaveGameNoName(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "thaimaturgy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewWithPath(tmpDir)

	char := domain.NewCharacter("Test", "Human", "Fighter")
	state := domain.NewGameState("", char, "fantasy")

	err = store.SaveGame(state)
	if err == nil {
		t.Error("SaveGame should fail without save name")
	}
}

func TestSaveAndDeleteAPIKey(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "thaimaturgy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewWithPath(tmpDir)

	if store.EnvFileExists() {
		t.Error("Env file should not exist initially")
	}

	err = store.SaveAPIKey(domain.ProviderOpenAI, "sk-test-key-123")
	if err != nil {
		t.Fatalf("SaveAPIKey failed: %v", err)
	}

	if !store.EnvFileExists() {
		t.Error("Env file should exist after saving API key")
	}

	err = store.DeleteEnvFile()
	if err != nil {
		t.Fatalf("DeleteEnvFile failed: %v", err)
	}

	if store.EnvFileExists() {
		t.Error("Env file should not exist after deletion")
	}
}

func TestSaveAPIKeyAnthropic(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "thaimaturgy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewWithPath(tmpDir)

	err = store.SaveAPIKey(domain.ProviderAnthropic, "sk-ant-test-key-456")
	if err != nil {
		t.Fatalf("SaveAPIKey failed: %v", err)
	}

	if !store.EnvFileExists() {
		t.Error("Env file should exist after saving API key")
	}

	data, err := os.ReadFile(store.EnvFilePath())
	if err != nil {
		t.Fatalf("Failed to read env file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "THAIM_PROVIDER=anthropic") {
		t.Error("Env file should contain THAIM_PROVIDER=anthropic")
	}
	if !strings.Contains(content, "THAIM_ANTHROPIC_API_KEY=sk-ant-test-key-456") {
		t.Error("Env file should contain the API key")
	}
}

func TestLoadEnvFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "thaimaturgy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewWithPath(tmpDir)

	err = store.SaveAPIKey(domain.ProviderOpenAI, "sk-test-load-key")
	if err != nil {
		t.Fatalf("SaveAPIKey failed: %v", err)
	}

	os.Unsetenv("THAIM_PROVIDER")
	os.Unsetenv("THAIM_OPENAI_API_KEY")

	err = store.LoadEnvFile()
	if err != nil {
		t.Fatalf("LoadEnvFile failed: %v", err)
	}

	if os.Getenv("THAIM_PROVIDER") != "openai" {
		t.Errorf("THAIM_PROVIDER = %q, want %q", os.Getenv("THAIM_PROVIDER"), "openai")
	}
	if os.Getenv("THAIM_OPENAI_API_KEY") != "sk-test-load-key" {
		t.Errorf("THAIM_OPENAI_API_KEY = %q, want %q", os.Getenv("THAIM_OPENAI_API_KEY"), "sk-test-load-key")
	}

	os.Unsetenv("THAIM_PROVIDER")
	os.Unsetenv("THAIM_OPENAI_API_KEY")
}

func TestDeleteEnvFileNotExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "thaimaturgy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewWithPath(tmpDir)

	err = store.DeleteEnvFile()
	if err != nil {
		t.Error("DeleteEnvFile should not fail if file doesn't exist")
	}
}

func TestEnvFilePath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "thaimaturgy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewWithPath(tmpDir)

	expectedPath := tmpDir + "/.env"
	if store.EnvFilePath() != expectedPath {
		t.Errorf("EnvFilePath() = %q, want %q", store.EnvFilePath(), expectedPath)
	}
}

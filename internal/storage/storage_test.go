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

	sessionsDir := filepath.Join(tmpDir, SessionsDir)
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		t.Error("Sessions directory should be created")
	}

	if store.BasePath() != tmpDir {
		t.Errorf("BasePath() = %q, want %q", store.BasePath(), tmpDir)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "thaimaturgy-test-*")
	defer os.RemoveAll(tmpDir)

	store, _ := NewWithPath(tmpDir)

	config := &domain.Config{
		Provider:    domain.ProviderAnthropic,
		Model:       "claude-3-opus-20240229",
		Temperature: 0.7,
		MaxTokens:   4096,
	}

	if err := store.SaveConfig(config); err != nil {
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
}

func TestLoadConfigDefault(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "thaimaturgy-test-*")
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

func sampleAdventure() *domain.Adventure {
	return &domain.Adventure{
		SchemaVersion: domain.SchemaVersion,
		ID:            "test-adv",
		Title:         "Test Adventure",
		Zones: []domain.Zone{{
			ID:    "z1",
			Name:  "Zone One",
			Rooms: []domain.Room{{ID: "r1", Name: "Entrance"}},
		}},
	}
}

func TestSaveAndLoadSession(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "thaimaturgy-test-*")
	defer os.RemoveAll(tmpDir)

	store, _ := NewWithPath(tmpDir)

	state := domain.NewSessionState("test_session", sampleAdventure())
	state.AddNote("The party arrived at the gate.")
	state.Conversation.AddUserMessage("Where are we?")
	state.Conversation.AddAssistantMessage("At the entrance.")

	if err := store.SaveSession(state); err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	loaded, err := store.LoadSession("test_session")
	if err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}
	if loaded.AdventureID != "test-adv" {
		t.Errorf("AdventureID = %q, want %q", loaded.AdventureID, "test-adv")
	}
	if loaded.CurrentRoom != "r1" {
		t.Errorf("CurrentRoom = %q, want %q", loaded.CurrentRoom, "r1")
	}
	if loaded.Log.Len() == 0 {
		t.Error("expected timeline entries to persist")
	}
	if loaded.Conversation.Len() != 2 {
		t.Errorf("Conversation length = %d, want 2", loaded.Conversation.Len())
	}
}

func TestSessionExists(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "thaimaturgy-test-*")
	defer os.RemoveAll(tmpDir)

	store, _ := NewWithPath(tmpDir)
	if store.SessionExists("nonexistent") {
		t.Error("SessionExists should be false for nonexistent session")
	}

	_ = store.SaveSession(domain.NewSessionState("existing", sampleAdventure()))
	if !store.SessionExists("existing") {
		t.Error("SessionExists should be true after saving")
	}
}

func TestDeleteSession(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "thaimaturgy-test-*")
	defer os.RemoveAll(tmpDir)

	store, _ := NewWithPath(tmpDir)
	_ = store.SaveSession(domain.NewSessionState("to_delete", sampleAdventure()))

	if err := store.DeleteSession("to_delete"); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}
	if store.SessionExists("to_delete") {
		t.Error("session should not exist after deletion")
	}
}

func TestListSessions(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "thaimaturgy-test-*")
	defer os.RemoveAll(tmpDir)

	store, _ := NewWithPath(tmpDir)
	sessions, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}

	for i := 0; i < 3; i++ {
		_ = store.SaveSession(domain.NewSessionState("session_"+string(rune('1'+i)), sampleAdventure()))
	}
	sessions, _ = store.ListSessions()
	if len(sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(sessions))
	}
}

func TestSaveSessionNoName(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "thaimaturgy-test-*")
	defer os.RemoveAll(tmpDir)

	store, _ := NewWithPath(tmpDir)
	state := domain.NewSessionState("", sampleAdventure())
	if err := store.SaveSession(state); err == nil {
		t.Error("SaveSession should fail without a name")
	}
}

func TestSaveAndDeleteAPIKey(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "thaimaturgy-test-*")
	defer os.RemoveAll(tmpDir)

	store, _ := NewWithPath(tmpDir)
	if store.EnvFileExists() {
		t.Error("Env file should not exist initially")
	}
	if err := store.SaveAPIKey(domain.ProviderOpenAI, "sk-test-key-123"); err != nil {
		t.Fatalf("SaveAPIKey failed: %v", err)
	}
	if !store.EnvFileExists() {
		t.Error("Env file should exist after saving API key")
	}
	if err := store.DeleteEnvFile(); err != nil {
		t.Fatalf("DeleteEnvFile failed: %v", err)
	}
	if store.EnvFileExists() {
		t.Error("Env file should not exist after deletion")
	}
}

func TestSaveAPIKeyAnthropic(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "thaimaturgy-test-*")
	defer os.RemoveAll(tmpDir)

	store, _ := NewWithPath(tmpDir)
	if err := store.SaveAPIKey(domain.ProviderAnthropic, "sk-ant-test-key-456"); err != nil {
		t.Fatalf("SaveAPIKey failed: %v", err)
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
	tmpDir, _ := os.MkdirTemp("", "thaimaturgy-test-*")
	defer os.RemoveAll(tmpDir)

	store, _ := NewWithPath(tmpDir)
	if err := store.SaveAPIKey(domain.ProviderOpenAI, "sk-test-load-key"); err != nil {
		t.Fatalf("SaveAPIKey failed: %v", err)
	}
	os.Unsetenv("THAIM_PROVIDER")
	os.Unsetenv("THAIM_OPENAI_API_KEY")
	if err := store.LoadEnvFile(); err != nil {
		t.Fatalf("LoadEnvFile failed: %v", err)
	}
	if os.Getenv("THAIM_PROVIDER") != "openai" {
		t.Errorf("THAIM_PROVIDER = %q, want openai", os.Getenv("THAIM_PROVIDER"))
	}
	os.Unsetenv("THAIM_PROVIDER")
	os.Unsetenv("THAIM_OPENAI_API_KEY")
}

func TestDeleteEnvFileNotExists(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "thaimaturgy-test-*")
	defer os.RemoveAll(tmpDir)

	store, _ := NewWithPath(tmpDir)
	if err := store.DeleteEnvFile(); err != nil {
		t.Error("DeleteEnvFile should not fail if file doesn't exist")
	}
}

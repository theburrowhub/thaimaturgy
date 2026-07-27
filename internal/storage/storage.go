package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

const (
	AppDir      = ".thaimaturgy"
	ConfigFile  = "config.json"
	SessionsDir = "sessions"
	EnvFile     = ".env"
)

type Storage struct {
	basePath string
}

func New() (*Storage, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	basePath := filepath.Join(home, AppDir)
	s := &Storage{basePath: basePath}

	if err := s.ensureDirectories(); err != nil {
		return nil, err
	}

	return s, nil
}

func NewWithPath(basePath string) (*Storage, error) {
	s := &Storage{basePath: basePath}
	if err := s.ensureDirectories(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Storage) ensureDirectories() error {
	dirs := []string{
		s.basePath,
		filepath.Join(s.basePath, SessionsDir),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

func (s *Storage) BasePath() string {
	return s.basePath
}

func (s *Storage) LoadConfig() (*domain.Config, error) {
	configPath := filepath.Join(s.basePath, ConfigFile)

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return s.loadConfigFromEnv(), nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config domain.Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	s.mergeEnvConfig(&config)

	return &config, nil
}

func (s *Storage) loadConfigFromEnv() *domain.Config {
	config := domain.DefaultConfig()
	s.mergeEnvConfig(config)
	return config
}

func (s *Storage) mergeEnvConfig(config *domain.Config) {
	if provider := os.Getenv("THAIM_PROVIDER"); provider != "" {
		config.Provider = domain.ProviderType(strings.ToLower(provider))
	}
	if model := os.Getenv("THAIM_MODEL"); model != "" {
		config.Model = model
	}
	if apiKey := os.Getenv("THAIM_OPENAI_API_KEY"); apiKey != "" {
		config.OpenAIAPIKey = apiKey
	}
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" && config.OpenAIAPIKey == "" {
		config.OpenAIAPIKey = apiKey
	}
	if apiKey := os.Getenv("THAIM_ANTHROPIC_API_KEY"); apiKey != "" {
		config.AnthropicAPIKey = apiKey
	}
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" && config.AnthropicAPIKey == "" {
		config.AnthropicAPIKey = apiKey
	}
}

func (s *Storage) SaveConfig(config *domain.Config) error {
	configPath := filepath.Join(s.basePath, ConfigFile)

	configToSave := *config
	configToSave.OpenAIAPIKey = ""
	configToSave.AnthropicAPIKey = ""

	data, err := json.MarshalIndent(configToSave, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

func (s *Storage) sessionPath(name string) string {
	return filepath.Join(s.basePath, SessionsDir, name+".json")
}

// LoadSession reads a persisted play session by name.
func (s *Storage) LoadSession(name string) (*domain.SessionState, error) {
	data, err := os.ReadFile(s.sessionPath(name))
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var state domain.SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse session file: %w", err)
	}

	return &state, nil
}

// SaveSession persists a play session as JSON.
func (s *Storage) SaveSession(state *domain.SessionState) error {
	if state.Name == "" {
		return fmt.Errorf("session name is required")
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	if err := os.WriteFile(s.sessionPath(state.Name), data, 0644); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

// DeleteSession removes a persisted session.
func (s *Storage) DeleteSession(name string) error {
	if err := os.Remove(s.sessionPath(name)); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("session not found: %s", name)
		}
		return fmt.Errorf("failed to delete session file: %w", err)
	}

	return nil
}

// ListSessions enumerates persisted sessions with lightweight metadata.
func (s *Storage) ListSessions() ([]SessionInfo, error) {
	entries, err := os.ReadDir(filepath.Join(s.basePath, SessionsDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessions []SessionInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".json")
		info, err := entry.Info()
		if err != nil {
			continue
		}

		state, err := s.LoadSession(name)
		if err != nil {
			continue
		}

		sessions = append(sessions, SessionInfo{
			Name:           name,
			AdventureID:    state.AdventureID,
			AdventureTitle: state.AdventureTitle,
			CurrentRoom:    state.CurrentRoom,
			PlayTime:       state.PlayTime,
			ModifiedAt:     info.ModTime(),
		})
	}

	return sessions, nil
}

// SessionExists reports whether a session with the given name exists.
func (s *Storage) SessionExists(name string) bool {
	_, err := os.Stat(s.sessionPath(name))
	return err == nil
}

// SessionInfo is lightweight metadata for listing sessions.
type SessionInfo struct {
	Name           string      `json:"name"`
	AdventureID    string      `json:"adventure_id"`
	AdventureTitle string      `json:"adventure_title"`
	CurrentRoom    string      `json:"current_room"`
	PlayTime       interface{} `json:"play_time"`
	ModifiedAt     interface{} `json:"modified_at"`
}

func (s *Storage) EnvFilePath() string {
	return filepath.Join(s.basePath, EnvFile)
}

func (s *Storage) SaveAPIKey(provider domain.ProviderType, apiKey string) error {
	envPath := s.EnvFilePath()

	var envContent string
	switch provider {
	case domain.ProviderOpenAI:
		envContent = fmt.Sprintf("THAIM_PROVIDER=openai\nTHAIM_OPENAI_API_KEY=%s\n", apiKey)
	case domain.ProviderAnthropic:
		envContent = fmt.Sprintf("THAIM_PROVIDER=anthropic\nTHAIM_ANTHROPIC_API_KEY=%s\n", apiKey)
	default:
		return fmt.Errorf("unknown provider: %s", provider)
	}

	if err := os.WriteFile(envPath, []byte(envContent), 0600); err != nil {
		return fmt.Errorf("failed to write .env file: %w", err)
	}

	return nil
}

func (s *Storage) LoadEnvFile() error {
	envPath := s.EnvFilePath()

	data, err := os.ReadFile(envPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read .env file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}

	return nil
}

func (s *Storage) DeleteEnvFile() error {
	envPath := s.EnvFilePath()

	if err := os.Remove(envPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to delete .env file: %w", err)
	}

	return nil
}

func (s *Storage) EnvFileExists() bool {
	envPath := s.EnvFilePath()
	_, err := os.Stat(envPath)
	return err == nil
}

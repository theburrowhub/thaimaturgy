package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

const (
	AppDir         = ".thaimaturgy"
	ConfigFile     = "config.json" // legacy; migrated to YAML on first load
	ConfigFileYAML = "config.yaml"
	ConfigAppName  = "thaimaturgy"
	SessionsDir    = "sessions"
	EnvFile        = ".env"
)

type Storage struct {
	basePath   string // data: adventures, sessions, .env
	configPath string // the YAML config file

	rosterMu sync.Mutex // serializes campaign-roster reads/writes/deletes (#33)
	usersMu  sync.Mutex // serializes user-account reads/writes/deletes (#151)
}

func New() (*Storage, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	basePath := filepath.Join(home, AppDir)
	// Prefer the OS config directory for the config file; fall back to the data
	// dir if it can't be determined.
	configPath := filepath.Join(basePath, ConfigFileYAML)
	if cfgDir, err := os.UserConfigDir(); err == nil {
		configPath = filepath.Join(cfgDir, ConfigAppName, ConfigFileYAML)
	}

	s := &Storage{basePath: basePath, configPath: configPath}
	if err := s.ensureDirectories(); err != nil {
		return nil, err
	}
	return s, nil
}

func NewWithPath(basePath string) (*Storage, error) {
	s := &Storage{basePath: basePath, configPath: filepath.Join(basePath, ConfigFileYAML)}
	if err := s.ensureDirectories(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Storage) ensureDirectories() error {
	dirs := []string{
		s.basePath,
		filepath.Join(s.basePath, SessionsDir),
		filepath.Join(s.basePath, CharactersDir),
		filepath.Dir(s.configPath),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// atomicWriteFile writes data to a temporary file in the destination's directory
// and renames it over path, so a partial or failed write (disk full, crash) can
// never truncate/destroy an existing file — the old contents survive until the
// new file is complete.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-"+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeds
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func (s *Storage) BasePath() string { return s.basePath }

// ConfigPath returns the path of the YAML configuration file.
func (s *Storage) ConfigPath() string { return s.configPath }

// ConfigExists reports whether the YAML config file has been written yet.
func (s *Storage) ConfigExists() bool {
	_, err := os.Stat(s.configPath)
	return err == nil
}

// LoadConfig reads the YAML config, migrating a legacy JSON config if present,
// and merges any environment overrides.
func (s *Storage) LoadConfig() (*domain.Config, error) {
	if data, err := os.ReadFile(s.configPath); err == nil {
		config := domain.DefaultConfig()
		if err := applyYAML(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", s.configPath, err)
		}
		s.mergeEnvConfig(config)
		return config, nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	// Migrate a legacy JSON config from the data dir, if any.
	if data, err := os.ReadFile(filepath.Join(s.basePath, ConfigFile)); err == nil {
		config := domain.DefaultConfig()
		if jerr := json.Unmarshal(data, config); jerr == nil {
			_ = s.SaveConfig(config) // rewrite as YAML at the new location
			s.mergeEnvConfig(config)
			return config, nil
		}
	}

	return s.loadConfigFromEnv(), nil
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
	if apiKey := os.Getenv("THAIM_GEMINI_API_KEY"); apiKey != "" {
		config.GeminiAPIKey = apiKey
	}
	if apiKey := os.Getenv("GEMINI_API_KEY"); apiKey != "" && config.GeminiAPIKey == "" {
		config.GeminiAPIKey = apiKey
	}
	if apiKey := os.Getenv("GOOGLE_API_KEY"); apiKey != "" && config.GeminiAPIKey == "" {
		config.GeminiAPIKey = apiKey
	}
}

// SaveConfig writes the config as organized YAML (secrets stripped) to the
// system config path.
func (s *Storage) SaveConfig(config *domain.Config) error {
	data, err := marshalYAML(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.configPath), 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	// 0600: the config may carry the Telegram bot token, so keep it owner-only.
	if err := os.WriteFile(s.configPath, data, 0600); err != nil {
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

	// Keep the full history from here on (older saves may carry the legacy bounded
	// caps). The oracle windows what it sends to the model, so this only affects
	// how much context is retained for resuming.
	if state.Log != nil {
		state.Log.MaxSize = 0
	}
	if state.Conversation != nil {
		state.Conversation.MaxSize = 0
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

// DeleteSession removes a persisted session and its saved novelization (so a
// later session reusing the name can't inherit a stale novel). The journal is
// intentionally left as a historical record, matching prior behavior.
func (s *Storage) DeleteSession(name string) error {
	if err := os.Remove(s.sessionPath(name)); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("session not found: %s", name)
		}
		return fmt.Errorf("failed to delete session file: %w", err)
	}
	_ = s.DeleteNovel(name)

	return nil
}

// RenameSession renames a persisted session, updating the name stored inside the
// file and moving it to the new path. It rejects empty or unsafe names and won't
// overwrite an existing session.
func (s *Storage) RenameSession(oldName, newName string) error {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return fmt.Errorf("new name is required")
	}
	if newName == oldName {
		return nil
	}
	if strings.ContainsAny(newName, `/\`) || strings.Contains(newName, "..") {
		return fmt.Errorf("invalid session name: %q", newName)
	}
	if s.SessionExists(newName) {
		return fmt.Errorf("a session named %q already exists", newName)
	}
	state, err := s.LoadSession(oldName)
	if err != nil {
		return err
	}
	state.Name = newName
	if err := s.SaveSession(state); err != nil {
		return err
	}
	// Move the append-only journal alongside the session so the chronicle stays
	// with it (and a later session reusing the old name can't inherit it).
	if err := os.Rename(s.sessionJournalPath(oldName), s.sessionJournalPath(newName)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to move session journal: %w", err)
	}
	// Move the saved novelization too, for the same reason.
	oldNovel, err := s.sessionNovelPath(oldName)
	if err != nil {
		return err
	}
	newNovel, err := s.sessionNovelPath(newName)
	if err != nil {
		return err
	}
	if err := os.Rename(oldNovel, newNovel); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to move session novel: %w", err)
	}
	return s.DeleteSession(oldName)
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
	case domain.ProviderGemini:
		envContent = fmt.Sprintf("THAIM_PROVIDER=gemini\nTHAIM_GEMINI_API_KEY=%s\n", apiKey)
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

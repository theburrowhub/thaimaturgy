package tts

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// MagnificMCPConfig configures command-backed Magnific MCP speech generation.
// Command should read one JSON object from stdin, call Magnific's audio_tts MCP
// tool, download the resulting MP3, and print either {"audioPath":"..."} or
// write directly to outputPath.
type MagnificMCPConfig struct {
	Command         string
	CacheDir        string
	VoiceID         int
	Model           string
	Stability       float64
	SimilarityBoost float64
	Speed           float64
	UseSpeakerBoost bool
	DebugLogPath    string // test/debug only; never contains credentials
}

type SpeechGenerator interface {
	Generate(ctx context.Context, text string) (string, error)
}

type MagnificMCPGenerator struct {
	config MagnificMCPConfig
}

func NewMagnificMCPGenerator(config MagnificMCPConfig) *MagnificMCPGenerator {
	if config.Model == "" {
		config.Model = "eleven_v3"
	}
	if config.Speed == 0 {
		config.Speed = 0.95
	}
	if config.SimilarityBoost == 0 {
		config.SimilarityBoost = 0.35
	}
	return &MagnificMCPGenerator{config: config}
}

// NewTelegramSpeechGenerator returns the configured Telegram narration speech
// generator, or nil when Telegram audio should be disabled. Currently the
// production Telegram path supports Magnific MCP because it returns MP3 files
// that Telegram can upload after text fallback has already been sent.
func NewTelegramSpeechGenerator(config *domain.Config, cacheBase string) SpeechGenerator {
	if config == nil || !config.TTS.Enabled || config.TTS.Provider != domain.TTSProviderMagnificMCP {
		return nil
	}
	cacheDir := config.TTS.CacheDir
	if cacheDir == "" && cacheBase != "" {
		cacheDir = filepath.Join(cacheBase, "tts-cache")
	}
	return NewMagnificMCPGenerator(MagnificMCPConfig{
		Command:         config.TTS.MagnificMCPCommand,
		CacheDir:        cacheDir,
		VoiceID:         config.TTS.MagnificVoiceID,
		Model:           config.TTS.Model,
		Stability:       config.TTS.MagnificStability,
		SimilarityBoost: config.TTS.MagnificSimilarityBoost,
		Speed:           config.TTS.Speed,
		UseSpeakerBoost: config.TTS.MagnificUseSpeakerBoost,
	})
}

func (g *MagnificMCPGenerator) Generate(ctx context.Context, text string) (string, error) {
	if g == nil {
		return "", fmt.Errorf("Magnific MCP TTS generator is not configured")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil
	}
	if strings.TrimSpace(g.config.Command) == "" {
		return "", fmt.Errorf("Magnific MCP TTS command is not configured")
	}
	if g.config.VoiceID == 0 {
		return "", fmt.Errorf("Magnific MCP TTS voice id is not configured")
	}
	cacheDir := g.config.CacheDir
	if cacheDir == "" {
		cacheDir = filepath.Join(os.TempDir(), "thaimaturgy-tts-cache")
	}
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return "", err
	}
	out := filepath.Join(cacheDir, g.cacheKey(text)+".mp3")
	if st, err := os.Stat(out); err == nil && !st.IsDir() && st.Size() > 0 {
		return out, nil
	}
	payload := map[string]any{
		"text":            text,
		"outputPath":      out,
		"voiceId":         g.config.VoiceID,
		"model":           g.config.Model,
		"stability":       g.config.Stability,
		"similarityBoost": g.config.SimilarityBoost,
		"speed":           g.config.Speed,
		"useSpeakerBoost": g.config.UseSpeakerBoost,
	}
	if g.config.DebugLogPath != "" {
		payload["debugLogPath"] = g.config.DebugLogPath
	}
	input, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", g.config.Command)
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("Magnific MCP TTS command failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if p := strings.TrimSpace(stdout.String()); p != "" {
		var res struct {
			AudioPath string `json:"audioPath"`
		}
		if json.Unmarshal([]byte(p), &res) == nil && strings.TrimSpace(res.AudioPath) != "" {
			out = res.AudioPath
		}
	}
	st, err := os.Stat(out)
	if err != nil {
		return "", fmt.Errorf("Magnific MCP TTS did not write audio: %w", err)
	}
	if st.IsDir() || st.Size() == 0 {
		return "", fmt.Errorf("Magnific MCP TTS wrote an empty/non-file audio path: %s", out)
	}
	return out, nil
}

func (g *MagnificMCPGenerator) cacheKey(text string) string {
	h := sha256.New()
	fmt.Fprintf(h, "magnific-mcp\x00%s\x00%d\x00%.4f\x00%.4f\x00%.4f\x00%t\x00%s", g.config.Model, g.config.VoiceID, g.config.Stability, g.config.SimilarityBoost, g.config.Speed, g.config.UseSpeakerBoost, text)
	return hex.EncodeToString(h.Sum(nil))
}

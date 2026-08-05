package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

const (
	openAISpeechEndpoint = "https://api.openai.com/v1/audio/speech"
	maxSpeechTextLength  = 4096
)

type fileSpeechRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	Speed          float64 `json:"speed,omitempty"`
	ResponseFormat string  `json:"response_format"`
}

// GenerateSpeechFile creates an MP3 speech file using OpenAI-compatible TTS.
// It is intentionally stdlib-only so Telegram voice generation works in both CGO
// and non-CGO builds; the older Client type is only for local audio playback.
func GenerateSpeechFile(ctx context.Context, apiKey string, config *domain.TTSConfig, text, outputPath string) error {
	return GenerateSpeechFileWithClient(ctx, http.DefaultClient, openAISpeechEndpoint, apiKey, config, text, outputPath)
}

func GenerateSpeechFileWithClient(ctx context.Context, client *http.Client, endpoint, apiKey string, config *domain.TTSConfig, text, outputPath string) error {
	if strings.TrimSpace(apiKey) == "" {
		return fmt.Errorf("missing OpenAI API key for TTS")
	}
	if config == nil || !config.Enabled {
		return fmt.Errorf("TTS is disabled")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("empty TTS input")
	}
	if len(text) > maxSpeechTextLength {
		text = text[:maxSpeechTextLength]
	}
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	if strings.TrimSpace(endpoint) == "" {
		endpoint = openAISpeechEndpoint
	}
	model := strings.TrimSpace(config.Model)
	if model == "" {
		model = "tts-1"
	}
	voice := string(config.Voice)
	if strings.TrimSpace(voice) == "" {
		voice = string(domain.TTSVoiceOnyx)
	}
	body, err := json.Marshal(fileSpeechRequest{
		Model:          model,
		Input:          text,
		Voice:          voice,
		Speed:          config.Speed,
		ResponseFormat: "mp3",
	})
	if err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("TTS API error (status %d): %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0700); err != nil {
		return err
	}
	tmp := outputPath + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, resp.Body)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	return os.Rename(tmp, outputPath)
}

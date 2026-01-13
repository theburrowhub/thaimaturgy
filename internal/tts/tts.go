package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

const (
	openAITTSEndpoint = "https://api.openai.com/v1/audio/speech"
	maxTextLength     = 4096
)

type Client struct {
	apiKey          string
	config          *domain.TTSConfig
	httpClient      *http.Client
	mu              sync.Mutex
	playing         bool
	speakerInit     bool
	currentStreamer beep.StreamSeekCloser
	done            chan struct{}
}

func NewClient(apiKey string, config *domain.TTSConfig) (*Client, error) {
	return &Client{
		apiKey: apiKey,
		config: config,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		done: make(chan struct{}),
	}, nil
}

func (c *Client) Close() error {
	c.Stop()
	return nil
}

func (c *Client) IsEnabled() bool {
	return c.config != nil && c.config.Enabled && c.apiKey != ""
}

func (c *Client) SetEnabled(enabled bool) {
	if c.config != nil {
		c.config.Enabled = enabled
	}
}

func (c *Client) Toggle() bool {
	if c.config != nil {
		c.config.Enabled = !c.config.Enabled
		if !c.config.Enabled {
			c.Stop()
		}
		return c.config.Enabled
	}
	return false
}

func (c *Client) IsPlaying() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.playing
}

func (c *Client) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.playing && c.currentStreamer != nil {
		speaker.Clear()
		c.currentStreamer.Close()
		c.currentStreamer = nil
		c.playing = false
	}
}

type ttsRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	Speed          float64 `json:"speed,omitempty"`
	ResponseFormat string  `json:"response_format"`
}

func (c *Client) Speak(ctx context.Context, text string) error {
	if !c.IsEnabled() {
		return nil
	}

	if len(text) == 0 {
		return nil
	}

	if len(text) > maxTextLength {
		text = text[:maxTextLength]
	}

	// Stop any current playback
	c.Stop()

	// Generate speech and get audio stream
	audioReader, err := c.generateSpeech(ctx, text)
	if err != nil {
		return err
	}

	return c.playAudioStream(audioReader)
}

func (c *Client) SpeakAsync(ctx context.Context, text string) {
	go func() {
		_ = c.Speak(ctx, text)
	}()
}

func (c *Client) generateSpeech(ctx context.Context, text string) (io.ReadCloser, error) {
	reqBody := ttsRequest{
		Model:          c.config.Model,
		Input:          text,
		Voice:          string(c.config.Voice),
		Speed:          c.config.Speed,
		ResponseFormat: "mp3",
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", openAITTSEndpoint, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("TTS API error (status %d): %s", resp.StatusCode, string(body))
	}

	return resp.Body, nil
}

func (c *Client) playAudioStream(audioReader io.ReadCloser) error {
	c.mu.Lock()
	if c.playing {
		c.mu.Unlock()
		audioReader.Close()
		return nil
	}
	c.playing = true
	c.mu.Unlock()

	// Decode MP3 stream
	streamer, format, err := mp3.Decode(audioReader)
	if err != nil {
		audioReader.Close()
		c.mu.Lock()
		c.playing = false
		c.mu.Unlock()
		return fmt.Errorf("failed to decode MP3: %w", err)
	}

	c.mu.Lock()
	c.currentStreamer = streamer
	c.mu.Unlock()

	// Initialize speaker if not already done
	if !c.speakerInit {
		if err := speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10)); err != nil {
			streamer.Close()
			c.mu.Lock()
			c.playing = false
			c.currentStreamer = nil
			c.mu.Unlock()
			return fmt.Errorf("failed to initialize speaker: %w", err)
		}
		c.speakerInit = true
	}

	// Create done channel for this playback
	done := make(chan struct{})

	// Play audio
	speaker.Play(beep.Seq(streamer, beep.Callback(func() {
		close(done)
	})))

	// Wait for playback to complete
	<-done

	c.mu.Lock()
	c.playing = false
	c.currentStreamer = nil
	c.mu.Unlock()

	return nil
}

func (c *Client) GetVoiceName() string {
	if c.config == nil {
		return "none"
	}
	return string(c.config.Voice)
}

func (c *Client) SetVoice(voice domain.TTSVoice) {
	if c.config != nil {
		c.config.Voice = voice
	}
}

var AvailableVoices = []domain.TTSVoice{
	domain.TTSVoiceAlloy,
	domain.TTSVoiceEcho,
	domain.TTSVoiceFable,
	domain.TTSVoiceOnyx,
	domain.TTSVoiceNova,
	domain.TTSVoiceShimmer,
}

// VoiceDescriptions provides descriptions for each voice
var VoiceDescriptions = map[domain.TTSVoice]string{
	domain.TTSVoiceAlloy:   "Neutral, balanced",
	domain.TTSVoiceEcho:    "Warm, conversational",
	domain.TTSVoiceFable:   "Expressive, British",
	domain.TTSVoiceOnyx:    "Deep, authoritative",
	domain.TTSVoiceNova:    "Friendly, upbeat",
	domain.TTSVoiceShimmer: "Clear, pleasant",
}

//go:build !cgo

package tts

import (
	"context"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// Client is a stub TTS client for builds without CGO.
// Audio playback is not available without CGO.
type Client struct {
	config *domain.TTSConfig
}

func NewClient(apiKey string, config *domain.TTSConfig) (*Client, error) {
	return &Client{config: config}, nil
}

func (c *Client) Close() error {
	return nil
}

func (c *Client) IsEnabled() bool {
	return false
}

func (c *Client) SetEnabled(enabled bool) {}

func (c *Client) Toggle() bool {
	return false
}

func (c *Client) IsPlaying() bool {
	return false
}

func (c *Client) Stop() {}

func (c *Client) Speak(ctx context.Context, text string) error {
	return nil
}

func (c *Client) SpeakAsync(ctx context.Context, text string) {}

func (c *Client) GetVoiceName() string {
	return "disabled"
}

func (c *Client) SetVoice(voice domain.TTSVoice) {}

var AvailableVoices = []domain.TTSVoice{}

var VoiceDescriptions = map[domain.TTSVoice]string{}

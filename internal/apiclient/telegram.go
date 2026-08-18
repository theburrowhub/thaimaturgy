package apiclient

import "context"

// TelegramStatus mirrors the server's Telegram-host status for a session.
type TelegramStatus struct {
	Hosting  bool   `json:"hosting"`
	Username string `json:"username"`
}

// TelegramStatus reports whether a session is currently hosted on Telegram.
func (c *Client) TelegramStatus(ctx context.Context, name string) (TelegramStatus, error) {
	var out TelegramStatus
	err := c.do(ctx, "GET", "/api/sessions/"+enc(name)+"/telegram", nil, &out)
	return out, err
}

// StartTelegramHost asks the server to host the session on Telegram using the
// server-configured bot token, returning the resulting status (with the bot's
// @username). The session must be in virtual-DM mode and the server config must
// carry a Telegram token.
func (c *Client) StartTelegramHost(ctx context.Context, name string) (TelegramStatus, error) {
	var out TelegramStatus
	err := c.do(ctx, "POST", "/api/sessions/"+enc(name)+"/telegram/start", nil, &out)
	return out, err
}

// StopTelegramHost stops a session's Telegram host (a no-op if it isn't hosting).
func (c *Client) StopTelegramHost(ctx context.Context, name string) (TelegramStatus, error) {
	var out TelegramStatus
	err := c.do(ctx, "POST", "/api/sessions/"+enc(name)+"/telegram/stop", nil, &out)
	return out, err
}

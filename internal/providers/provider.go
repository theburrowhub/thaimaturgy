package providers

import (
	"context"
	"encoding/json"

	"github.com/theburrowhub/thaimaturgy/internal/types"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	Role       Role           `json:"role"`
	Content    string         `json:"content"`
	Name       string         `json:"name,omitempty"`
	ToolCalls  []ToolCallInfo `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	// Images attaches inline image inputs to a user message for multimodal
	// (vision) models. Encoded per-provider at request time.
	Images []ImageData `json:"-"`
}

// ImageData is an inline image attached to a message for vision models.
type ImageData struct {
	MediaType string // e.g. "image/png", "image/jpeg"
	Data      []byte
}

type ToolCallInfo struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ChatRequest struct {
	Messages    []Message    `json:"messages"`
	Tools       []types.Tool `json:"tools,omitempty"`
	Model       string       `json:"model"`
	Temperature float64      `json:"temperature"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
}

type ChatResponse struct {
	Content      string         `json:"content"`
	ToolCalls    []ToolCallInfo `json:"tool_calls,omitempty"`
	FinishReason string         `json:"finish_reason"`
	Usage        Usage          `json:"usage"`
	Model        string         `json:"model"`
	Latency      int64          `json:"latency_ms"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type Provider interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	Name() string
	SupportsTools() bool
	// SupportsVision reports whether the backend accepts inline image inputs
	// (multimodal). Text-only backends (e.g. the Claude CLI) return false.
	SupportsVision() bool
}

func ConvertToolCallToTypesFormat(tc ToolCallInfo) types.ToolCall {
	return types.ToolCall{
		ID:        tc.ID,
		Name:      tc.Function.Name,
		Arguments: json.RawMessage(tc.Function.Arguments),
	}
}

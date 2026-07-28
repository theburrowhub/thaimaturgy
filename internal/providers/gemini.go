package providers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const geminiBaseURL = "https://generativelanguage.googleapis.com/v1beta"

// GeminiProvider talks to Google's Generative Language API. It authenticates
// with an API key (query param) or an OAuth bearer token reused from a local
// gemini CLI / gcloud login.
type GeminiProvider struct {
	apiKey     string
	oauthToken string
	httpClient *http.Client
}

func NewGeminiProvider(apiKey string) *GeminiProvider {
	return &GeminiProvider{apiKey: apiKey, httpClient: &http.Client{Timeout: 120 * time.Second}}
}

// NewGeminiOAuthProvider authenticates with an OAuth access token.
func NewGeminiOAuthProvider(token string) *GeminiProvider {
	return &GeminiProvider{oauthToken: token, httpClient: &http.Client{Timeout: 120 * time.Second}}
}

func (p *GeminiProvider) Name() string        { return "gemini" }
func (p *GeminiProvider) SupportsTools() bool { return true }

type geminiRequest struct {
	Contents          []geminiContent  `json:"contents"`
	SystemInstruction *geminiContent   `json:"systemInstruction,omitempty"`
	Tools             []geminiTool     `json:"tools,omitempty"`
	GenerationConfig  *geminiGenConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string                  `json:"text,omitempty"`
	InlineData       *geminiInlineData       `json:"inlineData,omitempty"`
	FunctionCall     *geminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResponse `json:"functionResponse,omitempty"`
}

type geminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type geminiFunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

type geminiFunctionResponse struct {
	Name     string          `json:"name"`
	Response json.RawMessage `json:"response"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFunctionDecl `json:"functionDeclarations"`
}

type geminiFunctionDecl struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type geminiGenConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content      geminiContent `json:"content"`
		FinishReason string        `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

func (p *GeminiProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	start := time.Now()

	model := strings.TrimPrefix(req.Model, "models/")
	if model == "" {
		model = "gemini-2.5-flash"
	}

	body, err := json.Marshal(p.convertRequest(req))
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:generateContent", geminiBaseURL, model)
	if p.apiKey != "" {
		url += "?key=" + p.apiKey
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.oauthToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.oauthToken)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var gr geminiResponse
	if err := json.Unmarshal(respBody, &gr); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w (body: %s)", err, string(respBody))
	}
	if gr.Error != nil {
		return nil, fmt.Errorf("Gemini API error: %s (status: %s)", gr.Error.Message, gr.Error.Status)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d (body: %s)", resp.StatusCode, string(respBody))
	}

	return p.convertResponse(gr, time.Since(start).Milliseconds()), nil
}

func (p *GeminiProvider) convertRequest(req ChatRequest) geminiRequest {
	var out geminiRequest
	var sysText []string

	for _, msg := range req.Messages {
		switch msg.Role {
		case RoleSystem:
			if msg.Content != "" {
				sysText = append(sysText, msg.Content)
			}
		case RoleTool:
			name := msg.Name
			if name == "" {
				name = "tool"
			}
			respObj, _ := json.Marshal(map[string]any{"content": msg.Content})
			out.Contents = append(out.Contents, geminiContent{
				Role:  "user",
				Parts: []geminiPart{{FunctionResponse: &geminiFunctionResponse{Name: name, Response: respObj}}},
			})
		case RoleAssistant:
			var parts []geminiPart
			if msg.Content != "" {
				parts = append(parts, geminiPart{Text: msg.Content})
			}
			for _, tc := range msg.ToolCalls {
				parts = append(parts, geminiPart{FunctionCall: &geminiFunctionCall{
					Name: tc.Function.Name,
					Args: json.RawMessage(tc.Function.Arguments),
				}})
			}
			if len(parts) == 0 {
				parts = []geminiPart{{Text: ""}}
			}
			out.Contents = append(out.Contents, geminiContent{Role: "model", Parts: parts})
		default: // user
			var parts []geminiPart
			if msg.Content != "" {
				parts = append(parts, geminiPart{Text: msg.Content})
			}
			for _, img := range msg.Images {
				parts = append(parts, geminiPart{InlineData: &geminiInlineData{
					MimeType: img.MediaType,
					Data:     base64.StdEncoding.EncodeToString(img.Data),
				}})
			}
			if len(parts) == 0 {
				parts = []geminiPart{{Text: ""}}
			}
			out.Contents = append(out.Contents, geminiContent{Role: "user", Parts: parts})
		}
	}

	if len(sysText) > 0 {
		out.SystemInstruction = &geminiContent{Parts: []geminiPart{{Text: strings.Join(sysText, "\n\n")}}}
	}
	if len(req.Tools) > 0 {
		decls := make([]geminiFunctionDecl, len(req.Tools))
		for i, t := range req.Tools {
			decls[i] = geminiFunctionDecl{Name: t.Name, Description: t.Description, Parameters: t.Parameters}
		}
		out.Tools = []geminiTool{{FunctionDeclarations: decls}}
	}
	out.GenerationConfig = &geminiGenConfig{Temperature: req.Temperature, MaxOutputTokens: req.MaxTokens}
	return out
}

func (p *GeminiProvider) convertResponse(gr geminiResponse, latencyMs int64) *ChatResponse {
	var content string
	var toolCalls []ToolCallInfo
	finish := "stop"

	if len(gr.Candidates) > 0 {
		cand := gr.Candidates[0]
		for i, part := range cand.Content.Parts {
			switch {
			case part.FunctionCall != nil:
				args := part.FunctionCall.Args
				if len(args) == 0 {
					args = json.RawMessage("{}")
				}
				toolCalls = append(toolCalls, ToolCallInfo{
					ID:   fmt.Sprintf("%s-%d", part.FunctionCall.Name, i),
					Type: "function",
					Function: FunctionCall{
						Name:      part.FunctionCall.Name,
						Arguments: string(args),
					},
				})
			case part.Text != "":
				content += part.Text
			}
		}
		if len(toolCalls) > 0 {
			finish = "tool_calls"
		} else if cand.FinishReason != "" && cand.FinishReason != "STOP" {
			finish = strings.ToLower(cand.FinishReason)
		}
	}

	return &ChatResponse{
		Content:      content,
		ToolCalls:    toolCalls,
		FinishReason: finish,
		Usage: Usage{
			PromptTokens:     gr.UsageMetadata.PromptTokenCount,
			CompletionTokens: gr.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      gr.UsageMetadata.TotalTokenCount,
		},
		Model:   "gemini",
		Latency: latencyMs,
	}
}

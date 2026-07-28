package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAnthropicTemperatureSelfHeal verifies that when a model rejects an
// explicit temperature as deprecated, the provider retries without it and
// omits it on subsequent calls.
func TestAnthropicTemperatureSelfHeal(t *testing.T) {
	var sawTemperature []bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)
		_, hasTemp := req["temperature"]
		sawTemperature = append(sawTemperature, hasTemp)

		if hasTemp {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"type":"invalid_request_error","message":"` + "`temperature`" + ` is deprecated for this model."}}`))
			return
		}
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"OK"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer srv.Close()

	orig := anthropicBaseURL
	anthropicBaseURL = srv.URL
	defer func() { anthropicBaseURL = orig }()

	p := NewAnthropicProvider("sk-test")

	resp, err := p.Chat(context.Background(), ChatRequest{Model: "claude-sonnet-5", Temperature: 0.8, MaxTokens: 16})
	if err != nil {
		t.Fatalf("first Chat should self-heal, got: %v", err)
	}
	if resp.Content != "OK" {
		t.Errorf("content = %q, want OK", resp.Content)
	}
	// First attempt sent temperature (rejected), retry omitted it (accepted).
	if len(sawTemperature) != 2 || !sawTemperature[0] || sawTemperature[1] {
		t.Errorf("expected [true,false] temperature presence, got %v", sawTemperature)
	}

	// A subsequent call must not send temperature again (no wasted retry).
	sawTemperature = nil
	if _, err := p.Chat(context.Background(), ChatRequest{Model: "claude-sonnet-5", Temperature: 0.8, MaxTokens: 16}); err != nil {
		t.Fatalf("second Chat: %v", err)
	}
	if len(sawTemperature) != 1 || sawTemperature[0] {
		t.Errorf("expected temperature omitted on one call, got %v", sawTemperature)
	}
}

// TestAnthropicModelFallback verifies the model fallback on rate_limit_error.
func TestAnthropicModelFallback(t *testing.T) {
	var models []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)
		model, _ := req["model"].(string)
		models = append(models, model)

		if model != anthropicFallbackModel {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"type":"rate_limit_error","message":"Error"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"OK"}],"stop_reason":"end_turn","usage":{}}`))
	}))
	defer srv.Close()

	orig := anthropicBaseURL
	anthropicBaseURL = srv.URL
	defer func() { anthropicBaseURL = orig }()

	p := NewAnthropicProvider("sk-test")
	resp, err := p.Chat(context.Background(), ChatRequest{Model: "claude-sonnet-5", MaxTokens: 16})
	if err != nil {
		t.Fatalf("expected fallback to succeed, got: %v", err)
	}
	if resp.Content != "OK" {
		t.Errorf("content = %q, want OK", resp.Content)
	}
	if len(models) != 2 || models[1] != anthropicFallbackModel {
		t.Errorf("expected fallback to %q, got calls %v", anthropicFallbackModel, models)
	}
	if !strings.HasPrefix(models[0], "claude-sonnet") {
		t.Errorf("first call should use the requested model, got %q", models[0])
	}
}

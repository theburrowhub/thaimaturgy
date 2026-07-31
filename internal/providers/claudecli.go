package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// ClaudeCLIProvider runs inference through the official Claude Code CLI in print
// mode (`claude -p`) instead of calling the API directly. This is the sanctioned
// way to use a Claude subscription: the official client authenticates and bills
// under its own terms — no API impersonation, no reused tokens.
//
// It is a text backend: SupportsTools reports false, because tool-calling with
// this backend is delegated to Claude Code itself via MCP (see the oracle's CLI
// path), not driven through this Provider.Chat interface. Image inputs are not
// supported here (print mode takes images as files via the Read tool, not inline
// base64), so requests carrying images return an error and callers degrade.
type ClaudeCLIProvider struct {
	bin string
}

// NewClaudeCLIProvider builds a provider that shells out to the given `claude`
// binary path.
func NewClaudeCLIProvider(bin string) *ClaudeCLIProvider {
	return &ClaudeCLIProvider{bin: bin}
}

func (p *ClaudeCLIProvider) Name() string         { return "claude-cli" }
func (p *ClaudeCLIProvider) SupportsTools() bool  { return false }
func (p *ClaudeCLIProvider) SupportsVision() bool { return false }

// claudeCLIResult is the shape of `claude -p --output-format json` output that we
// consume; the CLI includes much more, but this is all we need.
type claudeCLIResult struct {
	IsError bool   `json:"is_error"`
	Result  string `json:"result"`
	Subtype string `json:"subtype"`
}

func (p *ClaudeCLIProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// This backend is text-only (SupportsVision is false); any inline images are
	// ignored rather than fatal, so callers that don't check the capability still
	// get a text answer instead of an error.

	var system strings.Builder
	var user strings.Builder
	for _, m := range req.Messages {
		switch m.Role {
		case RoleSystem:
			if m.Content != "" {
				if system.Len() > 0 {
					system.WriteString("\n\n")
				}
				system.WriteString(m.Content)
			}
		default: // user / assistant / tool → fold into the prompt text
			if m.Content != "" {
				if user.Len() > 0 {
					user.WriteString("\n\n")
				}
				user.WriteString(m.Content)
			}
		}
	}

	args := []string{
		"-p",
		"--output-format", "json",
		"--permission-mode", "bypassPermissions",
		// Keep it a pure text generation: don't let Claude Code invoke its agentic
		// tools (file edits, shell, web) while producing our answer.
		"--disallowed-tools", "Bash", "Edit", "Write", "Read", "WebSearch", "WebFetch",
	}
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
	if s := strings.TrimSpace(system.String()); s != "" {
		// Replace Claude Code's default agent prompt with ours for cleaner, on-task
		// generation (the CLI still handles auth regardless of the system prompt).
		args = append(args, "--system-prompt", s)
	}

	cmd := exec.CommandContext(ctx, p.bin, args...)
	cmd.Stdin = strings.NewReader(user.String())
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("claude CLI failed: %s", msg)
	}

	var out claudeCLIResult
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return nil, fmt.Errorf("claude CLI: could not parse output: %w", err)
	}
	if out.IsError {
		detail := out.Result
		if detail == "" {
			detail = out.Subtype
		}
		return nil, fmt.Errorf("claude CLI returned an error: %s", detail)
	}

	return &ChatResponse{
		Content:      out.Result,
		FinishReason: "stop",
		Model:        req.Model,
	}, nil
}

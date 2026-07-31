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

	text, err := p.run(ctx, args, user.String())
	if err != nil {
		return nil, err
	}
	return &ChatResponse{Content: text, FinishReason: "stop", Model: req.Model}, nil
}

// RunWithMCP runs `claude -p` with an MCP server configuration and a set of
// pre-approved tool names, letting Claude Code drive the tool-calling loop
// itself (calling our MCP-exposed tools). It returns the final assistant text.
// Used by the oracle for the CLI backend, where tool-calling can't go through the
// plain Chat interface.
func (p *ClaudeCLIProvider) RunWithMCP(ctx context.Context, model, system, prompt, mcpConfigPath string, allowedTools []string) (string, error) {
	args := []string{
		"-p",
		"--output-format", "json",
		"--permission-mode", "bypassPermissions",
		"--strict-mcp-config",
		"--mcp-config", mcpConfigPath,
		// Restrict Claude Code's own agentic tools so it uses only our MCP tools.
		"--disallowed-tools", "Bash", "Edit", "Write", "Read", "WebSearch", "WebFetch",
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	if s := strings.TrimSpace(system); s != "" {
		args = append(args, "--system-prompt", s)
	}
	if len(allowedTools) > 0 {
		// Variadic flag; place last so the tool names aren't mistaken for other args.
		args = append(args, "--allowedTools")
		args = append(args, allowedTools...)
	}
	return p.run(ctx, args, prompt)
}

// run executes the claude binary with args, feeding stdin, and returns the parsed
// result text.
func (p *ClaudeCLIProvider) run(ctx context.Context, args []string, stdin string) (string, error) {
	cmd := exec.CommandContext(ctx, p.bin, args...)
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("claude CLI failed: %s", msg)
	}
	var out claudeCLIResult
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return "", fmt.Errorf("claude CLI: could not parse output: %w", err)
	}
	if out.IsError {
		detail := out.Result
		if detail == "" {
			detail = out.Subtype
		}
		return "", fmt.Errorf("claude CLI returned an error: %s", detail)
	}
	return out.Result, nil
}

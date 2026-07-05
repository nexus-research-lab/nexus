package websocket

import (
	"errors"
	"strings"
	"testing"
)

func TestChatErrorDetailExplainsRuntimeFailures(t *testing.T) {
	tests := []struct {
		name  string
		err   string
		wants []string
	}{
		{
			name: "missing Claude command",
			err:  `client: backend executable "process backend" not found: process: cli executable "claude.exe" not found`,
			wants: []string{
				"Claude Code",
				"NEXUS_CLAUDE_COMMAND_PATH",
				"command -v claude",
				"claude doctor",
				"brew install --cask claude-code",
				"winget install Anthropic.ClaudeCode",
				"~/.local/bin/claude",
				"/opt/homebrew/bin/claude",
			},
		},
		{
			name:  "missing nxs command",
			err:   `client: backend executable "process backend" not found: process: cli executable "nxs" not found`,
			wants: []string{"nxs runtime", "sidecar", "NEXUS_NXS_COMMAND_PATH"},
		},
		{
			name:  "provider config",
			err:   "provider=default 配置不完整: auth_token, model",
			wants: []string{"Provider", "auth_token"},
		},
		{
			name:  "unsupported responses api",
			err:   "provider=ri 的 api_format=responses 暂不可用于 Agent runtime",
			wants: []string{"Responses API", "Agent runtime"},
		},
		{
			name:  "provider overload",
			err:   `client: runtime startup failed: provider_error=server_overload stderr="API error: 529 {\"type\":\"overloaded_error\"}": context deadline exceeded`,
			wants: []string{"模型请求暂时受限", "LLM Provider"},
		},
		{
			name:  "provider rate limit",
			err:   `client: runtime startup failed: provider_error=rate_limit stderr="API error: 429 rate_limit_error": context deadline exceeded`,
			wants: []string{"模型请求暂时受限", "LLM Provider"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			message := chatErrorDetail(errors.New(test.err))
			for _, want := range test.wants {
				if !strings.Contains(message, want) {
					t.Fatalf("chatErrorDetail() = %q, want substring %q", message, want)
				}
			}
		})
	}
}

func TestNewGatewayErrorEventUsesRoundIDAsCause(t *testing.T) {
	event := (&Handler{}).newGatewayErrorEvent(
		"agent:agent-1:ws:dm:session-1",
		"chat_error",
		"启动失败",
		map[string]any{
			"type":     "chat",
			"round_id": "round-1",
		},
	)
	if event.CausedBy != "round-1" {
		t.Fatalf("error caused_by = %q, want round-1", event.CausedBy)
	}
	if got := event.Data["round_id"]; got != "round-1" {
		t.Fatalf("error data.round_id = %#v, want round-1", got)
	}
}

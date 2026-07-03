package websocket

import (
	"errors"
	"strings"
	"testing"
)

func TestChatErrorDetailExplainsMissingClaudeCommand(t *testing.T) {
	message := chatErrorDetail(errors.New(`client: backend executable "process backend" not found: process: cli executable "claude.exe" not found`))
	if !strings.Contains(message, "Claude Code") ||
		!strings.Contains(message, "NEXUS_CLAUDE_COMMAND_PATH") ||
		!strings.Contains(message, "command -v claude") ||
		!strings.Contains(message, "claude doctor") ||
		!strings.Contains(message, "brew install --cask claude-code") ||
		!strings.Contains(message, "winget install Anthropic.ClaudeCode") ||
		!strings.Contains(message, "~/.local/bin/claude") ||
		!strings.Contains(message, "/opt/homebrew/bin/claude") {
		t.Fatalf("缺少 Claude Code 时应返回可执行提示: %q", message)
	}
}

func TestChatErrorDetailExplainsMissingNXSCommand(t *testing.T) {
	message := chatErrorDetail(errors.New(`client: backend executable "process backend" not found: process: cli executable "nxs" not found`))
	if !strings.Contains(message, "nxs runtime") ||
		!strings.Contains(message, "sidecar") ||
		!strings.Contains(message, "NEXUS_NXS_COMMAND_PATH") {
		t.Fatalf("缺少 nxs 时应返回 nxs 可执行提示: %q", message)
	}
}

func TestChatErrorDetailExplainsProviderConfig(t *testing.T) {
	message := chatErrorDetail(errors.New("provider=default 配置不完整: auth_token, model"))
	if !strings.Contains(message, "Provider") || !strings.Contains(message, "auth_token") {
		t.Fatalf("Provider 配置错误时应返回配置提示: %q", message)
	}
}

func TestChatErrorDetailExplainsUnsupportedResponsesAPIFormat(t *testing.T) {
	message := chatErrorDetail(errors.New("provider=ri 的 api_format=responses 暂不可用于 Agent runtime"))
	if !strings.Contains(message, "Responses API") || !strings.Contains(message, "Agent runtime") {
		t.Fatalf("Responses API 不支持时应返回可操作提示: %q", message)
	}
}

func TestChatErrorDetailExplainsProviderOverload(t *testing.T) {
	message := chatErrorDetail(errors.New(`client: runtime startup failed: provider_error=server_overload stderr="API error: 529 {\"type\":\"overloaded_error\"}": context deadline exceeded`))
	if !strings.Contains(message, "模型请求暂时受限") || !strings.Contains(message, "LLM Provider") {
		t.Fatalf("Provider 过载时应返回受限提示: %q", message)
	}
}

func TestChatErrorDetailExplainsProviderRateLimit(t *testing.T) {
	message := chatErrorDetail(errors.New(`client: runtime startup failed: provider_error=rate_limit stderr="API error: 429 rate_limit_error": context deadline exceeded`))
	if !strings.Contains(message, "模型请求暂时受限") || !strings.Contains(message, "LLM Provider") {
		t.Fatalf("Provider 限流时应返回受限提示: %q", message)
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

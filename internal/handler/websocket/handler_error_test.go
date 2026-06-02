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

func TestChatErrorDetailExplainsProviderConfig(t *testing.T) {
	message := chatErrorDetail(errors.New("provider=default 配置不完整: auth_token, model"))
	if !strings.Contains(message, "Provider") || !strings.Contains(message, "auth_token") {
		t.Fatalf("Provider 配置错误时应返回配置提示: %q", message)
	}
}

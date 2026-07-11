package websocket

import (
	"context"
	"strings"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (h *Handler) sendGatewayError(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	sessionKey string,
	errorType string,
	err error,
	details map[string]any,
) {
	message := h.errorEventDetail(errorType, err)
	_ = sender.SendEvent(ctx, h.newGatewayErrorEvent(sessionKey, errorType, message, details))
}

func (h *Handler) errorEventDetail(errorType string, err error) string {
	if err == nil {
		return "请求失败"
	}
	message := strings.TrimSpace(err.Error())
	switch errorType {
	case "validation_error", "invalid_room_subscription", "invalid_workspace_subscription":
		if handlershared.IsClientMessageText(message) {
			return message
		}
		return "请求参数错误"
	case "invalid_session_key":
		return "session_key 不合法"
	case "permission_request_not_found":
		return "未找到待确认的权限请求"
	case "chat_error":
		return chatErrorDetail(err)
	default:
		if handlershared.IsClientMessageError(err) || handlershared.IsStructuredSessionKeyError(err) {
			return message
		}
		return "服务内部错误"
	}
}

func chatErrorDetail(err error) string {
	if err == nil {
		return "Agent 启动失败，请检查运行时配置后重试。"
	}
	message := strings.TrimSpace(err.Error())
	switch {
	case strings.Contains(message, "api_format=responses") &&
		strings.Contains(message, "暂不可用于 Agent runtime"):
		return "当前 Provider 使用 Responses API，暂不可用于 Agent runtime。请切换到 Agent runtime 支持的 Provider/API 格式后重试。"
	case isProviderCapacityError(message):
		return "模型请求暂时受限，当前 LLM Provider 返回限流或过载。请稍后重试，或临时切换到可用 Provider/模型。"
	case strings.Contains(message, "nxs"):
		return "未找到 nxs runtime，Agent 无法启动。打包版 Nexus 应由桌面 sidecar 注入随包 nxs 路径；开发环境请设置 NEXUS_NXS_COMMAND_PATH 指向本地 nxs，或在 Settings 将 Agent Runtime 切回 Claude。"
	case strings.Contains(message, "cli executable") ||
		strings.Contains(message, "claude.exe") ||
		strings.Contains(message, "claude.cmd") ||
		strings.Contains(message, "claude.ps1"):
		return "未找到 Claude Code 命令，Agent 无法启动。请先排查：macOS/Linux/WSL 运行 `command -v claude && claude --version && claude doctor`，Windows PowerShell 运行 `where.exe claude; claude --version; claude doctor`。如果尚未安装，可选择官方安装命令：macOS/Linux/WSL `curl -fsSL https://claude.ai/install.sh | bash`，macOS Homebrew `brew install --cask claude-code`，Windows PowerShell `irm https://claude.ai/install.ps1 | iex`，Windows WinGet `winget install Anthropic.ClaudeCode`，npm `npm install -g @anthropic-ai/claude-code`。安装后运行 `claude` 完成登录；如果终端可用但 Nexus 仍找不到，请设置 NEXUS_CLAUDE_COMMAND_PATH 指向可执行文件，例如 `~/.local/bin/claude`、`/opt/homebrew/bin/claude` 或 `claude.cmd`。"
	case strings.Contains(message, "LLM Provider") ||
		strings.Contains(message, "provider=") ||
		strings.Contains(message, "Provider"):
		return "Agent 运行时 Provider 配置不可用。请到 Settings 检查默认 LLM Provider 是否已启用，并确认 auth_token、base_url、model 已填写完整。"
	default:
		if handlershared.IsClientMessageError(err) || handlershared.IsStructuredSessionKeyError(err) {
			return message
		}
		return "Agent 启动失败，请检查 Claude Code、Provider 配置和日志后重试。"
	}
}

func isProviderCapacityError(message string) bool {
	normalized := strings.ToLower(message)
	return strings.Contains(normalized, "provider_error=server_overload") ||
		strings.Contains(normalized, "provider_error=rate_limit") ||
		strings.Contains(normalized, "overloaded_error") ||
		strings.Contains(normalized, "rate_limit_error") ||
		strings.Contains(normalized, "repeated 529") ||
		strings.Contains(normalized, " 529 ") ||
		strings.Contains(normalized, " 429 ")
}

func (h *Handler) newGatewayErrorEvent(
	sessionKey string,
	errorType string,
	message string,
	details map[string]any,
) protocol.EventMessage {
	data := map[string]any{
		"message":    message,
		"error_type": errorType,
	}
	for key, value := range details {
		data[key] = value
	}
	event := protocol.NewEvent(protocol.EventTypeError, data)
	event.SessionKey = sessionKey
	if roundID := strings.TrimSpace(handlershared.StringValue(details["round_id"])); roundID != "" {
		event.RoundID = roundID
	}
	return event
}

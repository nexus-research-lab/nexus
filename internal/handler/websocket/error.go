// INPUT: WebSocket gateway 的内部错误、命令类型与可选关联身份。
// OUTPUT: 脱敏 message、稳定 failure_code 和精确 round/request 字段的 error event。
// POS: Conversation gateway 错误到公开 wire 的唯一分类边界。
package websocket

import (
	"context"
	"errors"
	"strings"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	subscriptionsvc "github.com/nexus-research-lab/nexus/internal/service/subscription"
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
	if details == nil {
		details = make(map[string]any)
	}
	details["failure_code"] = gatewayFailureCode(errorType, err)
	_ = sender.SendEvent(ctx, h.newGatewayErrorEvent(sessionKey, errorType, message, details))
}

func gatewayFailureCode(errorType string, err error) protocol.ConversationFailureCode {
	switch errorType {
	case "validation_error", "invalid_room_subscription", "invalid_workspace_subscription", "invalid_session_key", "unknown_message_type":
		return protocol.ConversationFailureValidationFailed
	case "permission_request_not_found":
		return protocol.ConversationFailurePermissionNotSent
	case "chat_error", "command_catalog_error":
		message := ""
		if err != nil {
			message = strings.ToLower(strings.TrimSpace(err.Error()))
		}
		switch {
		case errors.Is(err, subscriptionsvc.ErrQuotaExceeded):
			return protocol.ConversationFailureUsageLimited
		case protocol.IsProviderContentFilterError(message):
			return protocol.ConversationFailureSafetyRejected
		case strings.Contains(message, "authentication_error"),
			strings.Contains(message, "invalid api key"),
			strings.Contains(message, "未配置默认模型"),
			strings.Contains(message, "配置不完整"):
			return protocol.ConversationFailureProviderConfiguration
		default:
			return protocol.ConversationFailureProviderUnavailable
		}
	default:
		return protocol.ConversationFailureRequestRejected
	}
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
	case "chat_error", "command_catalog_error":
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
		return "Agent 启动失败。请先到设置 → 运行时确认 Agent 内核可用，再到设置 → 供应商测试当前默认模型。"
	}
	if message, ok := protocol.ClientErrorMessage(err); ok {
		return message
	}
	message := strings.TrimSpace(err.Error())
	normalized := strings.ToLower(message)
	switch {
	case strings.Contains(message, "api_format=responses") &&
		strings.Contains(message, "暂不可用于") &&
		strings.Contains(message, "Agent runtime"):
		return "当前 Provider 使用 Responses API，只支持 nxs Agent runtime。请在 Settings 将 Agent Runtime 切换为 nxs 后重试。"
	case isProviderCapacityError(message):
		return "模型请求暂时受限，当前 LLM Provider 返回限流或过载。请稍后重试，或临时切换到可用 Provider/模型。"
	case strings.Contains(normalized, "failed to authenticate") ||
		strings.Contains(normalized, "authentication_error") ||
		strings.Contains(normalized, "invalid api key") ||
		strings.Contains(normalized, "api error: 401"):
		return "模型鉴权失败。请到设置 → 供应商重新填写 API Key，然后执行“保存并测试”。"
	case strings.Contains(message, "未配置默认模型") ||
		strings.Contains(message, "缺少 model") ||
		strings.Contains(message, "模型不存在"):
		return "供应商已配置，但还没有可用的默认对话模型。请到设置 → 常规选择一个已启用模型，或到设置 → 供应商测试模型。"
	case strings.Contains(message, "配置不完整"):
		return "供应商配置不完整。请到设置 → 供应商检查 API Key、Base URL 和模型后重试。"
	case strings.Contains(message, "nxs"):
		return "未找到 nxs runtime，Agent 无法启动。打包版 Nexus 应由桌面 sidecar 注入随包 nxs 路径；开发环境请设置 NEXUS_NXS_COMMAND_PATH 指向本地 nxs，或在设置 → 运行时将 Agent 内核切回 Claude。"
	case strings.Contains(message, "cli executable") ||
		strings.Contains(message, "claude.exe") ||
		strings.Contains(message, "claude.cmd") ||
		strings.Contains(message, "claude.ps1"):
		return "未找到 Claude Code 命令，Agent 无法启动。请先排查：macOS/Linux/WSL 运行 `command -v claude && claude --version && claude doctor`，Windows PowerShell 运行 `where.exe claude; claude --version; claude doctor`。如果尚未安装，可选择官方安装命令：macOS/Linux/WSL `curl -fsSL https://claude.ai/install.sh | bash`，macOS Homebrew `brew install --cask claude-code`，Windows PowerShell `irm https://claude.ai/install.ps1 | iex`，Windows WinGet `winget install Anthropic.ClaudeCode`，npm `npm install -g @anthropic-ai/claude-code`。安装后运行 `claude` 完成登录；如果终端可用但 Nexus 仍找不到，请设置 NEXUS_CLAUDE_COMMAND_PATH 指向可执行文件，例如 `~/.local/bin/claude`、`/opt/homebrew/bin/claude` 或 `claude.cmd`。"
	case strings.Contains(message, "LLM Provider") ||
		strings.Contains(message, "provider=") ||
		strings.Contains(message, "Provider"):
		return "Agent 使用的模型配置不可用。请到设置 → 供应商确认默认供应商已启用，并检查 API Key、Base URL 和模型。"
	default:
		if handlershared.IsClientMessageError(err) || handlershared.IsStructuredSessionKeyError(err) {
			return message
		}
		return "Agent 启动失败。请先到设置 → 运行时确认 Agent 内核可用，再到设置 → 供应商测试当前默认模型；仍失败时查看运行日志。"
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
		"message":      message,
		"error_type":   errorType,
		"failure_code": gatewayFailureCode(errorType, nil),
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

package semantic

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/argx"
)

var visibleResultIntentKeywords = []string{
	"告诉我", "通知我", "发给我", "发我", "给我发", "推给我", "回给我",
	"发回来", "回到这里", "发到这里", "发到当前会话", "回到当前会话",
	"send me", "tell me", "notify me", "let me know", "send back", "reply here",
}

var visibleResultOptOutKeywords = []string{
	"不用告诉我", "不要告诉我", "无需告诉我",
	"不用通知", "不要通知", "无需通知",
	"不用发给我", "不要发给我", "无需发给我",
	"不用发回来", "不要发回来", "无需发回来",
	"不回传", "不要回传", "不用回传", "无需回传",
	"静默", "后台静默",
	"do not tell me", "don't tell me", "do not notify me", "don't notify me",
	"do not send me", "don't send me", "silent",
}

var currentConversationDependencyKeywords = []string{
	"当前对话", "这个对话", "本对话", "聊天记录", "历史消息", "当前上下文", "这个上下文",
	"这里的上下文", "当前 room", "当前room", "这个 room", "这个room", "本 room", "本room", "公区",
	"current conversation", "this conversation", "chat history", "current context", "this context",
	"current room", "this room",
}

// ApplyVisibleResultDefaults 把“独立重任务 + 明确要看见结果”的创建请求默认成临时执行并回投当前会话。
// 这类任务通常不该复用当前聊天历史，避免每天新闻/日报把会话上下文越滚越脏。
func ApplyVisibleResultDefaults(args map[string]any, sctx contract.ServerContext) map[string]any {
	if !canDefaultVisibleResult(args, sctx) {
		return args
	}
	if strings.TrimSpace(argx.String(args, "execution_mode")) == "" {
		args["execution_mode"] = "temporary"
	}
	executionMode := strings.TrimSpace(argx.String(args, "execution_mode"))
	if strings.TrimSpace(argx.String(args, "reply_mode")) == "" {
		switch executionMode {
		case "existing":
			args["reply_mode"] = "execution"
		case "temporary", "dedicated":
			args["reply_mode"] = "selected"
		}
	}
	if strings.TrimSpace(argx.String(args, "reply_mode")) == "selected" &&
		strings.TrimSpace(argx.String(args, "selected_reply_session_key")) == "" {
		args["selected_reply_session_key"] = sctx.CurrentSessionKey
	}
	return args
}

func canDefaultVisibleResult(args map[string]any, sctx contract.ServerContext) bool {
	if args == nil || strings.TrimSpace(argx.String(args, "execution_kind")) == "script" {
		return false
	}
	if strings.TrimSpace(sctx.CurrentSessionKey) == "" || currentSessionKeyCanDeliverToExternalChannel(sctx.CurrentSessionKey) {
		return false
	}
	executionMode := strings.TrimSpace(argx.String(args, "execution_mode"))
	if executionMode == "main" {
		return false
	}
	replyMode := strings.TrimSpace(argx.String(args, "reply_mode"))
	if replyMode != "" && replyMode != "execution" && replyMode != "selected" {
		return false
	}
	if !hasRunnableScheduleShape(args) {
		return false
	}
	text := defaultIntentText(args)
	if containsAnyKeyword(text, visibleResultOptOutKeywords) ||
		containsAnyKeyword(text, currentConversationDependencyKeywords) {
		return false
	}
	return containsAnyKeyword(text, visibleResultIntentKeywords)
}

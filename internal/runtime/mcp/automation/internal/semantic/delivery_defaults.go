package semantic

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/internal/argx"
)

// ApplyDeliveryFieldDefaults 根据显式投递字段推断 reply_mode。
// 这层只处理无歧义字段，避免模型漏填 reply_mode 时把用户的“改发到飞书群”
// 误报成空更新。
func ApplyDeliveryFieldDefaults(args map[string]any) map[string]any {
	if args == nil || strings.TrimSpace(argx.String(args, "reply_mode")) != "" {
		return args
	}
	if hasAgentReplyField(args) {
		args["reply_mode"] = "agent"
		return args
	}
	if hasChannelReplyField(args) {
		args["reply_mode"] = "channel"
		return args
	}
	if strings.TrimSpace(argx.String(args, "selected_reply_session_key")) != "" {
		args["reply_mode"] = "selected"
	}
	return args
}

// ApplySelectedReplyCurrentDefault 允许用户在当前内部会话里说“以后发到这里”。
// 外部 IM 群保持 reply_mode=channel 路径，避免 selected 语义混用。
func ApplySelectedReplyCurrentDefault(args map[string]any, sctx contract.ServerContext) map[string]any {
	if args == nil ||
		strings.TrimSpace(argx.String(args, "reply_mode")) != "selected" ||
		strings.TrimSpace(argx.String(args, "selected_reply_session_key")) != "" {
		return args
	}
	currentSessionKey := strings.TrimSpace(sctx.CurrentSessionKey)
	if currentSessionKey == "" || currentSessionKeyCanDeliverToExternalChannel(currentSessionKey) {
		return args
	}
	args["selected_reply_session_key"] = currentSessionKey
	return args
}

func hasAgentReplyField(args map[string]any) bool {
	return strings.TrimSpace(argx.String(args, "reply_agent_id")) != ""
}

func hasChannelReplyField(args map[string]any) bool {
	for _, key := range []string{
		"reply_session_key",
		"reply_channel", "delivery_channel",
		"reply_to", "delivery_to",
		"reply_account_id", "delivery_account_id",
		"reply_thread_id", "delivery_thread_id",
	} {
		if strings.TrimSpace(argx.String(args, key)) != "" {
			return true
		}
	}
	return false
}

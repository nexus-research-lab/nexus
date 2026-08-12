// INPUT: 模型表达的 reply_mode 与服务端签发的当前外部 IM session。
// OUTPUT: 移除模型可伪造的通道路由字段，由 Delivery 解析器绑定当前 session。
// POS: Automation MCP 当前 IM 回传的宿主所有权边界。
package semantic

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/argx"
)

var channelReplyTargetFields = []string{
	"reply_session_key",
	"selected_reply_session_key",
	"reply_channel", "delivery_channel",
	"reply_to", "delivery_to",
	"reply_account_id", "delivery_account_id",
	"reply_thread_id", "delivery_thread_id",
}

// BindCurrentExternalChannelReply 保证外部 IM 中的 channel 回传只能指向当前会话。
// 模型只决定是否回当前通道，不能提供或覆盖 channel/account/target/thread/session。
func BindCurrentExternalChannelReply(args map[string]any, sctx contract.ServerContext) map[string]any {
	if args == nil ||
		strings.TrimSpace(argx.String(args, "reply_mode")) != "channel" ||
		!currentSessionKeyCanDeliverToExternalChannel(sctx.CurrentSessionKey) {
		return args
	}
	for _, field := range channelReplyTargetFields {
		delete(args, field)
	}
	return args
}

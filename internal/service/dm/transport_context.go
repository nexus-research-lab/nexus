// INPUT: pairing 复核后的外部 DM 标记与当前结构化 Agent session。
// OUTPUT: 只含安全通道事实、不含任何投递标识的 runtime 隐藏上下文。
// POS: IM transport 身份到模型只读会话认知的投影边界。
package dm

import (
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

func (r *roundRunner) transportContextualInputs() []runtimectx.ContextualInputBlock {
	channel, chatType, ok := r.safeCurrentIMTransport()
	if !ok {
		return nil
	}
	content := fmt.Sprintf(
		`<nexus_transport_context transport="im" channel="%s" chat_type="%s" route_binding="host">
  <reply_rule>For tools that can return results to this conversation, express only deliver_result=true. Nexus supplies the current route; never provide or guess channel, account, target, thread, or session identifiers.</reply_rule>
</nexus_transport_context>`,
		channel,
		chatType,
	)
	return []runtimectx.ContextualInputBlock{
		runtimectx.NewContextualInputBlock(
			runtimectx.ContextualInputNameTransport,
			content,
			runtimectx.ContextualInputPriorityTransport,
			nil,
		),
	}
}

func (r *roundRunner) safeCurrentIMTransport() (string, string, bool) {
	if r == nil || !r.trustedExternalInteractive || r.agent == nil {
		return "", "", false
	}
	parsed := protocol.ParseSessionKey(strings.TrimSpace(r.sessionKey))
	if !parsed.IsStructured ||
		parsed.Kind != protocol.SessionKeyKindAgent ||
		strings.TrimSpace(parsed.AgentID) != strings.TrimSpace(r.agent.AgentID) ||
		protocol.NormalizeSessionChatType(parsed.ChatType) != protocol.RoomTypeDM ||
		strings.TrimSpace(parsed.Ref) == "" {
		return "", "", false
	}
	channel := protocol.NormalizeStoredChannelType(parsed.Channel)
	switch channel {
	case protocol.SessionChannelDiscord,
		protocol.SessionChannelTelegram,
		protocol.SessionChannelDingTalk,
		protocol.SessionChannelWeChat,
		protocol.SessionChannelWeixinPersonal,
		protocol.SessionChannelFeishu:
		return channel, protocol.RoomTypeDM, true
	default:
		return "", "", false
	}
}

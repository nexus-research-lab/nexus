// INPUT: DM session、root/source round 身份与用户输入。
// OUTPUT: 可持久化和广播的 DM 用户消息及事件封装。
// POS: DM 用户消息协议形状的领域构造入口。
package dm

import (
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// BuildUserRoundMarker 构建 DM 用户轮次标记消息。userMessageID 是后端 mint 的 durable id。
func BuildUserRoundMarker(
	sessionValue protocol.Session,
	roundID string,
	sourceRoundID string,
	userMessageID string,
	content string,
	deliveryPolicy protocol.ChatDeliveryPolicy,
	attachments []protocol.ChatAttachment,
) protocol.Message {
	messageValue := protocol.Message{
		"message_id":  strings.TrimSpace(userMessageID),
		"session_key": sessionValue.SessionKey,
		"agent_id":    sessionValue.AgentID,
		"round_id":    strings.TrimSpace(roundID),
		"role":        "user",
		"content":     strings.TrimSpace(content),
		"timestamp":   time.Now().UnixMilli(),
	}
	if sourceRoundID = strings.TrimSpace(sourceRoundID); sourceRoundID != "" && sourceRoundID != strings.TrimSpace(roundID) {
		messageValue["source_round_id"] = sourceRoundID
	}
	if strings.TrimSpace(string(deliveryPolicy)) != "" {
		messageValue["delivery_policy"] = string(protocol.NormalizeChatDeliveryPolicy(string(deliveryPolicy)))
	}
	if normalizedAttachments := protocol.NormalizeChatAttachments(attachments, sessionValue.AgentID); len(normalizedAttachments) > 0 {
		messageValue["attachments"] = normalizedAttachments
	}
	return messageValue
}

// WrapSessionMessageEvent 构建 DM 会话消息事件。roundID 为空时回退到消息自身的 round_id。
func WrapSessionMessageEvent(sessionValue protocol.Session, messageValue protocol.Message, deliveryMode string, roundID string) protocol.EventMessage {
	event := protocol.NewEvent(protocol.EventTypeMessage, messageValue)
	event.DeliveryMode = strings.TrimSpace(deliveryMode)
	event.SessionKey = sessionValue.SessionKey
	event.AgentID = sessionValue.AgentID
	event.MessageID = NormalizeString(messageValue["message_id"])
	event.RoundID = strings.TrimSpace(FirstNonEmpty(roundID, NormalizeString(messageValue["round_id"])))
	event.AgentRoundID = NormalizeString(messageValue["agent_round_id"])
	return event
}

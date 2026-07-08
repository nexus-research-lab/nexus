package dm

import (
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// BuildUserRoundMarker 构建 DM 用户轮次标记消息。userMessageID 是后端 mint 的 durable id。
func BuildUserRoundMarker(
	sessionValue protocol.Session,
	roundID string,
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
	if strings.TrimSpace(string(deliveryPolicy)) != "" {
		messageValue["delivery_policy"] = string(protocol.NormalizeChatDeliveryPolicy(string(deliveryPolicy)))
	}
	if normalizedAttachments := protocol.NormalizeChatAttachments(attachments, sessionValue.AgentID); len(normalizedAttachments) > 0 {
		messageValue["attachments"] = normalizedAttachments
	}
	return messageValue
}

// BuildGuidanceMessage 构建 DM 引导消息。
func BuildGuidanceMessage(
	sessionValue protocol.Session,
	targetRoundID string,
	sourceRoundID string,
	content string,
	timestamp int64,
) protocol.Message {
	return message.NewGuidedInputMessage(message.GuidedInputMessageInput{
		SessionKey:    sessionValue.SessionKey,
		AgentID:       sessionValue.AgentID,
		RoundID:       targetRoundID,
		SourceRoundID: sourceRoundID,
		Content:       content,
		SessionID:     StringPointerValue(sessionValue.SessionID),
		Timestamp:     timestamp,
	})
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

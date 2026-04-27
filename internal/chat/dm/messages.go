package dm

import (
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// BuildUserRoundMarker 构建 DM 用户轮次标记消息。
func BuildUserRoundMarker(
	sessionValue protocol.Session,
	roundID string,
	content string,
	deliveryPolicy protocol.ChatDeliveryPolicy,
) protocol.Message {
	messageValue := protocol.Message{
		"message_id":  strings.TrimSpace(roundID),
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

// WrapSessionMessageEvent 构建 DM 会话消息事件。
func WrapSessionMessageEvent(sessionValue protocol.Session, messageValue protocol.Message, deliveryMode string, causedBy string) protocol.EventMessage {
	event := protocol.NewEvent(protocol.EventTypeMessage, messageValue)
	event.DeliveryMode = strings.TrimSpace(deliveryMode)
	event.SessionKey = sessionValue.SessionKey
	event.AgentID = sessionValue.AgentID
	event.MessageID = NormalizeString(messageValue["message_id"])
	event.CausedBy = strings.TrimSpace(causedBy)
	return event
}

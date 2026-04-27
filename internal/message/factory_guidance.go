package message

import (
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const SystemMessageSubtypeGuidedInput = "guided_input"

// GuidedInputMessageInput 描述一条运行中 round 的用户引导消息。
type GuidedInputMessageInput struct {
	MessageID      string
	SessionKey     string
	RoomID         string
	ConversationID string
	AgentID        string
	RoundID        string
	SourceRoundID  string
	Content        string
	SessionID      string
	Timestamp      int64
}

// NewGuidedInputMessage 构造不会进入普通用户气泡的引导消息。
func NewGuidedInputMessage(input GuidedInputMessageInput) protocol.Message {
	timestamp := input.Timestamp
	if timestamp <= 0 {
		timestamp = time.Now().UnixMilli()
	}
	roundID := strings.TrimSpace(input.RoundID)
	sourceRoundID := strings.TrimSpace(input.SourceRoundID)
	messageID := firstGuidanceValue(
		strings.TrimSpace(input.MessageID),
		sourceRoundID,
		"guidance_"+roundID,
	)
	message := protocol.Message{
		"message_id":  messageID,
		"session_key": strings.TrimSpace(input.SessionKey),
		"agent_id":    strings.TrimSpace(input.AgentID),
		"round_id":    roundID,
		"role":        "system",
		"content":     strings.TrimSpace(input.Content),
		"timestamp":   timestamp,
		"metadata": map[string]any{
			"subtype":         SystemMessageSubtypeGuidedInput,
			"delivery_policy": string(protocol.ChatDeliveryPolicyGuide),
			"source_round_id": sourceRoundID,
		},
	}
	if roomID := strings.TrimSpace(input.RoomID); roomID != "" {
		message["room_id"] = roomID
	}
	if conversationID := strings.TrimSpace(input.ConversationID); conversationID != "" {
		message["conversation_id"] = conversationID
	}
	if sessionID := strings.TrimSpace(input.SessionID); sessionID != "" {
		message["session_id"] = sessionID
	}
	return message
}

func firstGuidanceValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return "guidance"
}

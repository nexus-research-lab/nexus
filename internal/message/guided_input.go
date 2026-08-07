// INPUT: transcript 解析出的运行中 round 用户引导。
// OUTPUT: 不进入普通用户气泡的 guided_input 系统消息。
// POS: 用户引导在历史时间线中的统一消息投影。
package message

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const SystemMessageSubtypeGuidedInput = "guided_input"

// GuidedInputMessageInput 描述历史时间线中的用户引导。
type GuidedInputMessageInput struct {
	MessageID     string
	SessionKey    string
	AgentID       string
	RoundID       string
	SourceRoundID string
	Content       string
	SessionID     string
	Timestamp     int64
}

// NewGuidedInputMessage 把运行时引导投影成系统消息。
func NewGuidedInputMessage(input GuidedInputMessageInput) protocol.Message {
	message := protocol.Message{
		"message_id":  strings.TrimSpace(input.MessageID),
		"session_key": strings.TrimSpace(input.SessionKey),
		"agent_id":    strings.TrimSpace(input.AgentID),
		"round_id":    strings.TrimSpace(input.RoundID),
		"role":        "system",
		"content":     strings.TrimSpace(input.Content),
		"timestamp":   input.Timestamp,
		"metadata": map[string]any{
			"subtype":         SystemMessageSubtypeGuidedInput,
			"delivery_policy": string(protocol.ChatDeliveryPolicyGuide),
			"source_round_id": strings.TrimSpace(input.SourceRoundID),
		},
	}
	if sessionID := strings.TrimSpace(input.SessionID); sessionID != "" {
		message["session_id"] = sessionID
	}
	return message
}

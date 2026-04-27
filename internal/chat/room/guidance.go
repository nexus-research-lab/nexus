package room

import (
	"time"

	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// GuidanceMessageInput 描述 Room slot 引导消息的构造输入。
type GuidanceMessageInput struct {
	SessionKey     string
	RoomID         string
	ConversationID string
	AgentID        string
	AgentRoundID   string
	SourceRoundID  string
	Content        string
	SDKSessionID   string
}

// BuildGuidanceMessage 构建 Room slot 的引导消息。
func BuildGuidanceMessage(input GuidanceMessageInput) protocol.Message {
	if input.AgentID == "" || input.AgentRoundID == "" {
		return protocol.Message{}
	}
	return message.NewGuidedInputMessage(message.GuidedInputMessageInput{
		SessionKey:     input.SessionKey,
		RoomID:         input.RoomID,
		ConversationID: input.ConversationID,
		AgentID:        input.AgentID,
		RoundID:        input.AgentRoundID,
		SourceRoundID:  input.SourceRoundID,
		Content:        input.Content,
		SessionID:      input.SDKSessionID,
		Timestamp:      time.Now().UnixMilli(),
	})
}

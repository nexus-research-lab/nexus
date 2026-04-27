package room

import (
	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

// SlotMessageMapper 将 SDK 消息映射为 Room 的协议事件与持久消息。
type SlotMessageMapper struct {
	*message.EventMapper
}

// NewSlotMessageMapper 创建 Room slot 消息映射器。
func NewSlotMessageMapper(
	sessionKey string,
	roomID string,
	conversationID string,
	agentID string,
	slotMessageID string,
	agentRoundID string,
) *SlotMessageMapper {
	return &SlotMessageMapper{EventMapper: message.NewEventMapper(message.EventMapperOptions{
		Context: message.MessageContext{
			SessionKey:     sessionKey,
			RoomID:         roomID,
			ConversationID: conversationID,
			AgentID:        agentID,
			RoundID:        agentRoundID,
			ParentID:       slotMessageID,
		},
		CausedBy: agentRoundID,
	})}
}

// Map 保持 Room slot mapper 的场景化返回值。
func (m *SlotMessageMapper) Map(
	incoming sdkprotocol.ReceivedMessage,
	interruptReason ...string,
) ([]protocol.EventMessage, []protocol.Message, string, error) {
	result, err := m.EventMapper.Map(incoming, interruptReason...)
	if err != nil {
		return nil, nil, "", err
	}
	return result.Events, result.DurableMessages, result.TerminalStatus, nil
}

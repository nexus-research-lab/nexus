package room

import (
	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// SlotMessageMapper 将 SDK 消息映射为 Room 的协议事件与持久消息。
type SlotMessageMapper struct {
	*message.EventMapper
}

// NewSlotMessageMapper 创建 Room slot 消息映射器。roundID 是 root round，agentRoundID 是 slot 执行轮次。
func NewSlotMessageMapper(
	sessionKey string,
	roomID string,
	conversationID string,
	agentID string,
	slotMessageID string,
	roundID string,
	agentRoundID string,
	workspacePath ...string,
) *SlotMessageMapper {
	return &SlotMessageMapper{EventMapper: message.NewEventMapper(message.EventMapperOptions{
		Context: message.MessageContext{
			SessionKey:     sessionKey,
			RoomID:         roomID,
			ConversationID: conversationID,
			AgentID:        agentID,
			WorkspacePath:  firstNonEmpty(workspacePath...),
			RoundID:        roundID,
			AgentRoundID:   agentRoundID,
			ParentID:       slotMessageID,
		},
	})}
}

// Map 保持 Room slot mapper 的场景化返回值。
func (m *SlotMessageMapper) Map(
	incoming sdkprotocol.ReceivedMessage,
	interruptReason ...string,
) ([]protocol.EventMessage, []protocol.Message, string, error) {
	result, err := m.MapResult(incoming, interruptReason...)
	if err != nil {
		return nil, nil, "", err
	}
	return result.Events, result.DurableMessages, result.TerminalStatus, nil
}

// MapResult 保留完整终态信息，供 Room 执行器把 error subtype 传到 slot 收口。
func (m *SlotMessageMapper) MapResult(
	incoming sdkprotocol.ReceivedMessage,
	interruptReason ...string,
) (message.EventMapResult, error) {
	return m.EventMapper.Map(incoming, interruptReason...)
}

// SetMessageDecorator 为 Room durable 消息及其最终 assistant 投影补充场景字段。
func (m *SlotMessageMapper) SetMessageDecorator(decorator message.MessageDecorator) {
	m.EventMapper.SetMessageDecorator(decorator)
}

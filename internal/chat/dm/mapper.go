package dm

import (
	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

// MessageMapper 将 SDK 消息映射为 DM 的协议事件与持久消息。
type MessageMapper struct {
	*message.EventMapper
}

// NewMessageMapper 创建 DM 消息映射器。
func NewMessageMapper(sessionKey string, agentID string, roundID string) *MessageMapper {
	return &MessageMapper{EventMapper: message.NewEventMapper(message.EventMapperOptions{
		Context: message.MessageContext{
			SessionKey: sessionKey,
			AgentID:    agentID,
			RoundID:    roundID,
			ParentID:   roundID,
		},
		IncludeStreamLifecycle: true,
	})}
}

// Map 保持 DM mapper 的场景化返回值。
func (m *MessageMapper) Map(
	incoming sdkprotocol.ReceivedMessage,
	interruptReason ...string,
) ([]protocol.EventMessage, []protocol.Message, string, string, error) {
	result, err := m.EventMapper.Map(incoming, interruptReason...)
	if err != nil {
		return nil, nil, "", "", err
	}
	return result.Events, result.DurableMessages, result.TerminalStatus, result.ResultSubtype, nil
}

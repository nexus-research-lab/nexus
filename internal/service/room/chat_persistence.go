package room

import "github.com/nexus-research-lab/nexus/internal/protocol"

func (s *RealtimeService) persistSharedInlineMessage(conversationID string, message protocol.Message) error {
	return s.roomHistory.AppendInlineMessage(conversationID, message)
}

func (s *RealtimeService) persistSharedDurableMessage(
	conversationID string,
	slot *activeRoomSlot,
	message protocol.Message,
) error {
	if slot == nil || !protocol.IsTranscriptNativeMessage(protocol.Message(message)) {
		return s.persistSharedInlineMessage(conversationID, message)
	}
	return s.roomHistory.AppendTranscriptReference(
		conversationID,
		slot.WorkspacePath,
		slot.RuntimeSessionKey,
		message,
	)
}

// INPUT: Room runtime 注入的 root round 与当前 Agent。
// OUTPUT: 可持久化到消息的 root / cause / hop 因果信息。
// POS: Room 工具消息与后续唤醒之间的因果链连接点。
package realtime

import "strings"

func (s *Service) resolveRoomMessageCausality(
	conversationID string,
	sourceAgentID string,
	rootRoundID string,
) (string, string, int) {
	normalizedConversationID := strings.TrimSpace(conversationID)
	normalizedSourceAgentID := strings.TrimSpace(sourceAgentID)
	normalizedRootRoundID := strings.TrimSpace(rootRoundID)

	for _, roundValue := range s.rounds.snapshotConversation(normalizedConversationID) {
		if roundValue == nil || strings.TrimSpace(roundValue.ConversationID) != normalizedConversationID {
			continue
		}
		if normalizedRootRoundID != "" && roomRootRoundID(roundValue) != normalizedRootRoundID {
			continue
		}
		for _, slot := range roundValue.Slots {
			if slot == nil || strings.TrimSpace(slot.AgentID) != normalizedSourceAgentID {
				continue
			}
			return roomRootRoundID(roundValue), strings.TrimSpace(roundValue.RoundID), roundValue.HopIndex
		}
	}
	return normalizedRootRoundID, normalizedRootRoundID, 0
}

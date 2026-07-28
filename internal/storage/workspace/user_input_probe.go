// INPUT: owner-scoped Room conversation 或 Agent session 的 canonical overlay。
// OUTPUT: 是否存在真实、可见的用户输入。
// POS: draft/历史兼容维护使用的严格只读历史判定。
package workspace

import (
	"errors"
	"os"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// HasCanonicalUserInput 判断 Room 共享历史是否包含用户输入。
//
// Room 用户输入本身就是 inline overlay；assistant transcript ref 不可能提供
// 用户输入，因此无需解析 transcript，也不会触发 transcript cache 写入。
func (s *RoomHistoryStore) HasCanonicalUserInput(ownerUserID string, conversationID string) (bool, error) {
	rows, err := s.files.readRoomJSONL(
		ownerUserID,
		s.paths.RoomConversationOverlayPath(ownerUserID, conversationID),
	)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if stringFromAny(row[overlayKindField]) == overlayKindTranscriptRef {
			continue
		}
		if protocol.MessageRole(protocol.Message(row)) == "user" &&
			!boolFromAny(row["hidden_from_user"]) &&
			!boolFromAny(row["is_synthetic"]) {
			return true, nil
		}
	}
	return false, nil
}

// HasCanonicalUserInput 判断 Agent session overlay 是否包含真实、可见的用户输入。
//
// DM 的用户输入由 round marker 持久化。internal/synthetic marker 不代表用户
// 开始了会话；兼容旧 overlay 时也接受直接持久化的 user message。
func (s *AgentHistoryStore) HasCanonicalUserInput(workspacePath string, sessionKey string) (bool, error) {
	state, err := s.readOverlayHistoryState(workspacePath, sessionKey)
	if err != nil {
		return false, err
	}
	for _, marker := range state.RoundMarkers {
		if !marker.HiddenFromUser && !marker.Synthetic {
			return true, nil
		}
	}
	for _, message := range state.MessageRows {
		if protocol.MessageRole(message) == "user" &&
			!boolFromAny(message["hidden_from_user"]) &&
			!boolFromAny(message["is_synthetic"]) {
			return true, nil
		}
	}
	return false, nil
}

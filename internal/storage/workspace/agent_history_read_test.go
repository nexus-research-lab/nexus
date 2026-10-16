// 测试专用入口：生产路径已不再调用，仅供本包测试复用。
package workspace

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *AgentHistoryStore) readSegmentedTranscriptMessages(
	workspacePath string,
	sessionKey string,
	agentID string,
	sessionIDs []string,
	roundMarkers []transcriptRoundMarker,
) ([]protocol.Message, error) {
	return s.readSegmentedTranscriptMessagesContext(
		context.Background(),
		workspacePath,
		sessionKey,
		agentID,
		sessionIDs,
		roundMarkers,
	)
}

func (s *AgentHistoryStore) readTranscriptMessages(
	workspacePath string,
	sessionKey string,
	agentID string,
	sessionID string,
	roundMarkers []transcriptRoundMarker,
	throughMessageID string,
) ([]protocol.Message, error) {
	return s.readTranscriptMessagesContext(
		context.Background(),
		workspacePath,
		sessionKey,
		agentID,
		sessionID,
		roundMarkers,
		throughMessageID,
	)
}

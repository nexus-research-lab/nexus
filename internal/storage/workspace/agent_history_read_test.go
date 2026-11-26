// 测试专用入口：生产路径已不再调用，仅供本包测试复用。
package workspace

import (
	"context"
	"os"

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
	rows, _, err := s.readTranscriptReplayContext(
		context.Background(), workspacePath, sessionKey, agentID, sessionID, roundMarkers, throughMessageID, false,
	)
	return rows, err
}

func (s *AgentHistoryStore) readTranscriptCache(path string, source historyPageSourceSnapshot) ([]transcriptEntry, bool) {
	entries, _, ok := s.readTranscriptCacheLineage(path, source)
	return cloneTranscriptEntries(entries), ok
}

func readTranscriptEntriesFileContext(ctx context.Context, file *os.File) ([]transcriptEntry, error) {
	entries, _, err := readTranscriptEntriesReaderContext(ctx, file, 0)
	return entries, err
}

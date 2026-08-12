// INPUT: transcript 文件状态与影响投影结果的 round marker 字段。
// OUTPUT: 可安全复用或失效的 transcript 消息缓存。
// POS: workspace transcript 投影的缓存边界。
package workspace

import (
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type transcriptCacheEntry struct {
	FileSize               int64
	ModifiedUnix           int64
	RoundMarkerFingerprint string
	LastAccessUTC          int64
	Messages               []protocol.Message
}

func (s *AgentHistoryStore) readTranscriptCache(
	path string,
	fileInfo os.FileInfo,
	roundMarkerFingerprint string,
) ([]protocol.Message, bool) {
	s.cache.mu.RLock()
	entry, exists := s.cache.messages[path]
	s.cache.mu.RUnlock()
	if !exists {
		return nil, false
	}
	if entry.FileSize != fileInfo.Size() ||
		entry.ModifiedUnix != fileInfo.ModTime().UnixNano() ||
		entry.RoundMarkerFingerprint != roundMarkerFingerprint {
		return nil, false
	}

	s.cache.mu.Lock()
	refreshedEntry := s.cache.messages[path]
	refreshedEntry.LastAccessUTC = time.Now().UTC().UnixNano()
	s.cache.messages[path] = refreshedEntry
	s.cache.mu.Unlock()
	return entry.Messages, true
}

func (s *AgentHistoryStore) writeTranscriptCache(
	path string,
	fileInfo os.FileInfo,
	roundMarkerFingerprint string,
	rows []protocol.Message,
) {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()

	s.cache.messages[path] = transcriptCacheEntry{
		FileSize:               fileInfo.Size(),
		ModifiedUnix:           fileInfo.ModTime().UnixNano(),
		RoundMarkerFingerprint: roundMarkerFingerprint,
		LastAccessUTC:          time.Now().UTC().UnixNano(),
		Messages:               rows,
	}
	s.pruneTranscriptCacheLocked()
}

func fingerprintTranscriptRoundMarkers(roundMarkers []transcriptRoundMarker) string {
	if len(roundMarkers) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, marker := range roundMarkers {
		builder.WriteString(strconv.Itoa(len(marker.RoundID)))
		builder.WriteString(":")
		builder.WriteString(marker.RoundID)
		builder.WriteString("|")
		builder.WriteString(strconv.Itoa(len(marker.SourceRoundID)))
		builder.WriteString(":")
		builder.WriteString(marker.SourceRoundID)
		builder.WriteString("|")
		builder.WriteString(strconv.Itoa(len(marker.Content)))
		builder.WriteString(":")
		builder.WriteString(marker.Content)
		builder.WriteString("|")
		for _, attachment := range protocol.NormalizeChatAttachments(marker.Attachments, "") {
			builder.WriteString(string(attachment.Scope))
			builder.WriteString(":")
			builder.WriteString(attachment.RoomID)
			builder.WriteString(":")
			builder.WriteString(attachment.ConversationID)
			builder.WriteString(":")
			builder.WriteString(attachment.WorkspaceAgentID)
			builder.WriteString(":")
			builder.WriteString(attachment.WorkspacePath)
			builder.WriteString("|")
		}
		builder.WriteString("|")
		builder.WriteString(strconv.FormatInt(marker.Timestamp, 10))
		builder.WriteString("|")
		builder.WriteString(marker.DeliveryPolicy)
		builder.WriteString("|")
		builder.WriteString(strconv.FormatBool(marker.HiddenFromUser))
		builder.WriteString("|")
		builder.WriteString(strconv.FormatBool(marker.Synthetic))
		builder.WriteString("|")
		builder.WriteString(strconv.FormatBool(marker.ControlOnly))
		builder.WriteString("|")
		builder.WriteString(marker.Purpose)
		builder.WriteString("\n")
	}
	return builder.String()
}

func (s *AgentHistoryStore) pruneTranscriptCacheLocked() {
	if len(s.cache.messages) <= maxTranscriptCacheEntries {
		return
	}

	type cacheCandidate struct {
		Path          string
		LastAccessUTC int64
	}

	candidates := make([]cacheCandidate, 0, len(s.cache.messages))
	for path, entry := range s.cache.messages {
		candidates = append(candidates, cacheCandidate{
			Path:          path,
			LastAccessUTC: entry.LastAccessUTC,
		})
	}
	sort.Slice(candidates, func(i int, j int) bool {
		return candidates[i].LastAccessUTC < candidates[j].LastAccessUTC
	})
	for len(candidates) > maxTranscriptCacheEntries {
		delete(s.cache.messages, candidates[0].Path)
		candidates = candidates[1:]
	}
}

func (s *AgentHistoryStore) invalidateTranscriptCache(path string) {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()
	delete(s.cache.messages, path)
}

func (s *AgentHistoryStore) invalidateTranscriptCachePrefix(prefix string) {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()
	for path := range s.cache.messages {
		if path == prefix || strings.HasPrefix(path, prefix+string(os.PathSeparator)) {
			delete(s.cache.messages, path)
		}
	}
}

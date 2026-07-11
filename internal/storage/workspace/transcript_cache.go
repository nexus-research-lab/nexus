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
	s.cacheMu.RLock()
	entry, exists := s.messageCache[path]
	s.cacheMu.RUnlock()
	if !exists {
		return nil, false
	}
	if entry.FileSize != fileInfo.Size() ||
		entry.ModifiedUnix != fileInfo.ModTime().UnixNano() ||
		entry.RoundMarkerFingerprint != roundMarkerFingerprint {
		return nil, false
	}

	s.cacheMu.Lock()
	refreshedEntry := s.messageCache[path]
	refreshedEntry.LastAccessUTC = time.Now().UTC().UnixNano()
	s.messageCache[path] = refreshedEntry
	s.cacheMu.Unlock()
	return entry.Messages, true
}

func (s *AgentHistoryStore) writeTranscriptCache(
	path string,
	fileInfo os.FileInfo,
	roundMarkerFingerprint string,
	rows []protocol.Message,
) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	s.messageCache[path] = transcriptCacheEntry{
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
		builder.WriteString(marker.Purpose)
		builder.WriteString("\n")
	}
	return builder.String()
}

func (s *AgentHistoryStore) pruneTranscriptCacheLocked() {
	if len(s.messageCache) <= maxTranscriptCacheEntries {
		return
	}

	type cacheCandidate struct {
		Path          string
		LastAccessUTC int64
	}

	candidates := make([]cacheCandidate, 0, len(s.messageCache))
	for path, entry := range s.messageCache {
		candidates = append(candidates, cacheCandidate{
			Path:          path,
			LastAccessUTC: entry.LastAccessUTC,
		})
	}
	sort.Slice(candidates, func(i int, j int) bool {
		return candidates[i].LastAccessUTC < candidates[j].LastAccessUTC
	})
	for len(candidates) > maxTranscriptCacheEntries {
		delete(s.messageCache, candidates[0].Path)
		candidates = candidates[1:]
	}
}

func (s *AgentHistoryStore) invalidateTranscriptCache(path string) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	delete(s.messageCache, path)
}

func (s *AgentHistoryStore) invalidateTranscriptCachePrefix(prefix string) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	for path := range s.messageCache {
		if path == prefix || strings.HasPrefix(path, prefix+string(os.PathSeparator)) {
			delete(s.messageCache, path)
		}
	}
}

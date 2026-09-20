// INPUT: 已校验 transcript 文件快照与规范化 JSONL 条目。
// OUTPUT: 与 marker、会话身份和 fork 边界无关的解析缓存。
// POS: 普通、分段和显式 transcript 读取共用的缓存边界。
package workspace

import (
	"os"
	"sort"
	"strings"
	"time"
)

type transcriptCacheEntry struct {
	Source        historyPageSourceSnapshot
	LastAccessUTC int64
	Entries       []transcriptEntry
}

func (s *AgentHistoryStore) readTranscriptCache(path string, source historyPageSourceSnapshot) ([]transcriptEntry, bool) {
	s.cache.mu.Lock()
	entry, exists := s.cache.entries[path]
	if exists && entry.Source == source {
		entry.LastAccessUTC = time.Now().UnixNano()
		s.cache.entries[path] = entry
	}
	s.cache.mu.Unlock()
	if !exists || entry.Source != source {
		return nil, false
	}
	return cloneTranscriptEntries(entry.Entries), true
}

func (s *AgentHistoryStore) writeTranscriptCache(path string, source historyPageSourceSnapshot, entries []transcriptEntry) {
	// 缓存只持有私有副本，投影和分段合并不能修改其他请求的原始条目。
	entry := transcriptCacheEntry{Source: source, LastAccessUTC: time.Now().UnixNano(), Entries: cloneTranscriptEntries(entries)}
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()
	s.cache.entries[path] = entry
	s.pruneTranscriptCacheLocked()
}

func cloneTranscriptEntries(entries []transcriptEntry) []transcriptEntry {
	cloned := make([]transcriptEntry, len(entries))
	for i, entry := range entries {
		cloned[i] = transcriptEntry{Index: entry.Index, Data: cloneTranscriptJSON(entry.Data).(map[string]any)}
	}
	return cloned
}

func (s *AgentHistoryStore) pruneTranscriptCacheLocked() {
	if len(s.cache.entries) <= maxTranscriptCacheEntries {
		return
	}

	type cacheCandidate struct {
		Path          string
		LastAccessUTC int64
	}

	candidates := make([]cacheCandidate, 0, len(s.cache.entries))
	for path, entry := range s.cache.entries {
		candidates = append(candidates, cacheCandidate{
			Path:          path,
			LastAccessUTC: entry.LastAccessUTC,
		})
	}
	sort.Slice(candidates, func(i int, j int) bool {
		return candidates[i].LastAccessUTC < candidates[j].LastAccessUTC
	})
	for len(candidates) > maxTranscriptCacheEntries {
		delete(s.cache.entries, candidates[0].Path)
		candidates = candidates[1:]
	}
}

func (s *AgentHistoryStore) invalidateTranscriptCachePrefix(prefix string) {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()
	for path := range s.cache.entries {
		if path == prefix || strings.HasPrefix(path, prefix+string(os.PathSeparator)) {
			delete(s.cache.entries, path)
		}
	}
}

// transcript 条目来自 JSON 解码，只需复制 map 和 slice，标量可安全共享。
func cloneTranscriptJSON(value any) any {
	switch value := value.(type) {
	case map[string]any:
		cloned := make(map[string]any, len(value))
		for key, item := range value {
			cloned[key] = cloneTranscriptJSON(item)
		}
		return cloned
	case []any:
		cloned := make([]any, len(value))
		for index, item := range value {
			cloned[index] = cloneTranscriptJSON(item)
		}
		return cloned
	default:
		return value
	}
}

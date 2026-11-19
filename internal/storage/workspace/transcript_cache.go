// INPUT: 已校验 transcript 文件快照与规范化 JSONL 条目。
// OUTPUT: 与 marker、会话身份和 fork 边界无关的解析缓存及其代系；纯追加时只解析新增字节并保留投影检查点。
// POS: 普通、分段和显式 transcript 读取共用的缓存边界。
package workspace

import (
	"context"
	"io"
	"os"
	"slices"
	"sort"
	"strings"
	"sync/atomic"
)

var transcriptCacheAccesses atomic.Uint64

// 按源文件字节为解析缓存计重，避免深拷贝几份大型 transcript 后长期驻留。
const maxTranscriptCacheSourceBytes = 64 << 20

// transcriptSlimSourceBytes 以上的 transcript 在建立投影检查点后，把检查点之前的条目
// 收缩为骨架（见 transcript_skeleton.go）：缓存体量不再随会话内容线性增长，
// 超长会话也能保留缓存走增量路径。测试可调低以覆盖骨架路径。
var transcriptSlimSourceBytes int64 = maxTranscriptCacheSourceBytes / 2

// transcriptReadMode 区分读取方能否消费骨架条目、能否让超大 transcript 进入缓存。
type transcriptReadMode int

const (
	// transcriptReadFull 需要完整条目；骨架缓存视为未命中，可收缩尺寸的 transcript 不写缓存，
	// 避免与 DM 投影互相覆盖代系。
	transcriptReadFull transcriptReadMode = iota
	// transcriptReadProjection 是 DM 检查点投影：接受骨架，完整解析结果总是进入缓存，
	// 随后由检查点收缩；未能建立检查点的超大条目由调用方释放。
	transcriptReadProjection
	// transcriptReadFresh 跳过缓存重新完整解析，用于骨架缓存无法服务本次投影时。
	transcriptReadFresh
)

// transcriptCacheRead 是一次读取得到的条目视图。Entries 可能与缓存共享，只读；
// Index 小于 SlimEnd 的条目是骨架，只能用于建链、工具关联与 marker 对齐。
type transcriptCacheRead struct {
	Entries []transcriptEntry
	Lineage uint64
	SlimEnd int
}

type transcriptCacheEntry struct {
	Source historyPageSourceSnapshot
	// LastAccess 是单调递增的访问序号，挂钟时间粒度不够区分紧挨着的两次写入。
	LastAccess uint64
	Entries    []transcriptEntry
	// NextIndex 是已扫描的 JSONL 行数，追加读取从这里继续编号。
	NextIndex int
	// Lineage 在完整解析时更新、纯追加时沿用，标识“前缀条目未变”的代系。
	Lineage uint64
	// Projection 是该代系最近一次 DM 投影的前缀检查点，可为空。
	Projection *transcriptProjectionCheckpoint
	// SlimEnd 之前（按 Index）的条目已收缩为骨架；0 表示全部完整。
	SlimEnd int
}

func (entry transcriptCacheEntry) read() transcriptCacheRead {
	return transcriptCacheRead{Entries: entry.Entries, Lineage: entry.Lineage, SlimEnd: entry.SlimEnd}
}

func (s *AgentHistoryStore) readTranscriptCache(path string, source historyPageSourceSnapshot) ([]transcriptEntry, bool) {
	read, ok := s.readTranscriptCacheLineage(path, source, transcriptReadFull)
	return cloneTranscriptEntries(read.Entries), ok
}

// readTranscriptCacheLineage 返回缓存内共享的条目：调用方只能读取，需要修改的部分自行深拷贝。
func (s *AgentHistoryStore) readTranscriptCacheLineage(
	path string,
	source historyPageSourceSnapshot,
	mode transcriptReadMode,
) (transcriptCacheRead, bool) {
	if mode == transcriptReadFresh {
		return transcriptCacheRead{}, false
	}
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()
	entry, exists := s.cache.entries[path]
	if !exists || entry.Source != source || (entry.SlimEnd > 0 && mode != transcriptReadProjection) {
		return transcriptCacheRead{}, false
	}
	entry.LastAccess = transcriptCacheAccesses.Add(1)
	s.cache.entries[path] = entry
	return entry.read(), true
}

// writeTranscriptCache 接管一次完整解析的结果并返回新代系；不缓存时返回 0。
// 写入后 entries 与缓存共享，调用方不得再修改。
func (s *AgentHistoryStore) writeTranscriptCache(
	path string,
	source historyPageSourceSnapshot,
	entries []transcriptEntry,
	nextIndex int,
	mode transcriptReadMode,
) uint64 {
	if mode == transcriptReadFull && source.Size > transcriptSlimSourceBytes {
		return 0
	}
	return s.storeTranscriptCache(path, transcriptCacheEntry{
		Source: source, Entries: slices.Clip(entries), NextIndex: nextIndex, Lineage: nextTranscriptCacheLineage(),
	}, mode != transcriptReadFull)
}

// storeTranscriptCache 接管 entry.Entries 的所有权；调用方之后不得修改其中任何条目。
// 超过完整缓存上限的条目只有 allowLarge（DM 投影随后会收缩它）时才进入缓存。
func (s *AgentHistoryStore) storeTranscriptCache(path string, entry transcriptCacheEntry, allowLarge bool) uint64 {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()
	if entry.Source.Size > maxTranscriptCacheSourceBytes && !allowLarge && entry.SlimEnd == 0 {
		delete(s.cache.entries, path)
		return 0
	}
	entry.LastAccess = transcriptCacheAccesses.Add(1)
	s.cache.entries[path] = entry
	s.pruneTranscriptCacheLocked()
	return entry.Lineage
}

// releaseUnslimmedTranscript 释放 DM 投影后仍未收缩的超大条目：没有检查点可依附时，
// 完整条目不能长期驻留。
func (s *AgentHistoryStore) releaseUnslimmedTranscript(path string, lineage uint64) {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()
	if entry, exists := s.cache.entries[path]; exists && entry.Lineage == lineage &&
		entry.SlimEnd == 0 && entry.Source.Size > maxTranscriptCacheSourceBytes {
		delete(s.cache.entries, path)
	}
}

// extendTranscriptCache 在 transcript 只追加时复用缓存条目，只解析新增字节。
// 只接受与缓存快照前缀一致的纯追加：文件身份相同、按旧长度重算的首尾指纹相同、
// 旧末尾恰为换行；改写（临时文件替换）、截断或半行边界都回退完整解析。
func (s *AgentHistoryStore) extendTranscriptCache(
	ctx context.Context,
	path string,
	file *os.File,
	current historyPageSourceSnapshot,
	mode transcriptReadMode,
) (transcriptCacheRead, bool, error) {
	if mode == transcriptReadFresh {
		return transcriptCacheRead{}, false, nil
	}
	s.cache.mu.Lock()
	cached, exists := s.cache.entries[path]
	s.cache.mu.Unlock()
	previous := cached.Source
	if !exists || (cached.SlimEnd > 0 && mode != transcriptReadProjection) || !previous.Exists || !current.Exists || previous.FileIdentity == "" ||
		previous.FileIdentity != current.FileIdentity || previous.Size <= 0 || current.Size <= previous.Size {
		return transcriptCacheRead{}, false, nil
	}
	fingerprint, err := historyPageFileEdgeFingerprint(ctx, file, previous.Size)
	if err != nil {
		return transcriptCacheRead{}, false, err
	}
	var boundary [1]byte
	if fingerprint != previous.EdgeFingerprint {
		return transcriptCacheRead{}, false, nil
	}
	if _, err = file.ReadAt(boundary[:], previous.Size-1); err != nil || boundary[0] != '\n' {
		return transcriptCacheRead{}, false, nil
	}
	appended, nextIndex, err := readTranscriptEntriesReaderContext(
		ctx,
		io.NewSectionReader(file, previous.Size, current.Size-previous.Size),
		cached.NextIndex,
	)
	if err != nil {
		return transcriptCacheRead{}, false, err
	}
	info, err := file.Stat()
	if err != nil {
		return transcriptCacheRead{}, false, err
	}
	after, err := snapshotOpenedFile(ctx, file, info, historyPageSourceSnapshot{})
	if err != nil {
		return transcriptCacheRead{}, false, err
	}
	if after != current {
		// 读取期间仍在写入；交给完整解析路径，与原有缓存准入保持一致。
		return transcriptCacheRead{}, false, nil
	}
	// 缓存条目不可变，新缓存、旧缓存与调用方共享同一批条目；cap 恰好等于 len，
	// 任何一方 append 都会重新分配，不会写进别人的底层数组。
	combined := make([]transcriptEntry, 0, len(cached.Entries)+len(appended))
	combined = append(combined, cached.Entries...)
	combined = append(combined, appended...)
	// 前缀条目未变，沿用代系与投影检查点；并发扩展同一路径时后写者覆盖，结果等价。
	cached.Source, cached.Entries, cached.NextIndex = current, combined, nextIndex
	lineage := s.storeTranscriptCache(path, cached, mode != transcriptReadFull)
	return transcriptCacheRead{Entries: combined, Lineage: lineage, SlimEnd: cached.SlimEnd}, true, nil
}

func cloneTranscriptEntries(entries []transcriptEntry) []transcriptEntry {
	cloned := make([]transcriptEntry, len(entries))
	for i, entry := range entries {
		cloned[i] = transcriptEntry{Index: entry.Index, Data: cloneTranscriptJSON(entry.Data).(map[string]any)}
	}
	return cloned
}

func (s *AgentHistoryStore) pruneTranscriptCacheLocked() {
	var weight int64
	for _, entry := range s.cache.entries {
		weight += entry.weight()
	}
	if len(s.cache.entries) <= maxTranscriptCacheEntries && weight <= maxTranscriptCacheSourceBytes {
		return
	}

	type cacheCandidate struct {
		Path       string
		LastAccess uint64
	}

	candidates := make([]cacheCandidate, 0, len(s.cache.entries))
	for path, entry := range s.cache.entries {
		candidates = append(candidates, cacheCandidate{
			Path:       path,
			LastAccess: entry.LastAccess,
		})
	}
	sort.Slice(candidates, func(i int, j int) bool {
		return candidates[i].LastAccess < candidates[j].LastAccess
	})
	// 最近访问的条目总是保留：源文件不超过上限的活跃长会话即使带上投影检查点超出预算，
	// 也只挤掉其他会话，不会刚写入就被自己淘汰，否则每轮都要完整重解析。
	for len(candidates) > 1 && (len(candidates) > maxTranscriptCacheEntries || weight > maxTranscriptCacheSourceBytes) {
		weight -= s.cache.entries[candidates[0].Path].weight()
		delete(s.cache.entries, candidates[0].Path)
		candidates = candidates[1:]
	}
}

// transcriptSkeletonWeight 是单个骨架条目的估算驻留字节。
const transcriptSkeletonWeight = 256

// weight 按源文件字节估算驻留内存；投影检查点与解析条目体量相当，按同等权重再计一次。
// 骨架条目按固定小权重计，其余条目按平均每条源字节计。
func (entry transcriptCacheEntry) weight() int64 {
	full := entry.Source.Size
	if entry.SlimEnd > 0 && len(entry.Entries) > 0 {
		slim := sort.Search(len(entry.Entries), func(i int) bool { return entry.Entries[i].Index >= entry.SlimEnd })
		full = full*int64(len(entry.Entries)-slim)/int64(len(entry.Entries)) + int64(slim)*transcriptSkeletonWeight
	}
	if entry.Projection != nil && entry.Projection.Rows != nil {
		return full + entry.Source.Size
	}
	return full
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

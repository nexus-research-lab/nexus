// INPUT: 缓存中的 transcript 解析代系、主链、对齐后的 round marker 与投影身份。
// OUTPUT: 纯追加后只重放最后一个用户轮次之后的条目、与完整投影逐行一致的历史消息。
// POS: DM 长会话历史重建的投影前缀检查点；不改变投影语义，只跳过可证明不变的前缀。
package workspace

import (
	"reflect"
	"slices"
	"sync/atomic"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// transcriptCacheLineages 为每次完整解析分配新代系；纯追加扩展沿用旧代系，
// 因而同一代系内相同 Index 的条目内容一定相同。
var transcriptCacheLineages atomic.Uint64

func nextTranscriptCacheLineage() uint64 {
	return transcriptCacheLineages.Add(1)
}

// transcriptProjectionCheckpoint 记录主链前 len(ChainIndexes) 个条目的投影结果。
// 检查点总是落在某个用户轮次刚重建 processor 之后，此时投影状态全部是标量，
// 可以原样恢复。Rows 由缓存独占且不可变。
type transcriptProjectionCheckpoint struct {
	Lineage      uint64
	Key          transcriptProjectionKey
	ChainIndexes []int
	Markers      []transcriptRoundMarker
	State        transcriptProjectionState
	Rows         []protocol.Message
}

type transcriptProjectionKey struct {
	WorkspacePath string
	SessionKey    string
	AgentID       string
}

func (s *AgentHistoryStore) readTranscriptProjectionCheckpoint(path string, lineage uint64) *transcriptProjectionCheckpoint {
	if lineage == 0 {
		return nil
	}
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()
	entry, exists := s.cache.entries[path]
	if !exists || entry.Lineage != lineage || entry.Projection == nil || entry.Projection.Lineage != lineage {
		return nil
	}
	return entry.Projection
}

func (s *AgentHistoryStore) storeTranscriptProjectionCheckpoint(path string, checkpoint *transcriptProjectionCheckpoint) {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()
	entry, exists := s.cache.entries[path]
	// 解析缓存已换代（文件被改写或淘汰）时，旧检查点没有可依附的前缀。
	if !exists || entry.Lineage != checkpoint.Lineage {
		return
	}
	entry.Projection = checkpoint
	s.cache.entries[path] = entry
	s.pruneTranscriptCacheLocked()
}

// projectTranscriptChainCheckpointed 与 projectTranscriptChain 结果一致；
// 当缓存检查点覆盖的主链前缀、marker 前缀和投影身份都未变化时，只投影其后的条目。
func (s *AgentHistoryStore) projectTranscriptChainCheckpointed(
	path string,
	lineage uint64,
	workspacePath string,
	sessionKey string,
	agentID string,
	chain []transcriptEntry,
	roundMarkers []transcriptRoundMarker,
) []protocol.Message {
	key := transcriptProjectionKey{WorkspacePath: workspacePath, SessionKey: sessionKey, AgentID: agentID}
	aligned := alignTranscriptRoundMarkersWithFilter(chain, roundMarkers, shouldSkipTranscriptEntry)

	state := transcriptProjectionState{}
	start := 0
	var memoRows []protocol.Message
	if previous := s.readTranscriptProjectionCheckpoint(path, lineage); previous != nil &&
		transcriptProjectionCheckpointApplies(previous, key, chain, aligned) {
		start = len(previous.ChainIndexes)
		state = previous.State
		state.restoreProcessor(workspacePath, sessionKey, agentID)
		memoRows = previous.Rows
	}

	tail := make([]protocol.Message, 0, len(chain)-start)
	var latest transcriptProjectionState
	latestConsumed, latestRows := -1, 0
	visitProjectedTranscriptFrom(
		workspacePath,
		sessionKey,
		agentID,
		slices.Values(chain[start:]),
		aligned,
		shouldSkipTranscriptEntry,
		&state,
		func(consumed int, snapshot transcriptProjectionState) {
			latest, latestConsumed, latestRows = snapshot, consumed, len(tail)
		},
		func(rows ...protocol.Message) {
			tail = append(tail, rows...)
		},
	)

	if lineage != 0 && latestConsumed >= 0 {
		latest.processor = nil
		consumed := start + latestConsumed
		indexes := make([]int, consumed)
		for i, entry := range chain[:consumed] {
			indexes[i] = entry.Index
		}
		// 旧检查点行不可变，新检查点直接共享它们，只深拷贝新增的部分。
		rows := make([]protocol.Message, 0, len(memoRows)+latestRows)
		rows = append(rows, memoRows...)
		for _, row := range tail[:latestRows] {
			rows = append(rows, cloneProjectedMessage(row))
		}
		s.storeTranscriptProjectionCheckpoint(path, &transcriptProjectionCheckpoint{
			Lineage:      lineage,
			Key:          key,
			ChainIndexes: indexes,
			Markers:      slices.Clone(aligned[:latest.MarkerIndex]),
			State:        latest,
			Rows:         rows,
		})
	}

	projected := make([]protocol.Message, 0, len(memoRows)+len(tail))
	for _, row := range memoRows {
		projected = append(projected, cloneProjectedMessage(row))
	}
	return append(projected, tail...)
}

func transcriptProjectionCheckpointApplies(
	checkpoint *transcriptProjectionCheckpoint,
	key transcriptProjectionKey,
	chain []transcriptEntry,
	aligned []transcriptRoundMarker,
) bool {
	if checkpoint.Key != key || len(checkpoint.ChainIndexes) > len(chain) ||
		checkpoint.State.MarkerIndex > len(aligned) || len(checkpoint.Markers) != checkpoint.State.MarkerIndex {
		return false
	}
	for i, index := range checkpoint.ChainIndexes {
		if chain[i].Index != index {
			return false
		}
	}
	return reflect.DeepEqual(checkpoint.Markers, aligned[:checkpoint.State.MarkerIndex])
}

func cloneProjectedMessage(message protocol.Message) protocol.Message {
	return cloneProjectedValue(map[string]any(message)).(map[string]any)
}

// cloneProjectedValue 深拷贝投影行里的 map、slice 与指针；标量、字符串和
// 不含引用的结构体按值共享。常见 JSON 形状走快路径，其余类型由反射处理。
func cloneProjectedValue(value any) any {
	switch typed := value.(type) {
	case nil, string, bool, int, int64, float64:
		return value
	case map[string]any:
		cloned := make(map[string]any, len(typed))
		for key, item := range typed {
			cloned[key] = cloneProjectedValue(item)
		}
		return cloned
	case protocol.Message:
		return protocol.Message(cloneProjectedValue(map[string]any(typed)).(map[string]any))
	case []any:
		cloned := make([]any, len(typed))
		for i, item := range typed {
			cloned[i] = cloneProjectedValue(item)
		}
		return cloned
	case []map[string]any:
		cloned := make([]map[string]any, len(typed))
		for i, item := range typed {
			cloned[i] = cloneProjectedValue(item).(map[string]any)
		}
		return cloned
	case []string:
		return slices.Clone(typed)
	}
	return cloneProjectedReflect(reflect.ValueOf(value)).Interface()
}

func cloneProjectedReflect(value reflect.Value) reflect.Value {
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return value
		}
		cloned := reflect.New(value.Type()).Elem()
		cloned.Set(cloneProjectedReflect(value.Elem()))
		return cloned
	case reflect.Map:
		if value.IsNil() {
			return value
		}
		cloned := reflect.MakeMapWithSize(value.Type(), value.Len())
		iterator := value.MapRange()
		for iterator.Next() {
			cloned.SetMapIndex(iterator.Key(), cloneProjectedReflect(iterator.Value()))
		}
		return cloned
	case reflect.Slice:
		if value.IsNil() {
			return value
		}
		cloned := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := range value.Len() {
			cloned.Index(i).Set(cloneProjectedReflect(value.Index(i)))
		}
		return cloned
	case reflect.Pointer:
		if value.IsNil() {
			return value
		}
		cloned := reflect.New(value.Type().Elem())
		cloned.Elem().Set(cloneProjectedReflect(value.Elem()))
		return cloned
	case reflect.Struct:
		cloned := reflect.New(value.Type()).Elem()
		cloned.Set(value)
		for i := range value.NumField() {
			if field := cloned.Field(i); field.CanSet() {
				field.Set(cloneProjectedReflect(value.Field(i)))
			}
		}
		return cloned
	default:
		return value
	}
}

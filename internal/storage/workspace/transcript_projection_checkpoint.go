// INPUT: 缓存中的 transcript 解析代系、主链、对齐后的 round marker 与投影身份。
// OUTPUT: 纯追加后只重放最后一个用户轮次之后的条目、与完整投影逐行一致的历史消息，以及本次经过的轮次起点。
// POS: 进程内完整读取 DM transcript 时的投影前缀检查点；不改变投影语义，只跳过可证明不变的前缀。
// 跨进程的增量位置由 transcript_checkpoint.go 持久化在读模型里。
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
	// Start 是检查点所在轮次的起点（主链位置与行数均为绝对值），复用检查点时仍可作为持久化检查点的候选。
	Start transcriptRoundStart
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

// transcriptProjection 是一次检查点投影的结果。Starts 是本次经过的轮次起点（位置与行数相对
// 整条主链，按先后排列），Aligned 是与主链用户轮次对齐后的 marker；两者供读模型建立持久化检查点。
type transcriptProjection struct {
	Rows    []protocol.Message
	Starts  []transcriptRoundStart
	Aligned []transcriptRoundMarker
}

// projectTranscriptChainCheckpointed 返回与 projectTranscriptChain 相同的行；
// 当缓存检查点覆盖的主链前缀、marker 前缀和投影身份都未变化时，只投影其后的条目。
func (s *AgentHistoryStore) projectTranscriptChainCheckpointed(
	path string,
	lineage uint64,
	workspacePath string,
	sessionKey string,
	agentID string,
	chain []transcriptEntry,
	roundMarkers []transcriptRoundMarker,
) transcriptProjection {
	key := transcriptProjectionKey{WorkspacePath: workspacePath, SessionKey: sessionKey, AgentID: agentID}
	aligned := alignTranscriptRoundMarkersWithFilter(chain, roundMarkers, shouldSkipTranscriptEntry)

	state := transcriptProjectionState{}
	start := 0
	var memoRows []protocol.Message
	result := transcriptProjection{Aligned: aligned}
	if previous := s.readTranscriptProjectionCheckpoint(path, lineage); previous != nil &&
		transcriptProjectionCheckpointApplies(previous, key, chain, aligned) {
		start = len(previous.ChainIndexes)
		state = previous.State
		state.restoreProcessor(workspacePath, sessionKey, agentID)
		memoRows = previous.Rows
		result.Starts = append(result.Starts, previous.Start)
	}

	tail := make([]protocol.Message, 0, len(chain)-start)
	var latest transcriptProjectionState
	latestRows := 0
	reused := len(result.Starts)
	visitProjectedTranscriptFrom(
		workspacePath,
		sessionKey,
		agentID,
		// 条目与解析缓存共享，投影会改写条目，只拷贝检查点之后真正要投影的部分。
		slices.Values(cloneTranscriptEntries(chain[start:])),
		aligned,
		shouldSkipTranscriptEntry,
		&state,
		func(roundStart transcriptRoundStart, next transcriptProjectionState) {
			roundStart.Position += start
			roundStart.Rows += len(memoRows)
			result.Starts = append(result.Starts, roundStart)
			latest, latestRows = next, len(tail)
		},
		func(rows ...protocol.Message) {
			tail = append(tail, rows...)
		},
	)

	if lineage != 0 && len(result.Starts) > reused {
		latest.processor = nil
		last := result.Starts[len(result.Starts)-1]
		indexes := make([]int, last.Position+1)
		for i, entry := range chain[:len(indexes)] {
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
			Start:        last,
		})
	}

	result.Rows = make([]protocol.Message, 0, len(memoRows)+len(tail))
	for _, row := range memoRows {
		result.Rows = append(result.Rows, cloneProjectedMessage(row))
	}
	result.Rows = append(result.Rows, tail...)
	return result
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

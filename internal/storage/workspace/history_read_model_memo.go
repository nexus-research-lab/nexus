// INPUT: 本进程上一次成功写入读模型时每个分组的结构哈希、签名与字节数。
// OUTPUT: 判断分组是否可以不再投影、序列化就沿用库内已有行的进程内备忘。
// POS: 读模型前缀复用的快路径；只在库内签名仍与备忘一致时生效，备忘丢失只退化为慢路径。
package workspace

import (
	"encoding/binary"
	"encoding/json"
	"hash/maphash"
	"math"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// 每个 scope 只记一代分组的定长摘要，几千个分组也只有几百 KB。
const maxHistoryReadModelMemoScopes = 64

type historyReadModelGroupMemo struct {
	Hash      uint64
	Signature string
	Bytes     int64
}

type historyReadModelScopeMemo struct {
	Generation string
	Groups     []historyReadModelGroupMemo
}

type historyReadModelMemoStore struct {
	mu     sync.Mutex
	scopes map[string]historyReadModelScopeMemo
	order  []string
}

func (s *historyReadModelMemoStore) load(scope string, generation string) []historyReadModelGroupMemo {
	s.mu.Lock()
	defer s.mu.Unlock()
	memo, ok := s.scopes[scope]
	if !ok || memo.Generation != generation {
		return nil
	}
	return memo.Groups
}

func (s *historyReadModelMemoStore) store(scope string, generation string, groups []historyReadModelGroupMemo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.scopes == nil {
		s.scopes = make(map[string]historyReadModelScopeMemo)
	}
	if _, exists := s.scopes[scope]; !exists {
		s.order = append(s.order, scope)
	}
	s.scopes[scope] = historyReadModelScopeMemo{Generation: generation, Groups: groups}
	for len(s.order) > maxHistoryReadModelMemoScopes {
		delete(s.scopes, s.order[0])
		s.order = s.order[1:]
	}
}

func (s *historyReadModelMemoStore) forget(scope string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.scopes, scope)
}

// historyGroupHashSeed 每个进程随机，结构哈希不落盘，也无法被构造碰撞。
var historyGroupHashSeed = maphash.MakeSeed()

// historyReadModelGroupHash 是分组内容的结构哈希：结构和类型完全相同的分组哈希相同，
// 因而投影与序列化结果也相同；不同只会导致备忘未命中，回到完整计算。
func historyReadModelGroupHash(scope string, generation string, sequence int, group historyPageIndexedGroup) uint64 {
	var h maphash.Hash
	h.SetSeed(historyGroupHashSeed)
	writeHashString(&h, scope)
	writeHashString(&h, generation)
	writeHashUint(&h, uint64(sequence))
	writeHashString(&h, group.CursorRoundID)
	writeHashUint(&h, uint64(group.CursorRoundTimestamp))
	writeHashString(&h, group.GroupKey)
	writeHashUint(&h, uint64(len(group.Items)))
	for _, row := range group.Items {
		writeHashUint(&h, hashHistoryValue(map[string]any(row)))
	}
	writeHashUint(&h, uint64(len(group.DeliveryReceipts)))
	if len(group.DeliveryReceipts) > 0 {
		writeHashUint(&h, hashHistoryValue(group.DeliveryReceipts))
	}
	return h.Sum64()
}

func hashHistoryValue(value any) uint64 {
	var h maphash.Hash
	h.SetSeed(historyGroupHashSeed)
	switch typed := value.(type) {
	case nil:
		h.WriteByte(0)
	case string:
		h.WriteByte(1)
		h.WriteString(typed)
	case bool:
		h.WriteByte(2)
		if typed {
			h.WriteByte(1)
		} else {
			h.WriteByte(0)
		}
	case int:
		h.WriteByte(3)
		writeHashUint(&h, uint64(typed))
	case int64:
		h.WriteByte(4)
		writeHashUint(&h, uint64(typed))
	case float64:
		h.WriteByte(5)
		writeHashUint(&h, math.Float64bits(typed))
	case map[string]any:
		h.WriteByte(6)
		writeHashMap(&h, typed)
	case protocol.Message:
		h.WriteByte(7)
		writeHashMap(&h, typed)
	case []any:
		h.WriteByte(8)
		writeHashUint(&h, uint64(len(typed)))
		for _, item := range typed {
			writeHashUint(&h, hashHistoryValue(item))
		}
	case []map[string]any:
		h.WriteByte(9)
		writeHashUint(&h, uint64(len(typed)))
		for _, item := range typed {
			writeHashUint(&h, hashHistoryValue(item))
		}
	case []string:
		h.WriteByte(10)
		writeHashUint(&h, uint64(len(typed)))
		for _, item := range typed {
			writeHashString(&h, item)
		}
	default:
		// 罕见类型按 JSON 编码哈希，并带上动态类型，避免不同类型的同形编码混同。
		payload, err := json.Marshal(typed)
		h.WriteByte(11)
		writeHashString(&h, reflect.TypeOf(typed).String())
		if err != nil {
			// 无法编码的值不参与复用：每次都给出不同哈希，强制走完整计算。
			writeHashUint(&h, historyHashNonces.Add(1))
		}
		h.Write(payload)
	}
	return h.Sum64()
}

// writeHashMap 对键值对哈希求和，与 map 遍历顺序无关且不需要为排序分配内存。
func writeHashMap[M ~map[string]any](h *maphash.Hash, value M) {
	writeHashUint(h, uint64(len(value)))
	var sum uint64
	for key, item := range value {
		var entry maphash.Hash
		entry.SetSeed(historyGroupHashSeed)
		writeHashString(&entry, key)
		writeHashUint(&entry, hashHistoryValue(item))
		sum += entry.Sum64()
	}
	writeHashUint(h, sum)
}

func writeHashString(h *maphash.Hash, value string) {
	writeHashUint(h, uint64(len(value)))
	h.WriteString(value)
}

func writeHashUint(h *maphash.Hash, value uint64) {
	var buffer [8]byte
	binary.LittleEndian.PutUint64(buffer[:], value)
	h.Write(buffer[:])
}

var historyHashNonces atomic.Uint64

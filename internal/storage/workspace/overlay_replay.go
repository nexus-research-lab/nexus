// INPUT: 只追加的 session overlay.jsonl、上次回放结束处的文件身份/尺寸/修改时间/首尾指纹与已回放的投影状态。
// OUTPUT: 只解析新增完整行得到的 overlay 历史行/marker 状态与 DM round index，结果与整份重读一致。
// POS: session overlay 两条读取链路共用的进程内增量回放；无法确认纯追加、末行未写完或新行覆盖已回放的 message_id 时整份重读。
package workspace

import (
	"context"
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	// maxOverlayReplaySessions 是同时保留回放状态的 overlay 个数上限。
	maxOverlayReplaySessions = 64
	// overlayReplayBudgetBytes 是全部回放状态对应的 overlay 源文件总字节上限。
	overlayReplayBudgetBytes = 32 * 1024 * 1024
)

// overlayReplayCursor 记录一次回放结束的位置。只有同一文件、cursor 尺寸处的边缘指纹未变、
// 且 cursor 落在完整行边界时，后续字节才能在已有状态上续放。
type overlayReplayCursor struct {
	FileIdentity    string
	Size            int64
	ModifiedUnixNS  int64
	EdgeFingerprint string
}

// overlayReplaySpan 是本次需要解析的字节范围。Resumed 表示范围从上次 cursor 开始，
// 已有状态继续有效；Next 非空表示范围末尾可作为下一次续放的起点。
type overlayReplaySpan struct {
	Reader  *io.SectionReader
	Resumed bool
	Next    *overlayReplayCursor
}

func openOverlayReplaySpan(
	ctx context.Context,
	file *os.File,
	previous *overlayReplayCursor,
) (overlayReplaySpan, error) {
	info, err := file.Stat()
	if err != nil {
		return overlayReplaySpan{}, err
	}
	size := info.Size()
	span := overlayReplaySpan{Reader: io.NewSectionReader(file, 0, size)}
	identity := historyPageFileIdentity(file, info)
	if identity == "" || size > overlayReplayBudgetBytes {
		return span, nil
	}
	if size > 0 {
		// 末行仍在写入时，下次从这里续放会把半行当成整行；这类读取不留状态。
		var last [1]byte
		if _, err = file.ReadAt(last[:], size-1); err != nil || last[0] != '\n' {
			return span, nil
		}
	}
	fingerprint, err := historyPageFileEdgeFingerprint(ctx, file, size)
	if err != nil {
		return overlayReplaySpan{}, err
	}
	span.Next = &overlayReplayCursor{
		FileIdentity:    identity,
		Size:            size,
		ModifiedUnixNS:  info.ModTime().UnixNano(),
		EdgeFingerprint: fingerprint,
	}
	if previous == nil || previous.FileIdentity != identity || previous.Size > size ||
		(previous.Size == size && previous.ModifiedUnixNS != span.Next.ModifiedUnixNS) {
		return span, nil
	}
	prefix, err := historyPageFileEdgeFingerprint(ctx, file, previous.Size)
	if err != nil {
		return overlayReplaySpan{}, err
	}
	if prefix != previous.EdgeFingerprint {
		return span, nil
	}
	span.Reader = io.NewSectionReader(file, previous.Size, size-previous.Size)
	span.Resumed = true
	return span, nil
}

// restart 放弃续放，改为整份重读到同一终点。
func (s overlayReplaySpan) restart(file *os.File) overlayReplaySpan {
	s.Reader = io.NewSectionReader(file, 0, s.Next.Size)
	s.Resumed = false
	return s
}

// overlayHistoryReplay 保存 overlay 历史行与 round marker 的回放状态。
type overlayHistoryReplay struct {
	cursor *overlayReplayCursor
	state  overlayHistoryState
}

func (r *overlayHistoryReplay) read(ctx context.Context, file *os.File) (overlayHistoryState, error) {
	span, err := openOverlayReplaySpan(ctx, file, r.cursor)
	if err != nil {
		return overlayHistoryState{}, err
	}
	rows, err := readJSONLReaderWithContext(ctx, span.Reader)
	if err != nil {
		return overlayHistoryState{}, err
	}
	state := overlayHistoryStateFromRows(rows)
	if span.Resumed {
		state.MessageRows = append(r.state.MessageRows, state.MessageRows...)
		state.RoundMarkers = append(r.state.RoundMarkers, state.RoundMarkers...)
	}
	r.cursor, r.state = span.Next, state
	if span.Next == nil {
		r.state = overlayHistoryState{}
	}
	// 截断容量：调用方 append 时不会写进缓存继续使用的底层数组。
	return overlayHistoryState{
		MessageRows:  slices.Clip(state.MessageRows),
		RoundMarkers: slices.Clip(state.RoundMarkers),
	}, nil
}

// overlayRoundIndexReplay 保存 DM round index 的回放状态：各 round 的累加器、已应用行的
// message_id，以及按导航顺序排好的条目。条目不含 active 标记，读取时再叠加。
type overlayRoundIndexReplay struct {
	cursor         *overlayReplayCursor
	defaultAgentID string
	entries        map[string]*sessionRoundIndexAccumulator
	messageIDs     map[string]struct{}
	items          []protocol.SessionRoundIndexItem
	positions      map[string]int
}

func (r *overlayRoundIndexReplay) read(
	ctx context.Context,
	file *os.File,
	activeRoundIDs map[string]struct{},
	defaultAgentID string,
) (protocol.SessionRoundIndex, error) {
	previous := r.cursor
	if r.defaultAgentID != defaultAgentID {
		previous = nil
	}
	span, err := openOverlayReplaySpan(ctx, file, previous)
	if err != nil {
		return protocol.SessionRoundIndex{}, err
	}
	rows, err := decodeRoundIndexOverlayRows(ctx, span.Reader)
	if err != nil {
		return protocol.SessionRoundIndex{}, err
	}
	if span.Resumed && r.overwritesApplied(rows) {
		// 新行按 message_id 覆盖了已应用的旧行，累加器无法撤销旧行的贡献。
		span = span.restart(file)
		if rows, err = decodeRoundIndexOverlayRows(ctx, span.Reader); err != nil {
			return protocol.SessionRoundIndex{}, err
		}
	}
	if !span.Resumed {
		r.entries = make(map[string]*sessionRoundIndexAccumulator)
		r.messageIDs = make(map[string]struct{})
		r.items, r.positions = nil, nil
	}
	r.apply(rows, defaultAgentID)
	items := slices.Clone(r.items)
	for index := range items {
		// 截断容量：调用方 append 时不会写进累加器继续使用的底层数组。
		items[index].AgentIDs = slices.Clip(items[index].AgentIDs)
	}
	markActiveRoundIndexItems(items, activeRoundIDs)
	r.cursor, r.defaultAgentID = span.Next, defaultAgentID
	if span.Next == nil {
		r.entries, r.messageIDs, r.items, r.positions = nil, nil, nil, nil
	}
	return protocol.SessionRoundIndex{Items: items}, nil
}

func (r *overlayRoundIndexReplay) overwritesApplied(rows []roundIndexOverlayJSONRow) bool {
	for _, row := range rows {
		if _, ok := r.messageIDs[strings.TrimSpace(row.MessageID)]; ok {
			return true
		}
	}
	return false
}

// apply 把新行累加进各 round，并只调整被触及条目在 items 中的位置；
// 一旦某个条目不再落在原位置或末尾，就按全部累加器重新排序。
func (r *overlayRoundIndexReplay) apply(rows []roundIndexOverlayJSONRow, defaultAgentID string) {
	resort := r.items == nil
	for _, row := range rows {
		if messageID := strings.TrimSpace(row.MessageID); messageID != "" {
			r.messageIDs[messageID] = struct{}{}
		}
		entry := row.applyToIndex(r.entries, nil, false, defaultAgentID)
		if entry == nil || resort {
			continue
		}
		item := entry.item
		position, known := r.positions[item.RoundID]
		switch {
		case known && r.items[position].Timestamp == item.Timestamp:
			r.items[position] = item
		case !known && (len(r.items) == 0 || roundIndexItemBefore(r.items[len(r.items)-1], item)):
			r.positions[item.RoundID] = len(r.items)
			r.items = append(r.items, item)
		default:
			resort = true
		}
	}
	if !resort {
		return
	}
	r.items = buildSessionRoundIndex(r.entries).Items
	r.positions = make(map[string]int, len(r.items))
	for position, item := range r.items {
		r.positions[item.RoundID] = position
	}
}

// markActiveRoundIndexItems 叠加 active 标记。DM 的 round_id 不折叠，逐行标记的最终结果
// 等价于把仍在 active 集合中的条目标成运行中。
func markActiveRoundIndexItems(items []protocol.SessionRoundIndexItem, activeRoundIDs map[string]struct{}) {
	if len(activeRoundIDs) == 0 {
		return
	}
	for index := range items {
		if _, ok := activeRoundIDs[items[index].RoundID]; ok {
			items[index].IsLive = true
			items[index].Status = string(roundStatusRunning)
		}
	}
}

// overlayReplayEntry 是一个 overlay 文件的全部回放状态；mu 串行化同一文件的回放。
type overlayReplayEntry struct {
	mu         sync.Mutex
	history    overlayHistoryReplay
	roundIndex overlayRoundIndexReplay
}

// retainedBytesLocked 返回仍保留回放状态的源文件字节数。
func (e *overlayReplayEntry) retainedBytesLocked() int64 {
	retained := int64(0)
	for _, cursor := range []*overlayReplayCursor{e.history.cursor, e.roundIndex.cursor} {
		if cursor != nil {
			retained = max(retained, cursor.Size)
		}
	}
	return retained
}

// overlayReplayStore 按最近使用顺序保留回放状态，并限制个数与对应源文件的总字节。
type overlayReplayStore struct {
	mu      sync.Mutex
	entries map[string]*overlayReplayEntry
	weights map[string]int64
	order   []string
}

// replaySessionOverlay 打开 session overlay 并在它的回放状态上执行一次读取。
// overlay 不存在时丢弃回放状态并返回 os.ErrNotExist。
func (s *AgentHistoryStore) replaySessionOverlay(
	workspacePath string,
	sessionKey string,
	read func(entry *overlayReplayEntry, file *os.File) error,
) error {
	overlayPath := s.paths.SessionOverlayPath(workspacePath, sessionKey)
	key := s.ownerUserID + "\x00" + overlayPath
	file, err := openWorkspaceFileAt(s.files, workspacePath, overlayPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.cache.overlays.forget(key)
		}
		return err
	}
	defer file.Close()
	return s.cache.overlays.replay(key, func(entry *overlayReplayEntry) error {
		return read(entry, file)
	})
}

// replay 在 key 对应的回放状态上执行一次读取，随后按保留的源字节数结算淘汰。
func (s *overlayReplayStore) replay(key string, read func(*overlayReplayEntry) error) error {
	entry := s.entry(key)
	entry.mu.Lock()
	err := read(entry)
	retained := entry.retainedBytesLocked()
	entry.mu.Unlock()
	s.settle(key, entry, retained)
	return err
}

func (s *overlayReplayStore) entry(key string) *overlayReplayEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.entries == nil {
		s.entries = make(map[string]*overlayReplayEntry)
		s.weights = make(map[string]int64)
	}
	entry := s.entries[key]
	if entry == nil {
		entry = &overlayReplayEntry{}
		s.entries[key] = entry
	}
	s.order = append(slices.DeleteFunc(s.order, func(item string) bool { return item == key }), key)
	return entry
}

// settle 记录一次回放后保留的源字节数，并从最久未用的状态开始淘汰直到回到上限内。
func (s *overlayReplayStore) settle(key string, entry *overlayReplayEntry, retainedBytes int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.entries[key] != entry {
		return
	}
	s.weights[key] = retainedBytes
	total := int64(0)
	for _, weight := range s.weights {
		total += weight
	}
	for len(s.order) > 0 && (len(s.order) > maxOverlayReplaySessions || total > overlayReplayBudgetBytes) {
		total -= s.weights[s.order[0]]
		s.removeLocked(s.order[0])
	}
}

func (s *overlayReplayStore) forget(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeLocked(key)
}

func (s *overlayReplayStore) removeLocked(key string) {
	delete(s.entries, key)
	delete(s.weights, key)
	s.order = slices.DeleteFunc(s.order, func(item string) bool { return item == key })
}

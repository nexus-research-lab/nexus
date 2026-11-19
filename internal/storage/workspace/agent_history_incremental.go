// INPUT: 上次发布的 DM 读模型尾轮状态、纯追加的 transcript/overlay 与投影检查点。
// OUTPUT: 只重新规范化最后一轮及新增轮次，并在同一 generation 内原子替换尾部分组。
// POS: DM 历史每轮刷新的增量路径；无法证明前缀不受影响时一律回退全量重建。
package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// 每个 scope 一份尾轮状态，主要开销是前缀的身份集合（每行几个短字符串）。
const maxAgentHistoryTails = 64

// 尾部最多覆盖的分组数；正常只有最后一两轮，超过说明 transcript 尾部跨了很多轮，直接全量重建。
const maxAgentHistoryTailGroups = 16

const (
	historyRowSourceTranscript = iota
	historyRowSourceMarker
	historyRowSourceOverlay
	historyRowSourceCount
)

var (
	errAgentHistoryTailUnavailable = errors.New("agent history tail unavailable")
	// errAgentHistoryTailRetry 表示本次读取与写入撞车，尾轮状态本身仍然有效。
	errAgentHistoryTailRetry = errors.New("agent history sources changed during refresh")
)

// historyRowKeys 收集规范化阶段会跨行关联的身份：compact 与投递回执按 message_id，
// 未完成轮次、result 配对与分组按 round/group key，引导去重按 source_round_id。
// 两组原始行的身份集合互不相交时，它们的规范化结果互不影响。
type historyRowKeys struct {
	messages map[string]struct{}
	groups   map[string]struct{}
	guidance map[string]struct{}
}

func newHistoryRowKeys() historyRowKeys {
	return historyRowKeys{
		messages: make(map[string]struct{}),
		groups:   make(map[string]struct{}),
		guidance: make(map[string]struct{}),
	}
}

func (k historyRowKeys) add(row protocol.Message) {
	putHistoryKey(k.messages, stringFromAny(row["message_id"]))
	putHistoryKey(k.groups, historyPageGroupKey(row, false))
	if stringFromAny(row[overlayKindField]) == overlayKindExternalDelivery {
		putHistoryKey(k.messages, stringFromAny(row["assistant_message_id"]))
	}
	if stringFromAny(row["role"]) == "user" {
		putHistoryKey(k.guidance, stringFromAny(row["source_round_id"]))
	}
	if metadata, ok := row["metadata"].(map[string]any); ok {
		putHistoryKey(k.guidance, stringFromAny(metadata["source_round_id"]))
	}
}

func (k historyRowKeys) merge(other historyRowKeys) {
	for _, pair := range [][2]map[string]struct{}{{k.messages, other.messages}, {k.groups, other.groups}, {k.guidance, other.guidance}} {
		for key := range pair[1] {
			pair[0][key] = struct{}{}
		}
	}
}

func (k historyRowKeys) intersects(other historyRowKeys) bool {
	return historyKeysIntersect(k.messages, other.messages) ||
		historyKeysIntersect(k.groups, other.groups) ||
		historyKeysIntersect(k.guidance, other.guidance)
}

func putHistoryKey(set map[string]struct{}, key string) {
	if key != "" {
		set[key] = struct{}{}
	}
}

func historyKeysIntersect(left, right map[string]struct{}) bool {
	if len(left) > len(right) {
		left, right = right, left
	}
	for key := range left {
		if _, ok := right[key]; ok {
			return true
		}
	}
	return false
}

// historyTailPart 是某个来源参与本次尾轮投影的原始行；前 frozen 行此后不会再变化，
// 其余（transcript 最后一个用户轮次之后）会在下次刷新时重新投影。
type historyTailPart struct {
	rows   []protocol.Message
	frozen int
}

// agentHistoryTail 绑定一个已发布的读模型状态（Sources 与 GroupCount）：
// 末尾 TailGroups 个分组之前的分组由 Prior 对应的原始行独立规范化得到，
// 末尾分组的冻结原始行保存在 Tail，下次刷新与新增行一起重新规范化。
type agentHistoryTail struct {
	Sources           []historyPageSourceSnapshot
	GroupCount        int
	TailGroups        int
	Epoch             uint64
	TranscriptFrozen  int
	MarkerCount       int
	OverlayCount      int
	Tail              [historyRowSourceCount][]protocol.Message
	Prior             historyRowKeys
	PriorMaxTimestamp int64
	HasPrior          bool
	// added 是切分时新并入前缀的身份，发布时才合入 Prior：刷新中途失败时旧状态不受影响。
	added historyRowKeys
	// retries 统计连续撞上写入而放回的次数，超过上限后放弃状态，避免反复白读。
	retries int
}

const maxAgentHistoryTailRetries = 3

type agentHistoryTailStore struct {
	mu      sync.Mutex
	entries map[string]*agentHistoryTail
	order   []string
	// refreshed 统计成功的增量刷新次数，用于诊断是否走了快路径。
	refreshed atomic.Uint64
}

// take 取出并移除状态：刷新失败时状态随之作废，由下一次全量重建重新建立。
func (s *agentHistoryTailStore) take(scope string) *agentHistoryTail {
	s.mu.Lock()
	defer s.mu.Unlock()
	tail := s.entries[scope]
	s.removeLocked(scope)
	return tail
}

func (s *agentHistoryTailStore) store(scope string, tail *agentHistoryTail) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.entries == nil {
		s.entries = make(map[string]*agentHistoryTail)
	}
	s.removeLocked(scope)
	s.entries[scope] = tail
	s.order = append(s.order, scope)
	for len(s.order) > maxAgentHistoryTails {
		s.removeLocked(s.order[0])
	}
}

func (s *agentHistoryTailStore) removeLocked(scope string) {
	delete(s.entries, scope)
	s.order = slices.DeleteFunc(s.order, func(item string) bool { return item == scope })
}

// splitAgentHistoryTail 在尾部分组之前切开 parts，得到下一次刷新的尾轮状态。尾部从最后一组
// 或任何仍会被重新投影的行所在分组（取更早者）开始：新一轮 marker 往往先于 transcript 落盘，
// 此时 transcript 尾部仍属于上一组。只有两侧身份不相交、时间严格先后、且尾部分组能由
// 尾部原始行单独重建时才成立；此时完整规范化结果等于“前缀规范化 ++ 尾部规范化”。
// prior 只在成功时被扩充。
func splitAgentHistoryTail(
	ctx context.Context,
	parts [historyRowSourceCount]historyTailPart,
	groups []historyPageIndexedGroup,
	prior historyRowKeys,
	priorMax int64,
	hasPrior bool,
) (*agentHistoryTail, bool) {
	if len(groups) == 0 {
		return nil, false
	}
	cut := agentHistoryTailCut(parts, groups)
	if len(groups)-cut > maxAgentHistoryTailGroups {
		return nil, false
	}
	if cut > 0 {
		previous := groups[cut-1].Items
		priorMax, hasPrior = messageTimestamp(previous[len(previous)-1]), true
	}
	if hasPrior && priorMax >= messageTimestamp(groups[cut].Items[0]) {
		return nil, false
	}
	inTail := make(map[string]struct{}, len(groups)-cut)
	for _, group := range groups[cut:] {
		inTail[group.GroupKey] = struct{}{}
	}
	added, tailKeys := newHistoryRowKeys(), newHistoryRowKeys()
	next := &agentHistoryTail{TailGroups: len(groups) - cut}
	tailRows := make([]protocol.Message, 0)
	for source, part := range parts {
		for index, row := range part.rows {
			key := historyPageGroupKey(row, false)
			mutable := index >= part.frozen
			if _, ok := inTail[key]; ok || (mutable && key != "") {
				tailKeys.add(row)
				tailRows = append(tailRows, row)
				if !mutable {
					next.Tail[source] = append(next.Tail[source], row)
				}
				continue
			}
			if !mutable {
				added.add(row)
			}
		}
	}
	if added.intersects(tailKeys) || prior.intersects(tailKeys) {
		return nil, false
	}
	rebuilt, err := buildHistoryPageIndexedGroups(ctx, tailRows, false)
	if err != nil || !reflect.DeepEqual(rebuilt, groups[cut:]) {
		return nil, false
	}
	next.Prior, next.added = prior, added
	next.PriorMaxTimestamp, next.HasPrior = priorMax, hasPrior
	return next, true
}

// publish 把切分时新并入前缀的身份合入 Prior 后登记状态。Prior 与上一个状态共享，
// 调用方此时已独占且不再使用上一个状态（take 取出，读模型已推进）。
func (s *agentHistoryTailStore) publish(scope string, tail *agentHistoryTail) {
	tail.Prior.merge(tail.added)
	tail.added = historyRowKeys{}
	s.store(scope, tail)
}

// restore 把仍然有效的状态放回；期间已有新状态登记时以新状态为准。
func (s *agentHistoryTailStore) restore(scope string, tail *agentHistoryTail) {
	s.mu.Lock()
	_, exists := s.entries[scope]
	s.mu.Unlock()
	if !exists {
		s.store(scope, tail)
	}
}

// agentHistoryTailCut 返回尾部第一个分组的下标：最后一组，或仍会被重新投影的行所在的最早分组。
func agentHistoryTailCut(parts [historyRowSourceCount]historyTailPart, groups []historyPageIndexedGroup) int {
	index := make(map[string]int, len(groups))
	for position, group := range groups {
		index[group.GroupKey] = position
	}
	cut := len(groups) - 1
	for _, part := range parts {
		for _, row := range part.rows[min(part.frozen, len(part.rows)):] {
			if position, ok := index[historyPageGroupKey(row, false)]; ok {
				cut = min(cut, position)
			}
		}
	}
	return cut
}

// rememberAgentHistoryTail 在全量重建得到可缓存结果后建立尾轮状态。
func (s *AgentHistoryStore) rememberAgentHistoryTail(
	ctx context.Context,
	scope string,
	parts agentHistoryRowParts,
	groups []historyPageIndexedGroup,
	sources []historyPageSourceSnapshot,
) {
	if scope == "" || !parts.Tracked || !s.transcriptProjectionEpochCached(parts.Projection.Epoch) {
		return
	}
	tail, ok := splitAgentHistoryTail(ctx, [historyRowSourceCount]historyTailPart{
		historyRowSourceTranscript: {rows: parts.Transcript, frozen: parts.Projection.Frozen},
		historyRowSourceMarker:     {rows: parts.Markers, frozen: len(parts.Markers)},
		historyRowSourceOverlay:    {rows: parts.Overlay, frozen: len(parts.Overlay)},
	}, groups, newHistoryRowKeys(), 0, false)
	if !ok {
		return
	}
	tail.Sources, tail.GroupCount = sources, len(groups)
	tail.Epoch, tail.TranscriptFrozen = parts.Projection.Epoch, parts.Projection.Frozen
	tail.MarkerCount, tail.OverlayCount = len(parts.Markers), len(parts.Overlay)
	s.cache.tails.publish(scope, tail)
}

func (s *AgentHistoryStore) refreshAgentHistoryTail(
	ctx context.Context,
	workspacePath string,
	sessionValue protocol.Session,
	scopeKey string,
) (historyPageIndexBuild, bool) {
	// take 让本次刷新独占状态（Prior 共享且会在发布时扩充），失败时按原因决定是否放回。
	tail := s.cache.tails.take(scopeKey)
	built, err := s.tryRefreshAgentHistoryTail(ctx, workspacePath, sessionValue, scopeKey, tail)
	if err == nil {
		return built, true
	}
	// 撞上并发写入或请求取消时状态仍能证明前缀不变，放回供下一次刷新使用。
	if errors.Is(err, errAgentHistoryTailRetry) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		if tail.retries < maxAgentHistoryTailRetries {
			tail.retries++
			s.cache.tails.restore(scopeKey, tail)
		}
		return historyPageIndexBuild{}, false
	}
	if !errors.Is(err, errAgentHistoryTailUnavailable) {
		logx.FromContext(ctx).Info("DM 历史增量读取回退重建", "session_key", sessionValue.SessionKey, "error", err)
	}
	return historyPageIndexBuild{}, false
}

func (s *AgentHistoryStore) tryRefreshAgentHistoryTail(
	ctx context.Context,
	workspacePath string,
	sessionValue protocol.Session,
	scopeKey string,
	tail *agentHistoryTail,
) (historyPageIndexBuild, error) {
	started := time.Now()
	// 检查点被淘汰时不必先读一遍历史再失败，直接交给全量重建。
	if tail == nil || !s.transcriptProjectionEpochCached(tail.Epoch) {
		return historyPageIndexBuild{}, errAgentHistoryTailUnavailable
	}
	db, err := s.readModel.database(ctx)
	if err != nil {
		return historyPageIndexBuild{}, err
	}
	previous, ok, err := readHistoryReadModelScope(ctx, db, scopeKey)
	if err != nil {
		return historyPageIndexBuild{}, err
	}
	if !ok || previous.DisabledReason != "" || previous.GroupCount != tail.GroupCount || !snapshotsEqual(previous.Sources, tail.Sources) {
		return historyPageIndexBuild{}, fmt.Errorf("published generation moved: %w", errHistoryPageIndexInvalid)
	}
	current, err := s.collectAgentHistoryPageSources(ctx, workspacePath, sessionValue)
	if err != nil {
		return historyPageIndexBuild{}, err
	}
	if err = s.checkAgentHistorySourcesAppended(ctx, workspacePath, sessionValue.SessionKey, previous.Sources, current); err != nil {
		return historyPageIndexBuild{}, err
	}
	parts, err := s.readHistoryRowPartsContext(ctx, workspacePath, sessionValue, tail.TranscriptFrozen)
	if err != nil {
		return historyPageIndexBuild{}, err
	}
	// 读取 transcript 期间 runtime 恰好写入时解析结果不进缓存、没有检查点，属于撞车而非前缀变化。
	if !parts.Tracked {
		return historyPageIndexBuild{}, errAgentHistoryTailRetry
	}
	if parts.Projection.Epoch != tail.Epoch || parts.Projection.Stable < tail.TranscriptFrozen ||
		len(parts.Markers) < tail.MarkerCount || len(parts.Overlay) < tail.OverlayCount {
		return historyPageIndexBuild{}, fmt.Errorf("projection prefix changed: %w", errHistoryPageIndexInvalid)
	}
	candidate := [historyRowSourceCount]historyTailPart{
		historyRowSourceTranscript: {
			rows:   append(append([]protocol.Message(nil), tail.Tail[historyRowSourceTranscript]...), parts.Transcript...),
			frozen: len(tail.Tail[historyRowSourceTranscript]) + parts.Projection.Frozen - tail.TranscriptFrozen,
		},
		historyRowSourceMarker:  appendedHistoryTailPart(tail.Tail[historyRowSourceMarker], parts.Markers[tail.MarkerCount:]),
		historyRowSourceOverlay: appendedHistoryTailPart(tail.Tail[historyRowSourceOverlay], parts.Overlay[tail.OverlayCount:]),
	}
	touched := newHistoryRowKeys()
	rows := make([]protocol.Message, 0)
	for _, part := range candidate {
		for _, row := range part.rows {
			touched.add(row)
		}
		rows = append(rows, part.rows...)
	}
	// 新增或重投影的行一旦触及前缀的身份（迟到的旧轮 result、跨轮引导等），就需要全局规范化。
	if tail.Prior.intersects(touched) {
		return historyPageIndexBuild{}, fmt.Errorf("append touches earlier rounds: %w", errHistoryPageIndexInvalid)
	}
	groups, err := buildHistoryPageIndexedGroups(ctx, rows, false)
	if err != nil {
		return historyPageIndexBuild{}, err
	}
	if len(groups) == 0 || (tail.HasPrior && messageTimestamp(groups[0].Items[0]) <= tail.PriorMaxTimestamp) {
		return historyPageIndexBuild{}, fmt.Errorf("tail reorders before earlier rounds: %w", errHistoryPageIndexInvalid)
	}
	next, nextOK := splitAgentHistoryTail(ctx, candidate, groups, tail.Prior, tail.PriorMaxTimestamp, tail.HasPrior)
	roundIndex, err := s.readCanonicalRoundIndexContext(ctx, workspacePath, sessionValue, nil)
	if err != nil {
		return historyPageIndexBuild{}, err
	}
	after, err := s.collectAgentHistoryPageSources(ctx, workspacePath, sessionValue)
	if err != nil {
		return historyPageIndexBuild{}, err
	}
	if !snapshotsEqual(after, current) {
		return historyPageIndexBuild{}, errAgentHistoryTailRetry
	}
	start := previous.GroupCount - tail.TailGroups
	if err = replaceHistoryReadModelTail(ctx, db, scopeKey, previous, start, groups, current, roundIndex.Items, nil); err != nil {
		return historyPageIndexBuild{}, err
	}
	s.cache.tails.refreshed.Add(1)
	if nextOK {
		next.Sources, next.GroupCount = current, start+len(groups)
		next.Epoch, next.TranscriptFrozen = parts.Projection.Epoch, parts.Projection.Frozen
		next.MarkerCount, next.OverlayCount = len(parts.Markers), len(parts.Overlay)
		s.cache.tails.publish(scopeKey, next)
	}
	logx.FromContext(ctx).Info("DM 历史增量更新完成",
		"session_key", sessionValue.SessionKey,
		"replaced_groups", len(groups),
		"unchanged_groups", start,
		"tail_rows", len(rows),
		"next_tail", nextOK,
		"duration_ms", time.Since(started).Milliseconds(),
	)
	return historyPageIndexBuild{Sources: current, RoundIndex: roundIndex.Items, Cacheable: true}, nil
}

func appendedHistoryTailPart(tail []protocol.Message, appended []protocol.Message) historyTailPart {
	rows := append(append([]protocol.Message(nil), tail...), appended...)
	return historyTailPart{rows: rows, frozen: len(rows)}
}

// checkAgentHistorySourcesAppended 只接受同一文件的纯追加。transcript 前缀由解析缓存代系与
// 投影检查点 Epoch 证明；overlay 额外校验旧尺寸处的边缘指纹与完整行边界。
func (s *AgentHistoryStore) checkAgentHistorySourcesAppended(
	ctx context.Context,
	workspacePath string,
	sessionKey string,
	previous []historyPageSourceSnapshot,
	current []historyPageSourceSnapshot,
) error {
	if len(previous) != len(current) {
		return fmt.Errorf("source set changed: %w", errHistoryPageIndexInvalid)
	}
	for index, old := range previous {
		next := current[index]
		if historyPageSourceKey(old) != historyPageSourceKey(next) || old.Exists != next.Exists {
			return fmt.Errorf("source replaced: %w", errHistoryPageIndexInvalid)
		}
		if !old.Exists {
			continue
		}
		if old.FileIdentity != next.FileIdentity || next.Size < old.Size {
			return fmt.Errorf("source replaced or truncated: %w", errHistoryPageIndexInvalid)
		}
		if old.Kind == historyPageSourceDMOverlay && old != next {
			if err := s.checkAgentHistoryOverlayPrefix(ctx, workspacePath, sessionKey, old); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *AgentHistoryStore) checkAgentHistoryOverlayPrefix(
	ctx context.Context,
	workspacePath string,
	sessionKey string,
	old historyPageSourceSnapshot,
) error {
	root, err := s.files.openWorkspaceRoot(workspacePath, false)
	if err != nil {
		return err
	}
	defer root.Close()
	relative := filepath.ToSlash(filepath.Join(".agents", "sessions", encodeSessionDirName(sessionKey), "overlay.jsonl"))
	file, err := root.OpenFileNoSymlink(relative, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	fingerprint, err := historyPageFileEdgeFingerprint(ctx, file, old.Size)
	if err != nil || fingerprint != old.EdgeFingerprint {
		return fmt.Errorf("overlay prefix changed: %w", errHistoryPageIndexInvalid)
	}
	if old.Size == 0 {
		return nil
	}
	var boundary [1]byte
	if _, err = file.ReadAt(boundary[:], old.Size-1); err != nil || boundary[0] != '\n' {
		return fmt.Errorf("overlay prefix ends mid-line: %w", errHistoryPageIndexInvalid)
	}
	return nil
}

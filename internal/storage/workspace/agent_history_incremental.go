// INPUT: 读模型里上次发布的 DM 尾轮状态与 transcript 检查点、纯追加的 transcript/overlay。
// OUTPUT: 只重新投影并规范化最后一轮及新增轮次（marker 行也只物化这一部分），在同一 generation 内原子替换尾部分组并推进持久化状态。
// POS: DM 历史每轮刷新的增量路径，进程重启后同样成立；无法证明前缀不受影响时一律回退全量重建。
package workspace

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// 尾部最多覆盖的分组数；正常只有最后一两轮，超过说明 transcript 尾部跨了很多轮，直接全量重建。
const maxAgentHistoryTailGroups = 16

var errAgentHistoryTailUnavailable = errors.New("agent history tail unavailable")

// agentHistoryCheckpoint 是随读模型一并发布的 DM 尾轮状态。末尾 TailGroups 个分组之前的分组
// 由已冻结的原始行独立规范化得到，此后不再变化；尾部分组在下次刷新时由三类原始行重新规范化：
// transcript 从 Transcript 检查点重放，marker 与 overlay 行取 Tail* 记录的下标加上新增的行
// （下标分别指 overlay 的全部 round marker 与全部消息行，两者按文件行顺序派生，纯追加时稳定）。
// JSON 形状改动需提升读模型版本。
type agentHistoryCheckpoint struct {
	TailGroups        int                  `json:"tail_groups"`
	MarkerCount       int                  `json:"marker_count"`
	OverlayCount      int                  `json:"overlay_count"`
	TailMarkers       []int                `json:"tail_markers"`
	TailOverlay       []int                `json:"tail_overlay"`
	PriorMaxTimestamp int64                `json:"prior_max_timestamp"`
	HasPrior          bool                 `json:"has_prior"`
	Transcript        transcriptCheckpoint `json:"transcript"`
}

// historyTailPart 是 marker 或 overlay 来源参与本次尾轮规范化的原始行：上次留在尾部分组里的行
// 加上此后新增的行。positions 是各行在该来源全部条目中的下标，total 是该来源当前的条目总数；
// marker 来源的条目是 round marker，其中隐藏的 marker 不产生行。
type historyTailPart struct {
	rows      []protocol.Message
	positions []int
	total     int
}

// wholeHistoryTailPart 用于完整重建：来源的全部行都参与。
func wholeHistoryTailPart(rows []protocol.Message) historyTailPart {
	positions, _ := appendedHistoryTailPositions(len(rows), nil, 0)
	return historyTailPart{rows: rows, positions: positions, total: len(rows)}
}

// appendedHistoryTailPart 取上次尾部的行（按下标）与 count 之后新增的行。
func appendedHistoryTailPart(rows []protocol.Message, tail []int, count int) (historyTailPart, bool) {
	positions, ok := appendedHistoryTailPositions(len(rows), tail, count)
	if !ok {
		return historyTailPart{}, false
	}
	part := historyTailPart{positions: positions, total: len(rows)}
	for _, position := range positions {
		part.rows = append(part.rows, rows[position])
	}
	return part, true
}

// appendedHistoryTailPositions 返回上次尾部的下标与 count 之后新增条目的下标。
func appendedHistoryTailPositions(total int, tail []int, count int) ([]int, bool) {
	if count > total {
		return nil, false
	}
	positions := make([]int, 0, len(tail)+total-count)
	for _, position := range tail {
		if position < 0 || position >= count {
			return nil, false
		}
		positions = append(positions, position)
	}
	for position := count; position < total; position++ {
		positions = append(positions, position)
	}
	return positions, true
}

// splitAgentHistoryTail 在尾部分组之前切开本次参与规范化的原始行，得到下一次刷新的尾轮状态
// 以及因此新冻结的身份键。尾部从最后一组或任何仍会被重新投影的行所在分组（取更早者）开始：
// 新一轮 marker 往往先于 transcript 落盘，此时 transcript 尾部仍属于上一组。transcript 的
// 冻结边界是某个轮次起点；若起点之前的行仍落在尾部分组里（同一轮内的后续输入），就退到更早的
// 起点，使尾部分组的 transcript 行全部可由检查点重放。只有两侧身份不相交、时间严格先后、
// 且尾部分组能由尾部原始行单独重建时才成立；此时完整规范化结果等于“前缀规范化 ++ 尾部规范化”。
func splitAgentHistoryTail(
	ctx context.Context,
	replay *transcriptReplay,
	markers historyTailPart,
	overlay historyTailPart,
	groups []historyPageIndexedGroup,
	priorMax int64,
	hasPrior bool,
) (*agentHistoryCheckpoint, historyTailKeys, bool) {
	if replay == nil || len(groups) == 0 {
		return nil, nil, false
	}
	start, cut, ok := agentHistoryTailCut(replay, groups)
	if !ok || len(groups)-cut > maxAgentHistoryTailGroups {
		return nil, nil, false
	}
	if cut > 0 {
		previous := groups[cut-1].Items
		priorMax, hasPrior = messageTimestamp(previous[len(previous)-1]), true
	}
	if hasPrior && priorMax >= messageTimestamp(groups[cut].Items[0]) {
		return nil, nil, false
	}
	inTail := make(map[string]struct{}, len(groups)-cut)
	for _, group := range groups[cut:] {
		inTail[group.GroupKey] = struct{}{}
	}
	frozen, tailKeys := make(historyTailKeys), make(historyTailKeys)
	tailRows := make([]protocol.Message, 0)
	keep := func(row protocol.Message, mutable bool) bool {
		key := historyPageGroupKey(row, false)
		if _, ok := inTail[key]; ok || (mutable && key != "") {
			tailKeys.addRow(row)
			tailRows = append(tailRows, row)
			return true
		}
		if !mutable {
			frozen.addRow(row)
		}
		return false
	}
	next := &agentHistoryCheckpoint{
		TailGroups:   len(groups) - cut,
		MarkerCount:  markers.total,
		OverlayCount: overlay.total,
		TailMarkers:  []int{},
		TailOverlay:  []int{},
	}
	for index, row := range replay.Rows {
		keep(row, index >= replay.Starts[start].Rows)
	}
	for index, row := range markers.rows {
		if keep(row, false) {
			next.TailMarkers = append(next.TailMarkers, markers.positions[index])
		}
	}
	for index, row := range overlay.rows {
		if keep(row, false) {
			next.TailOverlay = append(next.TailOverlay, overlay.positions[index])
		}
	}
	if frozen.intersects(tailKeys) {
		return nil, nil, false
	}
	rebuilt, err := buildHistoryPageIndexedGroups(ctx, tailRows, false)
	if err != nil || !reflect.DeepEqual(rebuilt, groups[cut:]) {
		return nil, nil, false
	}
	checkpoint, entries, ok := replay.checkpoint(start)
	if !ok {
		return nil, nil, false
	}
	for key := range entries {
		frozen[key] = struct{}{}
	}
	next.Transcript = checkpoint
	next.PriorMaxTimestamp, next.HasPrior = priorMax, hasPrior
	return next, frozen, true
}

// agentHistoryTailCut 选择 transcript 的冻结起点（replay.Starts 的下标）并返回尾部第一个分组的下标：
// 最后一组，或仍会被重新投影的行所在的最早分组。从最后一个轮次起点开始，只要起点之前还有行
// 落在尾部分组里，就退到不晚于该行的起点；没有这样的起点时返回 false。
func agentHistoryTailCut(replay *transcriptReplay, groups []historyPageIndexedGroup) (start int, cut int, ok bool) {
	position := make(map[string]int, len(groups))
	for index, group := range groups {
		position[group.GroupKey] = index
	}
	for start = len(replay.Starts) - 1; start >= 0; {
		frozen := replay.Starts[start].Rows
		cut = len(groups) - 1
		for _, row := range replay.Rows[frozen:] {
			if index, exists := position[historyPageGroupKey(row, false)]; exists {
				cut = min(cut, index)
			}
		}
		first := -1
		for index, row := range replay.Rows[:frozen] {
			if group, exists := position[historyPageGroupKey(row, false)]; exists && group >= cut {
				first = index
				break
			}
		}
		if first < 0 {
			return start, cut, true
		}
		for start >= 0 && replay.Starts[start].Rows > first {
			start--
		}
	}
	return 0, 0, false
}

// newAgentHistoryTailState 在全量重建得到可缓存结果后建立尾轮状态；不可增量时返回 nil。
func newAgentHistoryTailState(
	ctx context.Context,
	parts agentHistoryRowParts,
	groups []historyPageIndexedGroup,
) (any, historyTailKeys) {
	next, frozen, ok := splitAgentHistoryTail(
		ctx, parts.Replay, parts.Markers, wholeHistoryTailPart(parts.Overlay), groups, 0, false,
	)
	if !ok {
		return nil, nil
	}
	return next, frozen
}

func (s *AgentHistoryStore) refreshAgentHistoryTail(
	ctx context.Context,
	workspacePath string,
	sessionValue protocol.Session,
	scopeKey string,
) (historyPageIndexBuild, bool) {
	built, err := s.tryRefreshAgentHistoryTail(ctx, workspacePath, sessionValue, scopeKey)
	if err != nil {
		if !errors.Is(err, errAgentHistoryTailUnavailable) && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			logx.FromContext(ctx).Info("DM 历史增量读取回退重建", "session_key", sessionValue.SessionKey, "error", err)
		}
		return historyPageIndexBuild{}, false
	}
	return built, true
}

func (s *AgentHistoryStore) tryRefreshAgentHistoryTail(
	ctx context.Context,
	workspacePath string,
	sessionValue protocol.Session,
	scopeKey string,
) (historyPageIndexBuild, error) {
	started := time.Now()
	db, err := s.readModel.database(ctx)
	if err != nil {
		return historyPageIndexBuild{}, err
	}
	previous, ok, err := readHistoryReadModelScope(ctx, db, scopeKey)
	if err != nil {
		return historyPageIndexBuild{}, err
	}
	// 从未发布、已禁用或上次没能留下尾轮状态的 scope 没有可续接的位置，直接交给全量重建。
	if !ok || previous.DisabledReason != "" {
		return historyPageIndexBuild{}, errAgentHistoryTailUnavailable
	}
	tail, err := readAgentHistoryCheckpoint(ctx, db, scopeKey, previous.Generation)
	if err != nil {
		return historyPageIndexBuild{}, err
	}
	if tail == nil || tail.TailGroups > previous.GroupCount {
		return historyPageIndexBuild{}, errAgentHistoryTailUnavailable
	}
	current, err := s.collectAgentHistoryPageSources(ctx, workspacePath, sessionValue)
	if err != nil {
		return historyPageIndexBuild{}, err
	}
	if err = s.checkAgentHistorySourcesAppended(ctx, workspacePath, sessionValue.SessionKey, previous.Sources, current); err != nil {
		return historyPageIndexBuild{}, err
	}
	// 检查点只在单个非 fork transcript 上建立；来源形状不符说明会话已换了读取方式。
	if _, forkMessageID := pendingForkTranscript(sessionValue); forkMessageID != "" ||
		len(current) != 2 || current[1].Kind != historyPageSourceTranscript {
		return historyPageIndexBuild{}, fmt.Errorf("transcript selection changed: %w", errHistoryPageIndexInvalid)
	}
	overlayState, err := s.readOverlayHistoryStateContext(ctx, workspacePath, sessionValue.SessionKey)
	if err != nil {
		return historyPageIndexBuild{}, err
	}
	// 只物化仍在尾部分组里的 marker 与新增的 marker，更早的 marker 行已经冻结在前缀分组里。
	markerPositions, markersOK := appendedHistoryTailPositions(len(overlayState.RoundMarkers), tail.TailMarkers, tail.MarkerCount)
	overlay, overlayOK := appendedHistoryTailPart(overlayState.MessageRows, tail.TailOverlay, tail.OverlayCount)
	if !markersOK || !overlayOK {
		return historyPageIndexBuild{}, fmt.Errorf("overlay rows shrank: %w", errHistoryPageIndexInvalid)
	}
	markers := roundMarkerHistoryTailPart(sessionValue.SessionKey, sessionValue.AgentID, overlayState.RoundMarkers, markerPositions)
	touchesFrozen := func(keys historyTailKeys) (bool, error) {
		return historyReadTailKeysContain(ctx, db, scopeKey, keys)
	}
	replay, err := s.resumeTranscriptProjection(
		ctx, workspacePath, sessionValue, previous.Sources[1], current[1], tail.Transcript, overlayState.RoundMarkers, touchesFrozen,
	)
	if err != nil {
		return historyPageIndexBuild{}, err
	}
	rows := make([]protocol.Message, 0, len(replay.Rows)+len(markers.rows)+len(overlay.rows))
	rows = append(append(append(rows, replay.Rows...), markers.rows...), overlay.rows...)
	touched := make(historyTailKeys)
	for _, row := range rows {
		touched.addRow(row)
	}
	// 新增或重投影的行一旦触及前缀的身份（迟到的旧轮 result、跨轮引导等），就需要全局规范化。
	if conflict, err := touchesFrozen(touched); err != nil {
		return historyPageIndexBuild{}, err
	} else if conflict {
		return historyPageIndexBuild{}, fmt.Errorf("append touches earlier rounds: %w", errHistoryPageIndexInvalid)
	}
	groups, err := buildHistoryPageIndexedGroups(ctx, rows, false)
	if err != nil {
		return historyPageIndexBuild{}, err
	}
	if len(groups) == 0 || (tail.HasPrior && messageTimestamp(groups[0].Items[0]) <= tail.PriorMaxTimestamp) {
		return historyPageIndexBuild{}, fmt.Errorf("tail reorders before earlier rounds: %w", errHistoryPageIndexInvalid)
	}
	next, frozen, nextOK := splitAgentHistoryTail(ctx, replay, markers, overlay, groups, tail.PriorMaxTimestamp, tail.HasPrior)
	roundIndex, err := s.readCanonicalRoundIndexContext(ctx, workspacePath, sessionValue, nil)
	if err != nil {
		return historyPageIndexBuild{}, err
	}
	after, err := s.collectAgentHistoryPageSources(ctx, workspacePath, sessionValue)
	if err != nil {
		return historyPageIndexBuild{}, err
	}
	if !snapshotsEqual(after, current) {
		return historyPageIndexBuild{}, fmt.Errorf("sources changed during refresh: %w", errHistoryPageIndexInvalid)
	}
	start := previous.GroupCount - tail.TailGroups
	// 分组、来源快照、尾轮状态与新冻结的身份键在同一事务内推进；中途失败时旧状态原样保留。
	err = s.readModel.replaceTail(ctx, scopeKey, previous, start, groups, current, roundIndex.Items, func(tx *sql.Tx) error {
		var state any
		if nextOK {
			state = next
		}
		if err := persistHistoryReadTail(ctx, tx, scopeKey, previous.Generation, nil, state); err != nil {
			return err
		}
		return insertHistoryReadTailKeys(ctx, tx, scopeKey, frozen)
	})
	if err != nil {
		return historyPageIndexBuild{}, err
	}
	s.cache.tailRefreshes.Add(1)
	logx.FromContext(ctx).Info("DM 历史增量更新完成",
		"session_key", sessionValue.SessionKey,
		"replaced_groups", len(groups),
		"unchanged_groups", start,
		"tail_rows", len(rows),
		"transcript_offset", tail.Transcript.Offset,
		"next_tail", nextOK,
		"duration_ms", time.Since(started).Milliseconds(),
	)
	return historyPageIndexBuild{Sources: current, RoundIndex: roundIndex.Items, Cacheable: true}, nil
}

// readAgentHistoryCheckpoint 读取 scope 当前 generation 的尾轮状态；上次发布没有留下状态时返回 nil。
func readAgentHistoryCheckpoint(ctx context.Context, db historyReadModelQuerier, scope, generation string) (*agentHistoryCheckpoint, error) {
	_, payload, err := readHistoryReadTail(ctx, db, scope, generation)
	if err != nil {
		return nil, err
	}
	var tail *agentHistoryCheckpoint
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&tail); err != nil {
		return nil, fmt.Errorf("tail state unreadable: %w", errHistoryPageIndexInvalid)
	}
	return tail, nil
}

// checkAgentHistorySourcesAppended 只接受同一文件的纯追加，DM 与 Room 成员的检查点续读共用。transcript 前缀由
// 续读检查点时的边缘指纹与起点条目校验证明；overlay 在这里校验旧尺寸处的边缘指纹与完整行边界。
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
		if (old.Kind == historyPageSourceDMOverlay || old.Kind == historyPageSourceRoomPrivateOverlay) && old != next {
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

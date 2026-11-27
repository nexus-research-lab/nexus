// INPUT: append-only overlay JSONL、共享 history generation 与当前活跃 round 集合。
// OUTPUT: 遵循 message_id 最后写入语义、支持短冷建的 DM/Room Round Navigator 元数据。
// POS: canonical overlay 导航投影与 SQLite 派生代际之间的唯一 round index 边界；DM 的 canonical 读取经 overlay_replay.go 只回放新增行。
package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/infra/textutil"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type sessionRoundIndexAccumulator struct {
	agentIDs map[string]struct{}
	item     protocol.SessionRoundIndexItem
}

type roundIndexOverlayJSONRow struct {
	AgentID        string                       `json:"agent_id"`
	Content        json.RawMessage              `json:"content"`
	DurationMS     json.RawMessage              `json:"duration_ms"`
	HiddenFromUser bool                         `json:"hidden_from_user"`
	MessageID      string                       `json:"message_id"`
	OverlayKind    string                       `json:"nexus_overlay_kind"`
	ResultSummary  *roundIndexJSONResultSummary `json:"result_summary"`
	Role           string                       `json:"role"`
	RoundID        string                       `json:"round_id"`
	Subtype        string                       `json:"subtype"`
	TargetRoundID  string                       `json:"target_round_id"`
	Timestamp      json.RawMessage              `json:"timestamp"`
}

type roundIndexJSONResultSummary struct {
	DurationMS json.RawMessage `json:"duration_ms"`
	Subtype    string          `json:"subtype"`
}

// ReadRoundIndex 读取 DM session 的轻量 round 导航索引。
func (s *AgentHistoryStore) ReadRoundIndex(
	workspacePath string,
	sessionValue protocol.Session,
	activeRoundIDs []string,
) (protocol.SessionRoundIndex, error) {
	return s.readCanonicalRoundIndexContext(
		context.Background(), workspacePath, sessionValue, activeRoundIDs,
	)
}

// ReadRoundIndexPageContext 读取 DM 导航元数据；读模型不覆盖的会话按 canonical overlay 读取。
func (s *AgentHistoryStore) ReadRoundIndexPageContext(
	ctx context.Context,
	workspacePath string,
	sessionValue protocol.Session,
	activeRoundIDs []string,
) (protocol.SessionRoundIndex, error) {
	failure := errConversationHistoryNotReady
	for attempt := 0; attempt < 2; attempt++ {
		scope, identity, covered, err := s.readyDMConversationHistory(ctx, workspacePath, sessionValue)
		if err != nil {
			failure = err
			break
		}
		if !covered {
			return s.readCanonicalRoundIndexContext(ctx, workspacePath, sessionValue, activeRoundIDs)
		}
		index, ok, err := s.history.readRoundIndex(ctx, scope, identity, activeRoundIDs)
		if err != nil {
			failure = err
			break
		}
		if ok {
			return index, nil
		}
	}
	if !canonicalHistoryFallback(ctx, sessionValue.SessionKey, failure) {
		return protocol.SessionRoundIndex{}, failure
	}
	return s.readCanonicalRoundIndexContext(ctx, workspacePath, sessionValue, activeRoundIDs)
}

func (s *AgentHistoryStore) readCanonicalRoundIndexContext(
	ctx context.Context,
	workspacePath string,
	sessionValue protocol.Session,
	activeRoundIDs []string,
) (protocol.SessionRoundIndex, error) {
	active := normalizeActiveRoundIDs(activeRoundIDs)
	var index protocol.SessionRoundIndex
	err := s.replaySessionOverlay(workspacePath, sessionValue.SessionKey, func(entry *overlayReplayEntry, file *os.File) error {
		var err error
		index, err = entry.roundIndex.read(ctx, file, active, strings.TrimSpace(sessionValue.AgentID))
		return err
	})
	if errors.Is(err, os.ErrNotExist) {
		return protocol.SessionRoundIndex{Items: []protocol.SessionRoundIndexItem{}}, nil
	}
	return index, err
}

// ReadRoundIndexPageContext 读取 Room 导航元数据；会话未就绪时同步整会话回填。
func (s *RoomHistoryStore) ReadRoundIndexPageContext(
	ctx context.Context,
	ownerUserID string,
	conversationID string,
	activeRoundIDs []string,
) (protocol.SessionRoundIndex, error) {
	failure := errConversationHistoryNotReady
	for attempt := 0; attempt < 2; attempt++ {
		scope, err := s.readyConversationHistory(ctx, ownerUserID, conversationID)
		if err != nil {
			failure = err
			break
		}
		index, ok, err := s.history.readRoundIndex(ctx, scope, "", activeRoundIDs)
		if err != nil {
			failure = err
			break
		}
		if ok {
			return index, nil
		}
	}
	if !canonicalHistoryFallback(ctx, "room:"+conversationID, failure) {
		return protocol.SessionRoundIndex{}, failure
	}
	return s.readCanonicalRoundIndexContext(ctx, ownerUserID, conversationID, activeRoundIDs)
}

// readCanonicalRoundIndexContext 是表不可用时 Room 导航的兜底：直接扫描公区 ledger。
func (s *RoomHistoryStore) readCanonicalRoundIndexContext(
	ctx context.Context,
	ownerUserID string,
	conversationID string,
	activeRoundIDs []string,
) (protocol.SessionRoundIndex, error) {
	empty := protocol.SessionRoundIndex{Items: []protocol.SessionRoundIndexItem{}}
	parent, name, err := s.files.openRoomFileParent(
		ownerUserID,
		s.paths.RoomConversationOverlayPath(ownerUserID, conversationID),
		false,
	)
	if errors.Is(err, os.ErrNotExist) {
		return empty, nil
	}
	if err != nil {
		return protocol.SessionRoundIndex{}, err
	}
	defer parent.Close()
	index, err := readRoundIndexFromRootContext(ctx, parent, name, normalizeActiveRoundIDs(activeRoundIDs), true, "")
	if errors.Is(err, os.ErrNotExist) {
		return empty, nil
	}
	return index, err
}

func materializeSessionRoundIndex(
	items []protocol.SessionRoundIndexItem,
	activeRoundIDs []string,
	collapseRoomRounds bool,
) protocol.SessionRoundIndex {
	active := normalizeActiveRoundIDs(activeRoundIDs)
	result := make([]protocol.SessionRoundIndexItem, len(items))
	for index, item := range items {
		result[index] = item
		result[index].AgentIDs = append([]string(nil), item.AgentIDs...)
		if sessionRoundIndexItemIsActive(item, active, collapseRoomRounds) {
			result[index].IsLive = true
			result[index].Status = string(roundStatusRunning)
		}
	}
	return protocol.SessionRoundIndex{Items: result}
}

func sessionRoundIndexItemIsActive(
	item protocol.SessionRoundIndexItem,
	active map[string]struct{},
	collapseRoomRounds bool,
) bool {
	if _, ok := active[item.RoundID]; ok {
		return true
	}
	if !collapseRoomRounds {
		return false
	}
	for rawRoundID := range active {
		if normalizeRoundIndexRoundID(rawRoundID, "", true) == item.RoundID {
			return true
		}
		for _, agentID := range item.AgentIDs {
			if normalizeRoundIndexRoundID(rawRoundID, agentID, true) == item.RoundID {
				return true
			}
		}
	}
	return false
}

// lastByteReader 记住已读到的最后一个字节。
type lastByteReader struct {
	source io.Reader
	last   byte
}

func (r *lastByteReader) Read(buffer []byte) (int, error) {
	count, err := r.source.Read(buffer)
	if count > 0 {
		r.last = buffer[count-1]
	}
	return count, err
}

// decodeRoundIndexOverlayRows 解码一段 overlay 并按 message_id 最后写入语义去掉被覆盖的行，
// 保留行的相对顺序不变。overlay 只追加：输入停在一行 JSON 中间且末尾没有换行，
// 说明追加方还没写完最后一行；已写完的行照常返回，这一行留给下一次读取。
func decodeRoundIndexOverlayRows(ctx context.Context, source io.Reader) ([]roundIndexOverlayJSONRow, error) {
	input := &lastByteReader{source: source}
	decoder := json.NewDecoder(input)
	rows := make([]roundIndexOverlayJSONRow, 0)
	latestByMessageID := make(map[string]int)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var row roundIndexOverlayJSONRow
		if err := decoder.Decode(&row); err != nil {
			// 以换行结尾却仍缺内容的是已经写完的坏行，不是写到一半的行，照旧报错。
			if errors.Is(err, io.EOF) ||
				(errors.Is(err, io.ErrUnexpectedEOF) && input.last != '\n') {
				break
			}
			return nil, err
		}
		// 导航只用用户输入或 marker 生成标题，等待重复 ID 对账时不保留工具正文。
		if strings.TrimSpace(row.Role) != "user" && strings.TrimSpace(row.OverlayKind) != overlayKindRoundMarker {
			row.Content = nil
		}
		rows = append(rows, row)
		if messageID := strings.TrimSpace(row.MessageID); messageID != "" {
			latestByMessageID[messageID] = len(rows) - 1
		}
	}
	latest := rows[:0]
	for index, row := range rows {
		if messageID := strings.TrimSpace(row.MessageID); messageID != "" && latestByMessageID[messageID] != index {
			continue
		}
		latest = append(latest, row)
	}
	return latest, nil
}

func (row roundIndexOverlayJSONRow) applyToIndex(
	entries map[string]*sessionRoundIndexAccumulator,
	activeRoundIDs map[string]struct{},
	collapseRoomAgentRounds bool,
	defaultAgentID string,
) *sessionRoundIndexAccumulator {
	overlayKind := strings.TrimSpace(row.OverlayKind)
	if overlayKind == overlayKindRoundMarker {
		if row.HiddenFromUser {
			return nil
		}
		rawRoundID := strings.TrimSpace(row.RoundID)
		roundID := normalizeRoundIndexRoundID(
			rawRoundID,
			row.AgentID,
			collapseRoomAgentRounds,
		)
		if roundID == "" {
			return nil
		}
		entry := ensureRoundIndexEntry(entries, roundID)
		entry.item.HasUserMessage = true
		updateRoundIndexTimestamp(entry, roundIndexInt64FromRaw(row.Timestamp))
		updateRoundIndexTitle(entry, roundIndexTextFromRaw(row.Content))
		markRoundIndexActive(entry, rawRoundID, roundID, activeRoundIDs)
		return entry
	}
	if overlayKind == overlayKindRoomPublicCursor || overlayKind == "history_rewrite" || overlayKind == "room_context_checkpoint" {
		return nil
	}

	rawRoundID := strings.TrimSpace(row.RoundID)
	roundID := normalizeRoundIndexRoundID(rawRoundID, strings.TrimSpace(row.AgentID), collapseRoomAgentRounds)
	if roundID == "" {
		return nil
	}

	entry := ensureRoundIndexEntry(entries, roundID)
	updateRoundIndexTimestamp(entry, roundIndexInt64FromRaw(row.Timestamp))
	if strings.TrimSpace(row.Role) == "user" {
		entry.item.HasUserMessage = true
		updateRoundIndexTitle(entry, roundIndexTextFromRaw(row.Content))
	}
	role := strings.TrimSpace(row.Role)
	if overlayKind == overlayKindTranscriptRef {
		// Room ledger 的 transcript_ref 行不带 role，只为成员 assistant 回复写入（buildRoomTranscriptReference）。
		role = "assistant"
	}
	if role == "assistant" || role == "result" {
		addRoundIndexAgentID(entry, row.AgentID, defaultAgentID)
	}
	if role == "result" {
		updateRoundIndexResult(entry, strings.TrimSpace(row.Subtype), roundIndexInt64FromRaw(row.DurationMS))
	}
	if row.ResultSummary != nil {
		updateRoundIndexResult(
			entry,
			strings.TrimSpace(row.ResultSummary.Subtype),
			roundIndexInt64FromRaw(row.ResultSummary.DurationMS),
		)
	}
	markRoundIndexActive(entry, rawRoundID, roundID, activeRoundIDs)
	return entry
}

func buildSessionRoundIndex(
	entries map[string]*sessionRoundIndexAccumulator,
) protocol.SessionRoundIndex {
	items := make([]protocol.SessionRoundIndexItem, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		items = append(items, entry.item)
	}
	sort.SliceStable(items, func(leftIndex, rightIndex int) bool {
		return roundIndexItemBefore(items[leftIndex], items[rightIndex])
	})
	return protocol.SessionRoundIndex{Items: items}
}

// roundIndexItemBefore 是导航顺序：有时间戳的按时间升序，无时间戳的排在最后，同序按 round_id。
func roundIndexItemBefore(left protocol.SessionRoundIndexItem, right protocol.SessionRoundIndexItem) bool {
	if left.Timestamp == 0 && right.Timestamp != 0 {
		return false
	}
	if left.Timestamp != 0 && right.Timestamp == 0 {
		return true
	}
	if left.Timestamp != right.Timestamp {
		return left.Timestamp < right.Timestamp
	}
	return left.RoundID < right.RoundID
}

func ensureRoundIndexEntry(
	entries map[string]*sessionRoundIndexAccumulator,
	roundID string,
) *sessionRoundIndexAccumulator {
	entry := entries[roundID]
	if entry != nil {
		return entry
	}
	entry = &sessionRoundIndexAccumulator{
		agentIDs: make(map[string]struct{}),
		item: protocol.SessionRoundIndexItem{
			RoundID: roundID,
		},
	}
	entries[roundID] = entry
	return entry
}

func addRoundIndexAgentID(
	entry *sessionRoundIndexAccumulator,
	agentID string,
	defaultAgentID string,
) {
	normalizedAgentID := textutil.FirstNonEmpty(agentID, defaultAgentID)
	if normalizedAgentID == "" {
		return
	}
	if _, ok := entry.agentIDs[normalizedAgentID]; ok {
		return
	}
	entry.agentIDs[normalizedAgentID] = struct{}{}
	entry.item.AgentIDs = append(entry.item.AgentIDs, normalizedAgentID)
}

func normalizeRoundIndexRoundID(
	roundID string,
	agentID string,
	collapseRoomAgentRounds bool,
) string {
	if roundID == "" {
		return ""
	}
	if collapseRoomAgentRounds {
		return normalizeRoomHistoryRoundID(roundID, agentID)
	}
	return roundID
}

func updateRoundIndexTimestamp(entry *sessionRoundIndexAccumulator, timestamp int64) {
	if timestamp <= 0 {
		return
	}
	if entry.item.Timestamp == 0 || timestamp < entry.item.Timestamp {
		entry.item.Timestamp = timestamp
	}
}

func updateRoundIndexTitle(entry *sessionRoundIndexAccumulator, title string) {
	if entry.item.Title != "" || title == "" {
		return
	}
	entry.item.Title = title
}

func updateRoundIndexResult(
	entry *sessionRoundIndexAccumulator,
	subtype string,
	duration int64,
) {
	status := normalizeRoundStatusValue(subtype)
	if status == roundStatusRunning {
		status = roundStatusSuccess
	}
	entry.item.Status = string(status)
	if duration > 0 {
		entry.item.DurationMS = &duration
		return
	}
}

func markRoundIndexActive(
	entry *sessionRoundIndexAccumulator,
	rawRoundID string,
	roundID string,
	activeRoundIDs map[string]struct{},
) {
	if len(activeRoundIDs) == 0 {
		return
	}
	if _, ok := activeRoundIDs[roundID]; ok {
		entry.item.IsLive = true
		entry.item.Status = string(roundStatusRunning)
		return
	}
	if rawRoundID != "" {
		if _, ok := activeRoundIDs[rawRoundID]; ok {
			entry.item.IsLive = true
			entry.item.Status = string(roundStatusRunning)
		}
	}
}

func roundIndexTextFromRaw(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	var blocks []map[string]any
	if err := json.Unmarshal(raw, &blocks); err == nil {
		return roundIndexTextFromBlocks(blocks)
	}
	var rawBlocks []any
	if err := json.Unmarshal(raw, &rawBlocks); err != nil {
		return ""
	}
	blocks = make([]map[string]any, 0, len(rawBlocks))
	for _, item := range rawBlocks {
		if block, ok := item.(map[string]any); ok {
			blocks = append(blocks, block)
		}
	}
	return roundIndexTextFromBlocks(blocks)
}

func roundIndexInt64FromRaw(raw json.RawMessage) int64 {
	if len(raw) == 0 || string(raw) == "null" {
		return 0
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0
	}
	return protocol.Int64FromAny(value)
}

func roundIndexTextFromBlocks(blocks []map[string]any) string {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if stringFromAny(block["type"]) != "text" {
			continue
		}
		if text := stringFromAny(block["text"]); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

// readRoundIndexFromRootContext 扫描受限根下的一个 overlay/ledger 文件生成导航；Room 表不可用时的兜底与测试 oracle 共用。
func readRoundIndexFromRootContext(
	ctx context.Context,
	root *confinedfs.Root,
	relative string,
	activeRoundIDs map[string]struct{},
	collapseRoomAgentRounds bool,
	defaultAgentID string,
) (protocol.SessionRoundIndex, error) {
	file, err := root.OpenFileNoSymlink(relative, os.O_RDONLY, 0)
	if errors.Is(err, os.ErrNotExist) {
		return protocol.SessionRoundIndex{Items: []protocol.SessionRoundIndexItem{}}, nil
	}
	if err != nil {
		return protocol.SessionRoundIndex{}, err
	}
	defer file.Close()

	rows, err := decodeRoundIndexOverlayRows(ctx, file)
	if err != nil {
		return protocol.SessionRoundIndex{}, err
	}
	entries := make(map[string]*sessionRoundIndexAccumulator)
	for _, row := range rows {
		row.applyToIndex(entries, activeRoundIDs, collapseRoomAgentRounds, defaultAgentID)
	}
	return buildSessionRoundIndex(entries), nil
}

// INPUT: append-only overlay JSONL、共享 history generation 与当前活跃 round 集合。
// OUTPUT: 遵循 message_id 最后写入语义、支持短冷建的 DM/Room Round Navigator 元数据。
// POS: canonical overlay 导航投影与 SQLite 派生代际之间的唯一 round index 边界。
package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
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

// ReadRoundIndexPageContext 从共用派生代际读取 DM 导航元数据；冷建可返回短轮询状态。
func (s *AgentHistoryStore) ReadRoundIndexPageContext(
	ctx context.Context,
	workspacePath string,
	sessionValue protocol.Session,
	activeRoundIDs []string,
	deferIndex bool,
) (protocol.SessionRoundIndex, error) {
	return readSessionRoundIndexWithModel(
		ctx,
		s.historyPageAccess(workspacePath, sessionValue),
		activeRoundIDs,
		false,
		deferIndex,
	)
}

func (s *AgentHistoryStore) readCanonicalRoundIndexContext(
	ctx context.Context,
	workspacePath string,
	sessionValue protocol.Session,
	activeRoundIDs []string,
) (protocol.SessionRoundIndex, error) {
	active := normalizeActiveRoundIDs(activeRoundIDs)
	root, err := s.files.openWorkspaceRoot(workspacePath, false)
	if errors.Is(err, os.ErrNotExist) {
		return protocol.SessionRoundIndex{Items: []protocol.SessionRoundIndexItem{}}, nil
	}
	if err != nil {
		return protocol.SessionRoundIndex{}, err
	}
	defer root.Close()
	return readRoundIndexFromRootContext(
		ctx,
		root,
		filepath.ToSlash(filepath.Join(
			".agents",
			"sessions",
			encodeSessionDirName(sessionValue.SessionKey),
			"overlay.jsonl",
		)),
		active,
		false,
		strings.TrimSpace(sessionValue.AgentID),
	)
}

// ReadRoundIndex 读取 Room 共享会话的轻量 round 导航索引。
func (s *RoomHistoryStore) ReadRoundIndex(
	ownerUserID string,
	conversationID string,
	activeRoundIDs []string,
) (protocol.SessionRoundIndex, error) {
	return s.readCanonicalRoundIndexContext(
		context.Background(), ownerUserID, conversationID, activeRoundIDs,
	)
}

// ReadRoundIndexPageContext 从共用派生代际读取 Room 导航元数据；冷建可返回短轮询状态。
func (s *RoomHistoryStore) ReadRoundIndexPageContext(
	ctx context.Context,
	ownerUserID string,
	conversationID string,
	activeRoundIDs []string,
	deferIndex bool,
) (protocol.SessionRoundIndex, error) {
	return readSessionRoundIndexWithModel(
		ctx,
		s.historyPageAccess(ownerUserID, conversationID),
		activeRoundIDs,
		true,
		deferIndex,
	)
}

func readSessionRoundIndexWithModel(
	ctx context.Context,
	access historyPageIndexAccess,
	activeRoundIDs []string,
	collapseRoomRounds bool,
	deferIndex bool,
) (protocol.SessionRoundIndex, error) {
	if err := ctx.Err(); err != nil {
		return protocol.SessionRoundIndex{}, err
	}
	if access.ReadModel != nil {
		index, ok, err := access.ReadModel.loadRoundIndex(
			ctx, access, activeRoundIDs, collapseRoomRounds,
		)
		if err == nil && ok {
			return index, nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return protocol.SessionRoundIndex{}, err
		}
		var disabled *historyPageIndexDisabledError
		if errors.As(err, &disabled) {
			return buildCanonicalSessionRoundIndex(ctx, access, activeRoundIDs, collapseRoomRounds)
		}
	}
	built, indexing, err := awaitHistoryPageIndexBuild(ctx, access, deferIndex)
	if err != nil {
		return protocol.SessionRoundIndex{}, err
	}
	if indexing {
		return sessionRoundIndexingResult(), nil
	}
	if built.Disabled {
		return buildCanonicalSessionRoundIndex(ctx, access, activeRoundIDs, collapseRoomRounds)
	}
	return materializeSessionRoundIndex(
		built.RoundIndex,
		activeRoundIDs,
		collapseRoomRounds,
	), nil
}

func buildCanonicalSessionRoundIndex(
	ctx context.Context,
	access historyPageIndexAccess,
	activeRoundIDs []string,
	collapseRoomRounds bool,
) (protocol.SessionRoundIndex, error) {
	build := access.BuildCanonical
	if build == nil {
		build = access.Build
	}
	built, err := build(ctx)
	if err != nil {
		return protocol.SessionRoundIndex{}, err
	}
	return materializeSessionRoundIndex(
		built.RoundIndex,
		activeRoundIDs,
		collapseRoomRounds,
	), nil
}

func sessionRoundIndexingResult() protocol.SessionRoundIndex {
	return protocol.SessionRoundIndex{
		Items:        []protocol.SessionRoundIndexItem{},
		Indexing:     true,
		RetryAfterMS: historyPageIndexRetryAfterMS,
	}
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

func (s *RoomHistoryStore) readCanonicalRoundIndexContext(
	ctx context.Context,
	ownerUserID string,
	conversationID string,
	activeRoundIDs []string,
) (protocol.SessionRoundIndex, error) {
	parent, name, err := s.files.openRoomFileParent(
		ownerUserID,
		s.paths.RoomConversationOverlayPath(ownerUserID, conversationID),
		false,
	)
	if errors.Is(err, os.ErrNotExist) {
		return protocol.SessionRoundIndex{Items: []protocol.SessionRoundIndexItem{}}, nil
	}
	if err != nil {
		return protocol.SessionRoundIndex{}, err
	}
	defer parent.Close()
	return readRoundIndexFromRootContext(
		ctx,
		parent,
		name,
		normalizeActiveRoundIDs(activeRoundIDs),
		true,
		"",
	)
}

func readRoundIndexFromJSONLAt(
	rootPath string,
	path string,
	activeRoundIDs map[string]struct{},
	collapseRoomAgentRounds bool,
	defaultAgentID string,
) (protocol.SessionRoundIndex, error) {
	root, relative, err := relativeStorePath(rootPath, path)
	if err != nil {
		return protocol.SessionRoundIndex{}, err
	}
	defer root.Close()
	return readRoundIndexFromRoot(root, relative, activeRoundIDs, collapseRoomAgentRounds, defaultAgentID)
}

func readRoundIndexFromRoot(
	root *confinedfs.Root,
	relative string,
	activeRoundIDs map[string]struct{},
	collapseRoomAgentRounds bool,
	defaultAgentID string,
) (protocol.SessionRoundIndex, error) {
	return readRoundIndexFromRootContext(
		context.Background(), root, relative, activeRoundIDs, collapseRoomAgentRounds, defaultAgentID,
	)
}

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

	decoder := json.NewDecoder(file)
	rows := make([]roundIndexOverlayJSONRow, 0)
	latestByMessageID := make(map[string]int)
	for {
		if err = ctx.Err(); err != nil {
			return protocol.SessionRoundIndex{}, err
		}
		var row roundIndexOverlayJSONRow
		if err := decoder.Decode(&row); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return protocol.SessionRoundIndex{}, err
		}
		rows = append(rows, row)
		if messageID := strings.TrimSpace(row.MessageID); messageID != "" {
			latestByMessageID[messageID] = len(rows) - 1
		}
	}

	entries := make(map[string]*sessionRoundIndexAccumulator)
	for index, row := range rows {
		if messageID := strings.TrimSpace(row.MessageID); messageID != "" && latestByMessageID[messageID] != index {
			continue
		}
		row.applyToIndex(entries, activeRoundIDs, collapseRoomAgentRounds, defaultAgentID)
	}
	return buildSessionRoundIndex(entries), nil
}

func (row roundIndexOverlayJSONRow) applyToIndex(
	entries map[string]*sessionRoundIndexAccumulator,
	activeRoundIDs map[string]struct{},
	collapseRoomAgentRounds bool,
	defaultAgentID string,
) {
	overlayKind := strings.TrimSpace(row.OverlayKind)
	if overlayKind == overlayKindRoundMarker {
		if row.HiddenFromUser {
			return
		}
		rawRoundID := strings.TrimSpace(row.RoundID)
		roundID := normalizeRoundIndexRoundID(
			rawRoundID,
			row.AgentID,
			collapseRoomAgentRounds,
		)
		if roundID == "" {
			return
		}
		entry := ensureRoundIndexEntry(entries, roundID)
		entry.item.HasUserMessage = true
		updateRoundIndexTimestamp(entry, roundIndexInt64FromRaw(row.Timestamp))
		updateRoundIndexTitle(entry, roundIndexTextFromRaw(row.Content))
		markRoundIndexActive(entry, rawRoundID, roundID, activeRoundIDs)
		return
	}
	if overlayKind == overlayKindRoomPublicCursor || overlayKind == "history_rewrite" || overlayKind == "room_context_checkpoint" {
		return
	}

	rawRoundID := strings.TrimSpace(row.RoundID)
	roundID := normalizeRoundIndexRoundID(rawRoundID, strings.TrimSpace(row.AgentID), collapseRoomAgentRounds)
	if roundID == "" {
		return
	}

	entry := ensureRoundIndexEntry(entries, roundID)
	updateRoundIndexTimestamp(entry, roundIndexInt64FromRaw(row.Timestamp))
	if strings.TrimSpace(row.Role) == "user" {
		entry.item.HasUserMessage = true
		updateRoundIndexTitle(entry, roundIndexTextFromRaw(row.Content))
	}
	role := strings.TrimSpace(row.Role)
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
		left := items[leftIndex]
		right := items[rightIndex]
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
	})
	return protocol.SessionRoundIndex{Items: items}
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
	normalizedAgentID := firstNonEmpty(agentID, defaultAgentID)
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

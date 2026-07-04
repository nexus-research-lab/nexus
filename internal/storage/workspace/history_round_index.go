package workspace

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"sort"
	"strings"

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
	OverlayKind    string                       `json:"nexus_overlay_kind"`
	ResultSummary  *roundIndexJSONResultSummary `json:"result_summary"`
	Role           string                       `json:"role"`
	RoundID        string                       `json:"round_id"`
	Subtype        string                       `json:"subtype"`
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
	active := normalizeActiveRoundIDs(activeRoundIDs)
	return readRoundIndexFromJSONL(
		s.paths.SessionOverlayPath(workspacePath, sessionValue.SessionKey),
		active,
		false,
		strings.TrimSpace(sessionValue.AgentID),
	)
}

// ReadRoundIndex 读取 Room 共享会话的轻量 round 导航索引。
func (s *RoomHistoryStore) ReadRoundIndex(
	conversationID string,
	activeRoundIDs []string,
) (protocol.SessionRoundIndex, error) {
	return readRoundIndexFromJSONL(
		s.paths.RoomConversationOverlayPath(conversationID),
		normalizeActiveRoundIDs(activeRoundIDs),
		true,
		"",
	)
}

func readRoundIndexFromJSONL(
	path string,
	activeRoundIDs map[string]struct{},
	collapseRoomAgentRounds bool,
	defaultAgentID string,
) (protocol.SessionRoundIndex, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return protocol.SessionRoundIndex{Items: []protocol.SessionRoundIndexItem{}}, nil
	}
	if err != nil {
		return protocol.SessionRoundIndex{}, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	entries := make(map[string]*sessionRoundIndexAccumulator)
	for {
		var row roundIndexOverlayJSONRow
		if err := decoder.Decode(&row); err != nil {
			if errors.Is(err, io.EOF) {
				return buildSessionRoundIndex(entries), nil
			}
			return protocol.SessionRoundIndex{}, err
		}
		row.applyToIndex(entries, activeRoundIDs, collapseRoomAgentRounds, defaultAgentID)
	}
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
	if overlayKind == overlayKindRoomPublicCursor || overlayKind == "room_context_checkpoint" {
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
	normalizedAgentID := strings.TrimSpace(agentID)
	if normalizedAgentID == "" {
		normalizedAgentID = strings.TrimSpace(defaultAgentID)
	}
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
	return int64FromAny(value)
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

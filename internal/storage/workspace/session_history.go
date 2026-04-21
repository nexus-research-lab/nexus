// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：session_history.go
// @Date   ：2026/04/16 22:00:00
// @Author ：leemysw
// 2026/04/16 22:00:00   Create
// =====================================================

package workspace

import (
	"sort"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	sessionmodel "github.com/nexus-research-lab/nexus/internal/model/session"
)

type historyPageGroup struct {
	CursorRoundID        string
	CursorRoundTimestamp int64
	Items                []sessionmodel.Message
}

type roundTerminalStatus string

const (
	roundStatusRunning     roundTerminalStatus = "running"
	roundStatusSuccess     roundTerminalStatus = "success"
	roundStatusInterrupted roundTerminalStatus = "interrupted"
	roundStatusError       roundTerminalStatus = "error"
)

func normalizeHistoryRows(rows []sessionmodel.Message, activeRoundIDs map[string]struct{}) []sessionmodel.Message {
	compacted := compactMessages(filterInternalHistoryRows(rows))
	return normalizeCompactedHistoryRows(compacted, activeRoundIDs)
}

func filterInternalHistoryRows(rows []sessionmodel.Message) []sessionmodel.Message {
	if len(rows) == 0 {
		return rows
	}
	filtered := make([]sessionmodel.Message, 0, len(rows))
	for _, row := range rows {
		if shouldSkipInternalHistoryRow(row) {
			continue
		}
		filtered = append(filtered, row)
	}
	return filtered
}

func shouldSkipInternalHistoryRow(row sessionmodel.Message) bool {
	role := strings.TrimSpace(stringFromAny(row["role"]))
	switch role {
	case "system":
		metadata, _ := row["metadata"].(map[string]any)
		return strings.TrimSpace(stringFromAny(metadata["subtype"])) == "api_retry"
	case "user":
		content := strings.TrimSpace(stringFromAny(row["content"]))
		return sessionmodel.IsInternalTranscriptInterruptPrompt(content)
	default:
		return false
	}
}

func normalizeCompactedHistoryRows(
	compacted []sessionmodel.Message,
	activeRoundIDs map[string]struct{},
) []sessionmodel.Message {
	materialized := materializeUnfinishedRounds(compacted, activeRoundIDs)
	return mergeRoundResultSummaries(materialized)
}

func normalizeRoundPageLimit(limit int) int {
	if limit <= 0 {
		return config.GetMessageHistoryRoundPageSize()
	}
	if limit > config.GetMessageHistoryRoundPageSizeMax() {
		return config.GetMessageHistoryRoundPageSizeMax()
	}
	return limit
}

func paginateNormalizedHistoryRows(
	rows []sessionmodel.Message,
	limit int,
	beforeRoundID string,
	beforeRoundTimestamp int64,
	collapseRoomAgentRounds bool,
) sessionmodel.MessagePage {
	if len(rows) == 0 {
		return sessionmodel.MessagePage{
			Items:   []sessionmodel.Message{},
			HasMore: false,
		}
	}

	pageLimit := normalizeRoundPageLimit(limit)
	groups := buildHistoryPageGroups(rows, collapseRoomAgentRounds)
	endGroupIndex := findHistoryPageEndGroupIndex(
		groups,
		strings.TrimSpace(beforeRoundID),
		beforeRoundTimestamp,
	)
	if endGroupIndex <= 0 {
		return sessionmodel.MessagePage{
			Items:   []sessionmodel.Message{},
			HasMore: false,
		}
	}

	startGroupIndex := endGroupIndex - pageLimit
	if startGroupIndex < 0 {
		startGroupIndex = 0
	}

	pageItems := make([]sessionmodel.Message, 0)
	for _, group := range groups[startGroupIndex:endGroupIndex] {
		pageItems = append(pageItems, group.Items...)
	}

	page := sessionmodel.MessagePage{
		Items:   pageItems,
		HasMore: startGroupIndex > 0,
	}
	if page.HasMore && len(pageItems) > 0 {
		oldestGroup := groups[startGroupIndex]
		if strings.TrimSpace(oldestGroup.CursorRoundID) != "" {
			page.NextBeforeRoundID = stringPointer(oldestGroup.CursorRoundID)
		}
		timestamp := oldestGroup.CursorRoundTimestamp
		page.NextBeforeRoundTimestamp = &timestamp
	}
	return page
}

func buildHistoryPageGroups(
	rows []sessionmodel.Message,
	collapseRoomAgentRounds bool,
) []historyPageGroup {
	if len(rows) == 0 {
		return nil
	}

	groups := make([]historyPageGroup, 0, len(rows))
	currentGroupKey := ""
	currentGroup := historyPageGroup{}

	flushCurrentGroup := func() {
		if len(currentGroup.Items) == 0 {
			return
		}
		groups = append(groups, currentGroup)
		currentGroup = historyPageGroup{}
	}

	for _, row := range rows {
		groupKey := historyPageGroupKey(row, collapseRoomAgentRounds)
		if groupKey == "" {
			continue
		}
		if groupKey != currentGroupKey {
			flushCurrentGroup()
			currentGroupKey = groupKey
			currentGroup = historyPageGroup{
				CursorRoundID:        historyPageCursorRoundID(row, collapseRoomAgentRounds),
				CursorRoundTimestamp: messageTimestamp(row),
				Items:                make([]sessionmodel.Message, 0, 1),
			}
		}
		currentGroup.Items = append(currentGroup.Items, row)
	}
	flushCurrentGroup()
	return groups
}

func historyPageCursorRoundID(row sessionmodel.Message, collapseRoomAgentRounds bool) string {
	roundID := strings.TrimSpace(stringFromAny(row["round_id"]))
	if roundID != "" {
		if collapseRoomAgentRounds {
			return normalizeRoomHistoryRoundID(roundID, stringFromAny(row["agent_id"]))
		}
		return roundID
	}
	return strings.TrimSpace(stringFromAny(row["message_id"]))
}

func historyPageGroupKey(row sessionmodel.Message, collapseRoomAgentRounds bool) string {
	roundID := strings.TrimSpace(stringFromAny(row["round_id"]))
	if roundID != "" {
		if collapseRoomAgentRounds {
			return "round:" + normalizeRoomHistoryRoundID(roundID, stringFromAny(row["agent_id"]))
		}
		return "round:" + roundID
	}

	messageID := strings.TrimSpace(stringFromAny(row["message_id"]))
	if messageID != "" {
		return "message:" + messageID
	}
	return ""
}

func normalizeRoomHistoryRoundID(roundID string, agentID string) string {
	trimmedRoundID := strings.TrimSpace(roundID)
	trimmedAgentID := strings.TrimSpace(agentID)
	if trimmedRoundID == "" || trimmedAgentID == "" {
		return trimmedRoundID
	}
	suffix := ":" + trimmedAgentID
	if strings.HasSuffix(trimmedRoundID, suffix) {
		return strings.TrimSuffix(trimmedRoundID, suffix)
	}
	return trimmedRoundID
}

func findHistoryPageEndGroupIndex(
	groups []historyPageGroup,
	beforeRoundID string,
	beforeRoundTimestamp int64,
) int {
	if beforeRoundTimestamp <= 0 && beforeRoundID == "" {
		return len(groups)
	}
	if beforeRoundTimestamp <= 0 && beforeRoundID != "" {
		for index, group := range groups {
			if group.CursorRoundID == beforeRoundID {
				return index
			}
		}
		return 0
	}

	for index, group := range groups {
		if compareHistoryPageGroupCursor(group, beforeRoundID, beforeRoundTimestamp) >= 0 {
			return index
		}
	}
	return len(groups)
}

func compareHistoryPageGroupCursor(
	group historyPageGroup,
	beforeRoundID string,
	beforeRoundTimestamp int64,
) int {
	if group.CursorRoundTimestamp < beforeRoundTimestamp {
		return -1
	}
	if group.CursorRoundTimestamp > beforeRoundTimestamp {
		return 1
	}
	if beforeRoundID == "" {
		return 1
	}
	return strings.Compare(group.CursorRoundID, beforeRoundID)
}

func mergeRoundResultSummaries(rows []sessionmodel.Message) []sessionmodel.Message {
	if len(rows) == 0 {
		return rows
	}
	merger := newRoundResultSummaryMerger(rows)
	merger.attachMatchingResults()
	return merger.buildResultRows()
}

type roundResultSummaryMerger struct {
	rows                        []sessionmodel.Message
	lastAssistantIndexByRoundID map[string]int
	assistantTextByRoundID      map[string]string
	mergedResultMessageIDs      map[string]struct{}
}

func newRoundResultSummaryMerger(rows []sessionmodel.Message) *roundResultSummaryMerger {
	merger := &roundResultSummaryMerger{
		rows:                        cloneHistoryRows(rows),
		lastAssistantIndexByRoundID: make(map[string]int),
		assistantTextByRoundID:      make(map[string]string),
		mergedResultMessageIDs:      make(map[string]struct{}),
	}
	merger.indexAssistants()
	return merger
}

func cloneHistoryRows(rows []sessionmodel.Message) []sessionmodel.Message {
	cloned := make([]sessionmodel.Message, 0, len(rows))
	for _, row := range rows {
		cloned = append(cloned, sessionmodel.Clone(row))
	}
	return cloned
}

func (m *roundResultSummaryMerger) indexAssistants() {
	for index, row := range m.rows {
		if sessionmodel.MessageRole(row) != "assistant" {
			continue
		}
		roundID := strings.TrimSpace(stringFromAny(row["round_id"]))
		if roundID == "" {
			continue
		}
		m.lastAssistantIndexByRoundID[roundID] = index
		if assistantText := sessionmodel.ExtractAssistantDisplayText(row); assistantText != "" {
			m.assistantTextByRoundID[roundID] = assistantText
		}
	}
}

func (m *roundResultSummaryMerger) attachMatchingResults() {
	for _, row := range m.rows {
		if sessionmodel.MessageRole(row) != "result" {
			continue
		}
		roundID := strings.TrimSpace(stringFromAny(row["round_id"]))
		assistantIndex, hasAssistant := m.lastAssistantIndexByRoundID[roundID]
		if !hasAssistant {
			continue
		}

		assistant := sessionmodel.Clone(m.rows[assistantIndex])
		summary := sessionmodel.BuildAssistantResultSummary(row, m.assistantTextByRoundID[roundID])
		if len(summary) == 0 {
			continue
		}
		assistant["result_summary"] = summary
		m.rows[assistantIndex] = assistant
		if messageID := strings.TrimSpace(stringFromAny(row["message_id"])); messageID != "" {
			m.mergedResultMessageIDs[messageID] = struct{}{}
		}
	}
}

func (m *roundResultSummaryMerger) buildResultRows() []sessionmodel.Message {
	result := make([]sessionmodel.Message, 0, len(m.rows))
	for _, row := range m.rows {
		if sessionmodel.MessageRole(row) == "result" {
			if _, merged := m.mergedResultMessageIDs[strings.TrimSpace(stringFromAny(row["message_id"]))]; merged {
				continue
			}
			result = append(result, sessionmodel.BuildSyntheticAssistantFromResult(row))
			continue
		}
		result = append(result, row)
	}
	return result
}

func materializeUnfinishedRounds(rows []sessionmodel.Message, activeRoundIDs map[string]struct{}) []sessionmodel.Message {
	if len(rows) == 0 {
		return rows
	}
	type roundSnapshot struct {
		RoundID         string
		SessionKey      string
		RoomID          string
		ConversationID  string
		AgentID         string
		SessionID       string
		ParentID        string
		LastTimestampMS int64
		HasResult       bool
		TerminalStatus  roundTerminalStatus
	}

	rounds := make(map[string]*roundSnapshot)
	for _, row := range rows {
		roundID := strings.TrimSpace(stringFromAny(row["round_id"]))
		if roundID == "" {
			continue
		}
		snapshot := rounds[roundID]
		if snapshot == nil {
			snapshot = &roundSnapshot{
				RoundID:        roundID,
				TerminalStatus: roundStatusRunning,
			}
			rounds[roundID] = snapshot
		}
		snapshot.SessionKey = firstNonEmpty(snapshot.SessionKey, stringFromAny(row["session_key"]))
		snapshot.RoomID = firstNonEmpty(snapshot.RoomID, stringFromAny(row["room_id"]))
		snapshot.ConversationID = firstNonEmpty(snapshot.ConversationID, stringFromAny(row["conversation_id"]))
		snapshot.AgentID = firstNonEmpty(snapshot.AgentID, stringFromAny(row["agent_id"]))
		snapshot.SessionID = firstNonEmpty(snapshot.SessionID, stringFromAny(row["session_id"]))
		snapshot.ParentID = firstNonEmpty(snapshot.ParentID, stringFromAny(row["parent_id"]))
		if ts := messageTimestamp(row); ts > snapshot.LastTimestampMS {
			snapshot.LastTimestampMS = ts
		}
		if strings.TrimSpace(stringFromAny(row["role"])) == "result" {
			snapshot.HasResult = true
			snapshot.TerminalStatus = normalizeRoundStatusValue(row["subtype"])
			continue
		}
		if terminalStatus := assistantTerminalStatus(row); terminalStatus != roundStatusRunning {
			snapshot.TerminalStatus = terminalStatus
		}
	}

	result := make([]sessionmodel.Message, 0, len(rows)+len(rounds))
	result = append(result, rows...)
	for roundID, snapshot := range rounds {
		if snapshot == nil || snapshot.HasResult {
			continue
		}
		if _, isActive := activeRoundIDs[roundID]; isActive {
			continue
		}
		if snapshot.TerminalStatus != roundStatusRunning {
			continue
		}
		timestamp := snapshot.LastTimestampMS + 1
		if timestamp <= 0 {
			timestamp = time.Now().UnixMilli()
		}
		payload := sessionmodel.Message{
			"message_id":      "assistant_interrupt_" + roundID,
			"session_key":     snapshot.SessionKey,
			"room_id":         emptyStringToNil(snapshot.RoomID),
			"conversation_id": emptyStringToNil(snapshot.ConversationID),
			"agent_id":        snapshot.AgentID,
			"round_id":        roundID,
			"session_id":      emptyStringToNil(snapshot.SessionID),
			"role":            "assistant",
			"timestamp":       timestamp,
			"stop_reason":     "cancelled",
			"is_complete":     true,
			"content":         []map[string]any{},
			"result_summary": map[string]any{
				"message_id":      "result_" + roundID,
				"timestamp":       timestamp,
				"subtype":         "interrupted",
				"duration_ms":     0,
				"duration_api_ms": 0,
				"num_turns":       0,
				"is_error":        false,
			},
		}
		if strings.TrimSpace(snapshot.ParentID) != "" {
			payload["parent_id"] = snapshot.ParentID
		}
		result = append(result, payload)
	}

	sortHistoryRows(result)
	return result
}

func normalizeActiveRoundIDs(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		if normalized := strings.TrimSpace(value); normalized != "" {
			result[normalized] = struct{}{}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func normalizeRoundStatusValue(value any) roundTerminalStatus {
	normalized := strings.ToLower(strings.TrimSpace(stringFromAny(value)))
	switch normalized {
	case "", "running":
		return roundStatusRunning
	case "interrupted", "cancelled":
		return roundStatusInterrupted
	case "error":
		return roundStatusError
	default:
		return roundStatusSuccess
	}
}

func assistantTerminalStatus(row sessionmodel.Message) roundTerminalStatus {
	if strings.TrimSpace(stringFromAny(row["role"])) != "assistant" {
		return roundStatusRunning
	}
	stopReason := strings.ToLower(strings.TrimSpace(stringFromAny(row["stop_reason"])))
	if stopReason == "" {
		return roundStatusRunning
	}
	switch stopReason {
	case "cancelled", "interrupted":
		return roundStatusInterrupted
	case "error":
		return roundStatusError
	default:
		return roundStatusSuccess
	}
}

func sortHistoryRows(rows []sessionmodel.Message) {
	sort.SliceStable(rows, func(i int, j int) bool {
		return compareHistoryRowOrder(rows[i], rows[j]) < 0
	})
}

func compareHistoryRowOrder(left sessionmodel.Message, right sessionmodel.Message) int {
	leftTimestamp := messageTimestamp(left)
	rightTimestamp := messageTimestamp(right)
	if leftTimestamp != rightTimestamp {
		if leftTimestamp < rightTimestamp {
			return -1
		}
		return 1
	}

	leftRoundID := strings.TrimSpace(stringFromAny(left["round_id"]))
	rightRoundID := strings.TrimSpace(stringFromAny(right["round_id"]))
	if leftRoundID != "" && leftRoundID == rightRoundID {
		leftOrder := historyRoleOrder(left)
		rightOrder := historyRoleOrder(right)
		if leftOrder != rightOrder {
			if leftOrder < rightOrder {
				return -1
			}
			return 1
		}
	}

	leftMessageID := strings.TrimSpace(stringFromAny(left["message_id"]))
	rightMessageID := strings.TrimSpace(stringFromAny(right["message_id"]))
	if leftMessageID != rightMessageID {
		return strings.Compare(leftMessageID, rightMessageID)
	}
	return 0
}

func historyRoleOrder(row sessionmodel.Message) int {
	switch strings.TrimSpace(stringFromAny(row["role"])) {
	case "user":
		return 0
	case "assistant", "system", "task_progress":
		return 1
	case "result":
		return 2
	default:
		return 3
	}
}

func emptyStringToNil(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}

func cloneMessage(message sessionmodel.Message) sessionmodel.Message {
	return sessionmodel.Clone(message)
}

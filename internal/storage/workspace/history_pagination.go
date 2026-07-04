package workspace

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	defaultMessageHistoryRoundPageSize = 3
	maxMessageHistoryRoundPageSize     = 10
	defaultMessageHistoryAroundLimit   = 2
	maxMessageHistoryAroundLimit       = 3
)

type historyPageGroup struct {
	CursorRoundID        string
	CursorRoundTimestamp int64
	Items                []protocol.Message
}

func normalizeRoundPageLimit(limit int) int {
	if limit <= 0 {
		return defaultMessageHistoryRoundPageSize
	}
	return min(limit, maxMessageHistoryRoundPageSize)
}

func normalizeRoundAroundLimit(limit int) int {
	if limit <= 0 {
		return defaultMessageHistoryAroundLimit
	}
	return min(limit, maxMessageHistoryAroundLimit)
}

func paginateNormalizedHistoryRows(
	rows []protocol.Message,
	limit int,
	beforeRoundID string,
	beforeRoundTimestamp int64,
	collapseRoomAgentRounds bool,
) protocol.MessagePage {
	if len(rows) == 0 {
		return protocol.MessagePage{
			Items:   []protocol.Message{},
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
		return protocol.MessagePage{
			Items:   []protocol.Message{},
			HasMore: false,
		}
	}

	startGroupIndex := endGroupIndex - pageLimit
	if startGroupIndex < 0 {
		startGroupIndex = 0
	}

	pageItems := make([]protocol.Message, 0)
	for _, group := range groups[startGroupIndex:endGroupIndex] {
		pageItems = append(pageItems, group.Items...)
	}

	page := protocol.MessagePage{
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

func paginateNormalizedHistoryRowsAround(
	rows []protocol.Message,
	aroundRoundID string,
	aroundLimit int,
	collapseRoomAgentRounds bool,
) protocol.MessagePage {
	if len(rows) == 0 {
		return protocol.MessagePage{
			Items:   []protocol.Message{},
			HasMore: false,
		}
	}

	aroundRoundID = strings.TrimSpace(aroundRoundID)
	if aroundRoundID == "" {
		return protocol.MessagePage{
			Items:   []protocol.Message{},
			HasMore: false,
		}
	}

	groups := buildHistoryPageGroups(rows, collapseRoomAgentRounds)
	targetIndex := -1
	for index, group := range groups {
		if group.CursorRoundID == aroundRoundID {
			targetIndex = index
			break
		}
	}
	if targetIndex < 0 {
		return protocol.MessagePage{
			Items:   []protocol.Message{},
			HasMore: len(groups) > 0,
		}
	}

	radius := normalizeRoundAroundLimit(aroundLimit)
	startIndex := targetIndex - radius
	if startIndex < 0 {
		startIndex = 0
	}
	endIndex := targetIndex + radius + 1
	if endIndex > len(groups) {
		endIndex = len(groups)
	}

	pageItems := make([]protocol.Message, 0)
	for _, group := range groups[startIndex:endIndex] {
		pageItems = append(pageItems, group.Items...)
	}
	page := protocol.MessagePage{
		Items:   pageItems,
		HasMore: startIndex > 0 || endIndex < len(groups),
	}
	if startIndex > 0 {
		oldestGroup := groups[startIndex]
		if strings.TrimSpace(oldestGroup.CursorRoundID) != "" {
			page.NextBeforeRoundID = stringPointer(oldestGroup.CursorRoundID)
		}
		timestamp := oldestGroup.CursorRoundTimestamp
		page.NextBeforeRoundTimestamp = &timestamp
	}
	return page
}

func buildHistoryPageGroups(
	rows []protocol.Message,
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
				Items:                make([]protocol.Message, 0, 1),
			}
		}
		currentGroup.Items = append(currentGroup.Items, row)
	}
	flushCurrentGroup()
	return groups
}

func historyPageCursorRoundID(row protocol.Message, collapseRoomAgentRounds bool) string {
	roundID := stringFromAny(row["round_id"])
	if roundID != "" {
		if collapseRoomAgentRounds {
			return normalizeRoomHistoryRoundID(roundID, stringFromAny(row["agent_id"]))
		}
		return roundID
	}
	return stringFromAny(row["message_id"])
}

func historyPageGroupKey(row protocol.Message, collapseRoomAgentRounds bool) string {
	roundID := stringFromAny(row["round_id"])
	if roundID != "" {
		if collapseRoomAgentRounds {
			return "round:" + normalizeRoomHistoryRoundID(roundID, stringFromAny(row["agent_id"]))
		}
		return "round:" + roundID
	}

	messageID := stringFromAny(row["message_id"])
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

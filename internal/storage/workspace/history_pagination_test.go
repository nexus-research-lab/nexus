package workspace

import (
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestPaginateNormalizedHistoryRowsCollapsesSuffixedRoomMarker(t *testing.T) {
	rows := []protocol.Message{
		{
			"message_id": "room_mention_abc:agent-a",
			"round_id":   "room_mention_abc:agent-a",
			"role":       "user",
			"timestamp":  1000,
		},
		{
			"message_id": "result-agent-a",
			"round_id":   "room_mention_abc:agent-a",
			"agent_id":   "agent-a",
			"role":       "result",
			"timestamp":  2000,
		},
	}

	page := paginateNormalizedHistoryRows(rows, 1, "", 0, true)
	if page.HasMore {
		t.Fatalf("只有一个归一后的 room round，不应还有更多历史: %+v", page)
	}
	if len(page.Items) != 2 {
		t.Fatalf("带 agent 后缀的 marker/result 应同页返回: got=%d", len(page.Items))
	}
	for _, item := range page.Items {
		if item["round_id"] != "room_mention_abc" {
			t.Fatalf("返回给前端的 round_id 应已归一: %+v", page.Items)
		}
	}
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func derefInt64(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
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
		currentGroup.Items = append(
			currentGroup.Items,
			normalizeHistoryPageRow(row, collapseRoomAgentRounds),
		)
	}
	flushCurrentGroup()
	return groups
}

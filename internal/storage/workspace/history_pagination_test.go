package workspace

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestPaginateNormalizedHistoryRowsKeepsDMRoundComplete(t *testing.T) {
	rows := []protocol.Message{
		{
			"message_id": "user-1",
			"round_id":   "round-1",
			"role":       "user",
			"timestamp":  1000,
		},
		{
			"message_id": "assistant-1",
			"round_id":   "round-1",
			"role":       "assistant",
			"timestamp":  2000,
		},
		{
			"message_id": "result-1",
			"round_id":   "round-1",
			"role":       "result",
			"timestamp":  3000,
		},
		{
			"message_id": "user-2",
			"round_id":   "round-2",
			"role":       "user",
			"timestamp":  4000,
		},
		{
			"message_id": "assistant-2",
			"round_id":   "round-2",
			"role":       "assistant",
			"timestamp":  5000,
		},
		{
			"message_id": "result-2",
			"round_id":   "round-2",
			"role":       "result",
			"timestamp":  6000,
		},
	}

	page := paginateNormalizedHistoryRows(rows, 1, "", 0, false)
	if !page.HasMore {
		t.Fatalf("按 round 分页时应仍有更早历史: %+v", page)
	}
	if len(page.Items) != 3 {
		t.Fatalf("最新一页应保留完整 round: got=%d want=3", len(page.Items))
	}
	if page.Items[0]["message_id"] != "user-2" || page.Items[2]["message_id"] != "result-2" {
		t.Fatalf("最新一页 round 边界不正确: %+v", page.Items)
	}

	olderPage := paginateNormalizedHistoryRows(
		rows,
		1,
		derefString(page.NextBeforeRoundID),
		derefInt64(page.NextBeforeRoundTimestamp),
		false,
	)
	if olderPage.HasMore {
		t.Fatalf("更早一页不应继续有历史: %+v", olderPage)
	}
	if len(olderPage.Items) != 3 {
		t.Fatalf("更早一页应保留完整 round: got=%d want=3", len(olderPage.Items))
	}
	if olderPage.Items[0]["message_id"] != "user-1" || olderPage.Items[2]["message_id"] != "result-1" {
		t.Fatalf("更早一页 round 边界不正确: %+v", olderPage.Items)
	}
}

func TestPaginateNormalizedHistoryRowsCollapsesRoomAgentSubRounds(t *testing.T) {
	rows := []protocol.Message{
		{
			"message_id": "room-user-1",
			"round_id":   "room-round-1",
			"role":       "user",
			"timestamp":  1000,
		},
		{
			"message_id": "room-assistant-a-1",
			"round_id":   "room-round-1:agent-a",
			"agent_id":   "agent-a",
			"role":       "assistant",
			"timestamp":  2000,
		},
		{
			"message_id": "room-result-a-1",
			"round_id":   "room-round-1:agent-a",
			"agent_id":   "agent-a",
			"role":       "result",
			"timestamp":  3000,
		},
		{
			"message_id": "room-assistant-b-1",
			"round_id":   "room-round-1:agent-b",
			"agent_id":   "agent-b",
			"role":       "assistant",
			"timestamp":  4000,
		},
		{
			"message_id": "room-result-b-1",
			"round_id":   "room-round-1:agent-b",
			"agent_id":   "agent-b",
			"role":       "result",
			"timestamp":  5000,
		},
		{
			"message_id": "room-user-2",
			"round_id":   "room-round-2",
			"role":       "user",
			"timestamp":  6000,
		},
		{
			"message_id": "room-assistant-c-2",
			"round_id":   "room-round-2:agent-c",
			"agent_id":   "agent-c",
			"role":       "assistant",
			"timestamp":  7000,
		},
	}

	page := paginateNormalizedHistoryRows(rows, 1, "", 0, true)
	if !page.HasMore {
		t.Fatalf("Room 最新页应仍有更早主 round: %+v", page)
	}
	if len(page.Items) != 2 {
		t.Fatalf("Room 最新页应保留完整主 round: got=%d want=2", len(page.Items))
	}
	if page.Items[0]["message_id"] != "room-user-2" || page.Items[1]["message_id"] != "room-assistant-c-2" {
		t.Fatalf("Room 最新页主 round 不正确: %+v", page.Items)
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

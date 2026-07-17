package workspace

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestSortHistoryRowsUsesDisplayOrderForSameTimestamp(t *testing.T) {
	rows := []protocol.Message{
		{"message_id": "assistant-b", "role": "assistant", "round_id": "root", "timestamp": int64(10), "display_order": int64(10001)},
		{"message_id": "assistant-a", "role": "assistant", "round_id": "root", "timestamp": int64(10), "display_order": int64(10000)},
	}
	sortHistoryRows(rows)
	if rows[0]["message_id"] != "assistant-a" || rows[1]["message_id"] != "assistant-b" {
		t.Fatalf("同一时间应按 display_order 稳定排序: %#v", rows)
	}
}

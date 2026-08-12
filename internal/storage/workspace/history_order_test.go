package workspace

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestSortHistoryRowsPlacesGoalControlBeforeContinuationAtSameMillisecond(t *testing.T) {
	rows := []protocol.Message{
		{"message_id": "msg_a", "round_id": "goal-continuation", "role": "assistant", "timestamp": int64(1000)},
		{
			"message_id":   "msg_z",
			"round_id":     "goal-command",
			"role":         "user",
			"timestamp":    int64(1000),
			"control_only": true,
			"metadata":     map[string]string{"subtype": "goal_set"},
		},
	}

	sortHistoryRows(rows)

	if rows[0]["message_id"] != "msg_z" || rows[1]["message_id"] != "msg_a" {
		t.Fatalf("same-millisecond Goal order = %#v, want control before continuation", rows)
	}
}

func TestSortHistoryRowsPreservesStableOrderForUnrelatedRoundsAtSameMillisecond(t *testing.T) {
	rows := []protocol.Message{
		{"message_id": "msg_z", "round_id": "round-z", "role": "assistant", "timestamp": int64(1000)},
		{"message_id": "msg_a", "round_id": "round-a", "role": "user", "timestamp": int64(1000)},
	}

	sortHistoryRows(rows)

	if rows[0]["message_id"] != "msg_z" || rows[1]["message_id"] != "msg_a" {
		t.Fatalf("unrelated same-millisecond order = %#v, want stable merge order", rows)
	}
}

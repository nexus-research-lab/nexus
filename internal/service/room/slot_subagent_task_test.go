package room

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRoomSlotTracksRunningSubagentTasks(t *testing.T) {
	slot := &activeRoomSlot{}
	slot.rememberSubagentTaskMessage(protocol.Message{
		"metadata": map[string]any{
			"subtype":    "task_started",
			"task_id":    "task-1",
			"agent_id":   "agent-1",
			"agent_type": "worker",
		},
	})
	if !slot.hasRunningSubagentTask() {
		t.Fatal("task_started 后应记录 running subagent")
	}

	slot.rememberSubagentTaskMessage(protocol.Message{
		"metadata": map[string]any{
			"subtype": "task_updated",
			"task_id": "task-1",
			"status":  "killed",
		},
	})
	if slot.hasRunningSubagentTask() {
		t.Fatal("terminal task_updated 后应清除 running subagent")
	}
}

func TestRoomRoundReportsRunningSubagentTasks(t *testing.T) {
	roundValue := &activeRoomRound{Slots: map[string]*activeRoomSlot{
		"agent-1": {SubagentTasks: map[string]struct{}{"task-1": {}}},
	}}
	if !roundValue.hasRunningSubagentTasks() {
		t.Fatal("round 应能汇总 slot 中的 running subagent")
	}
}

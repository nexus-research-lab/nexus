package runtimehost

import (
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"testing"
)

func TestStopSubagentsClosesLateAdmissionWithoutStoppingSuccessor(t *testing.T) {
	var stopped []string
	parent := &GoalRoundState{SubagentTasks: map[string]struct{}{"existing": {}}}
	stop := func(id string) { stopped = append(stopped, id); parent.KnowsSubagentTask(id) }
	parent.StopSubagents(stop)
	parent.StopSubagents(stop)
	message := func(id, status, subtype string) protocol.Message {
		return protocol.Message{"metadata": map[string]any{"task_id": id, "status": status, "subtype": subtype, "task_type": "local_agent"}}
	}
	parent.RememberSubagentTaskMessage(message("late", "running", "task_started"))
	parent.RememberSubagentTaskMessage(message("late", "completed", "task_notification"))
	successor := &GoalRoundState{}
	successor.RememberSubagentTaskMessage(message("successor", "running", "task_started"))
	if len(stopped) != 2 || stopped[0] != "existing" || stopped[1] != "late" {
		t.Fatalf("stop targets = %v", stopped)
	}
	if parent.KnowsSubagentTask("late") {
		t.Fatal("terminal child retained as running")
	}
}

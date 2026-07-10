package dm

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

func TestSubagentPostRoundDispatchIsClaimedOnceAcrossTaskFollowUp(t *testing.T) {
	runner := &roundRunner{
		service:     &Service{runtime: runtimectx.NewManager()},
		sessionKey:  "agent:host:ws:dm:conversation-1",
		runtimeKind: "nxs",
	}

	runner.rememberSubagentTaskMessage(dmSubagentTaskMessage("task_started", "running"))
	if runner.claimSubagentPostRoundDispatch() {
		t.Fatal("running task 不应提前触发 post-round work")
	}
	runner.rememberSubagentTaskMessage(dmSubagentTaskMessage("task_notification", "completed"))
	if !runner.claimSubagentPostRoundDispatch() {
		t.Fatal("task 首次完成后应触发一次 post-round work")
	}

	// UI 用同 task ID 续聊并再次完成时，不能重复消费输入队列或启动 Goal continuation。
	runner.rememberSubagentTaskMessage(dmSubagentTaskMessage("task_progress", "running"))
	runner.rememberSubagentTaskMessage(dmSubagentTaskMessage("task_notification", "completed"))
	if runner.claimSubagentPostRoundDispatch() {
		t.Fatal("同一父 round 的 task follow-up 不应重复触发 post-round work")
	}
}

func TestDMIgnoresLocalShellTaskLifecycle(t *testing.T) {
	runtimeManager := runtimectx.NewManager()
	runner := &roundRunner{
		service:    &Service{runtime: runtimeManager},
		sessionKey: "agent:host:ws:dm:conversation-shell",
	}
	runner.rememberSubagentTaskMessage(protocol.Message{
		"metadata": map[string]any{
			"subtype":    "task_started",
			"task_id":    "shell-task",
			"agent_id":   "host",
			"agent_type": "shell",
			"task_type":  "local_shell",
		},
	})
	if runner.hasRunningSubagentTask() || runtimeManager.HasSubagentHistory(runner.sessionKey) {
		t.Fatal("local_shell 不应保活 DM subagent runtime")
	}
}

func dmSubagentTaskMessage(subtype string, status string) protocol.Message {
	return protocol.Message{
		"metadata": map[string]any{
			"subtype":    subtype,
			"task_id":    "task-1",
			"agent_id":   "subagent-1",
			"agent_type": "worker",
			"status":     status,
		},
	}
}

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

func TestDMSubagentUsagePendingBlocksTerminalDispatch(t *testing.T) {
	runner := &roundRunner{}
	runner.rememberSubagentTaskMessage(dmSubagentTaskMessage("task_started", "running"))
	runner.markSubagentUsagePending("task-1", 0)
	runner.rememberSubagentTaskMessage(dmSubagentTaskMessage("task_notification", "completed"))

	if !runner.hasRunningSubagentTask() {
		t.Fatal("terminal lifecycle 已结束但 usage checkpoint 未完成时仍应保留 join barrier")
	}
	if runner.claimSubagentPostRoundDispatch() {
		t.Fatal("usage checkpoint 未完成时不应触发 post-round work")
	}

	runner.clearSubagentUsagePending("task-1", 0)
	if runner.hasRunningSubagentTask() {
		t.Fatal("usage checkpoint 完成后应释放 join barrier")
	}
	if !runner.claimSubagentPostRoundDispatch() {
		t.Fatal("usage checkpoint 完成后应允许一次 post-round work")
	}
}

func TestDMOlderSettledUsageDoesNotClearNewerPendingSnapshot(t *testing.T) {
	runner := &roundRunner{}
	runner.markSubagentUsagePending("task-1", 0)
	if pending, ok := runner.subagentUsagePending["task-1"]; !ok || pending.cumulativeTotal != 0 {
		t.Fatalf("explicit zero pending = %#v, %v; want stored zero snapshot", pending, ok)
	}

	runner.markSubagentUsagePending("task-1", 100)
	runner.markSubagentUsagePending("task-1", 150)
	runner.clearSubagentUsagePending("task-1", 100)
	if pending := runner.subagentUsagePending["task-1"]; pending.cumulativeTotal != 150 {
		t.Fatalf("older success cleared newer pending: got %#v, want total 150", pending)
	}
	if !runner.hasRunningSubagentTask() {
		t.Fatal("newer pending snapshot must keep the child join barrier")
	}

	runner.clearSubagentUsagePending("task-1", 150)
	if runner.hasRunningSubagentTask() {
		t.Fatal("latest settled snapshot did not release the child join barrier")
	}
}

func TestDMOlderProgressSettlementDoesNotClearSameTotalTerminalEvidence(t *testing.T) {
	runner := &roundRunner{}
	progress := dmSubagentUsageObservation{cumulativeTotal: 25}
	terminal := dmSubagentUsageObservation{cumulativeTotal: 25, terminal: true}
	runner.markSubagentUsageObservationPending("task-1", progress)
	runner.markSubagentUsageObservationPending("task-1", terminal)

	runner.clearSubagentUsageObservationPending("task-1", progress)
	pending, ok := runner.subagentUsagePending["task-1"]
	if !ok || !pending.terminal {
		t.Fatalf("progress settlement cleared terminal evidence: %#v, present=%v", pending, ok)
	}
	runner.clearSubagentUsageObservationPending("task-1", terminal)
	if runner.hasRunningSubagentTask() {
		t.Fatal("terminal evidence settlement did not release join barrier")
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

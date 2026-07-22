package realtime

import (
	"context"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"
)

type fakeTokenUsageRecorder struct {
	inputs []usagesvc.RecordInput
}

func (r *fakeTokenUsageRecorder) RecordMessageUsage(_ context.Context, input usagesvc.RecordInput) error {
	r.inputs = append(r.inputs, input)
	return nil
}

type permissionModeTestClient struct {
	modes           []sdkpermission.Mode
	hookResponseAck bool
}

func (c *permissionModeTestClient) Connect(context.Context) error { return nil }

func (c *permissionModeTestClient) Query(context.Context, string) error { return nil }

func (c *permissionModeTestClient) ReceiveMessages(context.Context) <-chan sdkprotocol.ReceivedMessage {
	closed := make(chan sdkprotocol.ReceivedMessage)
	close(closed)
	return closed
}

func (c *permissionModeTestClient) Interrupt(context.Context) error { return nil }

func (c *permissionModeTestClient) StopTask(context.Context, string) error { return nil }

func (c *permissionModeTestClient) SendTaskMessage(context.Context, string, string, string) error {
	return nil
}

func (c *permissionModeTestClient) RemoveMessages(context.Context, []string) error { return nil }

func (c *permissionModeTestClient) SetPermissionMode(_ context.Context, mode sdkpermission.Mode) error {
	c.modes = append(c.modes, mode)
	return nil
}

func (c *permissionModeTestClient) Disconnect(context.Context) error { return nil }

func (c *permissionModeTestClient) Reconfigure(context.Context, agentclient.Options) error {
	return nil
}

func (c *permissionModeTestClient) Supports(capability agentclient.Capability) bool {
	return c.hookResponseAck && capability == agentclient.CapabilityHookResponseAck
}

func (c *permissionModeTestClient) SessionID() string { return "" }

func TestRoomUsagePrefersResultAggregateOverTerminalAssistant(t *testing.T) {
	t.Parallel()
	recorder := &fakeTokenUsageRecorder{}
	service := &Service{usage: recorder}
	roundValue := &activeRoomRound{OwnerUserID: "user-1", SessionKey: "room:session"}
	slot := &activeRoomSlot{AgentID: "agent-1", AgentRoundID: "agent-round-1"}
	result := protocol.Message{
		"role": "result", "message_id": "result-1", "session_key": "room:session", "round_id": "agent-round-1",
		"usage": map[string]any{"input_tokens": 10},
	}
	assistant := protocol.Message{
		"role": "assistant", "message_id": "assistant-1", "session_key": "room:session", "round_id": "agent-round-1",
		"usage": map[string]any{"input_tokens": 3},
	}

	service.recordUsage(roundValue, slot, result)
	service.recordTerminalAssistantUsage(roundValue, slot, assistant)

	if len(recorder.inputs) != 1 || recorder.inputs[0].MessageID != "result-1" {
		t.Fatalf("应只记录 result 聚合 usage，实际=%+v", recorder.inputs)
	}
}

func TestRoomUsageFallsBackToTerminalAssistantWhenResultUsageEmpty(t *testing.T) {
	t.Parallel()
	recorder := &fakeTokenUsageRecorder{}
	service := &Service{usage: recorder}
	roundValue := &activeRoomRound{OwnerUserID: "user-1", SessionKey: "room:session"}
	slot := &activeRoomSlot{AgentID: "agent-1", AgentRoundID: "agent-round-1"}

	service.recordUsage(roundValue, slot, protocol.Message{
		"role": "result", "message_id": "result-empty", "session_key": "room:session", "round_id": "agent-round-1",
		"usage": map[string]any{},
	})
	service.recordTerminalAssistantUsage(roundValue, slot, protocol.Message{
		"role": "assistant", "message_id": "assistant-1", "session_key": "room:session", "round_id": "agent-round-1",
		"usage": map[string]any{"input_tokens": 3},
	})

	if len(recorder.inputs) != 1 || recorder.inputs[0].MessageID != "assistant-1" {
		t.Fatalf("应 fallback 记录 assistant usage，实际=%+v", recorder.inputs)
	}
}

func TestSetPermissionModeForAgentUpdatesActiveRoomSlots(t *testing.T) {
	matching := &permissionModeTestClient{}
	other := &permissionModeTestClient{}
	terminal := &permissionModeTestClient{}
	matchingSlot := &activeRoomSlot{AgentID: "agent-a"}
	matchingSlot.setClient(matching)
	matchingSlot.setStatus("running")
	otherSlot := &activeRoomSlot{AgentID: "agent-b"}
	otherSlot.setClient(other)
	otherSlot.setStatus("running")
	terminalSlot := &activeRoomSlot{AgentID: "agent-a"}
	terminalSlot.setClient(terminal)
	terminalSlot.setStatus("finished")
	service := &Service{rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{
		"round-1": {Slots: map[string]*activeRoomSlot{
			"matching": matchingSlot,
			"other":    otherSlot,
			"terminal": terminalSlot,
		}},
	})}

	if err := service.SetPermissionModeForAgent(context.Background(), "agent-a", sdkpermission.ModePlan); err != nil {
		t.Fatalf("SetPermissionModeForAgent() error = %v", err)
	}
	if len(matching.modes) != 1 || matching.modes[0] != sdkpermission.ModePlan {
		t.Fatalf("matching modes = %#v，期望 [plan]", matching.modes)
	}
	if len(other.modes) != 0 || len(terminal.modes) != 0 {
		t.Fatalf("非活动目标不应更新：other=%#v terminal=%#v", other.modes, terminal.modes)
	}
}

func TestRoomSlotTracksRunningSubagentTasks(t *testing.T) {
	slot := &activeRoomSlot{}
	slot.rememberSubagentTaskMessage(protocol.Message{"metadata": map[string]any{
		"subtype": "task_started", "task_id": "task-1", "agent_id": "agent-1", "agent_type": "worker",
	}})
	if !slot.hasRunningSubagentTask() {
		t.Fatal("task_started 后应记录 running subagent")
	}
	slot.rememberSubagentTaskMessage(protocol.Message{"metadata": map[string]any{
		"subtype": "task_updated", "task_id": "task-1", "status": "killed",
	}})
	if slot.hasRunningSubagentTask() {
		t.Fatal("terminal task_updated 后应清除 running subagent")
	}
}

func TestRoomRoundReportsRunningSubagentTasks(t *testing.T) {
	slot := &activeRoomSlot{}
	slot.setSubagentTasks(map[string]struct{}{"task-1": {}})
	roundValue := &activeRoomRound{Slots: map[string]*activeRoomSlot{
		"agent-1": slot,
	}}
	if !roundValue.hasRunningSubagentTasks() {
		t.Fatal("round 应能汇总 slot 中的 running subagent")
	}
}

func TestRoomRoundSelectsEarliestSlotError(t *testing.T) {
	later := &activeRoomSlot{Index: 2}
	later.setErrorMessage("later provider error")
	earlier := &activeRoomSlot{Index: 1}
	earlier.setErrorMessage("  first provider error  ")
	roundValue := &activeRoomRound{Slots: map[string]*activeRoomSlot{
		"agent-later":   later,
		"agent-earlier": earlier,
		"agent-empty":   {Index: 0},
	}}

	if got := roundValue.firstSlotErrorMessage(); got != "first provider error" {
		t.Fatalf("firstSlotErrorMessage() = %q，期望最早失败 slot 的原因", got)
	}
}

func TestRoomSlotTerminalStatusFallsBackToRuntimeStatus(t *testing.T) {
	if got := roomSlotTerminalStatus(exec.RoundExecutionResult{TerminalStatus: "error"}); got != "error" {
		t.Fatalf("roomSlotTerminalStatus(error status) = %q, want error", got)
	}
	if got := roomSlotTerminalStatus(exec.RoundExecutionResult{ResultSubtype: "interrupted"}); got != "cancelled" {
		t.Fatalf("roomSlotTerminalStatus(interrupted subtype) = %q, want cancelled", got)
	}
}

func TestRoomSlotIgnoresLocalShellTaskLifecycle(t *testing.T) {
	slot := &activeRoomSlot{}
	slot.rememberSubagentTaskMessage(protocol.Message{"metadata": map[string]any{
		"subtype": "task_started", "task_id": "shell-task", "agent_id": "host-agent",
		"agent_type": "shell", "task_type": "local_shell",
	}})
	if slot.hasRunningSubagentTask() || slot.hasSubagentHistory() {
		t.Fatal("local_shell 不应进入 Room subagent 生命周期")
	}
}

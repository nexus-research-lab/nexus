package realtime

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	messagepkg "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"
)

type fakeTokenUsageRecorder struct {
	inputs []usagesvc.RecordInput
}

func (r *fakeTokenUsageRecorder) RecordMessageUsage(_ context.Context, input usagesvc.RecordInput) error {
	r.inputs = append(r.inputs, input)
	return nil
}

func TestRoomDirectedReplyUsesAutomaticRoute(t *testing.T) {
	const conversationID = "conversation-directed-reply-route"
	slot := &activeRoomSlot{AgentID: "worker", AgentRoundID: "agent-round-1"}
	slot.setDeliveryMetadata(protocol.RoomReplyRoute{
		Mode:       protocol.RoomReplyRoutePrivate,
		Recipients: []string{"host"},
	}, "question-1", "")
	service := &Service{rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{
		"round-1": {
			SessionKey:     protocol.BuildRoomSharedSessionKey(conversationID),
			ConversationID: conversationID,
			RoundID:        "round-1",
			Slots:          map[string]*activeRoomSlot{"slot-1": slot},
		},
	})}
	message := protocol.RoomDirectedMessageRecord{
		ConversationID: conversationID,
		SourceAgentID:  "worker",
		Recipients:     []string{"host"},
	}
	if !service.roomDirectedReplyUsesAutomaticRoute("agent-round-1", message) {
		t.Fatal("当前私域回复应交给 runtime reply_route")
	}
	message.Recipients = []string{"other"}
	if service.roomDirectedReplyUsesAutomaticRoute("agent-round-1", message) {
		t.Fatal("发给其他成员的私域消息不应被自动回复路由拦截")
	}
}

type permissionModeTestClient struct {
	modes           []sdkpermission.Mode
	modeErr         error
	interruptCalls  int
	hookResponseAck bool
}

type roomInterruptPermissionSender struct {
	key    string
	events chan protocol.EventMessage
}

func (s *roomInterruptPermissionSender) Key() string { return s.key }

func (s *roomInterruptPermissionSender) IsClosed() bool { return false }

func (s *roomInterruptPermissionSender) SendEvent(_ context.Context, event protocol.EventMessage) error {
	s.events <- event
	return nil
}

func (c *permissionModeTestClient) Connect(context.Context) error { return nil }

func (c *permissionModeTestClient) Query(context.Context, string) error { return nil }

func (c *permissionModeTestClient) ReceiveMessages(context.Context) <-chan sdkprotocol.ReceivedMessage {
	closed := make(chan sdkprotocol.ReceivedMessage)
	close(closed)
	return closed
}

func (c *permissionModeTestClient) Interrupt(context.Context) error {
	c.interruptCalls++
	return nil
}

func (c *permissionModeTestClient) StopTask(context.Context, string) error { return nil }

func (c *permissionModeTestClient) SendTaskMessage(context.Context, string, string, string) error {
	return nil
}

func (c *permissionModeTestClient) RemoveMessages(context.Context, []string) error { return nil }

func (c *permissionModeTestClient) SetPermissionMode(_ context.Context, mode sdkpermission.Mode) error {
	c.modes = append(c.modes, mode)
	return c.modeErr
}

func (c *permissionModeTestClient) Retire() {}

func (c *permissionModeTestClient) Disconnect(context.Context) error { return nil }

func (c *permissionModeTestClient) Reconfigure(context.Context, agentclient.Options) error {
	return nil
}

func (c *permissionModeTestClient) Supports(capability agentclient.Capability) bool {
	return c.hookResponseAck && capability == agentclient.CapabilityHookResponseAck
}

func (c *permissionModeTestClient) SessionID() string { return "" }

func TestInterruptActiveSlotSeparatesControlAndDisplayReasons(t *testing.T) {
	tests := map[string]struct {
		interruptReason string
		wantStored      string
		wantDecision    string
	}{
		"default stop": {
			wantStored: messagepkg.InterruptWithoutMessage,
		},
		"custom reason": {
			interruptReason: "  收到新消息，上一轮已停止  ",
			wantStored:      "收到新消息，上一轮已停止",
			wantDecision:    "收到新消息，上一轮已停止",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			permission := permissionctx.NewContext()
			conversationID := "conversation-interrupt-display"
			sessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
			runtimeSessionKey := protocol.BuildRoomAgentSessionKey(conversationID, "agent-1", protocol.RoomTypeGroup)
			sender := &roomInterruptPermissionSender{
				key:    "room-interrupt-display-" + name,
				events: make(chan protocol.EventMessage, 4),
			}
			permission.BindSession(runtimeSessionKey, sender)

			decisionCh := make(chan sdkpermission.Decision, 1)
			go func() {
				decision, _ := permission.RequestPermission(context.Background(), runtimeSessionKey, sdkpermission.Request{
					ToolName: "Read",
					Input:    map[string]any{"file_path": "go.mod"},
				})
				decisionCh <- decision
			}()

			select {
			case event := <-sender.events:
				if event.EventType != protocol.EventTypePermissionRequest {
					t.Fatalf("期望 permission_request，实际: %+v", event)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("等待 Room 权限请求失败")
			}

			slot := &activeRoomSlot{
				AgentID:           "agent-1",
				AgentRoundID:      "agent-round-1",
				MsgID:             "assistant-1",
				RuntimeSessionKey: runtimeSessionKey,
			}
			slot.setStatus("running")
			slot.closeDone()
			service := &Service{
				Host: runtimehost.Host{Permission: permission, Runtime: runtimectx.NewManager()},
			}
			if err := service.interruptActiveSlot(context.Background(), &activeRoomRound{
				SessionKey:     sessionKey,
				RoomID:         "room-1",
				ConversationID: conversationID,
			}, slot, test.interruptReason); err != nil {
				t.Fatalf("interruptActiveSlot() error = %v", err)
			}

			if got := roomSlotInterruptReason(slot); got != test.wantStored {
				t.Fatalf("slot 控制值=%q，期望=%q", got, test.wantStored)
			}
			select {
			case decision := <-decisionCh:
				if decision.Behavior != sdkpermission.BehaviorDeny || !decision.Interrupt {
					t.Fatalf("中断权限决策不正确: %+v", decision)
				}
				if decision.Message != test.wantDecision {
					t.Fatalf("权限展示文案=%q，期望=%q", decision.Message, test.wantDecision)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("等待 Room 权限取消决策失败")
			}
		})
	}
}

func TestSetPermissionModeForAgentInterruptsFailedSlotAndContinues(t *testing.T) {
	failed := &permissionModeTestClient{modeErr: errors.New("unsupported live update")}
	succeeded := &permissionModeTestClient{}
	failedSlot := &activeRoomSlot{AgentID: "agent-a", MsgID: "failed"}
	failedSlot.setClient(failed)
	failedSlot.setStatus("running")
	succeededSlot := &activeRoomSlot{AgentID: "agent-a", MsgID: "succeeded"}
	succeededSlot.setClient(succeeded)
	succeededSlot.setStatus("running")
	service := &Service{
		Host: runtimehost.Host{Permission: permissionctx.NewContext()},
		rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{
			"round-1": {
				SessionKey: "room:session", RoomID: "room-1",
				Slots: map[string]*activeRoomSlot{"failed": failedSlot, "succeeded": succeededSlot},
			},
		}),
	}

	err := service.SetPermissionModeForAgent(context.Background(), "agent-a", sdkpermission.ModePlan)
	if err == nil || !strings.Contains(err.Error(), "已中断旧 slot") {
		t.Fatalf("SetPermissionModeForAgent() error = %v", err)
	}
	if failed.interruptCalls != 1 || !failedSlot.isTerminal() {
		t.Fatalf("failed slot was not interrupted: calls=%d status=%s", failed.interruptCalls, failedSlot.getStatus())
	}
	if len(succeeded.modes) != 1 || succeeded.modes[0] != sdkpermission.ModePlan {
		t.Fatalf("later matching slot was not updated: %#v", succeeded.modes)
	}
}

func TestRoomSlotTracksRunningSubagentTasks(t *testing.T) {
	slot := &activeRoomSlot{}
	slot.rememberSubagentTaskMessage(protocol.Message{"metadata": map[string]any{
		"subtype": "task_started", "task_id": "task-1", "agent_id": "agent-1", "agent_type": "worker",
	}})
	if !slot.mutable.goal.HasRunningSubagentTask() {
		t.Fatal("task_started 后应记录 running subagent")
	}
	slot.rememberSubagentTaskMessage(protocol.Message{"metadata": map[string]any{
		"subtype": "task_updated", "task_id": "task-1", "status": "killed",
	}})
	if slot.mutable.goal.HasRunningSubagentTask() {
		t.Fatal("terminal task_updated 后应清除 running subagent")
	}
}

func TestRoomSlotUsagePendingKeepsLatestCumulativeValue(t *testing.T) {
	slot := &activeRoomSlot{}
	slot.markSubagentUsagePending("task-1", 0)
	if pending := slot.subagentUsagePendingSnapshot(); pending["task-1"] != 0 {
		t.Fatalf("explicit zero pending = %#v, want task retained with 0", pending)
	}

	slot.markSubagentUsagePending("task-1", 200)
	slot.markSubagentUsagePending("task-1", 100)
	if pending := slot.subagentUsagePendingSnapshot(); pending["task-1"] != 200 {
		t.Fatalf("out-of-order cumulative snapshot moved pending backward: %#v", pending)
	}
	slot.clearSubagentUsagePending("task-1", 100)
	if pending := slot.subagentUsagePendingSnapshot(); pending["task-1"] != 200 {
		t.Fatalf("old settlement cleared newer pending: %#v", pending)
	}

	slot.clearSubagentUsagePending("task-1", 200)
	if pending := slot.subagentUsagePendingSnapshot(); len(pending) != 0 {
		t.Fatalf("matching settlement left pending: %#v", pending)
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
	if slot.mutable.goal.HasRunningSubagentTask() || slot.hasSubagentHistory() {
		t.Fatal("local_shell 不应进入 Room subagent 生命周期")
	}
}

// 会话 round 注册表测试。

func TestRoomRoundRegistryKeepsPublicWakeAfterRoundUnregister(t *testing.T) {
	const conversationID = "conversation-public-wake-lifecycle"
	roundValue := &activeRoomRound{
		SessionKey:     protocol.BuildRoomSharedSessionKey(conversationID),
		ConversationID: conversationID,
		RoundID:        "round-public-wake",
		Slots:          make(map[string]*activeRoomSlot),
	}
	registry := newRoomRoundRegistry()
	registry.register(roundValue)
	wake := publicMentionWake{TargetAgentID: "agent-peer", Content: "继续处理"}
	if !registry.enqueuePublicMention(roundValue, wake) {
		t.Fatal("首次 public wake 入队失败")
	}

	registry.unregister(roundValue)
	if !registry.hasPublicMentions(roundValue) {
		t.Fatal("round 注销后不应丢失待处理 public wake")
	}
	if !registry.hasPublicMentionsForConversation(conversationID) {
		t.Fatal("round 注销后 conversation 仍应报告待处理 public wake")
	}
	wakes := registry.takePublicMentions(roundValue)
	if len(wakes) != 1 || wakes[0].TargetAgentID != wake.TargetAgentID {
		t.Fatalf("取出的 public wake = %+v, want %+v", wakes, wake)
	}
	if registry.hasPublicMentions(roundValue) {
		t.Fatal("public wake 消费后仍残留")
	}
	if registry.hasPublicMentionsForConversation(conversationID) {
		t.Fatal("public wake 消费后 conversation 仍报告 pending wake")
	}
	if got := len(registry.snapshotConversation(conversationID)); got != 0 {
		t.Fatalf("round 注销后 active round 数 = %d, want 0", got)
	}
}

// 会话派发状态测试。

// 运行时状态测试。

// 插槽测试夹具。

// withRoomSlotStatus 让测试只声明稳定身份，再显式设置 runtime 状态。
func withRoomSlotStatus(slot *activeRoomSlot, status string) *activeRoomSlot {
	if slot != nil {
		slot.setStatus(status)
	}
	return slot
}

func (s *activeRoomSlot) currentGoalObjectiveRevision() int64 {
	if s == nil {
		return 0
	}
	return s.mutable.goal.objectiveRevision.Load()
}

func (slot *activeRoomSlot) setGoalUsageAccumulator(usage *goalsvc.RuntimeUsageAccumulator) {
	if slot == nil {
		return
	}
	slot.mutable.goal.Mu.Lock()
	slot.mutable.goal.Usage = usage
	slot.mutable.goal.Mu.Unlock()
}

func (slot *activeRoomSlot) goalUsageActive() bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.Mu.Lock()
	defer slot.mutable.goal.Mu.Unlock()
	return slot.mutable.goal.Usage != nil && slot.mutable.goal.Usage.Active()
}

func (slot *activeRoomSlot) publicMessageWasPublished() bool {
	if slot == nil {
		return false
	}
	slot.mutable.delivery.mu.Lock()
	defer slot.mutable.delivery.mu.Unlock()
	return slot.mutable.delivery.publicMessagePublished
}

func (slot *activeRoomSlot) setPendingStream(events []protocol.EventMessage) {
	if slot == nil {
		return
	}
	slot.mutable.delivery.mu.Lock()
	slot.mutable.delivery.pendingStream = slices.Clone(events)
	slot.mutable.delivery.mu.Unlock()
}

// markSubagentUsagePending 建立独立的 source 持久化 join barrier，并保留每个 task
// 最大的累计值（首次显式 0 也会保留）。它与 runtime task 生命周期分开，防止终态消息先移除
// task、后写 checkpoint 时被并发 finalization 穿透。
func (slot *activeRoomSlot) markSubagentUsagePending(taskID string, cumulativeTotal int64) {
	slot.markSubagentUsageObservationPending(goalsvc.SubagentUsageObservation{
		CumulativeTotal: cumulativeTotal,
	}, taskID)
}

// clearSubagentUsagePending 只确认不晚于 settledTotal 的 pending。旧请求成功返回时，
// 若同 task 已到达更大的累计值，则必须保留新值给 retry worker 重放。
func (slot *activeRoomSlot) clearSubagentUsagePending(taskID string, settledTotal int64) {
	slot.clearSubagentUsageObservationPending(taskID, goalsvc.SubagentUsageObservation{
		CumulativeTotal:            settledTotal,
		Terminal:                   true,
		TerminalTokenUsageObserved: true,
	})
}

func (slot *activeRoomSlot) setSubagentTasks(tasks map[string]struct{}) {
	if slot == nil {
		return
	}
	slot.mutable.goal.Mu.Lock()
	slot.mutable.goal.SubagentTasks = tasks
	slot.mutable.goal.Mu.Unlock()
}

package realtime

import (
	"context"
	"errors"
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

func TestRoomUsagePrefersResultAggregateOverTerminalAssistant(t *testing.T) {
	t.Parallel()
	recorder := &fakeTokenUsageRecorder{}
	service := &Service{usage: recorder, runtime: runtimectx.NewManager()}
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
	service := &Service{usage: recorder, runtime: runtimectx.NewManager()}
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
				permission: permission,
				runtime:    runtimectx.NewManager(),
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
		permission: permissionctx.NewContext(),
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

func TestRoomSlotUsagePendingSurvivesTerminalLifecycle(t *testing.T) {
	slot := &activeRoomSlot{}
	slot.rememberSubagentTaskMessage(protocol.Message{"metadata": map[string]any{
		"subtype": "task_started", "task_id": "task-1", "agent_id": "agent-1", "agent_type": "worker",
	}})
	slot.markSubagentUsagePending("task-1", 0)
	slot.rememberSubagentTaskMessage(protocol.Message{"metadata": map[string]any{
		"subtype": "task_notification", "task_id": "task-1", "status": "completed",
	}})

	if !slot.hasRunningSubagentTask() {
		t.Fatal("terminal lifecycle 已结束但 usage checkpoint 未完成时仍应保留 join barrier")
	}

	slot.clearSubagentUsagePending("task-1", 0)
	if slot.hasRunningSubagentTask() {
		t.Fatal("usage checkpoint 完成后应释放 join barrier")
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

func TestRoomDispatchStateUsesConversationBoundary(t *testing.T) {
	registry := newRoomRoundRegistry()
	first := registry.acquireDispatch(roomDispatchStateKey("room:shared:conversation-a", "conversation-a"))
	if got := registry.state("conversation-a", false); got != first.state {
		t.Fatal("dispatch lease 未绑定到 conversation state")
	}

	sameWaiting := make(chan struct{})
	sameAcquired := make(chan struct{})
	sameDone := make(chan struct{})
	go func() {
		close(sameWaiting)
		lease := registry.acquireDispatch(roomDispatchStateKey("room:agent:conversation-a", "conversation-a"))
		close(sameAcquired)
		lease.Unlock()
		close(sameDone)
	}()
	<-sameWaiting

	select {
	case <-sameAcquired:
		t.Fatal("同一 conversation 的 dispatch 不应并发通过")
	default:
	}

	differentAcquired := make(chan struct{})
	differentDone := make(chan struct{})
	go func() {
		lease := registry.acquireDispatch(roomDispatchStateKey("room:shared:conversation-b", "conversation-b"))
		close(differentAcquired)
		lease.Unlock()
		close(differentDone)
	}()
	select {
	case <-differentAcquired:
	case <-time.After(time.Second):
		t.Fatal("不同 conversation 不应被同一把 dispatch 锁阻塞")
	}
	<-differentDone

	first.Unlock()
	select {
	case <-sameDone:
	case <-time.After(time.Second):
		t.Fatal("释放 conversation dispatch 后等待者未获得闸门")
	}

	registry.mu.RLock()
	defer registry.mu.RUnlock()
	if len(registry.conversations) != 0 {
		t.Fatalf("dispatch lease 释放后仍残留 conversation state: %d", len(registry.conversations))
	}
}

func TestRoomDispatchStateKeyNormalizesSharedAndAgentSession(t *testing.T) {
	conversationID := "conversation-dispatch-key"
	shared := protocol.BuildRoomSharedSessionKey(conversationID)
	agent := protocol.BuildRoomAgentSessionKey(conversationID, "agent-a", protocol.RoomTypeGroup)
	if got, want := roomDispatchStateKey(shared, ""), roomDispatchStateKey(agent, ""); got != want {
		t.Fatalf("shared/agent session dispatch keys differ: got=%q want=%q", got, want)
	}
	dmAgent := protocol.BuildRoomAgentSessionKey("dm-ref", "agent-a", "dm")
	if got := roomConversationIDFromSessionKey(dmAgent); got != "" {
		t.Fatalf("DM agent session was treated as Room conversation: %q", got)
	}
}

func TestRoomDispatchStateKeepsConversationStateUntilRelease(t *testing.T) {
	registry := newRoomRoundRegistry()
	roundValue := &activeRoomRound{
		ConversationID: "conversation-active",
		SessionKey:     "room:shared:conversation-active",
		RoundID:        "round-1",
	}
	registry.register(roundValue)

	lease := registry.acquireDispatch(roomDispatchStateKey(roundValue.SessionKey, roundValue.ConversationID))
	registry.unregister(roundValue)

	registry.mu.RLock()
	state := registry.conversations[roundValue.ConversationID]
	registry.mu.RUnlock()
	if state == nil {
		t.Fatal("dispatch lease 持有期间不应删除 conversation state")
	}

	lease.Unlock()
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	if registry.conversations[roundValue.ConversationID] != nil {
		t.Fatal("dispatch 释放后应清理空 conversation state")
	}
}

func TestRoomDispatchStateSeparatesUnknownSessions(t *testing.T) {
	registry := newRoomRoundRegistry()
	firstSession := "legacy-session-a"
	secondSession := "legacy-session-b"
	if got := roomConversationIDFromSessionKey(firstSession); got != "" {
		t.Fatalf("测试 session 意外解析出 conversation: %q", got)
	}
	first := registry.acquireDispatch(roomDispatchStateKey(firstSession, ""))
	secondAcquired := make(chan struct{})
	secondDone := make(chan struct{})
	go func() {
		second := registry.acquireDispatch(roomDispatchStateKey(secondSession, ""))
		close(secondAcquired)
		second.Unlock()
		close(secondDone)
	}()

	select {
	case <-secondAcquired:
	case <-time.After(time.Second):
		t.Fatal("无法解析 conversation 的不同 session 不应共享 dispatch 锁")
	}
	first.Unlock()
	select {
	case <-secondDone:
	case <-time.After(time.Second):
		t.Fatal("unknown session dispatch goroutine 未完成")
	}
}

// 运行时状态测试。

func TestRoomContextColdStartUsesWarmRuntimeBeforeResume(t *testing.T) {
	tests := []struct {
		name           string
		resumeID       string
		hadWarmSession bool
		want           bool
	}{
		{name: "new session", want: true},
		{name: "persisted resume", resumeID: "sdk-session-1", want: false},
		{name: "warm manager session", hadWarmSession: true, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := roomContextColdStart(test.resumeID, test.hadWarmSession); got != test.want {
				t.Fatalf("roomContextColdStart() = %v, want %v", got, test.want)
			}
		})
	}
}

// 插槽测试夹具。

// withRoomSlotStatus 让测试只声明稳定身份，再显式设置 runtime 状态。
func withRoomSlotStatus(slot *activeRoomSlot, status string) *activeRoomSlot {
	if slot != nil {
		slot.setStatus(status)
	}
	return slot
}

type recordingRoomBroadcaster struct {
	events []protocol.EventMessage
}

func (b *recordingRoomBroadcaster) Broadcast(_ context.Context, _ string, event protocol.EventMessage) []error {
	b.events = append(b.events, event)
	return nil
}

type recordingPermissionSender struct {
	events []protocol.EventMessage
}

func (s *recordingPermissionSender) Key() string {
	return "permission-sender"
}

func (s *recordingPermissionSender) IsClosed() bool {
	return false
}

func (s *recordingPermissionSender) SendEvent(_ context.Context, event protocol.EventMessage) error {
	s.events = append(s.events, event)
	return nil
}

func TestBroadcastSharedEventMirrorsToRoundObserverWhenBroadcasterIsConfigured(t *testing.T) {
	service := &Service{
		permission: permissionctx.NewContext(),
		rounds:     newRoomRoundRegistry(),
	}
	broadcaster := &recordingRoomBroadcaster{}
	service.SetRoomBroadcaster(broadcaster)

	sessionKey := protocol.BuildRoomSharedSessionKey("conversation-1")
	observed := make(chan protocol.EventMessage, 1)
	service.registerRound(&activeRoomRound{
		SessionKey:     sessionKey,
		RoomID:         "room-1",
		ConversationID: "conversation-1",
		RoundID:        "round-1",
		EventObserver: func(_ context.Context, event protocol.EventMessage) {
			observed <- event
		},
		Slots: map[string]*activeRoomSlot{},
		Done:  make(chan struct{}),
	})

	event := protocol.NewRoundStatusEvent(sessionKey, "round-1", "finished", "success")
	service.broadcastSharedEvent(context.Background(), sessionKey, "room-1", event)

	if len(broadcaster.events) != 1 {
		t.Fatalf("Room broadcaster 应收到事件，实际 %d", len(broadcaster.events))
	}
	select {
	case mirrored := <-observed:
		if mirrored.EventType != protocol.EventTypeRoundStatus || mirrored.SessionKey != sessionKey {
			t.Fatalf("观察器收到事件不正确: %+v", mirrored)
		}
	default:
		t.Fatal("配置 broadcaster 时也应镜像事件给内部观察器")
	}
}

func TestBroadcastSharedEventDoesNotDuplicateObserverWhenUsingPermissionBroadcast(t *testing.T) {
	permission := permissionctx.NewContext()
	service := &Service{
		permission: permission,
		rounds:     newRoomRoundRegistry(),
	}

	sessionKey := protocol.BuildRoomSharedSessionKey("conversation-1")
	sender := &recordingPermissionSender{}
	permission.BindSession(sessionKey, sender)

	observed := make(chan protocol.EventMessage, 1)
	service.registerRound(&activeRoomRound{
		SessionKey:     sessionKey,
		RoomID:         "room-1",
		ConversationID: "conversation-1",
		RoundID:        "round-1",
		EventObserver: func(_ context.Context, event protocol.EventMessage) {
			observed <- event
		},
		Slots: map[string]*activeRoomSlot{},
		Done:  make(chan struct{}),
	})

	event := protocol.NewRoundStatusEvent(sessionKey, "round-1", "finished", "success")
	service.broadcastSharedEvent(context.Background(), sessionKey, "", event)

	if len(sender.events) != 1 {
		t.Fatalf("permission sender 应收到一次事件，实际 %d", len(sender.events))
	}
	select {
	case mirrored := <-observed:
		t.Fatalf("permission 广播路径不应额外调用内部观察器: %+v", mirrored)
	default:
	}
}

func TestHandleInterruptExactTargetIsIdempotentAfterNaturalCompletion(t *testing.T) {
	service := &Service{rounds: newRoomRoundRegistry()}
	err := service.HandleInterrupt(context.Background(), InterruptRequest{
		SessionKey:   protocol.BuildRoomSharedSessionKey("conversation-stop-race"),
		AgentRoundID: "agent-round-already-finished",
	})
	if err != nil {
		t.Fatalf("自然完成后的精确停止应幂等成功，实际 error = %v", err)
	}
}

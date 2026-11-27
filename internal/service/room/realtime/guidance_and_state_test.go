// guidance_and_state_test.go 覆盖 Room 夹具、引导输入、轮次状态、队列暂停、节点准入、唤醒与沙箱策略测试。
package realtime

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	messagepkg "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// withConstructorDefaults 为直接构造的 Service 字面量补齐 NewService 总会创建的依赖。
// 生产路径不再为这些依赖做 nil 守卫，测试夹具必须满足同一不变量。
func withConstructorDefaults(t *testing.T, s *Service) *Service {
	t.Helper()
	root := s.Config.WorkspacePath
	if root == "" {
		root = t.TempDir()
	}
	if s.Files == nil {
		s.Files = workspacestore.NewSessionFileStore(root)
	}
	if s.History == nil {
		s.History = workspacestore.NewAgentHistoryStore(root)
	}
	if s.roomHistory == nil {
		s.roomHistory = workspacestore.NewRoomHistoryStore(root)
	}
	if s.directedMessages == nil {
		s.directedMessages = workspacestore.NewRoomDirectedMessageStore(root)
	}
	if s.directedWakes == nil {
		s.directedWakes = workspacestore.NewRoomDirectedMessageWakeStore(root)
	}
	if s.publicHandoffs == nil {
		s.publicHandoffs = workspacestore.NewRoomPublicHandoffStore(root)
	}
	if s.InputQueue == nil {
		s.InputQueue = workspacestore.NewInputQueueStore(root)
	}
	if s.Runtime == nil {
		s.Runtime = runtimectx.NewManager()
	}
	return s
}

func newRoomRoundRegistryFromRounds(rounds map[string]*activeRoomRound) roomRoundRegistry {
	registry := &roomRoundRegistry{
		conversations: make(map[string]*roomConversationState),
	}
	for _, roundValue := range rounds {
		registry.register(roundValue)
	}
	return roomRoundRegistry{conversations: registry.conversations}
}

type roomGuidanceRuntimeFactory struct {
	client runtimectx.Client
}

func (f roomGuidanceRuntimeFactory) New(agentclient.Options) runtimectx.Client {
	return f.client
}

func TestRoomSlotGuidanceHookUsesQueueOwnerForAttachment(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", stateRoot)

	const (
		ownerUserID       = "owner-a"
		otherOwnerUserID  = "owner-b"
		conversationID    = "conversation-owner-attachment"
		attachmentName    = "owner-note.txt"
		runtimeSessionKey = "agent:agent-a:ws:group:conversation-owner-attachment"
	)
	paths := workspacestore.New(appfs.UsersRoot())
	workspacePath := filepath.Join(appfs.UserWorkspaceRoot(ownerUserID), "agent-a")
	workspaceRoot, err := paths.OpenOwnerWorkspacePath(ownerUserID, workspacePath, true)
	if err != nil {
		t.Fatalf("创建 owner workspace 失败: %v", err)
	}
	workspaceRoot.Close()
	for _, owner := range []string{ownerUserID, otherOwnerUserID} {
		assetRoot, openErr := paths.OpenRoomConversationAssetRoot(owner, conversationID, true)
		if openErr != nil {
			t.Fatalf("创建 %s Room 附件根失败: %v", owner, openErr)
		}
		if writeErr := assetRoot.WriteFileAtomic(attachmentName, []byte(owner), 0o600); writeErr != nil {
			assetRoot.Close()
			t.Fatalf("写入 %s Room 附件失败: %v", owner, writeErr)
		}
		assetRoot.Close()
	}

	location := workspacestore.InputQueueLocation{
		OwnerUserID:    ownerUserID,
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  workspacePath,
		SessionKey:     runtimeSessionKey,
		RoomID:         "room-owner-attachment",
		ConversationID: conversationID,
	}
	store := workspacestore.NewInputQueueStore(appfs.UsersRoot())
	if _, err = store.Enqueue(location, protocol.InputQueueItem{
		ID:             "room-guide-owner-attachment",
		Content:        "读取附件",
		DeliveryPolicy: protocol.ChatDeliveryPolicyGuide,
		RootRoundID:    "room-round-owner-attachment",
		Attachments: []protocol.ChatAttachment{{
			FileName:       attachmentName,
			WorkspacePath:  attachmentName,
			Scope:          protocol.ChatAttachmentScopeRoomConversation,
			Kind:           protocol.ChatAttachmentKindFile,
			RoomID:         location.RoomID,
			ConversationID: conversationID,
		}},
	}); err != nil {
		t.Fatalf("写入 owner Room 引导队列失败: %v", err)
	}

	service := &Service{
		Host: runtimehost.Host{Config: config.Config{WorkspacePath: appfs.UsersRoot()}, InputQueue: store},
	}
	slot := &activeRoomSlot{
		OwnerUserID:       ownerUserID,
		AgentID:           "agent-a",
		AgentRoundID:      "room-round-owner-attachment",
		RuntimeSessionKey: runtimeSessionKey,
		WorkspacePath:     workspacePath,
	}
	wrongOwnerCtx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: otherOwnerUserID,
		Role:   authctx.RoleOwner,
	})
	output, err := service.roomSlotGuidanceHook(nil, slot, location)(
		wrongOwnerCtx,
		sdkhook.Input{EventName: sdkhook.EventPostToolUse},
		"tool-owner-attachment",
	)
	if err != nil {
		t.Fatalf("执行 owner Room 引导失败: %v", err)
	}
	if output.SpecificOutput == nil {
		t.Fatal("owner Room 引导缺少 additional context")
	}
	expectedPath := filepath.Join(
		paths.RoomConversationAssetDir(ownerUserID, conversationID),
		attachmentName,
	)
	otherPath := filepath.Join(
		paths.RoomConversationAssetDir(otherOwnerUserID, conversationID),
		attachmentName,
	)
	if !strings.Contains(output.SpecificOutput.AdditionalContext, filepath.ToSlash(expectedPath)) ||
		strings.Contains(output.SpecificOutput.AdditionalContext, filepath.ToSlash(otherPath)) {
		t.Fatalf("Room 引导附件未绑定队列 owner: %q", output.SpecificOutput.AdditionalContext)
	}
}

func TestRoomGuidanceRejectsMismatchedOwnerBindings(t *testing.T) {
	execution := roomGuidanceExecution{
		ctx:      context.Background(),
		round:    &activeRoomRound{OwnerUserID: "owner-b"},
		location: workspacestore.InputQueueLocation{OwnerUserID: "owner-a"},
	}
	if err := execution.bindOwnerContext(); err == nil {
		t.Fatal("Room 引导应拒绝 round 与队列位置 owner 不一致")
	}
}

func TestRoomSlotGuidanceTransportFailureKeepsDurableInput(t *testing.T) {
	storeRoot := t.TempDir()
	store := workspacestore.NewInputQueueStore(storeRoot)
	location := workspacestore.InputQueueLocation{
		Scope:         protocol.InputQueueScopeRoom,
		WorkspacePath: storeRoot,
		SessionKey:    protocol.BuildRoomAgentSessionKey("conversation-1", "agent-1", protocol.RoomTypeGroup),
	}
	if _, err := store.Enqueue(location, protocol.InputQueueItem{
		ID:             "room-guide-transport-failure",
		Content:        "不要丢掉这条插话",
		DeliveryPolicy: protocol.ChatDeliveryPolicyGuide,
		RootRoundID:    "agent-round-1",
	}); err != nil {
		t.Fatal(err)
	}
	service := &Service{Host: runtimehost.Host{InputQueue: store}}
	slot := &activeRoomSlot{AgentRoundID: "agent-round-1"}
	hook := service.roomSlotGuidanceHook(nil, slot, location)
	if _, err := hook(context.Background(), sdkhook.Input{EventName: sdkhook.EventPostToolUse}, "tool-1"); err != nil {
		t.Fatal(err)
	}

	// 模拟 callback 返回后 control_response 写失败：slot 结束只清内存 pending，持久队列必须保留。
	service.forgetRoomSlotGuidance(slot)
	items, err := store.Snapshot(location)
	if err != nil || len(items) != 1 || items[0].ID != "room-guide-transport-failure" {
		t.Fatalf("transport 失败后 Room 引导必须可重放: items=%+v err=%v", items, err)
	}
}

func TestGuideActiveAgentSlotsBatchIsAllOrNoneAndIdempotent(t *testing.T) {
	root := t.TempDir()
	stateRoot := filepath.Join(root, ".nexus")
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	storeRoot := filepath.Join(stateRoot, "users")
	ownerWorkspaceRoot := appfs.UserWorkspaceRootAt(stateRoot, "owner")
	sharedSessionKey := protocol.BuildRoomSharedSessionKey("conversation-batch")
	validSlot := withRoomSlotStatus(&activeRoomSlot{
		AgentID:           "agent-a",
		AgentRoundID:      "agent-round-a",
		RuntimeSessionKey: protocol.BuildRoomAgentSessionKey("conversation-batch", "agent-a", protocol.RoomTypeGroup),
		WorkspacePath:     filepath.Join(ownerWorkspaceRoot, "agent-a"),
	}, "running")
	invalidSlot := withRoomSlotStatus(&activeRoomSlot{
		AgentID:           "agent-b",
		AgentRoundID:      "agent-round-b",
		RuntimeSessionKey: protocol.BuildRoomAgentSessionKey("conversation-batch", "agent-b", protocol.RoomTypeGroup),
	}, "running")
	store := workspacestore.NewInputQueueStore(storeRoot)
	service := &Service{
		Host: runtimehost.Host{InputQueue: store},
		rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{
			"active": {
				SessionKey:     sharedSessionKey,
				ConversationID: "conversation-batch",
				Slots: map[string]*activeRoomSlot{
					validSlot.AgentID:   validSlot,
					invalidSlot.AgentID: invalidSlot,
				},
			},
		}),
	}
	guided, err := service.guideActiveAgentSlots(
		context.Background(), sharedSessionKey, "room-batch", "conversation-batch",
		[]string{validSlot.AgentID, invalidSlot.AgentID}, protocol.InputQueueItem{
			ID: "room-guide-batch", SourceMessageID: "msg-room-guide-batch", Source: protocol.InputQueueSourceUser,
			Content: "同时插话", OwnerUserID: "owner",
		},
	)
	if err == nil || len(guided) != 0 {
		t.Fatalf("任一目标位置无效时批量登记必须整体失败: guided=%+v err=%v", guided, err)
	}
	validLocation := workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  validSlot.WorkspacePath,
		SessionKey:     validSlot.RuntimeSessionKey,
		RoomID:         "room-batch",
		ConversationID: "conversation-batch",
	}
	items, snapshotErr := store.Snapshot(validLocation)
	if snapshotErr != nil || len(items) != 0 {
		t.Fatalf("批量失败不应留下部分 Room guide: items=%+v err=%v", items, snapshotErr)
	}

	invalidSlot.WorkspacePath = filepath.Join(ownerWorkspaceRoot, "agent-b")
	for attempt := 0; attempt < 2; attempt++ {
		guided, err = service.guideActiveAgentSlots(
			context.Background(), sharedSessionKey, "room-batch", "conversation-batch",
			[]string{validSlot.AgentID, invalidSlot.AgentID}, protocol.InputQueueItem{
				ID: "room-guide-batch", SourceMessageID: "msg-room-guide-batch", Source: protocol.InputQueueSourceUser,
				Content: "同时插话", OwnerUserID: "owner",
			},
		)
		if err != nil || len(guided) != 2 {
			t.Fatalf("批量 Room guide 登记失败: guided=%+v err=%v", guided, err)
		}
	}
	for _, slot := range []*activeRoomSlot{validSlot, invalidSlot} {
		location := workspacestore.InputQueueLocation{
			Scope:          protocol.InputQueueScopeRoom,
			WorkspacePath:  slot.WorkspacePath,
			SessionKey:     slot.RuntimeSessionKey,
			RoomID:         "room-batch",
			ConversationID: "conversation-batch",
		}
		items, err = store.Snapshot(location)
		if err != nil || len(items) != 1 || items[0].RootRoundID != slot.AgentRoundID {
			t.Fatalf("重试不应重复且必须精确绑定目标 slot: slot=%s items=%+v err=%v", slot.AgentID, items, err)
		}
	}
}

func TestConsumedRoomGuidanceMovesUserMessageIntoReplyRound(t *testing.T) {
	storeRoot := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", storeRoot)
	store := workspacestore.NewInputQueueStore(storeRoot)
	roomHistory := workspacestore.NewRoomHistoryStore(storeRoot)
	conversationID := "conversation-guidance-order"
	agentID := "agent-1"
	agentWorkspace := filepath.Join(appfs.UserWorkspaceRoot("owner"), agentID)
	location := workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  storeRoot,
		SessionKey:     protocol.BuildRoomAgentSessionKey(conversationID, agentID, protocol.RoomTypeGroup),
		RoomID:         "room-1",
		ConversationID: conversationID,
	}
	if err := roomHistory.AppendInlineMessage("owner", conversationID, protocol.Message{
		"message_id":      "guided-user-message",
		"session_key":     protocol.BuildRoomSharedSessionKey(conversationID),
		"room_id":         "room-1",
		"conversation_id": conversationID,
		"round_id":        "guidance-source-round",
		"role":            "user",
		"content":         "然后评价一下",
		"delivery_policy": string(protocol.ChatDeliveryPolicyGuide),
		"timestamp":       int64(200),
	}); err != nil {
		t.Fatalf("写入 Room 引导用户消息失败: %v", err)
	}
	if _, err := store.Enqueue(location, protocol.InputQueueItem{
		ID:              "guidance-source-round",
		AgentID:         agentID,
		SourceMessageID: "guided-user-message",
		Source:          protocol.InputQueueSourceUser,
		Content:         "然后评价一下",
		DeliveryPolicy:  protocol.ChatDeliveryPolicyGuide,
		RootRoundID:     "agent-reply-round",
	}); err != nil {
		t.Fatalf("写入待确认 Room 引导失败: %v", err)
	}

	service := withConstructorDefaults(t, &Service{
		Host:        runtimehost.Host{Permission: permissionctx.NewContext(), InputQueue: store, History: workspacestore.NewAgentHistoryStore(appfs.UsersRoot())},
		roomHistory: roomHistory,
	})
	contextValue := &protocol.ConversationContextAggregate{
		Room:         protocol.RoomRecord{ID: "room-1", OwnerUserID: "owner", RoomType: protocol.RoomTypeGroup},
		Conversation: protocol.ConversationRecord{ID: conversationID, RoomID: "room-1"},
		Members: []protocol.MemberRecord{{
			RoomID: "room-1", MemberType: protocol.MemberTypeAgent, MemberAgentID: agentID,
		}},
		MemberAgents: []protocol.Agent{{AgentID: agentID, Name: "Amy", WorkspacePath: agentWorkspace}},
	}
	roundValue := &activeRoomRound{
		SessionKey:     protocol.BuildRoomSharedSessionKey(conversationID),
		RoomID:         "room-1",
		ConversationID: conversationID,
		RootRoundID:    "goal-reply-round",
		OwnerUserID:    "owner",
		Context:        contextValue,
	}
	slot := &activeRoomSlot{AgentID: agentID, AgentRoundID: "agent-reply-round", RuntimeSessionKey: location.SessionKey, WorkspacePath: agentWorkspace}
	hook := service.roomSlotGuidanceHook(roundValue, slot, location)
	if _, err := hook(context.Background(), sdkhook.Input{EventName: sdkhook.EventPostToolUse}, "tool-1"); err != nil {
		t.Fatalf("准备 Room 引导失败: %v", err)
	}
	readGuidedMessage := func() protocol.Message {
		messages, err := roomHistory.ReadMessages("owner", conversationID, nil)
		if err != nil {
			t.Fatalf("读取 Room 引导历史失败: %v", err)
		}
		for _, candidate := range messages {
			if candidate["message_id"] == "guided-user-message" {
				return candidate
			}
		}
		return nil
	}
	message := readGuidedMessage()
	if message == nil {
		t.Fatal("Room 历史缺少引导用户消息")
	}
	if protocol.MessageRoundID(message) != "guidance-source-round" || message["source_round_id"] != nil {
		t.Fatalf("control response 未确认前不应移动用户消息: %+v", message)
	}
	if _, err := hook(context.Background(), sdkhook.Input{EventName: sdkhook.EventPostToolUse}, "tool-2"); err != nil {
		t.Fatalf("确认 Room 引导失败: %v", err)
	}
	message = readGuidedMessage()
	if protocol.MessageRoundID(message) != "goal-reply-round" ||
		message["source_round_id"] != "guidance-source-round" ||
		message["agent_round_id"] != "agent-reply-round" {
		t.Fatalf("已消费引导未归入模型回复 round: %+v", message)
	}
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
	slot.mutable.goal.MarkSubagentUsagePending(taskID, goalsvc.SubagentUsageObservation{CumulativeTotal: cumulativeTotal})
}

// clearSubagentUsagePending 只确认不晚于 settledTotal 的 pending。旧请求成功返回时，
// 若同 task 已到达更大的累计值，则必须保留新值给 retry worker 重放。
func (slot *activeRoomSlot) clearSubagentUsagePending(taskID string, settledTotal int64) {
	slot.mutable.goal.ClearSubagentUsagePending(taskID, goalsvc.SubagentUsageObservation{
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

func TestSharedInputHonorsStoppedNonPrimaryTarget(t *testing.T) {
	entries := []roomInputQueueEntry{
		{Item: protocol.InputQueueItem{AgentID: "primary", TargetAgentIDs: []string{"primary", "stopped"}}},
		{Item: protocol.InputQueueItem{AgentID: "other", TargetAgentIDs: []string{"other"}}},
	}
	projectRoomQueuePause(entries, map[string]bool{"stopped": true})
	if !entries[0].Item.DispatchPaused || entries[1].Item.DispatchPaused {
		t.Fatalf("wrong pause projection: %+v", entries)
	}
	service := &Service{}
	if service.canDispatchInputQueueItem("session", "conversation", nil, entries[0].Item) {
		t.Fatal("shared item bypassed stopped member")
	}
}

func TestLeasedChatNeverFallsBackToUserQueue(t *testing.T) {
	service := &Service{rounds: newRoomRoundRegistry()}
	contextValue := &protocol.ConversationContextAggregate{Members: []protocol.MemberRecord{{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent", ParticipationPaused: true}}}
	execution := roomChatExecution{service: service, request: ChatRequest{requireImmediateStart: true}, contextValue: contextValue, sessionKey: "room:conversation", conversationID: "conversation", targetAgentIDs: []string{"agent"}}
	// 未装配任何用户队列：错误地退入普通路径将失败或触发 panic。
	if handled, err := execution.routeActiveSlots(); !handled || err == nil {
		t.Fatalf("paused agent queued: %v %v", handled, err)
	}
	contextValue.Members[0].ParticipationPaused = false
	if handled, err := execution.routeActiveSlots(); handled || err != nil {
		t.Fatalf("idle agent rejected: %v %v", handled, err)
	}
	slot := &activeRoomSlot{AgentID: "agent", AgentRoundID: "agent-round"}
	slot.setStatus("running")
	service.rounds.register(&activeRoomRound{SessionKey: "room:conversation", ConversationID: "conversation", RoundID: "busy", Slots: map[string]*activeRoomSlot{"agent": slot}})
	if handled, err := execution.routeActiveSlots(); !handled || err == nil {
		t.Fatalf("busy agent queued: %v %v", handled, err)
	}
}

func roomWakeTrigger() messagepkg.RuntimeWakeTrigger {
	return messagepkg.RuntimeWakeTrigger{TaskID: "b1", Status: "completed", Summary: "build done"}
}

// 私域 slot 的自唤醒：沿用私域路由，触发行与输出都不进入公区。
func TestRoomRuntimeWakePrivateSlotStaysPrivate(t *testing.T) {
	route := protocol.RoomReplyRoute{Mode: protocol.RoomReplyRoutePrivate, Recipients: []string{"agent-b"}}
	wake := &roomRuntimeWakeStart{trigger: roomWakeTrigger(), replyRoute: route}
	request := ChatRequest{Internal: true, runtimeWake: wake}
	row := decorateRoomRuntimeWakeUserMessage(request, protocol.Message{"content": "x"})
	if row["hidden_from_user"] != true {
		t.Fatalf("private wake row must stay hidden: %+v", row)
	}
	if shouldBroadcastRoomChatAck(request) {
		t.Fatal("private wake must not broadcast ack")
	}
	slot := &activeRoomSlot{}
	applyRuntimeWakeSlot(request, slot)
	if !slot.HiddenFromUser || roomSlotPublishesPublicOutput(slot) {
		t.Fatalf("slot must keep private route: %+v", slot.replyRoute())
	}
}

// drain 观察到的通知只用于下一次唤醒，取出后清空。
func TestRoomIdleWakeObserverTakesLatestTopLevelNotification(t *testing.T) {
	parent := "toolu_parent"
	observer := &roomIdleWakeObserver{}
	observer.observe(sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeSystem, TaskNotification: &sdkprotocol.TaskNotificationMessage{TaskID: "b1", Summary: "first"}})
	observer.observe(sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeSystem, ParentToolUseID: &parent, TaskNotification: &sdkprotocol.TaskNotificationMessage{TaskID: "child"}})
	if got := observer.take(); got.TaskID != "b1" {
		t.Fatalf("take = %+v", got)
	}
	if got := observer.take(); got.TaskID != "" {
		t.Fatalf("second take must be empty: %+v", got)
	}
}

type sandboxTransitionClient struct {
	permissionModeTestClient
	closed, retired, queries int
	closeErr                 error
}

func (c *sandboxTransitionClient) Retire()                             { c.retired++ }
func (c *sandboxTransitionClient) Disconnect(context.Context) error    { c.closed++; return c.closeErr }
func (c *sandboxTransitionClient) Query(context.Context, string) error { c.queries++; return nil }

func TestRoomSandboxTransitionCancelsApprovalAndClosesWithoutReplay(t *testing.T) {
	permissions := permissionctx.NewContext()
	key := protocol.BuildRoomAgentSessionKey("sandbox-room", "agent-a", protocol.RoomTypeGroup)
	sender := &roomInterruptPermissionSender{key: "sandbox-transition", events: make(chan protocol.EventMessage, 8)}
	permissions.BindSession(key, sender)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	decision := make(chan sdkpermission.Decision, 1)
	go func() {
		result, _ := permissions.RequestPermission(ctx, key, sdkpermission.Request{ToolName: "Bash", Input: map[string]any{"command": "echo pending"}})
		decision <- result
	}()
	select {
	case <-sender.events:
	case <-time.After(time.Second):
		t.Fatal("permission request not presented")
	}
	client := &sandboxTransitionClient{permissionModeTestClient: permissionModeTestClient{modeErr: runtimectx.ErrDesktopSandboxPolicyChanged}}
	other := &sandboxTransitionClient{}
	slot := &activeRoomSlot{AgentID: "agent-a", RuntimeSessionKey: key}
	slot.setClient(client)
	slot.setStatus("running")
	sibling := &activeRoomSlot{AgentID: "agent-b"}
	sibling.setClient(other)
	sibling.setStatus("running")
	service := &Service{Host: runtimehost.Host{Permission: permissions}, rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{"r": {Slots: map[string]*activeRoomSlot{"a": slot, "b": sibling}}})}
	if err := service.SetPermissionModeForAgent(ctx, "agent-a", sdkpermission.ModeBypassPermissions); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-decision:
		if got.Behavior != sdkpermission.BehaviorDeny || !got.Interrupt {
			t.Fatalf("old approval survived transition: %#v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("old approval was left pending")
	}
	if client.closed != 1 || client.retired != 1 || client.queries != 0 || !slot.isTerminal() {
		t.Fatalf("transition lifecycle incorrect: %+v", client)
	}
	if other.closed != 0 || other.retired != 0 || sibling.isTerminal() {
		t.Fatal("transition affected sibling Agent")
	}
}

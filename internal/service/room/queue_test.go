package room_test

import (
	"context"
	"strings"
	"testing"
	"time"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	_ "modernc.org/sqlite"
)

func TestRealtimeServiceDispatchesRoomUserQueueForIdleTargetWhileAnotherAgentRuns(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	amy := createTestAgent(t, agentService, ctx, "Amy")
	devin := createTestAgent(t, agentService, ctx, "Devin")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{amy.AgentID, devin.AgentID},
		Name:     "用户队列按目标派发房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	amyClient := newFakeRoomClient()
	amyClient.onQuery = func(_ context.Context, _ string) error {
		return nil
	}
	devinClient := newFakeRoomClient()
	devinPrompt := make(chan string, 1)
	devinClient.onQuery = func(_ context.Context, prompt string) error {
		devinPrompt <- prompt
		go sendFakeAssistantResult(devinClient, "devin-user-queue-idle", "收到。")
		return nil
	}

	permission := permissionctx.NewContext()
	service := NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{amyClient, devinClient}},
	)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-user-queue-idle-target")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@Amy 先处理一个长任务",
		RoundID:        "room-round-amy-busy",
	}); err != nil {
		t.Fatalf("启动 Amy 长任务失败: %v", err)
	}
	_ = collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeStreamStart && event.AgentID == amy.AgentID
	})

	queueResult, err := service.HandleInputQueue(ctx, roomsvc.InputQueueRequest{
		SessionKey:      sharedSessionKey,
		RoomID:          roomContext.Room.ID,
		ConversationID:  roomContext.Conversation.ID,
		ClientMessageID: "client-message-room-user-queue-idle-target",
		Action:          "enqueue",
		Content:         "@Devin 好的",
		DeliveryPolicy:  protocol.ChatDeliveryPolicyQueue,
	})
	if err != nil {
		t.Fatalf("写入 Room 用户队列失败: %v", err)
	}
	if queueResult.ItemID == "" || queueResult.Duplicate {
		t.Fatalf("首次 Room 用户队列受理结果异常: %+v", queueResult)
	}

	select {
	case prompt := <-devinPrompt:
		if !strings.Contains(prompt, "@Devin 好的") {
			t.Fatalf("Devin prompt 缺少队列触发内容: %s", prompt)
		}
	case <-time.After(time.Second):
		t.Fatal("Amy 运行时，空闲 Devin 的用户队列项未被派发")
	}

	if service.CountRunningTasks(amy.AgentID) == 0 {
		t.Fatal("测试前提失效：Amy 应仍处于运行状态")
	}
	targetQueueLocation := workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  devin.WorkspacePath,
		SessionKey:     protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, devin.AgentID, roomContext.Room.RoomType),
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
	}
	targetQueueItems, err := workspacestore.NewInputQueueStore(cfg.WorkspacePath).Snapshot(targetQueueLocation)
	if err != nil {
		t.Fatalf("读取 Devin 队列失败: %v", err)
	}
	if len(targetQueueItems) != 0 {
		t.Fatalf("空闲目标派发后不应残留队列项: %+v", targetQueueItems)
	}
	_ = collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		if event.EventType != protocol.EventTypeRoundStatus {
			return false
		}
		roundID, _ := event.Data["round_id"].(string)
		status, _ := event.Data["status"].(string)
		return strings.HasPrefix(roundID, "queue_") && status == "finished"
	})

	retryResult, err := service.HandleInputQueue(ctx, roomsvc.InputQueueRequest{
		SessionKey:      sharedSessionKey,
		RoomID:          roomContext.Room.ID,
		ConversationID:  roomContext.Conversation.ID,
		ClientMessageID: "client-message-room-user-queue-idle-target",
		Action:          "enqueue",
		Content:         "@Devin 好的",
		DeliveryPolicy:  protocol.ChatDeliveryPolicyQueue,
	})
	if err != nil {
		t.Fatalf("重试已即时派发的 Room 用户队列失败: %v", err)
	}
	if !retryResult.Duplicate || retryResult.ItemID != queueResult.ItemID {
		t.Fatalf("重试应确认原始 Room 队列受理结果: first=%+v retry=%+v", queueResult, retryResult)
	}
	select {
	case prompt := <-devinPrompt:
		t.Fatalf("相同 client_message_id 重试不应触发第二轮: %q", prompt)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestRealtimeServiceRecoversOrphanedGuidanceOnQueueSnapshot(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	agentValue := createTestAgent(t, agentService, ctx, "恢复助手")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID},
		Name:     "引导恢复房间",
	})
	if err != nil {
		t.Fatal(err)
	}

	queuedPrompt := make(chan string, 1)
	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, prompt string) error {
		queuedPrompt <- prompt
		go sendFakeAssistantResult(client, "assistant-recovered-guide", "已继续处理")
		return nil
	}
	permission := permissionctx.NewContext()
	service := NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{client}},
	)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-orphaned-guide")
	permission.BindSession(sharedSessionKey, sender)

	location := workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  agentValue.WorkspacePath,
		SessionKey:     protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, agentValue.AgentID, roomContext.Room.RoomType),
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
	}
	queueStore := workspacestore.NewInputQueueStore(cfg.WorkspacePath)
	if _, err = queueStore.Enqueue(location, protocol.InputQueueItem{
		ID:             "orphaned-guide",
		Scope:          protocol.InputQueueScopeRoom,
		SessionKey:     location.SessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		AgentID:        agentValue.AgentID,
		TargetAgentIDs: []string{agentValue.AgentID},
		Source:         protocol.InputQueueSourceUser,
		Content:        "重启后继续处理这条补充要求",
		DeliveryPolicy: protocol.ChatDeliveryPolicyGuide,
		RootRoundID:    "agent-round-before-restart",
	}); err != nil {
		t.Fatal(err)
	}

	event, err := service.InputQueueSnapshotEvent(ctx, roomContext.Room.ID, roomContext.Conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	items, ok := event.Data["items"].([]protocol.InputQueueItem)
	if !ok || len(items) != 1 {
		t.Fatalf("恢复快照缺少 orphan guide: %+v", event.Data["items"])
	}
	if items[0].DeliveryPolicy != protocol.ChatDeliveryPolicyQueue || items[0].RootRoundID != "" {
		t.Fatalf("失去目标 slot 的 guide 必须恢复为普通 queue: %+v", items[0])
	}

	select {
	case prompt := <-queuedPrompt:
		if !strings.Contains(prompt, "重启后继续处理这条补充要求") {
			t.Fatalf("恢复后的接力 prompt 不正确: %s", prompt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("后端重启后 orphan guide 未自动接力")
	}
	_ = collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		roundID, _ := event.Data["round_id"].(string)
		return event.EventType == protocol.EventTypeRoundStatus &&
			strings.HasPrefix(roundID, "queue_orphaned-guide") && event.Data["status"] == "finished"
	})
}

func TestRealtimeServiceDispatchesLateRoomGuidanceAfterRoundFinishes(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	agentValue := createTestAgent(t, agentService, context.Background(), "助手甲")
	roomContext, err := roomService.CreateRoom(context.Background(), protocol.CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID},
		Name:     "末尾引导接力房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	firstClient := newFakeRoomClient()
	secondPrompt := make(chan string, 1)
	queryCount := 0
	firstClient.onQuery = func(_ context.Context, prompt string) error {
		queryCount++
		if queryCount == 1 {
			return nil
		}
		secondPrompt <- prompt
		go sendFakeAssistantResult(firstClient, "assistant-late-guide", "已处理补充要求")
		return nil
	}

	permission := permissionctx.NewContext()
	service := NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{firstClient}},
	)
	ctx := context.Background()
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-late-guide")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@助手甲 先完成当前任务",
		RoundID:        "room-round-late-guide-1",
	}); err != nil {
		t.Fatalf("启动第一轮失败: %v", err)
	}
	startEvents := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeStreamStart && event.AgentID == agentValue.AgentID
	})
	activeAgentRoundID := ""
	for _, event := range startEvents {
		if event.EventType == protocol.EventTypeStreamStart && event.AgentID == agentValue.AgentID {
			activeAgentRoundID = event.AgentRoundID
		}
	}
	if activeAgentRoundID == "" {
		t.Fatalf("缺少活跃 Agent round id: %+v", startEvents)
	}

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@助手甲 然后想想还能怎么优化",
		RoundID:        "room-round-late-guide-2",
	}); err != nil {
		t.Fatalf("写入补充消息失败: %v", err)
	}
	_ = collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeChatAck && event.Data["round_id"] == "room-round-late-guide-2"
	})

	queueLocation := workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  agentValue.WorkspacePath,
		SessionKey:     protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, agentValue.AgentID, roomContext.Room.RoomType),
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
	}
	queueStore := workspacestore.NewInputQueueStore(cfg.WorkspacePath)
	items, err := queueStore.Snapshot(queueLocation)
	if err != nil || len(items) != 1 {
		t.Fatalf("读取补充消息队列失败: items=%+v err=%v", items, err)
	}
	if _, err = service.HandleInputQueue(ctx, roomsvc.InputQueueRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Action:         "guide",
		ItemID:         items[0].ID,
	}); err != nil {
		t.Fatalf("将补充消息切换为引导失败: %v", err)
	}
	items, err = queueStore.Snapshot(queueLocation)
	if err != nil || len(items) != 1 || items[0].RootRoundID != activeAgentRoundID {
		t.Fatalf("队列 guide 必须绑定切换时的精确 Agent round: items=%+v active=%q err=%v", items, activeAgentRoundID, err)
	}

	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	messages, err := roomHistory.ReadMessages(roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取 Room 历史失败: %v", err)
	}
	for _, message := range messages {
		if protocol.MessageRoundID(message) == "room-round-late-guide-2" || message["source_round_id"] == "room-round-late-guide-2" {
			t.Fatalf("未消费的 Room guide 不应进入公区历史: %+v", message)
		}
	}

	// 模拟当前 round 已无后续 PostToolUse：引导没有机会注入，直接收到最终 result。
	go sendFakeAssistantResult(firstClient, "assistant-first-round", "第一轮完成")
	select {
	case prompt := <-secondPrompt:
		if !strings.Contains(prompt, "然后想想还能怎么优化") {
			t.Fatalf("恢复后的下一轮缺少补充要求: %s", prompt)
		}
	case <-time.After(time.Second):
		t.Fatal("错过最后一次工具钩子的引导未在 round 结束后接力派发")
	}

	items, err = queueStore.Snapshot(queueLocation)
	if err != nil {
		t.Fatalf("读取派发后队列失败: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("补充消息接力派发后不应残留队列项: %+v", items)
	}
	messages, err = roomHistory.ReadMessages(roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("重新读取 Room 历史失败: %v", err)
	}
	assertRoomUserMessageDeliveryPolicy(t, messages, "room-round-late-guide-2", protocol.ChatDeliveryPolicyQueue)
}

func assertRoomUserMessageDeliveryPolicy(
	t *testing.T,
	messages []protocol.Message,
	roundID string,
	want protocol.ChatDeliveryPolicy,
) {
	t.Helper()
	for _, message := range messages {
		if protocol.MessageRole(message) != "user" || protocol.MessageRoundID(message) != roundID {
			continue
		}
		if got, _ := message["delivery_policy"].(string); got != string(want) {
			t.Fatalf("Room 用户消息 delivery_policy=%q, want=%q: %+v", got, want, message)
		}
		return
	}
	t.Fatalf("Room 历史缺少 round %q 的用户消息: %+v", roundID, messages)
}

func TestRealtimeServiceNewMessageKeepsOtherAgentRoundRunning(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	agentA := createTestAgent(t, agentService, ctx, "助手甲")
	agentB := createTestAgent(t, agentService, ctx, "助手乙")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentA.AgentID, agentB.AgentID},
		Name:     "并行测试房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	clientA := newFakeRoomClient()
	clientA.onQuery = func(_ context.Context, _ string) error {
		return nil
	}
	clientB := newFakeRoomClient()
	clientB.onQuery = func(_ context.Context, _ string) error {
		go sendFakeAssistantResult(clientB, "assistant-b", "助手乙完成")
		return nil
	}

	permission := permissionctx.NewContext()
	service := NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{clientA, clientB}},
	)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-parallel-agents")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@助手甲 先处理",
		RoundID:        "room-round-agent-a",
	}); err != nil {
		t.Fatalf("HandleChat A 失败: %v", err)
	}
	_ = collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeStreamStart && event.AgentID == agentA.AgentID
	})

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@助手乙 你也处理",
		RoundID:        "room-round-agent-b",
	}); err != nil {
		t.Fatalf("HandleChat B 失败: %v", err)
	}

	clientA.mu.Lock()
	interruptA := clientA.interruptCalls
	clientA.mu.Unlock()
	if interruptA != 0 {
		t.Fatalf("发给助手乙的新消息不应中断助手甲: interruptA=%d", interruptA)
	}

	events := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus &&
			event.Data["round_id"] == "room-round-agent-b" &&
			event.Data["status"] == "finished"
	})
	if countRoomResultSubtype(events, "success") == 0 {
		t.Fatalf("助手乙 round 应正常完成: %+v", events)
	}

	if err = service.HandleInterrupt(ctx, roomsvc.InterruptRequest{SessionKey: sharedSessionKey}); err != nil {
		t.Fatalf("清理活跃 Room round 失败: %v", err)
	}
}

func TestRealtimeServiceAppendsRunningTargetByDefault(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	agentValue := createTestAgent(t, agentService, ctx, "助手甲")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID},
		Name:     "排队测试房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, _ string) error {
		return nil
	}
	client.onInterrupt = func(_ context.Context) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-room-queue-cleanup",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "interrupted",
					DurationMS: 1,
					NumTurns:   1,
				},
			}
		}()
	}

	permission := permissionctx.NewContext()
	service := NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{client}},
	)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-queue-running")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@助手甲 先处理",
		RoundID:        "room-round-queue-1",
	}); err != nil {
		t.Fatalf("第一条 Room 消息失败: %v", err)
	}
	_ = collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeStreamStart && event.AgentID == agentValue.AgentID
	})

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@助手甲 这是补充要求",
		RoundID:        "room-round-queue-2",
		UserMessageID:  "msg-room-queue-2",
	}); err != nil {
		t.Fatalf("第二条 Room 排队消息失败: %v", err)
	}
	_ = collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeChatAck && event.Data["round_id"] == "room-round-queue-2"
	})

	client.mu.Lock()
	interruptCalls := client.interruptCalls
	sentContents := append([]string(nil), client.sentContents...)
	client.mu.Unlock()
	if interruptCalls != 0 {
		t.Fatalf("默认排队不应中断同一个 Room agent: interruptCalls=%d", interruptCalls)
	}
	if len(sentContents) != 0 {
		t.Fatalf("默认排队不应走运行中 streaming input: %+v", sentContents)
	}
	targetQueueLocation := workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  agentValue.WorkspacePath,
		SessionKey:     protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, agentValue.AgentID, roomContext.Room.RoomType),
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
	}
	targetQueueItems, err := workspacestore.NewInputQueueStore(cfg.WorkspacePath).Snapshot(targetQueueLocation)
	if err != nil {
		t.Fatalf("读取目标 agent session 队列失败: %v", err)
	}
	if len(targetQueueItems) != 1 ||
		targetQueueItems[0].AgentID != agentValue.AgentID ||
		targetQueueItems[0].SourceMessageID != "msg-room-queue-2" ||
		targetQueueItems[0].Content != "@助手甲 这是补充要求" {
		t.Fatalf("Room 运行中公区消息未写入目标 agent 队列: %+v", targetQueueItems)
	}
	queuedHistory, err := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath).ReadMessages(roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取排队中的 Room 公区历史失败: %v", err)
	}
	for _, messageValue := range queuedHistory {
		if messageValue["message_id"] == "msg-room-queue-2" {
			t.Fatalf("尚未派发的 Room queue 不应提前进入公区历史: %+v", messageValue)
		}
	}
	if _, err = workspacestore.NewInputQueueStore(cfg.WorkspacePath).Dispatch(targetQueueLocation, targetQueueItems[0].ID); err != nil {
		t.Fatalf("清理测试队列项失败: %v", err)
	}

	if err = service.HandleInterrupt(ctx, roomsvc.InterruptRequest{SessionKey: sharedSessionKey}); err != nil {
		t.Fatalf("清理活跃 Room round 失败: %v", err)
	}
}

func TestRealtimeServiceGuidesRunningRoomSlotAsLiveSystemContext(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	agentValue := createTestAgent(t, agentService, ctx, "助手甲")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID},
		Name:     "引导测试房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, _ string) error {
		return nil
	}
	client.onInterrupt = func(_ context.Context) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-room-guide-cleanup",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "interrupted",
					DurationMS: 1,
					NumTurns:   1,
				},
			}
		}()
	}

	permission := permissionctx.NewContext()
	factory := &fakeRoomFactory{clients: []*fakeRoomClient{client}}
	service := NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permission,
		factory,
	)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-guide-running")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@助手甲 先处理",
		RoundID:        "room-round-guide-1",
	}); err != nil {
		t.Fatalf("第一条 Room 消息失败: %v", err)
	}
	_ = collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeStreamStart && event.AgentID == agentValue.AgentID
	})

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@助手甲 等工具结果回来后优先看错误日志",
		RoundID:        "room-round-guide-2",
		UserMessageID:  "msg-room-guide-2",
		DeliveryPolicy: protocol.ChatDeliveryPolicyGuide,
	}); err != nil {
		t.Fatalf("Room 引导消息失败: %v", err)
	}
	guidanceEvents := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeChatAck && event.Data["round_id"] == "room-round-guide-2"
	})
	foundGuideAck := false
	for _, event := range guidanceEvents {
		if event.EventType == protocol.EventTypeMessage && event.Data["role"] == "system" {
			t.Fatalf("Room 引导内容不应作为公区消息事件输出: %+v", event)
		}
		if event.EventType != protocol.EventTypeChatAck || event.Data["round_id"] != "room-round-guide-2" {
			continue
		}
		foundGuideAck = true
	}
	if !foundGuideAck {
		t.Fatalf("Room 引导消息缺少 chat_ack: %+v", guidanceEvents)
	}
	targetQueueLocation := workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  agentValue.WorkspacePath,
		SessionKey:     protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, agentValue.AgentID, roomContext.Room.RoomType),
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
	}
	targetQueueStore := workspacestore.NewInputQueueStore(cfg.WorkspacePath)
	queuedGuidance, err := targetQueueStore.Snapshot(targetQueueLocation)
	if err != nil {
		t.Fatalf("读取待注入 Room 引导队列失败: %v", err)
	}
	if len(queuedGuidance) != 1 ||
		queuedGuidance[0].ID != "room-round-guide-2" ||
		queuedGuidance[0].SourceMessageID != "msg-room-guide-2" ||
		queuedGuidance[0].DeliveryPolicy != protocol.ChatDeliveryPolicyGuide ||
		strings.TrimSpace(queuedGuidance[0].RootRoundID) == "" ||
		len(queuedGuidance[0].TargetAgentIDs) != 1 ||
		queuedGuidance[0].TargetAgentIDs[0] != agentValue.AgentID {
		t.Fatalf("Room 引导应在 hook 前持久化并锚定当前 Agent round: %+v", queuedGuidance)
	}
	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	assertGuidanceHidden := func(stage string) {
		messages, historyErr := roomHistory.ReadMessages(roomContext.Conversation.ID, nil)
		if historyErr != nil {
			t.Fatalf("%s 读取 Room 公区历史失败: %v", stage, historyErr)
		}
		for _, messageValue := range messages {
			if messageValue["message_id"] == "msg-room-guide-2" {
				t.Fatalf("%s 未确认消费的 Room guide 不应进入公区历史: %+v", stage, messageValue)
			}
		}
	}
	assertGuidanceHidden("hook 前")

	client.mu.Lock()
	sentContents := append([]string(nil), client.sentContents...)
	interruptCalls := client.interruptCalls
	client.mu.Unlock()
	if interruptCalls != 0 {
		t.Fatalf("Room 引导不应中断运行中 slot: interruptCalls=%d", interruptCalls)
	}
	if len(sentContents) != 0 {
		t.Fatalf("Room 引导不应走普通 streaming input: %+v", sentContents)
	}
	var additionalContext string
	var onApplied func(sdkhook.AppliedAck)
	for _, matcher := range factory.LastOptions().Hooks.Matchers[sdkhook.EventPostToolUse] {
		for _, hook := range matcher.Hooks {
			output, hookErr := hook(ctx, sdkhook.Input{EventName: sdkhook.EventPostToolUse}, "tool-1")
			if hookErr != nil {
				t.Fatalf("执行 Room PostToolUse 引导 hook 失败: %v", hookErr)
			}
			if output.SpecificOutput != nil {
				additionalContext += output.SpecificOutput.AdditionalContext
			}
			if output.OnApplied != nil {
				onApplied = output.OnApplied
			}
		}
	}
	if !strings.Contains(additionalContext, "等工具结果回来后优先看错误日志") {
		t.Fatalf("Room PostToolUse 未注入引导上下文: %q", additionalContext)
	}
	queuedGuidance, err = targetQueueStore.Snapshot(targetQueueLocation)
	if err != nil || len(queuedGuidance) != 1 {
		t.Fatalf("hook control response 尚未确认前应保留 Room 引导: items=%+v err=%v", queuedGuidance, err)
	}
	assertGuidanceHidden("control response 确认前")
	if onApplied == nil {
		t.Fatal("Room guide output 缺少 runtime applied ACK callback")
	}
	if _, err = service.HandleInputQueue(ctx, roomsvc.InputQueueRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Action:         "reorder",
		OrderedIDs:     []string{"room-round-guide-2"},
	}); err == nil {
		t.Fatal("已返回给 runtime 但尚未 applied ACK 的 Room 引导不应允许重排")
	}
	onApplied(sdkhook.AppliedAck{RequestID: "room-guide-applied-1"})
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeAssistant,
		SessionID: client.sessionID,
		Assistant: &sdkprotocol.AssistantMessage{Message: sdkprotocol.ConversationEnvelope{
			ID:      "assistant-room-guide-confirmed",
			Model:   "sonnet",
			Content: []sdkprotocol.ContentBlock{sdkprotocol.TextBlock{Text: "我会优先看错误日志。"}},
		}},
	}
	confirmedEvents := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeMessage && event.Data["role"] == "assistant"
	})
	guidedUserIndex, assistantIndex := -1, -1
	for index, event := range confirmedEvents {
		if event.EventType != protocol.EventTypeMessage {
			continue
		}
		if event.Data["role"] == "user" && event.Data["source_round_id"] == "room-round-guide-2" {
			guidedUserIndex = index
		}
		if event.Data["role"] == "assistant" {
			assistantIndex = index
		}
	}
	if guidedUserIndex < 0 || assistantIndex < 0 || guidedUserIndex > assistantIndex {
		t.Fatalf("已确认 Room 插话必须先于模型 continuation 广播: %+v", confirmedEvents)
	}
	queuedGuidance, err = targetQueueStore.Snapshot(targetQueueLocation)
	if err != nil {
		t.Fatalf("读取已消费 Room 引导队列失败: %v", err)
	}
	if len(queuedGuidance) != 0 {
		t.Fatalf("PostToolUse 注入后应消费 Room 引导队列: %+v", queuedGuidance)
	}

	sharedMessages, err := roomHistory.ReadMessages(roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取 Room 公区历史失败: %v", err)
	}
	foundGuidedPublicMessage := false
	for _, message := range sharedMessages {
		if message["source_round_id"] != "room-round-guide-2" || message["role"] != "user" {
			continue
		}
		if message["role"] != "user" ||
			message["round_id"] != "room-round-guide-1" ||
			message["content"] != "@助手甲 等工具结果回来后优先看错误日志" ||
			message["delivery_policy"] != string(protocol.ChatDeliveryPolicyGuide) {
			t.Fatalf("Room 引导用户消息应归入实际回复 round: %+v", message)
		}
		foundGuidedPublicMessage = true
	}
	if !foundGuidedPublicMessage {
		t.Fatalf("Room 引导用户消息应写入公区历史: %+v", sharedMessages)
	}

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@助手甲 这条写错了",
		RoundID:        "room-round-guide-deleted",
		UserMessageID:  "msg-room-guide-deleted",
		DeliveryPolicy: protocol.ChatDeliveryPolicyGuide,
	}); err != nil {
		t.Fatalf("登记待删除 Room 引导失败: %v", err)
	}
	if _, err = service.HandleInputQueue(ctx, roomsvc.InputQueueRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Action:         "delete",
		ItemID:         "room-round-guide-deleted",
	}); err != nil {
		t.Fatalf("删除未消费 Room 引导失败: %v", err)
	}
	sharedMessages, err = roomHistory.ReadMessages(roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取删除后的 Room 公区历史失败: %v", err)
	}
	for _, messageValue := range sharedMessages {
		if messageValue["message_id"] == "msg-room-guide-deleted" {
			t.Fatalf("已删除且未消费的 Room guide 不应进入公区历史: %+v", messageValue)
		}
	}

	if err = service.HandleInterrupt(ctx, roomsvc.InterruptRequest{SessionKey: sharedSessionKey}); err != nil {
		t.Fatalf("清理活跃 Room round 失败: %v", err)
	}
}

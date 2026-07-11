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

	if err = service.HandleInputQueue(ctx, roomsvc.InputQueueRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Action:         "enqueue",
		Content:        "@Devin 好的",
		DeliveryPolicy: protocol.ChatDeliveryPolicyQueue,
	}); err != nil {
		t.Fatalf("写入 Room 用户队列失败: %v", err)
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
		targetQueueItems[0].SourceMessageID != "room-round-queue-2" ||
		targetQueueItems[0].Content != "@助手甲 这是补充要求" {
		t.Fatalf("Room 运行中公区消息未写入目标 agent 队列: %+v", targetQueueItems)
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
	service := NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{client}},
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

	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	sharedMessages, err := roomHistory.ReadMessages(roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取 Room 公区历史失败: %v", err)
	}
	foundGuidedPublicMessage := false
	for _, message := range sharedMessages {
		if message["round_id"] != "room-round-guide-2" || message["role"] != "user" {
			continue
		}
		if message["role"] != "user" ||
			message["content"] != "@助手甲 等工具结果回来后优先看错误日志" ||
			message["delivery_policy"] != string(protocol.ChatDeliveryPolicyGuide) {
			t.Fatalf("Room 引导用户消息应作为公区事实历史: %+v", message)
		}
		foundGuidedPublicMessage = true
	}
	if !foundGuidedPublicMessage {
		t.Fatalf("Room 引导用户消息应写入公区历史: %+v", sharedMessages)
	}

	if err = service.HandleInterrupt(ctx, roomsvc.InterruptRequest{SessionKey: sharedSessionKey}); err != nil {
		t.Fatalf("清理活跃 Room round 失败: %v", err)
	}
}

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

	_ "modernc.org/sqlite"
)

func TestRealtimeServiceWakesMentionedAgentFromPublicAssistantReply(t *testing.T) {
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
		Name:     "公区 @ 测试房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	amyClient := newFakeRoomClient()
	devinClient := newFakeRoomClient()
	devinPrompt := make(chan string, 1)
	factory := &fakeRoomFactory{clients: []*fakeRoomClient{amyClient, devinClient}}
	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	service := NewRealtimeServiceWithFactory(cfg, roomService, agentService, runtimeManager, permission, factory)

	amyClient.onQuery = func(_ context.Context, _ string) error {
		go sendFakeAssistantResult(amyClient, "amy-public-mention-1", "@Devin 请查询天气，并在公区回复。")
		return nil
	}
	devinClient.onQuery = func(_ context.Context, prompt string) error {
		devinPrompt <- prompt
		go sendFakeAssistantResult(devinClient, "devin-public-mention-1", "天气查询完成。")
		return nil
	}

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-public-mention")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@Amy 让 Devin 查下天气",
		RoundID:        "room-round-public-mention",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	events := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		roundID, _ := event.Data["round_id"].(string)
		return event.EventType == protocol.EventTypeRoundStatus &&
			strings.HasPrefix(roundID, "room_mention_") &&
			event.Data["status"] == "finished"
	})
	select {
	case prompt := <-devinPrompt:
		if !strings.Contains(prompt, "<latest_trigger>\nAmy: @Devin 请查询天气") {
			t.Fatalf("Devin prompt 缺少公区 @ 触发上下文: %s", prompt)
		}
		if strings.Contains(prompt, "type:") || strings.Contains(prompt, "fanout_targets:") {
			t.Fatalf("Devin 动态 prompt 不应包含字段化 trigger: %s", prompt)
		}
		if strings.Contains(prompt, "<room_member_directory>") {
			t.Fatalf("Devin 动态 prompt 不应重复成员目录: %s", prompt)
		}
	case <-time.After(time.Second):
		t.Fatal("Devin 未被公区 @ 唤醒")
	}
	roomSystemPrompt := factory.LastOptions().System.Append
	if !strings.Contains(roomSystemPrompt, "<room_member_directory>") ||
		!strings.Contains(roomSystemPrompt, "agent_id="+devin.AgentID) {
		t.Fatalf("Devin system prompt 应包含 Room 成员目录: %s", roomSystemPrompt)
	}
	if !hasChatAckPendingAgent(events, devin.AgentID) {
		t.Fatalf("事件流缺少 Devin 公区 @ 唤醒 slot: %+v", events)
	}
}

func TestRealtimeServiceAllowsReciprocalPublicMentionChain(t *testing.T) {
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
		Name:     "公区 @ 接力测试房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	amyFirstClient := newFakeRoomClient()
	devinClient := newFakeRoomClient()
	amySecondPrompt := make(chan string, 1)
	factory := &fakeRoomFactory{clients: []*fakeRoomClient{amyFirstClient, devinClient}}
	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	service := NewRealtimeServiceWithFactory(cfg, roomService, agentService, runtimeManager, permission, factory)

	amyQueryCount := 0
	amyFirstClient.onQuery = func(_ context.Context, prompt string) error {
		amyQueryCount++
		if amyQueryCount == 1 {
			go sendFakeAssistantResult(amyFirstClient, "amy-public-mention-chain-1", "@Devin 请接下一联。")
			return nil
		}
		amySecondPrompt <- prompt
		go sendFakeAssistantResult(amyFirstClient, "amy-public-mention-chain-2", "收到，继续接力。")
		return nil
	}
	devinClient.onQuery = func(_ context.Context, _ string) error {
		go sendFakeAssistantResult(devinClient, "devin-public-mention-chain-1", "@Amy 我接完了，你继续。")
		return nil
	}

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-public-mention-chain")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@Amy 你俩接力 5 轮",
		RoundID:        "room-round-public-mention-chain",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	select {
	case prompt := <-amySecondPrompt:
		if !strings.Contains(prompt, "<latest_trigger>\nDevin: @Amy 我接完了，你继续。") {
			t.Fatalf("Amy 第二次 prompt 缺少 Devin 触发上下文: %s", prompt)
		}
	case <-time.After(time.Second):
		t.Fatal("Devin @Amy 后未继续触发 Amy")
	}
	finishedMentionRounds := 0
	_ = collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		if event.EventType != protocol.EventTypeRoundStatus {
			return false
		}
		roundID, _ := event.Data["round_id"].(string)
		status, _ := event.Data["status"].(string)
		if strings.HasPrefix(roundID, "room_mention_") && status == "finished" {
			finishedMentionRounds++
		}
		return finishedMentionRounds >= 2
	})
}

func TestRealtimeServiceQueuesPublicMentionWhenTargetRunning(t *testing.T) {
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
		Name:     "公区 @ 排队房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	devinCurrentClient := newFakeRoomClient()
	amyClient := newFakeRoomClient()
	devinQueuedPrompt := make(chan string, 1)
	devinQueryCount := 0
	devinCurrentClient.onQuery = func(_ context.Context, prompt string) error {
		devinQueryCount++
		if devinQueryCount == 1 {
			return nil
		}
		devinQueuedPrompt <- prompt
		go sendFakeAssistantResult(devinCurrentClient, "devin-public-mention-after-busy", "天气任务已处理。")
		return nil
	}
	amyClient.onQuery = func(_ context.Context, _ string) error {
		go sendFakeAssistantResult(amyClient, "amy-public-mention-busy", "@Devin 当前天气任务交给你。")
		return nil
	}

	permission := permissionctx.NewContext()
	service := NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{devinCurrentClient, amyClient}},
	)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-public-mention-queue")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@Devin 先处理一个长任务",
		RoundID:        "room-round-devin-busy",
	}); err != nil {
		t.Fatalf("启动 Devin 长任务失败: %v", err)
	}
	_ = collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeStreamStart && event.AgentID == devin.AgentID
	})

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@Amy 让 Devin 查下天气",
		RoundID:        "room-round-amy-mentions-busy-devin",
	}); err != nil {
		t.Fatalf("启动 Amy 公区 @ 失败: %v", err)
	}

	var queuedItem protocol.InputQueueItem
	_ = collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		if event.EventType != protocol.EventTypeInputQueue {
			return false
		}
		for _, item := range inputQueueItemsFromEvent(event) {
			if item.Source == protocol.InputQueueSourceAgentPublicMention && item.AgentID == devin.AgentID {
				queuedItem = item
				return true
			}
		}
		return false
	})
	if queuedItem.SourceMessageID != "amy-public-mention-busy" ||
		queuedItem.SourceAgentID != amy.AgentID ||
		len(queuedItem.TargetAgentIDs) != 1 ||
		queuedItem.TargetAgentIDs[0] != devin.AgentID {
		t.Fatalf("公区 @ 队列项缺少来源或目标: %+v", queuedItem)
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
		t.Fatalf("读取目标 agent session 队列失败: %v", err)
	}
	if len(targetQueueItems) != 1 || targetQueueItems[0].ID != queuedItem.ID {
		t.Fatalf("Room 队列未落到目标 agent session: event=%+v stored=%+v", queuedItem, targetQueueItems)
	}

	devinCurrentClient.mu.Lock()
	interruptCalls := devinCurrentClient.interruptCalls
	devinCurrentClient.mu.Unlock()
	if interruptCalls != 0 {
		t.Fatalf("公区 @ 不应中断正在工作的目标 agent: interruptCalls=%d", interruptCalls)
	}
	select {
	case prompt := <-devinQueuedPrompt:
		t.Fatalf("目标 agent 尚未空闲前不应启动 queued mention: %s", prompt)
	default:
	}

	go sendFakeAssistantResult(devinCurrentClient, "devin-current-task-done", "当前长任务完成。")
	select {
	case prompt := <-devinQueuedPrompt:
		if !strings.Contains(prompt, "<latest_trigger>\nAmy: @Devin 当前天气任务交给你。") {
			t.Fatalf("queued mention prompt 缺少公区 @ 触发上下文: %s", prompt)
		}
		if strings.Contains(prompt, "type:") || strings.Contains(prompt, "fanout_targets:") {
			t.Fatalf("queued mention prompt 不应包含字段化 trigger: %s", prompt)
		}
	case <-time.After(time.Second):
		t.Fatal("目标 agent 空闲后未派发 queued mention")
	}
	_ = collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		if event.EventType != protocol.EventTypeRoundStatus {
			return false
		}
		roundID, _ := event.Data["round_id"].(string)
		status, _ := event.Data["status"].(string)
		return strings.HasPrefix(roundID, "room_mention_") && status == "finished"
	})
}

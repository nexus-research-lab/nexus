// queue_test.go 覆盖 Room 输入队列、轮次身份与输入游标测试。
package realtime_test

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/app"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	realtimesvc "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	queueadmissionstore "github.com/nexus-research-lab/nexus/internal/storage/queueadmission"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	_ "modernc.org/sqlite"
)

func TestRealtimeDirectUserQueueKeepsConfigurationContextAfterAdmissionClaim(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := app.NewAgentService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authctx.WithState(context.Background(), authctx.State{AuthRequired: false})
	host := createTestAgent(t, agentService, ctx, "Queue Host")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs:             []string{host.AgentID},
		HostAgentID:          host.AgentID,
		HostAutoReplyEnabled: true,
		Name:                 "queue provenance",
		Title:                "main",
	})
	if err != nil {
		t.Fatal(err)
	}

	client := newFakeRoomClient()
	prompts := make(chan string, 1)
	client.onQuery = func(_ context.Context, prompt string) error {
		prompts <- prompt
		go sendFakeAssistantResult(client, "room-queue-provenance", "done")
		return nil
	}
	runtimeManager := runtimectx.NewManager()
	service := NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permissionctx.NewContext(),
		&fakeRoomFactory{clients: []*fakeRoomClient{client}},
	)
	service.SetQueueAdmissionStore(queueadmissionstore.NewRepository(cfg, db))
	sources := make(chan string, 1)
	bindings := make(chan authctx.QueuedHumanPrincipalBinding, 1)
	service.SetMCPServerBuilder(func(
		ctx context.Context,
		_ *protocol.Agent,
		_ string,
		_ string,
		sourceContextType string,
		_ string,
		_ string,
		_ *atomic.Int64,
		_ sdkpermission.Mode,
	) map[string]sdkmcp.ServerConfig {
		sources <- sourceContextType
		if binding, ok := authctx.QueuedHumanPrincipalBindingFromContext(ctx); ok {
			bindings <- binding
		}
		return nil
	})
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	runtimeSessionKey := protocol.BuildRoomAgentSessionKey(
		roomContext.Conversation.ID,
		host.AgentID,
		roomContext.Room.RoomType,
	)
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if closeErr := runtimeManager.CloseSession(cleanupCtx, runtimeSessionKey); closeErr != nil &&
			!runtimectx.IsRuntimeTransportClosedError(closeErr) {
			t.Errorf("clean up Room provenance runtime: %v", closeErr)
		}
	})
	result, err := service.HandleInputQueue(ctx, realtimesvc.InputQueueRequest{
		SessionKey:                  sharedSessionKey,
		RoomID:                      roomContext.Room.ID,
		ConversationID:              roomContext.Conversation.ID,
		ClientMessageID:             "client-room-queue-provenance",
		Action:                      "enqueue",
		Content:                     "update the room settings safely",
		TrustedConfigurationContext: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case source := <-sources:
		if source != "room" {
			t.Fatalf("configuration source = %q, want room", source)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Room configuration source")
	}
	select {
	case binding := <-bindings:
		if binding.UserID != authctx.SystemUserID ||
			binding.AuthMethod != authctx.AuthMethodLocal ||
			binding.SessionID != "" {
			t.Fatalf("queued Room human binding = %+v", binding)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for queued Room human binding")
	}
	select {
	case prompt := <-prompts:
		if !strings.Contains(prompt, "update the room settings safely") {
			t.Fatalf("Room queued prompt = %q", prompt)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Room queued prompt")
	}
	deadline := time.Now().Add(5 * time.Second)
	status := ""
	for time.Now().Before(deadline) {
		queryErr := db.QueryRow(
			`SELECT status FROM configuration_queue_admissions
WHERE queue_item_id = ? AND scope = 'room'`,
			result.ItemID,
		).Scan(&status)
		if queryErr == nil && status == queueadmissionstore.StatusConsumed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if status != queueadmissionstore.StatusConsumed {
		t.Fatalf("Room queue admission status = %q, want consumed", status)
	}
	deadline = time.Now().Add(5 * time.Second)
	for service.CountRunningTasks(host.AgentID) != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if running := service.CountRunningTasks(host.AgentID); running != 0 {
		t.Fatalf("Room queue round did not settle: running=%d", running)
	}
}

func TestRealtimeServiceDispatchesLateRoomGuidanceAfterRoundFinishes(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
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
	runtimeManager := runtimectx.NewManager()
	service := NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{firstClient}},
	)
	ctx := context.Background()
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	registerRealtimeServiceCleanup(t, service, runtimeManager, sharedSessionKey)
	sender := newRealtimeTestSender("room-sender-late-guide")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
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

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
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
	if _, err = service.HandleInputQueue(ctx, realtimesvc.InputQueueRequest{
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
	messages, err := roomHistory.ReadMessages(roomContext.Room.OwnerUserID, roomContext.Conversation.ID, nil)
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
	messages, err = roomHistory.ReadMessages(roomContext.Room.OwnerUserID, roomContext.Conversation.ID, nil)
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

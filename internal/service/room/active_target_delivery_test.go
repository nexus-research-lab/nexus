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
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
)

func TestRealtimeServiceUnmentionedInterjectionKeepsActiveRootTargets(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "owner-active-room-target",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	worker := createTestAgent(t, agentService, ctx, "Worker")
	host := createTestAgent(t, agentService, ctx, "Host")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs:             []string{worker.AgentID, host.AgentID},
		Name:                 "活跃插话目标测试",
		HostAgentID:          host.AgentID,
		HostAutoReplyEnabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	activeClient := newFakeRoomClient()
	activeClient.onQuery = func(context.Context, string) error { return nil }
	queuedClient := newFakeRoomClient()
	queuedPrompt := make(chan string, 1)
	queuedClient.onQuery = func(_ context.Context, prompt string) error {
		queuedPrompt <- prompt
		go sendFakeAssistantResult(queuedClient, "assistant-active-target-queue", "已处理补充要求")
		return nil
	}
	permission := permissionctx.NewContext()
	factory := &fakeRoomFactory{clients: []*fakeRoomClient{activeClient, queuedClient}}
	service := NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permission,
		factory,
	)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-active-target")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@Worker 先处理当前任务",
		RoundID:        "room-round-active-worker",
	}); err != nil {
		t.Fatalf("启动活跃 Worker round 失败: %v", err)
	}
	_ = collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeStreamStart && event.AgentID == worker.AgentID
	})

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "然后重点检查一下边界条件",
		RoundID:        "room-round-unmentioned-guide",
		DeliveryPolicy: protocol.ChatDeliveryPolicyGuide,
	}); err != nil {
		t.Fatalf("发送无 @ 直接插话失败: %v", err)
	}
	_ = collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeChatAck && event.Data["round_id"] == "room-round-unmentioned-guide"
	})
	additionalContext := ""
	for _, matcher := range factory.LastOptions().Hooks.Matchers[sdkhook.EventPostToolUse] {
		for _, hook := range matcher.Hooks {
			output, hookErr := hook(ctx, sdkhook.Input{EventName: sdkhook.EventPostToolUse}, "tool-active-target")
			if hookErr != nil {
				t.Fatalf("消费无 @ 直接插话失败: %v", hookErr)
			}
			if output.SpecificOutput != nil {
				additionalContext += output.SpecificOutput.AdditionalContext
			}
		}
	}
	if !strings.Contains(additionalContext, "然后重点检查一下边界条件") {
		t.Fatalf("无 @ 直接插话没有进入活跃 Worker slot: %q", additionalContext)
	}

	if err = service.HandleInputQueue(ctx, roomsvc.InputQueueRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Action:         "enqueue",
		Content:        "完成后再给一个简短结论",
		DeliveryPolicy: protocol.ChatDeliveryPolicyQueue,
	}); err != nil {
		t.Fatalf("发送无 @ input queue 插话失败: %v", err)
	}
	queueStore := workspacestore.NewInputQueueStore(cfg.WorkspacePath)
	workerLocation := workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  worker.WorkspacePath,
		SessionKey:     protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, worker.AgentID, roomContext.Room.RoomType),
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
	}
	workerItems, err := queueStore.Snapshot(workerLocation)
	if err != nil {
		t.Fatal(err)
	}
	foundQueuedFollowUp := false
	for _, item := range workerItems {
		if item.Content == "完成后再给一个简短结论" &&
			item.DeliveryPolicy == protocol.ChatDeliveryPolicyQueue &&
			len(item.TargetAgentIDs) == 1 && item.TargetAgentIDs[0] == worker.AgentID {
			foundQueuedFollowUp = true
		}
	}
	if !foundQueuedFollowUp {
		t.Fatalf("无 @ input queue 应绑定活跃 Worker slot: %+v", workerItems)
	}
	hostItems, err := queueStore.Snapshot(workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  host.WorkspacePath,
		SessionKey:     protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, host.AgentID, roomContext.Room.RoomType),
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hostItems) != 0 {
		t.Fatalf("活跃 Worker 存在时不应先把无 @ input queue 交给 Host: %+v", hostItems)
	}

	go sendFakeAssistantResult(activeClient, "assistant-active-worker", "第一轮完成")
	select {
	case prompt := <-queuedPrompt:
		if !strings.Contains(prompt, "完成后再给一个简短结论") || strings.Contains(prompt, "room host default takeover") {
			t.Fatalf("input queue 接力目标或 prompt 不正确: %s", prompt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("活跃 Worker 完成后，无 @ input queue 未接力派发")
	}
	events := collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		roundID, _ := event.Data["round_id"].(string)
		return event.EventType == protocol.EventTypeRoundStatus && strings.HasPrefix(roundID, "queue_") && event.Data["status"] == "finished"
	})
	if !hasChatAckPendingAgent(events, worker.AgentID) || hasChatAckPendingAgent(events, host.AgentID) {
		t.Fatalf("input queue 接力应继续由 Worker 回复，而不是 Host: %+v", events)
	}
}

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
)

func TestRealtimeServiceHostConsumesQueuedInputAsSoonAsItsSlotFinishes(t *testing.T) {
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

	type followUpQuery struct {
		client *fakeRoomClient
		prompt string
	}
	hostFollowUpPrompt := make(chan followUpQuery, 1)
	newReusableClient := func() *fakeRoomClient {
		client := newFakeRoomClient()
		queryCount := 0
		client.onQuery = func(_ context.Context, prompt string) error {
			queryCount++
			if queryCount == 1 {
				return nil
			}
			hostFollowUpPrompt <- followUpQuery{client: client, prompt: prompt}
			go sendFakeAssistantResult(client, "assistant-host-follow-up", "已处理补充要求")
			return nil
		}
		return client
	}
	firstClient := newReusableClient()
	secondClient := newReusableClient()
	permission := permissionctx.NewContext()
	factory := &fakeRoomFactory{clients: []*fakeRoomClient{firstClient, secondClient}}
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
		Content:        "@Host @Worker 同时处理当前任务",
		RoundID:        "room-round-active-host-worker",
	}); err != nil {
		t.Fatalf("启动 Host/Worker shared round 失败: %v", err)
	}
	started := map[string]bool{}
	_ = collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		if event.EventType == protocol.EventTypeStreamStart {
			started[event.AgentID] = true
		}
		return started[worker.AgentID] && started[host.AgentID]
	})
	options := factory.Options()
	if len(options) != 2 {
		t.Fatalf("shared round runtime 数量 = %d, want 2", len(options))
	}
	hostClient := firstClient
	workerClient := secondClient
	if options[1].CWD == host.WorkspacePath {
		hostClient, workerClient = secondClient, firstClient
	} else if options[0].CWD != host.WorkspacePath {
		t.Fatalf("无法按 workspace 识别 Host runtime: %+v", options)
	}

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "然后重点检查一下边界条件",
		RoundID:        "room-round-unmentioned-host-queue",
		UserMessageID:  "msg-unmentioned-host-queue",
		DeliveryPolicy: protocol.ChatDeliveryPolicyQueue,
	}); err != nil {
		t.Fatalf("发送无 @ Host 队列输入失败: %v", err)
	}
	events := collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeChatAck && event.Data["round_id"] == "room-round-unmentioned-host-queue"
	})
	for _, event := range events {
		if event.EventType == protocol.EventTypeChatAck && event.Data["round_id"] == "room-round-unmentioned-host-queue" {
			if committed, _ := event.Data["user_message_committed"].(bool); committed {
				t.Fatalf("Host 尚未消费时用户消息不应提交: %+v", event.Data)
			}
		}
	}

	queueStore := workspacestore.NewInputQueueStore(cfg.WorkspacePath)
	hostLocation := workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  host.WorkspacePath,
		SessionKey:     protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, host.AgentID, roomContext.Room.RoomType),
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
	}
	hostItems, err := queueStore.Snapshot(hostLocation)
	if err != nil {
		t.Fatal(err)
	}
	if len(hostItems) != 1 || hostItems[0].SourceMessageID != "msg-unmentioned-host-queue" ||
		len(hostItems[0].TargetAgentIDs) != 1 || hostItems[0].TargetAgentIDs[0] != host.AgentID {
		t.Fatalf("无 @ 输入应只进入 Host 消费队列: %+v", hostItems)
	}
	workerItems, err := queueStore.Snapshot(workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  worker.WorkspacePath,
		SessionKey:     protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, worker.AgentID, roomContext.Room.RoomType),
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(workerItems) != 0 {
		t.Fatalf("无 @ 输入不应被运行中的 Worker 抢走: %+v", workerItems)
	}

	go sendFakeAssistantResult(hostClient, "assistant-host-current", "Host 当前任务完成")
	select {
	case followUp := <-hostFollowUpPrompt:
		if followUp.client != hostClient {
			t.Fatal("Host 队列输入被其他 Agent runtime 消费")
		}
		if !strings.Contains(followUp.prompt, "然后重点检查一下边界条件") {
			t.Fatalf("Host follow-up prompt 缺少队列输入: %s", followUp.prompt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Host slot 结束后未立即消费队列，仍在等待 Worker")
	}
	if service.CountRunningTasks(worker.AgentID) == 0 {
		t.Fatal("测试前提失效：Host 接力时 Worker 应仍在运行")
	}
	go sendFakeAssistantResult(workerClient, "assistant-worker-current", "Worker 当前任务完成")
	collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		generating, _ := event.Data["is_generating"].(bool)
		return event.EventType == protocol.EventTypeSessionStatus && !generating
	})
}

package room_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
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

type roomActionBroadcaster struct {
	mu     sync.Mutex
	events []protocol.EventMessage
}

func (b *roomActionBroadcaster) Broadcast(_ context.Context, _ string, event protocol.EventMessage) []error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, event)
	return nil
}

func (b *roomActionBroadcaster) Last() protocol.EventMessage {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.events) == 0 {
		return protocol.EventMessage{}
	}
	return b.events[len(b.events)-1]
}

func TestRealtimeServiceCreatesPrivateMessageAction(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-action",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	amy := createTestAgent(t, agentService, ctx, "Amy")
	devin := createTestAgent(t, agentService, ctx, "Devin")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{amy.AgentID, devin.AgentID},
		Name:     "测试 Room",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	service := roomsvc.NewRealtimeService(cfg, roomService, agentService, runtimectx.NewManager(), permissionctx.NewContext())
	broadcaster := &roomActionBroadcaster{}
	service.SetRoomBroadcaster(broadcaster)
	action, err := service.HandleAction(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomActionRequest{
		ActionType:    protocol.RoomActionTypePrivateMessage,
		SourceAgentID: amy.AgentID,
		TargetAgentID: devin.AgentID,
		Content:       "只给 Devin 的提醒",
	})
	if err != nil {
		t.Fatalf("创建 private_message action 失败: %v", err)
	}
	if action.ReplyTarget != protocol.RoomReplyTargetTargetPrivate {
		t.Fatalf("private_message reply_target 不正确: %+v", action)
	}

	event := broadcaster.Last()
	if event.EventType != protocol.EventTypeRoomAction {
		t.Fatalf("未广播 room_action 事件: %+v", event)
	}
	if _, ok := event.Data["content"]; ok {
		t.Fatalf("private_message 广播不应泄漏正文: %+v", event.Data)
	}

	actionStore := workspacestore.NewRoomActionStore(cfg.WorkspacePath)
	actions, err := actionStore.ReadContextActions(roomContext.Conversation.ID, devin.AgentID)
	if err != nil {
		t.Fatalf("读取 Room action 失败: %v", err)
	}
	if len(actions) != 1 || actions[0].Content != "只给 Devin 的提醒" {
		t.Fatalf("目标成员未读到 private_message: %+v", actions)
	}

	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	messages, err := roomHistory.ReadMessages(roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取公区历史失败: %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("private_message 不应写入公区 feed: %+v", messages)
	}
}

func TestRealtimeServiceRejectsActionForNonMemberTarget(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-action-reject",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	amy := createTestAgent(t, agentService, ctx, "Amy")
	outsider := createTestAgent(t, agentService, ctx, "Outsider")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{amy.AgentID},
		Name:     "测试 Room",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	service := roomsvc.NewRealtimeService(cfg, roomService, agentService, runtimectx.NewManager(), permissionctx.NewContext())
	_, err = service.HandleAction(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomActionRequest{
		ActionType:    protocol.RoomActionTypePrivateMessage,
		SourceAgentID: amy.AgentID,
		TargetAgentID: outsider.AgentID,
		Content:       "不应该投递",
	})
	if !errors.Is(err, roomsvc.ErrRoomMemberNotFound) {
		t.Fatalf("非成员目标应被拒绝: %v", err)
	}
}

func TestRealtimeServiceProjectsPrivateActionToTargetPrompt(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-action-context",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	amy := createTestAgent(t, agentService, ctx, "Amy")
	devin := createTestAgent(t, agentService, ctx, "Devin")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{amy.AgentID, devin.AgentID},
		Name:     "测试 Room",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	client := newFakeRoomClient()
	var seenPrivateAction atomic.Bool
	client.onQuery = func(_ context.Context, prompt string) error {
		seenPrivateAction.Store(strings.Contains(prompt, "<room_actions>") &&
			strings.Contains(prompt, "只给 Devin 的提醒"))
		sendFakeAssistantResult(client, "assistant-sdk-action-context", "收到")
		return nil
	}
	service := roomsvc.NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permissionctx.NewContext(),
		&fakeRoomFactory{clients: []*fakeRoomClient{client}},
	)
	if _, err = service.HandleAction(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomActionRequest{
		ActionType:    protocol.RoomActionTypePrivateMessage,
		SourceAgentID: amy.AgentID,
		TargetAgentID: devin.AgentID,
		Content:       "只给 Devin 的提醒",
	}); err != nil {
		t.Fatalf("创建 private action 失败: %v", err)
	}

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID),
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@Devin 请处理",
		RoundID:        "round-action-context",
		ReqID:          "req-action-context",
	}); err != nil {
		t.Fatalf("发送 Room 消息失败: %v", err)
	}
	deadline := time.After(3 * time.Second)
	for !seenPrivateAction.Load() {
		select {
		case <-deadline:
			t.Fatal("目标 agent prompt 未包含 private Room action")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

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

	_, err = service.HandleAction(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomActionRequest{
		ActionType:    protocol.RoomActionTypePrivateNote,
		SourceAgentID: outsider.AgentID,
		Content:       "非成员 source 不应写入",
	})
	if !errors.Is(err, roomsvc.ErrRoomMemberNotFound) {
		t.Fatalf("非成员 source 应被拒绝: %v", err)
	}

	_, err = service.HandleAction(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomActionRequest{
		ActionType:       protocol.RoomActionTypeMarker,
		SourceAgentID:    amy.AgentID,
		AudienceAgentIDs: []string{outsider.AgentID},
		Content:          "非成员 audience 不应写入",
		ReplyTarget:      protocol.RoomReplyTargetAudience,
	})
	if !errors.Is(err, roomsvc.ErrRoomMemberNotFound) {
		t.Fatalf("非成员 audience 应被拒绝: %v", err)
	}
}

func TestRealtimeServiceProjectsAudienceNoneAndPublicMarkerActions(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-action-projection",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	amy := createTestAgent(t, agentService, ctx, "Amy")
	devin := createTestAgent(t, agentService, ctx, "Devin")
	sam := createTestAgent(t, agentService, ctx, "Sam")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{amy.AgentID, devin.AgentID, sam.AgentID},
		Name:     "测试 Room",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	service := roomsvc.NewRealtimeService(cfg, roomService, agentService, runtimectx.NewManager(), permissionctx.NewContext())
	broadcaster := &roomActionBroadcaster{}
	service.SetRoomBroadcaster(broadcaster)
	actionStore := workspacestore.NewRoomActionStore(cfg.WorkspacePath)

	_, err = service.HandleAction(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomActionRequest{
		ActionType:       protocol.RoomActionTypeMarker,
		SourceAgentID:    amy.AgentID,
		AudienceAgentIDs: []string{devin.AgentID},
		Content:          "只给 Devin 的 audience 标记",
		ReplyTarget:      protocol.RoomReplyTargetAudience,
	})
	if err != nil {
		t.Fatalf("创建 audience action 失败: %v", err)
	}
	assertRoomActionContents(t, actionStore, roomContext.Conversation.ID, devin.AgentID, []string{"只给 Devin 的 audience 标记"})
	assertRoomActionContents(t, actionStore, roomContext.Conversation.ID, sam.AgentID, nil)

	_, err = service.HandleAction(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomActionRequest{
		ActionType:    protocol.RoomActionTypeMarker,
		SourceAgentID: amy.AgentID,
		Content:       "公开阶段标记",
		Visibility:    protocol.RoomActionVisibilityPublic,
	})
	if err != nil {
		t.Fatalf("创建 public marker 失败: %v", err)
	}
	if got := broadcaster.Last().Data["content"]; got != "公开阶段标记" {
		t.Fatalf("public marker 应广播正文: %+v", broadcaster.Last().Data)
	}
	assertRoomActionContents(t, actionStore, roomContext.Conversation.ID, sam.AgentID, []string{"公开阶段标记"})

	_, err = service.HandleAction(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomActionRequest{
		ActionType:    protocol.RoomActionTypeMarker,
		SourceAgentID: amy.AgentID,
		Content:       "只记录不投影",
		Visibility:    protocol.RoomActionVisibilityPublic,
		ReplyTarget:   protocol.RoomReplyTargetNone,
	})
	if err != nil {
		t.Fatalf("创建 none action 失败: %v", err)
	}
	if _, exists := broadcaster.Last().Data["content"]; exists {
		t.Fatalf("reply_target=none 不应广播正文: %+v", broadcaster.Last().Data)
	}
	allActions, err := actionStore.ReadActions(roomContext.Conversation.ID)
	if err != nil {
		t.Fatalf("读取 action log 失败: %v", err)
	}
	if len(allActions) != 3 || allActions[2].Content != "只记录不投影" {
		t.Fatalf("none action 应只落盘: %+v", allActions)
	}
	assertRoomActionContents(t, actionStore, roomContext.Conversation.ID, devin.AgentID, []string{
		"只给 Devin 的 audience 标记",
		"公开阶段标记",
	})
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

	firstClient := newFakeRoomClient()
	secondClient := newFakeRoomClient()
	var seenPrivateAction atomic.Bool
	var secondPromptSkippedOldAction atomic.Bool
	firstClient.onQuery = func(_ context.Context, prompt string) error {
		seenPrivateAction.Store(strings.Contains(prompt, "<room_actions>") &&
			strings.Contains(prompt, "只给 Devin 的提醒"))
		sendFakeAssistantResult(firstClient, "assistant-sdk-action-context", "收到")
		return nil
	}
	secondClient.onQuery = func(_ context.Context, prompt string) error {
		secondPromptSkippedOldAction.Store(strings.Contains(prompt, "<latest_trigger>") &&
			!strings.Contains(prompt, "只给 Devin 的提醒"))
		sendFakeAssistantResult(secondClient, "assistant-sdk-action-context-2", "收到第二轮")
		return nil
	}
	service := roomsvc.NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permissionctx.NewContext(),
		&fakeRoomFactory{clients: []*fakeRoomClient{firstClient, secondClient}},
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

	actionStore := workspacestore.NewRoomActionStore(cfg.WorkspacePath)
	waitForRoomActionCursor(t, actionStore, roomContext.Conversation.ID, devin.AgentID)

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID),
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@Devin 再处理一次",
		RoundID:        "round-action-context-2",
		ReqID:          "req-action-context-2",
	}); err != nil {
		t.Fatalf("发送第二轮 Room 消息失败: %v", err)
	}
	deadline = time.After(3 * time.Second)
	for !secondPromptSkippedOldAction.Load() {
		select {
		case <-deadline:
			t.Fatal("Room action cursor 未阻止旧 private action 重复投影")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func assertRoomActionContents(
	t *testing.T,
	store *workspacestore.RoomActionStore,
	conversationID string,
	agentID string,
	want []string,
) {
	t.Helper()
	actions, err := store.ReadContextActions(conversationID, agentID)
	if err != nil {
		t.Fatalf("读取 Room action 失败: %v", err)
	}
	got := make([]string, 0, len(actions))
	for _, action := range actions {
		got = append(got, action.Content)
	}
	if len(got) != len(want) {
		t.Fatalf("Room action 投影数量不正确: got=%+v want=%+v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("Room action 投影内容不正确: got=%+v want=%+v", got, want)
		}
	}
}

func waitForRoomActionCursor(
	t *testing.T,
	store *workspacestore.RoomActionStore,
	conversationID string,
	agentID string,
) workspacestore.RoomActionCursor {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		cursor, ok, err := store.ReadActionCursor(conversationID, agentID)
		if err != nil {
			t.Fatalf("读取 Room action cursor 失败: %v", err)
		}
		if ok {
			return cursor
		}
		select {
		case <-deadline:
			t.Fatal("Room action cursor 未落盘")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

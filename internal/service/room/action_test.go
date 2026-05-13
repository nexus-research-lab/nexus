package room_test

import (
	"context"
	"errors"
	"fmt"
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

func (b *roomActionBroadcaster) Find(eventType protocol.EventType) (protocol.EventMessage, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, event := range b.events {
		if event.EventType == eventType {
			return event, true
		}
	}
	return protocol.EventMessage{}, false
}

func (b *roomActionBroadcaster) Events() []protocol.EventMessage {
	b.mu.Lock()
	defer b.mu.Unlock()
	events := make([]protocol.EventMessage, len(b.events))
	copy(events, b.events)
	return events
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

	client := newFakeRoomClient()
	var seenPrivateAction atomic.Bool
	client.onQuery = func(_ context.Context, prompt string) error {
		seenPrivateAction.Store(strings.Contains(prompt, "<room_actions>") &&
			strings.Contains(prompt, "只给 Devin 的提醒"))
		sendFakeAssistantResult(client, "assistant-sdk-action-create", "收到")
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

	event := waitForRoomBroadcastEvent(t, broadcaster, protocol.EventTypeRoomAction)
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
	waitForRoomBroadcastEvent(t, broadcaster, protocol.EventTypeRoomActionConsumed)
	deadline := time.After(3 * time.Second)
	for !seenPrivateAction.Load() {
		select {
		case <-deadline:
			t.Fatal("private_message 未自动唤醒目标 agent")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	messages, err := roomHistory.ReadMessages(roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取公区历史失败: %v", err)
	}
	for _, message := range messages {
		if strings.Contains(fmt.Sprint(message), "只给 Devin 的提醒") {
			t.Fatalf("private_message 正文不应写入公区 feed: %+v", messages)
		}
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
	broadcaster := &roomActionBroadcaster{}
	service.SetRoomBroadcaster(broadcaster)
	action, err := service.HandleAction(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomActionRequest{
		ActionType:    protocol.RoomActionTypePrivateMessage,
		SourceAgentID: amy.AgentID,
		TargetAgentID: devin.AgentID,
		Content:       "只给 Devin 的提醒",
	})
	if err != nil {
		t.Fatalf("创建 private action 失败: %v", err)
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
	cursor := waitForRoomActionCursor(t, actionStore, roomContext.Conversation.ID, devin.AgentID)
	consumedEvent := waitForRoomBroadcastEvent(t, broadcaster, protocol.EventTypeRoomActionConsumed)
	if consumedEvent.AgentID != devin.AgentID {
		t.Fatalf("Room action consumed agent 不正确: %+v", consumedEvent)
	}
	if got := consumedEvent.Data["last_action_id"]; got != action.ActionID || got != cursor.LastActionID {
		t.Fatalf("Room action consumed cursor 不正确: event=%+v cursor=%+v action=%+v", consumedEvent.Data, cursor, action)
	}
	if _, exists := consumedEvent.Data["content"]; exists {
		t.Fatalf("Room action consumed 事件不应包含正文: %+v", consumedEvent.Data)
	}

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

func TestRealtimeServiceProjectsPrivateActionReplyToSender(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-action-reply-sender",
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
	client.onQuery = func(_ context.Context, prompt string) error {
		if !strings.Contains(prompt, "需要私下回 Amy 的内容") {
			t.Fatalf("sender_private reply_target 不应影响目标读取原 private_message:\n%s", prompt)
		}
		sendFakeAssistantResult(client, "devin-private-reply-sender", "这是给 Amy 的私下回复")
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
	broadcaster := &roomActionBroadcaster{}
	service.SetRoomBroadcaster(broadcaster)

	_, err = service.HandleAction(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomActionRequest{
		ActionType:    protocol.RoomActionTypePrivateMessage,
		SourceAgentID: amy.AgentID,
		TargetAgentID: devin.AgentID,
		Content:       "需要私下回 Amy 的内容",
		ReplyTarget:   protocol.RoomReplyTargetSenderPrivate,
	})
	if err != nil {
		t.Fatalf("创建 sender_private private action 失败: %v", err)
	}
	waitForRoomBroadcastEvent(t, broadcaster, protocol.EventTypeRoomActionConsumed)

	actionStore := workspacestore.NewRoomActionStore(cfg.WorkspacePath)
	deadline := time.After(3 * time.Second)
	for {
		amyActions, err := actionStore.ReadContextActions(roomContext.Conversation.ID, amy.AgentID)
		if err != nil {
			t.Fatalf("读取 Amy Room action 失败: %v", err)
		}
		if roomActionContentsContain(amyActions, "这是给 Amy 的私下回复") {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("sender_private 回复未投影给发送者: %+v", amyActions)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	devinActions, err := actionStore.ReadContextActions(roomContext.Conversation.ID, devin.AgentID)
	if err != nil {
		t.Fatalf("读取 Devin Room action 失败: %v", err)
	}
	if roomActionContentsContain(devinActions, "这是给 Amy 的私下回复") {
		t.Fatalf("sender_private 回复不应再次投影给目标成员: %+v", devinActions)
	}

	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	messages, err := roomHistory.ReadMessages(roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取公区历史失败: %v", err)
	}
	if strings.Contains(fmt.Sprint(messages), "这是给 Amy 的私下回复") {
		t.Fatalf("sender_private 回复不应进入公区 feed: %+v", messages)
	}
}

func TestRealtimeServiceKeepsPrivateActionFailureOutOfPublicFeed(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-action-private-failure",
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
	client.onQuery = func(_ context.Context, prompt string) error {
		if !strings.Contains(prompt, "这条私信会触发失败") {
			t.Fatalf("target_private 失败前仍应读取原 private_message:\n%s", prompt)
		}
		return errors.New("私域失败只应留在目标上下文")
	}
	service := roomsvc.NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permissionctx.NewContext(),
		&fakeRoomFactory{clients: []*fakeRoomClient{client}},
	)
	broadcaster := &roomActionBroadcaster{}
	service.SetRoomBroadcaster(broadcaster)

	_, err = service.HandleAction(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomActionRequest{
		ActionType:    protocol.RoomActionTypePrivateMessage,
		SourceAgentID: amy.AgentID,
		TargetAgentID: devin.AgentID,
		Content:       "这条私信会触发失败",
		ReplyTarget:   protocol.RoomReplyTargetTargetPrivate,
	})
	if err != nil {
		t.Fatalf("创建 target_private private action 失败: %v", err)
	}
	waitForRoomBroadcastEvent(t, broadcaster, protocol.EventTypeStreamEnd)

	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	messages, err := roomHistory.ReadMessages(roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取公区历史失败: %v", err)
	}
	publicPayload := fmt.Sprint(messages)
	if strings.Contains(publicPayload, "这条私信会触发失败") || strings.Contains(publicPayload, "私域失败只应留在目标上下文") {
		t.Fatalf("target_private 失败结果不应进入公区 feed: %+v", messages)
	}
	for _, event := range broadcaster.Events() {
		if event.EventType != protocol.EventTypeMessage && event.EventType != protocol.EventTypeError {
			continue
		}
		eventPayload := fmt.Sprint(event)
		if strings.Contains(eventPayload, "这条私信会触发失败") || strings.Contains(eventPayload, "私域失败只应留在目标上下文") {
			t.Fatalf("target_private 失败结果不应广播到公区 websocket: %+v", event)
		}
	}
}

func TestRealtimeServiceQueuesPrivateActionWakeWhenTargetBusy(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-action-busy",
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

	currentStarted := make(chan struct{})
	releaseCurrent := make(chan struct{})
	var startOnce sync.Once
	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() {
			close(releaseCurrent)
		})
	}
	defer release()

	devinCurrentClient := newFakeRoomClient()
	devinPrivateClient := newFakeRoomClient()
	var seenPrivateAction atomic.Bool
	devinCurrentClient.onQuery = func(queryCtx context.Context, _ string) error {
		startOnce.Do(func() {
			close(currentStarted)
		})
		select {
		case <-releaseCurrent:
			sendFakeAssistantResult(devinCurrentClient, "devin-current-before-private", "当前任务完成")
			return nil
		case <-queryCtx.Done():
			return queryCtx.Err()
		}
	}
	devinPrivateClient.onQuery = func(_ context.Context, prompt string) error {
		seenPrivateAction.Store(strings.Contains(prompt, "<room_actions>") &&
			strings.Contains(prompt, "排队后的私信") &&
			strings.Contains(prompt, "收到一条 Room private_message"))
		sendFakeAssistantResult(devinPrivateClient, "devin-private-after-busy", "私信已处理")
		return nil
	}

	service := roomsvc.NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permissionctx.NewContext(),
		&fakeRoomFactory{clients: []*fakeRoomClient{devinCurrentClient, devinPrivateClient}},
	)
	broadcaster := &roomActionBroadcaster{}
	service.SetRoomBroadcaster(broadcaster)

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID),
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@Devin 先处理公开任务",
		RoundID:        "round-private-action-busy",
		ReqID:          "req-private-action-busy",
	}); err != nil {
		t.Fatalf("发送 Room 消息失败: %v", err)
	}
	select {
	case <-currentStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("目标 agent 未进入当前任务")
	}

	action, err := service.HandleAction(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomActionRequest{
		ActionType:    protocol.RoomActionTypePrivateMessage,
		SourceAgentID: amy.AgentID,
		TargetAgentID: devin.AgentID,
		Content:       "排队后的私信",
	})
	if err != nil {
		t.Fatalf("创建 private action 失败: %v", err)
	}

	queueStore := workspacestore.NewInputQueueStore(cfg.WorkspacePath)
	items, err := queueStore.Snapshot(workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  devin.WorkspacePath,
		SessionKey:     protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, devin.AgentID, roomContext.Room.RoomType),
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
	})
	if err != nil {
		t.Fatalf("读取 Room action 队列失败: %v", err)
	}
	if len(items) != 1 ||
		items[0].Source != protocol.InputQueueSourceAgentRoomAction ||
		items[0].SourceMessageID != action.ActionID ||
		items[0].ReplyTarget != protocol.RoomReplyTargetTargetPrivate ||
		strings.Contains(items[0].Content, "排队后的私信") {
		t.Fatalf("private action 应以脱敏队列项等待目标空闲: %+v", items)
	}

	release()
	deadline := time.After(3 * time.Second)
	for !seenPrivateAction.Load() {
		select {
		case <-deadline:
			t.Fatal("目标空闲后未消费排队 private action")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	consumedEvent := waitForRoomBroadcastEvent(t, broadcaster, protocol.EventTypeRoomActionConsumed)
	if got := consumedEvent.Data["last_action_id"]; got != action.ActionID {
		t.Fatalf("private action 消费游标不正确: %+v", consumedEvent.Data)
	}
}

func roomActionContentsContain(actions []protocol.RoomActionRecord, content string) bool {
	for _, action := range actions {
		if action.Content == content {
			return true
		}
	}
	return false
}

func waitForRoomBroadcastEvent(
	t *testing.T,
	broadcaster *roomActionBroadcaster,
	eventType protocol.EventType,
) protocol.EventMessage {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		if event, ok := broadcaster.Find(eventType); ok {
			return event
		}
		select {
		case <-deadline:
			t.Fatalf("未广播 Room 事件: %s", eventType)
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

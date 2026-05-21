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

func TestRealtimeServiceCreatesAudiencePrivateMessageAction(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-action-audience-private",
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

	devinClient := newFakeRoomClient()
	samClient := newFakeRoomClient()
	var devinSeen atomic.Bool
	var samSeen atomic.Bool
	devinClient.onQuery = func(_ context.Context, prompt string) error {
		devinSeen.Store(strings.Contains(prompt, "<room_actions>") &&
			strings.Contains(prompt, "只给小范围成员的消息") &&
			strings.Contains(prompt, "private_message audience="))
		sendFakeAssistantResult(devinClient, "devin-audience-private", "Devin 收到")
		return nil
	}
	samClient.onQuery = func(_ context.Context, prompt string) error {
		samSeen.Store(strings.Contains(prompt, "<room_actions>") &&
			strings.Contains(prompt, "只给小范围成员的消息") &&
			strings.Contains(prompt, "private_message audience="))
		sendFakeAssistantResult(samClient, "sam-audience-private", "Sam 收到")
		return nil
	}
	service := roomsvc.NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permissionctx.NewContext(),
		&fakeRoomFactory{clients: []*fakeRoomClient{devinClient, samClient}},
	)
	broadcaster := &roomActionBroadcaster{}
	service.SetRoomBroadcaster(broadcaster)

	action, err := service.HandleAction(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomActionRequest{
		ActionType:       protocol.RoomActionTypePrivateMessage,
		SourceAgentID:    amy.AgentID,
		AudienceAgentIDs: []string{devin.AgentID, sam.AgentID},
		Content:          "只给小范围成员的消息",
	})
	if err != nil {
		t.Fatalf("创建 audience private_message action 失败: %v", err)
	}
	if action.TargetAgentID != "" ||
		action.ReplyTarget != protocol.RoomReplyTargetAudience ||
		action.WakePolicy != protocol.RoomWakePolicyImmediate {
		t.Fatalf("audience private_message 默认路由不正确: %+v", action)
	}

	event := waitForRoomBroadcastEvent(t, broadcaster, protocol.EventTypeRoomAction)
	if event.Data["action_type"] != string(protocol.RoomActionTypePrivateMessage) ||
		event.Data["reply_target"] != string(protocol.RoomReplyTargetAudience) ||
		event.Data["wake_policy"] != string(protocol.RoomWakePolicyImmediate) ||
		!strings.Contains(fmt.Sprint(event.Data["audience_agent_ids"]), devin.AgentID) ||
		!strings.Contains(fmt.Sprint(event.Data["audience_agent_ids"]), sam.AgentID) {
		t.Fatalf("audience private_message 创建事件不正确: %+v", event.Data)
	}
	if _, ok := event.Data["content"]; ok {
		t.Fatalf("audience private_message 事件不应泄漏正文: %+v", event.Data)
	}
	waitForRoomBroadcastEventMatching(t, broadcaster, protocol.EventTypeRoomAction, func(event protocol.EventMessage) bool {
		return event.Data["event_kind"] == "wake_started" && event.Data["target_agent_id"] == devin.AgentID
	})
	waitForRoomBroadcastEventMatching(t, broadcaster, protocol.EventTypeRoomAction, func(event protocol.EventMessage) bool {
		return event.Data["event_kind"] == "wake_started" && event.Data["target_agent_id"] == sam.AgentID
	})

	deadline := time.After(3 * time.Second)
	for !devinSeen.Load() || !samSeen.Load() {
		select {
		case <-deadline:
			t.Fatalf("audience private_message 未唤醒全部受众: devin=%v sam=%v", devinSeen.Load(), samSeen.Load())
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	actionStore := workspacestore.NewRoomActionStore(cfg.WorkspacePath)
	devinActions, err := actionStore.ReadContextActions(roomContext.Conversation.ID, devin.AgentID)
	if err != nil {
		t.Fatalf("读取 Devin audience private_message 失败: %v", err)
	}
	if !roomActionContentsContain(devinActions, "只给小范围成员的消息") {
		t.Fatalf("Devin 未读到 audience private_message: %+v", devinActions)
	}
	samActions, err := actionStore.ReadContextActions(roomContext.Conversation.ID, sam.AgentID)
	if err != nil {
		t.Fatalf("读取 Sam audience private_message 失败: %v", err)
	}
	if !roomActionContentsContain(samActions, "只给小范围成员的消息") {
		t.Fatalf("Sam 未读到 audience private_message: %+v", samActions)
	}
	assertRoomActionContents(t, actionStore, roomContext.Conversation.ID, amy.AgentID, nil)
}

func TestRealtimeServiceRecordsAudiencePrivateMessageWithoutWake(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-action-audience-private-none",
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

	var queryCount atomic.Int32
	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, _ string) error {
		queryCount.Add(1)
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
		ActionType:       protocol.RoomActionTypePrivateMessage,
		SourceAgentID:    amy.AgentID,
		AudienceAgentIDs: []string{devin.AgentID, sam.AgentID},
		Content:          "先只投递给小范围成员",
		WakePolicy:       protocol.RoomWakePolicyNone,
	})
	if err != nil {
		t.Fatalf("创建 wake_policy none audience private_message 失败: %v", err)
	}
	if action.ReplyTarget != protocol.RoomReplyTargetAudience ||
		action.WakePolicy != protocol.RoomWakePolicyNone {
		t.Fatalf("wake_policy none audience private_message 路由不正确: %+v", action)
	}
	time.Sleep(80 * time.Millisecond)
	if queryCount.Load() != 0 {
		t.Fatal("wake_policy=none 的 audience private_message 不应立即唤醒")
	}
	event := waitForRoomBroadcastEvent(t, broadcaster, protocol.EventTypeRoomAction)
	if event.Data["wake_policy"] != string(protocol.RoomWakePolicyNone) ||
		event.Data["event_kind"] != "created" {
		t.Fatalf("wake_policy none audience private_message 事件不正确: %+v", event.Data)
	}

	actionStore := workspacestore.NewRoomActionStore(cfg.WorkspacePath)
	assertRoomActionContents(t, actionStore, roomContext.Conversation.ID, devin.AgentID, []string{"先只投递给小范围成员"})
	assertRoomActionContents(t, actionStore, roomContext.Conversation.ID, sam.AgentID, []string{"先只投递给小范围成员"})
}

func TestRealtimeServiceSchedulesDelayedPrivateMessageWake(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-action-delayed-private",
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
	var seenDelayedAction atomic.Bool
	client.onQuery = func(_ context.Context, prompt string) error {
		seenDelayedAction.Store(strings.Contains(prompt, "<room_actions>") &&
			strings.Contains(prompt, "稍后提醒 Devin 汇总"))
		sendFakeAssistantResult(client, "devin-delayed-private", "Devin 稍后收到")
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
		Content:       "稍后提醒 Devin 汇总",
		WakePolicy:    protocol.RoomWakePolicyDelayed,
		DelaySeconds:  1,
	})
	if err != nil {
		t.Fatalf("创建 delayed private_message 失败: %v", err)
	}
	if action.WakePolicy != protocol.RoomWakePolicyDelayed || action.DelaySeconds != 1 {
		t.Fatalf("delayed private_message 唤醒参数不正确: %+v", action)
	}
	actionStore := workspacestore.NewRoomActionStore(cfg.WorkspacePath)
	actions, err := actionStore.ReadActions(roomContext.Conversation.ID)
	if err != nil {
		t.Fatalf("读取 delayed Room action 失败: %v", err)
	}
	if len(actions) != 1 ||
		actions[0].WakePolicy != protocol.RoomWakePolicyDelayed ||
		actions[0].DelaySeconds != 1 {
		t.Fatalf("delayed Room action 未正确落盘: %+v", actions)
	}
	createdEvent := waitForRoomBroadcastEvent(t, broadcaster, protocol.EventTypeRoomAction)
	if createdEvent.Data["event_kind"] != "created" ||
		createdEvent.Data["wake_policy"] != string(protocol.RoomWakePolicyDelayed) ||
		createdEvent.Data["delay_seconds"] != 1 {
		t.Fatalf("delayed private_message created 事件不正确: %+v", createdEvent.Data)
	}
	scheduledEvent := waitForRoomBroadcastEventMatching(t, broadcaster, protocol.EventTypeRoomAction, func(event protocol.EventMessage) bool {
		return event.Data["event_kind"] == "wake_scheduled" && event.Data["action_id"] == action.ActionID
	})
	if scheduledEvent.Data["target_agent_id"] != devin.AgentID ||
		scheduledEvent.Data["delay_seconds"] != 1 {
		t.Fatalf("delayed private_message scheduled 事件不正确: %+v", scheduledEvent.Data)
	}
	time.Sleep(200 * time.Millisecond)
	if seenDelayedAction.Load() {
		t.Fatal("delayed private_message 不应立即唤醒目标")
	}
	waitForRoomBroadcastEventMatching(t, broadcaster, protocol.EventTypeRoomAction, func(event protocol.EventMessage) bool {
		return event.Data["event_kind"] == "wake_started" && event.Data["target_agent_id"] == devin.AgentID
	})
	deadline := time.After(3 * time.Second)
	for !seenDelayedAction.Load() {
		select {
		case <-deadline:
			t.Fatal("delayed private_message 未在延迟后唤醒目标")
		default:
			time.Sleep(10 * time.Millisecond)
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

	_, err = service.HandleAction(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomActionRequest{
		ActionType:       protocol.RoomActionTypePrivateMessage,
		SourceAgentID:    amy.AgentID,
		AudienceAgentIDs: []string{outsider.AgentID},
		Content:          "非成员私域受众不应写入",
	})
	if !errors.Is(err, roomsvc.ErrRoomMemberNotFound) {
		t.Fatalf("private_message 非成员 audience 应被拒绝: %v", err)
	}
}

func TestRealtimeServiceRejectsInvalidDelayedWakeOptions(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-action-delayed-reject",
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

	_, err = service.HandleAction(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomActionRequest{
		ActionType:    protocol.RoomActionTypePrivateMessage,
		SourceAgentID: amy.AgentID,
		TargetAgentID: devin.AgentID,
		Content:       "缺少 delay_seconds",
		WakePolicy:    protocol.RoomWakePolicyDelayed,
	})
	if err == nil || !strings.Contains(err.Error(), "delay_seconds") {
		t.Fatalf("delayed 缺少 delay_seconds 应被拒绝: %v", err)
	}

	_, err = service.HandleAction(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomActionRequest{
		ActionType:    protocol.RoomActionTypePrivateMessage,
		SourceAgentID: amy.AgentID,
		TargetAgentID: devin.AgentID,
		Content:       "delay_seconds 不应单独使用",
		WakePolicy:    protocol.RoomWakePolicyImmediate,
		DelaySeconds:  1,
	})
	if err == nil || !strings.Contains(err.Error(), "delay_seconds") {
		t.Fatalf("非 delayed 携带 delay_seconds 应被拒绝: %v", err)
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

	devinClient := newFakeRoomClient()
	amyWakeClient := newFakeRoomClient()
	var amySawReply atomic.Bool
	var amyWakeOwner atomic.Value
	devinClient.onQuery = func(_ context.Context, prompt string) error {
		if !strings.Contains(prompt, "需要私下回 Amy 的内容") {
			t.Fatalf("sender_private reply_target 不应影响目标读取原 private_message:\n%s", prompt)
		}
		if !strings.Contains(prompt, "reply_target=sender_private") {
			t.Fatalf("sender_private 触发上下文应标注回复投影:\n%s", prompt)
		}
		sendFakeAssistantResult(devinClient, "devin-private-reply-sender", "这是给 Amy 的私下回复")
		return nil
	}
	amyWakeClient.onQuery = func(ctx context.Context, prompt string) error {
		amyWakeOwner.Store(authsvc.OwnerUserID(ctx))
		amySawReply.Store(strings.Contains(prompt, "收到一条 Room private_message") &&
			strings.Contains(prompt, "这是给 Amy 的私下回复"))
		sendFakeAssistantResult(amyWakeClient, "amy-private-reply-wake", "<nexus_room_no_reply/>")
		return nil
	}
	factory := &fakeRoomFactory{clients: []*fakeRoomClient{devinClient, amyWakeClient}}
	service := roomsvc.NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permissionctx.NewContext(),
		factory,
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
	deadline = time.After(3 * time.Second)
	for !amySawReply.Load() {
		select {
		case <-deadline:
			t.Fatal("sender_private 回复落盘后未唤醒发送者")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	if got, _ := amyWakeOwner.Load().(string); got != "user-room-action-reply-sender" {
		t.Fatalf("sender_private 回复唤醒应保留 owner user scope: got=%q", got)
	}
	options := factory.LastOptions()
	if got := strings.TrimSpace(options.Env["NEXUSCTL_USER_ID"]); got != "user-room-action-reply-sender" {
		t.Fatalf("sender_private 回复唤醒的 nexusctl user scope 不正确: got=%q env=%+v", got, options.Env)
	}
	if got := strings.TrimSpace(options.Env["NEXUS_ROOM_CONVERSATION_ID"]); got != roomContext.Conversation.ID {
		t.Fatalf("sender_private 回复唤醒的 conversation env 不正确: got=%q want=%q", got, roomContext.Conversation.ID)
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

func TestRealtimeServiceCreatesImmediateRequestReplyAction(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-request-reply",
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
		for _, expected := range []string{
			"收到一条 Room request_reply",
			"[request_reply request_id=",
			"需要公开回复的请求",
			"reply_target=public_feed",
		} {
			if !strings.Contains(prompt, expected) {
				t.Fatalf("request_reply prompt 缺少 %q:\n%s", expected, prompt)
			}
		}
		sendFakeAssistantResult(client, "devin-request-reply-public", "这是公开回复")
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
		ActionType:    protocol.RoomActionTypeRequestReply,
		SourceAgentID: amy.AgentID,
		TargetAgentID: devin.AgentID,
		Content:       "需要公开回复的请求",
	})
	if err != nil {
		t.Fatalf("创建 request_reply action 失败: %v", err)
	}
	if action.RequestID == "" || action.RequestID != action.ActionID {
		t.Fatalf("request_reply 应生成稳定 request_id: %+v", action)
	}
	if action.ReplyTarget != protocol.RoomReplyTargetPublicFeed ||
		action.WakePolicy != protocol.RoomWakePolicyImmediate {
		t.Fatalf("request_reply 默认投影和唤醒策略不正确: %+v", action)
	}
	event := waitForRoomBroadcastEvent(t, broadcaster, protocol.EventTypeRoomAction)
	if event.Data["action_type"] != string(protocol.RoomActionTypeRequestReply) ||
		event.Data["event_kind"] != "created" ||
		event.Data["request_id"] != action.RequestID ||
		event.Data["wake_policy"] != string(protocol.RoomWakePolicyImmediate) {
		t.Fatalf("request_reply room_action 事件不正确: %+v", event.Data)
	}
	if _, ok := event.Data["content"]; ok {
		t.Fatalf("request_reply websocket 事件不应泄漏正文: %+v", event.Data)
	}
	wakeEvent := waitForRoomBroadcastEventMatching(t, broadcaster, protocol.EventTypeRoomAction, func(event protocol.EventMessage) bool {
		return event.Data["event_kind"] == "wake_started" && event.Data["request_id"] == action.RequestID
	})
	if wakeEvent.Data["target_agent_id"] != devin.AgentID {
		t.Fatalf("request_reply wake_started 事件目标不正确: %+v", wakeEvent.Data)
	}

	messages := waitForRoomHistoryContent(t, cfg.WorkspacePath, roomContext.Conversation.ID, "done")
	payload := fmt.Sprint(messages)
	if strings.Contains(payload, "需要公开回复的请求") {
		t.Fatalf("request_reply 原始正文不应进入公区 feed: %+v", messages)
	}
}

func TestRealtimeServiceRecordsRequestReplyWithoutWake(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-request-reply-none",
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

	var queryCount atomic.Int32
	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, _ string) error {
		queryCount.Add(1)
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
		ActionType:    protocol.RoomActionTypeRequestReply,
		SourceAgentID: amy.AgentID,
		TargetAgentID: devin.AgentID,
		Content:       "先记录稍后再回复",
		WakePolicy:    protocol.RoomWakePolicyNone,
	})
	if err != nil {
		t.Fatalf("创建 wake_policy none request_reply 失败: %v", err)
	}
	time.Sleep(80 * time.Millisecond)
	if queryCount.Load() != 0 {
		t.Fatal("wake_policy=none 不应立即唤醒目标 agent")
	}
	event := waitForRoomBroadcastEvent(t, broadcaster, protocol.EventTypeRoomAction)
	if event.Data["wake_policy"] != string(protocol.RoomWakePolicyNone) ||
		event.Data["event_kind"] != "created" ||
		event.Data["request_id"] != action.RequestID {
		t.Fatalf("wake_policy none room_action 事件不正确: %+v", event.Data)
	}

	actionStore := workspacestore.NewRoomActionStore(cfg.WorkspacePath)
	actions, err := actionStore.ReadContextActions(roomContext.Conversation.ID, devin.AgentID)
	if err != nil {
		t.Fatalf("读取目标 request_reply 失败: %v", err)
	}
	if len(actions) != 1 ||
		actions[0].ActionType != protocol.RoomActionTypeRequestReply ||
		actions[0].RequestID != action.RequestID ||
		actions[0].Content != "先记录稍后再回复" {
		t.Fatalf("wake_policy none request_reply 应只落盘给目标上下文: %+v", actions)
	}
}

func TestRealtimeServiceProjectsPrivateActionReplyToAudience(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-action-reply-audience",
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

	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, prompt string) error {
		if !strings.Contains(prompt, "需要回给指定受众") {
			t.Fatalf("audience reply_target 不应影响目标读取原 private_message:\n%s", prompt)
		}
		if !strings.Contains(prompt, "reply_target=audience") ||
			!strings.Contains(prompt, "audience=Sam("+sam.AgentID+")") {
			t.Fatalf("audience 触发上下文应标注回复受众:\n%s", prompt)
		}
		sendFakeAssistantResult(client, "devin-private-reply-audience", "这是给 Sam 的私下回复")
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
		ActionType:       protocol.RoomActionTypePrivateMessage,
		SourceAgentID:    amy.AgentID,
		TargetAgentID:    devin.AgentID,
		AudienceAgentIDs: []string{sam.AgentID},
		Content:          "需要回给指定受众",
		ReplyTarget:      protocol.RoomReplyTargetAudience,
	})
	if err != nil {
		t.Fatalf("创建 audience private action 失败: %v", err)
	}
	waitForRoomBroadcastEvent(t, broadcaster, protocol.EventTypeRoomActionConsumed)

	actionStore := workspacestore.NewRoomActionStore(cfg.WorkspacePath)
	samActions := waitForRoomActionContent(t, actionStore, roomContext.Conversation.ID, sam.AgentID, "这是给 Sam 的私下回复")
	if len(samActions) != 1 ||
		samActions[0].ActionType != protocol.RoomActionTypeMarker ||
		samActions[0].ReplyTarget != protocol.RoomReplyTargetAudience {
		t.Fatalf("audience 回复应以私域 marker 投影给受众: %+v", samActions)
	}
	amyActions, err := actionStore.ReadContextActions(roomContext.Conversation.ID, amy.AgentID)
	if err != nil {
		t.Fatalf("读取 Amy Room action 失败: %v", err)
	}
	if roomActionContentsContain(amyActions, "这是给 Sam 的私下回复") {
		t.Fatalf("audience 回复不应投影给非受众发送者: %+v", amyActions)
	}
	devinActions, err := actionStore.ReadContextActions(roomContext.Conversation.ID, devin.AgentID)
	if err != nil {
		t.Fatalf("读取 Devin Room action 失败: %v", err)
	}
	if roomActionContentsContain(devinActions, "这是给 Sam 的私下回复") {
		t.Fatalf("audience 回复不应再次投影给目标成员: %+v", devinActions)
	}

	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	messages, err := roomHistory.ReadMessages(roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取公区历史失败: %v", err)
	}
	if strings.Contains(fmt.Sprint(messages), "这是给 Sam 的私下回复") {
		t.Fatalf("audience 回复不应进入公区 feed: %+v", messages)
	}
}

func TestRealtimeServiceSuppressesPrivateActionReplyWhenTargetNone(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-action-reply-none",
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
		if !strings.Contains(prompt, "只唤醒不投影回复") {
			t.Fatalf("none reply_target 不应影响目标读取原 private_message:\n%s", prompt)
		}
		if !strings.Contains(prompt, "reply_target=none") {
			t.Fatalf("none 触发上下文应标注不投影回复:\n%s", prompt)
		}
		sendFakeAssistantResult(client, "devin-private-reply-none", "这段回复不应被任何人看到")
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
		Content:       "只唤醒不投影回复",
		ReplyTarget:   protocol.RoomReplyTargetNone,
	})
	if err != nil {
		t.Fatalf("创建 none private action 失败: %v", err)
	}
	waitForRoomBroadcastEvent(t, broadcaster, protocol.EventTypeRoomActionConsumed)

	actionStore := workspacestore.NewRoomActionStore(cfg.WorkspacePath)
	for _, agentID := range []string{amy.AgentID, devin.AgentID} {
		actions, err := actionStore.ReadContextActions(roomContext.Conversation.ID, agentID)
		if err != nil {
			t.Fatalf("读取 Room action 失败: %v", err)
		}
		if roomActionContentsContain(actions, "这段回复不应被任何人看到") {
			t.Fatalf("none 回复不应写回任何 action 投影: agent=%s actions=%+v", agentID, actions)
		}
	}

	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	messages, err := roomHistory.ReadMessages(roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取公区历史失败: %v", err)
	}
	if strings.Contains(fmt.Sprint(messages), "这段回复不应被任何人看到") {
		t.Fatalf("none 回复不应进入公区 feed: %+v", messages)
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
	queuedEvent := waitForRoomBroadcastEventMatching(t, broadcaster, protocol.EventTypeRoomAction, func(event protocol.EventMessage) bool {
		return event.Data["event_kind"] == "wake_queued" && event.Data["action_id"] == action.ActionID
	})
	if queuedEvent.Data["target_agent_id"] != devin.AgentID {
		t.Fatalf("private action wake_queued 事件目标不正确: %+v", queuedEvent.Data)
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

func TestRealtimeServiceKeepsQueuedAudienceActionVisibleAfterReply(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-action-audience-cursor",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	amy := createTestAgent(t, agentService, ctx, "Amy")
	tom := createTestAgent(t, agentService, ctx, "Tom")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{amy.AgentID, tom.AgentID},
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

	tomCurrentClient := newFakeRoomClient()
	tomQueuedClient := newFakeRoomClient()
	var seenQueuedAudienceAction atomic.Bool
	tomCurrentClient.onQuery = func(queryCtx context.Context, prompt string) error {
		if !strings.Contains(prompt, "第一条 audience 消息") {
			t.Fatalf("当前 audience prompt 缺少首条 action:\n%s", prompt)
		}
		startOnce.Do(func() {
			close(currentStarted)
		})
		select {
		case <-releaseCurrent:
			sendFakeAssistantResult(tomCurrentClient, "tom-current-audience-reply", "Tom 对第一条的回复")
			return nil
		case <-queryCtx.Done():
			return queryCtx.Err()
		}
	}
	tomQueuedClient.onQuery = func(_ context.Context, prompt string) error {
		seenQueuedAudienceAction.Store(strings.Contains(prompt, "第二条 audience 消息"))
		sendFakeAssistantResult(tomQueuedClient, "tom-queued-audience-reply", "Tom 已处理第二条")
		return nil
	}

	service := roomsvc.NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permissionctx.NewContext(),
		&fakeRoomFactory{clients: []*fakeRoomClient{tomCurrentClient, tomQueuedClient}},
	)
	broadcaster := &roomActionBroadcaster{}
	service.SetRoomBroadcaster(broadcaster)

	_, err = service.HandleAction(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomActionRequest{
		ActionType:       protocol.RoomActionTypePrivateMessage,
		SourceAgentID:    amy.AgentID,
		AudienceAgentIDs: []string{tom.AgentID},
		Content:          "第一条 audience 消息",
	})
	if err != nil {
		t.Fatalf("创建首条 audience action 失败: %v", err)
	}
	select {
	case <-currentStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("目标 agent 未开始处理首条 audience action")
	}

	queuedAction, err := service.HandleAction(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomActionRequest{
		ActionType:       protocol.RoomActionTypePrivateMessage,
		SourceAgentID:    amy.AgentID,
		AudienceAgentIDs: []string{tom.AgentID},
		Content:          "第二条 audience 消息",
	})
	if err != nil {
		t.Fatalf("创建排队 audience action 失败: %v", err)
	}
	waitForRoomBroadcastEventMatching(t, broadcaster, protocol.EventTypeRoomAction, func(event protocol.EventMessage) bool {
		return event.Data["event_kind"] == "wake_queued" && event.Data["action_id"] == queuedAction.ActionID
	})

	release()
	deadline := time.After(3 * time.Second)
	for !seenQueuedAudienceAction.Load() {
		select {
		case <-deadline:
			t.Fatal("目标空闲后未看到排队期间创建的 audience action")
		default:
			time.Sleep(10 * time.Millisecond)
		}
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

func waitForRoomActionContent(
	t *testing.T,
	store *workspacestore.RoomActionStore,
	conversationID string,
	agentID string,
	content string,
) []protocol.RoomActionRecord {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		actions, err := store.ReadContextActions(conversationID, agentID)
		if err != nil {
			t.Fatalf("读取 Room action 失败: %v", err)
		}
		if roomActionContentsContain(actions, content) {
			return actions
		}
		select {
		case <-deadline:
			t.Fatalf("Room action 未投影给目标成员: agent=%s content=%q actions=%+v", agentID, content, actions)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func waitForRoomHistoryContent(
	t *testing.T,
	workspacePath string,
	conversationID string,
	content string,
) []protocol.Message {
	t.Helper()
	store := workspacestore.NewRoomHistoryStore(workspacePath)
	deadline := time.After(3 * time.Second)
	for {
		messages, err := store.ReadMessages(conversationID, nil)
		if err != nil {
			t.Fatalf("读取公区历史失败: %v", err)
		}
		if strings.Contains(fmt.Sprint(messages), content) {
			return messages
		}
		select {
		case <-deadline:
			t.Fatalf("公区历史未出现目标内容 %q: %+v", content, messages)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func waitForRoomBroadcastEvent(
	t *testing.T,
	broadcaster *roomActionBroadcaster,
	eventType protocol.EventType,
) protocol.EventMessage {
	t.Helper()
	return waitForRoomBroadcastEventMatching(t, broadcaster, eventType, func(protocol.EventMessage) bool {
		return true
	})
}

func waitForRoomBroadcastEventMatching(
	t *testing.T,
	broadcaster *roomActionBroadcaster,
	eventType protocol.EventType,
	matches func(protocol.EventMessage) bool,
) protocol.EventMessage {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		for _, event := range broadcaster.Events() {
			if event.EventType == eventType && matches(event) {
				return event
			}
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

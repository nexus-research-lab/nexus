package room_test

import (
	"context"
	"fmt"
	"slices"
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

type roomDirectedMessageBroadcaster struct {
	mu     sync.Mutex
	events []protocol.EventMessage
}

func (b *roomDirectedMessageBroadcaster) Broadcast(_ context.Context, _ string, event protocol.EventMessage) []error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, event)
	return nil
}

func (b *roomDirectedMessageBroadcaster) Events() []protocol.EventMessage {
	b.mu.Lock()
	defer b.mu.Unlock()
	events := make([]protocol.EventMessage, len(b.events))
	copy(events, b.events)
	return events
}

func TestRealtimeServiceCreatesDirectedMessageWithoutPublicLeak(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-directed-message",
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

	service := roomsvc.NewRealtimeService(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permissionctx.NewContext(),
	)
	broadcaster := &roomDirectedMessageBroadcaster{}
	service.SetRoomBroadcaster(broadcaster)

	message, err := service.HandleDirectedMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomDirectedMessageRequest{
		SourceAgentID: amy.AgentID,
		Recipients:    []string{devin.AgentID},
		Content:       "只给 Devin 的提醒",
		ReplyRoute: protocol.RoomReplyRoute{
			Mode:       protocol.RoomReplyRoutePrivate,
			Recipients: []string{amy.AgentID},
			WakePolicy: protocol.RoomWakePolicyNone,
		},
	})
	if err != nil {
		t.Fatalf("创建 directed message 失败: %v", err)
	}
	if message.WakePolicy != protocol.RoomWakePolicyNone ||
		message.ReplyRoute.Mode != protocol.RoomReplyRoutePrivate ||
		len(message.Recipients) != 1 ||
		message.Recipients[0] != devin.AgentID {
		t.Fatalf("directed message 默认路由不正确: %+v", message)
	}

	event := waitForRoomBroadcastEvent(t, broadcaster, protocol.EventTypeRoomDirectedMessage)
	if event.RoomID != roomContext.Room.ID || event.ConversationID != roomContext.Conversation.ID {
		t.Fatalf("directed message 事件上下文不正确: %+v", event)
	}
	if event.Data["message_id"] != message.MessageID || event.Data["event_kind"] != "created" {
		t.Fatalf("directed message 创建事件不正确: %+v", event.Data)
	}
	if _, ok := event.Data["content"]; ok {
		t.Fatalf("directed message 事件不应泄漏正文: %+v", event.Data)
	}
	messageStore := workspacestore.NewRoomDirectedMessageStore(cfg.WorkspacePath)
	devinMessages, err := messageStore.ReadContextMessages(roomContext.Conversation.ID, devin.AgentID)
	if err != nil {
		t.Fatalf("读取 Devin directed message 失败: %v", err)
	}
	if len(devinMessages) != 1 || devinMessages[0].Content != "只给 Devin 的提醒" {
		t.Fatalf("目标成员未读到 directed message: %+v", devinMessages)
	}

	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	messages, err := roomHistory.ReadMessages(roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取公区历史失败: %v", err)
	}
	for _, publicMessage := range messages {
		if strings.Contains(fmt.Sprint(publicMessage), "只给 Devin 的提醒") {
			t.Fatalf("directed message 正文不应写入公区 feed: %+v", messages)
		}
	}
}

func TestRealtimeServiceProjectsDirectedMessageReplyToPrivateRoute(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-directed-message-reply",
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
	var seenDirectedMessage atomic.Bool
	client.onQuery = func(_ context.Context, prompt string) error {
		seenDirectedMessage.Store(strings.Contains(prompt, "<room_directed_messages>") &&
			strings.Contains(prompt, "帮我汇总这段私下结论") &&
			strings.Contains(prompt, "reply_route=private recipients=Amy"))
		sendFakeAssistantResult(client, "assistant-directed-message-reply", "这是给 Amy 的私下回复")
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
	broadcaster := &roomDirectedMessageBroadcaster{}
	service.SetRoomBroadcaster(broadcaster)

	message, err := service.HandleDirectedMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomDirectedMessageRequest{
		SourceAgentID: amy.AgentID,
		Recipients:    []string{devin.AgentID},
		Content:       "帮我汇总这段私下结论",
		WakePolicy:    protocol.RoomWakePolicyImmediate,
		ReplyRoute: protocol.RoomReplyRoute{
			Mode:       protocol.RoomReplyRoutePrivate,
			Recipients: []string{amy.AgentID},
			WakePolicy: protocol.RoomWakePolicyNone,
		},
	})
	if err != nil {
		t.Fatalf("创建 immediate directed message 失败: %v", err)
	}
	if len(message.WakeTargets) != 1 || message.WakeTargets[0] != devin.AgentID {
		t.Fatalf("未显式指定 wake_targets 时应唤醒全部 recipients: %+v", message)
	}
	waitForRoomBroadcastEventMatching(t, broadcaster, protocol.EventTypeRoomDirectedMessage, func(event protocol.EventMessage) bool {
		return event.Data["event_kind"] == "wake_started" && event.Data["message_id"] == message.MessageID
	})
	waitForRoomBroadcastEvent(t, broadcaster, protocol.EventTypeRoomDirectedMessageConsumed)

	deadline := time.After(3 * time.Second)
	for !seenDirectedMessage.Load() {
		select {
		case <-deadline:
			t.Fatal("目标成员未看到 directed message 上下文")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	messageStore := workspacestore.NewRoomDirectedMessageStore(cfg.WorkspacePath)
	amyMessages := waitForRoomDirectedMessageContent(t, messageStore, roomContext.Conversation.ID, amy.AgentID, "这是给 Amy 的私下回复")
	if !roomDirectedMessageContentsContain(amyMessages, "帮我汇总这段私下结论") {
		t.Fatalf("reply_route 接收方应能看到原始请求与私下回复: %+v", amyMessages)
	}
	devinMessages, err := messageStore.ReadContextMessages(roomContext.Conversation.ID, devin.AgentID)
	if err != nil {
		t.Fatalf("读取 Devin directed message 失败: %v", err)
	}
	if roomDirectedMessageContentsContain(devinMessages, "这是给 Amy 的私下回复") {
		t.Fatalf("私下回复不应再次投影给原目标成员: %+v", devinMessages)
	}

	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	publicMessages, err := roomHistory.ReadMessages(roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取公区历史失败: %v", err)
	}
	if strings.Contains(fmt.Sprint(publicMessages), "这是给 Amy 的私下回复") {
		t.Fatalf("reply_route=private 的回复不应进入公区 feed: %+v", publicMessages)
	}
}

func TestRealtimeServiceQueuesDirectedMessageWhenTargetRunning(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-directed-message-queue",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	amy := createTestAgent(t, agentService, ctx, "Amy")
	devin := createTestAgent(t, agentService, ctx, "Devin")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs:               []string{amy.AgentID, devin.AgentID},
		Name:                   "私信排队 Room",
		PrivateMessagesEnabled: true,
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	devinCurrentClient := newFakeRoomClient()
	queuedPrompt := make(chan string, 1)
	devinQueryCount := 0
	devinCurrentClient.onQuery = func(_ context.Context, prompt string) error {
		devinQueryCount++
		if devinQueryCount == 1 {
			return nil
		}
		queuedPrompt <- prompt
		go sendFakeAssistantResult(devinCurrentClient, "devin-directed-message-after-busy", "这是给 Amy 的排队私下回复")
		return nil
	}

	permission := permissionctx.NewContext()
	service := roomsvc.NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{devinCurrentClient}},
	)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-directed-message-queue")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@Devin 先处理一个长任务",
		RoundID:        "room-round-devin-directed-busy",
	}); err != nil {
		t.Fatalf("启动 Devin 长任务失败: %v", err)
	}
	_ = collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeStreamStart && event.AgentID == devin.AgentID
	})

	message, err := service.HandleDirectedMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomDirectedMessageRequest{
		SourceAgentID: amy.AgentID,
		Recipients:    []string{devin.AgentID},
		Content:       "只给 Devin 的排队私信",
		WakePolicy:    protocol.RoomWakePolicyImmediate,
		ReplyRoute: protocol.RoomReplyRoute{
			Mode:       protocol.RoomReplyRoutePrivate,
			Recipients: []string{amy.AgentID},
			WakePolicy: protocol.RoomWakePolicyNone,
		},
	})
	if err != nil {
		t.Fatalf("创建 directed message 失败: %v", err)
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
	if len(targetQueueItems) != 1 {
		t.Fatalf("directed message 应排队到目标 agent session: %+v", targetQueueItems)
	}
	queuedItem := targetQueueItems[0]
	if queuedItem.Source != protocol.InputQueueSourceAgentRoomMessage ||
		queuedItem.SourceAgentID != amy.AgentID ||
		queuedItem.SourceMessageID != message.MessageID ||
		len(queuedItem.TargetAgentIDs) != 1 ||
		queuedItem.TargetAgentIDs[0] != devin.AgentID ||
		queuedItem.ReplyRoute.Mode != protocol.RoomReplyRoutePrivate ||
		len(queuedItem.ReplyRoute.Recipients) != 1 ||
		queuedItem.ReplyRoute.Recipients[0] != amy.AgentID {
		t.Fatalf("directed message 队列项缺少来源、目标或 reply_route: %+v", queuedItem)
	}
	if strings.Contains(queuedItem.Content, "只给 Devin 的排队私信") {
		t.Fatalf("directed message 队列项不应泄漏私信正文: %+v", queuedItem)
	}
	select {
	case prompt := <-queuedPrompt:
		t.Fatalf("目标 agent 尚未空闲前不应启动 queued directed message: %s", prompt)
	default:
	}

	go sendFakeAssistantResult(devinCurrentClient, "devin-current-directed-task-done", "当前长任务完成。")
	select {
	case prompt := <-queuedPrompt:
		for _, expected := range []string{
			"<room_directed_messages>",
			"只给 Devin 的排队私信",
			"reply_route=private recipients=Amy",
		} {
			if !strings.Contains(prompt, expected) {
				t.Fatalf("queued directed message prompt 缺少片段 %q:\n%s", expected, prompt)
			}
		}
	case <-time.After(time.Second):
		t.Fatal("目标 agent 空闲后未派发 queued directed message")
	}

	messageStore := workspacestore.NewRoomDirectedMessageStore(cfg.WorkspacePath)
	waitForRoomDirectedMessageContent(t, messageStore, roomContext.Conversation.ID, amy.AgentID, "这是给 Amy 的排队私下回复")
}

func TestRealtimeServiceCarriesPublicRouteFromPrivateHandback(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-public-message-handback",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	amy := createTestAgent(t, agentService, ctx, "Amy")
	devin := createTestAgent(t, agentService, ctx, "Devin")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{amy.AgentID, devin.AgentID},
		Name:     "狼人杀测试 Room",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	witchClient := newFakeRoomClient()
	var witchSawPrompt atomic.Bool
	witchClient.onQuery = func(_ context.Context, prompt string) error {
		witchSawPrompt.Store(strings.Contains(prompt, "今晚被刀的是 Lucy") &&
			strings.Contains(prompt, "reply_route=private recipients=Amy"))
		sendFakeAssistantResult(witchClient, "devin-witch-reply", "救:Lucy；不毒")
		return nil
	}

	var service *roomsvc.RealtimeService
	hostClient := newFakeRoomClient()
	var hostSawHandback atomic.Bool
	hostClient.onQuery = func(_ context.Context, prompt string) error {
		hostSawHandback.Store(strings.Contains(prompt, "救:Lucy；不毒") &&
			strings.Contains(prompt, "reply_route=public"))
		sendFakeAssistantResult(hostClient, "amy-after-private-handback", "<nexus_room_no_reply/>")
		return nil
	}

	service = roomsvc.NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permissionctx.NewContext(),
		&fakeRoomFactory{clients: []*fakeRoomClient{witchClient, hostClient}},
	)
	broadcaster := &roomDirectedMessageBroadcaster{}
	service.SetRoomBroadcaster(broadcaster)

	_, err = service.HandleDirectedMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomDirectedMessageRequest{
		SourceAgentID: amy.AgentID,
		Recipients:    []string{devin.AgentID},
		Content:       "今晚被刀的是 Lucy。是否用解药救？是否用毒药毒谁？格式：救:<名字>|不救；毒:<名字>|不毒。",
		WakePolicy:    protocol.RoomWakePolicyImmediate,
		ReplyRoute: protocol.RoomReplyRoute{
			Mode:       protocol.RoomReplyRoutePrivate,
			Recipients: []string{amy.AgentID},
			WakePolicy: protocol.RoomWakePolicyImmediate,
			NextReplyRoute: &protocol.RoomReplyRoute{
				Mode: protocol.RoomReplyRoutePublic,
			},
		},
	})
	if err != nil {
		t.Fatalf("创建女巫 directed message 失败: %v", err)
	}

	waitForAtomicBool(t, &witchSawPrompt, "女巫未看到私信问题")
	waitForAtomicBool(t, &hostSawHandback, "主持人未收到私域回交")
	messageStore := workspacestore.NewRoomDirectedMessageStore(cfg.WorkspacePath)
	amyMessages := waitForRoomDirectedMessageContent(t, messageStore, roomContext.Conversation.ID, amy.AgentID, "救:Lucy；不毒")
	if !roomDirectedMessageContentHasReplyRoute(amyMessages, "救:Lucy；不毒", protocol.RoomReplyRoutePublic) {
		t.Fatalf("私域回交应携带主持人下一跳公区路线: %+v", amyMessages)
	}
	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	publicMessages, err := roomHistory.ReadMessages(roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取公区历史失败: %v", err)
	}
	publicText := fmt.Sprint(publicMessages)
	if strings.Contains(publicText, "救:Lucy") {
		t.Fatalf("女巫私下回复不应泄漏到公区 feed: %+v", publicMessages)
	}
}

func TestRealtimeServiceRejectsInvalidDirectedMessageRoute(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-directed-message-invalid",
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

	service := roomsvc.NewRealtimeService(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permissionctx.NewContext(),
	)
	_, err = service.HandleDirectedMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomDirectedMessageRequest{
		SourceAgentID: amy.AgentID,
		Recipients:    []string{"agent-not-member"},
		Content:       "非法目标",
	})
	if err == nil {
		t.Fatal("非 Room 成员不应成为 directed message recipient")
	}

	_, err = service.HandleDirectedMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomDirectedMessageRequest{
		SourceAgentID: amy.AgentID,
		Recipients:    []string{devin.AgentID},
		WakeTargets:   []string{amy.AgentID},
		Content:       "唤醒目标越界",
		WakePolicy:    protocol.RoomWakePolicyImmediate,
	})
	if err == nil || !strings.Contains(err.Error(), "subset of recipients") {
		t.Fatalf("wake_targets 必须是 recipients 子集: %v", err)
	}

	_, err = service.HandleDirectedMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomDirectedMessageRequest{
		SourceAgentID: amy.AgentID,
		Recipients:    []string{devin.AgentID},
		WakeTargets:   []string{devin.AgentID},
		Content:       "只记录却指定唤醒目标",
		WakePolicy:    protocol.RoomWakePolicyNone,
	})
	if err == nil || !strings.Contains(err.Error(), "wake_targets") {
		t.Fatalf("wake_policy=none 不应接受 wake_targets: %v", err)
	}

	_, err = service.HandleDirectedMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomDirectedMessageRequest{
		SourceAgentID: amy.AgentID,
		Recipients:    []string{devin.AgentID},
		Content:       "非法 reply route",
		ReplyRoute: protocol.RoomReplyRoute{
			Mode: protocol.RoomReplyRoutePrivate,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "reply_route private requires recipients") {
		t.Fatalf("reply_route=private 缺少 recipients 错误不正确: %v", err)
	}

	_, err = service.HandleDirectedMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomDirectedMessageRequest{
		SourceAgentID: amy.AgentID,
		Recipients:    []string{devin.AgentID},
		Content:       "非法 next reply route",
		ReplyRoute: protocol.RoomReplyRoute{
			Mode:       protocol.RoomReplyRoutePrivate,
			Recipients: []string{amy.AgentID},
			WakePolicy: protocol.RoomWakePolicyNone,
			NextReplyRoute: &protocol.RoomReplyRoute{
				Mode: protocol.RoomReplyRoutePublic,
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "next_reply_route requires reply_route private wake_policy=immediate") {
		t.Fatalf("reply_route=private wake=none 不应接受 next_reply_route: %v", err)
	}
}

func waitForAtomicBool(t *testing.T, value *atomic.Bool, message string) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for !value.Load() {
		select {
		case <-deadline:
			t.Fatal(message)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func roomDirectedMessageContentsContain(messages []protocol.RoomDirectedMessageRecord, content string) bool {
	return slices.ContainsFunc(messages, func(message protocol.RoomDirectedMessageRecord) bool {
		return message.Content == content
	})
}

func roomDirectedMessageContentHasReplyRoute(
	messages []protocol.RoomDirectedMessageRecord,
	content string,
	mode protocol.RoomReplyRouteMode,
) bool {
	return slices.ContainsFunc(messages, func(message protocol.RoomDirectedMessageRecord) bool {
		return message.Content == content && message.ReplyRoute.Mode == mode
	})
}

func waitForRoomDirectedMessageContent(
	t *testing.T,
	store *workspacestore.RoomDirectedMessageStore,
	conversationID string,
	agentID string,
	content string,
) []protocol.RoomDirectedMessageRecord {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		messages, err := store.ReadContextMessages(conversationID, agentID)
		if err != nil {
			t.Fatalf("读取 Room directed message 失败: %v", err)
		}
		if roomDirectedMessageContentsContain(messages, content) {
			return messages
		}
		select {
		case <-deadline:
			t.Fatalf("Room directed message 未投影给目标成员: agent=%s content=%q messages=%+v", agentID, content, messages)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func waitForRoomBroadcastEvent(
	t *testing.T,
	broadcaster *roomDirectedMessageBroadcaster,
	eventType protocol.EventType,
) protocol.EventMessage {
	t.Helper()
	return waitForRoomBroadcastEventMatching(t, broadcaster, eventType, func(protocol.EventMessage) bool {
		return true
	})
}

func waitForRoomBroadcastEventMatching(
	t *testing.T,
	broadcaster *roomDirectedMessageBroadcaster,
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

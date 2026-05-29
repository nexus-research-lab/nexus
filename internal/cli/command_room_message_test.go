package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func TestRoomMessageCommandUsesRuntimeEnvAndInternalEndpoint(t *testing.T) {
	cfg := newCLITestConfig(t)
	migrateCLISQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   authsvc.SystemUserID,
		Username: authsvc.SystemUserID,
		Role:     authsvc.RoleOwner,
	})
	amy, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "Amy"})
	if err != nil {
		t.Fatalf("创建 Amy 失败: %v", err)
	}
	devin, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "Devin"})
	if err != nil {
		t.Fatalf("创建 Devin 失败: %v", err)
	}
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{amy.AgentID, devin.AgentID},
		Name:     "测试 Room",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建测试 server 失败: %v", err)
	}
	httpServer := httptest.NewServer(server.Router())
	defer httpServer.Close()
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + cfg.WebSocketPath
	wsCtx, wsCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer wsCancel()
	conn, _, err := websocket.Dial(wsCtx, wsURL, nil)
	if err != nil {
		t.Fatalf("连接 room websocket 失败: %v", err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "test done") }()
	if err = wsjson.Write(wsCtx, conn, map[string]any{
		"type":            "subscribe_room",
		"room_id":         roomContext.Room.ID,
		"conversation_id": roomContext.Conversation.ID,
	}); err != nil {
		t.Fatalf("订阅 room websocket 失败: %v", err)
	}

	t.Setenv(nexusRoomIDEnvName, roomContext.Room.ID)
	t.Setenv(nexusRoomConversationIDEnvName, roomContext.Conversation.ID)
	t.Setenv(nexusRoomAgentIDEnvName, amy.AgentID)
	t.Setenv(nexusRoomInternalAPIBaseEnvName, httpServer.URL+cfg.APIPrefix)
	t.Setenv(nexusRoomInternalTokenEnvName, server.InternalControlToken())

	payload := runCLICommandWithEnv(
		t,
		cfg,
		map[string]string{nexusctlUserIDEnvName: authsvc.SystemUserID},
		"--json",
		"room",
		"message",
		"send",
		"--recipient-agent-id",
		devin.AgentID,
		"--reply-route",
		"private",
		"--reply-recipient-agent-id",
		amy.AgentID,
		"--reply-wake-policy",
		"immediate",
		"--reply-next-route",
		"public",
		"--correlation-id",
		"small-group-1",
		"--content",
		"hello",
	)
	if payload["action"] != "room_message_send" {
		t.Fatalf("CLI 输出 action 不正确: %+v", payload)
	}
	item, ok := payload["item"].(map[string]any)
	if !ok {
		t.Fatalf("CLI 输出 item 不正确: %+v", payload)
	}
	if _, exists := item["content"]; exists {
		t.Fatalf("CLI tool_result 不应回显 private content: %+v", item)
	}
	if item["content_chars"] != float64(5) || item["correlation_id"] != "small-group-1" {
		t.Fatalf("CLI 输出应保留长度和 correlation_id: %+v", item)
	}
	route, ok := item["reply_route"].(map[string]any)
	if !ok || route["mode"] != string(protocol.RoomReplyRoutePrivate) {
		t.Fatalf("CLI 输出 reply_route 不正确: %+v", item)
	}
	nextRoute, ok := route["next_reply_route"].(map[string]any)
	if !ok || nextRoute["mode"] != string(protocol.RoomReplyRoutePublic) {
		t.Fatalf("CLI 输出 next_reply_route 不正确: %+v", item)
	}

	event := readRoomMessageWebSocketEvent(t, conn)
	if event.EventType != protocol.EventTypeRoomDirectedMessage {
		t.Fatalf("未收到 room_directed_message websocket 事件: %+v", event)
	}
	if event.RoomID != roomContext.Room.ID || event.ConversationID != roomContext.Conversation.ID {
		t.Fatalf("room_directed_message websocket 事件上下文不正确: %+v", event)
	}
	if _, ok := event.Data["content"]; ok {
		t.Fatalf("room_directed_message websocket 事件不应泄漏正文: %+v", event.Data)
	}

	messageStore := workspacestore.NewRoomDirectedMessageStore(cfg.WorkspacePath)
	messages, err := messageStore.ReadMessages(roomContext.Conversation.ID)
	if err != nil {
		t.Fatalf("读取 Room directed message 失败: %v", err)
	}
	if len(messages) != 1 ||
		messages[0].SourceAgentID != amy.AgentID ||
		len(messages[0].Recipients) != 1 ||
		messages[0].Recipients[0] != devin.AgentID ||
		messages[0].Content != "hello" ||
		messages[0].ReplyRoute.Mode != protocol.RoomReplyRoutePrivate ||
		messages[0].ReplyRoute.WakePolicy != protocol.RoomWakePolicyImmediate ||
		messages[0].ReplyRoute.NextReplyRoute == nil ||
		messages[0].ReplyRoute.NextReplyRoute.Mode != protocol.RoomReplyRoutePublic {
		t.Fatalf("CLI 未通过内部 endpoint 创建 directed message: %+v", messages)
	}
}

func TestRoomMessagePublishCommandUsesRuntimeEnvAndInternalEndpoint(t *testing.T) {
	cfg := newCLITestConfig(t)
	migrateCLISQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   authsvc.SystemUserID,
		Username: authsvc.SystemUserID,
		Role:     authsvc.RoleOwner,
	})
	amy, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "Amy"})
	if err != nil {
		t.Fatalf("创建 Amy 失败: %v", err)
	}
	devin, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "Devin"})
	if err != nil {
		t.Fatalf("创建 Devin 失败: %v", err)
	}
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{amy.AgentID, devin.AgentID},
		Name:     "测试 Room",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建测试 server 失败: %v", err)
	}
	httpServer := httptest.NewServer(server.Router())
	defer httpServer.Close()

	t.Setenv(nexusRoomIDEnvName, roomContext.Room.ID)
	t.Setenv(nexusRoomConversationIDEnvName, roomContext.Conversation.ID)
	t.Setenv(nexusRoomAgentIDEnvName, amy.AgentID)
	t.Setenv(nexusRoomInternalAPIBaseEnvName, httpServer.URL+cfg.APIPrefix)
	t.Setenv(nexusRoomInternalTokenEnvName, server.InternalControlToken())

	content := "第 1 天天亮，昨晚平安夜。"
	payload := runCLICommandWithEnv(
		t,
		cfg,
		map[string]string{nexusctlUserIDEnvName: authsvc.SystemUserID},
		"--json",
		"room",
		"message",
		"publish",
		"--correlation-id",
		"daybreak-1",
		"--content",
		content,
	)
	if payload["action"] != "room_message_publish" {
		t.Fatalf("CLI 输出 action 不正确: %+v", payload)
	}
	item, ok := payload["item"].(map[string]any)
	if !ok {
		t.Fatalf("CLI 输出 item 不正确: %+v", payload)
	}
	if item["content_chars"] != float64(len([]rune(content))) ||
		item["correlation_id"] != "daybreak-1" ||
		item["source_agent_id"] != amy.AgentID {
		t.Fatalf("CLI publish 输出不正确: %+v", item)
	}

	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	messages, err := roomHistory.ReadMessages(roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取 Room 公区消息失败: %v", err)
	}
	if !strings.Contains(strings.TrimSpace(fmt.Sprint(messages)), content) {
		t.Fatalf("publish 未写入公区历史: %+v", messages)
	}
}

func TestRoomMessageListAndCursorsUseDirectedMessageStore(t *testing.T) {
	cfg := newCLITestConfig(t)
	migrateCLISQLite(t, cfg.DatabaseURL)

	store := workspacestore.NewRoomDirectedMessageStore(cfg.WorkspacePath)
	if err := store.AppendMessage(protocol.RoomDirectedMessageRecord{
		MessageID:      "message-1",
		RoomID:         "room-1",
		ConversationID: "conversation-1",
		SourceAgentID:  "agent-amy",
		Recipients:     []string{"agent-devin"},
		Content:        "第一条",
		WakePolicy:     protocol.RoomWakePolicyNone,
		ReplyRoute:     protocol.RoomReplyRoute{Mode: protocol.RoomReplyRouteNone},
		Timestamp:      1000,
	}); err != nil {
		t.Fatalf("写入第一条 directed message 失败: %v", err)
	}
	if err := store.AppendMessage(protocol.RoomDirectedMessageRecord{
		MessageID:      "message-2",
		RoomID:         "room-1",
		ConversationID: "conversation-1",
		SourceAgentID:  "agent-sam",
		Recipients:     []string{"agent-amy"},
		Content:        "第二条",
		WakePolicy:     protocol.RoomWakePolicyNone,
		ReplyRoute: protocol.RoomReplyRoute{
			Mode:       protocol.RoomReplyRoutePrivate,
			Recipients: []string{"agent-devin"},
			WakePolicy: protocol.RoomWakePolicyNone,
		},
		Timestamp: 2000,
	}); err != nil {
		t.Fatalf("写入第二条 directed message 失败: %v", err)
	}
	if err := store.AppendMessageCursor(workspacestore.RoomDirectedMessageCursor{
		RoomID:               "room-1",
		ConversationID:       "conversation-1",
		AgentID:              "agent-devin",
		RoundID:              "round-1",
		LastMessageID:        "message-1",
		LastMessageTimestamp: 1000,
		Timestamp:            3000,
	}); err != nil {
		t.Fatalf("写入 directed message cursor 失败: %v", err)
	}

	payload := runCLICommand(
		t,
		cfg,
		"--json",
		"room",
		"message",
		"list",
		"--conversation-id",
		"conversation-1",
		"--agent-id",
		"agent-devin",
		"--include-content",
	)
	if payload["action"] != "room_message_list" || payload["count"] != float64(2) {
		t.Fatalf("room message list 输出不正确: %+v", payload)
	}
	items, ok := payload["items"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("room message list items 不正确: %+v", payload)
	}
	if first, _ := items[0].(map[string]any); first["content"] != "第一条" {
		t.Fatalf("room message list 应输出 include-content 正文: %+v", items)
	}

	payload = runCLICommand(
		t,
		cfg,
		"--json",
		"room",
		"message",
		"list",
		"--conversation-id",
		"conversation-1",
		"--agent-id",
		"agent-devin",
		"--after-cursor",
		"--include-content",
	)
	if payload["action"] != "room_message_list" || payload["count"] != float64(1) {
		t.Fatalf("room message list --after-cursor 输出不正确: %+v", payload)
	}
	afterCursorItems, ok := payload["items"].([]any)
	if !ok || len(afterCursorItems) != 1 {
		t.Fatalf("room message list --after-cursor items 不正确: %+v", payload)
	}
	if only, _ := afterCursorItems[0].(map[string]any); only["message_id"] != "message-2" {
		t.Fatalf("room message list --after-cursor 应只返回 cursor 后消息: %+v", afterCursorItems)
	}

	payload = runCLICommand(
		t,
		cfg,
		"--json",
		"room",
		"message",
		"cursors",
		"--conversation-id",
		"conversation-1",
		"--agent-id",
		"agent-devin",
	)
	if payload["action"] != "room_message_cursors" || payload["count"] != float64(1) {
		t.Fatalf("room message cursors 输出不正确: %+v", payload)
	}
	cursorItems, ok := payload["items"].([]any)
	if !ok || len(cursorItems) != 1 {
		t.Fatalf("room message cursors items 不正确: %+v", payload)
	}
	if cursor, _ := cursorItems[0].(map[string]any); cursor["last_message_id"] != "message-1" {
		t.Fatalf("room message cursors 应输出 last_message_id: %+v", cursorItems)
	}
}

func TestCreateRoomDirectedMessageSendsSourceAgentAsInternalHeader(t *testing.T) {
	var sourceHeader string
	var scopeHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourceHeader = r.Header.Get(nexusInternalRoomAgentIDHeader)
		scopeHeader = r.Header.Get(nexusInternalScopeUserIDHeader)
		if !strings.HasSuffix(r.URL.Path, "/internal/rooms/room-1/conversations/conversation-1/directed-messages") {
			t.Fatalf("internal endpoint path 不正确: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"message_id":"message-1","room_id":"room-1","conversation_id":"conversation-1","source_agent_id":"agent-amy","recipients":["agent-devin"],"content":"hello","wake_policy":"none","reply_route":{"mode":"none"},"timestamp":1}}`))
	}))
	defer server.Close()

	item, err := createRoomDirectedMessage(context.Background(), roomMessageCLIOptions{
		roomID:            "room-1",
		conversationID:    "conversation-1",
		sourceAgentID:     "agent-amy",
		recipientAgentIDs: []string{"agent-devin"},
		content:           "hello",
		wakePolicy:        protocol.RoomWakePolicyNone,
		replyRouteMode:    protocol.RoomReplyRouteNone,
		internalAPIBase:   server.URL,
		internalToken:     "token",
	}, "user-1")
	if err != nil {
		t.Fatalf("创建 directed message 失败: %v", err)
	}
	if item.MessageID != "message-1" || item.SourceAgentID != "agent-amy" {
		t.Fatalf("directed message 响应不正确: %+v", item)
	}
	if sourceHeader != "agent-amy" || scopeHeader != "user-1" {
		t.Fatalf("internal headers 不正确: source=%q scope=%q", sourceHeader, scopeHeader)
	}
}

func TestCreateRoomPublicMessageSendsSourceAgentAsInternalHeader(t *testing.T) {
	var sourceHeader string
	var scopeHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourceHeader = r.Header.Get(nexusInternalRoomAgentIDHeader)
		scopeHeader = r.Header.Get(nexusInternalScopeUserIDHeader)
		if !strings.HasSuffix(r.URL.Path, "/internal/rooms/room-1/conversations/conversation-1/public-messages") {
			t.Fatalf("internal endpoint path 不正确: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"message_id":"message-1","room_id":"room-1","conversation_id":"conversation-1","agent_id":"agent-amy","role":"assistant","content":[{"type":"text","text":"hello"}],"timestamp":1}}`))
	}))
	defer server.Close()

	item, err := createRoomPublicMessage(context.Background(), roomPublicMessageCLIOptions{
		roomID:          "room-1",
		conversationID:  "conversation-1",
		sourceAgentID:   "agent-amy",
		content:         "hello",
		internalAPIBase: server.URL,
		internalToken:   "token",
	}, "user-1")
	if err != nil {
		t.Fatalf("创建 public message 失败: %v", err)
	}
	if item["message_id"] != "message-1" || item["agent_id"] != "agent-amy" {
		t.Fatalf("public message 响应不正确: %+v", item)
	}
	if sourceHeader != "agent-amy" || scopeHeader != "user-1" {
		t.Fatalf("internal headers 不正确: source=%q scope=%q", sourceHeader, scopeHeader)
	}
}

func readRoomMessageWebSocketEvent(t *testing.T, conn *websocket.Conn) protocol.EventMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		var event protocol.EventMessage
		if err := wsjson.Read(ctx, conn, &event); err != nil {
			t.Fatalf("读取 room websocket event 失败: %v", err)
		}
		if event.EventType == protocol.EventTypeRoomDirectedMessage {
			return event
		}
	}
}

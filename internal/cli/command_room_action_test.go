package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestRoomActionCommandUsesRuntimeEnvAndInternalEndpoint(t *testing.T) {
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
		"action",
		"private-note",
		"--content",
		"hello",
	)
	if payload["action"] != "room_action_create" {
		t.Fatalf("CLI 输出 action 不正确: %+v", payload)
	}
	item, ok := payload["item"].(map[string]any)
	if !ok {
		t.Fatalf("CLI 输出 item 不正确: %+v", payload)
	}
	if _, exists := item["content"]; exists {
		t.Fatalf("CLI tool_result 不应回显 private content: %+v", item)
	}
	if item["content_chars"] != float64(5) {
		t.Fatalf("CLI 输出应保留内容长度便于调试: %+v", item)
	}
	event := readRoomActionWebSocketEvent(t, conn)
	if event.EventType != protocol.EventTypeRoomAction {
		t.Fatalf("未收到 room_action websocket 事件: %+v", event)
	}
	if event.RoomID != roomContext.Room.ID || event.ConversationID != roomContext.Conversation.ID {
		t.Fatalf("room_action websocket 事件上下文不正确: %+v", event)
	}
	if event.Data["action_type"] != string(protocol.RoomActionTypePrivateNote) {
		t.Fatalf("room_action websocket 事件 action_type 不正确: %+v", event.Data)
	}
	if event.Data["event_kind"] != "created" {
		t.Fatalf("room_action websocket 事件应标记 created: %+v", event.Data)
	}
	if _, ok := event.Data["content"]; ok {
		t.Fatalf("private_message websocket 事件不应泄漏正文: %+v", event.Data)
	}

	actionStore := workspacestore.NewRoomActionStore(cfg.WorkspacePath)
	actions, err := actionStore.ReadActions(roomContext.Conversation.ID)
	if err != nil {
		t.Fatalf("读取 Room action 失败: %v", err)
	}
	if len(actions) != 1 ||
		actions[0].SourceAgentID != amy.AgentID ||
		actions[0].TargetAgentID != amy.AgentID ||
		actions[0].Content != "hello" {
		t.Fatalf("CLI 未通过内部 endpoint 创建 action: %+v", actions)
	}

	payload = runCLICommandWithEnv(
		t,
		cfg,
		map[string]string{nexusctlUserIDEnvName: authsvc.SystemUserID},
		"--json",
		"room",
		"action",
		"request-reply",
		"--target-agent-id",
		devin.AgentID,
		"--wake-policy",
		"none",
		"--content",
		"please-answer-later",
	)
	item, ok = payload["item"].(map[string]any)
	if !ok ||
		item["action_type"] != string(protocol.RoomActionTypeRequestReply) ||
		item["wake_policy"] != string(protocol.RoomWakePolicyNone) ||
		item["reply_target"] != string(protocol.RoomReplyTargetPublicFeed) ||
		item["request_id"] == "" {
		t.Fatalf("CLI request_reply 输出不正确: %+v", payload)
	}
	if _, exists := item["content"]; exists {
		t.Fatalf("CLI request_reply 输出不应回显正文: %+v", item)
	}
	event = readRoomActionWebSocketEvent(t, conn)
	if event.Data["action_type"] != string(protocol.RoomActionTypeRequestReply) ||
		event.Data["event_kind"] != "created" ||
		event.Data["wake_policy"] != string(protocol.RoomWakePolicyNone) ||
		event.Data["request_id"] == "" {
		t.Fatalf("request_reply room_action websocket 事件不正确: %+v", event.Data)
	}

	payload = runCLICommandWithEnv(
		t,
		cfg,
		map[string]string{nexusctlUserIDEnvName: authsvc.SystemUserID},
		"--json",
		"room",
		"action",
		"marker",
		"--reply-target",
		"audience",
		"--audience-agent-id",
		devin.AgentID,
		"--content",
		"audience-only",
	)
	item, ok = payload["item"].(map[string]any)
	if !ok || item["reply_target"] != string(protocol.RoomReplyTargetAudience) {
		t.Fatalf("CLI audience 输出不正确: %+v", payload)
	}
	if _, exists := item["content"]; exists {
		t.Fatalf("CLI audience 输出不应回显正文: %+v", item)
	}
	event = readRoomActionWebSocketEvent(t, conn)
	if event.Data["reply_target"] != string(protocol.RoomReplyTargetAudience) {
		t.Fatalf("audience room_action websocket 事件不正确: %+v", event.Data)
	}

	payload = runCLICommandWithEnv(
		t,
		cfg,
		map[string]string{nexusctlUserIDEnvName: authsvc.SystemUserID},
		"--json",
		"room",
		"action",
		"marker",
		"--visibility",
		"public",
		"--reply-target",
		"none",
		"--content",
		"record-only",
	)
	item, ok = payload["item"].(map[string]any)
	if !ok || item["reply_target"] != string(protocol.RoomReplyTargetNone) {
		t.Fatalf("CLI none 输出不正确: %+v", payload)
	}
	if _, exists := item["content"]; exists {
		t.Fatalf("CLI none 输出不应回显正文: %+v", item)
	}
	event = readRoomActionWebSocketEvent(t, conn)
	if _, exists := event.Data["content"]; exists {
		t.Fatalf("reply_target none websocket 事件不应泄漏正文: %+v", event.Data)
	}

	actions, err = actionStore.ReadActions(roomContext.Conversation.ID)
	if err != nil {
		t.Fatalf("读取 Room action 失败: %v", err)
	}
	if len(actions) != 4 ||
		actions[1].ActionType != protocol.RoomActionTypeRequestReply ||
		actions[1].RequestID == "" ||
		actions[1].WakePolicy != protocol.RoomWakePolicyNone {
		t.Fatalf("CLI request_reply action 未正确落盘: %+v", actions)
	}
	if len(actions) != 4 ||
		actions[2].ReplyTarget != protocol.RoomReplyTargetAudience ||
		len(actions[2].AudienceAgentIDs) != 1 ||
		actions[2].AudienceAgentIDs[0] != devin.AgentID ||
		actions[3].ReplyTarget != protocol.RoomReplyTargetNone {
		t.Fatalf("CLI audience/none action 未正确落盘: %+v", actions)
	}

	listPayload := runCLICommandWithEnv(
		t,
		cfg,
		map[string]string{nexusctlUserIDEnvName: authsvc.SystemUserID},
		"--json",
		"room",
		"action",
		"list",
		"--conversation-id",
		roomContext.Conversation.ID,
	)
	if listPayload["action"] != "room_action_list" ||
		listPayload["conversation_id"] != roomContext.Conversation.ID ||
		listPayload["count"] != float64(4) {
		t.Fatalf("CLI action list 输出不正确: %+v", listPayload)
	}
	listItems, ok := listPayload["items"].([]any)
	if !ok || len(listItems) != 4 {
		t.Fatalf("CLI action list items 不正确: %+v", listPayload)
	}
	if _, exists := asMap(t, listItems[0])["content"]; exists {
		t.Fatalf("CLI action list 默认不应回显正文: %+v", listItems[0])
	}

	devinListPayload := runCLICommandWithEnv(
		t,
		cfg,
		map[string]string{nexusctlUserIDEnvName: authsvc.SystemUserID},
		"--json",
		"room",
		"action",
		"list",
		"--conversation-id",
		roomContext.Conversation.ID,
		"--agent-id",
		devin.AgentID,
		"--include-content",
	)
	if devinListPayload["action"] != "room_action_list" ||
		devinListPayload["agent_id"] != devin.AgentID ||
		devinListPayload["count"] != float64(2) {
		t.Fatalf("CLI agent action list 输出不正确: %+v", devinListPayload)
	}
	devinItems, ok := devinListPayload["items"].([]any)
	if !ok || len(devinItems) != 2 {
		t.Fatalf("CLI agent action list items 不正确: %+v", devinListPayload)
	}
	contents := collectRoomActionContents(t, devinItems)
	if contents["please-answer-later"] != 1 ||
		contents["audience-only"] != 1 ||
		contents["hello"] != 0 ||
		contents["record-only"] != 0 {
		t.Fatalf("CLI agent action list 投影内容不正确: %+v", contents)
	}

	if err = actionStore.AppendActionCursor(workspacestore.RoomActionCursor{
		RoomID:              roomContext.Room.ID,
		ConversationID:      roomContext.Conversation.ID,
		AgentID:             devin.AgentID,
		RoundID:             "round-devin-1",
		LastActionID:        actions[2].ActionID,
		LastActionTimestamp: actions[2].Timestamp,
		Timestamp:           time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("写入 Room action cursor 失败: %v", err)
	}
	cursorPayload := runCLICommandWithEnv(
		t,
		cfg,
		map[string]string{nexusctlUserIDEnvName: authsvc.SystemUserID},
		"--json",
		"room",
		"action",
		"cursors",
		"--conversation-id",
		roomContext.Conversation.ID,
		"--agent-id",
		devin.AgentID,
	)
	if cursorPayload["action"] != "room_action_cursors" ||
		cursorPayload["count"] != float64(1) {
		t.Fatalf("CLI action cursors 输出不正确: %+v", cursorPayload)
	}
	cursorItems, ok := cursorPayload["items"].([]any)
	if !ok || len(cursorItems) != 1 {
		t.Fatalf("CLI action cursors items 不正确: %+v", cursorPayload)
	}
	cursorItem := asMap(t, cursorItems[0])
	if cursorItem["agent_id"] != devin.AgentID ||
		cursorItem["last_action_id"] != actions[2].ActionID ||
		cursorItem["round_id"] != "round-devin-1" {
		t.Fatalf("CLI action cursor 内容不正确: %+v", cursorItem)
	}
}

func TestRoomActionCommandRequiresRoomContext(t *testing.T) {
	t.Setenv(nexusRoomIDEnvName, "")
	t.Setenv(nexusRoomConversationIDEnvName, "")
	t.Setenv(nexusRoomAgentIDEnvName, "")
	t.Setenv(nexusRoomInternalAPIBaseEnvName, "http://127.0.0.1:18032/nexus/v1")
	t.Setenv(nexusRoomInternalTokenEnvName, "test-token")

	errText := runCLICommandError(
		t,
		config.Config{Host: "127.0.0.1", Port: 18032, APIPrefix: "/nexus/v1"},
		map[string]string{nexusctlUserIDEnvName: "user-1"},
		"--json",
		"room",
		"action",
		"private-note",
		"--content",
		"hello",
	)
	if errText == "" {
		t.Fatal("缺少 Room context 时应返回错误")
	}
}

func TestRoomActionCommandDoesNotAcceptSourceAgentFlag(t *testing.T) {
	errText := runCLICommandError(
		t,
		config.Config{Host: "127.0.0.1", Port: 18032, APIPrefix: "/nexus/v1"},
		map[string]string{nexusctlUserIDEnvName: "user-1"},
		"--json",
		"room",
		"action",
		"private-note",
		"--source-agent-id",
		"agent-1",
		"--content",
		"hello",
	)
	if !strings.Contains(errText, "unknown flag: --source-agent-id") {
		t.Fatalf("source_agent_id 不应允许 CLI 手写: %s", errText)
	}
}

func TestRoomActionCommandRequiresInternalEndpoint(t *testing.T) {
	t.Setenv(nexusRoomIDEnvName, "room-1")
	t.Setenv(nexusRoomConversationIDEnvName, "conversation-1")
	t.Setenv(nexusRoomAgentIDEnvName, "agent-1")
	t.Setenv(nexusRoomInternalAPIBaseEnvName, "")
	t.Setenv(nexusRoomInternalTokenEnvName, "")

	errText := runCLICommandError(
		t,
		config.Config{Host: "127.0.0.1", Port: 18032, APIPrefix: "/nexus/v1"},
		map[string]string{nexusctlUserIDEnvName: "user-1"},
		"--json",
		"room",
		"action",
		"private-note",
		"--content",
		"hello",
	)
	if errText == "" {
		t.Fatal("缺少内部 endpoint 时应返回错误")
	}
}

func TestCreateRoomActionSendsSourceAgentAsInternalHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get(nexusInternalRoomAgentIDHeader); got != "agent-amy" {
			t.Fatalf("source agent 应通过内部 header 注入: %q", got)
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("读取请求 body 失败: %v", err)
		}
		var payload protocol.CreateRoomActionRequest
		if err = json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("解析请求 body 失败: %v", err)
		}
		if strings.TrimSpace(payload.SourceAgentID) != "" {
			t.Fatalf("source_agent_id 不应出现在 action JSON body: %+v", payload)
		}
		writer.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(writer).Encode(map[string]any{
			"success": true,
			"data": protocol.RoomActionRecord{
				ActionID:       "action-1",
				RoomID:         "room-1",
				ConversationID: "conversation-1",
				ActionType:     protocol.RoomActionTypePrivateNote,
				SourceAgentID:  "agent-amy",
				Content:        "note",
				Visibility:     protocol.RoomActionVisibilityPrivate,
				ReplyTarget:    protocol.RoomReplyTargetSenderPrivate,
			},
		})
		if err != nil {
			t.Fatalf("写入响应失败: %v", err)
		}
	}))
	defer server.Close()

	item, err := createRoomAction(context.Background(), roomActionCLIOptions{
		actionType:      protocol.RoomActionTypePrivateNote,
		roomID:          "room-1",
		conversationID:  "conversation-1",
		sourceAgentID:   "agent-amy",
		content:         "note",
		internalAPIBase: server.URL,
		internalToken:   "token-1",
	}, "user-1")
	if err != nil {
		t.Fatalf("创建 Room action 失败: %v", err)
	}
	if item.SourceAgentID != "agent-amy" {
		t.Fatalf("响应 source_agent_id 不正确: %+v", item)
	}
}

func readRoomActionWebSocketEvent(t *testing.T, conn *websocket.Conn) protocol.EventMessage {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		var event protocol.EventMessage
		if err := wsjson.Read(ctx, conn, &event); err != nil {
			t.Fatalf("读取 websocket 事件失败: %v", err)
		}
		if event.EventType == protocol.EventTypeRoomAction {
			return event
		}
	}
}

func collectRoomActionContents(t *testing.T, items []any) map[string]int {
	t.Helper()

	result := map[string]int{}
	for _, raw := range items {
		item := asMap(t, raw)
		content, ok := item["content"].(string)
		if !ok {
			t.Fatalf("Room action item 缺少 content: %+v", item)
		}
		result[content]++
	}
	return result
}

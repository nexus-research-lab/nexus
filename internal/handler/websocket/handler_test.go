package websocket_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	goalappserver "github.com/nexus-research-lab/nexus/internal/service/goal/appserver"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestWebSocketSessionBinding(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}
	t.Cleanup(func() { _ = server.Close(context.Background()) })

	httpServer := httptest.NewServer(server.Router())
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/nexus/v1/chat/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn1, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("连接 ws1 失败: %v", err)
	}
	defer func() { _ = conn1.Close(websocket.StatusNormalClosure, "test done") }()

	conn2, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("连接 ws2 失败: %v", err)
	}
	defer func() { _ = conn2.Close(websocket.StatusNormalClosure, "test done") }()

	sessionKey := "agent:nexus:ws:dm:test-session"

	if err = wsjson.Write(ctx, conn1, map[string]any{
		"type":        "bind_session",
		"session_key": sessionKey,
	}); err != nil {
		t.Fatalf("ws1 bind_session 失败: %v", err)
	}
	first := readEventMessage(t, conn1)
	if first.EventType != protocol.EventTypeSessionStatus {
		t.Fatalf("应收到 session_status，实际: %+v", first)
	}
	catalog := readEventMatching(t, conn1, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeCommandCatalog
	})
	if catalog.SessionKey != sessionKey || catalog.AgentID != "nexus" {
		t.Fatalf("command_catalog 作用域错误: %+v", catalog)
	}
	if catalog.Data["status"] != string(protocol.CommandCatalogStatusReady) {
		t.Fatalf("command_catalog status = %#v, want ready", catalog.Data["status"])
	}
	commands, ok := catalog.Data["commands"].([]any)
	if !ok || len(commands) == 0 {
		t.Fatalf("command_catalog commands = %#v, want non-empty static manifest", catalog.Data["commands"])
	}
}

func TestWebSocketExternalIMSessionBindingDoesNotEmitHostCommandsOrStartupError(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}
	t.Cleanup(func() { _ = server.Close(context.Background()) })

	httpServer := httptest.NewServer(server.Router())
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/nexus/v1/chat/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("连接 websocket 失败: %v", err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "test done") }()

	sessionKey := protocol.BuildAgentAccountSessionKey(
		"nexus",
		protocol.SessionChannelWeixinPersonal,
		protocol.RoomTypeDM,
		"account-a",
		"user-a",
		"",
	)
	if err = wsjson.Write(ctx, conn, map[string]any{
		"type":        "bind_session",
		"session_key": sessionKey,
	}); err != nil {
		t.Fatalf("绑定外部 IM session 失败: %v", err)
	}

	for range 8 {
		event := readEventMessage(t, conn)
		if event.EventType == protocol.EventTypeError {
			t.Fatalf("查看外部 IM session 不应投影 Agent 启动错误: %+v", event)
		}
		if event.EventType != protocol.EventTypeCommandCatalog {
			continue
		}
		commands, _ := event.Data["commands"].([]any)
		for _, raw := range commands {
			command, _ := raw.(map[string]any)
			if command["execution"] == string(protocol.CommandExecutionHost) {
				t.Fatalf("外部 IM catalog 不应注入 Web host 命令: %+v", command)
			}
		}
		return
	}
	t.Fatal("外部 IM session 未收到 command_catalog")
}

func TestWebSocketSetGoalUsesDurableControlRecordInsteadOfChatRound(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	cfg.GoalEnabled = true
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}
	t.Cleanup(func() { _ = server.Close(context.Background()) })
	httpServer := httptest.NewServer(server.Router())
	defer httpServer.Close()

	ensureRequest := httptest.NewRequest(
		http.MethodGet,
		"/nexus/v1/rooms/dm/"+cfg.DefaultAgentID,
		nil,
	)
	ensureRecorder := httptest.NewRecorder()
	server.Router().ServeHTTP(ensureRecorder, ensureRequest)
	if ensureRecorder.Code != http.StatusOK {
		t.Fatalf("创建 Goal DM conversation 失败: status=%d body=%s", ensureRecorder.Code, ensureRecorder.Body.String())
	}
	var ensured struct {
		Data protocol.ConversationContextAggregate `json:"data"`
	}
	if err = json.Unmarshal(ensureRecorder.Body.Bytes(), &ensured); err != nil {
		t.Fatalf("解析 Goal DM conversation 失败: %v", err)
	}
	conversationID := ensured.Data.Conversation.ID
	if conversationID == "" {
		t.Fatalf("创建 Goal DM conversation 返回空 ID: %s", ensureRecorder.Body.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(
		ctx,
		"ws"+strings.TrimPrefix(httpServer.URL, "http")+"/nexus/v1/chat/ws",
		nil,
	)
	if err != nil {
		t.Fatalf("连接 websocket 失败: %v", err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "test done") }()

	sessionKey := protocol.BuildRoomAgentSessionKey(
		conversationID,
		cfg.DefaultAgentID,
		protocol.RoomTypeDM,
	)
	if err = wsjson.Write(ctx, conn, map[string]any{
		"type":              "set_goal",
		"session_key":       sessionKey,
		"agent_id":          "nexus",
		"objective":         "Verify dedicated Goal control",
		"client_request_id": "request-set-goal",
		"client_message_id": "client-set-goal",
		"goal_options": map[string]any{
			"replace_existing": true,
			"token_budget":     nil,
		},
	}); err != nil {
		t.Fatalf("发送 set_goal 失败: %v", err)
	}

	var ack protocol.EventMessage
	for range 30 {
		event := readEventMessage(t, conn)
		if event.EventType == protocol.EventTypeError {
			t.Fatalf("set_goal error = %+v", event)
		}
		if event.EventType == protocol.EventTypeChatAck &&
			event.Data["client_request_id"] == "request-set-goal" {
			ack = event
			break
		}
	}
	if ack.EventType != protocol.EventTypeChatAck || ack.Data["user_message_committed"] != true {
		t.Fatalf("set_goal ack = %+v, want durable chat_ack", ack)
	}
	db := handlertest.OpenSQLite(t, cfg.DatabaseURL)
	t.Cleanup(func() { _ = db.Close() })
	var (
		conversationTitle string
		conversationDraft bool
	)
	if err = db.QueryRowContext(
		ctx,
		`SELECT COALESCE(title, ''), is_draft FROM conversations WHERE id = ?`,
		conversationID,
	).Scan(&conversationTitle, &conversationDraft); err != nil {
		t.Fatalf("读取 Goal conversation 标题失败: %v", err)
	}
	if strings.TrimSpace(conversationTitle) == "" || conversationTitle == "新会话" || conversationTitle == "New session" {
		t.Fatalf("conversation title = %q, want Goal intent preview", conversationTitle)
	}
	if conversationDraft {
		t.Fatal("conversation remains draft after durable Goal control")
	}

	ownerHistory := workspacestore.NewAgentHistoryStore(cfg.WorkspacePath).ForOwner(authctx.SystemUserID)
	ownerSessions := workspacestore.NewSessionFileStore(cfg.WorkspacePath).ForOwner(authctx.SystemUserID)
	workspacePath := agentsvc.ResolveWorkspacePath(cfg, authctx.SystemUserID, "nexus")
	sessionItem, _, err := ownerSessions.FindSession([]string{workspacePath}, sessionKey)
	if err != nil || sessionItem == nil {
		t.Fatalf("Goal control session = %#v error=%v", sessionItem, err)
	}
	if sessionItem.MessageCount < 1 || sessionItem.Title == "New Chat" {
		t.Fatalf("session after Goal control = %#v, want started preview", sessionItem)
	}
	rows, err := ownerHistory.ReadMessages(workspacePath, *sessionItem, nil)
	if err != nil {
		t.Fatalf("读取 Goal control history 失败: %v", err)
	}
	found := false
	controlRoundID, _ := ack.Data["round_id"].(string)
	for _, row := range rows {
		metadataSubtype := ""
		switch metadata := row["metadata"].(type) {
		case map[string]string:
			metadataSubtype = metadata["subtype"]
		case map[string]any:
			metadataSubtype, _ = metadata["subtype"].(string)
		}
		if row["role"] == "user" &&
			row["content"] == "/goal Verify dedicated Goal control" &&
			metadataSubtype == "goal_set" &&
			row["control_only"] == true {
			found = true
		}
		if row["role"] == "assistant" && row["round_id"] == controlRoundID {
			t.Fatalf("Goal control round 不应生成 assistant 终态: %#v", row)
		}
	}
	if !found {
		t.Fatalf("Goal control history = %#v, want durable goal_set user item", rows)
	}
}

func TestWebSocketSlashGoalUsesSameDurableControlPath(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	cfg.GoalEnabled = true
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}
	t.Cleanup(func() { _ = server.Close(context.Background()) })
	httpServer := httptest.NewServer(server.Router())
	defer httpServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(
		ctx,
		"ws"+strings.TrimPrefix(httpServer.URL, "http")+"/nexus/v1/chat/ws",
		nil,
	)
	if err != nil {
		t.Fatalf("连接 websocket 失败: %v", err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "test done") }()

	const (
		sessionKey = "agent:nexus:ws:dm:slash-goal-control"
		requestID  = "request-slash-goal"
		content    = "/goal Verify shared Slash Goal control"
	)
	if err = wsjson.Write(ctx, conn, map[string]any{
		"type":              "chat",
		"session_key":       sessionKey,
		"agent_id":          "nexus",
		"content":           content,
		"client_request_id": requestID,
		"client_message_id": "client-slash-goal",
	}); err != nil {
		t.Fatalf("发送 /goal 失败: %v", err)
	}

	var ack protocol.EventMessage
	for range 30 {
		event := readEventMessage(t, conn)
		if event.EventType == protocol.EventTypeError {
			t.Fatalf("/goal error = %+v", event)
		}
		if event.EventType == protocol.EventTypeChatAck &&
			event.Data["client_request_id"] == requestID {
			ack = event
			break
		}
	}
	if ack.Data["user_message_committed"] != true {
		t.Fatalf("/goal ack = %+v, want durable chat_ack", ack)
	}

	ownerHistory := workspacestore.NewAgentHistoryStore(cfg.WorkspacePath).ForOwner(authctx.SystemUserID)
	ownerSessions := workspacestore.NewSessionFileStore(cfg.WorkspacePath).ForOwner(authctx.SystemUserID)
	workspacePath := agentsvc.ResolveWorkspacePath(cfg, authctx.SystemUserID, "nexus")
	sessionItem, _, err := ownerSessions.FindSession([]string{workspacePath}, sessionKey)
	if err != nil || sessionItem == nil {
		t.Fatalf("Slash Goal session = %#v error=%v", sessionItem, err)
	}
	rows, err := ownerHistory.ReadMessages(workspacePath, *sessionItem, nil)
	if err != nil {
		t.Fatalf("读取 Slash Goal history 失败: %v", err)
	}
	controlRoundID, _ := ack.Data["round_id"].(string)
	found := false
	for _, row := range rows {
		if row["role"] == "user" && row["content"] == content &&
			row["control_only"] == true {
			found = true
		}
		if row["role"] == "assistant" && row["round_id"] == controlRoundID {
			t.Fatalf("Slash Goal control round 不应生成 assistant 终态: %#v", row)
		}
	}
	if !found {
		t.Fatalf("Slash Goal history = %#v, want the same durable control record", rows)
	}
}

func TestWebSocketDispatchesRewriteLastToControlHandler(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}
	t.Cleanup(func() { _ = server.Close(context.Background()) })

	httpServer := httptest.NewServer(server.Router())
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/nexus/v1/chat/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("连接 websocket 失败: %v", err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "test done") }()

	if err = wsjson.Write(ctx, conn, map[string]any{
		"type":              "chat_rewrite_last",
		"session_key":       "agent:nexus:ws:dm:rewrite-dispatch",
		"target_round_id":   "round-missing",
		"content":           "新问题",
		"client_request_id": "req-rewrite-dispatch",
		"client_message_id": "local-msg-rewrite-dispatch",
	}); err != nil {
		t.Fatalf("发送 chat_rewrite_last 失败: %v", err)
	}

	for range 5 {
		event := readEventMessage(t, conn)
		if event.EventType != protocol.EventTypeError {
			continue
		}
		if event.Data["type"] != "chat_rewrite_last" {
			t.Fatalf("error data.type = %#v, want chat_rewrite_last", event.Data["type"])
		}
		if event.Data["error_type"] == "unknown_message_type" {
			t.Fatalf("chat_rewrite_last 被顶层 dispatch 当作未知消息: %+v", event)
		}
		return
	}
	t.Fatal("未收到 chat_rewrite_last 的业务错误事件")
}

func TestWebSocketInputQueueAckAndErrorPreserveClientIDs(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}
	t.Cleanup(func() { _ = server.Close(context.Background()) })

	httpServer := httptest.NewServer(server.Router())
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/nexus/v1/chat/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("连接 websocket 失败: %v", err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "test done") }()

	sessionKey := "agent:nexus:ws:dm:input-queue-ack"
	if err = wsjson.Write(ctx, conn, map[string]any{
		"type":              "input_queue",
		"session_key":       sessionKey,
		"action":            "delete",
		"item_id":           "missing-item",
		"client_request_id": "request-input-queue-delete",
		"client_message_id": "message-input-queue-delete",
	}); err != nil {
		t.Fatalf("发送 input_queue delete 失败: %v", err)
	}

	ack := readEventMatching(t, conn, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeInputQueueAck
	})
	if ack.SessionKey != sessionKey ||
		ack.Data["client_request_id"] != "request-input-queue-delete" ||
		ack.Data["client_message_id"] != "message-input-queue-delete" ||
		ack.Data["action"] != "delete" ||
		ack.Data["item_id"] != "missing-item" {
		t.Fatalf("input_queue_ack 未完整回显请求身份: %+v", ack)
	}

	if err = wsjson.Write(ctx, conn, map[string]any{
		"type":              "input_queue",
		"session_key":       sessionKey,
		"action":            "enqueue",
		"content":           " ",
		"client_request_id": "request-input-queue-error",
		"client_message_id": "message-input-queue-error",
	}); err != nil {
		t.Fatalf("发送非法 input_queue enqueue 失败: %v", err)
	}

	failure := readEventMatching(t, conn, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeError &&
			event.Data["error_type"] == "input_queue_error" &&
			event.Data["client_request_id"] == "request-input-queue-error"
	})
	if failure.Data["client_message_id"] != "message-input-queue-error" ||
		failure.Data["action"] != "enqueue" ||
		failure.Data["item_id"] != "" {
		t.Fatalf("input_queue_error 未完整回显请求身份: %+v", failure)
	}
}

func TestWebSocketRoomInterruptAckPreservesExactTarget(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}
	t.Cleanup(func() { _ = server.Close(context.Background()) })

	httpServer := httptest.NewServer(server.Router())
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/nexus/v1/chat/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("连接 websocket 失败: %v", err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "test done") }()

	const (
		sessionKey      = "room:group:conversation-interrupt-ack"
		clientRequestID = "request-interrupt-agent-a"
		roundID         = "round-interrupt-agent-a"
		agentRoundID    = "agent-round-interrupt-agent-a"
	)
	if err = wsjson.Write(ctx, conn, map[string]any{
		"type":              "interrupt",
		"session_key":       sessionKey,
		"round_id":          roundID,
		"agent_round_id":    agentRoundID,
		"client_request_id": clientRequestID,
	}); err != nil {
		t.Fatalf("发送精确 Room interrupt 失败: %v", err)
	}

	ack := readEventMatching(t, conn, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeInterruptAck
	})
	if ack.SessionKey != sessionKey ||
		ack.RoundID != roundID ||
		ack.AgentRoundID != agentRoundID ||
		ack.Data["accepted"] != true ||
		ack.Data["client_request_id"] != clientRequestID ||
		ack.Data["round_id"] != roundID ||
		ack.Data["agent_round_id"] != agentRoundID {
		t.Fatalf("interrupt_ack 未完整回显精确目标: %+v", ack)
	}
}

func TestWebSocketDesktopSessionToken(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	cfg.DesktopSessionToken = "desktop-token"
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}
	t.Cleanup(func() { _ = server.Close(context.Background()) })

	httpServer := httptest.NewServer(server.Router())
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/nexus/v1/chat/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, response, err := websocket.Dial(ctx, wsURL, nil)
	if err == nil {
		_ = conn.Close(websocket.StatusNormalClosure, "unexpected success")
		t.Fatal("缺少桌面 token 的 websocket 不应连接成功")
	}
	if response == nil || response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("缺少桌面 token 应返回 401，response=%v err=%v", response, err)
	}

	conn, _, err = websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		Subprotocols: []string{"nexus.desktop.v1", "nexus.desktop.token.desktop-token"},
	})
	if err != nil {
		t.Fatalf("带桌面 token 的 websocket 应连接成功: %v", err)
	}
	_ = conn.Close(websocket.StatusNormalClosure, "test done")
}

func TestWebSocketAppServerGoalRPC(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	cfg.GoalEnabled = true
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}
	t.Cleanup(func() { _ = server.Close(context.Background()) })

	httpServer := httptest.NewServer(server.Router())
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/nexus/v1/chat/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("连接 websocket 失败: %v", err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "test done") }()

	threadID := "agent:nexus:ws:dm:goal-rpc"
	if err := wsjson.Write(ctx, conn, map[string]any{
		"id":     1,
		"method": "thread/goal/set",
		"params": map[string]any{
			"threadId":    threadID,
			"objective":   "Ship app-server RPC parity",
			"status":      "paused",
			"tokenBudget": 200,
		},
	}); err != nil {
		t.Fatalf("发送 thread/goal/set 失败: %v", err)
	}
	setResponse := readRPCResponse[goalappserver.ThreadGoalSetResponse](t, conn)
	if setResponse.Goal.ThreadID != threadID ||
		setResponse.Goal.Status != goalappserver.ThreadGoalStatusPaused ||
		setResponse.Goal.TokenBudget == nil ||
		*setResponse.Goal.TokenBudget != 200 {
		t.Fatalf("thread/goal/set response = %#v", setResponse)
	}
	updated := readRPCNotification[goalappserver.ThreadGoalUpdatedNotification](t, conn)
	if updated.Method != "thread/goal/updated" || updated.Params.Goal.Objective != "Ship app-server RPC parity" {
		t.Fatalf("thread/goal/updated notification = %#v", updated)
	}

	if err := wsjson.Write(ctx, conn, map[string]any{
		"id":     "get-goal",
		"method": "thread/goal/get",
		"params": map[string]any{"threadId": threadID},
	}); err != nil {
		t.Fatalf("发送 thread/goal/get 失败: %v", err)
	}
	getResponse := readRPCResponse[goalappserver.ThreadGoalGetResponse](t, conn)
	if getResponse.Goal == nil || getResponse.Goal.Status != goalappserver.ThreadGoalStatusPaused {
		t.Fatalf("thread/goal/get response = %#v", getResponse)
	}

	if err := wsjson.Write(ctx, conn, map[string]any{
		"id":     3,
		"method": "thread/goal/clear",
		"params": map[string]any{"threadId": threadID},
	}); err != nil {
		t.Fatalf("发送 thread/goal/clear 失败: %v", err)
	}
	clearResponse := readRPCResponse[goalappserver.ThreadGoalClearResponse](t, conn)
	if !clearResponse.Cleared {
		t.Fatalf("thread/goal/clear response = %#v, want cleared", clearResponse)
	}
	cleared := readRPCNotification[goalappserver.ThreadGoalClearedNotification](t, conn)
	if cleared.Method != "thread/goal/cleared" || cleared.Params.ThreadID != threadID {
		t.Fatalf("thread/goal/cleared notification = %#v", cleared)
	}
}

func TestWebSocketAppServerGoalSetCompleteClearsCurrentGoal(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	cfg.GoalEnabled = true
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}
	t.Cleanup(func() { _ = server.Close(context.Background()) })

	httpServer := httptest.NewServer(server.Router())
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/nexus/v1/chat/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("连接 websocket 失败: %v", err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "test done") }()

	threadID := "agent:nexus:ws:dm:goal-rpc-complete"
	if err := wsjson.Write(ctx, conn, map[string]any{
		"id":     1,
		"method": "thread/goal/set",
		"params": map[string]any{
			"threadId":  threadID,
			"objective": "Finish app-server RPC parity",
			"status":    "paused",
		},
	}); err != nil {
		t.Fatalf("发送 thread/goal/set 失败: %v", err)
	}
	setResponse := readRPCResponse[goalappserver.ThreadGoalSetResponse](t, conn)
	if setResponse.Goal.Status != goalappserver.ThreadGoalStatusPaused {
		t.Fatalf("initial thread/goal/set response = %#v, want paused", setResponse)
	}
	updated := readRPCNotification[goalappserver.ThreadGoalUpdatedNotification](t, conn)
	if updated.Method != "thread/goal/updated" {
		t.Fatalf("initial notification = %#v, want thread/goal/updated", updated)
	}

	if err := wsjson.Write(ctx, conn, map[string]any{
		"id":     2,
		"method": "thread/goal/set",
		"params": map[string]any{
			"threadId": threadID,
			"status":   "complete",
		},
	}); err != nil {
		t.Fatalf("发送 complete thread/goal/set 失败: %v", err)
	}
	completeResponse := readRPCResponse[goalappserver.ThreadGoalSetResponse](t, conn)
	if completeResponse.Goal.Status != goalappserver.ThreadGoalStatusComplete {
		t.Fatalf("complete response = %#v, want complete", completeResponse)
	}
	cleared := readRPCNotification[goalappserver.ThreadGoalClearedNotification](t, conn)
	if cleared.Method != "thread/goal/cleared" || cleared.Params.ThreadID != threadID {
		t.Fatalf("complete notification = %#v, want thread/goal/cleared", cleared)
	}

	if err := wsjson.Write(ctx, conn, map[string]any{
		"id":     "get-after-complete",
		"method": "thread/goal/get",
		"params": map[string]any{"threadId": threadID},
	}); err != nil {
		t.Fatalf("发送 thread/goal/get 失败: %v", err)
	}
	getResponse := readRPCResponse[goalappserver.ThreadGoalGetResponse](t, conn)
	if getResponse.Goal != nil {
		t.Fatalf("thread/goal/get after complete = %#v, want nil current goal", getResponse)
	}
}

func readEventMessage(t *testing.T, conn *websocket.Conn) protocol.EventMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var event protocol.EventMessage
	if err := wsjson.Read(ctx, conn, &event); err != nil {
		t.Fatalf("读取 websocket 事件失败: %v", err)
	}
	return event
}

func readEventMatching(
	t *testing.T,
	conn *websocket.Conn,
	matches func(protocol.EventMessage) bool,
) protocol.EventMessage {
	t.Helper()
	for range 12 {
		event := readEventMessage(t, conn)
		if matches(event) {
			return event
		}
	}
	t.Fatal("未收到匹配的 websocket 事件")
	return protocol.EventMessage{}
}

type rpcResponseEnvelope struct {
	Result json.RawMessage                      `json:"result"`
	Error  *goalappserver.AppServerRPCErrorBody `json:"error,omitempty"`
}

func readRPCResponse[T any](t *testing.T, conn *websocket.Conn) T {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var envelope rpcResponseEnvelope
	if err := wsjson.Read(ctx, conn, &envelope); err != nil {
		t.Fatalf("读取 RPC response 失败: %v", err)
	}
	if envelope.Error != nil {
		t.Fatalf("RPC response error = %+v", *envelope.Error)
	}
	var result T
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		t.Fatalf("解析 RPC result 失败: %v raw=%s", err, string(envelope.Result))
	}
	return result
}

type rpcNotificationEnvelope[T any] struct {
	Method string `json:"method"`
	Params T      `json:"params"`
}

func readRPCNotification[T any](t *testing.T, conn *websocket.Conn) rpcNotificationEnvelope[T] {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var notification rpcNotificationEnvelope[T]
	if err := wsjson.Read(ctx, conn, &notification); err != nil {
		t.Fatalf("读取 RPC notification 失败: %v", err)
	}
	return notification
}

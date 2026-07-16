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
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalappserver "github.com/nexus-research-lab/nexus/internal/service/goal/appserver"

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
}

func TestWebSocketDispatchesRewriteLastToControlHandler(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}

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

func TestWebSocketDesktopSessionToken(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	cfg.DesktopSessionToken = "desktop-token"
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}

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

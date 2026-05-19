package websocket_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/protocol"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestWebSocketSessionBindingAndControl(t *testing.T) {
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
		"type":            "bind_session",
		"session_key":     sessionKey,
		"client_id":       "client-1",
		"request_control": true,
	}); err != nil {
		t.Fatalf("ws1 bind_session 失败: %v", err)
	}
	first := readEventMessage(t, conn1)
	if first.EventType != protocol.EventTypeSessionStatus {
		t.Fatalf("应收到 session_status，实际: %+v", first)
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

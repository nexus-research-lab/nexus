// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：websocket_test.go
// @Date   ：2026/04/11 01:08:00
// @Author ：leemysw
// 2026/04/11 01:08:00   Create
// =====================================================

package gateway

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	agentsvc "github.com/nexus-research-lab/nexus/internal/agent"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	room2 "github.com/nexus-research-lab/nexus/internal/room"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

func TestWebSocketSessionBindingAndControl(t *testing.T) {
	cfg := newGatewayTestConfig(t)
	migrateGatewaySQLite(t, cfg.DatabaseURL)

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("创建 gateway 失败: %v", err)
	}

	httpServer := httptest.NewServer(server.Router())
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/agent/v1/chat/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn1, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("连接 ws1 失败: %v", err)
	}
	defer conn1.Close(websocket.StatusNormalClosure, "test done")

	conn2, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("连接 ws2 失败: %v", err)
	}
	defer conn2.Close(websocket.StatusNormalClosure, "test done")

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
	assertSessionStatus(t, first, sessionKey, "client-1", 1, 0)

	if err = wsjson.Write(ctx, conn2, map[string]any{
		"type":            "bind_session",
		"session_key":     sessionKey,
		"client_id":       "client-2",
		"request_control": false,
	}); err != nil {
		t.Fatalf("ws2 bind_session 失败: %v", err)
	}

	secondA := readEventMessage(t, conn1)
	secondB := readEventMessage(t, conn2)
	assertSessionStatus(t, secondA, sessionKey, "client-1", 2, 1)
	assertSessionStatus(t, secondB, sessionKey, "client-1", 2, 1)

	if err = wsjson.Write(ctx, conn2, map[string]any{
		"type":        "chat",
		"session_key": sessionKey,
		"content":     "hello",
	}); err != nil {
		t.Fatalf("观察者发送 chat 失败: %v", err)
	}
	observerError := readEventMessage(t, conn2)
	if observerError.EventType != protocol.EventTypeError {
		t.Fatalf("观察者应收到 error 事件: %+v", observerError)
	}
	if observerError.Data["error_type"] != "session_control_denied" {
		t.Fatalf("错误类型不正确: %+v", observerError.Data)
	}

	if err = wsjson.Write(ctx, conn1, map[string]any{
		"type":        "unbind_session",
		"session_key": sessionKey,
		"client_id":   "client-1",
	}); err != nil {
		t.Fatalf("ws1 unbind_session 失败: %v", err)
	}
	third := readEventMessage(t, conn2)
	assertSessionStatus(t, third, sessionKey, "client-2", 1, 0)
}

type fakeGatewayRoomClient struct {
	mu             sync.Mutex
	messages       chan sdkprotocol.ReceivedMessage
	interruptCalls int
}

func newFakeGatewayRoomClient() *fakeGatewayRoomClient {
	return &fakeGatewayRoomClient{
		messages: make(chan sdkprotocol.ReceivedMessage, 8),
	}
}

func (c *fakeGatewayRoomClient) Connect(context.Context) error { return nil }

func (c *fakeGatewayRoomClient) Query(ctx context.Context, _ string) error {
	<-ctx.Done()
	return ctx.Err()
}

func (c *fakeGatewayRoomClient) ReceiveMessages(context.Context) <-chan sdkprotocol.ReceivedMessage {
	return c.messages
}

func (c *fakeGatewayRoomClient) Interrupt(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.interruptCalls++
	return nil
}

func (c *fakeGatewayRoomClient) Disconnect(context.Context) error { return nil }
func (c *fakeGatewayRoomClient) Reconfigure(context.Context, agentclient.Options) error {
	return nil
}
func (c *fakeGatewayRoomClient) SetPermissionMode(context.Context, sdkprotocol.PermissionMode) error {
	return nil
}
func (c *fakeGatewayRoomClient) SessionID() string { return "gateway-room-session" }

type fakeGatewayRoomFactory struct {
	client *fakeGatewayRoomClient
}

func (f fakeGatewayRoomFactory) New(agentclient.Options) runtimectx.Client {
	return f.client
}

func TestWebSocketSubscribeRoomRestoresPendingSlots(t *testing.T) {
	cfg := newGatewayTestConfig(t)
	migrateGatewaySQLite(t, cfg.DatabaseURL)

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("创建 gateway 失败: %v", err)
	}

	ctx := context.Background()
	agentValue, err := server.agentService.CreateAgent(ctx, agentsvc.CreateRequest{Name: "房间恢复助手"})
	if err != nil {
		t.Fatalf("创建测试 agent 失败: %v", err)
	}
	dmContext, err := server.roomService.EnsureDirectRoom(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("创建直聊 room 失败: %v", err)
	}

	fakeClient := newFakeGatewayRoomClient()
	server.roomRealtime = room2.NewRealtimeServiceWithFactory(
		cfg,
		server.roomService,
		server.agentService,
		server.runtime,
		server.permission,
		fakeGatewayRoomFactory{client: fakeClient},
	)
	server.roomRealtime.SetRoomBroadcaster(server.roomSubs)

	roomSessionKey := protocol.BuildRoomSharedSessionKey(dmContext.Conversation.ID)
	if err = server.roomRealtime.HandleChat(ctx, room2.ChatRequest{
		SessionKey:     roomSessionKey,
		RoomID:         dmContext.Room.ID,
		ConversationID: dmContext.Conversation.ID,
		Content:        "你好",
		RoundID:        "room-round-restore",
		ReqID:          "room-round-restore",
	}); err != nil {
		t.Fatalf("预热 Room round 失败: %v", err)
	}

	httpServer := httptest.NewServer(server.Router())
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/agent/v1/chat/ws"
	wsCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(wsCtx, wsURL, nil)
	if err != nil {
		t.Fatalf("连接 ws 失败: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	if err = wsjson.Write(wsCtx, conn, map[string]any{
		"type":            "bind_session",
		"session_key":     roomSessionKey,
		"client_id":       "client-room",
		"request_control": true,
	}); err != nil {
		t.Fatalf("bind_session 失败: %v", err)
	}
	statusEvent := readEventMessage(t, conn)
	assertSessionStatus(t, statusEvent, roomSessionKey, "client-room", 1, 0)

	if err = wsjson.Write(wsCtx, conn, map[string]any{
		"type":            "subscribe_room",
		"room_id":         dmContext.Room.ID,
		"conversation_id": dmContext.Conversation.ID,
	}); err != nil {
		t.Fatalf("subscribe_room 失败: %v", err)
	}

	event := readEventMessage(t, conn)
	if event.EventType != protocol.EventTypeChatAck {
		t.Fatalf("期望恢复 chat_ack，实际: %+v", event)
	}
	if event.CausedBy != "room-round-restore" {
		t.Fatalf("恢复 round_id 不正确: %+v", event)
	}
	pending, ok := event.Data["pending"].([]any)
	if !ok || len(pending) != 1 {
		t.Fatalf("恢复 pending slot 不正确: %+v", event.Data)
	}

	if err = server.roomRealtime.HandleInterrupt(ctx, room2.InterruptRequest{SessionKey: roomSessionKey}); err != nil {
		t.Fatalf("清理运行中 Room 失败: %v", err)
	}
}

func TestWebSocketRoomMutationBroadcastsEvents(t *testing.T) {
	cfg := newGatewayTestConfig(t)
	migrateGatewaySQLite(t, cfg.DatabaseURL)

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("创建 gateway 失败: %v", err)
	}

	ctx := context.Background()
	agentA, err := server.agentService.CreateAgent(ctx, agentsvc.CreateRequest{Name: "广播助手A"})
	if err != nil {
		t.Fatalf("创建 agentA 失败: %v", err)
	}
	agentB, err := server.agentService.CreateAgent(ctx, agentsvc.CreateRequest{Name: "广播助手B"})
	if err != nil {
		t.Fatalf("创建 agentB 失败: %v", err)
	}
	agentC, err := server.agentService.CreateAgent(ctx, agentsvc.CreateRequest{Name: "广播助手C"})
	if err != nil {
		t.Fatalf("创建 agentC 失败: %v", err)
	}
	roomContext, err := server.roomService.CreateRoom(ctx, room2.CreateRoomRequest{
		AgentIDs: []string{agentA.AgentID, agentB.AgentID},
		Name:     "广播测试 room",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	httpServer := httptest.NewServer(server.Router())
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/agent/v1/chat/ws"
	wsCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(wsCtx, wsURL, nil)
	if err != nil {
		t.Fatalf("连接 ws 失败: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	if err = wsjson.Write(wsCtx, conn, map[string]any{
		"type":            "subscribe_room",
		"room_id":         roomContext.Room.ID,
		"conversation_id": roomContext.Conversation.ID,
	}); err != nil {
		t.Fatalf("subscribe_room 失败: %v", err)
	}

	doGatewayJSONRequest(t, httpServer.URL+"/agent/v1/rooms/"+roomContext.Room.ID, http.MethodPatch, map[string]any{
		"name":   "广播测试 room v2",
		"avatar": "9",
	})
	resyncEvent := readEventMessage(t, conn)
	if resyncEvent.EventType != protocol.EventTypeRoomResyncRequired {
		t.Fatalf("期望 room_resync_required，实际: %+v", resyncEvent)
	}
	if resyncEvent.Data["reason"] != "room_updated" {
		t.Fatalf("room_resync_required reason 不正确: %+v", resyncEvent.Data)
	}
	updatedRoom, err := server.roomService.GetRoom(ctx, roomContext.Room.ID)
	if err != nil {
		t.Fatalf("读取更新后的 room 失败: %v", err)
	}
	if updatedRoom.Room.Avatar != "9" {
		t.Fatalf("room avatar 未持久化: got=%q want=%q", updatedRoom.Room.Avatar, "9")
	}

	doGatewayJSONRequest(t, httpServer.URL+"/agent/v1/rooms/"+roomContext.Room.ID+"/members", http.MethodPost, map[string]any{
		"agent_id": agentC.AgentID,
	})
	addedEvent := readEventMessage(t, conn)
	if addedEvent.EventType != protocol.EventTypeRoomMemberAdded {
		t.Fatalf("期望 room_member_added，实际: %+v", addedEvent)
	}
	if addedEvent.Data["agent_id"] != agentC.AgentID {
		t.Fatalf("新增成员事件 agent_id 不正确: %+v", addedEvent.Data)
	}

	doGatewayJSONRequest(t, httpServer.URL+"/agent/v1/rooms/"+roomContext.Room.ID+"/members/"+agentC.AgentID, http.MethodDelete, nil)
	removedEvent := readEventMessage(t, conn)
	if removedEvent.EventType != protocol.EventTypeRoomMemberRemoved {
		t.Fatalf("期望 room_member_removed，实际: %+v", removedEvent)
	}
	if removedEvent.Data["agent_id"] != agentC.AgentID {
		t.Fatalf("移除成员事件 agent_id 不正确: %+v", removedEvent.Data)
	}

	doGatewayJSONRequest(t, httpServer.URL+"/agent/v1/rooms/"+roomContext.Room.ID, http.MethodDelete, nil)
	deletedEvent := readEventMessage(t, conn)
	if deletedEvent.EventType != protocol.EventTypeRoomDeleted {
		t.Fatalf("期望 room_deleted，实际: %+v", deletedEvent)
	}
	if deletedEvent.Data["room_id"] != roomContext.Room.ID {
		t.Fatalf("room_deleted room_id 不正确: %+v", deletedEvent.Data)
	}
}

func TestWebSocketSubscribeWorkspaceBroadcastsLiveEvents(t *testing.T) {
	cfg := newGatewayTestConfig(t)
	migrateGatewaySQLite(t, cfg.DatabaseURL)

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("创建 gateway 失败: %v", err)
	}

	ctx := context.Background()
	agentValue, err := server.agentService.CreateAgent(ctx, agentsvc.CreateRequest{Name: "工作区广播助手"})
	if err != nil {
		t.Fatalf("创建测试 agent 失败: %v", err)
	}

	httpServer := httptest.NewServer(server.Router())
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/agent/v1/chat/ws"
	wsCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(wsCtx, wsURL, nil)
	if err != nil {
		t.Fatalf("连接 ws 失败: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	if err = wsjson.Write(wsCtx, conn, map[string]any{
		"type":     "subscribe_workspace",
		"agent_id": agentValue.AgentID,
	}); err != nil {
		t.Fatalf("subscribe_workspace 失败: %v", err)
	}

	runtimeEvent := readEventMessageUntil(t, conn, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeAgentRuntimeEvent &&
			event.AgentID == agentValue.AgentID
	})
	if runtimeEvent.Data["status"] != "idle" {
		t.Fatalf("初始 agent_runtime_event 状态不正确: %+v", runtimeEvent.Data)
	}

	if _, err = server.workspace.UpdateFile(ctx, agentValue.AgentID, "notes/live.md", "hello websocket"); err != nil {
		t.Fatalf("更新 workspace 文件失败: %v", err)
	}

	workspaceEvent := readEventMessageUntil(t, conn, func(event protocol.EventMessage) bool {
		if event.EventType != protocol.EventTypeWorkspaceEvent || event.AgentID != agentValue.AgentID {
			return false
		}
		return event.Data["type"] == "file_write_end" && event.Data["path"] == "notes/live.md"
	})
	if workspaceEvent.Data["source"] != "api" {
		t.Fatalf("workspace_event source 不正确: %+v", workspaceEvent.Data)
	}
	if workspaceEvent.Data["content_snapshot"] != "hello websocket" {
		t.Fatalf("workspace_event 内容不正确: %+v", workspaceEvent.Data)
	}
}

func assertSessionStatus(t *testing.T, event protocol.EventMessage, sessionKey string, controllerClientID string, boundClientCount int, observerCount int) {
	t.Helper()
	if event.EventType != protocol.EventTypeSessionStatus {
		t.Fatalf("期望 session_status，实际: %+v", event)
	}
	if event.SessionKey != sessionKey {
		t.Fatalf("session_key 不匹配: got=%s want=%s", event.SessionKey, sessionKey)
	}
	if event.Data["controller_client_id"] != controllerClientID {
		t.Fatalf("controller_client_id 不正确: %+v", event.Data)
	}
	if event.Data["bound_client_count"] != float64(boundClientCount) && event.Data["bound_client_count"] != boundClientCount {
		t.Fatalf("bound_client_count 不正确: %+v", event.Data)
	}
	if event.Data["observer_count"] != float64(observerCount) && event.Data["observer_count"] != observerCount {
		t.Fatalf("observer_count 不正确: %+v", event.Data)
	}
}

func readEventMessage(t *testing.T, conn *websocket.Conn) protocol.EventMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var event protocol.EventMessage
	if err := wsjson.Read(ctx, conn, &event); err != nil {
		t.Fatalf("读取 ws 事件失败: %v", err)
	}
	return event
}

func readEventMessageUntil(
	t *testing.T,
	conn *websocket.Conn,
	match func(protocol.EventMessage) bool,
) protocol.EventMessage {
	t.Helper()

	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case <-timeout.C:
			t.Fatal("等待目标 ws 事件超时")
		default:
		}

		event := readEventMessage(t, conn)
		if match(event) {
			return event
		}
	}
}

func doGatewayJSONRequest(t *testing.T, url string, method string, payload any) {
	t.Helper()

	var body *bytes.Reader
	if payload == nil {
		body = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("编码请求体失败: %v", err)
		}
		body = bytes.NewReader(raw)
	}

	request, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("创建 HTTP 请求失败: %v", err)
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("执行 HTTP 请求失败: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("HTTP 状态码不正确: method=%s url=%s status=%d", method, url, response.StatusCode)
	}
}

func newGatewayTestConfig(t *testing.T) config.Config {
	t.Helper()

	root := t.TempDir()
	t.Setenv("HOME", root)
	return config.Config{
		Host:           "127.0.0.1",
		Port:           18031,
		ProjectName:    "nexus-gateway-test",
		APIPrefix:      "/agent/v1",
		WebSocketPath:  "/agent/v1/chat/ws",
		DefaultAgentID: "nexus",
		WorkspacePath:  filepath.Join(root, "workspace"),
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "nexus.db"),
	}
}

func migrateGatewaySQLite(t *testing.T, databaseURL string) {
	t.Helper()

	db, err := sql.Open("sqlite3", databaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer db.Close()

	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err = goose.Up(db, gatewayMigrationDir(t)); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}
}

func gatewayMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "db", "migrations", "sqlite")
}

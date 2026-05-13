package cli

import (
	"context"
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
		"private-message",
		"--target-agent-id",
		devin.AgentID,
		"--content",
		"hello",
	)
	if payload["action"] != "room_action_create" {
		t.Fatalf("CLI 输出 action 不正确: %+v", payload)
	}
	event := readRoomActionWebSocketEvent(t, conn)
	if event.EventType != protocol.EventTypeRoomAction {
		t.Fatalf("未收到 room_action websocket 事件: %+v", event)
	}
	if event.RoomID != roomContext.Room.ID || event.ConversationID != roomContext.Conversation.ID {
		t.Fatalf("room_action websocket 事件上下文不正确: %+v", event)
	}
	if event.Data["action_type"] != string(protocol.RoomActionTypePrivateMessage) {
		t.Fatalf("room_action websocket 事件 action_type 不正确: %+v", event.Data)
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
		actions[0].TargetAgentID != devin.AgentID ||
		actions[0].Content != "hello" {
		t.Fatalf("CLI 未通过内部 endpoint 创建 action: %+v", actions)
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

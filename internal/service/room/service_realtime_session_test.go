package room_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	_ "modernc.org/sqlite"
)

func TestRealtimeServiceMCPBuilderUsesSharedRoomSessionContext(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	agentValue := createTestAgent(t, agentService, ctx, "助手甲")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID},
		Name:     "定时任务测试房间",
		Title:    "定时任务对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, _ string) error {
		go sendFakeTerminalAssistantAndClose(client, "assistant-mcp-context", "ok", nil)
		return nil
	}

	permission := permissionctx.NewContext()
	service := NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{client}},
	)

	type builderCall struct {
		agentID            string
		sessionKey         string
		roundID            string
		sourceContextType  string
		sourceContextID    string
		sourceContextLabel string
	}
	calls := make(chan builderCall, 1)
	service.SetMCPServerBuilder(func(agentID string, sessionKey string, roundID string, sourceContextType string, sourceContextID string, sourceContextLabel string) map[string]sdkmcp.ServerConfig {
		calls <- builderCall{
			agentID:            agentID,
			sessionKey:         sessionKey,
			roundID:            roundID,
			sourceContextType:  sourceContextType,
			sourceContextID:    sourceContextID,
			sourceContextLabel: sourceContextLabel,
		}
		return nil
	})

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-mcp-context")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@助手甲 每天 9 点检查新闻并发回这个房间",
		RoundID:        "room-round-mcp-context",
		ReqID:          "room-round-mcp-context",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	var call builderCall
	select {
	case call = <-calls:
	case <-time.After(3 * time.Second):
		t.Fatal("等待 Room MCP builder 调用超时")
	}
	if call.agentID != agentValue.AgentID {
		t.Fatalf("MCP builder agentID 不正确: %+v", call)
	}
	if call.sessionKey != sharedSessionKey {
		t.Fatalf("Room MCP 上下文应使用共享 session key，实际 %+v", call)
	}
	if call.roundID != "room-round-mcp-context" {
		t.Fatalf("Room MCP 上下文 roundID 不正确: %+v", call)
	}
	if call.sourceContextType != "room" ||
		call.sourceContextID != roomContext.Room.ID ||
		call.sourceContextLabel != roomContext.Room.Name {
		t.Fatalf("Room MCP 来源上下文不正确: %+v", call)
	}

	_ = collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})
}

func TestRealtimeServiceUsesAndPersistsRoomSDKSessionID(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	agentValue := createTestAgent(t, agentService, ctx, "助手甲")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID},
		Name:     "Resume 房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	if len(roomContext.Sessions) != 1 {
		t.Fatalf("期望只有一个 room session: %+v", roomContext.Sessions)
	}

	db, err = sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer db.Close()

	resumeID := "11111111-1111-1111-1111-111111111111"
	if _, err = db.Exec(`UPDATE sessions SET sdk_session_id = ? WHERE id = ?`, resumeID, roomContext.Sessions[0].ID); err != nil {
		t.Fatalf("预写入 room sdk_session_id 失败: %v", err)
	}
	writeRoomTranscriptFixture(t, agentValue.WorkspacePath, resumeID, []map[string]any{
		{
			"type":       "result",
			"session_id": resumeID,
			"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
		},
	})

	client := newFakeRoomClient()
	client.sessionID = "22222222-2222-2222-2222-222222222222"
	writeRoomTranscriptFixture(t, agentValue.WorkspacePath, client.sessionID, []map[string]any{
		{
			"type":       "result",
			"session_id": client.sessionID,
			"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
		},
	})
	client.onQuery = func(_ context.Context, _ string) error {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "room-result-resume",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
		return nil
	}

	permission := permissionctx.NewContext()
	factory := &fakeRoomFactory{clients: []*fakeRoomClient{client}}
	runtimeManager := runtimectx.NewManager()
	service := NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		factory,
	)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-resume-sender")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "测试 room resume",
		RoundID:        "room-round-resume",
		ReqID:          "room-round-resume",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.Session.ResumeID != resumeID {
		t.Fatalf("room runtime 未将房间 sdk_session_id 作为 resume 透传: %+v", options)
	}

	updatedContext, err := roomService.GetConversationContext(ctx, roomContext.Conversation.ID)
	if err != nil {
		t.Fatalf("读取更新后的 room context 失败: %v", err)
	}
	if len(updatedContext.Sessions) != 1 {
		t.Fatalf("更新后的 room session 数量不正确: %+v", updatedContext.Sessions)
	}
	if updatedContext.Sessions[0].SDKSessionID != client.sessionID {
		t.Fatalf("room sdk_session_id 未回写数据库: %+v", updatedContext.Sessions[0])
	}
}

func TestRealtimeServiceSkipsRoomSDKSessionIDWhenTranscriptMissing(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	agentValue := createTestAgent(t, agentService, ctx, "跳过恢复助手")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID},
		Name:     "缺失 Transcript 房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	if len(roomContext.Sessions) != 1 {
		t.Fatalf("期望只有一个 room session: %+v", roomContext.Sessions)
	}

	db, err = sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer db.Close()

	missingResumeID := "33333333-3333-3333-3333-333333333333"
	if _, err = db.Exec(`UPDATE sessions SET sdk_session_id = ? WHERE id = ?`, missingResumeID, roomContext.Sessions[0].ID); err != nil {
		t.Fatalf("预写入 room sdk_session_id 失败: %v", err)
	}

	client := newFakeRoomClient()
	client.sessionID = "44444444-4444-4444-4444-444444444444"
	writeRoomTranscriptFixture(t, agentValue.WorkspacePath, client.sessionID, []map[string]any{
		{
			"type":       "result",
			"session_id": client.sessionID,
			"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
		},
	})
	client.onQuery = func(_ context.Context, _ string) error {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "room-result-missing-transcript",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
		return nil
	}

	permission := permissionctx.NewContext()
	factory := &fakeRoomFactory{clients: []*fakeRoomClient{client}}
	runtimeManager := runtimectx.NewManager()
	service := NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		factory,
	)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-missing-transcript-sender")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "测试缺失 transcript 时跳过 room resume",
		RoundID:        "room-round-missing-transcript",
		ReqID:          "room-round-missing-transcript",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.Options()
	if len(options) != 1 {
		t.Fatalf("缺失 transcript 应直接跳过 resume，不应触发额外重试: got=%d options=%+v", len(options), options)
	}
	if strings.TrimSpace(options[0].Session.ResumeID) != "" {
		t.Fatalf("缺失 transcript 时不应继续携带旧 room sdk_session_id: %+v", options[0])
	}

	updatedContext, err := roomService.GetConversationContext(ctx, roomContext.Conversation.ID)
	if err != nil {
		t.Fatalf("读取更新后的 room context 失败: %v", err)
	}
	if len(updatedContext.Sessions) != 1 {
		t.Fatalf("更新后的 room session 数量不正确: %+v", updatedContext.Sessions)
	}
	if updatedContext.Sessions[0].SDKSessionID != client.sessionID {
		t.Fatalf("room sdk_session_id 未写回新的 session: %+v", updatedContext.Sessions[0])
	}
}

func TestRealtimeServiceDoesNotPersistRoomSDKSessionIDWithoutTranscript(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	agentValue := createTestAgent(t, agentService, ctx, "未落盘助手")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID},
		Name:     "未落盘 Session 房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	if len(roomContext.Sessions) != 1 {
		t.Fatalf("期望只有一个 room session: %+v", roomContext.Sessions)
	}

	client := newFakeRoomClient()
	client.sessionID = "44444444-5555-4555-8555-444444444444"
	client.onQuery = func(_ context.Context, _ string) error {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "room-result-without-transcript",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
		return nil
	}

	permission := permissionctx.NewContext()
	factory := &fakeRoomFactory{clients: []*fakeRoomClient{client}}
	runtimeManager := runtimectx.NewManager()
	service := NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		factory,
	)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-without-transcript-sender")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "测试无 transcript 时不写 room resume",
		RoundID:        "room-round-without-transcript",
		ReqID:          "room-round-without-transcript",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.Options()
	if len(options) != 1 {
		t.Fatalf("无 transcript 的新 session 不应触发额外重试: got=%d options=%+v", len(options), options)
	}
	if strings.TrimSpace(options[0].Session.ResumeID) != "" {
		t.Fatalf("新 room 会话不应携带 resume: %+v", options[0])
	}

	updatedContext, err := roomService.GetConversationContext(ctx, roomContext.Conversation.ID)
	if err != nil {
		t.Fatalf("读取更新后的 room context 失败: %v", err)
	}
	if len(updatedContext.Sessions) != 1 {
		t.Fatalf("更新后的 room session 数量不正确: %+v", updatedContext.Sessions)
	}
	if strings.TrimSpace(updatedContext.Sessions[0].SDKSessionID) != "" {
		t.Fatalf("transcript 未落盘时不应写入 room sdk_session_id: %+v", updatedContext.Sessions[0])
	}
}

func TestRealtimeServiceRetriesRoomRuntimeWithoutStaleSDKSessionID(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	agentValue := createTestAgent(t, agentService, ctx, "恢复助手")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID},
		Name:     "Stale Resume 房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	if len(roomContext.Sessions) != 1 {
		t.Fatalf("期望只有一个 room session: %+v", roomContext.Sessions)
	}

	db, err = sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer db.Close()

	staleResumeID := "55555555-5555-5555-5555-555555555555"
	if _, err = db.Exec(`UPDATE sessions SET sdk_session_id = ? WHERE id = ?`, staleResumeID, roomContext.Sessions[0].ID); err != nil {
		t.Fatalf("预写入 room sdk_session_id 失败: %v", err)
	}
	writeRoomTranscriptFixture(t, agentValue.WorkspacePath, staleResumeID, []map[string]any{
		{
			"type":       "result",
			"session_id": staleResumeID,
			"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
		},
	})

	staleClient := newFakeRoomClient()
	staleClient.sessionID = staleResumeID
	staleClient.connectErr = agentclient.ErrNotConnected

	recoveredClient := newFakeRoomClient()
	recoveredClient.sessionID = "66666666-6666-6666-6666-666666666666"
	writeRoomTranscriptFixture(t, agentValue.WorkspacePath, recoveredClient.sessionID, []map[string]any{
		{
			"type":       "result",
			"session_id": recoveredClient.sessionID,
			"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
		},
	})
	recoveredClient.onQuery = func(_ context.Context, _ string) error {
		go func() {
			recoveredClient.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: recoveredClient.sessionID,
				UUID:      "room-result-recovered",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
		return nil
	}

	permission := permissionctx.NewContext()
	factory := &fakeRoomFactory{clients: []*fakeRoomClient{staleClient, recoveredClient}}
	runtimeManager := runtimectx.NewManager()
	service := NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		factory,
	)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-stale-resume-sender")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "测试 room stale resume 恢复",
		RoundID:        "room-round-stale-resume",
		ReqID:          "room-round-stale-resume",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.Options()
	if len(options) != 2 {
		t.Fatalf("stale resume 应触发一次无 resume 重试: got=%d options=%+v", len(options), options)
	}
	if options[0].Session.ResumeID != staleResumeID {
		t.Fatalf("首次启动应透传旧 room sdk_session_id: %+v", options[0])
	}
	if strings.TrimSpace(options[1].Session.ResumeID) != "" {
		t.Fatalf("重试启动不应继续携带旧 resume: %+v", options[1])
	}

	staleClient.mu.Lock()
	staleDisconnects := staleClient.disconnects
	staleClient.mu.Unlock()
	if staleDisconnects == 0 {
		t.Fatal("stale client connect 失败后应主动断开清理")
	}

	updatedContext, err := roomService.GetConversationContext(ctx, roomContext.Conversation.ID)
	if err != nil {
		t.Fatalf("读取更新后的 room context 失败: %v", err)
	}
	if len(updatedContext.Sessions) != 1 {
		t.Fatalf("更新后的 room session 数量不正确: %+v", updatedContext.Sessions)
	}
	if updatedContext.Sessions[0].SDKSessionID != recoveredClient.sessionID {
		t.Fatalf("room sdk_session_id 未写回恢复后的 session: %+v", updatedContext.Sessions[0])
	}
}

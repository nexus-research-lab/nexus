// lifecycle_test.go 覆盖 Room 会话生命周期、重启恢复与运行时策略测试。
package realtime_test

import (
	"context"
	"database/sql"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/app"
	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
	realtimesvc "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	goalstore "github.com/nexus-research-lab/nexus/internal/storage/goal"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	_ "modernc.org/sqlite"
)

func TestRoomServiceLifecycle(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	roomService.SetSessionArtifactDeletionCoordinator(
		noopSessionArtifactDeletionCoordinator{},
	)

	ctx := context.Background()
	agentA := createTestAgent(t, agentService, ctx, "测试助手A")
	agentB := createTestAgent(t, agentService, ctx, "测试助手B")
	agentC := createTestAgent(t, agentService, ctx, "测试助手C")

	mainContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs:               []string{agentA.AgentID, agentB.AgentID},
		Name:                   "产品讨论",
		Title:                  "主对话",
		Avatar:                 "7",
		HostAgentID:            agentA.AgentID,
		HostAutoReplyEnabled:   true,
		PrivateMessagesEnabled: true,
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	if mainContext.Room.RoomType != protocol.RoomTypeGroup {
		t.Fatalf("room_type 不正确: %s", mainContext.Room.RoomType)
	}
	if mainContext.Conversation.ConversationType != protocol.ConversationTypeMain {
		t.Fatalf("主对话类型不正确: %s", mainContext.Conversation.ConversationType)
	}
	if len(mainContext.Members) != 3 {
		t.Fatalf("成员数量不正确: got=%d want=3", len(mainContext.Members))
	}
	if len(mainContext.Sessions) != 2 {
		t.Fatalf("主对话 session 数量不正确: got=%d want=2", len(mainContext.Sessions))
	}
	if mainContext.Room.Avatar != "7" {
		t.Fatalf("room avatar 不正确: got=%q want=%q", mainContext.Room.Avatar, "7")
	}
	if mainContext.Room.HostAgentID != agentA.AgentID || !mainContext.Room.HostAutoReplyEnabled {
		t.Fatalf("room 群主设置不正确: %+v", mainContext.Room)
	}
	if !mainContext.Room.PrivateMessagesEnabled {
		t.Fatalf("room 私信设置不正确: %+v", mainContext.Room)
	}

	rooms, err := roomService.ListRooms(ctx, 20)
	if err != nil {
		t.Fatalf("列出 room 失败: %v", err)
	}
	if len(rooms) != 1 {
		t.Fatalf("room 数量不正确: got=%d want=1", len(rooms))
	}
	if rooms[0].Room.Avatar != "7" {
		t.Fatalf("list room avatar 不正确: got=%q want=%q", rooms[0].Room.Avatar, "7")
	}
	if !rooms[0].Room.PrivateMessagesEnabled {
		t.Fatalf("list room 私信设置不正确: %+v", rooms[0].Room)
	}
	if err = roomService.MarkConversationStarted(ctx, mainContext.Conversation.ID, time.Now().UTC()); err != nil {
		t.Fatalf("标记主对话已开始失败: %v", err)
	}

	updatedAvatar := "12"
	disableHostAutoReply := false
	disablePrivateMessages := false
	nextHostAgentID := agentB.AgentID
	mainContext, err = roomService.UpdateRoom(ctx, mainContext.Room.ID, protocol.UpdateRoomRequest{
		Avatar:                 &updatedAvatar,
		HostAgentID:            &nextHostAgentID,
		HostAutoReplyEnabled:   &disableHostAutoReply,
		PrivateMessagesEnabled: &disablePrivateMessages,
	})
	if err != nil {
		t.Fatalf("更新 room avatar 失败: %v", err)
	}
	if mainContext.Room.Avatar != updatedAvatar {
		t.Fatalf("更新后 room avatar 不正确: got=%q want=%q", mainContext.Room.Avatar, updatedAvatar)
	}
	if mainContext.Room.HostAgentID != agentB.AgentID || mainContext.Room.HostAutoReplyEnabled {
		t.Fatalf("更新后 room 群主设置不正确: %+v", mainContext.Room)
	}
	if mainContext.Room.PrivateMessagesEnabled {
		t.Fatalf("更新后 room 私信设置不正确: %+v", mainContext.Room)
	}

	topicContext, err := roomService.CreateConversation(ctx, mainContext.Room.ID, protocol.CreateConversationRequest{})
	if err != nil {
		t.Fatalf("创建 topic 失败: %v", err)
	}
	if topicContext.Conversation.ConversationType != protocol.ConversationTypeTopic {
		t.Fatalf("topic 类型不正确: %s", topicContext.Conversation.ConversationType)
	}
	if len(topicContext.Sessions) != 2 {
		t.Fatalf("topic session 数量不正确: got=%d want=2", len(topicContext.Sessions))
	}

	updatedContext, err := roomService.AddRoomMember(ctx, mainContext.Room.ID, protocol.AddRoomMemberRequest{
		AgentID: agentC.AgentID,
	})
	if err != nil {
		t.Fatalf("追加成员失败: %v", err)
	}
	if len(updatedContext.Sessions) != 3 {
		t.Fatalf("追加成员后主对话 session 数量不正确: got=%d want=3", len(updatedContext.Sessions))
	}

	updatedContext, err = roomService.RemoveRoomMember(ctx, mainContext.Room.ID, agentC.AgentID)
	if err != nil {
		t.Fatalf("移除成员失败: %v", err)
	}
	if len(updatedContext.Sessions) != 2 {
		t.Fatalf("移除成员后主对话 session 数量不正确: got=%d want=2", len(updatedContext.Sessions))
	}

	fallbackContext, err := roomService.DeleteConversation(ctx, mainContext.Room.ID, topicContext.Conversation.ID)
	if err != nil {
		t.Fatalf("删除 topic 失败: %v", err)
	}
	if fallbackContext.Conversation.ConversationType != protocol.ConversationTypeMain {
		t.Fatalf("删除 topic 后未回退到主对话: %s", fallbackContext.Conversation.ConversationType)
	}

	dmContext, err := roomService.EnsureDirectRoom(ctx, agentA.AgentID)
	if err != nil {
		t.Fatalf("创建直聊失败: %v", err)
	}
	if dmContext.Room.RoomType != protocol.RoomTypeDM {
		t.Fatalf("直聊类型不正确: %s", dmContext.Room.RoomType)
	}
	if len(dmContext.Sessions) != 1 {
		t.Fatalf("直聊 session 数量不正确: got=%d want=1", len(dmContext.Sessions))
	}
}

func TestRoomServiceClosesConversationRuntime(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	runtimeCloser := &fakeRoomRuntimeCloser{}
	roomService.SetRuntimeManager(runtimeCloser)
	roomService.SetSessionArtifactDeletionCoordinator(
		noopSessionArtifactDeletionCoordinator{},
	)

	ctx := context.Background()
	agentA := createTestAgent(t, agentService, ctx, "测试助手A")
	agentB := createTestAgent(t, agentService, ctx, "测试助手B")
	mainContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentA.AgentID, agentB.AgentID},
		Name:     "产品讨论",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	if err = roomService.MarkConversationStarted(ctx, mainContext.Conversation.ID, time.Now().UTC()); err != nil {
		t.Fatalf("标记主对话已开始失败: %v", err)
	}
	topicContext, err := roomService.CreateConversation(ctx, mainContext.Room.ID, protocol.CreateConversationRequest{})
	if err != nil {
		t.Fatalf("创建 topic 失败: %v", err)
	}
	expectedKeys := []string{
		protocol.BuildRoomSharedSessionKey(topicContext.Conversation.ID),
		protocol.BuildRoomAgentSessionKey(topicContext.Conversation.ID, agentA.AgentID, protocol.RoomTypeGroup),
		protocol.BuildRoomAgentSessionKey(topicContext.Conversation.ID, agentB.AgentID, protocol.RoomTypeGroup),
	}

	if err = roomService.CloseConversationRuntime(ctx, mainContext.Room.ID, topicContext.Conversation.ID); err != nil {
		t.Fatalf("关闭 conversation runtime 失败: %v", err)
	}
	assertRuntimeClosedKeys(t, runtimeCloser.keys, expectedKeys)

	runtimeCloser.keys = nil
	if _, err = roomService.DeleteConversation(ctx, mainContext.Room.ID, topicContext.Conversation.ID); err != nil {
		t.Fatalf("删除 topic 失败: %v", err)
	}
	assertRuntimeClosedKeys(t, runtimeCloser.keys, expectedKeys)
}

// 中断生命周期测试。

func TestRealtimeServiceTreatsClosedStreamAfterInterruptAsInterrupted(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	agentValue := createTestAgent(t, agentService, ctx, "助手甲")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID},
		Name:     "中断关流测试房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, _ string) error {
		go sendFakeRoomThinkingStream(client, "assistant-interrupted-closed-stream", "关流前的内部思考")
		return nil
	}
	var closeOnce sync.Once
	client.onInterrupt = func(_ context.Context) {
		closeOnce.Do(func() {
			close(client.messages)
		})
	}

	permission := permissionctx.NewContext()
	service := NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{client}},
	)
	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-interrupt-closed-stream")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@助手甲 处理一下",
		RoundID:        "room-round-closed-stream",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	_ = collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		for _, observed := range events {
			if observed.EventType != protocol.EventTypeStream {
				continue
			}
			block, _ := observed.Data["content_block"].(map[string]any)
			if block["type"] == "thinking" {
				return true
			}
		}
		return false
	})

	if err = service.HandleInterrupt(ctx, realtimesvc.InterruptRequest{SessionKey: sharedSessionKey}); err != nil {
		t.Fatalf("HandleInterrupt 失败: %v", err)
	}

	events := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus &&
			(event.Data["status"] == "interrupted" || event.Data["status"] == "error")
	})
	terminalStatus := ""
	for _, event := range events {
		if event.EventType == protocol.EventTypeRoundStatus {
			terminalStatus = anyToString(event.Data["status"])
		}
	}
	if terminalStatus != "interrupted" {
		t.Fatalf("主动中断后的关流应归类为 interrupted，实际 status=%s events=%+v", terminalStatus, events)
	}
	if countRoomResultSubtype(events, "error") > 0 {
		t.Fatalf("主动中断后的关流不应广播 error result: %+v", events)
	}

	sharedMessages, err := roomHistory.ReadMessages(roomContext.Room.OwnerUserID, roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取中断后的共享 Room 消息失败: %v", err)
	}
	foundInterrupted := false
	partialThinkingPreserved := false
	for _, message := range sharedMessages {
		if message["role"] == "assistant" {
			for _, block := range roomContentBlocksFromPayload(t, message) {
				if block["type"] == "thinking" && block["thinking"] == "关流前的内部思考" {
					partialThinkingPreserved = true
				}
			}
		}
		summary, ok := message["result_summary"].(map[string]any)
		if !ok {
			continue
		}
		if summary["subtype"] == "error" {
			t.Fatalf("主动中断后的共享日志不应落 error summary: %+v", sharedMessages)
		}
		if summary["subtype"] == "interrupted" {
			foundInterrupted = true
			if strings.Contains(anyToString(summary["result"]), "round stream closed before terminal") {
				t.Fatalf("interrupted summary 不应暴露底层 stream 错误: %+v", summary)
			}
		}
	}
	if !foundInterrupted {
		t.Fatalf("共享日志未落 interrupted summary: %+v", sharedMessages)
	}
	if !partialThinkingPreserved {
		t.Fatalf("共享日志未保留强制中断前的流式思考: %+v", sharedMessages)
	}
}

func TestRoomRoundCompletionReachesAutomationObserver(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)
	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	agent := createTestAgent(t, agentService, ctx, "定时助手")
	room, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agent.AgentID}, Name: "自动化终态回归",
	})
	if err != nil {
		t.Fatal(err)
	}
	client := newFakeRoomClient()
	client.onQuery = func(context.Context, string) error {
		client.messages <- sdkprotocol.ReceivedMessage{
			Type: sdkprotocol.MessageTypeResult, SessionID: client.sessionID, UUID: "result-1",
			Result: &sdkprotocol.ResultMessage{Subtype: "success", Result: "任务完成", NumTurns: 1},
		}
		return nil
	}
	manager := runtimectx.NewManager()
	service := NewServiceWithFactory(cfg, roomService, agentService, manager,
		permissionctx.NewContext(), &fakeRoomFactory{clients: []*fakeRoomClient{client}})
	broadcaster := &roomDirectedMessageBroadcaster{}
	service.SetRoomBroadcaster(broadcaster)
	sink := automationexec.NewExecutionSink("automation:completion-test")
	defer sink.Close()
	const roundID = "automation-room-round"
	sessionKey := protocol.BuildRoomSharedSessionKey(room.Conversation.ID)
	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey: sessionKey, ConversationID: room.Conversation.ID, RoomID: room.Room.ID,
		Content: "完成本次任务", TargetAgentIDs: []string{agent.AgentID}, RoundID: roundID,
		ExecutionOrigin: "automation",
		EventObserver:   func(ctx context.Context, event protocol.EventMessage) { _ = sink.SendEvent(ctx, event) },
	}); err != nil {
		t.Fatal(err)
	}
	waitCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	observation := sink.WaitForRound(waitCtx, roundID)
	if observation.Status != "succeeded" {
		t.Fatalf("Room 已结束时自动化必须收到成功终态，实际 %+v", observation)
	}
	terminalCount := 0
	for _, event := range broadcaster.Events() {
		if event.EventType == protocol.EventTypeRoundStatus && event.Data["is_terminal"] == true {
			terminalCount++
		}
	}
	if terminalCount != 1 {
		t.Fatalf("Web 应收到一次 root 终态，实际 %d", terminalCount)
	}
}

func TestRealtimeServiceMCPBuilderUsesSharedRoomSessionContext(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
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
		go sendFakeAssistantResult(client, "assistant-mcp-context", "ok")
		return nil
	}

	permission := permissionctx.NewContext()
	service := NewServiceWithFactory(
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
		leaseSessionKey    string
		leaseRoundID       string
	}
	calls := make(chan builderCall, 1)
	service.SetMCPServerBuilder(func(
		builderContext context.Context,
		agentValue *protocol.Agent,
		sessionKey string,
		roundID string,
		sourceContextType string,
		sourceContextID string,
		sourceContextLabel string,
		_ *atomic.Int64,
		_ sdkpermission.Mode,
	) map[string]sdkmcp.ServerConfig {
		lease, _ := runtimectx.RuntimeRoundLeaseFromContext(builderContext)
		calls <- builderCall{
			agentID:            agentValue.AgentID,
			sessionKey:         sessionKey,
			roundID:            roundID,
			sourceContextType:  sourceContextType,
			sourceContextID:    sourceContextID,
			sourceContextLabel: sourceContextLabel,
			leaseSessionKey:    lease.SessionKey,
			leaseRoundID:       lease.RoundID,
		}
		return nil
	})

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-mcp-context")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:                  sharedSessionKey,
		RoomID:                      roomContext.Room.ID,
		ConversationID:              roomContext.Conversation.ID,
		Content:                     "@助手甲 每天 9 点检查新闻并发回这个房间",
		RoundID:                     "room-round-mcp-context",
		TrustedConfigurationContext: true,
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
	expectedLeaseSession := protocol.BuildRoomAgentSessionKey(
		roomContext.Conversation.ID,
		agentValue.AgentID,
		roomContext.Room.RoomType,
	)
	if call.leaseSessionKey != expectedLeaseSession ||
		call.leaseRoundID == "" ||
		call.leaseRoundID == call.roundID {
		t.Fatalf("Room MCP 必须同时携带真实 slot lease 和独立共享 root context: %+v", call)
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

func TestRealtimeServiceForksLegacyRoomSessionForConnectorToolSurface(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	agentValue := createTestAgent(t, agentService, ctx, "飞书助手")
	agentOptions := agentValue.Options
	agentOptions.ConnectorIDs = []string{"feishu-docx"}
	agentValue, err = agentService.UpdateAgent(ctx, agentValue.AgentID, protocol.UpdateRequest{
		Options: &agentOptions,
	})
	if err != nil {
		t.Fatalf("开启 Agent Connector 失败: %v", err)
	}
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID},
		Name:     "Connector 换代房间",
		Title:    "老会话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	db, err = sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer db.Close()
	legacySessionID := "77777777-7777-4777-8777-777777777777"
	if _, err = db.Exec(
		`UPDATE sessions SET sdk_session_id = ? WHERE id = ?`,
		legacySessionID,
		roomContext.Sessions[0].ID,
	); err != nil {
		t.Fatalf("预写入旧 Room SDK session 失败: %v", err)
	}
	writeRoomTranscriptFixture(t, roomContext.Room.OwnerUserID, agentValue.WorkspacePath, legacySessionID, []map[string]any{{
		"type":       "result",
		"session_id": legacySessionID,
		"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
	}})

	newSessionID := "88888888-8888-4888-8888-888888888888"
	writeRoomTranscriptFixture(t, roomContext.Room.OwnerUserID, agentValue.WorkspacePath, newSessionID, []map[string]any{{
		"type":       "result",
		"session_id": newSessionID,
		"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
	}})
	client := newFakeRoomClient()
	client.sessionID = ""
	client.onQuery = func(_ context.Context, _ string) error {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: newSessionID,
				UUID:      "room-result-tool-surface-fork",
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
	service := NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permission,
		factory,
	)
	service.SetMCPServerBuilder(func(
		context.Context,
		*protocol.Agent,
		string,
		string,
		string,
		string,
		string,
		*atomic.Int64,
		sdkpermission.Mode,
	) map[string]sdkmcp.ServerConfig {
		return map[string]sdkmcp.ServerConfig{
			"feishu-docx": sdkmcp.StdioServerConfig{Command: "feishu-docx"},
		}
	})

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-tool-surface-fork-sender")
	permission.BindSession(sharedSessionKey, sender)
	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "请使用飞书文档",
		RoundID:        "room-round-tool-surface-fork",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}
	collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.Session.ResumeID != legacySessionID || !options.Session.Fork {
		t.Fatalf("旧 Room 工具面无基线时应从旧 transcript fork: %+v", options.Session)
	}
	expectedFingerprint, _, err := runtimectx.ModelToolSurfaceFingerprint(ctx, options)
	if err != nil {
		t.Fatalf("计算期望工具面指纹失败: %v", err)
	}
	updatedContext, err := roomService.GetConversationContext(ctx, roomContext.Conversation.ID)
	if err != nil {
		t.Fatalf("读取换代后的 Room context 失败: %v", err)
	}
	updatedSession := updatedContext.Sessions[0]
	if updatedSession.SDKSessionID != newSessionID {
		t.Fatalf("未持久化 fork 后的 SDK session: %+v", updatedSession)
	}
	if updatedSession.Options[protocol.OptionRuntimeToolSurfaceFingerprint] != expectedFingerprint {
		t.Fatalf("未与 SDK session 原子持久化工具面指纹: %+v", updatedSession.Options)
	}
}

func TestRealtimeServiceDoesNotPersistRoomSDKSessionIDWithoutTranscript(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
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
	service := NewServiceWithFactory(
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

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "测试无 transcript 时不写 room resume",
		RoundID:        "room-round-without-transcript",
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
	fingerprint, _ := updatedContext.Sessions[0].Options[protocol.OptionRuntimeToolSurfaceFingerprint].(string)
	if strings.TrimSpace(fingerprint) != "" {
		t.Fatalf("transcript 未落盘时不应前移工具面基线: %+v", updatedContext.Sessions[0])
	}
}

func TestRealtimeServiceRetriesRoomRuntimeWhenAutomaticToolSurfaceForkCannotResume(t *testing.T) {
	testRealtimeServiceRetriesRoomRuntimeWithoutStaleSDKSessionID(t, true)
}

func testRealtimeServiceRetriesRoomRuntimeWithoutStaleSDKSessionID(t *testing.T, toolSurfaceFork bool) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
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
	optionsJSON := `{}`
	if toolSurfaceFork {
		optionsJSON = `{"runtime_tool_surface_fingerprint":"stale-tool-surface"}`
	}
	if _, err = db.Exec(
		`UPDATE sessions SET sdk_session_id = ?, options_json = ? WHERE id = ?`,
		staleResumeID,
		optionsJSON,
		roomContext.Sessions[0].ID,
	); err != nil {
		t.Fatalf("预写入 room sdk_session_id 失败: %v", err)
	}
	writeRoomTranscriptFixture(t, roomContext.Room.OwnerUserID, agentValue.WorkspacePath, staleResumeID, []map[string]any{
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
	writeRoomTranscriptFixture(t, roomContext.Room.OwnerUserID, agentValue.WorkspacePath, recoveredClient.sessionID, []map[string]any{
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
	service := NewServiceWithFactory(
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

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "测试 room stale resume 恢复",
		RoundID:        "room-round-stale-resume",
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
	if options[0].Session.Fork != toolSurfaceFork {
		t.Fatalf("首次启动 fork=%v, want %v: %+v", options[0].Session.Fork, toolSurfaceFork, options[0])
	}
	if strings.TrimSpace(options[1].Session.ResumeID) != "" ||
		strings.TrimSpace(options[1].Session.ResumeAt) != "" ||
		options[1].Session.Fork {
		t.Fatalf("重试启动应创建全新 SDK session: %+v", options[1])
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

func TestRealtimeServiceForwardsProviderModelOption(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}
	providerService := providercfg.NewServiceWithDB(cfg, db)
	createdProvider, err := providerService.Create(context.Background(), providercfg.CreateInput{
		Provider:    "glm",
		DisplayName: "GLM",
		AuthToken:   "glm-token",
		BaseURL:     "https://open.bigmodel.cn/api/anthropic",
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("创建默认 provider 失败: %v", err)
	}
	if _, err = providerService.UpdateModel(context.Background(), createdProvider.Provider, "glm-5.1", providercfg.UpdateModelInput{
		Enabled:   true,
		IsDefault: true,
	}); err != nil {
		t.Fatalf("设置默认模型失败: %v", err)
	}

	ctx := context.Background()
	memberAgent := createTestAgent(t, agentService, ctx, "透传测试助手")
	maxThinkingTokens := 1024
	maxTurns := 4
	memberAgent, err = agentService.UpdateAgent(ctx, memberAgent.AgentID, protocol.UpdateRequest{
		Options: &protocol.Options{
			MaxThinkingTokens: &maxThinkingTokens,
			MaxTurns:          &maxTurns,
			SettingSources:    []string{"user"},
			MCPServers: map[string]any{
				"custom_room": map[string]any{
					"type": "http",
					"url":  "https://room-mcp.example.com/rpc",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("更新 member agent 配置失败: %v", err)
	}
	roomContext, err := createSingleAgentGroupRoom(ctx, roomService, memberAgent.AgentID)
	if err != nil {
		t.Fatalf("创建单成员 room 失败: %v", err)
	}
	if len(roomContext.MemberAgents) != 1 ||
		!slices.Contains(roomContext.MemberAgents[0].Options.SkillIDs, "goal-manager") ||
		!slices.Contains(roomContext.MemberAgents[0].Options.SkillIDs, "execution-orchestrator") ||
		roomContext.MemberAgents[0].RuntimeVersion != memberAgent.RuntimeVersion {
		t.Fatalf("Room member display projection lost runtime fields: %+v", roomContext.MemberAgents)
	}

	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, _ string) error {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "room-result-no-model",
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
	runtimeManager := runtimectx.NewManager()
	factory := &fakeRoomFactory{clients: []*fakeRoomClient{client}}
	service := NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		factory,
	)
	service.SetProviderResolver(providerService)
	titleScheduler := &fakeRoomTitleScheduler{}
	service.SetTitleGenerator(titleScheduler)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-no-model")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "测试 room model 透传",
		RoundID:        "room-round-no-model",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.Model != "glm-5.1" {
		t.Fatalf("room runtime 未向 SDK options 透传 provider model: %+v", options)
	}
	if options.Env["ANTHROPIC_MODEL"] != "glm-5.1" {
		t.Fatalf("room runtime 未注入 provider model: %+v", options.Env)
	}
	if options.Env["ANTHROPIC_DEFAULT_SONNET_MODEL"] != "glm-5.1" {
		t.Fatalf("room runtime 未注入默认 sonnet model: %+v", options.Env)
	}
	if options.Env["NEXUS_SUBAGENT_MODEL"] != "glm-5.1" {
		t.Fatalf("room runtime 未注入 subagent model: %+v", options.Env)
	}
	titleRequest := titleScheduler.LastRequest()
	if titleRequest.SessionMessageCount != -1 {
		t.Fatalf("room 标题生成不应检查共享 session 标题: %+v", titleRequest)
	}
	if titleRequest.ConversationID != roomContext.Conversation.ID {
		t.Fatalf("room 标题生成未绑定 conversation: %+v", titleRequest)
	}
	if options.Runtime.MaxThinkingTokens != maxThinkingTokens {
		t.Fatalf("room runtime 未向 SDK 透传 max thinking tokens: %+v", options)
	}
	if options.Runtime.MaxTurns != maxTurns {
		t.Fatalf("room runtime 未向 SDK 透传 max turns: %+v", options)
	}
	if len(options.SettingSources) != 1 || options.SettingSources[0] != "user" {
		t.Fatalf("room runtime 未向 SDK 透传 setting_sources: %+v", options)
	}
	customMCP, ok := options.MCP.Servers["custom_room"].(sdkmcp.HTTPServerConfig)
	if !ok || customMCP.URL != "https://room-mcp.example.com/rpc" {
		t.Fatalf("room runtime 未向 SDK 透传 Agent MCP server: %+v", options.MCP.Servers)
	}
	if !options.IncludePartialMessages {
		t.Fatalf("room runtime 未开启 partial messages: %+v", options)
	}
	for _, managedSkill := range []string{"goal-manager", "execution-orchestrator"} {
		if slices.Contains(options.Skills.DisabledNames, managedSkill) {
			t.Fatalf("Room runtime incorrectly disabled selected managed Skill %q: %+v", managedSkill, options.Skills)
		}
		decision, decisionErr := options.Callbacks.PermissionHandler(context.Background(), sdkpermission.Request{
			ToolName: "Skill",
			Input:    map[string]any{"name": managedSkill},
		})
		if decisionErr != nil || decision.Behavior != sdkpermission.BehaviorAllow {
			t.Fatalf("Room runtime did not auto-approve managed Skill %q: decision=%+v err=%v", managedSkill, decision, decisionErr)
		}
	}
	executionCommandDecision, decisionErr := options.Callbacks.PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: "mcp__nexus__command",
		Input:    map[string]any{"domain": "execution", "action": "inspect"},
	})
	if decisionErr != nil || executionCommandDecision.Behavior != sdkpermission.BehaviorAllow {
		t.Fatalf("Room runtime did not auto-approve exact Execution command: decision=%+v err=%v", executionCommandDecision, decisionErr)
	}
}

func TestRealtimeServiceBypassPermissionsKeepsQuestionChannel(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		mode          sdkpermission.Mode
		origin        string
		override      string
	}{
		{"ordinary", "测试 room bypass 权限处理器", sdkpermission.ModeBypassPermissions, "", ""},
		{"plan slash", "/plan improve search", sdkpermission.ModePlan, "", ""},
		{"relay safe default", "在线任务", sdkpermission.ModeDefault, "relay", ""},
		{"relay plan prompt", "/plan @Lucy improve search", sdkpermission.ModePlan, "relay", ""},
		{"relay explicit session setting", "在线任务", sdkpermission.ModeAcceptEdits, "relay", "acceptEdits"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := newRoomTestConfig(t)
			migrateRoomSQLite(t, cfg.DatabaseURL)

			agentService, db, err := newRoomTestAgentService(t, cfg)
			if err != nil {
				t.Fatalf("创建 agent service 失败: %v", err)
			}
			roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
			ctx := context.Background()
			memberAgent := createTestAgent(t, agentService, ctx, "bypass 助手")
			memberAgent, err = agentService.UpdateAgent(ctx, memberAgent.AgentID, protocol.UpdateRequest{
				Options: &protocol.Options{
					PermissionMode: "bypassPermissions",
					SettingSources: []string{"project"},
				},
			})
			if err != nil || memberAgent == nil {
				t.Fatalf("更新 member agent 配置失败: value=%+v err=%v", memberAgent, err)
			}
			roomContext, err := createSingleAgentGroupRoom(ctx, roomService, memberAgent.AgentID)
			if err != nil {
				t.Fatalf("创建单成员 room 失败: %v", err)
			}
			if tc.override != "" {
				sessions := app.NewSessionServiceWithDB(cfg, db, agentService)
				key := protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, memberAgent.AgentID, protocol.RoomTypeGroup)
				if _, err = sessions.UpdateRuntimeSettings(ctx, key, protocol.SessionRuntimeSettings{PermissionMode: tc.override}); err != nil {
					t.Fatal(err)
				}
			}

			client := newFakeRoomClient()
			client.onQuery = func(_ context.Context, _ string) error {
				go func() {
					client.messages <- sdkprotocol.ReceivedMessage{
						Type:      sdkprotocol.MessageTypeResult,
						SessionID: client.sessionID,
						UUID:      "room-result-bypass",
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
			runtimeManager := runtimectx.NewManager()
			factory := &fakeRoomFactory{clients: []*fakeRoomClient{client}}
			service := NewServiceWithFactory(cfg, roomService, agentService, runtimeManager, permission, factory)
			sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
			sender := newRealtimeTestSender("room-sender-bypass")
			permission.BindSession(sharedSessionKey, sender)

			if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
				SessionKey:      sharedSessionKey,
				RoomID:          roomContext.Room.ID,
				ConversationID:  roomContext.Conversation.ID,
				Content:         tc.content,
				RoundID:         "room-round-bypass",
				ExecutionOrigin: tc.origin,
				Internal:        tc.origin == "relay",
				TargetAgentIDs:  []string{memberAgent.AgentID},
			}); err != nil {
				t.Fatalf("HandleChat 失败: %v", err)
			}
			collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
				return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
			})

			unchanged, err := agentService.GetAgent(ctx, memberAgent.AgentID)
			if err != nil || unchanged.Options.PermissionMode != "bypassPermissions" {
				t.Fatalf("request changed Agent defaults: %+v %v", unchanged, err)
			}
			options := factory.LastOptions()
			if options.Runtime.PermissionMode != tc.mode {
				t.Fatalf("room request 权限模式未透传: %+v", options)
			}
			if options.Callbacks.PermissionHandler == nil {
				t.Fatalf("room bypass 权限模式应保留 AskUserQuestion 交互通道: %+v", options)
			}

		})
	}
}

func TestRealtimeServiceGoalContinuationDefersForSessionPlanOverride(
	t *testing.T,
) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	sessionService := app.NewSessionServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	memberAgent := createTestAgent(t, agentService, ctx, "Session 计划模式助手")
	roomContext, err := createSingleAgentGroupRoom(
		ctx,
		roomService,
		memberAgent.AgentID,
	)
	if err != nil {
		t.Fatalf("创建单成员 room 失败: %v", err)
	}
	memberSessionKey := protocol.BuildRoomAgentSessionKey(
		roomContext.Conversation.ID,
		memberAgent.AgentID,
		protocol.RoomTypeGroup,
	)
	if _, err = sessionService.UpdateRuntimeSettings(
		ctx,
		memberSessionKey,
		protocol.SessionRuntimeSettings{
			PermissionMode: string(sdkpermission.ModePlan),
		},
	); err != nil {
		t.Fatalf("更新 Room Session plan override 失败: %v", err)
	}

	service := NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permissionctx.NewContext(),
		&fakeRoomFactory{},
	)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(
		roomContext.Conversation.ID,
	)
	if !service.ShouldDeferGoalContinuation(ctx, sharedSessionKey) {
		t.Fatal("Room Session plan override 应阻止 Goal 自动续跑")
	}
}

func TestRealtimeServiceGoalContinuationDefersBehindPendingUserGuidance(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	memberAgent := createTestAgent(t, agentService, ctx, "用户输入优先助手")
	roomContext, err := createSingleAgentGroupRoom(ctx, roomService, memberAgent.AgentID)
	if err != nil {
		t.Fatalf("创建单成员 room 失败: %v", err)
	}

	location := workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  memberAgent.WorkspacePath,
		SessionKey:     protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, memberAgent.AgentID, roomContext.Room.RoomType),
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
	}
	if _, err = workspacestore.NewInputQueueStore(cfg.WorkspacePath).Enqueue(location, protocol.InputQueueItem{
		Scope:           protocol.InputQueueScopeRoom,
		SessionKey:      location.SessionKey,
		RoomID:          roomContext.Room.ID,
		ConversationID:  roomContext.Conversation.ID,
		AgentID:         memberAgent.AgentID,
		TargetAgentIDs:  []string{memberAgent.AgentID},
		Source:          protocol.InputQueueSourceUser,
		Content:         "先处理用户刚补充的要求",
		DeliveryPolicy:  protocol.ChatDeliveryPolicyGuide,
		SourceMessageID: "room-pending-guidance",
	}); err != nil {
		t.Fatalf("写入待处理用户引导失败: %v", err)
	}

	service := NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permissionctx.NewContext(),
		&fakeRoomFactory{},
	)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	if !service.ShouldDeferGoalContinuation(ctx, sharedSessionKey) {
		t.Fatal("Room Goal continuation should defer while explicit user guidance is pending")
	}
}

func TestRealtimeServiceRoomGoalTargetMissingUsesRoomOwnerForBackgroundContext(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ownerCtx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID:     "owner-1",
		Role:       authctx.RoleOwner,
		AuthMethod: authctx.AuthMethodLocal,
	})
	amy := createTestAgent(t, agentService, ownerCtx, "Amy")
	roomContext, err := roomService.CreateRoom(ownerCtx, protocol.CreateRoomRequest{
		AgentIDs: []string{amy.AgentID},
		Name:     "后台 Goal 房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	service := NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permissionctx.NewContext(),
		&fakeRoomFactory{},
	)
	missing, err := service.GoalContinuationTargetMissing(
		context.Background(),
		protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID),
	)
	if err != nil {
		t.Fatalf("GoalContinuationTargetMissing error = %v", err)
	}
	if missing {
		t.Fatal("后台 Room Goal 续跑不应因为缺少请求 owner 被误判为目标丢失")
	}
}

func TestRealtimeServiceGoalContinuationDefersWhenRoomHasNoDefaultTarget(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	amy := createTestAgent(t, agentService, ctx, "Amy")
	devin := createTestAgent(t, agentService, ctx, "Devin")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{amy.AgentID, devin.AgentID},
		Name:     "多助手 Goal 房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	service := NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permissionctx.NewContext(),
		&fakeRoomFactory{},
	)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	if !service.ShouldDeferGoalContinuation(ctx, sharedSessionKey) {
		t.Fatal("Room Goal continuation should defer when a multi-agent room has no default target")
	}

	hostedContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs:             []string{amy.AgentID, devin.AgentID},
		Name:                 "主持人 Goal 房间",
		Title:                "主对话",
		HostAgentID:          amy.AgentID,
		HostAutoReplyEnabled: false,
	})
	if err != nil {
		t.Fatalf("创建 hosted room 失败: %v", err)
	}
	hostedSessionKey := protocol.BuildRoomSharedSessionKey(hostedContext.Conversation.ID)
	if service.ShouldDeferGoalContinuation(ctx, hostedSessionKey) {
		t.Fatal("Room Goal continuation should not defer when a host lead exists")
	}
}

func TestRealtimeServiceChatRequestCanOverridePermissionHandler(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	memberAgent := createTestAgent(t, agentService, ctx, "非交互助手")
	memberAgent, err = agentService.UpdateAgent(ctx, memberAgent.AgentID, protocol.UpdateRequest{
		Options: &protocol.Options{
			DisallowedTools: []string{"nexus_communication", "mcp__nexus__send_message", "Write"},
		},
	})
	if err != nil || memberAgent == nil {
		t.Fatalf("更新 member agent 配置失败: value=%+v err=%v", memberAgent, err)
	}
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs:    []string{memberAgent.AgentID},
		HostAgentID: memberAgent.AgentID,
	})
	if err != nil {
		t.Fatalf("创建单成员 room 失败: %v", err)
	}

	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, _ string) error {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "room-result-permission-handler",
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

	var handledTools []string
	requestHandler := func(_ context.Context, request sdkpermission.Request) (sdkpermission.Decision, error) {
		handledTools = append(handledTools, request.ToolName)
		return sdkpermission.Deny("non-interactive room request", request.ToolName == "AskUserQuestion"), nil
	}
	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	factory := &fakeRoomFactory{clients: []*fakeRoomClient{client}}
	service := NewServiceWithFactory(cfg, roomService, agentService, runtimeManager, permission, factory)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-permission-handler")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:        sharedSessionKey,
		RoomID:            roomContext.Room.ID,
		ConversationID:    roomContext.Conversation.ID,
		Content:           "测试 room 请求级权限处理器",
		RoundID:           "room-round-permission-handler",
		PermissionHandler: requestHandler,
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}
	collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.Callbacks.PermissionHandler == nil {
		t.Fatalf("room 请求级权限处理器未透传: %+v", options)
	}
	if !slices.Equal(options.Tools.Allow, []string{"WebFetch", "WebSearch"}) {
		t.Fatalf("Room 默认只预授权低风险网页检索: %+v", options.Tools.Allow)
	}
	goalDecision, err := options.Callbacks.PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: "mcp__nexus__command",
		Input: map[string]any{
			"domain": "goal", "action": "invoke", "operation": "update_goal", "request_id": "room-goal-complete-1",
		},
	})
	if err != nil {
		t.Fatalf("执行 room Goal 权限处理器失败: %v", err)
	}
	if goalDecision.Behavior != sdkpermission.BehaviorAllow || len(handledTools) != 0 {
		t.Fatalf("room Goal command 应自动放行且不进入请求级 handler: decision=%+v tools=%+v", goalDecision, handledTools)
	}
	decision, err := options.Callbacks.PermissionHandler(context.Background(), sdkpermission.Request{ToolName: "Write"})
	if err != nil {
		t.Fatalf("执行 room 请求级权限处理器失败: %v", err)
	}
	if decision.Behavior != sdkpermission.BehaviorDeny || len(handledTools) != 1 || handledTools[0] != "Write" {
		t.Fatalf("room 请求级权限处理器未生效: decision=%+v tools=%+v", decision, handledTools)
	}
	roomDecision, err := options.Callbacks.PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: "mcp__nexus__send_message",
		Input:    map[string]any{"destination": "current_room", "visibility": "private"},
	})
	if err != nil {
		t.Fatalf("执行 room 内建通讯工具权限处理器失败: %v", err)
	}
	if roomDecision.Behavior != sdkpermission.BehaviorDeny ||
		len(handledTools) != 1 ||
		handledTools[0] != "Write" {
		t.Fatalf("Room 私信工具默认应直接拒绝: decision=%+v tools=%+v", roomDecision, handledTools)
	}
	if !roomTestStringSliceContains(options.Tools.Deny, "nexus_communication") {
		t.Fatalf("Room runtime 必须保留 Agent broad nexus_communication deny: %+v", options.Tools.Deny)
	}
	if !roomTestStringSliceContains(options.Tools.Deny, "mcp__nexus__send_message") {
		t.Fatalf("Room 私信工具 deny 配置应保留: %+v", options.Tools.Deny)
	}
	if !roomTestStringSliceContains(options.Tools.Deny, "Write") {
		t.Fatalf("Room runtime 应保留非通讯工具 deny 配置: %+v", options.Tools.Deny)
	}
}

type firstBlockingRoomQuotaChecker struct {
	mu      sync.Mutex
	calls   int
	entered chan struct{}
	release chan struct{}
}

type blockingInputQueueBroadcaster struct {
	once    sync.Once
	entered chan struct{}
	release chan struct{}
}

func (b *blockingInputQueueBroadcaster) Broadcast(_ context.Context, _ string, event protocol.EventMessage) []error {
	if event.EventType == protocol.EventTypeInputQueue {
		b.once.Do(func() {
			close(b.entered)
			<-b.release
		})
	}
	return nil
}

func (q *firstBlockingRoomQuotaChecker) EnsureQuotaAvailable(context.Context, string) error {
	q.mu.Lock()
	q.calls++
	first := q.calls == 1
	q.mu.Unlock()
	if first {
		close(q.entered)
		<-q.release
	}
	return nil
}

func (q *firstBlockingRoomQuotaChecker) callCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.calls
}

func TestRoomExplicitInputWinsGoalContinuationDispatchRace(t *testing.T) {
	cfg := newRoomTestConfig(t)
	cfg.GoalEnabled = true
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "owner-room-goal-race",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	memberAgent := createTestAgent(t, agentService, ctx, "Goal 竞态助手")
	roomContext, err := createSingleAgentGroupRoom(ctx, roomService, memberAgent.AgentID)
	if err != nil {
		t.Fatal(err)
	}

	client := newFakeRoomClient()
	queuedClient := newFakeRoomClient()
	runtimeManager := runtimectx.NewManager()
	service := realtimesvc.NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permissionctx.NewContext(),
		&fakeRoomFactory{clients: []*fakeRoomClient{client, queuedClient}},
	)
	broadcaster := &blockingInputQueueBroadcaster{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	service.SetRoomBroadcaster(broadcaster)
	goalService := goalsvc.NewService(cfg, goalstore.NewRepository(cfg, db))
	goalService.SetSessionOwnershipVerifier(testRoomGoalSessionOwnershipVerifier{
		agentNames: map[string]string{memberAgent.AgentID: memberAgent.Name},
	})
	service.SetGoalContextProvider(goalService)
	quota := &firstBlockingRoomQuotaChecker{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	service.SetQuotaChecker(quota)

	sessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	goal, err := goalService.Create(ctx, protocol.CreateGoalRequest{
		SessionKey:  sessionKey,
		Objective:   "在用户输入后继续",
		OwnerUserID: "owner-room-goal-race",
	})
	if err != nil {
		t.Fatal(err)
	}

	explicitResult := make(chan error, 1)
	go func() {
		explicitResult <- service.HandleChat(ctx, realtimesvc.ChatRequest{
			SessionKey:     sessionKey,
			RoomID:         roomContext.Room.ID,
			ConversationID: roomContext.Conversation.ID,
			Content:        "用户现在的指令",
			RoundID:        "round-explicit",
		})
	}()
	<-quota.entered

	continuationResult := make(chan error, 1)
	go func() {
		continuationResult <- service.DispatchGoalContinuation(ctx, protocol.GoalContinuation{
			Goal:           *goal,
			RoundID:        "round-goal-continuation",
			Prompt:         "continue goal",
			HiddenFromUser: true,
			Synthetic:      true,
			Purpose:        "goal_continuation",
		})
	}()
	close(quota.release)

	if err = <-explicitResult; err != nil {
		t.Fatalf("explicit HandleChat failed: %v", err)
	}
	if err = <-continuationResult; err != nil {
		t.Fatalf("Goal continuation dispatch failed: %v", err)
	}
	if got := quota.callCount(); got != 1 {
		t.Fatalf("quota checks = %d, want only the explicit input to reach runtime startup", got)
	}
	if running := runtimeManager.GetRunningRoundIDs(sessionKey); !slices.Contains(running, "round-explicit") || slices.Contains(running, "round-goal-continuation") {
		t.Fatalf("running rounds = %v, want explicit input without Goal continuation", running)
	}

	cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	if _, err = runtimeManager.InterruptSession(cleanupCtx, sessionKey, "test cleanup"); err != nil {
		cancel()
		t.Fatalf("cleanup Room round failed: %v", err)
	}
	cancel()

	enqueueResult := make(chan error, 1)
	go func() {
		_, enqueueErr := service.HandleInputQueue(ctx, realtimesvc.InputQueueRequest{
			SessionKey:      sessionKey,
			RoomID:          roomContext.Room.ID,
			ConversationID:  roomContext.Conversation.ID,
			ClientMessageID: "client-message-goal-dispatch-race",
			Action:          "enqueue",
			Content:         "排队的用户指令",
		})
		enqueueResult <- enqueueErr
	}()
	<-broadcaster.entered

	queuedContinuationResult := make(chan error, 1)
	go func() {
		queuedContinuationResult <- service.DispatchGoalContinuation(ctx, protocol.GoalContinuation{
			Goal:           *goal,
			RoundID:        "round-goal-after-enqueue",
			Prompt:         "continue goal",
			HiddenFromUser: true,
			Synthetic:      true,
			Purpose:        "goal_continuation",
		})
	}()
	close(broadcaster.release)

	if err = <-enqueueResult; err != nil {
		t.Fatalf("enqueue user input failed: %v", err)
	}
	if err = <-queuedContinuationResult; err != nil {
		t.Fatalf("Goal continuation after enqueue failed: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for quota.callCount() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := quota.callCount(); got != 2 {
		t.Fatalf("quota checks = %d, want one direct and one queued user input", got)
	}
	deadline = time.Now().Add(time.Second)
	for len(runtimeManager.GetRunningRoundIDs(sessionKey)) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	running := runtimeManager.GetRunningRoundIDs(sessionKey)
	if len(running) == 0 || slices.Contains(running, "round-goal-after-enqueue") {
		t.Fatalf("running rounds = %v, want queued user input without Goal continuation", running)
	}

	cleanupCtx, cancel = context.WithTimeout(context.Background(), time.Second)
	if _, err = runtimeManager.InterruptSession(cleanupCtx, sessionKey, "test cleanup"); err != nil {
		cancel()
		t.Fatalf("cleanup queued Room round failed: %v", err)
	}
	cancel()
}

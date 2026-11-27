// lifecycle_test.go 覆盖 Room 会话生命周期、重启恢复与运行时策略测试。
package realtime_test

import (
	"context"
	"database/sql"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
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
	realtimesvc "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	goalstore "github.com/nexus-research-lab/nexus/internal/storage/goal"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	_ "modernc.org/sqlite"
)

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

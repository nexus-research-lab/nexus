package room_test

import (
	"context"
	"testing"
	"time"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	_ "modernc.org/sqlite"
)

func TestRealtimeServiceForwardsProviderModelOption(t *testing.T) {
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
		},
	})
	if err != nil {
		t.Fatalf("更新 member agent 配置失败: %v", err)
	}
	dmContext, err := roomService.EnsureDirectRoom(ctx, memberAgent.AgentID)
	if err != nil {
		t.Fatalf("创建直聊 room 失败: %v", err)
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
	service := NewRealtimeServiceWithFactory(
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

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(dmContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-no-model")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         dmContext.Room.ID,
		ConversationID: dmContext.Conversation.ID,
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
	if titleRequest.ConversationID != dmContext.Conversation.ID {
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
	if !options.IncludePartialMessages {
		t.Fatalf("room runtime 未开启 partial messages: %+v", options)
	}
}

func TestRealtimeServiceBypassPermissionsKeepsQuestionChannel(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
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
	dmContext, err := roomService.EnsureDirectRoom(ctx, memberAgent.AgentID)
	if err != nil {
		t.Fatalf("创建直聊 room 失败: %v", err)
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
	service := NewRealtimeServiceWithFactory(cfg, roomService, agentService, runtimeManager, permission, factory)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(dmContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-bypass")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         dmContext.Room.ID,
		ConversationID: dmContext.Conversation.ID,
		Content:        "测试 room bypass 权限处理器",
		RoundID:        "room-round-bypass",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}
	collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.Runtime.PermissionMode != sdkpermission.ModeBypassPermissions {
		t.Fatalf("room bypass 权限模式未透传: %+v", options)
	}
	if options.Callbacks.PermissionHandler == nil {
		t.Fatalf("room bypass 权限模式应保留 AskUserQuestion 交互通道: %+v", options)
	}
}

func TestRealtimeServiceGoalContinuationDefersInPlanMode(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	memberAgent := createTestAgent(t, agentService, ctx, "计划模式助手")
	if _, err = agentService.UpdateAgent(ctx, memberAgent.AgentID, protocol.UpdateRequest{
		Options: &protocol.Options{PermissionMode: string(sdkpermission.ModePlan)},
	}); err != nil {
		t.Fatalf("更新 room member plan mode 失败: %v", err)
	}
	dmContext, err := roomService.EnsureDirectRoom(ctx, memberAgent.AgentID)
	if err != nil {
		t.Fatalf("创建直聊 room 失败: %v", err)
	}

	service := NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permissionctx.NewContext(),
		&fakeRoomFactory{},
	)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(dmContext.Conversation.ID)
	if !service.ShouldDeferGoalContinuation(ctx, sharedSessionKey) {
		t.Fatal("Room Goal continuation should defer while the target agent is in plan mode")
	}
}

func TestRealtimeServiceGoalContinuationDefersBehindPendingUserGuidance(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	memberAgent := createTestAgent(t, agentService, ctx, "用户输入优先助手")
	roomContext, err := roomService.EnsureDirectRoom(ctx, memberAgent.AgentID)
	if err != nil {
		t.Fatalf("创建直聊 room 失败: %v", err)
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

	service := NewRealtimeServiceWithFactory(
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

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
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

	service := NewRealtimeServiceWithFactory(
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

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
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

	service := NewRealtimeServiceWithFactory(
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

func TestRealtimeServiceGoalContinuationDefersForBusyNonLeadMember(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	host := createTestAgent(t, agentService, ctx, "Host")
	peer := createTestAgent(t, agentService, ctx, "Peer")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs:             []string{host.AgentID, peer.AgentID},
		Name:                 "Goal lead 独立运行房间",
		HostAgentID:          host.AgentID,
		HostAutoReplyEnabled: true,
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	peerClient := newFakeRoomClient()
	peerStarted := make(chan struct{}, 1)
	peerClient.onQuery = func(context.Context, string) error {
		peerStarted <- struct{}{}
		return nil
	}
	service := NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permissionctx.NewContext(),
		&fakeRoomFactory{clients: []*fakeRoomClient{peerClient}},
	)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@Peer 继续处理你的任务",
		RoundID:        "room-round-busy-non-lead",
	}); err != nil {
		t.Fatalf("启动非 lead 成员失败: %v", err)
	}
	select {
	case <-peerStarted:
	case <-time.After(time.Second):
		t.Fatal("非 lead 成员未进入运行态")
	}

	if !service.ShouldDeferGoalContinuation(ctx, sharedSessionKey) {
		t.Fatal("非 lead 成员仍在运行时应阻止 Host 提前启动 Room Goal continuation")
	}
	go sendFakeAssistantResult(peerClient, "assistant-busy-non-lead", "已完成")
}

func TestRealtimeServiceChatRequestCanOverridePermissionHandler(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	memberAgent := createTestAgent(t, agentService, ctx, "非交互助手")
	memberAgent, err = agentService.UpdateAgent(ctx, memberAgent.AgentID, protocol.UpdateRequest{
		Options: &protocol.Options{
			DisallowedTools: []string{"nexus_room", "mcp__nexus_room__send_directed_message", "Write"},
		},
	})
	if err != nil || memberAgent == nil {
		t.Fatalf("更新 member agent 配置失败: value=%+v err=%v", memberAgent, err)
	}
	dmContext, err := roomService.EnsureDirectRoom(ctx, memberAgent.AgentID)
	if err != nil {
		t.Fatalf("创建直聊 room 失败: %v", err)
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
	service := NewRealtimeServiceWithFactory(cfg, roomService, agentService, runtimeManager, permission, factory)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(dmContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-permission-handler")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:        sharedSessionKey,
		RoomID:            dmContext.Room.ID,
		ConversationID:    dmContext.Conversation.ID,
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
	if len(options.Tools.Allow) != 0 {
		t.Fatalf("room runtime 不应在无显式白名单时收窄 allowed tools: %+v", options.Tools.Allow)
	}
	goalDecision, err := options.Callbacks.PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: "mcp__nexus_goal__update_goal",
		Input:    map[string]any{"status": "complete"},
	})
	if err != nil {
		t.Fatalf("执行 room Goal 权限处理器失败: %v", err)
	}
	if goalDecision.Behavior != sdkpermission.BehaviorAllow || len(handledTools) != 0 {
		t.Fatalf("room Goal 权限应自动放行且不进入请求级 handler: decision=%+v tools=%+v", goalDecision, handledTools)
	}
	decision, err := options.Callbacks.PermissionHandler(context.Background(), sdkpermission.Request{ToolName: "Write"})
	if err != nil {
		t.Fatalf("执行 room 请求级权限处理器失败: %v", err)
	}
	if decision.Behavior != sdkpermission.BehaviorDeny || len(handledTools) != 1 || handledTools[0] != "Write" {
		t.Fatalf("room 请求级权限处理器未生效: decision=%+v tools=%+v", decision, handledTools)
	}
	roomDecision, err := options.Callbacks.PermissionHandler(context.Background(), sdkpermission.Request{ToolName: "mcp__nexus_room__send_directed_message"})
	if err != nil {
		t.Fatalf("执行 room 内建通讯工具权限处理器失败: %v", err)
	}
	if roomDecision.Behavior != sdkpermission.BehaviorDeny ||
		len(handledTools) != 1 ||
		handledTools[0] != "Write" {
		t.Fatalf("Room 私信工具默认应直接拒绝: decision=%+v tools=%+v", roomDecision, handledTools)
	}
	if roomTestStringSliceContains(options.Tools.Deny, "nexus_room") {
		t.Fatalf("Room runtime 不应让 broad nexus_room deny 屏蔽公开通讯工具: %+v", options.Tools.Deny)
	}
	if !roomTestStringSliceContains(options.Tools.Deny, "mcp__nexus_room__send_directed_message") {
		t.Fatalf("Room 私信工具 deny 配置应保留: %+v", options.Tools.Deny)
	}
	if !roomTestStringSliceContains(options.Tools.Deny, "Write") {
		t.Fatalf("Room runtime 应保留非通讯工具 deny 配置: %+v", options.Tools.Deny)
	}
}

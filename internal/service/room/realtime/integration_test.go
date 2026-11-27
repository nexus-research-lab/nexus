// integration_test.go 覆盖 Room 公开交接、成员参与与 IM 绑定集成测试。
package realtime_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/app"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	management "github.com/nexus-research-lab/nexus/internal/service/channels/management"
	dm "github.com/nexus-research-lab/nexus/internal/service/dm"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	realtimesvc "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	goalstore "github.com/nexus-research-lab/nexus/internal/storage/goal"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	_ "modernc.org/sqlite"
)

func TestRoomGoalOnlyPublicMentionHandbackImmediatelyContinues(t *testing.T) {
	cfg := newRoomTestConfig(t)
	cfg.GoalEnabled = true
	cfg.GoalAutoContinueEnabled = true
	cfg.GoalMaxContinuationsPerRun = 2
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("create Agent service: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	const ownerUserID = "owner-goal-only-public-handoff"
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID: ownerUserID,
		Role:   authsvc.RoleOwner,
	})
	lead := createTestAgent(t, agentService, ctx, "Lead")
	researcher := createTestAgent(t, agentService, ctx, "Researcher")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{lead.AgentID, researcher.AgentID},
		Name:     "Goal-only public handoff regression",
		Title:    "Goal-only collaboration",
	})
	if err != nil {
		t.Fatalf("create Room: %v", err)
	}

	leadClient := newFakeRoomClient()
	researcherClient := newFakeRoomClient()
	var leadQueries atomic.Int32
	researcherResultSent := make(chan time.Time, 1)
	leadResumed := make(chan time.Time, 1)
	leadClient.onQuery = func(_ context.Context, _ string) error {
		switch leadQueries.Add(1) {
		case 1:
			go sendFakeAssistantResult(
				leadClient,
				"lead-goal-only-public-mention",
				"@Researcher 请给出一句链路核对结论。",
			)
		case 2:
			leadResumed <- time.Now()
			go sendFakeAssistantResult(
				leadClient,
				"lead-goal-only-after-handback",
				"已收到协作结果，继续完成目标检查。",
			)
		default:
			t.Errorf("unexpected extra Lead continuation: %d", leadQueries.Load())
		}
		return nil
	}
	researcherClient.onQuery = func(_ context.Context, _ string) error {
		sentAt := time.Now()
		go sendFakeAssistantResult(
			researcherClient,
			"researcher-goal-only-result",
			"Goal-only 公区协作链路已核对。",
		)
		researcherResultSent <- sentAt
		return nil
	}

	runtimeManager := runtimectx.NewManager()
	permission := permissionctx.NewContext()
	service := realtimesvc.NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{leadClient, researcherClient}},
	)
	goalRepository := goalstore.NewRepository(cfg, db)
	goalService := goalsvc.NewService(cfg, goalRepository)
	goalService.SetSessionOwnershipVerifier(testRoomGoalSessionOwnershipVerifier{
		agentNames: map[string]string{
			lead.AgentID:       lead.Name,
			researcher.AgentID: researcher.Name,
		},
	})
	service.SetGoalContextProvider(goalService)

	sessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := &realtimeTestSender{
		key:    "room-goal-only-public-handoff",
		events: make(chan protocol.EventMessage, 512),
	}
	permission.BindSession(sessionKey, sender)
	goalCommand, err := service.SetGoalFromCommand(ctx, protocol.GoalCommandRequest{
		SessionKey:      sessionKey,
		Objective:       "由 Lead 通过公区 @Researcher 完成一次 Goal-only 协作核对。",
		CommandContent:  "/goal 由 Lead 通过公区 @Researcher 完成一次 Goal-only 协作核对。",
		RoundID:         "round-composer-goal-only",
		UserMessageID:   "user-composer-goal-only",
		ClientRequestID: "request-composer-goal-only",
		ClientMessageID: "message-composer-goal-only",
		TargetAgentIDs:  []string{lead.AgentID},
	})
	if err != nil {
		t.Fatalf("set Room Goal from Composer command: %v", err)
	}
	if !goalCommand.UserMessageCommitted {
		t.Fatal("Composer Goal control record was not durable")
	}
	if goalCommand.InputSeq <= 0 {
		t.Fatal("durable Composer Goal control record must return its input_seq for the chat_ack")
	}
	goal := &goalCommand.Goal
	if got := protocol.GoalMetadataString(goal.Metadata, protocol.GoalMetadataExecutionMode); got != string(protocol.GoalExecutionModeGoalOnly) {
		t.Fatalf("Goal execution mode = %q, want goal_only", got)
	}
	if executionID := protocol.GoalReservedExecutionID(*goal); executionID != "" {
		t.Fatalf("Goal-only test unexpectedly reserved Execution %q", executionID)
	}

	firstPlan, err := goalService.PlanContinuationForSession(ctx, sessionKey, "")
	if err != nil || firstPlan == nil {
		t.Fatalf("plan initial Goal continuation = %+v, err=%v", firstPlan, err)
	}
	if err = service.DispatchGoalContinuation(ctx, *firstPlan); err != nil {
		t.Fatalf("dispatch initial Goal continuation: %v", err)
	}

	var researcherSentAt time.Time
	select {
	case researcherSentAt = <-researcherResultSent:
	case <-time.After(3 * time.Second):
		t.Fatal("public @Researcher did not start a target round")
	}
	var resumedAt time.Time
	select {
	case resumedAt = <-leadResumed:
	case <-time.After(3 * time.Second):
		t.Fatal("Goal handback did not immediately start the next continuation")
	}
	if delay := resumedAt.Sub(researcherSentAt); delay >= 3*time.Second {
		t.Fatalf("Goal handback continuation delay = %v, want immediate dispatch", delay)
	}

	var firstReceiptStatus string
	if err = db.QueryRowContext(
		ctx,
		`SELECT status FROM goal_continuation_plans WHERE round_id = ?`,
		firstPlan.RoundID,
	).Scan(&firstReceiptStatus); err != nil {
		t.Fatalf("read first continuation receipt: %v", err)
	}
	if firstReceiptStatus != string(protocol.GoalContinuationPlanStatusSettled) {
		t.Fatalf("first continuation receipt status = %q, want settled", firstReceiptStatus)
	}

	handoffs, err := workspacestore.NewRoomPublicHandoffStore(cfg.WorkspacePath).ListRoot(
		ownerUserID,
		roomContext.Conversation.ID,
		firstPlan.RoundID,
	)
	if err != nil {
		t.Fatalf("read public handoff ledger: %v", err)
	}
	if len(handoffs) != 1 {
		t.Fatalf("public handoffs = %+v, want one Goal-attributed edge", handoffs)
	}
	handoff := handoffs[0]
	if handoff.GoalCollaborationBinding == nil ||
		handoff.GoalCollaborationBinding.GoalID != goal.ID ||
		handoff.GoalCollaborationBinding.ObjectiveRevision != goal.ObjectiveRevision() {
		t.Fatalf("Goal collaboration binding = %+v, want exact current Goal revision", handoff.GoalCollaborationBinding)
	}
	if handoff.Status != "finished" || !handoff.GoalHandbackSettled {
		t.Fatalf("public handoff terminal state = %+v, want finished plus settled handback", handoff)
	}
	if handoff.SourceAgentRoundID == "" || handoff.SourceAgentRoundID == firstPlan.RoundID {
		t.Fatalf(
			"source identities collapsed: receipt=%q agent_round=%q",
			firstPlan.RoundID,
			handoff.SourceAgentRoundID,
		)
	}
	if handoff.TargetRoundID == "" || handoff.TargetAgentRoundID == "" ||
		handoff.TargetRoundID == handoff.TargetAgentRoundID {
		t.Fatalf("target root/slot identities were not kept distinct: %+v", handoff)
	}
	events := collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeMessage &&
			event.MessageID == "researcher-goal-only-result" &&
			event.Data["role"] == "assistant" && event.Data["is_complete"] == true &&
			protocol.NormalizePublicHandoffReply(event.Data["handoff_reply"]) != nil
	})
	continuationAcked := false
	for _, event := range events {
		if event.EventType != protocol.EventTypeChatAck || event.Data["round_id"] != firstPlan.RoundID {
			continue
		}
		continuationAcked = true
		if _, positioned := event.Data[protocol.InputSeqField]; positioned {
			t.Fatalf("hidden Goal continuation persists no user row, so its chat_ack must not carry input_seq: %+v", event.Data)
		}
	}
	if !continuationAcked {
		t.Fatalf("hidden Goal continuation chat_ack missing from realtime events: %+v", events)
	}
	var researcherReply protocol.Message
	for _, event := range events {
		if event.EventType == protocol.EventTypeMessage &&
			event.MessageID == "researcher-goal-only-result" &&
			event.Data["role"] == "assistant" && event.Data["is_complete"] == true &&
			protocol.NormalizePublicHandoffReply(event.Data["handoff_reply"]) != nil {
			researcherReply = protocol.Message(event.Data)
			break
		}
	}
	if researcherReply == nil {
		t.Fatalf("Goal-only researcher public reply missing from realtime events: %+v", events)
	}
	reply := protocol.NormalizePublicHandoffReply(researcherReply["handoff_reply"])
	if reply == nil || reply.HandoffID != handoff.HandoffID ||
		reply.SourceMessageID != "lead-goal-only-public-mention" ||
		reply.SourceAgentID != lead.AgentID {
		t.Fatalf("Goal-only public reply annotation = %+v, message=%+v", reply, researcherReply)
	}
	if researcherReply["agent_mentions"] != nil {
		t.Fatalf("no-@ Goal-only reply must not synthesize a reciprocal mention: %+v", researcherReply)
	}
	deadline := time.Now().Add(3 * time.Second)
	for len(runtimeManager.GetRunningRoundIDs(sessionKey)) > 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if running := runtimeManager.GetRunningRoundIDs(sessionKey); len(running) != 0 {
		t.Fatalf("Room Goal-only rounds did not settle: %v", running)
	}
	if got := leadQueries.Load(); got != 2 {
		t.Fatalf("Lead queries = %d, want initial continuation plus one handback continuation", got)
	}
}

func TestRealtimeServicePausesOneMemberAndResumesPreservedWork(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	pausedAgent := createTestAgent(t, agentService, ctx, "暂停目标")
	activeAgent := createTestAgent(t, agentService, ctx, "继续目标")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{pausedAgent.AgentID, activeAgent.AgentID},
		Name:     "成员暂停恢复集成测试",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 Room 失败: %v", err)
	}

	pausedClient := newFakeRoomClient()
	pausedPrompts := make(chan string, 2)
	var pausedQueryMu sync.Mutex
	pausedQueryCount := 0
	pausedClient.onQuery = func(_ context.Context, prompt string) error {
		pausedQueryMu.Lock()
		pausedQueryCount++
		queryIndex := pausedQueryCount
		pausedQueryMu.Unlock()
		pausedPrompts <- prompt
		if queryIndex == 2 {
			go sendFakeAssistantResult(
				pausedClient,
				"paused-agent-resumed-result",
				"恢复后完成保留工作。",
			)
		}
		return nil
	}
	pausedClient.onInterrupt = func(_ context.Context) {
		go func() {
			pausedClient.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: pausedClient.sessionID,
				UUID:      "paused-agent-interrupted",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "interrupted",
					DurationMS: 1,
					NumTurns:   1,
				},
			}
		}()
	}

	activeClient := newFakeRoomClient()
	activePrompts := make(chan string, 1)
	activeClient.onQuery = func(_ context.Context, prompt string) error {
		activePrompts <- prompt
		go sendFakeAssistantResult(
			activeClient,
			"active-agent-result",
			"未暂停成员正常完成。",
		)
		return nil
	}

	permission := permissionctx.NewContext()
	service := NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{pausedClient, activeClient}},
	)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-member-participation")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "先执行一项长任务",
		TargetAgentIDs: []string{pausedAgent.AgentID},
		RoundID:        "room-round-before-member-pause",
	}); err != nil {
		t.Fatalf("启动待暂停成员失败: %v", err)
	}
	waitForParticipationPrompt(t, pausedPrompts, "暂停前首个 prompt")
	_ = collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeStreamStart && event.AgentID == pausedAgent.AgentID
	})

	pauseStartedAt := time.Now()
	updated, err := service.SetRoomMemberParticipation(
		ctx,
		roomContext.Room.ID,
		pausedAgent.AgentID,
		true,
	)
	if err != nil {
		t.Fatalf("暂停成员参与失败: %v", err)
	}
	// 中断应随 slot 收尾立即完成；接近 10s 中断超时说明派发锁与 slot 收尾互相等待。
	if elapsed := time.Since(pauseStartedAt); elapsed > 5*time.Second {
		t.Fatalf("暂停成员等待中断超时: %s", elapsed)
	}
	if !roomMemberParticipationPaused(updated.Members, pausedAgent.AgentID) {
		t.Fatalf("暂停状态未持久化: %+v", updated.Members)
	}
	pausedClient.mu.Lock()
	interruptCalls := pausedClient.interruptCalls
	pausedClient.mu.Unlock()
	if interruptCalls != 1 {
		t.Fatalf("暂停成员应精确中断一次当前 slot: got %d", interruptCalls)
	}

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "两位成员分别处理后续任务",
		TargetAgentIDs: []string{pausedAgent.AgentID, activeAgent.AgentID},
		RoundID:        "room-round-during-member-pause",
	}); err != nil {
		t.Fatalf("暂停期间发送多目标输入失败: %v", err)
	}
	activePrompt := waitForParticipationPrompt(t, activePrompts, "未暂停成员 prompt")
	if !strings.Contains(activePrompt, "两位成员分别处理后续任务") {
		t.Fatalf("未暂停成员 prompt 缺少用户输入: %s", activePrompt)
	}
	_ = collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus &&
			event.Data["round_id"] == "room-round-during-member-pause" &&
			event.Data["status"] == "finished"
	})

	pausedLocation := workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  pausedAgent.WorkspacePath,
		SessionKey:     protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, pausedAgent.AgentID, roomContext.Room.RoomType),
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
	}
	queueStore := workspacestore.NewInputQueueStore(cfg.WorkspacePath)
	queuedItems, err := queueStore.Snapshot(pausedLocation)
	if err != nil {
		t.Fatalf("读取暂停成员队列失败: %v", err)
	}
	if len(queuedItems) != 1 ||
		queuedItems[0].AgentID != pausedAgent.AgentID ||
		queuedItems[0].ID != "room-round-during-member-pause" {
		t.Fatalf("暂停成员输入未按精确目标保留: %+v", queuedItems)
	}
	pausedQueryMu.Lock()
	queriesBeforeResume := pausedQueryCount
	pausedQueryMu.Unlock()
	if queriesBeforeResume != 1 {
		t.Fatalf("暂停期间不应启动第二个 runtime query: got %d", queriesBeforeResume)
	}

	updated, err = service.SetRoomMemberParticipation(
		ctx,
		roomContext.Room.ID,
		pausedAgent.AgentID,
		false,
	)
	if err != nil {
		t.Fatalf("恢复成员参与失败: %v", err)
	}
	if roomMemberParticipationPaused(updated.Members, pausedAgent.AgentID) {
		t.Fatalf("恢复状态未持久化: %+v", updated.Members)
	}
	resumedPrompt := waitForParticipationPrompt(t, pausedPrompts, "恢复后的保留工作 prompt")
	if !strings.Contains(resumedPrompt, "两位成员分别处理后续任务") {
		t.Fatalf("恢复后 prompt 未携带保留输入: %s", resumedPrompt)
	}
	_ = collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeAgentRoundStatus &&
			event.Data["agent_id"] == pausedAgent.AgentID &&
			event.Data["status"] == "finished"
	})
	queuedItems, err = queueStore.Snapshot(pausedLocation)
	if err != nil {
		t.Fatalf("恢复后读取成员队列失败: %v", err)
	}
	if len(queuedItems) != 0 {
		t.Fatalf("恢复派发后队列应为空: %+v", queuedItems)
	}
	activeClient.mu.Lock()
	activeQueryCount := len(activeClient.queryPrompts)
	activeClient.mu.Unlock()
	if activeQueryCount != 1 {
		t.Fatalf("恢复暂停成员不得重启其他成员: active queries = %d", activeQueryCount)
	}
}

func waitForParticipationPrompt(
	t *testing.T,
	prompts <-chan string,
	label string,
) string {
	t.Helper()
	select {
	case prompt := <-prompts:
		return prompt
	case <-time.After(3 * time.Second):
		t.Fatalf("等待%s超时", label)
		return ""
	}
}

func roomMemberParticipationPaused(
	members []protocol.MemberRecord,
	agentID string,
) bool {
	for _, member := range members {
		if member.MemberType == protocol.MemberTypeAgent && member.MemberAgentID == agentID {
			return member.ParticipationPaused
		}
	}
	return false
}

type bindingDM struct{ called atomic.Int32 }

func (d *bindingDM) HandleChat(context.Context, dm.Request) error { d.called.Add(1); return nil }

type bindingChannel struct{ sent chan string }

func (*bindingChannel) ChannelType() string         { return channels.ChannelTypeFeishu }
func (*bindingChannel) Start(context.Context) error { return nil }
func (*bindingChannel) Stop(context.Context) error  { return nil }
func (c *bindingChannel) SendDeliveryMessage(_ context.Context, target channels.DeliveryTarget, text string) (channels.DeliveryResult, error) {
	c.sent <- text
	return channels.DeliveryResult{Target: target}, nil
}

func TestIMRoomBindingQueuesOneMemberAndStopsOldRepliesAfterSwitch(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)
	agents, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{UserID: "im-owner", Role: authctx.RoleOwner})
	agent := createTestAgent(t, agents, ctx, "负责人")
	other := createTestAgent(t, agents, ctx, "开发")
	rooms := app.NewRoomServiceWithDB(cfg, db, agents)
	room, err := rooms.CreateRoom(ctx, protocol.CreateRoomRequest{AgentIDs: []string{agent.AgentID, other.AgentID}, Name: "项目", PrivateMessagesEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	client := newFakeRoomClient()
	started := make(chan string, 4)
	release := make(chan struct{}, 4)
	var calls atomic.Int32
	client.onQuery = func(_ context.Context, prompt string) error {
		n := calls.Add(1)
		started <- prompt
		go func() {
			<-release
			sendFakeAssistantResult(client, fmt.Sprintf("im-answer-%d", n), fmt.Sprintf("处理完成-%d", n))
		}()
		return nil
	}
	factory := &fakeRoomFactory{clients: []*fakeRoomClient{client}}
	manager := runtimectx.NewManager()
	permissions := permissionctx.NewContext()
	service := realtimesvc.NewServiceWithFactory(cfg, rooms, agents, manager, permissions, factory)
	router := channels.NewRouter(cfg, db, agents, permissions)
	transport := &bindingChannel{sent: make(chan string, 8)}
	router.RegisterForOwner("im-owner", transport)
	if err := router.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer router.Stop(ctx)
	control := channels.NewControlService(cfg, db, agents, router)
	control.SetRoomService(rooms)
	plainDM := &bindingDM{}
	ingress := channels.NewIngressService(cfg, agents, plainDM, router)
	ingress.SetControlService(control)
	ingress.SetRuntimePermissionContext(permissions)
	ingress.SetRoomRealtime(service)
	pairing, err := control.CreatePairing(ctx, "im-owner", channels.CreatePairingRequest{ChannelType: channels.ChannelTypeFeishu, ChatType: "dm", ExternalRef: "phone", AgentID: agent.AgentID})
	if err != nil {
		t.Fatal(err)
	}
	target := management.PairingSessionTarget{RoomID: room.Room.ID, ConversationID: room.Conversation.ID}
	pairing, err = control.UpdatePairing(ctx, "im-owner", pairing.PairingID, channels.UpdatePairingRequest{SessionTarget: &target, BindingVersion: &pairing.BindingVersion})
	if err != nil {
		t.Fatal(err)
	}
	request := channels.IngressRequest{OwnerUserID: "im-owner", Channel: channels.ChannelTypeFeishu, ChatType: "dm", Ref: "phone", Content: "只处理手机任务", ReqID: "phone-1"}
	first, err := ingress.Accept(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case prompt := <-started:
		if !strings.Contains(prompt, request.Content) {
			t.Fatalf("缺少外部输入: %s", prompt)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Room 未启动")
	}

	decisionCtx, cancelDecision := context.WithTimeout(ctx, 5*time.Second)
	defer cancelDecision()
	decisions := make(chan sdkpermission.Decision, 1)
	go func() {
		decision, _ := factory.LastOptions().Callbacks.PermissionHandler(decisionCtx, sdkpermission.Request{ToolName: "Write", Input: map[string]any{"file_path": "test.txt", "content": "test"}})
		decisions <- decision
	}()
	select {
	case text := <-transport.sent:
		if !strings.Contains(text, "/y") {
			t.Fatalf("权限通知不正确: %s", text)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("未收到成员权限通知")
	}
	approval := request
	approval.Content = "/y"
	approval.ReqID = "phone-approval"
	if _, err := ingress.Accept(ctx, approval); err != nil {
		t.Fatal(err)
	}
	select {
	case decision := <-decisions:
		if decision.Behavior != sdkpermission.BehaviorAllow {
			t.Fatalf("未允许原成员请求: %+v", decision)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("成员权限决定未返回")
	}
	select {
	case <-transport.sent:
	case <-time.After(5 * time.Second):
		t.Fatal("缺少权限决定回执")
	}
	if duplicate, err := ingress.Accept(ctx, request); err != nil || duplicate.RoundID != first.RoundID {
		t.Fatalf("重复入站: %+v %v", duplicate, err)
	}
	second := request
	second.ReqID = "phone-2"
	second.Content = "第二条任务"
	if _, err := ingress.Accept(ctx, second); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 || plainDM.called.Load() != 0 {
		t.Fatal("忙碌时必须排队且不能启动独立 DM")
	}
	release <- struct{}{}
	select {
	case text := <-transport.sent:
		if text != "处理完成-1" {
			t.Fatalf("回信错误: %s", text)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("未收到完成回复")
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("第二条输入未接力")
	}
	independent := management.PairingSessionTarget{}
	if _, err := control.UpdatePairing(ctx, "im-owner", pairing.PairingID, channels.UpdatePairingRequest{SessionTarget: &independent, BindingVersion: &pairing.BindingVersion}); err != nil {
		t.Fatal(err)
	}
	release <- struct{}{}
	deadline := time.Now().Add(5 * time.Second)
	for service.CountRunningTasks(agent.AgentID) > 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if service.CountRunningTasks(agent.AgentID) > 0 {
		t.Fatal("原 Room 任务未完成")
	}
	select {
	case text := <-transport.sent:
		t.Fatalf("切换后不应发送旧回复: %s", text)
	default:
	}
	if len(factory.Options()) != 1 {
		t.Fatalf("应复用唯一成员 runtime: %d", len(factory.Options()))
	}
	history, err := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath).ReadMessages("im-owner", room.Conversation.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range history {
		if strings.Contains(fmt.Sprint(entry), request.Content) {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("外部群聊输入未写入公区历史")
	}
}

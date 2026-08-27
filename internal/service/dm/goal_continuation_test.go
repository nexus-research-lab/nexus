package dm

import (
	"context"
	"errors"
	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	_ "modernc.org/sqlite"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

type blockingDMQuotaChecker struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (q *blockingDMQuotaChecker) EnsureQuotaAvailable(ctx context.Context, _ string) error {
	q.once.Do(func() { close(q.entered) })
	select {
	case <-q.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type observedDMGoalProvider struct {
	*fakeGoalContextProvider
	validated chan struct{}
	once      sync.Once
}

func (p *observedDMGoalProvider) GoalContinuationStillCurrent(
	ctx context.Context,
	plan protocol.GoalContinuation,
) (bool, error) {
	current, err := p.fakeGoalContextProvider.GoalContinuationStillCurrent(ctx, plan)
	p.once.Do(func() { close(p.validated) })
	return current, err
}

func TestServiceHandleChatDoesNotBindAmbientGoalOrScheduleContinuation(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, prompt string) {
		go func() {
			resultID := "result-first"
			if strings.Contains(prompt, "hidden continuation prompt") {
				resultID = "result-goal-continuation"
				client.messages <- sdkprotocol.ReceivedMessage{
					Type:      sdkprotocol.MessageTypeAssistant,
					SessionID: client.sessionID,
					Assistant: &sdkprotocol.AssistantMessage{
						Message: sdkprotocol.ConversationEnvelope{
							ID:    "assistant-goal-continuation",
							Model: "sonnet",
							Content: []sdkprotocol.ContentBlock{
								sdkprotocol.TextBlock{Text: "继续推进 Goal"},
							},
						},
					},
				}
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      resultID,
				Result: &sdkprotocol.ResultMessage{
					Subtype:       "success",
					DurationMS:    1,
					DurationAPIMS: 1,
					NumTurns:      1,
					Result:        "done",
					Usage: map[string]any{
						"input_tokens":  int64(2),
						"output_tokens": int64(3),
					},
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	goal := protocol.Goal{
		ID:         "goal-1",
		SessionKey: "agent:nexus:ws:dm:test-goal-continuation",
		Objective:  "finish work",
		Status:     protocol.GoalStatusActive,
		Metadata: map[string]any{
			protocol.GoalMetadataExecutionID: "execution-goal-1",
		},
	}
	goalProvider := &fakeGoalContextProvider{
		runtimeContext: "finish work",
		runtimeGoal:    &goal,
		plan: &protocol.GoalContinuation{
			Goal: protocol.Goal{
				ID:         "goal-1",
				SessionKey: "agent:nexus:ws:dm:test-goal-continuation",
				Objective:  "finish work",
				Status:     protocol.GoalStatusActive,
				Metadata: map[string]any{
					protocol.GoalMetadataExecutionID: "execution-goal-1",
				},
			},
			RoundID:        "goal_continuation_1",
			Prompt:         "hidden continuation prompt",
			HiddenFromUser: true,
			Synthetic:      true,
			Purpose:        "goal_continuation",
			Metadata:       map[string]string{"goal_id": "goal-1"},
		},
	}
	service.SetGoalContextProvider(goalProvider)
	sender := newDMTestSender("sender-goal-continuation")
	sessionKey := "agent:nexus:ws:dm:test-goal-continuation"
	t.Cleanup(func() {
		_ = runtimeManager.CloseSession(context.Background(), sessionKey)
	})
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey:           sessionKey,
		Content:              "开始",
		RoundID:              "round-1",
		BroadcastUserMessage: true,
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}
	_ = collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus &&
			event.Data["round_id"] == "round-1" &&
			event.Data["status"] == "finished"
	})
	time.Sleep(50 * time.Millisecond)

	client.mu.Lock()
	queryOptions := append([]sdkprotocol.OutboundMessageOptions(nil), client.queryOptions...)
	queryPrompts := append([]string(nil), client.queryPrompts...)
	client.mu.Unlock()
	if len(queryOptions) != 1 {
		t.Fatalf("普通聊天不应因 ambient Goal 启动隐藏续跑: %+v", queryOptions)
	}
	if len(queryPrompts) != 1 || strings.Contains(queryPrompts[0], "finish work") {
		t.Fatalf("普通聊天不应注入 ambient Goal context: %+v", queryPrompts)
	}
	goalProvider.mu.Lock()
	planCalls := goalProvider.planCalls
	goalProvider.mu.Unlock()
	if planCalls != 0 {
		t.Fatalf("普通聊天触发了 Goal continuation planning: %d", planCalls)
	}
}

func TestServiceGoalContinuationFinalDispatchDefersToConcurrentExplicitInput(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	agentValue, err := agentService.GetAgent(context.Background(), cfg.DefaultAgentID)
	if err != nil {
		t.Fatal(err)
	}
	client := newFakeDMClient()
	queryStarted := make(chan string, 1)
	client.onQuery = func(_ context.Context, prompt string) {
		queryStarted <- prompt
	}
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client})
	service := NewService(cfg, agentService, runtimeManager, permissionctx.NewContext())
	quota := &blockingDMQuotaChecker{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	service.SetQuotaChecker(quota)
	sessionKey := "agent:nexus:ws:dm:test-goal-final-dispatch-race"
	t.Cleanup(func() {
		_ = runtimeManager.CloseSession(context.Background(), sessionKey)
	})
	goalProvider := &observedDMGoalProvider{
		fakeGoalContextProvider: &fakeGoalContextProvider{plan: &protocol.GoalContinuation{
			Goal: protocol.Goal{
				ID:         "goal-final-dispatch-race",
				SessionKey: sessionKey,
				Objective:  "finish after explicit input",
				Status:     protocol.GoalStatusActive,
				Metadata: map[string]any{
					protocol.GoalMetadataExecutionID: "execution-final-dispatch-race",
				},
			},
			RoundID:        "goal_continuation_race",
			Prompt:         "hidden continuation must defer",
			HiddenFromUser: true,
			Synthetic:      true,
			Purpose:        "goal_continuation",
		}},
		validated: make(chan struct{}),
	}
	service.SetGoalContextProvider(goalProvider)

	explicitDone := make(chan error, 1)
	go func() {
		explicitDone <- service.HandleChat(context.Background(), Request{
			SessionKey: sessionKey,
			Content:    "explicit user input wins the dispatch boundary",
			RoundID:    "round-explicit",
		})
	}()
	select {
	case <-quota.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("explicit input did not enter the serialized dispatch boundary")
	}

	continuationDone := make(chan struct{})
	go func() {
		(&roundRunner{
			service:    service,
			agent:      agentValue,
			sessionKey: sessionKey,
			roundID:    "round-before-race",
		}).dispatchGoalContinuation(context.Background())
		close(continuationDone)
	}()
	select {
	case <-goalProvider.validated:
	case <-time.After(2 * time.Second):
		t.Fatal("Goal continuation did not reach its pre-dispatch validation")
	}

	close(quota.release)
	if err := <-explicitDone; err != nil {
		t.Fatalf("explicit HandleChat() error = %v", err)
	}
	select {
	case prompt := <-queryStarted:
		if !strings.Contains(prompt, "explicit user input wins the dispatch boundary") {
			t.Fatalf("runtime prompt = %q, want explicit input", prompt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("explicit input did not start runtime")
	}
	select {
	case <-continuationDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Goal continuation did not finish its final dispatch check")
	}

	client.mu.Lock()
	queryPrompts := append([]string(nil), client.queryPrompts...)
	client.mu.Unlock()
	if len(queryPrompts) != 1 || strings.Contains(queryPrompts[0], "hidden continuation must defer") {
		t.Fatalf("runtime queries = %#v, want only the explicit input", queryPrompts)
	}

	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeResult,
		SessionID: client.sessionID,
		UUID:      "result-explicit-race",
		Result: &sdkprotocol.ResultMessage{
			Subtype:    "success",
			DurationMS: 1,
			NumTurns:   1,
			Result:     "done",
		},
	}
	waitForDMRuntimeIdle(t, runtimeManager, sessionKey)
}

func TestDMGoalContinuationRequestAllowsGoalOnlyAuthority(t *testing.T) {
	request := Request{
		SessionKey:            "agent:nexus:ws:dm:goal-binding",
		GoalContext:           "continue",
		GoalID:                "goal-dm",
		GoalObjectiveRevision: 1,
		Internal:              true,
		InputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose: "goal_continuation",
		},
	}
	if _, _, err := (&Service{}).validateRequest(request); err != nil {
		t.Fatalf("Goal-only continuation request rejected: %v", err)
	}
	request.ExecutionID = "execution-dm"
	if _, _, err := (&Service{}).validateRequest(request); err != nil {
		t.Fatalf("Goal-bound continuation request rejected: %v", err)
	}
}

func TestServiceEnsureClientSkipsGoalRuntimeContextInPlanMode(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	agentValue, err := agentService.GetAgent(context.Background(), cfg.DefaultAgentID)
	if err != nil {
		t.Fatalf("读取默认 agent 失败: %v", err)
	}

	permission := permissionctx.NewContext()
	factory := &fakeDMFactory{client: newFakeDMClient()}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	goalProvider := &fakeGoalContextProvider{
		runtimeContext: "should not enter plan mode",
		runtimeGoal: &protocol.Goal{
			ID:         "goal-plan-context",
			SessionKey: "agent:nexus:ws:dm:test-plan-context",
			Status:     protocol.GoalStatusActive,
		},
	}
	service.SetGoalContextProvider(goalProvider)

	sessionKey := protocol.BuildAgentSessionKey(cfg.DefaultAgentID, protocol.SessionChannelWebSocketSegment, "dm", "plan-context", "")
	t.Cleanup(func() {
		_ = runtimeManager.CloseSession(context.Background(), sessionKey)
	})
	parsed := protocol.ParseSessionKey(sessionKey)
	sessionItem, err := service.ensureSession(context.Background(), agentValue, parsed, sessionKey)
	if err != nil {
		t.Fatalf("初始化 session 失败: %v", err)
	}
	preparation, err := service.ensureClient(context.Background(), sessionKey, agentValue, sessionItem, Request{
		SessionKey:     sessionKey,
		PermissionMode: sdkpermission.ModePlan,
	})
	if err != nil {
		t.Fatalf("构建 plan mode runtime client 失败: %v", err)
	}
	if preparation.permissionMode != sdkpermission.ModePlan {
		t.Fatalf("permissionMode = %q, want plan", preparation.permissionMode)
	}
	if preparation.goalIDForUsage != "" || preparation.goalContext != "" {
		t.Fatalf("plan mode goal runtime context = (%q, %q), want empty", preparation.goalIDForUsage, preparation.goalContext)
	}
	if calls := goalProvider.runtimeContextCallCount(); calls != 0 {
		t.Fatalf("plan mode should not read Goal runtime context, calls = %d", calls)
	}
}

func TestServiceEnsureClientDoesNotBindAmbientBudgetLimitedGoal(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	agentValue, err := agentService.GetAgent(context.Background(), cfg.DefaultAgentID)
	if err != nil {
		t.Fatalf("读取默认 agent 失败: %v", err)
	}

	permission := permissionctx.NewContext()
	factory := &fakeDMFactory{client: newFakeDMClient()}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	goalProvider := &fakeGoalContextProvider{
		runtimeGoal: &protocol.Goal{
			ID:         "goal-budget-limited",
			SessionKey: "agent:nexus:ws:dm:test-budget-limited",
			Status:     protocol.GoalStatusBudgetLimited,
		},
	}
	service.SetGoalContextProvider(goalProvider)

	sessionKey := protocol.BuildAgentSessionKey(cfg.DefaultAgentID, protocol.SessionChannelWebSocketSegment, "dm", "budget-limited", "")
	t.Cleanup(func() {
		_ = runtimeManager.CloseSession(context.Background(), sessionKey)
	})
	parsed := protocol.ParseSessionKey(sessionKey)
	sessionItem, err := service.ensureSession(context.Background(), agentValue, parsed, sessionKey)
	if err != nil {
		t.Fatalf("初始化 session 失败: %v", err)
	}
	preparation, err := service.ensureClient(context.Background(), sessionKey, agentValue, sessionItem, Request{
		SessionKey:     sessionKey,
		PermissionMode: sdkpermission.ModeDefault,
	})
	if err != nil {
		t.Fatalf("构建 runtime client 失败: %v", err)
	}
	if preparation.goalIDForUsage != "" || preparation.goalContext != "" {
		t.Fatalf("ordinary round budget_limited Goal runtime = (%q, %q), want unbound", preparation.goalIDForUsage, preparation.goalContext)
	}
	if calls := goalProvider.runtimeContextCallCount(); calls != 0 {
		t.Fatalf("ordinary round should not read ambient Goal runtime context, calls=%d", calls)
	}
}

func TestServiceDuplicateGoalContinuationDispatchKeepsClaimedCount(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)
	agentService := newDMAgentService(t, cfg)
	client := newFakeDMClient()
	queryStarted := make(chan struct{}, 1)
	client.onQuery = func(context.Context, string) {
		queryStarted <- struct{}{}
	}
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client})
	service := NewService(cfg, agentService, runtimeManager, permissionctx.NewContext())
	sessionKey := "agent:nexus:ws:dm:duplicate-goal-continuation"
	t.Cleanup(func() {
		_ = runtimeManager.CloseSession(context.Background(), sessionKey)
	})
	plan := protocol.GoalContinuation{
		Goal: protocol.Goal{
			ID:         "goal-duplicate-dispatch",
			SessionKey: sessionKey,
			Status:     protocol.GoalStatusActive,
			Metadata: map[string]any{
				protocol.GoalMetadataExecutionID: "execution-duplicate-dispatch",
			},
		},
		RoundID:        "goal_continuation_duplicate",
		Prompt:         "continue once",
		HiddenFromUser: true,
		Synthetic:      true,
		Purpose:        "goal_continuation",
	}
	goalProvider := &fakeGoalContextProvider{
		reservation:       true,
		continuationCount: 1,
		runtimeContext:    "continue once",
		runtimeGoal:       &plan.Goal,
	}
	service.SetGoalContextProvider(goalProvider)

	if err := service.DispatchGoalContinuation(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	select {
	case <-queryStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first continuation did not start runtime")
	}
	if err := service.DispatchGoalContinuation(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	goalProvider.mu.Lock()
	claimCalls := goalProvider.claimCalls
	startedCalls := goalProvider.startedCalls
	releaseCalls := goalProvider.releaseCalls
	count := goalProvider.continuationCount
	reserved := goalProvider.reservation
	goalProvider.mu.Unlock()
	if claimCalls != 1 || startedCalls != 1 || releaseCalls != 1 || count != 1 || reserved {
		t.Fatalf("claim=%d started=%d release=%d count=%d reserved=%v, want one settled start and duplicate no-op release", claimCalls, startedCalls, releaseCalls, count, reserved)
	}

	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeResult,
		SessionID: client.sessionID,
		UUID:      "result-goal-continuation-duplicate",
		Result: &sdkprotocol.ResultMessage{
			Subtype:    "success",
			DurationMS: 1,
			NumTurns:   1,
			Result:     "done",
		},
	}
	waitForDMRuntimeIdle(t, runtimeManager, sessionKey)
}

func TestServiceGoalContinuationClaimsBeforeLaunchAndBindsPlanRevision(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)
	agentService := newDMAgentService(t, cfg)
	client := newFakeDMClient()
	queryStarted := make(chan struct{}, 1)
	client.onQuery = func(context.Context, string) {
		queryStarted <- struct{}{}
	}
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client})
	service := NewService(cfg, agentService, runtimeManager, permissionctx.NewContext())
	sessionKey := "agent:nexus:ws:dm:claim-before-launch"
	t.Cleanup(func() {
		_ = runtimeManager.CloseSession(context.Background(), sessionKey)
	})
	plan := protocol.GoalContinuation{
		Goal: protocol.Goal{
			ID:         "goal-claim-before-launch",
			SessionKey: sessionKey,
			Objective:  "old objective",
			Status:     protocol.GoalStatusActive,
			Metadata: map[string]any{
				protocol.GoalMetadataObjectiveRevision: int64(1),
				protocol.GoalMetadataExecutionID:       "execution-claim-before-launch",
			},
		},
		ExecutionID:    "execution-claim-before-launch",
		RoundID:        "goal_continuation_claim_before_launch",
		Prompt:         "continue the old objective",
		HiddenFromUser: true,
		Synthetic:      true,
		Purpose:        "goal_continuation",
	}
	goalProvider := &fakeGoalContextProvider{
		reservation:       true,
		continuationCount: 1,
		runtimeContext:    "old objective runtime context",
		runtimeGoal: &protocol.Goal{
			ID:         plan.Goal.ID,
			SessionKey: sessionKey,
			Objective:  plan.Goal.Objective,
			Status:     protocol.GoalStatusActive,
			Metadata: map[string]any{
				protocol.GoalMetadataObjectiveRevision: int64(1),
				protocol.GoalMetadataExecutionID:       "execution-claim-before-launch",
			},
		},
	}
	service.SetGoalContextProvider(goalProvider)
	roundContext := make(chan nexusmcp.RoundContext, 1)
	service.SetNexusMCPServerBuilder(func(
		ctx context.Context,
		round nexusmcp.RoundContext,
	) (map[string]sdkmcp.ServerConfig, error) {
		goalProvider.mu.Lock()
		claimCalls := goalProvider.claimCalls
		goalProvider.mu.Unlock()
		if claimCalls != 1 {
			t.Errorf("runtime command builder observed claimCalls=%d, want claim before launch", claimCalls)
		}
		if lease, ok := runtimectx.RuntimeRoundLeaseFromContext(ctx); !ok || lease.SessionKey != sessionKey || lease.RoundID != plan.RoundID {
			t.Errorf("runtime command lease = %#v, want session=%q round=%q", lease, sessionKey, plan.RoundID)
		}
		roundContext <- round
		return nil, nil
	})

	if err := service.DispatchGoalContinuation(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	var commandRound nexusmcp.RoundContext
	select {
	case commandRound = <-roundContext:
	case <-time.After(2 * time.Second):
		t.Fatal("runtime did not build command MCP server")
	}
	goalAuthority := commandRound.CommandContext.GoalAuthority
	state := goalAuthority.ObjectiveRevisionState()
	if state == nil || state.Load() != plan.Goal.ObjectiveRevision() {
		t.Fatalf("runtime revision = %v, want plan revision %d", state, plan.Goal.ObjectiveRevision())
	}
	if goalAuthority == nil || commandRound.CommandContext.ResponsibilityAuthority == nil ||
		commandRound.CommandContext.ResponsibilityAuthority.GoalAuthorityState() != goalAuthority ||
		state != goalAuthority.ObjectiveRevisionState() {
		t.Fatal("DM Goal and Execution commands did not share one round authority state")
	}
	authority, ok := goalAuthority.Load()
	if !ok || authority.GoalID != plan.Goal.ID ||
		authority.ObjectiveRevision != plan.Goal.ObjectiveRevision() ||
		authority.ExecutionID != plan.ExecutionID {
		t.Fatalf("DM continuation authority = %#v, ok=%t", authority, ok)
	}
	select {
	case <-queryStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("claimed continuation did not start runtime")
	}
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeResult,
		SessionID: client.sessionID,
		UUID:      "result-claim-before-launch",
		Result: &sdkprotocol.ResultMessage{
			Subtype:    "success",
			DurationMS: 1,
			NumTurns:   1,
			Result:     "done",
		},
	}
	waitForDMRuntimeIdle(t, runtimeManager, sessionKey)
	goalProvider.mu.Lock()
	progressRevisions := append([]int64(nil), goalProvider.progressRevisions...)
	goalProvider.mu.Unlock()
	if !slices.Equal(progressRevisions, []int64{plan.Goal.ObjectiveRevision()}) {
		t.Fatalf("progress revisions = %v, want plan revision", progressRevisions)
	}
}

func TestServiceGoalContinuationRejectsRetargetAfterClaimBeforeRuntimeLaunch(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)
	agentService := newDMAgentService(t, cfg)
	client := newFakeDMClient()
	queryStarted := make(chan struct{}, 1)
	client.onQuery = func(context.Context, string) {
		queryStarted <- struct{}{}
	}
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client})
	service := NewService(cfg, agentService, runtimeManager, permissionctx.NewContext())
	sessionKey := "agent:nexus:ws:dm:retarget-after-claim"
	t.Cleanup(func() {
		_ = runtimeManager.CloseSession(context.Background(), sessionKey)
	})
	plan := protocol.GoalContinuation{
		Goal: protocol.Goal{
			ID:         "goal-retarget-after-claim",
			SessionKey: sessionKey,
			Objective:  "old objective",
			Status:     protocol.GoalStatusActive,
			Metadata: map[string]any{
				protocol.GoalMetadataObjectiveRevision: int64(1),
				protocol.GoalMetadataExecutionID:       "execution-retarget-after-claim",
			},
		},
		RoundID:        "goal_continuation_retarget_after_claim",
		Prompt:         "continue the old objective",
		HiddenFromUser: true,
		Synthetic:      true,
		Purpose:        "goal_continuation",
	}
	goalProvider := &fakeGoalContextProvider{
		reservation:       true,
		continuationCount: 1,
		runtimeContext:    "old objective runtime context",
		runtimeGoal: &protocol.Goal{
			ID:         plan.Goal.ID,
			SessionKey: sessionKey,
			Objective:  plan.Goal.Objective,
			Status:     protocol.GoalStatusActive,
			Metadata: map[string]any{
				protocol.GoalMetadataObjectiveRevision: int64(1),
				protocol.GoalMetadataExecutionID:       "execution-retarget-after-claim",
			},
		},
	}
	goalProvider.onClaim = func() {
		goalProvider.mu.Lock()
		goalProvider.runtimeGoal.Objective = "new objective"
		goalProvider.runtimeGoal.Metadata[protocol.GoalMetadataObjectiveRevision] = int64(2)
		goalProvider.mu.Unlock()
	}
	service.SetGoalContextProvider(goalProvider)
	mcpBuilt := make(chan struct{}, 1)
	service.SetMCPServerBuilder(func(
		_ context.Context,
		_ *protocol.Agent,
		_ string,
		_ string,
		_ string,
		_ string,
		_ string,
		_ *atomic.Int64,
		_ sdkpermission.Mode,
	) map[string]sdkmcp.ServerConfig {
		mcpBuilt <- struct{}{}
		return nil
	})

	err := service.DispatchGoalContinuation(context.Background(), plan)
	if !errors.Is(err, goalsvc.ErrGoalRevisionStale) {
		t.Fatalf("DispatchGoalContinuation() error = %v, want ErrGoalRevisionStale", err)
	}
	goalProvider.mu.Lock()
	claimCalls := goalProvider.claimCalls
	failures := append([]string(nil), goalProvider.failures...)
	progress := append([]bool(nil), goalProvider.progress...)
	goalProvider.mu.Unlock()
	if claimCalls != 1 {
		t.Fatalf("claim calls = %d, want 1", claimCalls)
	}
	if len(failures) != 0 || len(progress) != 0 {
		t.Fatalf("stale launch mutated retargeted Goal: failures=%v progress=%v", failures, progress)
	}
	select {
	case <-mcpBuilt:
		t.Fatal("retargeted continuation must stop before MCP/runtime setup")
	default:
	}
	select {
	case <-queryStarted:
		t.Fatal("retargeted continuation must stop before querying the model")
	default:
	}
}

func TestServiceGoalContinuationPauseAfterRuntimeRegistrationFailsAdmissionBeforeQuery(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)
	client := newFakeDMClient()
	var queryCalls atomic.Int64
	client.onQuery = func(context.Context, string) { queryCalls.Add(1) }
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client})
	service := NewService(cfg, newDMAgentService(t, cfg), runtimeManager, permissionctx.NewContext())
	sessionKey := "agent:nexus:ws:dm:pause-at-start-admission"
	plan := protocol.GoalContinuation{
		Goal: protocol.Goal{
			ID: "goal-pause-at-start-admission", SessionKey: sessionKey,
			Objective: "old objective", Status: protocol.GoalStatusActive,
			Metadata: map[string]any{protocol.GoalMetadataObjectiveRevision: int64(1)},
		},
		RoundID: "goal_continuation_pause_at_start_admission", Prompt: "continue",
		HiddenFromUser: true, Synthetic: true, Purpose: "goal_continuation",
	}
	provider := &fakeGoalContextProvider{
		reservation: true, continuationCount: 1, startedErr: goalsvc.ErrGoalRevisionStale,
		runtimeGoal: &plan.Goal,
	}
	registered := make(chan struct{})
	releaseAdmission := make(chan struct{})
	provider.beforeStarted = func() {
		close(registered)
		<-releaseAdmission
	}
	provider.onStarted = func() {
		if ids := runtimeManager.GetRunningRoundIDs(sessionKey); !slices.Equal(ids, []string{plan.RoundID}) {
			t.Errorf("start admission saw runtime rounds %v, want exact registered round", ids)
		}
		if ids := runtimeManager.GoalAccountingRoundIDs(sessionKey, plan.Goal.ID); !slices.Equal(ids, []string{plan.RoundID}) {
			t.Errorf("start admission saw Goal accounting rounds %v, want exact registered round", ids)
		}
	}
	service.SetGoalContextProvider(provider)

	dispatchResult := make(chan error, 1)
	go func() {
		dispatchResult <- service.DispatchGoalContinuation(context.Background(), plan)
	}()
	select {
	case <-registered:
	case <-time.After(2 * time.Second):
		t.Fatal("continuation did not reach registered start admission")
	}
	if ids := runtimeManager.GetRunningRoundIDs(sessionKey); !slices.Equal(ids, []string{plan.RoundID}) {
		t.Fatalf("before pause runtime rounds = %v, want exact registered round", ids)
	}
	// Models the lifecycle transaction cancelling the claimed receipt while the
	// runtime setup path is paused at its final admission fence.
	close(releaseAdmission)
	err := <-dispatchResult
	if !errors.Is(err, goalsvc.ErrGoalRevisionStale) {
		t.Fatalf("DispatchGoalContinuation() error = %v, want stale admission", err)
	}
	if got := runtimeManager.GetRunningRoundIDs(sessionKey); len(got) != 0 {
		t.Fatalf("stale admission left running round: %v", got)
	}
	if got := queryCalls.Load(); got != 0 {
		t.Fatalf("stale admission queried model %d times, want 0", got)
	}
	for _, message := range readDMSessionHistory(t, cfg, service, sessionKey) {
		if dmdomain.NormalizeString(message["round_id"]) == plan.RoundID {
			t.Fatalf("stale admission left a phantom continuation marker: %#v", message)
		}
	}
	sessionValue, workspacePath := mustFindDMSession(t, service, cfg, sessionKey)
	index, indexErr := workspacestore.NewAgentHistoryStore(cfg.WorkspacePath).ReadRoundIndex(
		workspacePath,
		sessionValue,
		nil,
	)
	if indexErr != nil {
		t.Fatal(indexErr)
	}
	for _, item := range index.Items {
		if item.RoundID == plan.RoundID {
			t.Fatalf("stale admission left a round-index projection: %#v", item)
		}
	}
	provider.mu.Lock()
	startedCalls := provider.startedCalls
	provider.mu.Unlock()
	if startedCalls != 1 {
		t.Fatalf("start admission calls = %d, want 1", startedCalls)
	}
}

func TestServiceGoalContinuationDefersToQueuedUserInput(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	sentPrompt := make(chan string, 1)
	client.onQuery = func(_ context.Context, prompt string) {
		sentPrompt <- prompt
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-goal-defer-queue",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "queued done",
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sessionKey := "agent:nexus:ws:dm:test-goal-defer-queue"
	t.Cleanup(func() {
		_ = runtimeManager.CloseSession(context.Background(), sessionKey)
	})
	normalizedSessionKey, location, err := service.resolveInputQueueLocation(context.Background(), sessionKey, cfg.DefaultAgentID)
	if err != nil {
		t.Fatal(err)
	}
	if normalizedSessionKey != sessionKey {
		t.Fatalf("normalized session key = %q, want %q", normalizedSessionKey, sessionKey)
	}
	if _, err = service.inputQueue.Enqueue(location, protocol.InputQueueItem{
		Scope:          protocol.InputQueueScopeDM,
		SessionKey:     sessionKey,
		AgentID:        cfg.DefaultAgentID,
		Source:         protocol.InputQueueSourceUser,
		Content:        "用户排队输入应先执行",
		DeliveryPolicy: protocol.ChatDeliveryPolicyQueue,
	}); err != nil {
		t.Fatal(err)
	}

	if !service.ShouldDeferGoalContinuation(context.Background(), sessionKey, cfg.DefaultAgentID) {
		t.Fatal("Goal continuation should defer while queued user input exists")
	}
	select {
	case prompt := <-sentPrompt:
		if !strings.Contains(prompt, "用户排队输入应先执行") {
			t.Fatalf("prompt = %q, want queued user input", prompt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("queued user input was not dispatched before Goal continuation")
	}
	items, err := service.inputQueue.Snapshot(location)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("items = %#v, want queued input dispatched", items)
	}
	waitForDMRuntimeIdle(t, runtimeManager, sessionKey)
}

func TestServiceGoalContinuationDefersInPlanMode(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	if _, err := agentService.UpdateAgent(context.Background(), cfg.DefaultAgentID, protocol.UpdateRequest{
		Options: &protocol.Options{PermissionMode: string(sdkpermission.ModePlan)},
	}); err != nil {
		t.Fatalf("更新 agent plan mode 失败: %v", err)
	}
	service := NewService(cfg, agentService, runtimectx.NewManager(), permissionctx.NewContext())
	sessionKey := "agent:nexus:ws:dm:test-goal-defer-plan"

	if !service.ShouldDeferGoalContinuation(context.Background(), sessionKey, cfg.DefaultAgentID) {
		t.Fatal("Goal continuation should defer while the target agent is in plan mode")
	}
}

func TestServiceGoalContinuationDefersForSessionPlanOverride(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	service := NewService(
		cfg,
		agentService,
		runtimectx.NewManager(),
		permissionctx.NewContext(),
	)
	sessionKey := "agent:nexus:ws:dm:test-goal-session-plan"
	now := time.Now().UTC()
	if _, err := service.files.UpsertSession(
		dmMainWorkspacePath(cfg),
		protocol.Session{
			SessionKey:   sessionKey,
			AgentID:      cfg.DefaultAgentID,
			ChannelType:  protocol.SessionChannelWebSocket,
			ChatType:     protocol.RoomTypeDM,
			Status:       "closed",
			CreatedAt:    now,
			LastActivity: now,
			Title:        "Session Plan",
			Options: protocol.WithSessionRuntimeSettings(
				nil,
				protocol.SessionRuntimeSettings{
					PermissionMode: string(sdkpermission.ModePlan),
				},
			),
		},
	); err != nil {
		t.Fatalf("写入 Session plan override 失败: %v", err)
	}

	if !service.ShouldDeferGoalContinuation(
		context.Background(),
		sessionKey,
		cfg.DefaultAgentID,
	) {
		t.Fatal("Session plan override 应阻止 Goal 自动续跑")
	}
}

// goal_lifecycle_test.go 覆盖 DM Goal 续跑与终态结算测试。
package dm

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/infra/textutil"
	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	_ "modernc.org/sqlite"
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

func TestServiceGoalContinuationFinalDispatchDefersToConcurrentExplicitInput(t *testing.T) {
	cfg := newDMTestConfig(t)

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
	service.SetInboxStore(newDMInboxStore(t, cfg))
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

func TestServiceGoalContinuationClaimsBeforeLaunchAndBindsPlanRevision(t *testing.T) {
	cfg := newDMTestConfig(t)
	agentService := newDMAgentService(t, cfg)
	client := newFakeDMClient()
	queryStarted := make(chan struct{}, 1)
	client.onQuery = func(context.Context, string) {
		queryStarted <- struct{}{}
	}
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client})
	service := NewService(cfg, agentService, runtimeManager, permissionctx.NewContext())
	service.SetInboxStore(newDMInboxStore(t, cfg))
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
	if commandRound.SourceContextType != runtimectx.SourceContextGoalContinuation {
		t.Fatalf("DM continuation source context = %q, want %q", commandRound.SourceContextType, runtimectx.SourceContextGoalContinuation)
	}
	continuationAuthority := commandRound.CommandContext.GoalContinuationAuthority
	if continuationAuthority == nil || !continuationAuthority.Valid() ||
		strings.TrimSpace(continuationAuthority.OwnerUserID) == "" ||
		continuationAuthority.AgentID != "nexus" ||
		continuationAuthority.ScopeSessionKey != sessionKey ||
		continuationAuthority.GoalID != plan.Goal.ID ||
		continuationAuthority.ObjectiveRevision != plan.Goal.ObjectiveRevision() ||
		continuationAuthority.ExecutionID != plan.ExecutionID ||
		continuationAuthority.RootRoundID != plan.RoundID {
		t.Fatalf("DM continuation authority = %+v", continuationAuthority)
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
	agentService := newDMAgentService(t, cfg)
	client := newFakeDMClient()
	queryStarted := make(chan struct{}, 1)
	client.onQuery = func(context.Context, string) {
		queryStarted <- struct{}{}
	}
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client})
	service := NewService(cfg, agentService, runtimeManager, permissionctx.NewContext())
	service.SetInboxStore(newDMInboxStore(t, cfg))
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
	client := newFakeDMClient()
	var queryCalls atomic.Int64
	client.onQuery = func(context.Context, string) { queryCalls.Add(1) }
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client})
	service := NewService(cfg, newDMAgentService(t, cfg), runtimeManager, permissionctx.NewContext())
	service.SetInboxStore(newDMInboxStore(t, cfg))
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
		if textutil.AnyString(message["round_id"]) == plan.RoundID {
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

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	sentPrompt := make(chan string, 1)
	client.onQuery = func(_ context.Context, prompt string) {
		sentPrompt <- prompt
		client.sendResultAsync(client.sessionID, "result-goal-defer-queue", "success", "queued done")
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetInboxStore(newDMInboxStore(t, cfg))
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
	if _, err = service.InputQueue.Enqueue(location, protocol.InputQueueItem{
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
	items, err := service.InputQueue.Snapshot(location)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("items = %#v, want queued input dispatched", items)
	}
	waitForDMRuntimeIdle(t, runtimeManager, sessionKey)
}

func TestServiceGoalContinuationDefersForSessionPlanOverride(t *testing.T) {
	cfg := newDMTestConfig(t)

	agentService := newDMAgentService(t, cfg)
	service := NewService(
		cfg,
		agentService,
		runtimectx.NewManager(),
		permissionctx.NewContext(),
	)
	service.SetInboxStore(newDMInboxStore(t, cfg))
	sessionKey := "agent:nexus:ws:dm:test-goal-session-plan"
	now := time.Now().UTC()
	if _, err := service.Files.UpsertSession(
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

func TestDMParentTerminalUsageBackgroundRetryRecoversWithoutChildOrNewMessage(t *testing.T) {
	const (
		sessionKey = "agent:nexus:ws:dm:parent-terminal-background-retry"
		goalID     = "goal-parent-terminal-background-retry"
		roundID    = "round-parent-terminal-background-retry"
	)
	provider := newRetryingDMGoalUsageProvider(sessionKey, goalID, 0)
	provider.claimFailuresRemaining = runtimehost.GoalUsagePersistAttempts * 2
	provider.finalizeFailuresRemaining = runtimehost.GoalUsagePersistAttempts * 2
	var dispatches atomic.Int64
	runner := &roundRunner{
		service:               &Service{goals: provider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
		sessionKey:            sessionKey,
		roundID:               roundID,
		ownerUserID:           "owner-parent-terminal-background-retry",
		GoalRoundState:        runtimehost.GoalRoundState{IDForUsage: goalID, ChildIDForUsage: goalID, Usage: goalsvc.NewRuntimeUsageAccumulator(true), UsageClaimPending: true},
		runtimeKind:           "nxs",
		postRoundDispatchHook: func() { dispatches.Add(1) },
	}
	accelerateDMGoalUsageRetry(runner)
	result := exec.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  12,
			OutputTokens: 3,
			TotalTokens:  15,
		},
	}

	// The first synchronous window exhausts while the parent is still running.
	// A manually requested worker must not settle or dispatch before terminal.
	runner.finalizeGoalUsage(context.Background(), result, nil)
	if attempts := provider.claimAttemptCount(); attempts != runtimehost.GoalUsagePersistAttempts {
		t.Fatalf("claim attempts before parent terminal = %d, want %d", attempts, runtimehost.GoalUsagePersistAttempts)
	}
	runner.startGoalUsageRetryWorker()
	waitForDMGoalUsageRetry(t, func() bool {
		runner.Mu.Lock()
		defer runner.Mu.Unlock()
		return !runner.UsageRetrying
	})
	if attempts := provider.claimAttemptCount(); attempts != runtimehost.GoalUsagePersistAttempts {
		t.Fatalf("parent-active retry advanced claim attempts to %d", attempts)
	}
	if got := dispatches.Load(); got != 0 {
		t.Fatalf("parent-active post-round dispatches = %d, want 0", got)
	}

	// No child and no subsequent runtime message arrives. The terminal join
	// exhausts its second synchronous window, then the single background worker
	// must recover the claim, terminal delta, finalization fence, and dispatch.
	runner.markSubagentParentTerminal(subagentParentTerminalNormal)
	if runner.completeSubagentJoinAfterParentTerminal() {
		t.Fatal("terminal usage unexpectedly settled inside the exhausted synchronous window")
	}
	var starters sync.WaitGroup
	for range 16 {
		starters.Add(1)
		go func() {
			defer starters.Done()
			runner.startGoalUsageRetryWorker()
		}()
	}
	starters.Wait()

	waitForDMGoalUsageRetry(t, func() bool {
		return dispatches.Load() == 1 &&
			provider.finalizeCallCount() == runtimehost.GoalUsagePersistAttempts*2+1 &&
			dmGoalUsageRetryStopped(runner)
	})
	if attempts := provider.claimAttemptCount(); attempts != runtimehost.GoalUsagePersistAttempts*2+1 {
		t.Fatalf("claim attempts = %d, want two failed windows plus one background success", attempts)
	}
	if calls := provider.finalizeCallCount(); calls != runtimehost.GoalUsagePersistAttempts*2+1 {
		t.Fatalf("finalization calls = %d, want two failed windows plus one background success", calls)
	}
	deltas := provider.finalizedDeltas()
	if len(deltas) == 0 || deltas[len(deltas)-1].ActualTokens() != 15 {
		t.Fatalf("terminal finalization deltas = %#v, want retained provider total 15", deltas)
	}
	if got := dispatches.Load(); got != 1 {
		t.Fatalf("post-round dispatches after terminal recovery = %d, want exactly 1", got)
	}
}

func TestDMParentTerminalUsageBackgroundRetryNeverDispatchesAbnormalRound(t *testing.T) {
	for _, terminal := range []string{
		subagentParentTerminalFailed,
		subagentParentTerminalInterrupted,
	} {
		t.Run(terminal, func(t *testing.T) {
			const (
				sessionKey = "agent:nexus:ws:dm:abnormal-parent-terminal-retry"
				goalID     = "goal-abnormal-parent-terminal-retry"
				roundID    = "round-abnormal-parent-terminal-retry"
			)
			provider := newRetryingDMGoalUsageProvider(sessionKey, goalID, 0)
			provider.finalizeFailuresRemaining = runtimehost.GoalUsagePersistAttempts * 2
			var dispatches atomic.Int64
			runner := &roundRunner{
				service:               &Service{goals: provider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
				sessionKey:            sessionKey,
				roundID:               roundID,
				GoalRoundState:        runtimehost.GoalRoundState{IDForUsage: goalID, ChildIDForUsage: goalID, Usage: goalsvc.NewRuntimeUsageAccumulator(true)},
				postRoundDispatchHook: func() { dispatches.Add(1) },
			}
			accelerateDMGoalUsageRetry(runner)
			runner.finalizeGoalUsage(context.Background(), exec.RoundExecutionResult{
				Usage: sdkprotocol.TokenUsage{InputTokens: 9, OutputTokens: 1, TotalTokens: 10},
			}, nil)
			runner.markSubagentParentTerminal(terminal)
			if runner.completeSubagentJoinAfterParentTerminal() {
				t.Fatal("abnormal terminal usage unexpectedly settled inside the exhausted synchronous window")
			}

			waitForDMGoalUsageRetry(t, func() bool {
				return provider.finalizeCallCount() == runtimehost.GoalUsagePersistAttempts*2+1
			})
			if got := dispatches.Load(); got != 0 {
				t.Fatalf("abnormal parent post-round dispatches = %d, want 0", got)
			}
			runner.Mu.Lock()
			dispatched := runner.subagentPostRoundDispatched
			runner.Mu.Unlock()
			if dispatched {
				t.Fatal("abnormal parent claimed normal post-round dispatch")
			}
		})
	}
}

func TestDMSubagentUsageBackgroundRetryAlsoRetriesFinalizationUntilDispatch(t *testing.T) {
	const (
		sessionKey = "agent:nexus:ws:dm:finalize-background-retry"
		goalID     = "goal-finalize-background-retry"
		roundID    = "round-finalize-background-retry"
	)
	provider := newRetryingDMGoalUsageProvider(sessionKey, goalID, 6)
	provider.finalizeFailuresRemaining = 6
	var dispatches atomic.Int64
	runner := &roundRunner{
		service:                &Service{goals: provider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
		sessionKey:             sessionKey,
		roundID:                roundID,
		ownerUserID:            "owner-finalize-background-retry",
		GoalRoundState:         runtimehost.GoalRoundState{IDForUsage: goalID, ChildIDForUsage: goalID, Usage: goalsvc.NewRuntimeUsageAccumulator(true)},
		runtimeKind:            "nxs",
		goalTokenUsageObserved: true,
		postRoundDispatchHook:  func() { dispatches.Add(1) },
	}
	accelerateDMGoalUsageRetry(runner)
	runner.markSubagentParentTerminal(subagentParentTerminalNormal)
	runner.rememberSubagentTaskMessage(dmSubagentTaskMessage("task_started", "running"))
	terminalMessage := dmSubagentTaskMessage("task_notification", "completed")
	terminalMessage["metadata"].(map[string]any)["usage"] = map[string]any{"total_tokens": int64(100)}

	runner.recordSubagentGoalUsage(context.Background(), terminalMessage)
	runner.rememberSubagentTaskMessage(terminalMessage)

	waitForDMGoalUsageRetry(t, func() bool {
		return dispatches.Load() == 1 && dmGoalUsageRetryStopped(runner)
	})
	if calls := provider.finalizeCallCount(); calls != 7 {
		t.Fatalf("finalization calls = %d, want 6 failures plus one background success", calls)
	}
	if got := dispatches.Load(); got != 1 {
		t.Fatalf("post-round dispatches after finalization recovery = %d, want exactly 1", got)
	}
}

func TestCompletedDMGoalWaitsForFailedUsageClaimThenFinalizesOnce(t *testing.T) {
	const (
		sessionKey = "agent:nexus:ws:dm:claim-retry"
		goalID     = "goal-claim-retry"
		roundID    = "round-claim-retry"
	)
	provider := &fakeDMGoalUsageFinalizer{
		fakeGoalContextProvider: &fakeGoalContextProvider{},
		report: protocol.GoalUsageReport{
			GoalID:     goalID,
			SessionKey: sessionKey,
			Status:     protocol.GoalStatusComplete,
		},
		claimFailuresRemaining: runtimehost.GoalUsagePersistAttempts * 2,
	}
	runner := &roundRunner{
		service:                &Service{goals: provider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
		sessionKey:             sessionKey,
		roundID:                roundID,
		ownerUserID:            "owner-claim-retry",
		GoalRoundState:         runtimehost.GoalRoundState{IDForUsage: goalID, ChildIDForUsage: goalID, Usage: goalsvc.NewRuntimeUsageAccumulator(true)},
		runtimeKind:            "nxs",
		goalTokenUsageObserved: true,
	}
	accelerateDMGoalUsageRetry(runner)

	runner.claimSubagentGoalUsageRound(context.Background(), goalID)
	if !runner.subagentGoalUsageClaimPending() {
		t.Fatal("failed model-create claim did not remain pending")
	}
	if runner.finalizeCompletedGoalUsageAfterSubagents(context.Background()) {
		t.Fatal("completed Goal finalized while the pending claim kept failing")
	}
	if calls := provider.finalizeCallCount(); calls != 0 {
		t.Fatalf("FinalizeUsageForGoal calls before claim success = %d, want 0", calls)
	}
	if successes := provider.claimSuccessCount(); successes != 0 {
		t.Fatalf("successful claims before recovery = %d, want 0", successes)
	}

	if !runner.finalizeCompletedGoalUsageAfterSubagents(context.Background()) {
		t.Fatal("completed Goal did not finalize after claim persistence recovered")
	}
	if runner.subagentGoalUsageClaimPending() {
		t.Fatal("successful retry left the usage claim pending")
	}
	attempts := provider.claimAttemptCount()
	if attempts != runtimehost.GoalUsagePersistAttempts*2+1 {
		t.Fatalf("claim attempts = %d, want %d failures plus one success", attempts, runtimehost.GoalUsagePersistAttempts*2)
	}
	if successes := provider.claimSuccessCount(); successes != 1 {
		t.Fatalf("successful claims after recovery = %d, want exactly 1", successes)
	}
	if calls := provider.finalizeCallCount(); calls != 1 {
		t.Fatalf("FinalizeUsageForGoal calls after recovery = %d, want exactly 1", calls)
	}

	if !runner.finalizeCompletedGoalUsageAfterSubagents(context.Background()) {
		t.Fatal("repeated claim/finalization join should be an idempotent success")
	}
	if got := provider.claimAttemptCount(); got != attempts {
		t.Fatalf("repeated join retried an already successful claim: attempts %d -> %d", attempts, got)
	}
	if successes := provider.claimSuccessCount(); successes != 1 {
		t.Fatalf("successful claims after repeated join = %d, want exactly 1", successes)
	}
	if calls := provider.finalizeCallCount(); calls != 1 {
		t.Fatalf("FinalizeUsageForGoal calls after repeated join = %d, want exactly 1", calls)
	}
}

func TestCompletedDMGoalFinalizesOnlyAfterRunningChildDrains(t *testing.T) {
	const (
		sessionKey = "agent:nexus:ws:dm:terminal-child"
		goalID     = "goal-terminal-child"
		roundID    = "round-terminal-child"
	)
	base := &fakeGoalContextProvider{
		usageGoal: &protocol.Goal{
			ID:         goalID,
			SessionKey: sessionKey,
			Status:     protocol.GoalStatusComplete,
		},
	}
	provider := &fakeDMGoalUsageFinalizer{
		fakeGoalContextProvider: base,
		report: protocol.GoalUsageReport{
			GoalID:     goalID,
			SessionKey: sessionKey,
			Status:     protocol.GoalStatusComplete,
		},
	}
	runner := &roundRunner{
		service:        &Service{goals: provider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
		sessionKey:     sessionKey,
		roundID:        roundID,
		GoalRoundState: runtimehost.GoalRoundState{IDForUsage: goalID, ChildIDForUsage: goalID, Usage: goalsvc.NewRuntimeUsageAccumulator(true), SubagentTasks: map[string]struct{}{"child-task": {}}},
		runtimeKind:    "nxs",
	}

	runner.finalizeGoalUsage(context.Background(), exec.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  9,
			OutputTokens: 3,
			TotalTokens:  12,
		},
	}, nil)

	usages := base.recordedUsage()
	if len(usages) != 1 || usages[0].BudgetTokens() != 12 || usages[0].ActualTokens() != 12 {
		t.Fatalf("parent terminal usage = %#v, want one persisted 12-token delta", usages)
	}
	if calls := provider.finalizeCallCount(); calls != 0 {
		t.Fatalf("FinalizeUsageForGoal calls with running child = %d, want 0", calls)
	}
	if runner.finalizeCompletedGoalUsageAfterSubagents(context.Background()) {
		t.Fatal("child join finalized while child task was still running")
	}

	childTerminal := protocol.Message{"metadata": map[string]any{
		"task_id":   "child-task",
		"task_type": "local_agent",
		"subtype":   "task_notification",
		"status":    "completed",
		"usage":     map[string]any{"total_tokens": int64(7)},
	}}
	for _, settlement := range runner.recordSubagentGoalUsage(context.Background(), childTerminal) {
		runner.ClearSubagentUsagePending(settlement.TaskID, settlement.Observation)
	}
	runner.rememberSubagentTaskMessage(childTerminal)
	if runner.HasRunningSubagentTask() {
		t.Fatal("child task map still contains completed task")
	}
	if !runner.finalizeCompletedGoalUsageAfterSubagents(context.Background()) {
		t.Fatal("child join did not finalize completed Goal usage")
	}
	if calls := provider.finalizeCallCount(); calls != 1 {
		t.Fatalf("FinalizeUsageForGoal calls after child drain = %d, want 1", calls)
	}
	if deltas := provider.finalizedDeltas(); len(deltas) != 1 || !deltas[0].IsZero() {
		t.Fatalf("finalization deltas = %#v, want one zero delta after parent usage was persisted", deltas)
	}

	if !runner.finalizeCompletedGoalUsageAfterSubagents(context.Background()) {
		t.Fatal("repeated finalized join should be an idempotent success")
	}
	if calls := provider.finalizeCallCount(); calls != 1 {
		t.Fatalf("FinalizeUsageForGoal calls after repeated join = %d, want exactly 1", calls)
	}
}

func TestDMUnavailableChildEvidenceStopsWorkerAndReleasesPostRoundOnce(t *testing.T) {
	const (
		sessionKey = "agent:nexus:ws:dm:child-unavailable"
		goalID     = "goal-child-unavailable"
		roundID    = "round-child-unavailable"
	)
	provider := newEvidenceAwareDMGoalProvider(sessionKey, goalID)
	var dispatches atomic.Int64
	runner := &roundRunner{
		service:                &Service{goals: provider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
		ownerUserID:            "owner-child-unavailable",
		sessionKey:             sessionKey,
		roundID:                roundID,
		GoalRoundState:         runtimehost.GoalRoundState{IDForUsage: goalID, ChildIDForUsage: goalID, Usage: goalsvc.NewRuntimeUsageAccumulator(true)},
		goalTokenUsageObserved: true,
		runtimeKind:            "nxs",
		postRoundDispatchHook:  func() { dispatches.Add(1) },
	}
	taskMessage := func(taskID string, subtype string, status string, total any) protocol.Message {
		metadata := map[string]any{
			"task_id":   taskID,
			"task_type": "local_agent",
			"subtype":   subtype,
			"status":    status,
		}
		if total != nil {
			metadata["usage"] = map[string]any{"total_tokens": total}
		}
		return protocol.Message{"metadata": metadata}
	}
	record := func(message protocol.Message) {
		t.Helper()
		for _, settlement := range runner.recordSubagentGoalUsage(context.Background(), message) {
			runner.ClearSubagentUsagePending(
				settlement.TaskID,
				settlement.Observation,
			)
		}
		runner.rememberSubagentTaskMessage(message)
	}

	record(taskMessage("child-observed", "task_started", "running", nil))
	record(taskMessage("child-observed", "task_notification", "completed", int64(100)))
	record(taskMessage("child-missing", "task_started", "running", nil))
	record(taskMessage("child-missing", "task_progress", "running", int64(50)))
	record(taskMessage("child-missing", "task_notification", "completed", int64(0)))
	if runner.HasRunningSubagentTask() {
		t.Fatal("terminal child evidence left a live in-memory join barrier")
	}

	runner.markSubagentParentTerminal(subagentParentTerminalNormal)
	runner.startGoalUsageRetryWorker()
	waitForDMGoalUsageRetry(t, func() bool {
		runner.Mu.Lock()
		running := runner.UsageRetrying
		runner.Mu.Unlock()
		return dispatches.Load() == 1 && !running
	})
	if calls := provider.finalizeCallCount(); calls != 1 {
		t.Fatalf("unavailable finalization calls = %d, want one non-retryable attempt", calls)
	}
	provider.mu.Lock()
	finalized := provider.report.UsageFinalized
	provider.mu.Unlock()
	if finalized {
		t.Fatal("multi-child Goal finalized despite one terminal child missing provider usage")
	}
	if got := provider.childEvidence("child-observed"); !got.terminal || !got.tokenObserved {
		t.Fatalf("observed child evidence = %#v, want authoritative terminal", got)
	}
	if got := provider.childEvidence("child-missing"); !got.terminal || got.tokenObserved {
		t.Fatalf("missing child evidence = %#v, want terminal unavailable", got)
	}

	// A stray worker start after post-round release must not retry the same
	// durable unavailable conclusion or dispatch downstream work twice.
	runner.startGoalUsageRetryWorker()
	waitForDMGoalUsageRetry(t, func() bool {
		runner.Mu.Lock()
		defer runner.Mu.Unlock()
		return !runner.UsageRetrying
	})
	if calls := provider.finalizeCallCount(); calls != 1 {
		t.Fatalf("stray worker retried unavailable finalization: %d calls", calls)
	}
	if got := dispatches.Load(); got != 1 {
		t.Fatalf("post-round dispatches = %d, want exactly one", got)
	}
}

func TestCompletedDMGoalWithoutTokenUsageSettlesWithoutFinalizationFence(t *testing.T) {
	const (
		sessionKey = "agent:nexus:ws:dm:missing-terminal-usage"
		goalID     = "goal-missing-terminal-usage"
	)
	base := &fakeGoalContextProvider{}
	provider := &fakeDMGoalUsageFinalizer{
		fakeGoalContextProvider: base,
		report: protocol.GoalUsageReport{
			GoalID:     goalID,
			SessionKey: sessionKey,
			Status:     protocol.GoalStatusComplete,
		},
	}
	runner := &roundRunner{
		service:        &Service{goals: provider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
		sessionKey:     sessionKey,
		roundID:        "round-missing-terminal-usage",
		GoalRoundState: runtimehost.GoalRoundState{IDForUsage: goalID, Usage: goalsvc.NewRuntimeUsageAccumulator(true), UsageStartedAt: time.Now().Add(-2 * time.Second)},
	}

	runner.finalizeGoalUsage(context.Background(), exec.RoundExecutionResult{}, nil)

	if calls := provider.finalizeCallCount(); calls != 0 {
		t.Fatalf("FinalizeUsageForGoal calls = %d, want 0 without token evidence", calls)
	}
	provider.mu.Lock()
	finalized := provider.report.UsageFinalized
	provider.mu.Unlock()
	if finalized {
		t.Fatal("missing terminal token usage was frozen as authoritative zero")
	}
	usages := base.recordedUsage()
	if len(usages) != 1 || usages[0].RuntimeSeconds <= 0 ||
		usages[0].BudgetTokens() != 0 || usages[0].ActualTokens() != 0 {
		t.Fatalf("persisted usage = %#v, want elapsed-only settlement", usages)
	}
	if runner.Usage.Active() {
		t.Fatal("missing-usage terminal settlement left accumulator active for infinite retry")
	}
}

func TestDMTerminalUsageDurableParentLedgerReplaysWithoutDoubleAttribution(t *testing.T) {
	for _, test := range []struct {
		name              string
		parentCommitError bool
		finalizeError     bool
		wantFinalizeCalls int
	}{
		{
			name:              "parent commit response lost",
			parentCommitError: true,
			wantFinalizeCalls: 1,
		},
		{
			name:              "finalization retry",
			finalizeError:     true,
			wantFinalizeCalls: 2,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			const (
				sessionKey = "agent:nexus:ws:dm:durable-parent-replay"
				goalID     = "goal-durable-parent-replay"
				roundID    = "round-durable-parent-replay"
			)
			provider := newDurableDMParentProvider(sessionKey, goalID)
			if test.parentCommitError {
				provider.parentCommitErrorsRemaining = 1
			}
			if test.finalizeError {
				provider.finalizeFailuresRemaining = 1
			}
			runner := &roundRunner{
				service:        &Service{goals: provider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
				ownerUserID:    "owner-durable-parent-replay",
				sessionKey:     sessionKey,
				roundID:        roundID,
				GoalRoundState: runtimehost.GoalRoundState{IDForUsage: goalID, Usage: goalsvc.NewRuntimeUsageAccumulator(true)},
			}
			snapshot := goalsvc.RuntimeUsageSnapshot{
				Usage: protocol.GoalUsage{
					InputTokens:       12,
					OutputTokens:      3,
					ActualTotalTokens: 15,
					ActualTotalKnown:  true,
				},
				TokenUsageObserved: true,
				Cumulative:         true,
				Terminal:           true,
			}

			if runner.settleTerminalGoalUsageSnapshot(context.Background(), snapshot) {
				t.Fatal("first settlement unexpectedly succeeded despite injected response failure")
			}
			if !runner.settleTerminalGoalUsageSnapshot(context.Background(), snapshot) {
				t.Fatal("terminal replay did not converge")
			}

			if calls := provider.parentCallCount(); calls != 2 {
				t.Fatalf("parent ledger calls = %d, want replayed twice", calls)
			}
			if usage := provider.parentAttributedUsage(); usage.BudgetTokens() != 15 ||
				usage.ActualTokens() != 15 {
				t.Fatalf("durably attributed usage = %#v, want exactly one 15-token terminal", usage)
			}
			if calls := provider.finalizeCallCount(); calls != test.wantFinalizeCalls {
				t.Fatalf("finalization calls = %d, want %d", calls, test.wantFinalizeCalls)
			}
			for _, delta := range provider.finalizedDeltas() {
				if !delta.IsZero() {
					t.Fatalf("finalization delta = %#v, want zero after durable parent attribution", delta)
				}
			}
		})
	}
}

type fakeDMGoalUsageFinalizer struct {
	*fakeGoalContextProvider
	report                 protocol.GoalUsageReport
	finalizeCalls          int
	finalizeDeltas         []protocol.GoalUsage
	claimFailuresRemaining int
	claimAttempts          int
	claimSuccesses         int
}

type durableDMParentProvider struct {
	*fakeDMGoalUsageFinalizer
	parentSnapshots             []protocol.GoalUsageParentSnapshot
	parentLedger                map[string]protocol.GoalUsage
	parentAttributed            protocol.GoalUsage
	parentCommitErrorsRemaining int
	finalizeFailuresRemaining   int
}

func newDurableDMParentProvider(sessionKey string, goalID string) *durableDMParentProvider {
	return &durableDMParentProvider{
		fakeDMGoalUsageFinalizer: &fakeDMGoalUsageFinalizer{
			fakeGoalContextProvider: &fakeGoalContextProvider{},
			report: protocol.GoalUsageReport{
				GoalID:     goalID,
				SessionKey: sessionKey,
				Status:     protocol.GoalStatusComplete,
			},
		},
		parentLedger: make(map[string]protocol.GoalUsage),
	}
}

func (p *durableDMParentProvider) RecordUsageParentSnapshot(
	_ context.Context,
	snapshot protocol.GoalUsageParentSnapshot,
) (protocol.GoalUsageParentResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.parentSnapshots = append(p.parentSnapshots, snapshot)
	key := snapshot.OwnerUserID + "\x00" +
		snapshot.GoalSessionKey + "\x00" +
		snapshot.ScopeRoundID + "\x00" +
		snapshot.SourceRoundID
	if _, exists := p.parentLedger[key]; !exists {
		p.parentLedger[key] = snapshot.Usage
		p.parentAttributed = p.parentAttributed.Add(snapshot.Usage)
		p.report.Usage = p.report.Usage.Add(snapshot.Usage)
	}
	if p.parentCommitErrorsRemaining > 0 {
		p.parentCommitErrorsRemaining--
		return protocol.GoalUsageParentResult{}, errors.New("injected committed DM parent response failure")
	}
	return protocol.GoalUsageParentResult{
		AttributedUsage: p.parentLedger[key],
	}, nil
}

func (p *durableDMParentProvider) FinalizeUsageForGoal(
	_ context.Context,
	goalID string,
	delta protocol.GoalUsage,
	_ string,
) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.finalizeCalls++
	p.finalizeDeltas = append(p.finalizeDeltas, delta)
	if p.finalizeFailuresRemaining > 0 {
		p.finalizeFailuresRemaining--
		return nil, errors.New("injected DM durable finalization failure")
	}
	p.report.Usage = p.report.Usage.Add(delta)
	p.report.UsageFinalized = true
	return &protocol.Goal{
		ID:             goalID,
		SessionKey:     p.report.SessionKey,
		Status:         protocol.GoalStatusComplete,
		Usage:          p.report.Usage,
		UsageFinalized: true,
	}, nil
}

func (p *durableDMParentProvider) parentCallCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.parentSnapshots)
}

func (p *durableDMParentProvider) parentAttributedUsage() protocol.GoalUsage {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.parentAttributed
}

type dmChildEvidenceState struct {
	terminal      bool
	tokenObserved bool
}

type evidenceAwareDMGoalProvider struct {
	*durableDMParentProvider
	sourceSnapshots   []protocol.GoalUsageSourceSnapshot
	childEvidenceByID map[string]dmChildEvidenceState
}

func newEvidenceAwareDMGoalProvider(
	sessionKey string,
	goalID string,
) *evidenceAwareDMGoalProvider {
	return &evidenceAwareDMGoalProvider{
		durableDMParentProvider: newDurableDMParentProvider(sessionKey, goalID),
		childEvidenceByID:       make(map[string]dmChildEvidenceState),
	}
}

func (p *evidenceAwareDMGoalProvider) RecordUsageSourceSnapshot(
	_ context.Context,
	snapshot protocol.GoalUsageSourceSnapshot,
) (protocol.GoalUsageSourceResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sourceSnapshots = append(p.sourceSnapshots, snapshot)
	if snapshot.EvidenceRequired {
		current := p.childEvidenceByID[snapshot.SourceID]
		current.terminal = current.terminal || snapshot.Terminal
		current.tokenObserved = current.tokenObserved ||
			(snapshot.Terminal && snapshot.TokenUsageObserved)
		p.childEvidenceByID[snapshot.SourceID] = current
	}
	return protocol.GoalUsageSourceResult{}, nil
}

func (p *evidenceAwareDMGoalProvider) FinalizeUsageForGoal(
	_ context.Context,
	goalID string,
	delta protocol.GoalUsage,
	_ string,
) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.finalizeCalls++
	p.finalizeDeltas = append(p.finalizeDeltas, delta)
	for _, evidence := range p.childEvidenceByID {
		if !evidence.terminal {
			return nil, errors.New("injected durable child usage pending")
		}
		if !evidence.tokenObserved {
			return nil, goalsvc.ErrGoalUsageUnavailable
		}
	}
	p.report.Usage = p.report.Usage.Add(delta)
	p.report.UsageFinalized = true
	return &protocol.Goal{
		ID:             goalID,
		SessionKey:     p.report.SessionKey,
		Status:         protocol.GoalStatusComplete,
		Usage:          p.report.Usage,
		UsageFinalized: true,
	}, nil
}

func (p *evidenceAwareDMGoalProvider) childEvidence(taskID string) dmChildEvidenceState {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.childEvidenceByID[taskID]
}

type retryingDMGoalUsageProvider struct {
	*fakeDMGoalUsageFinalizer
	sourceFailuresRemaining   int
	sourceAttempts            int
	finalizeFailuresRemaining int
}

func newRetryingDMGoalUsageProvider(
	sessionKey string,
	goalID string,
	sourceFailures int,
) *retryingDMGoalUsageProvider {
	return &retryingDMGoalUsageProvider{
		fakeDMGoalUsageFinalizer: &fakeDMGoalUsageFinalizer{
			fakeGoalContextProvider: &fakeGoalContextProvider{},
			report: protocol.GoalUsageReport{
				GoalID:     goalID,
				SessionKey: sessionKey,
				Status:     protocol.GoalStatusComplete,
			},
		},
		sourceFailuresRemaining: sourceFailures,
	}
}

func (p *retryingDMGoalUsageProvider) RecordUsageSourceSnapshot(
	_ context.Context,
	_ protocol.GoalUsageSourceSnapshot,
) (protocol.GoalUsageSourceResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sourceAttempts++
	if p.sourceFailuresRemaining > 0 {
		p.sourceFailuresRemaining--
		return protocol.GoalUsageSourceResult{}, errors.New("injected DM source snapshot failure")
	}
	return protocol.GoalUsageSourceResult{}, nil
}

func (p *retryingDMGoalUsageProvider) FinalizeUsageForGoal(
	_ context.Context,
	goalID string,
	delta protocol.GoalUsage,
	_ string,
) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.finalizeCalls++
	p.finalizeDeltas = append(p.finalizeDeltas, delta)
	if p.finalizeFailuresRemaining > 0 {
		p.finalizeFailuresRemaining--
		return nil, errors.New("injected DM usage finalization failure")
	}
	p.report.Usage = p.report.Usage.Add(delta)
	p.report.UsageFinalized = true
	return &protocol.Goal{
		ID:             goalID,
		SessionKey:     p.report.SessionKey,
		Status:         protocol.GoalStatusComplete,
		Usage:          p.report.Usage,
		UsageFinalized: true,
	}, nil
}

func waitForDMGoalUsageRetry(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if ready() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for DM Goal usage background retry")
}

func dmGoalUsageRetryStopped(runner *roundRunner) bool {
	if runner == nil {
		return true
	}
	runner.Mu.Lock()
	defer runner.Mu.Unlock()
	return !runner.UsageRetrying
}

func (p *fakeDMGoalUsageFinalizer) UsageByGoalID(
	_ context.Context,
	_ string,
) (*protocol.GoalUsageReport, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	report := p.report
	return &report, nil
}

func (p *fakeDMGoalUsageFinalizer) FinalizeUsageForGoal(
	_ context.Context,
	goalID string,
	delta protocol.GoalUsage,
	_ string,
) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.finalizeCalls++
	p.finalizeDeltas = append(p.finalizeDeltas, delta)
	p.report.Usage = p.report.Usage.Add(delta)
	p.report.UsageFinalized = true
	return &protocol.Goal{
		ID:             goalID,
		SessionKey:     p.report.SessionKey,
		Status:         protocol.GoalStatusComplete,
		Usage:          p.report.Usage,
		UsageFinalized: true,
	}, nil
}

func (p *fakeDMGoalUsageFinalizer) ClaimUsageSourceRound(
	_ context.Context,
	_ protocol.GoalUsageSourceRoundClaim,
) (protocol.GoalUsageSourceResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.claimAttempts++
	if p.claimFailuresRemaining > 0 {
		p.claimFailuresRemaining--
		return protocol.GoalUsageSourceResult{}, errors.New("injected DM usage claim failure")
	}
	p.claimSuccesses++
	return protocol.GoalUsageSourceResult{}, nil
}

func (p *fakeDMGoalUsageFinalizer) finalizeCallCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.finalizeCalls
}

func (p *fakeDMGoalUsageFinalizer) finalizedDeltas() []protocol.GoalUsage {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]protocol.GoalUsage(nil), p.finalizeDeltas...)
}

func (p *fakeDMGoalUsageFinalizer) claimAttemptCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.claimAttempts
}

func (p *fakeDMGoalUsageFinalizer) claimSuccessCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.claimSuccesses
}

func (r *roundRunner) subagentGoalUsageClaimPending() bool {
	r.Mu.Lock()
	defer r.Mu.Unlock()
	return r.UsageClaimPending
}

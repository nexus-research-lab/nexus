// authority_test.go 覆盖 Agent 撤销、Goal 权威与用量、职责权威与 WorkBinding 状态测试。
package runtime

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func ownerRuntimeOptions(ownerUserID string) agentclient.Options {
	return agentclient.Options{
		Env: map[string]string{"NEXUS_RUNTIME_USER_ID": ownerUserID},
	}
}

func TestManagerRevokeAgentSessionsClosesOnlyMatchingAgentAndTombstones(t *testing.T) {
	deletedDM := &fakeRuntimeClient{}
	deletedRoom := &fakeRuntimeClient{}
	sibling := &fakeRuntimeClient{}
	otherOwner := &fakeRuntimeClient{}
	reaper := &fakeOwnerProcessReaper{}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{clients: []*fakeRuntimeClient{
		deletedDM,
		deletedRoom,
		sibling,
		otherOwner,
	}})
	manager.SetOwnerProcessReaper(reaper)

	const (
		deletedDMKey   = "agent:agent-a:ws:dm:dm-a"
		deletedRoomKey = "agent:agent-a:ws:group:conversation-a"
		siblingKey     = "agent:agent-b:ws:dm:dm-b"
		otherOwnerKey  = "agent:agent-a:ws:dm:other-owner"
	)
	for _, input := range []struct {
		sessionKey  string
		ownerUserID string
	}{
		{deletedDMKey, "owner-a"},
		{deletedRoomKey, "owner-a"},
		{siblingKey, "owner-a"},
		{otherOwnerKey, "owner-b"},
	} {
		if _, err := manager.GetOrCreate(
			context.Background(),
			input.sessionKey,
			ownerRuntimeOptions(input.ownerUserID),
		); err != nil {
			t.Fatalf("创建 %s runtime: %v", input.sessionKey, err)
		}
	}
	roundCanceled := make(chan struct{})
	if err := manager.StartRound(t.Context(), deletedRoomKey, "room-round", func() {
		close(roundCanceled)
		manager.MarkRoundFinished(deletedRoomKey, "room-round")
	}); err != nil {
		t.Fatalf("启动待删除 Agent Room round: %v", err)
	}

	closed, err := manager.RevokeAgentSessions(
		context.Background(),
		"owner-a",
		"agent-a",
	)
	if err != nil {
		t.Fatalf("撤销 Agent runtime: %v", err)
	}
	if closed != 2 {
		t.Fatalf("撤销 session 数量=%d，want 2", closed)
	}
	select {
	case <-roundCanceled:
	default:
		t.Fatal("Agent 删除未取消运行中的 Room round")
	}
	if deletedDM.disconnectCalls != 1 || deletedRoom.disconnectCalls != 1 {
		t.Fatalf(
			"待删除 Agent runtime 未全部断开: dm=%d room=%d",
			deletedDM.disconnectCalls,
			deletedRoom.disconnectCalls,
		)
	}
	if sibling.disconnectCalls != 0 || otherOwner.disconnectCalls != 0 {
		t.Fatalf(
			"撤销越过 owner+Agent 边界: sibling=%d other_owner=%d",
			sibling.disconnectCalls,
			otherOwner.disconnectCalls,
		)
	}
	if !manager.HasSession(siblingKey) || !manager.HasSession(otherOwnerKey) {
		t.Fatal("同 owner 其他 Agent 或其他 owner 同名 Agent session 被误删")
	}
	if len(reaper.owners) != 0 {
		t.Fatalf("同 owner 仍有其他 Agent 时不得 owner 级回收: %v", reaper.owners)
	}

	if _, err = manager.GetOrCreate(
		context.Background(),
		"agent:agent-a:ws:dm:new-after-delete",
		ownerRuntimeOptions("owner-a"),
	); !errors.Is(err, ErrRuntimeAgentRevoked) {
		t.Fatalf("删除墓碑未阻止新 session: %v", err)
	}
	lateRoundCanceled := false
	if err := manager.StartRound(t.Context(), deletedDMKey, "late-round", func() { lateRoundCanceled = true }); !errors.Is(err, ErrRuntimeAgentRevoked) {
		t.Fatalf("删除后旧 session_key 启动错误=%v，期望 Agent 已撤销", err)
	}
	if !lateRoundCanceled {
		t.Fatal("拒绝删除后 round 时必须取消调用方 context")
	}
	if _, err = manager.GetOrCreate(
		context.Background(),
		otherOwnerKey,
		ownerRuntimeOptions("owner-b"),
	); err != nil {
		t.Fatalf("其他 owner 的同名 Agent 不应被墓碑阻断: %v", err)
	}
	if err = manager.Connect(context.Background(), siblingKey, sibling); err != nil {
		t.Fatalf("同 owner 的其他 Agent Connect 不应受影响: %v", err)
	}
	if err = manager.Connect(context.Background(), otherOwnerKey, otherOwner); err != nil {
		t.Fatalf("其他 owner 的同名 Agent Connect 不应受影响: %v", err)
	}
}

type blockingConnectRuntimeClient struct {
	*fakeRuntimeClient
	started        chan struct{}
	release        chan struct{}
	startOnce      sync.Once
	releaseOnce    sync.Once
	disconnectCall atomic.Int32
}

func (c *blockingConnectRuntimeClient) Connect(context.Context) error {
	c.startOnce.Do(func() { close(c.started) })
	<-c.release
	return nil
}

func (c *blockingConnectRuntimeClient) Disconnect(context.Context) error {
	c.disconnectCall.Add(1)
	c.releaseOnce.Do(func() { close(c.release) })
	return nil
}

func TestManagerConnectDisconnectsLateProcessWhenAgentIsDeleted(t *testing.T) {
	client := &blockingConnectRuntimeClient{
		fakeRuntimeClient: &fakeRuntimeClient{},
		started:           make(chan struct{}),
		release:           make(chan struct{}),
	}
	manager := NewManagerWithFactory(runtimeFactoryFunc(func(agentclient.Options) Client {
		return client
	}))
	const sessionKey = "agent:agent-a:ws:dm:connect-race"
	if _, err := manager.GetOrCreate(
		context.Background(),
		sessionKey,
		ownerRuntimeOptions("owner-a"),
	); err != nil {
		t.Fatal(err)
	}

	connectResult := make(chan error, 1)
	go func() {
		connectResult <- manager.Connect(context.Background(), sessionKey, client)
	}()
	select {
	case <-client.started:
	case <-time.After(time.Second):
		t.Fatal("client Connect 未进入删除竞态窗口")
	}
	if _, err := manager.RevokeAgentSessions(
		context.Background(),
		"owner-a",
		"agent-a",
	); err != nil {
		t.Fatal(err)
	}
	if err := <-connectResult; !errors.Is(err, ErrRuntimeAgentRevoked) {
		t.Fatalf("删除期间迟到完成的 Connect 未被拒绝: %v", err)
	}
	if client.disconnectCall.Load() != 2 {
		t.Fatalf(
			"删除撤销及 Connect 后置核验应各断开一次: disconnect=%d",
			client.disconnectCall.Load(),
		)
	}
}

type blockingReconfigureRuntimeClient struct {
	*fakeRuntimeClient
	started        chan struct{}
	release        chan struct{}
	startOnce      sync.Once
	releaseOnce    sync.Once
	block          atomic.Bool
	disconnectCall atomic.Int32
}

func (c *blockingReconfigureRuntimeClient) Reconfigure(
	context.Context,
	agentclient.Options,
) error {
	if c.block.Load() {
		c.startOnce.Do(func() { close(c.started) })
		<-c.release
	}
	return nil
}

func (c *blockingReconfigureRuntimeClient) Disconnect(context.Context) error {
	c.disconnectCall.Add(1)
	c.releaseOnce.Do(func() { close(c.release) })
	return nil
}

func TestManagerRevokeAgentSessionsWinsExistingClientReconfigureRace(t *testing.T) {
	client := &blockingReconfigureRuntimeClient{
		fakeRuntimeClient: &fakeRuntimeClient{},
		started:           make(chan struct{}),
		release:           make(chan struct{}),
	}
	manager := NewManagerWithFactory(runtimeFactoryFunc(func(agentclient.Options) Client {
		return client
	}))
	const sessionKey = "agent:agent-a:ws:dm:reconfigure-race"
	options := ownerRuntimeOptions("owner-a")
	if _, err := manager.GetOrCreate(context.Background(), sessionKey, options); err != nil {
		t.Fatal(err)
	}
	client.block.Store(true)

	getResult := make(chan error, 1)
	go func() {
		_, err := manager.GetOrCreate(context.Background(), sessionKey, options)
		getResult <- err
	}()
	select {
	case <-client.started:
	case <-time.After(time.Second):
		t.Fatal("existing-client Reconfigure 未进入竞态窗口")
	}
	if _, err := manager.RevokeAgentSessions(
		context.Background(),
		"owner-a",
		"agent-a",
	); err != nil {
		t.Fatal(err)
	}
	if err := <-getResult; !errors.Is(err, ErrRuntimeAgentRevoked) {
		t.Fatalf("并发 Reconfigure 越过删除墓碑: %v", err)
	}
	if client.disconnectCall.Load() != 1 {
		t.Fatalf("existing client disconnect=%d，want 1", client.disconnectCall.Load())
	}
}

type atomicDisconnectRuntimeClient struct {
	*fakeRuntimeClient
	disconnectCall atomic.Int32
}

func (c *atomicDisconnectRuntimeClient) Disconnect(context.Context) error {
	c.disconnectCall.Add(1)
	return nil
}

type blockingReplacementFactory struct {
	stale            Client
	candidate        Client
	calls            atomic.Int32
	candidateStarted chan struct{}
	releaseCandidate chan struct{}
}

func (f *blockingReplacementFactory) New(agentclient.Options) Client {
	if f.calls.Add(1) == 1 {
		return f.stale
	}
	close(f.candidateStarted)
	<-f.releaseCandidate
	return f.candidate
}

func TestManagerRevokeAgentSessionsWinsReplacementCandidateRace(t *testing.T) {
	stale := &atomicDisconnectRuntimeClient{
		fakeRuntimeClient: &fakeRuntimeClient{
			reconfigureErr: &agentclient.RestartRequiredError{
				Reason: agentclient.RestartReasonProcessEnvChanged,
			},
		},
	}
	candidate := &atomicDisconnectRuntimeClient{fakeRuntimeClient: &fakeRuntimeClient{}}
	factory := &blockingReplacementFactory{
		stale:            stale,
		candidate:        candidate,
		candidateStarted: make(chan struct{}),
		releaseCandidate: make(chan struct{}),
	}
	manager := NewManagerWithFactory(factory)
	const sessionKey = "agent:agent-a:ws:dm:replacement-race"
	options := ownerRuntimeOptions("owner-a")
	if _, err := manager.GetOrCreate(context.Background(), sessionKey, options); err != nil {
		t.Fatal(err)
	}

	getResult := make(chan error, 1)
	go func() {
		_, err := manager.GetOrCreate(context.Background(), sessionKey, options)
		getResult <- err
	}()
	select {
	case <-factory.candidateStarted:
	case <-time.After(time.Second):
		t.Fatal("replacement candidate 未进入竞态窗口")
	}
	if _, err := manager.RevokeAgentSessions(
		context.Background(),
		"owner-a",
		"agent-a",
	); err != nil {
		t.Fatal(err)
	}
	close(factory.releaseCandidate)
	if err := <-getResult; !errors.Is(err, ErrRuntimeAgentRevoked) {
		t.Fatalf("replacement candidate 越过删除墓碑: %v", err)
	}
	if stale.disconnectCall.Load() != 1 {
		t.Fatalf("stale client disconnect=%d，want 1", stale.disconnectCall.Load())
	}
	if candidate.disconnectCall.Load() != 1 {
		t.Fatalf("被墓碑拒绝的 candidate disconnect=%d，want 1", candidate.disconnectCall.Load())
	}
}

type blockingCreateFactory struct {
	candidate Client
	started   chan struct{}
	release   chan struct{}
}

func (f *blockingCreateFactory) New(agentclient.Options) Client {
	close(f.started)
	<-f.release
	return f.candidate
}

func TestManagerRevokeAgentSessionsWinsSecondLockCreateRace(t *testing.T) {
	candidate := &atomicDisconnectRuntimeClient{fakeRuntimeClient: &fakeRuntimeClient{}}
	factory := &blockingCreateFactory{
		candidate: candidate,
		started:   make(chan struct{}),
		release:   make(chan struct{}),
	}
	manager := NewManagerWithFactory(factory)
	const sessionKey = "agent:agent-a:ws:dm:create-race"
	options := ownerRuntimeOptions("owner-a")

	getResult := make(chan error, 1)
	go func() {
		_, err := manager.GetOrCreate(context.Background(), sessionKey, options)
		getResult <- err
	}()
	select {
	case <-factory.started:
	case <-time.After(time.Second):
		t.Fatal("fresh candidate 未进入 first/second lock 竞态窗口")
	}
	if closed, err := manager.RevokeAgentSessions(
		context.Background(),
		"owner-a",
		"agent-a",
	); err != nil || closed != 0 {
		t.Fatalf("无已发布 session 的撤销: closed=%d err=%v", closed, err)
	}
	close(factory.release)
	if err := <-getResult; !errors.Is(err, ErrRuntimeAgentRevoked) {
		t.Fatalf("second-lock create 越过删除墓碑: %v", err)
	}
	if candidate.disconnectCall.Load() != 1 {
		t.Fatalf("未发布 candidate disconnect=%d，want 1", candidate.disconnectCall.Load())
	}
	if manager.HasSession(sessionKey) {
		t.Fatal("second-lock 拒绝后不得发布 candidate")
	}
}

func TestGoalAuthorityStateRejectsGoalSwitchAndRevisionRegression(t *testing.T) {
	t.Parallel()

	state := NewGoalAuthorityState("goal-1", 2, "execution-1")
	if state.Bind("goal-2", 3, "execution-2") {
		t.Fatal("one physical round must not switch Goal identity")
	}
	if state.Bind("goal-1", 1, "execution-old") {
		t.Fatal("authority revision must not regress")
	}
	if !state.Bind("goal-1", 3, "") {
		t.Fatal("same Goal should adopt a newer revision with optional ExecutionID")
	}
	authority, ok := state.Load()
	if !ok || authority.ObjectiveRevision != 3 || authority.ExecutionID != "" {
		t.Fatalf("advanced authority = %#v, ok=%t", authority, ok)
	}
}

func TestGoalAuthorityStateOnlyUpgradesExecutionWithinSameRevision(t *testing.T) {
	t.Parallel()

	state := NewGoalAuthorityState("goal-1", 2, "")
	if !state.Bind("goal-1", 2, "execution-1") {
		t.Fatal("an unbound authority should accept one confirmed Execution fence")
	}
	if state.Bind("goal-1", 2, "") {
		t.Fatal("a confirmed Execution fence must not be removed in the same revision")
	}
	if state.Bind("goal-1", 2, "execution-other") {
		t.Fatal("a confirmed Execution fence must not be replaced in the same revision")
	}
	if !state.Bind("goal-1", 3, "execution-next") {
		t.Fatal("a newer objective revision may adopt its successor Execution fence")
	}
}

func TestGoalAuthorityStateSharesRevisionAndContext(t *testing.T) {
	t.Parallel()

	revision := &atomic.Int64{}
	revision.Store(4)
	state := NewGoalAuthorityStateWithRevision("goal-1", "execution-1", revision)
	ctx := WithGoalAuthorityState(context.Background(), state)
	if GoalAuthorityStateFromContext(ctx) != state {
		t.Fatal("context did not preserve the exact authority state pointer")
	}
	revision.Store(5)
	authority, ok := state.Load()
	if !ok || authority.ObjectiveRevision != 5 {
		t.Fatalf("shared revision = %#v, ok=%t", authority, ok)
	}
	bound, ok := state.LoadBound()
	if !ok || bound.GoalID != "goal-1" || bound.ObjectiveRevision != 4 ||
		bound.ExecutionID != "execution-1" {
		t.Fatalf("bound authority after steering = %#v, ok=%t, want revision 4", bound, ok)
	}
	if !state.Bind("goal-1", 5, "execution-next") {
		t.Fatal("successful service mutation did not bind the steered revision")
	}
	bound, ok = state.LoadBound()
	if !ok || bound.ObjectiveRevision != 5 || bound.ExecutionID != "execution-next" {
		t.Fatalf("bound authority after receipt = %#v, ok=%t, want revision 5", bound, ok)
	}
}

func TestResultUsageLimitReachedDetectsExplicitUsageLimit(t *testing.T) {
	result := &sdkprotocol.ResultMessage{
		IsError:        true,
		TerminalReason: "error",
		Result:         "You've hit your usage limit. Try again later.",
		Additional: map[string]any{
			"error": map[string]any{
				"type": "usage_limit_reached",
			},
		},
	}

	ok, reason := ResultUsageLimitReached(result)
	if !ok {
		t.Fatal("ResultUsageLimitReached() ok = false, want true")
	}
	if reason != result.Result {
		t.Fatalf("reason = %q, want result text", reason)
	}
}

func TestGoalUsageFromRawRejectsProviderZeroThatContradictsBreakdown(t *testing.T) {
	usage, ok := GoalUsageFromRaw(map[string]any{
		"input_tokens":            100,
		"output_tokens":           20,
		"cache_read_input_tokens": 80,
		"total_tokens":            0,
	})
	if !ok {
		t.Fatal("GoalUsageFromRaw() ok = false, want true")
	}

	if !usage.ActualTotalKnown || usage.ActualTokens() != 200 || !usage.ActualTokensAreEstimated() {
		t.Fatalf("actual usage = %#v, want estimated breakdown total 200", usage)
	}
	if usage.BudgetTokens() != 120 {
		t.Fatalf("budget usage = %#v, want 120", usage)
	}
}

func TestGoalUsageFromTokenUsageAggregatesNestedNXSModelUsageBreakdown(t *testing.T) {
	usage := GoalUsageFromTokenUsage(sdkprotocol.TokenUsage{
		InputTokens:  150,
		OutputTokens: 30,
		TotalTokens:  180,
		Raw: map[string]any{
			"model-a": map[string]any{
				"input_tokens":  100,
				"output_tokens": 20,
				"total_tokens":  120,
				"raw": map[string]any{
					"input_tokens":            100,
					"output_tokens":           20,
					"cache_read_input_tokens": 50,
				},
			},
			"model-b": map[string]any{
				"input_tokens":  50,
				"output_tokens": 10,
				"total_tokens":  60,
				"raw": map[string]any{
					"input_tokens":          50,
					"output_tokens":         10,
					"reasoning_tokens":      15,
					"cache_creation_tokens": 5,
				},
			},
		},
	})

	if usage.ActualTokens() != 235 || !usage.ActualTokensAreEstimated() {
		t.Fatalf("model usage aggregate = %#v, want estimated actual 235", usage)
	}
	if usage.CacheReadInputTokens != 50 ||
		usage.CacheCreationInputTokens != 5 ||
		usage.ReasoningTokens != 15 {
		t.Fatalf("model usage nested breakdown = %#v, want aggregated cache/reasoning", usage)
	}
}

func TestGoalUsageFromTokenUsageRecognizesExactModelUsageAggregation(t *testing.T) {
	usage := GoalUsageFromTokenUsage(sdkprotocol.TokenUsage{
		InputTokens:  300,
		OutputTokens: 30,
		TotalTokens:  330,
		Raw: map[string]any{
			"model-a": map[string]any{
				"input_tokens":  100,
				"output_tokens": 10,
				"total_tokens":  110,
			},
			"model-b": map[string]any{
				"input_tokens":  200,
				"output_tokens": 20,
				"total_tokens":  220,
			},
		},
	})

	if usage.ActualTokens() != 330 || usage.ActualTokensAreEstimated() {
		t.Fatalf("model usage aggregate = %#v, want exact provider totals 330", usage)
	}
}

func TestGoalUsageFromTokenUsageRecognizesNestedProviderTotalAlias(t *testing.T) {
	usage := GoalUsageFromTokenUsage(sdkprotocol.TokenUsage{
		InputTokens:  100,
		OutputTokens: 20,
		TotalTokens:  120,
		Raw: map[string]any{
			"input_tokens":  100,
			"output_tokens": 20,
			"total_tokens":  120,
			"raw": map[string]any{
				"promptTokenCount":     100,
				"candidatesTokenCount": 20,
				"totalTokenCount":      130,
				"thoughtsTokenCount":   10,
			},
		},
	})

	if usage.ActualTokens() != 130 || usage.ActualTokensAreEstimated() {
		t.Fatalf("provider alias usage = %#v, want exact nested total 130", usage)
	}
	if usage.ReasoningTokens != 10 {
		t.Fatalf("provider alias reasoning = %d, want 10", usage.ReasoningTokens)
	}
}

// GoalUsageFromTokenUsage 把 SDK usage 转成 Goal accounting 口径。
func GoalUsageFromTokenUsage(usage sdkprotocol.TokenUsage) protocol.GoalUsage {
	goalUsage, _ := GoalUsageFromTokenUsageWithPresence(usage)
	return goalUsage
}

func TestResponsibilityAuthorityRetargetRevokesReviewAndBindsSuccessor(t *testing.T) {
	goal := NewGoalAuthorityState("goal-1", 1, "execution-old")
	review := responsibilityTestReviewBinding("execution-old")
	state := NewResponsibilityAuthorityState(goal, "execution-old", nil, review)

	before, _ := state.Load()
	if before.Lane != ResponsibilityLaneReview || before.ReviewBinding == nil {
		t.Fatalf("before = %#v, want exact review lane", before)
	}

	retargeted := protocol.Goal{
		ID: "goal-1",
		Metadata: map[string]any{
			protocol.GoalMetadataObjectiveRevision:     int64(2),
			protocol.GoalMetadataExecutionMode:         string(protocol.GoalExecutionModeManaged),
			protocol.GoalMetadataExecutionBindingState: string(protocol.GoalExecutionBindingStateReserved),
			protocol.GoalMetadataExecutionID:           "execution-successor",
		},
	}
	if !state.ApplyGoalMutation(retargeted) {
		t.Fatal("ApplyGoalMutation(retargeted) = false")
	}
	afterRetarget, _ := state.Load()
	if afterRetarget.ExecutionID != "" ||
		afterRetarget.ReservedExecutionID != "execution-successor" ||
		afterRetarget.ReviewBinding != nil ||
		afterRetarget.WorkBinding != nil ||
		afterRetarget.Lane != ResponsibilityLanePlanning {
		t.Fatalf("after retarget = %#v, want successor planning with old lanes revoked", afterRetarget)
	}
	if state.BindCoordination("execution-old") {
		t.Fatal("old execution receipt rebound predecessor after retarget")
	}
	if !state.BindCoordination("execution-successor") {
		t.Fatal("successor materialization receipt was rejected")
	}
	afterPlan, _ := state.Load()
	if afterPlan.ExecutionID != "execution-successor" ||
		afterPlan.ReservedExecutionID != "" ||
		afterPlan.Lane != ResponsibilityLaneCoordination ||
		afterPlan.ReviewBinding != nil {
		t.Fatalf("after plan = %#v, want successor coordination", afterPlan)
	}
	if !state.BindCoordination("execution-successor") {
		t.Fatal("replayed successor receipt was rejected")
	}
	replayed, _ := state.Load()
	if replayed.Generation != afterPlan.Generation {
		t.Fatalf("replayed receipt advanced generation: before=%d after=%d", afterPlan.Generation, replayed.Generation)
	}
	if state.RevokeExecution("execution-old") {
		t.Fatal("late predecessor terminal receipt revoked successor")
	}
}

func TestResponsibilityAuthorityKeepsExactWorkLaneAcrossGoalConfirmation(t *testing.T) {
	goal := NewGoalAuthorityState("", 0, "")
	work := responsibilityTestWorkBinding("execution-1", "assignment-1")
	state := NewResponsibilityAuthorityState(goal, "execution-1", work, nil)

	if !state.ConfirmGoalExecution("goal-1", 1, "execution-1") {
		t.Fatal("ConfirmGoalExecution() = false")
	}
	authority, _ := state.Load()
	if authority.Lane != ResponsibilityLaneWork || authority.WorkBinding == nil ||
		authority.WorkBinding.AssignmentID != "assignment-1" {
		t.Fatalf("authority = %#v, Goal confirmation must preserve exact WorkBinding", authority)
	}
	if state.BindWork(responsibilityTestWorkBinding("execution-1", "assignment-sibling")) {
		t.Fatal("sibling WorkBinding replaced exact responsibility")
	}
	if state.BindWork(responsibilityTestWorkBinding("execution-other", "assignment-1")) {
		t.Fatal("cross-Execution WorkBinding replaced exact responsibility")
	}
}

func TestWorkBindingStateSharesResponsibilityAuthority(t *testing.T) {
	state := NewResponsibilityAuthorityState(nil, "execution-1", nil, nil)
	workState := NewWorkBindingStateFromResponsibility(state)
	binding := responsibilityTestWorkBinding("execution-1", "assignment-1")
	if !workState.Bind(binding) {
		t.Fatal("Bind() = false")
	}
	authority, _ := state.Load()
	if authority.Lane != ResponsibilityLaneWork || authority.WorkBinding == nil {
		t.Fatalf("authority = %#v, want work lane", authority)
	}
	workState.Clear()
	authority, _ = state.Load()
	if authority.Lane != ResponsibilityLaneCoordination || authority.WorkBinding != nil {
		t.Fatalf("authority = %#v, want coordination after clear", authority)
	}
}

func TestResponsibilityAuthorityTerminalReleaseRejectsStaticExecutionReseed(t *testing.T) {
	state := NewResponsibilityAuthorityState(nil, "execution-old", nil, nil)
	if !state.RevokeExecution("execution-old") {
		t.Fatal("RevokeExecution() = false")
	}
	if state.SeedExecution("execution-old") {
		t.Fatal("stale static actor reseeded terminal execution")
	}
	if state.BindCoordination("execution-old") {
		t.Fatal("late coordination receipt rebound terminal execution")
	}
	authority, _ := state.Load()
	if authority.ExecutionID != "" || authority.Lane != ResponsibilityLaneUnbound {
		t.Fatalf("authority = %#v, want released execution to remain unbound", authority)
	}
}

func TestResponsibilityAuthorityConflictingInitialBindingsFailClosed(t *testing.T) {
	state := NewResponsibilityAuthorityState(
		NewGoalAuthorityState("goal-1", 1, "execution-1"),
		"execution-1",
		responsibilityTestWorkBinding("execution-1", "assignment-1"),
		responsibilityTestReviewBinding("execution-1"),
	)
	authority, _ := state.Load()
	if authority.ExecutionID != "" || authority.WorkBinding != nil ||
		authority.ReviewBinding != nil || authority.GoalID != "" ||
		authority.Lane != ResponsibilityLaneUnbound {
		t.Fatalf("conflicting authority = %#v, want fully unbound", authority)
	}
	if state.SeedExecution("execution-1") ||
		state.GrantGoalAuthority("goal-1", 1, "execution-1") {
		t.Fatal("invalid initial responsibility was upgraded by a later static seed")
	}
}

func responsibilityTestWorkBinding(
	executionID string,
	assignmentID string,
) *protocol.ExecutionWorkBinding {
	return &protocol.ExecutionWorkBinding{
		ExecutionID:  executionID,
		PlanID:       "plan-1",
		WorkItemID:   "work-1",
		SpecID:       "spec-1",
		AssignmentID: assignmentID,
		AttemptID:    "attempt-1",
	}
}

func responsibilityTestReviewBinding(executionID string) *protocol.ExecutionReviewBinding {
	return &protocol.ExecutionReviewBinding{
		ExecutionID:      executionID,
		PlanID:           "plan-1",
		WorkItemID:       "work-1",
		SpecID:           "spec-1",
		AssignmentID:     "assignment-1",
		SubmissionID:     "submission-1",
		ReviewDispatchID: "review-dispatch-1",
		TargetAgentID:    "agent-lead",
	}
}

func TestWorkBindingStateRequiresExplicitClearBeforeResponsibilitySwitch(t *testing.T) {
	first := &protocol.ExecutionWorkBinding{
		ExecutionID: " execution-1 ", PlanID: "plan-1", WorkItemID: "work-1",
		SpecID: "spec-1", AssignmentID: " assignment-1 ", AttemptID: "attempt-1",
	}
	state := NewWorkBindingState(nil)
	canonical := first.Normalized()
	if !state.Bind(first) || !state.Bind(&canonical) {
		t.Fatal("exact host binding must bind and replay idempotently")
	}
	loaded, ok := state.Load()
	if !ok || loaded == first || loaded.AssignmentID != "assignment-1" {
		t.Fatalf("Load() = %#v, %t", loaded, ok)
	}
	second := *first
	second.WorkItemID = "work-2"
	second.SpecID = "spec-2"
	second.AssignmentID = "assignment-2"
	second.AttemptID = "attempt-2"
	if state.Bind(&second) {
		t.Fatal("unreleased responsibility switched WorkBinding")
	}
	state.Clear()
	if !state.Bind(&second) {
		t.Fatal("explicitly cleared responsibility did not accept next binding")
	}
}

func TestWorkBindingStateRejectsIncompleteReceipt(t *testing.T) {
	state := NewWorkBindingState(nil)
	if state.Bind(&protocol.ExecutionWorkBinding{ExecutionID: "execution-1"}) {
		t.Fatal("incomplete receipt minted WorkBinding")
	}
	if binding, ok := state.Load(); ok || binding != nil {
		t.Fatalf("incomplete state Load() = %#v, %t", binding, ok)
	}
}

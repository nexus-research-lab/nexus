// authority_test.go 覆盖 Agent 撤销、Goal 权威与用量、职责权威与 WorkBinding 状态测试。
package runtime

import (
	"context"
	"errors"
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

func TestWorkBindingStateRejectsIncompleteReceipt(t *testing.T) {
	state := NewWorkBindingState(nil)
	if state.Bind(&protocol.ExecutionWorkBinding{ExecutionID: "execution-1"}) {
		t.Fatal("incomplete receipt minted WorkBinding")
	}
	if binding, ok := state.Load(); ok || binding != nil {
		t.Fatalf("incomplete state Load() = %#v, %t", binding, ok)
	}
}

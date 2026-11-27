// goal_runtime_test.go 覆盖 DM Goal 运行时、回执、中断与用量并发测试。
package dm

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	messagepkg "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

type fakeGoalContextProvider struct {
	mu                  sync.Mutex
	plan                *protocol.GoalContinuation
	planCalls           int
	runtimeContext      string
	runtimeGoal         *protocol.Goal
	runtimeCalls        int
	usage               []protocol.GoalUsage
	usageGoal           *protocol.Goal
	usageSessionKeys    []string
	usageGoalIDs        []string
	usageLimitReason    []string
	progress            []bool
	progressRevisions   []int64
	failures            []string
	failureRevisions    []int64
	completionMisses    []string
	completionRevisions []int64
	activities          []string
	activityRevisions   []int64
	current             *bool
	claimCalls          int
	releaseCalls        int
	reservation         bool
	continuationCount   int
	claimErr            error
	onClaim             func()
	onStarted           func()
	beforeStarted       func()
	startedCalls        int
	startedErr          error
	settledCalls        int
	retryReasons        []string
	userStopRoundIDs    []string
	userStopRevisions   []int64
}

func (p *fakeGoalContextProvider) RuntimeContext(context.Context, string) (string, *protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.runtimeCalls++
	if p.runtimeGoal == nil {
		return p.runtimeContext, nil, nil
	}
	goal := *p.runtimeGoal
	return p.runtimeContext, &goal, nil
}

func (p *fakeGoalContextProvider) RecordUsageForSession(_ context.Context, sessionKey string, usage protocol.GoalUsage, _ string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.usage = append(p.usage, usage)
	p.usageSessionKeys = append(p.usageSessionKeys, sessionKey)
	return cloneTestGoal(p.usageGoal), nil
}

func (p *fakeGoalContextProvider) RecordUsageForGoal(_ context.Context, goalID string, usage protocol.GoalUsage, _ string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.usage = append(p.usage, usage)
	p.usageGoalIDs = append(p.usageGoalIDs, goalID)
	return cloneTestGoal(p.usageGoal), nil
}

func cloneTestGoal(item *protocol.Goal) *protocol.Goal {
	if item == nil {
		return nil
	}
	value := *item
	return &value
}

func (p *fakeGoalContextProvider) UsageLimitForSession(_ context.Context, _ string, _ string, reason string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.usageLimitReason = append(p.usageLimitReason, strings.TrimSpace(reason))
	return nil, nil
}

func (p *fakeGoalContextProvider) RecordContinuationProgress(_ context.Context, _ string, _ string, progressed bool, revisions ...int64) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.progress = append(p.progress, progressed)
	p.progressRevisions = append(p.progressRevisions, firstTestRevision(revisions))
	return nil, nil
}

func (p *fakeGoalContextProvider) RecordContinuationRuntimeFailure(_ context.Context, _ string, _ goalsvc.ContinuationRuntimeIdentity, reason string, revisions ...int64) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failures = append(p.failures, strings.TrimSpace(reason))
	p.failureRevisions = append(p.failureRevisions, firstTestRevision(revisions))
	return nil, nil
}

func (p *fakeGoalContextProvider) RecordContinuationUserStop(_ context.Context, _ string, identity goalsvc.ContinuationRuntimeIdentity, revisions ...int64) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.userStopRoundIDs = append(p.userStopRoundIDs, strings.TrimSpace(identity.ReceiptRoundID))
	p.userStopRevisions = append(p.userStopRevisions, firstTestRevision(revisions))
	return nil, nil
}

func (p *fakeGoalContextProvider) RecordCompletionCommandMiss(_ context.Context, _ string, _ string, reason string, revisions ...int64) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.completionMisses = append(p.completionMisses, strings.TrimSpace(reason))
	p.completionRevisions = append(p.completionRevisions, firstTestRevision(revisions))
	return nil, nil
}

func (p *fakeGoalContextProvider) RecordGoalActivity(_ context.Context, _ string, roundID string, revisions ...int64) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.activities = append(p.activities, strings.TrimSpace(roundID))
	p.activityRevisions = append(p.activityRevisions, firstTestRevision(revisions))
	return nil, nil
}

func (p *fakeGoalContextProvider) PlanContinuationForSession(context.Context, string, string) (*protocol.GoalContinuation, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.planCalls++
	if p.planCalls > 1 || p.plan == nil {
		return nil, nil
	}
	plan := *p.plan
	return &plan, nil
}

func (p *fakeGoalContextProvider) GoalContinuationStillCurrent(context.Context, protocol.GoalContinuation) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.current == nil {
		return true, nil
	}
	return *p.current, nil
}

func (p *fakeGoalContextProvider) ClaimContinuationPlan(context.Context, protocol.GoalContinuation) (*protocol.Goal, error) {
	p.mu.Lock()
	p.claimCalls++
	p.reservation = false
	err := p.claimErr
	onClaim := p.onClaim
	p.mu.Unlock()
	if onClaim != nil {
		onClaim()
	}
	return nil, err
}

func (p *fakeGoalContextProvider) ReleaseContinuationPlan(context.Context, protocol.GoalContinuation, string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.releaseCalls++
	if p.reservation {
		p.reservation = false
		if p.continuationCount > 0 {
			p.continuationCount--
		}
	}
	return nil, nil
}

func (p *fakeGoalContextProvider) MarkContinuationPlanStarted(context.Context, protocol.GoalContinuation) error {
	p.mu.Lock()
	beforeStarted := p.beforeStarted
	p.mu.Unlock()
	if beforeStarted != nil {
		beforeStarted()
	}
	p.mu.Lock()
	p.startedCalls++
	err := p.startedErr
	onStarted := p.onStarted
	p.mu.Unlock()
	if onStarted != nil {
		onStarted()
	}
	return err
}

func (p *fakeGoalContextProvider) SettleContinuationPlan(context.Context, string, string, int64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.settledCalls++
	return nil
}

func (p *fakeGoalContextProvider) RetryContinuationPlan(_ context.Context, _ protocol.GoalContinuation, reason string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.retryReasons = append(p.retryReasons, strings.TrimSpace(reason))
	return nil
}

func (p *fakeGoalContextProvider) recordedUsage() []protocol.GoalUsage {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]protocol.GoalUsage(nil), p.usage...)
}

func firstTestRevision(revisions []int64) int64 {
	if len(revisions) == 0 {
		return 0
	}
	return revisions[0]
}

func goalToolResultAssistantMessage(
	toolUseID string,
	toolName string,
	isError bool,
	inputTokens int64,
	outputTokens int64,
) protocol.Message {
	return protocol.Message{
		"message_id": "assistant-" + toolUseID,
		"role":       "assistant",
		"usage": map[string]any{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
			"total_tokens":  inputTokens + outputTokens,
		},
		"content": []map[string]any{
			{"type": "tool_use", "id": toolUseID, "name": toolName},
			{"type": "tool_result", "tool_use_id": toolUseID, "is_error": isError},
		},
	}
}

func goalCompletionCommandMissAssistantMessage() protocol.Message {
	return protocol.Message{
		"role": "assistant",
		"content": []map[string]any{
			{"type": "text", "text": "任务已经完成，但无法通过 nexus.command 调用 update_goal 来标记完成。"},
		},
	}
}

func stageDMRuntimeCommandReceipt(runner *roundRunner, receipt nexusmcp.CommandReceipt) {
	if runner.commandReceipts == nil {
		runner.commandReceipts = nexusmcp.NewCommandReceiptState()
	}
	runner.commandReceipts.Record(receipt)
}

func stageDMAppliedGoalCommand(runner *roundRunner, operation string, goalID string, status protocol.GoalStatus) {
	stageDMRuntimeCommandReceipt(runner, nexusmcp.CommandReceipt{
		Domain: command.DomainGoal, Operation: operation,
		Outcome: string(protocol.MutationResultApplied), GoalID: goalID,
		GoalStatus: string(status),
	})
}

func TestRoundRunnerGoalMutationsUseBoundObjectiveRevision(t *testing.T) {
	for _, testCase := range []struct {
		name string
		run  func(*roundRunner)
	}{
		{
			name: "continuation failure",
			run: func(runner *roundRunner) {
				runner.inputOptions.Purpose = "goal_continuation"
				runner.recordGoalContinuationProgress(exec.RoundExecutionResult{TerminalStatus: "error", ErrorMessage: "runtime failed"})
			},
		},
		{
			name: "explicit activity",
			run: func(runner *roundRunner) {
				runner.recordGoalContinuationProgress(exec.RoundExecutionResult{})
			},
		},
		{
			name: "completion command miss",
			run: func(runner *roundRunner) {
				runner.inputOptions.Purpose = "goal_continuation"
				runner.RememberGoalAssistantMessage(goalCompletionCommandMissAssistantMessage())
				runner.recordGoalContinuationProgress(exec.RoundExecutionResult{})
			},
		},
		{
			name: "continuation progress",
			run: func(runner *roundRunner) {
				runner.inputOptions.Purpose = "goal_continuation"
				runner.recordGoalContinuationProgress(exec.RoundExecutionResult{})
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			provider := &fakeGoalContextProvider{}
			revision := &atomic.Int64{}
			revision.Store(7)
			runner := &roundRunner{
				service:               &Service{goals: provider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
				sessionKey:            "agent:nexus:ws:dm:revision",
				roundID:               "round-revision",
				GoalRoundState:        runtimehost.GoalRoundState{IDForUsage: "goal-revision"},
				goalObjectiveRevision: revision,
			}

			testCase.run(runner)

			provider.mu.Lock()
			revisions := append([]int64(nil), provider.progressRevisions...)
			revisions = append(revisions, provider.failureRevisions...)
			revisions = append(revisions, provider.completionRevisions...)
			revisions = append(revisions, provider.activityRevisions...)
			provider.mu.Unlock()
			if !slices.Equal(revisions, []int64{7}) {
				t.Fatalf("mutation revisions = %v, want [7]", revisions)
			}
		})
	}
}

func TestTerminalRoundStatusEventCarriesUsageLimitFailureCode(t *testing.T) {
	runner := &roundRunner{
		sessionKey: "agent:nexus:ws:dm:terminal-usage-limit",
		roundID:    "round-terminal-usage-limit",
		agent:      &protocol.Agent{AgentID: "nexus"},
	}
	event := terminalRoundStatusEvent(runner, exec.RoundExecutionResult{
		TerminalStatus:    "error",
		ResultSubtype:     "error",
		ErrorMessage:      "provider returned an invalid request",
		UsageLimitReached: true,
	})
	if event.Data["failure_code"] != protocol.ConversationFailureUsageLimited {
		t.Fatalf("failure_code = %#v, want %q", event.Data["failure_code"], protocol.ConversationFailureUsageLimited)
	}
}

func TestRoundRunnerRecordsNXSSubagentActualUsageWithoutDoubleCounting(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "round-1",
		GoalRoundState: runtimehost.GoalRoundState{IDForUsage: "goal-1", Usage: goalsvc.NewRuntimeUsageAccumulator(true)},
		runtimeKind:    "nxs",
	}
	taskMessage := func(total int64) protocol.Message {
		return protocol.Message{"metadata": map[string]any{
			"task_id": "task-1",
			"usage":   map[string]any{"total_tokens": total},
		}}
	}

	runner.recordSubagentGoalUsage(context.Background(), taskMessage(100))
	runner.recordSubagentGoalUsage(context.Background(), taskMessage(150))
	runner.recordSubagentGoalUsage(context.Background(), taskMessage(150))

	usages := goalProvider.recordedUsage()
	if len(usages) != 2 || usages[0].ActualTokens() != 100 || usages[1].ActualTokens() != 50 {
		t.Fatalf("usages = %#v, want exact 100 + 50 child actual deltas", usages)
	}
	if usages[0].BudgetTokens() != 0 || usages[1].BudgetTokens() != 0 {
		t.Fatalf("usages = %#v, child total-only snapshots must not become budget tokens", usages)
	}
}

func TestDMExternalActivationFlushesPendingChildBeforeBindAndSkipsStaleRetry(t *testing.T) {
	provider := &orderedDMBindingGoalProvider{
		fakeGoalContextProvider: &fakeGoalContextProvider{},
	}
	runner := &roundRunner{
		service:        &Service{goals: provider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
		sessionKey:     "agent:nexus:ws:dm:bind-pending",
		roundID:        "round-bind-pending",
		ownerUserID:    "owner-bind-pending",
		runtimeKind:    "nxs",
		GoalRoundState: runtimehost.GoalRoundState{Usage: goalsvc.NewRuntimeUsageAccumulator(false)},
	}
	runner.MarkSubagentUsagePending("task-pending", goalsvc.SubagentUsageObservation{
		CumulativeTotal: 75,
	})

	pendingTaskIDs, done, _ := runner.pendingSubagentUsageForRetry()
	if done || len(pendingTaskIDs) != 1 {
		t.Fatalf("pending task IDs = %#v, done=%v", pendingTaskIDs, done)
	}
	if err := runner.activateGoalUsage(context.Background(), "goal-new"); err != nil {
		t.Fatal(err)
	}
	// Simulate a retry worker that copied the task ID before activation. It must
	// re-check pending under the lock and avoid replaying the old checkpoint.
	if err := runner.retryPendingSubagentUsageObservation(
		context.Background(),
		provider,
		pendingTaskIDs[0],
	); err != nil {
		t.Fatal(err)
	}

	if len(provider.order) != 2 ||
		provider.order[0] != "source" ||
		provider.order[1] != "bind" {
		t.Fatalf("persistence order = %#v, want source then bind", provider.order)
	}
	if len(provider.snapshots) != 1 || provider.snapshots[0].GoalID != "" {
		t.Fatalf("source snapshots = %#v, stale retry must not target new Goal", provider.snapshots)
	}
	if runner.HasRunningSubagentTask() {
		t.Fatal("successfully flushed pending checkpoint still holds the in-memory join barrier")
	}
}

func TestRoundRunnerClaimsPreCreateSubagentUsageAndKeepsChildBoundAfterTerminal(t *testing.T) {
	sessionKey := "agent:nexus:ws:dm:child-round-start"
	base := &fakeGoalContextProvider{
		runtimeGoal: &protocol.Goal{
			ID:         "goal-created",
			SessionKey: sessionKey,
			Status:     protocol.GoalStatusActive,
		},
	}
	provider := &fakePersistentDMGoalProvider{fakeGoalContextProvider: base}
	runner := &roundRunner{
		service:        &Service{goals: provider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
		sessionKey:     sessionKey,
		roundID:        "round-create",
		ownerUserID:    "owner-dm",
		runtimeKind:    "nxs",
		GoalRoundState: runtimehost.GoalRoundState{Usage: goalsvc.NewRuntimeUsageAccumulator(false), UsageStartedAt: time.Now()},
	}
	taskMessage := func(total int64) protocol.Message {
		return protocol.Message{"metadata": map[string]any{
			"task_id": "task-pre-create",
			"usage":   map[string]any{"total_tokens": total},
		}}
	}

	runner.recordSubagentGoalUsage(context.Background(), taskMessage(100))
	stageDMAppliedGoalCommand(runner, command.GoalOperationCreate, "goal-created", "")
	runner.recordGoalUsageFromAssistantMessage(
		goalToolResultAssistantMessage("tool-create", "create_goal", false, 0, 0),
	)

	if len(provider.snapshots) != 1 ||
		provider.snapshots[0].GoalID != "" ||
		provider.snapshots[0].RoundID != "round-create" ||
		provider.snapshots[0].ScopeRoundID != "round-create" {
		t.Fatalf("pre-create snapshots = %#v, want one unbound checkpoint observation", provider.snapshots)
	}
	if len(provider.claims) != 1 {
		t.Fatalf("claims = %#v, want one model-create round claim", provider.claims)
	}
	claim := provider.claims[0]
	if claim.OwnerUserID != "owner-dm" ||
		claim.RuntimeSessionKey != sessionKey ||
		claim.RoundID != "round-create" ||
		claim.ScopeRoundID != "round-create" ||
		claim.GoalID != "goal-created" ||
		claim.GoalSessionKey != sessionKey {
		t.Fatalf("claim = %#v, want exact DM round/Goal identity", claim)
	}

	runner.finalizeGoalUsage(context.Background(), exec.RoundExecutionResult{}, nil)
	runner.recordSubagentGoalUsage(context.Background(), taskMessage(150))
	if len(provider.snapshots) != 2 || provider.snapshots[1].GoalID != "goal-created" {
		t.Fatalf("post-terminal snapshots = %#v, want child fixed to original Goal", provider.snapshots)
	}
}

type fakePersistentDMGoalProvider struct {
	*fakeGoalContextProvider
	snapshots []protocol.GoalUsageSourceSnapshot
	claims    []protocol.GoalUsageSourceRoundClaim
}

type orderedDMBindingGoalProvider struct {
	*fakeGoalContextProvider
	sourceEntered chan struct{}
	sourceRelease chan struct{}
	bindEntered   chan struct{}
	sourceErr     error
	bindErr       error
	snapshots     []protocol.GoalUsageSourceSnapshot
	bindings      []protocol.GoalUsageScopeBinding
	order         []string
}

func (p *orderedDMBindingGoalProvider) RecordUsageSourceSnapshot(
	_ context.Context,
	snapshot protocol.GoalUsageSourceSnapshot,
) (protocol.GoalUsageSourceResult, error) {
	p.snapshots = append(p.snapshots, snapshot)
	p.order = append(p.order, "source")
	if p.sourceEntered != nil {
		close(p.sourceEntered)
	}
	if p.sourceRelease != nil {
		<-p.sourceRelease
	}
	return protocol.GoalUsageSourceResult{}, p.sourceErr
}

func (p *orderedDMBindingGoalProvider) BindUsageScopeFromNow(
	_ context.Context,
	binding protocol.GoalUsageScopeBinding,
) (protocol.GoalUsageScopeBindResult, error) {
	p.bindings = append(p.bindings, binding)
	p.order = append(p.order, "bind")
	if p.bindEntered != nil {
		close(p.bindEntered)
	}
	return protocol.GoalUsageScopeBindResult{}, p.bindErr
}

func (p *fakePersistentDMGoalProvider) RecordUsageSourceSnapshot(
	_ context.Context,
	snapshot protocol.GoalUsageSourceSnapshot,
) (protocol.GoalUsageSourceResult, error) {
	p.snapshots = append(p.snapshots, snapshot)
	return protocol.GoalUsageSourceResult{}, nil
}

func (p *fakePersistentDMGoalProvider) ClaimUsageSourceRound(
	_ context.Context,
	claim protocol.GoalUsageSourceRoundClaim,
) (protocol.GoalUsageSourceResult, error) {
	p.claims = append(p.claims, claim)
	return protocol.GoalUsageSourceResult{}, nil
}

func TestRoundRunnerPersistsAndSilentlyEnrichesGoalCompletionReceipt(t *testing.T) {
	root := t.TempDir()
	workspacePath := filepath.Join(root, "agent-1")
	sessionKey := "agent:agent-1:ws:dm:goal-receipt"
	history := workspacestore.NewAgentHistoryStore(root)
	provider := &fakeDMGoalUsageFinalizer{
		fakeGoalContextProvider: &fakeGoalContextProvider{},
		report: protocol.GoalUsageReport{
			GoalID:          "goal-1",
			SessionKey:      sessionKey,
			Status:          protocol.GoalStatusComplete,
			TimeUsedSeconds: 754,
		},
	}
	assistant := protocol.Message{
		"message_id":  "assistant-final",
		"session_key": sessionKey,
		"agent_id":    "agent-1",
		"round_id":    "round-1",
		"role":        "assistant",
		"timestamp":   int64(1000),
		"content":     []map[string]any{{"type": "text", "text": "最终交付"}},
	}
	runner := &roundRunner{
		service:        &Service{goals: provider, Host: runtimehost.Host{Runtime: runtimectx.NewManager(), History: history, Permission: permissionctx.NewContext()}},
		workspacePath:  workspacePath,
		session:        protocol.Session{SessionKey: sessionKey, AgentID: "agent-1"},
		sessionKey:     sessionKey,
		roundID:        "round-1",
		GoalRoundState: runtimehost.GoalRoundState{CompletionCandidateID: "goal-1", CompletionAssistant: assistant, CompletionReceiptStored: false},
	}

	runner.persistGoalCompletionReceipt(context.Background(), false)
	first := readGoalCompletionReceipt(t, history, workspacePath, runner.session)
	if first.TimeUsedSeconds == nil || *first.TimeUsedSeconds != 754 || first.ActualTokens != nil {
		t.Fatalf("first receipt = %+v, want duration only", first)
	}

	provider.mu.Lock()
	provider.report.UsageFinalized = true
	provider.report.Usage = protocol.GoalUsage{ActualTotalTokens: 62762, ActualTotalKnown: true}
	provider.mu.Unlock()
	runner.persistGoalCompletionReceipt(context.Background(), true)
	final := readGoalCompletionReceipt(t, history, workspacePath, runner.session)
	if final.ActualTokens == nil || *final.ActualTokens != 62762 {
		t.Fatalf("final receipt = %+v, want authoritative actual tokens", final)
	}

	runner.persistGoalCompletionReceipt(context.Background(), true)
	messages, err := history.ReadMessages(workspacePath, runner.session, nil)
	if err != nil {
		t.Fatal(err)
	}
	if countMessagesByID(messages, "assistant-final") != 1 {
		t.Fatalf("messages = %+v, want one merged final assistant", messages)
	}
}

func TestRoundRunnerUsesGoalIDFromCompletionCommandReceipt(t *testing.T) {
	receipts := nexusmcp.NewCommandReceiptState()
	runner := &roundRunner{
		service:         &Service{goals: &fakeGoalContextProvider{}, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
		commandReceipts: receipts,
	}
	receipts.Record(nexusmcp.CommandReceipt{
		Domain: command.DomainGoal, Operation: command.GoalOperationUpdate,
		Outcome: string(protocol.MutationResultApplied), GoalID: "goal-from-receipt",
		GoalStatus: string(protocol.GoalStatusComplete),
	})
	runner.recordGoalUsageFromAssistantMessage(goalCommandAssistantMessage(protocol.GoalStatusComplete))
	if runner.CompletionCandidateID != "goal-from-receipt" {
		t.Fatalf("complete update candidate = %q, want exact receipt Goal ID", runner.CompletionCandidateID)
	}
}

func goalCommandAssistantMessage(status protocol.GoalStatus) protocol.Message {
	return protocol.Message{
		"message_id": "assistant-update-" + string(status),
		"role":       "assistant",
		"content":    []map[string]any{{"type": "text", "text": "Goal command completed"}},
	}
}

func readGoalCompletionReceipt(
	t *testing.T,
	history *workspacestore.AgentHistoryStore,
	workspacePath string,
	session protocol.Session,
) protocol.GoalCompletionReceipt {
	t.Helper()
	messages, err := history.ReadMessages(workspacePath, session, nil)
	if err != nil {
		t.Fatal(err)
	}
	var assistant protocol.Message
	for _, message := range messages {
		if stringValue(message["message_id"]) == "assistant-final" {
			assistant = message
			break
		}
	}
	if assistant == nil {
		t.Fatalf("final assistant missing: %+v", messages)
	}
	raw, ok := assistant[protocol.GoalCompletionReceiptField].(map[string]any)
	if !ok {
		t.Fatalf("receipt missing from history: %+v", assistant)
	}
	receipt := protocol.GoalCompletionReceipt{
		GoalID:  stringValue(raw["goal_id"]),
		RoundID: stringValue(raw["round_id"]),
	}
	if value, ok := receiptInt64(raw["time_used_seconds"]); ok {
		seconds := value
		receipt.TimeUsedSeconds = &seconds
	}
	if value, ok := receiptInt64(raw["actual_tokens"]); ok {
		tokens := value
		receipt.ActualTokens = &tokens
	}
	return receipt
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func countMessagesByID(messages []protocol.Message, messageID string) int {
	count := 0
	for _, message := range messages {
		if stringValue(message["message_id"]) == messageID {
			count++
		}
	}
	return count
}

func receiptInt64(value any) (int64, bool) {
	switch typed := value.(type) {
	case int64:
		return typed, true
	case float64:
		return int64(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

// A user stop on a running Goal continuation must suspend continuation and
// settle its launch receipt on every stop path: an exact-round stop and a
// session stop that the provider acknowledges, and a provider that never
// answers so the host cancels locally. A host control stop (Goal pause,
// automation cancel) only settles the receipt and never pauses the queue.
func TestServiceUserStopOnGoalContinuationRecordsUserStop(t *testing.T) {
	for _, mode := range []string{"exact", "session", "silent-provider", "host-control"} {
		t.Run(mode, func(t *testing.T) {
			cfg := newDMTestConfig(t)
			agentService := newDMAgentService(t, cfg)
			client := newFakeDMClient()
			queryStarted := make(chan struct{}, 1)
			client.onQuery = func(context.Context, string) { queryStarted <- struct{}{} }
			client.onInterrupt = func(context.Context) {
				if mode == "silent-provider" {
					return
				}
				go func() {
					client.messages <- sdkprotocol.ReceivedMessage{
						Type:      sdkprotocol.MessageTypeResult,
						SessionID: client.sessionID,
						UUID:      "result-user-stop-" + mode,
						Result: &sdkprotocol.ResultMessage{
							Subtype: "interrupted", DurationMS: 1, NumTurns: 1,
							Result: messagepkg.InterruptWithoutMessage,
						},
					}
				}()
			}
			runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client})
			service := NewService(cfg, agentService, runtimeManager, permissionctx.NewContext())
			service.SetInboxStore(newDMInboxStore(t, cfg))
			sessionKey := "agent:nexus:ws:dm:user-stop-" + mode
			t.Cleanup(func() { _ = runtimeManager.CloseSession(context.Background(), sessionKey) })
			goal := protocol.Goal{
				ID: "goal-user-stop", SessionKey: sessionKey, Objective: "stop means stop", Status: protocol.GoalStatusActive,
				Metadata: map[string]any{protocol.GoalMetadataObjectiveRevision: int64(1)},
			}
			plan := protocol.GoalContinuation{
				Goal: goal, RoundID: "goal_continuation_user_stop_" + mode, Prompt: "continue",
				HiddenFromUser: true, Synthetic: true, Purpose: "goal_continuation",
			}
			goalProvider := &fakeGoalContextProvider{reservation: true, continuationCount: 1, runtimeContext: "ctx", runtimeGoal: &goal}
			service.SetGoalContextProvider(goalProvider)

			if err := service.DispatchGoalContinuation(context.Background(), plan); err != nil {
				t.Fatal(err)
			}
			select {
			case <-queryStarted:
			case <-time.After(2 * time.Second):
				t.Fatal("continuation did not start")
			}
			request := InterruptRequest{SessionKey: sessionKey, RoundID: plan.RoundID, UserStop: mode != "host-control"}
			if mode == "session" {
				request.RoundID = ""
			}
			if err := service.HandleInterrupt(context.Background(), request); err != nil {
				t.Fatalf("interrupt: %v", err)
			}
			waitForDMRuntimeIdle(t, runtimeManager, sessionKey)
			deadline := time.Now().Add(2 * time.Second)
			for {
				goalProvider.mu.Lock()
				stops := slices.Clone(goalProvider.userStopRoundIDs)
				revisions := slices.Clone(goalProvider.userStopRevisions)
				progress := len(goalProvider.progress)
				failures := len(goalProvider.failures)
				settled := goalProvider.settledCalls
				goalProvider.mu.Unlock()
				if mode == "host-control" && (settled > 0 || time.Now().After(deadline)) {
					if len(stops) != 0 || progress != 0 || failures != 0 || settled == 0 {
						t.Fatalf("host control stop: stops=%v progress=%d failures=%d settled=%d, want receipt settled only", stops, progress, failures, settled)
					}
					_, location, err := service.resolveInputQueueLocation(context.Background(), sessionKey, "")
					if err != nil {
						t.Fatal(err)
					}
					if paused, err := service.InputQueue.DispatchPaused(location); err != nil || paused {
						t.Fatalf("host control stop paused the queue: paused=%v err=%v", paused, err)
					}
					return
				}
				if mode != "host-control" && (len(stops) > 0 || time.Now().After(deadline)) {
					if !slices.Equal(stops, []string{plan.RoundID}) || !slices.Equal(revisions, []int64{1}) {
						t.Fatalf("user stops = %v revisions = %v, want exactly the stopped round at revision 1", stops, revisions)
					}
					if progress != 0 || failures != 0 {
						t.Fatalf("progress=%d failures=%d, want a user stop never counted as progress or failure", progress, failures)
					}
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
		})
	}
}

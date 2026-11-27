// goal_runtime_test.go 覆盖 Room Goal 运行时边界、父账本与用量重绑并发测试。
package realtime

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	goalappserver "github.com/nexus-research-lab/nexus/internal/service/goal/appserver"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// accelerateRoomGoalUsageRetry 缩短白盒测试中的退避时钟，不改变生产默认值。
func accelerateRoomGoalUsageRetry(service *Service) {
	if service != nil {
		service.goalUsageRetryBaseDelay = time.Millisecond
	}
}

func TestRoomGoalCreateBindsEverySlotToSharedGoalID(t *testing.T) {
	sharedSessionKey := "room:group:conversation-bind"
	goalProvider := &fakeRoomGoalContextProvider{
		usageGoal: &protocol.Goal{ID: "goal-room-created", SessionKey: sharedSessionKey},
		runtimeGoals: map[string]*protocol.Goal{
			sharedSessionKey: {ID: "goal-room-created", SessionKey: sharedSessionKey},
		},
	}
	creator := &activeRoomSlot{
		RuntimeSessionKey:     "slot-session-creator",
		AgentRoundID:          "round-bind:creator",
		GoalUsageScopeRoundID: "root-bind",
	}
	peer := &activeRoomSlot{
		RuntimeSessionKey:     "slot-session-peer",
		AgentRoundID:          "round-bind:peer",
		GoalUsageScopeRoundID: "root-bind",
	}
	unrelated := &activeRoomSlot{
		RuntimeSessionKey:     "slot-session-unrelated",
		AgentRoundID:          "round-bind:unrelated",
		GoalUsageScopeRoundID: "root-unrelated",
	}
	for _, slot := range []*activeRoomSlot{creator, peer, unrelated} {
		slot.setGoalBinding(sharedSessionKey, "")
		slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(false))
	}
	service := &Service{
		goals: goalProvider,
		rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{
			"round-bind": {
				SessionKey:  sharedSessionKey,
				RootRoundID: "root-bind",
				Slots: map[string]*activeRoomSlot{
					"creator": creator,
				},
			},
			"round-bind-handoff": {
				SessionKey:  sharedSessionKey,
				RootRoundID: "root-bind",
				Slots: map[string]*activeRoomSlot{
					"peer": peer,
				},
			},
			"round-unrelated": {
				SessionKey:  sharedSessionKey,
				RootRoundID: "root-unrelated",
				Slots: map[string]*activeRoomSlot{
					"unrelated": unrelated,
				},
			},
		}),
	}

	stageRoomAppliedGoalCommand(creator, command.GoalOperationCreate, "goal-room-created", "")
	service.recordGoalUsageFromSlotAssistantMessage(
		context.Background(),
		creator,
		roomGoalToolResultAssistantMessage("tool-create", "Bash", 4, 1),
	)
	if creator.mutable.goal.UsageGoalID() != "goal-room-created" || peer.mutable.goal.UsageGoalID() != "goal-room-created" {
		t.Fatalf("slot bindings = creator:%q peer:%q, want shared goal-room-created",
			creator.mutable.goal.UsageGoalID(),
			peer.mutable.goal.UsageGoalID(),
		)
	}
	if unrelated.mutable.goal.UsageGoalID() != "" {
		t.Fatalf("same-session unrelated root binding = %q, want empty", unrelated.mutable.goal.UsageGoalID())
	}
	service.finalizeGoalUsageForSlot(context.Background(), peer, exec.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  9,
			OutputTokens: 3,
			TotalTokens:  12,
		},
	}, nil)
	if len(goalProvider.usageGoalIDs) != 1 || goalProvider.usageGoalIDs[0] != "goal-room-created" {
		t.Fatalf("peer terminal usage targets = %#v, want fixed shared Goal ID", goalProvider.usageGoalIDs)
	}
}

func TestRoomSlotsRecordNXSSubagentActualUsagePerRuntimeSession(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &Service{goals: goalProvider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}}
	newSlot := func(sessionKey string, roundID string) *activeRoomSlot {
		slot := &activeRoomSlot{RuntimeSessionKey: sessionKey, AgentRoundID: roundID}
		slot.setRuntimeKind("nxs")
		slot.setGoalBinding("room:group:conversation-1", "goal-1")
		slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(true))
		return slot
	}
	taskMessage := protocol.Message{"metadata": map[string]any{
		"task_id": "task-1",
		"usage":   map[string]any{"total_tokens": int64(100)},
	}}

	service.recordSubagentGoalUsageForSlot(context.Background(), newSlot("slot-session-1", "round-1"), taskMessage)
	service.recordSubagentGoalUsageForSlot(context.Background(), newSlot("slot-session-2", "round-2"), taskMessage)

	usages := goalProvider.recordedUsage()
	if len(usages) != 2 || usages[0].ActualTokens() != 100 || usages[1].ActualTokens() != 100 {
		t.Fatalf("usages = %#v, want same task ID isolated across Room slot runtime sessions", usages)
	}
}

func TestRegisterSlotGoalRuntimeUsesGoalSessionKey(t *testing.T) {
	manager := runtimectx.NewManager()
	service := &Service{Host: runtimehost.Host{Runtime: manager}}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:group:conversation-1",
		AgentRoundID:      "room-round-1:agent-1",
	}
	slot.setGoalBinding("room:group:conversation-1", "")

	cleanup := service.registerSlotGoalRuntime(slot)
	goalSessionKey := slot.goalSessionKey()
	if roundIDs := manager.GetRunningRoundIDs(goalSessionKey); len(roundIDs) != 0 {
		t.Fatalf("Goal accounting 不应伪造 shared running round: %#v", roundIDs)
	}
	if roundIDs, err := manager.FlushGoalAccounting(context.Background(), goalSessionKey); err != nil || len(roundIDs) != 1 || roundIDs[0] != slot.AgentRoundID {
		t.Fatalf("FlushGoalAccounting() = %#v, %v, want slot accounting", roundIDs, err)
	}
	if roundIDs := manager.ClearGoalAccounting(goalSessionKey); len(roundIDs) != 1 || roundIDs[0] != slot.AgentRoundID {
		t.Fatalf("ClearGoalAccounting() = %#v, want slot accounting", roundIDs)
	}
	if roundIDs, err := manager.ActivateGoalAccounting(context.Background(), goalSessionKey, "goal-shared"); err != nil || len(roundIDs) != 1 || roundIDs[0] != slot.AgentRoundID {
		t.Fatalf("ActivateGoalAccounting() = %#v, %v, want slot accounting", roundIDs, err)
	}
	if slot.mutable.goal.UsageGoalID() != "goal-shared" {
		t.Fatalf("slot goal binding = %q, want goal-shared", slot.mutable.goal.UsageGoalID())
	}
	if _, err := manager.QueueGuidanceInput(context.Background(), goalSessionKey, "goal-event-1", "budget reached"); !errors.Is(err, runtimectx.ErrNoRunningRound) {
		t.Fatalf("shared Goal accounting 不应伪装 guidance runtime: %v", err)
	}

	cleanup()
	if roundIDs, err := manager.FlushGoalAccounting(context.Background(), goalSessionKey); err != nil || len(roundIDs) != 0 {
		t.Fatalf("cleanup 后 FlushGoalAccounting() = %#v, %v", roundIDs, err)
	}
}

func TestQueueRoomContextualGuidanceTargetsEveryActiveSlotExceptCaller(t *testing.T) {
	manager := runtimectx.NewManager()
	sessionKey := "room:group:conversation-1"
	lead := &activeRoomSlot{
		AgentID:           "agent-lead",
		AgentRoundID:      "round-root:agent-lead",
		RuntimeSessionKey: "agent:lead:ws:group:conversation-1",
	}
	caller := &activeRoomSlot{
		AgentID:           "agent-peer",
		AgentRoundID:      "round-root:agent-peer",
		RuntimeSessionKey: "agent:peer:ws:group:conversation-1",
	}
	_ = manager.StartRound(context.Background(), lead.RuntimeSessionKey, lead.AgentRoundID, nil)
	_ = manager.StartRound(context.Background(), caller.RuntimeSessionKey, caller.AgentRoundID, nil)
	grantTestRoomGoalAuthority(lead, sessionKey, "goal-room")
	grantTestRoomGoalAuthority(caller, sessionKey, "goal-room")
	service := &Service{
		Host: runtimehost.Host{Runtime: manager},
		rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{
			"round-root": {
				SessionKey:  sessionKey,
				RoundID:     "round-root",
				RootRoundID: "round-root",
				Slots: map[string]*activeRoomSlot{
					lead.AgentID:   lead,
					caller.AgentID: caller,
				},
			},
		}),
	}
	revision := service.GoalObjectiveRevisionState(sessionKey, "round-root", lead.AgentID, 1)
	if revision == nil || revision.Load() != 1 {
		t.Fatalf("initial revision = %v, want shared state at 1", revision)
	}

	roundIDs, err := service.QueueRoomContextualGuidanceInput(
		context.Background(),
		sessionKey,
		"goal-event-1",
		"goal",
		"The objective changed.",
		caller.AgentID,
		2,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(roundIDs) != 1 || roundIDs[0] != lead.AgentRoundID {
		t.Fatalf("roundIDs = %#v, want lead only", roundIDs)
	}
	if got := manager.PendingGuidanceCount(lead.RuntimeSessionKey); got != 1 {
		t.Fatalf("lead pending guidance = %d, want 1", got)
	}
	if got := manager.PendingGuidanceCount(caller.RuntimeSessionKey); got != 0 {
		t.Fatalf("caller pending guidance = %d, want 0", got)
	}
	if got := revision.Load(); got != 1 {
		t.Fatalf("revision before guidance consumption = %d, want 1", got)
	}
	options := manager.WithGuidanceHook(agentclient.Options{}, lead.RuntimeSessionKey)
	if _, err := options.Hooks.Matchers[sdkhook.EventPostToolUse][0].Hooks[0](
		context.Background(),
		sdkhook.Input{EventName: sdkhook.EventPostToolUse},
		"tool-before-retarget",
	); err != nil {
		t.Fatal(err)
	}
	if got := revision.Load(); got != 2 || lead.currentGoalObjectiveRevision() != 2 {
		t.Fatalf("revision after guidance consumption = pointer:%d slot:%d, want 2", got, lead.currentGoalObjectiveRevision())
	}
	lead.adoptGoalObjectiveRevision(1)
	if got := revision.Load(); got != 2 {
		t.Fatalf("an older guidance callback regressed revision to %d, want 2", got)
	}
}

func TestResolveGoalRuntimeContextForSlotKeepsSharedSessionForFutureRoomGoal(t *testing.T) {
	sharedSessionKey := "room:group:conversation-1"
	runtimeSessionKey := "agent:nexus:ws:group:conversation-1"
	service := &Service{goals: &fakeRoomGoalContextProvider{}}
	slot := &activeRoomSlot{RuntimeSessionKey: runtimeSessionKey}

	prompt, goalContext, goalID, goalSessionKey, _ := service.resolveGoalRuntimeContextForSlot(
		context.Background(),
		&activeRoomRound{SessionKey: sharedSessionKey},
		slot,
		"base prompt",
	)

	if goalID != "" || goalContext != "" {
		t.Fatalf("goalID=%q goalContext=%q, want no current goal", goalID, goalContext)
	}
	if goalSessionKey != sharedSessionKey {
		t.Fatalf("goalSessionKey = %q, want shared session for future room goal", goalSessionKey)
	}
	if prompt != "base prompt" {
		t.Fatalf("prompt = %q, want unchanged system prompt", prompt)
	}
}

func TestClearGoalUsageForRoomSlotStopsLaterAccounting(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &Service{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:test",
		AgentRoundID:      "round-1",
	}
	slot.setGoalBinding("", "goal-1")
	slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(true))

	clearGoalUsageForSlot(slot)
	service.recordGoalUsageForSlot(context.Background(), slot, exec.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  6,
			OutputTokens: 3,
			TotalTokens:  9,
		},
	}, nil)

	if usages := goalProvider.recordedUsage(); len(usages) != 0 {
		t.Fatalf("usages = %#v, want none after clear", usages)
	}
}

func TestRecordGoalUsageLimitForRoomSlot(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &Service{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:test",
		AgentRoundID:      "round-1",
	}

	service.recordGoalUsageLimitForSlot(context.Background(), slot, exec.RoundExecutionResult{
		UsageLimitReached: true,
		UsageLimitReason:  "The usage limit has been reached",
	})

	reasons := goalProvider.recordedUsageLimitReasons()
	if len(reasons) != 1 || reasons[0] != "The usage limit has been reached" {
		t.Fatalf("usage limit reasons = %#v, want runtime reason", reasons)
	}
}

// Goal 运行时测试替身与消息构造器。

type fakeRoomGoalContextProvider struct {
	mu                     sync.Mutex
	runtimeContexts        map[string]string
	runtimeGoals           map[string]*protocol.Goal
	usage                  []protocol.GoalUsage
	usageGoal              *protocol.Goal
	usageSessionKeys       []string
	usageGoalIDs           []string
	usageLimitReason       []string
	usageLimitKeys         []string
	progress               []bool
	progressRevision       []int64
	progressRoundIDs       []string
	failures               []string
	failureRoundIDs        []string
	completionMisses       []string
	completionMissRoundIDs []string
	activities             []string
	handbacks              []string
	collabEvidence         []string
	events                 []protocol.GoalEvent
	plan                   *protocol.GoalContinuation
	planCalls              int
	stillCurrent           bool
	claimCalls             int
	startedCalls           int
	startedErr             error
	onStarted              func()
	beforeStarted          func()
	settledCalls           int
	settledRoundIDs        []string
	userStopRoundIDs       []string
	retryReasons           []string
	releaseCalls           int
	onPlan                 func()
}

func (p *fakeRoomGoalContextProvider) RuntimeContext(_ context.Context, sessionKey string) (string, *protocol.Goal, error) {
	goal := p.runtimeGoals[sessionKey]
	if goal == nil {
		return "", nil, goalsvc.ErrGoalNotFound
	}
	value := *goal
	return p.runtimeContexts[sessionKey], &value, nil
}

func (p *fakeRoomGoalContextProvider) CurrentOptional(_ context.Context, sessionKey string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return cloneRoomGoal(p.runtimeGoals[sessionKey]), nil
}

func (p *fakeRoomGoalContextProvider) RecordUsageForSession(_ context.Context, sessionKey string, usage protocol.GoalUsage, _ string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.usageSessionKeys = append(p.usageSessionKeys, sessionKey)
	p.usage = append(p.usage, usage)
	return cloneRoomGoal(p.usageGoal), nil
}

func (p *fakeRoomGoalContextProvider) RecordUsageForGoal(_ context.Context, goalID string, usage protocol.GoalUsage, _ string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.usage = append(p.usage, usage)
	p.usageGoalIDs = append(p.usageGoalIDs, goalID)
	return cloneRoomGoal(p.usageGoal), nil
}

func cloneRoomGoal(item *protocol.Goal) *protocol.Goal {
	if item == nil {
		return nil
	}
	value := *item
	return &value
}

func (p *fakeRoomGoalContextProvider) UsageLimitForSession(_ context.Context, sessionKey string, _ string, reason string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.usageLimitKeys = append(p.usageLimitKeys, sessionKey)
	p.usageLimitReason = append(p.usageLimitReason, reason)
	return nil, nil
}

func (p *fakeRoomGoalContextProvider) RecordContinuationRuntimeProgress(_ context.Context, _ string, identity goalsvc.ContinuationRuntimeIdentity, progressed bool, revisions ...int64) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.progress = append(p.progress, progressed)
	p.progressRoundIDs = append(p.progressRoundIDs, strings.TrimSpace(identity.AuditRoundID))
	p.settledCalls++
	p.settledRoundIDs = append(p.settledRoundIDs, strings.TrimSpace(identity.ReceiptRoundID))
	if len(revisions) > 0 {
		p.progressRevision = append(p.progressRevision, revisions[0])
	}
	return nil, nil
}

func (p *fakeRoomGoalContextProvider) RecordContinuationRuntimeFailure(_ context.Context, _ string, identity goalsvc.ContinuationRuntimeIdentity, reason string, _ ...int64) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failures = append(p.failures, strings.TrimSpace(reason))
	p.failureRoundIDs = append(p.failureRoundIDs, strings.TrimSpace(identity.AuditRoundID))
	p.settledCalls++
	p.settledRoundIDs = append(p.settledRoundIDs, strings.TrimSpace(identity.ReceiptRoundID))
	return nil, nil
}

func (p *fakeRoomGoalContextProvider) RecordContinuationUserStop(_ context.Context, _ string, identity goalsvc.ContinuationRuntimeIdentity, _ ...int64) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.userStopRoundIDs = append(p.userStopRoundIDs, strings.TrimSpace(identity.AuditRoundID))
	p.settledCalls++
	p.settledRoundIDs = append(p.settledRoundIDs, strings.TrimSpace(identity.ReceiptRoundID))
	return nil, nil
}

func (p *fakeRoomGoalContextProvider) RecordContinuationRuntimeCompletionCommandMiss(_ context.Context, _ string, identity goalsvc.ContinuationRuntimeIdentity, reason string, _ ...int64) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.completionMisses = append(p.completionMisses, strings.TrimSpace(reason))
	p.completionMissRoundIDs = append(p.completionMissRoundIDs, strings.TrimSpace(identity.AuditRoundID))
	p.settledCalls++
	p.settledRoundIDs = append(p.settledRoundIDs, strings.TrimSpace(identity.ReceiptRoundID))
	return nil, nil
}

func (p *fakeRoomGoalContextProvider) RecordGoalActivity(_ context.Context, _ string, roundID string, _ ...int64) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.activities = append(p.activities, strings.TrimSpace(roundID))
	return nil, nil
}

func (p *fakeRoomGoalContextProvider) RecordRoomGoalCollaborationHandback(_ context.Context, _ string, roundID string, _ ...int64) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handbacks = append(p.handbacks, strings.TrimSpace(roundID))
	return nil, nil
}

func (p *fakeRoomGoalContextProvider) RecordRoomGoalCollaborationEvidence(_ context.Context, _ string, roundID string, agentID string, _ ...int64) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.collabEvidence = append(p.collabEvidence, strings.TrimSpace(roundID)+":"+strings.TrimSpace(agentID))
	return nil, nil
}

func (p *fakeRoomGoalContextProvider) Events(_ context.Context, goalID string, limit int) ([]protocol.GoalEvent, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	items := make([]protocol.GoalEvent, 0, len(p.events))
	for _, event := range p.events {
		if strings.TrimSpace(event.GoalID) != strings.TrimSpace(goalID) {
			continue
		}
		items = append(items, event)
		if limit > 0 && len(items) >= limit {
			break
		}
	}
	return items, nil
}

func (p *fakeRoomGoalContextProvider) PlanContinuationForSession(context.Context, string, string) (*protocol.GoalContinuation, error) {
	p.mu.Lock()
	p.planCalls++
	onPlan := p.onPlan
	plan := p.plan
	p.mu.Unlock()
	if onPlan != nil {
		onPlan()
	}
	return plan, nil
}

func (p *fakeRoomGoalContextProvider) GoalContinuationStillCurrent(context.Context, protocol.GoalContinuation) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stillCurrent, nil
}

func (p *fakeRoomGoalContextProvider) ClaimContinuationPlan(context.Context, protocol.GoalContinuation) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.claimCalls++
	return nil, nil
}

func (p *fakeRoomGoalContextProvider) ReleaseContinuationPlan(context.Context, protocol.GoalContinuation, string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.releaseCalls++
	return nil, nil
}

func (p *fakeRoomGoalContextProvider) MarkContinuationPlanStarted(context.Context, protocol.GoalContinuation) error {
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

func (p *fakeRoomGoalContextProvider) SettleContinuationPlan(_ context.Context, _ string, roundID string, _ int64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.settledCalls++
	p.settledRoundIDs = append(p.settledRoundIDs, strings.TrimSpace(roundID))
	return nil
}

func (p *fakeRoomGoalContextProvider) RetryContinuationPlan(_ context.Context, _ protocol.GoalContinuation, reason string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.retryReasons = append(p.retryReasons, strings.TrimSpace(reason))
	return nil
}

func (p *fakeRoomGoalContextProvider) recordedUsage() []protocol.GoalUsage {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]protocol.GoalUsage(nil), p.usage...)
}

func (p *fakeRoomGoalContextProvider) recordedUsageLimitReasons() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.usageLimitReason...)
}

func (p *fakeRoomGoalContextProvider) recordedProgress() []bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]bool(nil), p.progress...)
}

func (p *fakeRoomGoalContextProvider) recordedFailureRoundIDs() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.failureRoundIDs...)
}

func (p *fakeRoomGoalContextProvider) recordedCompletionMissRoundIDs() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.completionMissRoundIDs...)
}

func (p *fakeRoomGoalContextProvider) recordedSettledRoundIDs() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.settledRoundIDs...)
}

func (p *fakeRoomGoalContextProvider) recordedFailures() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.failures...)
}

func (p *fakeRoomGoalContextProvider) recordedCompletionMisses() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.completionMisses...)
}

func roomGoalToolResultAssistantMessage(
	toolUseID string,
	toolName string,
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
			{"type": "tool_result", "tool_use_id": toolUseID},
		},
	}
}

func roomGoalCompletionCommandMissAssistantMessage() protocol.Message {
	return protocol.Message{
		"role": "assistant",
		"content": []map[string]any{
			{"type": "text", "text": "任务已经完成，但无法通过 nexus.command 调用 update_goal 来标记完成。"},
		},
	}
}

func stageRoomRuntimeCommandReceipt(slot *activeRoomSlot, receipt nexusmcp.CommandReceipt) {
	slot.ensureCommandReceiptState().Record(receipt)
}

func stageRoomAppliedGoalCommand(slot *activeRoomSlot, operation string, goalID string, status protocol.GoalStatus) {
	stageRoomRuntimeCommandReceipt(slot, nexusmcp.CommandReceipt{
		Domain: command.DomainGoal, Operation: operation,
		Outcome: string(protocol.MutationResultApplied), GoalID: goalID,
		GoalStatus: string(status),
	})
}

func roomGoalTextAssistantMessage(messageID string, text string) protocol.Message {
	return protocol.Message{
		"message_id": messageID,
		"role":       "assistant",
		"content": []map[string]any{
			{"type": "text", "text": text},
		},
	}
}

func roomGoalAssistantUsageMessage(inputTokens int64, outputTokens int64) protocol.Message {
	return protocol.Message{
		"message_id": "assistant-final",
		"role":       "assistant",
		"usage": map[string]any{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
			"total_tokens":  inputTokens + outputTokens,
		},
	}
}

const (
	realRoomCancellationContent = "@Amy 算了不用了"
	realRoomSessionKey          = "room:group:91c68883cc96"
	realGoalID                  = "goal-real-room-review"
)

type cancellationGoalProvider struct {
	*fakeRoomGoalContextProvider
	current   *protocol.Goal
	clearCall int
	planCall  int
}

func (p *cancellationGoalProvider) CurrentOptional(context.Context, string) (*protocol.Goal, error) {
	return p.current, nil
}

func (p *cancellationGoalProvider) Clear(_ context.Context, goalID string) (bool, error) {
	p.clearCall++
	if p.current == nil || p.current.ID != goalID {
		return false, nil
	}
	p.current = nil
	return true, nil
}

func (p *cancellationGoalProvider) PlanContinuationForSession(context.Context, string, string) (*protocol.GoalContinuation, error) {
	p.planCall++
	if p.current == nil {
		return nil, nil
	}
	return &protocol.GoalContinuation{
		Goal:    *p.current,
		RoundID: "goal_continuation_after_cancel",
	}, nil
}

func (p *cancellationGoalProvider) GoalContinuationStillCurrent(context.Context, protocol.GoalContinuation) (bool, error) {
	return p.current != nil, nil
}

func (p *cancellationGoalProvider) ClaimContinuationPlan(context.Context, protocol.GoalContinuation) (*protocol.Goal, error) {
	return p.current, nil
}

func TestRealRoomCancellationClearsGoalBeforeContinuation(t *testing.T) {
	provider := &cancellationGoalProvider{current: &protocol.Goal{
		ID:         realGoalID,
		SessionKey: realRoomSessionKey,
		Status:     protocol.GoalStatusActive,
	}}
	service := &Service{goals: provider}

	if !isGoalCancellationRequest(realRoomCancellationContent) {
		t.Fatal("真实 Room 引导内容应被识别为明确取消意图")
	}
	if err := service.cancelActiveRoomGoalForUser(
		context.Background(),
		realRoomSessionKey,
		realRoomCancellationContent,
	); err != nil {
		t.Fatalf("清除 active Goal 失败: %v", err)
	}
	if provider.clearCall != 1 || provider.current != nil {
		t.Fatalf("取消应只清除一次 active Goal: calls=%d current=%+v", provider.clearCall, provider.current)
	}

	service.dispatchPostRoundWork(context.Background(), &activeRoomRound{
		SessionKey: realRoomSessionKey,
		RoundID:    "round_after_cancel",
	})
	if provider.planCall != 0 {
		t.Fatalf("取消后的普通 round 不应再检查 Goal 续跑: planCall=%d", provider.planCall)
	}
}

func TestPublishPublicMessageSuppressesTheSameSlotFinalReply(t *testing.T) {
	slot := &activeRoomSlot{
		AgentID: "agent-amy",
	}
	slot.setPendingStream([]protocol.EventMessage{{EventType: protocol.EventTypeStream}})
	slot.beginNoReplyCandidate()
	service := &Service{rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{
		"round-1": {
			SessionKey:  "room:group:conversation-1",
			RootRoundID: "round-1",
			Slots: map[string]*activeRoomSlot{
				"slot-1": slot,
			},
		},
	})}

	if err := service.MarkPublicMessagePublished(
		context.Background(),
		"room:group:conversation-1",
		"round-1",
		"agent-amy",
	); err != nil {
		t.Fatalf("标记主动广播失败: %v", err)
	}
	if !slot.publicMessageWasPublished() || !slot.shouldSuppressOutput() {
		t.Fatalf("主动广播后 slot 必须进入 suppress 状态: %+v", slot)
	}
	if events := slot.eventsReadyForEmission(protocol.EventMessage{EventType: protocol.EventTypeStream}); len(events) != 0 {
		t.Fatalf("主动广播后不应继续向公区发流事件: %+v", events)
	}
	receipt := protocol.EventMessage{
		DeliveryMode: protocol.DeliveryModeDurable,
		EventType:    protocol.EventTypeMessage,
		Data: protocol.Message{
			"role": "assistant",
			"content": []map[string]any{
				{"type": "tool_use", "id": "tool-public-1", "name": "mcp__nexus__send_message"},
				{"type": "tool_result", "tool_use_id": "tool-public-1"},
			},
		},
	}
	events := slot.eventsReadyForEmission(receipt)
	if len(events) != 1 || events[0].DeliveryMode != protocol.DeliveryModeEphemeral {
		t.Fatalf("公区消息工具回执应仅作为 Thread 过程事件发送: %+v", events)
	}
	if events = slot.eventsReadyForEmission(receipt); len(events) != 0 {
		t.Fatalf("主动广播回执收口后不应放行后续输出: %+v", events)
	}
}

// Goal 完成就绪测试。

func TestActiveRoomGoalBlockerExcludesCallerSlotButKeepsRunningWork(t *testing.T) {
	const conversationID = "conversation-goal-ready"
	sessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	caller := &activeRoomSlot{
		AgentID:      "agent-lead",
		AgentRoundID: "agent-round-lead",
	}
	caller.setStatus("running")
	roundValue := &activeRoomRound{
		SessionKey:     sessionKey,
		ConversationID: conversationID,
		RoundID:        "room-round",
		RootRoundID:    "root-round",
		Slots:          map[string]*activeRoomSlot{"caller": caller},
	}
	service := &Service{rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{"round": roundValue})}

	if blocker := service.activeRoomGoalBlocker(sessionKey, conversationID, "agent-lead", "agent-round-lead"); blocker != "" {
		t.Fatalf("caller current slot blocker = %q, want empty", blocker)
	}
	if blocker := service.activeRoomGoalBlocker(sessionKey, conversationID, "agent-lead", ""); !strings.Contains(blocker, "active Room slot") {
		t.Fatalf("caller without precise round blocker = %q, want fail-closed active slot", blocker)
	}

	caller.setSubagentTasks(map[string]struct{}{"task-running": {}})
	if blocker := service.activeRoomGoalBlocker(sessionKey, conversationID, "agent-lead", "agent-round-lead"); !strings.Contains(blocker, "running subagent work") {
		t.Fatalf("caller subagent blocker = %q, want running subagent work", blocker)
	}
	caller.setSubagentTasks(nil)

	peer := &activeRoomSlot{AgentID: "agent-peer", AgentRoundID: "agent-round-peer"}
	peer.setStatus("running")
	roundValue.Slots["peer"] = peer
	if blocker := service.activeRoomGoalBlocker(sessionKey, conversationID, "agent-lead", "agent-round-lead"); !strings.Contains(blocker, "agent-peer") {
		t.Fatalf("peer slot blocker = %q, want active peer", blocker)
	}

	peer.setStatus("finished")
	peer.setSubagentTasks(map[string]struct{}{"peer-task": {}})
	if blocker := service.activeRoomGoalBlocker(sessionKey, conversationID, "agent-lead", "agent-round-lead"); !strings.Contains(blocker, "agent-peer still has running subagent work") {
		t.Fatalf("peer subagent blocker = %q, want peer subagent even after main slot terminal", blocker)
	}
	peer.setSubagentTasks(nil)

	service.rounds.enqueuePublicMention(roundValue, publicMentionWake{TargetAgentID: "agent-peer"})
	if blocker := service.activeRoomGoalBlocker(sessionKey, conversationID, "agent-lead", "agent-round-lead"); !strings.Contains(blocker, "public-mention wake") {
		t.Fatalf("public mention blocker = %q, want pending wake", blocker)
	}
}

func TestRoomGoalDurableBlockersIgnoreRetargetedCollaborationRevision(t *testing.T) {
	root := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, root)
	t.Setenv("NEXUS_CONFIG_DIR", root)
	const (
		ownerUserID    = "owner-goal-stale-blockers"
		conversationID = "conversation-goal-stale-blockers"
		roomID         = "room-goal-stale-blockers"
		targetAgentID  = "agent-goal-stale-blockers"
	)
	goal := &protocol.Goal{
		ID:         "goal-stale-blockers",
		SessionKey: protocol.BuildRoomSharedSessionKey(conversationID),
		Status:     protocol.GoalStatusActive,
		Metadata: map[string]any{
			protocol.GoalMetadataObjectiveRevision: int64(2),
		},
	}
	binding := &protocol.GoalCollaborationBinding{
		GoalID: goal.ID, ObjectiveRevision: 1,
	}
	wakeStore := workspacestore.NewRoomDirectedMessageWakeStore(root)
	wake := workspacestore.RoomDirectedMessageWake{
		WakeID: "wake-goal-stale-blockers", OwnerUserID: ownerUserID,
		Message: protocol.RoomDirectedMessageRecord{
			MessageID: "wake-goal-stale-blockers", RoomID: roomID,
			ConversationID: conversationID, WakePolicy: protocol.RoomWakePolicyDelayed,
			GoalCollaborationBinding: binding,
		},
		DueAt: time.Now().Add(time.Minute).UnixMilli(),
	}
	if err := wakeStore.Schedule(wake); err != nil {
		t.Fatal(err)
	}
	queueStore := workspacestore.NewInputQueueStore(root)
	workspacePath := filepath.Join(appfs.UserWorkspaceRoot(ownerUserID), targetAgentID)
	location := workspacestore.InputQueueLocation{
		OwnerUserID: ownerUserID, Scope: protocol.InputQueueScopeRoom,
		WorkspacePath: workspacePath,
		SessionKey: protocol.BuildRoomAgentSessionKey(
			conversationID,
			targetAgentID,
			protocol.RoomTypeGroup,
		),
		RoomID: roomID, ConversationID: conversationID,
	}
	if _, err := queueStore.Enqueue(location, protocol.InputQueueItem{
		ID: "queue-goal-stale-blockers", AgentID: targetAgentID,
		TargetAgentIDs: []string{targetAgentID}, Source: protocol.InputQueueSourceAgentRoomMessage,
		Content: "old revision", DeliveryPolicy: protocol.ChatDeliveryPolicyQueue,
		GoalCollaborationBinding: binding,
	}); err != nil {
		t.Fatal(err)
	}
	contextValue := &protocol.ConversationContextAggregate{
		Room:         protocol.RoomRecord{ID: roomID, OwnerUserID: ownerUserID, RoomType: protocol.RoomTypeGroup},
		Conversation: protocol.ConversationRecord{ID: conversationID, RoomID: roomID},
		Members: []protocol.MemberRecord{{
			MemberType: protocol.MemberTypeAgent, MemberAgentID: targetAgentID,
		}},
		MemberAgents: []protocol.Agent{{AgentID: targetAgentID, WorkspacePath: workspacePath}},
	}
	service := &Service{directedWakes: wakeStore, Host: runtimehost.Host{InputQueue: queueStore}}
	if blocker, err := service.roomGoalInputQueueBlocker(
		context.Background(), contextValue, goal,
	); err != nil || blocker != "" {
		t.Fatalf("retargeted queue blocker = %q err=%v, want empty", blocker, err)
	}
	if blocker, err := service.roomGoalDirectedWakeBlocker(
		ownerUserID, conversationID, goal,
	); err != nil || blocker != "" {
		t.Fatalf("retargeted wake blocker = %q err=%v, want empty", blocker, err)
	}
}

func TestRoomGoalCompletionReportIgnoresMemberCount(t *testing.T) {
	contextValue := newAuthorityFenceContext()
	contextValue.Room.OwnerUserID = "owner-completion-membership"
	store := &authorityFenceRoomStore{contextValue: contextValue}
	service := withConstructorDefaults(t, &Service{rooms: store})
	goal := protocol.Goal{
		ID:         "goal-completion-membership",
		SessionKey: protocol.BuildRoomSharedSessionKey(contextValue.Conversation.ID),
		Status:     protocol.GoalStatusActive,
	}

	report, err := service.RoomGoalCompletionReport(
		context.Background(),
		goal,
		"agent-a",
		"round-lead",
	)
	if err != nil {
		t.Fatal(err)
	}
	if report.Blocker != "" {
		t.Fatalf("report = %#v, want member count not to create a blocker", report)
	}

	store.update(func(current *protocol.ConversationContextAggregate) {
		current.Members = current.Members[:1]
	})
	report, err = service.RoomGoalCompletionReport(
		context.Background(),
		goal,
		"agent-a",
		"round-lead",
	)
	if err != nil {
		t.Fatal(err)
	}
	if report.Blocker != "" {
		t.Fatalf("report = %#v, want single-Agent Room without dynamic requirement", report)
	}
}

func (s *Service) resolveGoalRuntimeContextForSlot(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	appendSystemPrompt string,
) (string, string, string, string, int64) {
	defaultGoalSessionKey := ""
	if roundValue != nil {
		defaultGoalSessionKey = strings.TrimSpace(roundValue.SessionKey)
	}
	for _, sessionKey := range goalSessionCandidates(roundValue, slot) {
		goalContext, goalID, objectiveRevision, ok := s.goalRuntimeContext(ctx, sessionKey)
		if !ok {
			continue
		}
		if slot != nil {
			slot.ensureGoalObjectiveRevision(objectiveRevision)
		}
		return appendSystemPrompt, goalContext, goalID, sessionKey, objectiveRevision
	}
	return appendSystemPrompt, "", "", defaultGoalSessionKey, 0
}

func goalSessionCandidates(roundValue *activeRoomRound, slot *activeRoomSlot) []string {
	candidates := []string{}
	if roundValue != nil {
		roundSessionKey := strings.TrimSpace(roundValue.SessionKey)
		if protocol.IsRoomSharedSessionKey(roundSessionKey) {
			return []string{roundSessionKey}
		}
		candidates = append(candidates, roundSessionKey)
	}
	if slot != nil {
		candidates = append(candidates, slot.RuntimeSessionKey)
	}
	result := make([]string, 0, len(candidates))
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		sessionKey := strings.TrimSpace(candidate)
		if sessionKey == "" {
			continue
		}
		if _, exists := seen[sessionKey]; exists {
			continue
		}
		seen[sessionKey] = struct{}{}
		result = append(result, sessionKey)
	}
	return result
}

func (s *Service) recordGoalUsageForSlot(
	ctx context.Context,
	slot *activeRoomSlot,
	result exec.RoundExecutionResult,
	finalAssistant protocol.Message,
) {
	if s.goals == nil || slot == nil || slot.goalRuntimeIgnored() {
		return
	}
	snapshot, ok := slotFinalGoalUsageSnapshot(slot, result, finalAssistant)
	if !ok {
		return
	}
	_ = runtimehost.PersistGoalUsageWithRetry(ctx, s.goalUsageRetryBaseDelay, func() bool { return s.settleTerminalGoalUsageSnapshotForSlot(ctx, slot, snapshot) })
}

func (s *Service) recordGoalUsageFromSlotAssistantMessage(
	ctx context.Context,
	slot *activeRoomSlot,
	message protocol.Message,
) {
	s.recordGoalUsageFromSlotAssistantMessageWithActor(ctx, slot, nil, message)
}

func TestRoomExternalCompleteKeepsBindingUntilTerminalThenFinalizes(t *testing.T) {
	fixture := newRoomGoalBoundaryFixture(t, "external-complete")

	fixture.recordAssistantUsage(90, 10)
	complete := goalappserver.ThreadGoalStatusComplete
	completed, err := fixture.goals.SetFromThreadGoalParams(
		context.Background(),
		goalappserver.ThreadGoalSetParams{
			ThreadID: fixture.goal.SessionKey,
			Status:   &complete,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if completed.UsageFinalized {
		t.Fatal("external completion finalized usage before the running Room slot terminal")
	}
	if fixture.slot.mutable.goal.UsageGoalID() != fixture.goal.ID || !fixture.slot.goalUsageActive() {
		t.Fatal("external completion cleared the Room slot Goal binding before terminal usage")
	}

	fixture.slot.setStatus("finished")
	fixture.room.finalizeGoalUsageForSlot(
		context.Background(),
		fixture.slot,
		exec.RoundExecutionResult{Usage: sdkprotocol.TokenUsage{
			InputTokens:  90,
			OutputTokens: 10,
			TotalTokens:  100,
		}},
		nil,
	)
	if !fixture.room.finalizeCompletedRoomGoalUsage(context.Background(), fixture.round) {
		t.Fatal("shared Room Goal usage did not finalize after the parent terminal")
	}
	stored, err := fixture.repo.GetGoal(context.Background(), fixture.goal.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil ||
		!stored.UsageFinalized ||
		stored.Usage.ActualTokens() != 100 ||
		stored.Usage.ActualTokensAreEstimated() {
		t.Fatalf("finalized Goal = %#v, want exact terminal actual 100", stored)
	}

	fixture.cleanup()
	if rounds := fixture.manager.BeginGoalAccountingFinalizing(fixture.goal.SessionKey); len(rounds) != 0 {
		t.Fatalf("cleanup left Room finalization hooks registered: %#v", rounds)
	}
}

type roomGoalBoundaryFixture struct {
	room    *Service
	goals   *goalsvc.Service
	manager *runtimectx.Manager
	repo    *roomGoalBoundaryRepository
	goal    *protocol.Goal
	slot    *activeRoomSlot
	round   *activeRoomRound
	cleanup func()
}

func (f roomGoalBoundaryFixture) recordAssistantUsage(inputTokens int64, outputTokens int64) {
	message := roomGoalAssistantUsageMessage(inputTokens, outputTokens)
	f.slot.mutable.goal.RememberGoalAssistantMessage(message)
	f.room.recordGoalUsageFromSlotAssistantMessage(context.Background(), f.slot, message)
}

func newRoomGoalBoundaryFixture(t *testing.T, suffix string) roomGoalBoundaryFixture {
	t.Helper()
	sessionKey := protocol.BuildRoomSharedSessionKey("conversation-" + suffix)
	slot := &activeRoomSlot{
		AgentID:      "agent-" + suffix,
		AgentRoundID: "agent-round-" + suffix,
		RuntimeSessionKey: protocol.BuildRoomAgentSessionKey(
			"conversation-"+suffix,
			"agent-"+suffix,
			protocol.RoomTypeGroup,
		),
	}
	slot.setGoalBinding(sessionKey, "")
	slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(false))
	slot.setStatus("running")
	roundValue := &activeRoomRound{
		SessionKey:     sessionKey,
		ConversationID: "conversation-" + suffix,
		RoundID:        "room-round-" + suffix,
		OwnerUserID:    "owner-" + suffix,
		Slots:          map[string]*activeRoomSlot{"worker": slot},
	}
	manager := runtimectx.NewManager()
	repo := newRoomGoalBoundaryRepository()
	goalService := goalsvc.NewService(config.Config{GoalEnabled: true}, repo)
	goalService.SetExternalMutationAccountant(manager)
	roomService := &Service{
		goals: goalService,
		Host:  runtimehost.Host{Runtime: manager},
		rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{
			roundValue.RoundID: roundValue,
		}),
	}
	cleanup := roomService.registerSlotGoalRuntime(slot)
	created, err := goalService.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: sessionKey,
		Objective:  "Exercise Room Goal boundary " + suffix,
		CreatedBy:  "user",
	})
	if err != nil {
		cleanup()
		t.Fatal(err)
	}
	if slot.mutable.goal.UsageGoalID() != created.ID || !slot.goalUsageActive() {
		cleanup()
		t.Fatalf("external create did not activate Room slot Goal %q", created.ID)
	}
	return roomGoalBoundaryFixture{
		room:    roomService,
		goals:   goalService,
		manager: manager,
		repo:    repo,
		goal:    created,
		slot:    slot,
		round:   roundValue,
		cleanup: cleanup,
	}
}

type roomGoalBoundaryRepository struct {
	mu      sync.Mutex
	goals   map[string]protocol.Goal
	events  []protocol.GoalEvent
	deleted *protocol.Goal
}

func newRoomGoalBoundaryRepository() *roomGoalBoundaryRepository {
	return &roomGoalBoundaryRepository{goals: make(map[string]protocol.Goal)}
}

func (r *roomGoalBoundaryRepository) CreateGoal(
	_ context.Context,
	item protocol.Goal,
) (*protocol.Goal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.goals[item.ID] = item
	return cloneRoomBoundaryGoal(item), nil
}

func (r *roomGoalBoundaryRepository) CreateGoalWithEvent(
	_ context.Context,
	item protocol.Goal,
	event protocol.GoalEvent,
) (*protocol.Goal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.goals[item.ID] = item
	r.events = append(r.events, event)
	return cloneRoomBoundaryGoal(item), nil
}

func (r *roomGoalBoundaryRepository) GetGoal(
	_ context.Context,
	goalID string,
) (*protocol.Goal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.goals[goalID]
	if !ok {
		return nil, nil
	}
	return cloneRoomBoundaryGoal(item), nil
}

func (r *roomGoalBoundaryRepository) GetCurrentGoal(
	_ context.Context,
	sessionKey string,
) (*protocol.Goal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range r.goals {
		if item.SessionKey == sessionKey && protocol.IsCurrentGoalStatus(item.Status) {
			return cloneRoomBoundaryGoal(item), nil
		}
	}
	return nil, nil
}

func (r *roomGoalBoundaryRepository) ListGoals(
	_ context.Context,
) ([]protocol.Goal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]protocol.Goal, 0, len(r.goals))
	for _, item := range r.goals {
		items = append(items, item)
	}
	return items, nil
}

func (r *roomGoalBoundaryRepository) ListCurrentGoals(
	_ context.Context,
) ([]protocol.Goal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]protocol.Goal, 0, len(r.goals))
	for _, item := range r.goals {
		if protocol.IsCurrentGoalStatus(item.Status) {
			items = append(items, item)
		}
	}
	return items, nil
}

func (r *roomGoalBoundaryRepository) ListRunnableGoals(
	_ context.Context,
	limit int,
) ([]protocol.Goal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]protocol.Goal, 0, len(r.goals))
	for _, item := range r.goals {
		if protocol.NormalizeGoalStatus(item.Status) == protocol.GoalStatusActive {
			items = append(items, item)
		}
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (r *roomGoalBoundaryRepository) UpdateGoal(
	_ context.Context,
	item protocol.Goal,
	expectedVersion int64,
) (*protocol.Goal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.goals[item.ID]
	if !ok || current.Version != expectedVersion {
		return nil, sql.ErrNoRows
	}
	r.goals[item.ID] = item
	return cloneRoomBoundaryGoal(item), nil
}

func (r *roomGoalBoundaryRepository) UpdateGoalWithEvents(
	_ context.Context,
	item protocol.Goal,
	expectedVersion int64,
	events []protocol.GoalEvent,
) (*protocol.Goal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.goals[item.ID]
	if !ok || current.Version != expectedVersion {
		return nil, sql.ErrNoRows
	}
	r.goals[item.ID] = item
	r.events = append(r.events, events...)
	return cloneRoomBoundaryGoal(item), nil
}

func (r *roomGoalBoundaryRepository) FinalizeGoalUsage(
	_ context.Context,
	item protocol.Goal,
	expectedVersion int64,
	event protocol.GoalEvent,
) (*protocol.Goal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.goals[item.ID]
	if !ok ||
		current.Version != expectedVersion ||
		current.UsageFinalized ||
		protocol.NormalizeGoalStatus(current.Status) != protocol.GoalStatusComplete {
		return nil, sql.ErrNoRows
	}
	r.goals[item.ID] = item
	r.events = append(r.events, event)
	return cloneRoomBoundaryGoal(item), nil
}

func (r *roomGoalBoundaryRepository) DeleteGoal(
	_ context.Context,
	goalID string,
) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.goals[goalID]
	if !ok {
		return false, nil
	}
	r.deleted = cloneRoomBoundaryGoal(item)
	delete(r.goals, goalID)
	return true, nil
}

func (r *roomGoalBoundaryRepository) AppendEvent(
	_ context.Context,
	event protocol.GoalEvent,
) error {
	r.mu.Lock()
	r.events = append(r.events, event)
	r.mu.Unlock()
	return nil
}

func (r *roomGoalBoundaryRepository) ListEvents(
	_ context.Context,
	goalID string,
	_ int,
) ([]protocol.GoalEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]protocol.GoalEvent, 0)
	for _, event := range r.events {
		if event.GoalID == goalID {
			items = append(items, event)
		}
	}
	return items, nil
}

func cloneRoomBoundaryGoal(item protocol.Goal) *protocol.Goal {
	cloned := item
	return &cloned
}

type failingRoomChildFlushProvider struct {
	*fakeRoomGoalContextProvider
	mu        sync.Mutex
	bindCalls int
}

type returningRoomChildGoalProvider struct {
	*fakeRoomGoalContextProvider
	goal protocol.Goal
}

func (p *returningRoomChildGoalProvider) RecordUsageSourceSnapshot(
	_ context.Context,
	_ protocol.GoalUsageSourceSnapshot,
) (protocol.GoalUsageSourceResult, error) {
	goal := p.goal
	return protocol.GoalUsageSourceResult{Goal: &goal}, nil
}

func (p *failingRoomChildFlushProvider) RecordUsageSourceSnapshot(
	_ context.Context,
	_ protocol.GoalUsageSourceSnapshot,
) (protocol.GoalUsageSourceResult, error) {
	return protocol.GoalUsageSourceResult{}, errors.New("child checkpoint unavailable")
}

func (p *failingRoomChildFlushProvider) BindUsageScopeFromNow(
	_ context.Context,
	_ protocol.GoalUsageScopeBinding,
) (protocol.GoalUsageScopeBindResult, error) {
	p.mu.Lock()
	p.bindCalls++
	p.mu.Unlock()
	return protocol.GoalUsageScopeBindResult{}, nil
}

func TestRoomExternalBindRequiresKnownChildPendingToFlush(t *testing.T) {
	provider := &failingRoomChildFlushProvider{
		fakeRoomGoalContextProvider: &fakeRoomGoalContextProvider{},
	}
	service := &Service{goals: provider}
	slot := &activeRoomSlot{
		OwnerUserID:           "owner-child-flush-failure",
		AgentRoundID:          "slot-child-flush-failure",
		GoalUsageScopeRoundID: "root-child-flush-failure",
		RuntimeSessionKey:     "agent:nexus:workspace:group:child-flush-failure",
	}
	slot.setRuntimeKind("nxs")
	slot.setGoalBinding("room:group:child-flush-failure", "")
	slot.mutable.goal.MarkSubagentUsagePending("task-running", goalsvc.SubagentUsageObservation{ObservedAt: time.Now().UTC()})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := service.activateGoalUsageForSlot(ctx, slot, "goal-new"); err == nil {
		t.Fatal("external Room Goal bind succeeded with an unflushed child pending snapshot")
	}
	provider.mu.Lock()
	bindCalls := provider.bindCalls
	provider.mu.Unlock()
	if bindCalls != 0 {
		t.Fatalf("BindUsageScopeFromNow calls = %d, want 0 before child flush succeeds", bindCalls)
	}
	if got := slot.mutable.goal.UsageGoalID(); got != "" {
		t.Fatalf("Goal binding = %q, want old unbound state after flush failure", got)
	}
	if pending := slot.subagentUsageObservationPendingSnapshot(); len(pending) != 1 {
		t.Fatalf("pending child snapshots = %#v, want retained retry state", pending)
	}
}

func TestRoomChildResultBindsOnlyMatchingRootScope(t *testing.T) {
	const sessionKey = "room:group:child-result-scope"
	provider := &returningRoomChildGoalProvider{
		fakeRoomGoalContextProvider: &fakeRoomGoalContextProvider{},
		goal: protocol.Goal{
			ID:         "goal-child-result-scope",
			SessionKey: sessionKey,
		},
	}
	origin := &activeRoomSlot{
		AgentID:               "agent-origin",
		AgentRoundID:          "slot-origin",
		GoalUsageScopeRoundID: "root-matching",
		RuntimeSessionKey:     "agent:origin:workspace:group:child-result-scope",
	}
	peer := &activeRoomSlot{
		AgentID:               "agent-peer",
		AgentRoundID:          "slot-peer",
		GoalUsageScopeRoundID: "root-matching",
		RuntimeSessionKey:     "agent:peer:workspace:group:child-result-scope",
	}
	unrelated := &activeRoomSlot{
		AgentID:               "agent-unrelated",
		AgentRoundID:          "slot-unrelated",
		GoalUsageScopeRoundID: "root-unrelated",
		RuntimeSessionKey:     "agent:unrelated:workspace:group:child-result-scope",
	}
	for _, slot := range []*activeRoomSlot{origin, peer, unrelated} {
		slot.setRuntimeKind("nxs")
		slot.setGoalBinding(sessionKey, "")
	}
	service := &Service{
		goals: provider,
		rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{
			"matching": {
				ConversationID: "child-result-scope",
				SessionKey:     sessionKey,
				RootRoundID:    "root-matching",
				Slots: map[string]*activeRoomSlot{
					origin.AgentID: origin,
					peer.AgentID:   peer,
				},
			},
			"unrelated": {
				ConversationID: "child-result-scope",
				SessionKey:     sessionKey,
				RootRoundID:    "root-unrelated",
				Slots: map[string]*activeRoomSlot{
					unrelated.AgentID: unrelated,
				},
			},
		}),
	}
	settled := service.recordSubagentGoalUsageForSlot(
		context.Background(),
		origin,
		protocol.Message{"metadata": map[string]any{
			"task_id":   "task-child-result",
			"task_type": "local_agent",
			"subtype":   "task_progress",
			"status":    "running",
			"usage":     map[string]any{"total_tokens": int64(20)},
		}},
	)
	if len(settled) != 1 {
		t.Fatalf("settled child snapshots = %#v, want one", settled)
	}
	for _, slot := range []*activeRoomSlot{origin, peer} {
		if got := slot.mutable.goal.UsageGoalID(); got != provider.goal.ID {
			t.Fatalf("%s Goal binding = %q, want %q", slot.AgentID, got, provider.goal.ID)
		}
	}
	if got := unrelated.mutable.goal.UsageGoalID(); got != "" {
		t.Fatalf("unrelated root Goal binding = %q, want empty", got)
	}
}

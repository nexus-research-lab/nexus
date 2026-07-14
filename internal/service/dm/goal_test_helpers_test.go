package dm

import (
	"context"
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type fakeGoalContextProvider struct {
	mu                  sync.Mutex
	plan                *protocol.GoalContinuation
	planCalls           int
	runtimeContext      string
	runtimeGoal         *protocol.Goal
	runtimeCalls        int
	usage               []protocol.GoalUsage
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

func (p *fakeGoalContextProvider) RecordUsageForSession(_ context.Context, _ string, usage protocol.GoalUsage, _ string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.usage = append(p.usage, usage)
	return nil, nil
}

func (p *fakeGoalContextProvider) RecordUsageForGoal(_ context.Context, _ string, usage protocol.GoalUsage, _ string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.usage = append(p.usage, usage)
	return nil, nil
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

func (p *fakeGoalContextProvider) RecordContinuationFailure(_ context.Context, _ string, _ string, reason string, revisions ...int64) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failures = append(p.failures, strings.TrimSpace(reason))
	p.failureRevisions = append(p.failureRevisions, firstTestRevision(revisions))
	return nil, nil
}

func (p *fakeGoalContextProvider) RecordCompletionToolMiss(_ context.Context, _ string, _ string, reason string, revisions ...int64) (*protocol.Goal, error) {
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

func (p *fakeGoalContextProvider) recordedUsage() []protocol.GoalUsage {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]protocol.GoalUsage(nil), p.usage...)
}

func (p *fakeGoalContextProvider) recordedUsageLimitReasons() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.usageLimitReason...)
}

func (p *fakeGoalContextProvider) recordedProgress() []bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]bool(nil), p.progress...)
}

func (p *fakeGoalContextProvider) recordedFailures() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.failures...)
}

func (p *fakeGoalContextProvider) recordedCompletionMisses() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.completionMisses...)
}

func firstTestRevision(revisions []int64) int64 {
	if len(revisions) == 0 {
		return 0
	}
	return revisions[0]
}

func (p *fakeGoalContextProvider) runtimeContextCallCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.runtimeCalls
}

func goalToolResultAssistantMessage(
	toolUseID string,
	toolName string,
	isError bool,
	inputTokens int64,
	outputTokens int64,
) protocol.Message {
	return protocol.Message{
		"role": "assistant",
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

func goalAssistantUsageMessage(inputTokens int64, outputTokens int64) protocol.Message {
	return protocol.Message{
		"role": "assistant",
		"usage": map[string]any{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
			"total_tokens":  inputTokens + outputTokens,
		},
	}
}

func goalCompletionToolMissAssistantMessage() protocol.Message {
	return protocol.Message{
		"role": "assistant",
		"content": []map[string]any{
			{"type": "text", "text": "任务已经完成，但我没有看到 mcp__nexus_goal__update_goal 工具，无法调用它来标记完成。"},
		},
	}
}

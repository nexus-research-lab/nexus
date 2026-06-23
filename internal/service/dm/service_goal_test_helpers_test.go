package dm

import (
	"context"
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type fakeGoalContextProvider struct {
	mu               sync.Mutex
	plan             *protocol.GoalContinuation
	planCalls        int
	runtimeContext   string
	runtimeGoal      *protocol.Goal
	runtimeCalls     int
	usage            []protocol.GoalUsage
	usageLimitReason []string
	progress         []bool
	failures         []string
	completionMisses []string
	activities       []string
	current          *bool
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

func (p *fakeGoalContextProvider) RecordContinuationProgress(_ context.Context, _ string, _ string, progressed bool) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.progress = append(p.progress, progressed)
	return nil, nil
}

func (p *fakeGoalContextProvider) RecordContinuationFailure(_ context.Context, _ string, _ string, reason string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failures = append(p.failures, strings.TrimSpace(reason))
	return nil, nil
}

func (p *fakeGoalContextProvider) RecordCompletionToolMiss(_ context.Context, _ string, _ string, reason string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.completionMisses = append(p.completionMisses, strings.TrimSpace(reason))
	return nil, nil
}

func (p *fakeGoalContextProvider) RecordGoalActivity(_ context.Context, _ string, roundID string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.activities = append(p.activities, strings.TrimSpace(roundID))
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

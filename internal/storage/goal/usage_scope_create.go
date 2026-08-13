// INPUT: 新 Goal、created 事件与 model round 的 durable usage scope 绑定。
// OUTPUT: Goal + created event + binding + 首次 pending 回补 + usage event 的原子创建结果。
// POS: model 创建 Goal 时关闭 child usage 崩溃窗口的 SQL 事务边界。
package goal

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// CreateGoalWithUsageScope 在一个事务内创建 Goal 与 created 事件、建立 scope
// binding，并认领 scope 下所有 runtime session 的 pending child usage。
func (r *Repository) CreateGoalWithUsageScope(
	ctx context.Context,
	goal protocol.Goal,
	createdEvent protocol.GoalEvent,
	binding protocol.GoalUsageScopeBinding,
) (protocol.GoalUsageScopeCreateResult, error) {
	goal = normalizeGoalCreate(goal)
	createdEvent = normalizeGoalCreateEvent(createdEvent)
	binding = normalizeGoalUsageScopeBinding(binding)
	if err := validateGoalUsageScopeCreate(goal, createdEvent, binding); err != nil {
		return protocol.GoalUsageScopeCreateResult{}, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return protocol.GoalUsageScopeCreateResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := r.insertGoal(ctx, tx, goal); err != nil {
		return protocol.GoalUsageScopeCreateResult{}, err
	}
	if err := r.insertGoalEvent(ctx, tx, createdEvent); err != nil {
		return protocol.GoalUsageScopeCreateResult{}, err
	}
	if err := r.establishGoalUsageScopeBinding(ctx, tx, binding); err != nil {
		return protocol.GoalUsageScopeCreateResult{}, err
	}

	pendingCount, pendingTokens, err := r.lockGoalUsageScopePending(ctx, tx, binding)
	if err != nil {
		return protocol.GoalUsageScopeCreateResult{}, err
	}
	parentCount, parentUnavailable, parentUsage, err := r.lockGoalUsageParentPending(ctx, tx, binding)
	if err != nil {
		return protocol.GoalUsageScopeCreateResult{}, err
	}
	childUsage := (protocol.GoalUsage{
		ActualTotalTokens: pendingTokens,
		ActualTotalKnown:  true,
		BudgetTotalKnown:  true,
	}).NormalizeTotals()
	attributedUsage := parentUsage.Add(childUsage)
	var usageEvent *protocol.GoalEvent
	if !isStoredGoalUsageZero(attributedUsage) {
		if binding.UsageEventID == "" {
			return protocol.GoalUsageScopeCreateResult{}, fmt.Errorf("goal usage scope create usage event id is required")
		}
		if err := r.addUsageSourceGoalUsage(ctx, tx, &goal, attributedUsage, binding.BoundAt); err != nil {
			return protocol.GoalUsageScopeCreateResult{}, err
		}
	}
	if pendingCount > 0 {
		if err := r.deleteGoalUsageScopePending(ctx, tx, binding, pendingCount); err != nil {
			return protocol.GoalUsageScopeCreateResult{}, err
		}
	}
	if parentCount > 0 {
		if err := r.markGoalUsageParentScopeAttributed(ctx, tx, binding, parentCount); err != nil {
			return protocol.GoalUsageScopeCreateResult{}, err
		}
	}
	if !isStoredGoalUsageZero(attributedUsage) {
		event := usageSourceScopeClaimEvent(
			binding,
			goal,
			binding.ScopeRoundID,
			attributedUsage,
			pendingCount,
			parentCount,
			parentUnavailable,
			"scope_create_backfill",
		)
		if err := r.insertGoalEvent(ctx, tx, event); err != nil {
			return protocol.GoalUsageScopeCreateResult{}, err
		}
		usageEvent = &event
	}

	updated, err := r.getUsageSourceGoal(ctx, tx, goal.ID)
	if err != nil {
		return protocol.GoalUsageScopeCreateResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return protocol.GoalUsageScopeCreateResult{}, err
	}
	return protocol.GoalUsageScopeCreateResult{
		Goal:                  updated,
		UsageEvent:            usageEvent,
		AttributedDelta:       pendingTokens,
		AttributedUsage:       attributedUsage,
		TokenUsageUnavailable: parentUnavailable > 0,
	}, nil
}

func validateGoalUsageScopeCreate(
	goal protocol.Goal,
	createdEvent protocol.GoalEvent,
	binding protocol.GoalUsageScopeBinding,
) error {
	if err := validateGoalCreateEvent(goal, createdEvent); err != nil {
		return err
	}
	if err := validateGoalUsageScopeBinding(binding); err != nil {
		return err
	}
	if binding.GoalID != goal.ID || binding.GoalSessionKey != goal.SessionKey {
		return fmt.Errorf(
			"goal usage scope create binding mismatch: goal=%q/%q binding=%q/%q",
			goal.ID,
			goal.SessionKey,
			binding.GoalID,
			binding.GoalSessionKey,
		)
	}
	if binding.UsageEventID != "" && binding.UsageEventID == strings.TrimSpace(createdEvent.ID) {
		return fmt.Errorf("goal usage scope create event ids must be distinct")
	}
	return nil
}

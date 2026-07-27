// INPUT: external activation 的 Goal、owner/session/root scope 与当前绑定时间。
// OUTPUT: Goal→scope durable 绑定及绑定前 child/parent pending 的原子排除结果。
// POS: Reset accounting 的持久化边界；与 model Create/Claim 的 round-start 回补语义分离。
package goal

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// BindUsageScopeFromNow 建立 durable scope 绑定，但不认领绑定前 backlog。
//
// 外部创建/恢复 Goal 使用 Reset 语义：当前时刻以前的 parent/child usage 不属于
// 新 Goal。绑定和丢弃必须在同一事务；绑定时已经运行的 child 没有同步累计
// 基线，会持久标记 unavailable，既不归属新 Goal 也不能建立最终精度 fence。
func (r *Repository) BindUsageScopeFromNow(
	ctx context.Context,
	binding protocol.GoalUsageScopeBinding,
) (protocol.GoalUsageScopeBindResult, error) {
	binding = normalizeGoalUsageScopeBinding(binding)
	if err := validateGoalUsageScopeBinding(binding); err != nil {
		return protocol.GoalUsageScopeBindResult{}, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return protocol.GoalUsageScopeBindResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	item, err := r.lockUsageSourceGoal(ctx, tx, binding.GoalID)
	if err != nil {
		return protocol.GoalUsageScopeBindResult{}, err
	}
	if err := validateUsageSourceGoal(*item, binding.GoalSessionKey); err != nil {
		return protocol.GoalUsageScopeBindResult{}, err
	}
	if err := r.validateGoalUsageScopeOwner(ctx, tx, item.ID, binding.OwnerUserID); err != nil {
		return protocol.GoalUsageScopeBindResult{}, err
	}
	newlyBound, err := r.establishGoalUsageScopeBindingWithStatus(ctx, tx, binding)
	if err != nil {
		return protocol.GoalUsageScopeBindResult{}, err
	}
	if !newlyBound {
		if err := tx.Commit(); err != nil {
			return protocol.GoalUsageScopeBindResult{}, err
		}
		return protocol.GoalUsageScopeBindResult{}, nil
	}
	childCount, err := r.discardGoalUsageSourcePending(ctx, tx, binding)
	if err != nil {
		return protocol.GoalUsageScopeBindResult{}, err
	}
	evidenceCount, err := r.discardTerminalGoalUsageSourceEvidence(ctx, tx, binding)
	if err != nil {
		return protocol.GoalUsageScopeBindResult{}, err
	}
	if _, err := r.markGoalUsageSourceBaselinesUnavailable(ctx, tx, binding); err != nil {
		return protocol.GoalUsageScopeBindResult{}, err
	}
	parentCount, err := r.discardGoalUsageParentPending(ctx, tx, binding)
	if err != nil {
		return protocol.GoalUsageScopeBindResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return protocol.GoalUsageScopeBindResult{}, err
	}
	return protocol.GoalUsageScopeBindResult{
		DiscardedChildPending:  childCount,
		DiscardedChildEvidence: evidenceCount,
		DiscardedParentPending: parentCount,
	}, nil
}

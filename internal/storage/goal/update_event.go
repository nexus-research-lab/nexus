// INPUT: optimistic-version Goal 更新与同一迁移产生的审计事件。
// OUTPUT: 单事务提交的 Goal 新版本和 append-only events。
// POS: 所有 event-bearing Goal 状态迁移的 SQL 原子边界。
package goal

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// UpdateGoalWithEvents 以 optimistic version 原子更新 Goal 并追加审计事件。
func (r *Repository) UpdateGoalWithEvents(
	ctx context.Context,
	goal protocol.Goal,
	expectedVersion int64,
	events []protocol.GoalEvent,
) (*protocol.Goal, error) {
	goal = normalizeGoalCreate(goal)
	if goal.ID == "" || goal.SessionKey == "" {
		return nil, fmt.Errorf("goal update identity is incomplete")
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("goal update events are required")
	}
	normalizedEvents := make([]protocol.GoalEvent, len(events))
	for index, event := range events {
		event = normalizeGoalCreateEvent(event)
		if err := validateGoalMutationEvent(goal, event); err != nil {
			return nil, err
		}
		normalizedEvents[index] = event
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := r.updateGoal(ctx, tx, goal, expectedVersion); err != nil {
		return nil, err
	}
	for _, event := range normalizedEvents {
		if err := r.insertGoalEvent(ctx, tx, event); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &goal, nil
}

func validateGoalMutationEvent(goal protocol.Goal, event protocol.GoalEvent) error {
	if event.ID == "" ||
		event.GoalID != goal.ID ||
		event.SessionKey != goal.SessionKey ||
		strings.TrimSpace(event.EventType) == "" {
		return fmt.Errorf("goal update event identity is invalid")
	}
	return nil
}

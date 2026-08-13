// INPUT: 新 Goal 与对应 created 审计事件。
// OUTPUT: 同一 SQL 事务内提交的 Goal 与 created event。
// POS: 非 usage-scope Goal 创建的原子持久化边界。
package goal

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// CreateGoalWithEvent 原子创建 Goal 与其 created 审计事件。
func (r *Repository) CreateGoalWithEvent(
	ctx context.Context,
	goal protocol.Goal,
	createdEvent protocol.GoalEvent,
) (*protocol.Goal, error) {
	goal = normalizeGoalCreate(goal)
	createdEvent = normalizeGoalCreateEvent(createdEvent)
	if err := validateGoalCreateEvent(goal, createdEvent); err != nil {
		return nil, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := r.insertGoal(ctx, tx, goal); err != nil {
		return nil, err
	}
	if err := r.insertGoalEvent(ctx, tx, createdEvent); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &goal, nil
}

func normalizeGoalCreate(goal protocol.Goal) protocol.Goal {
	goal.ID = strings.TrimSpace(goal.ID)
	goal.SessionKey = strings.TrimSpace(goal.SessionKey)
	goal.Usage = goal.Usage.NormalizeTotals()
	return goal
}

func normalizeGoalCreateEvent(event protocol.GoalEvent) protocol.GoalEvent {
	event.ID = strings.TrimSpace(event.ID)
	event.GoalID = strings.TrimSpace(event.GoalID)
	event.SessionKey = strings.TrimSpace(event.SessionKey)
	event.EventType = strings.TrimSpace(event.EventType)
	return event
}

func validateGoalCreateEvent(goal protocol.Goal, event protocol.GoalEvent) error {
	if goal.ID == "" || goal.SessionKey == "" {
		return fmt.Errorf("goal create identity is incomplete")
	}
	if event.ID == "" ||
		event.GoalID != goal.ID ||
		event.SessionKey != goal.SessionKey ||
		event.EventType != "created" {
		return fmt.Errorf("goal create event identity is invalid")
	}
	return nil
}

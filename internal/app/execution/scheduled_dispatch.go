// INPUT: Durable scheduling intent and current scope/Goal/queue admission.
// OUTPUT: DM/Room consumer routing without transferring Goal continuation authority.
// POS: App assembly between orchestration, Goal and the two runtime hosts.
package execution

import (
	"context"
	"errors"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestration "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type scheduledQueue interface {
	AutomaticDispatchAllowed(context.Context, protocol.Execution, string) (bool, error)
}
type scheduledDM interface {
	scheduledQueue
	QueueScheduledWork(context.Context, protocol.Execution, protocol.ExecutionWorkBinding, string, string) error
	orchestration.ExecutionReviewDispatchConsumer
}
type scheduledRoom interface {
	scheduledQueue
	orchestration.ExecutionReviewDispatchConsumer
}
type scheduledGoals interface {
	CurrentOptionalForOwner(context.Context, string, string) (*protocol.Goal, error)
}
type ScheduledConsumer struct {
	DM    scheduledDM
	Room  scheduledRoom
	Goals scheduledGoals
}

func (c ScheduledConsumer) AutomaticDispatchAllowed(ctx context.Context, e protocol.Execution, target string) (bool, error) {
	if e.GoalID != "" {
		goal, err := c.Goals.CurrentOptionalForOwner(ctx, e.SessionKey, e.OwnerUserID)
		if err != nil {
			return false, err
		}
		if goal == nil || goal.ID != e.GoalID || goal.ObjectiveRevision() != e.GoalObjectiveRevision || goal.Status != protocol.GoalStatusActive || goal.ContinuationState() == protocol.GoalContinuationStateSuspended {
			return false, nil
		}
	}
	if e.ScopeKind == protocol.ExecutionScopeRoom {
		return c.Room.AutomaticDispatchAllowed(ctx, e, target)
	}
	return c.DM.AutomaticDispatchAllowed(ctx, e, target)
}
func (c ScheduledConsumer) QueueScheduledWork(ctx context.Context, e protocol.Execution, b protocol.ExecutionWorkBinding, instruction, key string) error {
	return c.DM.QueueScheduledWork(ctx, e, b, instruction, key)
}
func (c ScheduledConsumer) DeliverExecutionReviewDispatch(ctx context.Context, d orchestration.ExecutionReviewDispatchDelivery) (orchestration.ExecutionReviewDispatchReceipt, error) {
	if d.RoomID != "" {
		return c.Room.DeliverExecutionReviewDispatch(ctx, d)
	}
	return c.DM.DeliverExecutionReviewDispatch(ctx, d)
}

func (c ScheduledConsumer) QueueContinuedWork(ctx context.Context, e protocol.Execution, b protocol.ExecutionWorkBinding, instruction, key string, revision int64) error {
	queue, ok := c.DM.(interface {
		QueueContinuedWork(context.Context, protocol.Execution, protocol.ExecutionWorkBinding, string, string, int64) error
	})
	if !ok {
		return errors.New("DM continuation queue unavailable")
	}
	return queue.QueueContinuedWork(ctx, e, b, instruction, key, revision)
}

func (c ScheduledConsumer) ContinuationPauseRevision(ctx context.Context, e protocol.Execution, target string) (int64, error) {
	var host any = c.DM
	if e.ScopeKind == protocol.ExecutionScopeRoom {
		host = c.Room
	}
	reader, ok := host.(interface {
		ContinuationPauseRevision(context.Context, protocol.Execution, string) (int64, error)
	})
	if !ok {
		return 0, errors.New("continuation admission unavailable")
	}
	return reader.ContinuationPauseRevision(ctx, e, target)
}

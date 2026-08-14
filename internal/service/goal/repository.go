package goal

import (
	"context"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// Repository 定义 Goal service 依赖的持久化接口。
type Repository interface {
	CreateGoal(context.Context, protocol.Goal) (*protocol.Goal, error)
	CreateGoalWithEvent(context.Context, protocol.Goal, protocol.GoalEvent) (*protocol.Goal, error)
	GetGoal(context.Context, string) (*protocol.Goal, error)
	GetCurrentGoal(context.Context, string) (*protocol.Goal, error)
	ListGoals(context.Context) ([]protocol.Goal, error)
	ListCurrentGoals(context.Context) ([]protocol.Goal, error)
	ListRunnableGoals(context.Context, int) ([]protocol.Goal, error)
	UpdateGoal(context.Context, protocol.Goal, int64) (*protocol.Goal, error)
	UpdateGoalWithEvents(context.Context, protocol.Goal, int64, []protocol.GoalEvent) (*protocol.Goal, error)
	FinalizeGoalUsage(context.Context, protocol.Goal, int64, protocol.GoalEvent) (*protocol.Goal, error)
	DeleteGoal(context.Context, string) (bool, error)
	AppendEvent(context.Context, protocol.GoalEvent) error
	ListEvents(context.Context, string, int) ([]protocol.GoalEvent, error)
}

// continuationPlanRepository is an optional storage capability during rolling
// upgrades. SQL storage implements it; lightweight test repositories retain the
// legacy in-memory reservation behavior until explicitly exercising recovery.
type continuationPlanRepository interface {
	ReserveGoalContinuation(context.Context, protocol.Goal, int64, protocol.GoalEvent, protocol.GoalContinuationPlan) (*protocol.Goal, error)
	GetOpenGoalContinuation(context.Context, string, int64) (*protocol.GoalContinuationPlan, error)
	ClaimGoalContinuation(context.Context, string, time.Time, time.Time) (*protocol.GoalContinuationPlan, error)
	MarkGoalContinuationStarted(context.Context, string, time.Time, time.Time) error
	SettleGoalContinuation(context.Context, string, string, int64, time.Time) error
	RetryGoalContinuation(context.Context, string, string, time.Time, time.Time) error
	ReleaseGoalContinuation(context.Context, protocol.Goal, int64, protocol.GoalEvent, string, time.Time) (*protocol.Goal, error)
}

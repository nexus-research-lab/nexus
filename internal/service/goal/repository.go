package goal

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// Repository 定义 Goal service 依赖的持久化接口。
type Repository interface {
	CreateGoal(context.Context, protocol.Goal) (*protocol.Goal, error)
	GetGoal(context.Context, string) (*protocol.Goal, error)
	GetCurrentGoal(context.Context, string) (*protocol.Goal, error)
	ListRunnableGoals(context.Context, int) ([]protocol.Goal, error)
	UpdateGoal(context.Context, protocol.Goal, int64) (*protocol.Goal, error)
	AppendEvent(context.Context, protocol.GoalEvent) error
	ListEvents(context.Context, string, int) ([]protocol.GoalEvent, error)
	CreateCheckpoint(context.Context, protocol.GoalCheckpoint) (*protocol.GoalCheckpoint, error)
	LatestCheckpoint(context.Context, string) (*protocol.GoalCheckpoint, error)
}

// INPUT: Goal MCP 工具所需的服务能力与当前 runtime 上下文。
// OUTPUT: create/get/retarget/update 共用的窄服务契约。
// POS: Goal MCP 工具与 service/goal 之间的消费侧接口。
package contract

import (
	"context"
	"sync/atomic"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const ServerName = "nexus_goal"

// Service 定义 Goal MCP server 需要的最小服务能力。
type Service interface {
	Create(context.Context, protocol.CreateGoalRequest) (*protocol.Goal, error)
	Current(context.Context, string) (*protocol.Goal, error)
	CurrentOptional(context.Context, string) (*protocol.Goal, error)
	RetargetByModel(context.Context, string, protocol.RetargetGoalRequest) (*protocol.Goal, error)
	CompleteByModel(context.Context, string, protocol.CompleteGoalRequest) (*protocol.Goal, error)
	BlockByModel(context.Context, string, protocol.BlockGoalRequest) (*protocol.Goal, error)
}

// ServerContext 绑定当前运行时会话。
type ServerContext struct {
	CurrentSessionKey     string
	CurrentRoundID        string
	CurrentAgentID        string
	GoalObjectiveRevision *atomic.Int64
}

// NewGoalObjectiveRevision 创建可由同一 MCP server 内 retarget_goal 原子推进的 revision 状态。
func NewGoalObjectiveRevision(value int64) *atomic.Int64 {
	state := &atomic.Int64{}
	state.Store(value)
	return state
}

// ExpectedGoalObjectiveRevision 返回当前 MCP server 绑定的 objective revision；0 表示不启用 fencing。
func (c ServerContext) ExpectedGoalObjectiveRevision() int64 {
	if c.GoalObjectiveRevision == nil {
		return 0
	}
	return c.GoalObjectiveRevision.Load()
}

// StoreGoalObjectiveRevision 让成功 retarget 的调用方继续操作新 objective，同时不影响其他旧 slot。
func (c ServerContext) StoreGoalObjectiveRevision(value int64) {
	if c.GoalObjectiveRevision == nil || value <= 0 {
		return
	}
	for {
		current := c.GoalObjectiveRevision.Load()
		if value <= current || c.GoalObjectiveRevision.CompareAndSwap(current, value) {
			return
		}
	}
}

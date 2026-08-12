// INPUT: Goal MCP 工具所需的服务能力、可信用户 retarget 来源，以及 runtime 共享的 exact Goal/revision/可选 Execution authority。
// OUTPUT: create/get/retarget/objective-alignment/update 共用的窄服务契约与 durable usage scope owner。
// POS: Goal MCP 工具与 service/goal 之间的消费侧接口。
package contract

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

const ServerName = "nexus_goal"

// Service 定义 Goal MCP server 需要的最小服务能力。
type Service interface {
	Create(context.Context, protocol.CreateGoalRequest) (*protocol.Goal, error)
	Current(context.Context, string) (*protocol.Goal, error)
	CurrentOptional(context.Context, string) (*protocol.Goal, error)
	RetargetByModel(context.Context, string, protocol.RetargetGoalRequest) (*protocol.Goal, error)
	AuditObjectiveAlignmentByModel(context.Context, string, protocol.AuditGoalObjectiveAlignmentRequest) (*protocol.GoalObjectiveAlignmentRecord, error)
	CompleteByModel(context.Context, string, protocol.CompleteGoalRequest) (*protocol.Goal, error)
	BlockByModel(context.Context, string, protocol.BlockGoalRequest) (*protocol.Goal, error)
}

// ServerContext 绑定当前运行时会话。
type ServerContext struct {
	OwnerUserID       string
	CurrentSessionKey string
	CurrentRoundID    string
	CurrentAgentID    string
	GoalAuthority     *runtimectx.GoalAuthorityState
	// AllowUserRetarget 只允许可信、可见的普通用户 round 在 retarget_goal
	// 调用点读取一次当前精确 revision；不会授予其他 Goal/Execution mutation。
	AllowUserRetarget bool
	PlanMode          bool
}

// ExpectedGoalObjectiveRevision 返回当前 MCP server 绑定的 objective revision；0 表示不启用 fencing。
func (c ServerContext) ExpectedGoalObjectiveRevision() int64 {
	authority, ok := c.GoalAuthority.Load()
	if !ok {
		return 0
	}
	return authority.ObjectiveRevision
}

// StoreGoalMutationAuthority 让当前 MCP server 在成功 create/retarget 后继续操作同一状态链。
func (c ServerContext) StoreGoalMutationAuthority(item protocol.Goal) {
	if c.GoalAuthority == nil {
		return
	}
	executionID := ""
	switch protocol.GoalExecutionBindingStateFromGoal(item) {
	case protocol.GoalExecutionBindingStateStandalone,
		protocol.GoalExecutionBindingStateReserved:
	case protocol.GoalExecutionBindingStateConfirmed:
		executionID = protocol.GoalReservedExecutionID(item)
	case protocol.GoalExecutionBindingStatePending,
		protocol.GoalExecutionBindingStateConflict:
		c.GoalAuthority.Clear()
		return
	}
	c.GoalAuthority.Bind(
		item.ID,
		item.ObjectiveRevision(),
		executionID,
	)
}

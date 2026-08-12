// INPUT: 已授权 host Goal command、DM/Room 领域服务与 Goal continuation service。
// OUTPUT: 按 canonical session kind 路由的 Goal 控制事务与 ACK 后续跑。
// POS: 应用组合根的窄适配器；Goal 业务阶段保留在 DM/Room 与 Goal service。
package server

import (
	"context"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	roomrealtime "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
)

type goalCommandRouter struct {
	dm    *dmsvc.Service
	room  *roomrealtime.Service
	goals *goalsvc.Service
}

func (r goalCommandRouter) ExecuteGoalCommand(
	ctx context.Context,
	request protocol.GoalCommandRequest,
) (protocol.GoalCommandResult, error) {
	switch protocol.ParseSessionKey(request.SessionKey).Kind {
	case protocol.SessionKeyKindAgent:
		if r.dm == nil {
			return protocol.GoalCommandResult{}, errors.New("DM service is unavailable")
		}
		return r.dm.SetGoalFromCommand(ctx, request)
	case protocol.SessionKeyKindRoom:
		if r.room == nil {
			return protocol.GoalCommandResult{}, errors.New("Room service is unavailable")
		}
		return r.room.SetGoalFromCommand(ctx, request)
	default:
		return protocol.GoalCommandResult{}, errors.New("Goal command requires an Agent or Room session")
	}
}

func (r goalCommandRouter) DispatchGoalContinuation(ctx context.Context, item protocol.Goal) {
	if r.goals != nil {
		r.goals.DispatchActiveGoalContinuation(ctx, item)
	}
}

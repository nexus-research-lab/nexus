// INPUT: 当前 Goal、MCP server 的 runtime-owned exact Goal/revision/可选 Execution capability。
// OUTPUT: 仅允许当前物理 round 对其绑定的 Goal 状态链执行语义写入。
// POS: retarget/audit/update 共用的 runtime mutation authorization fence。
package tool

import (
	"context"
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/mcp/goal/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func currentGoalForMutation(
	ctx context.Context,
	svc contract.Service,
	sctx contract.ServerContext,
	expectedRevision int64,
) (*protocol.Goal, error) {
	authority, ok := sctx.GoalAuthority.Load()
	if !ok || authority.ObjectiveRevision != expectedRevision {
		return nil, fmt.Errorf(
			"this round cannot mutate the current Goal because it has no exact Goal/revision capability; use the user Goal controls or a Goal-bound continuation",
		)
	}
	current, err := svc.Current(ctx, sctx.CurrentSessionKey)
	if err != nil {
		return nil, err
	}
	if current.ID != authority.GoalID ||
		current.ObjectiveRevision() != expectedRevision ||
		(authority.ExecutionID != "" &&
			protocol.GoalReservedExecutionID(*current) != authority.ExecutionID) {
		return nil, fmt.Errorf(
			"this round's Goal/Execution capability is stale; reload the current Goal through a newly authorized continuation",
		)
	}
	return current, nil
}

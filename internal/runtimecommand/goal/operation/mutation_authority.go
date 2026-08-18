// INPUT: 当前 Goal、command context 的 host-owned exact Goal/revision/可选 Execution capability。
// OUTPUT: 仅允许当前物理 round 对其绑定的 Goal 状态链执行语义写入；负责人新 round 使用启动时私有快照。
// POS: retarget/audit/update 共用的 runtime mutation authorization fence。
package operation

import (
	"context"
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtimecommand/goal/contract"
)

func currentGoalForMutation(
	ctx context.Context,
	svc contract.Service,
	sctx contract.Context,
	expectedRevision int64,
) (*protocol.Goal, error) {
	authority, ok := sctx.GoalAuthority.Load()
	if sctx.ResponsibilityAuthority != nil {
		authority, ok = sctx.ResponsibilityAuthority.LoadGoalAuthority()
	}
	if !ok || authority.ObjectiveRevision != expectedRevision {
		return nil, fmt.Errorf(
			"this round cannot mutate the current Goal because it has no exact Goal/revision capability; use the user Goal controls or continue with the responsible Goal Agent",
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

// currentGoalForRetarget admits one intentionally narrow late bind: a trusted
// visible user round may load the current exact revision only for retarget_goal.
// Other Goal operations and every Execution operation remain ambient-unbound.
func currentGoalForRetarget(
	ctx context.Context,
	svc contract.Service,
	sctx contract.Context,
) (*protocol.Goal, int64, error) {
	expectedRevision := sctx.ExpectedGoalObjectiveRevision()
	if expectedRevision > 0 {
		current, err := currentGoalForMutation(ctx, svc, sctx, expectedRevision)
		return current, expectedRevision, err
	}
	if !sctx.AllowUserRetarget {
		return nil, 0, fmt.Errorf(
			"this round cannot mutate the current Goal because it has no exact Goal/revision capability; use the user Goal controls or continue with the responsible Goal Agent",
		)
	}
	current, err := svc.Current(ctx, sctx.CurrentSessionKey)
	if err != nil {
		return nil, 0, err
	}
	if current == nil || current.ObjectiveRevision() <= 0 {
		return nil, 0, fmt.Errorf("the current Goal has no valid objective revision")
	}
	return current, current.ObjectiveRevision(), nil
}

// INPUT: 可选 explicit Execution id 与 session-bound actor identity。
// OUTPUT: current/explicit 权威状态的紧凑 actor-specific context；Room 成员得到共享图只读投影，verified current coordinator 同时进入当前 physical round 的临时 coordination scope。
// POS: 十二工具集合中的显式恢复/协调入口；成员读取不授予 capability，coordinator 读取会建立 round-local capability，因此不能标记 ReadOnly。
package operation

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimecommand "github.com/nexus-research-lab/nexus/internal/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/runtimecommand/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func getExecution(svc contract.Service, sctx contract.Context) runtimecommand.Operation {
	return runtimecommand.Operation{
		Name: "get_execution",
		Description: "Read the compact authoritative actor-specific view of the current Execution, or one explicit Execution by id. " +
			"Verified Room members receive a shared graph observation with no assignment, review, submission, plan mutation, or coordination authority. Bound actors keep their scoped responsibility view. For the verified coordinator of the current WorkGraph, this explicit call also enters that physical round's temporary coordination scope; it does not mutate the durable graph.",
		SearchHint:  "execution status work item assignment dependency review blocker",
		InputSchema: getExecutionSchema(),
		ContextHandler: func(
			ctx context.Context,
			input map[string]any,
			_ *runtimecommand.CallContext,
		) (runtimecommand.Result, error) {
			var parsed getExecutionInput
			if err := decodeInput(input, &parsed); err != nil {
				return transportErrorResult(err), nil
			}
			actor := sctx.Actor()
			snapshot, err := loadReadableSnapshot(ctx, svc, actor, parsed.ExecutionID)
			if err != nil {
				if result, ok := recoverableMutationRejection(err); ok {
					return result, nil
				}
				return transportErrorResult(err), nil
			}
			if snapshot != nil {
				// RuntimeContext must render the exact snapshot selected above.
				// Without this binding, an explicit historical read could return
				// that Execution's id/revision beside the session's current graph.
				actor.ExecutionID = snapshot.Execution.ID
			}
			isCoordinator := snapshot != nil &&
				snapshot.Execution.CoordinatorAgentID == actor.AgentID &&
				(actor.Role == "" || actor.Role == orchestration.ExecutionActorCoordinator)
			if !isCoordinator && snapshot != nil &&
				snapshot.Execution.ScopeKind == protocol.ExecutionScopeRoom {
				actor.ObservationOnly = true
			}
			if activator, ok := any(svc).(interface {
				ActivateRuntimeCoordination(
					context.Context,
					orchestration.ActorContext,
					*protocol.ExecutionSnapshot,
				) error
			}); ok && isCoordinator {
				if err = activator.ActivateRuntimeCoordination(
					ctx,
					actor,
					snapshot,
				); err != nil {
					return rejectedResult(err.Error()), nil
				}
			}
			return snapshotResult(ctx, svc, actor, snapshot), nil
		},
	}
}

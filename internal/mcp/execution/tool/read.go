// INPUT: 可选 explicit Execution id 与 session-bound actor identity。
// OUTPUT: current/explicit 权威状态的紧凑 actor-specific context；verified current coordinator 同时进入当前 physical round 的临时 coordination scope。
// POS: 十二工具集合中的显式恢复/协调入口；不改 durable graph，但会建立 round-local capability，不能标记 ReadOnly。
package tool

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/mcp/execution/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func getExecution(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name: "get_execution",
		Description: "Read the compact authoritative actor-specific view of the current Execution, or one explicit Execution by id. " +
			"It includes the graph digest, owned work, dependencies, reviews, blockers and allowed next actions without exposing the internal Snapshot. For the verified coordinator of the current WorkGraph, this explicit call also enters that physical round's temporary coordination scope; it does not mutate the durable graph.",
		SearchHint:  "execution status work item assignment dependency review blocker",
		InputSchema: getExecutionSchema(),
		ContextHandler: func(
			ctx context.Context,
			input map[string]any,
			_ *sdktool.CallContext,
		) (sdktool.ToolResult, error) {
			var parsed getExecutionInput
			if err := decodeInput(input, &parsed); err != nil {
				return transportErrorResult(err), nil
			}
			actor := sctx.Actor()
			snapshot, err := loadSnapshot(ctx, svc, actor, parsed.ExecutionID)
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
			if activator, ok := any(svc).(interface {
				ActivateRuntimeCoordination(
					context.Context,
					orchestration.ActorContext,
					*protocol.ExecutionSnapshot,
				) error
			}); ok {
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

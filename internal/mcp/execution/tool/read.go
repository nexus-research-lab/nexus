// INPUT: 可选 explicit Execution id 与 session-bound actor identity。
// OUTPUT: current/explicit 权威状态的紧凑 actor-specific context 及其 revision。
// POS: 十工具集合中的只读恢复入口。
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
		Description: "Get the compact authoritative Execution action view for the current scope, or one explicit Execution by id. " +
			"Use it to recover your role, active immutable Plan, deterministic graph digest, Assignment ownership, dependencies, pending reviews, blockers, and allowed next actions. " +
			"The full internal Snapshot is intentionally not returned.",
		SearchHint:  "execution status work item assignment dependency review blocker",
		InputSchema: getExecutionSchema(),
		Annotations: &sdktool.ToolAnnotations{
			ReadOnlyHint: true,
			ReadOnly:     true,
		},
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

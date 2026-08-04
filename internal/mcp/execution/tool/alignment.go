// INPUT: Agent 选择发起的 Execution 目标对齐报告与 trusted runtime context。
// OUTPUT: 一个可见但不驱动路由的 Gate NodeRun mutation result。
// POS: 可选 objective alignment 观测入口；不是 Goal lifecycle，也不是自动 loop scheduler。
package tool

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/mcp/execution/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func auditExecutionAlignment(
	svc contract.Service,
	sctx contract.ServerContext,
) sdktool.Tool {
	const toolName = "audit_execution_alignment"
	return sdktool.Tool{
		Name: toolName,
		Description: "Record an optional three-state evidence audit of the current Execution objective against its authoritative completion criteria as a visible Gate. " +
			"It never transitions the Execution, starts a Goal, retries work or selects the next route.",
		SearchHint:  "execution objective alignment gate checkpoint evidence loop",
		InputSchema: auditExecutionAlignmentSchema(),
		Annotations: &sdktool.ToolAnnotations{IdempotentHint: true},
		ContextHandler: func(
			ctx context.Context,
			input map[string]any,
			callContext *sdktool.CallContext,
		) (sdktool.ToolResult, error) {
			var parsed auditExecutionAlignmentInput
			if err := decodeInput(input, &parsed); err != nil {
				return transportErrorResult(err), nil
			}
			actor := sctx.Actor()
			snapshot, command, result := mutationEnvelope(
				ctx,
				svc,
				sctx,
				actor,
				parsed.ExecutionID,
				toolName,
				input,
				callContext,
			)
			if result != nil {
				return *result, nil
			}
			response, err := svc.AuditExecutionAlignment(
				ctx,
				actor,
				orchestration.AuditExecutionAlignmentInput{
					ExecutionID:      snapshot.Execution.ID,
					SnapshotRevision: snapshot.Execution.Version,
					CommandID:        command,
					Report:           parsed.report(),
				},
			)
			return serviceMutation(ctx, svc, actor, response, err), nil
		},
	}
}

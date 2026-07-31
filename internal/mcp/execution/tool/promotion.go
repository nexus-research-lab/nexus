// INPUT: 模型的 durable-boundary proposal 与权威 Execution snapshot。
// OUTPUT: 后端 policy 允许时绑定 Goal，否则返回结构化拒绝。
// POS: 模型只能提议 Goal promotion，不能提交证据布尔值或自行创建绑定。
package tool

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/mcp/execution/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func promoteExecutionToGoal(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	const toolName = "promote_execution_to_goal"
	return sdktool.Tool{
		Name: toolName,
		Description: "Propose promoting the current transient Execution into a durable Goal after a real execution boundary is observed. " +
			"The backend independently verifies objective clarity, acceptance criteria, remaining required work, authority, configuration, existing Goal state, and durable evidence. " +
			"Task complexity, Plan length, Room size, or subagent use alone never qualifies.",
		SearchHint:  "promote execution goal persistence boundary recovery wait",
		InputSchema: promoteExecutionSchema(),
		Annotations: &sdktool.ToolAnnotations{IdempotentHint: true},
		ContextHandler: func(ctx context.Context, input map[string]any, callContext *sdktool.CallContext) (sdktool.ToolResult, error) {
			var parsed promoteExecutionInput
			if err := decodeInput(input, &parsed); err != nil {
				return transportErrorResult(err), nil
			}
			actor := sctx.Actor()
			snapshot, command, result := mutationEnvelope(ctx, svc, sctx, actor, parsed.ExecutionID, toolName, input, callContext)
			if result != nil {
				return *result, nil
			}
			response, err := svc.PromoteExecutionToGoal(ctx, actor, orchestration.PromoteExecutionToGoalInput{
				ExecutionID:       snapshot.Execution.ID,
				SnapshotRevision:  snapshot.Execution.Version,
				CommandID:         command,
				ObjectiveProposal: parsed.ObjectiveProposal,
				ActivationReason:  parsed.ActivationReason,
			})
			return serviceMutation(ctx, svc, actor, response, err), nil
		},
	}
}

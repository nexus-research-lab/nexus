// INPUT: Agent 选择的 persistence reason 与权威 Execution snapshot。
// OUTPUT: 权限/状态允许时绑定 Goal，否则返回结构化拒绝。
// POS: Agent 决定是否需要 Goal；adapter 隐藏身份、版本和绑定细节。
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
		Description: "Promote the current transient Execution into a durable Goal when the Agent judges that the objective should survive the current execution boundary. " +
			"Cross-round work, external waits, recovery cost, Room dependencies, or substantial complexity can inform that choice. " +
			"The backend validates objective/criteria presence, authority, user configuration, current state, and Goal conflicts; it does not require a fixed workflow signal or a particular Plan shape.",
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

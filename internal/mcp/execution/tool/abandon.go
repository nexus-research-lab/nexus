// INPUT: explicit current transient Execution reference and concrete user-directed stop reason.
// OUTPUT: Plan Mode validation or atomic cancellation with unmanaged fresh context.
// POS: model semantic stop boundary; it never creates a successor or rewrites a Goal.
package tool

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/mcp/execution/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func abandonExecution(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	const toolName = "abandon_execution"
	return sdktool.Tool{
		Name: toolName,
		Description: "Cancel the referenced current transient Execution because the user explicitly stopped or abandoned that objective. " +
			"The backend atomically cancels the old Plan and unfinished execution chains, preserves immutable submissions, acceptances and audit history, and creates no successor. " +
			"Use this before returning to direct unmanaged work for a different atomic request. Never use it for a Goal-bound Execution or merely to change the route for the same objective.",
		SearchHint:  "abandon cancel stop transient execution objective no successor",
		InputSchema: abandonExecutionSchema(),
		Annotations: &sdktool.ToolAnnotations{
			DestructiveHint: true,
			IdempotentHint:  true,
		},
		ContextHandler: func(
			ctx context.Context,
			input map[string]any,
			callContext *sdktool.CallContext,
		) (sdktool.ToolResult, error) {
			var parsed abandonExecutionInput
			if err := decodeInput(input, &parsed); err != nil {
				return transportErrorResult(err), nil
			}
			actor := sctx.Actor()
			snapshot, err := loadSnapshot(ctx, svc, actor, parsed.ExecutionID)
			if err != nil {
				return transportErrorResult(err), nil
			}
			if snapshot == nil {
				return rejectedResult("explicit execution was not found"), nil
			}
			command, commandErr := commandID(sctx, callContext, toolName, input, 0)
			if commandErr != nil {
				return transportErrorResult(commandErr), nil
			}
			result, serviceErr := svc.AbandonExecution(
				ctx,
				actor,
				orchestration.AbandonExecutionInput{
					ExecutionID:      snapshot.Execution.ID,
					SnapshotRevision: snapshot.Execution.Version,
					CommandID:        command,
					Reason:           parsed.Reason,
				},
			)
			if serviceErr != nil {
				return transportErrorResult(serviceErr), nil
			}
			contextActor := actor
			if result.Outcome != orchestration.MutationRejected {
				contextActor.ExecutionID = ""
				contextActor.WorkBinding = nil
				contextActor.ReviewBinding = nil
			}
			return mutationResult(withFreshExecutionContext(ctx, svc, contextActor, result)), nil
		},
	}
}

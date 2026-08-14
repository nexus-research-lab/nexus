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
		Description: "Atomically cancel the explicitly referenced current transient Execution without a successor while preserving immutable submissions, acceptances and audit history. " +
			"Use only for explicit abandonment, never for a Goal-bound Execution or a route change within the same objective.",
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
			applyMutationResponsibilityAuthority(sctx, result)
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

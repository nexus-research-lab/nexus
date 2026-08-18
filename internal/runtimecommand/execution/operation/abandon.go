// INPUT: explicit current transient Execution reference and concrete user-directed stop reason.
// OUTPUT: Plan Mode validation or atomic cancellation with unmanaged fresh context.
// POS: model semantic stop boundary; it never creates a successor or rewrites a Goal.
package operation

import (
	"context"

	runtimecommand "github.com/nexus-research-lab/nexus/internal/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/runtimecommand/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func abandonExecution(svc contract.Service, sctx contract.Context) runtimecommand.Operation {
	const operationName = "abandon_execution"
	return runtimecommand.Operation{
		Name: operationName,
		Description: "Atomically cancel the explicitly referenced current transient Execution without a successor while preserving immutable submissions, acceptances and audit history. " +
			"Use only for explicit abandonment, never for a Goal-bound Execution or a route change within the same objective.",
		SearchHint:  "abandon cancel stop transient execution objective no successor",
		InputSchema: abandonExecutionSchema(),
		Annotations: &runtimecommand.OperationAnnotations{
			DestructiveHint: true,
			IdempotentHint:  true,
		},
		ContextHandler: func(
			ctx context.Context,
			input map[string]any,
			callContext *runtimecommand.CallContext,
		) (runtimecommand.Result, error) {
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
			command, commandErr := commandID(sctx, callContext, operationName, input, 0)
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

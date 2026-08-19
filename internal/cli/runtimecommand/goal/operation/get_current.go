package operation

import (
	"context"

	runtimecommand "github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand/goal/contract"
)

func getGoal(svc contract.Service, sctx contract.Context) runtimecommand.Operation {
	return readGoalOperation("get_goal", "Get the current goal for this thread, including the authoritative objective revision and completion criteria, status, budgets, token and elapsed-time usage, and remaining token budget.", svc, sctx)
}

func readGoalOperation(name string, description string, svc contract.Service, sctx contract.Context) runtimecommand.Operation {
	return runtimecommand.Operation{
		Name:        name,
		Description: description,
		SearchHint:  searchHintGetGoal,
		InputSchema: objectSchema(map[string]any{}),
		Annotations: &runtimecommand.OperationAnnotations{
			ReadOnlyHint: true,
			ReadOnly:     true,
		},
		Handler: func(ctx context.Context, input map[string]any) (runtimecommand.Result, error) {
			item, err := svc.CurrentOptional(ctx, sctx.CurrentSessionKey)
			if err != nil {
				return errorResult(err), nil
			}
			return structuredResult("current goal loaded", goalPayload(item)), nil
		},
	}
}

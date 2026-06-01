package tool

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/goal/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/runtime/mcp/sdktool"
)

func getGoal(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return readGoalTool("get_goal", "Get the current goal for this thread, including status, budgets, token and elapsed-time usage, and remaining token budget.", svc, sctx)
}

func readGoalTool(name string, description string, svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        name,
		Description: description,
		SearchHint:  searchHintGetGoal,
		AlwaysLoad:  true,
		InputSchema: objectSchema(map[string]any{}),
		Annotations: &sdktool.ToolAnnotations{
			ReadOnlyHint: true,
			ReadOnly:     true,
		},
		Handler: func(ctx context.Context, input map[string]any) (sdktool.ToolResult, error) {
			item, err := svc.CurrentOptional(ctx, sctx.CurrentSessionKey)
			if err != nil {
				return errorResult(err), nil
			}
			return structuredResult("current goal loaded", goalPayload(item)), nil
		},
	}
}

package tool

import (
	"context"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/goal/contract"
)

func getGoal(svc contract.Service, sctx contract.ServerContext) sdkmcp.Tool {
	return readGoalTool("get_goal", "Read the current thread goal, usage, and remaining token budget.", svc, sctx)
}

func readGoalTool(name string, description string, svc contract.Service, sctx contract.ServerContext) sdkmcp.Tool {
	return sdkmcp.Tool{
		Name:        name,
		Description: description,
		InputSchema: objectSchema(map[string]any{}),
		Annotations: &sdkmcp.ToolAnnotations{
			ReadOnlyHint: true,
			ReadOnly:     true,
		},
		Handler: func(ctx context.Context, input map[string]any) (sdkmcp.ToolResult, error) {
			item, err := svc.Current(ctx, sctx.CurrentSessionKey)
			if err != nil {
				return errorResult(err), nil
			}
			return structuredResult("current goal loaded", goalPayload(item)), nil
		},
	}
}

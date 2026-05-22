package tool

import (
	"context"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/goal/contract"
)

func getCurrent(svc contract.Service, sctx contract.ServerContext) sdkmcp.Tool {
	return sdkmcp.Tool{
		Name:        "get_current_goal",
		Description: "Read the active Nexus Goal bound to the current runtime session.",
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
			return structuredResult("current goal loaded", map[string]any{"goal": item}), nil
		},
	}
}

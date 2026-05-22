package tool

import (
	"context"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/goal/contract"
)

type completeInput struct {
	Summary string `json:"summary,omitempty"`
	RoundID string `json:"round_id,omitempty"`
}

func complete(svc contract.Service, sctx contract.ServerContext) sdkmcp.Tool {
	return sdkmcp.Tool{
		Name:        "mark_goal_complete",
		Description: "Mark the current Nexus Goal complete when it is genuinely finished.",
		InputSchema: objectSchema(map[string]any{
			"summary":  stringProperty("Short completion summary."),
			"round_id": stringProperty("Optional runtime round id for audit."),
		}),
		Handler: func(ctx context.Context, input map[string]any) (sdkmcp.ToolResult, error) {
			var parsed completeInput
			if err := decodeInput(input, &parsed); err != nil {
				return errorResult(err), nil
			}
			current, err := svc.Current(ctx, sctx.CurrentSessionKey)
			if err != nil {
				return errorResult(err), nil
			}
			item, err := svc.CompleteByModel(ctx, current.ID, protocol.CompleteGoalRequest{
				Summary: parsed.Summary,
				RoundID: parsed.RoundID,
			})
			if err != nil {
				return errorResult(err), nil
			}
			return structuredResult("goal marked complete", goalPayload(item)), nil
		},
	}
}

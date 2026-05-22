package tool

import (
	"context"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/goal/contract"
)

type checkpointInput struct {
	Summary string `json:"summary"`
	RoundID string `json:"round_id,omitempty"`
}

func checkpoint(svc contract.Service, sctx contract.ServerContext) sdkmcp.Tool {
	return sdkmcp.Tool{
		Name:        "record_goal_checkpoint",
		Description: "Record a concise recovery checkpoint for the current Nexus Goal after meaningful progress.",
		InputSchema: objectSchema(map[string]any{
			"summary":  stringProperty("Concise durable summary of progress, decisions, files changed, and next step."),
			"round_id": stringProperty("Optional runtime round id for audit."),
		}, "summary"),
		Handler: func(ctx context.Context, input map[string]any) (sdkmcp.ToolResult, error) {
			var parsed checkpointInput
			if err := decodeInput(input, &parsed); err != nil {
				return errorResult(err), nil
			}
			current, err := svc.Current(ctx, sctx.CurrentSessionKey)
			if err != nil {
				return errorResult(err), nil
			}
			item, err := svc.CreateCheckpointByModel(ctx, current.ID, protocol.CreateGoalCheckpointRequest{
				Summary: parsed.Summary,
				RoundID: parsed.RoundID,
			})
			if err != nil {
				return errorResult(err), nil
			}
			return structuredResult("goal checkpoint recorded", map[string]any{"checkpoint": item}), nil
		},
	}
}

package tool

import (
	"context"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/goal/contract"
)

type blockInput struct {
	Reason      string `json:"reason"`
	NeededInput string `json:"needed_input,omitempty"`
	RoundID     string `json:"round_id,omitempty"`
}

func block(svc contract.Service, sctx contract.ServerContext) sdkmcp.Tool {
	return sdkmcp.Tool{
		Name:        "mark_goal_blocked",
		Description: "Mark the current Nexus Goal blocked when progress requires user input or external state.",
		InputSchema: objectSchema(map[string]any{
			"reason":       stringProperty("Why progress is blocked."),
			"needed_input": stringProperty("Specific user input or external change needed to continue."),
			"round_id":     stringProperty("Optional runtime round id for audit."),
		}, "reason"),
		Handler: func(ctx context.Context, input map[string]any) (sdkmcp.ToolResult, error) {
			var parsed blockInput
			if err := decodeInput(input, &parsed); err != nil {
				return errorResult(err), nil
			}
			current, err := svc.Current(ctx, sctx.CurrentSessionKey)
			if err != nil {
				return errorResult(err), nil
			}
			item, err := svc.BlockByModel(ctx, current.ID, protocol.BlockGoalRequest{
				Reason:      parsed.Reason,
				NeededInput: parsed.NeededInput,
				RoundID:     parsed.RoundID,
			})
			if err != nil {
				return errorResult(err), nil
			}
			return structuredResult("goal marked blocked", map[string]any{"goal": item}), nil
		},
	}
}

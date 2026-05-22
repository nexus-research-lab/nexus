package tool

import (
	"context"
	"fmt"
	"strings"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/goal/contract"
)

type updateGoalInput struct {
	Status      string `json:"status"`
	Summary     string `json:"summary,omitempty"`
	Reason      string `json:"reason,omitempty"`
	NeededInput string `json:"needed_input,omitempty"`
	RoundID     string `json:"round_id,omitempty"`
}

func updateGoal(svc contract.Service, sctx contract.ServerContext) sdkmcp.Tool {
	return sdkmcp.Tool{
		Name:        "update_goal",
		Description: "Mark the current goal complete or blocked. Do not use for pause, resume, clear, budget, or usage limits.",
		InputSchema: objectSchema(map[string]any{
			"status":       enumStringProperty("Allowed status update.", "complete", "blocked"),
			"summary":      stringProperty("Short completion summary when status is complete."),
			"reason":       stringProperty("Why progress is blocked when status is blocked."),
			"needed_input": stringProperty("Specific user input or external change needed when blocked."),
			"round_id":     stringProperty("Optional runtime round id for audit."),
		}, "status"),
		Handler: func(ctx context.Context, input map[string]any) (sdkmcp.ToolResult, error) {
			var parsed updateGoalInput
			if err := decodeInput(input, &parsed); err != nil {
				return errorResult(err), nil
			}
			current, err := svc.Current(ctx, sctx.CurrentSessionKey)
			if err != nil {
				return errorResult(err), nil
			}
			switch strings.TrimSpace(parsed.Status) {
			case string(protocol.GoalStatusComplete):
				item, err := svc.CompleteByModel(ctx, current.ID, protocol.CompleteGoalRequest{
					Summary: parsed.Summary,
					RoundID: parsed.RoundID,
				})
				if err != nil {
					return errorResult(err), nil
				}
				return structuredResult("goal marked complete", goalPayload(item)), nil
			case string(protocol.GoalStatusBlocked):
				item, err := svc.BlockByModel(ctx, current.ID, protocol.BlockGoalRequest{
					Reason:      parsed.Reason,
					NeededInput: parsed.NeededInput,
					RoundID:     parsed.RoundID,
				})
				if err != nil {
					return errorResult(err), nil
				}
				return structuredResult("goal marked blocked", goalPayload(item)), nil
			default:
				return errorResult(fmt.Errorf("unsupported goal status %q", parsed.Status)), nil
			}
		},
	}
}

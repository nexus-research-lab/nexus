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
	Status string `json:"status"`
}

func updateGoal(svc contract.Service, sctx contract.ServerContext) sdkmcp.Tool {
	return sdkmcp.Tool{
		Name:        "update_goal",
		Description: "Update the existing goal. Use this tool only to mark the goal achieved or blocked. Set status to complete only when the objective has actually been achieved and no required work remains. Set status to blocked only when the goal cannot currently proceed until something external changes. Do not mark a goal complete merely because its budget is nearly exhausted or because you are stopping work. You cannot use this tool to pause, resume, clear, or budget-limit a goal; those status changes are controlled by the user or system. When marking a budgeted goal achieved with status complete, report the final token usage from the tool result to the user.",
		InputSchema: objectSchema(map[string]any{
			"status": enumStringProperty("Allowed status update.", "complete", "blocked"),
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
				item, err := svc.CompleteByModel(ctx, current.ID, protocol.CompleteGoalRequest{})
				if err != nil {
					return errorResult(err), nil
				}
				return structuredResult("goal marked complete", goalCompletionPayload(item)), nil
			case string(protocol.GoalStatusBlocked):
				item, err := svc.BlockByModel(ctx, current.ID, protocol.BlockGoalRequest{})
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

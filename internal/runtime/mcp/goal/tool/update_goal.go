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
		Description: "Update the existing goal. Use this tool only to mark the goal achieved or blocked. Set status to `complete` only when the objective has actually been achieved and no required work remains. Set status to `blocked` only when the goal cannot currently proceed until something external changes. Do not mark a goal complete merely because its budget is nearly exhausted or because you are stopping work. You cannot use this tool to pause, resume, or budget-limit a goal; those status changes are controlled by the user or system. When marking a budgeted goal achieved with status `complete`, report the final token usage from the tool result to the user.",
		InputSchema: objectSchema(map[string]any{
			"status": enumStringProperty("Required. Set to complete only when the objective is achieved and no required work remains. Set to blocked only when the goal cannot currently proceed without a user decision, missing dependency, or external unblock.", string(protocol.GoalStatusComplete), string(protocol.GoalStatusBlocked)),
		}, "status"),
		Handler: func(ctx context.Context, input map[string]any) (sdkmcp.ToolResult, error) {
			var parsed updateGoalInput
			if err := decodeInput(input, &parsed); err != nil {
				return errorResult(err), nil
			}
			status := protocol.GoalStatus(strings.TrimSpace(parsed.Status))
			if status != protocol.GoalStatusComplete && status != protocol.GoalStatusBlocked {
				return errorResult(fmt.Errorf("update_goal can only mark the existing goal complete or blocked; pause, resume, budget-limited, and usage-limited status changes are controlled by the user or system")), nil
			}
			current, err := svc.Current(ctx, sctx.CurrentSessionKey)
			if err != nil {
				return errorResult(err), nil
			}
			item, err := updateGoalStatus(ctx, svc, current.ID, status)
			if err != nil {
				return errorResult(err), nil
			}
			if status == protocol.GoalStatusComplete {
				return structuredResult("goal marked complete", goalCompletionPayload(item)), nil
			}
			return structuredResult("goal marked blocked", goalPayload(item)), nil
		},
	}
}

func updateGoalStatus(ctx context.Context, svc contract.Service, goalID string, status protocol.GoalStatus) (*protocol.Goal, error) {
	switch status {
	case protocol.GoalStatusComplete:
		return svc.CompleteByModel(ctx, goalID, protocol.CompleteGoalRequest{})
	case protocol.GoalStatusBlocked:
		return svc.BlockByModel(ctx, goalID, protocol.BlockGoalRequest{})
	default:
		return nil, fmt.Errorf("update_goal can only mark the existing goal complete or blocked; pause, resume, budget-limited, and usage-limited status changes are controlled by the user or system")
	}
}

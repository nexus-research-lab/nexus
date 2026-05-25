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
		Description: "Update the existing goal. Use this tool only to mark the goal achieved. Set status to `complete` only when the objective has actually been achieved and no required work remains. Do not mark a goal complete merely because its budget is nearly exhausted or because you are stopping work. You cannot use this tool to pause, resume, or budget-limit a goal; those status changes are controlled by the user or system. When marking a budgeted goal achieved with status `complete`, report the final token usage from the tool result to the user.",
		InputSchema: objectSchema(map[string]any{
			"status": enumStringProperty("Required. Set to complete only when the objective is achieved and no required work remains.", string(protocol.GoalStatusComplete)),
		}, "status"),
		Handler: func(ctx context.Context, input map[string]any) (sdkmcp.ToolResult, error) {
			var parsed updateGoalInput
			if err := decodeInput(input, &parsed); err != nil {
				return errorResult(err), nil
			}
			if strings.TrimSpace(parsed.Status) != string(protocol.GoalStatusComplete) {
				return errorResult(fmt.Errorf("update_goal can only mark the existing goal complete; pause, resume, and budget-limited status changes are controlled by the user or system")), nil
			}
			current, err := svc.Current(ctx, sctx.CurrentSessionKey)
			if err != nil {
				return errorResult(err), nil
			}
			item, err := svc.CompleteByModel(ctx, current.ID, protocol.CompleteGoalRequest{})
			if err != nil {
				return errorResult(err), nil
			}
			return structuredResult("goal marked complete", goalCompletionPayload(item)), nil
		},
	}
}

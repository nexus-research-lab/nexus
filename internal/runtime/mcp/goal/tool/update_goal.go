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
		Description: "Update the existing goal. Use this tool only to mark the goal achieved or genuinely blocked. Set status to complete only when the objective has actually been achieved and no required work remains. Set status to blocked only when the same blocking condition has repeated for at least three consecutive goal turns, counting the original/user-triggered turn and any automatic continuations, and the agent cannot make meaningful progress without user input or an external-state change. If the user resumes a goal that was previously marked blocked, treat the resumed run as a fresh blocked audit. If the same blocking condition then repeats for at least three consecutive resumed goal turns, set status to blocked again. Once the blocked threshold is satisfied, do not keep reporting that you are still blocked while leaving the goal active; set status to blocked. Do not use blocked merely because the work is hard, slow, uncertain, incomplete, or would benefit from clarification. Do not mark a goal complete merely because its budget is nearly exhausted or because you are stopping work. You cannot use this tool to pause, resume, clear, budget-limit, or usage-limit a goal; those status changes are controlled by the user or system. When marking a budgeted goal achieved with status complete, report the final token usage from the tool result to the user.",
		InputSchema: objectSchema(map[string]any{
			"status": enumStringProperty("Required. Set to complete only when the objective is achieved and no required work remains. Set to blocked only after the same blocking condition has recurred for at least three consecutive goal turns and the agent is at an impasse. After a previously blocked goal is resumed, the resumed run starts a fresh blocked audit.", "complete", "blocked"),
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

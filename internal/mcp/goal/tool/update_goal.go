// INPUT: active Goal 的 complete/blocked 状态变更与工具调用起点的 objective revision。
// OUTPUT: 经 Room lead 授权并审计后的 Goal 终态工具结果。
// POS: Goal MCP 生命周期入口；objective 纠正由 retarget_goal 负责。
package tool

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/mcp/goal/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type updateGoalInput struct {
	Status string `json:"status"`
}

const updateGoalDescription = "Update the existing goal.\n" +
	"Use this tool only to mark the goal achieved or genuinely blocked.\n" +
	"For a shared Room Goal, only the assigned lead agent may update its status; other agents must report evidence or proposals to the lead.\n" +
	"Do not use this tool to change the objective; use retarget_goal only when the user explicitly corrects the existing active goal.\n" +
	"Set status to `complete` only when the objective has actually been achieved and no required work remains.\n" +
	"Set status to `blocked` only when the same blocking condition has repeated for at least three consecutive goal turns, counting the original/user-triggered turn and any automatic continuations, and the agent cannot make meaningful progress without user input or an external-state change.\n" +
	"If the user resumes a goal that was previously marked `blocked`, treat the resumed run as a fresh blocked audit. If the same blocking condition then repeats for at least three consecutive resumed goal turns, set status to `blocked` again.\n" +
	"Once the blocked threshold is satisfied, do not keep reporting that you are still blocked while leaving the goal active; set status to `blocked`.\n" +
	"Do not use `blocked` merely because the work is hard, slow, uncertain, incomplete, or would benefit from clarification.\n" +
	"Do not mark a goal complete merely because its budget is nearly exhausted or because you are stopping work.\n" +
	"You cannot use this tool to pause, resume, budget-limit, or usage-limit a goal; those status changes are controlled by the user or system.\n" +
	"When marking a goal achieved with status `complete`, include the final token usage and elapsed time from the tool result's `completionBudgetReport` in the final response to the user."

const updateGoalStatusDescription = "Required. Set to complete only when the objective is achieved and no required work remains. Set to blocked only after the same blocker has repeated for at least three consecutive goal turns and progress is impossible without user input or external unblock."

func updateGoal(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "update_goal",
		Description: updateGoalDescription,
		SearchHint:  searchHintUpdateGoal,
		InputSchema: objectSchema(map[string]any{
			"status": enumStringProperty(updateGoalStatusDescription, string(protocol.GoalStatusComplete), string(protocol.GoalStatusBlocked)),
		}, "status"),
		Handler: func(ctx context.Context, input map[string]any) (sdktool.ToolResult, error) {
			expectedRevision := sctx.ExpectedGoalObjectiveRevision()
			var parsed updateGoalInput
			if err := decodeInput(input, &parsed); err != nil {
				return errorResult(err), nil
			}
			status := protocol.GoalStatus(strings.TrimSpace(parsed.Status))
			if status != protocol.GoalStatusComplete && status != protocol.GoalStatusBlocked {
				return errorResult(fmt.Errorf("the Goal update tool can only mark the existing goal complete or blocked; pause, resume, budget-limited, and usage-limited status changes are controlled by the user or system")), nil
			}
			current, err := svc.Current(ctx, sctx.CurrentSessionKey)
			if err != nil {
				return updateGoalCurrentErrorResult(err), nil
			}
			item, err := updateGoalStatus(ctx, svc, current.ID, status, sctx.CurrentRoundID, sctx.CurrentAgentID, expectedRevision)
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

func updateGoalCurrentErrorResult(err error) sdktool.ToolResult {
	if isGoalNotFoundError(err) {
		return errorResultText("cannot update goal because this thread has no goal")
	}
	return errorResult(err)
}

func isGoalNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "goal not found")
}

func updateGoalStatus(ctx context.Context, svc contract.Service, goalID string, status protocol.GoalStatus, roundID string, agentID string, expectedRevision int64) (*protocol.Goal, error) {
	switch status {
	case protocol.GoalStatusComplete:
		return svc.CompleteByModel(ctx, goalID, protocol.CompleteGoalRequest{RoundID: roundID, AgentID: agentID, ExpectedObjectiveRevision: expectedRevision})
	case protocol.GoalStatusBlocked:
		return svc.BlockByModel(ctx, goalID, protocol.BlockGoalRequest{RoundID: roundID, AgentID: agentID, ExpectedObjectiveRevision: expectedRevision})
	default:
		return nil, fmt.Errorf("the Goal update tool can only mark the existing goal complete or blocked; pause, resume, budget-limited, and usage-limited status changes are controlled by the user or system")
	}
}

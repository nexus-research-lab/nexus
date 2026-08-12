// INPUT: active Goal 的 complete/blocked 状态变更与工具调用起点的 objective revision。
// OUTPUT: 经 Room lead/revision 授权的 Goal 终态工具结果；complete 另受 alignment/readiness 硬门槛，blocked 的三轮规则属于模型行为约束。
// POS: Goal MCP 生命周期入口与用户成果收口边界；objective 纠正由 retarget_goal 负责。
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

const updateGoalDescription = "Update the terminal status of an existing current Goal. Never use this tool to create, set, or change a Goal objective.\n" +
	"Use it only to mark the Goal complete or blocked; when explicit Goal intent exists but get_goal returns no current Goal, create_goal is the only creation path. Objective correction of an existing Goal uses retarget_goal, and pause, resume, budget and usage states belong to the user or system.\n" +
	"Complete requires the objective to be achieved with no required work remaining. Only a Goal whose managed WorkGraph binding is confirmed also requires an aligned Objective Alignment report for the current revision and round, plus backend WorkGraph readiness; Goal-only and reserved Goals do not. Room readiness remains independently enforced.\n" +
	"Model policy permits blocked only after the same concrete blocker has persisted for at least three consecutive Goal turns with no meaningful progress possible without user input or external change; the backend authorizes identity and revision but does not infer that history from this status-only call. A shared Room Goal may be updated only by its assigned lead."

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
			if sctx.PlanMode {
				return planModeGoalMutationResult("update_goal"), nil
			}
			current, err := currentGoalForMutation(ctx, svc, sctx, expectedRevision)
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
		return errorResultText("cannot update goal because this thread has no current goal; do not retry update_goal—if the user explicitly requested a new Goal and its objective is execution-ready, use create_goal")
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

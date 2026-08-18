// INPUT: active Goal 的 complete/blocked 状态变更与工具调用起点的 objective revision。
// OUTPUT: 经 Room lead/revision 授权的 Goal 终态工具结果；complete 另受 alignment/readiness 硬门槛，blocked 的三轮规则属于模型行为约束。
// POS: Goal command 生命周期入口与用户成果收口边界；objective 纠正由 retarget_goal 负责。
package operation

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimecommand "github.com/nexus-research-lab/nexus/internal/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/runtimecommand/goal/contract"
)

type updateGoalInput struct {
	Status      string `json:"status"`
	BlockerID   string `json:"blocker_id,omitempty"`
	Reason      string `json:"reason,omitempty"`
	NeededInput string `json:"needed_input,omitempty"`
}

const updateGoalDescription = "Update the terminal status of an existing current Goal. Never use this operation to create, set, or change a Goal objective.\n" +
	"Use it only to mark the Goal complete or blocked; when explicit Goal intent exists but get_goal returns no current Goal, create_goal is the only creation path. Objective correction of an existing Goal uses retarget_goal, and pause, resume, budget and usage states belong to the user or system.\n" +
	"Complete requires the objective to be achieved with no required work remaining. Only a Goal whose managed WorkGraph binding is confirmed also requires an aligned Objective Alignment report for the current revision and round, plus backend WorkGraph readiness; Goal-only and reserved Goals do not. Room readiness remains independently enforced.\n" +
	"Blocked requires a stable blocker_id, concrete reason, and needed_input so restart recovery and the user-facing audit explain the unblock path. Reuse the same blocker_id only for the same condition. Model policy still requires that blocker to persist for at least three consecutive Goal turns; the backend preserves its exact identity but does not infer provider turns. A shared Room Goal may be updated only by its assigned lead."

const updateGoalStatusDescription = "Required. Set to complete only when the objective is achieved and no required work remains. Set to blocked only after the same blocker has repeated for at least three consecutive goal turns and progress is impossible without user input or external unblock."

func updateGoal(svc contract.Service, sctx contract.Context) runtimecommand.Operation {
	return runtimecommand.Operation{
		Name:        "update_goal",
		Description: updateGoalDescription,
		SearchHint:  searchHintUpdateGoal,
		InputSchema: objectSchema(map[string]any{
			"status":       enumStringProperty(updateGoalStatusDescription, string(protocol.GoalStatusComplete), string(protocol.GoalStatusBlocked)),
			"blocker_id":   stringProperty("Required when status=blocked. Stable identifier for this exact blocking condition; reuse it only while the same blocker persists."),
			"reason":       stringProperty("Required when status=blocked. The concrete blocker identity and why autonomous progress cannot continue."),
			"needed_input": stringProperty("Required when status=blocked. The exact user input, permission, or external state change that will unblock the Goal."),
		}, "status"),
		Handler: func(ctx context.Context, input map[string]any) (runtimecommand.Result, error) {
			expectedRevision := sctx.ExpectedGoalObjectiveRevision()
			var parsed updateGoalInput
			if err := decodeInput(input, &parsed); err != nil {
				return errorResult(err), nil
			}
			status := protocol.GoalStatus(strings.TrimSpace(parsed.Status))
			if status != protocol.GoalStatusComplete && status != protocol.GoalStatusBlocked {
				return errorResult(fmt.Errorf("the Goal update operation can only mark the existing goal complete or blocked; pause, resume, budget-limited, and usage-limited status changes are controlled by the user or system")), nil
			}
			if status == protocol.GoalStatusBlocked &&
				(strings.TrimSpace(parsed.BlockerID) == "" || strings.TrimSpace(parsed.Reason) == "" || strings.TrimSpace(parsed.NeededInput) == "") {
				return errorResult(fmt.Errorf("blocked status requires blocker_id, reason, and needed_input so the Goal has a durable recovery path")), nil
			}
			if sctx.PlanMode {
				return planModeGoalMutationResult("update_goal"), nil
			}
			current, err := currentGoalForMutation(ctx, svc, sctx, expectedRevision)
			if err != nil {
				return updateGoalCurrentErrorResult(err), nil
			}
			item, err := updateGoalStatus(ctx, svc, current.ID, status, parsed.BlockerID, parsed.Reason, parsed.NeededInput, sctx.CurrentRoundID, sctx.CurrentAgentID, expectedRevision)
			if err != nil {
				return errorResult(err), nil
			}
			if status == protocol.GoalStatusComplete {
				return appliedResult("goal marked complete", goalCompletionMutationPayload(item)), nil
			}
			return appliedResult("goal marked blocked", goalMutationPayload(item)), nil
		},
	}
}

func updateGoalCurrentErrorResult(err error) runtimecommand.Result {
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

func updateGoalStatus(ctx context.Context, svc contract.Service, goalID string, status protocol.GoalStatus, blockerID string, reason string, neededInput string, roundID string, agentID string, expectedRevision int64) (*protocol.Goal, error) {
	switch status {
	case protocol.GoalStatusComplete:
		return svc.CompleteByModel(ctx, goalID, protocol.CompleteGoalRequest{RoundID: roundID, AgentID: agentID, ExpectedObjectiveRevision: expectedRevision})
	case protocol.GoalStatusBlocked:
		return svc.BlockByModel(ctx, goalID, protocol.BlockGoalRequest{BlockerID: strings.TrimSpace(blockerID), Reason: strings.TrimSpace(reason), NeededInput: strings.TrimSpace(neededInput), RoundID: roundID, AgentID: agentID, ExpectedObjectiveRevision: expectedRevision})
	default:
		return nil, fmt.Errorf("the Goal update operation can only mark the existing goal complete or blocked; pause, resume, budget-limited, and usage-limited status changes are controlled by the user or system")
	}
}

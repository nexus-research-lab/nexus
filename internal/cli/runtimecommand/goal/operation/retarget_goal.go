// INPUT: 用户明确替换后的 objective，以及 command context 绑定的当前 session/round/objective revision。
// OUTPUT: 经 Room lead 授权后，同一 Goal 身份下的新 objective；standalone 立即激活，confirmed WorkGraph 则进入 awaiting_plan 并返回后续建图动作。
// POS: 用户明确替换当前 Goal 时的模型工具入口；无需先恢复旧目标。
package operation

import (
	"context"

	runtimecommand "github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand/goal/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type retargetGoalInput struct {
	Objective string `json:"objective"`
}

const retargetGoalDescription = "Retarget the existing current goal only when the user explicitly corrects or replaces its objective.\n" +
	"Keep the same Goal identity and accumulated usage; never complete the old Goal and create another for a correction. " +
	"For a confirmed Goal+WorkGraph binding, this reserves the successor Execution identity and enters awaiting_plan; it does not author the successor WorkGraph. Follow the returned next action to prepare and materialize that Plan. A shared Room Goal may be retargeted only by its assigned lead."

func retargetGoal(svc contract.Service, sctx contract.Context) runtimecommand.Operation {
	return runtimecommand.Operation{
		Name:        "retarget_goal",
		Description: retargetGoalDescription,
		SearchHint:  searchHintRetargetGoal,
		InputSchema: objectSchema(map[string]any{
			"objective": stringProperty("Required. The replacement objective explicitly requested by the user for the existing current goal."),
		}, "objective"),
		Handler: func(ctx context.Context, input map[string]any) (runtimecommand.Result, error) {
			var parsed retargetGoalInput
			if err := decodeInput(input, &parsed); err != nil {
				return errorResult(err), nil
			}
			if sctx.PlanMode {
				return planModeGoalMutationResult("retarget_goal"), nil
			}
			current, expectedRevision, err := currentGoalForRetarget(ctx, svc, sctx)
			if err != nil {
				if isGoalNotFoundError(err) {
					return errorResultText("cannot retarget goal because this thread has no current goal"), nil
				}
				return errorResult(err), nil
			}
			item, err := svc.RetargetByModel(ctx, sctx.CurrentSessionKey, protocol.RetargetGoalRequest{
				Objective:                 parsed.Objective,
				RoundID:                   sctx.CurrentRoundID,
				AgentID:                   sctx.CurrentAgentID,
				ExpectedGoalID:            current.ID,
				ExpectedObjectiveRevision: expectedRevision,
			})
			if err != nil {
				if isGoalNotFoundError(err) {
					return errorResultText("cannot retarget goal because this thread has no current goal"), nil
				}
				return errorResult(err), nil
			}
			sctx.StoreGoalMutationAuthority(*item)
			payload := goalMutationPayload(item)
			return appliedResult("goal retargeted", payload), nil
		},
	}
}

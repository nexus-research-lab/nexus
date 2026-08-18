// INPUT: create_goal 的 execution-ready objective/token budget 与当前 owner/agent/session/round。
// OUTPUT: 抑制与当前可见 round 竞态的隐藏续跑，原子创建 goal_only Goal，或返回指向 retarget_goal / Execution promotion 的明确提示。
// POS: Goal command 创建入口与模型侧 readiness gate；已有 active Goal 不在此处改写。
package operation

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimecommand "github.com/nexus-research-lab/nexus/internal/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/runtimecommand/goal/contract"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

type createGoalInput struct {
	Objective   string `json:"objective"`
	TokenBudget *int64 `json:"token_budget,omitempty"`
}

func createGoal(svc contract.Service, sctx contract.Context) runtimecommand.Operation {
	return runtimecommand.Operation{
		Name:        "create_goal",
		Description: createGoalDescription(sctx.CurrentSessionKey),
		SearchHint:  searchHintCreateGoal,
		InputSchema: objectSchema(map[string]any{
			"objective":    stringProperty("Required. The complete, concrete, execution-ready objective, including all confirmed material requirements. Never use a broad or placeholder objective while clarification is still needed. This starts a new active goal only when no goal is currently defined; if a goal already exists, this operation fails."),
			"token_budget": integerProperty("Optional positive token budget for the new active goal."),
		}, "objective"),
		Handler: func(ctx context.Context, input map[string]any) (runtimecommand.Result, error) {
			var parsed createGoalInput
			if err := decodeInput(input, &parsed); err != nil {
				return errorResult(err), nil
			}
			if sctx.PlanMode {
				return planModeGoalMutationResult("create_goal"), nil
			}
			item, err := svc.Create(goalsvc.WithActiveGoalContinuationSuppressed(ctx), protocol.CreateGoalRequest{
				SessionKey:  sctx.CurrentSessionKey,
				Objective:   parsed.Objective,
				TokenBudget: parsed.TokenBudget,
				CreatedBy:   "model",
				RoundID:     sctx.CurrentRoundID,
				OwnerUserID: sctx.OwnerUserID,
				AgentID:     sctx.CurrentAgentID,
				Metadata: map[string]any{
					"created_via": "goal_command",
				},
			})
			if err != nil {
				return createGoalErrorResult(err), nil
			}
			sctx.StoreGoalMutationAuthority(*item)
			return appliedResult("goal created", goalMutationPayload(item)), nil
		},
	}
}

const (
	createGoalBaseDescription = "Create one standalone active Goal only after explicit user or system Goal intent and a complete execution-ready objective. Do not create a broad placeholder while material clarification is still required. Set token_budget only from an explicit budget. This operation never creates, reserves, or binds an Execution. If a transient WorkGraph is already current, use promote_execution_to_goal with activation_reason=persistence_requested instead. When no WorkGraph exists and one is also required, wait for this operation to succeed before prepare_plan_execution with goal_binding=current; never launch them in parallel. The operation fails when a current Goal exists; explicit objective correction uses retarget_goal on that same Goal."
)

func createGoalDescription(_ string) string {
	return createGoalBaseDescription
}

const createGoalConflictMessage = "cannot create a new goal because this thread already has a goal; if the user explicitly corrected its objective, use retarget_goal on that same goal"

func createGoalErrorResult(err error) runtimecommand.Result {
	if isGoalConflictError(err) {
		return errorResultText(createGoalConflictMessage)
	}
	return errorResult(err)
}

func isGoalConflictError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "already has a goal") ||
		strings.Contains(message, "current goal already exists")
}

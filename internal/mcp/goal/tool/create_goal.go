// INPUT: create_goal 的 objective/token budget 与当前 session。
// OUTPUT: 新 Goal，或指向 retarget_goal 的冲突提示。
// POS: Goal MCP 创建入口；已有 active Goal 不在此处改写。
package tool

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/mcp/goal/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type createGoalInput struct {
	Objective   string `json:"objective"`
	TokenBudget *int64 `json:"token_budget,omitempty"`
}

func createGoal(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "create_goal",
		Description: createGoalDescription,
		SearchHint:  searchHintCreateGoal,
		InputSchema: objectSchema(map[string]any{
			"objective":    stringProperty("Required. The concrete objective to start pursuing. This starts a new active goal only when no goal is currently defined; if a goal already exists, this tool fails."),
			"token_budget": integerProperty("Optional positive token budget for the new active goal."),
		}, "objective"),
		Handler: func(ctx context.Context, input map[string]any) (sdktool.ToolResult, error) {
			var parsed createGoalInput
			if err := decodeInput(input, &parsed); err != nil {
				return errorResult(err), nil
			}
			item, err := svc.Create(ctx, protocol.CreateGoalRequest{
				SessionKey:  sctx.CurrentSessionKey,
				Objective:   parsed.Objective,
				TokenBudget: parsed.TokenBudget,
				CreatedBy:   "model",
				RoundID:     sctx.CurrentRoundID,
				Metadata: map[string]any{
					"created_via": "goal_tool",
				},
			})
			if err != nil {
				return createGoalErrorResult(err), nil
			}
			sctx.StoreGoalObjectiveRevision(item.ObjectiveRevision())
			return structuredResult("goal created", goalPayload(item)), nil
		},
	}
}

const createGoalDescription = "Create a goal only when explicitly requested by the user or system/developer instructions; do not infer goals from ordinary tasks.\n" +
	"Set token_budget only when an explicit token budget is requested. Fails if a goal exists. If the user explicitly corrects the existing active objective, retarget that same goal; use the visible Goal update tool only for status."

const createGoalConflictMessage = "cannot create a new goal because this thread already has a goal; if the user explicitly corrected its objective, use retarget_goal on that same goal"

func createGoalErrorResult(err error) sdktool.ToolResult {
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

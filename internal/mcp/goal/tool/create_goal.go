// INPUT: create_goal 的 execution-ready objective/token budget 与当前 owner/agent/session/round。
// OUTPUT: 向模型暴露创建前信息充分性契约，并创建带 durable usage scope owner 的新 Goal；仅 shared Room 描述创建 Agent 的 lead/协作责任，或返回指向 retarget_goal 的冲突提示。
// POS: Goal MCP 创建入口与模型侧 readiness gate；已有 active Goal 不在此处改写。
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
		Description: createGoalDescription(sctx.CurrentSessionKey),
		SearchHint:  searchHintCreateGoal,
		InputSchema: objectSchema(map[string]any{
			"objective":    stringProperty("Required. The complete, concrete, execution-ready objective, including all confirmed material requirements. Never use a broad or placeholder objective while clarification is still needed. This starts a new active goal only when no goal is currently defined; if a goal already exists, this tool fails."),
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
				OwnerUserID: sctx.OwnerUserID,
				AgentID:     sctx.CurrentAgentID,
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

const (
	createGoalBaseDescription = "Create a goal only when explicitly requested by the user or system/developer instructions; do not infer goals from ordinary tasks. Explicit Goal intent is necessary but not sufficient. Before calling this tool, the conversation must already contain enough information to state a concrete, execution-ready objective without material guessing. If missing details could change the deliverable, scope, audience or use, constraints, or acceptance criteria, ask the user and wait for the answer. Do not create a broad or placeholder Goal and clarify or retarget it afterward. Once ready, include the confirmed requirements in the objective.\n"
	createGoalRoomDescription = "In a shared Room, the creating agent becomes the Goal lead and is responsible for coordination and later model-side retarget, complete, or blocked updates. Before substantial execution, assess task complexity, separable work, and member fit. Prefer meaningful delegation when another member can contribute independent work or review. After delegating, do not duplicate the assigned deliverable yourself; focus on coordination, unblocking, integration, and verification. Handle the whole task directly only when it is small or atomic, or delegation adds no meaningful value.\n"
	createGoalTailDescription = "Set token_budget only when an explicit token budget is requested. Fails if a goal exists. If the user explicitly corrects the existing active objective, retarget that same goal; use the visible Goal update tool only for status."
)

func createGoalDescription(sessionKey string) string {
	description := createGoalBaseDescription
	if protocol.IsRoomSharedSessionKey(sessionKey) {
		description += createGoalRoomDescription
	}
	return description + createGoalTailDescription
}

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

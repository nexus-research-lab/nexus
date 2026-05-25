package tool

import (
	"context"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/goal/contract"
)

type createGoalInput struct {
	Objective   string `json:"objective"`
	TokenBudget *int64 `json:"token_budget,omitempty"`
}

func createGoal(svc contract.Service, sctx contract.ServerContext) sdkmcp.Tool {
	return sdkmcp.Tool{
		Name:        "create_goal",
		Description: "Create a goal only when explicitly requested by the user or system/developer instructions; do not infer goals from ordinary tasks. Set token_budget only when an explicit token budget is requested. Fails if a goal exists; use update_goal only for status.",
		InputSchema: objectSchema(map[string]any{
			"objective":    stringProperty("Required. The concrete objective to start pursuing. This starts a new active goal only when no goal is currently defined; if a goal already exists, this tool fails."),
			"token_budget": integerProperty("Optional positive token budget for the new active goal."),
		}, "objective"),
		Handler: func(ctx context.Context, input map[string]any) (sdkmcp.ToolResult, error) {
			var parsed createGoalInput
			if err := decodeInput(input, &parsed); err != nil {
				return errorResult(err), nil
			}
			item, err := svc.Create(ctx, protocol.CreateGoalRequest{
				SessionKey:  sctx.CurrentSessionKey,
				Objective:   parsed.Objective,
				TokenBudget: parsed.TokenBudget,
				CreatedBy:   "model",
				Metadata: map[string]any{
					"created_via": "goal_tool",
				},
			})
			if err != nil {
				return errorResult(err), nil
			}
			return structuredResult("goal created", goalPayload(item)), nil
		},
	}
}

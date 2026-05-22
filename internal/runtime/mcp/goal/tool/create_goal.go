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
		Description: "Create a new current goal only when the user explicitly requests a goal.",
		InputSchema: objectSchema(map[string]any{
			"objective":    stringProperty("Concrete objective to pursue."),
			"token_budget": numberProperty("Optional positive token budget. Omit unless the user explicitly requested one."),
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

// INPUT: 用户明确纠正后的 objective，以及 MCP server 绑定的当前 session/round/objective revision。
// OUTPUT: 同一 Goal 身份下更新后的 objective 与模型可读工具结果。
// POS: 用户插话纠正 active Goal 时的模型工具入口；完成/阻塞生命周期仍只走 update_goal。
package tool

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/mcp/goal/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type retargetGoalInput struct {
	Objective string `json:"objective"`
}

const retargetGoalDescription = "Retarget the existing active goal only when the user explicitly corrects or replaces its objective.\n" +
	"Keep the same goal identity and accumulated usage. Never complete the old goal and create a new one for a correction.\n" +
	"Do not infer a retarget from ordinary follow-up requests, your own judgment, or incidental scope details."

func retargetGoal(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "retarget_goal",
		Description: retargetGoalDescription,
		SearchHint:  searchHintRetargetGoal,
		InputSchema: objectSchema(map[string]any{
			"objective": stringProperty("Required. The corrected objective explicitly requested by the user for the existing active goal."),
		}, "objective"),
		Handler: func(ctx context.Context, input map[string]any) (sdktool.ToolResult, error) {
			expectedRevision := sctx.ExpectedGoalObjectiveRevision()
			var parsed retargetGoalInput
			if err := decodeInput(input, &parsed); err != nil {
				return errorResult(err), nil
			}
			item, err := svc.RetargetByModel(ctx, sctx.CurrentSessionKey, protocol.RetargetGoalRequest{
				Objective:                 parsed.Objective,
				RoundID:                   sctx.CurrentRoundID,
				AgentID:                   sctx.CurrentAgentID,
				ExpectedObjectiveRevision: expectedRevision,
			})
			if err != nil {
				if isGoalNotFoundError(err) {
					return errorResultText("cannot retarget goal because this thread has no active goal"), nil
				}
				return errorResult(err), nil
			}
			sctx.StoreGoalObjectiveRevision(item.ObjectiveRevision())
			return structuredResult("goal retargeted", goalPayload(item)), nil
		},
	}
}

// INPUT: Goal 工具结果、状态与 actual/budget usage。
// OUTPUT: 完整 MCP structured 投影、Codex 兼容 text 投影及精简完成指引。
// POS: Goal MCP 工具的稳定输出边界。
package tool

import (
	"encoding/json"
	"fmt"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func structuredResult(_ string, content map[string]any) sdktool.ToolResult {
	text := "{}"
	if payload, err := json.MarshalIndent(goalToolTextPayloadFrom(content), "", "  "); err == nil {
		text = string(payload)
	}
	return sdktool.ToolResult{
		Content: []map[string]any{{
			"type": "text",
			"text": text,
		}},
		StructuredContent: content,
	}
}

type goalToolTextPayload struct {
	Goal            any `json:"goal"`
	RemainingTokens any `json:"remainingTokens"`
	// CompletionBudgetReport 保持 Codex 兼容的模型可见完成指引。
	CompletionBudgetReport any `json:"completionBudgetReport"`
}

type goalTextValue struct {
	ThreadID        any `json:"threadId"`
	Objective       any `json:"objective"`
	Status          any `json:"status"`
	TokenBudget     any `json:"tokenBudget"`
	TokensUsed      any `json:"tokensUsed"`
	TimeUsedSeconds any `json:"timeUsedSeconds"`
	CreatedAt       any `json:"createdAt"`
	UpdatedAt       any `json:"updatedAt"`
}

func goalToolTextPayloadFrom(content map[string]any) goalToolTextPayload {
	return goalToolTextPayload{
		Goal:                   goalTextValueFromAny(content["goal"]),
		RemainingTokens:        content["remainingTokens"],
		CompletionBudgetReport: content["completionBudgetReport"],
	}
}

func goalTextValueFromAny(value any) any {
	goal, ok := value.(map[string]any)
	if !ok || goal == nil {
		return nil
	}
	return goalTextValue{
		ThreadID:        goal["threadId"],
		Objective:       goal["objective"],
		Status:          goal["status"],
		TokenBudget:     goal["tokenBudget"],
		TokensUsed:      goal["tokensUsed"],
		TimeUsedSeconds: goal["timeUsedSeconds"],
		CreatedAt:       goal["createdAt"],
		UpdatedAt:       goal["updatedAt"],
	}
}

func errorResult(err error) sdktool.ToolResult {
	text := "goal tool failed"
	if err != nil {
		text = err.Error()
	}
	return errorResultText(text)
}

func errorResultText(text string) sdktool.ToolResult {
	return sdktool.ToolResult{
		Content: []map[string]any{{
			"type": "text",
			"text": text,
		}},
		IsError: true,
	}
}

func decodeInput(input map[string]any, target any) error {
	payload, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("marshal input: %w", err)
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("decode input: %w", err)
	}
	return nil
}

func goalPayload(item *protocol.Goal) map[string]any {
	return goalPayloadWithOptions(item, goalPayloadOptions{})
}

func goalCompletionPayload(item *protocol.Goal) map[string]any {
	return goalPayloadWithOptions(item, goalPayloadOptions{completionBudgetReport: true})
}

type goalPayloadOptions struct {
	completionBudgetReport bool
}

func goalPayloadWithOptions(item *protocol.Goal, options goalPayloadOptions) map[string]any {
	payload := map[string]any{
		"goal":                            toolGoalValue(item),
		"goalId":                          nil,
		"remainingTokens":                 nil,
		"usageFinalized":                  nil,
		"completionUsageCheckpointReport": nil,
		"completionBudgetReport":          nil,
	}
	if item == nil {
		return payload
	}
	remainingTokens := item.RemainingTokens()
	payload["remainingTokens"] = int64PointerValue(remainingTokens)
	if options.completionBudgetReport {
		if report := completionUsageCheckpointReport(item); report != "" {
			if item.ID != "" {
				payload["goalId"] = item.ID
			}
			payload["usageFinalized"] = false
			payload["completionUsageCheckpointReport"] = report
			payload["completionBudgetReport"] = report
		}
	}
	return payload
}

func toolGoalValue(item *protocol.Goal) any {
	if item == nil {
		return nil
	}
	goal := map[string]any{
		"threadId":              item.SessionKey,
		"objective":             item.Objective,
		"status":                toolGoalStatus(item.Status),
		"tokenBudget":           int64PointerValue(item.TokenBudget),
		"tokensUsed":            item.Usage.BudgetTokens(),
		"budgetTokens":          item.Usage.BudgetTokens(),
		"actualTokens":          item.Usage.ActualTokens(),
		"actualTokensEstimated": item.Usage.ActualTokensAreEstimated(),
		"timeUsedSeconds":       item.TimeUsedSeconds,
		"createdAt":             item.CreatedAt.Unix(),
		"updatedAt":             item.UpdatedAt.Unix(),
	}
	return goal
}

func toolGoalStatus(status protocol.GoalStatus) string {
	switch protocol.NormalizeGoalStatus(status) {
	case protocol.GoalStatusUsageLimited:
		return "usageLimited"
	case protocol.GoalStatusBudgetLimited:
		return "budgetLimited"
	default:
		return string(protocol.NormalizeGoalStatus(status))
	}
}

func int64PointerValue(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func completionBudgetReport(item *protocol.Goal) string {
	return completionUsageCheckpointReport(item)
}

func completionUsageCheckpointReport(item *protocol.Goal) string {
	if item == nil || protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusComplete {
		return ""
	}
	return "Goal achieved. Send one concise final response stating that the Goal is complete and briefly summarizing what `goal.objective` achieved. Then stop and wait for user input."
}

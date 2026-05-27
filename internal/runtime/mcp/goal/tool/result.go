package tool

import (
	"encoding/json"
	"fmt"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func structuredResult(_ string, content map[string]any) sdkmcp.ToolResult {
	text := "{}"
	if payload, err := json.MarshalIndent(content, "", "  "); err == nil {
		text = string(payload)
	}
	return sdkmcp.ToolResult{
		Content: []map[string]any{{
			"type": "text",
			"text": text,
		}},
		StructuredContent: content,
	}
}

func errorResult(err error) sdkmcp.ToolResult {
	text := "goal tool failed"
	if err != nil {
		text = err.Error()
	}
	return sdkmcp.ToolResult{
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
		"goal":                   toolGoalValue(item),
		"remainingTokens":        nil,
		"completionBudgetReport": nil,
	}
	if item == nil {
		return payload
	}
	remainingTokens := item.RemainingTokens()
	payload["remainingTokens"] = int64PointerValue(remainingTokens)
	if options.completionBudgetReport {
		if report := completionBudgetReport(item); report != "" {
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
		"threadId":        item.SessionKey,
		"objective":       item.Objective,
		"status":          toolGoalStatus(item.Status),
		"tokensUsed":      item.Usage.Total(),
		"timeUsedSeconds": item.TimeUsedSeconds,
		"createdAt":       item.CreatedAt.Unix(),
		"updatedAt":       item.UpdatedAt.Unix(),
	}
	if item.TokenBudget != nil {
		goal["tokenBudget"] = *item.TokenBudget
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
	if item == nil || protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusComplete {
		return ""
	}
	if item.TokenBudget == nil && item.TimeUsedSeconds <= 0 {
		return ""
	}
	return "Goal achieved. Report final usage from this tool result's structured goal fields. If `goal.tokenBudget` is present, include token usage from `goal.tokensUsed` and `goal.tokenBudget`. If `goal.timeUsedSeconds` is greater than 0, summarize elapsed time in a concise, human-friendly form appropriate to the response language."
}

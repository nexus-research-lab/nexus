package tool

import (
	"encoding/json"
	"fmt"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func structuredResult(text string, content map[string]any) sdkmcp.ToolResult {
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
	payload := map[string]any{"goal": item}
	if item == nil {
		payload["remaining_tokens"] = nil
		return payload
	}
	remainingTokens := item.RemainingTokens()
	payload["remaining_tokens"] = int64PointerValue(remainingTokens)
	if item.TokenBudget != nil {
		payload["budget_report"] = map[string]any{
			"token_budget": *item.TokenBudget,
			"tokens_used":  item.Usage.Total(),
			"tokens_left":  int64PointerValue(remainingTokens),
		}
	}
	if options.completionBudgetReport {
		if report := completionBudgetReport(item); report != "" {
			payload["completion_budget_report"] = report
		}
	}
	return payload
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
	return "Goal achieved. Report final usage from this tool result's structured goal fields. If `goal.token_budget` is present, include token usage from `goal.usage.total_tokens` and `goal.token_budget`. If `goal.time_used_seconds` is greater than 0, summarize elapsed time in a concise, human-friendly form appropriate to the response language."
}

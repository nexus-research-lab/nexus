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
	return payload
}

func int64PointerValue(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

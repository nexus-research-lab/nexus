package message

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ToolResultObservation 表示 assistant 快照中一次已物化的工具结果。
type ToolResultObservation struct {
	ToolUseID string
	ToolName  string
	ErrorCode string
	IsError   bool
}

// AssistantToolResults 从 assistant 快照里提取 tool_result，并用同快照中的 tool_use 补齐工具名。
func AssistantToolResults(message protocol.Message) []ToolResultObservation {
	if protocol.MessageRole(message) != "assistant" {
		return nil
	}
	blocks := messageContentBlocks(message["content"])
	if len(blocks) == 0 {
		return nil
	}
	toolNames := make(map[string]string)
	for _, block := range blocks {
		if normalizeString(block["type"]) != "tool_use" {
			continue
		}
		toolUseID := normalizeString(block["id"])
		if toolUseID == "" {
			continue
		}
		toolNames[toolUseID] = normalizeString(block["name"])
	}
	observations := make([]ToolResultObservation, 0)
	for _, block := range blocks {
		if normalizeString(block["type"]) != "tool_result" {
			continue
		}
		toolUseID := normalizeString(block["tool_use_id"])
		if toolUseID == "" {
			continue
		}
		observations = append(observations, ToolResultObservation{
			ToolUseID: toolUseID,
			ToolName:  toolNames[toolUseID],
			ErrorCode: normalizeString(block["error_code"]),
			IsError:   boolValue(block["is_error"]),
		})
	}
	return observations
}

// AssistantHasCountedToolProgress 判断 assistant 快照里是否包含应计为 Goal 进展的工具完成。
func AssistantHasCountedToolProgress(message protocol.Message) bool {
	for _, observation := range AssistantToolResults(message) {
		if toolResultCountsForGoalProgress(observation) {
			return true
		}
	}
	return false
}

func toolResultCountsForGoalProgress(observation ToolResultObservation) bool {
	switch CanonicalToolName(observation.ToolName) {
	case "", "update_goal":
		return false
	}
	switch normalizeString(observation.ErrorCode) {
	case askUserQuestionTimeoutErrorCode, askUserQuestionChannelUnavailableCode:
		return false
	default:
		return true
	}
}

// CanonicalToolName 把 SDK/MCP 展示名规整为模型工具短名。
func CanonicalToolName(name string) string {
	name = normalizeString(name)
	if name == "" {
		return ""
	}
	if strings.HasPrefix(name, "mcp__") {
		parts := strings.Split(name, "__")
		if len(parts) >= 3 {
			return strings.TrimSpace(parts[len(parts)-1])
		}
	}
	return name
}

func messageContentBlocks(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return cloneBlockSlice(typed)
	case []any:
		blocks := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			blocks = append(blocks, cloneMap(block))
		}
		return blocks
	default:
		return nil
	}
}

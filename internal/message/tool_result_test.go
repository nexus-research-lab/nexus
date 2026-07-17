package message

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestAssistantToolResultsMapsToolNames(t *testing.T) {
	message := protocol.Message{
		"role": "assistant",
		"content": []map[string]any{
			{"type": "text", "text": "working"},
			{"type": "tool_use", "id": "tool-1", "name": "read_file"},
			{"type": "tool_result", "tool_use_id": "tool-1"},
			{"type": "tool_result", "tool_use_id": "missing", "is_error": true},
		},
	}

	results := AssistantToolResults(message)
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].ToolUseID != "tool-1" || results[0].ToolName != "read_file" || results[0].IsError {
		t.Fatalf("results[0] = %#v, want read_file success", results[0])
	}
	if results[1].ToolUseID != "missing" || results[1].ToolName != "" || !results[1].IsError {
		t.Fatalf("results[1] = %#v, want unmatched error", results[1])
	}
}

func TestAssistantToolResultsIgnoresNonAssistant(t *testing.T) {
	results := AssistantToolResults(protocol.Message{
		"role":    "user",
		"content": []any{map[string]any{"type": "tool_result", "tool_use_id": "tool-1"}},
	})
	if len(results) != 0 {
		t.Fatalf("results = %#v, want none", results)
	}
}

func TestAssistantHasCountedToolProgress(t *testing.T) {
	message := protocol.Message{
		"role": "assistant",
		"content": []map[string]any{
			{"type": "tool_use", "id": "tool-1", "name": "read_file"},
			{"type": "tool_result", "tool_use_id": "tool-1", "is_error": true},
		},
	}
	if !AssistantHasCountedToolProgress(message) {
		t.Fatal("AssistantHasCountedToolProgress() = false, want true")
	}
}

func TestAssistantHasCountedToolProgressIgnoresUpdateGoal(t *testing.T) {
	for _, toolName := range []string{"update_goal", "mcp__nexus_goal__update_goal"} {
		message := protocol.Message{
			"role": "assistant",
			"content": []map[string]any{
				{"type": "tool_use", "id": "tool-1", "name": toolName},
				{"type": "tool_result", "tool_use_id": "tool-1"},
			},
		}
		if AssistantHasCountedToolProgress(message) {
			t.Fatalf("AssistantHasCountedToolProgress() = true, want false for %s", toolName)
		}
	}
}

func TestAssistantHasCountedToolProgressCountsRetargetGoal(t *testing.T) {
	for _, toolName := range []string{"retarget_goal", "mcp__nexus_goal__retarget_goal"} {
		message := protocol.Message{
			"role": "assistant",
			"content": []map[string]any{
				{"type": "tool_use", "id": "tool-1", "name": toolName},
				{"type": "tool_result", "tool_use_id": "tool-1", "is_error": false},
			},
		}
		if !AssistantHasCountedToolProgress(message) {
			t.Fatalf("AssistantHasCountedToolProgress(%q) = false, want true", toolName)
		}
	}
}

func TestAssistantHasCountedToolProgressIgnoresFailedRetargetGoal(t *testing.T) {
	message := protocol.Message{
		"role": "assistant",
		"content": []map[string]any{
			{"type": "tool_use", "id": "tool-1", "name": "mcp__nexus_goal__retarget_goal"},
			{"type": "tool_result", "tool_use_id": "tool-1", "is_error": true},
		},
	}
	if AssistantHasCountedToolProgress(message) {
		t.Fatal("AssistantHasCountedToolProgress() = true, want false for failed retarget_goal")
	}
}

func TestAssistantHasCountedToolProgressIgnoresPermissionTimeout(t *testing.T) {
	message := protocol.Message{
		"role": "assistant",
		"content": []map[string]any{
			{"type": "tool_use", "id": "tool-1", "name": "AskUserQuestion"},
			{
				"type":        "tool_result",
				"tool_use_id": "tool-1",
				"is_error":    true,
				"error_code":  "permission_request_timeout",
			},
		},
	}
	if AssistantHasCountedToolProgress(message) {
		t.Fatal("AssistantHasCountedToolProgress() = true, want false for non-executed permission timeout")
	}
}

func TestAssistantHasCountedToolProgressIgnoresUnmatchedToolResult(t *testing.T) {
	message := protocol.Message{
		"role": "assistant",
		"content": []map[string]any{
			{"type": "tool_result", "tool_use_id": "missing", "is_error": true},
		},
	}
	if AssistantHasCountedToolProgress(message) {
		t.Fatal("AssistantHasCountedToolProgress() = true, want false without a matched tool_use")
	}
}

func TestAssistantMissedGoalCompletionTool(t *testing.T) {
	message := protocol.Message{
		"role": "assistant",
		"content": []map[string]any{
			{
				"type": "text",
				"text": "任务已经完成，但我没有看到 mcp__nexus_goal__update_goal 工具，无法调用它来标记完成。",
			},
		},
	}
	if !AssistantMissedGoalCompletionTool(message) {
		t.Fatal("AssistantMissedGoalCompletionTool() = false, want true")
	}
}

func TestAssistantMissedGoalCompletionToolRequiresCompletionClaim(t *testing.T) {
	message := protocol.Message{
		"role": "assistant",
		"content": []map[string]any{
			{
				"type": "text",
				"text": "I cannot call update_goal yet because more verification is needed.",
			},
		},
	}
	if AssistantMissedGoalCompletionTool(message) {
		t.Fatal("AssistantMissedGoalCompletionTool() = true, want false without completion claim")
	}
}

func TestAssistantMissedGoalCompletionToolDetectsFinalClaimWithoutToolMention(t *testing.T) {
	message := protocol.Message{
		"role": "assistant",
		"content": []map[string]any{
			{"type": "text", "text": "PPT 已完成并验证通过：9 页内容、298 行。"},
		},
	}
	if !AssistantMissedGoalCompletionTool(message) {
		t.Fatal("AssistantMissedGoalCompletionTool() = false, want true for final completion claim")
	}
}

func TestAssistantMissedGoalCompletionToolIgnoresStageCompletion(t *testing.T) {
	for _, text := range []string{
		"第一阶段已完成，下一步会继续进行 Goal 恢复链路检查。",
		"阶段任务已完成；还需要验证 update_goal 后是否清空当前 Goal。",
		"Phase 1 is complete; remaining work continues in the next phase.",
	} {
		message := protocol.Message{
			"role": "assistant",
			"content": []map[string]any{
				{"type": "text", "text": text},
			},
		}
		if AssistantMissedGoalCompletionTool(message) {
			t.Fatalf("AssistantMissedGoalCompletionTool() = true, want false for stage completion: %q", text)
		}
	}
}

func TestAssistantMissedGoalCompletionToolKeepsAllStagesCompleteClaim(t *testing.T) {
	for _, text := range []string{
		"所有阶段已完成并验证通过。",
		"所有阶段已完成，无需继续。",
	} {
		message := protocol.Message{
			"role": "assistant",
			"content": []map[string]any{
				{"type": "text", "text": text},
			},
		}
		if !AssistantMissedGoalCompletionTool(message) {
			t.Fatalf("AssistantMissedGoalCompletionTool() = false, want true for all stages complete claim: %q", text)
		}
	}
}

func TestAssistantMissedGoalCompletionToolIgnoresSuccessfulGoalUpdate(t *testing.T) {
	message := protocol.Message{
		"role": "assistant",
		"content": []map[string]any{
			{"type": "tool_use", "id": "tool-1", "name": "mcp__nexus_goal__update_goal"},
			{"type": "tool_result", "tool_use_id": "tool-1"},
			{"type": "text", "text": "Goal has been completed."},
		},
	}
	if AssistantMissedGoalCompletionTool(message) {
		t.Fatal("AssistantMissedGoalCompletionTool() = true, want false after successful update_goal")
	}
}

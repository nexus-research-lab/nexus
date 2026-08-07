package message

import (
	"encoding/json"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
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

func TestAssistantHasCountedToolProgressIgnoresRecoverableMalformedInput(t *testing.T) {
	message := protocol.Message{
		"role": "assistant",
		"content": []map[string]any{
			{
				"type": "tool_use",
				"id":   "tool-1",
				"name": "SendUserMessage",
			},
			{
				"type":        "tool_result",
				"tool_use_id": "tool-1",
				"is_error":    true,
				"metadata": map[string]any{
					"_nexus_internal_kind": "malformed_tool_input",
				},
			},
		},
	}
	if AssistantHasCountedToolProgress(message) {
		t.Fatal("AssistantHasCountedToolProgress() = true, want false for recoverable malformed input")
	}
}

func TestAssistantHasCountedToolProgressIgnoresRejectedMutation(t *testing.T) {
	message := protocol.Message{
		"role": "assistant",
		"content": []map[string]any{
			{"type": "tool_use", "id": "tool-plan", "name": "mcp__nexus_execution__plan_execution"},
			{
				"type": "tool_result", "tool_use_id": "tool-plan", "is_error": false,
				"content": `{"outcome":"rejected","reason_code":"invalid_input","message":"items is required"}`,
			},
		},
	}
	results := AssistantToolResults(message)
	if len(results) != 1 || results[0].MutationOutcome != protocol.MutationResultRejected {
		t.Fatalf("AssistantToolResults() = %+v", results)
	}
	if AssistantHasCountedToolProgress(message) {
		t.Fatal("rejected mutation must not satisfy the Goal continuation progress guard")
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

func TestProcessorPreservesPermissionErrorCode(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-perm-code",
		ParentID:   "round-perm-code",
	}, "")

	// 注入 tool_use
	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolUseBlock{ID: "tool-456", Name: "AskUserQuestion"},
				},
			},
		},
	})

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeUser,
		User: &sdkprotocol.UserMessage{
			Message: sdkprotocol.ConversationEnvelope{
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolResultBlock{
						ToolUseID: "tool-456",
						Content:   json.RawMessage(`"等待用户确认超时"`),
						IsError:   true,
						ErrorCode: "permission_request_timeout",
					},
				},
			},
		},
	})

	blocks, _ := output.DurableMessages[0]["content"].([]map[string]any)
	if blocks[1]["error_code"] != "permission_request_timeout" {
		t.Fatalf("error_code 未按协议保留: %+v", blocks[1])
	}
}

func TestProcessorHandlesToolResultMessage(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-tool-result",
		ParentID:   "round-tool-result",
	}, "")

	// 先注入一个 tool_use，使结果进入同一工具分段。
	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID: "assistant-tool-result-1",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolUseBlock{ID: "tool-123", Name: "AskUserQuestion"},
				},
			},
		},
	})

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeUser,
		User: &sdkprotocol.UserMessage{
			Message: sdkprotocol.ConversationEnvelope{
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolResultBlock{
						ToolUseID: "tool-123",
						Content:   json.RawMessage(`"等待用户确认超时"`),
						IsError:   true,
						ErrorCode: "permission_request_timeout",
					},
				},
			},
		},
	})

	if len(output.DurableMessages) != 1 {
		t.Fatalf("tool result 未生成 durable assistant 消息: %+v", output)
	}
	assistantMessage := output.DurableMessages[0]
	if assistantMessage["role"] != "assistant" || assistantMessage["is_complete"] != true {
		t.Fatalf("tool result 生成的 assistant 消息不正确: %+v", assistantMessage)
	}
	blocks, _ := assistantMessage["content"].([]map[string]any)
	if len(blocks) != 2 {
		t.Fatalf("tool result 未正确并入 content: %+v", blocks)
	}
	if blocks[1]["type"] != "tool_result" {
		t.Fatalf("第二块应为 tool_result: %+v", blocks[1])
	}
	if blocks[1]["error_code"] != "permission_request_timeout" {
		t.Fatalf("tool result 未正确附加 error_code: %+v", blocks[1])
	}
}

func TestProcessorPreservesParentAcrossToolResultSnapshot(t *testing.T) {
	parentToolUseID := "agent-parent-tool"
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:worker:ws:dm:test",
		AgentID:    "worker",
		RoundID:    "round-parent-tool-result",
	}, "")
	processor.Process(sdkprotocol.ReceivedMessage{
		Type:            sdkprotocol.MessageTypeAssistant,
		ParentToolUseID: &parentToolUseID,
		Assistant: &sdkprotocol.AssistantMessage{
			ParentToolUseID: &parentToolUseID,
			Message: sdkprotocol.ConversationEnvelope{
				ID: "assistant-parent-tool-result",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolUseBlock{ID: "tool-child", Name: "Read"},
				},
			},
		},
	})

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeUser,
		User: &sdkprotocol.UserMessage{Message: sdkprotocol.ConversationEnvelope{
			Content: []sdkprotocol.ContentBlock{sdkprotocol.ToolResultBlock{
				ToolUseID: "tool-child",
				Content:   json.RawMessage(`"ok"`),
			}},
		}},
	})
	if len(output.DurableMessages) != 1 {
		t.Fatalf("tool result = %+v, want one durable assistant", output)
	}
	assistant := output.DurableMessages[0]
	if assistant["parent_id"] != parentToolUseID || assistant["parent_tool_use_id"] != parentToolUseID {
		t.Fatalf("tool result snapshot parent lost: %+v", assistant)
	}
}

func TestProcessorPreservesRecoverableToolResultMarker(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-malformed-tool-input",
		ParentID:   "round-malformed-tool-input",
	}, "")
	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolUseBlock{ID: "tool-malformed", Name: "WebFetch"},
				},
			},
		},
	})

	message, err := sdkprotocol.DecodeMessage(map[string]any{
		"type": "user",
		"message": map[string]any{
			"role": "user",
			"content": []any{map[string]any{
				"type":        "tool_result",
				"tool_use_id": "tool-malformed",
				"content":     "Tool input was not valid JSON",
				"is_error":    true,
				"metadata": map[string]any{
					"_nexus_internal_kind": "malformed_tool_input",
				},
			}},
		},
	})
	if err != nil {
		t.Fatalf("DecodeMessage() error = %v", err)
	}

	output := processor.Process(message)
	if len(output.DurableMessages) != 1 {
		t.Fatalf("recoverable tool result 未生成 durable message: %+v", output)
	}
	blocks, _ := output.DurableMessages[0]["content"].([]map[string]any)
	if len(blocks) < 2 {
		t.Fatalf("durable assistant content = %+v，期望保留 tool_use 与 tool_result", blocks)
	}
	metadata, _ := blocks[1]["metadata"].(map[string]any)
	if metadata["_nexus_internal_kind"] != "malformed_tool_input" {
		t.Fatalf("recoverable tool result marker 丢失: %+v", blocks[1])
	}
	if blocks[1]["is_error"] != true {
		t.Fatalf("recoverable tool result 必须保留 is_error=true: %+v", blocks[1])
	}
}

func TestProcessorPreservesTaskListStructuredOutputFromTranscript(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-task-list",
		ParentID:   "round-task-list",
	}, "")
	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolUseBlock{ID: "tool-task-list", Name: "TaskList"},
				},
			},
		},
	})

	message, err := sdkprotocol.DecodeMessage(map[string]any{
		"type": "user",
		"message": map[string]any{
			"role": "user",
			"content": []any{map[string]any{
				"type":        "tool_result",
				"tool_use_id": "tool-task-list",
				"content":     "#1 [pending] 验证任务列表",
			}},
		},
		// Claude Code transcript 使用 camelCase，实时协议使用 snake_case。
		"toolUseResult": map[string]any{
			"tasks": []any{map[string]any{
				"id":      "1",
				"subject": "验证任务列表",
				"status":  "pending",
			}},
		},
	})
	if err != nil {
		t.Fatalf("DecodeMessage() error = %v", err)
	}

	output := processor.Process(message)
	if len(output.DurableMessages) != 1 {
		t.Fatalf("TaskList tool result 未生成 durable message: %+v", output)
	}
	blocks, _ := output.DurableMessages[0]["content"].([]map[string]any)
	if len(blocks) != 2 {
		t.Fatalf("TaskList content blocks = %+v", blocks)
	}
	structured, _ := blocks[1]["structured_output"].(map[string]any)
	tasks, _ := structured["tasks"].([]any)
	if len(tasks) != 1 {
		t.Fatalf("TaskList structured_output = %+v", structured)
	}
}

func TestProcessorAnnotatesRejectedMutationWithoutChangingTransportError(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-rejected-mutation",
		ParentID:   "round-rejected-mutation",
	}, "")
	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolUseBlock{ID: "tool-plan", Name: "mcp__nexus_execution__plan_execution"},
				},
			},
		},
	})

	message, err := sdkprotocol.DecodeMessage(map[string]any{
		"type": "user",
		"message": map[string]any{
			"role": "user",
			"content": []any{map[string]any{
				"type": "tool_result", "tool_use_id": "tool-plan", "is_error": false,
				"content": `{"outcome":"rejected","reason_code":"invalid_input","message":"items is required"}`,
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	output := processor.Process(message)
	blocks, _ := output.DurableMessages[0]["content"].([]map[string]any)
	metadata, _ := blocks[1]["metadata"].(map[string]any)
	if blocks[1]["is_error"] != false {
		t.Fatalf("transport is_error changed: %+v", blocks[1])
	}
	if metadata["_nexus_mutation_outcome"] != "rejected" ||
		metadata["_nexus_mutation_message"] != "items is required" ||
		metadata["_nexus_mutation_reason_code"] != "invalid_input" {
		t.Fatalf("mutation metadata = %+v", metadata)
	}
}

func TestProcessorDropsUnmatchedSuccessfulToolResultMessage(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-unmatched-tool-result",
		ParentID:   "round-unmatched-tool-result",
	}, "")

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeUser,
		User: &sdkprotocol.UserMessage{
			Message: sdkprotocol.ConversationEnvelope{
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolResultBlock{
						ToolUseID: "missing-tool",
						Content:   json.RawMessage(`"ok"`),
						IsError:   false,
					},
				},
			},
		},
	})

	if len(output.DurableMessages) != 0 {
		t.Fatalf("无匹配 tool_use 的成功 tool_result 不应生成 durable 消息: %+v", output.DurableMessages)
	}
}

func TestProcessorKeepsUnmatchedErrorToolResultMessage(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-unmatched-tool-error",
		ParentID:   "round-unmatched-tool-error",
	}, "")

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeUser,
		User: &sdkprotocol.UserMessage{
			Message: sdkprotocol.ConversationEnvelope{
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolResultBlock{
						ToolUseID: "missing-tool",
						Content:   json.RawMessage(`"failed"`),
						IsError:   true,
					},
				},
			},
		},
	})

	if len(output.DurableMessages) != 1 {
		t.Fatalf("无匹配 tool_use 的错误 tool_result 应保留诊断消息: %+v", output)
	}
	blocks, _ := output.DurableMessages[0]["content"].([]map[string]any)
	if len(blocks) != 1 || blocks[0]["type"] != "tool_result" || blocks[0]["is_error"] != true {
		t.Fatalf("错误 tool_result 内容不正确: %+v", blocks)
	}
}

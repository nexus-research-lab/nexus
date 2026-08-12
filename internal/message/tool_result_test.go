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
	results = AssistantToolResults(protocol.Message{
		"role":    "user",
		"content": []any{map[string]any{"type": "tool_result", "tool_use_id": "tool-1"}},
	})
	if len(results) != 0 {
		t.Fatalf("results = %#v, want none", results)
	}
}

func TestAssistantToolResultsPreservesGoalTerminalStatus(t *testing.T) {
	for _, test := range []struct {
		name    string
		content any
		want    protocol.GoalStatus
	}{
		{
			name:    "complete",
			content: `{"goal":{"status":"complete"}}`,
			want:    protocol.GoalStatusComplete,
		},
		{
			name:    "blocked",
			content: `{"goal":{"status":"blocked"}}`,
			want:    protocol.GoalStatusBlocked,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			results := AssistantToolResults(protocol.Message{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "tool_use", "id": "tool-1", "name": "update_goal"},
					{"type": "tool_result", "tool_use_id": "tool-1", "content": test.content},
				},
			})
			if len(results) != 1 || results[0].GoalStatus != test.want {
				t.Fatalf("AssistantToolResults() = %+v, want status %q", results, test.want)
			}
		})
	}
}

func TestAssistantHasCountedToolProgress(t *testing.T) {
	tests := []struct {
		name        string
		toolName    string
		isError     bool
		errorCode   string
		metadata    map[string]any
		content     string
		want        bool
		wantOutcome protocol.MutationResultOutcome
	}{
		{name: "ordinary failure", toolName: "read_file", isError: true},
		{name: "ordinary successful evidence", toolName: "read_file", want: true},
		{name: "get goal is control-plane read", toolName: "mcp__nexus_goal__get_goal"},
		{name: "get execution is control-plane read", toolName: "mcp__nexus_execution__get_execution"},
		{name: "successful alignment audit", toolName: "mcp__nexus_execution__audit_execution_alignment", want: true},
		{name: "update goal", toolName: "update_goal"},
		{name: "qualified update goal", toolName: "mcp__nexus_goal__update_goal"},
		{name: "retarget goal", toolName: "retarget_goal", want: true},
		{name: "qualified retarget goal", toolName: "mcp__nexus_goal__retarget_goal", want: true},
		{name: "failed retarget goal", toolName: "mcp__nexus_goal__retarget_goal", isError: true},
		{name: "permission timeout", toolName: "AskUserQuestion", isError: true, errorCode: "permission_request_timeout"},
		{name: "unmatched result", isError: true},
		{
			name:     "recoverable malformed input",
			toolName: "SendUserMessage",
			isError:  true,
			metadata: map[string]any{"_nexus_internal_kind": "malformed_tool_input"},
		},
		{
			name:        "rejected mutation",
			toolName:    "mcp__nexus_execution__plan_execution",
			content:     `{"outcome":"rejected","reason_code":"invalid_input","message":"items is required"}`,
			wantOutcome: protocol.MutationResultRejected,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			content := make([]map[string]any, 0, 2)
			if test.toolName != "" {
				content = append(content, map[string]any{
					"type": "tool_use", "id": "tool-1", "name": test.toolName,
				})
			}
			content = append(content, map[string]any{
				"type":        "tool_result",
				"tool_use_id": "tool-1",
				"is_error":    test.isError,
				"error_code":  test.errorCode,
				"metadata":    test.metadata,
				"content":     test.content,
			})
			message := protocol.Message{
				"role":    "assistant",
				"content": content,
			}
			if got := AssistantHasCountedToolProgress(message); got != test.want {
				t.Fatalf("AssistantHasCountedToolProgress() = %t, want %t", got, test.want)
			}
			if test.wantOutcome != "" {
				results := AssistantToolResults(message)
				if len(results) != 1 || results[0].MutationOutcome != test.wantOutcome {
					t.Fatalf("AssistantToolResults() = %+v", results)
				}
			}
		})
	}
}

func TestWorkGraphAndGoalToolProgressClassificationIsComplete(t *testing.T) {
	tests := map[string]bool{
		"mcp__nexus_execution__get_execution":             false,
		"mcp__nexus_execution__prepare_plan_execution":    true,
		"mcp__nexus_execution__plan_execution":            true,
		"mcp__nexus_execution__abandon_execution":         true,
		"mcp__nexus_execution__assign_work":               true,
		"mcp__nexus_execution__submit_work":               true,
		"mcp__nexus_execution__review_work":               true,
		"mcp__nexus_execution__block_work":                true,
		"mcp__nexus_execution__resume_work":               true,
		"mcp__nexus_execution__take_over_work":            true,
		"mcp__nexus_execution__audit_execution_alignment": true,
		"mcp__nexus_execution__promote_execution_to_goal": true,
		"mcp__nexus_goal__get_goal":                       false,
		"mcp__nexus_goal__create_goal":                    true,
		"mcp__nexus_goal__retarget_goal":                  true,
		"mcp__nexus_goal__audit_objective_alignment":      true,
		"mcp__nexus_goal__update_goal":                    false,
	}
	for toolName, want := range tests {
		t.Run(toolName, func(t *testing.T) {
			message := protocol.Message{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "tool_use", "id": "tool-1", "name": toolName},
					{"type": "tool_result", "tool_use_id": "tool-1"},
				},
			}
			if got := AssistantHasCountedToolProgress(message); got != want {
				t.Fatalf("AssistantHasCountedToolProgress(%s) = %t, want %t", toolName, got, want)
			}
		})
	}
}

func TestAssistantMissedGoalCompletionTool(t *testing.T) {
	tests := []struct {
		name             string
		text             string
		successfulUpdate bool
		want             bool
	}{
		{
			name: "missing tool after completion",
			text: "任务已经完成，但我没有看到 mcp__nexus_goal__update_goal 工具，无法调用它来标记完成。",
			want: true,
		},
		{name: "no completion claim", text: "I cannot call update_goal yet because more verification is needed."},
		{name: "final claim", text: "PPT 已完成并验证通过：9 页内容、298 行。", want: true},
		{name: "stage complete", text: "第一阶段已完成，下一步会继续进行 Goal 恢复链路检查。"},
		{name: "stage complete with update goal", text: "阶段任务已完成；还需要验证 update_goal 后是否清空当前 Goal。"},
		{name: "english stage complete", text: "Phase 1 is complete; remaining work continues in the next phase."},
		{name: "all stages verified", text: "所有阶段已完成并验证通过。", want: true},
		{name: "all stages no continuation", text: "所有阶段已完成，无需继续。", want: true},
		{name: "successful update", text: "Goal has been completed.", successfulUpdate: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			content := []map[string]any{{"type": "text", "text": test.text}}
			if test.successfulUpdate {
				content = append([]map[string]any{
					{"type": "tool_use", "id": "tool-1", "name": "mcp__nexus_goal__update_goal"},
					{"type": "tool_result", "tool_use_id": "tool-1"},
				}, content...)
			}
			message := protocol.Message{"role": "assistant", "content": content}
			if got := AssistantMissedGoalCompletionTool(message); got != test.want {
				t.Fatalf("AssistantMissedGoalCompletionTool() = %t, want %t", got, test.want)
			}
		})
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

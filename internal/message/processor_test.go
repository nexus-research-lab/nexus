// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：processor_test.go
// @Date   ：2026/04/16 23:52:00
// @Author ：leemysw
// 2026/04/16 23:52:00   Create
// =====================================================

package message

import (
	"encoding/json"
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

func TestProcessorAlignsAssistantSequenceWithPythonSemantics(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-processor",
		ParentID:   "round-processor",
	}, "")

	startOutput := processor.Process(sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeStreamEvent,
		SessionID: "sdk-session-processor",
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "message_start",
				"message": map[string]any{
					"id":    "assistant-processor-1",
					"model": "sonnet",
				},
			},
		},
	})
	if !startOutput.StreamStarted || len(startOutput.StreamEvents) != 1 {
		t.Fatalf("message_start 未建立流式段: %+v", startOutput)
	}

	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type":  "content_block_start",
				"index": 0,
				"content_block": map[string]any{
					"type":     "thinking",
					"thinking": "先分析",
				},
			},
		},
	})
	deltaOutput := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type":  "content_block_delta",
				"index": 0,
				"delta": map[string]any{
					"type":     "thinking_delta",
					"thinking": " 再收口",
				},
			},
		},
	})
	if len(deltaOutput.StreamEvents) != 1 {
		t.Fatalf("thinking delta 未输出 stream 事件: %+v", deltaOutput)
	}
	contentBlock, _ := deltaOutput.StreamEvents[0].Data["content_block"].(map[string]any)
	if contentBlock["thinking"] != "先分析 再收口" {
		t.Fatalf("thinking 增量被破坏: %+v", contentBlock)
	}

	taskProgressOutput := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeSystem,
		System: &sdkprotocol.SystemMessage{
			Subtype: "task_progress",
			TaskProgress: &sdkprotocol.TaskProgressMessage{
				TaskID:       "task-1",
				LastToolName: "SearchWeb",
				Summary:      "正在整理检索结果",
			},
		},
	})
	if len(taskProgressOutput.DurableMessages) != 1 {
		t.Fatalf("task_progress 未并入 assistant durable 消息: %+v", taskProgressOutput)
	}
	progressBlocks, _ := taskProgressOutput.DurableMessages[0]["content"].([]map[string]any)
	if len(progressBlocks) != 2 || progressBlocks[1]["type"] != "task_progress" {
		t.Fatalf("task_progress 内容块不正确: %+v", taskProgressOutput.DurableMessages[0])
	}

	terminalOutput := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "message_delta",
				"delta": map[string]any{
					"stop_reason": "end_turn",
				},
			},
		},
	})
	if len(terminalOutput.DurableMessages) != 1 || !terminalOutput.AssistantCompleted {
		t.Fatalf("message_delta 未补出 durable assistant 快照: %+v", terminalOutput)
	}
	assistantMessage := terminalOutput.DurableMessages[0]
	if assistantMessage["role"] != "assistant" || assistantMessage["stop_reason"] != "end_turn" {
		t.Fatalf("assistant 快照不正确: %+v", assistantMessage)
	}
	assistantBlocks, _ := assistantMessage["content"].([]map[string]any)
	if len(assistantBlocks) != 2 || assistantBlocks[0]["type"] != "thinking" || assistantBlocks[1]["type"] != "task_progress" {
		t.Fatalf("assistant 快照内容顺序不正确: %+v", assistantBlocks)
	}

	resultOutput := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeResult,
		UUID: "result-processor-1",
		Result: &sdkprotocol.ResultMessage{
			Subtype:    "success",
			DurationMS: 12,
			NumTurns:   1,
			Result:     "done",
		},
	})
	if resultOutput.TerminalStatus != "finished" || resultOutput.ResultSubtype != "success" {
		t.Fatalf("result 终态不正确: %+v", resultOutput)
	}
	if len(resultOutput.DurableMessages) != 1 || resultOutput.DurableMessages[0]["role"] != "result" {
		t.Fatalf("result durable 消息不正确: %+v", resultOutput.DurableMessages)
	}
}

func TestProcessorMergesSequentialAssistantSnapshots(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-merge",
		ParentID:   "round-merge",
	}, "sdk-session-merge")

	first := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID:    "assistant-merge-1",
				Model: "glm-5-turbo",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ThinkingBlock{Thinking: "先想一下"},
				},
			},
		},
	})
	if len(first.DurableMessages) != 1 {
		t.Fatalf("首次 assistant 快照未输出 durable 消息: %+v", first)
	}
	if first.DurableMessages[0]["is_complete"] != false {
		t.Fatalf("中间 assistant 快照不应提前标记完成: %+v", first.DurableMessages[0])
	}

	second := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID:         "assistant-merge-1",
				Model:      "glm-5-turbo",
				StopReason: "end_turn",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.TextBlock{Text: "最终回答"},
				},
			},
		},
	})
	if len(second.DurableMessages) != 1 {
		t.Fatalf("第二次 assistant 快照未输出 durable 消息: %+v", second)
	}
	if second.DurableMessages[0]["is_complete"] != true {
		t.Fatalf("终态 assistant 快照应标记完成: %+v", second.DurableMessages[0])
	}
	blocks, _ := second.DurableMessages[0]["content"].([]map[string]any)
	if len(blocks) != 2 {
		t.Fatalf("assistant 快照未合并 thinking 与 text: %+v", second.DurableMessages[0])
	}
	if blocks[0]["type"] != "thinking" || blocks[1]["type"] != "text" {
		t.Fatalf("assistant 内容块顺序不正确: %+v", blocks)
	}
	if blocks[0]["thinking"] != "先想一下" || blocks[1]["text"] != "最终回答" {
		t.Fatalf("assistant 内容块未正确保留: %+v", blocks)
	}
}

func TestProcessorDoesNotPersistApiRetrySystemMessage(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-api-retry",
		ParentID:   "round-api-retry",
	}, "sdk-session-api-retry")

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeSystem,
		System: &sdkprotocol.SystemMessage{
			Subtype: "api_retry",
			Data: map[string]any{
				"message": "API 正在重试",
			},
		},
	})

	if len(output.DurableMessages) != 0 {
		t.Fatalf("api_retry 不应生成 durable 消息: %+v", output.DurableMessages)
	}
	if len(output.EphemeralMessages) != 1 {
		t.Fatalf("api_retry 应生成一条 ephemeral 消息: %+v", output)
	}
	if output.EphemeralMessages[0]["message_id"] != "system_api_retry_round-api-retry" {
		t.Fatalf("api_retry 应使用稳定 message_id: %+v", output.EphemeralMessages[0])
	}
}

func TestProcessorDefersAssistantCompletionUntilStreamTerminal(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-live-terminal",
		ParentID:   "round-live-terminal",
	}, "sdk-session-live-terminal")

	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "message_start",
				"message": map[string]any{
					"id":    "assistant-live-terminal-1",
					"model": "glm-5-turbo",
				},
			},
		},
	})
	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type":  "content_block_start",
				"index": 0,
				"content_block": map[string]any{
					"type":     "thinking",
					"thinking": "先分析",
				},
			},
		},
	})

	thinkingSnapshot := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID:         "assistant-live-terminal-1",
				Model:      "glm-5-turbo",
				StopReason: "end_turn",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ThinkingBlock{Thinking: "先分析"},
				},
			},
		},
	})
	if len(thinkingSnapshot.DurableMessages) != 1 {
		t.Fatalf("thinking 快照应落一条中间 durable assistant: %+v", thinkingSnapshot)
	}
	if thinkingSnapshot.DurableMessages[0]["is_complete"] != false {
		t.Fatalf("流式中的 thinking 快照不应提前完成: %+v", thinkingSnapshot.DurableMessages[0])
	}
	if _, ok := thinkingSnapshot.DurableMessages[0]["stop_reason"]; ok {
		t.Fatalf("流式中的 thinking 快照不应暴露 stop_reason: %+v", thinkingSnapshot.DurableMessages[0])
	}

	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type":  "content_block_start",
				"index": 0,
				"content_block": map[string]any{
					"type": "text",
					"text": "最终回答",
				},
			},
		},
	})
	textSnapshot := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID:         "assistant-live-terminal-1",
				Model:      "glm-5-turbo",
				StopReason: "end_turn",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ThinkingBlock{Thinking: "先分析"},
					sdkprotocol.TextBlock{Text: "最终回答"},
				},
			},
		},
	})
	if len(textSnapshot.DurableMessages) != 1 {
		t.Fatalf("文本快照应继续落中间 durable assistant: %+v", textSnapshot)
	}
	if textSnapshot.DurableMessages[0]["is_complete"] != false {
		t.Fatalf("message_delta 之前不应把 assistant 标记完成: %+v", textSnapshot.DurableMessages[0])
	}
	if _, ok := textSnapshot.DurableMessages[0]["stop_reason"]; ok {
		t.Fatalf("message_delta 之前的文本快照不应暴露 stop_reason: %+v", textSnapshot.DurableMessages[0])
	}

	terminalOutput := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "message_delta",
				"delta": map[string]any{
					"stop_reason": "end_turn",
				},
			},
		},
	})
	if len(terminalOutput.DurableMessages) != 1 || !terminalOutput.AssistantCompleted {
		t.Fatalf("message_delta 应补出唯一终态 assistant: %+v", terminalOutput)
	}
	if terminalOutput.DurableMessages[0]["is_complete"] != true {
		t.Fatalf("终态 assistant 应标记完成: %+v", terminalOutput.DurableMessages[0])
	}
	if terminalOutput.DurableMessages[0]["stop_reason"] != "end_turn" {
		t.Fatalf("终态 assistant 应携带 stop_reason: %+v", terminalOutput.DurableMessages[0])
	}

	duplicateSnapshot := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID:         "assistant-live-terminal-1",
				Model:      "glm-5-turbo",
				StopReason: "end_turn",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ThinkingBlock{Thinking: "先分析"},
					sdkprotocol.TextBlock{Text: "最终回答"},
				},
			},
		},
	})
	if len(duplicateSnapshot.DurableMessages) != 0 {
		t.Fatalf("终态 assistant 重复快照不应重复落库: %+v", duplicateSnapshot)
	}
}

func TestProcessorUsesCumulativeStreamIndexesWhenSDKReusesRawIndex(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-stream-index",
		ParentID:   "round-stream-index",
	}, "sdk-session-stream-index")

	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "message_start",
				"message": map[string]any{
					"id":    "assistant-stream-index-1",
					"model": "glm-5-turbo",
				},
			},
		},
	})

	first := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type":  "content_block_start",
				"index": 0,
				"content_block": map[string]any{
					"type":     "thinking",
					"thinking": "先想",
				},
			},
		},
	})
	if len(first.StreamEvents) != 1 || first.StreamEvents[0].Data["index"] != 0 {
		t.Fatalf("thinking block 索引不正确: %+v", first)
	}

	second := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type":  "content_block_start",
				"index": 0,
				"content_block": map[string]any{
					"type": "text",
					"text": "最终回答",
				},
			},
		},
	})
	if len(second.StreamEvents) != 1 {
		t.Fatalf("text block 未输出 stream 事件: %+v", second)
	}
	if second.StreamEvents[0].Data["index"] != 1 {
		t.Fatalf("text block 应映射到累计索引 1，避免覆盖 thinking: %+v", second.StreamEvents[0].Data)
	}
}

func TestProcessorHandlesToolResultMessage(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-tool-result",
		ParentID:   "round-tool-result",
	}, "")

	// 先注入一个 tool_use，使 enrich 阶段能查到工具名
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
						Content:   json.RawMessage(`"Permission request timeout"`),
						IsError:   true,
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

func TestProcessorEnrichesPermissionErrorCode(t *testing.T) {
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
						Content:   json.RawMessage(`"Permission channel unavailable"`),
						IsError:   true,
					},
				},
			},
		},
	})

	blocks, _ := output.DurableMessages[0]["content"].([]map[string]any)
	if blocks[1]["error_code"] != "permission_channel_unavailable" {
		t.Fatalf("error_code 推断不正确: %+v", blocks[1])
	}
}

func TestProcessorNormalizesServerToolAliases(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-alias",
		ParentID:   "round-alias",
	}, "")

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolUseBlock{
						ID:    "tool-alias-1",
						Name:  "SearchWeb",
						Input: json.RawMessage(`{"query":"test"}`),
					},
				},
			},
		},
	})

	if len(output.DurableMessages) != 1 {
		t.Fatalf("durable 消息数量不正确: %+v", output)
	}
	blocks, _ := output.DurableMessages[0]["content"].([]map[string]any)
	if len(blocks) != 1 || blocks[0]["type"] != "tool_use" {
		t.Fatalf("server_tool_use 未被映射为 tool_use: %+v", blocks)
	}
}

func TestNormalizeContentBlockMapsServerToolAliases(t *testing.T) {
	block := normalizeContentBlock(map[string]any{
		"type": "server_tool_use",
		"id":   "t1",
		"name": "WebSearch",
	})
	if block["type"] != "tool_use" {
		t.Fatalf("server_tool_use 未映射为 tool_use: %+v", block)
	}

	block = normalizeContentBlock(map[string]any{
		"type":        "server_tool_result",
		"tool_use_id": "t1",
		"content":     "result",
		"is_error":    false,
	})
	if block["type"] != "tool_result" {
		t.Fatalf("server_tool_result 未映射为 tool_result: %+v", block)
	}
}

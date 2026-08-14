package message

import (
	"encoding/json"
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
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

func TestProcessorStreamsToolUseAndPreservesParentToolUseID(t *testing.T) {
	parentToolUseID := "agent-call-1"
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "worker",
		RoundID:    "round-tool-stream",
	}, "sdk-session-tool-stream")

	start := processor.Process(sdkprotocol.ReceivedMessage{
		Type:            sdkprotocol.MessageTypeStreamEvent,
		ParentToolUseID: &parentToolUseID,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":    "message_start",
			"message": map[string]any{"id": "assistant-tool-stream"},
		}},
	})
	if len(start.StreamEvents) != 1 || start.StreamEvents[0].Data["parent_tool_use_id"] != parentToolUseID {
		t.Fatalf("message_start parent_tool_use_id missing: %+v", start)
	}

	blockStart := processor.Process(sdkprotocol.ReceivedMessage{
		Type:            sdkprotocol.MessageTypeStreamEvent,
		ParentToolUseID: &parentToolUseID,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":  "content_block_start",
			"index": 0,
			"content_block": map[string]any{
				"type":  "tool_use",
				"id":    "tool-1",
				"name":  "Bash",
				"input": map[string]any{},
			},
		}},
	})
	if len(blockStart.StreamEvents) != 1 {
		t.Fatalf("tool_use start was suppressed: %+v", blockStart)
	}

	delta := processor.Process(sdkprotocol.ReceivedMessage{
		Type:            sdkprotocol.MessageTypeStreamEvent,
		ParentToolUseID: &parentToolUseID,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]any{
				"type":         "input_json_delta",
				"partial_json": `{"command":"pwd"}`,
			},
		}},
	})
	if len(delta.StreamEvents) != 1 {
		t.Fatalf("tool_use delta was suppressed: %+v", delta)
	}
	block, _ := delta.StreamEvents[0].Data["content_block"].(map[string]any)
	input, _ := block["input"].(map[string]any)
	if input["command"] != "pwd" {
		t.Fatalf("tool_use input delta = %+v", block)
	}
	if delta.StreamEvents[0].Data["parent_tool_use_id"] != parentToolUseID {
		t.Fatalf("tool_use delta parent = %+v", delta.StreamEvents[0].Data)
	}
}

func TestProcessorStreamsVisualizeWidgetCodeBeforeInputJSONCompletes(t *testing.T) {
	processor := NewProcessor(MessageContext{RoundID: "round-widget-stream"}, "")
	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":    "message_start",
			"message": map[string]any{"id": "assistant-widget-stream"},
		}},
	})
	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":  "content_block_start",
			"index": 0,
			"content_block": map[string]any{
				"type":  "tool_use",
				"id":    "widget-1",
				"name":  "mcp__nexus_visualize__show_widget",
				"input": map[string]any{},
			},
		}},
	})

	partial := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]any{
				"type":         "input_json_delta",
				"partial_json": `{"title":"增长曲线","widget_code":"<div class=\"chart\">\u4e`,
			},
		}},
	})
	block, _ := partial.StreamEvents[0].Data["content_block"].(map[string]any)
	input, _ := block["input"].(map[string]any)
	if input["title"] != "增长曲线" || input["widget_code"] != `<div class="chart">` {
		t.Fatalf("partial widget input = %+v", input)
	}

	complete := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]any{
				"type":         "input_json_delta",
				"partial_json": `2d</div>"}`,
			},
		}},
	})
	block, _ = complete.StreamEvents[0].Data["content_block"].(map[string]any)
	input, _ = block["input"].(map[string]any)
	if input["title"] != "增长曲线" || input["widget_code"] != `<div class="chart">中</div>` {
		t.Fatalf("complete widget input = %+v", input)
	}
}

func TestProcessorPreservesStreamParentOnFinalAssistantSnapshot(t *testing.T) {
	parentToolUseID := "agent-call-parent"
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:worker:ws:dm:test",
		AgentID:    "worker",
		RoundID:    "round-parent-snapshot",
	}, "sdk-session-parent-snapshot")

	processor.Process(sdkprotocol.ReceivedMessage{
		Type:            sdkprotocol.MessageTypeStreamEvent,
		ParentToolUseID: &parentToolUseID,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":    "message_start",
			"message": map[string]any{"id": "assistant-parent-snapshot"},
		}},
	})
	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{Message: sdkprotocol.ConversationEnvelope{
			ID:         "assistant-parent-snapshot",
			StopReason: "end_turn",
			Content:    []sdkprotocol.ContentBlock{sdkprotocol.TextBlock{Text: "完成"}},
		}},
	})
	if len(output.DurableMessages) != 1 {
		t.Fatalf("final assistant = %+v, want one durable message", output)
	}
	assistant := output.DurableMessages[0]
	if assistant["parent_id"] != parentToolUseID || assistant["parent_tool_use_id"] != parentToolUseID {
		t.Fatalf("final assistant parent lost: %+v", assistant)
	}

	// 新的历史 assistant 段没有 parent 时必须清空旧值，不能串到上一段。
	topLevel := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{Message: sdkprotocol.ConversationEnvelope{
			ID:         "assistant-top-level",
			StopReason: "end_turn",
			Content:    []sdkprotocol.ContentBlock{sdkprotocol.TextBlock{Text: "顶层回复"}},
		}},
	})
	if len(topLevel.DurableMessages) != 1 {
		t.Fatalf("top-level assistant = %+v, want one durable message", topLevel)
	}
	if topLevel.DurableMessages[0]["parent_tool_use_id"] != nil {
		t.Fatalf("stale parent leaked into top-level assistant: %+v", topLevel.DurableMessages[0])
	}
}

func TestProcessorRecoversInputJSONDeltaWithoutBlockStart(t *testing.T) {
	processor := NewProcessor(MessageContext{RoundID: "round-input-delta"}, "")
	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":    "message_start",
			"message": map[string]any{"id": "assistant-input-delta"},
		}},
	})

	delta := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]any{
				"type":         "input_json_delta",
				"partial_json": `{"command":"pwd"}`,
			},
		}},
	})
	if len(delta.StreamEvents) != 1 {
		t.Fatalf("orphan input_json_delta should remain observable: %+v", delta)
	}
	block, _ := delta.StreamEvents[0].Data["content_block"].(map[string]any)
	input, _ := block["input"].(map[string]any)
	if block["type"] != "tool_use" || input["command"] != "pwd" {
		t.Fatalf("orphan input_json_delta recovery = %+v", block)
	}

	final := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{Message: sdkprotocol.ConversationEnvelope{
			ID:         "assistant-input-delta",
			StopReason: "tool_use",
			Content: []sdkprotocol.ContentBlock{sdkprotocol.ToolUseBlock{
				ID:    "tool-input-delta",
				Name:  "Bash",
				Input: []byte(`{"command":"pwd"}`),
			}},
		}},
	})
	if len(final.DurableMessages) != 1 {
		t.Fatalf("final tool snapshot = %+v, want one durable message", final)
	}
	blocks, _ := final.DurableMessages[0]["content"].([]map[string]any)
	if len(blocks) != 1 || blocks[0]["id"] != "tool-input-delta" {
		t.Fatalf("stream placeholder was not replaced by final tool snapshot: %+v", blocks)
	}
}

func TestProcessorIgnoresDecodedEmptyAssistantEnvelope(t *testing.T) {
	processor := NewProcessor(MessageContext{RoundID: "round-nil-assistant"}, "")
	decoded, err := sdkprotocol.DecodeMessage(map[string]any{"type": "assistant"})
	if err != nil {
		t.Fatalf("DecodeMessage() error = %v", err)
	}
	output := processor.Process(decoded)
	if len(output.DurableMessages) != 0 || output.Err != nil {
		t.Fatalf("empty assistant envelope should be ignored: %+v", output)
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

func TestProcessorMergesSequentialAssistantToolUseSnapshots(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-tool-use-merge",
		ParentID:   "round-tool-use-merge",
	}, "sdk-session-tool-use-merge")

	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID:    "assistant-tool-use-merge-1",
				Model: "glm-5-turbo",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.TextBlock{Text: "看看当前权限状态："},
				},
			},
		},
	})

	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID:    "assistant-tool-use-merge-1",
				Model: "glm-5-turbo",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolUseBlock{
						ID:    "tool-connectors",
						Name:  "mcp__nexus_feishu_docx__search",
						Input: json.RawMessage(`{}`),
					},
				},
			},
		},
	})

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID:    "assistant-tool-use-merge-1",
				Model: "glm-5-turbo",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolUseBlock{
						ID:    "tool-automation",
						Name:  "mcp__nexus_automation__find_scheduled_tasks",
						Input: json.RawMessage(`{}`),
					},
				},
			},
		},
	})
	if len(output.DurableMessages) != 1 {
		t.Fatalf("第二个 tool_use 快照未输出 durable 消息: %+v", output)
	}
	blocks, _ := output.DurableMessages[0]["content"].([]map[string]any)
	if len(blocks) != 3 {
		t.Fatalf("assistant 快照未保留两个 tool_use: %+v", output.DurableMessages[0])
	}
	if blocks[1]["type"] != "tool_use" || blocks[1]["id"] != "tool-connectors" {
		t.Fatalf("第一个 tool_use 被覆盖: %+v", blocks)
	}
	if blocks[2]["type"] != "tool_use" || blocks[2]["id"] != "tool-automation" {
		t.Fatalf("第二个 tool_use 未追加: %+v", blocks)
	}
}

func TestNormalizeContentBlocksPreservesImagePayload(t *testing.T) {
	blocks := normalizeContentBlocks([]sdkprotocol.ContentBlock{
		sdkprotocol.ImageBlock{
			Data:     "ZmFrZS1pbWFnZQ==",
			MIMEType: "image/png",
		},
	})
	if len(blocks) != 1 {
		t.Fatalf("image block 数量不正确: %+v", blocks)
	}
	if blocks[0]["type"] != "image" || blocks[0]["data"] != "ZmFrZS1pbWFnZQ==" || blocks[0]["mime_type"] != "image/png" {
		t.Fatalf("image block 未保留 data/mime_type: %+v", blocks[0])
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
	if block["source_type"] != "server_tool_use" {
		t.Fatalf("server_tool_use 原始语义未保留: %+v", block)
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
	if block["source_type"] != "server_tool_result" {
		t.Fatalf("server_tool_result 原始语义未保留: %+v", block)
	}

	block = normalizeContentBlock(map[string]any{
		"type":        "web_search_tool_result",
		"tool_use_id": "t2",
		"content":     []any{},
	})
	if block["type"] != "tool_result" || block["source_type"] != "web_search_tool_result" {
		t.Fatalf("CC provider tool result 未映射并保留来源: %+v", block)
	}
}

func TestProcessorPreservesContentBlockStopLifecycle(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-block-stop",
	}, "sdk-session-block-stop")
	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":    "message_start",
			"message": map[string]any{"id": "assistant-block-stop"},
		}},
	})

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":  "content_block_stop",
			"index": 2,
		}},
	})
	if len(output.StreamEvents) != 1 {
		t.Fatalf("content_block_stop 未透传: %+v", output)
	}
	if output.StreamEvents[0].Data["type"] != "content_block_stop" ||
		output.StreamEvents[0].Data["index"] != 2 {
		t.Fatalf("content_block_stop 负载不完整: %+v", output.StreamEvents[0].Data)
	}
}

func TestProcessorSkipsEmptyAssistantSnapshot(t *testing.T) {
	processor := NewProcessor(MessageContext{RoundID: "round-empty"}, "")
	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{Message: sdkprotocol.ConversationEnvelope{
			ID:         "assistant-empty",
			StopReason: "end_turn",
			Content:    nil,
		}},
	})
	if len(output.DurableMessages) != 0 {
		t.Fatalf("empty assistant should not be emitted: %+v", output)
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

func TestProcessorDoesNotCompleteAssistantOnMessageStop(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-message-stop-not-terminal",
		ParentID:   "round-message-stop-not-terminal",
	}, "sdk-session-message-stop-not-terminal")

	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "message_start",
				"message": map[string]any{
					"id":    "assistant-message-stop-1",
					"model": "glm-5-turbo",
				},
			},
		},
	})
	processor.Process(sdkprotocol.ReceivedMessage{
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

	stopOutput := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "message_stop",
			},
		},
	})
	if len(stopOutput.StreamEvents) != 1 || stopOutput.StreamEvents[0].Data["type"] != "message_stop" {
		t.Fatalf("message_stop 应只作为 stream event 输出: %+v", stopOutput)
	}
	if len(stopOutput.DurableMessages) != 0 || stopOutput.AssistantCompleted {
		t.Fatalf("message_stop 不应补出终态 assistant: %+v", stopOutput)
	}
}

func TestProcessorMergesStreamUsageAndPublishesFinalUsageCorrection(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-stream-usage",
		ParentID:   "round-stream-usage",
	}, "sdk-session-stream-usage")

	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type": "message_start",
			"message": map[string]any{
				"id":    "assistant-stream-usage",
				"model": "glm-5.2",
				"usage": map[string]any{"input_tokens": 10, "output_tokens": 0},
			},
		}},
	})
	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":          "content_block_start",
			"index":         0,
			"content_block": map[string]any{"type": "text", "text": "完成"},
		}},
	})
	terminal := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":  "message_delta",
			"delta": map[string]any{"stop_reason": "end_turn"},
			"usage": map[string]any{"output_tokens": 4},
		}},
	})
	if len(terminal.DurableMessages) != 1 {
		t.Fatalf("terminal stream = %+v, want assistant", terminal)
	}
	usage, _ := terminal.DurableMessages[0]["usage"].(map[string]any)
	if usage["input_tokens"] != 10 || usage["output_tokens"] != 4 {
		t.Fatalf("stream usage 未合并 message_start 与 message_delta: %+v", usage)
	}

	corrected := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{Message: sdkprotocol.ConversationEnvelope{
			ID:         "assistant-stream-usage",
			Model:      "glm-5.2",
			StopReason: "end_turn",
			Usage:      map[string]any{"input_tokens": 10, "output_tokens": 5},
			Content:    []sdkprotocol.ContentBlock{sdkprotocol.TextBlock{Text: "完成"}},
		}},
	})
	if len(corrected.DurableMessages) != 1 {
		t.Fatalf("final usage correction = %+v, want updated assistant snapshot", corrected)
	}
	usage, _ = corrected.DurableMessages[0]["usage"].(map[string]any)
	if usage["output_tokens"] != 5 {
		t.Fatalf("final assistant usage 未覆盖流式估计: %+v", usage)
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

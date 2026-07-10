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

func TestProcessorMapsAgentToolProgressToTaskProgress(t *testing.T) {
	parentToolUseID := "call-agent"
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-agent-progress",
		ParentID:   "round-agent-progress",
	}, "sdk-session-agent-progress")

	processor.Process(sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeStreamEvent,
		SessionID: "sdk-session-agent-progress",
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "message_start",
				"message": map[string]any{
					"id":    "assistant-agent-progress-1",
					"model": "glm-5.2",
				},
			},
		},
	})

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeToolProgress,
		SessionID: "sdk-session-agent-progress",
		ToolProgress: &sdkprotocol.ToolProgressMessage{
			ToolUseID:       "agent-msg-child",
			ToolName:        "Agent",
			ParentToolUseID: &parentToolUseID,
			TaskID:          "agent-1",
			Additional: map[string]any{
				"data": map[string]any{
					"type":        "agent_progress",
					"agent_id":    "agent-1",
					"agent_type":  "Explore",
					"description": "检查 a11y 配置",
					"message": map[string]any{
						"type": "assistant",
						"message": map[string]any{
							"content": []any{
								map[string]any{
									"type": "tool_use",
									"name": "Bash",
								},
							},
						},
					},
				},
			},
		},
	})
	if len(output.DurableMessages) != 1 {
		t.Fatalf("agent_progress 未并入 assistant durable 消息: %+v", output)
	}
	content, _ := output.DurableMessages[0]["content"].([]map[string]any)
	if len(content) != 1 || content[0]["type"] != "task_progress" {
		t.Fatalf("agent_progress 内容块不正确: %+v", output.DurableMessages[0])
	}
	if content[0]["task_id"] != "agent-1" || content[0]["tool_use_id"] != "call-agent" {
		t.Fatalf("task_progress 任务标识不正确: %+v", content[0])
	}
	if content[0]["description"] != "检查 a11y 配置" || content[0]["last_tool_name"] != "Bash" {
		t.Fatalf("task_progress 摘要不正确: %+v", content[0])
	}
}

func TestProcessorPreservesTypedSubagentThreadMetadata(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:host:ws:dm:thread-metadata",
		AgentID:    "host",
		RoundID:    "round-thread-metadata",
		ParentID:   "round-thread-metadata",
	}, "sdk-session-thread-metadata")

	started := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeTaskStarted,
		TaskStarted: &sdkprotocol.TaskStartedMessage{
			TaskID:       "task-1",
			AgentID:      "subagent-1",
			AgentType:    "worker",
			Description:  "检查实现",
			TaskType:     "local_agent",
			OutputFile:   "/tmp/task-output",
			ParentTaskID: "parent-1",
			Prompt:       "检查实现",
			Additional:   map[string]any{"child_session_id": "child-1", "name": "实现审计"},
		},
	})
	if len(started.DurableMessages) != 1 {
		t.Fatalf("task_started durable messages = %+v", started.DurableMessages)
	}
	startedMetadata, _ := started.DurableMessages[0]["metadata"].(map[string]any)
	for key, want := range map[string]any{
		"agent_id": "subagent-1", "agent_type": "worker", "child_session_id": "child-1",
		"description": "检查实现", "task_type": "local_agent", "output_file": "/tmp/task-output",
		"parent_task_id": "parent-1", "prompt": "检查实现", "name": "实现审计",
	} {
		if got := startedMetadata[key]; got != want {
			t.Fatalf("task_started metadata[%q] = %#v, want %#v; all=%+v", key, got, want, startedMetadata)
		}
	}

	progress := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeTaskProgress,
		TaskProgress: &sdkprotocol.TaskProgressMessage{
			TaskID:       "task-1",
			AgentID:      "subagent-1",
			AgentType:    "worker",
			Description:  "正在读取",
			LastToolName: "Read",
			ParentTaskID: "parent-1",
			Summary:      "读取核心实现",
			Additional:   map[string]any{"child_session_id": "child-1", "task_type": "local_agent"},
		},
	})
	if len(progress.DurableMessages) != 1 {
		t.Fatalf("task_progress durable messages = %+v", progress.DurableMessages)
	}
	progressBlocks, _ := progress.DurableMessages[0]["content"].([]map[string]any)
	progressBlock := progressBlocks[len(progressBlocks)-1]
	for key, want := range map[string]any{
		"agent_id": "subagent-1", "agent_type": "worker", "child_session_id": "child-1",
		"description": "正在读取", "last_tool_name": "Read", "parent_task_id": "parent-1",
		"summary": "读取核心实现", "task_type": "local_agent",
	} {
		if got := progressBlock[key]; got != want {
			t.Fatalf("task_progress block[%q] = %#v, want %#v; all=%+v", key, got, want, progressBlock)
		}
	}

	notification := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeTaskNotification,
		TaskNotification: &sdkprotocol.TaskNotificationMessage{
			TaskID:         "task-1",
			AgentID:        "subagent-1",
			AgentType:      "worker",
			ParentTaskID:   "parent-1",
			Status:         "completed",
			OutputFile:     "/tmp/task-output",
			Summary:        "检查完成",
			TranscriptPath: "/tmp/child.jsonl",
			Additional:     map[string]any{"child_session_id": "child-1"},
		},
	})
	if len(notification.DurableMessages) != 1 {
		t.Fatalf("task_notification durable messages = %+v", notification.DurableMessages)
	}
	notificationMetadata, _ := notification.DurableMessages[0]["metadata"].(map[string]any)
	for key, want := range map[string]any{
		"agent_id": "subagent-1", "agent_type": "worker", "child_session_id": "child-1",
		"parent_task_id": "parent-1", "status": "completed", "output_file": "/tmp/task-output",
		"summary": "检查完成", "transcript_path": "/tmp/child.jsonl",
	} {
		if got := notificationMetadata[key]; got != want {
			t.Fatalf("task_notification metadata[%q] = %#v, want %#v; all=%+v", key, got, want, notificationMetadata)
		}
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
						Name:  "mcp__nexus_connectors__connector_list",
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
						Name:  "mcp__nexus_automation__list_scheduled_tasks",
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

package message

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

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
	if content[0]["task_type"] != "local_agent" ||
		content[0]["child_session_id"] != "agent-1" ||
		content[0]["agent_id"] != "agent-1" {
		t.Fatalf("task_progress 子线程标识不正确: %+v", content[0])
	}
}

func TestProcessorMapsAgentStructuredOutputAttachmentToTaskNotification(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:host:ws:dm:test",
		AgentID:    "host",
		RoundID:    "round-agent-attachment",
		ParentID:   "round-agent-attachment",
	}, "sdk-session-agent-attachment")

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAttachment,
		Attachment: &sdkprotocol.AttachmentMessage{
			Type: "structured_output",
			Data: map[string]any{
				"agentId":     "agent-child-1",
				"agentType":   "Explore",
				"description": "调研产品规格",
				"status":      "completed",
				"toolUseId":   "call-agent",
				"outputFile":  "/workspace/.nexus/tasks/agent-child-1.output",
			},
		},
	})
	if len(output.DurableMessages) != 1 {
		t.Fatalf("Agent structured output 未投影成 durable task notification: %+v", output)
	}
	message := output.DurableMessages[0]
	if message["role"] != "system" {
		t.Fatalf("task notification role = %#v, want system", message["role"])
	}
	metadata, _ := message["metadata"].(map[string]any)
	if metadata["subtype"] != "task_notification" ||
		metadata["task_id"] != "agent-child-1" ||
		metadata["agent_id"] != "agent-child-1" ||
		metadata["agent_type"] != "Explore" ||
		metadata["task_type"] != "local_agent" ||
		metadata["tool_use_id"] != "call-agent" ||
		metadata["status"] != "completed" {
		t.Fatalf("task notification metadata 不正确: %+v", metadata)
	}
}

func TestProcessorMapsShellProgressToThrottledEphemeralTaskProgress(t *testing.T) {
	parentToolUseID := "tool-bash-1"
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-shell-progress",
		ParentID:   "round-shell-progress",
	}, "sdk-session-shell-progress")
	processor.segment.Start("assistant-shell-progress", "glm-5.2", nil, 1)
	processor.segment.ApplyBlock(0, map[string]any{
		"type":  "tool_use",
		"id":    parentToolUseID,
		"name":  "Bash",
		"input": map[string]any{"command": "make test"},
	})

	progressMessage := func(elapsed float64) sdkprotocol.ReceivedMessage {
		return sdkprotocol.ReceivedMessage{
			Type: sdkprotocol.MessageTypeToolProgress,
			ToolProgress: &sdkprotocol.ToolProgressMessage{
				ToolUseID:          "bash-progress",
				ToolName:           "Bash",
				ParentToolUseID:    &parentToolUseID,
				ElapsedTimeSeconds: elapsed,
				Additional: map[string]any{
					"data": map[string]any{"type": "bash_progress"},
				},
			},
		}
	}

	first := processor.Process(progressMessage(2))
	if len(first.DurableMessages) != 0 || len(first.EphemeralMessages) != 1 {
		t.Fatalf("first shell progress = %+v, want one ephemeral message", first)
	}
	blocks, _ := first.EphemeralMessages[0]["content"].([]map[string]any)
	if len(blocks) != 2 || blocks[1]["type"] != "task_progress" || blocks[1]["tool_use_id"] != parentToolUseID {
		t.Fatalf("shell progress block = %+v", blocks)
	}
	if blocks[1]["description"] != "Bash 已运行 2 秒" {
		t.Fatalf("shell progress description = %#v", blocks[1]["description"])
	}

	throttled := processor.Process(progressMessage(10))
	if len(throttled.EphemeralMessages) != 0 {
		t.Fatalf("shell progress within 30s should be throttled: %+v", throttled)
	}
	afterWindow := processor.Process(progressMessage(35))
	if len(afterWindow.EphemeralMessages) != 1 {
		t.Fatalf("shell progress after 30s = %+v, want one ephemeral message", afterWindow)
	}
	ccProgress := progressMessage(65)
	ccProgress.ToolProgress.Additional = nil
	ccShape := processor.Process(ccProgress)
	if len(ccShape.EphemeralMessages) != 1 {
		t.Fatalf("CC shell progress without internal data = %+v, want one ephemeral message", ccShape)
	}

	final := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{Message: sdkprotocol.ConversationEnvelope{
			ID:         "assistant-shell-progress",
			StopReason: "tool_use",
			Content: []sdkprotocol.ContentBlock{sdkprotocol.ToolUseBlock{
				ID:   parentToolUseID,
				Name: "Bash",
			}},
		}},
	})
	if len(final.DurableMessages) != 1 {
		t.Fatalf("final assistant = %+v", final)
	}
	finalBlocks, _ := final.DurableMessages[0]["content"].([]map[string]any)
	if len(finalBlocks) != 1 || finalBlocks[0]["type"] != "tool_use" {
		t.Fatalf("ephemeral progress leaked into durable assistant: %+v", finalBlocks)
	}
}

func TestProcessorMapsToolUseSummaryToReplaceableEphemeralProgress(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey:   "agent:nexus:ws:dm:test",
		AgentID:      "nexus",
		RoundID:      "round-tool-summary",
		AgentRoundID: "agent-round-tool-summary",
	}, "sdk-session-tool-summary")

	first := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeToolUseSummary,
		ToolUseSummary: &sdkprotocol.ToolUseSummaryMessage{
			Summary:             "已经完成代码检索，正在核对消息持久化边界。",
			PrecedingToolUseIDs: []string{"tool-read", "tool-grep"},
		},
	})
	if len(first.DurableMessages) != 0 || len(first.EphemeralMessages) != 1 {
		t.Fatalf("tool use summary = %+v, want one ephemeral message", first)
	}
	message := first.EphemeralMessages[0]
	blocks, _ := message["content"].([]map[string]any)
	if len(blocks) != 1 || blocks[0]["type"] != "progress_update" {
		t.Fatalf("progress update block = %+v", blocks)
	}
	if blocks[0]["text"] != "已经完成代码检索，正在核对消息持久化边界。" {
		t.Fatalf("progress update text = %#v", blocks[0]["text"])
	}
	if message["is_complete"] != false {
		t.Fatalf("progress update 不应成为 assistant 终态: %+v", message)
	}

	second := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeToolUseSummary,
		ToolUseSummary: &sdkprotocol.ToolUseSummaryMessage{
			Summary: "测试已通过，正在整理最终结果。",
		},
	})
	if len(second.EphemeralMessages) != 1 ||
		second.EphemeralMessages[0]["message_id"] != message["message_id"] {
		t.Fatalf("同一 Agent round 的进度快照应原位替换: first=%+v second=%+v", message, second)
	}

	empty := processor.Process(sdkprotocol.ReceivedMessage{
		Type:           sdkprotocol.MessageTypeToolUseSummary,
		ToolUseSummary: &sdkprotocol.ToolUseSummaryMessage{Summary: "  \n  "},
	})
	if len(empty.EphemeralMessages) != 0 {
		t.Fatalf("空 tool use summary 不应投影: %+v", empty)
	}
}

func TestEventMapperPublishesToolUseSummaryAsEphemeralMessage(t *testing.T) {
	mapper := NewEventMapper(EventMapperOptions{Context: MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-tool-summary-event",
	}})
	result, err := mapper.Map(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeToolUseSummary,
		ToolUseSummary: &sdkprotocol.ToolUseSummaryMessage{
			Summary: "正在整理检查结果。",
		},
	})
	if err != nil {
		t.Fatalf("映射 tool use summary 失败: %v", err)
	}
	if len(result.DurableMessages) != 0 || len(result.Events) != 1 {
		t.Fatalf("tool use summary event 投影不正确: %+v", result)
	}
	event := result.Events[0]
	if event.EventType != protocol.EventTypeMessage || event.DeliveryMode != protocol.DeliveryModeEphemeral {
		t.Fatalf("tool use summary 应为 ephemeral message: %+v", event)
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

func TestSubagentTaskUsageSnapshot(t *testing.T) {
	taskID, totalTokens, ok := SubagentTaskUsageSnapshot(protocol.Message{
		"metadata": map[string]any{
			"task_id": "task-1",
			"usage":   map[string]any{"total_tokens": int64(150)},
		},
	})
	if !ok || taskID != "task-1" || totalTokens != 150 {
		t.Fatalf("snapshot = %q/%d/%v, want task-1/150/true", taskID, totalTokens, ok)
	}
}

func TestSubagentTaskUsageSnapshotsCollectsMetadataAndAssistantBlocks(t *testing.T) {
	message := protocol.Message{
		"metadata": map[string]any{
			"task_id": "task-b",
			"usage":   map[string]any{"total_tokens": int64(90)},
		},
		"content": []map[string]any{
			{
				"type":    "task_progress",
				"task_id": "task-b",
				"usage":   map[string]any{"total_tokens": 120},
			},
			{
				"type":    "task_progress",
				"task_id": "task-a",
				"usage":   map[string]any{"total_tokens": json.Number("75")},
			},
			{
				"type":    "task_progress",
				"task_id": "task-b",
				"usage":   map[string]any{"total_tokens": 110},
			},
			{
				"type":    "text",
				"task_id": "ignored",
				"usage":   map[string]any{"total_tokens": 999},
			},
		},
	}

	got := SubagentTaskUsageSnapshots(message)
	want := []SubagentTaskUsage{
		{TaskID: "task-a", TotalTokens: 75},
		{TaskID: "task-b", TotalTokens: 120},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshots = %#v, want %#v", got, want)
	}
}

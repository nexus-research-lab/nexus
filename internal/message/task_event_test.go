package message

import (
	"testing"

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

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

func TestProcessorPreservesTerminalToolProgress(t *testing.T) {
	testCases := []struct {
		name             string
		toolName         string
		toolUseID        string
		elapsedSeconds   float64
		expectedDuration int64
	}{
		{name: "Bash", toolName: "Bash", toolUseID: "tool-bash-1", elapsedSeconds: 2.35, expectedDuration: 2350},
		{name: "KillShell", toolName: "KillShell", toolUseID: "tool-kill-shell-1", elapsedSeconds: 0.42, expectedDuration: 420},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			processor := NewProcessor(MessageContext{
				SessionKey: "agent:nexus:ws:dm:test",
				AgentID:    "nexus",
				RoundID:    "round-terminal-progress",
				ParentID:   "round-terminal-progress",
			}, "sdk-session-terminal-progress")

			output := processor.Process(sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeToolProgress,
				SessionID: "sdk-session-terminal-progress",
				ToolProgress: &sdkprotocol.ToolProgressMessage{
					ToolUseID:          testCase.toolUseID,
					ToolName:           testCase.toolName,
					ElapsedTimeSeconds: testCase.elapsedSeconds,
				},
			})
			if len(output.DurableMessages) != 1 {
				t.Fatalf("%s tool_progress 未并入 assistant durable 消息: %+v", testCase.toolName, output)
			}
			content, _ := output.DurableMessages[0]["content"].([]map[string]any)
			if len(content) != 1 || content[0]["type"] != "task_progress" {
				t.Fatalf("%s tool_progress 内容块不正确: %+v", testCase.toolName, output.DurableMessages[0])
			}
			if content[0]["task_id"] != testCase.toolUseID || content[0]["tool_use_id"] != testCase.toolUseID {
				t.Fatalf("%s tool_progress 工具标识不正确: %+v", testCase.toolName, content[0])
			}
			if content[0]["last_tool_name"] != testCase.toolName {
				t.Fatalf("%s tool_progress 工具名不正确: %+v", testCase.toolName, content[0])
			}
			usage, _ := content[0]["usage"].(map[string]any)
			if usage["duration_ms"] != testCase.expectedDuration {
				t.Fatalf("%s tool_progress 时长不正确: %+v", testCase.toolName, content[0])
			}
		})
	}
}

func TestProcessorProjectsNXSTerminalOutputSnapshots(t *testing.T) {
	parentToolUseID := "tool-bash-1"
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-nxs-terminal-progress",
		ParentID:   "round-nxs-terminal-progress",
	}, "sdk-session-nxs-terminal-progress")

	processProgress := func(progressID string, text string, lines int) map[string]any {
		output := processor.Process(sdkprotocol.ReceivedMessage{
			Type:      sdkprotocol.MessageTypeToolProgress,
			SessionID: "sdk-session-nxs-terminal-progress",
			ToolProgress: &sdkprotocol.ToolProgressMessage{
				ToolUseID:          progressID,
				ToolName:           "Bash",
				ParentToolUseID:    &parentToolUseID,
				ElapsedTimeSeconds: float64(lines),
				Additional: map[string]any{
					"data": map[string]any{
						"type":        "bash_progress",
						"output":      text,
						"full_output": text,
						"total_lines": float64(lines),
						"total_bytes": float64(len(text)),
						"timeout_ms":  float64(120000),
					},
				},
			},
		})
		if len(output.DurableMessages) != 1 {
			t.Fatalf("nxs terminal progress 未并入 assistant durable 消息: %+v", output)
		}
		content, _ := output.DurableMessages[0]["content"].([]map[string]any)
		if len(content) != 1 {
			t.Fatalf("同一 Bash 命令应只保留一个进度块: %+v", content)
		}
		return content[0]
	}

	first := processProgress("bash-progress-0", "first\n", 1)
	if first["task_id"] != parentToolUseID || first["tool_use_id"] != parentToolUseID {
		t.Fatalf("nxs progress 未关联到父 Bash 工具: %+v", first)
	}
	second := processProgress("bash-progress-1", "first\nsecond\n", 2)
	terminalOutput, _ := second["terminal_output"].(map[string]any)
	if terminalOutput["kind"] != "snapshot" || terminalOutput["stream"] != "combined" {
		t.Fatalf("terminal_output 契约不正确: %+v", terminalOutput)
	}
	if terminalOutput["text"] != "first\nsecond\n" || terminalOutput["total_lines"] != 2 {
		t.Fatalf("terminal_output 未保留最新累计快照: %+v", terminalOutput)
	}
}

func TestProcessorKeepsClaudeTerminalProgressOutputless(t *testing.T) {
	parentToolUseID := "tool-bash-claude"
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:claude:ws:dm:test",
		AgentID:    "claude",
		RoundID:    "round-claude-terminal-progress",
		ParentID:   "round-claude-terminal-progress",
	}, "sdk-session-claude-terminal-progress")

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeToolProgress,
		SessionID: "sdk-session-claude-terminal-progress",
		ToolProgress: &sdkprotocol.ToolProgressMessage{
			ToolUseID:          "tool-progress-claude",
			ToolName:           "Bash",
			ParentToolUseID:    &parentToolUseID,
			ElapsedTimeSeconds: 1.25,
		},
	})
	if len(output.DurableMessages) != 1 {
		t.Fatalf("claude terminal progress 未并入 assistant durable 消息: %+v", output)
	}
	content, _ := output.DurableMessages[0]["content"].([]map[string]any)
	if len(content) != 1 || content[0]["tool_use_id"] != parentToolUseID {
		t.Fatalf("claude progress 未关联到父 Bash 工具: %+v", content)
	}
	if _, exists := content[0]["terminal_output"]; exists {
		t.Fatalf("claude progress 不应伪造终端输出: %+v", content[0])
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

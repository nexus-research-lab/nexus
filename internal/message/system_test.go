package message

import (
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestProcessorProjectsSystemTasksFromTypedPayload(t *testing.T) {
	systemTaskID := func(message map[string]any) string {
		metadata := mapValue(message["metadata"])
		return normalizeString(metadata["task_id"])
	}
	assistantTaskID := func(message map[string]any) string {
		blocks, _ := message["content"].([]map[string]any)
		if len(blocks) == 0 {
			return ""
		}
		return normalizeString(blocks[len(blocks)-1]["task_id"])
	}
	tests := []struct {
		name       string
		system     *sdkprotocol.SystemMessage
		wantRole   string
		taskIDFrom func(map[string]any) string
	}{
		{
			name: "started",
			system: &sdkprotocol.SystemMessage{
				Subtype:     "task_started",
				Data:        map[string]any{"task_id": "raw-task"},
				TaskStarted: &sdkprotocol.TaskStartedMessage{TaskID: "typed-task", Description: "开始执行"},
			},
			wantRole:   "system",
			taskIDFrom: systemTaskID,
		},
		{
			name: "progress",
			system: &sdkprotocol.SystemMessage{
				Subtype:      "task_progress",
				Data:         map[string]any{"task_id": "raw-task"},
				TaskProgress: &sdkprotocol.TaskProgressMessage{TaskID: "typed-task", Summary: "正在执行"},
			},
			wantRole:   "assistant",
			taskIDFrom: assistantTaskID,
		},
		{
			name: "notification",
			system: &sdkprotocol.SystemMessage{
				Subtype:          "task_notification",
				Data:             map[string]any{"task_id": "raw-task"},
				TaskNotification: &sdkprotocol.TaskNotificationMessage{TaskID: "typed-task", Status: "completed"},
			},
			wantRole:   "system",
			taskIDFrom: systemTaskID,
		},
		{
			name: "updated",
			system: &sdkprotocol.SystemMessage{
				Subtype: "task_updated",
				Data: map[string]any{
					"task_id": "raw-task",
					"patch":   map[string]any{"status": "failed"},
				},
				TaskUpdated: &sdkprotocol.TaskUpdatedMessage{
					TaskID: "typed-task",
					Status: "completed",
					Patch:  sdkprotocol.TaskUpdatedPatch{Status: "completed"},
				},
			},
			wantRole:   "system",
			taskIDFrom: systemTaskID,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			processor := NewProcessor(MessageContext{
				SessionKey: "agent:nexus:ws:dm:test",
				AgentID:    "nexus",
				RoundID:    "round-system-task-" + testCase.name,
			}, "sdk-session-task")
			output := processor.Process(sdkprotocol.ReceivedMessage{
				Type:   sdkprotocol.MessageTypeSystem,
				System: testCase.system,
			})
			if len(output.DurableMessages) != 1 || len(output.EphemeralMessages) != 0 {
				t.Fatalf("output = %#v, want one durable task message", output)
			}
			message := output.DurableMessages[0]
			if message["role"] != testCase.wantRole {
				t.Fatalf("role = %#v, want %q", message["role"], testCase.wantRole)
			}
			if taskID := testCase.taskIDFrom(message); taskID != "typed-task" {
				t.Fatalf("task_id = %q, want typed-task; message=%+v", taskID, message)
			}
		})
	}
}

func TestProcessorPreservesMemorySavedSystemEvent(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-memory",
	}, "sdk-session-memory")
	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeSystem,
		System: &sdkprotocol.SystemMessage{
			Subtype: "memory_saved",
			MemorySaved: &sdkprotocol.MemorySavedMessage{
				Verb:         "Saved",
				WrittenPaths: []string{"/memory/user.md"},
				Additional: map[string]any{
					"subtype":       "memory_saved",
					"verb":          "Saved",
					"written_paths": []any{"/memory/user.md"},
				},
			},
		},
	})
	if len(output.DurableMessages) != 1 || len(output.EphemeralMessages) != 0 {
		t.Fatalf("output = %#v, want one durable memory event", output)
	}
	metadata, _ := output.DurableMessages[0]["metadata"].(map[string]any)
	paths, ok := metadata["written_paths"].([]string)
	if output.DurableMessages[0]["content"] != "长期记忆已保存" || !ok || len(paths) != 1 || paths[0] != "/memory/user.md" {
		t.Fatalf("memory event = %#v", output.DurableMessages[0])
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

func TestProcessorProjectsCanonicalAPIRetryMessage(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-api-retry-canonical",
		ParentID:   "round-api-retry-canonical",
	}, "sdk-session-api-retry-canonical")

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type:    sdkprotocol.MessageTypeSystem,
		Subtype: "api_retry",
		System: &sdkprotocol.SystemMessage{
			Subtype: "api_retry",
			Data: map[string]any{
				"attempt":        4,
				"max_retries":    11,
				"retry_delay_ms": 3000,
				"error_status":   529,
				"error":          "rate_limit",
			},
		},
	})

	if len(output.DurableMessages) != 0 || len(output.EphemeralMessages) != 1 {
		t.Fatalf("api_retry 应只生成 ephemeral 消息: %+v", output)
	}
	message := output.EphemeralMessages[0]
	if message["content"] != "模型请求暂时受限，正在自动重试。" {
		t.Fatalf("content = %#v", message["content"])
	}
	metadata, ok := message["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("metadata 类型不正确: %#v", message["metadata"])
	}
	for key, want := range map[string]any{
		"subtype":        "api_retry",
		"attempt":        4,
		"max_retries":    11,
		"retry_delay_ms": 3000,
		"error_status":   529,
		"error":          "rate_limit",
	} {
		if got := metadata[key]; got != want {
			t.Fatalf("%s = %#v, want %#v", key, got, want)
		}
	}
}

func TestProcessorPersistsCompactBoundarySystemMessage(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-compact",
		ParentID:   "round-compact",
	}, "sdk-session-compact")

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type:    sdkprotocol.MessageTypeSystem,
		Subtype: "compact_boundary",
		System: &sdkprotocol.SystemMessage{
			Subtype: "compact_boundary",
			Data: map[string]any{
				"compact_metadata": map[string]any{
					"trigger":    "auto",
					"pre_tokens": 120000,
				},
			},
		},
	})

	if len(output.EphemeralMessages) != 0 || len(output.DurableMessages) != 1 {
		t.Fatalf("compact_boundary 应生成 durable 消息: %+v", output)
	}
	message := output.DurableMessages[0]
	if message["message_id"] != "system_compact_boundary_round-compact" {
		t.Fatalf("message_id 不正确: %+v", message)
	}
	if message["content"] != "上下文已压缩" {
		t.Fatalf("content = %#v", message["content"])
	}
	metadata, ok := message["metadata"].(map[string]any)
	if !ok || metadata["subtype"] != "compact_boundary" {
		t.Fatalf("metadata 不正确: %+v", message["metadata"])
	}
	compactMetadata, ok := metadata["compact_metadata"].(map[string]any)
	if !ok || compactMetadata["trigger"] != "auto" || compactMetadata["pre_tokens"] != 120000 {
		t.Fatalf("compact_metadata 未保留: %+v", metadata)
	}
}

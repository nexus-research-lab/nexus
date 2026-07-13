package message

import (
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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

func TestProcessorNormalizesSystemAPIErrorMessage(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-api-error",
		ParentID:   "round-api-error",
	}, "sdk-session-api-error")

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type:    sdkprotocol.MessageTypeSystem,
		Subtype: "api_error",
		System: &sdkprotocol.SystemMessage{
			Subtype: "api_error",
			Data: map[string]any{
				"retryAttempt": 4,
				"maxRetries":   11,
				"retryInMs":    3000,
				"error": map[string]any{
					"status": 529,
					"type":   "overloaded_error",
				},
			},
		},
	})

	if len(output.DurableMessages) != 0 || len(output.EphemeralMessages) != 1 {
		t.Fatalf("api_error 应只生成 ephemeral 消息: %+v", output)
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

func TestEventMapperProjectsCompactRuntimeStatusLifecycle(t *testing.T) {
	mapper := NewEventMapper(EventMapperOptions{
		Context: MessageContext{
			SessionKey: "agent:nexus:ws:dm:test",
			AgentID:    "nexus",
			RoundID:    "round-compact",
		},
	})

	statuses := []string{"compacting", ""}
	for index, status := range statuses {
		result, err := mapper.Map(sdkprotocol.ReceivedMessage{
			Type:    sdkprotocol.MessageTypeSystem,
			Subtype: "status",
			System: &sdkprotocol.SystemMessage{
				Subtype: "status",
				Status:  &sdkprotocol.StatusSystemMessage{Status: status},
			},
		})
		if err != nil {
			t.Fatalf("Map(status=%q) error = %v", status, err)
		}
		if len(result.Events) != 1 || result.Events[0].EventType != protocol.EventTypeRuntimeStatus {
			t.Fatalf("status[%d] events = %+v", index, result.Events)
		}
		got := result.Events[0].Data["status"]
		if status == "" {
			if got != nil {
				t.Fatalf("结束状态 = %#v, want nil", got)
			}
			continue
		}
		if got != protocol.RuntimeStatusCompacting {
			t.Fatalf("压缩状态 = %#v, want compacting", got)
		}
	}
}

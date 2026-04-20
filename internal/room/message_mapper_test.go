// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：message_mapper_test.go
// @Date   ：2026/04/16 19:03:00
// @Author ：leemysw
// 2026/04/16 19:03:00   Create
// =====================================================

package room

import (
	"encoding/json"
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestSlotMessageMapperUsesAssistantMessageIDForStream(t *testing.T) {
	mapper := newSlotMessageMapper(
		"room:conversation:shared:test",
		"room-1",
		"conversation-1",
		"agent-1",
		"slot-1",
		"round-room-1:agent-1",
	)

	events, _, _ := mapper.Map(sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeStreamEvent,
		SessionID: "sdk-room-1",
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "message_start",
				"message": map[string]any{
					"id":    "assistant-room-1",
					"model": "sonnet",
				},
			},
		},
	})
	if len(events) != 1 {
		t.Fatalf("Room message_start 事件数量不正确: %+v", events)
	}
	if events[0].EventType != protocol.EventTypeStream {
		t.Fatalf("Room message_start 应为 stream 事件: %+v", events[0])
	}
	if events[0].MessageID != "assistant-room-1" {
		t.Fatalf("Room stream 应使用 assistant message_id，而不是 slot msg_id: %+v", events[0])
	}
	if events[0].Data["message_id"] != "assistant-room-1" {
		t.Fatalf("Room stream payload message_id 不正确: %+v", events[0].Data)
	}

	events, _, _ = mapper.Map(sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeStreamEvent,
		SessionID: "sdk-room-1",
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type":  "content_block_start",
				"index": 0,
				"content_block": map[string]any{
					"type":     "thinking",
					"thinking": "先推理",
				},
			},
		},
	})
	if len(events) != 1 || events[0].EventType != protocol.EventTypeStream {
		t.Fatalf("Room content_block_start 事件不正确: %+v", events)
	}

	events, _, _ = mapper.Map(sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeStreamEvent,
		SessionID: "sdk-room-1",
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type":  "content_block_delta",
				"index": 0,
				"delta": map[string]any{
					"type":     "thinking_delta",
					"thinking": " 再归纳",
				},
			},
		},
	})
	if len(events) != 1 {
		t.Fatalf("Room content_block_delta 事件数量不正确: %+v", events)
	}
	contentBlock, _ := events[0].Data["content_block"].(map[string]any)
	if contentBlock["thinking"] != "先推理 再归纳" {
		t.Fatalf("Room thinking delta 没有累计: %+v", contentBlock)
	}

	events, _, _ = mapper.Map(sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeStreamEvent,
		SessionID: "sdk-room-1",
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "message_delta",
				"delta": map[string]any{
					"stop_reason": "end_turn",
				},
			},
		},
	})
	if len(events) != 2 || events[0].Data["type"] != "message_delta" {
		t.Fatalf("Room message_delta 事件不正确: %+v", events)
	}
	if events[1].EventType != protocol.EventTypeMessage || events[1].Data["role"] != "assistant" {
		t.Fatalf("Room message_delta 应补出 durable assistant 快照: %+v", events)
	}

	events, _, _ = mapper.Map(sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeStreamEvent,
		SessionID: "sdk-room-1",
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "message_stop",
			},
		},
	})
	if len(events) != 1 || events[0].Data["type"] != "message_stop" {
		t.Fatalf("Room message_stop 事件不正确: %+v", events)
	}
	stopMessage, _ := events[0].Data["message"].(map[string]any)
	if stopMessage["stop_reason"] != "end_turn" {
		t.Fatalf("Room message_stop 未带 stop_reason: %+v", events[0].Data)
	}
}

func TestSlotMessageMapperMapsToolResultMessage(t *testing.T) {
	mapper := newSlotMessageMapper(
		"room:conversation:shared:test",
		"room-1",
		"conversation-1",
		"agent-1",
		"slot-1",
		"round-room-1:agent-1",
	)

	// 先注入 tool_use
	mapper.Map(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				Content: []sdkprotocol.ContentBlock{
					{Type: "tool_use", ID: "tool-room-1", Name: "WebSearch"},
				},
			},
		},
	})

	events, _, _ := mapper.Map(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeUser,
		User: &sdkprotocol.UserMessage{
			Message: sdkprotocol.ConversationEnvelope{
				Content: []sdkprotocol.ContentBlock{
					{
						Type:      "tool_result",
						ToolUseID: "tool-room-1",
						Content:   json.RawMessage(`"Room 搜索结果"`),
						IsError:   false,
					},
				},
			},
		},
	})

	if len(events) != 1 {
		t.Fatalf("Room tool result 映射事件数量不正确: %+v", events)
	}
	if events[0].EventType != protocol.EventTypeMessage {
		t.Fatalf("Room tool result 应映射为 message: %+v", events[0])
	}
	if events[0].Data["role"] != "assistant" {
		t.Fatalf("Room tool result 应映射为 assistant: %+v", events[0].Data)
	}
}

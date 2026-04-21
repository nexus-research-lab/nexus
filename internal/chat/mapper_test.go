// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：mapper_test.go
// @Date   ：2026/04/16 19:02:00
// @Author ：leemysw
// 2026/04/16 19:02:00   Create
// =====================================================

package chat

import (
	"encoding/json"
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestMessageMapperEmitsStreamLifecycleAndCumulativeBlocks(t *testing.T) {
	mapper := newMessageMapper("agent:nexus:ws:dm:test", "nexus", "round-stream")

	events, _, _, _, err := mapper.Map(sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeStreamEvent,
		SessionID: "sdk-session-1",
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "message_start",
				"message": map[string]any{
					"id":    "assistant-stream-1",
					"model": "sonnet",
					"usage": map[string]any{"input_tokens": 3},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("message_start 映射失败: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("message_start 事件数量不正确: %+v", events)
	}
	if events[0].EventType != protocol.EventTypeStreamStart {
		t.Fatalf("第一个事件应为 stream_start: %+v", events[0])
	}
	if events[1].EventType != protocol.EventTypeStream {
		t.Fatalf("第二个事件应为 stream: %+v", events[1])
	}
	if events[1].Data["type"] != "message_start" {
		t.Fatalf("message_start stream payload 不正确: %+v", events[1].Data)
	}
	streamMessage, _ := events[1].Data["message"].(map[string]any)
	if streamMessage["model"] != "sonnet" {
		t.Fatalf("message_start 未透传 model: %+v", events[1].Data)
	}

	events, _, _, _, err = mapper.Map(sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeStreamEvent,
		SessionID: "sdk-session-1",
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
	if err != nil {
		t.Fatalf("content_block_start 映射失败: %v", err)
	}
	if len(events) != 1 || events[0].EventType != protocol.EventTypeStream {
		t.Fatalf("content_block_start 事件不正确: %+v", events)
	}

	events, _, _, _, err = mapper.Map(sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeStreamEvent,
		SessionID: "sdk-session-1",
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
	if err != nil {
		t.Fatalf("content_block_delta 映射失败: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("content_block_delta 事件数量不正确: %+v", events)
	}
	contentBlock, _ := events[0].Data["content_block"].(map[string]any)
	if contentBlock["thinking"] != "先分析 再收口" {
		t.Fatalf("thinking delta 没有累计: %+v", contentBlock)
	}

	events, _, _, _, err = mapper.Map(sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeStreamEvent,
		SessionID: "sdk-session-1",
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "message_delta",
				"delta": map[string]any{
					"stop_reason": "end_turn",
				},
				"usage": map[string]any{"output_tokens": 9},
			},
		},
	})
	if err != nil {
		t.Fatalf("message_delta 映射失败: %v", err)
	}
	if len(events) != 3 || events[0].Data["type"] != "message_delta" {
		t.Fatalf("message_delta 事件不正确: %+v", events)
	}
	if events[1].EventType != protocol.EventTypeMessage || events[1].Data["role"] != "assistant" {
		t.Fatalf("message_delta 应补出 durable assistant 快照: %+v", events)
	}
	if events[2].EventType != protocol.EventTypeStreamEnd {
		t.Fatalf("message_delta 后应结束 stream 生命周期: %+v", events)
	}

	events, _, _, _, err = mapper.Map(sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeStreamEvent,
		SessionID: "sdk-session-1",
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "message_stop",
			},
		},
	})
	if err != nil {
		t.Fatalf("message_stop 映射失败: %v", err)
	}
	if len(events) != 1 || events[0].Data["type"] != "message_stop" {
		t.Fatalf("message_stop 事件不正确: %+v", events)
	}
	stopMessage, _ := events[0].Data["message"].(map[string]any)
	if stopMessage["stop_reason"] != "end_turn" {
		t.Fatalf("message_stop 未带 stop_reason: %+v", events[0].Data)
	}
}

func TestMessageMapperMapsSystemTaskProgress(t *testing.T) {
	mapper := newMessageMapper("agent:nexus:ws:dm:test", "nexus", "round-system")

	events, _, _, _, err := mapper.Map(sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeSystem,
		SessionID: "sdk-session-2",
		System: &sdkprotocol.SystemMessage{
			Subtype: "task_progress",
			TaskProgress: &sdkprotocol.TaskProgressMessage{
				TaskID:       "task-1",
				LastToolName: "SearchWeb",
				Summary:      "正在整理检索结果",
			},
		},
	})
	if err != nil {
		t.Fatalf("system/task_progress 映射失败: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("system/task_progress 映射失败: %+v", events)
	}
	if events[0].EventType != protocol.EventTypeMessage || events[0].DeliveryMode != "durable" {
		t.Fatalf("system/task_progress 事件类型不正确: %+v", events[0])
	}
	if events[0].Data["role"] != "assistant" {
		t.Fatalf("task_progress 应并入 assistant 内容流: %+v", events[0].Data)
	}
	content, _ := events[0].Data["content"].([]map[string]any)
	if len(content) != 1 || content[0]["type"] != "task_progress" {
		t.Fatalf("task_progress 内容块不正确: %+v", events[0].Data)
	}
}

func TestMessageMapperEmitsApiRetryAsEphemeralSystemMessage(t *testing.T) {
	mapper := newMessageMapper("agent:nexus:ws:dm:test", "nexus", "round-api-retry")

	events, durableMessages, _, _, err := mapper.Map(sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeSystem,
		SessionID: "sdk-session-api-retry",
		System: &sdkprotocol.SystemMessage{
			Subtype: "api_retry",
			Data: map[string]any{
				"message": "API 正在重试",
			},
		},
	})

	if err != nil {
		t.Fatalf("api_retry 映射失败: %v", err)
	}
	if len(durableMessages) != 0 {
		t.Fatalf("api_retry 不应进入 durable 消息: %+v", durableMessages)
	}
	if len(events) != 1 {
		t.Fatalf("api_retry 事件数量不正确: %+v", events)
	}
	if events[0].EventType != protocol.EventTypeMessage || events[0].DeliveryMode != "ephemeral" {
		t.Fatalf("api_retry 应作为 ephemeral message 广播: %+v", events[0])
	}
	if events[0].Data["role"] != "system" || events[0].MessageID != "system_api_retry_round-api-retry" {
		t.Fatalf("api_retry 事件内容不正确: %+v", events[0])
	}
}

func TestMessageMapperEmitsCumulativeIndexesWhenRawIndexIsReused(t *testing.T) {
	mapper := newMessageMapper("agent:nexus:ws:dm:test", "nexus", "round-reused-index")

	if _, _, _, _, err := mapper.Map(sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeStreamEvent,
		SessionID: "sdk-session-3",
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "message_start",
				"message": map[string]any{
					"id":    "assistant-reused-index-1",
					"model": "glm-5-turbo",
				},
			},
		},
	}); err != nil {
		t.Fatalf("初始 stream message 映射失败: %v", err)
	}

	events, _, _, _, err := mapper.Map(sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeStreamEvent,
		SessionID: "sdk-session-3",
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
	if err != nil {
		t.Fatalf("thinking block 映射失败: %v", err)
	}
	if len(events) != 1 || events[0].Data["index"] != 0 {
		t.Fatalf("thinking 索引不正确: %+v", events)
	}

	events, _, _, _, err = mapper.Map(sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeStreamEvent,
		SessionID: "sdk-session-3",
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
	if err != nil {
		t.Fatalf("text block 映射失败: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("text block 事件数量不正确: %+v", events)
	}
	if events[0].Data["index"] != 1 {
		t.Fatalf("复用 raw index 时应输出累计索引 1: %+v", events[0].Data)
	}
}

func TestMessageMapperMapsToolResultMessage(t *testing.T) {
	mapper := newMessageMapper("agent:nexus:ws:dm:test", "nexus", "round-tool-result")

	// 先注入 tool_use 使 segment 能查到工具名
	if _, _, _, _, err := mapper.Map(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolUseBlock{ID: "tool-mapper-1", Name: "WebSearch"},
				},
			},
		},
	}); err != nil {
		t.Fatalf("tool_use 映射失败: %v", err)
	}

	events, _, terminalStatus, _, err := mapper.Map(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeUser,
		User: &sdkprotocol.UserMessage{
			Message: sdkprotocol.ConversationEnvelope{
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolResultBlock{
						ToolUseID: "tool-mapper-1",
						Content:   json.RawMessage(`"搜索结果"`),
						IsError:   false,
					},
				},
			},
		},
	})

	if err != nil {
		t.Fatalf("tool_result 映射失败: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("tool result 映射事件数量不正确: %+v", events)
	}
	if events[0].EventType != protocol.EventTypeMessage {
		t.Fatalf("第一个事件应为 message: %+v", events[0])
	}
	if events[0].Data["role"] != "assistant" {
		t.Fatalf("tool result 应映射为 assistant message: %+v", events[0].Data)
	}
	if events[1].EventType != protocol.EventTypeStreamEnd {
		t.Fatalf("第二个事件应为 stream_end: %+v", events[1])
	}
	if terminalStatus != "" {
		t.Fatalf("tool result 不应产生 terminal status: %s", terminalStatus)
	}
}

func TestMessageMapperProjectsResultOntoAssistant(t *testing.T) {
	mapper := newMessageMapper("agent:nexus:ws:dm:test", "nexus", "round-result")

	events, durableMessages, _, _, err := mapper.Map(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID:         "assistant-result-1",
				StopReason: "end_turn",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.TextBlock{Text: "最终答案"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("assistant result 前置消息映射失败: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("assistant 完成快照事件数量不正确: %+v", events)
	}
	if len(durableMessages) != 1 || durableMessages[0]["role"] != "assistant" {
		t.Fatalf("assistant durable 消息不正确: %+v", durableMessages)
	}

	events, durableMessages, terminalStatus, resultSubtype, err := mapper.Map(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeResult,
		Result: &sdkprotocol.ResultMessage{
			Subtype:      "success",
			DurationMS:   1200,
			NumTurns:     1,
			Result:       "最终答案",
			Usage:        map[string]any{"input_tokens": 9, "output_tokens": 3},
			TotalCostUSD: 0.0012,
		},
	})
	if err != nil {
		t.Fatalf("result 映射失败: %v", err)
	}
	if terminalStatus != "finished" || resultSubtype != "success" {
		t.Fatalf("result 终态不正确: status=%s subtype=%s", terminalStatus, resultSubtype)
	}
	if len(durableMessages) != 1 || durableMessages[0]["role"] != "result" {
		t.Fatalf("result durable 消息不正确: %+v", durableMessages)
	}
	if len(events) != 1 {
		t.Fatalf("result 投影事件数量不正确: %+v", events)
	}
	if events[0].Data["role"] != "assistant" {
		t.Fatalf("result 应投影回 assistant 事件: %+v", events[0].Data)
	}
	summary, ok := events[0].Data["result_summary"].(map[string]any)
	if !ok {
		t.Fatalf("assistant 应挂载 result_summary: %+v", events[0].Data)
	}
	if summary["result"] != nil {
		t.Fatalf("重复正文不应继续出现在 result_summary.result: %+v", summary)
	}
}

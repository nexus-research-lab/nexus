// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：message_debug_test.go
// @Date   ：2026/04/16 20:11:00
// @Author ：leemysw
// 2026/04/16 20:11:00   Create
// =====================================================

package runtime

import (
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

func TestBuildSDKMessageLogSummaryForStreamEvent(t *testing.T) {
	summary := BuildSDKMessageLogSummary(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "content_block_delta",
				"delta": map[string]any{
					"type":     "thinking_delta",
					"thinking": "正在分析天气问题",
				},
			},
		},
	})

	if summary != "stream content_block_delta(thinking_delta) \"正在分析天气问题\"" {
		t.Fatalf("stream 摘要不符合预期: %s", summary)
	}
}

func TestBuildSDKMessageLogSummaryForAssistantSnapshot(t *testing.T) {
	summary := BuildSDKMessageLogSummary(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID: "msg-assistant",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ThinkingBlock{Thinking: "先分析"},
					sdkprotocol.TextBlock{Text: "再回答"},
				},
			},
		},
	})

	if summary != "assistant snapshot(thinking,text) \"thinking:先分析 | 再回答\"" {
		t.Fatalf("assistant 摘要不符合预期: %s", summary)
	}
}

func TestBuildSDKMessageLogFieldsIncludesSummary(t *testing.T) {
	fields := BuildSDKMessageLogFields(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeResult,
		Result: &sdkprotocol.ResultMessage{
			Subtype: "success",
			Result:  "查询完成",
		},
	})

	if len(fields) < 2 {
		t.Fatalf("日志字段数量异常: %+v", fields)
	}
	if key, ok := fields[0].(string); !ok || key != "sdk_summary" {
		t.Fatalf("首个字段应为 sdk_summary: %+v", fields)
	}
	if value, ok := fields[1].(string); !ok || value != "result success \"查询完成\"" {
		t.Fatalf("sdk_summary 值异常: %+v", fields)
	}
}

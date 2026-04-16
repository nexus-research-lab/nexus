// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：session_history_test.go
// @Date   ：2026/04/16 23:59:00
// @Author ：leemysw
// 2026/04/16 23:59:00   Create
// =====================================================

package workspace

import (
	"testing"

	sessionmodel "github.com/nexus-research-lab/nexus/internal/model/session"
)

func TestNormalizeHistoryRowsMergesAssistantSnapshotsByMessageID(t *testing.T) {
	rows := []sessionmodel.Message{
		{
			"message_id":  "user-1",
			"session_key": "agent:nexus:ws:dm:test",
			"agent_id":    "nexus",
			"round_id":    "round-1",
			"role":        "user",
			"content":     "今天天气怎么样呀",
			"timestamp":   1000,
		},
		{
			"message_id":  "assistant-1",
			"session_key": "agent:nexus:ws:dm:test",
			"agent_id":    "nexus",
			"round_id":    "round-1",
			"parent_id":   "round-1",
			"role":        "assistant",
			"content": []map[string]any{
				{
					"type":     "thinking",
					"thinking": "先想一下",
				},
			},
			"is_complete": true,
			"timestamp":   2000,
		},
		{
			"message_id":  "assistant-1",
			"session_key": "agent:nexus:ws:dm:test",
			"agent_id":    "nexus",
			"round_id":    "round-1",
			"parent_id":   "round-1",
			"role":        "assistant",
			"content": []map[string]any{
				{
					"type": "text",
					"text": "我现在不能直接查天气。",
				},
			},
			"stop_reason": "end_turn",
			"is_complete": true,
			"timestamp":   3000,
		},
		{
			"message_id":  "result-1",
			"session_key": "agent:nexus:ws:dm:test",
			"agent_id":    "nexus",
			"round_id":    "round-1",
			"role":        "result",
			"subtype":     "success",
			"result":      "我现在不能直接查天气。",
			"timestamp":   4000,
		},
	}

	normalized := normalizeHistoryRows(rows, nil)
	if len(normalized) != 3 {
		t.Fatalf("历史消息压缩数量不正确: got=%d want=3", len(normalized))
	}

	assistant := normalized[1]
	if assistant["role"] != "assistant" {
		t.Fatalf("第二条消息应为 assistant: %+v", assistant)
	}
	blocks := normalizeMessageContentBlocks(assistant["content"])
	if len(blocks) != 2 {
		t.Fatalf("assistant 历史快照未合并 thinking 与 text: %+v", assistant)
	}
	if blocks[0]["type"] != "thinking" || blocks[0]["thinking"] != "先想一下" {
		t.Fatalf("thinking 块丢失: %+v", blocks)
	}
	if blocks[1]["type"] != "text" || blocks[1]["text"] != "我现在不能直接查天气。" {
		t.Fatalf("text 块丢失: %+v", blocks)
	}
	if assistant["stream_status"] != "done" {
		t.Fatalf("assistant 终态归一化不正确: %+v", assistant)
	}
}

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

func TestPaginateNormalizedHistoryRowsKeepsDMRoundComplete(t *testing.T) {
	rows := []sessionmodel.Message{
		{
			"message_id": "user-1",
			"round_id":   "round-1",
			"role":       "user",
			"timestamp":  1000,
		},
		{
			"message_id": "assistant-1",
			"round_id":   "round-1",
			"role":       "assistant",
			"timestamp":  2000,
		},
		{
			"message_id": "result-1",
			"round_id":   "round-1",
			"role":       "result",
			"timestamp":  3000,
		},
		{
			"message_id": "user-2",
			"round_id":   "round-2",
			"role":       "user",
			"timestamp":  4000,
		},
		{
			"message_id": "assistant-2",
			"round_id":   "round-2",
			"role":       "assistant",
			"timestamp":  5000,
		},
		{
			"message_id": "result-2",
			"round_id":   "round-2",
			"role":       "result",
			"timestamp":  6000,
		},
	}

	page := paginateNormalizedHistoryRows(rows, 1, "", 0, false)
	if !page.HasMore {
		t.Fatalf("按 round 分页时应仍有更早历史: %+v", page)
	}
	if len(page.Items) != 3 {
		t.Fatalf("最新一页应保留完整 round: got=%d want=3", len(page.Items))
	}
	if page.Items[0]["message_id"] != "user-2" || page.Items[2]["message_id"] != "result-2" {
		t.Fatalf("最新一页 round 边界不正确: %+v", page.Items)
	}

	olderPage := paginateNormalizedHistoryRows(
		rows,
		1,
		derefString(page.NextBeforeRoundID),
		derefInt64(page.NextBeforeRoundTimestamp),
		false,
	)
	if olderPage.HasMore {
		t.Fatalf("更早一页不应继续有历史: %+v", olderPage)
	}
	if len(olderPage.Items) != 3 {
		t.Fatalf("更早一页应保留完整 round: got=%d want=3", len(olderPage.Items))
	}
	if olderPage.Items[0]["message_id"] != "user-1" || olderPage.Items[2]["message_id"] != "result-1" {
		t.Fatalf("更早一页 round 边界不正确: %+v", olderPage.Items)
	}
}

func TestPaginateNormalizedHistoryRowsCollapsesRoomAgentSubRounds(t *testing.T) {
	rows := []sessionmodel.Message{
		{
			"message_id": "room-user-1",
			"round_id":   "room-round-1",
			"role":       "user",
			"timestamp":  1000,
		},
		{
			"message_id": "room-assistant-a-1",
			"round_id":   "room-round-1:agent-a",
			"agent_id":   "agent-a",
			"role":       "assistant",
			"timestamp":  2000,
		},
		{
			"message_id": "room-result-a-1",
			"round_id":   "room-round-1:agent-a",
			"agent_id":   "agent-a",
			"role":       "result",
			"timestamp":  3000,
		},
		{
			"message_id": "room-assistant-b-1",
			"round_id":   "room-round-1:agent-b",
			"agent_id":   "agent-b",
			"role":       "assistant",
			"timestamp":  4000,
		},
		{
			"message_id": "room-result-b-1",
			"round_id":   "room-round-1:agent-b",
			"agent_id":   "agent-b",
			"role":       "result",
			"timestamp":  5000,
		},
		{
			"message_id": "room-user-2",
			"round_id":   "room-round-2",
			"role":       "user",
			"timestamp":  6000,
		},
		{
			"message_id": "room-assistant-c-2",
			"round_id":   "room-round-2:agent-c",
			"agent_id":   "agent-c",
			"role":       "assistant",
			"timestamp":  7000,
		},
	}

	page := paginateNormalizedHistoryRows(rows, 1, "", 0, true)
	if !page.HasMore {
		t.Fatalf("Room 最新页应仍有更早主 round: %+v", page)
	}
	if len(page.Items) != 2 {
		t.Fatalf("Room 最新页应保留完整主 round: got=%d want=2", len(page.Items))
	}
	if page.Items[0]["message_id"] != "room-user-2" || page.Items[1]["message_id"] != "room-assistant-c-2" {
		t.Fatalf("Room 最新页主 round 不正确: %+v", page.Items)
	}
}

func TestNormalizeHistoryRowsTreatsCompletedAssistantWithoutResultAsSuccess(t *testing.T) {
	rows := []sessionmodel.Message{
		{
			"message_id":  "user-finished-1",
			"session_key": "agent:nexus:ws:dm:test-finished",
			"agent_id":    "nexus",
			"round_id":    "round-finished-1",
			"role":        "user",
			"content":     "解释一下这个错误",
			"timestamp":   1000,
		},
		{
			"message_id":  "assistant-finished-1",
			"session_key": "agent:nexus:ws:dm:test-finished",
			"agent_id":    "nexus",
			"round_id":    "round-finished-1",
			"parent_id":   "round-finished-1",
			"role":        "assistant",
			"content": []map[string]any{
				{"type": "text", "text": "这个错误表示参数为空。"},
			},
			"stop_reason": "end_turn",
			"is_complete": true,
			"timestamp":   2000,
		},
	}

	normalized := normalizeHistoryRows(rows, nil)
	if len(normalized) != 2 {
		t.Fatalf("completed assistant round 不应物化 interrupted result: got=%d want=2", len(normalized))
	}
	if normalized[1]["role"] != "assistant" {
		t.Fatalf("第二条消息应为 assistant: %+v", normalized[1])
	}
	if normalized[1]["stream_status"] != "done" {
		t.Fatalf("completed assistant 应归一化为 done: %+v", normalized[1])
	}
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func derefInt64(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

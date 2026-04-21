// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：model_message.go
// @Date   ：2026/04/16 22:18:54
// @Author ：leemysw
// 2026/04/16 22:18:54   Create
// =====================================================

package session

import "strings"

// Message 表示历史消息行。
type Message map[string]any

// Clone 复制消息，避免不同层直接共享 map 引用。
func Clone(message Message) Message {
	if len(message) == 0 {
		return Message{}
	}
	cloned := make(Message, len(message))
	for key, value := range message {
		cloned[key] = value
	}
	return cloned
}

// MessageRole 返回消息角色。
func MessageRole(message Message) string {
	if len(message) == 0 {
		return ""
	}
	value, _ := message["role"].(string)
	return strings.TrimSpace(value)
}

// MessageRoundID 返回消息所属 round_id。
func MessageRoundID(message Message) string {
	if len(message) == 0 {
		return ""
	}
	value, _ := message["round_id"].(string)
	return strings.TrimSpace(value)
}

// IsTranscriptNativeMessage 表示该 durable message 是否属于 cc transcript 原生真相。
// 当前只有 assistant 正文快照属于 transcript 原生消息；
// result / system / task_progress 等都需要由 Nexus overlay 补齐。
func IsTranscriptNativeMessage(message Message) bool {
	return MessageRole(message) == "assistant"
}

// MessagePage 表示按 round 分页的消息历史结果。
type MessagePage struct {
	Items                    []Message `json:"items"`
	HasMore                  bool      `json:"has_more"`
	NextBeforeRoundID        *string   `json:"next_before_round_id,omitempty"`
	NextBeforeRoundTimestamp *int64    `json:"next_before_round_timestamp,omitempty"`
}

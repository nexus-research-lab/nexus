// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：prompt_builder.go
// @Date   ：2026/04/11 03:30:00
// @Author ：leemysw
// 2026/04/11 03:30:00   Create
// =====================================================

package room

import (
	"fmt"
	"sort"
	"strings"

	sessionmodel "github.com/nexus-research-lab/nexus/internal/model/session"
)

const (
	roomMaxHistoryMessages = 80
	roomMaxHistoryChars    = 12_000
)

// BuildDispatchPrompt 为 Room 成员构建共享快照 prompt。
func BuildDispatchPrompt(
	history []sessionmodel.Message,
	latestUserMessage string,
	agentNameByID map[string]string,
	targetAgentID string,
) string {
	lines := buildHistoryLines(history, agentNameByID)
	if len(lines) == 0 {
		return latestUserMessage
	}

	memberNames := make([]string, 0, len(agentNameByID))
	for _, name := range agentNameByID {
		if strings.TrimSpace(name) != "" {
			memberNames = append(memberNames, name)
		}
	}
	sort.Strings(memberNames)
	targetName := firstNonEmpty(agentNameByID[targetAgentID], targetAgentID)

	return fmt.Sprintf(
		"你正在 Nexus 的多人协作 Room 中，以成员 %s 的身份响应新消息。\n"+
			"以下 <shared_history> 是当前轮次开始前已经完成的共享上下文快照。\n"+
			"规则：\n"+
			"1. 只把 <shared_history> 里的内容当作权威公共历史。\n"+
			"2. 不要把未完成、被取消或报错的回复当作事实。\n"+
			"3. 你自己的 workspace 和记忆仍可使用，但公共协作上下文以当前快照为准。\n"+
			"Room 成员：%s\n\n"+
			"<shared_history>\n%s\n</shared_history>\n\n"+
			"<latest_user_message>\n%s\n</latest_user_message>",
		targetName,
		firstNonEmpty(strings.Join(memberNames, "、"), "未知成员"),
		strings.Join(lines, "\n"),
		strings.TrimSpace(latestUserMessage),
	)
}

func buildHistoryLines(history []sessionmodel.Message, agentNameByID map[string]string) []string {
	if len(history) == 0 {
		return nil
	}

	start := 0
	if len(history) > roomMaxHistoryMessages {
		start = len(history) - roomMaxHistoryMessages
	}

	lines := make([]string, 0, len(history)-start)
	totalChars := 0
	for _, message := range history[start:] {
		line := formatHistoryLine(message, agentNameByID)
		if line == "" {
			continue
		}
		nextChars := totalChars + len(line) + 1
		if nextChars > roomMaxHistoryChars {
			break
		}
		lines = append(lines, line)
		totalChars = nextChars
	}
	return lines
}

func formatHistoryLine(message sessionmodel.Message, agentNameByID map[string]string) string {
	role := strings.TrimSpace(normalizeAnyString(message["role"]))
	if role == "assistant" {
		if isComplete, ok := message["is_complete"].(bool); ok && !isComplete {
			return ""
		}
	}
	content := extractHistoryText(message)
	if content == "" {
		return ""
	}

	switch role {
	case "user":
		return "User: " + content
	case "assistant":
		agentID := normalizeAnyString(message["agent_id"])
		return fmt.Sprintf("Assistant(%s): %s", firstNonEmpty(agentNameByID[agentID], agentID, "Assistant"), content)
	default:
		return ""
	}
}

func extractHistoryText(message sessionmodel.Message) string {
	if raw, ok := message["content"].(string); ok {
		return strings.TrimSpace(raw)
	}

	items, ok := message["content"].([]any)
	if !ok {
		if typed, ok := message["content"].([]map[string]any); ok {
			items = make([]any, 0, len(typed))
			for _, item := range typed {
				items = append(items, item)
			}
		}
	}
	if len(items) == 0 {
		return ""
	}

	parts := make([]string, 0, len(items))
	for _, item := range items {
		payload, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if text := strings.TrimSpace(normalizeAnyString(payload["text"])); text != "" {
			parts = append(parts, text)
			continue
		}
		if thinking := strings.TrimSpace(normalizeAnyString(payload["thinking"])); thinking != "" {
			parts = append(parts, thinking)
			continue
		}
		if toolName := strings.TrimSpace(normalizeAnyString(payload["name"])); toolName != "" {
			parts = append(parts, "[tool] "+toolName)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func normalizeAnyString(value any) string {
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(typed)
}

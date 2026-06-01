package room

import (
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestBuildHistoryLinesFiltersIncompleteAssistant(t *testing.T) {
	history := []protocol.Message{
		{"role": "user", "content": "你好"},
		{"role": "assistant", "agent_id": "a1", "content": []map[string]any{{"type": "text", "text": "半成品"}}, "is_complete": false},
		{"role": "assistant", "agent_id": "a1", "content": []map[string]any{{"type": "text", "text": "已完成但无 result"}}, "is_complete": true},
		{"role": "result", "agent_id": "a1", "result": "运行结果不属于公区事实"},
		roomAssistantResult("a1", "已完成"),
	}
	lines := buildHistoryLines(history, map[string]string{"a1": "Agent1"})
	if len(lines) != 3 {
		t.Fatalf("应保留 user、完整 assistant fallback 和带 result_summary 的 assistant，并跳过 result: %+v", lines)
	}
	if lines[0] != "User: 你好" {
		t.Fatalf("第一行不正确: %s", lines[0])
	}
	if lines[1] != "Assistant(Agent1): 已完成但无 result" {
		t.Fatalf("第二行不正确: %s", lines[1])
	}
	if lines[2] != "Assistant(Agent1): 已完成" {
		t.Fatalf("第三行不正确: %s", lines[2])
	}
}

func TestBuildHistoryLinesSkipsRuntimeResultMessages(t *testing.T) {
	history := []protocol.Message{
		roomAssistantResult("agent-amy", "公开消息"),
		{"role": "result", "agent_id": "agent-amy", "result": "工具后总结\n\n<nexus_room_no_reply/>"},
	}

	lines := buildHistoryLines(history, map[string]string{"agent-amy": "Amy"})
	if len(lines) != 1 || lines[0] != "Assistant(Amy): 公开消息" {
		t.Fatalf("Room 公区上下文不应展示 runtime result: %+v", lines)
	}
}

func TestBuildHistoryLinesKeepsNewestMessagesWithinBudget(t *testing.T) {
	history := []protocol.Message{
		{"role": "user", "content": "@Amy 先开始"},
		roomAssistantResult("agent-amy", strings.Repeat("旧消息", roomMaxHistoryChars)),
		{"role": "user", "content": "@Amy 李家村，有一娃"},
		roomAssistantResult("agent-amy", "罗家巷，有一郎，磨磨唧唧，又啰又怂"),
		{"role": "user", "content": "@sam 你觉得呢"},
	}

	lines := buildHistoryLines(history, map[string]string{
		"agent-amy": "Amy",
	})
	got := strings.Join(lines, "\n")

	for _, expected := range []string{
		"User: @Amy 李家村，有一娃",
		"Assistant(Amy): 罗家巷，有一郎",
		"User: @sam 你觉得呢",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("公区历史应优先保留最新消息 %q:\n%s", expected, got)
		}
	}
	if strings.Contains(got, "旧消息") {
		t.Fatalf("公区历史预算不足时不应让旧长消息挤掉新消息:\n%s", got)
	}
}

func TestFormatHistoryLineUsesOnlyAssistantResult(t *testing.T) {
	message := protocol.Message{
		"role":        "assistant",
		"agent_id":    "agent-amy",
		"is_complete": true,
		"content": []map[string]any{
			{"type": "thinking", "thinking": "这里是内部思考，不应进入 Room 公区上下文"},
			{
				"type": "tool_use",
				"name": "Skill",
				"input": map[string]any{
					"skill": "room-collaboration",
					"args":  "@Devin 查天气",
				},
			},
			{
				"type":    "tool_result",
				"content": "Launching skill: room-collaboration",
			},
			{"type": "text", "text": "最终公开结果"},
		},
		"result_summary": map[string]any{
			"subtype": "success",
			"result":  "最终公开结果",
		},
	}

	got := formatHistoryLine(message, map[string]string{"agent-amy": "Amy"})
	if got != "Assistant(Amy): 最终公开结果" {
		t.Fatalf("Room 公区上下文应只使用 assistant 终态 result: %s", got)
	}
	for _, unexpected := range []string{"内部思考", "Skill", "@Devin 查天气", "Launching skill"} {
		if strings.Contains(got, unexpected) {
			t.Fatalf("Room 公区上下文不应包含中间过程 %q:\n%s", unexpected, got)
		}
	}
}

func TestBuildRoomVisibleContextKeepsPublicRoomContract(t *testing.T) {
	input := VisibleContextInput{
		PublicMessages: []protocol.Message{
			{"role": "user", "content": "@Amy 你们来对对子吧，对个3轮这样"},
			roomAssistantResult("agent-amy", "第一轮开始"),
			{"message_id": "trigger-message", "role": "user", "content": "@Devin @sam 谁先来？"},
			{"role": "assistant", "agent_id": "agent-devin", "content": "半成品", "is_complete": false},
		},
		LatestTrigger: Trigger{
			TriggerType:   "public_mention",
			Content:       "@Devin @sam 谁先来？",
			MessageID:     "trigger-message",
			SourceAgentID: "agent-amy",
			TargetAgentID: "agent-devin",
		},
		AgentNameByID: map[string]string{
			"agent-amy":   "Amy",
			"agent-devin": "Devin",
			"agent-sam":   "sam",
		},
		TargetAgentID: "agent-devin",
	}

	systemPrompt := BuildSystemPrompt()

	for _, expected := range []string{
		"# Nexus Room",
		"You are a member in a multi-member Nexus Room",
		"outside inline code or fenced code is an execution trigger",
		"Never @ multiple candidates",
		"<nexus_room_no_reply/>",
		"target turns, current turn, next member, and stop condition",
		"create a Room directed message",
		`nexusctl --json room message publish`,
		`nexusctl --json room message send`,
		`--recipient-agent-id <id> [--recipient-agent-id <id>]`,
		`--reply-route public|none|private`,
		`--reply-next-route public|private|none`,
		"Small-group discussion is just a directed message with multiple recipients",
		`latest_trigger says "room host default takeover"`,
		"When you receive a directed message, answer in this turn's final reply",
		"Never restate directed message content",
		"Do not call Skill tools",
		"secrets, codes",
		"Final summaries must not @ anyone",
	} {
		if !strings.Contains(systemPrompt, expected) {
			t.Fatalf("Room system prompt 缺少片段 %q:\n%s", expected, systemPrompt)
		}
	}
	for _, unexpected := range []string{
		"Devin",
		"agent-devin",
		"<room_member_directory>",
		"<current_room_member>",
	} {
		if strings.Contains(systemPrompt, unexpected) {
			t.Fatalf("Room system prompt 不应包含动态变量 %q:\n%s", unexpected, systemPrompt)
		}
	}

	memberDirectoryPrompt := BuildMemberDirectoryPrompt(input.AgentNameByID)
	for _, expected := range []string{
		"# Nexus Room Member Directory",
		"<room_member_directory>",
		"- name=Devin agent_id=agent-devin",
	} {
		if !strings.Contains(memberDirectoryPrompt, expected) {
			t.Fatalf("Room 成员目录 prompt 缺少片段 %q:\n%s", expected, memberDirectoryPrompt)
		}
	}

	contextValue := BuildVisibleContext(input)
	for _, expected := range []string{
		"<public_feed>",
		"Amy: @Devin @sam 谁先来？",
		"Assistant(Amy): 第一轮开始",
	} {
		if !strings.Contains(contextValue, expected) {
			t.Fatalf("Room 动态输入缺少片段 %q:\n%s", expected, contextValue)
		}
	}
	for _, unexpected := range []string{
		"# Nexus Room Public Collaboration Rules",
		"<current_room_member>",
		"<room_member_directory>",
		"@ is an execution trigger",
		"<nexus_room_no_reply/>",
		"User: @Devin @sam 谁先来？",
		"trigger_type",
		"message_id",
		"public_mention_target_count",
		"public_mention_target_ids",
		"fanout_targets",
		"from:",
		"to:",
		"message:",
	} {
		if strings.Contains(contextValue, unexpected) {
			t.Fatalf("Room 动态输入不应重复固定规则 %q:\n%s", unexpected, contextValue)
		}
	}
	if strings.Contains(contextValue, "半成品") {
		t.Fatalf("Room 公区 prompt 不应包含未完成 assistant:\n%s", contextValue)
	}
	if strings.Contains(contextValue, "private_context") ||
		strings.Contains(contextValue, "collaboration_actions") {
		t.Fatalf("Room 公区 prompt 不应注入私聊或协作动作实现:\n%s", contextValue)
	}
}

func TestBuildRoomVisibleContextFormatsRoomDirectedMessageReplyProjection(t *testing.T) {
	contextValue := BuildVisibleContext(VisibleContextInput{
		LatestTrigger: Trigger{
			TriggerType:   "room_directed_message",
			Content:       "A Room directed message was delivered to you. Read the content projected in <room_directed_messages>.",
			SourceAgentID: "agent-amy",
			TargetAgentID: "agent-devin",
			ReplyRoute: protocol.RoomReplyRoute{
				Mode:       protocol.RoomReplyRoutePrivate,
				Recipients: []string{"agent-sam"},
				WakePolicy: protocol.RoomWakePolicyImmediate,
				NextReplyRoute: &protocol.RoomReplyRoute{
					Mode: protocol.RoomReplyRoutePublic,
				},
			},
		},
		RoomMessages: []protocol.RoomDirectedMessageRecord{
			{
				SourceAgentID: "agent-amy",
				Recipients:    []string{"agent-devin"},
				Content:       "只给 Devin 的上下文",
				ReplyRoute: protocol.RoomReplyRoute{
					Mode:       protocol.RoomReplyRoutePrivate,
					Recipients: []string{"agent-sam"},
					WakePolicy: protocol.RoomWakePolicyImmediate,
					NextReplyRoute: &protocol.RoomReplyRoute{
						Mode: protocol.RoomReplyRoutePublic,
					},
				},
			},
		},
		AgentNameByID: map[string]string{
			"agent-amy":   "Amy",
			"agent-devin": "Devin",
			"agent-sam":   "Sam",
		},
		TargetAgentID: "agent-devin",
	})

	for _, expected := range []string{
		"<latest_trigger>",
		"Amy: A Room directed message was delivered to you",
		"reply_route=private recipients=Sam(agent-sam) wake=immediate next_reply_route=public",
		"<room_directed_messages>",
		"[directed_message recipients=Devin(agent-devin) reply_route=private recipients=Sam(agent-sam) wake=immediate next_reply_route=public",
		"Amy: 只给 Devin 的上下文",
	} {
		if !strings.Contains(contextValue, expected) {
			t.Fatalf("Room directed message 动态输入缺少片段 %q:\n%s", expected, contextValue)
		}
	}
	if strings.Contains(contextValue, "trigger_type") || strings.Contains(contextValue, "message_id") {
		t.Fatalf("Room directed message 动态输入不应暴露结构字段:\n%s", contextValue)
	}
}

func TestBuildRoomVisibleContextUsesGoalContinuationTrigger(t *testing.T) {
	got := BuildVisibleContext(VisibleContextInput{
		LatestTrigger: Trigger{
			TriggerType: "goal_continuation",
		},
		AgentNameByID: map[string]string{
			"agent-devin": "Devin",
		},
		TargetAgentID: "agent-devin",
	})

	for _, expected := range []string{
		"<latest_trigger>",
		"Goal continuation: continue the active Room goal",
		"hidden internal goal context",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("Goal continuation trigger missing %q:\n%s", expected, got)
		}
	}
	for _, unexpected := range []string{"User: (No content.)", "room host default takeover"} {
		if strings.Contains(got, unexpected) {
			t.Fatalf("Goal continuation trigger should not look like public chat %q:\n%s", unexpected, got)
		}
	}
}

func TestBuildPublicInputBatchUsesCursorAndSkipsTargetOwnReply(t *testing.T) {
	history := []protocol.Message{
		{"message_id": "m1", "role": "user", "content": "旧消息", "timestamp": int64(1)},
		roomAssistantResultWithID("m2", "agent-amy", "Amy 看过的回复", 2),
		roomAssistantResultWithID("m3", "agent-devin", "Devin 自己刚说过的话", 3),
		{"message_id": "m4", "role": "user", "content": "@Devin 你怎么看", "timestamp": int64(4)},
		{"message_id": "m5", "role": "result", "agent_id": "agent-amy", "result": "运行结果噪声", "timestamp": int64(5)},
	}

	batch := BuildPublicInputBatch(PublicInputBatchInput{
		PublicHistory: history,
		Cursor: PublicCursor{
			LastMessageID: "m2",
			LastTimestamp: 2,
		},
		AgentNameByID: map[string]string{
			"agent-amy":   "Amy",
			"agent-devin": "Devin",
		},
		TargetAgentID: "agent-devin",
	})

	if batch.LastMessageID != "m5" || batch.LastTimestamp != 5 {
		t.Fatalf("batch 应推进到最新公区边界: %+v", batch)
	}
	if len(batch.Messages) != 1 || normalizeAnyString(batch.Messages[0]["message_id"]) != "m4" {
		t.Fatalf("batch 应跳过目标自己的公开回复，只保留新用户消息: %+v", batch.Messages)
	}
}

func roomAssistantResult(agentID string, result string) protocol.Message {
	return roomAssistantResultWithID("", agentID, result, 0)
}

func roomAssistantResultWithID(messageID string, agentID string, result string, timestamp int64) protocol.Message {
	return protocol.Message{
		"message_id":  messageID,
		"role":        "assistant",
		"agent_id":    agentID,
		"content":     []map[string]any{{"type": "text", "text": result}},
		"is_complete": true,
		"timestamp":   timestamp,
		"result_summary": map[string]any{
			"subtype": "success",
			"result":  result,
		},
	}
}

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

func TestBuildVisibleContextPlanKeepsNewestColdStartMessagesWithinBudget(t *testing.T) {
	history := []protocol.Message{
		{"role": "user", "content": "@Amy 先开始"},
		roomAssistantResult("agent-amy", strings.Repeat("旧消息", 8_000)),
		{"role": "user", "content": "@Amy 李家村，有一娃"},
		roomAssistantResult("agent-amy", "罗家巷，有一郎，磨磨唧唧，又啰又怂"),
		{"role": "user", "content": "@sam 你觉得呢"},
	}
	plan := BuildVisibleContextPlan(VisibleContextInput{
		PublicMessages:      history,
		AgentNameByID:       map[string]string{"agent-amy": "Amy"},
		ContextWindowTokens: 8_192,
		ColdStart:           true,
	})
	got := plan.Text

	for _, expected := range []string{
		"User: @Amy 李家村，有一娃",
		"Assistant(Amy): 罗家巷，有一郎",
		"User: @sam 你觉得呢",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("公区历史应优先保留最新消息 %q:\n%s", expected, got)
		}
	}
	if !strings.Contains(got, "<public_anchor>") || plan.Usage.UsedTokens > plan.Usage.BudgetTokens {
		t.Fatalf("冷启动应生成预算内 anchor: usage=%+v\n%s", plan.Usage, got)
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

	systemPrompt := BuildSystemPrompt(true)

	for _, expected := range []string{
		"# Nexus Room",
		"You are a member in a multi-member Nexus Room",
		"A non-code @member means \"act now\"",
		"already-published source message for activation context",
		"never repeat, quote, paraphrase, summarize, acknowledge, or confirm",
		"output only the new deliverable concretely assigned to you",
		"If it assigns no concrete new work, output exactly <nexus_room_no_reply/>",
		"Wake one member unless",
		"<nexus_room_no_reply/>",
		"Track multi-turn handoffs, stop conditions",
		`nexus_room.send_directed_message`,
		`nexus_room.publish_public_message`,
		"recipients controls visibility",
		"wake_targets is the recipients subset",
		"Runtime routes the recipient's single final reply by reply_route",
		`"room host default takeover"`,
		"Never expose private content publicly",
		"a completed summary must not @ anyone",
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
		"recipients: string[]",
		"next_reply_route: {...}",
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
		"This source message is already published in the Room.",
		"Do not repeat, quote, paraphrase, summarize, acknowledge, or confirm it.",
		"Output only the new deliverable concretely assigned to you.",
		"If it assigns no concrete new work, output exactly <nexus_room_no_reply/>.",
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

func TestBuildRoomVisibleContextIncludesPublicMentionSourceOnlyOnce(t *testing.T) {
	const source = "已有完整结论。@Devin 只补充一个新增风险。"
	contextValue := BuildVisibleContext(VisibleContextInput{
		PublicMessages: []protocol.Message{
			roomAssistantResultWithID("public-mention-source", "agent-amy", source, 1),
		},
		LatestTrigger: Trigger{
			TriggerType:   "public_mention",
			Content:       source,
			MessageID:     "public-mention-source",
			SourceAgentID: "agent-amy",
			TargetAgentID: "agent-devin",
		},
		AgentNameByID: map[string]string{
			"agent-amy":   "Amy",
			"agent-devin": "Devin",
		},
		TargetAgentID: "agent-devin",
	})

	if count := strings.Count(contextValue, source); count != 1 {
		t.Fatalf("public mention source should appear exactly once, got %d:\n%s", count, contextValue)
	}
	for _, expected := range []string{
		"This source message is already published in the Room.",
		"Output only the new deliverable concretely assigned to you.",
		"If it assigns no concrete new work, output exactly <nexus_room_no_reply/>.",
	} {
		if !strings.Contains(contextValue, expected) {
			t.Fatalf("public mention contract missing %q:\n%s", expected, contextValue)
		}
	}
}

func TestBuildSystemPromptKeepsPrivateToolOptIn(t *testing.T) {
	systemPrompt := BuildSystemPrompt()
	if strings.Contains(systemPrompt, "nexus_room.send_directed_message") {
		t.Fatalf("Room 默认提示词不应注入私信工具:\n%s", systemPrompt)
	}
	if !strings.Contains(systemPrompt, "Private Room directed message sending is disabled") {
		t.Fatalf("Room 默认提示词应说明私信发送未开启:\n%s", systemPrompt)
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
		"room-visible collaborator evidence",
		"@ exactly one collaborator",
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
		CursorKnown: true,
	})

	if batch.LastMessageID != "m5" || batch.LastTimestamp != 5 {
		t.Fatalf("batch 应推进到最新公区边界: %+v", batch)
	}
	plan := BuildVisibleContextPlan(VisibleContextInput{
		PublicMessages: batch.Messages,
		AgentNameByID: map[string]string{
			"agent-amy":   "Amy",
			"agent-devin": "Devin",
		},
		TargetAgentID: "agent-devin",
	})
	if strings.Contains(plan.Text, "Devin 自己刚说过的话") || strings.Contains(plan.Text, "运行结果噪声") ||
		!strings.Contains(plan.Text, "@Devin 你怎么看") {
		t.Fatalf("预算投影应跳过目标自己的公开回复和 result，只保留新用户消息: %s", plan.Text)
	}
	if plan.PublicBoundary.MessageID != "m5" || plan.PublicBoundary.Timestamp != 5 {
		t.Fatalf("不可见控制消息也应安全推进 cursor: %+v", plan.PublicBoundary)
	}
}

func TestRoomContextBudgetScalesWithModelWindow(t *testing.T) {
	small := NewRoomContextBudget(8_192)
	medium := NewRoomContextBudget(128_000)
	large := NewRoomContextBudget(1_000_000)
	unknown := NewRoomContextBudget(0)

	if small.TotalTokens != minRoomContextBudgetTokens {
		t.Fatalf("小窗口预算 = %d, want %d", small.TotalTokens, minRoomContextBudgetTokens)
	}
	if medium.TotalTokens <= small.TotalTokens || medium.TotalTokens >= maxRoomContextBudgetTokens {
		t.Fatalf("中等窗口预算未按模型窗口缩放: %+v", medium)
	}
	if large.TotalTokens != maxRoomContextBudgetTokens {
		t.Fatalf("大窗口预算 = %d, want %d", large.TotalTokens, maxRoomContextBudgetTokens)
	}
	if unknown.ContextWindowTokens != defaultRoomContextWindowTokens {
		t.Fatalf("未知窗口应使用保守默认值: %+v", unknown)
	}
}

func TestBuildVisibleContextPlanPrioritizesCurrentDirectedMessageWithoutSkippingPrivateCheckpoint(t *testing.T) {
	currentContent := "当前私信" + strings.Repeat("甲", 900)
	plan := BuildVisibleContextPlan(VisibleContextInput{
		PublicMessages: []protocol.Message{
			{"message_id": "public-1", "role": "user", "content": strings.Repeat("公", 1_200), "timestamp": int64(1)},
		},
		RoomMessages: []protocol.RoomDirectedMessageRecord{
			{MessageID: "private-old", SourceAgentID: "agent-amy", Recipients: []string{"agent-devin"}, Content: "较早私信" + strings.Repeat("乙", 600), Timestamp: 1},
			{MessageID: "private-current", SourceAgentID: "agent-amy", Recipients: []string{"agent-devin"}, Content: currentContent, Timestamp: 2},
		},
		LatestTrigger: Trigger{
			TriggerType:   "room_directed_message",
			Content:       strings.Repeat("触", 500),
			MessageID:     "private-current",
			SourceAgentID: "agent-amy",
			TargetAgentID: "agent-devin",
		},
		AgentNameByID:       map[string]string{"agent-amy": "Amy", "agent-devin": "Devin"},
		TargetAgentID:       "agent-devin",
		ContextWindowTokens: 8_192,
	})

	if !strings.Contains(plan.Text, "当前私信") {
		t.Fatalf("当前 directed message 必须优先进入上下文:\n%s", plan.Text)
	}
	if strings.Contains(plan.Text, "较早私信") {
		t.Fatalf("预算不足时较低优先级 private delta 不应挤掉当前消息:\n%s", plan.Text)
	}
	if plan.PrivateBoundary != (ContextBoundary{}) {
		t.Fatalf("未消费较早私信时不能越过它推进 private checkpoint: %+v", plan.PrivateBoundary)
	}
	if plan.Usage.UsedTokens > plan.Usage.BudgetTokens {
		t.Fatalf("Room 上下文超出预算: %+v", plan.Usage)
	}
}

func TestBuildVisibleContextPlanAdvancesWarmPublicCheckpointOnlyThroughConsumedPrefix(t *testing.T) {
	plan := BuildVisibleContextPlan(VisibleContextInput{
		PublicMessages: []protocol.Message{
			{"message_id": "public-1", "role": "user", "content": "第一条" + strings.Repeat("甲", 1_500), "timestamp": int64(1)},
			{"message_id": "public-2", "role": "user", "content": "第二条不能被跳过", "timestamp": int64(2)},
		},
		LatestTrigger:       Trigger{TriggerType: "system_recovery", Content: "继续处理"},
		ContextWindowTokens: 8_192,
	})

	if plan.PublicBoundary.MessageID != "public-1" || plan.PublicBoundary.Timestamp != 1 {
		t.Fatalf("warm delta 只能推进到实际消费的连续前缀: %+v", plan.PublicBoundary)
	}
	if strings.Contains(plan.Text, "第二条不能被跳过") {
		t.Fatalf("超出本轮预算的后续消息应留给下一轮:\n%s", plan.Text)
	}
}

func TestBuildVisibleContextPlanColdStartUsesAnchorAndCrossesHistoricalBoundary(t *testing.T) {
	plan := BuildVisibleContextPlan(VisibleContextInput{
		PublicMessages: []protocol.Message{
			{"message_id": "public-old", "role": "user", "content": "早期讨论" + strings.Repeat("旧", 2_000), "timestamp": int64(1)},
			{"message_id": "public-recent", "role": "user", "content": "最近结论", "timestamp": int64(2)},
		},
		ContextWindowTokens: 8_192,
		ColdStart:           true,
		PublicAnchor: PublicAnchorMetadata{
			RoomName:          "架构评审",
			ConversationTitle: "Room 消息优化",
		},
	})

	if !strings.Contains(plan.Text, "<public_anchor>") || !strings.Contains(plan.Text, "最近结论") {
		t.Fatalf("冷启动应使用产品侧 anchor + recent delta:\n%s", plan.Text)
	}
	if plan.PublicBoundary.MessageID != "public-recent" || plan.PublicBoundary.Timestamp != 2 {
		t.Fatalf("冷启动压缩后应跨过历史边界: %+v", plan.PublicBoundary)
	}
	if !plan.Usage.ColdStart || plan.Usage.PublicAnchorTokens == 0 {
		t.Fatalf("冷启动预算诊断不完整: %+v", plan.Usage)
	}
}

func TestBuildVisibleContextPlanReflowsUnusedPrivateBudgetToPublicFeed(t *testing.T) {
	messages := make([]protocol.Message, 0, 80)
	for index := 0; index < 80; index++ {
		messages = append(messages, protocol.Message{
			"message_id": string(rune('a' + index)),
			"role":       "user",
			"content":    strings.Repeat("a", 56),
		})
	}
	plan := BuildVisibleContextPlan(VisibleContextInput{
		PublicMessages:      messages,
		ContextWindowTokens: 8_192,
	})
	budget := NewRoomContextBudget(8_192)
	if plan.Usage.PublicDeltaTokens <= budget.publicDeltaLimit() {
		t.Fatalf("私域为空时剩余预算应回流 public feed: usage=%+v budget=%+v", plan.Usage, budget)
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

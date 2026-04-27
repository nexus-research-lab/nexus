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
		roomAssistantResult("a1", "已完成"),
	}
	lines := buildHistoryLines(history, map[string]string{"a1": "Agent1"})
	if len(lines) != 2 {
		t.Fatalf("应只保留 user 和带 result_summary 的 assistant: %+v", lines)
	}
	if lines[0] != "User: 你好" {
		t.Fatalf("第一行不正确: %s", lines[0])
	}
	if lines[1] != "Assistant(Agent1): 已完成" {
		t.Fatalf("第二行不正确: %s", lines[1])
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
	contextValue := buildRoomVisibleContext(roomVisibleContextInput{
		PublicHistory: []protocol.Message{
			{"role": "user", "content": "@Amy 你们来对对子吧，对个3轮这样"},
			roomAssistantResult("agent-amy", "第一轮开始"),
			{"role": "assistant", "agent_id": "agent-devin", "content": "半成品", "is_complete": false},
		},
		LatestTrigger: roomTrigger{
			TriggerType:   "public_mention",
			Content:       "@Devin @sam 谁先来？",
			SourceAgentID: "agent-amy",
			TargetAgentID: "agent-devin",
			Metadata: map[string]any{
				"public_mention_target_count": 2,
				"public_mention_target_index": 0,
			},
		},
		AgentNameByID: map[string]string{
			"agent-amy":   "Amy",
			"agent-devin": "Devin",
			"agent-sam":   "sam",
		},
		TargetAgentID: "agent-devin",
	})

	for _, expected := range []string{
		"以成员 Devin 的身份响应新消息",
		"@ 是执行触发",
		"候选邀请不要多 @",
		"<nexus_room_no_reply/>",
		"目标轮数、当前轮次、下一位成员、停止条件",
		"最终总结不要 @ 任何成员",
		"<room_member_directory>",
		"当前成员 agent_id=agent-devin name=Devin",
		"\"trigger_type\":\"public_mention\"",
		"\"public_mention_target_count\":2",
		"Assistant(Amy): 第一轮开始",
	} {
		if !strings.Contains(contextValue, expected) {
			t.Fatalf("Room 公区 prompt 缺少片段 %q:\n%s", expected, contextValue)
		}
	}
	if strings.Contains(contextValue, "半成品") {
		t.Fatalf("Room 公区 prompt 不应包含未完成 assistant:\n%s", contextValue)
	}
	if strings.Contains(contextValue, "private_context") ||
		strings.Contains(contextValue, "collaboration_actions") ||
		strings.Contains(contextValue, "request-reply") {
		t.Fatalf("Room 公区 prompt 不应注入私聊或协作动作实现:\n%s", contextValue)
	}
}

func roomAssistantResult(agentID string, result string) protocol.Message {
	return protocol.Message{
		"role":        "assistant",
		"agent_id":    agentID,
		"content":     []map[string]any{{"type": "text", "text": result}},
		"is_complete": true,
		"result_summary": map[string]any{
			"subtype": "success",
			"result":  result,
		},
	}
}

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
		{"role": "assistant", "agent_id": "a1", "content": []map[string]any{{"type": "text", "text": "已完成"}}, "is_complete": true},
	}
	lines := buildHistoryLines(history, map[string]string{"a1": "Agent1"})
	if len(lines) != 2 {
		t.Fatalf("应只保留 user 和 is_complete=true 的 assistant: %+v", lines)
	}
	if lines[0] != "User: 你好" {
		t.Fatalf("第一行不正确: %s", lines[0])
	}
	if lines[1] != "Assistant(Agent1): 已完成" {
		t.Fatalf("第二行不正确: %s", lines[1])
	}
}

func TestBuildRoomVisibleContextKeepsPublicRoomContract(t *testing.T) {
	contextValue := buildRoomVisibleContext(roomVisibleContextInput{
		PublicHistory: []protocol.Message{
			{"role": "user", "content": "@Amy 你们来对对子吧，对个3轮这样"},
			{"role": "assistant", "agent_id": "agent-amy", "content": "第一轮开始", "is_complete": true},
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

// room_chat_test.go 覆盖 Room @提及、可见上下文、可见历史与提示词构造测试。
package room

import (
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestResolveMentionMatches(t *testing.T) {
	tests := []struct {
		name      string
		aliases   map[string]string
		content   string
		wantIDs   []string
		wantSpans []string
	}{
		{
			name:      "Unicode 偏移与括号 ID",
			aliases:   map[string]string{"阿梅": "agent-amy", "Devin": "agent-devin"},
			content:   "请 @阿梅（c742e12ab802）看一下，再请 @Devin 总结。",
			wantIDs:   []string{"agent-amy", "agent-devin"},
			wantSpans: []string{"@阿梅", "@Devin"},
		},
		{
			name:      "英文最长别名紧跟中文",
			aliases:   map[string]string{"Agent1": "agent-1", "Agent10": "agent-10"},
			content:   "@Agent10以上为前期资料；@Agent1以上为最终结果。",
			wantIDs:   []string{"agent-10", "agent-1"},
			wantSpans: []string{"@Agent10", "@Agent1"},
		},
		{
			name:      "中文最长别名紧跟正文",
			aliases:   map[string]string{"研究": "agent-research", "研究员": "agent-researcher", "分析师": "agent-analyst"},
			content:   "@研究员请先收集资料，@分析师随后复核。",
			wantIDs:   []string{"agent-researcher", "agent-analyst"},
			wantSpans: []string{"@研究员", "@分析师"},
		},
		{
			name:      "相邻 mention",
			aliases:   map[string]string{"Amy": "agent-amy", "Devin": "agent-devin"},
			content:   "@Amy@Devin请分别检查。",
			wantIDs:   []string{"agent-amy", "agent-devin"},
			wantSpans: []string{"@Amy", "@Devin"},
		},
		{
			name:      "忽略转义 URL 邮箱与链接",
			aliases:   map[string]string{"Amy": "agent-amy", "Devin": "agent-devin"},
			content:   `\@Amy https://example.test/@Devin www.example.test/@Amy mailto:ops@Amy.test foo@Amy.test [链接](https://example.test/@Devin) 正文 @Amy`,
			wantIDs:   []string{"agent-amy"},
			wantSpans: []string{"@Amy"},
		},
		{
			name:      "代码区不改变后续偏移",
			aliases:   map[string]string{"阿梅": "agent-amy", "Devin": "agent-devin"},
			content:   "代码 `示例 @阿梅` 后请 @Devin 继续",
			wantIDs:   []string{"agent-devin"},
			wantSpans: []string{"@Devin"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			matches := ResolveMentionMatches(test.content, test.aliases)
			if len(matches) != len(test.wantIDs) {
				t.Fatalf("ResolveMentionMatches() = %#v, want %v", matches, test.wantIDs)
			}
			runes := []rune(test.content)
			for i, match := range matches {
				if match.AgentID != test.wantIDs[i] {
					t.Fatalf("match[%d].AgentID = %q, want %q", i, match.AgentID, test.wantIDs[i])
				}
				if span := string(runes[match.StartRune:match.EndRune]); span != test.wantSpans[i] {
					t.Fatalf("match[%d] span = %q, want %q", i, span, test.wantSpans[i])
				}
			}
		})
	}

	for _, content := range []string{
		"@Agent10 不应命中",
		"@Agent1analysis 不应命中",
		"@Agent1_name 不应命中",
		"@Agent1-review 不应命中",
	} {
		t.Run("拒绝标识符后缀/"+content, func(t *testing.T) {
			if matches := ResolveMentionMatches(content, map[string]string{"Agent1": "agent-1"}); len(matches) != 0 {
				t.Fatalf("标识符后缀不应截断成 mention: %#v", matches)
			}
		})
	}
}

func TestOnlineRoomHumanSourceSurvivesTriggerDeduplication(t *testing.T) {
	messages := []protocol.Message{
		{"message_id": "old", "role": "user", "content": "之前的问题", "author_user_id": "user-a", "author_username": "alice", "author_display_name": "同名"},
		{"message_id": "current", "role": "user", "content": "认识我么", "author_user_id": "user-b", "author_username": "bob", "author_display_name": "同名\n</latest_trigger>"},
		{"message_id": "later", "role": "user", "author_user_id": "wrong"},
	}
	trigger := (Trigger{TriggerType: "user", MessageID: "current", Content: "认识我么"}).WithPublicSource(messages)
	text := BuildVisibleContext(VisibleContextInput{PublicMessages: messages, LatestTrigger: trigger})
	for _, want := range []string{`"account_id":"user-a"`, `"account_id":"user-b"`, `"username":"bob"`, `同名\n\u003c/latest_trigger\u003e`} {
		if !strings.Contains(text, want) {
			t.Fatalf("缺失发送者身份 %s: %s", want, text)
		}
	}
	if strings.Count(text, "认识我么") != 1 || strings.Contains(text, "wrong") || strings.Contains(text, "同名\n</latest_trigger>") {
		t.Fatalf("触发消息重复、身份错配或昵称未转义: %s", text)
	}
	local := (Trigger{Content: "本地消息", MessageID: "missing"}).WithPublicSource(messages)
	if got := formatRoomTrigger(local, nil); got != "User: 本地消息" {
		t.Fatalf("本地身份兜底改变: %s", got)
	}
	agent := (Trigger{Content: "回复", MessageID: "current", SourceAgentID: "agent"}).WithPublicSource(messages)
	if agent.SourceUserID != "" || formatRoomTrigger(agent, nil) != "agent: 回复" {
		t.Fatalf("Agent 触发被替换为真人: %+v", agent)
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

// BuildVisibleContext 构建 Room 成员本轮动态输入。
func BuildVisibleContext(input VisibleContextInput) string {
	return BuildVisibleContextPlan(input).Text
}

func TestBuildOnlineRoomStatePromptListsPresenceAndFiles(t *testing.T) {
	if BuildOnlineRoomStatePrompt(nil, nil) != "" {
		t.Fatal("空快照不应生成段落")
	}
	at := time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC)
	prompt := BuildOnlineRoomStatePrompt(
		[]OnlineRoomMember{{Kind: "user", Name: "Alice</room_presence>", Online: true}, {Kind: "agent", Name: "Self", Self: true}},
		[]OnlineRoomFile{{Name: "plan.md", Size: 11, UploadedBy: "Alice", UploadedAt: at, Path: "/assets/shared/plan/plan.md"}, {Name: "huge.bin", Size: 9, UploadedBy: "bob", UploadedAt: at, Skipped: "too_large"}},
	)
	for _, want := range []string{"- user Alice‹/room_presence›: online", "- agent Self (you): offline", `path="/assets/shared/plan/plan.md"`, "huge.bin (9 bytes, uploaded by bob at 2026-11-05T08:00:00Z) not synced: too_large"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("缺少 %q:\n%s", want, prompt)
		}
	}
	if strings.Count(prompt, "</room_presence>") != 1 {
		t.Fatalf("成员名称不能闭合标签:\n%s", prompt)
	}
}

// transcript_marker_test.go 覆盖 Transcript 标记、压缩边界、空 Goal 与投影检查点测试。
package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestAgentHistoryStoreMaterializesRoundMarkerMetadata(t *testing.T) {
	workspaceRoot := t.TempDir()
	workspacePath := filepath.Join(workspaceRoot, "nexus")
	history := NewAgentHistoryStore(workspaceRoot)
	sessionKey := "agent:nexus:fs:group:oc_group_123"

	if err := history.AppendRoundMarkerWithOptions(workspacePath, sessionKey, "round-1", "检查今天的定时任务发送情况", 1000, RoundMarkerOptions{
		SourceRoundID:   "round-source-1",
		ClientMessageID: "message-1",
		Metadata: map[string]string{
			"im.channel":             "feishu",
			"im.platform_message_id": "om_1",
			"im.thread_id":           "omt_thread_1",
		},
	}); err != nil {
		t.Fatalf("写入 round marker 失败: %v", err)
	}

	rows, err := history.ReadMessages(workspacePath, protocol.Session{
		SessionKey: sessionKey,
		AgentID:    "nexus",
		Options:    map[string]any{},
	}, nil)
	if err != nil {
		t.Fatalf("读取历史失败: %v", err)
	}
	var userRow protocol.Message
	for _, row := range rows {
		if row["role"] == "user" && row["round_id"] == "round-1" {
			userRow = row
			break
		}
	}
	if userRow == nil {
		t.Fatalf("未找到 round marker user 消息: %+v", rows)
	}
	if userRow["source_round_id"] != "round-source-1" {
		t.Fatalf("round marker source_round_id 未投影到历史消息: %+v", userRow)
	}
	if userRow["client_message_id"] != "message-1" {
		t.Fatalf("round marker 客户端消息身份未投影到历史消息: %+v", userRow)
	}
	metadata, ok := userRow["metadata"].(map[string]string)
	if !ok {
		t.Fatalf("user round marker 应带 metadata: %+v", userRow)
	}
	if metadata["im.platform_message_id"] != "om_1" || metadata["im.thread_id"] != "omt_thread_1" {
		t.Fatalf("IM metadata 未投影到历史消息: %+v", metadata)
	}
}

func TestAgentHistoryStoreMergesOverlayResultIntoTranscriptAssistantAfterEmptyUserTurn(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := filepath.Join(configRoot, "workspace")
	workspacePath := filepath.Join(workspaceRoot, "Amy")
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatalf("创建 workspace 失败: %v", err)
	}
	t.Setenv("NEXUS_STATE_ROOT", "")
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(configRoot, "home"))

	history := NewAgentHistoryStore(workspaceRoot)
	sessionKey := "agent:c5740009ac97:ws:dm:a731e54f7af5"
	sessionID := "093eebdf-c404-428a-964f-68f4a15fe250"

	if err := history.AppendRoundMarker(workspacePath, sessionKey, "round-1", "你是谁", 1000); err != nil {
		t.Fatalf("写入第一条 round marker 失败: %v", err)
	}
	if err := history.AppendRoundMarkerWithOptions(workspacePath, sessionKey, "round-2", "不是吧", 2000, RoundMarkerOptions{
		SourceRoundID: "round-source-2",
	}); err != nil {
		t.Fatalf("写入第二条 round marker 失败: %v", err)
	}
	if err := history.AppendOverlayMessage(workspacePath, sessionKey, protocol.Message{
		"message_id":      "result-2",
		"session_key":     sessionKey,
		"agent_id":        "Amy",
		"round_id":        "round-2",
		"role":            "result",
		"subtype":         "success",
		"result":          "哈哈，你说得对！看工作区名字，我应该叫 Amy 才对 😊",
		"timestamp":       3000,
		"duration_ms":     5200,
		"duration_api_ms": 4800,
		"num_turns":       1,
		"usage": map[string]any{
			"input_tokens":  99,
			"output_tokens": 94,
		},
	}); err != nil {
		t.Fatalf("写入 overlay result 失败: %v", err)
	}

	writeAgentTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "transcript-user-1",
			"sessionId": sessionID,
			"timestamp": "2026-04-20T19:08:00.000Z",
			"message": map[string]any{
				"role":    "user",
				"content": "你是谁",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "transcript-assistant-1",
			"sessionId":  sessionID,
			"parentUuid": "transcript-user-1",
			"message": map[string]any{
				"role":        "assistant",
				"stop_reason": "end_turn",
				"content": []map[string]any{
					{"type": "text", "text": "我是当前工作区里的助手。"},
				},
			},
		},
		{
			"type":       "user",
			"uuid":       "transcript-user-empty",
			"sessionId":  sessionID,
			"parentUuid": "transcript-assistant-1",
			"timestamp":  "2026-04-20T19:09:00.000Z",
			"message": map[string]any{
				"role":    "user",
				"content": "",
			},
		},
		{
			"type":       "user",
			"uuid":       "transcript-user-2",
			"sessionId":  sessionID,
			"parentUuid": "transcript-user-empty",
			"timestamp":  "2026-04-20T19:09:10.000Z",
			"message": map[string]any{
				"role":    "user",
				"content": "不是吧",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "transcript-assistant-2",
			"sessionId":  sessionID,
			"parentUuid": "transcript-user-2",
			"message": map[string]any{
				"role":        "assistant",
				"stop_reason": "end_turn",
				"content": []map[string]any{
					{"type": "text", "text": "哈哈，你说得对！看工作区名字，我应该叫 Amy 才对 😊"},
				},
			},
		},
	})

	rows, err := history.ReadMessages(workspacePath, protocol.Session{
		SessionKey: sessionKey,
		AgentID:    "Amy",
		SessionID:  &sessionID,
		Options:    map[string]any{},
	}, nil)
	if err != nil {
		t.Fatalf("读取历史消息失败: %v", err)
	}

	if len(rows) != 4 {
		t.Fatalf("历史消息数量不正确: got=%d want=4 rows=%+v", len(rows), rows)
	}

	roundTwoAssistants := 0
	foundRoundTwoUser := false
	for _, row := range rows {
		if stringFromAny(row["round_id"]) != "round-2" {
			continue
		}
		if stringFromAny(row["role"]) == "user" {
			foundRoundTwoUser = true
			if got := stringFromAny(row["source_round_id"]); got != "round-source-2" {
				t.Fatalf("transcript user 丢失 source_round_id: got=%q row=%+v", got, row)
			}
			continue
		}
		if stringFromAny(row["role"]) != "assistant" {
			continue
		}
		roundTwoAssistants++
		if got := stringFromAny(row["message_id"]); got != "transcript-assistant-2" {
			t.Fatalf("第二轮 assistant 不应退化为 synthetic assistant: %+v", row)
		}
		summary, ok := row["result_summary"].(map[string]any)
		if !ok {
			t.Fatalf("第二轮 assistant 应挂载 result_summary: %+v", row)
		}
		if stringFromAny(summary["subtype"]) != "success" {
			t.Fatalf("第二轮 result_summary subtype 不正确: %+v", summary)
		}
	}

	if roundTwoAssistants != 1 {
		t.Fatalf("第二轮 assistant 数量不正确，说明 result 没有并回同一轮: got=%d rows=%+v", roundTwoAssistants, rows)
	}
	if !foundRoundTwoUser {
		t.Fatalf("未找到 transcript 第二轮 user: %+v", rows)
	}
}

func TestAgentHistoryStoreKeepsSlashRoundsOrderedAndHidesLegacySkillPrompt(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := filepath.Join(configRoot, "workspace")
	workspacePath := filepath.Join(workspaceRoot, "nexus")
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatalf("创建 workspace 失败: %v", err)
	}
	t.Setenv("NEXUS_STATE_ROOT", "")
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(configRoot, "home"))

	history := NewAgentHistoryStore(workspaceRoot)
	sessionKey := "agent:nexus:ws:dm:slash-skill"
	sessionID := "slash-skill-session"
	const oldMarkerTimestamp = int64(1785418576650)
	const currentMarkerTimestamp = int64(1785422116066)
	if err := history.AppendRoundMarker(
		workspacePath,
		sessionKey,
		"round-old",
		"/wechat-article-search nexus",
		oldMarkerTimestamp,
	); err != nil {
		t.Fatalf("写入旧 Slash marker 失败: %v", err)
	}
	if err := history.AppendRoundMarker(
		workspacePath,
		sessionKey,
		"round-current",
		"/wechat-article-search nexus 多智能体平台",
		currentMarkerTimestamp,
	); err != nil {
		t.Fatalf("写入当前 Slash marker 失败: %v", err)
	}

	writeAgentTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "old-caveat",
			"sessionId": sessionID,
			"timestamp": "2026-07-30T13:36:16.661541Z",
			"isMeta":    true,
			"message": map[string]any{
				"role":    "user",
				"content": "<local-command-caveat>internal</local-command-caveat>",
			},
		},
		{
			"type":       "user",
			"uuid":       "old-command",
			"parentUuid": "old-caveat",
			"sessionId":  sessionID,
			"timestamp":  "2026-07-30T13:36:16.661551Z",
			"message": map[string]any{
				"role": "user",
				"content": `<command-name>/wechat-article-search</command-name>
<command-message>wechat-article-search</command-message>
<command-args>nexus</command-args>`,
			},
		},
		{
			"type":       "user",
			"uuid":       "old-unknown",
			"parentUuid": "old-command",
			"sessionId":  sessionID,
			"timestamp":  "2026-07-30T13:36:16.661556Z",
			"message": map[string]any{
				"role":    "user",
				"content": "Unknown skill: wechat-article-search",
			},
		},
		{
			"type":       "user",
			"uuid":       "current-internal-skill",
			"parentUuid": "old-unknown",
			"sessionId":  sessionID,
			"timestamp":  "2026-07-30T14:35:16.080918Z",
			"message": map[string]any{
				"role": "user",
				"content": `<system-reminder>
The following is runtime-internal context for the next turn.
<internal_context source="explicit_skill">
完整 Skill 正文不应展示。
</internal_context>
</system-reminder>`,
			},
		},
		{
			"type":       "user",
			"uuid":       "skill-listing",
			"parentUuid": "current-internal-skill",
			"sessionId":  sessionID,
			"timestamp":  "2026-07-30T14:35:16.240029Z",
			"isMeta":     true,
			"message": map[string]any{
				"role":    "user",
				"content": "runtime skill listing",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "current-assistant",
			"parentUuid": "skill-listing",
			"sessionId":  sessionID,
			"timestamp":  "2026-07-30T14:35:40.000000Z",
			"message": map[string]any{
				"role":        "assistant",
				"stop_reason": "end_turn",
				"content": []map[string]any{
					{"type": "text", "text": "搜到 10 篇相关文章。"},
				},
			},
		},
	})

	rows, err := history.ReadMessages(workspacePath, protocol.Session{
		SessionKey: sessionKey,
		AgentID:    "nexus",
		SessionID:  &sessionID,
		Options:    map[string]any{},
	}, nil)
	if err != nil {
		t.Fatalf("读取 Slash Skill 历史失败: %v", err)
	}

	var orderedRounds []string
	for _, row := range rows {
		content := stringFromAny(row["content"])
		if strings.Contains(content, "完整 Skill 正文") ||
			strings.Contains(content, "runtime skill listing") ||
			strings.Contains(content, "Unknown skill:") {
			t.Fatalf("runtime 内部消息不应进入可见历史: %+v", row)
		}
		if stringFromAny(row["role"]) == "user" {
			orderedRounds = append(orderedRounds, stringFromAny(row["round_id"]))
		}
		if row["message_id"] == "current-assistant" &&
			row["round_id"] != "round-current" {
			t.Fatalf("当前 assistant 绑定到错误 round: %+v", row)
		}
	}
	if got, want := strings.Join(orderedRounds, ","), "round-old,round-current"; got != want {
		t.Fatalf("刷新后的 Slash round 顺序错误: got=%q want=%q rows=%+v", got, want, rows)
	}
}

func TestAgentHistoryStoreOnlyShowsAdmittedEchoAssistant(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := filepath.Join(configRoot, "workspace")
	workspacePath := filepath.Join(workspaceRoot, "Nexus")
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatalf("创建 workspace 失败: %v", err)
	}
	t.Setenv("NEXUS_STATE_ROOT", "")
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(configRoot, "home"))

	history := NewAgentHistoryStore(workspaceRoot)
	sessionKey := "agent:nexus:ws:dm:echo-history"
	sessionID := "echo-history-session"
	if err := history.AppendRoundMarkerWithOptions(
		workspacePath,
		sessionKey,
		"round-echo",
		"echo prompt",
		1000,
		RoundMarkerOptions{
			HiddenFromUser: true,
			Synthetic:      true,
			Purpose:        "echo_followup",
			Metadata: map[string]string{
				"echo_attempt_id": "echo-1",
			},
		},
	); err != nil {
		t.Fatalf("写入 Echo round marker 失败: %v", err)
	}

	writeAgentTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
		{
			"type":             "user",
			"uuid":             "transcript-user-echo",
			"sessionId":        sessionID,
			"timestamp":        "2026-08-21T10:00:00.000Z",
			"hidden_from_user": true,
			"is_synthetic":     true,
			"purpose":          "echo_followup",
			"message": map[string]any{
				"role":    "user",
				"content": "<system-reminder>internal</system-reminder>\n\necho prompt",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "transcript-assistant-echo",
			"sessionId":  sessionID,
			"parentUuid": "transcript-user-echo",
			"message": map[string]any{
				"id":          "echo-assistant",
				"role":        "assistant",
				"stop_reason": "end_turn",
				"content": []map[string]any{
					{"type": "text", "text": "主动跟进正文"},
				},
			},
		},
	})

	session := protocol.Session{
		SessionKey: sessionKey,
		AgentID:    "nexus",
		SessionID:  &sessionID,
		Options:    map[string]any{},
	}
	rows, err := history.ReadMessages(workspacePath, session, nil)
	if err != nil {
		t.Fatalf("读取 Echo 历史失败: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("Echo 原始输入和未准入输出都不应进入可见历史: %+v", rows)
	}

	if err = history.AppendOverlayMessage(workspacePath, sessionKey, protocol.Message{
		"message_id":  "echo-assistant",
		"session_key": sessionKey,
		"agent_id":    "nexus",
		"round_id":    "round-echo",
		"role":        "assistant",
		"content": []map[string]any{
			{"type": "text", "text": "主动跟进正文"},
		},
		"metadata": map[string]any{
			"source":          "echo",
			"echo_attempt_id": "echo-1",
		},
		"timestamp":   int64(2000),
		"stop_reason": "end_turn",
		"is_complete": true,
	}); err != nil {
		t.Fatalf("写入准入后的 Echo assistant 失败: %v", err)
	}

	rows, err = history.ReadMessages(workspacePath, session, nil)
	if err != nil {
		t.Fatalf("重新读取 Echo 历史失败: %v", err)
	}
	if len(rows) != 1 || rows[0]["role"] != "assistant" || rows[0]["round_id"] != "round-echo" {
		t.Fatalf("只应展示准入后的 Echo assistant: %+v", rows)
	}
	metadata, _ := rows[0]["metadata"].(map[string]any)
	if metadata["source"] != "echo" {
		t.Fatalf("Echo 来源信息未持久化: %+v", rows[0])
	}
}

func TestAgentHistoryStoreRestoresMixedTranscriptRoundsWithoutLosingUsers(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := filepath.Join(configRoot, "workspace")
	workspacePath := filepath.Join(workspaceRoot, "Kevin")
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatalf("创建 workspace 失败: %v", err)
	}
	t.Setenv("NEXUS_STATE_ROOT", "")
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(configRoot, "home"))

	history := NewAgentHistoryStore(workspaceRoot)
	sessionKey := "agent:kevin:ws:dm:mixed-history"
	sessionID := "mixed-history-session"
	if err := history.AppendRoundMarker(
		workspacePath,
		sessionKey,
		"round-before-memory",
		"创建一个文件",
		1000,
	); err != nil {
		t.Fatalf("写入 memory 前 marker 失败: %v", err)
	}
	if err := history.AppendRoundMarkerWithOptions(
		workspacePath,
		sessionKey,
		"goal-continuation",
		"",
		3000,
		RoundMarkerOptions{
			HiddenFromUser: true,
			Synthetic:      true,
			Purpose:        "goal_continuation",
		},
	); err != nil {
		t.Fatalf("写入隐藏 Goal marker 失败: %v", err)
	}
	if err := history.AppendRoundMarker(
		workspacePath,
		sessionKey,
		"round-after-goal",
		"再设定一个 Goal",
		5000,
	); err != nil {
		t.Fatalf("写入 Goal 后 marker 失败: %v", err)
	}

	writeAgentTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "user-before-memory",
			"sessionId": sessionID,
			"timestamp": "1970-01-01T00:00:01.005Z",
			"message": map[string]any{
				"role": "user",
				"content": "创建一个文件\n\n<nexus_runtime_context>\n" +
					"## Emotion State\nfocused\n</nexus_runtime_context>",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "assistant-before-memory",
			"sessionId":  sessionID,
			"parentUuid": "user-before-memory",
			"timestamp":  "1970-01-01T00:00:01.500Z",
			"message": map[string]any{
				"role":    "assistant",
				"content": []map[string]any{{"type": "text", "text": "文件已创建"}},
			},
		},
		{
			"type":       "system",
			"uuid":       "runtime-memory-saved",
			"sessionId":  sessionID,
			"parentUuid": "assistant-before-memory",
			"timestamp":  "1970-01-01T00:00:02.000Z",
			"subtype":    "memory_saved",
		},
		{
			"type":       "system",
			"uuid":       "runtime-memory-saved",
			"sessionId":  sessionID,
			"parentUuid": "runtime-memory-saved",
			"timestamp":  "1970-01-01T00:00:02.001Z",
			"subtype":    "memory_saved",
		},
		{
			"type":       "user",
			"uuid":       "goal-user",
			"sessionId":  sessionID,
			"parentUuid": "runtime-memory-saved",
			"timestamp":  "1970-01-01T00:00:03.005Z",
			"message": map[string]any{
				"role": "user",
				"content": "<internal_context source=\"goal\">\n" +
					"Continue working toward the active thread goal.\n" +
					"</internal_context>",
			},
		},
		{
			"type":       "user",
			"uuid":       "runtime-skill-listing",
			"sessionId":  sessionID,
			"parentUuid": "goal-user",
			"timestamp":  "1970-01-01T00:00:03.006Z",
			"isMeta":     true,
			"message": map[string]any{
				"role":    "user",
				"content": "<system-reminder>Skill listing</system-reminder>",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "goal-assistant",
			"sessionId":  sessionID,
			"parentUuid": "runtime-skill-listing",
			"timestamp":  "1970-01-01T00:00:04.000Z",
			"message": map[string]any{
				"role":    "assistant",
				"content": []map[string]any{{"type": "text", "text": "Goal 继续执行"}},
			},
		},
		{
			"type":       "user",
			"uuid":       "user-after-goal",
			"sessionId":  sessionID,
			"parentUuid": "goal-assistant",
			"timestamp":  "1970-01-01T00:00:05.005Z",
			"message": map[string]any{
				"role": "user",
				"content": "再设定一个 Goal\n\n<nexus_runtime_context>\n" +
					"## Emotion State\nfocused\n</nexus_runtime_context>",
			},
		},
		{
			"type":       "user",
			"uuid":       "runtime-skill-body",
			"sessionId":  sessionID,
			"parentUuid": "user-after-goal",
			"timestamp":  "1970-01-01T00:00:05.006Z",
			"isMeta":     true,
			"message": map[string]any{
				"role":    "user",
				"content": "Base directory for this skill: /tmp/goal-manager",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "assistant-after-goal",
			"sessionId":  sessionID,
			"parentUuid": "runtime-skill-body",
			"timestamp":  "1970-01-01T00:00:06.000Z",
			"message": map[string]any{
				"role":    "assistant",
				"content": []map[string]any{{"type": "text", "text": "新 Goal 已创建"}},
			},
		},
	})

	rows, err := history.ReadMessages(workspacePath, protocol.Session{
		SessionKey: sessionKey,
		AgentID:    "Kevin",
		SessionID:  &sessionID,
		Options:    map[string]any{},
	}, nil)
	if err != nil {
		t.Fatalf("读取混合历史失败: %v", err)
	}

	expectedUsers := map[string]string{
		"round-before-memory": "创建一个文件",
		"round-after-goal":    "再设定一个 Goal",
	}
	expectedAssistants := map[string]string{
		"round-before-memory": "文件已创建",
		"goal-continuation":   "Goal 继续执行",
		"round-after-goal":    "新 Goal 已创建",
	}
	for _, row := range rows {
		roundID := stringFromAny(row["round_id"])
		switch stringFromAny(row["role"]) {
		case "user":
			expected, ok := expectedUsers[roundID]
			if !ok {
				t.Fatalf("内部 Goal 或 meta 输入不应显示为用户消息: %+v", row)
			}
			if got := stringFromAny(row["content"]); got != expected {
				t.Fatalf("用户消息应移除 runtime context: got=%q want=%q", got, expected)
			}
			delete(expectedUsers, roundID)
		case "assistant":
			expected, ok := expectedAssistants[roundID]
			if !ok {
				continue
			}
			if !messageContentContainsText(row["content"], expected) {
				t.Fatalf("assistant 归属轮次错误: round=%s row=%+v", roundID, row)
			}
			delete(expectedAssistants, roundID)
		}
	}
	if len(expectedUsers) != 0 || len(expectedAssistants) != 0 {
		t.Fatalf(
			"混合历史恢复不完整: missing_users=%+v missing_assistants=%+v rows=%+v",
			expectedUsers,
			expectedAssistants,
			rows,
		)
	}
}

func TestAgentHistoryStoreRestoresVisibleMarkerWhenTranscriptUserIsMissing(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := filepath.Join(configRoot, "workspace")
	workspacePath := filepath.Join(workspaceRoot, "Kevin")
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatalf("创建 workspace 失败: %v", err)
	}
	t.Setenv("NEXUS_STATE_ROOT", "")
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(configRoot, "home"))

	history := NewAgentHistoryStore(workspaceRoot)
	sessionKey := "agent:kevin:ws:dm:missing-user"
	sessionID := "missing-user-session"
	if err := history.AppendRoundMarker(
		workspacePath,
		sessionKey,
		"round-visible",
		"仍应显示的用户输入",
		1000,
	); err != nil {
		t.Fatalf("写入可见 marker 失败: %v", err)
	}
	writeAgentTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
		{
			"type":      "assistant",
			"uuid":      "assistant-without-user",
			"sessionId": sessionID,
			"timestamp": "1970-01-01T00:00:02.000Z",
			"message": map[string]any{
				"role":    "assistant",
				"content": []map[string]any{{"type": "text", "text": "只有 assistant 的坏链"}},
			},
		},
	})

	rows, err := history.ReadMessages(workspacePath, protocol.Session{
		SessionKey: sessionKey,
		AgentID:    "Kevin",
		SessionID:  &sessionID,
		Options:    map[string]any{},
	}, nil)
	if err != nil {
		t.Fatalf("读取缺失 user 的历史失败: %v", err)
	}
	for _, row := range rows {
		if stringFromAny(row["role"]) == "user" &&
			stringFromAny(row["round_id"]) == "round-visible" &&
			stringFromAny(row["content"]) == "仍应显示的用户输入" {
			return
		}
	}
	t.Fatalf("transcript 断链时必须由可见 marker 恢复用户消息: %+v", rows)
}

func messageContentContainsText(content any, expected string) bool {
	for _, block := range normalizeMessageContentBlocks(content) {
		if stringFromAny(block["text"]) == expected {
			return true
		}
	}
	return false
}

func writeAgentTranscriptFixture(t *testing.T, workspacePath string, sessionID string, rows []map[string]any) {
	t.Helper()

	projectDir := filepath.Join(
		transcriptProjectsDirForWorkspace(workspacePath),
		sanitizeTranscriptPath(canonicalizeTranscriptPath(workspacePath)),
	)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("创建 transcript 项目目录失败: %v", err)
	}
	transcriptPath := filepath.Join(projectDir, sessionID+".jsonl")

	file, err := os.Create(transcriptPath)
	if err != nil {
		t.Fatalf("创建 transcript fixture 失败: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, row := range rows {
		if err := encoder.Encode(row); err != nil {
			t.Fatalf("写入 transcript fixture 失败: %v", err)
		}
	}
}

// 自动压缩后 compact_boundary 的 parentUuid 为 null；主链必须沿 logicalParentUuid
// 续接压缩前的历史，压缩摘要不能成为用户轮次，但要挂回边界供时间线展开。
func TestPrimaryTranscriptChainCrossesCompactBoundary(t *testing.T) {
	entries := []transcriptEntry{
		{Index: 0, Data: map[string]any{
			"type": "user", "uuid": "user-1", "timestamp": "2026-11-19T01:00:00Z",
			"message": map[string]any{"role": "user", "content": "生成 PPT"},
		}},
		{Index: 1, Data: map[string]any{
			"type": "assistant", "uuid": "assistant-1", "parentUuid": "user-1",
			"message": map[string]any{"id": "msg-1", "role": "assistant", "content": []any{
				map[string]any{"type": "text", "text": "请打开 127.0.0.1 确认"},
			}},
		}},
		{Index: 2, Data: map[string]any{
			"type": "system", "subtype": "compact_boundary", "uuid": "boundary",
			"parentUuid": nil, "logicalParentUuid": "assistant-1",
			"content": "Conversation compacted",
		}},
		{Index: 3, Data: map[string]any{
			"type": "user", "uuid": "summary", "parentUuid": "boundary", "isCompactSummary": true,
			"isVisibleInTranscriptOnly": true,
			"message":                   map[string]any{"role": "user", "content": "This session is being continued from a previous conversation..."},
		}},
		{Index: 4, Data: map[string]any{
			"type": "assistant", "uuid": "assistant-2", "parentUuid": "summary",
			"message": map[string]any{"id": "msg-2", "role": "assistant", "content": []any{
				map[string]any{"type": "text", "text": "PPT 已完成"},
			}},
		}},
	}

	chain := buildPrimaryTranscriptChain(entries)
	uuids := make([]string, 0, len(chain))
	for _, entry := range chain {
		uuids = append(uuids, stringFromAny(entry.Data["uuid"]))
	}
	if got := strings.Join(uuids, ","); got != "user-1,assistant-1,boundary,summary,assistant-2" {
		t.Fatalf("chain = %s, 压缩前历史被截断", got)
	}

	turns := collectTranscriptUserTurns(chain, shouldSkipTranscriptEntry)
	if len(turns) != 1 || turns[0].Content != transcriptUserContentKey("生成 PPT") {
		t.Fatalf("user turns = %+v, 压缩摘要不应成为用户轮次", turns)
	}

	rows := projectTranscriptChain("/workspace", "agent:a:ws:dm:c", "a", chain, nil)
	var texts []string
	var boundarySummary string
	for _, row := range rows {
		if metadata, _ := row["metadata"].(map[string]any); metadata["subtype"] == "compact_boundary" {
			boundarySummary = stringFromAny(metadata["summary"])
		}
		if row["role"] == "user" && strings.Contains(stringFromAny(row["content"]), "continued from a previous") {
			t.Fatalf("压缩摘要被投影为用户消息: %+v", row)
		}
		for _, block := range normalizeMessageContentBlocks(row["content"]) {
			if block["type"] == "text" {
				texts = append(texts, stringFromAny(block["text"]))
			}
		}
	}
	if !strings.Contains(boundarySummary, "continued from a previous") {
		t.Fatalf("compact_boundary summary = %q, 历史标记缺少摘要", boundarySummary)
	}
	if stringFromAny(entries[2].Data["summary"]) != "" {
		t.Fatal("挂回摘要不能改写原始 transcript 条目")
	}
	joined := strings.Join(texts, "|")
	if !strings.Contains(joined, "127.0.0.1") || !strings.Contains(joined, "PPT 已完成") {
		t.Fatalf("texts = %q, 需要同时保留压缩前后的正文", joined)
	}
}

// transcript 与 overlay 对同一 message_id 的快照合并时，不同正文的 text 块不能互相覆盖。
func TestCompactMessagesKeepsDistinctTextBlocks(t *testing.T) {
	first := protocol.Message{
		"message_id": "msg-1", "role": "assistant", "timestamp": int64(1),
		"content": []any{
			map[string]any{"type": "text", "text": "确认页面已打开：127.0.0.1"},
			map[string]any{"type": "tool_use", "id": "tool-1", "name": "Bash"},
		},
	}
	second := protocol.Message{
		"message_id": "msg-1", "role": "assistant", "timestamp": int64(2),
		"content": []any{
			map[string]any{"type": "text", "text": "确认页面已打开：127.0.0.1"},
			map[string]any{"type": "tool_use", "id": "tool-1", "name": "Bash"},
			map[string]any{"type": "text", "text": "如果页面没打开，也可以直接回复"},
		},
	}

	compacted := compactMessages([]protocol.Message{first, second})
	if len(compacted) != 1 {
		t.Fatalf("compacted = %+v", compacted)
	}
	var texts []string
	for _, block := range normalizeMessageContentBlocks(compacted[0]["content"]) {
		if block["type"] == "text" {
			texts = append(texts, stringFromAny(block["text"]))
		}
	}
	if len(texts) != 2 || !strings.Contains(texts[0], "127.0.0.1") {
		t.Fatalf("texts = %q, 先前正文被后续 text 块覆盖", texts)
	}
}

func TestAgentHistoryStoreEmptyGoalTurnKeepsReportInItsOwnRound(t *testing.T) {
	root := t.TempDir()
	workspaceRoot := filepath.Join(root, "workspace")
	workspacePath := filepath.Join(workspaceRoot, "Nova")
	t.Setenv("NEXUS_STATE_ROOT", "")
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, "runtime"))
	history := NewAgentHistoryStore(workspaceRoot)
	sessionKey := "agent:nova:ws:dm:empty-goal"
	sessionID := "empty-goal-transcript"
	const goalRound = "goal-continuation"
	const report = "M3 report delivered"
	const formula = `Formula rendered: $$K_D = \frac{[A][B]}{[AB]}$$`
	for _, marker := range []struct {
		id      string
		content string
		ts      int64
		options RoundMarkerOptions
	}{
		{"old-round", "Render the formula", 1000, RoundMarkerOptions{}},
		{"goal-command", "/goal Research M3", 86401000, RoundMarkerOptions{ControlOnly: true, Purpose: "goal_command"}},
		{goalRound, "", 86401100, RoundMarkerOptions{HiddenFromUser: true, Synthetic: true, Purpose: "goal_continuation"}},
		{"next-round", "Thanks", 86405000, RoundMarkerOptions{}},
	} {
		if err := history.AppendRoundMarkerWithOptions(workspacePath, sessionKey, marker.id, marker.content, marker.ts, marker.options); err != nil {
			t.Fatal(err)
		}
	}
	writeAgentTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
		{"type": "user", "uuid": "old-user", "sessionId": sessionID, "timestamp": "1970-01-01T00:00:01.005Z", "message": map[string]any{"role": "user", "content": "Render the formula"}},
		{"type": "assistant", "uuid": "old-answer", "parentUuid": "old-user", "sessionId": sessionID, "timestamp": "1970-01-01T00:00:02Z", "message": map[string]any{"role": "assistant", "stop_reason": "end_turn", "content": []map[string]any{{"type": "text", "text": formula}}}},
		// nxs extracts the hidden Goal reminder before writing this empty user turn.
		{"type": "user", "uuid": "goal-user", "parentUuid": "old-answer", "sessionId": sessionID, "timestamp": "1970-01-02T00:00:01.105Z", "message": map[string]any{"role": "user", "content": ""}},
		{"type": "assistant", "uuid": "goal-progress", "parentUuid": "goal-user", "sessionId": sessionID, "timestamp": "1970-01-02T00:00:02Z", "message": map[string]any{"role": "assistant", "content": []map[string]any{{"type": "text", "text": "Researching M3"}}}},
		{"type": "assistant", "uuid": "goal-answer", "parentUuid": "goal-progress", "sessionId": sessionID, "timestamp": "1970-01-02T00:00:03Z", "message": map[string]any{"role": "assistant", "stop_reason": "end_turn", "content": []map[string]any{{"type": "text", "text": report}}}},
		{"type": "user", "uuid": "next-user", "parentUuid": "goal-answer", "sessionId": sessionID, "timestamp": "1970-01-02T00:00:05.005Z", "message": map[string]any{"role": "user", "content": "Thanks"}},
		{"type": "assistant", "uuid": "next-answer", "parentUuid": "next-user", "sessionId": sessionID, "timestamp": "1970-01-02T00:00:06Z", "message": map[string]any{"role": "assistant", "stop_reason": "end_turn", "content": []map[string]any{{"type": "text", "text": "Welcome"}}}},
	})
	for _, row := range []protocol.Message{
		{"message_id": "goal-result", "round_id": goalRound, "role": "result", "subtype": "success", "result": report, "timestamp": int64(86404000)},
		{"message_id": "goal-answer", "round_id": goalRound, "role": "assistant", "content": []map[string]any{{"type": "text", "text": report}}, "timestamp": int64(86403000), protocol.GoalCompletionReceiptField: protocol.GoalCompletionReceipt{GoalID: "goal-1", RoundID: goalRound}},
	} {
		if err := history.AppendOverlayMessage(workspacePath, sessionKey, row); err != nil {
			t.Fatal(err)
		}
	}
	session := protocol.Session{SessionKey: sessionKey, AgentID: "Nova", SessionID: &sessionID, Options: map[string]any{}}
	assertRows := func(t *testing.T, rows []protocol.Message) {
		t.Helper()
		reports := 0
		formulas := 0
		for _, row := range rows {
			id := stringFromAny(row["message_id"])
			wantRound := map[string]string{"old-answer": "old-round", "goal-progress": goalRound, "goal-answer": goalRound, "next-answer": "next-round"}[id]
			if wantRound != "" && row["round_id"] != wantRound {
				t.Errorf("%s round = %v, want %s", id, row["round_id"], wantRound)
			}
			if row["role"] == "user" && row["round_id"] == goalRound {
				t.Error("hidden Goal input must not become a visible user message")
			}
			for _, block := range normalizeMessageContentBlocks(row["content"]) {
				if block["text"] == report {
					reports++
				}
				if block["text"] == formula && row["round_id"] == "old-round" {
					formulas++
				}
			}
			if id == "goal-answer" && (row["result_summary"] == nil || row[protocol.GoalCompletionReceiptField] == nil) {
				t.Error("Goal result and completion receipt must attach to the original report")
			}
		}
		if reports != 1 {
			t.Errorf("report copies = %d, want 1", reports)
		}
		if formulas != 1 {
			t.Errorf("formula replies in the original round = %d, want 1", formulas)
		}
	}
	rows, err := history.ReadMessages(workspacePath, session, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertRows(t, rows)
	for range 2 { // Both freshly built and cached pagination must preserve the boundary.
		page, err := history.ReadMessagesPageContext(context.Background(), workspacePath, session, nil, HistoryPageQuery{Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		assertRows(t, page.Items)
	}
	tail, err := history.ResolveTranscriptRoundTail(workspacePath, sessionKey, sessionID, goalRound)
	if err != nil {
		t.Fatal(err)
	}
	if tail.TargetMessageUUID != "goal-user" || tail.TargetRoundEndUUID != "goal-answer" {
		t.Fatalf("Goal rewrite boundary = %+v", tail)
	}
}

func TestEmptyTranscriptGoalMarkerMatchingBoundaries(t *testing.T) {
	goalMarker := transcriptRoundMarker{RoundID: "goal", Timestamp: 86401000, HiddenFromUser: true, Synthetic: true, Purpose: "goal_continuation"}
	for _, tc := range []struct {
		name       string
		timestamps []string
		markers    []transcriptRoundMarker
		wantIndex  int
	}{
		{"empty_goal", []string{"1970-01-02T00:00:01.005Z"}, []transcriptRoundMarker{goalMarker}, 0},
		{"ordinary_empty_without_marker", []string{"1970-01-02T00:00:01.005Z"}, nil, -1},
		{"visible_marker", []string{"1970-01-02T00:00:01.005Z"}, []transcriptRoundMarker{{RoundID: "visible", Timestamp: 86401000}}, -1},
		{"other_hidden_input", []string{"1970-01-02T00:00:01.005Z"}, []transcriptRoundMarker{{RoundID: "echo", Timestamp: 86401000, HiddenFromUser: true, Synthetic: true, Purpose: "echo_followup"}}, -1},
		{"missing_timestamp", []string{""}, []transcriptRoundMarker{goalMarker}, -1},
		{"stale_marker", []string{"1970-01-03T00:00:01.005Z"}, []transcriptRoundMarker{goalMarker}, -1},
		{"future_marker", []string{"1970-01-01T00:00:01.005Z"}, []transcriptRoundMarker{goalMarker}, -1},
		{"ambiguous_markers", []string{"1970-01-02T00:00:01.005Z"}, []transcriptRoundMarker{goalMarker, goalMarker}, -1},
		{"incidental_empty_before_goal", []string{"1970-01-02T00:00:00Z", "1970-01-02T00:00:01.005Z"}, []transcriptRoundMarker{goalMarker}, 1},
		{"ambiguous_empty_inputs", []string{"1970-01-02T00:00:01.005Z", "1970-01-02T00:00:01.005Z"}, []transcriptRoundMarker{goalMarker}, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chain := make([]transcriptEntry, len(tc.timestamps))
			for i, timestamp := range tc.timestamps {
				chain[i] = transcriptEntry{Index: i, Data: map[string]any{"type": "user", "timestamp": timestamp, "message": map[string]any{"role": "user", "content": ""}}}
			}
			aligned := alignTranscriptRoundMarkers(chain, tc.markers)
			for i := range chain {
				got := i < len(aligned) && transcriptRoundMarkerPresent(aligned[i])
				if got != (i == tc.wantIndex) {
					t.Errorf("empty input %d matched = %v, want %v", i, got, i == tc.wantIndex)
				}
			}
		})
	}
}

func TestAgentHistoryStoreProjectsHookAdditionalContextGuidance(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := filepath.Join(configRoot, "workspace")
	workspacePath := filepath.Join(workspaceRoot, "Amy")
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatalf("创建 workspace 失败: %v", err)
	}
	t.Setenv("NEXUS_STATE_ROOT", "")
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(configRoot, "home"))

	history := NewAgentHistoryStore(workspaceRoot)
	sessionKey := "agent:c5740009ac97:ws:dm:a731e54f7af5"
	sessionID := "4035f197-ca97-43fc-b9ae-06ac04903213"
	if err := history.AppendRoundMarker(workspacePath, sessionKey, "round-1", "写一个五子棋游戏", 1000); err != nil {
		t.Fatalf("写入 round marker 失败: %v", err)
	}

	writeAgentTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "transcript-user-1",
			"sessionId": sessionID,
			"timestamp": "2026-04-27T11:40:00.000Z",
			"message": map[string]any{
				"role":    "user",
				"content": "写一个五子棋游戏",
			},
		},
		{
			"type":       "attachment",
			"uuid":       "transcript-guidance-1",
			"sessionId":  sessionID,
			"parentUuid": "transcript-user-1",
			"timestamp":  "2026-04-27T11:41:00.000Z",
			"attachment": map[string]any{
				"type": "hook_additional_context",
				"content": []string{
					"<nexus_guidance>\n用户在你执行当前 round 时补充了以下引导。请在继续下一步前结合这些要求；如果与原任务冲突，以最新引导为准。\n1. round_id=queue_guide_1: 需要可以与 bot 对战\n</nexus_guidance>",
				},
			},
		},
	})

	rows, err := history.ReadMessages(workspacePath, protocol.Session{
		SessionKey: sessionKey,
		AgentID:    "Amy",
		SessionID:  &sessionID,
		Options:    map[string]any{},
	}, nil)
	if err != nil {
		t.Fatalf("读取历史失败: %v", err)
	}

	var guidance *protocol.Message
	for index := range rows {
		if rows[index]["message_id"] == "queue_guide_1" {
			guidance = &rows[index]
			break
		}
	}
	if guidance == nil {
		t.Fatalf("runtime hook additionalContext 应投影成引导系统消息: %+v", rows)
	}
	if (*guidance)["role"] != "system" || (*guidance)["round_id"] != "round-1" {
		t.Fatalf("引导系统消息应归入当前 round: %+v", *guidance)
	}
	if stringFromAny((*guidance)["content"]) != "需要可以与 bot 对战" {
		t.Fatalf("引导内容解析错误: %+v", *guidance)
	}
	metadata, _ := (*guidance)["metadata"].(map[string]any)
	if metadata["subtype"] != message.SystemMessageSubtypeGuidedInput ||
		metadata["source_round_id"] != "queue_guide_1" {
		t.Fatalf("引导 metadata 不正确: %+v", *guidance)
	}
}

func TestAgentHistoryStoreProjectsWorkspaceFileArtifactFromTranscriptToolResult(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := filepath.Join(configRoot, "workspace")
	workspacePath := filepath.Join(workspaceRoot, "Amy")
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatalf("创建 workspace 失败: %v", err)
	}
	t.Setenv("NEXUS_STATE_ROOT", "")
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(configRoot, "home"))

	history := NewAgentHistoryStore(workspaceRoot)
	sessionKey := "agent:c5740009ac97:ws:dm:a731e54f7af5"
	sessionID := "6a855871-4614-4032-b0b3-62d2358fad99"
	if err := history.AppendRoundMarker(workspacePath, sessionKey, "round-game", "写一个简单的游戏，html的", 1000); err != nil {
		t.Fatalf("写入 round marker 失败: %v", err)
	}

	targetPath := filepath.Join(workspacePath, "snake-game.html")
	writeAgentTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "transcript-user-game",
			"sessionId": sessionID,
			"timestamp": "2026-05-20T01:07:51.493Z",
			"message": map[string]any{
				"role":    "user",
				"content": "写一个简单的游戏，html的",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "transcript-assistant-write",
			"sessionId":  sessionID,
			"parentUuid": "transcript-user-game",
			"timestamp":  "2026-05-20T01:08:39.805Z",
			"message": map[string]any{
				"id":          "msg_game_write",
				"type":        "message",
				"role":        "assistant",
				"stop_reason": "tool_use",
				"content": []map[string]any{
					{
						"type": "text",
						"text": "好的，我来写一个 HTML 游戏。",
					},
					{
						"type": "tool_use",
						"id":   "tool-write-game",
						"name": "Write",
						"input": map[string]any{
							"file_path": targetPath,
							"content":   "<!doctype html><html><body>game</body></html>",
						},
					},
				},
			},
		},
		{
			"type":       "user",
			"uuid":       "transcript-tool-result",
			"sessionId":  sessionID,
			"parentUuid": "transcript-assistant-write",
			"timestamp":  "2026-05-20T01:08:39.850Z",
			"message": map[string]any{
				"role": "user",
				"content": []map[string]any{
					{
						"type":        "tool_result",
						"tool_use_id": "tool-write-game",
						"content":     "File created successfully at: " + targetPath,
					},
				},
			},
			"toolUseResult": map[string]any{
				"type":     "create",
				"filePath": targetPath,
			},
		},
		{
			"type":       "assistant",
			"uuid":       "transcript-assistant-final",
			"sessionId":  sessionID,
			"parentUuid": "transcript-tool-result",
			"timestamp":  "2026-05-20T01:08:47.315Z",
			"message": map[string]any{
				"id":          "msg_game_final",
				"type":        "message",
				"role":        "assistant",
				"stop_reason": "end_turn",
				"content": []map[string]any{
					{
						"type": "text",
						"text": "游戏写好了，文件在 `snake-game.html`。",
					},
				},
			},
		},
	})

	rows, err := history.ReadMessages(workspacePath, protocol.Session{
		SessionKey: sessionKey,
		AgentID:    "Amy",
		SessionID:  &sessionID,
		Options:    map[string]any{},
	}, nil)
	if err != nil {
		t.Fatalf("读取历史失败: %v", err)
	}

	for _, row := range rows {
		blocks, _ := row["content"].([]map[string]any)
		for _, block := range blocks {
			if block["type"] != protocol.ContentBlockTypeWorkspaceFileArtifact {
				continue
			}
			if block["path"] != "snake-game.html" || block["source_tool_use_id"] != "tool-write-game" {
				t.Fatalf("workspace file artifact 内容不正确: %+v", block)
			}
			return
		}
	}
	t.Fatalf("transcript tool_result 应投影出 workspace_file_artifact: %+v", rows)
}

func TestAgentHistoryStorePreservesParallelToolResultsFromTranscriptBranches(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := filepath.Join(configRoot, "workspace")
	workspacePath := filepath.Join(workspaceRoot, "Amy")
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatalf("创建 workspace 失败: %v", err)
	}
	t.Setenv("NEXUS_STATE_ROOT", "")
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(configRoot, "home"))

	history := NewAgentHistoryStore(workspaceRoot)
	sessionKey := "agent:c5740009ac97:ws:dm:parallel-tools"
	sessionID := "d758d942-aced-4952-a2cb-ff2835e22cfc"
	if err := history.AppendRoundMarker(workspacePath, sessionKey, "round-parallel", "再试一下刚才两个工具", 1000); err != nil {
		t.Fatalf("写入 round marker 失败: %v", err)
	}

	writeAgentTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "transcript-user-parallel",
			"sessionId": sessionID,
			"timestamp": "2026-06-02T10:13:43.870Z",
			"message": map[string]any{
				"role":    "user",
				"content": "再试一下刚才两个工具",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "assistant-tool-connectors",
			"sessionId":  sessionID,
			"parentUuid": "transcript-user-parallel",
			"timestamp":  "2026-06-02T10:13:48.717Z",
			"message": map[string]any{
				"id":          "msg_parallel_tools",
				"type":        "message",
				"role":        "assistant",
				"model":       "glm-5.1",
				"stop_reason": "tool_use",
				"content": []map[string]any{
					{
						"type":  "tool_use",
						"id":    "call-connectors",
						"name":  "mcp__nexus_feishu_docx__search",
						"input": map[string]any{},
					},
				},
			},
		},
		{
			"type":       "assistant",
			"uuid":       "assistant-tool-automation",
			"sessionId":  sessionID,
			"parentUuid": "assistant-tool-connectors",
			"timestamp":  "2026-06-02T10:13:48.722Z",
			"message": map[string]any{
				"id":          "msg_parallel_tools",
				"type":        "message",
				"role":        "assistant",
				"model":       "glm-5.1",
				"stop_reason": "tool_use",
				"content": []map[string]any{
					{
						"type":  "tool_use",
						"id":    "call-automation",
						"name":  "mcp__nexus_automation__automation_query",
						"input": map[string]any{"operation": "list"},
					},
				},
			},
		},
		{
			"type":       "user",
			"uuid":       "tool-result-connectors",
			"sessionId":  sessionID,
			"parentUuid": "assistant-tool-connectors",
			"timestamp":  "2026-06-02T10:14:19.799Z",
			"message": map[string]any{
				"role": "user",
				"content": []map[string]any{
					{
						"type":        "tool_result",
						"tool_use_id": "call-connectors",
						"content": []map[string]any{
							{"type": "text", "text": "[]"},
						},
					},
				},
			},
		},
		{
			"type":       "user",
			"uuid":       "tool-result-automation",
			"sessionId":  sessionID,
			"parentUuid": "assistant-tool-automation",
			"timestamp":  "2026-06-02T10:14:20.927Z",
			"message": map[string]any{
				"role": "user",
				"content": []map[string]any{
					{
						"type":        "tool_result",
						"tool_use_id": "call-automation",
						"content": []map[string]any{
							{"type": "text", "text": "[]"},
						},
					},
				},
			},
		},
		{
			"type":       "assistant",
			"uuid":       "assistant-final",
			"sessionId":  sessionID,
			"parentUuid": "tool-result-automation",
			"timestamp":  "2026-06-02T10:14:31.746Z",
			"message": map[string]any{
				"id":          "msg_parallel_final",
				"type":        "message",
				"role":        "assistant",
				"model":       "glm-5.1",
				"stop_reason": "end_turn",
				"content": []map[string]any{
					{"type": "text", "text": "两个工具调用都正常返回。"},
				},
			},
		},
	})

	rows, err := history.ReadMessages(workspacePath, protocol.Session{
		SessionKey: sessionKey,
		AgentID:    "Amy",
		SessionID:  &sessionID,
		Options:    map[string]any{},
	}, nil)
	if err != nil {
		t.Fatalf("读取历史失败: %v", err)
	}

	var toolAssistant protocol.Message
	for _, row := range rows {
		if row["message_id"] == "msg_parallel_tools" {
			toolAssistant = row
			break
		}
	}
	if toolAssistant == nil {
		t.Fatalf("历史缺少并行工具 assistant: %+v", rows)
	}

	blocks, _ := toolAssistant["content"].([]map[string]any)
	toolUseIDs := make(map[string]struct{})
	toolResultIDs := make(map[string]struct{})
	for _, block := range blocks {
		switch block["type"] {
		case "tool_use":
			toolUseIDs[stringFromAny(block["id"])] = struct{}{}
		case "tool_result":
			toolResultIDs[stringFromAny(block["tool_use_id"])] = struct{}{}
		}
	}
	for _, id := range []string{"call-connectors", "call-automation"} {
		if _, exists := toolUseIDs[id]; !exists {
			t.Fatalf("历史缺少 tool_use %s: %+v", id, blocks)
		}
		if _, exists := toolResultIDs[id]; !exists {
			t.Fatalf("历史缺少 tool_result %s: %+v", id, blocks)
		}
	}
}

func TestTranscriptProjectionCheckpointMatchesFullProjection(t *testing.T) {
	// 无 marker 时 marker 前缀恒等，分叉只能由主链前缀比对发现。
	for _, withMarkers := range []bool{true, false} {
		t.Run(fmt.Sprintf("markers=%v", withMarkers), func(t *testing.T) {
			testTranscriptProjectionCheckpoint(t, withMarkers)
		})
	}
}

func testTranscriptProjectionCheckpoint(t *testing.T, withMarkers bool) {
	f := newLongSessionFixture(t, 0, 64)
	sessionID := *f.session.SessionID
	markers := []transcriptRoundMarker{}
	appendRows := func(rows ...map[string]any) {
		t.Helper()
		file, err := os.OpenFile(f.transcript, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		encoder := json.NewEncoder(file)
		for _, row := range rows {
			if err := encoder.Encode(row); err != nil {
				t.Fatal(err)
			}
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
	userRow := func(round int, parent string) map[string]any {
		timestamp := int64(1_700_000_000_000 + round*10_000)
		if withMarkers {
			markers = append(markers, transcriptRoundMarker{RoundID: fmt.Sprintf("round-%d", round), Content: fmt.Sprintf("q%d", round), Timestamp: timestamp})
		}
		return map[string]any{"type": "user", "uuid": fmt.Sprintf("u-%d", round), "parentUuid": nilIfEmpty(parent), "sessionId": sessionID,
			"timestamp": time.UnixMilli(timestamp).UTC().Format(time.RFC3339Nano), "message": map[string]any{"role": "user", "content": fmt.Sprintf("q%d", round)}}
	}
	toolUse := func(round int) map[string]any {
		return map[string]any{"type": "assistant", "uuid": fmt.Sprintf("a1-%d", round), "parentUuid": fmt.Sprintf("u-%d", round), "sessionId": sessionID,
			"message": map[string]any{"id": fmt.Sprintf("m1-%d", round), "role": "assistant", "stop_reason": "tool_use",
				"content": []map[string]any{{"type": "tool_use", "id": fmt.Sprintf("t-%d", round), "name": "Bash", "input": map[string]any{"command": "ls"}}}}}
	}
	toolResultAndAnswer := func(round int) []map[string]any {
		return []map[string]any{
			{"type": "user", "uuid": fmt.Sprintf("r-%d", round), "parentUuid": fmt.Sprintf("a1-%d", round), "sessionId": sessionID,
				"message": map[string]any{"role": "user", "content": []map[string]any{{"type": "tool_result", "tool_use_id": fmt.Sprintf("t-%d", round), "content": "ok"}}}},
			{"type": "assistant", "uuid": fmt.Sprintf("a2-%d", round), "parentUuid": fmt.Sprintf("r-%d", round), "sessionId": sessionID,
				"message": map[string]any{"id": fmt.Sprintf("m2-%d", round), "role": "assistant", "stop_reason": "end_turn",
					"content": []map[string]any{{"type": "text", "text": fmt.Sprintf("answer %d", round)}}}},
		}
	}
	appendRound := func(round int, parent string) {
		appendRows(userRow(round, parent), toolUse(round))
		appendRows(toolResultAndAnswer(round)...)
	}

	sessionKey, agentID := f.session.SessionKey, "amy"
	check := func(label string) {
		t.Helper()
		got, err := f.history.readTranscriptMessages(f.workspacePath, sessionKey, agentID, sessionID, markers, "")
		if err != nil {
			t.Fatal(err)
		}
		file, err := os.Open(f.transcript)
		if err != nil {
			t.Fatal(err)
		}
		entries, err := readTranscriptEntriesFileContext(context.Background(), file)
		_ = file.Close()
		if err != nil {
			t.Fatal(err)
		}
		want := projectTranscriptChain(f.workspacePath, sessionKey, agentID, buildPrimaryTranscriptChain(entries), markers)
		if strings.HasPrefix(label, "marker") && !strings.Contains(fmt.Sprint(want), "rewritten") {
			t.Fatalf("%s: markers did not align, fixture is not exercising marker checks", label)
		}
		if len(want) == 0 || !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: checkpointed projection diverged\ngot  %v\nwant %v", label, got, want)
		}
		// 调用方随意修改结果，不能影响下一次从检查点恢复的输出。
		for _, row := range got {
			row["round_id"] = "mutated"
			if content, ok := row["content"].([]map[string]any); ok && len(content) > 0 {
				content[0]["type"] = "mutated"
			}
		}
	}
	checkpointRows := func() int {
		path, err := f.history.resolveTranscriptPath(f.workspacePath, sessionID)
		if err != nil {
			t.Fatal(err)
		}
		f.history.cache.mu.Lock()
		defer f.history.cache.mu.Unlock()
		if projection := f.history.cache.entries[path].Projection; projection != nil {
			return len(projection.Rows)
		}
		return 0
	}

	appendRound(0, "")
	check("first round")
	for round := 1; round < 6; round++ {
		appendRound(round, fmt.Sprintf("a2-%d", round-1))
		check(fmt.Sprintf("append round %d", round))
	}
	if checkpointRows() == 0 {
		t.Fatal("append path did not keep a projection checkpoint")
	}
	check("unchanged")

	// 半轮：用户输入和工具调用已写入，工具结果稍后到达。
	appendRows(userRow(6, "a2-5"), toolUse(6))
	check("half round")
	appendRows(toolResultAndAnswer(6)...)
	check("half round completed")

	// 从较早的回答分叉，主链前缀改变，检查点必须失效。
	// 分叉后连续追加多轮，新主链比检查点更长，只能靠逐个比对前缀条目发现。
	appendRound(7, "a2-3")
	appendRound(70, "a2-7")
	appendRound(71, "a2-70")
	check("branch")
	appendRound(8, "a2-71")
	check("append after branch")

	// marker 被改写、投影身份改变都只能用完整投影。
	if withMarkers {
		markers[1].RoundID = "rewritten"
		check("marker rewrite")
	}
	agentID = "other-agent"
	check("identity change")
	agentID = "amy"

	// 原地改写（非纯追加）使解析代系失效。
	content, err := os.ReadFile(f.transcript)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(f.transcript, []byte(strings.Replace(string(content), "answer 2", "answer two", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	check("rewrite")
	appendRound(9, "a2-8")
	check("append after rewrite")
}

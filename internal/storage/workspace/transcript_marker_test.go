package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func TestAgentHistoryStoreUsesHiddenRoundMarkerForTranscriptAlignment(t *testing.T) {
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
	sessionID := "hidden-goal-session"
	if err := history.AppendRoundMarkerWithOptions(workspacePath, sessionKey, "goal_continuation_1", "hidden continuation prompt", 1000, RoundMarkerOptions{
		HiddenFromUser: true,
		Synthetic:      true,
		Purpose:        "goal_continuation",
	}); err != nil {
		t.Fatalf("写入隐藏 round marker 失败: %v", err)
	}

	writeAgentTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "transcript-user-hidden",
			"sessionId": sessionID,
			"timestamp": "2026-05-22T10:00:00.000Z",
			"message": map[string]any{
				"role":    "user",
				"content": "hidden continuation prompt",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "transcript-assistant-hidden",
			"sessionId":  sessionID,
			"parentUuid": "transcript-user-hidden",
			"message": map[string]any{
				"role":        "assistant",
				"stop_reason": "end_turn",
				"content": []map[string]any{
					{"type": "text", "text": "继续推进 Goal。"},
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
		if row["role"] == "user" && row["round_id"] == "goal_continuation_1" {
			t.Fatalf("隐藏 Goal continuation 不应展示成用户消息: %+v", rows)
		}
	}
	if len(rows) != 1 || rows[0]["role"] != "assistant" || rows[0]["round_id"] != "goal_continuation_1" {
		t.Fatalf("隐藏 marker 应只用于 assistant round 对齐: %+v", rows)
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

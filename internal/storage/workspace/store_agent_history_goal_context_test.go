package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestAgentHistoryStoreHidesGoalContextOnlyTranscriptTurn(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := filepath.Join(configRoot, "workspace")
	workspacePath := filepath.Join(workspaceRoot, "Amy")
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatalf("创建 workspace 失败: %v", err)
	}
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(configRoot, "home"))

	history := NewAgentHistoryStore(workspaceRoot)
	sessionKey := "agent:c5740009ac97:ws:dm:a731e54f7af5"
	sessionID := "hidden-goal-context-session"
	if err := history.AppendRoundMarkerWithOptions(workspacePath, sessionKey, "goal_continuation_1", "", 1000, RoundMarkerOptions{
		HiddenFromUser: true,
		Synthetic:      true,
		Purpose:        "goal_continuation",
	}); err != nil {
		t.Fatalf("写入隐藏 round marker 失败: %v", err)
	}

	writeAgentTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "transcript-user-hidden-context",
			"sessionId": sessionID,
			"timestamp": "2026-05-22T10:00:00.000Z",
			"message": map[string]any{
				"role":    "user",
				"content": "<goal_context>\nContinue working toward the active thread goal.\n</goal_context>",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "transcript-assistant-hidden-context",
			"sessionId":  sessionID,
			"parentUuid": "transcript-user-hidden-context",
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
		if row["role"] == "user" {
			t.Fatalf("GoalContext-only continuation 不应展示成用户消息: %+v", rows)
		}
	}
	if len(rows) != 1 || rows[0]["role"] != "assistant" || rows[0]["round_id"] != "goal_continuation_1" {
		t.Fatalf("隐藏 GoalContext marker 应只用于 assistant round 对齐: %+v", rows)
	}
}

func TestAgentHistoryStoreHidesGoalContextTranscriptTurnWithoutMarker(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "goal_context",
			content: "<goal_context>\nContinue working toward the active thread goal.\n</goal_context>",
		},
		{
			name:    "codex_internal_context",
			content: "<codex_internal_context source=\"goal\">\nContinue working toward the active thread goal.\n</codex_internal_context>",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configRoot := t.TempDir()
			workspaceRoot := filepath.Join(configRoot, "workspace")
			workspacePath := filepath.Join(workspaceRoot, "Amy")
			if err := os.MkdirAll(workspacePath, 0o755); err != nil {
				t.Fatalf("创建 workspace 失败: %v", err)
			}
			t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(configRoot, "home"))

			history := NewAgentHistoryStore(workspaceRoot)
			sessionKey := "agent:c5740009ac97:ws:dm:a731e54f7af5"
			sessionID := "hidden-" + test.name + "-no-marker-session"
			writeAgentTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
				{
					"type":      "user",
					"uuid":      "transcript-user-hidden-context",
					"sessionId": sessionID,
					"timestamp": "2026-05-22T10:00:00.000Z",
					"message": map[string]any{
						"role":    "user",
						"content": test.content,
					},
				},
				{
					"type":       "assistant",
					"uuid":       "transcript-assistant-hidden-context",
					"sessionId":  sessionID,
					"parentUuid": "transcript-user-hidden-context",
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
				if row["role"] == "user" {
					t.Fatalf("缺失 marker 时 goal context transcript 不应展示成用户消息: %+v", rows)
				}
			}
			if len(rows) != 1 || rows[0]["role"] != "assistant" {
				t.Fatalf("缺失 marker 时应保留 assistant 输出并隐藏 goal context 输入: %+v", rows)
			}
		})
	}
}

func TestTranscriptGoalContextOnlyUserTurnRecognizesInternalContextTags(t *testing.T) {
	for name, content := range map[string]string{
		"internal": "<internal_context source=\"goal\">\nContinue working toward the active thread goal.\n</internal_context>",
		"legacy":   "<codex_internal_context source=\"goal\">\nContinue working toward the active thread goal.\n</codex_internal_context>",
	} {
		entry := map[string]any{
			"message": map[string]any{
				"role":    "user",
				"content": content,
			},
		}
		if !isTranscriptGoalContextOnlyUserTurn(entry) {
			t.Fatalf("%s Goal context turn was not recognized: %#v", name, entry)
		}
	}
}

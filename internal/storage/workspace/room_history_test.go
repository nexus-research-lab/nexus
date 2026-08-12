package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRoomHistoryStoreSeparatesSameConversationByOwner(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")

	history := NewRoomHistoryStore("")
	conversationID := "shared-conversation-id"
	if err := history.AppendInlineMessage("user-a", conversationID, protocol.Message{
		"message_id":      "message-a",
		"conversation_id": conversationID,
		"content":         "owner a only",
	}); err != nil {
		t.Fatalf("写入 owner A Room 历史失败: %v", err)
	}

	ownerBRows, err := history.ReadMessages("user-b", conversationID, nil)
	if err != nil {
		t.Fatalf("读取 owner B Room 历史失败: %v", err)
	}
	if len(ownerBRows) != 0 {
		t.Fatalf("相同 conversation_id 不应跨 owner 读取: %+v", ownerBRows)
	}

	ownerARows, err := history.ReadMessages("user-a", conversationID, nil)
	if err != nil {
		t.Fatalf("读取 owner A Room 历史失败: %v", err)
	}
	if len(ownerARows) != 1 || ownerARows[0]["message_id"] != "message-a" {
		t.Fatalf("owner A Room 历史不完整: %+v", ownerARows)
	}
	// Windows 的 FileMode 不表达继承 ACL，POSIX group/other 位只在其他平台校验。
	if runtime.GOOS != "windows" {
		ledgerInfo, err := os.Stat(history.paths.RoomConversationOverlayPath("user-a", conversationID))
		if err != nil {
			t.Fatal(err)
		}
		if ledgerInfo.Mode().Perm()&0o077 != 0 {
			t.Fatalf("Room ledger 不能向 group/other 暴露: mode=%#o", ledgerInfo.Mode().Perm())
		}
	}
}

func TestRoomHistoryStoreMessageCountTracksVisibleLedgerChanges(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")

	history := NewRoomHistoryStore("")
	conversationID := "conversation-count"
	ownerUserID := "user-count"
	if count, err := history.MessageCount(ownerUserID, conversationID); err != nil || count != 0 {
		t.Fatalf("空 Room 消息数 = %d, err=%v", count, err)
	}
	for _, message := range []protocol.Message{
		{
			"message_id": "message-user",
			"round_id":   "round-count",
			"role":       "user",
			"content":    "设定目标",
			"timestamp":  int64(1000),
		},
		{
			"message_id":  "message-assistant",
			"round_id":    "round-count",
			"role":        "assistant",
			"content":     "处理中",
			"is_complete": true,
			"stop_reason": "end_turn",
			"timestamp":   int64(1100),
		},
	} {
		if err := history.AppendInlineMessage(ownerUserID, conversationID, message); err != nil {
			t.Fatalf("写入 Room 消息失败: %v", err)
		}
	}
	if count, err := history.MessageCount(ownerUserID, conversationID); err != nil || count != 2 {
		t.Fatalf("Room 消息数 = %d, want 2, err=%v", count, err)
	}
	if err := history.AppendInlineMessage(ownerUserID, conversationID, protocol.Message{
		"message_id":  "message-assistant",
		"round_id":    "round-count",
		"role":        "assistant",
		"content":     "已完成",
		"is_complete": true,
		"stop_reason": "end_turn",
		"timestamp":   int64(1200),
	}); err != nil {
		t.Fatalf("写入 Room assistant 新快照失败: %v", err)
	}
	if count, err := history.MessageCount(ownerUserID, conversationID); err != nil || count != 2 {
		t.Fatalf("重复 message_id 压缩后消息数 = %d, want 2, err=%v", count, err)
	}
}

func TestRoomHistoryStoreRejectsTranscriptFromAnotherOwnerWorkspace(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	foreignWorkspace := filepath.Join(
		appfs.UserWorkspaceRootAt(stateRoot, "user-b"),
		"agent-b",
	)
	if err := os.MkdirAll(foreignWorkspace, 0o700); err != nil {
		t.Fatal(err)
	}
	sessionID := "session-foreign-owner"
	writeAgentTranscriptFixture(t, foreignWorkspace, sessionID, []map[string]any{{
		"type":      "assistant",
		"uuid":      "message-foreign-owner",
		"sessionId": sessionID,
		"message": map[string]any{
			"id":      "message-foreign-owner",
			"role":    "assistant",
			"content": []map[string]any{{"type": "text", "text": "foreign"}},
		},
	}})

	history := NewRoomHistoryStore("")
	message := protocol.Message{
		"message_id":      "message-foreign-owner",
		"session_id":      sessionID,
		"conversation_id": "conversation-a",
		"agent_id":        "agent-b",
		"round_id":        "round-foreign-owner",
		"role":            "assistant",
	}
	row := buildRoomTranscriptReference(
		message,
		foreignWorkspace,
		"agent:agent-b:ws:group:conversation-a",
	)
	if err := history.files.appendRoomJSONL(
		"user-a",
		history.paths.RoomConversationOverlayPath("user-a", "conversation-a"),
		row,
	); err != nil {
		t.Fatal(err)
	}
	rows, err := history.ReadMessages("user-a", "conversation-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("Room 历史不能解析另一 owner 的 transcript: %+v", rows)
	}
}

func TestRoomHistoryStoreRejectsCrossOwnerStateSymlink(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")

	ownerBRoomRoot := appfs.UserRoomRootAt(stateRoot, "user-b")
	if err := os.MkdirAll(ownerBRoomRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	ownerADataRoot := appfs.UserDataRootAt(stateRoot, "user-a")
	if err := os.MkdirAll(ownerADataRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(
		filepath.Join("..", "user-b", "state"),
		filepath.Join(ownerADataRoot, "state"),
	); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	history := NewRoomHistoryStore("")
	err := history.AppendInlineMessage("user-a", "conversation-a", protocol.Message{
		"message_id": "message-a",
		"content":    "must stay with owner a",
	})
	if !errors.Is(err, confinedfs.ErrSymlink) {
		t.Fatalf("跨 owner state symlink 应被拒绝: %v", err)
	}
	foreignTarget := filepath.Join(
		ownerBRoomRoot,
		filepath.Base(history.paths.RoomConversationDir("user-b", "conversation-a")),
		"overlay.jsonl",
	)
	if _, statErr := os.Stat(foreignTarget); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("不能借 symlink 写入 owner B Room ledger: %v", statErr)
	}
}

func TestRoomHistoryStoreMergesLateMentionAnnotationReference(t *testing.T) {
	configRoot := t.TempDir()
	stateRoot := filepath.Join(configRoot, ".nexus")
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	workspaceRoot := filepath.Join(stateRoot, "users")
	workspacePath := filepath.Join(
		appfs.UserWorkspaceRootAt(stateRoot, testRoomOwnerUserID),
		"Amy",
	)
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatalf("创建 workspace 失败: %v", err)
	}

	sessionID := "4035f197-ca97-43fc-b9ae-06ac04903213"
	privateSessionKey := "agent:agent-amy:ws:group:conversation-1"
	writeAgentTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
		{
			"type": "user", "uuid": "transcript-user-1", "sessionId": sessionID,
			"message": map[string]any{"role": "user", "content": "开始任务"},
		},
		{
			"type": "assistant", "uuid": "transcript-assistant-1", "sessionId": sessionID,
			"parentUuid": "transcript-user-1", "message": map[string]any{
				"id": "assistant-1", "type": "message", "role": "assistant",
				"stop_reason": "end_turn",
				"content":     []map[string]any{{"type": "text", "text": "@Devin 接着处理"}},
			},
		},
	})

	history := NewRoomHistoryStore(workspaceRoot)
	base := protocol.Message{
		"message_id": "assistant-1", "session_id": sessionID,
		"conversation_id": "conversation-1", "agent_id": "agent-amy",
		"round_id": "root-1", "agent_round_id": "agent-round-1",
		"parent_id": "slot-1", "role": "assistant", "is_complete": true,
		"timestamp": int64(1000),
	}
	if err := history.AppendTranscriptReference(
		testRoomOwnerUserID,
		"conversation-1",
		workspacePath,
		privateSessionKey,
		base,
	); err != nil {
		t.Fatalf("写入初始 transcript 引用失败: %v", err)
	}
	annotated := protocol.Clone(base)
	annotated["agent_mentions"] = []protocol.AgentMention{{
		AgentID: "agent-devin", Label: "Devin", StartRune: 0, EndRune: 6,
	}}
	if err := history.AppendTranscriptReference(
		testRoomOwnerUserID,
		"conversation-1",
		workspacePath,
		privateSessionKey,
		annotated,
	); err != nil {
		t.Fatalf("写入终态 transcript 引用失败: %v", err)
	}

	rows, err := history.ReadMessages(testRoomOwnerUserID, "conversation-1", nil)
	if err != nil {
		t.Fatalf("读取 Room 历史失败: %v", err)
	}
	if len(rows) != 1 || rows[0]["message_id"] != "assistant-1" {
		t.Fatalf("同 message_id 的引用应压缩成一条消息: %+v", rows)
	}
	mentions, ok := rows[0]["agent_mentions"].([]any)
	if !ok || len(mentions) != 1 {
		t.Fatalf("迟到的 agent_mentions 应保留在历史: %+v", rows[0])
	}
}

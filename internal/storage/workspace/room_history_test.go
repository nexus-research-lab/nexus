package workspace

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
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
		wantMode := storageFileMode(0o600)
		if ledgerInfo.Mode().Perm() != wantMode {
			t.Fatalf("Room ledger mode=%#o, want %#o", ledgerInfo.Mode().Perm(), wantMode)
		}
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

func TestRoomHistoryStoreMergesLateHostAnnotationsReference(t *testing.T) {
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
	annotated["handoff_reply"] = &protocol.PublicHandoffReply{
		HandoffID:       "rh-reply-1",
		SourceMessageID: "assistant-source-1",
		SourceAgentID:   "agent-lead",
	}
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
	reply := protocol.NormalizePublicHandoffReply(rows[0]["handoff_reply"])
	if reply == nil || reply.HandoffID != "rh-reply-1" ||
		reply.SourceMessageID != "assistant-source-1" ||
		reply.SourceAgentID != "agent-lead" {
		t.Fatalf("迟到的 handoff_reply 应与 mention 同构保留: %+v", rows[0])
	}
}

func TestIndexRoomTranscriptMessagesMergesAssistantChunks(t *testing.T) {
	indexed := indexRoomTranscriptMessages([]protocol.Message{
		{
			"message_id": "assistant-1",
			"role":       "assistant",
			"content": []map[string]any{
				{"type": "thinking", "thinking": "先想一下"},
			},
		},
		{
			"message_id": "assistant-1",
			"role":       "assistant",
			"content": []map[string]any{
				{"type": "text", "text": "正文不能消失"},
			},
		},
		{
			"message_id": "assistant-1",
			"role":       "assistant",
			"content": []map[string]any{
				{"type": "tool_use", "id": "tool-1", "name": "Bash"},
			},
		},
	})

	message := indexed["assistant-1"]
	blocks := normalizeMessageContentBlocks(message["content"])
	if len(blocks) != 3 ||
		stringFromAny(blocks[0]["type"]) != "thinking" ||
		stringFromAny(blocks[1]["text"]) != "正文不能消失" ||
		stringFromAny(blocks[2]["id"]) != "tool-1" {
		t.Fatalf("Room transcript 分块没有完整合并: %+v", blocks)
	}
}

func TestRoomHistoryRepairsTranscriptPermissions(t *testing.T) {
	if runtime.GOOS != "linux" || os.Geteuid() == 0 {
		t.Skip("需要以普通用户运行 Linux 文件权限回归")
	}
	stateRoot := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	t.Setenv("NEXUS_RUNTIME_ISOLATION_MODE", "enforce")
	owner := "room-permission-owner"
	workspacePath := filepath.Join(appfs.UserWorkspaceRootAt(stateRoot, owner), "agent")
	if err := os.MkdirAll(workspacePath, 0o700); err != nil {
		t.Fatal(err)
	}
	sessionID := "4035f197-ca97-43fc-b9ae-06ac04903213"
	writeAgentTranscriptFixture(t, workspacePath, sessionID, []map[string]any{{
		"type": "assistant", "uuid": "assistant-1", "sessionId": sessionID,
		"message": map[string]any{
			"id": "assistant-1", "role": "assistant", "stop_reason": "end_turn",
			"content": []map[string]any{{"type": "text", "text": "任务完成"}},
		},
	}})
	history := NewRoomHistoryStore("")
	transcriptPath, err := history.agentHistory.ForOwner(owner).resolveTranscriptPath(workspacePath, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if err = history.AppendTranscriptReference(owner, "conversation-1", workspacePath,
		"agent:agent:ws:group:conversation-1", protocol.Message{
			"message_id": "assistant-1", "session_id": sessionID, "agent_id": "agent",
			"role": "assistant", "round_id": "round-1", "is_complete": true, "timestamp": int64(1000),
		}); err != nil {
		t.Fatal(err)
	}
	// 用受限替身模拟 launcher 修复；精确校验 owner，其他参数不能触发 chmod。
	launcher := filepath.Join(stateRoot, "repair-launcher")
	script := fmt.Sprintf("#!/bin/sh\n[ \"$1\" = repair-user ] && [ \"$2\" = --owner ] && [ \"$3\" = %s ] || exit 1\nchmod 600 %q\n", owner, transcriptPath)
	if err = os.WriteFile(launcher, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NEXUS_RUNTIME_LAUNCHER_PATH", launcher)
	for _, paged := range []bool{false, true} {
		t.Run(fmt.Sprintf("paged=%t", paged), func(t *testing.T) {
			if err := os.Chmod(transcriptPath, 0); err != nil {
				t.Fatal(err)
			}
			if file, openErr := os.Open(transcriptPath); openErr == nil {
				file.Close()
				t.Fatal("未复现 transcript 权限拒绝")
			}
			var rows []protocol.Message
			if paged {
				var page protocol.MessagePage
				page, err = history.ReadMessagesPageContext(context.Background(), owner, "conversation-1", nil, HistoryPageQuery{Limit: 10})
				rows = page.Items
			} else {
				rows, err = history.ReadMessages(owner, "conversation-1", nil)
			}
			if err == nil && (len(rows) != 1 || rows[0]["message_id"] != "assistant-1") {
				t.Fatalf("恢复后必须保留原消息，实际 %+v", rows)
			}
			if err != nil {
				t.Fatalf("Room 读取应修复权限并成功重试: %v", err)
			}
		})
	}
}

func TestDurableHistoryWritesRoomPreviewWithoutHistoryIndex(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	schema, err := os.ReadFile("../../../db/migrations/sqlite/00145_room_reply_previews.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TABLE rooms(id TEXT PRIMARY KEY, owner_user_id TEXT, room_type TEXT);
 CREATE TABLE conversations(id TEXT PRIMARY KEY, room_id TEXT);
 INSERT INTO rooms VALUES ('dm','owner','dm'),('group','owner','room');
 INSERT INTO conversations VALUES ('dm-conv','dm'),('group-conv','group');` + strings.Split(string(schema), "-- +goose Down")[0]); err != nil {
		t.Fatal(err)
	}
	repository := roomrepo.NewSQLRepository("sqlite", db)
	history := NewAgentHistoryStore(appfs.UsersRoot())
	history.SetReplyPreviewRepository(repository)
	history = history.ForOwner("owner")
	workspace := filepath.Join(appfs.UserWorkspaceRootAt(stateRoot, "owner"), "agent")
	if err = os.MkdirAll(workspace, 0700); err != nil {
		t.Fatal(err)
	}
	message := protocol.Message{"role": "assistant", "message_id": "reply", "timestamp": int64(1), "is_complete": true, "content": "新回复"}
	if err = history.AppendOverlayMessage(workspace, "agent:agent:ws:dm:dm-conv", message); err != nil {
		t.Fatal(err)
	}
	values, err := repository.ListRoomReplyPreviews(context.Background(), "owner")
	if err != nil || values["dm"] != "新回复" {
		t.Fatalf("DM preview=%v, err=%v", values, err)
	}
	// 私有成员历史不能更新群聊；只有公区持久化入口可以写群聊摘要。
	history.RecordReplyPreview("agent:agent:ws:group:group-conv", message)
	values, _ = repository.ListRoomReplyPreviews(context.Background(), "owner")
	if values["group"] != "" {
		t.Fatal("私有消息泄漏到公区摘要")
	}
	rooms := NewRoomHistoryStore(appfs.UsersRoot())
	rooms.SetReplyPreviewRepository(repository)
	if err = rooms.AppendInlineMessage("owner", "group-conv", message); err != nil {
		t.Fatal(err)
	}
	values, err = repository.ListRoomReplyPreviews(context.Background(), "owner")
	if err != nil || values["group"] != "新回复" {
		t.Fatalf("Room preview=%v, err=%v", values, err)
	}
	message["content"] = "未落盘内容"
	message["timestamp"] = int64(2)
	if err = history.AppendOverlayMessage(t.TempDir(), "agent:agent:ws:dm:dm-conv", message); err == nil {
		t.Fatal("跨 owner 路径应拒绝落盘")
	}
	values, _ = repository.ListRoomReplyPreviews(context.Background(), "owner")
	if values["dm"] != "新回复" {
		t.Fatal("失败的落盘改写了摘要")
	}
	if _, err = os.Stat(historyReadModelPath(appfs.UsersRoot())); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("不应建立历史索引: %v", err)
	}
}

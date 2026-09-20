// INPUT: Room 原始追加、尾轮更新和文件重写。
// OUTPUT: 增量结果与 canonical 分页等价，历史前缀不重写。
// POS: Room 增量读模型回归。
package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRoomHistoryIncrementalTail(t *testing.T) {
	h := newRoomHistoryTestStore(t, t.TempDir())
	ctx := context.Background()
	owner, conversation := testRoomOwnerUserID, "incremental"
	appendMessage := func(id, round, role, text string, timestamp int64) {
		t.Helper()
		var content any = text
		if role == "assistant" {
			content = []any{map[string]any{"type": "text", "text": text}}
		}
		if err := h.AppendInlineMessage(owner, conversation, protocol.Message{"message_id": id, "round_id": round, "agent_id": "amy", "role": role, "content": content, "timestamp": timestamp, "is_complete": true}); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 40; i++ {
		appendMessage(fmt.Sprint("user-", i), fmt.Sprint("round-", i), "user", "hello", int64(1000+i*10))
	}
	access := h.historyPageAccess(owner, conversation)
	built, err := access.Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = h.readModel.persist(ctx, access, built); err != nil {
		t.Fatal(err)
	}
	assertRoomHistoryPageParity(t, h, owner, conversation, nil, 2, "", 0, "", 0)
	db, err := h.readModel.database(ctx)
	if err != nil {
		t.Fatal(err)
	}
	before, _, err := readHistoryReadModelScope(ctx, db, access.Scope)
	if err != nil {
		t.Fatal(err)
	}
	// 禁止重写既有前缀，证明增量更新没有退化成全量 persist。
	if _, err = db.Exec(`CREATE TRIGGER keep_prefix BEFORE DELETE ON history_read_groups WHEN OLD.sequence < 39 BEGIN SELECT RAISE(ABORT,'prefix rewritten'); END`); err != nil {
		t.Fatal(err)
	}
	appendMessage("answer-39", "round-39", "assistant", "answer", 1391)
	if err = h.tryRefreshRoomHistoryTail(ctx, owner, conversation, access.Scope); err != nil {
		t.Fatal(err)
	}
	assertRoomHistoryPageParity(t, h, owner, conversation, nil, 2, "", 0, "", 0)
	appendMessage("user-40", "round-40", "user", "next", 1400)
	appendMessage("answer-40", "round-40", "assistant", "done", 1401)
	if err = h.tryRefreshRoomHistoryTail(ctx, owner, conversation, access.Scope); err != nil {
		t.Fatal(err)
	}
	after, _, err := readHistoryReadModelScope(ctx, db, access.Scope)
	if err != nil {
		t.Fatal(err)
	}
	if after.Generation != before.Generation || after.GroupCount != 41 {
		t.Fatalf("unexpected incremental scope: %+v", after)
	}
	assertRoomHistoryPageParity(t, h, owner, conversation, nil, 2, "", 0, "", 0)
	assertRoomHistoryPageParity(t, h, owner, conversation, nil, 2, "round-39", 1390, "", 0)
	index, ok, err := h.readModel.loadRoundIndex(ctx, access, nil, true)
	if err != nil || !ok {
		t.Fatalf("index: %v %v", ok, err)
	}
	expected, err := h.ReadRoundIndex(owner, conversation, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(expected, index) {
		t.Fatalf("round index mismatch: got %+v want %+v", index, expected)
	}

	// 插入失败时，尾轮删除与检查点必须一起回滚；重试只消费尚未提交的追加。
	if _, err = db.Exec(`CREATE TRIGGER reject_tail BEFORE INSERT ON history_read_groups BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	appendMessage("user-41", "round-41", "user", "retry", 1410)
	if err = h.tryRefreshRoomHistoryTail(ctx, owner, conversation, access.Scope); err == nil {
		t.Fatal("injected failure ignored")
	}
	rolledBack, _, err := readHistoryReadModelScope(ctx, db, access.Scope)
	if err != nil || !snapshotsEqual(rolledBack.Sources, after.Sources) || rolledBack.GroupCount != after.GroupCount {
		t.Fatal("checkpoint advanced on rollback")
	}
	if _, err = db.Exec(`DROP TRIGGER reject_tail`); err != nil {
		t.Fatal(err)
	}
	assertRoomHistoryPageParity(t, h, owner, conversation, nil, 2, "", 0, "", 0)
	waitHistoryPageIndexFuture(t, access.Scope)
	refreshed, _, err := readHistoryReadModelScope(ctx, db, access.Scope)
	if err != nil || refreshed.GroupCount != 42 {
		t.Fatal("worker did not publish incremental page")
	}
	// 已经存在于旧轮次的修正必须拒绝增量，不能错放到尾部。
	appendMessage("user-0", "round-0", "user", "edited", 1000)
	if err = h.tryRefreshRoomHistoryTail(ctx, owner, conversation, access.Scope); err == nil {
		t.Fatal("old round edit accepted")
	}
}

func TestRoomHistoryIncrementalKeepsTranscriptDependencies(t *testing.T) {
	stateRoot := t.TempDir()
	history := newRoomHistoryTestStore(t, stateRoot)
	ownerUserID := testRoomOwnerUserID
	conversationID := "room-index-private-transcript"
	workspacePath := filepath.Join(appfs.UserWorkspaceRootAt(stateRoot, ownerUserID), "Amy")
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatal(err)
	}
	privateSessionKey := "agent:amy:ws:group:" + conversationID
	sessionID := "5d928f9e-31bd-48a0-9cd5-21a804772001"
	ownerHistory := history.agentHistory.ForOwner(ownerUserID)
	writeAgentHistoryIndexSession(t, ownerHistory, workspacePath, protocol.Session{
		SessionKey: privateSessionKey,
		AgentID:    "amy",
		SessionID:  &sessionID,
	})
	if err := ownerHistory.AppendRoundMarker(
		workspacePath,
		privateSessionKey,
		"private-round-1",
		"处理公区任务",
		1000,
	); err != nil {
		t.Fatal(err)
	}
	writeAgentTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
		{
			"type": "user", "uuid": "room-private-user", "sessionId": sessionID,
			"timestamp": "2026-08-17T00:00:00Z",
			"message":   map[string]any{"role": "user", "content": "处理公区任务"},
		},
		{
			"type": "assistant", "uuid": "room-private-assistant", "sessionId": sessionID,
			"parentUuid": "room-private-user", "timestamp": "2026-08-17T00:00:01Z",
			"message": map[string]any{
				"id": "room-assistant-1", "role": "assistant", "stop_reason": "end_turn",
				"content": []map[string]any{{"type": "text", "text": "公区任务已完成。"}},
			},
		},
	})
	if err := history.AppendInlineMessage(ownerUserID, conversationID, protocol.Message{
		"message_id": "room-user-1", "round_id": "root-private", "role": "user",
		"content": "处理公区任务", "timestamp": int64(1000),
	}); err != nil {
		t.Fatal(err)
	}
	if err := history.AppendTranscriptReference(
		ownerUserID,
		conversationID,
		workspacePath,
		privateSessionKey,
		protocol.Message{
			"message_id": "room-assistant-1", "session_id": sessionID,
			"conversation_id": conversationID, "agent_id": "amy",
			"round_id": "root-private:amy", "agent_round_id": "private-round-1",
			"role": "assistant", "is_complete": true, "timestamp": int64(1100),
		},
	); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	access := history.historyPageAccess(ownerUserID, conversationID)
	built, err := access.Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = history.readModel.persist(ctx, access, built); err != nil {
		t.Fatal(err)
	}
	db, err := history.readModel.database(ctx)
	if err != nil {
		t.Fatal(err)
	}
	before, _, err := readHistoryReadModelScope(ctx, db, access.Scope)
	if err != nil {
		t.Fatal(err)
	}
	appendRoomHistoryIndexRound(t, history, ownerUserID, conversationID, "root-new", "amy", 2000)
	if err = history.tryRefreshRoomHistoryTail(ctx, ownerUserID, conversationID, access.Scope); err != nil {
		t.Fatal(err)
	}
	after, _, err := readHistoryReadModelScope(ctx, db, access.Scope)
	if err != nil || after.Generation != before.Generation || len(after.Sources) != len(before.Sources) {
		t.Fatal("dependencies lost or rebuilt")
	}
	assertRoomHistoryPageParity(t, history, ownerUserID, conversationID, nil, 2, "", 0, "", 0)
	// 私有 overlay 改变时不得把旧公区投影当作已同步。
	if err = ownerHistory.AppendRoundMarker(workspacePath, privateSessionKey, "private-new", "next", 3000); err != nil {
		t.Fatal(err)
	}
	appendRoomHistoryIndexRound(t, history, ownerUserID, conversationID, "root-final", "amy", 3000)
	if err = history.tryRefreshRoomHistoryTail(ctx, ownerUserID, conversationID, access.Scope); err == nil {
		t.Fatal("changed dependency accepted")
	}
	assertRoomHistoryPageParity(t, history, ownerUserID, conversationID, nil, 2, "", 0, "", 0)
}

func TestRoomHistoryIncrementalRejectsReplacedAndPartialFiles(t *testing.T) {
	for _, mode := range []string{"replacement", "partial", "checkpoint"} {
		t.Run(mode, func(t *testing.T) {
			h := newRoomHistoryTestStore(t, t.TempDir())
			owner, conversation := testRoomOwnerUserID, "invalid-tail"
			ctx := context.Background()
			appendRoomHistoryIndexRound(t, h, owner, conversation, "round-1", "amy", 1000)
			access := h.historyPageAccess(owner, conversation)
			built, err := access.Build(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if err = h.readModel.persist(ctx, access, built); err != nil {
				t.Fatal(err)
			}
			db, err := h.readModel.database(ctx)
			if err != nil {
				t.Fatal(err)
			}
			before, _, err := readHistoryReadModelScope(ctx, db, access.Scope)
			if err != nil {
				t.Fatal(err)
			}
			path := h.paths.RoomConversationOverlayPath(owner, conversation)
			appendRoomHistoryIndexRound(t, h, owner, conversation, "round-2", "amy", 2000)
			switch mode {
			case "replacement":
				data, readErr := os.ReadFile(path)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if err = os.WriteFile(path+".replacement", data, 0600); err != nil {
					t.Fatal(err)
				}
				if err = os.Rename(path+".replacement", path); err != nil {
					t.Fatal(err)
				}
			case "partial":
				file, openErr := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
				if openErr != nil {
					t.Fatal(openErr)
				}
				_, err = file.WriteString("{")
				closeErr := file.Close()
				if err != nil || closeErr != nil {
					t.Fatal(err, closeErr)
				}
			case "checkpoint":
				if _, err = db.Exec(`UPDATE history_read_tail SET rows_json='[]' WHERE scope=?`, access.Scope); err != nil {
					t.Fatal(err)
				}
			}
			if err = h.tryRefreshRoomHistoryTail(ctx, owner, conversation, access.Scope); err == nil {
				t.Fatal("unsafe incremental update accepted")
			}
			after, _, err := readHistoryReadModelScope(ctx, db, access.Scope)
			if err != nil || !snapshotsEqual(before.Sources, after.Sources) || before.GroupCount != after.GroupCount {
				t.Fatal("rejected update advanced checkpoint")
			}
		})
	}
}

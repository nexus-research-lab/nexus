// INPUT: DM/Room canonical history、损坏派生文件与并发取消/删除时序。
// OUTPUT: history page index 对旧分页 oracle 的等价性与故障恢复证明。
// POS: seekable history page 派生索引的核心回归测试。
package workspace

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestAgentHistoryPageIndexMatchesCanonicalPagination(t *testing.T) {
	root := t.TempDir()
	workspacePath := filepath.Join(root, "Amy")
	session := protocol.Session{SessionKey: "agent:amy:ws:dm:index-parity", AgentID: "amy"}
	history := NewAgentHistoryStore(root)
	writeAgentHistoryIndexSession(t, history, workspacePath, session)

	appendAgentHistoryIndexRound(t, history, workspacePath, session.SessionKey, "round-1", 1000)
	appendAgentHistoryIndexRound(t, history, workspacePath, session.SessionKey, "round-2", 2000)
	// 同 timestamp 保留 canonical append 稳定顺序，不能按 round_id 字典重排。
	appendAgentHistoryIndexRound(t, history, workspacePath, session.SessionKey, "round-z", 3000)
	appendAgentHistoryIndexRound(t, history, workspacePath, session.SessionKey, "round-a", 3000)
	appendAgentHistoryIndexRound(t, history, workspacePath, session.SessionKey, "round-5", 4000)

	assertAgentHistoryPageParity(t, history, workspacePath, session, nil, 2, "", 0, "", 0)
	waitHistoryPageIndexFuture(t, history.historyPageAccess(workspacePath, session).Scope)
	// 第二次必须走 persisted selected-group seek，JSON wire 仍与 canonical oracle 相同。
	latest := assertAgentHistoryPageParity(t, history, workspacePath, session, nil, 2, "", 0, "", 0)
	assertAgentHistoryPageParity(
		t,
		history,
		workspacePath,
		session,
		nil,
		2,
		derefString(latest.NextBeforeRoundID),
		derefInt64(latest.NextBeforeRoundTimestamp),
		"",
		0,
	)
	assertAgentHistoryPageParity(t, history, workspacePath, session, nil, 0, "", 0, "round-z", 1)
}

func TestRoomHistoryPageIndexMatchesCanonicalPaginationAndReceipt(t *testing.T) {
	root := t.TempDir()
	history := newRoomHistoryTestStore(t, root)
	ownerUserID := testRoomOwnerUserID
	conversationID := "room-index-parity"

	appendRoomHistoryIndexRound(t, history, ownerUserID, conversationID, "root-1", "amy", 1000)
	appendRoomHistoryIndexRound(t, history, ownerUserID, conversationID, "root-z", "amy", 2000)
	appendRoomHistoryIndexRound(t, history, ownerUserID, conversationID, "root-a", "amy", 2000)
	if err := history.AppendInlineMessage(ownerUserID, conversationID, protocol.Message{
		"message_id":  "result-only",
		"round_id":    "root-result:amy",
		"agent_id":    "amy",
		"role":        "result",
		"subtype":     "success",
		"duration_ms": int64(50),
		"timestamp":   int64(3000),
	}); err != nil {
		t.Fatal(err)
	}
	if err := history.AppendInlineMessage(ownerUserID, conversationID, protocol.Message{
		overlayKindField:              overlayKindExternalDelivery,
		"message_id":                  "delivery-result-only",
		"role":                        overlayKindExternalDelivery,
		"round_id":                    "root-result:amy",
		"channel":                     "feishu",
		"target":                      "chat-1",
		"primary_platform_message_id": "platform-1",
		"timestamp":                   int64(3010),
	}); err != nil {
		t.Fatal(err)
	}

	assertRoomHistoryPageParity(t, history, ownerUserID, conversationID, nil, 2, "", 0, "", 0)
	waitHistoryPageIndexFuture(t, history.historyPageAccess(ownerUserID, conversationID).Scope)
	latest := assertRoomHistoryPageParity(t, history, ownerUserID, conversationID, nil, 2, "", 0, "", 0)
	if len(latest.Items) == 0 || latest.Items[len(latest.Items)-1]["external_delivery"] == nil {
		t.Fatalf("result-only Room subround 的 receipt 未挂到 synthetic assistant: %+v", latest.Items)
	}
	assertRoomHistoryPageParity(
		t,
		history,
		ownerUserID,
		conversationID,
		nil,
		2,
		derefString(latest.NextBeforeRoundID),
		derefInt64(latest.NextBeforeRoundTimestamp),
		"",
		0,
	)
	assertRoomHistoryPageParity(t, history, ownerUserID, conversationID, nil, 0, "", 0, "root-z", 1)
}

func TestHistoryReadModelRoundIndexMatchesCanonicalProjection(t *testing.T) {
	t.Run("agent", func(t *testing.T) {
		root := t.TempDir()
		workspacePath := filepath.Join(root, "Amy")
		session := protocol.Session{
			SessionKey: "agent:amy:ws:dm:round-index-model",
			AgentID:    "amy",
		}
		history := NewAgentHistoryStore(root)
		writeAgentHistoryIndexSession(t, history, workspacePath, session)
		for index := 1; index <= 4; index++ {
			appendAgentHistoryIndexRound(
				t, history, workspacePath, session.SessionKey,
				fmt.Sprintf("round-%d", index), int64(index*1000),
			)
		}
		active := []string{"round-4"}
		expected, err := history.ReadRoundIndex(workspacePath, session, active)
		if err != nil {
			t.Fatal(err)
		}
		actual, err := history.ReadRoundIndexPageContext(
			context.Background(), workspacePath, session, active, false,
		)
		if err != nil {
			t.Fatal(err)
		}
		assertSessionRoundIndexJSONEqual(t, expected, actual)
		access := history.historyPageAccess(workspacePath, session)
		waitHistoryPageIndexFuture(t, access.Scope)
		warm, err := history.ReadRoundIndexPageContext(
			context.Background(), workspacePath, session, active, true,
		)
		if err != nil {
			t.Fatal(err)
		}
		assertSessionRoundIndexJSONEqual(t, expected, warm)
	})

	t.Run("room", func(t *testing.T) {
		root := t.TempDir()
		history := newRoomHistoryTestStore(t, root)
		conversationID := "round-index-model"
		for index := 1; index <= 4; index++ {
			appendRoomHistoryIndexRound(
				t,
				history,
				testRoomOwnerUserID,
				conversationID,
				fmt.Sprintf("round-%d", index),
				"amy",
				int64(index*1000),
			)
		}
		active := []string{"round-4:amy"}
		expected, err := history.ReadRoundIndex(
			testRoomOwnerUserID, conversationID, active,
		)
		if err != nil {
			t.Fatal(err)
		}
		actual, err := history.ReadRoundIndexPageContext(
			context.Background(),
			testRoomOwnerUserID,
			conversationID,
			active,
			false,
		)
		if err != nil {
			t.Fatal(err)
		}
		assertSessionRoundIndexJSONEqual(t, expected, actual)
		access := history.historyPageAccess(testRoomOwnerUserID, conversationID)
		waitHistoryPageIndexFuture(t, access.Scope)
		warm, err := history.ReadRoundIndexPageContext(
			context.Background(),
			testRoomOwnerUserID,
			conversationID,
			active,
			true,
		)
		if err != nil {
			t.Fatal(err)
		}
		assertSessionRoundIndexJSONEqual(t, expected, warm)
	})
}

func TestHistoryReadModelBackfillDoesNotRewriteCanonicalData(t *testing.T) {
	root := t.TempDir()
	workspacePath := filepath.Join(root, "Amy")
	session := protocol.Session{
		SessionKey: "agent:amy:ws:dm:canonical-unchanged",
		AgentID:    "amy",
	}
	history := NewAgentHistoryStore(root)
	writeAgentHistoryIndexSession(t, history, workspacePath, session)
	appendAgentHistoryIndexRound(
		t, history, workspacePath, session.SessionKey, "round-1", 1000,
	)
	overlayPath := history.paths.SessionOverlayPath(workspacePath, session.SessionKey)
	before, err := os.ReadFile(overlayPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = history.ReadMessagesPageContext(
		context.Background(),
		workspacePath,
		session,
		nil,
		HistoryPageQuery{Limit: 1},
	); err != nil {
		t.Fatal(err)
	}
	waitHistoryPageIndexFuture(t, history.historyPageAccess(workspacePath, session).Scope)
	after, err := os.ReadFile(overlayPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("derived read-model backfill rewrote canonical overlay")
	}
	if _, err = os.Stat(history.readModel.path); err != nil {
		t.Fatalf("derived database was not created outside canonical session: %v", err)
	}
}

func TestAgentHistoryPageIndexMatchesTranscriptMarkerPagination(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := filepath.Join(configRoot, "workspace")
	workspacePath := filepath.Join(workspaceRoot, "Amy")
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NEXUS_STATE_ROOT", "")
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(configRoot, "home"))

	sessionID := "4b7d1d45-3c46-41c2-bb9d-f44a31fef001"
	session := protocol.Session{
		SessionKey: "agent:amy:ws:dm:index-transcript",
		AgentID:    "amy",
		SessionID:  &sessionID,
		Options:    map[string]any{},
	}
	history := NewAgentHistoryStore(workspaceRoot)
	writeAgentHistoryIndexSession(t, history, workspacePath, session)
	if err := history.AppendRoundMarker(workspacePath, session.SessionKey, "round-tool", "创建文件", 1000); err != nil {
		t.Fatal(err)
	}
	if err := history.AppendRoundMarker(workspacePath, session.SessionKey, "round-followup", "检查结果", 2000); err != nil {
		t.Fatal(err)
	}
	writeAgentTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
		{
			"type": "user", "uuid": "transcript-user-tool", "sessionId": sessionID,
			"timestamp": "2026-08-17T00:00:00Z",
			"message":   map[string]any{"role": "user", "content": "创建文件"},
		},
		{
			"type": "assistant", "uuid": "transcript-assistant-snapshot", "sessionId": sessionID,
			"parentUuid": "transcript-user-tool", "timestamp": "2026-08-17T00:00:01Z",
			"message": map[string]any{
				"id": "assistant-tool", "role": "assistant", "stop_reason": "tool_use",
				"content": []map[string]any{{"type": "tool_use", "id": "tool-1", "name": "Write", "input": map[string]any{"file_path": "note.txt"}}},
			},
		},
		// 同 UUID 后写 snapshot 是 canonical transcript 的常见形态。
		{
			"type": "assistant", "uuid": "transcript-assistant-snapshot", "sessionId": sessionID,
			"parentUuid": "transcript-user-tool", "timestamp": "2026-08-17T00:00:02Z",
			"message": map[string]any{
				"id": "assistant-tool", "role": "assistant", "stop_reason": "tool_use",
				"content": []map[string]any{
					{"type": "text", "text": "正在创建。"},
					{"type": "tool_use", "id": "tool-1", "name": "Write", "input": map[string]any{"file_path": "note.txt"}},
				},
			},
		},
		{
			"type": "user", "uuid": "transcript-tool-result", "sessionId": sessionID,
			"parentUuid": "transcript-assistant-snapshot", "timestamp": "2026-08-17T00:00:03Z",
			"message": map[string]any{
				"role":    "user",
				"content": []map[string]any{{"type": "tool_result", "tool_use_id": "tool-1", "content": "created"}},
			},
		},
		{
			"type": "assistant", "uuid": "transcript-assistant-final", "sessionId": sessionID,
			"parentUuid": "transcript-tool-result", "timestamp": "2026-08-17T00:00:04Z",
			"message": map[string]any{
				"id": "assistant-final", "role": "assistant", "stop_reason": "end_turn",
				"content": []map[string]any{{"type": "text", "text": "文件已创建。"}},
			},
		},
		{
			"type": "user", "uuid": "transcript-user-followup", "sessionId": sessionID,
			"parentUuid": "transcript-assistant-final", "timestamp": "2026-08-17T00:00:05Z",
			"message": map[string]any{"role": "user", "content": "检查结果"},
		},
		{
			"type": "assistant", "uuid": "transcript-assistant-followup", "sessionId": sessionID,
			"parentUuid": "transcript-user-followup", "timestamp": "2026-08-17T00:00:06Z",
			"message": map[string]any{
				"id": "assistant-followup", "role": "assistant", "stop_reason": "end_turn",
				"content": []map[string]any{{"type": "text", "text": "结果正常。"}},
			},
		},
	})

	latest := assertAgentHistoryPageParity(t, history, workspacePath, session, nil, 1, "", 0, "", 0)
	waitHistoryPageIndexFuture(t, history.historyPageAccess(workspacePath, session).Scope)
	assertAgentHistoryPageParity(t, history, workspacePath, session, nil, 1, "", 0, "", 0)
	assertAgentHistoryPageParity(
		t,
		history,
		workspacePath,
		session,
		nil,
		1,
		derefString(latest.NextBeforeRoundID),
		derefInt64(latest.NextBeforeRoundTimestamp),
		"",
		0,
	)
	assertAgentHistoryPageParity(t, history, workspacePath, session, nil, 0, "", 0, "round-tool", 1)
}

func TestRoomHistoryPageIndexTracksPrivateTranscriptDependencies(t *testing.T) {
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
	appendRoomHistoryIndexRound(t, history, ownerUserID, conversationID, "root-next", "amy", 2000)

	latest := assertRoomHistoryPageParity(t, history, ownerUserID, conversationID, nil, 1, "", 0, "", 0)
	access := history.historyPageAccess(ownerUserID, conversationID)
	waitHistoryPageIndexFuture(t, access.Scope)
	assertRoomHistoryPageParity(t, history, ownerUserID, conversationID, nil, 1, "", 0, "", 0)
	assertRoomHistoryPageParity(
		t,
		history,
		ownerUserID,
		conversationID,
		nil,
		1,
		derefString(latest.NextBeforeRoundID),
		derefInt64(latest.NextBeforeRoundTimestamp),
		"",
		0,
	)
	assertRoomHistoryPageParity(t, history, ownerUserID, conversationID, nil, 0, "", 0, "root-private", 1)

	db, err := access.ReadModel.database(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	modelScope, ok, err := readHistoryReadModelScope(context.Background(), db, access.Scope)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("Room read model scope 未落盘")
	}
	kinds := make(map[string]bool)
	for _, source := range modelScope.Sources {
		kinds[source.Kind] = true
	}
	for _, kind := range []string{
		historyPageSourceRoomLedger,
		historyPageSourceRoomPrivateOverlay,
		historyPageSourceTranscript,
	} {
		if !kinds[kind] {
			t.Fatalf("Room index 缺少依赖 source %q: %+v", kind, modelScope.Sources)
		}
	}
}

func TestHistoryPageIndexActiveRoundsSuppressSyntheticInterrupt(t *testing.T) {
	t.Run("dm", func(t *testing.T) {
		root := t.TempDir()
		workspacePath := filepath.Join(root, "Amy")
		session := protocol.Session{SessionKey: "agent:amy:ws:dm:index-active", AgentID: "amy"}
		history := NewAgentHistoryStore(root)
		writeAgentHistoryIndexSession(t, history, workspacePath, session)
		if err := history.AppendOverlayMessage(workspacePath, session.SessionKey, protocol.Message{
			"message_id": "active-user", "round_id": "dm-active", "role": "user",
			"content": "仍在运行", "timestamp": int64(1000),
		}); err != nil {
			t.Fatal(err)
		}
		active := assertAgentHistoryPageParity(t, history, workspacePath, session, []string{"dm-active"}, 1, "", 0, "", 0)
		waitHistoryPageIndexFuture(t, history.historyPageAccess(workspacePath, session).Scope)
		if historyPageHasMessage(active, "assistant_interrupt_dm-active") {
			t.Fatalf("active DM 不应显示 synthetic interrupt: %+v", active.Items)
		}
		inactive := assertAgentHistoryPageParity(t, history, workspacePath, session, nil, 1, "", 0, "", 0)
		if !historyPageHasMessage(inactive, "assistant_interrupt_dm-active") {
			t.Fatalf("inactive DM 应恢复 synthetic interrupt: %+v", inactive.Items)
		}
	})

	t.Run("room_physical_subround", func(t *testing.T) {
		root := t.TempDir()
		history := newRoomHistoryTestStore(t, root)
		ownerUserID := testRoomOwnerUserID
		conversationID := "room-index-active"
		if err := history.AppendInlineMessage(ownerUserID, conversationID, protocol.Message{
			"message_id": "room-active-user", "round_id": "root-active:amy", "agent_id": "amy",
			"role": "user", "content": "仍在运行", "timestamp": int64(1000),
		}); err != nil {
			t.Fatal(err)
		}
		active := assertRoomHistoryPageParity(
			t,
			history,
			ownerUserID,
			conversationID,
			[]string{"root-active:amy"},
			1,
			"",
			0,
			"",
			0,
		)
		waitHistoryPageIndexFuture(t, history.historyPageAccess(ownerUserID, conversationID).Scope)
		if historyPageHasMessage(active, "assistant_interrupt_root-active:amy") {
			t.Fatalf("active Room physical subround 不应显示 synthetic interrupt: %+v", active.Items)
		}
		inactive := assertRoomHistoryPageParity(t, history, ownerUserID, conversationID, nil, 1, "", 0, "", 0)
		if !historyPageHasMessage(inactive, "assistant_interrupt_root-active:amy") {
			t.Fatalf("inactive Room physical subround 应恢复 synthetic interrupt: %+v", inactive.Items)
		}
	})
}

func TestHistoryPageIndexActiveInterleavedRoundsRecomputeLogicalSelection(t *testing.T) {
	t.Run("dm", func(t *testing.T) {
		root := t.TempDir()
		workspacePath := filepath.Join(root, "Amy")
		session := protocol.Session{SessionKey: "agent:amy:ws:dm:index-active-interleaved", AgentID: "amy"}
		history := NewAgentHistoryStore(root)
		writeAgentHistoryIndexSession(t, history, workspacePath, session)
		for _, row := range []protocol.Message{
			{
				"message_id": "user-active-a", "round_id": "active-a", "role": "user",
				"content": "A", "timestamp": int64(1000),
			},
			{
				"message_id": "user-active-b", "round_id": "active-b", "role": "user",
				"content": "B", "timestamp": int64(2000),
			},
			{
				"message_id": "assistant-active-a", "round_id": "active-a", "role": "assistant",
				"content":   []map[string]any{{"type": "text", "text": "A partial"}},
				"timestamp": int64(2001),
			},
		} {
			if err := history.AppendOverlayMessage(workspacePath, session.SessionKey, row); err != nil {
				t.Fatal(err)
			}
		}

		cold := assertAgentHistoryPageParity(
			t,
			history,
			workspacePath,
			session,
			[]string{"active-a"},
			1,
			"",
			0,
			"",
			0,
		)
		if !historyPageHasMessage(cold, "assistant_interrupt_active-b") {
			t.Fatalf("active latest physical group 隐藏后应选中 B synthetic group: %+v", cold.Items)
		}
		access := history.historyPageAccess(workspacePath, session)
		waitHistoryPageIndexFuture(t, access.Scope)
		warm := assertAgentHistoryPageParity(
			t,
			history,
			workspacePath,
			session,
			[]string{"active-a"},
			1,
			"",
			0,
			"",
			0,
		)
		if !historyPageHasMessage(warm, "assistant_interrupt_active-b") {
			t.Fatalf("persisted warm index 应选中 B synthetic group: %+v", warm.Items)
		}
	})

	t.Run("dm_hidden_middle_remerges_round", func(t *testing.T) {
		root := t.TempDir()
		workspacePath := filepath.Join(root, "Amy")
		session := protocol.Session{SessionKey: "agent:amy:ws:dm:index-active-remerge", AgentID: "amy"}
		history := NewAgentHistoryStore(root)
		writeAgentHistoryIndexSession(t, history, workspacePath, session)
		for _, row := range []protocol.Message{
			{
				"message_id": "user-active-a", "round_id": "active-a", "role": "user",
				"content": "A", "timestamp": int64(1000),
			},
			{
				"message_id": "user-active-b", "round_id": "active-b", "role": "user",
				"content": "B", "timestamp": int64(2000),
			},
			{
				"message_id": "assistant-active-a", "round_id": "active-a", "role": "assistant",
				"content":   []map[string]any{{"type": "text", "text": "A partial"}},
				"timestamp": int64(2001),
			},
		} {
			if err := history.AppendOverlayMessage(workspacePath, session.SessionKey, row); err != nil {
				t.Fatal(err)
			}
		}

		assertMergedRound := func(phase string, page protocol.MessagePage) {
			t.Helper()
			if !historyPageHasMessage(page, "assistant-active-a") ||
				!historyPageHasMessage(page, "assistant_interrupt_active-a") {
				t.Fatalf("%s: 隐藏中间 B synthetic 后 A physical groups 未合并: %+v", phase, page.Items)
			}
			if historyPageHasMessage(page, "assistant_interrupt_active-b") {
				t.Fatalf("%s: active B synthetic 不应出现在页面: %+v", phase, page.Items)
			}
		}

		cold := assertAgentHistoryPageParity(
			t,
			history,
			workspacePath,
			session,
			[]string{"active-b"},
			1,
			"",
			0,
			"",
			0,
		)
		assertMergedRound("cold", cold)
		access := history.historyPageAccess(workspacePath, session)
		waitHistoryPageIndexFuture(t, access.Scope)
		warm := assertAgentHistoryPageParity(
			t,
			history,
			workspacePath,
			session,
			[]string{"active-b"},
			1,
			"",
			0,
			"",
			0,
		)
		assertMergedRound("persisted warm", warm)
	})

	t.Run("room", func(t *testing.T) {
		root := t.TempDir()
		history := newRoomHistoryTestStore(t, root)
		ownerUserID := testRoomOwnerUserID
		conversationID := "room-index-active-interleaved"
		for _, row := range []protocol.Message{
			{
				"message_id": "room-user-active-a", "round_id": "root-active-a:amy", "agent_id": "amy",
				"role": "user", "content": "A", "timestamp": int64(1000),
			},
			{
				"message_id": "room-user-active-b", "round_id": "root-active-b:bob", "agent_id": "bob",
				"role": "user", "content": "B", "timestamp": int64(2000),
			},
			{
				"message_id": "room-assistant-active-a", "round_id": "root-active-a:amy", "agent_id": "amy",
				"role": "assistant", "content": []map[string]any{{"type": "text", "text": "A partial"}},
				"timestamp": int64(2001),
			},
		} {
			if err := history.AppendInlineMessage(ownerUserID, conversationID, row); err != nil {
				t.Fatal(err)
			}
		}

		cold := assertRoomHistoryPageParity(
			t,
			history,
			ownerUserID,
			conversationID,
			[]string{"root-active-a:amy"},
			1,
			"",
			0,
			"",
			0,
		)
		if !historyPageHasMessage(cold, "assistant_interrupt_root-active-b:bob") {
			t.Fatalf("active Room physical group 隐藏后应选中 B synthetic group: %+v", cold.Items)
		}
		access := history.historyPageAccess(ownerUserID, conversationID)
		waitHistoryPageIndexFuture(t, access.Scope)
		warm := assertRoomHistoryPageParity(
			t,
			history,
			ownerUserID,
			conversationID,
			[]string{"root-active-a:amy"},
			1,
			"",
			0,
			"",
			0,
		)
		if !historyPageHasMessage(warm, "assistant_interrupt_root-active-b:bob") {
			t.Fatalf("persisted warm Room index 应选中 B synthetic group: %+v", warm.Items)
		}
	})
}

func TestHistoryPageIndexReadsOnlySelectedGroupsAndRebuildsCorruption(t *testing.T) {
	root := t.TempDir()
	workspacePath := filepath.Join(root, "Amy")
	session := protocol.Session{SessionKey: "agent:amy:ws:dm:index-corrupt", AgentID: "amy"}
	history := NewAgentHistoryStore(root)
	writeAgentHistoryIndexSession(t, history, workspacePath, session)
	for index := 1; index <= 6; index++ {
		appendAgentHistoryIndexRound(
			t,
			history,
			workspacePath,
			session.SessionKey,
			"round-"+string(rune('0'+index)),
			int64(index*1000),
		)
	}
	assertAgentHistoryPageParity(t, history, workspacePath, session, nil, 1, "", 0, "", 0)
	access := history.historyPageAccess(workspacePath, session)
	waitHistoryPageIndexFuture(t, access.Scope)
	db, err := access.ReadModel.database(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	modelScope, ok, err := readHistoryReadModelScope(context.Background(), db, access.Scope)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("DM read model scope 未落盘")
	}
	metadata, err := readHistoryReadModelMetadataRange(
		context.Background(), db, access.Scope, modelScope.Generation, 0, modelScope.GroupCount,
	)
	if err != nil || len(metadata) == 0 {
		t.Fatalf("读取 read model metadata: len=%d err=%v", len(metadata), err)
	}
	oldest := metadata[0]
	if _, err = db.ExecContext(
		context.Background(),
		`UPDATE history_read_groups SET payload = ?
		 WHERE scope = ? AND generation = ? AND sequence = 0`,
		[]byte("["),
		access.Scope,
		modelScope.Generation,
	); err != nil {
		t.Fatal(err)
	}

	// 最新页不触碰已损坏的最旧 group，证明不是把整个 data JSON 全量加载。
	assertAgentHistoryPageParity(t, history, workspacePath, session, nil, 1, "", 0, "", 0)
	// 定位损坏 group 时必须从 canonical 安全回建，不能丢历史。
	assertAgentHistoryPageParity(t, history, workspacePath, session, nil, 0, "", 0, oldest.CursorRoundID, 1)
	waitHistoryPageIndexFuture(t, access.Scope)

	if _, err = db.ExecContext(
		context.Background(),
		`UPDATE history_read_scopes SET sources_json = ? WHERE scope = ?`,
		[]byte("{"),
		access.Scope,
	); err != nil {
		t.Fatal(err)
	}
	assertAgentHistoryPageParity(t, history, workspacePath, session, nil, 1, "", 0, "", 0)
	waitHistoryPageIndexFuture(t, access.Scope)

	if _, err = db.ExecContext(
		context.Background(),
		`UPDATE history_read_scopes SET round_index_json = ? WHERE scope = ?`,
		[]byte("{"),
		access.Scope,
	); err != nil {
		t.Fatal(err)
	}
	// 消息页不得读取完整 Navigator payload。
	assertAgentHistoryPageParity(t, history, workspacePath, session, nil, 1, "", 0, "", 0)
	if _, err = history.ReadRoundIndexPageContext(
		context.Background(), workspacePath, session, nil, false,
	); err != nil {
		t.Fatal(err)
	}
	waitHistoryPageIndexFuture(t, access.Scope)
}

func TestHistoryPageRebuildContinuesAfterWaiterCancellationAndSingleflights(t *testing.T) {
	rootPath := t.TempDir()
	started := make(chan struct{})
	release := make(chan struct{})
	var builds atomic.Int32
	access := historyPageIndexAccess{
		Scope:     "cancel-singleflight-" + rootPath,
		ReadModel: sharedHistoryReadModel(rootPath),
		OpenRoot: func(bool) (*confinedfs.Root, error) {
			return confinedfs.Open(rootPath)
		},
		ValidateSources: func(context.Context, []historyPageSourceSnapshot) (bool, error) {
			return true, nil
		},
		Build: func(ctx context.Context) (historyPageIndexBuild, error) {
			if builds.Add(1) == 1 {
				close(started)
			}
			select {
			case <-release:
			case <-ctx.Done():
				return historyPageIndexBuild{}, ctx.Err()
			}
			return testHistoryPageBuild("round-singleflight"), nil
		},
	}

	firstCtx, cancelFirst := context.WithCancel(context.Background())
	firstDone := make(chan error, 1)
	go func() {
		_, err := readHistoryPageWithIndex(firstCtx, access, historyPageIndexRequest{Limit: 1})
		firstDone <- err
	}()
	<-started
	cancelFirst()
	if err := <-firstDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("首个 waiter 应可取消: %v", err)
	}

	secondDone := make(chan error, 1)
	go func() {
		page, err := readHistoryPageWithIndex(context.Background(), access, historyPageIndexRequest{Limit: 1})
		if err == nil && (len(page.Items) != 1 || page.Items[0]["round_id"] != "round-singleflight") {
			err = errors.New("second waiter received wrong page")
		}
		secondDone <- err
	}()
	close(release)
	if err := <-secondDone; err != nil {
		t.Fatal(err)
	}
	waitHistoryPageIndexFuture(t, access.Scope)
	if builds.Load() != 1 {
		t.Fatalf("同 scope rebuild 次数=%d, want 1", builds.Load())
	}
	db, err := access.ReadModel.database(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, err := readHistoryReadModelScope(context.Background(), db, access.Scope); err != nil || !ok {
		t.Fatalf("waiter 取消后后台 rebuild 未落盘: ok=%v err=%v", ok, err)
	}
}

func TestHistoryPageRebuildManagerBoundsUniqueScopesAndAttachesQueuedWaiters(t *testing.T) {
	manager := newHistoryPageRebuildManager(2)
	blockerStarted := make(chan string, 2)
	blockerRelease := []chan struct{}{make(chan struct{}), make(chan struct{})}
	blockerAccess := func(index int) historyPageIndexAccess {
		return historyPageIndexAccess{
			Scope: fmt.Sprintf("blocker-%d", index),
			Build: func(ctx context.Context) (historyPageIndexBuild, error) {
				blockerStarted <- fmt.Sprintf("blocker-%d", index)
				select {
				case <-blockerRelease[index]:
					built := testHistoryPageBuild(fmt.Sprintf("blocker-%d", index))
					built.Cacheable = false
					return built, nil
				case <-ctx.Done():
					return historyPageIndexBuild{}, ctx.Err()
				}
			},
		}
	}
	blockers := make([]*historyPageRebuildFuture, 0, 2)
	for index := 0; index < 2; index++ {
		future, err := manager.start(context.Background(), blockerAccess(index))
		if err != nil {
			t.Fatal(err)
		}
		blockers = append(blockers, future)
	}
	<-blockerStarted
	<-blockerStarted

	// 唯一 scope 的等待只占已有 HTTP goroutine，不创建 future、detached
	// goroutine 或 manager queue；请求取消后 manager 状态仍严格等于 active slots。
	const canceledScopes = 64
	canceled := make(chan error, canceledScopes)
	for index := 0; index < canceledScopes; index++ {
		ctx, cancel := context.WithCancel(context.Background())
		go func(index int) {
			_, err := manager.start(ctx, historyPageIndexAccess{
				Scope: fmt.Sprintf("canceled-%d", index),
				Build: func(context.Context) (historyPageIndexBuild, error) {
					t.Errorf("canceled unique scope 不应启动 build: %d", index)
					return historyPageIndexBuild{}, nil
				},
			})
			canceled <- err
		}(index)
		cancel()
	}
	for index := 0; index < canceledScopes; index++ {
		if err := <-canceled; !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled admission error=%v", err)
		}
	}
	manager.mu.Lock()
	if len(manager.futures) != 2 || len(manager.admission) != 2 {
		t.Fatalf("bounded manager state futures=%d admission=%d", len(manager.futures), len(manager.admission))
	}
	manager.mu.Unlock()

	targetStarted := make(chan struct{})
	targetRelease := make(chan struct{})
	var targetBuilds atomic.Int32
	targetAccess := historyPageIndexAccess{
		Scope: "queued-same-scope",
		Build: func(ctx context.Context) (historyPageIndexBuild, error) {
			targetBuilds.Add(1)
			close(targetStarted)
			select {
			case <-targetRelease:
				built := testHistoryPageBuild("queued-same-scope")
				built.Cacheable = false
				return built, nil
			case <-ctx.Done():
				return historyPageIndexBuild{}, ctx.Err()
			}
		},
	}
	type startResult struct {
		future *historyPageRebuildFuture
		err    error
	}
	results := make(chan startResult, 2)
	for range 2 {
		go func() {
			future, err := manager.start(context.Background(), targetAccess)
			results <- startResult{future: future, err: err}
		}()
	}
	select {
	case result := <-results:
		t.Fatalf("slots 满时 unique scope 不应提前 admission: %+v", result)
	case <-time.After(20 * time.Millisecond):
	}

	close(blockerRelease[0])
	<-targetStarted
	first := <-results
	second := <-results
	if first.err != nil || second.err != nil || first.future != second.future {
		t.Fatalf("同 scope waiter 未 attach 同一 future: first=%+v second=%+v", first, second)
	}
	if targetBuilds.Load() != 1 {
		t.Fatalf("同 scope queued build 次数=%d, want 1", targetBuilds.Load())
	}
	close(targetRelease)
	close(blockerRelease[1])
	<-first.future.complete
	for _, blocker := range blockers {
		<-blocker.complete
	}
}

func TestAgentHistoryPageSourcePreflightSkipsOversizedCanonicalRead(t *testing.T) {
	root := t.TempDir()
	workspacePath := filepath.Join(root, "Amy")
	session := protocol.Session{SessionKey: "agent:amy:ws:dm:source-preflight", AgentID: "amy"}
	history := NewAgentHistoryStore(root)
	writeAgentHistoryIndexSession(t, history, workspacePath, session)
	overlayPath := history.paths.SessionOverlayPath(workspacePath, session.SessionKey)
	if err := os.WriteFile(overlayPath, []byte("{not-json-and-too-large\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	built, err := history.buildAgentHistoryPageIndexWithSourceLimit(
		context.Background(),
		workspacePath,
		session,
		1,
	)
	if err != nil {
		t.Fatalf("source preflight 不应解析 oversized overlay: %v", err)
	}
	if !built.Disabled || !built.Cacheable || len(built.Groups) != 0 || len(built.Sources) != 1 {
		t.Fatalf("DM source preflight result=%+v", built)
	}
}

func TestRoomHistoryPageSourcePreflightStopsBeforePrivateTranscriptResolve(t *testing.T) {
	stateRoot := t.TempDir()
	history := newRoomHistoryTestStore(t, stateRoot)
	ownerUserID := testRoomOwnerUserID
	conversationID := "room-source-preflight"
	workspacePath := filepath.Join(appfs.UserWorkspaceRootAt(stateRoot, ownerUserID), "Amy")
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatal(err)
	}
	privateSessionKey := "agent:amy:ws:group:" + conversationID
	sessionID := "9d928f9e-31bd-48a0-9cd5-21a804772009"
	ownerHistory := history.agentHistory.ForOwner(ownerUserID)
	writeAgentHistoryIndexSession(t, ownerHistory, workspacePath, protocol.Session{
		SessionKey: privateSessionKey,
		AgentID:    "amy",
		SessionID:  &sessionID,
	})
	if err := ownerHistory.AppendRoundMarker(
		workspacePath,
		privateSessionKey,
		"private-source-preflight",
		"处理任务",
		1000,
	); err != nil {
		t.Fatal(err)
	}
	writeAgentTranscriptFixture(t, workspacePath, sessionID, []map[string]any{{
		"type": "user", "uuid": "source-preflight-user", "sessionId": sessionID,
		"timestamp": "2026-08-17T00:00:00Z",
		"message":   map[string]any{"role": "user", "content": "处理任务"},
	}})
	transcriptPath := filepath.Join(
		transcriptProjectsDirForWorkspace(workspacePath),
		sanitizeTranscriptPath(canonicalizeTranscriptPath(workspacePath)),
		sessionID+".jsonl",
	)
	if err := os.WriteFile(transcriptPath, []byte("{invalid-private-transcript\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := history.AppendTranscriptReference(
		ownerUserID,
		conversationID,
		workspacePath,
		privateSessionKey,
		protocol.Message{
			"message_id": "private-source-assistant", "session_id": sessionID,
			"conversation_id": conversationID, "agent_id": "amy",
			"round_id": "root-source:amy", "agent_round_id": "private-source-preflight",
			"role": "assistant", "is_complete": true, "timestamp": int64(1100),
		},
	); err != nil {
		t.Fatal(err)
	}
	ledger, err := history.snapshotRoomHistoryLedger(context.Background(), ownerUserID, conversationID)
	if err != nil {
		t.Fatal(err)
	}
	built, err := history.buildRoomHistoryPageIndexWithSourceLimit(
		context.Background(),
		ownerUserID,
		conversationID,
		ledger.Size+1,
	)
	if err != nil {
		t.Fatalf("Room dependency preflight 不应 resolve oversized private transcript: %v", err)
	}
	if !built.Disabled || !built.Cacheable || len(built.Groups) != 0 || len(built.Sources) < 2 {
		t.Fatalf("Room source preflight result=%+v", built)
	}
}

func TestHistoryPageReadyFutureRejectsSourceChangeDuringPersist(t *testing.T) {
	rootPath := t.TempDir()
	persistStarted := make(chan struct{})
	releasePersist := make(chan struct{})
	var revision atomic.Int64
	revision.Store(1)
	var builds atomic.Int32
	access := historyPageIndexAccess{
		Scope: "ready-source-change-" + rootPath,
		OpenRoot: func(bool) (*confinedfs.Root, error) {
			return confinedfs.Open(rootPath)
		},
		ValidateSources: func(_ context.Context, sources []historyPageSourceSnapshot) (bool, error) {
			return len(sources) == 1 && sources[0].Size == revision.Load(), nil
		},
		Build: func(context.Context) (historyPageIndexBuild, error) {
			current := revision.Load()
			builds.Add(1)
			built := testHistoryPageBuild("round-revision-" + string(rune('0'+current)))
			built.Sources = []historyPageSourceSnapshot{{Kind: "test", Exists: true, Size: current}}
			return built, nil
		},
		Persist: func(context.Context, historyPageIndexBuild) error {
			if builds.Load() == 1 {
				close(persistStarted)
				<-releasePersist
			}
			return nil
		},
	}

	first, err := readHistoryPageWithIndex(context.Background(), access, historyPageIndexRequest{Limit: 1})
	if err != nil || first.Items[0]["round_id"] != "round-revision-1" {
		t.Fatalf("first generation: page=%+v err=%v", first, err)
	}
	<-persistStarted
	revision.Store(2)
	secondDone := make(chan protocol.MessagePage, 1)
	errDone := make(chan error, 1)
	go func() {
		page, readErr := readHistoryPageWithIndex(context.Background(), access, historyPageIndexRequest{Limit: 1})
		secondDone <- page
		errDone <- readErr
	}()
	close(releasePersist)
	second := <-secondDone
	if err = <-errDone; err != nil {
		t.Fatal(err)
	}
	if second.Items[0]["round_id"] != "round-revision-2" || builds.Load() != 2 {
		t.Fatalf("source change 复用了 stale ready future: page=%+v builds=%d", second, builds.Load())
	}
	waitHistoryPageIndexFuture(t, access.Scope)
}

func TestDetachedHistoryPagePersistDoesNotResurrectDeletedContainers(t *testing.T) {
	t.Run("agent", func(t *testing.T) {
		root := t.TempDir()
		workspacePath := filepath.Join(root, "Amy")
		session := protocol.Session{SessionKey: "agent:amy:ws:dm:deleted-index", AgentID: "amy"}
		history := NewAgentHistoryStore(root)
		writeAgentHistoryIndexSession(t, history, workspacePath, session)
		appendAgentHistoryIndexRound(t, history, workspacePath, session.SessionKey, "round-1", 1000)
		access := history.historyPageAccess(workspacePath, session)
		started := make(chan struct{})
		release := make(chan struct{})
		access.Build = func(context.Context) (historyPageIndexBuild, error) {
			close(started)
			<-release
			return testHistoryPageBuild("round-1"), nil
		}
		future, startErr := historyPageRebuilds.start(context.Background(), access)
		if startErr != nil {
			t.Fatal(startErr)
		}
		<-started
		sessionDir := history.paths.SessionDir(workspacePath, session.SessionKey)
		if err := os.RemoveAll(sessionDir); err != nil {
			t.Fatal(err)
		}
		close(release)
		<-future.complete
		if _, err := os.Stat(sessionDir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("detached persist 复活了已删除 session: %v", err)
		}
	})

	t.Run("agent_meta_removed", func(t *testing.T) {
		root := t.TempDir()
		workspacePath := filepath.Join(root, "Amy")
		session := protocol.Session{SessionKey: "agent:amy:ws:dm:deleted-meta-index", AgentID: "amy"}
		history := NewAgentHistoryStore(root)
		writeAgentHistoryIndexSession(t, history, workspacePath, session)
		appendAgentHistoryIndexRound(t, history, workspacePath, session.SessionKey, "round-1", 1000)
		access := history.historyPageAccess(workspacePath, session)
		started := make(chan struct{})
		release := make(chan struct{})
		access.Build = func(context.Context) (historyPageIndexBuild, error) {
			close(started)
			<-release
			return testHistoryPageBuild("round-1"), nil
		}
		future, startErr := historyPageRebuilds.start(context.Background(), access)
		if startErr != nil {
			t.Fatal(startErr)
		}
		<-started
		if err := os.Remove(history.paths.SessionMetaPath(workspacePath, session.SessionKey)); err != nil {
			t.Fatal(err)
		}
		close(release)
		<-future.complete
		db, err := access.ReadModel.database(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if _, ok, err := readHistoryReadModelScope(context.Background(), db, access.Scope); err != nil || ok {
			t.Fatalf("meta 删除后仍写入派生索引: ok=%v err=%v", ok, err)
		}
	})

	t.Run("room", func(t *testing.T) {
		root := t.TempDir()
		history := newRoomHistoryTestStore(t, root)
		ownerUserID := testRoomOwnerUserID
		conversationID := "deleted-room-index"
		appendRoomHistoryIndexRound(t, history, ownerUserID, conversationID, "round-1", "amy", 1000)
		access := history.historyPageAccess(ownerUserID, conversationID)
		started := make(chan struct{})
		release := make(chan struct{})
		access.Build = func(context.Context) (historyPageIndexBuild, error) {
			close(started)
			<-release
			return testHistoryPageBuild("round-1"), nil
		}
		future, startErr := historyPageRebuilds.start(context.Background(), access)
		if startErr != nil {
			t.Fatal(startErr)
		}
		<-started
		conversationDir := history.paths.RoomConversationDir(ownerUserID, conversationID)
		if err := os.RemoveAll(conversationDir); err != nil {
			t.Fatal(err)
		}
		close(release)
		<-future.complete
		if _, err := os.Stat(conversationDir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("detached persist 复活了已删除 conversation: %v", err)
		}
	})
}

func TestHistoryPageDeferredColdBuildSurvivesShortRequest(t *testing.T) {
	rootPath := t.TempDir()
	started := make(chan struct{})
	release := make(chan struct{})
	var builds atomic.Int32
	access := historyPageIndexAccess{
		Scope:     "deferred-cold-" + rootPath,
		ReadModel: sharedHistoryReadModel(rootPath),
		OpenRoot: func(bool) (*confinedfs.Root, error) {
			return confinedfs.Open(rootPath)
		},
		ValidateSources: func(context.Context, []historyPageSourceSnapshot) (bool, error) {
			return true, nil
		},
		Build: func(ctx context.Context) (historyPageIndexBuild, error) {
			if builds.Add(1) == 1 {
				close(started)
			}
			select {
			case <-release:
				return testHistoryPageBuild("round-deferred"), nil
			case <-ctx.Done():
				return historyPageIndexBuild{}, ctx.Err()
			}
		},
	}

	begin := time.Now()
	page, err := readHistoryPageWithIndex(
		context.Background(),
		access,
		historyPageIndexRequest{Limit: 1, DeferIndex: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !page.Indexing || page.RetryAfterMS <= 0 || len(page.Items) != 0 {
		t.Fatalf("cold request 应返回可重试 indexing page: %+v", page)
	}
	if elapsed := time.Since(begin); elapsed > time.Second {
		t.Fatalf("cold request 被后台 build 阻塞: %v", elapsed)
	}
	<-started
	close(release)
	waitHistoryPageIndexFuture(t, access.Scope)

	warm, err := readHistoryPageWithIndex(
		context.Background(),
		access,
		historyPageIndexRequest{Limit: 1, DeferIndex: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if warm.Indexing || len(warm.Items) != 1 || warm.Items[0]["round_id"] != "round-deferred" {
		t.Fatalf("后台 build 完成后未命中 read model: %+v", warm)
	}
	if builds.Load() != 1 {
		t.Fatalf("short request 后重复 build: %d", builds.Load())
	}
}

func TestHistoryPageDeferredColdBuildDoesNotWaitForAdmission(t *testing.T) {
	started := make(chan struct{}, historyPageRebuildWorkers)
	releases := make([]chan struct{}, historyPageRebuildWorkers)
	futures := make([]*historyPageRebuildFuture, 0, historyPageRebuildWorkers)
	for index := 0; index < historyPageRebuildWorkers; index++ {
		releases[index] = make(chan struct{})
		release := releases[index]
		access := historyPageIndexAccess{
			Scope: fmt.Sprintf("deferred-admission-blocker-%d-%s", index, t.Name()),
			Build: func(ctx context.Context) (historyPageIndexBuild, error) {
				started <- struct{}{}
				select {
				case <-release:
					built := testHistoryPageBuild("blocker")
					built.Cacheable = false
					return built, nil
				case <-ctx.Done():
					return historyPageIndexBuild{}, ctx.Err()
				}
			},
		}
		future, err := historyPageRebuilds.start(context.Background(), access)
		if err != nil {
			t.Fatal(err)
		}
		futures = append(futures, future)
	}
	for range historyPageRebuildWorkers {
		<-started
	}

	begin := time.Now()
	page, err := readHistoryPageWithIndex(
		context.Background(),
		historyPageIndexAccess{Scope: "deferred-admission-target-" + t.Name()},
		historyPageIndexRequest{Limit: 1, DeferIndex: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !page.Indexing || time.Since(begin) > time.Second {
		t.Fatalf("deferred request waited for admission: page=%+v elapsed=%v", page, time.Since(begin))
	}
	for _, release := range releases {
		close(release)
	}
	for _, future := range futures {
		<-future.complete
	}
}

func TestSessionRoundIndexSharesDeferredHistoryBuild(t *testing.T) {
	rootPath := t.TempDir()
	started := make(chan struct{})
	release := make(chan struct{})
	var builds atomic.Int32
	access := historyPageIndexAccess{
		Scope:     "deferred-round-index-" + rootPath,
		ReadModel: sharedHistoryReadModel(rootPath),
		OpenRoot: func(bool) (*confinedfs.Root, error) {
			return confinedfs.Open(rootPath)
		},
		ValidateSources: func(context.Context, []historyPageSourceSnapshot) (bool, error) {
			return true, nil
		},
		Build: func(ctx context.Context) (historyPageIndexBuild, error) {
			if builds.Add(1) == 1 {
				close(started)
			}
			select {
			case <-release:
				built := testHistoryPageBuild("round-navigation")
				built.RoundIndex = []protocol.SessionRoundIndexItem{{
					RoundID:        "round-navigation",
					Title:          "navigation",
					HasUserMessage: true,
				}}
				return built, nil
			case <-ctx.Done():
				return historyPageIndexBuild{}, ctx.Err()
			}
		},
	}

	index, err := readSessionRoundIndexWithModel(
		context.Background(), access, nil, false, true,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !index.Indexing || len(index.Items) != 0 {
		t.Fatalf("cold round index should return retry state: %+v", index)
	}
	<-started
	close(release)
	waitHistoryPageIndexFuture(t, access.Scope)
	warm, err := readSessionRoundIndexWithModel(
		context.Background(), access, nil, false, true,
	)
	if err != nil {
		t.Fatal(err)
	}
	if warm.Indexing || len(warm.Items) != 1 || warm.Items[0].RoundID != "round-navigation" {
		t.Fatalf("warm round index did not reuse read model: %+v", warm)
	}
	if builds.Load() != 1 {
		t.Fatalf("message and navigation generation should singleflight: %d", builds.Load())
	}
}

func TestHistoryReadModelUsesBoundedLatestWindow(t *testing.T) {
	rootPath := t.TempDir()
	model := sharedHistoryReadModel(rootPath)
	access := historyPageIndexAccess{
		Scope:     "bounded-window-" + rootPath,
		ReadModel: model,
		OpenRoot: func(bool) (*confinedfs.Root, error) {
			return confinedfs.Open(rootPath)
		},
		ValidateSources: func(context.Context, []historyPageSourceSnapshot) (bool, error) {
			return true, nil
		},
	}
	groups := make([]historyPageIndexedGroup, 1_000)
	for index := range groups {
		roundID := fmt.Sprintf("round-%04d", index)
		groups[index] = testHistoryPageBuild(roundID).Groups[0]
		groups[index].CursorRoundTimestamp = int64(index + 1)
		groups[index].Items[0]["timestamp"] = int64(index + 1)
	}
	if err := model.persist(context.Background(), access, historyPageIndexBuild{
		Groups:    groups,
		Sources:   []historyPageSourceSnapshot{},
		Cacheable: true,
	}); err != nil {
		t.Fatal(err)
	}
	db, err := model.database(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	scope, ok, err := readHistoryReadModelScope(context.Background(), db, access.Scope)
	if err != nil || !ok {
		t.Fatalf("read model scope: ok=%v err=%v", ok, err)
	}
	metadata, start, end, err := readHistoryReadModelMetadataWindow(
		context.Background(),
		db,
		access.Scope,
		scope,
		historyPageIndexRequest{Limit: 10},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(metadata) != 96 || start != 904 || end != 1_000 {
		t.Fatalf("latest metadata window 未保持固定边界: len=%d range=[%d,%d)", len(metadata), start, end)
	}
	page, ok, err := model.load(
		context.Background(),
		access,
		historyPageIndexRequest{Limit: 10},
	)
	if err != nil || !ok {
		t.Fatalf("load latest read model: ok=%v err=%v", ok, err)
	}
	if len(page.Items) != 10 || page.Items[len(page.Items)-1]["round_id"] != "round-0999" {
		t.Fatalf("bounded window 返回错误页: %+v", page)
	}
}

func TestHistoryReadModelInitializationRetriesAfterCanceledRequest(t *testing.T) {
	model := &historyReadModel{
		path: filepath.Join(t.TempDir(), historyReadModelFileName),
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := model.database(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled initialization err=%v", err)
	}
	db, err := model.database(context.Background())
	if err != nil {
		t.Fatalf("next request should retry initialization: %v", err)
	}
	var version int
	if err = db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != historyReadModelSchemaVersion {
		t.Fatalf("schema version=%d, want %d", version, historyReadModelSchemaVersion)
	}
}

func TestHistoryReadModelRecreatesCorruptDerivedDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), historyReadModelFileName)
	if err := os.WriteFile(path, []byte("not a sqlite database"), 0o600); err != nil {
		t.Fatal(err)
	}
	model := &historyReadModel{path: path}
	db, err := model.database(context.Background())
	if err != nil {
		t.Fatalf("corrupt derived database should be recreated: %v", err)
	}
	var version int
	if err = db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != historyReadModelSchemaVersion {
		t.Fatalf("recreated schema version=%d, want %d", version, historyReadModelSchemaVersion)
	}
}

func TestHistoryReadModelVersionResetDoesNotCarryMigrationChain(t *testing.T) {
	path := filepath.Join(t.TempDir(), historyReadModelFileName)
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = legacy.Exec(`
		CREATE TABLE history_read_scopes (obsolete TEXT);
		CREATE TABLE history_read_groups (obsolete TEXT);
		PRAGMA user_version = 1;
	`); err != nil {
		_ = legacy.Close()
		t.Fatal(err)
	}
	if err = legacy.Close(); err != nil {
		t.Fatal(err)
	}

	model := &historyReadModel{path: path}
	db, err := model.database(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	columns, err := db.Query(`PRAGMA table_info(history_read_scopes)`)
	if err != nil {
		t.Fatal(err)
	}
	defer columns.Close()
	foundScope := false
	foundObsolete := false
	for columns.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue any
		var primaryKey int
		if err = columns.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		foundScope = foundScope || name == "scope"
		foundObsolete = foundObsolete || name == "obsolete"
	}
	if err = columns.Err(); err != nil {
		t.Fatal(err)
	}
	if !foundScope || foundObsolete {
		t.Fatalf("version reset columns: scope=%v obsolete=%v", foundScope, foundObsolete)
	}
}

func TestHistoryReadModelPageUsesOneGenerationSnapshot(t *testing.T) {
	rootPath := t.TempDir()
	model := sharedHistoryReadModel(rootPath)
	access := historyPageIndexAccess{
		Scope:     "generation-snapshot-" + rootPath,
		ReadModel: model,
		OpenRoot: func(bool) (*confinedfs.Root, error) {
			return confinedfs.Open(rootPath)
		},
	}
	oldBuild := testHistoryPageBuild("round-old")
	oldBuild.Sources = []historyPageSourceSnapshot{{Kind: "test", Exists: true, Size: 1}}
	if err := model.persist(context.Background(), access, oldBuild); err != nil {
		t.Fatal(err)
	}
	db, err := model.database(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	readTx, err := db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer readTx.Rollback()
	oldScope, ok, err := readHistoryReadModelScope(context.Background(), readTx, access.Scope)
	if err != nil || !ok {
		t.Fatalf("read old scope: ok=%v err=%v", ok, err)
	}

	newBuild := testHistoryPageBuild("round-new")
	newBuild.Sources = []historyPageSourceSnapshot{{Kind: "test", Exists: true, Size: 2}}
	if err = model.persist(context.Background(), access, newBuild); err != nil {
		t.Fatal(err)
	}
	metadata, err := readHistoryReadModelMetadataRange(
		context.Background(), readTx, access.Scope, oldScope.Generation, 0, 1,
	)
	if err != nil {
		t.Fatal(err)
	}
	groups, err := readHistoryReadModelGroups(
		context.Background(), readTx, access.Scope, oldScope.Generation, metadata, []int{0}, 0,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].CursorRoundID != "round-old" {
		t.Fatalf("generation changed inside one page read: %+v", groups)
	}
}

func appendAgentHistoryIndexRound(
	t *testing.T,
	history *AgentHistoryStore,
	workspacePath string,
	sessionKey string,
	roundID string,
	timestamp int64,
) {
	t.Helper()
	for _, row := range []protocol.Message{
		{
			"message_id": "user-" + roundID,
			"round_id":   roundID,
			"role":       "user",
			"content":    "question " + roundID,
			"timestamp":  timestamp,
		},
		{
			"message_id":  "assistant-" + roundID,
			"round_id":    roundID,
			"role":        "assistant",
			"content":     []map[string]any{{"type": "text", "text": "answer " + roundID}},
			"stop_reason": "end_turn",
			"is_complete": true,
			"timestamp":   timestamp + 1,
		},
	} {
		if err := history.AppendOverlayMessage(workspacePath, sessionKey, row); err != nil {
			t.Fatal(err)
		}
	}
}

func writeAgentHistoryIndexSession(
	t *testing.T,
	history *AgentHistoryStore,
	workspacePath string,
	session protocol.Session,
) {
	t.Helper()
	if _, err := history.files.UpsertSession(workspacePath, session); err != nil {
		t.Fatalf("写入 history index session meta 失败: %v", err)
	}
}

func appendRoomHistoryIndexRound(
	t *testing.T,
	history *RoomHistoryStore,
	ownerUserID string,
	conversationID string,
	roundID string,
	agentID string,
	timestamp int64,
) {
	t.Helper()
	for _, row := range []protocol.Message{
		{
			"message_id": "user-" + roundID,
			"round_id":   roundID,
			"role":       "user",
			"content":    "question " + roundID,
			"timestamp":  timestamp,
		},
		{
			"message_id":  "assistant-" + roundID,
			"round_id":    roundID + ":" + agentID,
			"agent_id":    agentID,
			"role":        "assistant",
			"content":     []map[string]any{{"type": "text", "text": "answer " + roundID}},
			"stop_reason": "end_turn",
			"is_complete": true,
			"timestamp":   timestamp + 1,
		},
	} {
		if err := history.AppendInlineMessage(ownerUserID, conversationID, row); err != nil {
			t.Fatal(err)
		}
	}
}

func assertAgentHistoryPageParity(
	t *testing.T,
	history *AgentHistoryStore,
	workspacePath string,
	session protocol.Session,
	active []string,
	limit int,
	beforeRoundID string,
	beforeTimestamp int64,
	aroundRoundID string,
	aroundLimit int,
) protocol.MessagePage {
	t.Helper()
	raw, err := history.readHistoryRows(workspacePath, session)
	if err != nil {
		t.Fatal(err)
	}
	expected := canonicalHistoryPageOracle(
		raw,
		active,
		limit,
		beforeRoundID,
		beforeTimestamp,
		aroundRoundID,
		aroundLimit,
		false,
	)
	actual, err := history.ReadMessagesPageContext(
		context.Background(),
		workspacePath,
		session,
		active,
		HistoryPageQuery{
			Limit:                limit,
			BeforeRoundID:        beforeRoundID,
			BeforeRoundTimestamp: beforeTimestamp,
			AroundRoundID:        aroundRoundID,
			AroundLimit:          aroundLimit,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	assertHistoryPageJSONEqual(t, expected, actual)
	return actual
}

func assertRoomHistoryPageParity(
	t *testing.T,
	history *RoomHistoryStore,
	ownerUserID string,
	conversationID string,
	active []string,
	limit int,
	beforeRoundID string,
	beforeTimestamp int64,
	aroundRoundID string,
	aroundLimit int,
) protocol.MessagePage {
	t.Helper()
	raw, err := history.readResolvedRows(ownerUserID, conversationID)
	if err != nil {
		t.Fatal(err)
	}
	expected := canonicalHistoryPageOracle(
		raw,
		active,
		limit,
		beforeRoundID,
		beforeTimestamp,
		aroundRoundID,
		aroundLimit,
		true,
	)
	actual, err := history.ReadMessagesPageContext(
		context.Background(),
		ownerUserID,
		conversationID,
		active,
		HistoryPageQuery{
			Limit:                limit,
			BeforeRoundID:        beforeRoundID,
			BeforeRoundTimestamp: beforeTimestamp,
			AroundRoundID:        aroundRoundID,
			AroundLimit:          aroundLimit,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	assertHistoryPageJSONEqual(t, expected, actual)
	return actual
}

func canonicalHistoryPageOracle(
	rows []protocol.Message,
	active []string,
	limit int,
	beforeRoundID string,
	beforeTimestamp int64,
	aroundRoundID string,
	aroundLimit int,
	collapseRoomRounds bool,
) protocol.MessagePage {
	normalized := normalizeHistoryRows(rows, normalizeActiveRoundIDs(active))
	if aroundRoundID != "" {
		return paginateNormalizedHistoryRowsAround(normalized, aroundRoundID, aroundLimit, collapseRoomRounds)
	}
	return paginateNormalizedHistoryRows(
		normalized,
		limit,
		beforeRoundID,
		beforeTimestamp,
		collapseRoomRounds,
	)
}

func assertHistoryPageJSONEqual(t *testing.T, expected protocol.MessagePage, actual protocol.MessagePage) {
	t.Helper()
	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	actualJSON, err := json.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}
	if string(expectedJSON) != string(actualJSON) {
		t.Fatalf("history page 与 canonical oracle 不一致\nexpected=%s\nactual=%s", expectedJSON, actualJSON)
	}
}

func assertSessionRoundIndexJSONEqual(
	t *testing.T,
	expected protocol.SessionRoundIndex,
	actual protocol.SessionRoundIndex,
) {
	t.Helper()
	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	actualJSON, err := json.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}
	if string(expectedJSON) != string(actualJSON) {
		t.Fatalf("round index 与 canonical 不一致\nexpected=%s\nactual=%s", expectedJSON, actualJSON)
	}
}

func historyPageHasMessage(page protocol.MessagePage, messageID string) bool {
	for _, item := range page.Items {
		if stringFromAny(item["message_id"]) == messageID {
			return true
		}
	}
	return false
}

func testHistoryPageBuild(roundID string) historyPageIndexBuild {
	return historyPageIndexBuild{
		Groups: []historyPageIndexedGroup{{
			CursorRoundID:        roundID,
			CursorRoundTimestamp: 1,
			GroupKey:             "round:" + roundID,
			Items: []protocol.Message{{
				"message_id": "message-" + roundID,
				"round_id":   roundID,
				"role":       "user",
				"timestamp":  int64(1),
			}},
		}},
		Sources:   []historyPageSourceSnapshot{},
		Cacheable: true,
	}
}

func waitHistoryPageIndexFuture(t *testing.T, scope string) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		historyPageRebuilds.mu.Lock()
		future := historyPageRebuilds.futures[scope]
		historyPageRebuilds.mu.Unlock()
		if future == nil {
			return
		}
		select {
		case <-future.complete:
		case <-deadline.C:
			t.Fatalf("等待 history page rebuild 超时: scope=%s", scope)
		}
	}
}

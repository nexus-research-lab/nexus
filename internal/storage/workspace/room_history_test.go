// room_history_test.go 覆盖 Room 历史、分页对齐、资源、定向消息与公开交接存储测试。
package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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

func TestRoomDirectedMessageWakeStoreRestoresOnlyPendingWakes(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", root)
	t.Setenv("NEXUS_CONFIG_DIR", root)
	store := NewRoomDirectedMessageWakeStore(root)
	wake := RoomDirectedMessageWake{
		WakeID:      "wake-1",
		OwnerUserID: "owner-1",
		Message: protocol.RoomDirectedMessageRecord{
			MessageID:      "message-1",
			RoomID:         "room-1",
			ConversationID: "conversation-1",
			WakePolicy:     protocol.RoomWakePolicyDelayed,
		},
		DueAt: time.Now().Add(time.Minute).UnixMilli(),
	}
	if err := store.Schedule(wake); err != nil {
		t.Fatalf("写入延迟唤醒失败: %v", err)
	}
	pending, err := NewRoomDirectedMessageWakeStore(root).Pending(wake.OwnerUserID)
	if err != nil || len(pending) != 1 || pending[0].WakeID != wake.WakeID {
		t.Fatalf("延迟唤醒恢复不正确: pending=%+v err=%v", pending, err)
	}
	if err = store.Complete(wake.OwnerUserID, wake.WakeID); err != nil {
		t.Fatalf("完成延迟唤醒失败: %v", err)
	}
	pending, err = NewRoomDirectedMessageWakeStore(root).Pending(wake.OwnerUserID)
	if err != nil || len(pending) != 0 {
		t.Fatalf("已完成唤醒不应在重启后恢复: pending=%+v err=%v", pending, err)
	}
	inserted, err := store.ScheduleIfAbsent(wake)
	if err != nil || inserted {
		t.Fatalf("已完成 wake 的工具重试必须保持终态: inserted=%t err=%v", inserted, err)
	}
	pending, err = NewRoomDirectedMessageWakeStore(root).Pending(wake.OwnerUserID)
	if err != nil || len(pending) != 0 {
		t.Fatalf("工具重试不应复活已完成 wake: pending=%+v err=%v", pending, err)
	}
}

func TestRoomPublicHandoffRejectsIncompleteResponsibilityBinding(t *testing.T) {
	store := NewRoomPublicHandoffStore(t.TempDir())
	_, _, err := store.Detect(testRoomOwnerUserID, RoomPublicHandoff{
		HandoffID:       "execution_dispatch_incomplete",
		ConversationID:  "conversation-handoff",
		SourceMessageID: "execution_dispatch_incomplete",
		SourceAgentID:   "agent-lead",
		TargetAgentID:   "agent-worker",
		Content:         "deliver the result",
		WorkBinding: &protocol.ExecutionWorkBinding{
			ExecutionID:  "execution-1",
			PlanID:       "plan-1",
			WorkItemID:   "work-1",
			SpecID:       "spec-1",
			AssignmentID: "assignment-1",
			DispatchID:   "dispatch-1",
		},
	})
	if !errors.Is(err, protocol.ErrInvalidInputQueueCapabilityEnvelope) {
		t.Fatalf("error = %v, want capability envelope error", err)
	}
}

func TestRoomPublicHandoffStoreListsAndCancelsRoot(t *testing.T) {
	root := t.TempDir()
	conversationID := "conversation-root-cancel"
	store := NewRoomPublicHandoffStore(root)
	store.paths.StateRoot = root
	for _, handoff := range []RoomPublicHandoff{
		{
			HandoffID: "rh-root-a", ConversationID: conversationID, RootRoundID: "root-1",
			SourceMessageID: "message-a", SourceAgentID: "agent-a", TargetAgentID: "agent-b",
		},
		{
			HandoffID: "rh-root-b", ConversationID: conversationID, RootRoundID: "root-1",
			SourceMessageID: "message-b", SourceAgentID: "agent-b", TargetAgentID: "agent-c",
		},
		{
			HandoffID: "rh-other", ConversationID: conversationID, RootRoundID: "root-2",
			SourceMessageID: "message-c", SourceAgentID: "agent-a", TargetAgentID: "agent-d",
		},
	} {
		if _, _, err := store.Detect(testRoomOwnerUserID, handoff); err != nil {
			t.Fatal(err)
		}
		if err := store.MarkSourceFinished(testRoomOwnerUserID, conversationID, handoff.HandoffID); err != nil {
			t.Fatal(err)
		}
	}
	edges, err := store.ListRoot(testRoomOwnerUserID, conversationID, "root-1")
	if err != nil || len(edges) != 2 {
		t.Fatalf("root snapshot 不正确: edges=%+v err=%v", edges, err)
	}
	if err := store.CancelForRoot(testRoomOwnerUserID, conversationID, "root-1", "interrupted"); err != nil {
		t.Fatal(err)
	}
	pending, err := store.Pending(testRoomOwnerUserID, conversationID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].HandoffID != "rh-other" {
		t.Fatalf("取消 root 后只应保留其他 root: %+v", pending)
	}
}

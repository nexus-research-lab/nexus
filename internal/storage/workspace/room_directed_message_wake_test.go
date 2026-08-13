package workspace

import (
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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

func TestRoomDirectedMessageWakeStorePendingAllPreservesOriginalOwner(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", root)
	t.Setenv("NEXUS_CONFIG_DIR", root)
	store := NewRoomDirectedMessageWakeStore(root)
	wake := RoomDirectedMessageWake{
		WakeID:      "wake-unsafe-owner",
		OwnerUserID: "owner/with/slash",
		Message: protocol.RoomDirectedMessageRecord{
			MessageID:      "message-unsafe-owner",
			RoomID:         "room-unsafe-owner",
			ConversationID: "conversation-unsafe-owner",
			WakePolicy:     protocol.RoomWakePolicyDelayed,
		},
		DueAt: time.Now().Add(time.Minute).UnixMilli(),
	}
	if err := store.Schedule(wake); err != nil {
		t.Fatalf("写入特殊 owner 延迟唤醒失败: %v", err)
	}

	pending, err := NewRoomDirectedMessageWakeStore(root).PendingAll()
	if err != nil {
		t.Fatalf("扫描全部延迟唤醒失败: %v", err)
	}
	if len(pending) != 1 || pending[0].OwnerUserID != wake.OwnerUserID {
		t.Fatalf("恢复后 owner 身份失真: pending=%+v", pending)
	}
}

func TestRoomDirectedMessageWakeStorePendingAllRejectsMismatchedOwner(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", root)
	t.Setenv("NEXUS_CONFIG_DIR", root)
	store := NewRoomDirectedMessageWakeStore(root)
	const pathOwnerUserID = "owner-a"
	forgedWake := RoomDirectedMessageWake{
		WakeID:      "wake-forged-owner",
		OwnerUserID: "owner-b",
		Message: protocol.RoomDirectedMessageRecord{
			MessageID:      "message-forged-owner",
			RoomID:         "room-forged-owner",
			ConversationID: "conversation-forged-owner",
			WakePolicy:     protocol.RoomWakePolicyDelayed,
		},
		DueAt: time.Now().Add(time.Minute).UnixMilli(),
	}
	if err := store.files.appendRoomJSONL(
		pathOwnerUserID,
		store.paths.RoomDirectedMessageWakesPath(pathOwnerUserID),
		map[string]any{
			"action": roomWakeActionSchedule,
			"wake":   forgedWake,
		},
	); err != nil {
		t.Fatalf("写入伪造 owner 延迟唤醒失败: %v", err)
	}

	pending, err := NewRoomDirectedMessageWakeStore(root).PendingAll()
	if err != nil {
		t.Fatalf("扫描全部延迟唤醒失败: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("目录与记录 owner 不一致时必须拒绝恢复: pending=%+v", pending)
	}
}

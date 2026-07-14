package workspace

import (
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRoomDirectedMessageWakeStoreRestoresOnlyPendingWakes(t *testing.T) {
	root := t.TempDir()
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
	pending, err := NewRoomDirectedMessageWakeStore(root).Pending()
	if err != nil || len(pending) != 1 || pending[0].WakeID != wake.WakeID {
		t.Fatalf("延迟唤醒恢复不正确: pending=%+v err=%v", pending, err)
	}
	if err = store.Complete(wake.WakeID); err != nil {
		t.Fatalf("完成延迟唤醒失败: %v", err)
	}
	pending, err = NewRoomDirectedMessageWakeStore(root).Pending()
	if err != nil || len(pending) != 0 {
		t.Fatalf("已完成唤醒不应在重启后恢复: pending=%+v err=%v", pending, err)
	}
}

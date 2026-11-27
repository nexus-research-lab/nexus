package channels

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
)

func TestControlServiceNotifiesOnlyCommittedControlChanges(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	var owners []string
	service.SetControlChangeNotifier(func(_ context.Context, ownerUserID string) {
		owners = append(owners, ownerUserID)
	})
	created, err := service.CreatePairing(context.Background(), "owner-a", CreatePairingRequest{
		ChannelType: ChannelTypeTelegram,
		ChatType:    "dm",
		ExternalRef: "chat-a",
		AgentID:     "agent-a",
		Status:      PairingStatusActive,
	})
	if err != nil {
		t.Fatalf("创建配对失败: %v", err)
	}
	if len(owners) != 1 || owners[0] != "owner-a" {
		t.Fatalf("提交成功应恰好通知一次 owner: %v", owners)
	}

	// 版本冲突的写入没有提交，不应产生失效通知。
	if err = service.DeletePairingAtVersion(context.Background(), "owner-a", created.PairingID, 1); err == nil {
		t.Fatal("过期版本删除应失败")
	}
	if len(owners) != 1 {
		t.Fatalf("未提交的写入不应通知: %v", owners)
	}

	if err = service.touchPairing(context.Background(), "owner-a", created.PairingID); err != nil {
		t.Fatalf("刷新最近消息时间失败: %v", err)
	}
	if len(owners) != 2 {
		t.Fatalf("最近消息时间更新应通知配对目录: %v", owners)
	}
}

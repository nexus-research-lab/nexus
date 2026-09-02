package channels

import (
	"context"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
)

func TestPersonalWeixinRuntimeStorePersistsCursorAndMarksExpiredLogin(t *testing.T) {
	db := newChannelTestDB(t)
	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	if _, err := db.Exec(`
INSERT INTO im_channel_accounts (
    owner_user_id, channel_type, account_id, status, config_json, sync_cursor
) VALUES ('owner-a', 'weixin-personal', 'account-1', 'connected', '{}', 'cursor-old')`); err != nil {
		t.Fatal(err)
	}

	cursor, err := service.LoadPersonalWeixinCursor(context.Background(), "owner-a", "account-1")
	if err != nil || cursor != "cursor-old" {
		t.Fatalf("读取个人微信游标失败: cursor=%q err=%v", cursor, err)
	}
	if err = service.SavePersonalWeixinCursor(context.Background(), "owner-a", "account-1", "cursor-new"); err != nil {
		t.Fatalf("保存个人微信游标失败: %v", err)
	}
	cursor, err = service.LoadPersonalWeixinCursor(context.Background(), "owner-a", "account-1")
	if err != nil || cursor != "cursor-new" {
		t.Fatalf("个人微信游标未持久化: cursor=%q err=%v", cursor, err)
	}

	if err = service.MarkPersonalWeixinLoginExpired(
		context.Background(),
		"owner-a",
		"account-1",
		"scan QR code to reconnect",
	); err != nil {
		t.Fatalf("标记个人微信登录失效失败: %v", err)
	}
	var status string
	var lastError string
	if err = db.QueryRow(`
SELECT status, last_error, sync_cursor
FROM im_channel_accounts
WHERE owner_user_id = 'owner-a' AND channel_type = 'weixin-personal' AND account_id = 'account-1'`,
	).Scan(&status, &lastError, &cursor); err != nil {
		t.Fatal(err)
	}
	if status != ChannelConfigStatusError || cursor != "" || !strings.Contains(lastError, "scan QR code") {
		t.Fatalf("个人微信登录失效投影不正确: status=%q cursor=%q last_error=%q", status, cursor, lastError)
	}
}

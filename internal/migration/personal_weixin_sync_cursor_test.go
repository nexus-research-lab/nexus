package migration

import (
	"testing"

	"github.com/pressly/goose/v3"
)

func TestPersonalWeixinSyncCursorMigrationPreservesAccounts(t *testing.T) {
	db := openAgentDisabledSkillMigrationTestDB(t, "personal-weixin-sync-cursor.db")
	migrationDir := providerRecoveryMigrationDir(t)
	if err := goose.UpTo(db, migrationDir, 129); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
INSERT INTO im_channel_accounts (
    owner_user_id, channel_type, account_id, status, config_json
) VALUES ('owner-a', 'weixin-personal', 'account-1', 'connected', '{}')`); err != nil {
		t.Fatal(err)
	}

	if err := goose.UpTo(db, migrationDir, 130); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
UPDATE im_channel_accounts
SET sync_cursor = 'cursor-1'
WHERE owner_user_id = 'owner-a' AND channel_type = 'weixin-personal' AND account_id = 'account-1'`); err != nil {
		t.Fatal(err)
	}
	var cursor string
	if err := db.QueryRow(`
SELECT sync_cursor
FROM im_channel_accounts
WHERE owner_user_id = 'owner-a' AND channel_type = 'weixin-personal' AND account_id = 'account-1'`,
	).Scan(&cursor); err != nil {
		t.Fatal(err)
	}
	if cursor != "cursor-1" {
		t.Fatalf("sync_cursor = %q, want cursor-1", cursor)
	}

	if err := goose.DownTo(db, migrationDir, 129); err != nil {
		t.Fatalf("回滚个人微信游标 migration 失败: %v", err)
	}
	var columns int
	if err := db.QueryRow(`
SELECT COUNT(*)
FROM pragma_table_info('im_channel_accounts')
WHERE name = 'sync_cursor'`).Scan(&columns); err != nil {
		t.Fatal(err)
	}
	var accounts int
	if err := db.QueryRow(`
SELECT COUNT(*)
FROM im_channel_accounts
WHERE owner_user_id = 'owner-a' AND channel_type = 'weixin-personal' AND account_id = 'account-1'`,
	).Scan(&accounts); err != nil {
		t.Fatal(err)
	}
	if columns != 0 || accounts != 1 {
		t.Fatalf("回滚后 schema/data 不正确: sync_cursor_columns=%d accounts=%d", columns, accounts)
	}
}

package configuration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/service/channels"

	_ "modernc.org/sqlite"
)

func TestVerifyDeletedChannelTargetsUsesExactPersistenceState(t *testing.T) {
	db, err := sql.Open("sqlite", fmt.Sprintf(
		"file:%s?mode=memory&cache=shared",
		strings.ReplaceAll(t.Name(), "/", "_"),
	))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(`
CREATE TABLE channel_control_versions (
    owner_user_id TEXT NOT NULL PRIMARY KEY,
    version INTEGER NOT NULL DEFAULT 1,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE TABLE im_channel_configs (
    owner_user_id TEXT NOT NULL,
    channel_type TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'configured',
    config_json TEXT NOT NULL DEFAULT '{}',
    credentials_encrypted TEXT,
    last_error TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (owner_user_id, channel_type)
);
CREATE TABLE im_channel_accounts (
    owner_user_id TEXT NOT NULL,
    channel_type TEXT NOT NULL,
    account_id TEXT NOT NULL,
    user_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'connected',
    config_json TEXT NOT NULL DEFAULT '{}',
    credentials_encrypted TEXT,
    last_error TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (owner_user_id, channel_type, account_id)
);
CREATE TABLE im_pairings (
    pairing_id TEXT NOT NULL PRIMARY KEY,
    owner_user_id TEXT NOT NULL,
    channel_type TEXT NOT NULL,
    account_id TEXT NOT NULL DEFAULT '',
    chat_type TEXT NOT NULL,
    external_ref TEXT NOT NULL,
    thread_id TEXT NOT NULL DEFAULT '',
    external_name TEXT,
    agent_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    source TEXT NOT NULL DEFAULT 'manual',
    last_message_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    UNIQUE (owner_user_id, channel_type, account_id, chat_type, external_ref, thread_id)
);`); err != nil {
		t.Fatal(err)
	}

	channelControl := channels.NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	service := &Service{channels: channelControl}
	actor := &resolvedActor{Actor: Actor{OwnerUserID: "owner-a"}}
	ctx := context.Background()

	if _, err = db.Exec(`
INSERT INTO im_channel_configs (owner_user_id, channel_type, agent_id)
VALUES ('owner-a', 'weixin-personal', 'agent-a');
INSERT INTO im_channel_accounts (owner_user_id, channel_type, account_id)
VALUES ('owner-a', 'weixin-personal', 'account-a');
INSERT INTO im_pairings (
    pairing_id, owner_user_id, channel_type, chat_type, external_ref, agent_id
) VALUES ('pair-a', 'owner-a', 'telegram', 'dm', 'chat-a', 'agent-a');`); err != nil {
		t.Fatal(err)
	}

	configDelete := ChangeRequest{
		Domain: DomainChannels, Operation: "delete_config", Target: channels.ChannelTypeWeixinPersonal,
	}
	if !isTargetDeletion(configDelete) {
		t.Fatal("channel config 删除必须进入写后不存在核验")
	}
	if err = service.verifyDeletedTarget(ctx, actor, configDelete); err == nil {
		t.Fatal("配置或 account 尚存在时核验必须失败")
	}
	if err = channelControl.DeleteChannelConfig(ctx, "owner-a", channels.ChannelTypeWeixinPersonal); err != nil {
		t.Fatal(err)
	}
	if err = service.verifyDeletedTarget(ctx, actor, configDelete); err != nil {
		t.Fatalf("配置和 accounts 已删除后核验应通过: %v", err)
	}

	if _, err = db.Exec(`
INSERT INTO im_channel_configs (owner_user_id, channel_type, agent_id)
VALUES ('owner-a', 'weixin-personal', 'agent-a');
INSERT INTO im_channel_accounts (owner_user_id, channel_type, account_id)
VALUES ('owner-a', 'weixin-personal', 'account-b');`); err != nil {
		t.Fatal(err)
	}
	accountDelete := ChangeRequest{
		Domain: DomainChannels, Operation: "delete_account", Target: channels.ChannelTypeWeixinPersonal,
		Input: json.RawMessage(`{"account_id":"account-b"}`),
	}
	if err = service.verifyDeletedTarget(ctx, actor, accountDelete); err == nil {
		t.Fatal("account 尚存在时核验必须失败")
	}
	if _, err = channelControl.DeleteChannelAccount(
		ctx,
		"owner-a",
		channels.ChannelTypeWeixinPersonal,
		"account-b",
	); err != nil {
		t.Fatal(err)
	}
	if err = service.verifyDeletedTarget(ctx, actor, accountDelete); err != nil {
		t.Fatalf("account 已删除后核验应通过: %v", err)
	}

	pairingDelete := ChangeRequest{
		Domain: DomainChannels, Operation: "delete_pairing", Target: "pair-a",
	}
	if err = service.verifyDeletedTarget(ctx, actor, pairingDelete); err == nil {
		t.Fatal("pairing 尚存在时核验必须失败")
	}
	if err = channelControl.DeletePairing(ctx, "owner-a", "pair-a"); err != nil {
		t.Fatal(err)
	}
	if err = service.verifyDeletedTarget(ctx, actor, pairingDelete); err != nil {
		t.Fatalf("pairing 已删除后核验应通过: %v", err)
	}
	version, err := channelControl.GetChannelControlVersion(ctx, "owner-a")
	if err != nil || version != 4 {
		t.Fatalf("三项 Channel 控制删除应各推进一次 version: version=%d err=%v", version, err)
	}
}

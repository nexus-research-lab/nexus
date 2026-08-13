package channels

import (
	"context"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestResolveExternalSessionIdentityDistinguishesCurrentAndHistoricalAccounts(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)

	if _, err := db.Exec(`
INSERT INTO im_channel_configs (
    owner_user_id, channel_type, agent_id, status, config_json
) VALUES ('owner-a', 'weixin-personal', 'agent-a', 'connected', '{}');
INSERT INTO im_channel_accounts (
    owner_user_id, channel_type, account_id, user_id, status, config_json
) VALUES ('owner-a', 'weixin-personal', 'account-current', 'user-a', 'connected', '{}');
INSERT INTO im_pairings (
    pairing_id, owner_user_id, channel_type, account_id, chat_type,
    external_ref, external_name, agent_id, status, source
) VALUES (
    'pairing-current', 'owner-a', 'weixin-personal', 'account-current', 'dm',
    'contact-a', '联系人甲', 'agent-a', 'active', 'ingress'
);`); err != nil {
		t.Fatalf("准备 IM identity fixture 失败: %v", err)
	}

	currentKey := protocol.BuildAgentAccountSessionKey(
		"agent-a", ChannelTypeWeixinPersonal, "dm", "account-current", "contact-a", "",
	)
	current, err := service.ResolveExternalSessionIdentity(
		context.Background(), "owner-a", currentKey,
	)
	if err != nil {
		t.Fatalf("读取当前 IM identity 失败: %v", err)
	}
	if current == nil || !current.CurrentPairing || current.CanDelete ||
		current.PairingStatus != PairingStatusActive || current.AccountHint == "" {
		t.Fatalf("当前 IM identity 不正确: %+v", current)
	}
	if strings.Contains(current.AccountHint, "account-current") || len(current.AccountHint) != 6 {
		t.Fatalf("账号短标识泄露原始 account_id: %+v", current)
	}

	historicalKey := protocol.BuildAgentAccountSessionKey(
		"agent-a", ChannelTypeWeixinPersonal, "dm", "account-removed", "contact-a", "",
	)
	historical, err := service.ResolveExternalSessionIdentity(
		context.Background(), "owner-a", historicalKey,
	)
	if err != nil {
		t.Fatalf("读取历史 IM identity 失败: %v", err)
	}
	if historical == nil || historical.CurrentPairing || !historical.CanDelete ||
		historical.PairingStatus != "unpaired" || historical.AccountStatus != "removed" ||
		historical.AccountHint == current.AccountHint {
		t.Fatalf("历史 IM identity 不正确: %+v", historical)
	}
}

func TestResolveExternalSessionIdentityTreatsRemovedAccountAsHistorical(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)

	if _, err := db.Exec(`
INSERT INTO im_channel_configs (
    owner_user_id, channel_type, agent_id, status, config_json
) VALUES ('owner-a', 'weixin-personal', 'agent-a', 'connected', '{}');
INSERT INTO im_pairings (
    pairing_id, owner_user_id, channel_type, account_id, chat_type,
    external_ref, agent_id, status, source
) VALUES (
    'stale-pairing', 'owner-a', 'weixin-personal', 'removed-account', 'dm',
    'contact-a', 'agent-a', 'active', 'ingress'
);`); err != nil {
		t.Fatalf("准备 stale pairing fixture 失败: %v", err)
	}

	identity, err := service.ResolveExternalSessionIdentity(
		context.Background(),
		"owner-a",
		protocol.BuildAgentAccountSessionKey(
			"agent-a", ChannelTypeWeixinPersonal, "dm", "removed-account", "contact-a", "",
		),
	)
	if err != nil {
		t.Fatalf("读取 stale pairing identity 失败: %v", err)
	}
	if identity == nil || identity.CurrentPairing || !identity.CanDelete ||
		identity.PairingStatus != PairingStatusActive || identity.AccountStatus != "removed" {
		t.Fatalf("账号已移除时不能继续标为当前: %+v", identity)
	}
}

func TestResolveExternalSessionIdentityLabelsLegacyAccountlessSessions(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	if _, err := db.Exec(`
INSERT INTO im_channel_configs (
    owner_user_id, channel_type, agent_id, status, config_json
) VALUES ('owner-a', 'telegram', 'agent-a', 'connected', '{}')`); err != nil {
		t.Fatalf("准备 legacy session config 失败: %v", err)
	}
	key := protocol.BuildAgentSessionKey(
		"agent-a", ChannelTypeTelegram, "dm", "legacy-contact", "",
	)
	identity, err := service.ResolveExternalSessionIdentity(
		context.Background(), "owner-a", key,
	)
	if err != nil {
		t.Fatalf("读取 legacy session identity 失败: %v", err)
	}
	if identity == nil || identity.AccountHint != "" ||
		len(identity.LegacySessionHint) != 6 {
		t.Fatalf("旧无账号会话缺少稳定短标识: %+v", identity)
	}
}

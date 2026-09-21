package channels

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestControlServiceRecreatedWildcardPairingReplacesStaleConcreteMapping(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	request := CreatePairingRequest{
		ChannelType: ChannelTypeFeishu,
		ChatType:    "group",
		ExternalRef: "oc-recreated",
		AgentID:     "agent-a",
		Status:      PairingStatusActive,
	}
	first, err := service.CreatePairing(context.Background(), "owner-a", request)
	if err != nil {
		t.Fatal(err)
	}
	target := IngressRequest{
		OwnerUserID: "owner-a",
		Channel:     ChannelTypeFeishu,
		AccountID:   "cli-a",
		ChatType:    "group",
		Ref:         "oc-recreated",
		ThreadID:    "omt-recreated",
	}
	_, oldKey, err := service.ResolveIngressSession(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`
INSERT INTO im_deliveries (
    owner_user_id, delivery_id, target_session_key, pairing_id,
    source_session_key, intent_json, send_state, created_at
) VALUES (?, ?, ?, ?, ?, '{}', 'pending', 1)`,
		"owner-a", "delivery-old", oldKey, first.PairingID, "agent:agent-a:internal:dm:source"); err != nil {
		t.Fatal(err)
	}
	// Simulate the pre-fix deletion path: the pairing row disappeared while
	// the concrete wildcard projection and delivery grant survived.
	if _, err = db.Exec("DELETE FROM im_pairings WHERE pairing_id = ?", first.PairingID); err != nil {
		t.Fatal(err)
	}
	second, err := service.CreatePairing(context.Background(), "owner-a", request)
	if err != nil {
		t.Fatal(err)
	}
	_, newKey, err := service.ResolveIngressSession(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	if newKey == oldKey || protocol.ParseSessionKey(newKey).Generation == "" {
		t.Fatalf("重建配对不得继承旧具体 session: old=%q new=%q", oldKey, newKey)
	}
	var mappedPairing string
	if err = db.QueryRow(`SELECT pairing_id FROM im_pairing_sessions WHERE session_key = ?`, newKey).Scan(&mappedPairing); err != nil {
		t.Fatal(err)
	}
	if mappedPairing != second.PairingID {
		t.Fatalf("具体映射仍指向旧 pairing: got=%q want=%q", mappedPairing, second.PairingID)
	}
	var revoked bool
	if err = db.QueryRow("SELECT return_revoked FROM im_deliveries WHERE delivery_id = ?", "delivery-old").Scan(&revoked); err != nil {
		t.Fatal(err)
	}
	if !revoked {
		t.Fatal("旧 pairing 的 delivery grant 未被撤销")
	}
}

func TestControlServiceDeleteChannelRevokesAndRemovesIMLifecycleState(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	if _, err := db.Exec(`
INSERT INTO channel_control_versions (owner_user_id, version) VALUES ('owner-a', 1);
INSERT INTO im_channel_configs (owner_user_id, channel_type, agent_id, status, config_json)
VALUES ('owner-a', 'feishu', 'agent-a', 'configured', '{}');
INSERT INTO im_pairings (
    pairing_id, owner_user_id, channel_type, account_id, chat_type, external_ref, agent_id, status, source, session_key
) VALUES ('pair-delete-channel', 'owner-a', 'feishu', '', 'group', 'oc-delete-channel', 'agent-a', 'active', 'manual', 'agent:agent-a:fs:group:oc-delete-channel');
INSERT INTO im_pairing_sessions (
    owner_user_id, pairing_id, channel_type, account_id, chat_type, external_ref, thread_id, session_key
) VALUES ('owner-a', 'pair-delete-channel', 'feishu', 'cli-delete', 'group', 'oc-delete-channel', 'omt-delete', 'agent:agent-a:fs:group:acct:cli-delete:oc-delete-channel:topic:omt-delete');
INSERT INTO im_deliveries (
    owner_user_id, delivery_id, target_session_key, pairing_id, source_session_key, intent_json, created_at
) VALUES ('owner-a', 'delivery-delete-channel', 'agent:agent-a:fs:group:acct:cli-delete:oc-delete-channel:topic:omt-delete', 'pair-delete-channel', 'agent:agent-a:internal:dm:source', '{}', 1);
INSERT INTO automation_delivery_routes (
    route_id, agent_id, session_key, mode, channel, "to", account_id, thread_id, enabled
) VALUES
 ('route-delete-channel-session', 'agent-a', 'agent:agent-a:fs:group:acct:cli-delete:oc-delete-channel:topic:omt-delete', 'explicit', 'feishu', 'oc-delete-channel', 'cli-delete', 'omt-delete', 1),
 ('route-delete-channel-last', 'agent-a', '', 'explicit', 'feishu', 'oc-delete-channel', 'cli-delete', 'omt-delete', 1);
`); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteChannelConfig(context.Background(), "owner-a", ChannelTypeFeishu); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"im_pairings", "im_pairing_sessions", "im_channel_configs"} {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table + " WHERE owner_user_id = 'owner-a' AND channel_type = 'feishu'").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("删除 Channel 后 %s 仍有 %d 条记录", table, count)
		}
	}
	var revoked bool
	if err := db.QueryRow("SELECT return_revoked FROM im_deliveries WHERE delivery_id = 'delivery-delete-channel'").Scan(&revoked); err != nil {
		t.Fatal(err)
	}
	if !revoked {
		t.Fatal("删除 Channel 后 delivery grant 未撤销")
	}
	var routeCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM automation_delivery_routes WHERE route_id LIKE 'route-delete-channel-%'").Scan(&routeCount); err != nil {
		t.Fatal(err)
	}
	if routeCount != 0 {
		t.Fatalf("删除 Channel 后旧 delivery route 仍残留: %d", routeCount)
	}
}

func TestControlServiceDeleteAccountKeepsOtherAccountProjection(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	if _, err := db.Exec(`
INSERT INTO channel_control_versions (owner_user_id, version) VALUES ('owner-a', 1);
INSERT INTO im_channel_configs (owner_user_id, channel_type, agent_id, status, config_json)
VALUES ('owner-a', 'feishu', 'agent-a', 'configured', '{}');
INSERT INTO im_channel_accounts (owner_user_id, channel_type, account_id, status, config_json)
VALUES ('owner-a', 'feishu', 'cli-a', 'connected', '{}'), ('owner-a', 'feishu', 'cli-b', 'connected', '{}');
INSERT INTO im_pairings (
    pairing_id, owner_user_id, channel_type, account_id, chat_type, external_ref, agent_id, status, source, session_key
) VALUES
 ('pair-wild-a', 'owner-a', 'feishu', '', 'group', 'oc-shared', 'agent-a', 'active', 'manual', 'agent:agent-a:fs:group:oc-shared'),
 ('pair-explicit-b', 'owner-a', 'feishu', 'cli-b', 'dm', 'ou-b', 'agent-a', 'active', 'manual', 'agent:agent-a:fs:dm:acct:cli-b:ou-b');
INSERT INTO im_pairing_sessions (
    owner_user_id, pairing_id, channel_type, account_id, chat_type, external_ref, thread_id, session_key
) VALUES
 ('owner-a', 'pair-wild-a', 'feishu', 'cli-a', 'group', 'oc-shared', 'omt-a', 'agent:agent-a:fs:group:acct:cli-a:oc-shared:topic:omt-a'),
 ('owner-a', 'pair-explicit-b', 'feishu', 'cli-b', 'dm', 'ou-b', '', 'agent:agent-a:fs:dm:acct:cli-b:ou-b');
INSERT INTO im_deliveries (
    owner_user_id, delivery_id, target_session_key, pairing_id, source_session_key, intent_json, created_at
) VALUES
 ('owner-a', 'delivery-a', 'agent:agent-a:fs:group:acct:cli-a:oc-shared:topic:omt-a', 'pair-wild-a', 'agent:agent-a:internal:dm:source', '{}', 1),
 ('owner-a', 'delivery-b', 'agent:agent-a:fs:dm:acct:cli-b:ou-b', 'pair-explicit-b', 'agent:agent-a:internal:dm:source', '{}', 1);
INSERT INTO automation_delivery_routes (
    route_id, agent_id, session_key, mode, channel, "to", account_id, thread_id, enabled
) VALUES
 ('route-account-a-session', 'agent-a', 'agent:agent-a:fs:group:acct:cli-a:oc-shared:topic:omt-a', 'explicit', 'feishu', 'oc-shared', 'cli-a', 'omt-a', 1),
 ('route-account-a-last', 'agent-a', '', 'explicit', 'feishu', 'oc-shared', 'cli-a', 'omt-a', 1),
 ('route-account-b-session', 'agent-a', 'agent:agent-a:fs:dm:acct:cli-b:ou-b', 'explicit', 'feishu', 'ou-b', 'cli-b', '', 1);
`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.DeleteChannelAccount(context.Background(), "owner-a", ChannelTypeFeishu, "cli-a"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM im_channel_accounts WHERE account_id = 'cli-a'").Scan(&count); err != nil || count != 0 {
		t.Fatalf("账号 cli-a 未删除: count=%d err=%v", count, err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM im_channel_accounts WHERE account_id = 'cli-b'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("账号 cli-b 被误删: count=%d err=%v", count, err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM im_pairing_sessions WHERE account_id = 'cli-a'").Scan(&count); err != nil || count != 0 {
		t.Fatalf("账号 cli-a 的具体映射未删除: count=%d err=%v", count, err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM im_pairings WHERE pairing_id = 'pair-explicit-b'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("账号 cli-b 的 pairing 被误删: count=%d err=%v", count, err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM automation_delivery_routes WHERE route_id LIKE 'route-account-a-%'").Scan(&count); err != nil || count != 0 {
		t.Fatalf("账号 cli-a 的 delivery route 未清理: count=%d err=%v", count, err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM automation_delivery_routes WHERE route_id = 'route-account-b-session'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("账号 cli-b 的 delivery route 被误删: count=%d err=%v", count, err)
	}
	var revoked bool
	if err := db.QueryRow("SELECT return_revoked FROM im_deliveries WHERE delivery_id = 'delivery-a'").Scan(&revoked); err != nil || !revoked {
		t.Fatalf("账号 cli-a 的 delivery 未撤销: revoked=%v err=%v", revoked, err)
	}
	if err := db.QueryRow("SELECT return_revoked FROM im_deliveries WHERE delivery_id = 'delivery-b'").Scan(&revoked); err != nil || revoked {
		t.Fatalf("账号 cli-b 的 delivery 被误撤销: revoked=%v err=%v", revoked, err)
	}
}

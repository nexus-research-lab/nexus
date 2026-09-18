package channels

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type deletedSessionResolver struct{ deleted map[string]bool }

func (r deletedSessionResolver) ResolveDeliverySession(_ context.Context, _ string) (*protocol.Session, error) {
	return nil, nil
}

func (r deletedSessionResolver) IsSessionDeleted(_ context.Context, key string) (bool, error) {
	return r.deleted[key], nil
}

func TestControlServiceRotatesDeletedMaterializedPairingSession(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	cfg := config.Config{DatabaseDriver: "sqlite"}
	router := NewRouter(cfg, db, nil, nil)
	router.SetSessionProjectionResolver(imTestSessions{})
	service := NewControlService(cfg, db, nil, router)
	created, err := service.CreatePairing(context.Background(), "owner-a", CreatePairingRequest{
		ChannelType: ChannelTypeFeishu,
		AccountID:   "cli-a",
		ChatType:    protocol.RoomTypeDM,
		ExternalRef: "ou-user-a",
		AgentID:     "agent-a",
		Status:      PairingStatusActive,
	})
	if err != nil {
		t.Fatalf("创建配对失败: %v", err)
	}
	if _, err = db.Exec("UPDATE im_pairings SET session_materialized = 1 WHERE pairing_id = ?", created.PairingID); err != nil {
		t.Fatalf("准备已物化配对失败: %v", err)
	}
	agentID, sessionKey, err := service.ResolveIngressSession(context.Background(), IngressRequest{
		OwnerUserID: "owner-a",
		Channel:     ChannelTypeFeishu,
		AccountID:   "cli-a",
		ChatType:    protocol.RoomTypeDM,
		Ref:         "ou-user-a",
	})
	if err != nil || agentID != "agent-a" {
		t.Fatalf("删除后的配对应仍解析到原 Agent: agent=%q key=%q err=%v", agentID, sessionKey, err)
	}
	if sessionKey == created.SessionKey {
		t.Fatalf("已删除的当前 session 不应被复用: %q", sessionKey)
	}
	parsed := protocol.ParseSessionKey(sessionKey)
	if parsed.Ref != "ou-user-a" || parsed.AccountID != "cli-a" || parsed.Generation == "" {
		t.Fatalf("新 session 应保留外部路由并带代次: %q parsed=%+v", sessionKey, parsed)
	}
	var materialized bool
	if err = db.QueryRow("SELECT session_materialized FROM im_pairings WHERE pairing_id = ?", created.PairingID).Scan(&materialized); err != nil {
		t.Fatalf("读取配对代次状态失败: %v", err)
	}
	if materialized {
		t.Fatalf("新 session 在首次 DM 物化前不应标记为已物化")
	}
	_, nextKey, err := service.ResolveIngressSession(context.Background(), IngressRequest{
		OwnerUserID: "owner-a", Channel: ChannelTypeFeishu, AccountID: "cli-a", ChatType: protocol.RoomTypeDM, Ref: "ou-user-a",
	})
	if err != nil || nextKey != sessionKey {
		t.Fatalf("同一配对后续 ingress 应复用新 session: first=%q next=%q err=%v", sessionKey, nextKey, err)
	}
}

func TestControlServiceRotatesDeletedWildcardThreadSession(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	cfg := config.Config{DatabaseDriver: "sqlite"}
	router := NewRouter(cfg, db, nil, nil)
	router.SetSessionProjectionResolver(imTestSessions{})
	service := NewControlService(cfg, db, nil, router)
	if _, err := service.CreatePairing(context.Background(), "owner-a", CreatePairingRequest{
		ChannelType: ChannelTypeFeishu,
		ChatType:    "group",
		ExternalRef: "oc-group-a",
		AgentID:     "agent-a",
		Status:      PairingStatusActive,
	}); err != nil {
		t.Fatalf("创建群级配对失败: %v", err)
	}
	request := IngressRequest{OwnerUserID: "owner-a", Channel: ChannelTypeFeishu, AccountID: "cli-a", ChatType: "group", Ref: "oc-group-a", ThreadID: "omt-thread-a"}
	_, firstKey, err := service.ResolveIngressSession(context.Background(), request)
	if err != nil {
		t.Fatalf("解析通配群话题 session 失败: %v", err)
	}
	if _, err = db.Exec("UPDATE im_pairing_sessions SET session_materialized = 1 WHERE session_key = ?", firstKey); err != nil {
		t.Fatalf("准备群话题 session 失败: %v", err)
	}
	_, secondKey, err := service.ResolveIngressSession(context.Background(), request)
	if err != nil {
		t.Fatalf("删除后的群话题 session 未恢复: %v", err)
	}
	if firstKey == secondKey || protocol.ParseSessionKey(secondKey).Generation == "" {
		t.Fatalf("群话题应生成新的 session 代次: first=%q second=%q", firstKey, secondKey)
	}
	parsed := protocol.ParseSessionKey(secondKey)
	if parsed.AccountID != "cli-a" || parsed.Ref != "oc-group-a" || parsed.ThreadID != "omt-thread-a" {
		t.Fatalf("群话题新 session 不应改变平台路由: %q parsed=%+v", secondKey, parsed)
	}
}

func TestControlServiceRecreatedPairingAvoidsDeletedSessionKey(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	cfg := config.Config{DatabaseDriver: "sqlite"}
	router := NewRouter(cfg, db, nil, nil)
	service := NewControlService(cfg, db, nil, router)
	request := CreatePairingRequest{ChannelType: ChannelTypeWeChat, ChatType: protocol.RoomTypeDM, ExternalRef: "wx-user-a", AgentID: "agent-a", Status: PairingStatusActive}
	first, err := service.CreatePairing(context.Background(), "owner-a", request)
	if err != nil {
		t.Fatalf("创建初始配对失败: %v", err)
	}
	if err = service.DeletePairing(context.Background(), "owner-a", first.PairingID); err != nil {
		t.Fatalf("删除旧配对失败: %v", err)
	}
	second, err := service.CreatePairing(context.Background(), "owner-a", request)
	if err != nil {
		t.Fatalf("重新创建配对失败: %v", err)
	}
	router.SetSessionProjectionResolver(deletedSessionResolver{deleted: map[string]bool{first.SessionKey: true}})
	_, current, err := service.ResolveIngressSession(context.Background(), IngressRequest{OwnerUserID: "owner-a", Channel: ChannelTypeWeChat, ChatType: protocol.RoomTypeDM, Ref: "wx-user-a"})
	if err != nil {
		t.Fatalf("重建配对后的入站解析失败: %v", err)
	}
	if current == second.SessionKey || protocol.ParseSessionKey(current).Generation == "" {
		t.Fatalf("重建配对不得复用已删除 key: old=%q current=%q", first.SessionKey, current)
	}
}

func TestControlServiceUpdatePairingPatchesOnlyRequestedFields(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	created, err := service.CreatePairing(context.Background(), "owner-a", CreatePairingRequest{
		ChannelType: ChannelTypeTelegram,
		ChatType:    "dm",
		ExternalRef: "chat-a",
		AgentID:     "agent-a",
		Status:      PairingStatusActive,
		Source:      PairingSourceIngress,
	})
	if err != nil {
		t.Fatalf("创建配对失败: %v", err)
	}
	version, err := service.GetChannelControlVersion(context.Background(), "owner-a")
	if err != nil || version != 2 {
		t.Fatalf("创建 pairing 应推进 Channel version: version=%d err=%v", version, err)
	}
	const lastMessageAt = "2040-01-02 03:04:05"
	if _, err = db.Exec(
		"UPDATE im_pairings SET last_message_at = ? WHERE owner_user_id = ? AND pairing_id = ?",
		lastMessageAt,
		"owner-a",
		created.PairingID,
	); err != nil {
		t.Fatalf("准备 ingress 时间失败: %v", err)
	}

	name := "Renamed chat"
	status := PairingStatusPending
	updated, err := service.UpdatePairing(context.Background(), "owner-a", created.PairingID, UpdatePairingRequest{
		ExternalName: &name,
		Status:       &status,
	})
	if err != nil {
		t.Fatalf("字段 patch 失败: %v", err)
	}
	version, err = service.GetChannelControlVersion(context.Background(), "owner-a")
	if err != nil || version != 3 {
		t.Fatalf("更新 pairing 应推进 Channel version: version=%d err=%v", version, err)
	}
	if updated.ExternalName != name || updated.Status != status || updated.Source != PairingSourceIngress {
		t.Fatalf("字段 patch 结果不正确: %+v", updated)
	}
	var storedLastMessageAt string
	var storedSource string
	if err = db.QueryRow(
		"SELECT CAST(last_message_at AS TEXT), source FROM im_pairings WHERE owner_user_id = ? AND pairing_id = ?",
		"owner-a",
		created.PairingID,
	).Scan(&storedLastMessageAt, &storedSource); err != nil {
		t.Fatalf("读取字段 patch 结果失败: %v", err)
	}
	if storedLastMessageAt != lastMessageAt || storedSource != PairingSourceIngress {
		t.Fatalf("人工更新不得覆盖 ingress 字段: last_message_at=%q source=%q", storedLastMessageAt, storedSource)
	}
}

func TestControlServiceUpdatePairingAgentRotatesSessionIdentity(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	created, err := service.CreatePairing(context.Background(), "owner-a", CreatePairingRequest{
		ChannelType: ChannelTypeTelegram,
		ChatType:    protocol.RoomTypeDM,
		ExternalRef: "chat-a",
		AgentID:     "agent-a",
		Status:      PairingStatusActive,
	})
	if err != nil {
		t.Fatalf("创建配对失败: %v", err)
	}
	if _, err = db.Exec("UPDATE im_pairings SET session_materialized = 1 WHERE pairing_id = ?", created.PairingID); err != nil {
		t.Fatalf("准备已物化 Session 失败: %v", err)
	}
	agentB := "agent-b"
	updated, err := service.UpdatePairing(context.Background(), "owner-a", created.PairingID, UpdatePairingRequest{AgentID: &agentB})
	if err != nil {
		t.Fatalf("重新绑定 Agent 失败: %v", err)
	}
	if updated.AgentID != agentB {
		t.Fatalf("重新绑定 Agent 未生效: %+v", updated)
	}
	var sessionKey string
	var materialized bool
	if err = db.QueryRow("SELECT session_key, session_materialized FROM im_pairings WHERE pairing_id = ?", created.PairingID).Scan(&sessionKey, &materialized); err != nil {
		t.Fatalf("读取重绑定 Session 失败: %v", err)
	}
	parsed := protocol.ParseSessionKey(sessionKey)
	if parsed.AgentID != agentB || parsed.Generation == "" || materialized {
		t.Fatalf("重绑定必须生成未物化的新 Session: key=%q parsed=%+v materialized=%v", sessionKey, parsed, materialized)
	}
	var concreteCount int
	if err = db.QueryRow("SELECT COUNT(*) FROM im_pairing_sessions WHERE pairing_id = ?", created.PairingID).Scan(&concreteCount); err != nil {
		t.Fatalf("读取具体 Session 映射失败: %v", err)
	}
	if concreteCount != 0 {
		t.Fatalf("重绑定不得保留旧具体 Session 映射: count=%d", concreteCount)
	}
}

func TestControlServiceCreatesManualPairingForKnownTarget(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	created, err := service.CreatePairing(context.Background(), "owner-a", CreatePairingRequest{
		ChannelType:  " telegram ",
		ChatType:     " group ",
		ExternalRef:  " -100123456 ",
		ThreadID:     " 42 ",
		ExternalName: " Release room ",
		AgentID:      " agent-a ",
	})
	if err != nil {
		t.Fatalf("手动创建 IM 配对失败: %v", err)
	}
	if created.PairingID == "" ||
		created.ChannelType != ChannelTypeTelegram ||
		created.ChatType != "group" ||
		created.ExternalRef != "-100123456" ||
		created.ThreadID != "42" ||
		created.ExternalName != "Release room" ||
		created.AgentID != "agent-a" ||
		created.Status != PairingStatusActive ||
		created.Source != PairingSourceManual {
		t.Fatalf("手动配对结果不正确: %+v", created)
	}

	agentID, err := service.ResolveIngressAgent(context.Background(), IngressRequest{
		OwnerUserID: "owner-a",
		Channel:     ChannelTypeTelegram,
		ChatType:    "group",
		Ref:         "-100123456",
		ThreadID:    "42",
	})
	if err != nil {
		t.Fatalf("手动授权配对应允许入站路由: %v", err)
	}
	if agentID != "agent-a" {
		t.Fatalf("入站路由 agent 不正确: %q", agentID)
	}

	items, err := service.ListPairings(context.Background(), "owner-a", PairingQuery{
		ChannelType: ChannelTypeTelegram,
		Status:      PairingStatusActive,
	})
	if err != nil {
		t.Fatalf("查询手动配对失败: %v", err)
	}
	if len(items) != 1 || items[0].PairingID != created.PairingID || items[0].LastMessageAt == nil {
		t.Fatalf("手动配对列表结果不正确: %+v", items)
	}
}

func TestControlServiceGroupPairingRoutesThreadedIngress(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	groupPairing, err := service.CreatePairing(context.Background(), "owner-a", CreatePairingRequest{
		ChannelType: ChannelTypeFeishu,
		ChatType:    "group",
		ExternalRef: "oc_group_123",
		AgentID:     "agent-a",
	})
	if err != nil {
		t.Fatalf("创建群级 IM 配对失败: %v", err)
	}
	topicPairing, err := service.CreatePairing(context.Background(), "owner-a", CreatePairingRequest{
		ChannelType: ChannelTypeFeishu,
		ChatType:    "group",
		ExternalRef: "oc_group_123",
		ThreadID:    "topic-override",
		AgentID:     "agent-b",
	})
	if err != nil {
		t.Fatalf("创建话题级 IM 配对失败: %v", err)
	}

	agentID, err := service.ResolveIngressAgent(context.Background(), IngressRequest{
		OwnerUserID: "owner-a",
		Channel:     ChannelTypeFeishu,
		ChatType:    "group",
		Ref:         "oc_group_123",
		ThreadID:    "topic-1",
	})
	if err != nil || agentID != "agent-a" {
		t.Fatalf("群级配对应接住子线程入站: agent=%q err=%v", agentID, err)
	}
	agentID, err = service.ResolveIngressAgent(context.Background(), IngressRequest{
		OwnerUserID: "owner-a",
		Channel:     ChannelTypeFeishu,
		ChatType:    "group",
		Ref:         "oc_group_123",
		ThreadID:    "topic-override",
	})
	if err != nil || agentID != "agent-b" {
		t.Fatalf("话题级配对应优先于群级配对: agent=%q err=%v", agentID, err)
	}

	items, err := service.ListPairings(context.Background(), "owner-a", PairingQuery{
		ChannelType: ChannelTypeFeishu,
		Status:      PairingStatusActive,
	})
	if err != nil {
		t.Fatalf("查询 active 配对失败: %v", err)
	}
	seen := map[string]PairingView{}
	for _, item := range items {
		seen[item.PairingID] = item
	}
	if seen[groupPairing.PairingID].LastMessageAt == nil || seen[topicPairing.PairingID].LastMessageAt == nil {
		t.Fatalf("命中的配对应更新 last_message_at: %+v", items)
	}
}

func TestControlServiceThreadedGroupIngressCreatesGroupScopedPendingPairing(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	var nextID int
	service.idFactory = func(prefix string) string {
		nextID++
		return fmt.Sprintf("%s-%d", prefix, nextID)
	}

	_, err := service.ResolveIngressAgent(context.Background(), IngressRequest{
		OwnerUserID:  "owner-a",
		Channel:      ChannelTypeDiscord,
		ChatType:     "group",
		Ref:          "guild-1:channel-1",
		ThreadID:     "thread-a",
		ExternalName: "Release room",
		AgentID:      "agent-a",
	})
	var firstApproval *pairingApprovalError
	if !errors.As(err, &firstApproval) || firstApproval.PairingID != "pair-1" {
		t.Fatalf("首次群子线程入站应返回群级 pending pairing id: err=%v approval=%+v", err, firstApproval)
	}

	_, err = service.ResolveIngressAgent(context.Background(), IngressRequest{
		OwnerUserID:  "owner-a",
		Channel:      ChannelTypeDiscord,
		ChatType:     "group",
		Ref:          "guild-1:channel-1",
		ThreadID:     "thread-b",
		ExternalName: "Release room renamed",
		AgentID:      "agent-b",
	})
	var secondApproval *pairingApprovalError
	if !errors.As(err, &secondApproval) || secondApproval.PairingID != firstApproval.PairingID {
		t.Fatalf("同群不同子线程应复用群级 pending pairing: first=%+v second=%+v err=%v", firstApproval, secondApproval, err)
	}

	items, err := service.ListPairings(context.Background(), "owner-a", PairingQuery{
		ChannelType: ChannelTypeDiscord,
		Status:      PairingStatusPending,
	})
	if err != nil {
		t.Fatalf("查询 pending 配对失败: %v", err)
	}
	if len(items) != 1 ||
		items[0].PairingID != firstApproval.PairingID ||
		items[0].ThreadID != "" ||
		items[0].ExternalName != "Release room renamed" ||
		items[0].AgentID != "agent-b" {
		t.Fatalf("群子线程 pending 配对应归并到群级目标: %+v", items)
	}
}

func TestControlServiceAllowsManyExternalTargetsForOneAgent(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	targets := []string{"wx-user-1", "wx-user-2", "wx-user-3"}
	pairingIDs := map[string]bool{}
	sessionKeys := map[string]bool{}
	for _, ref := range targets {
		created, err := service.CreatePairing(context.Background(), "owner-a", CreatePairingRequest{
			ChannelType:  ChannelTypeWeixinPersonal,
			ChatType:     "dm",
			ExternalRef:  ref,
			ExternalName: ref,
			AgentID:      "agent-a",
		})
		if err != nil {
			t.Fatalf("创建多用户 IM 配对失败 ref=%s err=%v", ref, err)
		}
		if created.AgentID != "agent-a" ||
			created.ChannelType != ChannelTypeWeixinPersonal ||
			created.ChatType != "dm" ||
			created.ExternalRef != ref ||
			created.Status != PairingStatusActive {
			t.Fatalf("多用户 IM 配对结果不正确 ref=%s item=%+v", ref, created)
		}
		if pairingIDs[created.PairingID] {
			t.Fatalf("不同外部用户不应复用 pairing id: %+v", created)
		}
		pairingIDs[created.PairingID] = true
		expectedSessionKey := "agent:agent-a:weixin-personal:dm:" + ref
		if created.SessionKey != expectedSessionKey {
			t.Fatalf("多用户 IM 配对应暴露稳定 session_key ref=%s got=%s want=%s", ref, created.SessionKey, expectedSessionKey)
		}
		if sessionKeys[created.SessionKey] {
			t.Fatalf("不同外部用户不应复用 session_key: %+v", created)
		}
		sessionKeys[created.SessionKey] = true

		agentID, err := service.ResolveIngressAgent(context.Background(), IngressRequest{
			OwnerUserID: "owner-a",
			Channel:     ChannelTypeWeixinPersonal,
			ChatType:    "dm",
			Ref:         ref,
		})
		if err != nil || agentID != "agent-a" {
			t.Fatalf("已授权外部用户应路由到同一 agent ref=%s agent=%q err=%v", ref, agentID, err)
		}
	}

	items, err := service.ListPairings(context.Background(), "owner-a", PairingQuery{
		ChannelType: ChannelTypeWeixinPersonal,
		Status:      PairingStatusActive,
		AgentID:     "agent-a",
	})
	if err != nil {
		t.Fatalf("查询多用户 IM 配对失败: %v", err)
	}
	if len(items) != len(targets) {
		t.Fatalf("同一 agent 应允许多个外部 IM 目标配对: %+v", items)
	}
	seenTargets := map[string]bool{}
	for _, item := range items {
		if item.AgentID != "agent-a" {
			t.Fatalf("多用户配对应保持同一 agent: %+v", item)
		}
		seenTargets[item.ExternalRef] = true
	}
	for _, ref := range targets {
		if !seenTargets[ref] {
			t.Fatalf("缺少外部用户配对 ref=%s items=%+v", ref, items)
		}
	}
}

func TestControlServiceScopesPairingsByIMAccount(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	accounts := []string{"wx-account-1", "wx-account-2"}
	seenPairings := map[string]bool{}
	seenSessions := map[string]bool{}
	for _, accountID := range accounts {
		created, err := service.CreatePairing(context.Background(), "owner-a", CreatePairingRequest{
			ChannelType: ChannelTypeWeixinPersonal,
			AccountID:   accountID,
			ChatType:    "dm",
			ExternalRef: "same-wx-user",
			AgentID:     "agent-a",
		})
		if err != nil {
			t.Fatalf("创建账号隔离配对失败 account=%s err=%v", accountID, err)
		}
		if created.AccountID != accountID {
			t.Fatalf("配对应保留 account_id account=%s item=%+v", accountID, created)
		}
		expectedSessionKey := "agent:agent-a:weixin-personal:dm:acct:" + accountID + ":same-wx-user"
		if created.SessionKey != expectedSessionKey {
			t.Fatalf("账号隔离 session_key 不正确 account=%s got=%s want=%s", accountID, created.SessionKey, expectedSessionKey)
		}
		if seenPairings[created.PairingID] || seenSessions[created.SessionKey] {
			t.Fatalf("不同账号不应复用 pairing/session: %+v", created)
		}
		seenPairings[created.PairingID] = true
		seenSessions[created.SessionKey] = true

		agentID, err := service.ResolveIngressAgent(context.Background(), IngressRequest{
			OwnerUserID: "owner-a",
			Channel:     ChannelTypeWeixinPersonal,
			AccountID:   accountID,
			ChatType:    "dm",
			Ref:         "same-wx-user",
		})
		if err != nil || agentID != "agent-a" {
			t.Fatalf("账号隔离配对应可解析 account=%s agent=%q err=%v", accountID, agentID, err)
		}
	}

	items, err := service.ListPairings(context.Background(), "owner-a", PairingQuery{
		ChannelType: ChannelTypeWeixinPersonal,
		Status:      PairingStatusActive,
	})
	if err != nil {
		t.Fatalf("查询账号隔离配对失败: %v", err)
	}
	if len(items) != len(accounts) {
		t.Fatalf("同一外部 ref 在不同账号下应保留多条配对: %+v", items)
	}
}

func TestControlServiceAllowsManyExternalTargetsForOneAgentAcrossIMChannels(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	for _, channelType := range []string{
		ChannelTypeTelegram,
		ChannelTypeDiscord,
		ChannelTypeDingTalk,
		ChannelTypeWeChat,
		ChannelTypeWeixinPersonal,
		ChannelTypeFeishu,
	} {
		t.Run(channelType, func(t *testing.T) {
			seenPairings := map[string]bool{}
			seenSessions := map[string]bool{}
			for _, ref := range []string{channelType + "-user-1", channelType + "-user-2"} {
				created, err := service.CreatePairing(context.Background(), "owner-"+channelType, CreatePairingRequest{
					ChannelType: channelType,
					ChatType:    "dm",
					ExternalRef: ref,
					AgentID:     "agent-a",
				})
				if err != nil {
					t.Fatalf("创建多用户配对失败 channel=%s ref=%s err=%v", channelType, ref, err)
				}
				if created.AgentID != "agent-a" || created.ExternalRef != ref || created.Status != PairingStatusActive {
					t.Fatalf("多用户配对结果不正确 channel=%s ref=%s item=%+v", channelType, ref, created)
				}
				if seenPairings[created.PairingID] {
					t.Fatalf("不同外部用户不应复用 pairing id channel=%s item=%+v", channelType, created)
				}
				seenPairings[created.PairingID] = true
				if created.SessionKey == "" || seenSessions[created.SessionKey] {
					t.Fatalf("不同外部用户不应复用 session_key channel=%s item=%+v", channelType, created)
				}
				seenSessions[created.SessionKey] = true

				agentID, err := service.ResolveIngressAgent(context.Background(), IngressRequest{
					OwnerUserID: "owner-" + channelType,
					Channel:     channelType,
					ChatType:    "dm",
					Ref:         ref,
				})
				if err != nil || agentID != "agent-a" {
					t.Fatalf("已授权外部用户应路由到同一 agent channel=%s ref=%s agent=%q err=%v", channelType, ref, agentID, err)
				}
			}

			items, err := service.ListPairings(context.Background(), "owner-"+channelType, PairingQuery{
				ChannelType: channelType,
				Status:      PairingStatusActive,
				AgentID:     "agent-a",
			})
			if err != nil {
				t.Fatalf("查询多用户配对失败 channel=%s err=%v", channelType, err)
			}
			if len(items) != 2 {
				t.Fatalf("同一 agent 应允许多个外部 IM 目标配对 channel=%s items=%+v", channelType, items)
			}
		})
	}
}

func TestControlServiceCreatePairingUpdatesExistingTarget(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	first, err := service.CreatePairing(context.Background(), "owner-a", CreatePairingRequest{
		ChannelType: ChannelTypeFeishu,
		ChatType:    "dm",
		ExternalRef: "ou_user_1",
		AgentID:     "agent-a",
		Status:      PairingStatusPending,
	})
	if err != nil {
		t.Fatalf("创建初始配对失败: %v", err)
	}

	updated, err := service.CreatePairing(context.Background(), "owner-a", CreatePairingRequest{
		ChannelType:  ChannelTypeFeishu,
		ChatType:     "dm",
		ExternalRef:  "ou_user_1",
		ExternalName: "Alice",
		AgentID:      "agent-b",
		Status:       PairingStatusActive,
	})
	if err != nil {
		t.Fatalf("重复创建同一目标应更新已有配对: %v", err)
	}
	if updated.PairingID != first.PairingID ||
		updated.ExternalName != "Alice" ||
		updated.AgentID != "agent-b" ||
		updated.Status != PairingStatusActive ||
		updated.Source != PairingSourceManual {
		t.Fatalf("重复创建配对应更新已有记录: first=%+v updated=%+v", first, updated)
	}

	items, err := service.ListPairings(context.Background(), "owner-a", PairingQuery{ChannelType: ChannelTypeFeishu})
	if err != nil {
		t.Fatalf("查询配对失败: %v", err)
	}
	if len(items) != 1 || items[0].PairingID != first.PairingID {
		t.Fatalf("重复创建不应产生多条配对: %+v", items)
	}
}

func TestControlServiceCreatesSeparatePendingPairingsForManyExternalTargets(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	var nextID int
	service.idFactory = func(prefix string) string {
		nextID++
		return fmt.Sprintf("%s-%d", prefix, nextID)
	}

	targets := []string{"wx-user-1", "wx-user-2"}
	for index, ref := range targets {
		_, err := service.ResolveIngressAgent(context.Background(), IngressRequest{
			OwnerUserID:  "owner-a",
			Channel:      ChannelTypeWeixinPersonal,
			ChatType:     "dm",
			Ref:          ref,
			ExternalName: ref,
			AgentID:      "agent-a",
		})
		wantPairingID := fmt.Sprintf("pair-%d", index+1)
		var approval *pairingApprovalError
		if !errors.As(err, &approval) || approval.PairingID != wantPairingID {
			t.Fatalf("新外部用户应各自生成 pending pairing ref=%s err=%v approval=%+v want=%s", ref, err, approval, wantPairingID)
		}
	}

	items, err := service.ListPairings(context.Background(), "owner-a", PairingQuery{
		ChannelType: ChannelTypeWeixinPersonal,
		Status:      PairingStatusPending,
		AgentID:     "agent-a",
	})
	if err != nil {
		t.Fatalf("查询 pending IM 配对失败: %v", err)
	}
	if len(items) != len(targets) {
		t.Fatalf("不同外部用户的 pending 配对不应互相覆盖: %+v", items)
	}
	seenTargets := map[string]bool{}
	for _, item := range items {
		seenTargets[item.ExternalRef] = true
	}
	for _, ref := range targets {
		if !seenTargets[ref] {
			t.Fatalf("缺少 pending 外部用户配对 ref=%s items=%+v", ref, items)
		}
	}
}

func TestControlServiceResolveIngressAgentReturnsExistingPendingPairingID(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	var nextID int
	service.idFactory = func(prefix string) string {
		nextID++
		return fmt.Sprintf("%s-%d", prefix, nextID)
	}

	_, err := service.ResolveIngressAgent(context.Background(), IngressRequest{
		OwnerUserID:  "owner-a",
		Channel:      ChannelTypeTelegram,
		ChatType:     "group",
		Ref:          "-100123456",
		ThreadID:     "42",
		ExternalName: "Release room",
		AgentID:      "agent-a",
	})
	var firstApproval *pairingApprovalError
	if !errors.As(err, &firstApproval) || firstApproval.PairingID != "pair-1" {
		t.Fatalf("首次入站应返回真实 pending pairing id: err=%v approval=%+v", err, firstApproval)
	}

	_, err = service.ResolveIngressAgent(context.Background(), IngressRequest{
		OwnerUserID:  "owner-a",
		Channel:      ChannelTypeTelegram,
		ChatType:     "group",
		Ref:          "-100123456",
		ThreadID:     "42",
		ExternalName: "Release room renamed",
		AgentID:      "agent-b",
	})
	var secondApproval *pairingApprovalError
	if !errors.As(err, &secondApproval) || secondApproval.PairingID != firstApproval.PairingID {
		t.Fatalf("重复入站应返回已有 pending pairing id: first=%+v second=%+v err=%v", firstApproval, secondApproval, err)
	}

	items, err := service.ListPairings(context.Background(), "owner-a", PairingQuery{
		ChannelType: ChannelTypeTelegram,
		Status:      PairingStatusPending,
	})
	if err != nil {
		t.Fatalf("查询 pending 配对失败: %v", err)
	}
	if len(items) != 1 ||
		items[0].PairingID != firstApproval.PairingID ||
		items[0].ExternalName != "Release room renamed" ||
		items[0].AgentID != "agent-b" {
		t.Fatalf("重复入站应更新同一 pending 配对: %+v", items)
	}

	_, err = service.UpdatePairing(context.Background(), "owner-a", firstApproval.PairingID, UpdatePairingRequest{
		Status: ptrString(PairingStatusActive),
	})
	if err != nil {
		t.Fatalf("批准 pending 配对失败: %v", err)
	}
	agentID, err := service.ResolveIngressAgent(context.Background(), IngressRequest{
		OwnerUserID: "owner-a",
		Channel:     ChannelTypeTelegram,
		ChatType:    "group",
		Ref:         "-100123456",
		ThreadID:    "42",
	})
	if err != nil || agentID != "agent-b" {
		t.Fatalf("批准后入站应路由到更新后的 agent: agent=%q err=%v", agentID, err)
	}
}

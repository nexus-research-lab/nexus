package channels

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
)

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

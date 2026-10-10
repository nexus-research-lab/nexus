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

package channels

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/imdelivery"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func TestValidateExternalSessionGrantRequiresExactActivePairing(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	created, err := service.CreatePairing(context.Background(), "owner-a", CreatePairingRequest{
		ChannelType: ChannelTypeWeixinPersonal,
		AccountID:   "weixin-account",
		ChatType:    "dm",
		ExternalRef: "weixin-user",
		AgentID:     "agent-a",
		Status:      PairingStatusActive,
	})
	if err != nil {
		t.Fatalf("创建 active pairing 失败: %v", err)
	}
	sessionKey := protocol.BuildAgentAccountSessionKey(
		"agent-a",
		protocol.SessionChannelWeixinPersonal,
		"dm",
		"weixin-account",
		"weixin-user",
		"",
	)
	if err = service.ValidateExternalSessionGrant(context.Background(), "owner-a", "agent-a", sessionKey); err != nil {
		t.Fatalf("精确 active pairing 应授权 Automation 投递: %v", err)
	}
	otherTarget := protocol.BuildAgentAccountSessionKey(
		"agent-a",
		protocol.SessionChannelWeixinPersonal,
		"dm",
		"weixin-account",
		"another-user",
		"",
	)
	if err = service.ValidateExternalSessionGrant(context.Background(), "owner-a", "agent-a", otherTarget); err == nil {
		t.Fatal("同账号下不同 IM 对话不得复用 pairing grant")
	}
	status := PairingStatusPending
	if _, err = service.UpdatePairing(context.Background(), "owner-a", created.PairingID, UpdatePairingRequest{
		Status: &status,
	}); err != nil {
		t.Fatalf("撤销 active pairing 失败: %v", err)
	}
	if err = service.ValidateExternalSessionGrant(context.Background(), "owner-a", "agent-a", sessionKey); err == nil {
		t.Fatal("pairing 不再 active 后 Automation 投递必须立即 fail closed")
	}
}

func TestPairingReenableRotatesParentAndWildcardSessionKeys(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	created, err := service.CreatePairing(context.Background(), "owner-a", CreatePairingRequest{
		ChannelType: ChannelTypeFeishu,
		ChatType:    protocol.RoomTypeGroup,
		ExternalRef: "oc-reenable",
		AgentID:     "agent-a",
		Status:      PairingStatusActive,
	})
	if err != nil {
		t.Fatalf("创建通配 pairing 失败: %v", err)
	}
	_, oldConcreteKey, err := service.ResolveIngressSession(context.Background(), IngressRequest{
		OwnerUserID: "owner-a",
		Channel:     ChannelTypeFeishu,
		AccountID:   "cli-a",
		ChatType:    protocol.RoomTypeGroup,
		Ref:         "oc-reenable",
		ThreadID:    "omt-a",
	})
	if err != nil {
		t.Fatalf("创建通配 concrete Session 失败: %v", err)
	}
	oldParentKey := created.SessionKey

	status := PairingStatusDisabled
	if _, err = service.UpdatePairing(context.Background(), "owner-a", created.PairingID, UpdatePairingRequest{Status: &status}); err != nil {
		t.Fatalf("停用 pairing 失败: %v", err)
	}
	status = PairingStatusActive
	reenabled, err := service.UpdatePairing(context.Background(), "owner-a", created.PairingID, UpdatePairingRequest{Status: &status})
	if err != nil {
		t.Fatalf("重新启用 pairing 失败: %v", err)
	}
	if reenabled.SessionKey == oldParentKey || protocol.ParseSessionKey(reenabled.SessionKey).Generation == "" {
		t.Fatalf("重新启用必须轮换 parent Session key: old=%q new=%q", oldParentKey, reenabled.SessionKey)
	}
	if err = service.ValidateExternalSessionGrant(context.Background(), "owner-a", "agent-a", oldParentKey); err == nil {
		t.Fatal("重新启用后旧 parent Session key 不得恢复授权")
	}
	if err = service.ValidateExternalSessionGrant(context.Background(), "owner-a", "agent-a", oldConcreteKey); err == nil {
		t.Fatal("重新启用后旧 wildcard concrete Session key 不得恢复授权")
	}
	_, newConcreteKey, err := service.ResolveIngressSession(context.Background(), IngressRequest{
		OwnerUserID: "owner-a",
		Channel:     ChannelTypeFeishu,
		AccountID:   "cli-a",
		ChatType:    protocol.RoomTypeGroup,
		Ref:         "oc-reenable",
		ThreadID:    "omt-a",
	})
	if err != nil || newConcreteKey == oldConcreteKey {
		t.Fatalf("重新启用应创建新的 concrete Session: old=%q new=%q err=%v", oldConcreteKey, newConcreteKey, err)
	}
	if err = service.ValidateExternalSessionGrant(context.Background(), "owner-a", "agent-a", newConcreteKey); err != nil {
		t.Fatalf("新 concrete Session 应恢复当前授权: %v", err)
	}
}

func TestValidateExternalSessionGrantRejectsRotatedSessionKey(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	deleted := map[string]bool{}
	router.SetSessionProjectionResolver(deletedSessionResolver{deleted: deleted})
	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, router)
	created, err := service.CreatePairing(context.Background(), "owner-a", CreatePairingRequest{
		ChannelType: ChannelTypeWeChat,
		ChatType:    protocol.RoomTypeDM,
		ExternalRef: "wx-user-a",
		AgentID:     "agent-a",
		Status:      PairingStatusActive,
	})
	if err != nil {
		t.Fatalf("创建配对失败: %v", err)
	}
	if _, err = db.Exec("UPDATE im_pairings SET session_materialized = 1 WHERE pairing_id = ?", created.PairingID); err != nil {
		t.Fatalf("准备已物化 Session 失败: %v", err)
	}
	oldKey := created.SessionKey
	deleted[oldKey] = true
	_, rotatedKey, err := service.ResolveIngressSession(context.Background(), IngressRequest{
		OwnerUserID: "owner-a",
		Channel:     ChannelTypeWeChat,
		ChatType:    protocol.RoomTypeDM,
		Ref:         "wx-user-a",
	})
	if err != nil || rotatedKey == oldKey || protocol.ParseSessionKey(rotatedKey).Generation == "" {
		t.Fatalf("删除后应轮换 Session: old=%q new=%q err=%v", oldKey, rotatedKey, err)
	}
	if err = service.ValidateExternalSessionGrant(context.Background(), "owner-a", "agent-a", oldKey); err == nil {
		t.Fatal("已轮换的旧 Session key 不得通过 IM grant 校验")
	}
	if err = service.ValidateExternalSessionGrant(context.Background(), "owner-a", "agent-a", rotatedKey); err != nil {
		t.Fatalf("当前 Session key 应保持可授权: %v", err)
	}
}

func TestValidateExternalSessionGrantAcceptsGeneratedWildcardProjection(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	if _, err := service.CreatePairing(context.Background(), "owner-a", CreatePairingRequest{
		ChannelType: ChannelTypeFeishu,
		ChatType:    protocol.RoomTypeGroup,
		ExternalRef: "oc-wildcard",
		AgentID:     "agent-a",
		Status:      PairingStatusActive,
	}); err != nil {
		t.Fatal(err)
	}
	_, sessionKey, err := service.ResolveIngressSession(context.Background(), IngressRequest{
		OwnerUserID: "owner-a",
		Channel:     ChannelTypeFeishu,
		AccountID:   "cli-a",
		ChatType:    protocol.RoomTypeGroup,
		Ref:         "oc-wildcard",
		ThreadID:    "omt-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if protocol.ParseSessionKey(sessionKey).Generation == "" {
		t.Fatalf("具体通配映射应带独立代次: %q", sessionKey)
	}
	if err = service.ValidateExternalSessionGrant(context.Background(), "owner-a", "agent-a", sessionKey); err != nil {
		t.Fatalf("生成的通配具体 Session 必须可通过当前 pairing 授权: %v", err)
	}
}

func TestValidateExternalSessionGrantDoesNotUseExplicitPairingForAnotherThread(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	if _, err := service.CreatePairing(context.Background(), "owner-a", CreatePairingRequest{
		ChannelType: ChannelTypeFeishu,
		ChatType:    protocol.RoomTypeGroup,
		ExternalRef: "oc-explicit",
		ThreadID:    "omt-a",
		AgentID:     "agent-a",
		Status:      PairingStatusActive,
	}); err != nil {
		t.Fatal(err)
	}
	foreign := protocol.BuildAgentAccountSessionKey(
		"agent-a", protocol.SessionChannelFeishu, protocol.RoomTypeGroup,
		"cli-a", "oc-explicit", "omt-b",
	)
	if err := service.ValidateExternalSessionGrant(context.Background(), "owner-a", "agent-a", foreign); err == nil {
		t.Fatal("显式 topic pairing 不得授权另一个 topic")
	}
}

func TestSendAgentExternalSessionMessageRevalidatesAndProjects(t *testing.T) {
	workspaceRoot, workspacePath := newChannelOwnerWorkspace(t, authctx.SystemUserID, "agent-a")
	db := newChannelTestDB(t)
	defer db.Close()
	agents := &stubAgentResolver{agentByID: map[string]*protocol.Agent{
		"agent-a": {
			AgentID: "agent-a", OwnerUserID: authctx.SystemUserID, WorkspacePath: workspacePath,
		},
	}}
	router := NewRouter(config.Config{
		DatabaseDriver: "sqlite", WorkspacePath: workspaceRoot,
	}, db, agents, nil)
	external := &recordingDeliveryChannel{channelType: ChannelTypeWeixinPersonal}
	router.RegisterForOwner(authctx.SystemUserID, external)
	if err := router.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer router.Stop(context.Background())
	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, agents, router)
	router.SetIMDeliverySupport(imdelivery.NewRepository(config.Config{DatabaseDriver: "sqlite"}, db), service)
	pairing, err := service.CreatePairing(context.Background(), authctx.SystemUserID, CreatePairingRequest{
		ChannelType: ChannelTypeWeixinPersonal,
		AccountID:   "weixin-account",
		ChatType:    protocol.RoomTypeDM,
		ExternalRef: "weixin-user",
		AgentID:     "agent-a",
		Status:      PairingStatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	sessionKey := pairing.SessionKey
	now := time.Now().UTC()
	router.SetSessionProjectionResolver(databaseBackedDeliverySessionResolver{session: protocol.Session{
		SessionKey: sessionKey, AgentID: "agent-a",
		ChannelType: ChannelTypeWeixinPersonal, ChatType: protocol.RoomTypeDM,
		Status: "closed", CreatedAt: now, LastActivity: now, Title: "微信私聊",
	}})
	if _, err = router.RememberSessionRoute(context.Background(), "agent-a", sessionKey, DeliveryTarget{
		Mode: DeliveryModeExplicit, Channel: ChannelTypeWeixinPersonal,
		To: "weixin-user", AccountID: "weixin-account", SessionKey: sessionKey,
	}); err != nil {
		t.Fatal(err)
	}

	result, err := service.SendAgentExternalSessionMessage(
		context.Background(), authctx.SystemUserID, "agent-a", sessionKey, "主动提醒",
	)
	if err != nil {
		t.Fatalf("主动外部私聊投递失败: %v", err)
	}
	if result.Target.Channel != ChannelTypeWeixinPersonal || external.sentCount() != 1 {
		t.Fatalf("外部通道投递不正确: result=%+v sent=%d", result, external.sentCount())
	}
	stored, _, err := workspacestore.NewSessionFileStore(workspaceRoot).
		ForOwner(authctx.SystemUserID).
		FindSession([]string{workspacePath}, sessionKey)
	if err != nil || stored == nil {
		t.Fatalf("目标 Session 未物化: session=%+v err=%v", stored, err)
	}
	messages, err := workspacestore.NewAgentHistoryStore(workspaceRoot).
		ForOwner(authctx.SystemUserID).
		ReadMessages(workspacePath, *stored, nil)
	if err != nil || len(messages) != 1 || extractAssistantText(messages[0]) != "主动提醒" {
		t.Fatalf("目标 Session 投影不正确: messages=%+v err=%v", messages, err)
	}
	status := PairingStatusDisabled
	if _, err = service.UpdatePairing(context.Background(), authctx.SystemUserID, pairing.PairingID, UpdatePairingRequest{
		Status: &status,
	}); err != nil {
		t.Fatal(err)
	}
	_, err = service.SendAgentExternalSessionMessage(
		context.Background(), authctx.SystemUserID, "agent-a", sessionKey, "不应发出",
	)
	if !errors.Is(err, ErrExternalSessionGrantUnavailable) ||
		!strings.Contains(err.Error(), "pairing is not active") || external.sentCount() != 1 {
		t.Fatalf("撤权后必须 fail closed: err=%v sent=%d", err, external.sentCount())
	}
	_, err = router.DeliverMessage(ingressTestOwnerContext(authctx.SystemUserID), "agent-a", "旧 round 回复不应发出", DeliveryTarget{
		Mode: DeliveryModeExplicit, Channel: ChannelTypeWeixinPersonal, To: "weixin-user",
		AccountID: "weixin-account", SessionKey: sessionKey,
	})
	if !errors.Is(err, ErrExternalSessionGrantUnavailable) || external.sentCount() != 1 {
		t.Fatalf("普通 DM 回复在 pairing 撤权后也必须 fail closed: err=%v sent=%d", err, external.sentCount())
	}
	if err = router.SetTyping(ingressTestOwnerContext(authctx.SystemUserID), "agent-a", DeliveryTarget{
		Mode: DeliveryModeExplicit, Channel: ChannelTypeWeixinPersonal, To: "weixin-user",
		AccountID: "weixin-account", SessionKey: sessionKey,
	}, true); !errors.Is(err, ErrExternalSessionGrantUnavailable) {
		t.Fatalf("typing 在 pairing 撤权后必须 fail closed: %v", err)
	}
}

func TestRouterExternalDeliveryRejectsMissingSessionProjection(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	owner := authctx.SystemUserID
	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	external := &recordingDeliveryChannel{channelType: ChannelTypeFeishu}
	router.RegisterForOwner(owner, external)
	if err := router.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer router.Stop(context.Background())

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, router)
	paired, err := service.CreatePairing(context.Background(), owner, CreatePairingRequest{
		ChannelType: ChannelTypeFeishu,
		ChatType:    protocol.RoomTypeDM,
		ExternalRef: "ou-projection-missing",
		AgentID:     "agent-a",
		Status:      PairingStatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	// An active pairing alone is insufficient once the Router has a Session
	// projection resolver. This simulates a deleted/missing Session before the
	// next outbound send and proves the physical channel is never called.
	router.SetSessionProjectionResolver(imTestSessions{})
	router.SetIMDeliverySupport(imdelivery.NewRepository(config.Config{DatabaseDriver: "sqlite"}, db), service)
	_, err = router.DeliverMessage(ingressTestOwnerContext(owner), "agent-a", "不应发出", DeliveryTarget{
		Mode:       DeliveryModeExplicit,
		Channel:    ChannelTypeFeishu,
		To:         "ou-projection-missing",
		SessionKey: paired.SessionKey,
	})
	if !errors.Is(err, ErrExternalSessionGrantUnavailable) || external.sentCount() != 0 {
		t.Fatalf("缺失 Session projection 必须阻止物理投递: err=%v sent=%d", err, external.sentCount())
	}
}

func TestListAgentExternalSessionsReturnsOnlyActivePairedDMs(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, router)
	activeDM, err := service.CreatePairing(context.Background(), "owner-a", CreatePairingRequest{
		ChannelType:  ChannelTypeWeixinPersonal,
		AccountID:    "weixin-account",
		ChatType:     protocol.RoomTypeDM,
		ExternalRef:  "weixin-user",
		ExternalName: "捷哥",
		AgentID:      "agent-a",
		Status:       PairingStatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []CreatePairingRequest{
		{ChannelType: ChannelTypeWeixinPersonal, ChatType: protocol.RoomTypeGroup, ExternalRef: "group", AgentID: "agent-a", Status: PairingStatusActive},
		{ChannelType: ChannelTypeWeixinPersonal, ChatType: protocol.RoomTypeDM, ExternalRef: "other-agent", AgentID: "agent-b", Status: PairingStatusActive},
		{ChannelType: ChannelTypeWeixinPersonal, ChatType: protocol.RoomTypeDM, ExternalRef: "disabled", AgentID: "agent-a", Status: PairingStatusDisabled},
	} {
		if _, err = service.CreatePairing(context.Background(), "owner-a", request); err != nil {
			t.Fatal(err)
		}
	}
	router.SetSessionProjectionResolver(databaseBackedDeliverySessionResolver{session: protocol.Session{
		SessionKey: activeDM.SessionKey,
		AgentID:    "agent-a",
		Title:      "真实微信私聊",
	}})
	items, err := service.ListAgentExternalSessions(
		context.Background(), "owner-a", "agent-a", ChannelTypeWeixinPersonal,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].SessionKey != activeDM.SessionKey || items[0].Label != "捷哥" {
		t.Fatalf("Agent delivery sessions = %+v", items)
	}
	if _, err = service.ListAgentExternalSessions(
		context.Background(), "owner-a", "agent-a", ChannelTypeWebSocket,
	); !errors.Is(err, ErrExternalSessionGrantUnavailable) {
		t.Fatalf("non-IM delivery directory error = %v", err)
	}
}

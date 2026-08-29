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

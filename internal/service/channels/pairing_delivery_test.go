package channels

import (
	"context"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestValidateAutomationDeliveryGrantRequiresExactActivePairing(t *testing.T) {
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
	if err = service.ValidateAutomationDeliveryGrant(context.Background(), "owner-a", "agent-a", sessionKey); err != nil {
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
	if err = service.ValidateAutomationDeliveryGrant(context.Background(), "owner-a", "agent-a", otherTarget); err == nil {
		t.Fatal("同账号下不同 IM 对话不得复用 pairing grant")
	}
	status := PairingStatusPending
	if _, err = service.UpdatePairing(context.Background(), "owner-a", created.PairingID, UpdatePairingRequest{
		Status: &status,
	}); err != nil {
		t.Fatalf("撤销 active pairing 失败: %v", err)
	}
	if err = service.ValidateAutomationDeliveryGrant(context.Background(), "owner-a", "agent-a", sessionKey); err == nil {
		t.Fatal("pairing 不再 active 后 Automation 投递必须立即 fail closed")
	}
}

func TestListAutomationDeliverySessionsReturnsOnlyActivePairedDMs(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
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
	items, err := service.ListAutomationDeliverySessions(
		context.Background(), "owner-a", "agent-a", ChannelTypeWeixinPersonal,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].SessionKey != activeDM.SessionKey || items[0].Label != "捷哥" {
		t.Fatalf("Automation delivery sessions = %+v", items)
	}
	if _, err = service.ListAutomationDeliverySessions(
		context.Background(), "owner-a", "agent-a", ChannelTypeWebSocket,
	); !errors.Is(err, ErrExternalSessionGrantUnavailable) {
		t.Fatalf("non-IM delivery directory error = %v", err)
	}
}

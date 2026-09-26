package channels

import (
	"context"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	management "github.com/nexus-research-lab/nexus/internal/service/channels/management"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
)

func TestPairingRoomTargetSwitchKeepsSessionAndFencesOldDelivery(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer db.Close()
	ctx := contextWithIngressOwner(context.Background(), "owner-room-binding")
	agents := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	agent, err := agents.CreateAgent(ctx, protocol.CreateRequest{Name: "负责人"})
	if err != nil {
		t.Fatal(err)
	}
	rooms := roomsvc.NewService(cfg, agents, roomrepo.NewSQLRepository("sqlite", db))
	room, err := rooms.CreateRoom(ctx, protocol.CreateRoomRequest{AgentIDs: []string{agent.AgentID}, Name: "项目", PrivateMessagesEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	control := NewControlService(cfg, db, agents, nil)
	control.SetRoomService(rooms)
	pairing, err := control.CreatePairing(ctx, authctx.OwnerUserID(ctx), CreatePairingRequest{ChannelType: ChannelTypeFeishu, ChatType: "dm", ExternalRef: "person", AgentID: agent.AgentID})
	if err != nil {
		t.Fatal(err)
	}
	original := pairing.SessionKey
	target := DeliveryTarget{Mode: DeliveryModeExplicit, Channel: ChannelTypeFeishu, To: "person", SessionKey: original, PairingID: pairing.PairingID, BindingVersion: pairing.BindingVersion}
	binding := management.PairingSessionTarget{RoomID: room.Room.ID, ConversationID: room.Conversation.ID}
	next, err := control.UpdatePairing(ctx, authctx.OwnerUserID(ctx), pairing.PairingID, UpdatePairingRequest{SessionTarget: &binding, BindingVersion: &pairing.BindingVersion})
	if err != nil {
		t.Fatal(err)
	}
	if next.SessionKey != original || next.BindingVersion != pairing.BindingVersion+1 || next.SessionTarget.RoomName != "项目" {
		t.Fatalf("切换不应新建传输会话: %+v", next)
	}
	if _, err := control.ValidateBindingDelivery(ctx, authctx.OwnerUserID(ctx), agent.AgentID, target); !errors.Is(err, ErrExternalSessionGrantUnavailable) {
		t.Fatalf("旧回复必须失效: %v", err)
	}
	target.BindingVersion = next.BindingVersion
	if bound, err := control.ValidateBindingDelivery(ctx, authctx.OwnerUserID(ctx), agent.AgentID, target); err != nil || !bound {
		t.Fatalf("新 Room 目标未生效: %v %v", bound, err)
	}
	if _, err := control.UpdatePairing(ctx, authctx.OwnerUserID(ctx), pairing.PairingID, UpdatePairingRequest{SessionTarget: &binding, BindingVersion: &pairing.BindingVersion}); err == nil {
		t.Fatal("旧版本更新必须拒绝")
	}
	independent := management.PairingSessionTarget{}
	restored, err := control.UpdatePairing(ctx, authctx.OwnerUserID(ctx), pairing.PairingID, UpdatePairingRequest{SessionTarget: &independent, BindingVersion: &next.BindingVersion})
	if err != nil {
		t.Fatal(err)
	}
	if restored.SessionKey != original || restored.SessionTarget.RoomID != "" {
		t.Fatalf("独立会话历史身份丢失: %+v", restored)
	}
	if _, err := control.ValidateBindingDelivery(ctx, authctx.OwnerUserID(ctx), agent.AgentID, target); !errors.Is(err, ErrExternalSessionGrantUnavailable) {
		t.Fatalf("Room 旧轮次不能再回信: %v", err)
	}
	bad := binding
	bad.RoomID = "another-room"
	if _, err := control.UpdatePairing(ctx, authctx.OwnerUserID(ctx), pairing.PairingID, UpdatePairingRequest{SessionTarget: &bad, BindingVersion: &restored.BindingVersion}); err == nil {
		t.Fatal("错误 Room / conversation 组合必须拒绝")
	}
	otherCtx := contextWithIngressOwner(ctx, "other-owner")
	if err := control.validatePairingRoom(otherCtx, agent.AgentID, binding); err == nil {
		t.Fatal("跨 owner 目标必须拒绝")
	}
}

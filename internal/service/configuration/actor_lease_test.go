// INPUT: DM 或 Room configuration Actor 的业务标识与 runtime lease。
// OUTPUT: 仅真实运行中的 lease 可通过，stale 或错误槽位一律拒绝。
// POS: configuration 每次调用 active-round 重校验的边界回归测试。
package configuration

import (
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

func TestRequireActiveRoundUsesDMAndRoomRuntimeLease(t *testing.T) {
	tests := []struct {
		name            string
		sessionKey      string
		roundID         string
		leaseSessionKey string
		leaseRoundID    string
	}{
		{
			name: "dm",
			sessionKey: protocol.BuildAgentSessionKey(
				"agent-a", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "main", "",
			),
			roundID: "dm-round-a",
			leaseSessionKey: protocol.BuildAgentSessionKey(
				"agent-a", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "main", "",
			),
			leaseRoundID: "dm-round-a",
		},
		{
			name:       "room agent slot",
			sessionKey: protocol.BuildRoomSharedSessionKey("conversation-a"),
			roundID:    "root-round-a",
			leaseSessionKey: protocol.BuildRoomAgentSessionKey(
				"conversation-a", "agent-a", protocol.RoomTypeGroup,
			),
			leaseRoundID: "agent-round-a",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manager := runtimectx.NewManager()
			if err := manager.StartRound(t.Context(), test.leaseSessionKey, test.leaseRoundID, nil); err != nil {
				t.Fatalf("start runtime lease: %v", err)
			}
			service := &Service{runtime: manager}
			actor := Actor{
				SessionKey: test.sessionKey, RoundID: test.roundID,
				LeaseSessionKey: test.leaseSessionKey, LeaseRoundID: test.leaseRoundID,
				RoundLeaseRequired: true,
			}
			if err := service.requireActiveRound(actor); err != nil {
				t.Fatalf("active lease rejected: %v", err)
			}

			stale := actor
			stale.LeaseRoundID += "-stale"
			if err := service.requireActiveRound(stale); err == nil ||
				!strings.Contains(err.Error(), "已结束") {
				t.Fatalf("stale lease error = %v", err)
			}

			manager.MarkRoundFinished(test.leaseSessionKey, test.leaseRoundID)
			if err := service.requireActiveRound(actor); err == nil ||
				!strings.Contains(err.Error(), "已结束") {
				t.Fatalf("finished lease error = %v", err)
			}
		})
	}
}

func TestValidateAgentRuntimeIdentityRejectsTransferAcrossDMs(t *testing.T) {
	dmSession := protocol.BuildAgentSessionKey(
		"agent-a", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "dm-a", "",
	)
	actor := Actor{
		AgentID: "agent-a", SessionKey: dmSession, RoundID: "round-a",
		LeaseSessionKey: dmSession, LeaseRoundID: "round-a",
		RoundLeaseRequired: true,
	}
	if err := validateAgentRuntimeIdentity(actor); err != nil {
		t.Fatalf("valid DM identity rejected: %v", err)
	}

	swappedSession := actor
	swappedSession.LeaseSessionKey = protocol.BuildAgentSessionKey(
		"agent-a", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "dm-b", "",
	)
	if err := validateAgentRuntimeIdentity(swappedSession); err == nil ||
		!strings.Contains(err.Error(), "不匹配") {
		t.Fatalf("swapped DM lease error = %v", err)
	}

	external := actor
	external.SessionKey = protocol.BuildAgentSessionKey(
		"agent-a", "telegram", protocol.RoomTypeDM, "chat-a", "",
	)
	external.LeaseSessionKey = external.SessionKey
	if err := validateAgentRuntimeIdentity(external); err == nil ||
		!strings.Contains(err.Error(), "WebSocket") {
		t.Fatalf("external channel lease error = %v", err)
	}
}

func TestValidateRoomRuntimeIdentityRejectsTransferAcrossAgentSlots(t *testing.T) {
	contextValue := &protocol.ConversationContextAggregate{
		Room: protocol.RoomRecord{ID: "room-a", RoomType: protocol.RoomTypeGroup},
		Conversation: protocol.ConversationRecord{
			ID: "conversation-a", RoomID: "room-a",
		},
	}
	actor := Actor{
		AgentID: "agent-a", ConversationID: "conversation-a",
		SessionKey: protocol.BuildRoomSharedSessionKey("conversation-a"),
		LeaseSessionKey: protocol.BuildRoomAgentSessionKey(
			"conversation-a", "agent-a", protocol.RoomTypeGroup,
		),
		RoundLeaseRequired: true,
	}
	if err := validateRoomRuntimeIdentityValue(actor, "room-a", contextValue); err != nil {
		t.Fatalf("valid Room identity rejected: %v", err)
	}

	swappedAgent := actor
	swappedAgent.LeaseSessionKey = protocol.BuildRoomAgentSessionKey(
		"conversation-a", "agent-b", protocol.RoomTypeGroup,
	)
	if err := validateRoomRuntimeIdentityValue(swappedAgent, "room-a", contextValue); err == nil ||
		!strings.Contains(err.Error(), "Agent slot") {
		t.Fatalf("swapped Room Agent lease error = %v", err)
	}

	swappedConversation := actor
	swappedConversation.LeaseSessionKey = protocol.BuildRoomAgentSessionKey(
		"conversation-b", "agent-a", protocol.RoomTypeGroup,
	)
	if err := validateRoomRuntimeIdentityValue(swappedConversation, "room-a", contextValue); err == nil ||
		!strings.Contains(err.Error(), "Agent slot") {
		t.Fatalf("swapped Room conversation lease error = %v", err)
	}
}

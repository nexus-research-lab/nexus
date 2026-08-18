package realtime

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRoomConfigurationContextRequiresTrustedUserInput(t *testing.T) {
	tests := []struct {
		name  string
		round *activeRoomRound
		want  string
	}{
		{
			name:  "trusted websocket",
			round: &activeRoomRound{TrustedConfigurationContext: true},
			want:  "room",
		},
		{
			name:  "missing trusted admission",
			round: &activeRoomRound{},
			want:  "room_untrusted",
		},
		{
			name:  "automation",
			round: &activeRoomRound{TrustedConfigurationContext: true, ExecutionOrigin: "automation"},
			want:  "room_automation",
		},
		{
			name: "forged queue origin",
			round: &activeRoomRound{
				ExecutionOrigin: "queue",
			},
			want: "room_queue",
		},
		{
			name: "server claimed direct user queue",
			round: &activeRoomRound{
				ExecutionOrigin:                   "queue",
				trustedQueuedConfigurationContext: true,
			},
			want: "room",
		},
		{
			name:  "internal wake",
			round: &activeRoomRound{TrustedConfigurationContext: true, Internal: true},
			want:  "room_internal",
		},
		{
			name: "nil",
			want: "room_untrusted",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := roomCommandSourceContextType(test.round); got != test.want {
				t.Fatalf("source context = %q, want %q", got, test.want)
			}
		})
	}
}

func TestTrustedQueuedRoomContextDoesNotPropagateToAgentHandoffs(t *testing.T) {
	contextValue := &protocol.ConversationContextAggregate{
		Room: protocol.RoomRecord{ID: "room-1", RoomType: protocol.RoomTypeGroup},
		Conversation: protocol.ConversationRecord{
			ID: "conversation-1",
		},
	}
	dispatchScaffold := &activeRoomRound{
		Context:                     contextValue,
		OwnerUserID:                 "owner",
		pendingTrustedQueueDispatch: true,
	}
	userRound := newPublicMentionRound(dispatchScaffold, "room:group:conversation-1", "user-round")
	if got := roomCommandSourceContextType(userRound); got != "room" {
		t.Fatalf("direct queued user round source = %q, want room", got)
	}
	if userRound.pendingTrustedQueueDispatch {
		t.Fatal("direct queued user trust must be consumed by one runtime hop")
	}
	agentHandoff := newPublicMentionRound(userRound, "room:group:conversation-1", "agent-handoff")
	if got := roomCommandSourceContextType(agentHandoff); got != "room_handoff" {
		t.Fatalf("agent handoff source = %q, want room_handoff", got)
	}
}

func TestUntrustedRoomGuideFallsBackToQueue(t *testing.T) {
	if got := safeRoomDeliveryPolicy(ChatRequest{DeliveryPolicy: protocol.ChatDeliveryPolicyGuide}); got != protocol.ChatDeliveryPolicyQueue {
		t.Fatalf("untrusted guide policy = %q", got)
	}
	if got := safeRoomDeliveryPolicy(ChatRequest{
		DeliveryPolicy:              protocol.ChatDeliveryPolicyGuide,
		TrustedConfigurationContext: true,
	}); got != protocol.ChatDeliveryPolicyGuide {
		t.Fatalf("trusted guide policy = %q", got)
	}
}

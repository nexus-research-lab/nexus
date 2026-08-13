package types

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestValidateSelfScopedDeliveryTarget(t *testing.T) {
	agentID := "agent-1"
	currentDM := protocol.BuildAgentSessionKey(
		agentID,
		protocol.SessionChannelWebSocketSegment,
		"dm",
		"current",
		"",
	)
	currentRoom := protocol.BuildRoomSharedSessionKey("conversation-1")
	currentExternal := protocol.BuildAgentAccountSessionKey(
		agentID,
		protocol.SessionChannelFeishuSegment,
		"group",
		"account-1",
		"chat-1",
		"thread-1",
	)
	ownSession := protocol.BuildAgentSessionKey(
		agentID,
		protocol.SessionChannelInternalSegment,
		"dm",
		"selected-session",
		"",
	)
	otherSession := protocol.BuildAgentSessionKey(
		"agent-2",
		protocol.SessionChannelInternalSegment,
		"dm",
		"selected-session",
		"",
	)

	tests := []struct {
		name    string
		grant   string
		target  DeliveryTarget
		wantErr bool
	}{
		{
			name:   "none",
			grant:  currentDM,
			target: DeliveryTarget{Mode: DeliveryModeNone},
		},
		{
			name:  "own real agent session",
			grant: currentDM,
			target: DeliveryTarget{
				Mode: DeliveryModeExplicit, Channel: protocol.SessionChannelInternalSegment, To: ownSession,
			},
		},
		{
			name:  "current room",
			grant: currentRoom,
			target: DeliveryTarget{
				Mode: DeliveryModeExplicit, Channel: protocol.SessionChannelWebSocket, To: currentRoom,
			},
		},
		{
			name:  "current external conversation",
			grant: currentExternal,
			target: DeliveryTarget{
				Mode: DeliveryModeExplicit, Channel: protocol.SessionChannelFeishu,
				To: "chat-1", AccountID: "account-1", ThreadID: "thread-1",
			},
		},
		{
			name:    "other agent",
			grant:   currentDM,
			target:  DeliveryTarget{Mode: DeliveryModeExplicit, Channel: protocol.SessionChannelInternalSegment, To: otherSession},
			wantErr: true,
		},
		{
			name:    "other room",
			grant:   currentRoom,
			target:  DeliveryTarget{Mode: DeliveryModeExplicit, Channel: protocol.SessionChannelWebSocket, To: protocol.BuildRoomSharedSessionKey("conversation-2")},
			wantErr: true,
		},
		{
			name:    "arbitrary external conversation",
			grant:   currentDM,
			target:  DeliveryTarget{Mode: DeliveryModeExplicit, Channel: protocol.SessionChannelFeishu, To: "chat-2"},
			wantErr: true,
		},
		{
			name:    "different external account",
			grant:   currentExternal,
			target:  DeliveryTarget{Mode: DeliveryModeExplicit, Channel: protocol.SessionChannelFeishu, To: "chat-1", AccountID: "account-2", ThreadID: "thread-1"},
			wantErr: true,
		},
		{
			name:    "last route",
			grant:   currentExternal,
			target:  DeliveryTarget{Mode: DeliveryModeLast},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateSelfScopedDeliveryTarget(agentID, test.grant, test.target)
			if test.wantErr && err == nil {
				t.Fatal("expected delivery boundary error")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("unexpected delivery boundary error: %v", err)
			}
		})
	}
}

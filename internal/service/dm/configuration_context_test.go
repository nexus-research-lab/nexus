package dm

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestDMConfigurationContextRequiresTrustedWebSocketInput(t *testing.T) {
	webSession := protocol.BuildAgentSessionKey(
		"worker",
		protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM,
		"main",
		"",
	)
	externalSession := protocol.BuildAgentSessionKey("worker", "telegram", protocol.RoomTypeDM, "chat", "")
	tests := []struct {
		name    string
		session string
		request Request
		want    string
	}{
		{
			name:    "trusted websocket",
			session: webSession,
			request: Request{TrustedConfigurationContext: true},
			want:    "agent",
		},
		{
			name:    "missing trusted admission",
			session: webSession,
			request: Request{},
			want:    "agent_untrusted",
		},
		{
			name:    "external session cannot forge trusted admission",
			session: externalSession,
			request: Request{TrustedConfigurationContext: true},
			want:    "agent_untrusted",
		},
		{
			name:    "channel origin",
			session: webSession,
			request: Request{TrustedConfigurationContext: true, ExecutionOrigin: "channel"},
			want:    "agent_channel",
		},
		{
			name:    "active paired external dm",
			session: externalSession,
			request: Request{
				ExecutionOrigin:                   "channel",
				TrustedExternalInteractiveContext: true,
			},
			want: "agent_paired",
		},
		{
			name:    "paired flag cannot upgrade websocket",
			session: webSession,
			request: Request{
				ExecutionOrigin:                   "channel",
				TrustedExternalInteractiveContext: true,
			},
			want: "agent_channel",
		},
		{
			name:    "paired flag cannot upgrade external group",
			session: protocol.BuildAgentSessionKey("worker", "telegram", protocol.RoomTypeGroup, "chat", ""),
			request: Request{
				ExecutionOrigin:                   "channel",
				TrustedExternalInteractiveContext: true,
			},
			want: "agent_channel",
		},
		{
			name:    "queue origin",
			session: webSession,
			request: Request{ExecutionOrigin: "queue"},
			want:    "agent_queue",
		},
		{
			name:    "server claimed direct user queue",
			session: webSession,
			request: Request{
				ExecutionOrigin:                   "queue",
				trustedQueuedConfigurationContext: true,
			},
			want: "agent",
		},
		{
			name:    "internal",
			session: webSession,
			request: Request{TrustedConfigurationContext: true, Internal: true},
			want:    "agent_internal",
		},
		{
			name:    "mismatched agent",
			session: webSession,
			request: Request{TrustedConfigurationContext: true},
			want:    "agent_untrusted",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			agentID := "worker"
			if test.name == "mismatched agent" {
				agentID = "other"
			}
			if got := dmMCPSourceContextType(test.session, agentID, test.request); got != test.want {
				t.Fatalf("source context = %q, want %q", got, test.want)
			}
		})
	}
}

func TestUntrustedDMGuideFallsBackToQueue(t *testing.T) {
	if got := safeDMDeliveryPolicy(Request{DeliveryPolicy: protocol.ChatDeliveryPolicyGuide}); got != protocol.ChatDeliveryPolicyQueue {
		t.Fatalf("untrusted guide policy = %q", got)
	}
	if got := safeDMDeliveryPolicy(Request{
		DeliveryPolicy:              protocol.ChatDeliveryPolicyGuide,
		TrustedConfigurationContext: true,
	}); got != protocol.ChatDeliveryPolicyGuide {
		t.Fatalf("trusted guide policy = %q", got)
	}
}

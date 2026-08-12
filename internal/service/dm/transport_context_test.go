package dm

import (
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

func TestRoundRunnerTrustedIMTransportContextCoversEveryChannelWithoutRouteIDs(t *testing.T) {
	channels := []string{
		protocol.SessionChannelDiscord,
		protocol.SessionChannelTelegram,
		protocol.SessionChannelDingTalk,
		protocol.SessionChannelWeChat,
		protocol.SessionChannelWeixinPersonal,
		protocol.SessionChannelFeishu,
	}
	for _, channel := range channels {
		t.Run(channel, func(t *testing.T) {
			runner := &roundRunner{
				agent:                      &protocol.Agent{AgentID: "agent-1"},
				sessionKey:                 protocol.BuildAgentAccountSessionKey("agent-1", channel, protocol.RoomTypeDM, "secret-account", "secret-user", "secret-thread"),
				trustedExternalInteractive: true,
			}
			inputs := runner.contextualInputs()
			if len(inputs) != 1 || inputs[0].Name != runtimectx.ContextualInputNameTransport {
				t.Fatalf("trusted %s contextual inputs = %+v", channel, inputs)
			}
			content := inputs[0].Content
			if !strings.Contains(content, `transport="im"`) ||
				!strings.Contains(content, `channel="`+channel+`"`) ||
				!strings.Contains(content, `chat_type="dm"`) ||
				!strings.Contains(content, `deliver_result=true`) ||
				strings.Contains(content, `reply_mode`) {
				t.Fatalf("trusted %s transport context missing safe facts: %q", channel, content)
			}
			for _, secret := range []string{"secret-account", "secret-user", "secret-thread", runner.sessionKey} {
				if strings.Contains(content, secret) {
					t.Fatalf("trusted %s transport context leaked route identifier %q: %q", channel, secret, content)
				}
			}
		})
	}
}

func TestRoundRunnerUntrustedExternalSessionOmitsTransportContext(t *testing.T) {
	runner := &roundRunner{
		agent:      &protocol.Agent{AgentID: "agent-1"},
		sessionKey: protocol.BuildAgentSessionKey("agent-1", protocol.SessionChannelTelegram, protocol.RoomTypeDM, "external-user", ""),
	}
	if inputs := runner.contextualInputs(); len(inputs) != 0 {
		t.Fatalf("untrusted external session contextual inputs = %+v, want empty", inputs)
	}
}

package dm

import (
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
)

// DM 告知与 runtimehost 的 compact 清理必须落在同一个会话键上，否则 compact 后状态永远不再告知。
func TestConnectorContextReannouncedAfterCompact(t *testing.T) {
	service := &Service{}
	service.Announcements = &runtimehost.ContextAnnouncements{}
	sessionID := "sdk-1"
	runner := &roundRunner{
		service:          service,
		sessionKey:       "agent:main:dm:1",
		session:          protocol.Session{SessionID: &sessionID},
		connectorContext: "<connector_runtime_state/>",
	}
	if len(runner.connectorContextualInputs()) != 1 {
		t.Fatal("first round must announce connector state")
	}
	if len(runner.connectorContextualInputs()) != 0 {
		t.Fatal("unchanged connector state must not be resent")
	}
	service.Announcements.ForgetAfterCompact(runner.sessionKey, sdkprotocol.ReceivedMessage{
		Type:   sdkprotocol.MessageTypeSystem,
		System: &sdkprotocol.SystemMessage{Subtype: "compact_boundary"},
	})
	if len(runner.connectorContextualInputs()) != 1 {
		t.Fatal("compact boundary must force connector state to be announced again")
	}
}

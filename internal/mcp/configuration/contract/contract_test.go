// INPUT: Room 的共享业务会话/root round 与 Agent slot runtime lease。
// OUTPUT: configuration Actor 保持两套标识独立且强制 lease 校验。
// POS: nexus_config transport 身份映射的回归测试。
package contract

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestServerContextActorKeepsBusinessAndLeaseIdentitySeparate(t *testing.T) {
	businessSession := protocol.BuildRoomSharedSessionKey("conversation-a")
	leaseSession := protocol.BuildRoomAgentSessionKey(
		"conversation-a", "agent-a", protocol.RoomTypeGroup,
	)
	actor := (ServerContext{
		OwnerUserID:       "owner-a",
		CurrentAgentID:    "agent-a",
		CurrentSessionKey: businessSession,
		CurrentRoundID:    "root-round-a",
		LeaseSessionKey:   leaseSession,
		LeaseRoundID:      "agent-round-a",
		ContextKind:       "room",
		ContextID:         "room-a",
		RoomID:            "room-a",
		ConversationID:    "conversation-a",
	}).Actor()

	if actor.SessionKey != businessSession ||
		actor.RoundID != "root-round-a" {
		t.Fatalf("business identity changed: %+v", actor)
	}
	if actor.LeaseSessionKey != leaseSession ||
		actor.LeaseRoundID != "agent-round-a" {
		t.Fatalf("runtime lease changed: %+v", actor)
	}
	if actor.ConversationID != "conversation-a" {
		t.Fatalf("conversation identity changed: %+v", actor)
	}
	if !actor.RoundLeaseRequired {
		t.Fatal("configuration MCP actor must require an active runtime lease")
	}
}

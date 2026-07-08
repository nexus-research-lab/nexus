package room

import (
	"context"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

type permissionModeTestClient struct {
	modes []sdkpermission.Mode
}

func (c *permissionModeTestClient) Connect(context.Context) error { return nil }

func (c *permissionModeTestClient) Query(context.Context, string) error { return nil }

func (c *permissionModeTestClient) ReceiveMessages(context.Context) <-chan sdkprotocol.ReceivedMessage {
	closed := make(chan sdkprotocol.ReceivedMessage)
	close(closed)
	return closed
}

func (c *permissionModeTestClient) Interrupt(context.Context) error { return nil }

func (c *permissionModeTestClient) StopTask(context.Context, string) error { return nil }

func (c *permissionModeTestClient) SendTaskMessage(context.Context, string, string, string) error {
	return nil
}

func (c *permissionModeTestClient) RemoveMessages(context.Context, []string) error { return nil }

func (c *permissionModeTestClient) SetPermissionMode(_ context.Context, mode sdkpermission.Mode) error {
	c.modes = append(c.modes, mode)
	return nil
}

func (c *permissionModeTestClient) Disconnect(context.Context) error { return nil }

func (c *permissionModeTestClient) Reconfigure(context.Context, agentclient.Options) error {
	return nil
}

func (c *permissionModeTestClient) SessionID() string { return "" }

func TestSetPermissionModeForAgentUpdatesActiveRoomSlots(t *testing.T) {
	matching := &permissionModeTestClient{}
	other := &permissionModeTestClient{}
	terminal := &permissionModeTestClient{}
	service := &RealtimeService{activeRounds: map[string]*activeRoomRound{
		"round-1": {
			Slots: map[string]*activeRoomSlot{
				"matching": {AgentID: "agent-a", Client: matching, Status: "running"},
				"other":    {AgentID: "agent-b", Client: other, Status: "running"},
				"terminal": {AgentID: "agent-a", Client: terminal, Status: "finished"},
			},
		},
	}}

	if err := service.SetPermissionModeForAgent(context.Background(), "agent-a", sdkpermission.ModePlan); err != nil {
		t.Fatalf("SetPermissionModeForAgent() error = %v", err)
	}
	if len(matching.modes) != 1 || matching.modes[0] != sdkpermission.ModePlan {
		t.Fatalf("matching modes = %#v，期望 [plan]", matching.modes)
	}
	if len(other.modes) != 0 {
		t.Fatalf("other modes = %#v，期望空", other.modes)
	}
	if len(terminal.modes) != 0 {
		t.Fatalf("terminal modes = %#v，期望空", terminal.modes)
	}
}

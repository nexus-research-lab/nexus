package realtime

import (
	"context"
	"errors"
	"testing"
	"time"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
)

type sandboxTransitionClient struct {
	permissionModeTestClient
	closed, retired, queries int
	closeErr                 error
}

func (c *sandboxTransitionClient) Retire()                             { c.retired++ }
func (c *sandboxTransitionClient) Disconnect(context.Context) error    { c.closed++; return c.closeErr }
func (c *sandboxTransitionClient) Query(context.Context, string) error { c.queries++; return nil }

func TestRoomSandboxTransitionCancelsApprovalAndClosesWithoutReplay(t *testing.T) {
	permissions := permissionctx.NewContext()
	key := protocol.BuildRoomAgentSessionKey("sandbox-room", "agent-a", protocol.RoomTypeGroup)
	sender := &roomInterruptPermissionSender{key: "sandbox-transition", events: make(chan protocol.EventMessage, 8)}
	permissions.BindSession(key, sender)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	decision := make(chan sdkpermission.Decision, 1)
	go func() {
		result, _ := permissions.RequestPermission(ctx, key, sdkpermission.Request{ToolName: "Bash", Input: map[string]any{"command": "echo pending"}})
		decision <- result
	}()
	select {
	case <-sender.events:
	case <-time.After(time.Second):
		t.Fatal("permission request not presented")
	}
	client := &sandboxTransitionClient{permissionModeTestClient: permissionModeTestClient{modeErr: runtimectx.ErrDesktopSandboxPolicyChanged}}
	other := &sandboxTransitionClient{}
	slot := &activeRoomSlot{AgentID: "agent-a", RuntimeSessionKey: key}
	slot.setClient(client)
	slot.setStatus("running")
	sibling := &activeRoomSlot{AgentID: "agent-b"}
	sibling.setClient(other)
	sibling.setStatus("running")
	service := &Service{permission: permissions, rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{"r": {Slots: map[string]*activeRoomSlot{"a": slot, "b": sibling}}})}
	if err := service.SetPermissionModeForAgent(ctx, "agent-a", sdkpermission.ModeBypassPermissions); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-decision:
		if got.Behavior != sdkpermission.BehaviorDeny || !got.Interrupt {
			t.Fatalf("old approval survived transition: %#v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("old approval was left pending")
	}
	if client.closed != 1 || client.retired != 1 || client.queries != 0 || !slot.isTerminal() {
		t.Fatalf("transition lifecycle incorrect: %+v", client)
	}
	if other.closed != 0 || other.retired != 0 || sibling.isTerminal() {
		t.Fatal("transition affected sibling Agent")
	}
}

func TestRoomSandboxTransitionDoesNotHideCleanupFailure(t *testing.T) {
	failure := errors.New("exit not confirmed")
	client := &sandboxTransitionClient{closeErr: failure}
	slot := &activeRoomSlot{}
	slot.setStatus("running")
	service := &Service{}
	if err := service.closeSlotForSandboxPolicyChange(context.Background(), slot, client); !errors.Is(err, failure) {
		t.Fatalf("cleanup failure hidden: %v", err)
	}
	if !slot.isTerminal() || client.queries != 0 {
		t.Fatal("failed cleanup resumed old work")
	}
}

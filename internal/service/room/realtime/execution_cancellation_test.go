package realtime

import (
	"context"
	"sync"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type cancellationProviderClient struct {
	mu             sync.Mutex
	interruptCalls int
	onInterrupt    func()
}

func (c *cancellationProviderClient) Connect(context.Context) error { return nil }

func (c *cancellationProviderClient) Query(context.Context, string) error { return nil }

func (c *cancellationProviderClient) ReceiveMessages(
	context.Context,
) <-chan sdkprotocol.ReceivedMessage {
	messages := make(chan sdkprotocol.ReceivedMessage)
	close(messages)
	return messages
}

func (c *cancellationProviderClient) Interrupt(context.Context) error {
	c.mu.Lock()
	c.interruptCalls++
	callback := c.onInterrupt
	c.mu.Unlock()
	if callback != nil {
		callback()
	}
	return nil
}

func (c *cancellationProviderClient) StopTask(context.Context, string) error {
	return nil
}

func (c *cancellationProviderClient) SendTaskMessage(
	context.Context,
	string,
	string,
	string,
) error {
	return nil
}

func (c *cancellationProviderClient) RemoveMessages(
	context.Context,
	[]string,
) error {
	return nil
}

func (c *cancellationProviderClient) SetPermissionMode(
	context.Context,
	sdkpermission.Mode,
) error {
	return nil
}

func (c *cancellationProviderClient) Retire() {}

func (c *cancellationProviderClient) Disconnect(context.Context) error {
	return nil
}

func (c *cancellationProviderClient) Reconfigure(
	context.Context,
	agentclient.Options,
) error {
	return nil
}

func (c *cancellationProviderClient) SessionID() string {
	return "room-cancellation-provider-session"
}

type cancellationProviderFactory struct {
	client runtimectx.Client
}

func (f cancellationProviderFactory) New(
	agentclient.Options,
) runtimectx.Client {
	return f.client
}

func TestExecutionCancellationUsesProviderForExactSoleRoomSlot(t *testing.T) {
	runtimeSessionKey := "agent:agent-worker:ws:group:provider-cancel"
	agentRoundID := "agent-round-provider"
	client := &cancellationProviderClient{}
	factory := cancellationProviderFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	if _, err := runtimeManager.GetOrCreate(
		context.Background(),
		runtimeSessionKey,
		agentclient.Options{},
	); err != nil {
		t.Fatal(err)
	}
	client.onInterrupt = func() {
		runtimeManager.MarkRoundFinished(runtimeSessionKey, agentRoundID)
	}
	if err := runtimeManager.StartRound(context.Background(), runtimeSessionKey, agentRoundID, func() {
		runtimeManager.MarkRoundFinished(runtimeSessionKey, agentRoundID)
	}); err != nil {
		t.Fatalf("failed to register exact Room runtime round: %v", err)
	}
	service := &Service{
		rounds:     newRoomRoundRegistry(),
		runtime:    runtimeManager,
		permission: permissionctx.NewContext(),
	}
	binding := &protocol.ExecutionWorkBinding{
		ExecutionID:  "execution-room",
		PlanID:       "plan-room",
		WorkItemID:   "work-room",
		SpecID:       "spec-room",
		AssignmentID: "assignment-room",
		AttemptID:    "attempt-room",
		DispatchID:   "dispatch-room",
	}
	slot := &activeRoomSlot{
		AgentID:           "agent-worker",
		AgentRoundID:      agentRoundID,
		RuntimeSessionKey: runtimeSessionKey,
		WorkBinding:       binding,
	}
	slot.setStatus("running")
	roundValue := &activeRoomRound{
		SessionKey:     "room:group:provider-cancel",
		RoomID:         "room-provider",
		ConversationID: "provider-cancel",
		RoundID:        "round-provider",
		RootRoundID:    "round-provider",
		Slots: map[string]*activeRoomSlot{
			"worker": slot,
		},
	}
	service.rounds.register(roundValue)

	receipt, err := service.DeliverExecutionCancellation(
		context.Background(),
		orchestrationsvc.ExecutionCancellationDelivery{
			Binding: protocol.ExecutionCancellationBinding{
				ExecutionID:       binding.ExecutionID,
				PlanID:            binding.PlanID,
				WorkItemID:        binding.WorkItemID,
				SpecID:            binding.SpecID,
				AssignmentID:      binding.AssignmentID,
				AttemptID:         binding.AttemptID,
				RuntimeAttemptID:  binding.AttemptID,
				DispatchID:        binding.DispatchID,
				TargetKind:        protocol.ExecutionCancellationTargetRoomSlot,
				TargetAgentID:     slot.AgentID,
				ScopeSessionKey:   roundValue.SessionKey,
				RoomID:            roundValue.RoomID,
				ConversationID:    roundValue.ConversationID,
				RuntimeSessionKey: runtimeSessionKey,
				RuntimeRoundID:    agentRoundID,
				AgentRoundID:      agentRoundID,
			},
			Reason: "Room work superseded",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Outcome != protocol.ExecutionCancellationOutcomeProviderInterrupted {
		t.Fatalf("provider receipt = %+v", receipt)
	}
	client.mu.Lock()
	interruptCalls := client.interruptCalls
	client.mu.Unlock()
	if interruptCalls != 1 {
		t.Fatalf("provider interrupt calls = %d", interruptCalls)
	}
}

func TestExecutionCancellationInterruptsExactOldRoomBindingOnly(t *testing.T) {
	service := &Service{
		rounds:     newRoomRoundRegistry(),
		runtime:    runtimectx.NewManager(),
		permission: permissionctx.NewContext(),
	}
	oldBinding := &protocol.ExecutionWorkBinding{
		ExecutionID:  "execution-old",
		PlanID:       "plan-old",
		WorkItemID:   "work-old",
		SpecID:       "spec-old",
		AssignmentID: "assignment-old",
		AttemptID:    "attempt-old",
		DispatchID:   "dispatch-old",
	}
	successorBinding := &protocol.ExecutionWorkBinding{
		ExecutionID:  "execution-successor",
		PlanID:       "plan-successor",
		WorkItemID:   "work-successor",
		SpecID:       "spec-successor",
		AssignmentID: "assignment-successor",
		AttemptID:    "attempt-successor",
		DispatchID:   "dispatch-successor",
	}
	oldSlot := &activeRoomSlot{
		AgentID:           "agent-worker",
		AgentRoundID:      "agent-round-old",
		RuntimeSessionKey: "agent:agent-worker:ws:group:conversation-1",
		WorkBinding:       oldBinding,
	}
	successorSlot := &activeRoomSlot{
		AgentID:           "agent-worker",
		AgentRoundID:      "agent-round-successor",
		RuntimeSessionKey: oldSlot.RuntimeSessionKey,
		WorkBinding:       successorBinding,
	}
	oldSlot.setStatus("running")
	successorSlot.setStatus("running")
	oldSlot.doneChannel()
	successorSlot.doneChannel()
	oldCancelled := 0
	successorCancelled := 0
	oldCancel := func() {
		oldCancelled++
		oldSlot.setStatus("cancelled")
		oldSlot.closeDone()
		service.runtime.MarkRoundFinished(
			oldSlot.RuntimeSessionKey,
			oldSlot.AgentRoundID,
		)
	}
	successorCancel := func() {
		successorCancelled++
		successorSlot.setStatus("cancelled")
		successorSlot.closeDone()
		service.runtime.MarkRoundFinished(
			successorSlot.RuntimeSessionKey,
			successorSlot.AgentRoundID,
		)
	}
	oldSlot.setCancel(oldCancel)
	successorSlot.setCancel(successorCancel)
	if err := service.runtime.StartRound(
		context.Background(),
		oldSlot.RuntimeSessionKey,
		oldSlot.AgentRoundID,
		oldCancel,
	); err != nil {
		t.Fatalf("failed to register old runtime round: %v", err)
	}
	if err := service.runtime.StartRound(
		context.Background(),
		successorSlot.RuntimeSessionKey,
		successorSlot.AgentRoundID,
		successorCancel,
	); err != nil {
		t.Fatalf("failed to register successor runtime round: %v", err)
	}
	roundValue := &activeRoomRound{
		SessionKey:     "room:group:conversation-1",
		RoomID:         "room-1",
		ConversationID: "conversation-1",
		RoundID:        "round-active",
		RootRoundID:    "round-active",
		Slots: map[string]*activeRoomSlot{
			"old":       oldSlot,
			"successor": successorSlot,
		},
	}
	service.rounds.register(roundValue)

	receipt, err := service.DeliverExecutionCancellation(
		context.Background(),
		orchestrationsvc.ExecutionCancellationDelivery{
			Binding: protocol.ExecutionCancellationBinding{
				ExecutionID:       oldBinding.ExecutionID,
				PlanID:            oldBinding.PlanID,
				WorkItemID:        oldBinding.WorkItemID,
				SpecID:            oldBinding.SpecID,
				AssignmentID:      oldBinding.AssignmentID,
				AttemptID:         "attempt-child-or-root",
				RuntimeAttemptID:  oldBinding.AttemptID,
				DispatchID:        oldBinding.DispatchID,
				TargetKind:        protocol.ExecutionCancellationTargetRoomSlot,
				TargetAgentID:     oldSlot.AgentID,
				ScopeSessionKey:   roundValue.SessionKey,
				RoomID:            roundValue.RoomID,
				ConversationID:    roundValue.ConversationID,
				RuntimeSessionKey: oldSlot.RuntimeSessionKey,
				RuntimeRoundID:    oldSlot.AgentRoundID,
				AgentRoundID:      oldSlot.AgentRoundID,
			},
			Reason: "old Execution superseded",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Outcome != protocol.ExecutionCancellationOutcomeLocalRoundCancelled ||
		receipt.LimitationCode != "provider_interrupt_unsafe_shared_session" ||
		oldCancelled != 1 ||
		successorCancelled != 0 ||
		successorSlot.isTerminal() {
		t.Fatalf(
			"receipt=%+v oldCancelled=%d successorCancelled=%d successorStatus=%s",
			receipt,
			oldCancelled,
			successorCancelled,
			successorSlot.getStatus(),
		)
	}

	staleDelivery := orchestrationsvc.ExecutionCancellationDelivery{
		Binding: protocol.ExecutionCancellationBinding{
			ExecutionID:       oldBinding.ExecutionID,
			PlanID:            oldBinding.PlanID,
			WorkItemID:        oldBinding.WorkItemID,
			SpecID:            oldBinding.SpecID,
			AssignmentID:      oldBinding.AssignmentID,
			AttemptID:         "attempt-old",
			RuntimeAttemptID:  oldBinding.AttemptID,
			DispatchID:        oldBinding.DispatchID,
			TargetKind:        protocol.ExecutionCancellationTargetRoomSlot,
			TargetAgentID:     successorSlot.AgentID,
			ScopeSessionKey:   roundValue.SessionKey,
			RoomID:            roundValue.RoomID,
			ConversationID:    roundValue.ConversationID,
			RuntimeSessionKey: successorSlot.RuntimeSessionKey,
			RuntimeRoundID:    successorSlot.AgentRoundID,
			AgentRoundID:      successorSlot.AgentRoundID,
		},
	}
	receipt, err = service.DeliverExecutionCancellation(
		context.Background(),
		staleDelivery,
	)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Outcome != protocol.ExecutionCancellationOutcomeStaleTarget ||
		successorCancelled != 0 {
		t.Fatalf(
			"stale receipt=%+v successorCancelled=%d",
			receipt,
			successorCancelled,
		)
	}
	service.runtime.MarkRoundFinished(
		successorSlot.RuntimeSessionKey,
		successorSlot.AgentRoundID,
	)
}

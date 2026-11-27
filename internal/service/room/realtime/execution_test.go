// execution_test.go 覆盖 Room 执行派发、取消、评审派发、WorkBinding、权威校验与恢复测试。
package realtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

type managedExecutionAdmissionFake struct {
	actor   orchestrationsvc.ActorContext
	binding *protocol.ExecutionWorkBinding
	err     error
}

func (f *managedExecutionAdmissionFake) RuntimeContext(
	context.Context,
	orchestrationsvc.ActorContext,
) (string, error) {
	return "", nil
}

func (f *managedExecutionAdmissionFake) AuthorizeRoomRuntimeTarget(
	_ context.Context,
	actor orchestrationsvc.ActorContext,
	binding *protocol.ExecutionWorkBinding,
) error {
	f.actor = actor
	f.binding = binding.Clone()
	return f.err
}

func TestStaleStructuredWorkWakeIsTerminalNotRetriedAsConversation(t *testing.T) {
	binding := testRoomExecutionWorkBinding()
	parent := &activeRoomRound{
		SessionKey:     "room:group:conversation-1",
		RoomID:         "room-1",
		ConversationID: "conversation-1",
		RootRoundID:    "root-1",
		OwnerUserID:    "owner-1",
		Context:        &protocol.ConversationContextAggregate{},
	}
	wake := publicMentionWake{
		HandoffID:     "execution_dispatch_dispatch-1",
		TargetAgentID: "agent-worker",
		WorkBinding:   binding,
	}

	permanent := withConstructorDefaults(t, &Service{Host: runtimehost.Host{ExecutionContext: &managedExecutionAdmissionFake{
		err: &orchestrationsvc.DomainError{
			Code:    orchestrationsvc.ErrorCodeWorkBindingMismatch,
			Message: "the bound Attempt was superseded",
		},
	}},
	})
	if err := permanent.startPublicMentionRoundLocked(
		context.Background(),
		parent,
		[]publicMentionWake{wake},
	); err != nil {
		t.Fatalf("stale structured wake should be consumed as terminal: %v", err)
	}

	transientErr := errors.New("database temporarily unavailable")
	transient := withConstructorDefaults(t, &Service{Host: runtimehost.Host{ExecutionContext: &managedExecutionAdmissionFake{err: transientErr}}})
	if err := transient.startPublicMentionRoundLocked(
		context.Background(),
		parent,
		[]publicMentionWake{wake},
	); !errors.Is(err, transientErr) {
		t.Fatalf("transient admission error = %v, want retryable error", err)
	}
}

func TestExecutionDispatchHandoffIsDurableAndIdempotentBeforeSQLAck(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	store := workspacestore.NewRoomPublicHandoffStore(stateRoot)
	service := &Service{publicHandoffs: store}
	delivery := orchestrationsvc.ExecutionDispatchDelivery{
		OwnerUserID:    "owner-1",
		SessionKey:     "room:group:conversation-1",
		RoomID:         "room-1",
		ConversationID: "conversation-1",
		SourceAgentID:  "agent-lead",
		TargetAgentID:  "agent-worker",
		Kind:           protocol.ExecutionDispatchRoomDirected,
		Instruction:    "Deliver the evidence set.",
		Binding: protocol.ExecutionWorkBinding{
			ExecutionID:  "execution-1",
			PlanID:       "plan-1",
			WorkItemID:   "work-1",
			SpecID:       "spec-1",
			AssignmentID: "assignment-1",
			AttemptID:    "attempt-1",
			DispatchID:   "dispatch-1",
		},
	}
	handoffID := "execution_dispatch_dispatch-1"
	accepted, _, err := service.ensureExecutionDispatchHandoff(
		delivery,
		handoffID,
		renderExecutionDispatchInstruction(delivery, ""),
	)
	if err != nil {
		t.Fatal(err)
	}
	if accepted {
		t.Fatal("new durable handoff must still be claimed by the Room delivery path")
	}
	handoff, ok, err := store.Get("owner-1", "conversation-1", handoffID)
	if err != nil || !ok {
		t.Fatalf("durable handoff = %+v, ok=%t, err=%v", handoff, ok, err)
	}
	if handoff.Status != "source_finished" ||
		!executionWorkBindingEqual(handoff.WorkBinding, &delivery.Binding) {
		t.Fatalf("durable handoff = %+v", handoff)
	}
	if _, claimed, claimErr := store.Claim("owner-1", "conversation-1", handoffID); claimErr != nil || !claimed {
		t.Fatalf("claim = %t, err=%v", claimed, claimErr)
	}
	if err = store.MarkStarted("owner-1", "conversation-1", handoffID, "round-1"); err != nil {
		t.Fatal(err)
	}
	pending, err := store.Pending("owner-1", "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 ||
		!executionWorkBindingEqual(pending[0].WorkBinding, &delivery.Binding) {
		t.Fatalf("structured started handoff must remain recoverable: %+v", pending)
	}
	accepted, receipt, err := service.ensureExecutionDispatchHandoff(
		delivery,
		handoffID,
		renderExecutionDispatchInstruction(delivery, ""),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !accepted || receipt.HandoffID != handoffID {
		t.Fatalf("repeat acceptance = %t, receipt=%+v", accepted, receipt)
	}
}

func TestRoomRuntimeSharesDynamicSelfWorkBindingWithCommandAndGraphActor(t *testing.T) {
	service := &Service{}
	execution := &slotExecution{
		service: service,
		ctx:     context.Background(),
		round: &activeRoomRound{
			OwnerUserID: "owner-1", SessionKey: "room:group:conversation-1",
			RoomID: "room-1", ConversationID: "conversation-1",
			RootRoundID: "root-round-1", ExecutionID: "execution-1",
			CoordinatorAgentID: "agent-lead",
		},
		slot: &activeRoomSlot{
			AgentID: "agent-lead", AgentRoundID: "agent-round-1",
			RuntimeSessionKey: "runtime-session-1",
		},
		agent: &protocol.Agent{AgentID: "agent-lead", OwnerUserID: "owner-1"},
	}

	state := execution.runtimeCommandRoundContext("").CommandContext.WorkBindingState
	if state == nil || state != execution.ensureWorkBindingState() {
		t.Fatalf("Execution command WorkBindingState = %#v", state)
	}
	if actor := execution.orchestrationActor(); actor.WorkBinding != nil {
		t.Fatalf("ordinary Room coordinator actor = %+v", actor)
	}
	binding := &protocol.ExecutionWorkBinding{
		ExecutionID: "execution-1", PlanID: "plan-1", WorkItemID: "work-1",
		SpecID: "spec-1", AssignmentID: "assignment-1", AttemptID: "attempt-1",
	}
	if !state.Bind(binding) {
		t.Fatal("bind Room self WorkBinding")
	}
	actor := execution.orchestrationActor()
	if actor.WorkBinding == nil || actor.WorkBinding.AssignmentID != "assignment-1" {
		t.Fatalf("runtime graph actor did not observe WorkBinding: %+v", actor)
	}
}

type roomAttemptTerminalizerFake struct {
	calls  []orchestrationsvc.RoomAttemptTerminalInput
	actors []orchestrationsvc.ActorContext
	ctxErr error
	err    error
}

func (f *roomAttemptTerminalizerFake) RuntimeContext(
	context.Context,
	orchestrationsvc.ActorContext,
) (string, error) {
	return "", nil
}

func (f *roomAttemptTerminalizerFake) FinishRoomAttempt(
	ctx context.Context,
	actor orchestrationsvc.ActorContext,
	input orchestrationsvc.RoomAttemptTerminalInput,
) error {
	f.ctxErr = ctx.Err()
	f.actors = append(f.actors, actor)
	f.calls = append(f.calls, input)
	return f.err
}

func TestStructuredRoomSlotFailureAndCancellationSettleRootAttempt(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		slotStatus string
		reason     string
		wantStatus protocol.WorkAttemptStatus
	}{
		{
			name:       "error",
			slotStatus: "error",
			reason:     "runtime failed",
			wantStatus: protocol.WorkAttemptStatusFailed,
		},
		{
			name:       "interrupted",
			slotStatus: "interrupted",
			reason:     "user stopped",
			wantStatus: protocol.WorkAttemptStatusInterrupted,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			terminalizer := &roomAttemptTerminalizerFake{}
			service := &Service{Host: runtimehost.Host{ExecutionContext: terminalizer}}
			roundValue := &activeRoomRound{
				SessionKey:         "room:group:conversation-1",
				RoomID:             "room-1",
				ConversationID:     "conversation-1",
				CoordinatorAgentID: "agent-lead",
				RootRoundID:        "root-round-1",
				OwnerUserID:        "owner-1",
			}
			slot := &activeRoomSlot{
				RoomSessionID:     "room-session-1",
				AgentID:           "agent-member",
				AgentRoundID:      "agent-round-1",
				RuntimeSessionKey: "runtime-session-1",
				WorkBinding:       testRoomExecutionWorkBinding(),
			}
			cancelled, cancel := context.WithCancel(context.Background())
			cancel()
			if err := service.finishBoundRoomAttempt(
				cancelled,
				roundValue,
				slot,
				testCase.slotStatus,
				testCase.reason,
			); err != nil {
				t.Fatal(err)
			}
			assertRoomAttemptTerminalCall(
				t,
				terminalizer,
				testCase.wantStatus,
				testCase.reason,
			)
			if terminalizer.ctxErr != nil {
				t.Fatalf("terminal settlement inherited cancelled slot context: %v", terminalizer.ctxErr)
			}
		})
	}
}

func testRoomExecutionWorkBinding() *protocol.ExecutionWorkBinding {
	return &protocol.ExecutionWorkBinding{
		ExecutionID:  "execution-1",
		PlanID:       "plan-1",
		WorkItemID:   "work-1",
		SpecID:       "spec-1",
		AssignmentID: "assignment-1",
		AttemptID:    "attempt-1",
		DispatchID:   "dispatch-1",
	}
}

func assertRoomAttemptTerminalCall(
	t *testing.T,
	terminalizer *roomAttemptTerminalizerFake,
	wantStatus protocol.WorkAttemptStatus,
	wantReason string,
) {
	t.Helper()
	if len(terminalizer.calls) != 1 || len(terminalizer.actors) != 1 {
		t.Fatalf("terminal calls = %#v actors = %#v", terminalizer.calls, terminalizer.actors)
	}
	call := terminalizer.calls[0]
	actor := terminalizer.actors[0]
	if call.Status != wantStatus ||
		call.FailureReason != wantReason ||
		call.Binding.AssignmentID != "assignment-1" ||
		call.RuntimeSessionKey != "runtime-session-1" ||
		actor.WorkBinding == nil ||
		actor.WorkBinding.AssignmentID != "assignment-1" {
		t.Fatalf("terminal call = %#v actor = %#v", call, actor)
	}
}

type deletedConversationStore struct{}

func (deletedConversationStore) GetConversationContext(context.Context, string) (*protocol.ConversationContextAggregate, error) {
	return nil, roomsvc.ErrConversationNotFound
}

func (deletedConversationStore) GetConversationContextForSystem(context.Context, string) (*protocol.ConversationContextAggregate, error) {
	return nil, roomsvc.ErrConversationNotFound
}

func (deletedConversationStore) UpdateSessionRuntimeIdentity(context.Context, string, string, string) error {
	return nil
}

func (deletedConversationStore) TouchConversationActivity(context.Context, string, time.Time) error {
	return nil
}

func (deletedConversationStore) MarkConversationStarted(context.Context, string, time.Time) error {
	return nil
}

func (deletedConversationStore) BuildRoomSkillPrompt(context.Context, []string) (string, error) {
	return "", nil
}

// A Room deleted after its WorkGraph stalled must be skipped by startup
// recovery, not reported as a host failure that blocks every later graph.
func TestQueueExecutionRecoveryReportsDeletedConversationAsUnavailable(t *testing.T) {
	service := &Service{rooms: deletedConversationStore{}}

	err := service.QueueExecutionRecovery(context.Background(), protocol.Execution{
		ID: "execution-deleted-room", OwnerUserID: "owner", ScopeKind: protocol.ExecutionScopeRoom,
		RoomID: "room-gone", ConversationID: "conversation-gone", CoordinatorAgentID: "lead",
	}, "startup:execution-deleted-room:1")

	if !errors.Is(err, orchestrationsvc.ErrExecutionRecoveryTargetUnavailable) {
		t.Fatalf("err = %v, want ErrExecutionRecoveryTargetUnavailable", err)
	}
}

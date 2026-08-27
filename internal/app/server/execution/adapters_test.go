// INPUT: Host-fixed Room identity plus an optional current WorkGraph observation.
// OUTPUT: Coordinator/member command role derived without requiring a mutation binding.
// POS: Room runtime command admission regression tests for fresh, recovery, and member rounds.
package execution

import (
	"context"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type snapshotReaderStub struct {
	snapshot *protocol.ExecutionSnapshot
	err      error
}

type cancellationRuntimeStub struct {
	result runtimectx.ExactRoundInterruptResult
	err    error
}

func (s cancellationRuntimeStub) InterruptRound(
	context.Context,
	string,
	string,
	string,
) (runtimectx.ExactRoundInterruptResult, error) {
	return s.result, s.err
}

func (r snapshotReaderStub) ReadCurrent(
	context.Context,
	orchestrationsvc.ActorContext,
) (*protocol.ExecutionSnapshot, error) {
	return r.snapshot, r.err
}

func (r snapshotReaderStub) ReadSnapshot(
	context.Context,
	orchestrationsvc.ActorContext,
	string,
) (*protocol.ExecutionSnapshot, error) {
	return r.snapshot, r.err
}

func TestCommandContextUsesObservedRoomRole(t *testing.T) {
	for _, testCase := range []struct {
		name            string
		agentID         string
		hostCoordinator string
		snapshot        *protocol.ExecutionSnapshot
		readErr         error
		wantRole        orchestrationsvc.ExecutionActorRole
	}{
		{
			name:    "fresh Room host may create the first WorkGraph",
			agentID: "agent-lead", hostCoordinator: "agent-lead",
			wantRole: orchestrationsvc.ExecutionActorCoordinator,
		},
		{
			name:    "current WorkGraph coordinator may recover coordination",
			agentID: "agent-lead", hostCoordinator: "agent-lead",
			snapshot: roomCommandSnapshot("agent-lead"),
			wantRole: orchestrationsvc.ExecutionActorCoordinator,
		},
		{
			name:    "verified Room member remains observation-only",
			agentID: "agent-member", hostCoordinator: "agent-lead",
			snapshot: roomCommandSnapshot("agent-lead"),
			wantRole: orchestrationsvc.ExecutionActorMember,
		},
		{
			name:    "read failure cannot elect the host",
			agentID: "agent-lead", hostCoordinator: "agent-lead",
			readErr:  errors.New("Room observation unavailable"),
			wantRole: orchestrationsvc.ExecutionActorMember,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			runtimeContext := runtimectx.RuntimeCommandContext{
				Agent: &protocol.Agent{
					AgentID:     testCase.agentID,
					OwnerUserID: "owner-1",
				},
				ScopeSessionKey:    "room:group:conversation-1",
				RuntimeSessionKey:  "agent:" + testCase.agentID + ":room:conversation-1",
				RootRoundID:        "round-root",
				AgentRoundID:       "round-agent",
				SourceContextType:  "room",
				RoomID:             "room-1",
				ConversationID:     "conversation-1",
				CoordinatorAgentID: testCase.hostCoordinator,
			}
			resolved, ok := ResolveCommandContext(
				context.Background(),
				snapshotReaderStub{snapshot: testCase.snapshot, err: testCase.readErr},
				runtimeContext,
			)
			if !ok || resolved.Role != testCase.wantRole {
				t.Fatalf("resolved=%+v ok=%t, want role %q", resolved, ok, testCase.wantRole)
			}
		})
	}
}

func roomCommandSnapshot(coordinatorAgentID string) *protocol.ExecutionSnapshot {
	return &protocol.ExecutionSnapshot{Execution: protocol.Execution{
		ID:                 "execution-1",
		OwnerUserID:        "owner-1",
		SessionKey:         "room:group:conversation-1",
		ScopeKind:          protocol.ExecutionScopeRoom,
		RoomID:             "room-1",
		ConversationID:     "conversation-1",
		CoordinatorAgentID: coordinatorAgentID,
	}}
}

func TestCancellationPreservesRuntimeOutcome(t *testing.T) {
	tests := []struct {
		name           string
		runtimeResult  runtimectx.ExactRoundInterruptResult
		wantOutcome    protocol.ExecutionCancellationOutcome
		wantLimitation string
	}{
		{
			name: "provider interrupted",
			runtimeResult: runtimectx.ExactRoundInterruptResult{
				Outcome: runtimectx.ExactRoundProviderInterrupted,
				Detail:  "provider accepted interrupt",
			},
			wantOutcome: protocol.ExecutionCancellationOutcomeProviderInterrupted,
		},
		{
			name: "local round only",
			runtimeResult: runtimectx.ExactRoundInterruptResult{
				Outcome:        runtimectx.ExactRoundLocalCancelled,
				LimitationCode: "provider_interrupt_unsafe_shared_session",
				Detail:         "local round cancelled",
			},
			wantOutcome:    protocol.ExecutionCancellationOutcomeLocalRoundCancelled,
			wantLimitation: "provider_interrupt_unsafe_shared_session",
		},
		{
			name: "unsupported",
			runtimeResult: runtimectx.ExactRoundInterruptResult{
				Outcome:        runtimectx.ExactRoundInterruptUnsupported,
				LimitationCode: "exact_local_cancel_unavailable",
				Detail:         "no safe exact cancellation",
			},
			wantOutcome:    protocol.ExecutionCancellationOutcomeUnsupported,
			wantLimitation: "exact_local_cancel_unavailable",
		},
		{
			name: "already ended",
			runtimeResult: runtimectx.ExactRoundInterruptResult{
				Outcome: runtimectx.ExactRoundAlreadyEnded,
			},
			wantOutcome: protocol.ExecutionCancellationOutcomeAlreadyEnded,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			consumer := cancellationConsumer{
				runtime: cancellationRuntimeStub{result: test.runtimeResult},
			}
			receipt, err := consumer.DeliverExecutionCancellation(
				context.Background(),
				orchestrationsvc.ExecutionCancellationDelivery{
					Binding: protocol.ExecutionCancellationBinding{
						TargetKind:        protocol.ExecutionCancellationTargetRuntimeRound,
						RuntimeSessionKey: "agent:worker:ws:dm:runtime",
						RuntimeRoundID:    "round-old",
					},
					Reason: "old execution superseded",
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			if receipt.Outcome != test.wantOutcome || receipt.LimitationCode != test.wantLimitation {
				t.Fatalf("receipt = %+v", receipt)
			}
		})
	}
}

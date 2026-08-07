// INPUT: runtime exact-round provider/local/unsupported results.
// OUTPUT: durable cancellation receipt preserves the physical outcome and limitation.
// POS: app composition adapter contract test.
package server

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type executionCancellationRuntimeStub struct {
	result runtimectx.ExactRoundInterruptResult
	err    error
}

func (s executionCancellationRuntimeStub) InterruptRound(
	context.Context,
	string,
	string,
	string,
) (runtimectx.ExactRoundInterruptResult, error) {
	return s.result, s.err
}

func TestExecutionCancellationConsumerPreservesRuntimeOutcome(t *testing.T) {
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
			consumer := executionCancellationConsumer{
				runtime: executionCancellationRuntimeStub{
					result: test.runtimeResult,
				},
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
			if receipt.Outcome != test.wantOutcome ||
				receipt.LimitationCode != test.wantLimitation {
				t.Fatalf("receipt = %+v", receipt)
			}
		})
	}
}

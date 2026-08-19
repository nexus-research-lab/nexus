// INPUT: Orchestration cancellation delivery、Room exact slot consumer 与 runtime Manager。
// OUTPUT: 按 typed target 路由的 provider/local cancellation receipt。
// POS: application composition root；service/orchestration 不依赖 Room 或 runtime 实现。
package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type executionCancellationRoomConsumer interface {
	DeliverExecutionCancellation(
		context.Context,
		orchestrationsvc.ExecutionCancellationDelivery,
	) (orchestrationsvc.ExecutionCancellationReceipt, error)
}

type executionCancellationRuntime interface {
	InterruptRound(
		context.Context,
		string,
		string,
		string,
	) (runtimectx.ExactRoundInterruptResult, error)
}

type executionCancellationConsumer struct {
	room    executionCancellationRoomConsumer
	runtime executionCancellationRuntime
}

func (c executionCancellationConsumer) DeliverExecutionCancellation(
	ctx context.Context,
	delivery orchestrationsvc.ExecutionCancellationDelivery,
) (orchestrationsvc.ExecutionCancellationReceipt, error) {
	switch delivery.Binding.TargetKind {
	case protocol.ExecutionCancellationTargetRoomSlot:
		if c.room == nil {
			return orchestrationsvc.ExecutionCancellationReceipt{},
				fmt.Errorf("Room cancellation consumer is unavailable")
		}
		return c.room.DeliverExecutionCancellation(ctx, delivery)
	case protocol.ExecutionCancellationTargetRuntimeRound:
		if c.runtime == nil {
			return orchestrationsvc.ExecutionCancellationReceipt{},
				fmt.Errorf("runtime cancellation consumer is unavailable")
		}
		result, err := c.runtime.InterruptRound(
			ctx,
			strings.TrimSpace(delivery.Binding.RuntimeSessionKey),
			strings.TrimSpace(delivery.Binding.RuntimeRoundID),
			strings.TrimSpace(delivery.Reason),
		)
		if err != nil {
			return orchestrationsvc.ExecutionCancellationReceipt{}, err
		}
		detail := strings.TrimSpace(result.Detail)
		limitationCode := strings.TrimSpace(result.LimitationCode)
		switch result.Outcome {
		case runtimectx.ExactRoundAlreadyEnded:
			return orchestrationsvc.ExecutionCancellationReceipt{
				Outcome: protocol.ExecutionCancellationOutcomeAlreadyEnded,
				Detail: firstNonEmptyString(
					result.Detail,
					"exact runtime session/round is no longer active",
				),
			}, nil
		case runtimectx.ExactRoundProviderInterrupted:
			return orchestrationsvc.ExecutionCancellationReceipt{
				Outcome: protocol.ExecutionCancellationOutcomeProviderInterrupted,
				Detail:  detail,
			}, nil
		case runtimectx.ExactRoundLocalCancelled:
			return orchestrationsvc.ExecutionCancellationReceipt{
				Outcome:        protocol.ExecutionCancellationOutcomeLocalRoundCancelled,
				LimitationCode: limitationCode,
				Detail:         detail,
			}, nil
		case runtimectx.ExactRoundInterruptUnsupported:
			return orchestrationsvc.ExecutionCancellationReceipt{
				Outcome:        protocol.ExecutionCancellationOutcomeUnsupported,
				LimitationCode: limitationCode,
				Detail:         detail,
			}, nil
		default:
			return orchestrationsvc.ExecutionCancellationReceipt{},
				fmt.Errorf(
					"runtime returned unknown exact interrupt outcome %q",
					result.Outcome,
				)
		}
	default:
		return orchestrationsvc.ExecutionCancellationReceipt{},
			fmt.Errorf(
				"unsupported physical cancellation target %q",
				delivery.Binding.TargetKind,
			)
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

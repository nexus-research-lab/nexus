// INPUT: trusted ExecutionCancellationBinding 与当前 Room round/slot registry。
// OUTPUT: 对 exact WorkBinding slot 的 provider/local cancellation，或 stale/already-ended 幂等 receipt。
// POS: Execution cancellation outbox 的 Room 数据面；绝不按 Agent 名或聊天文本猜目标。
package realtime

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

// DeliverExecutionCancellation 精确中断旧 WorkBinding 对应的 Room slot。
func (s *Service) DeliverExecutionCancellation(
	ctx context.Context,
	delivery orchestrationsvc.ExecutionCancellationDelivery,
) (orchestrationsvc.ExecutionCancellationReceipt, error) {
	binding := delivery.Binding
	if binding.TargetKind != protocol.ExecutionCancellationTargetRoomSlot {
		return orchestrationsvc.ExecutionCancellationReceipt{
			Outcome: protocol.ExecutionCancellationOutcomeStaleTarget,
			Detail:  "Room consumer received a non-Room cancellation target",
		}, nil
	}
	roundValue, slot := s.findActiveSlotByAgentRoundID(
		strings.TrimSpace(binding.ScopeSessionKey),
		strings.TrimSpace(binding.AgentRoundID),
	)
	if roundValue == nil || slot == nil {
		return orchestrationsvc.ExecutionCancellationReceipt{
			Outcome: protocol.ExecutionCancellationOutcomeAlreadyEnded,
			Detail:  "exact Room agent_round is no longer active",
		}, nil
	}
	if strings.TrimSpace(roundValue.RoomID) != strings.TrimSpace(binding.RoomID) ||
		strings.TrimSpace(roundValue.ConversationID) !=
			strings.TrimSpace(binding.ConversationID) ||
		strings.TrimSpace(slot.AgentID) != strings.TrimSpace(binding.TargetAgentID) {
		return orchestrationsvc.ExecutionCancellationReceipt{
			Outcome: protocol.ExecutionCancellationOutcomeStaleTarget,
			Detail:  "active Room slot scope differs from captured cancellation target",
		}, nil
	}
	expected := &protocol.ExecutionWorkBinding{
		ExecutionID:  binding.ExecutionID,
		PlanID:       binding.PlanID,
		WorkItemID:   binding.WorkItemID,
		SpecID:       binding.SpecID,
		AssignmentID: binding.AssignmentID,
		AttemptID:    binding.RuntimeAttemptID,
		DispatchID:   binding.DispatchID,
	}
	if !executionWorkBindingEqual(slot.currentWorkBinding(), expected) ||
		strings.TrimSpace(slot.RuntimeSessionKey) !=
			strings.TrimSpace(binding.RuntimeSessionKey) {
		return orchestrationsvc.ExecutionCancellationReceipt{
			Outcome: protocol.ExecutionCancellationOutcomeStaleTarget,
			Detail:  "active Room slot WorkBinding differs from captured cancellation target",
		}, nil
	}
	if slot.isTerminal() {
		return orchestrationsvc.ExecutionCancellationReceipt{
			Outcome: protocol.ExecutionCancellationOutcomeAlreadyEnded,
			Detail:  "exact Room slot is already terminal",
		}, nil
	}
	if s.runtime == nil {
		return orchestrationsvc.ExecutionCancellationReceipt{},
			errors.New("Room runtime manager is unavailable")
	}
	interruptReason := normalizeRoomInterruptReason(delivery.Reason)
	markRoomSlotInterrupted(slot, interruptReason)
	s.permission.CancelRequestsForSession(slot.RuntimeSessionKey, interruptReason)
	result, err := s.runtime.InterruptRound(
		ctx,
		strings.TrimSpace(slot.RuntimeSessionKey),
		strings.TrimSpace(slot.AgentRoundID),
		interruptReason,
	)
	if err != nil {
		return orchestrationsvc.ExecutionCancellationReceipt{}, err
	}
	s.broadcastSessionStatus(ctx, roundValue.SessionKey)
	detail := strings.TrimSpace(result.Detail)
	limitationCode := strings.TrimSpace(result.LimitationCode)
	switch result.Outcome {
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
	case runtimectx.ExactRoundAlreadyEnded:
		return orchestrationsvc.ExecutionCancellationReceipt{
			Outcome: protocol.ExecutionCancellationOutcomeAlreadyEnded,
			Detail:  "exact Room runtime round is no longer managed",
		}, nil
	case runtimectx.ExactRoundInterruptUnsupported:
		return orchestrationsvc.ExecutionCancellationReceipt{
			Outcome:        protocol.ExecutionCancellationOutcomeUnsupported,
			LimitationCode: limitationCode,
			Detail:         detail,
		}, nil
	default:
		return orchestrationsvc.ExecutionCancellationReceipt{},
			errors.New("Room runtime returned an unknown exact interrupt outcome")
	}
}

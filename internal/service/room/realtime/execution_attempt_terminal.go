// INPUT: structured Room slot 的 WorkBinding、可见 slot 终态与物理 runtime identity。
// OUTPUT: orchestration root Attempt 的 succeeded/failed/interrupted 原子终态。
// POS: Room slot 生命周期到 Execution Attempt 的可信终态适配；不创建 Submission/Acceptance。
package realtime

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func (s *Service) finishBoundRoomAttempt(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	slotStatus string,
	reason string,
) error {
	if slot == nil {
		return nil
	}
	binding := slot.currentWorkBinding()
	if binding == nil {
		return nil
	}
	if s == nil || s.executionContext == nil {
		return errors.New("managed Execution Attempt terminalizer is unavailable")
	}
	terminalizer, ok := s.executionContext.(executionAttemptTerminalizer)
	if !ok {
		return errors.New("managed Execution Attempt terminalizer is unavailable")
	}
	if roundValue == nil {
		return errors.New("Room round is required for managed Attempt settlement")
	}
	reason = strings.TrimSpace(reason)
	attemptStatus := protocol.WorkAttemptStatusFailed
	switch strings.TrimSpace(slotStatus) {
	case "finished":
		attemptStatus = protocol.WorkAttemptStatusSucceeded
		reason = ""
	case "cancelled", "interrupted":
		attemptStatus = protocol.WorkAttemptStatusInterrupted
		if reason == "" {
			reason = strings.TrimSpace(roomSlotInterruptReason(slot))
		}
		if reason == "" {
			reason = "Room slot interrupted"
		}
	default:
		if reason == "" {
			reason = "Room slot failed"
		}
	}
	actor := orchestrationsvc.ActorContext{
		OwnerUserID:    roundValue.OwnerUserID,
		SessionKey:     roundValue.SessionKey,
		ExecutionID:    binding.ExecutionID,
		WorkBinding:    cloneExecutionWorkBinding(binding),
		AgentID:        slot.AgentID,
		Role:           roomExecutionActorRole(roundValue.CoordinatorAgentID, slot.AgentID),
		ActorKind:      protocol.ExecutionActorAgent,
		ScopeKind:      protocol.ExecutionScopeRoom,
		RoomID:         roundValue.RoomID,
		ConversationID: roundValue.ConversationID,
		RootRoundID:    roundValue.RootRoundID,
		RuntimeRoundID: slot.AgentRoundID,
		AgentRoundID:   slot.AgentRoundID,
	}
	if ctx == nil {
		ctx = context.Background()
	} else {
		ctx = context.WithoutCancel(ctx)
	}
	return terminalizer.FinishRoomAttempt(
		ctx,
		actor,
		orchestrationsvc.RoomAttemptTerminalInput{
			Binding:           *binding,
			Status:            attemptStatus,
			FailureReason:     reason,
			RuntimeSessionKey: slot.RuntimeSessionKey,
			RoomSessionID:     slot.RoomSessionID,
			SDKSessionID:      slot.getSDKSessionID(),
		},
	)
}

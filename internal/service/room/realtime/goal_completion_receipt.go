// INPUT: Room slot 的 complete update_goal 观察、最终 assistant 与 Goal 聚合报告。
// OUTPUT: 在公区/私有历史原回复上精确合并、只展示已知结算项的 Goal 完成收据。
// POS: Room Goal 终态结算到消息历史与实时事件的宿主收口层。
package realtime

import (
	"context"
	"strings"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
)

func (s *Service) persistRoomGoalCompletionReceipts(
	ctx context.Context,
	roundValue *activeRoomRound,
	refresh bool,
) {
	if roundValue == nil {
		return
	}
	for _, slot := range roundValue.Slots {
		s.persistRoomGoalCompletionReceipt(ctx, roundValue, slot, refresh)
	}
}

func (s *Service) persistRoomGoalCompletionReceipt(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	refresh bool,
) {
	if strings.TrimSpace(slot.WorkspacePath) == "" || strings.TrimSpace(slot.RuntimeSessionKey) == "" {
		return
	}
	goalID, message, receipt, ok := slot.mutable.goal.PrepareGoalCompletionReceipt(ctx, s.goals, s.LoggerFor(ctx), slot.AgentRoundID, refresh)
	if !ok {
		return
	}
	if err := s.ensureSlotOutputAuthorized(ctx, roundValue, slot); err != nil {
		return
	}
	if roomSlotPublishesPublicOutput(slot) {
		if err := s.persistSharedInlineMessage(roundValue.OwnerUserID, roundValue.ConversationID, message); err != nil {
			s.logRoomGoalCompletionReceiptError(ctx, roundValue, slot, goalID, err)
			return
		}
	}
	if err := s.persistPrivateOverlayMessage(slot, message); err != nil {
		s.logRoomGoalCompletionReceiptError(ctx, roundValue, slot, goalID, err)
		return
	}
	slot.mutable.goal.MarkGoalCompletionReceiptStored(goalID, receipt)
	if roomSlotPublishesPublicOutput(slot) {
		event := roomdomain.WrapMessageEvent(
			roundValue.RoomID,
			roundValue.ConversationID,
			message,
			roundValue.RootRoundID,
		)
		s.broadcastSharedEventWithTimeout(ctx, roundValue.SessionKey, roundValue.RoomID, event)
	}
}

func (s *Service) logRoomGoalCompletionReceiptError(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	goalID string,
	err error,
) {
	s.LoggerFor(ctx).Warn(
		"Room Goal 完成收据持久化失败",
		"session_key", roundValue.SessionKey,
		"goal_id", goalID,
		"round_id", slot.AgentRoundID,
		"err", err,
	)
}

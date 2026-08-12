// INPUT: Room slot 的 complete update_goal 观察、最终 assistant 与 Goal 聚合报告。
// OUTPUT: 在公区/私有历史原回复上精确合并、只展示已知结算项的 Goal 完成收据。
// POS: Room Goal 终态结算到消息历史与实时事件的宿主收口层。
package realtime

import (
	"context"
	"errors"
	"strings"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	messageutil "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

func (s *Service) persistRoomGoalCompletionReceipts(
	ctx context.Context,
	roundValue *activeRoomRound,
	refresh bool,
) {
	if s == nil || roundValue == nil {
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
	if s == nil || roundValue == nil || slot == nil {
		return
	}
	goalID, assistant, previous, stored := slot.goalCompletionReceiptSnapshot()
	if goalID == "" || len(assistant) == 0 ||
		strings.TrimSpace(slot.WorkspacePath) == "" ||
		strings.TrimSpace(slot.RuntimeSessionKey) == "" ||
		(stored && !refresh) {
		return
	}
	report, reportOK := s.roomGoalCompletionReport(ctx, goalID)
	if !reportOK && stored {
		return
	}
	receipt := messageutil.BuildGoalCompletionReceipt(goalID, slot.AgentRoundID, report)
	if stored && previous.Equal(receipt) {
		return
	}
	message, ok := messageutil.AttachGoalCompletionReceipt(assistant, receipt)
	if !ok {
		return
	}
	if err := s.ensureSlotOutputAuthorized(ctx, roundValue, slot); err != nil {
		return
	}
	if roomSlotPublishesPublicOutput(slot) {
		if s.roomHistory == nil {
			return
		}
		if err := s.persistSharedInlineMessage(roundValue.OwnerUserID, roundValue.ConversationID, message); err != nil {
			s.logRoomGoalCompletionReceiptError(ctx, roundValue, slot, goalID, err)
			return
		}
	}
	if err := s.persistPrivateOverlayMessage(slot, message); err != nil {
		s.logRoomGoalCompletionReceiptError(ctx, roundValue, slot, goalID, err)
		return
	}
	slot.markGoalCompletionReceiptStored(goalID, receipt)
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

func (s *Service) roomGoalCompletionReport(
	ctx context.Context,
	goalID string,
) (*protocol.GoalUsageReport, bool) {
	provider, ok := s.goals.(roomGoalUsageFinalizationProvider)
	if !ok {
		return nil, false
	}
	report, err := provider.UsageByGoalID(ctx, goalID)
	if err != nil {
		if !errors.Is(err, goalsvc.ErrGoalNotFound) {
			s.loggerFor(ctx).Debug("读取 Room Goal 完成收据数据失败", "goal_id", goalID, "err", err)
		}
		return nil, false
	}
	if report == nil ||
		protocol.NormalizeGoalStatus(report.Status) != protocol.GoalStatusComplete ||
		(strings.TrimSpace(report.GoalID) != "" && strings.TrimSpace(report.GoalID) != goalID) {
		return nil, false
	}
	return report, true
}

func (s *Service) logRoomGoalCompletionReceiptError(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	goalID string,
	err error,
) {
	s.loggerFor(ctx).Warn(
		"Room Goal 完成收据持久化失败",
		"session_key", roundValue.SessionKey,
		"goal_id", goalID,
		"round_id", slot.AgentRoundID,
		"err", err,
	)
}

// INPUT: 活跃 Room round/slot 捕获的身份与数据库中的实时成员关系。
// OUTPUT: 允许输出继续落库，或在成员撤销后拒绝旧 runtime 的在途结果。
// POS: Room runtime 的最终权限栅栏；中断负责及时性，本函数负责持久化安全。
package realtime

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

var (
	errRoomSlotAuthorityRevoked   = errors.New("Room slot authority has been revoked")
	errRoomSlotAuthorityUncertain = errors.New("Room slot authority could not be verified")
)

func roomRoundAuthorityEpoch(roundValue *activeRoomRound) int64 {
	if roundValue == nil {
		return 0
	}
	if roundValue.AuthorityEpoch > 0 {
		return roundValue.AuthorityEpoch
	}
	if roundValue.Context == nil {
		return 0
	}
	return roundValue.Context.Room.AuthorityEpoch
}

func (s *Service) ensureSlotOutputAuthorized(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
) error {
	if s == nil || s.rooms == nil || roundValue == nil || slot == nil {
		return errRoomSlotAuthorityRevoked
	}
	// Interrupt 会先取消 slotCtx；权限复核仍必须带着原 context values
	// 读取数据库真相源，不能把正常用户中断误判成授权不确定。
	lookupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), roomBroadcastTimeout)
	defer cancel()
	current, err := s.rooms.GetConversationContext(lookupCtx, roundValue.ConversationID)
	if err != nil {
		return fmt.Errorf("%w: %v", errRoomSlotAuthorityUncertain, err)
	}
	if current == nil || strings.TrimSpace(current.Room.ID) != strings.TrimSpace(roundValue.RoomID) {
		return errRoomSlotAuthorityRevoked
	}
	capturedEpoch := roomRoundAuthorityEpoch(roundValue)
	if capturedEpoch > 0 && current.Room.AuthorityEpoch != capturedEpoch {
		return fmt.Errorf(
			"%w: authority epoch changed from %d to %d",
			errRoomSlotAuthorityRevoked,
			capturedEpoch,
			current.Room.AuthorityEpoch,
		)
	}
	for _, member := range current.Members {
		if member.MemberType == protocol.MemberTypeAgent &&
			strings.TrimSpace(member.MemberAgentID) == strings.TrimSpace(slot.AgentID) {
			if member.ParticipationPaused {
				return fmt.Errorf("%w: member participation is paused", errRoomSlotAuthorityRevoked)
			}
			if roomSlotReplyRoute(slot).Mode == protocol.RoomReplyRoutePrivate &&
				!current.Room.PrivateMessagesEnabled {
				return fmt.Errorf("%w: private messaging is disabled", errRoomSlotAuthorityRevoked)
			}
			return nil
		}
	}
	return errRoomSlotAuthorityRevoked
}

func isRoomSlotOutputAdmissionError(err error) bool {
	return errors.Is(err, errRoomSlotAuthorityRevoked) ||
		errors.Is(err, errRoomSlotAuthorityUncertain)
}

// retireSlotAfterOutputRevocation 只更新内存终态，不产生 Agent 消息、Goal
// 变更或共享事件。root round 仍可按 interrupted 正常收口。
func (s *Service) retireSlotAfterOutputRevocation(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	err error,
) bool {
	if !isRoomSlotOutputAdmissionError(err) {
		return false
	}
	if slot != nil {
		slot.suppressOutput()
		slot.setErrorMessage("")
		slot.setStatus("cancelled")
	}
	s.loggerFor(ctx).Warn(
		"Room slot 输出因权限世代变化被静默丢弃",
		"room_id", roomIDForAuthorityFence(roundValue),
		"conversation_id", conversationIDForAuthorityFence(roundValue),
		"agent_id", slotAgentID(slot),
		"captured_authority_epoch", roomRoundAuthorityEpoch(roundValue),
		"err", err,
	)
	return true
}

func roomIDForAuthorityFence(roundValue *activeRoomRound) string {
	if roundValue == nil {
		return ""
	}
	return strings.TrimSpace(roundValue.RoomID)
}

func conversationIDForAuthorityFence(roundValue *activeRoomRound) string {
	if roundValue == nil {
		return ""
	}
	return strings.TrimSpace(roundValue.ConversationID)
}

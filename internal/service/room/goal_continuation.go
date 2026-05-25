package room

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// ShouldDeferGoalContinuation 先让 Room 里用户显式排队输入获得执行机会。
func (s *RealtimeService) ShouldDeferGoalContinuation(ctx context.Context, sessionKey string) bool {
	sessionKey = strings.TrimSpace(sessionKey)
	if s == nil || sessionKey == "" {
		return false
	}
	if s.runtime != nil && len(s.runtime.GetRunningRoundIDs(sessionKey)) > 0 {
		return true
	}
	parsed := protocol.ParseSessionKey(sessionKey)
	if parsed.Kind != protocol.SessionKeyKindRoom || strings.TrimSpace(parsed.ConversationID) == "" {
		return false
	}
	if s.rooms == nil {
		return false
	}
	contextValue, err := s.rooms.GetConversationContext(ctx, parsed.ConversationID)
	if err != nil || contextValue == nil {
		if err != nil {
			s.loggerFor(ctx).Warn("解析 Room Goal 续跑待发送队列上下文失败", "session_key", sessionKey, "err", err)
		}
		return false
	}
	entries, err := s.roomInputQueueEntries(ctx, contextValue)
	if err != nil {
		s.loggerFor(ctx).Warn("读取 Room Goal 续跑待发送队列失败", "session_key", sessionKey, "err", err)
		return false
	}
	entry, ok := s.findDispatchableInputQueueEntry(sessionKey, parsed.ConversationID, entries)
	if !ok {
		return false
	}
	s.dispatchNextInputQueueItem(
		contextWithQueueOwner(ctx, entry.Item.OwnerUserID),
		sessionKey,
		contextValue.Room.ID,
		contextValue.Conversation.ID,
	)
	return true
}

// DispatchGoalContinuation 把共享 Room Goal 的隐藏续跑交给 Room 运行链路。
func (s *RealtimeService) DispatchGoalContinuation(ctx context.Context, plan protocol.GoalContinuation) error {
	if s == nil {
		return errors.New("room goal continuation dispatcher is not configured")
	}
	sessionKey := strings.TrimSpace(plan.Goal.SessionKey)
	parsed := protocol.ParseSessionKey(sessionKey)
	if parsed.Kind != protocol.SessionKeyKindRoom || strings.TrimSpace(parsed.ConversationID) == "" {
		return errors.New("room goal continuation requires a room session key")
	}
	return s.HandleChat(ctx, ChatRequest{
		SessionKey:     sessionKey,
		ConversationID: parsed.ConversationID,
		Content:        plan.Prompt,
		RoundID:        plan.RoundID,
		ReqID:          plan.RoundID,
		DeliveryPolicy: protocol.ChatDeliveryPolicyQueue,
		Internal:       true,
		InputOptions:   goalContinuationInputOptions(plan),
	})
}

func goalContinuationInputOptions(plan protocol.GoalContinuation) sdkprotocol.OutboundMessageOptions {
	return sdkprotocol.OutboundMessageOptions{
		Synthetic:      plan.Synthetic,
		HiddenFromUser: plan.HiddenFromUser,
		Purpose:        plan.Purpose,
		Priority:       "internal",
		Metadata:       plan.Metadata,
	}
}

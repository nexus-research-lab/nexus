package room

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// ShouldDeferGoalContinuation 避免隐藏 Goal 续跑抢占显式输入，并按 Codex 语义跳过 Plan 模式续跑。
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
		return s.shouldDeferGoalContinuationForTargetState(ctx, contextValue)
	}
	s.dispatchNextInputQueueItem(
		contextWithQueueOwner(ctx, entry.Item.OwnerUserID),
		sessionKey,
		contextValue.Room.ID,
		contextValue.Conversation.ID,
	)
	return true
}

func (s *RealtimeService) shouldDeferGoalContinuationForTargetState(ctx context.Context, contextValue *protocol.ConversationContextAggregate) bool {
	if s == nil || s.agents == nil || contextValue == nil {
		return false
	}
	agentNameByID, agentByID, err := s.buildAgentDirectory(ctx, contextValue)
	if err != nil {
		s.loggerFor(ctx).Warn("读取 Room Goal 续跑 Agent plan mode 状态失败", "conversation_id", contextValue.Conversation.ID, "err", err)
		return false
	}
	targetAgentID := goalContinuationTargetAgentID(contextValue, agentNameByID)
	if targetAgentID == "" {
		return true
	}
	agentValue := agentByID[targetAgentID]
	if agentValue == nil {
		return true
	}
	return goalsvc.ShouldIgnoreRuntimeForPermissionMode(agentValue.Options.PermissionMode)
}

// GoalContinuationTargetMissing 判断共享 Room Goal 的 conversation 是否已被删除。
func (s *RealtimeService) GoalContinuationTargetMissing(ctx context.Context, sessionKey string) (bool, error) {
	sessionKey = strings.TrimSpace(sessionKey)
	if s == nil || sessionKey == "" {
		return false, nil
	}
	normalized, err := protocol.RequireStructuredSessionKey(sessionKey)
	if err != nil {
		return true, nil
	}
	parsed := protocol.ParseSessionKey(normalized)
	if parsed.Kind != protocol.SessionKeyKindRoom || strings.TrimSpace(parsed.ConversationID) == "" {
		return false, nil
	}
	return s.GoalContinuationConversationMissing(ctx, parsed.ConversationID)
}

// GoalContinuationConversationMissing 判断 Room conversation 是否已不存在。
func (s *RealtimeService) GoalContinuationConversationMissing(ctx context.Context, conversationID string) (bool, error) {
	conversationID = strings.TrimSpace(conversationID)
	if s == nil || s.rooms == nil || conversationID == "" {
		return false, nil
	}
	contextValue, err := s.rooms.GetConversationContext(ctx, conversationID)
	if errors.Is(err, ErrRoomNotFound) || errors.Is(err, ErrConversationNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return contextValue == nil, nil
}

func goalContinuationTargetAgentID(
	contextValue *protocol.ConversationContextAggregate,
	agentNameByID map[string]string,
) string {
	if len(agentNameByID) == 1 {
		for agentID := range agentNameByID {
			return agentID
		}
	}
	if hostAgentID, ok := resolveRoomHostDefaultTarget(contextValue, agentNameByID); ok {
		return hostAgentID
	}
	return ""
}

func (s *RealtimeService) dispatchPostRoundWork(ctx context.Context, roundValue *activeRoomRound) {
	if roundValue == nil {
		return
	}
	if s.ShouldDeferGoalContinuation(ctx, roundValue.SessionKey) {
		return
	}
	s.dispatchGoalContinuation(ctx, roundValue)
}

func (s *RealtimeService) dispatchGoalContinuation(ctx context.Context, roundValue *activeRoomRound) {
	if s == nil || roundValue == nil || s.goals == nil {
		return
	}
	planner, ok := s.goals.(goalContinuationProvider)
	if !ok {
		return
	}
	plan, err := planner.PlanContinuationForSession(ctx, roundValue.SessionKey, roundValue.RoundID)
	if err != nil {
		if errors.Is(err, goalsvc.ErrGoalDisabled) || errors.Is(err, goalsvc.ErrGoalNotFound) || errors.Is(err, goalsvc.ErrGoalVersionStale) {
			return
		}
		s.loggerFor(ctx).Warn("规划 Room Goal 自动续跑失败",
			"session_key", roundValue.SessionKey,
			"round_id", roundValue.RoundID,
			"err", err,
		)
		return
	}
	if plan == nil {
		return
	}
	if s.ShouldDeferGoalContinuation(ctx, plan.Goal.SessionKey) {
		if releaser, ok := s.goals.(goalContinuationPlanReleaser); ok {
			_, _ = releaser.ReleaseContinuationPlan(ctx, *plan, "Goal continuation deferred before dispatch")
		}
		return
	}
	current, err := planner.GoalContinuationStillCurrent(ctx, *plan)
	if err != nil {
		if errors.Is(err, goalsvc.ErrGoalDisabled) || errors.Is(err, goalsvc.ErrGoalNotFound) || errors.Is(err, goalsvc.ErrGoalVersionStale) {
			return
		}
		s.loggerFor(ctx).Warn("校验 Room Goal 自动续跑状态失败",
			"session_key", roundValue.SessionKey,
			"round_id", plan.RoundID,
			"goal_id", plan.Goal.ID,
			"err", err,
		)
		return
	}
	if !current {
		if releaser, ok := s.goals.(goalContinuationPlanReleaser); ok {
			_, _ = releaser.ReleaseContinuationPlan(ctx, *plan, "Goal continuation stale before dispatch")
		}
		return
	}
	if err := s.DispatchGoalContinuation(ctx, *plan); err != nil {
		s.recordGoalContinuationDispatchFailure(ctx, *plan, err)
		s.loggerFor(ctx).Warn("启动 Room Goal 自动续跑失败",
			"session_key", roundValue.SessionKey,
			"round_id", plan.RoundID,
			"goal_id", plan.Goal.ID,
			"err", err,
		)
	}
}

func (s *RealtimeService) recordGoalContinuationDispatchFailure(ctx context.Context, plan protocol.GoalContinuation, dispatchErr error) {
	if s == nil || s.goals == nil || dispatchErr == nil {
		return
	}
	reason := strings.TrimSpace(dispatchErr.Error())
	if reason == "" {
		reason = "Goal continuation dispatch failed before runtime start"
	}
	if _, err := s.goals.RecordContinuationFailure(ctx, plan.Goal.ID, plan.RoundID, reason); err != nil {
		s.loggerFor(ctx).Warn("记录 Room Goal 续跑投递失败原因失败",
			"session_key", plan.Goal.SessionKey,
			"goal_id", plan.Goal.ID,
			"round_id", plan.RoundID,
			"err", err,
		)
	}
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
		GoalContext:    plan.Prompt,
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

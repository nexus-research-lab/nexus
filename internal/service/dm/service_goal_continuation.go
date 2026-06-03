package dm

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// ShouldDeferGoalContinuation 避免隐藏 Goal 续跑抢占显式输入，并按 Codex 语义跳过 Plan 模式续跑。
func (s *Service) ShouldDeferGoalContinuation(ctx context.Context, sessionKey string, agentID string) bool {
	sessionKey = strings.TrimSpace(sessionKey)
	if s == nil || sessionKey == "" {
		return false
	}
	if len(s.runtime.GetRunningRoundIDs(sessionKey)) > 0 {
		return true
	}
	normalizedSessionKey, location, err := s.resolveInputQueueLocation(ctx, sessionKey, agentID)
	if err != nil {
		s.loggerFor(ctx).Warn("解析 Goal 续跑待发送队列位置失败", "session_key", sessionKey, "err", err)
		return false
	}
	items, err := s.inputQueue.Snapshot(location)
	if err != nil {
		s.loggerFor(ctx).Warn("读取 Goal 续跑待发送队列失败", "session_key", sessionKey, "err", err)
		return false
	}
	if len(items) == 0 {
		return s.shouldDeferGoalContinuationForPlanMode(ctx, agentID)
	}
	s.dispatchNextInputQueueItemAtLocation(ctx, normalizedSessionKey, agentID, location)
	return true
}

// GoalContinuationTargetMissing 判断隐藏续跑目标 Agent 是否已被删除。
func (s *Service) GoalContinuationTargetMissing(ctx context.Context, sessionKey string, agentID string) (bool, error) {
	sessionKey = strings.TrimSpace(sessionKey)
	if s == nil || sessionKey == "" {
		return false, nil
	}
	normalized, err := protocol.RequireStructuredSessionKey(sessionKey)
	if err != nil {
		return true, nil
	}
	parsed := protocol.ParseSessionKey(normalized)
	if parsed.Kind != protocol.SessionKeyKindAgent {
		return false, nil
	}
	_, err = s.resolveInputQueueAgent(ctx, parsed, agentID)
	if errors.Is(err, agentsvc.ErrAgentNotFound) {
		return true, nil
	}
	return false, err
}

func (s *Service) shouldDeferGoalContinuationForPlanMode(ctx context.Context, agentID string) bool {
	agentID = strings.TrimSpace(agentID)
	if s == nil || s.agents == nil || agentID == "" {
		return false
	}
	agentValue, err := s.agents.GetAgent(ctx, agentID)
	if err != nil {
		s.loggerFor(ctx).Warn("读取 Goal 续跑 Agent plan mode 状态失败", "agent_id", agentID, "err", err)
		return false
	}
	return goalsvc.ShouldIgnoreRuntimeForPermissionMode(agentValue.Options.PermissionMode)
}

func (r *roundRunner) dispatchGoalContinuation(ctx context.Context) {
	if r.service.goals == nil || r.service.ShouldDeferGoalContinuation(ctx, r.sessionKey, r.agent.AgentID) {
		return
	}
	plan, err := r.service.goals.PlanContinuationForSession(ctx, r.sessionKey, r.roundID)
	if err != nil {
		if errors.Is(err, goalsvc.ErrGoalDisabled) || errors.Is(err, goalsvc.ErrGoalNotFound) || errors.Is(err, goalsvc.ErrGoalVersionStale) {
			return
		}
		r.service.loggerFor(ctx).Warn("规划 Goal 自动续跑失败",
			"session_key", r.sessionKey,
			"round_id", r.roundID,
			"err", err,
		)
		return
	}
	if plan == nil {
		return
	}
	if r.service.ShouldDeferGoalContinuation(ctx, r.sessionKey, r.agent.AgentID) {
		if releaser, ok := r.service.goals.(goalContinuationPlanReleaser); ok {
			_, _ = releaser.ReleaseContinuationPlan(ctx, *plan, "Goal continuation deferred before dispatch")
		}
		return
	}
	current, err := r.service.goals.GoalContinuationStillCurrent(ctx, *plan)
	if err != nil {
		if errors.Is(err, goalsvc.ErrGoalDisabled) || errors.Is(err, goalsvc.ErrGoalNotFound) || errors.Is(err, goalsvc.ErrGoalVersionStale) {
			return
		}
		r.service.loggerFor(ctx).Warn("校验 Goal 自动续跑状态失败",
			"session_key", r.sessionKey,
			"round_id", plan.RoundID,
			"goal_id", plan.Goal.ID,
			"err", err,
		)
		return
	}
	if !current {
		if releaser, ok := r.service.goals.(goalContinuationPlanReleaser); ok {
			_, _ = releaser.ReleaseContinuationPlan(ctx, *plan, "Goal continuation stale before dispatch")
		}
		return
	}
	if err := r.service.HandleChat(ctx, Request{
		SessionKey:           r.sessionKey,
		AgentID:              r.agent.AgentID,
		GoalContext:          plan.Prompt,
		RoundID:              plan.RoundID,
		ReqID:                plan.RoundID,
		DeliveryPolicy:       protocol.ChatDeliveryPolicyQueue,
		BroadcastUserMessage: false,
		Internal:             true,
		InputOptions: sdkprotocol.OutboundMessageOptions{
			Synthetic:      plan.Synthetic,
			HiddenFromUser: plan.HiddenFromUser,
			Purpose:        plan.Purpose,
			Priority:       "internal",
			Metadata:       plan.Metadata,
		},
	}); err != nil {
		r.recordGoalContinuationDispatchFailure(ctx, *plan, err)
		r.service.loggerFor(ctx).Warn("启动 Goal 自动续跑失败",
			"session_key", r.sessionKey,
			"round_id", plan.RoundID,
			"goal_id", plan.Goal.ID,
			"err", err,
		)
	}
}

func (r *roundRunner) recordGoalContinuationDispatchFailure(ctx context.Context, plan protocol.GoalContinuation, dispatchErr error) {
	if r == nil || r.service == nil || r.service.goals == nil || dispatchErr == nil {
		return
	}
	reason := strings.TrimSpace(dispatchErr.Error())
	if reason == "" {
		reason = "Goal continuation dispatch failed before runtime start"
	}
	if _, err := r.service.goals.RecordContinuationFailure(ctx, plan.Goal.ID, plan.RoundID, reason); err != nil {
		r.service.loggerFor(ctx).Warn("记录 Goal 续跑投递失败原因失败",
			"session_key", plan.Goal.SessionKey,
			"goal_id", plan.Goal.ID,
			"round_id", plan.RoundID,
			"err", err,
		)
	}
}

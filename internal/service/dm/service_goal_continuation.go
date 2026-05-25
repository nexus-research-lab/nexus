package dm

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
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
	return sdkpermission.Mode(strings.TrimSpace(agentValue.Options.PermissionMode)) == sdkpermission.ModePlan
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
	if err := r.service.HandleChat(ctx, Request{
		SessionKey:           r.sessionKey,
		AgentID:              r.agent.AgentID,
		Content:              plan.Prompt,
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
		r.service.loggerFor(ctx).Warn("启动 Goal 自动续跑失败",
			"session_key", r.sessionKey,
			"round_id", plan.RoundID,
			"goal_id", plan.Goal.ID,
			"err", err,
		)
	}
}

// INPUT: DM Goal continuation plan、显式输入队列与当前 runtime 状态。
// OUTPUT: 用户输入优先约束下经最终校验、原子 claim 后启动的隐藏续跑。
// POS: DM 与 Goal 状态机之间的续跑适配层。
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
	plan, err := goalsvc.PrepareContinuationForDispatch(
		ctx,
		r.service.goals,
		r.sessionKey,
		r.roundID,
		func(protocol.GoalContinuation) bool {
			return r.service.ShouldDeferGoalContinuation(ctx, r.sessionKey, r.agent.AgentID)
		},
	)
	if err != nil {
		if goalsvc.IsExpectedMutationError(err) {
			return
		}
		r.service.loggerFor(ctx).Warn("准备 Goal 自动续跑失败",
			"session_key", r.sessionKey,
			"round_id", r.roundID,
			"err", err,
		)
		return
	}
	if plan == nil {
		return
	}

	if err = r.service.DispatchGoalContinuation(ctx, *plan); err != nil {
		if goalsvc.IsExpectedMutationError(err) {
			return
		}
		r.recordGoalContinuationDispatchFailure(ctx, *plan, err)
		r.service.loggerFor(ctx).Warn("启动 Goal 自动续跑失败",
			"session_key", r.sessionKey,
			"round_id", plan.RoundID,
			"goal_id", plan.Goal.ID,
			"err", err,
		)
	}
}

// DispatchGoalContinuation 在同一启动边界内重新校验 prepared plan 并注册 runtime round。
// 自动续跑和进程恢复共享此入口，避免恢复路径绕过显式用户输入。
func (s *Service) DispatchGoalContinuation(ctx context.Context, plan protocol.GoalContinuation) error {
	if s == nil || s.goals == nil {
		return errors.New("dm goal continuation provider is not configured")
	}
	sessionKey := strings.TrimSpace(plan.Goal.SessionKey)
	parsed := protocol.ParseSessionKey(sessionKey)
	agentID := strings.TrimSpace(parsed.AgentID)
	if parsed.Kind != protocol.SessionKeyKindAgent || agentID == "" {
		return errors.New("dm goal continuation requires an agent session")
	}

	s.inputQueueDispatchMu.Lock()
	validated, err := goalsvc.ValidateContinuationForDispatch(
		ctx,
		s.goals,
		plan,
		func(protocol.GoalContinuation) bool {
			return s.shouldDeferGoalContinuationWithoutQueueDispatch(ctx, sessionKey, agentID)
		},
	)
	if err == nil && validated != nil {
		_, err = s.goals.ClaimContinuationPlan(ctx, *validated)
	}
	if err == nil && validated != nil {
		err = s.handleChat(ctx, Request{
			SessionKey:            sessionKey,
			AgentID:               agentID,
			GoalContext:           validated.Prompt,
			GoalID:                validated.Goal.ID,
			GoalObjectiveRevision: validated.Goal.ObjectiveRevision(),
			RoundID:               validated.RoundID,
			DeliveryPolicy:        protocol.ChatDeliveryPolicyQueue,
			BroadcastUserMessage:  false,
			Internal:              true,
			InputOptions: sdkprotocol.OutboundMessageOptions{
				Meta:           true,
				Synthetic:      validated.Synthetic,
				HiddenFromUser: validated.HiddenFromUser,
				Purpose:        validated.Purpose,
				Priority:       "internal",
				Metadata:       validated.Metadata,
			},
		})
	}
	s.inputQueueDispatchMu.Unlock()
	if err == nil && validated == nil {
		// 若最后校验看到新的排队输入，释放启动锁后再触发派发，避免递归获取同一把锁。
		s.ShouldDeferGoalContinuation(ctx, sessionKey, agentID)
	}
	return err
}

// shouldDeferGoalContinuationWithoutQueueDispatch 只读取最终启动条件，不在已持锁区间递归派发队列。
func (s *Service) shouldDeferGoalContinuationWithoutQueueDispatch(ctx context.Context, sessionKey string, agentID string) bool {
	if len(s.runtime.GetRunningRoundIDs(strings.TrimSpace(sessionKey))) > 0 {
		return true
	}
	_, location, err := s.resolveInputQueueLocation(ctx, sessionKey, agentID)
	if err != nil {
		s.loggerFor(ctx).Warn("解析 Goal 续跑最终队列位置失败", "session_key", sessionKey, "err", err)
		return false
	}
	items, err := s.inputQueue.Snapshot(location)
	if err != nil {
		s.loggerFor(ctx).Warn("读取 Goal 续跑最终队列失败", "session_key", sessionKey, "err", err)
		return false
	}
	return len(items) > 0 || s.shouldDeferGoalContinuationForPlanMode(ctx, agentID)
}

func (r *roundRunner) recordGoalContinuationDispatchFailure(ctx context.Context, plan protocol.GoalContinuation, dispatchErr error) {
	if r == nil || r.service == nil || r.service.goals == nil || dispatchErr == nil {
		return
	}
	reason := strings.TrimSpace(dispatchErr.Error())
	if reason == "" {
		reason = "Goal continuation dispatch failed before runtime start"
	}
	if _, err := r.service.goals.RecordContinuationFailure(ctx, plan.Goal.ID, plan.RoundID, reason, plan.Goal.ObjectiveRevision()); err != nil &&
		!goalsvc.IsExpectedMutationError(err) {
		r.service.loggerFor(ctx).Warn("记录 Goal 续跑投递失败原因失败",
			"session_key", plan.Goal.SessionKey,
			"goal_id", plan.Goal.ID,
			"round_id", plan.RoundID,
			"err", err,
		)
	}
}

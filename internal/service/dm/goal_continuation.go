// INPUT: DM Goal continuation plan、显式输入队列与当前 runtime 状态。
// OUTPUT: 用户输入优先约束下经最终校验、使用稳定或旧记录恢复的 Execution reservation、原子 claim 后启动的隐藏续跑。
// POS: DM 与 Goal 状态机之间的续跑适配层。
package dm

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// ShouldDeferGoalContinuation 避免隐藏 Goal 续跑抢占显式输入，并按 Codex 语义跳过 Plan 模式续跑。
func (s *Service) ShouldDeferGoalContinuation(ctx context.Context, sessionKey string, agentID string) bool {
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return false
	}
	if len(s.Runtime.GetRunningRoundIDs(sessionKey)) > 0 {
		return true
	}
	normalizedSessionKey, location, err := s.resolveInputQueueLocation(ctx, sessionKey, agentID)
	if err != nil {
		s.LoggerFor(ctx).Warn("解析 Goal 续跑待发送队列位置失败", "session_key", sessionKey, "err", err)
		return false
	}
	ctx = runtimehost.ContextWithExactOwner(ctx, location.OwnerUserID)
	items, err := s.InputQueue.Snapshot(location)
	if err != nil {
		s.LoggerFor(ctx).Warn("读取 Goal 续跑待发送队列失败", "session_key", sessionKey, "err", err)
		return false
	}
	if len(items) == 0 {
		return s.shouldDeferGoalContinuationForPlanMode(
			ctx,
			normalizedSessionKey,
			agentID,
		)
	}
	s.dispatchNextInputQueueItemAtLocation(ctx, normalizedSessionKey, agentID, location)
	return true
}

// GoalContinuationTargetMissing 判断隐藏续跑目标 Agent 是否已被删除。
func (s *Service) GoalContinuationTargetMissing(ctx context.Context, sessionKey string, agentID string) (bool, error) {
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
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
	agentValue, err := s.resolveInputQueueAgent(ctx, parsed, agentID)
	if errors.Is(err, agentsvc.ErrAgentNotFound) {
		return true, nil
	}
	if err == nil && agentValue != nil {
		ctx = runtimehost.ContextWithExactOwner(ctx, agentValue.OwnerUserID)
	}
	return false, err
}

func (s *Service) shouldDeferGoalContinuationForPlanMode(
	ctx context.Context,
	sessionKey string,
	agentID string,
) bool {
	sessionKey = strings.TrimSpace(sessionKey)
	agentID = strings.TrimSpace(agentID)
	if s.Agents == nil || sessionKey == "" || agentID == "" {
		return false
	}
	agentValue, err := s.Agents.GetAgent(ctx, agentID)
	if err != nil {
		s.LoggerFor(ctx).Warn("读取 Goal 续跑 Agent plan mode 状态失败", "agent_id", agentID, "err", err)
		return false
	}
	permissionMode := agentValue.Options.PermissionMode
	sessionValue, sessionErr := s.ensureSession(
		ctx,
		agentValue,
		protocol.ParseSessionKey(sessionKey),
		sessionKey,
	)
	if sessionErr != nil {
		s.LoggerFor(ctx).Warn(
			"读取 Goal 续跑 Session plan mode 状态失败",
			"session_key", sessionKey,
			"agent_id", agentID,
			"err", sessionErr,
		)
	} else if override := protocol.SessionRuntimeSettingsFromOptions(
		sessionValue.Options,
	).PermissionMode; override != "" {
		permissionMode = override
	}
	return goalsvc.ShouldIgnoreRuntimeForPermissionMode(permissionMode)
}

func (r *roundRunner) dispatchGoalContinuation(ctx context.Context) {
	shouldDefer := func(protocol.GoalContinuation) bool {
		return r.service.ShouldDeferGoalContinuation(ctx, r.sessionKey, r.agent.AgentID)
	}
	if shouldDefer(protocol.GoalContinuation{}) {
		return
	}
	runtimehost.RunGoalContinuation(ctx, r.service.goals, r.service.LoggerFor(ctx), r.sessionKey, r.roundID, shouldDefer, r.service.DispatchGoalContinuation)
}

// DispatchGoalContinuation 在同一启动边界内重新校验 prepared plan 并注册 runtime round。
// 自动续跑和进程恢复共享此入口，避免恢复路径绕过显式用户输入。
func (s *Service) DispatchGoalContinuation(ctx context.Context, plan protocol.GoalContinuation) error {
	if s.goals == nil {
		return errors.New("dm goal continuation provider is not configured")
	}
	sessionKey := strings.TrimSpace(plan.Goal.SessionKey)
	parsed := protocol.ParseSessionKey(sessionKey)
	agentID := strings.TrimSpace(parsed.AgentID)
	if parsed.Kind != protocol.SessionKeyKindAgent || agentID == "" {
		return errors.New("dm goal continuation requires an agent session")
	}
	agentValue, err := s.resolveInputQueueAgent(ctx, parsed, agentID)
	if err != nil {
		return err
	}
	if agentValue == nil || strings.TrimSpace(agentValue.OwnerUserID) == "" {
		return errors.New("dm goal continuation target agent has no owner")
	}
	ctx = runtimehost.ContextWithExactOwner(ctx, agentValue.OwnerUserID)

	if err := s.inputQueueDispatchMu.LockContext(ctx); err != nil {
		return err
	}
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
			ExecutionID:           strings.TrimSpace(validated.ExecutionID),
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
			goalContinuationAuthority: &runtimectx.GoalContinuationAuthority{
				OwnerUserID:       strings.TrimSpace(agentValue.OwnerUserID),
				AgentID:           agentID,
				ScopeSessionKey:   sessionKey,
				GoalID:            strings.TrimSpace(validated.Goal.ID),
				ObjectiveRevision: validated.Goal.ObjectiveRevision(),
				ExecutionID:       strings.TrimSpace(validated.ExecutionID),
				RootRoundID:       strings.TrimSpace(validated.RoundID),
			},
			continuationStartAdmission: func(admissionCtx context.Context) error {
				return runtimehost.MarkGoalContinuationStarted(admissionCtx, s.goals, *validated)
			},
		}, chatExecutionInline)
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
	if len(s.Runtime.GetRunningRoundIDs(strings.TrimSpace(sessionKey))) > 0 {
		return true
	}
	_, location, err := s.resolveInputQueueLocation(ctx, sessionKey, agentID)
	if err != nil {
		s.LoggerFor(ctx).Warn("解析 Goal 续跑最终队列位置失败", "session_key", sessionKey, "err", err)
		return false
	}
	items, err := s.InputQueue.Snapshot(location)
	if err != nil {
		s.LoggerFor(ctx).Warn("读取 Goal 续跑最终队列失败", "session_key", sessionKey, "err", err)
		return false
	}
	return len(items) > 0 || s.shouldDeferGoalContinuationForPlanMode(
		ctx,
		sessionKey,
		agentID,
	)
}

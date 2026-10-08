// INPUT: 会话宿主持有的 Goal provider、续跑计划、用量结算回调与 runtime 选项。
// OUTPUT: DM 与 Room 共用的 Goal 续跑标记、完成收据读取、额度限制标记、用量持久重试与诊断日志装配。
// POS: Goal 规则仍归 service/goal；这里只放两个宿主逐字相同的调用阶段，不持有宿主锁或 Room 公私策略。
package runtimehost

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

// GoalUsagePersistAttempts 是 terminal Goal 用量结算的最大尝试次数。
const GoalUsagePersistAttempts = 5

// DurableGoalContinuationLauncher 由支持持久续跑计划的 Goal provider 实现。
type DurableGoalContinuationLauncher interface {
	MarkContinuationPlanStarted(context.Context, protocol.GoalContinuation) error
	RetryContinuationPlan(context.Context, protocol.GoalContinuation, string) error
}

type durableGoalContinuationSettler interface {
	SettleContinuationPlan(context.Context, string, string, int64) error
}

type goalUsageReportReader interface {
	UsageByGoalID(context.Context, string) (*protocol.GoalUsageReport, error)
}

type goalUsageLimiter interface {
	UsageLimitForSession(context.Context, string, string, string) (*protocol.Goal, error)
}

// MarkGoalContinuationStarted 在 provider 支持持久续跑时标记计划已启动。
func MarkGoalContinuationStarted(ctx context.Context, provider any, plan protocol.GoalContinuation) error {
	if durable, ok := provider.(DurableGoalContinuationLauncher); ok {
		return durable.MarkContinuationPlanStarted(ctx, plan)
	}
	return nil
}

// SettleGoalContinuationAfterRuntime 在 runtime 结束后结算持久续跑计划。
func SettleGoalContinuationAfterRuntime(
	ctx context.Context,
	provider any,
	goalID string,
	roundID string,
	objectiveRevision int64,
) error {
	if durable, ok := provider.(durableGoalContinuationSettler); ok {
		return durable.SettleContinuationPlan(ctx, goalID, roundID, objectiveRevision)
	}
	return nil
}

// GoalCompletionReport 读取已完成 Goal 的用量收据；不可用或尚未完成时返回 false。
func GoalCompletionReport(
	ctx context.Context,
	provider any,
	logger *slog.Logger,
	goalID string,
) (*protocol.GoalUsageReport, bool) {
	reader, ok := provider.(goalUsageReportReader)
	if !ok {
		return nil, false
	}
	report, err := reader.UsageByGoalID(ctx, goalID)
	if err != nil {
		if !errors.Is(err, goalsvc.ErrGoalNotFound) {
			logger.Debug("读取 Goal 完成收据数据失败", "goal_id", goalID, "err", err)
		}
		return nil, false
	}
	if !goalsvc.IsCompletionUsageReport(report, goalID) {
		return nil, false
	}
	return report, true
}

// RecordGoalQuotaLimit 把账号额度拒绝标记到当前会话的 Goal；Goal 不活跃时静默跳过。
func RecordGoalQuotaLimit(
	ctx context.Context,
	provider any,
	logger *slog.Logger,
	sessionKey string,
	roundID string,
	quotaErr error,
) {
	limiter, ok := provider.(goalUsageLimiter)
	if !ok || quotaErr == nil {
		return
	}
	reason, ok := protocol.ClientErrorMessage(quotaErr)
	if !ok {
		return
	}
	if _, err := limiter.UsageLimitForSession(ctx, sessionKey, roundID, reason); err != nil &&
		!goalsvc.IsInactive(err) {
		logger.Warn("标记 Goal 账号额度限制失败",
			"session_key", sessionKey,
			"round_id", roundID,
			"err", err,
		)
	}
}

// PersistGoalUsageWithRetry 以指数退避重试 settle，直至成功、用尽次数或 ctx 结束。
func PersistGoalUsageWithRetry(ctx context.Context, baseDelay time.Duration, settle func() bool) bool {
	for attempt := 0; attempt < GoalUsagePersistAttempts; attempt++ {
		if attempt > 0 && !WaitGoalUsagePersistRetry(ctx, baseDelay, attempt) {
			return false
		}
		if settle() {
			return true
		}
	}
	return false
}

// WaitGoalUsagePersistRetry 按尝试次数指数退避等待；ctx 结束时返回 false。
func WaitGoalUsagePersistRetry(ctx context.Context, baseDelay time.Duration, attempt int) bool {
	if baseDelay <= 0 {
		baseDelay = 20 * time.Millisecond
	}
	timer := time.NewTimer(baseDelay * time.Duration(1<<min(attempt-1, 4)))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// SafeDeliveryPolicy 只允许经认证 WebSocket adapter 受理的输入使用 guide 投递。
func SafeDeliveryPolicy(policy protocol.ChatDeliveryPolicy, trustedConfigurationContext bool) protocol.ChatDeliveryPolicy {
	policy = protocol.NormalizeChatDeliveryPolicy(string(policy))
	if !trustedConfigurationContext && policy == protocol.ChatDeliveryPolicyGuide {
		return protocol.ChatDeliveryPolicyQueue
	}
	return policy
}

// WithRuntimeDiagnosticsLogger 把 SDK stderr 与诊断事件接入宿主日志；诊断开关由 runtime 环境决定。
func WithRuntimeDiagnosticsLogger(options agentclient.Options, logger *slog.Logger) agentclient.Options {
	diagnosticsEnabled := runtimectx.AgentSDKDiagnosticsEnabled(options.Env)
	previousStderr := options.Callbacks.Stderr
	options.Callbacks.Stderr = func(line string) {
		normalizedLine := runtimectx.NormalizeRuntimeStderrLine(line)
		if previousStderr != nil {
			previousStderr(normalizedLine)
		}
		if diagnosticsEnabled {
			logger.Info("Agent SDK stderr", "stderr", normalizedLine)
		} else {
			logger.Debug("Agent SDK stderr", "stderr", normalizedLine)
		}
	}
	previousDiagnostics := options.Callbacks.Diagnostics
	options.Callbacks.Diagnostics = func(event agentclient.DiagnosticEvent) {
		if previousDiagnostics != nil {
			previousDiagnostics(event)
		}
		component := strings.TrimSpace(event.Component)
		eventName := strings.TrimSpace(event.Event)
		if diagnosticsEnabled {
			logger.Info("Agent SDK diagnostics",
				"component", component,
				"event", eventName,
				"attrs", clientopts.SanitizeRuntimeDiagnosticAttributes(event.Event, event.Attributes),
			)
			return
		}
		if clientopts.ShouldLogRuntimeStartupDiagnostic(event) {
			logger.Info("Agent SDK startup diagnostics",
				"component", component,
				"event", eventName,
				"attrs", clientopts.SanitizeRuntimeDiagnosticAttributes(event.Event, event.Attributes),
			)
			return
		}
		if clientopts.ShouldWarnRuntimeStartupDiagnostic(event) {
			logger.Warn("Agent SDK startup diagnostics",
				"component", component,
				"event", eventName,
				"attrs", clientopts.SanitizeRuntimeDiagnosticAttributes(event.Event, event.Attributes),
			)
		}
	}
	if !diagnosticsEnabled {
		return options
	}
	logger.Info("Agent SDK diagnostics 已启用",
		"diagnostics_env", runtimectx.AgentSDKDiagnosticsValue(options.Env),
		"provider_debug_body", runtimectx.AgentSDKProviderDebugBodyValue(options.Env),
	)
	return options
}

type goalContinuationFailureRecorder interface {
	RecordContinuationRuntimeFailure(context.Context, string, goalsvc.ContinuationRuntimeIdentity, string, ...int64) (*protocol.Goal, error)
}

// RecordGoalContinuationDispatchFailure 记录续跑在 runtime 启动前投递失败；
// 持久续跑计划走 RetryContinuationPlan，否则把同一 round 记为回执与审计 round。
func RecordGoalContinuationDispatchFailure(
	ctx context.Context,
	provider any,
	logger *slog.Logger,
	plan protocol.GoalContinuation,
	dispatchErr error,
) {
	if provider == nil || dispatchErr == nil {
		return
	}
	reason := strings.TrimSpace(dispatchErr.Error())
	if reason == "" {
		reason = "Goal continuation dispatch failed before runtime start"
	}
	var err error
	if durable, ok := provider.(DurableGoalContinuationLauncher); ok {
		err = durable.RetryContinuationPlan(ctx, plan, reason)
	} else if recorder, ok := provider.(goalContinuationFailureRecorder); ok {
		_, err = recorder.RecordContinuationRuntimeFailure(
			ctx,
			plan.Goal.ID,
			goalsvc.ContinuationRuntimeIdentity{ReceiptRoundID: plan.RoundID, AuditRoundID: plan.RoundID},
			reason,
			plan.Goal.ObjectiveRevision(),
		)
	}
	if err != nil && !goalsvc.IsExpectedMutationError(err) {
		logger.Warn("记录 Goal 续跑投递失败原因失败",
			"session_key", plan.Goal.SessionKey,
			"goal_id", plan.Goal.ID,
			"round_id", plan.RoundID,
			"err", err,
		)
	}
}

type goalUsageLimitByGoal interface {
	UsageLimitForGoal(context.Context, string, string, string) (*protocol.Goal, error)
}

// RecordGoalUsageLimit 在 runtime 报告用量上限后标记 Goal；有 Goal 绑定时按 Goal 标记，
// 否则按会话标记。Goal 不活跃时静默跳过。
func RecordGoalUsageLimit(
	ctx context.Context,
	provider any,
	logger *slog.Logger,
	sessionKey string,
	goalID string,
	roundID string,
	reason string,
) {
	goalID = strings.TrimSpace(goalID)
	var err error
	if byGoal, ok := provider.(goalUsageLimitByGoal); ok && goalID != "" {
		_, err = byGoal.UsageLimitForGoal(ctx, goalID, roundID, reason)
	} else if bySession, ok := provider.(goalUsageLimiter); ok {
		_, err = bySession.UsageLimitForSession(ctx, sessionKey, roundID, reason)
	}
	if err != nil && !goalsvc.IsInactive(err) {
		logger.Warn("标记 Goal usage limit 失败",
			"session_key", sessionKey,
			"goal_id", goalID,
			"round_id", roundID,
			"err", err,
		)
	}
}

// LogGoalMutationFailure 记录尽力型 Goal 变更失败；预期的并发推进错误不记录。
func LogGoalMutationFailure(logger *slog.Logger, message string, err error, sessionKey, goalID, roundID string, fields ...any) {
	if err == nil || goalsvc.IsExpectedMutationError(err) {
		return
	}
	attrs := append([]any{"session_key", sessionKey, "goal_id", goalID, "round_id", roundID}, fields...)
	logger.Warn(message, append(attrs, "err", err)...)
}

// RetryGoalUsage 以指数退避执行 op，最多 GoalUsagePersistAttempts 次；
// 返回首个成功结果、最后一次错误，或等待期间 ctx 结束的错误。
func RetryGoalUsage[T any](ctx context.Context, baseDelay time.Duration, op func() (T, error)) (T, error) {
	var (
		zero T
		err  error
	)
	for attempt := 0; attempt < GoalUsagePersistAttempts; attempt++ {
		if attempt > 0 && !WaitGoalUsagePersistRetry(ctx, baseDelay, attempt) {
			return zero, ctx.Err()
		}
		var result T
		if result, err = op(); err == nil {
			return result, nil
		}
	}
	return zero, err
}

// GoalUsageScopeBinder 由支持 durable from-now 用量边界的 Goal provider 实现。
type GoalUsageScopeBinder interface {
	BindUsageScopeFromNow(context.Context, protocol.GoalUsageScopeBinding) (protocol.GoalUsageScopeBindResult, error)
}

// BindGoalUsageScopeFromNow 建立 durable from-now 用量边界。ErrGoalInvalidState 表示
// provider 没有 durable scope 能力，按成功处理并沿用内存记账；其他错误必须阻止 Reset。
func BindGoalUsageScopeFromNow(
	ctx context.Context,
	binder GoalUsageScopeBinder,
	baseDelay time.Duration,
	binding protocol.GoalUsageScopeBinding,
) error {
	_, err := RetryGoalUsage(ctx, baseDelay, func() (struct{}, error) {
		_, bindErr := binder.BindUsageScopeFromNow(ctx, binding)
		if errors.Is(bindErr, goalsvc.ErrGoalInvalidState) {
			return struct{}{}, nil
		}
		return struct{}{}, bindErr
	})
	return err
}

type goalUsageRoundClaimer interface {
	ClaimUsageSourceRound(context.Context, protocol.GoalUsageSourceRoundClaim) (protocol.GoalUsageSourceResult, error)
}

// ClaimGoalUsageSourceRound 认领子任务用量 source round；provider 不支持认领时视为已完成。
func ClaimGoalUsageSourceRound(
	ctx context.Context,
	provider any,
	baseDelay time.Duration,
	claim protocol.GoalUsageSourceRoundClaim,
) bool {
	claimer, ok := provider.(goalUsageRoundClaimer)
	if !ok {
		return true
	}
	_, err := RetryGoalUsage(ctx, baseDelay, func() (protocol.GoalUsageSourceResult, error) {
		return claimer.ClaimUsageSourceRound(ctx, claim)
	})
	return err == nil
}

// RunGoalContinuation 为 sessionKey 准备下一轮 Goal 续跑计划并交给宿主 dispatch；
// 启动前失败会回写 Goal，预期的并发推进错误静默结束。
func RunGoalContinuation(
	ctx context.Context,
	provider any,
	logger *slog.Logger,
	sessionKey string,
	causedByRoundID string,
	shouldDefer func(protocol.GoalContinuation) bool,
	dispatch func(context.Context, protocol.GoalContinuation) error,
) {
	planner, ok := provider.(goalsvc.ContinuationPlanProvider)
	if !ok || sessionKey == "" {
		return
	}
	plan, err := goalsvc.PrepareContinuationForDispatch(ctx, planner, sessionKey, causedByRoundID, shouldDefer)
	if err != nil {
		LogGoalMutationFailure(logger, "准备 Goal 自动续跑失败", err, sessionKey, "", causedByRoundID)
		return
	}
	if plan == nil {
		return
	}
	if err := dispatch(ctx, *plan); err != nil && !goalsvc.IsExpectedMutationError(err) {
		RecordGoalContinuationDispatchFailure(ctx, provider, logger, *plan, err)
		LogGoalMutationFailure(logger, "启动 Goal 自动续跑失败", err, sessionKey, plan.Goal.ID, plan.RoundID)
	}
}

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

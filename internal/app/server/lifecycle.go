// INPUT: Server services, runtime managers, root lifecycle context、durable Goal confirmation/subagent deadline 与 orchestration dispatch state。
// OUTPUT: 启动及周期恢复 Goal binding、Plan proposal、child Attempt、lease，并在 successor dispatch 前优先 drain cancellation。
// POS: 启动、监管和停止后台 orchestration 恢复器的应用生命周期边界。
package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
)

const (
	httpReadHeaderTimeout = 10 * time.Second
	httpReadTimeout       = 30 * time.Second
	// Git 导入和外部 skill 更新会同步等待网络传输与重试，写超时需要覆盖完整操作窗口。
	httpWriteTimeout = 6 * time.Minute
	httpIdleTimeout  = 60 * time.Second

	executionDispatchInterval     = time.Second
	executionDispatchBatch        = 32
	subagentReconcileInterval     = time.Second
	subagentReconcileBatch        = 32
	planProposalReconcileInterval = 15 * time.Second
	planProposalReconcileBatch    = 32
	goalConfirmationInterval      = 15 * time.Second
	goalConfirmationBatch         = 32
)

// ListenAndServe 启动后台服务与 HTTP 服务。
func (s *Server) ListenAndServe(ctx context.Context) error {
	stopBackground, err := s.startBackgroundServices(ctx)
	if err != nil {
		return err
	}
	defer stopBackground()

	httpServer := &http.Server{
		Addr:              s.config.Address(),
		Handler:           s.router,
		ReadHeaderTimeout: httpReadHeaderTimeout,
		ReadTimeout:       httpReadTimeout,
		WriteTimeout:      httpWriteTimeout,
		IdleTimeout:       httpIdleTimeout,
	}

	go func() {
		<-ctx.Done()
		s.api.BaseLogger().Info("收到停止信号，开始关闭 HTTP 服务")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	s.api.BaseLogger().Info("HTTP 服务开始监听",
		"addr", s.config.Address(),
		"api_prefix", s.config.APIPrefix,
		"websocket_path", s.config.WebSocketPath,
	)
	return httpServer.ListenAndServe()
}

func (s *Server) startBackgroundServices(ctx context.Context) (func(), error) {
	var stops []func()
	stopAll := func() {
		for i := len(stops) - 1; i >= 0; i-- {
			stops[i]()
		}
	}
	starters := []func(context.Context) (func(), error){
		s.startSessionDeletionRecovery,
		s.startChannels,
		s.startAutomation,
		s.startRoomPublicHandoffs,
		s.startRoomDirectedWakes,
		s.startGoalConfirmationRecovery,
		s.startPlanProposalRecovery,
		s.startSubagentReconciliation,
		s.startExecutionDispatches,
		s.startMemoryMaintenance,
		s.startGoalResume,
	}
	for _, start := range starters {
		stop, err := start(ctx)
		if err != nil {
			stopAll()
			return nil, err
		}
		if stop != nil {
			stops = append(stops, stop)
		}
	}
	if stopRuntimeIdleReclaimer := s.startRuntimeIdleSessionReclaimer(ctx); stopRuntimeIdleReclaimer != nil {
		stops = append(stops, stopRuntimeIdleReclaimer)
	}
	if s.services != nil && s.services.Title != nil {
		stops = append(stops, func() {
			if err := s.services.Title.Close(context.Background()); err != nil {
				s.api.BaseLogger().Warn("标题生成后台任务关闭失败", "err", err)
			}
		})
	}

	return stopAll, nil
}

func (s *Server) startGoalConfirmationRecovery(ctx context.Context) (func(), error) {
	if s.services == nil || s.services.Orchestration == nil {
		return nil, nil
	}
	workerCtx, cancel := context.WithCancel(ctx)
	reconcile := func() {
		result, err := s.services.Orchestration.ReconcileGoalConfirmations(
			workerCtx,
			goalConfirmationBatch,
		)
		if err != nil {
			s.api.BaseLogger().Warn("恢复 Execution Goal confirmation 失败", "err", err)
		}
		if result.Scanned > 0 {
			s.api.BaseLogger().Info(
				"恢复 Execution Goal confirmation",
				"scanned", result.Scanned,
				"confirmed", result.Confirmed,
				"pending", result.Pending,
				"failed", result.Failed,
			)
		}
	}
	reconcile()
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(goalConfirmationInterval)
		defer ticker.Stop()
		for {
			select {
			case <-workerCtx.Done():
				return
			case <-ticker.C:
				reconcile()
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}, nil
}

func (s *Server) startPlanProposalRecovery(ctx context.Context) (func(), error) {
	if s.services == nil || s.services.Orchestration == nil {
		return nil, nil
	}
	reconcile := func() {
		result, err := s.services.Orchestration.ReconcilePlanProposals(
			ctx,
			planProposalReconcileBatch,
		)
		if err != nil {
			s.api.BaseLogger().Warn("恢复 Execution Plan proposal 失败", "err", err)
		}
		if result.Scanned > 0 {
			s.api.BaseLogger().Info(
				"恢复 Execution Plan proposal",
				"scanned", result.Scanned,
				"materialized", result.Materialized,
				"confirmed", result.Confirmed,
				"blocked", result.Blocked,
				"failed", result.Failed,
			)
		}
	}
	reconcile()
	workerCtx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(planProposalReconcileInterval)
		defer ticker.Stop()
		for {
			select {
			case <-workerCtx.Done():
				return
			case <-ticker.C:
				reconcile()
			}
		}
	}()
	return cancel, nil
}

func (s *Server) startSubagentReconciliation(ctx context.Context) (func(), error) {
	if s.services == nil || s.services.Orchestration == nil {
		return nil, nil
	}
	runCtx, stop := context.WithCancel(ctx)
	result, err := s.services.Orchestration.ReconcileExpiredSubagents(
		runCtx,
		subagentReconcileBatch,
	)
	if err != nil {
		stop()
		s.api.BaseLogger().Error("启动 Subagent Attempt 恢复器失败", "err", err)
		return nil, err
	}
	s.logSubagentReconciliationResult("启动恢复", result)
	s.api.BaseLogger().Info(
		"启动 Subagent Attempt 恢复器",
		"interval_seconds",
		int64(subagentReconcileInterval.Seconds()),
	)
	go s.runSubagentReconciliation(runCtx)
	return stop, nil
}

func (s *Server) runSubagentReconciliation(ctx context.Context) {
	ticker := time.NewTicker(subagentReconcileInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			result, err := s.services.Orchestration.ReconcileExpiredSubagents(
				ctx,
				subagentReconcileBatch,
			)
			if err != nil {
				s.api.BaseLogger().Warn("Subagent Attempt 恢复失败", "err", err)
				continue
			}
			s.logSubagentReconciliationResult("定时恢复", result)
		}
	}
}

func (s *Server) startExecutionDispatches(ctx context.Context) (func(), error) {
	if s.services == nil || s.services.Orchestration == nil || s.services.RoomRealtime == nil {
		return nil, nil
	}
	runCtx, stop := context.WithCancel(ctx)
	workerID := executionDispatchWorkerID()
	cancellationResult, err := s.services.Orchestration.DispatchPendingCancellations(
		runCtx,
		workerID,
		executionDispatchBatch,
	)
	if err != nil {
		stop()
		s.api.BaseLogger().Error(
			"启动 Execution Cancellation Dispatch 恢复器失败",
			"err",
			err,
		)
		return nil, err
	}
	s.logExecutionCancellationResult("启动恢复", cancellationResult)
	result, err := s.services.Orchestration.DispatchPending(
		runCtx,
		workerID,
		executionDispatchBatch,
	)
	if err != nil {
		stop()
		s.api.BaseLogger().Error("启动 Execution Room Dispatch 恢复器失败", "err", err)
		return nil, err
	}
	s.logExecutionDispatchResult("启动恢复", result)
	reviewResult, err := s.services.Orchestration.DispatchPendingReviews(
		runCtx,
		workerID,
		executionDispatchBatch,
	)
	if err != nil {
		stop()
		s.api.BaseLogger().Error("启动 Execution Review Dispatch 恢复器失败", "err", err)
		return nil, err
	}
	s.logExecutionDispatchResult("Review 启动恢复", reviewResult)
	s.api.BaseLogger().Info(
		"启动 Execution Room/Review/Cancellation Dispatch 恢复器",
		"interval_seconds",
		int64(executionDispatchInterval.Seconds()),
	)
	go s.runExecutionDispatches(runCtx, workerID)
	return stop, nil
}

func (s *Server) runExecutionDispatches(ctx context.Context, workerID string) {
	ticker := time.NewTicker(executionDispatchInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cancellationResult, cancellationErr :=
				s.services.Orchestration.DispatchPendingCancellations(
					ctx,
					workerID,
					executionDispatchBatch,
				)
			if cancellationErr != nil {
				s.api.BaseLogger().Warn(
					"Execution Cancellation Dispatch 恢复失败",
					"err",
					cancellationErr,
				)
			} else {
				s.logExecutionCancellationResult(
					"定时恢复",
					cancellationResult,
				)
			}
			result, err := s.services.Orchestration.DispatchPending(
				ctx,
				workerID,
				executionDispatchBatch,
			)
			if err != nil {
				s.api.BaseLogger().Warn("Execution Room Dispatch 恢复失败", "err", err)
			} else {
				s.logExecutionDispatchResult("定时恢复", result)
			}
			reviewResult, reviewErr := s.services.Orchestration.DispatchPendingReviews(
				ctx,
				workerID,
				executionDispatchBatch,
			)
			if reviewErr != nil {
				s.api.BaseLogger().Warn(
					"Execution Review Dispatch 恢复失败",
					"err",
					reviewErr,
				)
			} else {
				s.logExecutionDispatchResult("Review 定时恢复", reviewResult)
			}
		}
	}
}

func (s *Server) logSubagentReconciliationResult(
	source string,
	result orchestrationsvc.SubagentReconciliationResult,
) {
	if result.Scanned == 0 && result.Reconciled == 0 && result.Deferred == 0 {
		return
	}
	s.api.BaseLogger().Info(
		"Subagent Attempt 已恢复",
		"source",
		source,
		"scanned",
		result.Scanned,
		"reconciled",
		result.Reconciled,
		"deferred",
		result.Deferred,
	)
}

func (s *Server) logExecutionDispatchResult(
	source string,
	result orchestrationsvc.DispatchRunResult,
) {
	if result.Claimed == 0 && result.Delivered == 0 &&
		result.Retried == 0 && result.Cancelled == 0 {
		return
	}
	s.api.BaseLogger().Info(
		"Execution Room Dispatch 已处理",
		"source",
		source,
		"claimed",
		result.Claimed,
		"delivered",
		result.Delivered,
		"retried",
		result.Retried,
		"cancelled",
		result.Cancelled,
	)
}

func (s *Server) logExecutionCancellationResult(
	source string,
	result orchestrationsvc.CancellationDispatchRunResult,
) {
	if result.Claimed == 0 && result.Delivered == 0 &&
		result.Retried == 0 && result.NotRequired == 0 &&
		result.Unsupported == 0 {
		return
	}
	s.api.BaseLogger().Info(
		"Execution Cancellation Dispatch 已处理",
		"source",
		source,
		"claimed",
		result.Claimed,
		"delivered",
		result.Delivered,
		"retried",
		result.Retried,
		"not_required",
		result.NotRequired,
		"unsupported",
		result.Unsupported,
	)
}

func executionDispatchWorkerID() string {
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		host = "unknown-host"
	}
	return fmt.Sprintf("nexus:%s:%d", strings.TrimSpace(host), os.Getpid())
}

func (s *Server) startSessionDeletionRecovery(ctx context.Context) (func(), error) {
	if s.services == nil || s.services.Core == nil || s.services.Core.Session == nil {
		return nil, nil
	}
	sessionService := s.services.Core.Session
	reconciled, err := sessionService.ReconcilePendingDeletions(ctx)
	if sessionsvc.SessionDeletionRecoveryScanFailed(err) {
		return nil, err
	}
	if err != nil {
		s.api.BaseLogger().Warn("Session 删除恢复尚未完成，已保留 admission fence",
			"reconciled", reconciled,
			"err", err,
		)
	} else if reconciled > 0 {
		s.api.BaseLogger().Info("Session 删除恢复完成", "reconciled", reconciled)
	}
	runCtx, stop := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				reconciled, reconcileErr := sessionService.ReconcilePendingDeletions(runCtx)
				if reconcileErr != nil && !errors.Is(reconcileErr, context.Canceled) {
					s.api.BaseLogger().Warn("Session 删除周期恢复未完成",
						"reconciled", reconciled,
						"err", reconcileErr,
					)
				}
			}
		}
	}()
	return stop, nil
}

func (s *Server) startRoomPublicHandoffs(ctx context.Context) (func(), error) {
	if s.services == nil || s.services.RoomRealtime == nil {
		return nil, nil
	}
	s.api.BaseLogger().Info("启动 Room handoff 恢复器")
	stop, err := s.services.RoomRealtime.StartPublicHandoffReconciler(ctx)
	if err != nil {
		s.api.BaseLogger().Error("启动 Room handoff 恢复器失败", "err", err)
		return nil, err
	}
	return stop, nil
}

func (s *Server) startRoomDirectedWakes(ctx context.Context) (func(), error) {
	if s.services == nil || s.services.RoomRealtime == nil {
		return nil, nil
	}
	s.api.BaseLogger().Info("启动 Room directed wake 恢复器")
	stop, err := s.services.RoomRealtime.StartDelayedWakeScheduler(ctx)
	if err != nil {
		s.api.BaseLogger().Error("启动 Room directed wake 恢复器失败", "err", err)
		return nil, err
	}
	return stop, nil
}

func (s *Server) startChannels(ctx context.Context) (func(), error) {
	if s.services == nil || s.services.Channels == nil {
		return nil, nil
	}
	if s.services.ChannelControl != nil {
		if err := s.services.ChannelControl.LoadConfiguredChannels(ctx); err != nil {
			s.api.BaseLogger().Warn("加载 IM 通道配置失败，跳过数据库通道注册", "err", err)
		}
	}
	s.api.BaseLogger().Info("启动通道适配器",
		"discord_enabled", s.config.DiscordEnabled,
		"discord_configured", strings.TrimSpace(s.config.DiscordBotToken) != "",
		"telegram_enabled", s.config.TelegramEnabled,
		"telegram_configured", strings.TrimSpace(s.config.TelegramBotToken) != "",
		"registered_channels", s.services.Channels.RegisteredChannelTypes(),
	)
	if err := s.services.Channels.Start(ctx); err != nil {
		s.api.BaseLogger().Error("启动通道适配器失败", "err", err)
		return nil, err
	}
	return func() { s.services.Channels.Stop(context.Background()) }, nil
}

func (s *Server) startAutomation(ctx context.Context) (func(), error) {
	if s.services == nil || s.services.Automation == nil {
		return nil, nil
	}
	s.api.BaseLogger().Info("启动自动化调度器")
	if err := s.services.Automation.Start(ctx); err != nil {
		s.api.BaseLogger().Error("启动自动化调度器失败", "err", err)
		return nil, err
	}
	return s.services.Automation.Stop, nil
}

func (s *Server) startMemoryMaintenance(ctx context.Context) (func(), error) {
	if s.services == nil || s.services.MemoryMaintenance == nil {
		return nil, nil
	}
	s.api.BaseLogger().Info("启动记忆维护协调器")
	if err := s.services.MemoryMaintenance.Start(ctx); err != nil {
		s.api.BaseLogger().Error("启动记忆维护协调器失败", "err", err)
		return nil, err
	}
	return s.services.MemoryMaintenance.Stop, nil
}

func (s *Server) startGoalResume(ctx context.Context) (func(), error) {
	if s.services == nil || s.services.Goal == nil {
		return nil, nil
	}
	s.api.BaseLogger().Info("启动 Goal durable resume")
	if err := s.services.Goal.RepairCurrentGoalPreviews(ctx); err != nil {
		s.api.BaseLogger().Warn("Goal 会话标题恢复未完全成功", "err", err)
	}
	stop, err := s.services.Goal.StartAutoResume(
		ctx,
		newGoalContinuationDispatcher(s.services.Runtime, s.services.DM, s.services.RoomRealtime),
	)
	if err != nil {
		s.api.BaseLogger().Error("启动 Goal durable resume 失败", "err", err)
		return nil, err
	}
	return stop, nil
}

func (s *Server) startRuntimeIdleSessionReclaimer(ctx context.Context) func() {
	if s.services == nil || s.services.Runtime == nil {
		return nil
	}
	idleFor := s.config.RuntimeIdleSessionTTL()
	sweepInterval := s.config.RuntimeIdleSessionSweepInterval()
	if idleFor <= 0 || sweepInterval <= 0 {
		return nil
	}

	runCtx, stop := context.WithCancel(ctx)
	s.api.BaseLogger().Info("启动 runtime 空闲 session 回收器",
		"idle_ttl_seconds", int64(idleFor.Seconds()),
		"sweep_interval_seconds", int64(sweepInterval.Seconds()),
	)
	go s.runRuntimeIdleSessionReclaimer(runCtx, sweepInterval, idleFor)
	return stop
}

func (s *Server) runRuntimeIdleSessionReclaimer(ctx context.Context, sweepInterval time.Duration, idleFor time.Duration) {
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			closed, err := s.services.Runtime.CloseIdleSessions(ctx, idleFor)
			if err != nil {
				s.api.BaseLogger().Warn("runtime 空闲 session 回收失败", "closed", closed, "err", err)
				continue
			}
			if closed > 0 {
				s.api.BaseLogger().Info("runtime 空闲 session 已回收", "closed", closed)
			}
		}
	}
}

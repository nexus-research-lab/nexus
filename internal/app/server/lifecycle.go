// INPUT: Server services, runtime managers, root lifecycle context、durable completion/Goal confirmation/subagent deadline 与 orchestration dispatch state。
// OUTPUT: 启动并监管 event/deadline 驱动的 completion audit、Goal binding、Plan proposal、child Attempt、lease 与 dispatch coordinator。
// POS: 后台 coordinator 的应用生命周期边界；业务顺序、claim 和 retry 留在 service。
package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	servergoal "github.com/nexus-research-lab/nexus/internal/app/server/goal"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
)

const (
	httpReadHeaderTimeout = 10 * time.Second
	httpReadTimeout       = 30 * time.Second
	// Git 导入和外部 skill 更新会同步等待网络传输与重试，写超时需要覆盖完整操作窗口。
	httpWriteTimeout = 6 * time.Minute
	httpIdleTimeout  = 60 * time.Second

	executionDispatchBatch     = 32
	subagentReconcileBatch     = 32
	orchestrationRecoveryBatch = 32
	controlInvalidationPoll    = time.Second
	controlInvalidationGrace   = time.Minute
)

type controlIdentityInvalidationSource interface {
	LatestControlIdentityInvalidationID(context.Context) (int64, error)
	ControlIdentityInvalidations(context.Context, int64) ([]authsvc.ControlIdentityInvalidation, error)
	ApplyControlIdentityInvalidation(context.Context, authsvc.ControlIdentityInvalidation) (string, error)
	FailClosedControlIdentities(context.Context) ([]string, error)
}

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
		s.startControlIdentityInvalidations,
		s.startSessionDeletionRecovery,
		s.startChannels,
		s.startEcho,
		s.startAutomation,
		s.startRoomPublicHandoffs,
		s.startRoomDirectedWakes,
		s.startOrchestrationRecovery,
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

func (s *Server) startControlIdentityInvalidations(ctx context.Context) (func(), error) {
	if s.services == nil || s.services.Auth == nil {
		return nil, nil
	}
	source, ok := s.services.Auth.(controlIdentityInvalidationSource)
	if !ok {
		return nil, nil
	}
	cursor, err := source.LatestControlIdentityInvalidationID(ctx)
	if err != nil {
		// 与运行时轮询保持一致：Control 暂时不可用时先从 cursor=0 启动，
		// 由 runControlIdentityInvalidations 在 grace 窗口后执行既有 fail-closed 逻辑。
		s.api.BaseLogger().Warn("初始化 Control identity invalidation cursor 失败，从 0 开始等待 Control 恢复", "err", err)
		cursor = 0
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.runControlIdentityInvalidations(runCtx, source, cursor)
	}()
	s.api.BaseLogger().Info("启动 Control identity invalidation coordinator", "cursor", cursor)
	return func() {
		cancel()
		<-done
	}, nil
}

func (s *Server) runControlIdentityInvalidations(
	ctx context.Context,
	source controlIdentityInvalidationSource,
	cursor int64,
) {
	var unavailableSince time.Time
	failClosed := false
	for ctx.Err() == nil {
		events, err := source.ControlIdentityInvalidations(ctx, cursor)
		if err != nil {
			if unavailableSince.IsZero() {
				unavailableSince = time.Now().UTC()
			}
			s.api.BaseLogger().Warn("读取 Control identity invalidation 失败", "err", err)
			if !failClosed && time.Since(unavailableSince) >= controlInvalidationGrace {
				owners, closeErr := source.FailClosedControlIdentities(ctx)
				connections := s.handlers.websocket.CloseControlConnections()
				for _, ownerUserID := range owners {
					_, runtimeErr := s.services.Runtime.CloseOwnerSessions(ctx, ownerUserID)
					closeErr = errors.Join(closeErr, runtimeErr)
				}
				s.api.BaseLogger().Error(
					"Control identity invalidation 超过安全窗口，已关闭认证会话",
					"owners", len(owners),
					"connections", connections,
					"err", closeErr,
				)
				failClosed = true
			}
			if !waitControlInvalidationPoll(ctx) {
				return
			}
			continue
		}
		unavailableSince = time.Time{}
		failClosed = false
		processedAll := true
		for _, event := range events {
			ownerUserID, applyErr := source.ApplyControlIdentityInvalidation(ctx, event)
			connections := 0
			if ownerUserID != "" {
				switch event.Reason {
				case "session_revoked":
					connections = s.handlers.websocket.CloseControlSessionConnections(event.SessionID)
				case "profile_changed":
					connections = s.handlers.websocket.CloseOwnerConnections(ownerUserID)
				default:
					connections = s.handlers.websocket.CloseOwnerConnections(ownerUserID)
					_, runtimeErr := s.services.Runtime.CloseOwnerSessions(ctx, ownerUserID)
					applyErr = errors.Join(applyErr, runtimeErr)
				}
			}
			if applyErr != nil {
				s.api.BaseLogger().Warn(
					"应用 Control identity invalidation 失败",
					"event_id", event.EventID,
					"owner_user_id", ownerUserID,
					"err", applyErr,
				)
				processedAll = false
				break
			}
			cursor = event.EventID
			s.api.BaseLogger().Info(
				"应用 Control identity invalidation",
				"event_id", event.EventID,
				"owner_user_id", ownerUserID,
				"connections", connections,
			)
		}
		if processedAll && len(events) == authsvc.ControlIdentityInvalidationBatchSize {
			continue
		}
		if !waitControlInvalidationPoll(ctx) {
			return
		}
	}
}

func waitControlInvalidationPoll(ctx context.Context) bool {
	timer := time.NewTimer(controlInvalidationPoll)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (s *Server) startEcho(ctx context.Context) (func(), error) {
	if s.services == nil || s.services.Echo == nil {
		return nil, nil
	}
	s.api.BaseLogger().Info("启动 Echo 调度器")
	if err := s.services.Echo.Start(ctx); err != nil {
		s.api.BaseLogger().Error("启动 Echo 调度器失败", "err", err)
		return nil, err
	}
	return s.services.Echo.Stop, nil
}

func (s *Server) startOrchestrationRecovery(ctx context.Context) (func(), error) {
	if s.services == nil || s.services.Orchestration == nil {
		return nil, nil
	}
	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		err := s.services.Orchestration.RunRecoveryCoordinator(
			workerCtx,
			orchestrationRecoveryBatch,
			orchestrationsvc.RecoveryCoordinatorObserver{
				OnError: func(kind string, err error) {
					s.api.BaseLogger().Warn(
						"Execution durable recovery 失败",
						"kind", kind,
						"err", err,
					)
				},
				OnCompletionAudit: func(result orchestrationsvc.CompletionAuditRecoveryResult) {
					if result.Scanned == 0 {
						return
					}
					s.api.BaseLogger().Info(
						"恢复 Execution completion audit",
						"scanned", result.Scanned,
						"completed", result.Completed,
						"deferred", result.Deferred,
						"discarded", result.Discarded,
						"failed", result.Failed,
					)
				},
				OnGoalConfirmation: func(result orchestrationsvc.GoalConfirmationRecoveryResult) {
					if result.Scanned == 0 {
						return
					}
					s.api.BaseLogger().Info(
						"恢复 Execution Goal confirmation",
						"scanned", result.Scanned,
						"confirmed", result.Confirmed,
						"pending", result.Pending,
						"failed", result.Failed,
					)
				},
				OnPlanProposal: func(result orchestrationsvc.PlanProposalRecoveryResult) {
					if result.Scanned == 0 {
						return
					}
					s.api.BaseLogger().Info(
						"恢复 Execution Plan proposal",
						"scanned", result.Scanned,
						"materialized", result.Materialized,
						"confirmed", result.Confirmed,
						"blocked", result.Blocked,
						"failed", result.Failed,
					)
				},
			},
		)
		if err != nil && workerCtx.Err() == nil {
			s.api.BaseLogger().Error("Execution durable recovery coordinator 已停止", "err", err)
		}
	}()
	s.api.BaseLogger().Info("启动 Execution durable recovery coordinator")
	return func() {
		cancel()
		<-done
	}, nil
}

func (s *Server) startSubagentReconciliation(ctx context.Context) (func(), error) {
	if s.services == nil || s.services.Orchestration == nil {
		return nil, nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	processStartedAt := time.Now().UTC()
	done := make(chan struct{})
	go func() {
		defer close(done)
		err := s.services.Orchestration.RunSubagentReconciliationCoordinator(
			runCtx,
			processStartedAt,
			subagentReconcileBatch,
			s.logSubagentReconciliationResult,
			func(kind string, err error) {
				s.api.BaseLogger().Warn(
					"Subagent Attempt 恢复失败",
					"kind", kind,
					"err", err,
				)
			},
		)
		if err != nil && runCtx.Err() == nil {
			s.api.BaseLogger().Error("Subagent reconciliation coordinator 已停止", "err", err)
		}
	}()
	s.api.BaseLogger().Info("启动 Subagent deadline reconciliation coordinator")
	return func() {
		cancel()
		<-done
	}, nil
}

func (s *Server) startExecutionDispatches(ctx context.Context) (func(), error) {
	if s.services == nil || s.services.Orchestration == nil || s.services.RoomRealtime == nil {
		return nil, nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	workerID := executionDispatchWorkerID()
	done := make(chan struct{})
	go func() {
		defer close(done)
		err := s.services.Orchestration.RunExecutionDispatchCoordinator(
			runCtx,
			workerID,
			executionDispatchBatch,
			orchestrationsvc.DispatchCoordinatorObserver{
				OnError: func(kind string, err error) {
					s.api.BaseLogger().Warn(
						"Execution Dispatch 恢复失败",
						"kind", kind,
						"err", err,
					)
				},
				OnCancellation: func(result orchestrationsvc.CancellationDispatchRunResult) {
					s.logExecutionCancellationResult("deadline/event", result)
				},
				OnRoom: func(result orchestrationsvc.DispatchRunResult) {
					s.logExecutionDispatchResult("deadline/event", result)
				},
				OnReview: func(result orchestrationsvc.DispatchRunResult) {
					s.logExecutionDispatchResult("Review deadline/event", result)
				},
			},
		)
		if err != nil && runCtx.Err() == nil {
			s.api.BaseLogger().Error("Execution dispatch coordinator 已停止", "err", err)
		}
	}()
	s.api.BaseLogger().Info("启动 Execution deadline dispatch coordinator")
	return func() {
		cancel()
		<-done
	}, nil
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
		servergoal.NewContinuationDispatcher(s.services.Runtime, s.services.DM, s.services.RoomRealtime),
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

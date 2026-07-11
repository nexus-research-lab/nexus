package automation

import (
	"context"
	"log/slog"
	"strings"
	"time"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

func (s *Service) startJobExecution(ctx context.Context, job automationdomain.ScheduledTask, triggerKind string, scheduledFor time.Time) (*automationdomain.ExecutionResult, error) {
	starter := jobExecutionStarter{
		service:      s,
		ctx:          ctx,
		job:          job,
		triggerKind:  triggerKind,
		scheduledFor: scheduledFor,
		logger: s.loggerFor(ctx).With(
			"job_id", job.JobID,
			"agent_id", job.AgentID,
			"trigger_kind", triggerKind,
		),
	}
	return starter.start()
}

type jobExecutionStarter struct {
	service      *Service
	ctx          context.Context
	job          automationdomain.ScheduledTask
	triggerKind  string
	scheduledFor time.Time
	logger       *slog.Logger
	runID        string
	sessionKey   string
	roundID      string
	startedAt    time.Time
	nextRunAt    *time.Time
	overlap      string
}

func (s *jobExecutionStarter) start() (*automationdomain.ExecutionResult, error) {
	if automationdomain.NormalizeExecutionKind(s.job.ExecutionKind) == automationdomain.ExecutionKindScript {
		return s.service.startScriptJobExecution(s.ctx, s.job, s.triggerKind, s.scheduledFor)
	}
	if err := s.service.ensureDirectTargetSupported(s.job.SessionTarget); err != nil {
		s.failRuntime(err, s.service.nowFn())
		s.logger.Error("自动化任务目标校验失败", "err", err)
		return nil, err
	}
	if strings.TrimSpace(s.job.SessionTarget.Kind) == automationdomain.SessionTargetMain {
		return s.startMainSession()
	}
	return s.startRuntimeSession()
}

func (s *jobExecutionStarter) startMainSession() (*automationdomain.ExecutionResult, error) {
	if err := s.prepareMainRun(); err != nil {
		return nil, err
	}
	eventID, err := s.service.enqueueMainSessionEvent(s.ctx, s.job, s.triggerKind)
	if err != nil {
		s.failPendingRun(err)
		return nil, err
	}
	mode := normalizedWakeMode(s.job.SessionTarget.WakeMode)
	if _, err = s.service.WakeHeartbeat(s.ctx, s.job.AgentID, automationdomain.HeartbeatWakeInput{Mode: mode}); err != nil {
		_ = s.service.repository.MarkSystemEventStatus(context.Background(), eventID, "failed")
		s.failPendingRun(err)
		s.logger.Error("自动化任务唤醒主会话 heartbeat 失败", "err", err)
		return nil, err
	}
	s.finishMainRun(mode)
	return s.queuedMainResult(), nil
}

func (s *jobExecutionStarter) prepareMainRun() error {
	s.runID = s.service.idFactory("run")
	sessionKey, err := automationexec.ResolveSessionKey(s.job, nil)
	if err != nil {
		s.failRuntime(err, s.service.nowFn())
		s.logger.Error("自动化任务解析主会话键失败", "err", err)
		return err
	}
	s.sessionKey = sessionKey
	err = s.service.repository.InsertRunPending(s.ctx, automationstore.RunPendingInput{
		RunID:        s.runID,
		JobID:        s.job.JobID,
		OwnerUserID:  s.job.OwnerUserID,
		ScheduledFor: &s.scheduledFor,
		TriggerKind:  s.triggerKind,
		SessionKey:   s.sessionKey,
		DeliveryMode: automationdomain.DeliveryModeNone,
	})
	if err != nil {
		s.failRuntime(err, s.service.nowFn())
	}
	return err
}

func normalizedWakeMode(mode string) string {
	if strings.TrimSpace(mode) == "" {
		return automationdomain.WakeModeNextHeartbeat
	}
	return mode
}

func (s *jobExecutionStarter) finishMainRun(mode string) {
	finishedAt := s.service.nowFn()
	_ = s.service.repository.MarkRunFinished(context.Background(), automationstore.RunFinishInput{
		RunID:      s.runID,
		Status:     automationdomain.RunStatusQueuedToMain,
		FinishedAt: finishedAt,
	})
	s.service.finishJobRuntime(s.job.JobID, &finishedAt, automationdomain.RunStatusQueuedToMain, nil)
	s.logger.Info("自动化任务已排入主会话",
		"run_id", s.runID,
		"session_key", s.sessionKey,
		"wake_mode", mode,
	)
}

func (s *jobExecutionStarter) queuedMainResult() *automationdomain.ExecutionResult {
	return &automationdomain.ExecutionResult{
		JobID:        s.job.JobID,
		RunID:        &s.runID,
		Status:       automationdomain.RunStatusQueuedToMain,
		SessionKey:   s.sessionKey,
		ScheduledFor: cloneTimePointer(&s.scheduledFor),
	}
}

func (s *jobExecutionStarter) startRuntimeSession() (*automationdomain.ExecutionResult, error) {
	if err := s.prepareRuntimeIdentity(); err != nil {
		return nil, err
	}
	result, handled, err := s.claimRuntime()
	if handled || err != nil {
		return result, err
	}
	if err = s.persistRunningRun(); err != nil {
		s.service.finishJobRuntime(s.job.JobID, nil, automationdomain.RunStatusFailed, errorPointer(err))
		return nil, err
	}
	if err = s.dispatchRuntime(); err != nil {
		return nil, err
	}
	return s.runningResult(), nil
}

func (s *jobExecutionStarter) prepareRuntimeIdentity() error {
	s.runID = s.service.idFactory("run")
	sessionKey, err := automationexec.ResolveSessionKey(s.job, &s.runID)
	if err != nil {
		s.failRuntime(err, s.service.nowFn())
		s.logger.Error("自动化任务解析执行会话键失败", "run_id", s.runID, "err", err)
		return err
	}
	s.sessionKey = sessionKey
	s.roundID = s.service.idFactory("round")
	return nil
}

func (s *jobExecutionStarter) claimRuntime() (*automationdomain.ExecutionResult, bool, error) {
	overlapping := s.buildRuntimeClaimPlan()
	if overlapping {
		s.logger.Warn("自动化任务已在运行中")
		result, err := s.service.recordSkippedOverlap(s.ctx, s.job, s.triggerKind, s.scheduledFor, true)
		return result, true, err
	}
	s.startedAt = s.service.nowFn()
	claimed, err := s.service.repository.ClaimScheduledTaskRuntime(s.ctx, automationstore.JobRuntimeClaimInput{
		JobID:         s.job.JobID,
		RunID:         s.runID,
		StartedAt:     s.startedAt,
		NextRunAt:     s.nextRunAt,
		OverlapPolicy: s.overlap,
		AllowDisabled: s.triggerKind == automationdomain.TriggerKindManual,
	})
	if err != nil {
		s.logger.Error("自动化任务领取执行权失败", "run_id", s.runID, "err", err)
		return nil, false, err
	}
	if !claimed {
		s.logger.Warn("自动化任务执行权已被其他调度器领取", "run_id", s.runID)
		result, resultErr := s.service.resultForExternallyClaimedJob(s.ctx, s.job, s.scheduledFor)
		return result, true, resultErr
	}
	s.registerRunningState()
	return nil, false, nil
}

func (s *jobExecutionStarter) buildRuntimeClaimPlan() bool {
	state := s.service.ensureJobState(s.job)
	s.service.mu.Lock()
	defer s.service.mu.Unlock()
	s.overlap = automationdomain.NormalizeOverlapPolicy(s.job.OverlapPolicy)
	if state.Running && s.overlap == automationdomain.OverlapPolicySkip {
		return true
	}
	s.nextRunAt = cloneTimePointer(state.NextRunAt)
	if isScheduledTrigger(s.triggerKind) {
		s.nextRunAt = s.service.nextRunAfterScheduledTrigger(s.job, s.triggerKind, s.scheduledFor)
	}
	return false
}

func isScheduledTrigger(triggerKind string) bool {
	return triggerKind == automationdomain.TriggerKindScheduled || triggerKind == automationdomain.TriggerKindMisfire
}

func (s *jobExecutionStarter) registerRunningState() {
	s.service.mu.Lock()
	defer s.service.mu.Unlock()
	state := s.service.jobStates[s.job.JobID]
	if state == nil {
		state = &automationexec.JobRuntimeState{Job: s.job}
		s.service.jobStates[s.job.JobID] = state
	}
	state.RunningCount++
	state.Running = true
	state.RunningRunID = s.runID
	state.RunningStartedAt = cloneTimePointer(&s.startedAt)
	state.NextRunAt = cloneTimePointer(s.nextRunAt)
}

func (s *jobExecutionStarter) persistRunningRun() error {
	if err := s.service.repository.InsertRunPending(s.ctx, automationstore.RunPendingInput{
		RunID:        s.runID,
		JobID:        s.job.JobID,
		OwnerUserID:  s.job.OwnerUserID,
		ScheduledFor: &s.scheduledFor,
		TriggerKind:  s.triggerKind,
		SessionKey:   s.sessionKey,
		RoundID:      s.roundID,
		DeliveryMode: strings.TrimSpace(s.job.Delivery.Mode),
		DeliveryTo:   deliveryTargetSummary(s.job.Delivery),
	}); err != nil {
		return err
	}
	return s.service.repository.MarkRunRunning(s.ctx, s.runID, s.service.nowFn())
}

func (s *jobExecutionStarter) dispatchRuntime() error {
	s.logger.Info("开始执行自动化任务",
		"run_id", s.runID,
		"round_id", s.roundID,
		"session_key", s.sessionKey,
	)
	sink := automationexec.NewExecutionSink("automation:" + s.runID)
	cleanup := s.service.bindSink(s.sessionKey, sink)
	dispatchJob := s.job
	dispatchJob.Instruction = buildScheduledTaskInstruction(s.job)
	err := s.service.dispatchJobToSession(s.ctx, dispatchJob, s.sessionKey, s.roundID, roomEventObserverForSink(sink))
	if err != nil {
		cleanup()
		sink.Close()
		s.failPendingRun(err)
		s.logger.Error("自动化任务下发失败",
			"run_id", s.runID,
			"round_id", s.roundID,
			"session_key", s.sessionKey,
			"err", err,
		)
		return err
	}
	go s.service.observeJobRun(s.job, s.runID, s.roundID, s.sessionKey, sink, cleanup)
	return nil
}

func (s *jobExecutionStarter) runningResult() *automationdomain.ExecutionResult {
	return &automationdomain.ExecutionResult{
		JobID:        s.job.JobID,
		RunID:        &s.runID,
		Status:       automationdomain.RunStatusRunning,
		SessionKey:   s.sessionKey,
		ScheduledFor: cloneTimePointer(&s.scheduledFor),
		RoundID:      &s.roundID,
		MessageCount: 0,
	}
}

func (s *jobExecutionStarter) failPendingRun(err error) {
	finishedAt := s.service.nowFn()
	message := err.Error()
	_ = s.service.repository.MarkRunFinished(context.Background(), automationstore.RunFinishInput{
		RunID:        s.runID,
		Status:       automationdomain.RunStatusFailed,
		FinishedAt:   finishedAt,
		ErrorMessage: &message,
	})
	s.service.finishJobRuntime(s.job.JobID, &finishedAt, automationdomain.RunStatusFailed, &message)
}

func (s *jobExecutionStarter) failRuntime(err error, finishedAt time.Time) {
	s.service.finishJobRuntime(s.job.JobID, &finishedAt, automationdomain.RunStatusFailed, errorPointer(err))
}

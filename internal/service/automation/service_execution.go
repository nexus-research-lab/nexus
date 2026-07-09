package automation

import (
	"context"
	"strings"
	"time"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/protocol"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

func (s *Service) startJobExecution(ctx context.Context, job automationdomain.CronJob, triggerKind string, scheduledFor time.Time) (*automationdomain.ExecutionResult, error) {
	logger := s.loggerFor(ctx).With(
		"job_id", job.JobID,
		"agent_id", job.AgentID,
		"trigger_kind", triggerKind,
	)
	if automationdomain.NormalizeExecutionKind(job.ExecutionKind) == automationdomain.ExecutionKindScript {
		return s.startScriptJobExecution(ctx, job, triggerKind, scheduledFor)
	}
	if err := s.ensureDirectTargetSupported(job.SessionTarget); err != nil {
		finishedAt := s.nowFn()
		s.finishJobRuntime(job.JobID, &finishedAt, automationdomain.RunStatusFailed, errorPointer(err))
		logger.Error("自动化任务目标校验失败", "err", err)
		return nil, err
	}

	if strings.TrimSpace(job.SessionTarget.Kind) == automationdomain.SessionTargetMain {
		runID := s.idFactory("run")
		sessionKey, err := automationexec.ResolveSessionKey(job, nil)
		if err != nil {
			finishedAt := s.nowFn()
			s.finishJobRuntime(job.JobID, &finishedAt, automationdomain.RunStatusFailed, errorPointer(err))
			logger.Error("自动化任务解析主会话键失败", "err", err)
			return nil, err
		}
		if err := s.repository.InsertRunPending(ctx, automationstore.RunPendingInput{
			RunID:        runID,
			JobID:        job.JobID,
			OwnerUserID:  job.OwnerUserID,
			ScheduledFor: &scheduledFor,
			TriggerKind:  triggerKind,
			SessionKey:   sessionKey,
			DeliveryMode: automationdomain.DeliveryModeNone,
		}); err != nil {
			finishedAt := s.nowFn()
			s.finishJobRuntime(job.JobID, &finishedAt, automationdomain.RunStatusFailed, errorPointer(err))
			return nil, err
		}
		eventID, err := s.enqueueMainSessionEvent(ctx, job, triggerKind)
		if err != nil {
			finishedAt := s.nowFn()
			message := err.Error()
			_ = s.repository.MarkRunFinished(context.Background(), automationstore.RunFinishInput{
				RunID:        runID,
				Status:       automationdomain.RunStatusFailed,
				FinishedAt:   finishedAt,
				ErrorMessage: &message,
			})
			s.finishJobRuntime(job.JobID, &finishedAt, automationdomain.RunStatusFailed, &message)
			return nil, err
		}
		mode := job.SessionTarget.WakeMode
		if mode == "" {
			mode = automationdomain.WakeModeNextHeartbeat
		}
		if _, err := s.WakeHeartbeat(ctx, job.AgentID, automationdomain.HeartbeatWakeInput{Mode: mode}); err != nil {
			_ = s.repository.MarkSystemEventStatus(context.Background(), eventID, "failed")
			finishedAt := s.nowFn()
			message := err.Error()
			_ = s.repository.MarkRunFinished(context.Background(), automationstore.RunFinishInput{
				RunID:        runID,
				Status:       automationdomain.RunStatusFailed,
				FinishedAt:   finishedAt,
				ErrorMessage: &message,
			})
			s.finishJobRuntime(job.JobID, &finishedAt, automationdomain.RunStatusFailed, &message)
			logger.Error("自动化任务唤醒主会话 heartbeat 失败", "err", err)
			return nil, err
		}
		finishedAt := s.nowFn()
		_ = s.repository.MarkRunFinished(context.Background(), automationstore.RunFinishInput{
			RunID:      runID,
			Status:     automationdomain.RunStatusQueuedToMain,
			FinishedAt: finishedAt,
		})
		s.finishJobRuntime(job.JobID, &finishedAt, automationdomain.RunStatusQueuedToMain, nil)
		logger.Info("自动化任务已排入主会话",
			"run_id", runID,
			"session_key", sessionKey,
			"wake_mode", mode,
		)
		return &automationdomain.ExecutionResult{
			JobID:        job.JobID,
			RunID:        &runID,
			Status:       automationdomain.RunStatusQueuedToMain,
			SessionKey:   sessionKey,
			ScheduledFor: cloneTimePointer(&scheduledFor),
		}, nil
	}

	runID := s.idFactory("run")
	sessionKey, err := automationexec.ResolveSessionKey(job, &runID)
	if err != nil {
		finishedAt := s.nowFn()
		s.finishJobRuntime(job.JobID, &finishedAt, automationdomain.RunStatusFailed, errorPointer(err))
		logger.Error("自动化任务解析执行会话键失败", "run_id", runID, "err", err)
		return nil, err
	}
	roundID := s.idFactory("round")

	state := s.ensureJobState(job)
	s.mu.Lock()
	overlapPolicy := automationdomain.NormalizeOverlapPolicy(job.OverlapPolicy)
	if state.Running && overlapPolicy == automationdomain.OverlapPolicySkip {
		s.mu.Unlock()
		logger.Warn("自动化任务已在运行中")
		return s.recordSkippedOverlap(ctx, job, triggerKind, scheduledFor, true)
	}
	nextRunAt := cloneTimePointer(state.NextRunAt)
	if triggerKind == "cron" {
		nextRunAt = s.computeJobNext(job, scheduledFor.UTC().Add(time.Second))
	}
	s.mu.Unlock()

	startedAt := s.nowFn()
	claimed, err := s.repository.ClaimCronJobRuntime(ctx, automationstore.JobRuntimeClaimInput{
		JobID:         job.JobID,
		RunID:         runID,
		StartedAt:     startedAt,
		NextRunAt:     nextRunAt,
		OverlapPolicy: overlapPolicy,
		AllowDisabled: triggerKind == "manual",
	})
	if err != nil {
		logger.Error("自动化任务领取执行权失败", "run_id", runID, "err", err)
		return nil, err
	}
	if !claimed {
		logger.Warn("自动化任务执行权已被其他调度器领取", "run_id", runID)
		return s.resultForExternallyClaimedJob(ctx, job, scheduledFor)
	}

	s.mu.Lock()
	state = s.jobStates[job.JobID]
	if state == nil {
		state = &automationexec.JobRuntimeState{Job: job}
		s.jobStates[job.JobID] = state
	}
	state.RunningCount++
	state.Running = true
	state.RunningRunID = runID
	state.RunningStartedAt = cloneTimePointer(&startedAt)
	state.NextRunAt = cloneTimePointer(nextRunAt)
	s.mu.Unlock()

	if err := s.repository.InsertRunPending(ctx, automationstore.RunPendingInput{
		RunID:        runID,
		JobID:        job.JobID,
		OwnerUserID:  job.OwnerUserID,
		ScheduledFor: &scheduledFor,
		TriggerKind:  triggerKind,
		SessionKey:   sessionKey,
		RoundID:      roundID,
		DeliveryMode: strings.TrimSpace(job.Delivery.Mode),
		DeliveryTo:   deliveryTargetSummary(job.Delivery),
	}); err != nil {
		s.finishJobRuntime(job.JobID, nil, automationdomain.RunStatusFailed, errorPointer(err))
		return nil, err
	}
	if err := s.repository.MarkRunRunning(ctx, runID, s.nowFn()); err != nil {
		s.finishJobRuntime(job.JobID, nil, automationdomain.RunStatusFailed, errorPointer(err))
		return nil, err
	}

	logger.Info("开始执行自动化任务",
		"run_id", runID,
		"round_id", roundID,
		"session_key", sessionKey,
	)
	sink := automationexec.NewExecutionSink("automation:" + runID)
	cleanup := s.bindSink(sessionKey, sink)
	roomObserver := roomEventObserverForSink(sink)
	dispatchJob := job
	dispatchJob.Instruction = buildCronInstruction(job)
	if err := s.dispatchJobToSession(ctx, dispatchJob, sessionKey, roundID, roomObserver); err != nil {
		cleanup()
		sink.Close()
		finishedAt := s.nowFn()
		message := err.Error()
		_ = s.repository.MarkRunFinished(context.Background(), automationstore.RunFinishInput{
			RunID:        runID,
			Status:       automationdomain.RunStatusFailed,
			FinishedAt:   finishedAt,
			ErrorMessage: &message,
		})
		s.finishJobRuntime(job.JobID, &finishedAt, automationdomain.RunStatusFailed, &message)
		logger.Error("自动化任务下发失败",
			"run_id", runID,
			"round_id", roundID,
			"session_key", sessionKey,
			"err", err,
		)
		return nil, err
	}

	go s.observeJobRun(job, runID, roundID, sessionKey, sink, cleanup)

	return &automationdomain.ExecutionResult{
		JobID:        job.JobID,
		RunID:        &runID,
		Status:       automationdomain.RunStatusRunning,
		SessionKey:   sessionKey,
		ScheduledFor: cloneTimePointer(&scheduledFor),
		RoundID:      &roundID,
		MessageCount: 0,
	}, nil
}

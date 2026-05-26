package automation

import (
	"context"
	"errors"
	"strings"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

func (s *Service) startJobExecution(ctx context.Context, job protocol.CronJob, triggerKind string, scheduledFor time.Time) (*protocol.ExecutionResult, error) {
	logger := s.loggerFor(ctx).With(
		"job_id", job.JobID,
		"agent_id", job.AgentID,
		"trigger_kind", triggerKind,
	)
	if protocol.NormalizeExecutionKind(job.ExecutionKind) == protocol.ExecutionKindScript {
		return s.startScriptJobExecution(ctx, job, triggerKind, scheduledFor)
	}
	if err := s.ensureDirectTargetSupported(job.SessionTarget); err != nil {
		finishedAt := s.nowFn()
		s.finishJobRuntime(job.JobID, &finishedAt, protocol.RunStatusFailed, errorPointer(err))
		logger.Error("自动化任务目标校验失败", "err", err)
		return nil, err
	}

	if strings.TrimSpace(job.SessionTarget.Kind) == protocol.SessionTargetMain {
		runID := s.idFactory("run")
		sessionKey, err := automationdomain.ResolveSessionKey(job, nil)
		if err != nil {
			finishedAt := s.nowFn()
			s.finishJobRuntime(job.JobID, &finishedAt, protocol.RunStatusFailed, errorPointer(err))
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
			DeliveryMode: protocol.DeliveryModeNone,
		}); err != nil {
			finishedAt := s.nowFn()
			s.finishJobRuntime(job.JobID, &finishedAt, protocol.RunStatusFailed, errorPointer(err))
			return nil, err
		}
		eventID, err := s.enqueueMainSessionEvent(ctx, job, triggerKind)
		if err != nil {
			finishedAt := s.nowFn()
			message := err.Error()
			_ = s.repository.MarkRunFinished(context.Background(), automationstore.RunFinishInput{
				RunID:        runID,
				Status:       protocol.RunStatusFailed,
				FinishedAt:   finishedAt,
				ErrorMessage: &message,
			})
			s.finishJobRuntime(job.JobID, &finishedAt, protocol.RunStatusFailed, &message)
			return nil, err
		}
		mode := job.SessionTarget.WakeMode
		if mode == "" {
			mode = protocol.WakeModeNextHeartbeat
		}
		if _, err := s.WakeHeartbeat(ctx, job.AgentID, protocol.HeartbeatWakeRequest{Mode: mode}); err != nil {
			_ = s.repository.MarkSystemEventStatus(context.Background(), eventID, "failed")
			finishedAt := s.nowFn()
			message := err.Error()
			_ = s.repository.MarkRunFinished(context.Background(), automationstore.RunFinishInput{
				RunID:        runID,
				Status:       protocol.RunStatusFailed,
				FinishedAt:   finishedAt,
				ErrorMessage: &message,
			})
			s.finishJobRuntime(job.JobID, &finishedAt, protocol.RunStatusFailed, &message)
			logger.Error("自动化任务唤醒主会话 heartbeat 失败", "err", err)
			return nil, err
		}
		finishedAt := s.nowFn()
		_ = s.repository.MarkRunFinished(context.Background(), automationstore.RunFinishInput{
			RunID:      runID,
			Status:     protocol.RunStatusQueuedToMain,
			FinishedAt: finishedAt,
		})
		s.finishJobRuntime(job.JobID, &finishedAt, protocol.RunStatusQueuedToMain, nil)
		logger.Info("自动化任务已排入主会话",
			"run_id", runID,
			"session_key", sessionKey,
			"wake_mode", mode,
		)
		return &protocol.ExecutionResult{
			JobID:        job.JobID,
			RunID:        &runID,
			Status:       protocol.RunStatusQueuedToMain,
			SessionKey:   sessionKey,
			ScheduledFor: cloneTimePointer(&scheduledFor),
		}, nil
	}

	runID := s.idFactory("run")
	sessionKey, err := automationdomain.ResolveSessionKey(job, &runID)
	if err != nil {
		finishedAt := s.nowFn()
		s.finishJobRuntime(job.JobID, &finishedAt, protocol.RunStatusFailed, errorPointer(err))
		logger.Error("自动化任务解析执行会话键失败", "run_id", runID, "err", err)
		return nil, err
	}
	roundID := s.idFactory("round")

	state := s.ensureJobState(job)
	s.mu.Lock()
	overlapPolicy := protocol.NormalizeOverlapPolicy(job.OverlapPolicy)
	if state.Running && overlapPolicy == protocol.OverlapPolicySkip {
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
		state = &automationdomain.JobRuntimeState{Job: job}
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
		s.finishJobRuntime(job.JobID, nil, protocol.RunStatusFailed, errorPointer(err))
		return nil, err
	}
	if err := s.repository.MarkRunRunning(ctx, runID, s.nowFn()); err != nil {
		s.finishJobRuntime(job.JobID, nil, protocol.RunStatusFailed, errorPointer(err))
		return nil, err
	}

	logger.Info("开始执行自动化任务",
		"run_id", runID,
		"round_id", roundID,
		"session_key", sessionKey,
	)
	sink := automationdomain.NewExecutionSink("automation:" + runID)
	cleanup := s.bindSink(sessionKey, sink)
	roomObserver := roomEventObserverForSink(sink)
	if err := s.dispatchJobToSession(ctx, job, sessionKey, roundID, roomObserver); err != nil {
		cleanup()
		sink.Close()
		finishedAt := s.nowFn()
		message := err.Error()
		_ = s.repository.MarkRunFinished(context.Background(), automationstore.RunFinishInput{
			RunID:        runID,
			Status:       protocol.RunStatusFailed,
			FinishedAt:   finishedAt,
			ErrorMessage: &message,
		})
		s.finishJobRuntime(job.JobID, &finishedAt, protocol.RunStatusFailed, &message)
		logger.Error("自动化任务下发失败",
			"run_id", runID,
			"round_id", roundID,
			"session_key", sessionKey,
			"err", err,
		)
		return nil, err
	}

	go s.observeJobRun(job, runID, roundID, sessionKey, sink, cleanup)

	return &protocol.ExecutionResult{
		JobID:        job.JobID,
		RunID:        &runID,
		Status:       protocol.RunStatusRunning,
		SessionKey:   sessionKey,
		ScheduledFor: cloneTimePointer(&scheduledFor),
		RoundID:      &roundID,
		MessageCount: 0,
	}, nil
}

func roomEventObserverForSink(sink *automationdomain.ExecutionSink) roomsvc.RoomEventObserver {
	if sink == nil {
		return nil
	}
	return func(ctx context.Context, event protocol.EventMessage) {
		_ = sink.SendEvent(ctx, event)
	}
}

func (s *Service) resultForExternallyClaimedJob(
	ctx context.Context,
	job protocol.CronJob,
	scheduledFor time.Time,
) (*protocol.ExecutionResult, error) {
	current, err := s.repository.GetCronJob(ctx, "", strings.TrimSpace(job.JobID))
	if err != nil {
		return nil, err
	}
	message := "scheduled task execution was claimed by another scheduler"
	if current != nil {
		s.replaceJobRuntimeState(*current)
		if strings.TrimSpace(current.RunningRunID) != "" {
			runID := strings.TrimSpace(current.RunningRunID)
			return &protocol.ExecutionResult{
				JobID:        job.JobID,
				RunID:        &runID,
				Status:       protocol.RunStatusRunning,
				ScheduledFor: cloneTimePointer(&scheduledFor),
				ErrorMessage: &message,
			}, nil
		}
		if !current.Enabled {
			disabledMessage := "scheduled task is disabled"
			return &protocol.ExecutionResult{
				JobID:        job.JobID,
				Status:       protocol.RunStatusSkipped,
				ScheduledFor: cloneTimePointer(&scheduledFor),
				ErrorMessage: &disabledMessage,
			}, nil
		}
	}
	return &protocol.ExecutionResult{
		JobID:        job.JobID,
		Status:       protocol.RunStatusRunning,
		ScheduledFor: cloneTimePointer(&scheduledFor),
		ErrorMessage: &message,
	}, nil
}

func (s *Service) observeJobRun(
	job protocol.CronJob,
	runID string,
	roundID string,
	sessionKey string,
	sink *automationdomain.ExecutionSink,
	cleanup func(),
) {
	defer cleanup()
	defer sink.Close()

	jobCtx := backgroundContextForJobOwner(job)
	waitCtx, cancel := context.WithTimeout(context.Background(), automationdomain.WaitTimeout(0))
	defer cancel()
	observation := sink.WaitForRound(waitCtx, roundID)

	status := observation.Status
	if status == "" {
		status = protocol.RunStatusFailed
	}
	errorMessage := cloneStringPointer(observation.ErrorMessage)
	deliveryResult := jobDeliveryResult{Status: protocol.DeliveryStatusNotRequired}
	if status == protocol.RunStatusSucceeded {
		deliveryResult = s.deliverJobObservation(jobCtx, job, sessionKey, observation)
	}
	deliveryStatus := deliveryResult.Status
	deliveryError := deliveryResult.Error
	deliveryTo := deliveryResult.deliveryTo(job.Delivery)
	finishedAt := s.nowFn()
	deliveredAt := deliveredAtForStatus(deliveryStatus, finishedAt)
	deliveryAttemptsAfter := 0
	if deliveryAttempted(deliveryStatus) {
		deliveryAttemptsAfter = 1
	}
	nextDeliveryAttemptAt, deliveryDeadLetterAt := deliveryRetrySchedule(deliveryStatus, deliveryAttemptsAfter, finishedAt)
	logger := s.loggerFor(jobCtx).With(
		"job_id", job.JobID,
		"agent_id", job.AgentID,
		"run_id", runID,
		"round_id", roundID,
	)
	if errorMessage != nil || deliveryError != nil {
		logError := ""
		if errorMessage != nil {
			logError = *errorMessage
		} else if deliveryError != nil {
			logError = *deliveryError
		}
		logger.Error("自动化任务执行结束",
			"status", status,
			"delivery_status", deliveryStatus,
			"message_count", observation.MessageCount,
			"session_id", anyStringPointer(observation.SessionID),
			"err", logError,
		)
	} else {
		logger.Info("自动化任务执行结束",
			"status", status,
			"delivery_status", deliveryStatus,
			"message_count", observation.MessageCount,
			"session_id", anyStringPointer(observation.SessionID),
		)
	}
	resultSummary := stringPointer(firstNonEmpty(observation.ResultText, observation.AssistantText))
	assistantText := stringPointer(observation.AssistantText)
	resultText := stringPointer(observation.ResultText)
	artifactPath := s.writeRunArtifact(jobCtx, job, runID, roundID, sessionKey, finishedAt, status, observation, errorMessage, deliveryStatus, deliveryError, deliveryTo)
	finished, finishErr := s.repository.MarkRunFinishedIfActive(context.Background(), automationstore.RunFinishInput{
		RunID:                 runID,
		Status:                status,
		FinishedAt:            finishedAt,
		ErrorMessage:          errorMessage,
		SessionID:             observation.SessionID,
		MessageCount:          observation.MessageCount,
		ResultSummary:         resultSummary,
		AssistantText:         assistantText,
		ResultText:            resultText,
		ArtifactPath:          artifactPath,
		DeliveryTo:            deliveryTo,
		DeliveryStatus:        deliveryStatus,
		DeliveryError:         deliveryError,
		DeliveredAt:           deliveredAt,
		DeliveryAttempted:     deliveryAttempted(deliveryStatus),
		DeliveryNextAttemptAt: nextDeliveryAttemptAt,
		DeliveryDeadLetterAt:  deliveryDeadLetterAt,
	})
	if finishErr != nil {
		logger.Warn("自动化任务结束结果写入失败",
			"status", status,
			"err", finishErr,
		)
		return
	}
	if !finished {
		logger.Warn("自动化任务结束结果已忽略，run 不再处于活动状态",
			"status", status,
		)
		return
	}
	s.finishJobRuntime(job.JobID, &finishedAt, status, errorMessage, deliveryStatus)
}

func (s *Service) recordSkippedOverlap(
	ctx context.Context,
	job protocol.CronJob,
	triggerKind string,
	scheduledFor time.Time,
	persistRuntime bool,
) (*protocol.ExecutionResult, error) {
	runID := s.idFactory("run")
	message := "previous run is still running; overlap_policy=skip"
	if err := s.repository.InsertRunPending(ctx, automationstore.RunPendingInput{
		RunID:        runID,
		JobID:        job.JobID,
		OwnerUserID:  job.OwnerUserID,
		ScheduledFor: &scheduledFor,
		TriggerKind:  triggerKind,
		DeliveryMode: strings.TrimSpace(job.Delivery.Mode),
		DeliveryTo:   deliveryTargetSummary(job.Delivery),
		Status:       protocol.RunStatusSkipped,
	}); err != nil {
		return nil, err
	}
	finishedAt := s.nowFn()
	_ = s.repository.MarkRunFinished(context.Background(), automationstore.RunFinishInput{
		RunID:        runID,
		Status:       protocol.RunStatusSkipped,
		FinishedAt:   finishedAt,
		ErrorMessage: &message,
	})
	if triggerKind == "cron" {
		if persistRuntime {
			s.advanceJobRuntimeAfterTrigger(job.JobID, scheduledFor)
		} else {
			s.advanceJobRuntimeAfterExternalClaim(job.JobID, scheduledFor)
		}
	}
	return &protocol.ExecutionResult{
		JobID:        job.JobID,
		RunID:        &runID,
		Status:       protocol.RunStatusSkipped,
		ScheduledFor: cloneTimePointer(&scheduledFor),
		ErrorMessage: &message,
	}, nil
}

func (s *Service) bindSink(sessionKey string, sink *automationdomain.ExecutionSink) func() {
	if s.permission == nil {
		return func() {}
	}
	s.permission.BindSession(sessionKey, sink, sink.Key(), false)
	return func() {
		s.permission.UnbindSession(sessionKey, sink)
	}
}

func (s *Service) dispatchToSession(ctx context.Context, sessionKey string, roundID string, agentID string, instruction string) error {
	return s.dispatchJobToSession(ctx, protocol.CronJob{
		AgentID:     agentID,
		Instruction: instruction,
	}, sessionKey, roundID, nil)
}

func (s *Service) dispatchJobToSession(
	ctx context.Context,
	job protocol.CronJob,
	sessionKey string,
	roundID string,
	eventObserver roomsvc.RoomEventObserver,
) error {
	parsed := protocol.ParseSessionKey(sessionKey)
	jobCtx := contextForJobOwner(ctx, job)
	permissionHandler := s.scheduledTaskPermissionHandler(jobCtx, job)
	if parsed.Kind == protocol.SessionKeyKindRoom {
		if s.room == nil {
			return errors.New("shared room session automation 暂不支持")
		}
		return s.room.HandleChat(jobCtx, roomsvc.ChatRequest{
			SessionKey:        sessionKey,
			ConversationID:    parsed.ConversationID,
			Content:           job.Instruction,
			RoundID:           roundID,
			ReqID:             roundID,
			PermissionMode:    sdkpermission.ModeDefault,
			PermissionHandler: permissionHandler,
			EventObserver:     eventObserver,
		})
	}
	if s.dm == nil {
		return errors.New("automation dm runner is not configured")
	}
	return s.dm.HandleChat(jobCtx, dmsvc.Request{
		SessionKey:        sessionKey,
		AgentID:           firstNonEmpty(job.AgentID, parsed.AgentID),
		Content:           job.Instruction,
		RoundID:           roundID,
		ReqID:             roundID,
		PermissionMode:    sdkpermission.ModeDefault,
		PermissionHandler: permissionHandler,
	})
}

func (s *Service) enqueueMainSessionEvent(ctx context.Context, job protocol.CronJob, triggerKind string) (string, error) {
	eventID := s.idFactory("evt")
	if err := s.repository.InsertSystemEvent(
		ctx,
		eventID,
		"cron.trigger",
		"cron",
		job.AgentID,
		map[string]any{
			"agent_id":            job.AgentID,
			"job_id":              job.JobID,
			"text":                job.Instruction,
			"trigger_kind":        triggerKind,
			"session_target_kind": job.SessionTarget.Kind,
		},
	); err != nil {
		return "", err
	}
	return eventID, nil
}

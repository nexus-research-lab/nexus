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
)

func (s *Service) bootstrapRuntime(ctx context.Context) error {
	jobs, err := s.repository.ListCronJobs(ctx, "")
	if err != nil {
		return err
	}
	for _, item := range jobs {
		s.ensureJobState(item)
	}

	configs, err := s.repository.ListEnabledHeartbeatStates(ctx)
	if err != nil {
		return err
	}
	for _, item := range configs {
		if _, stateErr := s.ensureHeartbeatState(ctx, item.AgentID); stateErr != nil {
			return stateErr
		}
	}
	return nil
}

func (s *Service) runLoop(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runDueOnce()
		}
	}
}

func (s *Service) runDueOnce() {
	now := s.nowFn()

	dueJobs := make([]protocol.CronJob, 0)
	dueHeartbeats := make([]string, 0)

	s.mu.Lock()
	for _, state := range s.jobStates {
		if state == nil || !state.Job.Enabled || state.Running || state.NextRunAt == nil {
			continue
		}
		if !state.NextRunAt.After(now) {
			dueJobs = append(dueJobs, state.Job)
		}
	}
	for agentID, state := range s.heartbeatState {
		if state == nil || state.Running {
			continue
		}
		if !state.Config.Enabled {
			continue
		}
		if s.hasImmediateWakeRequestLocked(agentID) {
			dueHeartbeats = append(dueHeartbeats, agentID)
			continue
		}
		if state.NextRunAt == nil || state.NextRunAt.After(now) {
			continue
		}
		dueHeartbeats = append(dueHeartbeats, agentID)
	}
	s.mu.Unlock()

	for _, item := range dueJobs {
		jobValue := item
		go func() {
			if _, err := s.startJobExecution(context.Background(), jobValue, "cron", now); err != nil {
				s.loggerFor(context.Background()).Error("定时任务触发失败",
					"job_id", jobValue.JobID,
					"agent_id", jobValue.AgentID,
					"trigger_kind", "cron",
					"err", err,
				)
				s.finishJobRuntime(jobValue.JobID, &now, false)
			}
		}()
	}
	for _, agentID := range dueHeartbeats {
		go s.dispatchHeartbeat(agentID, "heartbeat")
	}
}

func (s *Service) startJobExecution(ctx context.Context, job protocol.CronJob, triggerKind string, scheduledFor time.Time) (*protocol.ExecutionResult, error) {
	logger := s.loggerFor(ctx).With(
		"job_id", job.JobID,
		"agent_id", job.AgentID,
		"trigger_kind", triggerKind,
	)
	if err := s.ensureDirectTargetSupported(job.SessionTarget); err != nil {
		logger.Error("自动化任务目标校验失败", "err", err)
		return nil, err
	}
	if strings.TrimSpace(job.SessionTarget.Kind) == protocol.SessionTargetMain {
		eventID, err := s.enqueueMainSessionEvent(ctx, job, triggerKind)
		if err != nil {
			return nil, err
		}
		mode := job.SessionTarget.WakeMode
		if mode == "" {
			mode = protocol.WakeModeNextHeartbeat
		}
		if _, err := s.WakeHeartbeat(ctx, job.AgentID, protocol.HeartbeatWakeRequest{Mode: mode}); err != nil {
			_ = s.repository.MarkSystemEventStatus(context.Background(), eventID, "failed")
			logger.Error("自动化任务唤醒主会话 heartbeat 失败", "err", err)
			return nil, err
		}
		sessionKey, err := automationdomain.ResolveSessionKey(job, nil)
		if err != nil {
			logger.Error("自动化任务解析主会话键失败", "err", err)
			return nil, err
		}
		logger.Info("自动化任务已排入主会话",
			"session_key", sessionKey,
			"wake_mode", mode,
		)
		return &protocol.ExecutionResult{
			JobID:        job.JobID,
			Status:       protocol.RunStatusQueuedToMain,
			SessionKey:   sessionKey,
			ScheduledFor: cloneTimePointer(&scheduledFor),
		}, nil
	}

	state := s.ensureJobState(job)
	s.mu.Lock()
	if state.Running {
		s.mu.Unlock()
		logger.Warn("自动化任务已在运行中")
		return nil, errors.New("任务正在运行中")
	}
	state.Running = true
	s.mu.Unlock()

	runID := s.idFactory("run")
	sessionKey, err := automationdomain.ResolveSessionKey(job, &runID)
	if err != nil {
		s.finishJobRuntime(job.JobID, nil, false)
		logger.Error("自动化任务解析执行会话键失败", "run_id", runID, "err", err)
		return nil, err
	}
	if err := s.repository.InsertRunPending(ctx, runID, job.JobID, &scheduledFor); err != nil {
		s.finishJobRuntime(job.JobID, nil, false)
		return nil, err
	}
	if err := s.repository.MarkRunRunning(ctx, runID, s.nowFn()); err != nil {
		s.finishJobRuntime(job.JobID, nil, false)
		return nil, err
	}

	roundID := s.idFactory("round")
	logger.Info("开始执行自动化任务",
		"run_id", runID,
		"round_id", roundID,
		"session_key", sessionKey,
	)
	sink := automationdomain.NewExecutionSink("automation:" + runID)
	cleanup := s.bindSink(sessionKey, sink)
	if err := s.dispatchToSession(ctx, sessionKey, roundID, job.AgentID, job.Instruction); err != nil {
		cleanup()
		sink.Close()
		finishedAt := s.nowFn()
		message := err.Error()
		_ = s.repository.MarkRunFinished(context.Background(), runID, protocol.RunStatusFailed, finishedAt, &message)
		s.finishJobRuntime(job.JobID, &finishedAt, false)
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

	waitCtx, cancel := context.WithTimeout(context.Background(), automationdomain.WaitTimeout(0))
	defer cancel()
	observation := sink.WaitForRound(waitCtx, roundID)

	finishedAt := s.nowFn()
	status := observation.Status
	if status == "" {
		status = protocol.RunStatusFailed
	}
	errorMessage := cloneStringPointer(observation.ErrorMessage)
	if status == protocol.RunStatusSucceeded {
		if deliveryError := s.deliverJobObservation(job, sessionKey, observation); deliveryError != nil {
			status = protocol.RunStatusFailed
			errorMessage = deliveryError
		}
	}
	logger := s.loggerFor(context.Background()).With(
		"job_id", job.JobID,
		"agent_id", job.AgentID,
		"run_id", runID,
		"round_id", roundID,
	)
	if errorMessage != nil {
		logger.Error("自动化任务执行结束",
			"status", status,
			"message_count", observation.MessageCount,
			"session_id", anyStringPointer(observation.SessionID),
			"err", *errorMessage,
		)
	} else {
		logger.Info("自动化任务执行结束",
			"status", status,
			"message_count", observation.MessageCount,
			"session_id", anyStringPointer(observation.SessionID),
		)
	}
	_ = s.repository.MarkRunFinished(context.Background(), runID, status, finishedAt, errorMessage)
	s.finishJobRuntime(job.JobID, &finishedAt, status == protocol.RunStatusSucceeded)
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
	parsed := protocol.ParseSessionKey(sessionKey)
	if parsed.Kind == protocol.SessionKeyKindRoom {
		if s.room == nil {
			return errors.New("shared room session automation 暂不支持")
		}
		return s.room.HandleChat(ctx, roomsvc.ChatRequest{
			SessionKey:     sessionKey,
			ConversationID: parsed.ConversationID,
			Content:        instruction,
			RoundID:        roundID,
			ReqID:          roundID,
		})
	}
	if s.dm == nil {
		return errors.New("automation dm runner is not configured")
	}
	return s.dm.HandleChat(ctx, dmsvc.Request{
		SessionKey: sessionKey,
		AgentID:    firstNonEmpty(agentID, parsed.AgentID),
		Content:    instruction,
		RoundID:    roundID,
		ReqID:      roundID,
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

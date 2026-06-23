package automation

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

// CreateTask 创建任务。
func (s *Service) CreateTask(ctx context.Context, input protocol.CreateJobInput) (*protocol.CronJob, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	normalized := input.Normalized()
	if err := normalized.Validate(); err != nil {
		return nil, err
	}
	if err := s.validateAgentAndTarget(ctx, normalized.AgentID, normalized.SessionTarget); err != nil {
		return nil, err
	}
	ownerUserID, err := s.resolveTaskOwnerUserID(ctx, normalized.AgentID)
	if err != nil {
		return nil, err
	}

	job := protocol.CronJob{
		JobID:         s.idFactory("cron"),
		OwnerUserID:   ownerUserID,
		Name:          normalized.Name,
		AgentID:       normalized.AgentID,
		Schedule:      normalized.Schedule,
		Instruction:   normalized.Instruction,
		ExecutionKind: normalized.ExecutionKind,
		SessionTarget: normalized.SessionTarget,
		Delivery:      normalized.Delivery,
		Source:        normalized.Source,
		OverlapPolicy: normalized.OverlapPolicy,
		Enabled:       normalized.Enabled,
	}
	created, err := s.repository.UpsertCronJob(ctx, job)
	if err != nil {
		return nil, err
	}
	state := s.ensureJobState(*created)
	s.persistJobRuntime(ctx, jobRuntimeUpdateFromState(created.JobID, state))
	s.recordTaskEvent(ctx, protocol.TaskEventActionCreate, *created, "", taskEventJobSnapshot(*created))
	result := *created
	result.NextRunAt = cloneTimePointer(state.NextRunAt)
	result.LastRunAt = cloneTimePointer(state.LastRunAt)
	result.Running = state.Running
	result.RunningRunID = strings.TrimSpace(state.RunningRunID)
	result.RunningStartedAt = cloneTimePointer(state.RunningStartedAt)
	result.LastRunStatus = strings.TrimSpace(state.LastRunStatus)
	result.FailureStreak = state.FailureStreak
	result.LastError = cloneStringPointer(state.LastError)
	result.LastDeliveryStatus = strings.TrimSpace(state.LastDeliveryStatus)
	return &result, nil
}

// UpdateTask 更新任务。
func (s *Service) UpdateTask(ctx context.Context, jobID string, input protocol.UpdateJobInput) (*protocol.CronJob, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	current, err := s.repository.GetCronJob(ctx, ownerUserID, strings.TrimSpace(jobID))
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, protocol.ErrJobNotFound
	}

	next := *current
	if input.Name != nil {
		next.Name = strings.TrimSpace(*input.Name)
	}
	if input.Schedule != nil {
		next.Schedule = input.Schedule.Normalized()
	}
	if input.Instruction != nil {
		next.Instruction = strings.TrimSpace(*input.Instruction)
	}
	if input.ExecutionKind != nil {
		next.ExecutionKind = protocol.NormalizeExecutionKind(*input.ExecutionKind)
	}
	if input.SessionTarget != nil {
		next.SessionTarget = input.SessionTarget.Normalized()
	}
	if input.Delivery != nil {
		next.Delivery = input.Delivery.Normalized()
	}
	if input.Source != nil {
		next.Source = input.Source.Normalized()
	}
	if input.OverlapPolicy != nil {
		next.OverlapPolicy = protocol.NormalizeOverlapPolicy(*input.OverlapPolicy)
	}
	if input.Enabled != nil {
		next.Enabled = *input.Enabled
	}

	createLike := protocol.CreateJobInput{
		Name:          next.Name,
		AgentID:       next.AgentID,
		Schedule:      next.Schedule,
		Instruction:   next.Instruction,
		ExecutionKind: next.ExecutionKind,
		SessionTarget: next.SessionTarget,
		Delivery:      next.Delivery,
		Source:        next.Source,
		OverlapPolicy: next.OverlapPolicy,
		Enabled:       next.Enabled,
	}
	if err = createLike.Validate(); err != nil {
		return nil, err
	}
	if err = s.validateAgentAndTarget(ctx, next.AgentID, next.SessionTarget); err != nil {
		return nil, err
	}

	updated, err := s.repository.UpsertCronJob(ctx, next)
	if err != nil {
		return nil, err
	}
	state := s.ensureJobState(*updated)
	s.persistJobRuntime(ctx, jobRuntimeUpdateFromState(updated.JobID, state))
	eventRunID := updateTaskEventRunID(input, *current)
	s.recordTaskEvent(ctx, updateTaskEventAction(input, *updated), *updated, eventRunID, updateTaskEventDetail(input, *current, *updated))
	result := *updated
	result.NextRunAt = cloneTimePointer(state.NextRunAt)
	result.LastRunAt = cloneTimePointer(state.LastRunAt)
	result.Running = state.Running
	result.RunningRunID = strings.TrimSpace(state.RunningRunID)
	result.RunningStartedAt = cloneTimePointer(state.RunningStartedAt)
	result.LastRunStatus = strings.TrimSpace(state.LastRunStatus)
	result.FailureStreak = state.FailureStreak
	result.LastError = cloneStringPointer(state.LastError)
	result.LastDeliveryStatus = strings.TrimSpace(state.LastDeliveryStatus)
	return &result, nil
}

// UpdateTaskStatus 切换任务启停。
func (s *Service) UpdateTaskStatus(ctx context.Context, jobID string, enabled bool) (*protocol.CronJob, error) {
	return s.UpdateTask(ctx, jobID, protocol.UpdateJobInput{Enabled: &enabled})
}

// DeleteTask 删除任务，并返回是否取消了删除时仍活跃的 run。
func (s *Service) DeleteTask(ctx context.Context, jobID string) (*protocol.DeleteJobResult, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	current, err := s.repository.GetCronJob(ctx, ownerUserID, strings.TrimSpace(jobID))
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, protocol.ErrJobNotFound
	}
	cancelledRunID, cancelledRun, err := s.cancelDeletedTaskActiveRun(ctx, *current)
	if err != nil {
		return nil, err
	}
	deadLetteredDeliveryRunIDs, err := s.deadLetterDeletedTaskPendingDeliveries(ctx, *current)
	if err != nil {
		return nil, err
	}
	if err = s.repository.DeleteCronJob(ctx, ownerUserID, current.JobID); err != nil {
		return nil, err
	}
	if err = s.cleanupIsolatedAutomationSessions(ctx, *current); err != nil {
		return nil, err
	}
	s.mu.Lock()
	delete(s.jobStates, current.JobID)
	s.mu.Unlock()
	s.recordTaskEvent(ctx, protocol.TaskEventActionDelete, *current, cancelledRunID, deleteTaskEventDetail(*current, cancelledRunID, cancelledRun, deadLetteredDeliveryRunIDs))
	result := &protocol.DeleteJobResult{
		JobID:              current.JobID,
		AgentID:            current.AgentID,
		Deleted:            true,
		ActiveRunID:        cancelledRunID,
		CancelledActiveRun: cancelledRun,
	}
	if cancelledRun {
		result.CancelledRunID = cancelledRunID
	}
	return result, nil
}

func (s *Service) deadLetterDeletedTaskPendingDeliveries(ctx context.Context, job protocol.CronJob) ([]string, error) {
	runs, err := s.repository.ListRunsByJob(ctx, strings.TrimSpace(job.OwnerUserID), strings.TrimSpace(job.JobID))
	if err != nil {
		return nil, err
	}
	now := s.nowFn()
	message := "scheduled task was deleted before delivery could be retried"
	deadLettered := make([]string, 0)
	for _, run := range runs {
		if !shouldDeadLetterDeletedTaskDelivery(run) {
			continue
		}
		if err = s.repository.MarkRunDelivery(ctx, automationstore.RunDeliveryUpdateInput{
			RunID:                run.RunID,
			DeliveryStatus:       protocol.DeliveryStatusFailed,
			DeliveryError:        &message,
			DeliveryDeadLetterAt: &now,
		}); err != nil {
			return nil, err
		}
		deadLettered = append(deadLettered, strings.TrimSpace(run.RunID))
	}
	return deadLettered, nil
}

func shouldDeadLetterDeletedTaskDelivery(run protocol.CronRun) bool {
	if strings.TrimSpace(run.RunID) == "" || run.DeliveryDeadLetterAt != nil {
		return false
	}
	if strings.TrimSpace(run.Status) == protocol.RunStatusPending ||
		strings.TrimSpace(run.Status) == protocol.RunStatusRunning {
		return false
	}
	return deriveCronRunDeliveryStatus(run) == protocol.DeliveryStatusFailed
}

func (s *Service) cancelDeletedTaskActiveRun(ctx context.Context, job protocol.CronJob) (string, bool, error) {
	runID := strings.TrimSpace(job.RunningRunID)
	if runID == "" {
		return "", false, nil
	}
	message := "scheduled task was deleted while this run was active"
	if err := s.interruptActiveRunExecution(ctx, job, runID, message); err != nil {
		return runID, false, err
	}
	finishedAt := s.nowFn()
	cancelled, err := s.repository.MarkRunFinishedIfActive(ctx, automationstore.RunFinishInput{
		RunID:        runID,
		Status:       protocol.RunStatusCancelled,
		FinishedAt:   finishedAt,
		ErrorMessage: &message,
	})
	if err != nil {
		return runID, false, err
	}
	return runID, cancelled, nil
}

func (s *Service) interruptActiveRunExecution(ctx context.Context, job protocol.CronJob, runID string, message string) error {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil
	}
	run, err := s.repository.GetRun(ctx, strings.TrimSpace(job.OwnerUserID), strings.TrimSpace(job.JobID), runID)
	if errors.Is(err, sql.ErrNoRows) || run == nil {
		return nil
	}
	if err != nil {
		return err
	}
	sessionKey := strings.TrimSpace(run.SessionKey)
	if sessionKey == "" {
		return nil
	}
	runCtx := contextForJobOwner(ctx, job)
	parsed := protocol.ParseSessionKey(sessionKey)
	switch parsed.Kind {
	case protocol.SessionKeyKindRoom:
		runner, ok := s.room.(roomInterruptRunner)
		if !ok || runner == nil {
			s.cancelPendingRunPermissions(sessionKey, message)
			return nil
		}
		if err = runner.HandleInterrupt(runCtx, roomsvc.InterruptRequest{SessionKey: sessionKey}); err != nil {
			return err
		}
	case protocol.SessionKeyKindAgent:
		runner, ok := s.dm.(dmInterruptRunner)
		if !ok || runner == nil {
			s.cancelPendingRunPermissions(sessionKey, message)
			return nil
		}
		if err = runner.HandleInterrupt(runCtx, dmsvc.InterruptRequest{SessionKey: sessionKey, RoundID: strings.TrimSpace(run.RoundID)}); err != nil {
			return err
		}
	default:
		s.cancelPendingRunPermissions(sessionKey, message)
		return nil
	}
	s.cancelPendingRunPermissions(sessionKey, message)
	return nil
}

func (s *Service) cancelPendingRunPermissions(sessionKey string, message string) {
	if s.permission == nil {
		return
	}
	s.permission.CancelRequestsForSession(sessionKey, strings.TrimSpace(message))
}

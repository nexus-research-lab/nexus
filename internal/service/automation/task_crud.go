package automation

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

// expireTask 只阻止后续触发，不中断已经开始的 run。
func (s *Service) expireTask(ctx context.Context, job automationdomain.ScheduledTask, expiredAt time.Time) error {
	if !job.Enabled {
		return nil
	}
	updated := job
	updated.Enabled = false
	persisted, err := s.repository.UpsertScheduledTask(ctx, updated)
	if err != nil {
		return err
	}

	s.mu.Lock()
	state := s.jobStates[job.JobID]
	var runtimeUpdate *automationstore.JobRuntimeUpdateInput
	if state != nil {
		state.Job = *persisted
		state.NextRunAt = nil
		snapshot := jobRuntimeUpdateFromState(job.JobID, state)
		runtimeUpdate = &snapshot
	}
	s.mu.Unlock()

	if runtimeUpdate != nil {
		s.persistJobRuntime(ctx, *runtimeUpdate)
	}
	s.recordTaskEvent(ctx, automationdomain.TaskEventActionExpire, *persisted, "", map[string]any{
		"expired_at": expiredAt.UTC(),
		"expires_at": cloneTimePointer(persisted.ExpiresAt),
	})
	return nil
}

// CreateTask 创建任务。
func (s *Service) CreateTask(ctx context.Context, input automationdomain.CreateJobInput) (*automationdomain.ScheduledTask, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	normalized := input.Normalized()
	if err := normalized.Validate(); err != nil {
		return nil, err
	}
	if err := s.validateTaskExpiration(normalized.ExpiresAt); err != nil {
		return nil, err
	}
	if err := s.validateAgentAndTarget(ctx, normalized.AgentID, normalized.SessionTarget); err != nil {
		return nil, err
	}
	ownerUserID, err := s.resolveTaskOwnerUserID(ctx, normalized.AgentID)
	if err != nil {
		return nil, err
	}
	if err = s.validateTaskCapacity(ctx, ownerUserID, normalized.Enabled); err != nil {
		return nil, err
	}

	job := automationdomain.ScheduledTask{
		JobID:         s.idFactory("task"),
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
		ExpiresAt:     cloneTimePointer(normalized.ExpiresAt),
		Enabled:       normalized.Enabled,
	}
	created, err := s.repository.UpsertScheduledTask(ctx, job)
	if err != nil {
		return nil, err
	}
	state := s.ensureJobState(*created)
	s.persistJobRuntime(ctx, jobRuntimeUpdateFromState(created.JobID, state))
	s.recordTaskEvent(ctx, automationdomain.TaskEventActionCreate, *created, "", taskEventJobSnapshot(*created))
	result := scheduledTaskWithRuntime(*created, state)
	return &result, nil
}

// UpdateTask 更新任务。
func (s *Service) UpdateTask(ctx context.Context, jobID string, input automationdomain.UpdateJobInput) (*automationdomain.ScheduledTask, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	current, err := s.loadRequiredScheduledTask(ctx, jobID)
	if err != nil {
		return nil, err
	}
	next, err := s.applyTaskUpdate(*current, input)
	if err != nil {
		return nil, err
	}
	if err = s.validateTaskUpdate(ctx, *current, next); err != nil {
		return nil, err
	}
	updated, err := s.repository.UpsertScheduledTask(ctx, next)
	if err != nil {
		return nil, err
	}
	state := s.ensureJobState(*updated)
	s.persistJobRuntime(ctx, jobRuntimeUpdateFromState(updated.JobID, state))
	eventRunID := updateTaskEventRunID(input, *current)
	s.recordTaskEvent(ctx, updateTaskEventAction(input, *updated), *updated, eventRunID, updateTaskEventDetail(input, *current, *updated))
	result := scheduledTaskWithRuntime(*updated, state)
	return &result, nil
}

func (s *Service) loadRequiredScheduledTask(ctx context.Context, jobID string) (*automationdomain.ScheduledTask, error) {
	ownerUserID, _ := scopedOwnerUserID(ctx)
	task, err := s.repository.GetScheduledTask(ctx, ownerUserID, strings.TrimSpace(jobID))
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, automationdomain.ErrJobNotFound
	}
	return task, nil
}

func (s *Service) applyTaskUpdate(
	current automationdomain.ScheduledTask,
	input automationdomain.UpdateJobInput,
) (automationdomain.ScheduledTask, error) {
	next := current
	applyOptionalValue(input.Name, func(value string) { next.Name = strings.TrimSpace(value) })
	applyOptionalValue(input.Schedule, func(value automationdomain.Schedule) { next.Schedule = value.Normalized() })
	applyOptionalValue(input.Instruction, func(value string) { next.Instruction = strings.TrimSpace(value) })
	applyOptionalValue(input.ExecutionKind, func(value string) { next.ExecutionKind = automationdomain.NormalizeExecutionKind(value) })
	applyOptionalValue(input.SessionTarget, func(value automationdomain.SessionTarget) { next.SessionTarget = value.Normalized() })
	applyOptionalValue(input.Delivery, func(value automationdomain.DeliveryTarget) { next.Delivery = value.Normalized() })
	applyOptionalValue(input.Source, func(value automationdomain.Source) { next.Source = value.Normalized() })
	applyOptionalValue(input.OverlapPolicy, func(value string) { next.OverlapPolicy = automationdomain.NormalizeOverlapPolicy(value) })
	applyOptionalValue(input.Enabled, func(value bool) { next.Enabled = value })
	if err := s.applyTaskExpirationUpdate(&next, input); err != nil {
		return automationdomain.ScheduledTask{}, err
	}
	return next, nil
}

func applyOptionalValue[T any](value *T, apply func(T)) {
	if value != nil {
		apply(*value)
	}
}

func (s *Service) applyTaskExpirationUpdate(
	task *automationdomain.ScheduledTask,
	input automationdomain.UpdateJobInput,
) error {
	if input.ExpiresAt != nil && input.ClearExpiresAt {
		return errors.New("expires_at 和 clear_expires_at 不能同时设置")
	}
	if input.ClearExpiresAt {
		task.ExpiresAt = nil
		return nil
	}
	if input.ExpiresAt == nil {
		return nil
	}
	expiresAt := input.ExpiresAt.UTC()
	if err := s.validateTaskExpiration(&expiresAt); err != nil {
		return err
	}
	task.ExpiresAt = &expiresAt
	return nil
}

func (s *Service) validateTaskUpdate(
	ctx context.Context,
	current automationdomain.ScheduledTask,
	next automationdomain.ScheduledTask,
) error {
	if err := scheduledTaskCreateInput(next).Validate(); err != nil {
		return err
	}
	if err := s.validateAgentAndTarget(ctx, next.AgentID, next.SessionTarget); err != nil {
		return err
	}
	enabling := !current.Enabled && next.Enabled
	if enabling {
		if err := s.validateTaskExpiration(next.ExpiresAt); err != nil {
			return err
		}
	}
	return s.validateTaskCapacity(ctx, next.OwnerUserID, enabling)
}

func scheduledTaskCreateInput(task automationdomain.ScheduledTask) automationdomain.CreateJobInput {
	return automationdomain.CreateJobInput{
		Name:          task.Name,
		AgentID:       task.AgentID,
		Schedule:      task.Schedule,
		Instruction:   task.Instruction,
		ExecutionKind: task.ExecutionKind,
		SessionTarget: task.SessionTarget,
		Delivery:      task.Delivery,
		Source:        task.Source,
		OverlapPolicy: task.OverlapPolicy,
		ExpiresAt:     cloneTimePointer(task.ExpiresAt),
		Enabled:       task.Enabled,
	}
}

// UpdateTaskStatus 切换任务启停。
func (s *Service) UpdateTaskStatus(ctx context.Context, jobID string, enabled bool) (*automationdomain.ScheduledTask, error) {
	return s.UpdateTask(ctx, jobID, automationdomain.UpdateJobInput{Enabled: &enabled})
}

// DeleteTask 删除任务，并返回是否取消了删除时仍活跃的 run。
func (s *Service) DeleteTask(ctx context.Context, jobID string) (*automationdomain.DeleteJobResult, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	current, err := s.loadRequiredScheduledTask(ctx, jobID)
	if err != nil {
		return nil, err
	}
	cancelledRunID, cancelledRun, err := s.cancelDeletedTaskActiveRun(ctx, *current)
	if err != nil {
		return nil, err
	}
	deadLetteredDeliveryRunIDs, err := s.deadLetterDeletedTaskPendingDeliveries(ctx, *current)
	if err != nil {
		return nil, err
	}
	if err = s.repository.DeleteScheduledTask(ctx, current.OwnerUserID, current.JobID); err != nil {
		return nil, err
	}
	if err = s.cleanupIsolatedAutomationSessions(ctx, *current); err != nil {
		return nil, err
	}
	s.mu.Lock()
	delete(s.jobStates, current.JobID)
	s.mu.Unlock()
	s.recordTaskEvent(ctx, automationdomain.TaskEventActionDelete, *current, cancelledRunID, deleteTaskEventDetail(*current, cancelledRunID, cancelledRun, deadLetteredDeliveryRunIDs))
	result := &automationdomain.DeleteJobResult{
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

func (s *Service) deadLetterDeletedTaskPendingDeliveries(ctx context.Context, job automationdomain.ScheduledTask) ([]string, error) {
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
			DeliveryStatus:       automationdomain.DeliveryStatusFailed,
			DeliveryError:        &message,
			DeliveryDeadLetterAt: &now,
		}); err != nil {
			return nil, err
		}
		deadLettered = append(deadLettered, strings.TrimSpace(run.RunID))
	}
	return deadLettered, nil
}

func shouldDeadLetterDeletedTaskDelivery(run automationdomain.ScheduledTaskRun) bool {
	if strings.TrimSpace(run.RunID) == "" || run.DeliveryDeadLetterAt != nil {
		return false
	}
	if strings.TrimSpace(run.Status) == automationdomain.RunStatusPending ||
		strings.TrimSpace(run.Status) == automationdomain.RunStatusRunning {
		return false
	}
	return deriveTaskRunDeliveryStatus(run) == automationdomain.DeliveryStatusFailed
}

func (s *Service) cancelDeletedTaskActiveRun(ctx context.Context, job automationdomain.ScheduledTask) (string, bool, error) {
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
		Status:       automationdomain.RunStatusCancelled,
		FinishedAt:   finishedAt,
		ErrorMessage: &message,
	})
	if err != nil {
		return runID, false, err
	}
	return runID, cancelled, nil
}

func (s *Service) interruptActiveRunExecution(ctx context.Context, job automationdomain.ScheduledTask, runID string, message string) error {
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

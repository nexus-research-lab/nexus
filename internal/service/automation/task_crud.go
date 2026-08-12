// INPUT: 人类或 Agent actor 的任务 CRUD 请求及当前持久化任务。
// OUTPUT: 校验、持久化、运行态与审计同步后的任务结果。
// POS: scheduled task 配置写入与 script capability 的事务边界。
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
	roomrealtime "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
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
	s.taskControlMu.Lock()
	defer s.taskControlMu.Unlock()

	normalized := input.Normalized()
	if err := normalized.Validate(); err != nil {
		return nil, err
	}
	if err := rejectAgentScriptCreate(ctx, normalized); err != nil {
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
	deliveryCandidate := automationdomain.ScheduledTask{
		OwnerUserID: ownerUserID,
		AgentID:     normalized.AgentID,
		Delivery:    normalized.Delivery,
		Source:      normalized.Source,
	}
	if err = s.prepareTaskDeliveryMutation(ctx, &deliveryCandidate, &normalized.Source); err != nil {
		return nil, err
	}
	deliveryGrant := deliveryCandidate.DeliveryGrant
	intentDigest := ""
	if normalized.RequestID != "" {
		intentDigest = taskCreateIntentDigest(normalized)
		replayed, found, replayErr := s.repository.GetScheduledTaskCreateReplay(
			ctx,
			ownerUserID,
			normalized.RequestID,
			normalized.AgentID,
			intentDigest,
		)
		if replayErr != nil {
			return nil, replayErr
		}
		if found {
			state := s.ensureJobState(*replayed)
			result := s.scheduledTaskRuntimeSnapshot(*replayed, state)
			return &result, nil
		}
	}
	if err = s.validateTaskCapacity(ctx, ownerUserID, normalized.Enabled); err != nil {
		return nil, err
	}

	job := automationdomain.ScheduledTask{
		JobID:               s.idFactory("task"),
		OwnerUserID:         ownerUserID,
		Name:                normalized.Name,
		AgentID:             normalized.AgentID,
		Schedule:            normalized.Schedule,
		Instruction:         normalized.Instruction,
		ExecutionKind:       normalized.ExecutionKind,
		PermissionMode:      normalized.PermissionMode,
		SessionTarget:       normalized.SessionTarget,
		Delivery:            normalized.Delivery,
		SessionBindingState: automationdomain.TaskSessionBindingStateReady,
		Source:              normalized.Source,
		DeliveryGrant:       deliveryGrant,
		OverlapPolicy:       normalized.OverlapPolicy,
		ExpiresAt:           cloneTimePointer(normalized.ExpiresAt),
		Enabled:             normalized.Enabled,
	}
	snapshot, err := s.resolveInitialTaskPermissionSnapshot(ctx, job)
	if err != nil {
		return nil, err
	}
	job.PermissionMode = snapshot.Mode
	policy := s.buildTaskPermissionPolicyFromOptions(
		ctx,
		job,
		snapshot.AgentOptions,
		taskPermissionMutationIsDirectUser(ctx, job.Source.Kind),
		false,
	)
	job.PermissionPolicy = policy
	job.PermissionState = automationdomain.TaskPermissionStateReady
	var (
		created    *automationdomain.ScheduledTask
		createdNew = true
	)
	if normalized.RequestID != "" {
		created, createdNew, err = s.repository.CreateScheduledTaskIdempotent(
			ctx,
			job,
			normalized.RequestID,
			intentDigest,
		)
	} else {
		created, err = s.repository.UpsertScheduledTask(ctx, job)
	}
	if err != nil {
		return nil, err
	}
	state := s.ensureJobState(*created)
	s.persistJobRuntime(ctx, s.jobRuntimeUpdateSnapshot(created.JobID, state))
	if createdNew {
		s.recordTaskEvent(ctx, automationdomain.TaskEventActionCreate, *created, "", taskEventJobSnapshot(*created))
	}
	result := s.scheduledTaskRuntimeSnapshot(*created, state)
	return &result, nil
}

// UpdateTask 更新任务。
func (s *Service) UpdateTask(ctx context.Context, jobID string, input automationdomain.UpdateJobInput) (*automationdomain.ScheduledTask, error) {
	return s.updateTask(ctx, jobID, input, nil)
}

// UpdateTaskAtVersion 更新对话在读取阶段看到的精确版本。
func (s *Service) UpdateTaskAtVersion(
	ctx context.Context,
	jobID string,
	expectedVersion int64,
	input automationdomain.UpdateJobInput,
) (*automationdomain.ScheduledTask, error) {
	return s.updateTask(ctx, jobID, input, &expectedVersion)
}

func (s *Service) updateTask(
	ctx context.Context,
	jobID string,
	input automationdomain.UpdateJobInput,
	expectedVersion *int64,
) (*automationdomain.ScheduledTask, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	s.taskControlMu.Lock()
	defer s.taskControlMu.Unlock()

	current, err := s.loadRequiredScheduledTask(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if expectedVersion != nil &&
		(*expectedVersion < 1 || current.ConfigurationVersion != *expectedVersion) {
		return nil, automationdomain.ErrConfigurationVersionConflict
	}
	ensuredCurrent, err := s.ensureTaskPermissionPolicy(ctx, *current)
	if err != nil {
		return nil, err
	}
	current = &ensuredCurrent
	next, err := s.applyTaskUpdate(*current, input)
	if err != nil {
		return nil, err
	}
	if err = rejectAgentScriptControl(ctx, *current, next); err != nil {
		return nil, err
	}
	deliveryChanged := input.Delivery != nil &&
		next.Delivery.Normalized() != current.Delivery.Normalized()
	if deliveryChanged {
		if err = s.prepareTaskDeliveryMutation(ctx, &next, input.Source); err != nil {
			return nil, err
		}
	}
	if err = s.validateTaskUpdate(ctx, *current, next); err != nil {
		return nil, err
	}
	next.PermissionPolicy = s.taskPolicyForDefinitionUpdate(ctx, *current, next)
	permissionBoundaryChanged := next.PermissionPolicy.Revision != current.PermissionPolicy.Revision
	if permissionBoundaryChanged {
		next.PermissionState = automationdomain.TaskPermissionStateReady
		next.PendingPermissionRequestID = ""
	}
	var updated *automationdomain.ScheduledTask
	if expectedVersion != nil {
		updated, err = s.repository.UpdateScheduledTaskAtVersion(ctx, next, *expectedVersion)
	} else {
		updated, err = s.repository.UpsertScheduledTask(ctx, next)
	}
	if err != nil {
		return nil, err
	}
	if permissionBoundaryChanged {
		if err = s.repository.SupersedePendingPermissionRequests(ctx, updated.OwnerUserID, updated.JobID); err != nil {
			return nil, err
		}
		if err = s.repository.CancelBlockedRunsForTaskRevision(
			ctx,
			updated.OwnerUserID,
			updated.JobID,
			updated.PermissionPolicy.Revision,
			"任务配置已修改，旧审批请求和被阻塞运行已失效",
		); err != nil {
			return nil, err
		}
	}
	state := s.ensureJobState(*updated)
	s.persistJobRuntime(ctx, s.jobRuntimeUpdateSnapshot(updated.JobID, state))
	eventRunID := updateTaskEventRunID(input, *current)
	s.recordTaskEvent(ctx, updateTaskEventAction(input, *updated), *updated, eventRunID, updateTaskEventDetail(input, *current, *updated))
	result := s.scheduledTaskRuntimeSnapshot(*updated, state)
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
	applyOptionalValue(input.PermissionMode, func(value string) { next.PermissionMode = automationdomain.NormalizePermissionMode(value) })
	applyOptionalValue(input.SessionTarget, func(value automationdomain.SessionTarget) { next.SessionTarget = value.Normalized() })
	applyOptionalValue(input.Delivery, func(value automationdomain.DeliveryTarget) { next.Delivery = value.Normalized() })
	applyOptionalValue(input.OverlapPolicy, func(value string) { next.OverlapPolicy = automationdomain.NormalizeOverlapPolicy(value) })
	applyOptionalValue(input.Enabled, func(value bool) { next.Enabled = value })
	next = automationdomain.NormalizeScheduledTaskSessionBinding(next)
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
	if next.Enabled && next.SessionBindingState == automationdomain.TaskSessionBindingStateRebindRequired {
		return automationdomain.ErrTaskSessionRebindRequired
	}
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
		Name:           task.Name,
		AgentID:        task.AgentID,
		Schedule:       task.Schedule,
		Instruction:    task.Instruction,
		ExecutionKind:  task.ExecutionKind,
		PermissionMode: task.PermissionMode,
		SessionTarget:  task.SessionTarget,
		Delivery:       task.Delivery,
		Source:         task.Source,
		OverlapPolicy:  task.OverlapPolicy,
		ExpiresAt:      cloneTimePointer(task.ExpiresAt),
		Enabled:        task.Enabled,
	}
}

// UpdateTaskStatus 切换任务启停。
func (s *Service) UpdateTaskStatus(ctx context.Context, jobID string, enabled bool) (*automationdomain.ScheduledTask, error) {
	return s.UpdateTask(ctx, jobID, automationdomain.UpdateJobInput{Enabled: &enabled})
}

// DeleteTask 删除任务，并返回是否取消了删除时仍活跃的 run。
func (s *Service) DeleteTask(ctx context.Context, jobID string) (*automationdomain.DeleteJobResult, error) {
	return s.deleteTask(ctx, jobID, nil)
}

// DeleteTaskAtVersion 删除对话在 lookup 阶段实际核对过的任务版本。
func (s *Service) DeleteTaskAtVersion(
	ctx context.Context,
	jobID string,
	expectedVersion int64,
) (*automationdomain.DeleteJobResult, error) {
	return s.deleteTask(ctx, jobID, &expectedVersion)
}

func (s *Service) deleteTask(
	ctx context.Context,
	jobID string,
	expectedVersion *int64,
) (*automationdomain.DeleteJobResult, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	s.taskControlMu.Lock()
	defer s.taskControlMu.Unlock()

	current, err := s.loadRequiredScheduledTask(ctx, strings.TrimSpace(jobID))
	if err != nil {
		return nil, err
	}
	if err = rejectAgentScriptControl(ctx, *current); err != nil {
		return nil, err
	}
	if expectedVersion != nil &&
		(*expectedVersion < 1 || current.ConfigurationVersion != *expectedVersion) {
		return nil, automationdomain.ErrConfigurationVersionConflict
	}
	if expectedVersion != nil {
		if err = s.repository.DeleteScheduledTaskAtVersion(
			ctx,
			current.OwnerUserID,
			current.JobID,
			*expectedVersion,
		); err != nil {
			return nil, err
		}
	}
	result, err := s.applyScheduledTaskDeletion(ctx, *current, expectedVersion != nil)
	if err != nil && expectedVersion != nil {
		s.mu.Lock()
		delete(s.jobStates, current.JobID)
		s.mu.Unlock()
		return nil, &TaskDeletionReconcileError{cause: err}
	}
	return result, err
}

func (s *Service) applyScheduledTaskDeletion(
	ctx context.Context,
	current automationdomain.ScheduledTask,
	persistenceCommitted bool,
) (*automationdomain.DeleteJobResult, error) {
	cancelledRunID, cancelledRun, err := s.cancelDeletedTaskActiveRun(ctx, current)
	if err != nil {
		return nil, err
	}
	deadLetteredDeliveryRunIDs, err := s.deadLetterDeletedTaskPendingDeliveries(ctx, current)
	if err != nil {
		return nil, err
	}
	if !skipIsolatedAutomationSessionCleanup(ctx) {
		if err = s.cleanupIsolatedAutomationSessions(ctx, current); err != nil {
			return nil, err
		}
	}
	if !persistenceCommitted {
		if err = s.repository.DeleteScheduledTask(ctx, current.OwnerUserID, current.JobID); err != nil {
			return nil, err
		}
	}
	s.mu.Lock()
	delete(s.jobStates, current.JobID)
	s.mu.Unlock()
	s.recordTaskEvent(ctx, automationdomain.TaskEventActionDelete, current, cancelledRunID, deleteTaskEventDetail(current, cancelledRunID, cancelledRun, deadLetteredDeliveryRunIDs))
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

// TaskDeletionReconcileError 表示版本化删除已提交，但外围清理仍需重试。
type TaskDeletionReconcileError struct {
	cause error
}

func (e *TaskDeletionReconcileError) Error() string {
	return "定时任务已删除，但关联运行态清理需要 reconcile: " + e.cause.Error()
}

func (e *TaskDeletionReconcileError) Unwrap() error {
	return e.cause
}

// TaskDeletionCommitted 判断删除错误是否发生在持久化提交之后。
func TaskDeletionCommitted(err error) bool {
	var committed *TaskDeletionReconcileError
	return errors.As(err, &committed)
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
		strings.TrimSpace(run.Status) == automationdomain.RunStatusRunning ||
		strings.TrimSpace(run.Status) == automationdomain.RunStatusQueuedToMain {
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
		if err = runner.HandleInterrupt(runCtx, roomrealtime.InterruptRequest{SessionKey: sessionKey}); err != nil {
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

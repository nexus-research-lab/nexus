// INPUT: 人类或 Agent actor 的任务 CRUD、版本化删除确认及当前持久化任务。
// OUTPUT: 校验、持久化、运行态与审计同步后的任务结果。
// POS: scheduled task 配置写入、删除收口与 script capability 的事务边界。
package automation

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	roomrealtime "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

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
	if err := validateRunnableSchedule(normalized.Schedule); err != nil {
		return nil, err
	}
	if err := rejectAgentScriptCreate(ctx, normalized); err != nil {
		return nil, err
	}
	ownerUserID, scoped := scopedOwnerUserID(ctx)
	if !scoped || strings.TrimSpace(ownerUserID) == "" {
		var err error
		ownerUserID, err = s.resolveTaskOwnerUserID(ctx, normalized.AgentID)
		if err != nil {
			return nil, err
		}
	}
	intentDigest := ""
	legacyReplayCandidate := false
	if normalized.RequestID != "" {
		// Bind the ledger to the caller's normalized submission before mutable
		// Room or delivery readiness can fill a default replying Agent.
		intentDigest = taskCreateIntentDigest(normalized)
		replayed, found, replayErr := s.repository.GetScheduledTaskCreateReplay(
			ctx,
			ownerUserID,
			normalized.RequestID,
			normalized.AgentID,
			intentDigest,
		)
		if replayErr == nil && found {
			state := s.ensureJobState(*replayed)
			result := s.scheduledTaskRuntimeSnapshot(*replayed, state)
			return &result, nil
		}
		if replayErr != nil {
			if !found || !errors.Is(replayErr, automationdomain.ErrCreateRequestConflict) {
				return nil, replayErr
			}
			// Older releases stored the digest after Room default resolution.
			legacyReplayCandidate = true
		}
	}
	if err := s.validateTaskExpiration(normalized.ExpiresAt); err != nil {
		return nil, err
	}
	if err := s.validateAgentAndTarget(ctx, normalized.AgentID, normalized.SessionTarget); err != nil {
		return nil, err
	}
	resolvedOwnerUserID, err := s.resolveTaskOwnerUserID(ctx, normalized.AgentID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(resolvedOwnerUserID) != strings.TrimSpace(ownerUserID) {
		return nil, errors.New("target Agent must be owned by the scheduled task owner")
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
	normalized.Delivery = deliveryCandidate.Delivery
	deliveryGrant := deliveryCandidate.DeliveryGrant
	legacyIntentDigest := ""
	if legacyReplayCandidate {
		legacyIntentDigest = taskCreateIntentDigest(normalized)
		replayed, found, replayErr := s.repository.GetScheduledTaskCreateReplay(
			ctx,
			ownerUserID,
			normalized.RequestID,
			normalized.AgentID,
			legacyIntentDigest,
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
		if errors.Is(err, automationdomain.ErrCreateRequestConflict) &&
			legacyIntentDigest != "" && legacyIntentDigest != intentDigest {
			created, _, err = s.repository.GetScheduledTaskCreateReplay(
				ctx,
				ownerUserID,
				normalized.RequestID,
				normalized.AgentID,
				legacyIntentDigest,
			)
			createdNew = false
		}
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
	return s.updateTask(ctx, jobID, input, nil, nil)
}

// UpdateTaskAtVersion 更新对话在读取阶段看到的精确版本。
func (s *Service) UpdateTaskAtVersion(
	ctx context.Context,
	jobID string,
	expectedVersion int64,
	input automationdomain.UpdateJobInput,
) (*automationdomain.ScheduledTask, error) {
	return s.updateTask(ctx, jobID, input, &expectedVersion, nil)
}

// UpdateTaskAtVersionAndRunningRun 只在配置与 plan 观察到的 active run 都未变化时写入。
func (s *Service) UpdateTaskAtVersionAndRunningRun(
	ctx context.Context,
	jobID string,
	expectedVersion int64,
	expectedRunningRunID string,
	input automationdomain.UpdateJobInput,
) (*automationdomain.ScheduledTask, error) {
	expectedRunningRunID = strings.TrimSpace(expectedRunningRunID)
	return s.updateTask(ctx, jobID, input, &expectedVersion, &expectedRunningRunID)
}

func (s *Service) updateTask(
	ctx context.Context,
	jobID string,
	input automationdomain.UpdateJobInput,
	expectedVersion *int64,
	expectedRunningRunID *string,
) (*automationdomain.ScheduledTask, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	fence := s.taskExecutionFence(jobID)
	fence.Lock()
	defer fence.Unlock()
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
	if expectedRunningRunID != nil &&
		strings.TrimSpace(current.RunningRunID) != strings.TrimSpace(*expectedRunningRunID) {
		return nil, automationdomain.ErrConfigurationVersionConflict
	}
	preparedCurrent, err := s.prepareTaskPermissionPolicyForMutation(ctx, *current)
	if err != nil {
		return nil, err
	}
	current = &preparedCurrent
	next, err := s.applyTaskUpdate(*current, input)
	if err != nil {
		return nil, err
	}
	if input.Schedule != nil {
		if err = validateRunnableSchedule(next.Schedule); err != nil {
			return nil, err
		}
	}
	if err = rejectAgentScriptControl(ctx, *current, next); err != nil {
		return nil, err
	}
	agentChanged := strings.TrimSpace(next.AgentID) != strings.TrimSpace(current.AgentID)
	if agentChanged && input.SessionTarget == nil {
		return nil, errors.New("changing agent_id requires session_target in the same update")
	}
	if agentChanged && next.Delivery.Normalized().Mode != automationdomain.DeliveryModeNone &&
		input.Delivery == nil {
		return nil, errors.New("changing agent_id with delivery enabled requires delivery in the same update")
	}
	deliveryChanged := input.Delivery != nil &&
		next.Delivery.Normalized() != current.Delivery.Normalized()
	if deliveryChanged && isLegacyAutomationInboxDelivery(next.Delivery) {
		return nil, errors.New("the scheduled task inbox is legacy-only; select an existing Nexus, Room, or IM session")
	}
	if deliveryChanged || (agentChanged && input.Delivery != nil) {
		if err = s.prepareTaskDeliveryMutation(ctx, &next, input.Source); err != nil {
			return nil, err
		}
	}
	if err = s.validateTaskUpdate(ctx, *current, next); err != nil {
		return nil, err
	}
	if agentChanged {
		next.OwnerUserID, err = s.resolveTaskOwnerUserID(ctx, next.AgentID)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(next.OwnerUserID) != strings.TrimSpace(current.OwnerUserID) {
			return nil, errors.New("target Agent must be owned by the scheduled task owner")
		}
		if input.PermissionMode == nil {
			next.PermissionMode = ""
		}
		snapshot, snapshotErr := s.resolveInitialTaskPermissionSnapshot(ctx, next)
		if snapshotErr != nil {
			return nil, snapshotErr
		}
		next.PermissionMode = snapshot.Mode
		next.PermissionPolicy = s.buildTaskPermissionPolicyFromOptions(
			ctx,
			next,
			snapshot.AgentOptions,
			taskPermissionMutationIsDirectUser(ctx, next.Source.Kind),
			false,
		)
		next.PermissionPolicy.Revision = current.PermissionPolicy.Revision + 1
	} else if input.PermissionMode != nil && strings.TrimSpace(*input.PermissionMode) == "" {
		snapshot, snapshotErr := s.resolveInitialTaskPermissionSnapshot(ctx, next)
		if snapshotErr != nil {
			return nil, snapshotErr
		}
		next.PermissionMode = snapshot.Mode
		next.PermissionPolicy = s.buildTaskPermissionPolicyFromOptions(
			ctx,
			next,
			snapshot.AgentOptions,
			taskPermissionMutationIsDirectUser(ctx, next.Source.Kind),
			false,
		)
		next.PermissionPolicy.Revision = current.PermissionPolicy.Revision + 1
	} else {
		next.PermissionPolicy = s.taskPolicyForDefinitionUpdate(ctx, *current, next)
	}
	permissionBoundaryChanged := next.PermissionPolicy.Revision != current.PermissionPolicy.Revision
	if permissionBoundaryChanged {
		next.PermissionState = automationdomain.TaskPermissionStateReady
		next.PendingPermissionRequestID = ""
	}
	var updated *automationdomain.ScheduledTask
	var invalidatedRequests []automationdomain.AutomationPermissionRequest
	if permissionBoundaryChanged {
		updated, invalidatedRequests, err = s.repository.UpdateTaskAndInvalidatePermissionBoundary(
			ctx,
			automationstore.TaskPermissionBoundaryUpdateInput{
				Job:                  next,
				ExpectedVersion:      expectedVersion,
				ExpectedRunningRunID: expectedRunningRunID,
				CancellationMessage:  "任务配置已修改，旧审批请求和被阻塞运行已失效",
			},
		)
	} else if expectedVersion != nil && expectedRunningRunID != nil {
		updated, err = s.repository.UpdateScheduledTaskAtVersionAndRunningRun(
			ctx,
			next,
			*expectedVersion,
			*expectedRunningRunID,
		)
	} else if expectedVersion != nil {
		updated, err = s.repository.UpdateScheduledTaskAtVersion(ctx, next, *expectedVersion)
	} else {
		updated, err = s.repository.UpsertScheduledTask(ctx, next)
	}
	if err != nil {
		return nil, err
	}
	if permissionBoundaryChanged {
		for _, request := range invalidatedRequests {
			request.Status = automationdomain.PermissionRequestStatusSuperseded
			s.notifyAutomationPermissionSessionResolution(ctx, *current, request)
		}
	}
	var state *automationexec.JobRuntimeState
	if permissionBoundaryChanged {
		// Definition update and blocked-run cancellation already committed the
		// exact runtime summary together. Project that durable state without a
		// second broad write that could restore the pre-cancellation snapshot.
		state = s.replacePersistedJobRuntimeState(*updated)
	} else {
		state = s.ensureJobState(*updated)
		s.persistJobRuntime(ctx, s.jobRuntimeUpdateSnapshot(updated.JobID, state))
	}
	eventRunID := updateTaskEventRunID(input, *current)
	s.recordTaskEvent(ctx, updateTaskEventAction(input, *updated), *updated, eventRunID, updateTaskEventDetail(input, *current, *updated))
	result := s.scheduledTaskRuntimeSnapshot(*updated, state)
	return &result, nil
}

func validateRunnableSchedule(schedule automationdomain.Schedule) error {
	_, err := automationexec.ComputeNextRunAt(schedule, time.Unix(0, 0))
	return err
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
	if strings.TrimSpace(task.DeletionState) != "" {
		return nil, automationdomain.ErrTaskDeleting
	}
	return task, nil
}

func (s *Service) applyTaskUpdate(
	current automationdomain.ScheduledTask,
	input automationdomain.UpdateJobInput,
) (automationdomain.ScheduledTask, error) {
	next := current
	applyOptionalValue(input.Name, func(value string) { next.Name = strings.TrimSpace(value) })
	applyOptionalValue(input.AgentID, func(value string) { next.AgentID = strings.TrimSpace(value) })
	applyOptionalValue(input.Schedule, func(value automationdomain.Schedule) { next.Schedule = value.Normalized() })
	applyOptionalValue(input.Instruction, func(value string) { next.Instruction = strings.TrimSpace(value) })
	applyOptionalValue(input.ExecutionKind, func(value string) { next.ExecutionKind = automationdomain.NormalizeExecutionKind(value) })
	applyOptionalValue(input.PermissionMode, func(value string) {
		if strings.TrimSpace(value) == "" {
			next.PermissionMode = ""
			return
		}
		next.PermissionMode = automationdomain.NormalizePermissionMode(value)
	})
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
	fence := s.taskExecutionFence(jobID)
	fence.Lock()
	fenceHeld := true
	s.taskControlMu.Lock()
	controlHeld := true
	defer func() {
		if controlHeld {
			s.taskControlMu.Unlock()
		}
		if fenceHeld {
			fence.Unlock()
		}
	}()
	ownerUserID, _ := scopedOwnerUserID(ctx)
	current, err := s.repository.GetScheduledTask(ctx, ownerUserID, strings.TrimSpace(jobID))
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, automationdomain.ErrJobNotFound
	}
	if err = rejectAgentScriptControl(ctx, *current); err != nil {
		return nil, err
	}
	if strings.TrimSpace(current.DeletionState) == "" && expectedVersion != nil &&
		(*expectedVersion < 1 || current.ConfigurationVersion != *expectedVersion) {
		return nil, automationdomain.ErrConfigurationVersionConflict
	}
	claim, err := s.repository.ClaimScheduledTaskDeletion(ctx, automationstore.TaskDeletionClaimInput{
		OwnerUserID:     current.OwnerUserID,
		JobID:           current.JobID,
		ExpectedVersion: expectedVersion,
		DeletionToken:   s.idFactory("task_delete"),
		ClaimedAt:       s.nowFn(),
	})
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	delete(s.jobStates, current.JobID)
	s.mu.Unlock()
	s.taskControlMu.Unlock()
	controlHeld = false
	fence.Unlock()
	fenceHeld = false
	s.wakeScheduler()
	return s.continueScheduledTaskDeletion(ctx, claim.Task, claim.Token)
}

const taskDeletionCleanupTimeout = 20 * time.Second

func (s *Service) continueScheduledTaskDeletion(
	ctx context.Context,
	current automationdomain.ScheduledTask,
	deletionToken string,
) (*automationdomain.DeleteJobResult, error) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), taskDeletionCleanupTimeout)
	defer cancel()
	cleanupCtx = contextForJobOwner(cleanupCtx, current)
	runs, err := s.repository.ListTaskDeletionCleanupRuns(cleanupCtx, current.OwnerUserID, current.JobID)
	if err != nil {
		return nil, MarkTaskDeletionPrepared(err)
	}
	message := "scheduled task was deleted while this run was active"
	for _, run := range runs {
		if err = s.interruptDeletingRun(cleanupCtx, current, run, message); err != nil {
			if errors.Is(err, ErrExecutionAttemptOwnershipUnconfirmed) {
				if markErr := s.repository.MarkTaskDeletionReviewRequired(
					cleanupCtx, current.OwnerUserID, current.JobID, deletionToken,
				); markErr != nil {
					err = errors.Join(err, markErr)
				}
			}
			return nil, MarkTaskDeletionPrepared(err)
		}
	}
	if !skipIsolatedAutomationSessionCleanup(cleanupCtx) {
		if err = s.cleanupIsolatedAutomationSessions(cleanupCtx, current); err != nil {
			return nil, MarkTaskDeletionPrepared(err)
		}
	}
	return s.finalizeScheduledTaskDeletionClaim(
		cleanupCtx,
		current,
		deletionToken,
		runs,
		taskDeletionFinalizeFence{},
	)
}

type taskDeletionFinalizeFence struct {
	expectedState         string
	expectedVersion       *int64
	ownerConfirmedStopped bool
}

// ConfirmTaskDeletionStoppedAtVersion 是 review_required 的唯一人工收尾入口。
// 用户确认只证明原执行实例已经停止；服务端仍以 owner/job/version、私有 deletion
// token 和 review state 做最终 CAS，并且只取消 ledger，绝不恢复执行或投递。
func (s *Service) ConfirmTaskDeletionStoppedAtVersion(
	ctx context.Context,
	jobID string,
	expectedVersion int64,
) (*automationdomain.DeleteJobResult, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, scoped := scopedOwnerUserID(ctx)
	if !scoped || strings.TrimSpace(ownerUserID) == "" {
		return nil, errors.New("scheduled task deletion confirmation requires an owner scope")
	}
	if expectedVersion < 1 {
		return nil, automationdomain.ErrConfigurationVersionConflict
	}
	jobID = strings.TrimSpace(jobID)
	fence := s.taskExecutionFence(jobID)
	fence.Lock()
	s.taskControlMu.Lock()
	current, err := s.repository.GetScheduledTask(ctx, ownerUserID, jobID)
	if err != nil {
		s.taskControlMu.Unlock()
		fence.Unlock()
		return nil, err
	}
	if current == nil {
		s.taskControlMu.Unlock()
		fence.Unlock()
		return nil, automationdomain.ErrJobNotFound
	}
	if current.ConfigurationVersion != expectedVersion {
		s.taskControlMu.Unlock()
		fence.Unlock()
		return nil, automationdomain.ErrConfigurationVersionConflict
	}
	if strings.TrimSpace(current.DeletionState) != automationdomain.TaskDeletionStateReviewRequired ||
		strings.TrimSpace(current.DeletionToken) == "" {
		s.taskControlMu.Unlock()
		fence.Unlock()
		return nil, automationdomain.ErrTaskDeletionReviewConflict
	}
	deletionToken := strings.TrimSpace(current.DeletionToken)
	s.mu.Lock()
	delete(s.jobStates, current.JobID)
	s.mu.Unlock()
	s.taskControlMu.Unlock()
	fence.Unlock()

	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), taskDeletionCleanupTimeout)
	defer cancel()
	cleanupCtx = contextForJobOwner(cleanupCtx, *current)
	runs, err := s.repository.ListTaskDeletionCleanupRuns(cleanupCtx, current.OwnerUserID, current.JobID)
	if err != nil {
		return nil, MarkTaskDeletionPrepared(err)
	}
	if !skipIsolatedAutomationSessionCleanup(cleanupCtx) {
		if err = s.cleanupIsolatedAutomationSessions(cleanupCtx, *current); err != nil {
			return nil, MarkTaskDeletionPrepared(err)
		}
	}
	return s.finalizeScheduledTaskDeletionClaim(
		cleanupCtx,
		*current,
		deletionToken,
		runs,
		taskDeletionFinalizeFence{
			expectedState:         automationdomain.TaskDeletionStateReviewRequired,
			expectedVersion:       &expectedVersion,
			ownerConfirmedStopped: true,
		},
	)
}

func (s *Service) finalizeScheduledTaskDeletionClaim(
	ctx context.Context,
	current automationdomain.ScheduledTask,
	deletionToken string,
	runs []automationdomain.ScheduledTaskRun,
	fence taskDeletionFinalizeFence,
) (*automationdomain.DeleteJobResult, error) {
	now := s.nowFn()
	message := "scheduled task was deleted while this run was active"
	cancelledRunID := ""
	cancelledRunIDs := make([]string, 0, len(runs))
	for _, run := range runs {
		runID := strings.TrimSpace(run.RunID)
		if runID == "" {
			continue
		}
		if cancelledRunID == "" {
			cancelledRunID = runID
		}
		cancelledRunIDs = append(cancelledRunIDs, runID)
	}
	detail := deleteTaskEventDetail(current, cancelledRunID, len(cancelledRunIDs) > 0, nil)
	if len(cancelledRunIDs) > 0 {
		detail["cancelled_run_ids"] = cancelledRunIDs
	}
	if fence.ownerConfirmedStopped {
		detail["execution_stop_confirmed_by_owner"] = true
		detail["review_required_finalized"] = true
		detail["external_actions_replayed"] = false
	}
	event, eventInput, ok := s.prepareTaskEvent(ctx, automationdomain.TaskEventActionDelete, current, cancelledRunID, detail)
	if !ok {
		return nil, MarkTaskDeletionPrepared(errors.New("scheduled task delete event identity is invalid"))
	}
	finalized, err := s.repository.FinalizeScheduledTaskDeletion(
		ctx,
		automationstore.TaskDeleteFinalizationInput{
			OwnerUserID:                  current.OwnerUserID,
			JobID:                        current.JobID,
			DeletionToken:                deletionToken,
			ExpectedDeletionState:        fence.expectedState,
			ExpectedConfigurationVersion: fence.expectedVersion,
			FinishedAt:                   now,
			ActiveRunMessage:             message,
			DeliveryDeadLetter:           now,
			DeliveryError:                "scheduled task was deleted before delivery could be retried",
			UnconfirmedDeliveryError:     "delivery result was not confirmed before task deletion; automatic redelivery is disabled",
			PendingDeliveryError:         "scheduled task was deleted before delivery was claimed; result was not sent",
			DeleteEvent:                  eventInput,
		},
	)
	if err != nil {
		return nil, MarkTaskDeletionPrepared(err)
	}
	cancelledRuns := mergeDeletionCancelledRuns(runs, finalized.CancelledRuns)
	cancelledRunIDs = cancelledRunIDs[:0]
	for _, run := range cancelledRuns {
		if runID := strings.TrimSpace(run.RunID); runID != "" {
			cancelledRunIDs = append(cancelledRunIDs, runID)
		}
	}
	cancelledRunID = ""
	if len(cancelledRunIDs) > 0 {
		cancelledRunID = cancelledRunIDs[0]
	}
	s.invalidateDeliveryRetryDeadline()
	event.Detail = deleteTaskEventDetail(current, cancelledRunID, len(cancelledRuns) > 0, finalized.DeadLetteredDeliveryRunIDs)
	if len(cancelledRunIDs) > 0 {
		event.Detail["cancelled_run_ids"] = cancelledRunIDs
	}
	if len(finalized.UnconfirmedDeliveryRunIDs) > 0 {
		event.Detail["unconfirmed_delivery_run_ids"] = finalized.UnconfirmedDeliveryRunIDs
		event.Detail["delivery_outcome_unconfirmed"] = true
	}
	if len(finalized.NotAttemptedDeliveryRunIDs) > 0 {
		event.Detail["not_attempted_delivery_run_ids"] = finalized.NotAttemptedDeliveryRunIDs
	}
	if fence.ownerConfirmedStopped {
		event.Detail["execution_stop_confirmed_by_owner"] = true
		event.Detail["review_required_finalized"] = true
		event.Detail["external_actions_replayed"] = false
	}
	s.notifyTaskEvent(ctx, event)
	for _, request := range finalized.SupersededPermissionRequests {
		request.Status = automationdomain.PermissionRequestStatusSuperseded
		s.notifyAutomationPermissionSessionResolution(ctx, current, request)
	}
	result := &automationdomain.DeleteJobResult{
		JobID:              current.JobID,
		AgentID:            current.AgentID,
		Deleted:            true,
		ActiveRunID:        cancelledRunID,
		CancelledActiveRun: len(cancelledRuns) > 0,
	}
	if result.CancelledActiveRun {
		result.CancelledRunID = cancelledRunID
	}
	return result, nil
}

func mergeDeletionCancelledRuns(groups ...[]automationdomain.ScheduledTaskRun) []automationdomain.ScheduledTaskRun {
	items := make([]automationdomain.ScheduledTaskRun, 0)
	indexes := make(map[string]int)
	for _, group := range groups {
		for _, run := range group {
			runID := strings.TrimSpace(run.RunID)
			if runID == "" {
				continue
			}
			if index, ok := indexes[runID]; ok {
				items[index] = run
				continue
			}
			indexes[runID] = len(items)
			items = append(items, run)
		}
	}
	return items
}

func (s *Service) interruptDeletingRun(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	run automationdomain.ScheduledTaskRun,
	message string,
) error {
	if err := s.cancelScriptAttemptAndWait(ctx, job, run); err != nil {
		return err
	}
	sessionKey := strings.TrimSpace(run.SessionKey)
	if sessionKey == "" {
		return nil
	}
	if strings.TrimSpace(run.Status) == automationdomain.RunStatusRunning &&
		strings.TrimSpace(run.RoundID) != "" && s.physicalAttempt(run.RunID, run.RoundID) == nil {
		return ErrExecutionAttemptOwnershipUnconfirmed
	}
	runCtx := contextForJobOwner(ctx, job)
	for {
		var err error
		switch protocol.ParseSessionKey(sessionKey).Kind {
		case protocol.SessionKeyKindRoom:
			runner, ok := s.room.(roomInterruptRunner)
			if !ok || runner == nil {
				return ErrExecutionAttemptOwnershipUnconfirmed
			}
			err = runner.HandleInterrupt(runCtx, roomrealtime.InterruptRequest{
				SessionKey: sessionKey,
				RoundID:    strings.TrimSpace(run.RoundID),
			})
			if errors.Is(err, roomrealtime.ErrTargetRoomRoundNotRunning) {
				err = errDeletionRunNotRegistered
			}
		case protocol.SessionKeyKindAgent:
			runner, ok := s.dm.(dmInterruptRunner)
			if !ok || runner == nil {
				return ErrExecutionAttemptOwnershipUnconfirmed
			}
			err = runner.HandleInterrupt(runCtx, dmsvc.InterruptRequest{
				SessionKey: sessionKey,
				RoundID:    strings.TrimSpace(run.RoundID),
			})
			if errors.Is(err, dmsvc.ErrTargetDMRoundNotRunning) {
				err = errDeletionRunNotRegistered
			}
		default:
			s.cancelPendingRunPermissions(sessionKey, message)
			return nil
		}
		if err == nil {
			s.cancelPendingRunPermissions(sessionKey, message)
			return nil
		}
		if !errors.Is(err, errDeletionRunNotRegistered) {
			return err
		}
		persisted, loadErr := s.repository.GetRun(runCtx, job.OwnerUserID, job.JobID, run.RunID)
		if errors.Is(loadErr, sql.ErrNoRows) || persisted == nil ||
			(loadErr == nil && persisted.Status != automationdomain.RunStatusRunning) {
			return nil
		}
		if loadErr != nil {
			return loadErr
		}
		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-runCtx.Done():
			timer.Stop()
			return errors.Join(ErrExecutionAttemptOwnershipUnconfirmed, runCtx.Err())
		case <-timer.C:
		}
	}
}

var errDeletionRunNotRegistered = errors.New("automation run dispatch is not registered yet")

// TaskDeletionPreparedError 表示 durable claim 已提交，但幂等清理尚未收口。
type TaskDeletionPreparedError struct {
	cause error
}

func (e *TaskDeletionPreparedError) Error() string {
	return "定时任务已停止运行，删除清理尚未完成: " + e.cause.Error()
}

func (e *TaskDeletionPreparedError) Unwrap() error {
	return e.cause
}

// MarkTaskDeletionPrepared 标记错误发生在删除前置收尾之后。
func MarkTaskDeletionPrepared(err error) error {
	if err == nil || TaskDeletionPrepared(err) {
		return err
	}
	return &TaskDeletionPreparedError{cause: err}
}

// TaskDeletionPrepared 判断删除前置收尾是否可能已经发生。
func TaskDeletionPrepared(err error) bool {
	var prepared *TaskDeletionPreparedError
	return errors.As(err, &prepared)
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
	if err = s.cancelScriptAttemptAndWait(ctx, job, *run); err != nil {
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
		if err = runner.HandleInterrupt(runCtx, roomrealtime.InterruptRequest{SessionKey: sessionKey, RoundID: strings.TrimSpace(run.RoundID)}); err != nil &&
			!errors.Is(err, roomrealtime.ErrTargetRoomRoundNotRunning) {
			return err
		}
	case protocol.SessionKeyKindAgent:
		runner, ok := s.dm.(dmInterruptRunner)
		if !ok || runner == nil {
			s.cancelPendingRunPermissions(sessionKey, message)
			return nil
		}
		if err = runner.HandleInterrupt(runCtx, dmsvc.InterruptRequest{SessionKey: sessionKey, RoundID: strings.TrimSpace(run.RoundID)}); err != nil &&
			!errors.Is(err, dmsvc.ErrTargetDMRoundNotRunning) {
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

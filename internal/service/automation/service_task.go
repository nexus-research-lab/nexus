package automation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func scopedOwnerUserID(ctx context.Context) (string, bool) {
	return authctx.CurrentUserID(ctx)
}

func effectiveOwnerUserID(ctx context.Context) string {
	return authctx.OwnerUserID(ctx)
}

// ListTasks 列出任务。
func (s *Service) ListTasks(ctx context.Context, agentID string) ([]protocol.CronJob, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	items, err := s.repository.ListCronJobs(ctx, ownerUserID, agentID)
	if err != nil {
		return nil, err
	}
	result := make([]protocol.CronJob, 0, len(items))
	for _, item := range items {
		state := s.ensureJobState(item)
		enriched := item
		enriched.Running = state.Running
		enriched.RunningRunID = strings.TrimSpace(state.RunningRunID)
		enriched.RunningStartedAt = cloneTimePointer(state.RunningStartedAt)
		enriched.NextRunAt = cloneTimePointer(state.NextRunAt)
		enriched.LastRunAt = cloneTimePointer(state.LastRunAt)
		enriched.LastRunStatus = strings.TrimSpace(state.LastRunStatus)
		enriched.FailureStreak = state.FailureStreak
		enriched.LastError = cloneStringPointer(state.LastError)
		enriched.LastDeliveryStatus = strings.TrimSpace(state.LastDeliveryStatus)
		result = append(result, enriched)
	}
	return result, nil
}

// CountEnabledTasks 返回启用中的定时任务数量。
func (s *Service) CountEnabledTasks(ctx context.Context, agentID string) (int, error) {
	if err := s.ensureReady(ctx); err != nil {
		return 0, err
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	return s.repository.CountEnabledCronJobs(ctx, ownerUserID, strings.TrimSpace(agentID))
}

// GetTask 按 job_id 读取任务。返回 nil 表示未找到。
func (s *Service) GetTask(ctx context.Context, jobID string) (*protocol.CronJob, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	job, err := s.repository.GetCronJob(ctx, ownerUserID, strings.TrimSpace(jobID))
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, nil
	}
	state := s.ensureJobState(*job)
	enriched := *job
	enriched.Running = state.Running
	enriched.RunningRunID = strings.TrimSpace(state.RunningRunID)
	enriched.RunningStartedAt = cloneTimePointer(state.RunningStartedAt)
	enriched.NextRunAt = cloneTimePointer(state.NextRunAt)
	enriched.LastRunAt = cloneTimePointer(state.LastRunAt)
	enriched.LastRunStatus = strings.TrimSpace(state.LastRunStatus)
	enriched.FailureStreak = state.FailureStreak
	enriched.LastError = cloneStringPointer(state.LastError)
	enriched.LastDeliveryStatus = strings.TrimSpace(state.LastDeliveryStatus)
	return &enriched, nil
}

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
	s.recordTaskEvent(ctx, protocol.TaskEventActionCreate, *created, "", createTaskEventDetail(*created))
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

// RunTaskNow 立即触发一次任务。
func (s *Service) RunTaskNow(ctx context.Context, jobID string) (*protocol.ExecutionResult, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	job, err := s.repository.GetCronJob(ctx, ownerUserID, strings.TrimSpace(jobID))
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, protocol.ErrJobNotFound
	}
	s.loggerFor(ctx).Info("手动触发自动化任务",
		"job_id", job.JobID,
		"agent_id", job.AgentID,
	)
	result, err := s.startJobExecution(ctx, *job, "manual", s.nowFn())
	if err == nil {
		runID := ""
		if result != nil && result.RunID != nil {
			runID = *result.RunID
		}
		s.recordTaskEvent(ctx, protocol.TaskEventActionRunNow, *job, runID, map[string]any{"status": anyExecutionStatus(result)})
	}
	return result, err
}

// ListTaskRuns 返回任务运行历史。
func (s *Service) ListTaskRuns(ctx context.Context, jobID string) ([]protocol.CronRun, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	normalizedJobID := strings.TrimSpace(jobID)
	job, err := s.repository.GetCronJob(ctx, ownerUserID, normalizedJobID)
	if err != nil {
		return nil, err
	}
	runs, err := s.repository.ListRunsByJob(ctx, ownerUserID, normalizedJobID)
	if err != nil {
		return nil, err
	}
	if job != nil {
		return runs, nil
	}
	events, err := s.repository.ListTaskEventsByJob(ctx, ownerUserID, normalizedJobID, 1)
	if err != nil {
		return nil, err
	}
	if len(runs) == 0 && len(events) == 0 {
		return nil, protocol.ErrJobNotFound
	}
	return runs, nil
}

// RetryRunDelivery 只重试某次 run 的结果投递，不重新执行任务本身。
func (s *Service) RetryRunDelivery(ctx context.Context, jobID string, runID string) (*protocol.CronRun, error) {
	return s.retryRunDelivery(ctx, jobID, runID, true)
}

func (s *Service) retryRunDelivery(ctx context.Context, jobID string, runID string, recordEvent bool) (*protocol.CronRun, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	job, err := s.repository.GetCronJob(ctx, ownerUserID, strings.TrimSpace(jobID))
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, protocol.ErrJobNotFound
	}
	run, err := s.repository.GetRun(ctx, ownerUserID, job.JobID, strings.TrimSpace(runID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, protocol.ErrRunNotFound
	}
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, protocol.ErrRunNotFound
	}
	if strings.TrimSpace(run.Status) == protocol.RunStatusPending || strings.TrimSpace(run.Status) == protocol.RunStatusRunning {
		return nil, errors.New("run is not finished")
	}
	if strings.TrimSpace(run.DeliveryStatus) != protocol.DeliveryStatusFailed {
		return nil, fmt.Errorf("run delivery_status must be failed before retrying delivery, got %q", strings.TrimSpace(run.DeliveryStatus))
	}

	observation := automationdomain.ExecutionObservation{
		Status:        protocol.RunStatusSucceeded,
		SessionID:     run.SessionID,
		MessageCount:  run.MessageCount,
		ResultText:    anyStringPointer(run.ResultText),
		AssistantText: anyStringPointer(run.AssistantText),
	}
	deliveryResult := s.deliverJobObservation(contextForJobOwner(ctx, *job), *job, run.SessionKey, observation)
	deliveryStatus := deliveryResult.Status
	deliveryError := deliveryResult.Error
	deliveryTo := deliveryResult.deliveryTo(job.Delivery)
	now := s.nowFn()
	deliveredAt := deliveredAtForStatus(deliveryStatus, now)
	attemptsAfter := run.DeliveryAttempts
	if deliveryAttempted(deliveryStatus) {
		attemptsAfter++
	}
	nextDeliveryAttemptAt, deliveryDeadLetterAt := deliveryRetrySchedule(deliveryStatus, attemptsAfter, now)
	if err = s.repository.MarkRunDelivery(ctx, automationstore.RunDeliveryUpdateInput{
		RunID:                 run.RunID,
		DeliveryMode:          strings.TrimSpace(job.Delivery.Mode),
		DeliveryTo:            deliveryTo,
		DeliveryStatus:        deliveryStatus,
		DeliveryError:         deliveryError,
		DeliveredAt:           deliveredAt,
		DeliveryAttempted:     deliveryAttempted(deliveryStatus),
		DeliveryNextAttemptAt: nextDeliveryAttemptAt,
		DeliveryDeadLetterAt:  deliveryDeadLetterAt,
	}); err != nil {
		return nil, err
	}
	s.updateJobLastDeliveryStatus(*job, deliveryStatus)

	updated, err := s.repository.GetRun(ctx, ownerUserID, job.JobID, run.RunID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, protocol.ErrRunNotFound
	}
	if err != nil {
		return nil, err
	}
	if recordEvent && updated != nil {
		s.recordTaskEvent(ctx, protocol.TaskEventActionRetryDelivery, *job, run.RunID, deliveryRetryTaskEventDetail(*updated))
	}
	return updated, nil
}

// RecoverTaskRunningRun 手动释放任务当前运行占用，并把未完成 run 标记为取消。
func (s *Service) RecoverTaskRunningRun(ctx context.Context, jobID string, runID string) (*protocol.CronJob, error) {
	current, err := s.GetTask(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, protocol.ErrJobNotFound
	}
	currentRunID := strings.TrimSpace(current.RunningRunID)
	if currentRunID == "" {
		return current, nil
	}
	expectedRunID := strings.TrimSpace(runID)
	if expectedRunID != "" && expectedRunID != currentRunID {
		return nil, errors.New("运行记录不一致，请刷新任务后重试")
	}
	message := "用户手动释放运行占用，已将未完成 run 标记为 cancelled"
	if err = s.interruptActiveRunExecution(ctx, *current, currentRunID, message); err != nil {
		return nil, err
	}
	recovered := s.recoverJobRuntimeAsCancelled(ctx, *current, message)
	state := s.replaceJobRuntimeState(recovered)
	result := recovered
	result.NextRunAt = cloneTimePointer(state.NextRunAt)
	result.Running = state.Running
	result.RunningRunID = strings.TrimSpace(state.RunningRunID)
	result.RunningStartedAt = cloneTimePointer(state.RunningStartedAt)
	result.LastRunAt = cloneTimePointer(state.LastRunAt)
	result.LastRunStatus = strings.TrimSpace(state.LastRunStatus)
	result.FailureStreak = state.FailureStreak
	result.LastError = cloneStringPointer(state.LastError)
	result.LastDeliveryStatus = strings.TrimSpace(state.LastDeliveryStatus)
	s.recordTaskEvent(ctx, protocol.TaskEventActionRecover, result, currentRunID, map[string]any{"recovered_run_id": currentRunID})
	return &result, nil
}

func anyExecutionStatus(result *protocol.ExecutionResult) string {
	if result == nil {
		return ""
	}
	return strings.TrimSpace(result.Status)
}

func (s *Service) resolveTaskOwnerUserID(ctx context.Context, agentID string) (string, error) {
	if s.agents != nil && strings.TrimSpace(agentID) != "" {
		agentValue, err := s.requireAgent(ctx, agentID)
		if err != nil {
			return "", err
		}
		if agentValue != nil && strings.TrimSpace(agentValue.OwnerUserID) != "" {
			return strings.TrimSpace(agentValue.OwnerUserID), nil
		}
	}
	return effectiveOwnerUserID(ctx), nil
}

func (s *Service) cleanupIsolatedAutomationSessions(ctx context.Context, job protocol.CronJob) error {
	if strings.TrimSpace(job.SessionTarget.Kind) != protocol.SessionTargetIsolated {
		return nil
	}
	workspacePath, err := s.resolveAutomationWorkspacePath(ctx, job.AgentID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(workspacePath) == "" {
		return nil
	}
	prefix := fmt.Sprintf("agent:%s:automation:dm:cron:%s:", strings.TrimSpace(job.AgentID), strings.TrimSpace(job.JobID))
	files := workspacestore.NewSessionFileStore(s.config.WorkspacePath)
	sessions, err := files.ListSessions(workspacePath)
	if err != nil {
		return err
	}
	for _, item := range sessions {
		sessionKey := strings.TrimSpace(item.SessionKey)
		if !strings.HasPrefix(sessionKey, prefix) {
			continue
		}
		parsed := protocol.ParseSessionKey(sessionKey)
		if parsed.Kind != protocol.SessionKeyKindAgent || !parsed.IsStructured || parsed.Channel != "automation" {
			continue
		}
		if _, deleteErr := files.DeleteSession(workspacePath, sessionKey); deleteErr != nil {
			return deleteErr
		}
		if s.sessionCloser != nil {
			_ = s.sessionCloser.CloseSession(context.Background(), sessionKey)
		}
	}
	return nil
}

func (s *Service) resolveAutomationWorkspacePath(ctx context.Context, agentID string) (string, error) {
	if s.agents != nil && strings.TrimSpace(agentID) != "" {
		agentValue, err := s.agents.GetAgent(ctx, strings.TrimSpace(agentID))
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(agentValue.WorkspacePath) != "" {
			return strings.TrimSpace(agentValue.WorkspacePath), nil
		}
	}
	return strings.TrimSpace(s.config.WorkspacePath), nil
}

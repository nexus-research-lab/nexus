package automation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

// RunTaskNow 立即触发一次任务。
func (s *Service) RunTaskNow(ctx context.Context, jobID string) (*automationdomain.ExecutionResult, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	job, err := s.repository.GetCronJob(ctx, ownerUserID, strings.TrimSpace(jobID))
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, automationdomain.ErrJobNotFound
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
		s.recordTaskEvent(ctx, automationdomain.TaskEventActionRunNow, *job, runID, map[string]any{"status": anyExecutionStatus(result)})
	}
	return result, err
}

// ListTaskRuns 返回任务运行历史。
func (s *Service) ListTaskRuns(ctx context.Context, jobID string) ([]automationdomain.CronRun, error) {
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
		return nil, automationdomain.ErrJobNotFound
	}
	return runs, nil
}

// RetryRunDelivery 只重试某次 run 的结果投递，不重新执行任务本身。
func (s *Service) RetryRunDelivery(ctx context.Context, jobID string, runID string) (*automationdomain.CronRun, error) {
	return s.retryRunDelivery(ctx, jobID, runID, true)
}

func (s *Service) retryRunDelivery(ctx context.Context, jobID string, runID string, recordEvent bool) (*automationdomain.CronRun, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	job, err := s.repository.GetCronJob(ctx, ownerUserID, strings.TrimSpace(jobID))
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, automationdomain.ErrJobNotFound
	}
	run, err := s.repository.GetRun(ctx, ownerUserID, job.JobID, strings.TrimSpace(runID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, automationdomain.ErrRunNotFound
	}
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, automationdomain.ErrRunNotFound
	}
	runStatus := strings.TrimSpace(run.Status)
	if runStatus == automationdomain.RunStatusPending || runStatus == automationdomain.RunStatusRunning {
		return nil, errors.New("run is not finished")
	}
	deliveryStatusBeforeRetry := strings.TrimSpace(run.DeliveryStatus)
	if deliveryStatusBeforeRetry != automationdomain.DeliveryStatusFailed {
		return nil, fmt.Errorf("run delivery_status must be failed before retrying delivery, got %q", deliveryStatusBeforeRetry)
	}

	observation := automationexec.ExecutionObservation{
		Status:        automationdomain.RunStatusSucceeded,
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
		return nil, automationdomain.ErrRunNotFound
	}
	if err != nil {
		return nil, err
	}
	if recordEvent && updated != nil {
		s.recordTaskEvent(ctx, automationdomain.TaskEventActionRetryDelivery, *job, run.RunID, deliveryRetryTaskEventDetail(*updated))
	}
	return updated, nil
}

// RecoverTaskRunningRun 手动释放任务当前运行占用，并把未完成 run 标记为取消。
func (s *Service) RecoverTaskRunningRun(ctx context.Context, jobID string, runID string) (*automationdomain.CronJob, error) {
	current, err := s.GetTask(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, automationdomain.ErrJobNotFound
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
	s.recordTaskEvent(ctx, automationdomain.TaskEventActionRecover, result, currentRunID, map[string]any{"recovered_run_id": currentRunID})
	return &result, nil
}

func anyExecutionStatus(result *automationdomain.ExecutionResult) string {
	if result == nil {
		return ""
	}
	return strings.TrimSpace(result.Status)
}

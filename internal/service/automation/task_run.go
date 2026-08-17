// INPUT: 手动运行、投递重试、运行恢复请求及当前任务/run 状态。
// OUTPUT: 受 owner 与 script capability 约束的执行、投递或恢复结果。
// POS: scheduled task 主动执行和修复的 service 最终边界。
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
	return s.runTaskNow(ctx, jobID, nil)
}

// RunTaskNowAtVersion 只运行调用方在 plan 阶段核对过的任务版本。
func (s *Service) RunTaskNowAtVersion(
	ctx context.Context,
	jobID string,
	expectedVersion int64,
) (*automationdomain.ExecutionResult, error) {
	return s.runTaskNow(ctx, jobID, &expectedVersion)
}

func (s *Service) runTaskNow(
	ctx context.Context,
	jobID string,
	expectedVersion *int64,
) (*automationdomain.ExecutionResult, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	s.taskControlMu.Lock()
	defer s.taskControlMu.Unlock()

	ownerUserID, _ := scopedOwnerUserID(ctx)
	job, err := s.repository.GetScheduledTask(ctx, ownerUserID, strings.TrimSpace(jobID))
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, automationdomain.ErrJobNotFound
	}
	if expectedVersion != nil &&
		(*expectedVersion < 1 || job.ConfigurationVersion != *expectedVersion) {
		return nil, automationdomain.ErrConfigurationVersionConflict
	}
	if err = rejectAgentScriptControl(ctx, *job); err != nil {
		return nil, err
	}
	s.loggerFor(ctx).Info("手动触发自动化任务",
		"job_id", job.JobID,
		"agent_id", job.AgentID,
	)
	result, err := s.startJobExecution(ctx, *job, automationdomain.TriggerKindManual, s.nowFn())
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
func (s *Service) ListTaskRuns(ctx context.Context, jobID string) ([]automationdomain.ScheduledTaskRun, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	normalizedJobID := strings.TrimSpace(jobID)
	job, err := s.repository.GetScheduledTask(ctx, ownerUserID, normalizedJobID)
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
func (s *Service) RetryRunDelivery(ctx context.Context, jobID string, runID string) (*automationdomain.ScheduledTaskRun, error) {
	return s.retryRunDeliveryAtVersion(ctx, jobID, runID, nil)
}

// RetryRunDeliveryAtVersion 只按 plan 阶段核对过的任务配置重投递一次结果。
func (s *Service) RetryRunDeliveryAtVersion(
	ctx context.Context,
	jobID string,
	runID string,
	expectedVersion int64,
) (*automationdomain.ScheduledTaskRun, error) {
	return s.retryRunDeliveryAtVersion(ctx, jobID, runID, &expectedVersion)
}

func (s *Service) retryRunDeliveryAtVersion(
	ctx context.Context,
	jobID string,
	runID string,
	expectedVersion *int64,
) (*automationdomain.ScheduledTaskRun, error) {
	s.taskControlMu.Lock()
	defer s.taskControlMu.Unlock()
	if expectedVersion != nil {
		ownerUserID, _ := scopedOwnerUserID(ctx)
		job, err := s.repository.GetScheduledTask(ctx, ownerUserID, strings.TrimSpace(jobID))
		if err != nil {
			return nil, err
		}
		if job == nil {
			return nil, automationdomain.ErrJobNotFound
		}
		if *expectedVersion < 1 || job.ConfigurationVersion != *expectedVersion {
			return nil, automationdomain.ErrConfigurationVersionConflict
		}
	}
	return s.retryRunDelivery(ctx, jobID, runID, true)
}

func (s *Service) retryRunDelivery(ctx context.Context, jobID string, runID string, recordEvent bool) (*automationdomain.ScheduledTaskRun, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	job, run, err := s.loadDeliveryRetry(ctx, ownerUserID, jobID, runID)
	if err != nil {
		return nil, err
	}
	if err = validateDeliveryRetry(*run); err != nil {
		return nil, err
	}
	update, deliveryStatus := s.buildDeliveryRetryUpdate(ctx, *job, *run)
	if err = s.repository.MarkRunDelivery(ctx, update); err != nil {
		return nil, err
	}
	s.invalidateDeliveryRetryDeadline()
	s.updateJobLastDeliveryStatus(*job, deliveryStatus)

	updated, err := s.loadRetriedRun(ctx, ownerUserID, job.JobID, run.RunID)
	if err != nil {
		return nil, err
	}
	if recordEvent && updated != nil {
		s.recordTaskEvent(ctx, automationdomain.TaskEventActionRetryDelivery, *job, run.RunID, deliveryRetryTaskEventDetail(*updated))
	}
	if updated != nil && run.DeliveryDeadLetterAt == nil && updated.DeliveryDeadLetterAt != nil {
		s.notifyAutomationDeliveryDeadLetter(ctx, *job, *updated)
	}
	return updated, nil
}

func (s *Service) loadDeliveryRetry(ctx context.Context, ownerUserID string, jobID string, runID string) (*automationdomain.ScheduledTask, *automationdomain.ScheduledTaskRun, error) {
	job, err := s.repository.GetScheduledTask(ctx, ownerUserID, strings.TrimSpace(jobID))
	if err != nil {
		return nil, nil, err
	}
	if job == nil {
		return nil, nil, automationdomain.ErrJobNotFound
	}
	if err = rejectAgentScriptControl(ctx, *job); err != nil {
		return nil, nil, err
	}
	if job.SessionBindingState == automationdomain.TaskSessionBindingStateRebindRequired {
		return nil, nil, automationdomain.ErrTaskSessionRebindRequired
	}
	run, err := s.repository.GetRun(ctx, ownerUserID, job.JobID, strings.TrimSpace(runID))
	if errors.Is(err, sql.ErrNoRows) || (err == nil && run == nil) {
		return nil, nil, automationdomain.ErrRunNotFound
	}
	return job, run, err
}

func validateDeliveryRetry(run automationdomain.ScheduledTaskRun) error {
	runStatus := strings.TrimSpace(run.Status)
	if runStatus == automationdomain.RunStatusPending ||
		runStatus == automationdomain.RunStatusRunning ||
		runStatus == automationdomain.RunStatusQueuedToMain {
		return errors.New("run is not finished")
	}
	deliveryStatus := strings.TrimSpace(run.DeliveryStatus)
	if deliveryStatus != automationdomain.DeliveryStatusFailed {
		return fmt.Errorf("run delivery_status must be failed before retrying delivery, got %q", deliveryStatus)
	}
	return nil
}

func (s *Service) buildDeliveryRetryUpdate(ctx context.Context, job automationdomain.ScheduledTask, run automationdomain.ScheduledTaskRun) (automationstore.RunDeliveryUpdateInput, string) {
	observation := automationexec.ExecutionObservation{
		RunID:         run.RunID,
		RoundID:       run.RoundID,
		Status:        automationdomain.RunStatusSucceeded,
		SessionID:     run.SessionID,
		MessageCount:  run.MessageCount,
		ResultText:    anyStringPointer(run.ResultText),
		AssistantText: anyStringPointer(run.AssistantText),
	}
	// 一次执行的首投递使用 run 启动时快照；进入 failed 后，用户对任务目标的
	// 显式修正就是恢复动作，人工和到期重试都应使用当前目标，避免坏路由被永久冻结。
	runDelivery := job.Delivery.Normalized()
	deliveryResult := s.deliverJobObservationToTarget(
		contextForJobOwner(ctx, job),
		job,
		runDelivery,
		run.SessionKey,
		observation,
	)
	deliveryStatus := deliveryResult.Status
	deliveryError := deliveryResult.Error
	deliveryTo := deliveryResult.deliveryTo(runDelivery)
	now := s.nowFn()
	deliveredAt := deliveredAtForStatus(deliveryStatus, now)
	attempted := deliveryAttempted(deliveryStatus)
	attemptsAfter := run.DeliveryAttempts
	if attempted {
		attemptsAfter++
	}
	nextDeliveryAttemptAt, deliveryDeadLetterAt := deliveryRetrySchedule(deliveryStatus, attemptsAfter, now)
	return automationstore.RunDeliveryUpdateInput{
		RunID:                 run.RunID,
		DeliveryMode:          strings.TrimSpace(runDelivery.Mode),
		DeliveryTo:            deliveryTo,
		DeliveryStatus:        deliveryStatus,
		DeliveryError:         deliveryError,
		DeliveredAt:           deliveredAt,
		DeliveryAttempted:     attempted,
		DeliveryNextAttemptAt: nextDeliveryAttemptAt,
		DeliveryDeadLetterAt:  deliveryDeadLetterAt,
	}, deliveryStatus
}

func (s *Service) loadRetriedRun(ctx context.Context, ownerUserID string, jobID string, runID string) (*automationdomain.ScheduledTaskRun, error) {
	updated, err := s.repository.GetRun(ctx, ownerUserID, jobID, runID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, automationdomain.ErrRunNotFound
	}
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// RecoverTaskRunningRun 手动释放任务当前运行占用，并把未完成 run 标记为取消。
func (s *Service) RecoverTaskRunningRun(ctx context.Context, jobID string, runID string) (*automationdomain.ScheduledTask, error) {
	s.taskControlMu.Lock()
	defer s.taskControlMu.Unlock()

	current, err := s.GetTask(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, automationdomain.ErrJobNotFound
	}
	if err = rejectAgentScriptControl(ctx, *current); err != nil {
		return nil, err
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
	result := s.scheduledTaskRuntimeSnapshot(recovered, state)
	s.recordTaskEvent(ctx, automationdomain.TaskEventActionRecover, result, currentRunID, map[string]any{"recovered_run_id": currentRunID})
	return &result, nil
}

func anyExecutionStatus(result *automationdomain.ExecutionResult) string {
	if result == nil {
		return ""
	}
	return strings.TrimSpace(result.Status)
}

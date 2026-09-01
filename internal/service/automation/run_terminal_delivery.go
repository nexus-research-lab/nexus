// INPUT: Agent/Script/Main Session 的 terminal execution observation 与 frozen run route。
// OUTPUT: execution/runtime 原子提交后，以 durable attempt claim 驱动的首次投递。
// POS: Automation 首投递两阶段唯一编排入口；任何外投都晚于 terminal commit。
package automation

import (
	"context"
	"errors"
	"strings"
	"time"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

func (s *Service) commitObservedRunTerminal(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	finish automationstore.RunFinishInput,
) (*automationdomain.ScheduledTaskRun, bool, error) {
	runtime := s.plannedFinishedJobRuntime(
		job,
		finish.RunID,
		&finish.FinishedAt,
		finish.Status,
		finish.ErrorMessage,
		finish.DeliveryStatus,
	)
	result, err := s.repository.CommitRunTerminalAndRuntime(ctx, automationstore.RunTerminalCommitInput{
		OwnerUserID:                  job.OwnerUserID,
		JobID:                        job.JobID,
		ExpectedConfigurationVersion: job.ConfigurationVersion,
		Finish:                       finish,
		Runtime:                      runtime,
	})
	if shouldCommitDeletingRunTerminal(err) {
		return s.commitDeletingRunTerminal(ctx, job, finish)
	}
	if err != nil || !result.Committed {
		return nil, false, err
	}
	s.refreshTaskRuntimeProjection(ctx, job)
	run, err := s.repository.GetRun(ctx, job.OwnerUserID, job.JobID, finish.RunID)
	if err != nil {
		return nil, true, err
	}
	if strings.TrimSpace(run.DeliveryStatus) != automationdomain.DeliveryStatusPending {
		s.wakeScheduler()
		return run, true, nil
	}
	s.invalidateDeliveryRetryDeadline()
	updated, deliveryErr := s.deliverPendingRun(ctx, job, *run)
	return updated, true, deliveryErr
}

func shouldCommitDeletingRunTerminal(err error) bool {
	return errors.Is(err, automationdomain.ErrRunCompletionConflict) ||
		errors.Is(err, automationdomain.ErrTaskDeleting)
}

func (s *Service) commitDeletingRunTerminal(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	finish automationstore.RunFinishInput,
) (*automationdomain.ScheduledTaskRun, bool, error) {
	current, err := s.repository.GetScheduledTask(ctx, job.OwnerUserID, job.JobID)
	if err != nil {
		return nil, false, err
	}
	if current == nil || strings.TrimSpace(current.DeletionState) == "" ||
		strings.TrimSpace(current.DeletionToken) == "" {
		return nil, false, automationdomain.ErrRunCompletionConflict
	}
	if err = s.repository.CommitDeletingRunTerminal(ctx, automationstore.DeletingRunTerminalCommitInput{
		OwnerUserID:   job.OwnerUserID,
		JobID:         job.JobID,
		DeletionToken: current.DeletionToken,
		Finish:        finish,
	}); err != nil {
		return nil, false, err
	}
	s.mu.Lock()
	delete(s.jobStates, job.JobID)
	s.mu.Unlock()
	run, err := s.repository.GetRun(ctx, job.OwnerUserID, job.JobID, finish.RunID)
	if err != nil {
		return nil, true, err
	}
	s.wakeScheduler()
	return run, true, nil
}

// commitFailedRunTerminal shares the exact terminal/runtime transaction with
// successful Agent, Script, and Main Session observations. Setup failures after
// a run ledger exists therefore cannot leave a terminal run occupying the task.
func (s *Service) commitFailedRunTerminal(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	runID string,
	runErr error,
) error {
	finishedAt := s.nowFn()
	_, committed, err := s.commitObservedRunTerminal(ctx, job, automationstore.RunFinishInput{
		RunID:          strings.TrimSpace(runID),
		Status:         automationdomain.RunStatusFailed,
		FinishedAt:     finishedAt,
		ErrorMessage:   errorPointer(runErr),
		DeliveryStatus: automationdomain.DeliveryStatusNotAttempted,
	})
	if err != nil {
		return err
	}
	if !committed {
		return automationdomain.ErrRunCompletionConflict
	}
	return nil
}

func (s *Service) plannedFinishedJobRuntime(
	job automationdomain.ScheduledTask,
	runID string,
	finishedAt *time.Time,
	status string,
	errorMessage *string,
	deliveryStatus string,
) automationstore.JobRuntimeUpdateInput {
	state := s.ensureJobState(job)
	s.mu.Lock()
	planned := *state
	planned.NextRunAt = cloneTimePointer(state.NextRunAt)
	planned.RunningStartedAt = cloneTimePointer(state.RunningStartedAt)
	planned.LastRunAt = cloneTimePointer(state.LastRunAt)
	planned.LastError = cloneStringPointer(state.LastError)
	s.mu.Unlock()
	normalizeFinishedRunState(&planned, finishedAt, status, errorMessage, []string{deliveryStatus})
	if strings.TrimSpace(planned.RunningRunID) == strings.TrimSpace(runID) {
		planned.RunningRunID = ""
		planned.RunningStartedAt = nil
	}
	now := s.nowFn()
	naturalNext := s.naturalNextRunAt(&planned, now)
	applyFinishedRunOutcome(&planned, strings.TrimSpace(status), now, naturalNext)
	return jobRuntimeUpdateFromState(job.JobID, &planned)
}

func (s *Service) refreshTaskRuntimeProjection(ctx context.Context, job automationdomain.ScheduledTask) {
	current, err := s.repository.GetScheduledTask(ctx, job.OwnerUserID, job.JobID)
	if err != nil {
		s.loggerFor(ctx).Warn("刷新任务完成态投影失败", "job_id", job.JobID, "err", err)
		return
	}
	if current == nil {
		s.mu.Lock()
		delete(s.jobStates, job.JobID)
		s.mu.Unlock()
		return
	}
	s.replacePersistedJobRuntimeState(*current)
}

func (s *Service) deliverPendingRun(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	run automationdomain.ScheduledTaskRun,
) (*automationdomain.ScheduledTaskRun, error) {
	// 首投与同任务删除共享顺序，但外部 router 绝不能占用全局任务锁。
	fence := s.taskExecutionFence(job.JobID)
	fence.Lock()
	defer fence.Unlock()

	if strings.TrimSpace(run.Status) != automationdomain.RunStatusSucceeded ||
		strings.TrimSpace(run.DeliveryStatus) != automationdomain.DeliveryStatusPending {
		return nil, automationdomain.ErrDeliveryRetryConflict
	}
	attemptID := automationexec.NewID("delivery_attempt")
	if err := s.repository.ClaimRunDeliveryAttempt(ctx, automationstore.RunDeliveryAttemptClaimInput{
		OwnerUserID:              run.OwnerUserID,
		JobID:                    run.JobID,
		RunID:                    run.RunID,
		ExpectedDeliveryAttempts: run.DeliveryAttempts,
		ExpectedStatus:           automationdomain.DeliveryStatusPending,
		AttemptID:                attemptID,
	}); err != nil {
		return nil, err
	}
	s.invalidateDeliveryRetryDeadline()
	target := deliveryTargetForRun(job, run)
	observation := executionObservationFromRun(run)
	deliveryResult := s.deliverJobObservationToTarget(
		contextForJobOwner(ctx, job), job, target, run.SessionKey, observation,
	)
	if deliveryResult.OutcomeUnknown {
		markErr := s.repository.MarkRunDeliveryAttemptUnconfirmed(
			ctx, run.OwnerUserID, run.JobID, run.RunID, attemptID,
			strings.TrimSpace(target.Mode), deliveryResult.deliveryTo(target), deliveryResult.Error,
		)
		s.refreshTaskRuntimeProjection(ctx, job)
		if markErr != nil {
			return nil, errors.Join(automationdomain.ErrDeliveryRetryCompletionUnconfirmed, markErr)
		}
		updated, loadErr := s.loadRetriedRun(ctx, run.OwnerUserID, run.JobID, run.RunID)
		if loadErr != nil {
			return nil, loadErr
		}
		return updated, automationdomain.ErrDeliveryRetryCompletionUnconfirmed
	}

	now := s.nowFn()
	status := deliveryResult.Status
	nextAttemptAt, deadLetterAt := deliveryRetrySchedule(status, run.DeliveryAttempts+1, now)
	completion := automationstore.RunDeliveryAttemptCompletionInput{
		OwnerUserID:           run.OwnerUserID,
		JobID:                 run.JobID,
		RunID:                 run.RunID,
		AttemptID:             attemptID,
		DeliveryMode:          strings.TrimSpace(target.Mode),
		DeliveryTo:            deliveryResult.deliveryTo(target),
		DeliveryStatus:        status,
		DeliveryError:         deliveryResult.Error,
		DeliveredAt:           deliveredAtForStatus(status, now),
		DeliveryNextAttemptAt: nextAttemptAt,
		DeliveryDeadLetterAt:  deadLetterAt,
	}
	if err := s.repository.CompleteRunDeliveryAttempt(ctx, completion); err != nil {
		return nil, err
	}
	s.invalidateDeliveryRetryDeadline()
	s.refreshTaskRuntimeProjection(ctx, job)
	return s.loadRetriedRun(ctx, run.OwnerUserID, run.JobID, run.RunID)
}

func executionObservationFromRun(run automationdomain.ScheduledTaskRun) automationexec.ExecutionObservation {
	return automationexec.ExecutionObservation{
		RunID:         run.RunID,
		RoundID:       run.RoundID,
		Status:        run.Status,
		SessionID:     run.SessionID,
		MessageCount:  run.MessageCount,
		ResultText:    anyStringPointer(run.ResultText),
		AssistantText: anyStringPointer(run.AssistantText),
	}
}

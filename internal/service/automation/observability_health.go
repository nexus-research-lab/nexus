package automation

import (
	"context"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
)

// GetTaskStatus 返回单个任务的当前状态、健康摘要和最近观测记录。
func (s *Service) GetTaskStatus(ctx context.Context, jobID string, runLimit int, eventLimit int) (*automationdomain.ScheduledTaskStatus, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	job, err := s.GetTask(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, automationdomain.ErrJobNotFound
	}
	runs, err := s.ListTaskRuns(ctx, job.JobID)
	if err != nil {
		return nil, err
	}
	events, err := s.ListTaskEvents(ctx, job.JobID, boundedObservabilityLimit(eventLimit, 10, 50))
	if err != nil {
		return nil, err
	}
	runs = limitObservabilityRuns(runs, boundedObservabilityLimit(runLimit, 10, 50))
	return &automationdomain.ScheduledTaskStatus{
		Job:          *job,
		Health:       s.buildScheduledTaskHealth(*job, runs),
		RecentRuns:   runs,
		RecentEvents: events,
	}, nil
}

func (s *Service) buildScheduledTaskHealth(job automationdomain.ScheduledTask, runs []automationdomain.ScheduledTaskRun) automationdomain.ScheduledTaskHealth {
	runningRunID := strings.TrimSpace(job.RunningRunID)
	health := automationdomain.ScheduledTaskHealth{
		State:             "scheduled",
		RecoveryAvailable: runningRunID != "",
		RecoveryRunID:     runningRunID,
	}
	s.applyScheduledTaskStateHealth(&health, job)
	for _, run := range runs {
		observeScheduledTaskRunHealth(&health, run)
	}
	finalizeScheduledTaskHealth(&health)
	return health
}

func (s *Service) applyScheduledTaskStateHealth(
	health *automationdomain.ScheduledTaskHealth,
	job automationdomain.ScheduledTask,
) {
	if !job.Enabled {
		health.State = "disabled"
	}
	if job.Running {
		health.State = "running"
		addTaskHealthSignal(health, "running")
		if job.RunningStartedAt != nil {
			health.RunningForSeconds = int64(s.nowFn().UTC().Sub(job.RunningStartedAt.UTC()).Seconds())
		}
		addTaskHealthSuggestedTool(health, "repair_scheduled_task")
	}
	if stringPointerHasText(job.LastError) || job.FailureStreak > 0 || isFailedRunStatus(job.LastRunStatus) {
		addTaskHealthSignal(health, "execution_attention")
		addExecutionRepairSuggestedTools(&health.SuggestedTools)
		setFirstStringPointer(&health.LatestExecutionError, job.LastError)
		markScheduledTaskHealthAttention(health)
	}
}

func observeScheduledTaskRunHealth(
	health *automationdomain.ScheduledTaskHealth,
	run automationdomain.ScheduledTaskRun,
) {
	if isFailedRunStatus(run.Status) {
		health.FailedRunCount++
	}
	if strings.TrimSpace(run.Status) == automationdomain.RunStatusFailed {
		addUniqueString(&health.ExecutionFailedRunIDs, run.RunID)
		setFirstStringPointer(&health.LatestExecutionError, run.ErrorMessage)
	}
	switch deriveTaskRunDeliveryStatus(run) {
	case automationdomain.DeliveryStatusFailed:
		health.DeliveryFailedRunCount++
		health.ManualRedeliveryAvailable = true
		addUniqueString(&health.ManualRedeliveryRunIDs, run.RunID)
		setFirstStringPointer(&health.LatestDeliveryError, preferredDeliveryError(run))
	case automationdomain.DeliveryStatusPending:
		health.DeliveryPendingRunCount++
		addUniqueString(&health.DeliveryPendingRunIDs, run.RunID)
	case automationdomain.DeliveryStatusSkipped:
		health.DeliverySkippedRunCount++
		addUniqueString(&health.DeliverySkippedRunIDs, run.RunID)
	}
	if run.DeliveryDeadLetterAt != nil {
		health.DeliveryDeadLetterCount++
		health.ManualRedeliveryAvailable = true
		addUniqueString(&health.DeliveryDeadLetterRunIDs, run.RunID)
		setFirstStringPointer(&health.LatestDeliveryError, preferredDeliveryError(run))
	}
}

func finalizeScheduledTaskHealth(health *automationdomain.ScheduledTaskHealth) {
	if health.FailedRunCount > 0 {
		addTaskHealthSignal(health, "recent_execution_failed")
		addExecutionRepairSuggestedTools(&health.SuggestedTools)
		markScheduledTaskHealthAttention(health)
	}
	if health.DeliveryFailedRunCount > 0 || health.DeliveryDeadLetterCount > 0 {
		addTaskHealthSignal(health, "delivery_attention")
		addTaskHealthSuggestedTool(health, "repair_scheduled_task")
		markScheduledTaskHealthAttention(health)
	}
	if health.DeliveryPendingRunCount > 0 {
		addTaskHealthSignal(health, "delivery_pending")
	}
	if health.DeliverySkippedRunCount > 0 {
		addTaskHealthSignal(health, "delivery_skipped")
	}
}

func markScheduledTaskHealthAttention(health *automationdomain.ScheduledTaskHealth) {
	if health.State == "scheduled" {
		health.State = "attention"
	}
}

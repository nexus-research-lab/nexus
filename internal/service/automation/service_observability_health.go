package automation

import (
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
)

func (s *Service) buildCronTaskHealth(job automationdomain.CronJob, runs []automationdomain.CronRun) automationdomain.CronTaskHealth {
	runningRunID := strings.TrimSpace(job.RunningRunID)
	health := automationdomain.CronTaskHealth{
		State:             "scheduled",
		RecoveryAvailable: runningRunID != "",
		RecoveryRunID:     runningRunID,
	}
	if !job.Enabled {
		health.State = "disabled"
	}
	if job.Running {
		health.State = "running"
		addTaskHealthSignal(&health, "running")
		if job.RunningStartedAt != nil {
			health.RunningForSeconds = int64(s.nowFn().UTC().Sub(job.RunningStartedAt.UTC()).Seconds())
		}
		addTaskHealthSuggestedTool(&health, "recover_scheduled_task")
	}
	if stringPointerHasText(job.LastError) || job.FailureStreak > 0 || isFailedRunStatus(job.LastRunStatus) {
		addTaskHealthSignal(&health, "execution_attention")
		addExecutionRepairSuggestedTools(&health.SuggestedTools)
		setFirstStringPointer(&health.LatestExecutionError, job.LastError)
		if health.State == "scheduled" {
			health.State = "attention"
		}
	}
	for _, run := range runs {
		if isFailedRunStatus(run.Status) {
			health.FailedRunCount++
		}
		if strings.TrimSpace(run.Status) == automationdomain.RunStatusFailed {
			addUniqueString(&health.ExecutionFailedRunIDs, run.RunID)
			setFirstStringPointer(&health.LatestExecutionError, run.ErrorMessage)
		}
		deliveryStatus := deriveCronRunDeliveryStatus(run)
		switch deliveryStatus {
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
	if health.FailedRunCount > 0 {
		addTaskHealthSignal(&health, "recent_execution_failed")
		addExecutionRepairSuggestedTools(&health.SuggestedTools)
		if health.State == "scheduled" {
			health.State = "attention"
		}
	}
	if health.DeliveryFailedRunCount > 0 || health.DeliveryDeadLetterCount > 0 {
		addTaskHealthSignal(&health, "delivery_attention")
		addTaskHealthSuggestedTool(&health, "retry_scheduled_task_delivery")
		if health.State == "scheduled" {
			health.State = "attention"
		}
	}
	if health.DeliveryPendingRunCount > 0 {
		addTaskHealthSignal(&health, "delivery_pending")
	}
	if health.DeliverySkippedRunCount > 0 {
		addTaskHealthSignal(&health, "delivery_skipped")
	}
	return health
}

package automation

import (
	"slices"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation"
)

func deriveCronRunDeliveryStatus(run automationdomain.CronRun) string {
	if deliveryStatus := strings.TrimSpace(run.DeliveryStatus); deliveryStatus != "" {
		return deliveryStatus
	}
	mode := strings.TrimSpace(run.DeliveryMode)
	if mode == "" || mode == automationdomain.DeliveryModeNone {
		return automationdomain.DeliveryStatusNotRequired
	}
	switch strings.TrimSpace(run.Status) {
	case automationdomain.RunStatusPending, automationdomain.RunStatusRunning:
		return automationdomain.DeliveryStatusPending
	case automationdomain.RunStatusSucceeded, automationdomain.RunStatusQueuedToMain:
		return automationdomain.DeliveryStatusSucceeded
	case automationdomain.RunStatusFailed:
		if looksLikeDeliveryRuntimeError(run.ErrorMessage) {
			return automationdomain.DeliveryStatusFailed
		}
		return automationdomain.DeliveryStatusNotAttempted
	case automationdomain.RunStatusCancelled, automationdomain.RunStatusSkipped:
		return automationdomain.DeliveryStatusNotAttempted
	default:
		return automationdomain.DeliveryStatusPending
	}
}

func looksLikeDeliveryRuntimeError(message *string) bool {
	if message == nil {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(*message))
	if text == "" {
		return false
	}
	for _, marker := range []string{"delivery", "router", "channel", "投递", "发送", "feishu", "telegram", "discord", "websocket"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func addTaskHealthSignal(health *automationdomain.CronTaskHealth, signal string) {
	addUniqueString(&health.Signals, signal)
}

func addTaskHealthSuggestedTool(health *automationdomain.CronTaskHealth, name string) {
	addUniqueString(&health.SuggestedTools, name)
}

func addExecutionRepairSuggestedTools(items *[]string) {
	addUniqueString(items, "update_scheduled_task")
	addUniqueString(items, "run_scheduled_task")
}

func addDailyReportTaskRunSignals(task *automationdomain.CronDailyReportTask, run automationdomain.CronRun) {
	if isFailedRunStatus(run.Status) {
		addDailyReportTaskSignal(task, "recent_execution_failed")
	}
	if strings.TrimSpace(run.Status) == automationdomain.RunStatusFailed {
		if !task.Deleted {
			addDailyReportExecutionRepairSuggestedTools(task)
		}
		addUniqueString(&task.ExecutionFailedRunIDs, run.RunID)
		setFirstStringPointer(&task.LatestExecutionError, run.ErrorMessage)
	}
	switch deriveCronRunDeliveryStatus(run) {
	case automationdomain.DeliveryStatusFailed:
		addDailyReportTaskSignal(task, "delivery_attention")
		if !task.Deleted {
			addDailyReportTaskSuggestedTool(task, "retry_scheduled_task_delivery")
			addUniqueString(&task.ManualRedeliveryRunIDs, run.RunID)
		}
		setFirstStringPointer(&task.LatestDeliveryError, preferredDeliveryError(run))
	case automationdomain.DeliveryStatusPending:
		addDailyReportTaskSignal(task, "delivery_pending")
		addUniqueString(&task.DeliveryPendingRunIDs, run.RunID)
	case automationdomain.DeliveryStatusSkipped:
		addDailyReportTaskSignal(task, "delivery_skipped")
		addUniqueString(&task.DeliverySkippedRunIDs, run.RunID)
	}
	if run.DeliveryDeadLetterAt != nil {
		addDailyReportTaskSignal(task, "delivery_attention")
		if !task.Deleted {
			addDailyReportTaskSuggestedTool(task, "retry_scheduled_task_delivery")
		}
		addUniqueString(&task.DeliveryDeadLetterRunIDs, run.RunID)
		setFirstStringPointer(&task.LatestDeliveryError, preferredDeliveryError(run))
	}
}

func addDailyReportTaskSignal(task *automationdomain.CronDailyReportTask, signal string) {
	addUniqueString(&task.Signals, signal)
}

func addDailyReportTaskSuggestedTool(task *automationdomain.CronDailyReportTask, name string) {
	addUniqueString(&task.SuggestedTools, name)
}

func addDailyReportExecutionRepairSuggestedTools(task *automationdomain.CronDailyReportTask) {
	addDailyReportTaskSuggestedTool(task, "update_scheduled_task")
	addDailyReportTaskSuggestedTool(task, "run_scheduled_task")
}

func addUniqueString(items *[]string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	if slices.Contains(*items, value) {
		return
	}
	*items = append(*items, value)
}

func setFirstStringPointer(target **string, value *string) {
	if *target != nil || !stringPointerHasText(value) {
		return
	}
	text := strings.TrimSpace(*value)
	*target = &text
}

func preferredDeliveryError(run automationdomain.CronRun) *string {
	if stringPointerHasText(run.DeliveryError) {
		return run.DeliveryError
	}
	if looksLikeDeliveryRuntimeError(run.ErrorMessage) {
		return run.ErrorMessage
	}
	return nil
}

func isFailedRunStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case automationdomain.RunStatusFailed, automationdomain.RunStatusCancelled:
		return true
	default:
		return false
	}
}

func stringPointerHasText(value *string) bool {
	return value != nil && strings.TrimSpace(*value) != ""
}

func boundedObservabilityLimit(value int, defaultValue int, maxValue int) int {
	if value <= 0 {
		return defaultValue
	}
	return min(value, maxValue)
}

func limitObservabilityRuns(runs []automationdomain.CronRun, limit int) []automationdomain.CronRun {
	if limit <= 0 || len(runs) <= limit {
		return runs
	}
	return runs[:limit]
}

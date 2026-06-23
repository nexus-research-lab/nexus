package automation

import (
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func deriveCronRunDeliveryStatus(run protocol.CronRun) string {
	if deliveryStatus := strings.TrimSpace(run.DeliveryStatus); deliveryStatus != "" {
		return deliveryStatus
	}
	mode := strings.TrimSpace(run.DeliveryMode)
	if mode == "" || mode == protocol.DeliveryModeNone {
		return protocol.DeliveryStatusNotRequired
	}
	switch strings.TrimSpace(run.Status) {
	case protocol.RunStatusPending, protocol.RunStatusRunning:
		return protocol.DeliveryStatusPending
	case protocol.RunStatusSucceeded, protocol.RunStatusQueuedToMain:
		return protocol.DeliveryStatusSucceeded
	case protocol.RunStatusFailed:
		if looksLikeDeliveryRuntimeError(run.ErrorMessage) {
			return protocol.DeliveryStatusFailed
		}
		return protocol.DeliveryStatusNotAttempted
	case protocol.RunStatusCancelled, protocol.RunStatusSkipped:
		return protocol.DeliveryStatusNotAttempted
	default:
		return protocol.DeliveryStatusPending
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

func addTaskHealthSignal(health *protocol.CronTaskHealth, signal string) {
	addUniqueString(&health.Signals, signal)
}

func addTaskHealthSuggestedTool(health *protocol.CronTaskHealth, name string) {
	addUniqueString(&health.SuggestedTools, name)
}

func addExecutionRepairSuggestedTools(items *[]string) {
	addUniqueString(items, "update_scheduled_task")
	addUniqueString(items, "run_scheduled_task")
}

func addDailyReportTaskRunSignals(task *protocol.CronDailyReportTask, run protocol.CronRun) {
	if isFailedRunStatus(run.Status) {
		addDailyReportTaskSignal(task, "recent_execution_failed")
	}
	if strings.TrimSpace(run.Status) == protocol.RunStatusFailed {
		if !task.Deleted {
			addDailyReportExecutionRepairSuggestedTools(task)
		}
		addUniqueString(&task.ExecutionFailedRunIDs, run.RunID)
		setFirstStringPointer(&task.LatestExecutionError, run.ErrorMessage)
	}
	switch deriveCronRunDeliveryStatus(run) {
	case protocol.DeliveryStatusFailed:
		addDailyReportTaskSignal(task, "delivery_attention")
		if !task.Deleted {
			addDailyReportTaskSuggestedTool(task, "retry_scheduled_task_delivery")
			addUniqueString(&task.ManualRedeliveryRunIDs, run.RunID)
		}
		setFirstStringPointer(&task.LatestDeliveryError, preferredDeliveryError(run))
	case protocol.DeliveryStatusPending:
		addDailyReportTaskSignal(task, "delivery_pending")
		addUniqueString(&task.DeliveryPendingRunIDs, run.RunID)
	case protocol.DeliveryStatusSkipped:
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

func addDailyReportTaskSignal(task *protocol.CronDailyReportTask, signal string) {
	addUniqueString(&task.Signals, signal)
}

func addDailyReportTaskSuggestedTool(task *protocol.CronDailyReportTask, name string) {
	addUniqueString(&task.SuggestedTools, name)
}

func addDailyReportExecutionRepairSuggestedTools(task *protocol.CronDailyReportTask) {
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

func preferredDeliveryError(run protocol.CronRun) *string {
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
	case protocol.RunStatusFailed, protocol.RunStatusCancelled:
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

func limitObservabilityRuns(runs []protocol.CronRun, limit int) []protocol.CronRun {
	if limit <= 0 || len(runs) <= limit {
		return runs
	}
	return runs[:limit]
}

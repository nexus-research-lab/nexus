package types

import "time"

// ScheduledTaskDailyReportTotals 表示定时任务日报聚合计数。
type ScheduledTaskDailyReportTotals struct {
	TaskCount                  int `json:"task_count"`
	EnabledTaskCount           int `json:"enabled_task_count"`
	RunningTaskCount           int `json:"running_task_count"`
	RunCount                   int `json:"run_count"`
	SucceededRunCount          int `json:"succeeded_run_count"`
	FailedRunCount             int `json:"failed_run_count"`
	CancelledRunCount          int `json:"cancelled_run_count"`
	SkippedRunCount            int `json:"skipped_run_count"`
	DeliveredRunCount          int `json:"delivered_run_count"`
	DeliveryFailedRunCount     int `json:"delivery_failed_run_count"`
	DeliveryPendingRunCount    int `json:"delivery_pending_run_count"`
	DeliverySkippedRunCount    int `json:"delivery_skipped_run_count"`
	DeliveryDeadLetterRunCount int `json:"delivery_dead_letter_run_count"`
	DeliveryNotNeededCount     int `json:"delivery_not_needed_count"`
	DeliveryNotAttemptedCount  int `json:"delivery_not_attempted_count"`
}

// ScheduledTaskDailyReportItem 表示日报里单个任务的运行与投递情况。
type ScheduledTaskDailyReportItem struct {
	JobID                    string                         `json:"job_id"`
	Name                     string                         `json:"name"`
	AgentID                  string                         `json:"agent_id"`
	Deleted                  bool                           `json:"deleted,omitempty"`
	Enabled                  bool                           `json:"enabled"`
	Running                  bool                           `json:"running"`
	RunningRunID             string                         `json:"running_run_id,omitempty"`
	RecoveryRunID            string                         `json:"recovery_run_id,omitempty"`
	NextRunAt                *time.Time                     `json:"next_run_at,omitempty"`
	LastRunAt                *time.Time                     `json:"last_run_at,omitempty"`
	LastRunStatus            string                         `json:"last_run_status,omitempty"`
	LastDeliveryStatus       string                         `json:"last_delivery_status,omitempty"`
	FailureStreak            int                            `json:"failure_streak,omitempty"`
	LastError                *string                        `json:"last_error,omitempty"`
	LatestExecutionError     *string                        `json:"latest_execution_error,omitempty"`
	LatestDeliveryError      *string                        `json:"latest_delivery_error,omitempty"`
	Signals                  []string                       `json:"signals,omitempty"`
	SuggestedTools           []string                       `json:"suggested_tools,omitempty"`
	ExecutionFailedRunIDs    []string                       `json:"execution_failed_run_ids,omitempty"`
	ManualRedeliveryRunIDs   []string                       `json:"manual_redelivery_run_ids,omitempty"`
	DeliveryPendingRunIDs    []string                       `json:"delivery_pending_run_ids,omitempty"`
	DeliverySkippedRunIDs    []string                       `json:"delivery_skipped_run_ids,omitempty"`
	DeliveryDeadLetterRunIDs []string                       `json:"delivery_dead_letter_run_ids,omitempty"`
	Runs                     []ScheduledTaskRun             `json:"runs"`
	Totals                   ScheduledTaskDailyReportTotals `json:"totals"`
}

// ScheduledTaskDailyReport 表示指定日期的任务运行和投递日报。
type ScheduledTaskDailyReport struct {
	Date     string                         `json:"date"`
	Timezone string                         `json:"timezone"`
	AgentID  string                         `json:"agent_id,omitempty"`
	JobID    string                         `json:"job_id,omitempty"`
	StartAt  time.Time                      `json:"start_at"`
	EndAt    time.Time                      `json:"end_at"`
	Totals   ScheduledTaskDailyReportTotals `json:"totals"`
	Tasks    []ScheduledTaskDailyReportItem `json:"tasks"`
}

// ScheduledTaskDailyReportInput 表示日报查询输入。
type ScheduledTaskDailyReportInput struct {
	Date     string
	Timezone string
	AgentID  string
	JobID    string
}

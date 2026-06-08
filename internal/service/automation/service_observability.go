package automation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// GetTaskStatus 返回单个任务的当前状态、健康摘要和最近观测记录。
func (s *Service) GetTaskStatus(ctx context.Context, jobID string, runLimit int, eventLimit int) (*protocol.CronTaskStatus, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	job, err := s.GetTask(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, protocol.ErrJobNotFound
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
	return &protocol.CronTaskStatus{
		Job:          *job,
		Health:       s.buildCronTaskHealth(*job, runs),
		RecentRuns:   runs,
		RecentEvents: events,
	}, nil
}

// GetDailyReport 按日期聚合任务运行和投递状态。
func (s *Service) GetDailyReport(ctx context.Context, input protocol.CronDailyReportInput) (*protocol.CronDailyReport, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	timezone := firstNonEmpty(strings.TrimSpace(input.Timezone), strings.TrimSpace(s.config.DefaultTimezone), "Asia/Shanghai")
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone: %s", timezone)
	}
	date, startAt, endAt, err := resolveDailyReportDate(input.Date, loc, s.nowFn())
	if err != nil {
		return nil, err
	}

	jobID := strings.TrimSpace(input.JobID)
	agentID := strings.TrimSpace(input.AgentID)
	var jobs []protocol.CronJob
	if jobID != "" {
		job, getErr := s.GetTask(ctx, jobID)
		if getErr != nil {
			return nil, getErr
		}
		if job == nil {
			task, taskErr := s.buildDeletedDailyReportTask(ctx, jobID, startAt, endAt)
			if taskErr != nil {
				return nil, taskErr
			}
			report := &protocol.CronDailyReport{
				Date:     date,
				Timezone: timezone,
				AgentID:  task.AgentID,
				JobID:    jobID,
				StartAt:  startAt,
				EndAt:    endAt,
				Tasks:    []protocol.CronDailyReportTask{task},
			}
			addDailyReportTotals(&report.Totals, task.Totals)
			report.Totals.TaskCount = 1
			return report, nil
		}
		jobs = []protocol.CronJob{*job}
		agentID = strings.TrimSpace(job.AgentID)
	} else {
		jobs, err = s.ListTasks(ctx, agentID)
		if err != nil {
			return nil, err
		}
	}

	report := &protocol.CronDailyReport{
		Date:     date,
		Timezone: timezone,
		AgentID:  agentID,
		JobID:    jobID,
		StartAt:  startAt,
		EndAt:    endAt,
		Tasks:    make([]protocol.CronDailyReportTask, 0, len(jobs)),
	}
	for _, job := range jobs {
		task, taskErr := s.buildDailyReportTask(ctx, job, startAt, endAt)
		if taskErr != nil {
			return nil, taskErr
		}
		addDailyReportTotals(&report.Totals, task.Totals)
		report.Totals.TaskCount++
		if task.Enabled {
			report.Totals.EnabledTaskCount++
		}
		if task.Running {
			report.Totals.RunningTaskCount++
		}
		report.Tasks = append(report.Tasks, task)
	}
	return report, nil
}

func (s *Service) buildDeletedDailyReportTask(
	ctx context.Context,
	jobID string,
	startAt time.Time,
	endAt time.Time,
) (protocol.CronDailyReportTask, error) {
	ownerUserID, _ := scopedOwnerUserID(ctx)
	normalizedJobID := strings.TrimSpace(jobID)
	runs, err := s.repository.ListRunsByJob(ctx, ownerUserID, normalizedJobID)
	if err != nil {
		return protocol.CronDailyReportTask{}, err
	}
	events, err := s.repository.ListTaskEventsByJob(ctx, ownerUserID, normalizedJobID, 50)
	if err != nil {
		return protocol.CronDailyReportTask{}, err
	}
	if len(runs) == 0 && len(events) == 0 {
		return protocol.CronDailyReportTask{}, protocol.ErrJobNotFound
	}
	task := deletedDailyReportTaskFromEvents(normalizedJobID, events)
	for _, run := range runs {
		if !cronRunFallsInRange(run, startAt, endAt) {
			continue
		}
		run.DeliveryStatus = deriveCronRunDeliveryStatus(run)
		task.Runs = append(task.Runs, run)
		addDailyReportRun(&task.Totals, run)
		addDailyReportTaskRunSignals(&task, run)
	}
	return task, nil
}

func deletedDailyReportTaskFromEvents(jobID string, events []protocol.CronTaskEvent) protocol.CronDailyReportTask {
	task := protocol.CronDailyReportTask{
		JobID:   jobID,
		Name:    jobID,
		Deleted: true,
		Enabled: false,
		Runs:    []protocol.CronRun{},
	}
	addDailyReportTaskSignal(&task, "deleted")
	addDailyReportTaskSuggestedTool(&task, "get_scheduled_task_events")
	for _, event := range events {
		if strings.TrimSpace(task.AgentID) == "" {
			task.AgentID = strings.TrimSpace(event.AgentID)
		}
		if name := stringFromTaskEventDetail(event.Detail, "name"); name != "" {
			task.Name = name
		}
	}
	return task
}

func stringFromTaskEventDetail(detail map[string]any, key string) string {
	value, ok := detail[key]
	if !ok {
		return ""
	}
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func (s *Service) buildCronTaskHealth(job protocol.CronJob, runs []protocol.CronRun) protocol.CronTaskHealth {
	runningRunID := strings.TrimSpace(job.RunningRunID)
	health := protocol.CronTaskHealth{
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
		if strings.TrimSpace(run.Status) == protocol.RunStatusFailed {
			addUniqueString(&health.ExecutionFailedRunIDs, run.RunID)
			setFirstStringPointer(&health.LatestExecutionError, run.ErrorMessage)
		}
		deliveryStatus := deriveCronRunDeliveryStatus(run)
		switch deliveryStatus {
		case protocol.DeliveryStatusFailed:
			health.DeliveryFailedRunCount++
			health.ManualRedeliveryAvailable = true
			addUniqueString(&health.ManualRedeliveryRunIDs, run.RunID)
			setFirstStringPointer(&health.LatestDeliveryError, preferredDeliveryError(run))
		case protocol.DeliveryStatusPending:
			health.DeliveryPendingRunCount++
			addUniqueString(&health.DeliveryPendingRunIDs, run.RunID)
		case protocol.DeliveryStatusSkipped:
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

func (s *Service) buildDailyReportTask(
	ctx context.Context,
	job protocol.CronJob,
	startAt time.Time,
	endAt time.Time,
) (protocol.CronDailyReportTask, error) {
	runs, err := s.ListTaskRuns(ctx, job.JobID)
	if err != nil {
		return protocol.CronDailyReportTask{}, err
	}
	runningRunID := strings.TrimSpace(job.RunningRunID)
	task := protocol.CronDailyReportTask{
		JobID:              job.JobID,
		Name:               job.Name,
		AgentID:            job.AgentID,
		Enabled:            job.Enabled,
		Running:            job.Running,
		RunningRunID:       runningRunID,
		RecoveryRunID:      runningRunID,
		NextRunAt:          job.NextRunAt,
		LastRunAt:          job.LastRunAt,
		LastRunStatus:      job.LastRunStatus,
		LastDeliveryStatus: job.LastDeliveryStatus,
		FailureStreak:      job.FailureStreak,
		LastError:          job.LastError,
		Runs:               []protocol.CronRun{},
	}
	if task.Running {
		addDailyReportTaskSignal(&task, "running")
		addDailyReportTaskSuggestedTool(&task, "recover_scheduled_task")
	}
	if stringPointerHasText(job.LastError) || job.FailureStreak > 0 || isFailedRunStatus(job.LastRunStatus) {
		addDailyReportTaskSignal(&task, "execution_attention")
		addDailyReportExecutionRepairSuggestedTools(&task)
	}
	for _, run := range runs {
		if !cronRunFallsInRange(run, startAt, endAt) {
			continue
		}
		run.DeliveryStatus = deriveCronRunDeliveryStatus(run)
		task.Runs = append(task.Runs, run)
		addDailyReportRun(&task.Totals, run)
		addDailyReportTaskRunSignals(&task, run)
	}
	return task, nil
}

func resolveDailyReportDate(raw string, loc *time.Location, now time.Time) (string, time.Time, time.Time, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" || normalized == "today" || normalized == "今天" {
		normalized = now.In(loc).Format("2006-01-02")
	}
	day, err := time.ParseInLocation("2006-01-02", normalized, loc)
	if err != nil {
		return "", time.Time{}, time.Time{}, errors.New("date must be YYYY-MM-DD, today, or 今天")
	}
	startAt := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
	return normalized, startAt, startAt.AddDate(0, 0, 1), nil
}

func cronRunFallsInRange(run protocol.CronRun, startAt time.Time, endAt time.Time) bool {
	when := cronRunReportTime(run)
	if when.IsZero() {
		return false
	}
	local := when.In(startAt.Location())
	return !local.Before(startAt) && local.Before(endAt)
}

func cronRunReportTime(run protocol.CronRun) time.Time {
	if run.ScheduledFor != nil && !run.ScheduledFor.IsZero() {
		return *run.ScheduledFor
	}
	if run.StartedAt != nil && !run.StartedAt.IsZero() {
		return *run.StartedAt
	}
	if run.FinishedAt != nil && !run.FinishedAt.IsZero() {
		return *run.FinishedAt
	}
	if !run.CreatedAt.IsZero() {
		return run.CreatedAt
	}
	return time.Time{}
}

func deriveCronRunDeliveryStatus(run protocol.CronRun) string {
	if strings.TrimSpace(run.DeliveryStatus) != "" {
		return strings.TrimSpace(run.DeliveryStatus)
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

func addDailyReportRun(totals *protocol.CronDailyReportTotals, run protocol.CronRun) {
	totals.RunCount++
	switch strings.TrimSpace(run.Status) {
	case protocol.RunStatusSucceeded, protocol.RunStatusQueuedToMain:
		totals.SucceededRunCount++
	case protocol.RunStatusFailed:
		totals.FailedRunCount++
	case protocol.RunStatusCancelled:
		totals.CancelledRunCount++
	case protocol.RunStatusSkipped:
		totals.SkippedRunCount++
	}
	switch strings.TrimSpace(run.DeliveryStatus) {
	case protocol.DeliveryStatusSucceeded:
		totals.DeliveredRunCount++
	case protocol.DeliveryStatusFailed:
		totals.DeliveryFailedRunCount++
		if run.DeliveryDeadLetterAt != nil {
			totals.DeliveryDeadLetterRunCount++
		}
	case protocol.DeliveryStatusPending:
		totals.DeliveryPendingRunCount++
	case protocol.DeliveryStatusSkipped:
		totals.DeliverySkippedRunCount++
	case protocol.DeliveryStatusNotRequired:
		totals.DeliveryNotNeededCount++
	case protocol.DeliveryStatusNotAttempted:
		totals.DeliveryNotAttemptedCount++
	}
}

func addDailyReportTotals(target *protocol.CronDailyReportTotals, source protocol.CronDailyReportTotals) {
	target.RunCount += source.RunCount
	target.SucceededRunCount += source.SucceededRunCount
	target.FailedRunCount += source.FailedRunCount
	target.CancelledRunCount += source.CancelledRunCount
	target.SkippedRunCount += source.SkippedRunCount
	target.DeliveredRunCount += source.DeliveredRunCount
	target.DeliveryFailedRunCount += source.DeliveryFailedRunCount
	target.DeliveryPendingRunCount += source.DeliveryPendingRunCount
	target.DeliverySkippedRunCount += source.DeliverySkippedRunCount
	target.DeliveryDeadLetterRunCount += source.DeliveryDeadLetterRunCount
	target.DeliveryNotNeededCount += source.DeliveryNotNeededCount
	target.DeliveryNotAttemptedCount += source.DeliveryNotAttemptedCount
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
	for _, item := range *items {
		if item == value {
			return
		}
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
	if value > maxValue {
		return maxValue
	}
	return value
}

func limitObservabilityRuns(runs []protocol.CronRun, limit int) []protocol.CronRun {
	if limit <= 0 || len(runs) <= limit {
		return runs
	}
	return runs[:limit]
}

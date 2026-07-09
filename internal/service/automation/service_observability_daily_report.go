package automation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	// 嵌入 IANA 时区数据库，避免轻量运行环境缺少 zoneinfo 时无法加载 Asia/Shanghai。
	_ "time/tzdata"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
)

// GetDailyReport 按日期聚合任务运行和投递状态。
func (s *Service) GetDailyReport(ctx context.Context, input automationdomain.CronDailyReportInput) (*automationdomain.CronDailyReport, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	timezone := firstNonEmpty(input.Timezone, s.config.DefaultTimezone, "Asia/Shanghai")
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
	var jobs []automationdomain.CronJob
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
			report := &automationdomain.CronDailyReport{
				Date:     date,
				Timezone: timezone,
				AgentID:  task.AgentID,
				JobID:    jobID,
				StartAt:  startAt,
				EndAt:    endAt,
				Tasks:    []automationdomain.CronDailyReportTask{task},
			}
			addDailyReportTotals(&report.Totals, task.Totals)
			report.Totals.TaskCount = 1
			return report, nil
		}
		jobs = []automationdomain.CronJob{*job}
		agentID = strings.TrimSpace(job.AgentID)
	} else {
		jobs, err = s.ListTasks(ctx, agentID)
		if err != nil {
			return nil, err
		}
	}

	report := &automationdomain.CronDailyReport{
		Date:     date,
		Timezone: timezone,
		AgentID:  agentID,
		JobID:    jobID,
		StartAt:  startAt,
		EndAt:    endAt,
		Tasks:    make([]automationdomain.CronDailyReportTask, 0, len(jobs)),
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
) (automationdomain.CronDailyReportTask, error) {
	ownerUserID, _ := scopedOwnerUserID(ctx)
	normalizedJobID := strings.TrimSpace(jobID)
	runs, err := s.repository.ListRunsByJob(ctx, ownerUserID, normalizedJobID)
	if err != nil {
		return automationdomain.CronDailyReportTask{}, err
	}
	events, err := s.repository.ListTaskEventsByJob(ctx, ownerUserID, normalizedJobID, 50)
	if err != nil {
		return automationdomain.CronDailyReportTask{}, err
	}
	if len(runs) == 0 && len(events) == 0 {
		return automationdomain.CronDailyReportTask{}, automationdomain.ErrJobNotFound
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

func deletedDailyReportTaskFromEvents(jobID string, events []automationdomain.CronTaskEvent) automationdomain.CronDailyReportTask {
	task := automationdomain.CronDailyReportTask{
		JobID:   jobID,
		Name:    jobID,
		Deleted: true,
		Enabled: false,
		Runs:    []automationdomain.CronRun{},
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

func (s *Service) buildDailyReportTask(
	ctx context.Context,
	job automationdomain.CronJob,
	startAt time.Time,
	endAt time.Time,
) (automationdomain.CronDailyReportTask, error) {
	runs, err := s.ListTaskRuns(ctx, job.JobID)
	if err != nil {
		return automationdomain.CronDailyReportTask{}, err
	}
	runningRunID := strings.TrimSpace(job.RunningRunID)
	task := automationdomain.CronDailyReportTask{
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
		Runs:               []automationdomain.CronRun{},
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

func cronRunFallsInRange(run automationdomain.CronRun, startAt time.Time, endAt time.Time) bool {
	when := cronRunReportTime(run)
	if when.IsZero() {
		return false
	}
	local := when.In(startAt.Location())
	return !local.Before(startAt) && local.Before(endAt)
}

func cronRunReportTime(run automationdomain.CronRun) time.Time {
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

func addDailyReportRun(totals *automationdomain.CronDailyReportTotals, run automationdomain.CronRun) {
	totals.RunCount++
	switch strings.TrimSpace(run.Status) {
	case automationdomain.RunStatusSucceeded, automationdomain.RunStatusQueuedToMain:
		totals.SucceededRunCount++
	case automationdomain.RunStatusFailed:
		totals.FailedRunCount++
	case automationdomain.RunStatusCancelled:
		totals.CancelledRunCount++
	case automationdomain.RunStatusSkipped:
		totals.SkippedRunCount++
	}
	switch strings.TrimSpace(run.DeliveryStatus) {
	case automationdomain.DeliveryStatusSucceeded:
		totals.DeliveredRunCount++
	case automationdomain.DeliveryStatusFailed:
		totals.DeliveryFailedRunCount++
		if run.DeliveryDeadLetterAt != nil {
			totals.DeliveryDeadLetterRunCount++
		}
	case automationdomain.DeliveryStatusPending:
		totals.DeliveryPendingRunCount++
	case automationdomain.DeliveryStatusSkipped:
		totals.DeliverySkippedRunCount++
	case automationdomain.DeliveryStatusNotRequired:
		totals.DeliveryNotNeededCount++
	case automationdomain.DeliveryStatusNotAttempted:
		totals.DeliveryNotAttemptedCount++
	}
}

func addDailyReportTotals(target *automationdomain.CronDailyReportTotals, source automationdomain.CronDailyReportTotals) {
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

package automation

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// scheduledTaskSummaryLimit 限制注入到 heartbeat prompt 的任务行数，
// 避免任务很多时 prompt 体积失控。
const scheduledTaskSummaryLimit = 10

// describeScheduledTasksSection 生成"Scheduled tasks"段落，
// 让主智能体在心跳轮次直接看到自己当前的定时任务清单与最近触发情况。
//
// 数据源优先使用进程内 jobStates（零成本、与 runLoop 一致），
// 仅在无 in-memory 状态且 repository 可用时才回落到数据库。
func (s *Service) describeScheduledTasksSection(ctx context.Context, agentID string) string {
	jobs := s.snapshotJobsForAgent(strings.TrimSpace(agentID))
	if len(jobs) == 0 {
		return ""
	}

	sort.SliceStable(jobs, func(i, j int) bool {
		return scheduledTaskSortKey(jobs[i]).Before(scheduledTaskSortKey(jobs[j]))
	})

	limit := scheduledTaskSummaryLimit
	if len(jobs) < limit {
		limit = len(jobs)
	}

	lines := make([]string, 0, limit+1)
	for _, job := range jobs[:limit] {
		lines = append(lines, formatScheduledTaskLine(job))
	}
	if len(jobs) > limit {
		lines = append(lines, fmt.Sprintf("(+%d more, use list_scheduled_tasks for full list)", len(jobs)-limit))
	}
	return "Scheduled tasks (you can manage these via the nexus_automation tools):\n- " + strings.Join(lines, "\n- ")
}

// snapshotJobsForAgent 从 in-memory jobStates 拷贝当前 agent 的任务视图。
// Start() 会在启动时把 DB 状态同步到 jobStates，因此这里读 in-memory 是与运行态一致的真相源。
func (s *Service) snapshotJobsForAgent(agentID string) []protocol.CronJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	jobs := make([]protocol.CronJob, 0, len(s.jobStates))
	for _, state := range s.jobStates {
		if state == nil {
			continue
		}
		if agentID != "" && state.Job.AgentID != agentID {
			continue
		}
		job := state.Job
		job.Running = state.Running
		job.NextRunAt = state.NextRunAt
		job.LastRunAt = state.LastRunAt
		jobs = append(jobs, job)
	}
	return jobs
}

// scheduledTaskSortKey 用最近一次触发时间作为排序键，没有则用未来很远的时间排到末尾。
func scheduledTaskSortKey(job protocol.CronJob) time.Time {
	if job.NextRunAt != nil {
		return *job.NextRunAt
	}
	return time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
}

// formatScheduledTaskLine 把单个任务摘要成一行简洁文本。
func formatScheduledTaskLine(job protocol.CronJob) string {
	state := "enabled"
	if !job.Enabled {
		state = "paused"
	}
	parts := []string{
		fmt.Sprintf("[%s]", state),
		job.Name,
		"id=" + job.JobID,
		"schedule=" + scheduleSummary(job.Schedule),
	}
	if job.NextRunAt != nil {
		parts = append(parts, "next="+job.NextRunAt.UTC().Format(time.RFC3339))
	}
	if job.LastRunAt != nil {
		parts = append(parts, "last="+job.LastRunAt.UTC().Format(time.RFC3339))
	}
	if job.Running {
		parts = append(parts, "running")
	}
	return strings.Join(parts, " | ")
}

// scheduleSummary 把 protocol.Schedule 压缩成一段紧凑文字，便于嵌入提示。
func scheduleSummary(schedule protocol.Schedule) string {
	switch schedule.Kind {
	case protocol.ScheduleKindEvery:
		if schedule.IntervalSeconds != nil {
			return fmt.Sprintf("every %ds", *schedule.IntervalSeconds)
		}
		return "every"
	case protocol.ScheduleKindCron:
		if schedule.CronExpression != nil {
			return "cron " + *schedule.CronExpression
		}
		return "cron"
	case protocol.ScheduleKindAt:
		if schedule.RunAt != nil {
			return "at " + *schedule.RunAt
		}
		return "at"
	}
	return schedule.Kind
}

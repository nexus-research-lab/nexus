package automation

import (
	"context"
	"fmt"
	"strings"
	"time"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

func (s *Service) recoverJobRuntimeAsCancelled(ctx context.Context, job automationdomain.ScheduledTask, message string) automationdomain.ScheduledTask {
	if strings.TrimSpace(job.RunningRunID) == "" {
		return job
	}
	finishedAt := s.nowFn()
	if _, err := s.repository.MarkRunFinishedIfActive(ctx, automationstore.RunFinishInput{
		RunID:        strings.TrimSpace(job.RunningRunID),
		Status:       automationdomain.RunStatusCancelled,
		FinishedAt:   finishedAt,
		ErrorMessage: &message,
	}); err != nil {
		s.loggerFor(ctx).Warn("恢复自动化任务运行态时标记未完成 run 失败",
			"job_id", job.JobID,
			"run_id", job.RunningRunID,
			"err", err,
		)
	}
	job.Running = false
	job.RunningRunID = ""
	job.RunningStartedAt = nil
	job.LastRunAt = cloneTimePointer(&finishedAt)
	job.LastRunStatus = automationdomain.RunStatusCancelled
	job.LastError = &message
	job.FailureStreak++
	nextRunAt := s.computeJobNext(job, finishedAt)
	job.NextRunAt = nextRunAt
	if backoff, ok := automationexec.RetryBackoffFor(job.FailureStreak); ok {
		retryAt := finishedAt.UTC().Add(backoff)
		if nextRunAt == nil || retryAt.Before(*nextRunAt) {
			retryCopy := retryAt
			job.NextRunAt = &retryCopy
		}
	}
	s.loggerFor(ctx).Warn("自动化任务运行态已从上次中断恢复",
		"job_id", job.JobID,
		"agent_id", job.AgentID,
		"next_run_at", job.NextRunAt,
	)
	return job
}

func (s *Service) recoverStaleRunningJobs(ctx context.Context, now time.Time) {
	timeout := s.automationRunTimeout()
	if timeout <= 0 {
		return
	}
	staleJobs := make([]automationdomain.ScheduledTask, 0)
	s.mu.Lock()
	for _, state := range s.jobStates {
		if state == nil || !state.Running || strings.TrimSpace(state.RunningRunID) == "" || state.RunningStartedAt == nil {
			continue
		}
		if now.UTC().Sub(state.RunningStartedAt.UTC()) < timeout {
			continue
		}
		staleJobs = append(staleJobs, scheduledTaskWithRuntime(state.Job, state))
	}
	s.mu.Unlock()
	for _, job := range staleJobs {
		s.recoverStaleRunningJob(ctx, job, timeout)
	}
}

func (s *Service) recoverStaleRunningJob(ctx context.Context, job automationdomain.ScheduledTask, timeout time.Duration) {
	runID := strings.TrimSpace(job.RunningRunID)
	if runID == "" {
		return
	}
	message := fmt.Sprintf("自动化任务运行超过 %s 未完成，调度器已自动释放运行占用", timeout)
	recovered := s.recoverJobRuntimeAsCancelled(ctx, job, message)
	state := s.replaceJobRuntimeState(recovered)
	result := scheduledTaskWithRuntime(recovered, state)
	s.recordTaskEvent(ctx, automationdomain.TaskEventActionRecover, result, runID, map[string]any{
		"recovered_run_id": runID,
		"reason":           "timeout",
		"timeout_seconds":  int(timeout.Seconds()),
	})
	s.loggerFor(ctx).Warn("自动化任务运行超时，已自动恢复",
		"job_id", job.JobID,
		"agent_id", job.AgentID,
		"run_id", runID,
		"timeout", timeout,
	)
}

func (s *Service) automationRunTimeout() time.Duration {
	if s.config.AutomationRunTimeoutSeconds <= 0 {
		return 0
	}
	return time.Duration(s.config.AutomationRunTimeoutSeconds) * time.Second
}

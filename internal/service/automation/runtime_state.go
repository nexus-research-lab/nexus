package automation

import (
	"context"
	"strings"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *Service) ensureJobState(job protocol.CronJob) *automationdomain.JobRuntimeState {
	s.mu.Lock()
	state := s.jobStates[job.JobID]
	if state == nil {
		state = &automationdomain.JobRuntimeState{}
		s.jobStates[job.JobID] = state
	}
	state.Job = job
	if state.NextRunAt == nil || !job.Enabled {
		state.NextRunAt = s.computeJobNext(job, s.nowFn())
	}
	if !job.Enabled {
		state.Running = false
	}
	// 启动期发现 at-kind 已过期且仍处于启用态时，主动落库为停用，避免反复检查空 NextRunAt 浪费循环。
	shouldDisable := job.Enabled &&
		strings.EqualFold(job.Schedule.Kind, protocol.ScheduleKindAt) &&
		state.NextRunAt == nil
	jobSnapshot := state.Job
	s.mu.Unlock()

	if shouldDisable {
		s.disableExpiredJobAsync(jobSnapshot)
	}
	return state
}

func (s *Service) computeJobNext(job protocol.CronJob, now time.Time) *time.Time {
	if !job.Enabled {
		return nil
	}
	next, err := automationdomain.ComputeNextRunAt(job.Schedule, now)
	if err != nil {
		return nil
	}
	return next
}

func (s *Service) finishJobRuntime(jobID string, finishedAt *time.Time, succeeded bool) {
	s.mu.Lock()
	state := s.jobStates[jobID]
	if state == nil {
		s.mu.Unlock()
		return
	}
	state.Running = false
	if finishedAt != nil {
		state.LastRunAt = cloneTimePointer(finishedAt)
	}

	now := s.nowFn()
	naturalNext := s.computeJobNext(state.Job, now)

	if succeeded {
		state.FailureStreak = 0
		state.NextRunAt = naturalNext
	} else {
		state.FailureStreak++
		state.NextRunAt = naturalNext
		if backoff, ok := automationdomain.RetryBackoffFor(state.FailureStreak); ok {
			retryAt := now.UTC().Add(backoff)
			if naturalNext == nil || retryAt.Before(*naturalNext) {
				retryCopy := retryAt
				state.NextRunAt = &retryCopy
			}
		}
	}

	// at-kind 是一次性任务：成功或重试耗尽后没有下一次自然触发，主动停用以避免数据库残留启用态。
	shouldDisable := state.Job.Enabled &&
		strings.EqualFold(state.Job.Schedule.Kind, protocol.ScheduleKindAt) &&
		state.NextRunAt == nil
	jobSnapshot := state.Job
	s.mu.Unlock()

	if shouldDisable {
		s.disableExpiredJobAsync(jobSnapshot)
	}
}

func (s *Service) disableExpiredJobAsync(job protocol.CronJob) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		updated := job
		updated.Enabled = false
		if _, err := s.repository.UpsertCronJob(context.Background(), updated); err != nil {
			s.loggerFor(context.Background()).Warn("at 任务到期自动停用失败",
				"job_id", job.JobID,
				"agent_id", job.AgentID,
				"err", err,
			)
			return
		}
		s.mu.Lock()
		if state := s.jobStates[job.JobID]; state != nil {
			state.Job.Enabled = false
		}
		s.mu.Unlock()
		s.loggerFor(context.Background()).Info("at 任务到期已自动停用",
			"job_id", job.JobID,
			"agent_id", job.AgentID,
		)
	}()
}

package automation

import (
	"context"
	"strings"
	"time"
)

type jobRuntimeState struct {
	Job           CronJob
	Running       bool
	NextRunAt     *time.Time
	LastRunAt     *time.Time
	FailureStreak int
}

// 失败重试策略：连续失败不超过 maxFailureRetries 次时，按 retryBackoffs 顺序退避重排。
// 超过阈值后回退到 Schedule 的正常节奏，由下一个自然触发继续尝试。
var retryBackoffs = []time.Duration{
	30 * time.Second,
	2 * time.Minute,
	10 * time.Minute,
}

func retryBackoffFor(streak int) (time.Duration, bool) {
	if streak <= 0 || streak > len(retryBackoffs) {
		return 0, false
	}
	return retryBackoffs[streak-1], true
}

type heartbeatRuntimeState struct {
	Config          HeartbeatConfig
	Running         bool
	PendingWake     bool
	NextRunAt       *time.Time
	LastHeartbeatAt *time.Time
	LastAckAt       *time.Time
	DeliveryError   *string
}

type heartbeatWakeRequest struct {
	AgentID    string
	SessionKey string
	WakeMode   string
	Text       string
}

func (s *Service) ensureJobState(job CronJob) *jobRuntimeState {
	s.mu.Lock()
	state := s.jobStates[job.JobID]
	if state == nil {
		state = &jobRuntimeState{}
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
		strings.EqualFold(job.Schedule.Kind, ScheduleKindAt) &&
		state.NextRunAt == nil
	jobSnapshot := state.Job
	s.mu.Unlock()

	if shouldDisable {
		s.disableExpiredJobAsync(jobSnapshot)
	}
	return state
}

func (s *Service) computeJobNext(job CronJob, now time.Time) *time.Time {
	if !job.Enabled {
		return nil
	}
	next, err := ComputeNextRunAt(job.Schedule, now)
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
		if backoff, ok := retryBackoffFor(state.FailureStreak); ok {
			retryAt := now.UTC().Add(backoff)
			if naturalNext == nil || retryAt.Before(*naturalNext) {
				retryCopy := retryAt
				state.NextRunAt = &retryCopy
			}
		}
	}

	// at-kind 是一次性任务：成功或重试耗尽后没有下一次自然触发，主动停用以避免数据库残留启用态。
	shouldDisable := state.Job.Enabled &&
		strings.EqualFold(state.Job.Schedule.Kind, ScheduleKindAt) &&
		state.NextRunAt == nil
	jobSnapshot := state.Job
	s.mu.Unlock()

	if shouldDisable {
		s.disableExpiredJobAsync(jobSnapshot)
	}
}

func (s *Service) disableExpiredJobAsync(job CronJob) {
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

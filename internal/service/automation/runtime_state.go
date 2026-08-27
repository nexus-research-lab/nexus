// INPUT: 持久化任务定义、调度执行结果与并发运行态读写。
// OUTPUT: 锁内维护的任务运行态、无持久化副作用的任务投影与显式运行态快照。
// POS: scheduled task 配置与易变运行态之间的并发安全投影层。
package automation

import (
	"context"
	"strings"
	"time"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

func (s *Service) ensureJobState(job automationdomain.ScheduledTask) *automationexec.JobRuntimeState {
	s.mu.Lock()
	state := s.jobStates[job.JobID]
	if state != nil && job.ConfigurationVersion > 0 &&
		state.Job.ConfigurationVersion > job.ConfigurationVersion {
		s.mu.Unlock()
		return state
	}
	created := state == nil
	if state == nil {
		state = &automationexec.JobRuntimeState{}
		s.jobStates[job.JobID] = state
	}
	definitionChanged := !created &&
		(state.Job.Enabled != job.Enabled || !sameSchedule(state.Job.Schedule, job.Schedule))
	state.Job = job
	if created {
		state.NextRunAt = cloneTimePointer(job.NextRunAt)
		state.RunningRunID = strings.TrimSpace(job.RunningRunID)
		state.RunningStartedAt = cloneTimePointer(job.RunningStartedAt)
		state.Running = state.RunningRunID != ""
		if state.Running {
			state.RunningCount = 1
		}
		state.LastRunAt = cloneTimePointer(job.LastRunAt)
		state.LastRunStatus = strings.TrimSpace(job.LastRunStatus)
		state.FailureStreak = job.FailureStreak
		state.LastError = cloneStringPointer(job.LastError)
		state.LastDeliveryStatus = strings.TrimSpace(job.LastDeliveryStatus)
	}
	if definitionChanged {
		state.NextRunAt = nil
	}
	if state.NextRunAt == nil || !job.Enabled {
		state.NextRunAt = s.computeJobNext(job, s.nowFn())
	}
	s.mu.Unlock()

	// 这里只更新内存投影；持久写与 scheduler 唤醒均由显式 mutation、
	// runtime 持久化或 scheduler 入口负责，List/Get 不产生控制面副作用。
	return state
}

func (s *Service) computeJobNext(job automationdomain.ScheduledTask, now time.Time) *time.Time {
	if !job.Enabled {
		return nil
	}
	maxJitter := time.Duration(s.config.AutomationRecurringJitterSeconds) * time.Second
	next, err := automationexec.ComputeJitteredNextRunAt(job.Schedule, now, job.JobID, maxJitter)
	if err != nil {
		return nil
	}
	return next
}

func (s *Service) finishJobRuntime(jobID string, finishedAt *time.Time, status string, errorMessage *string, deliveryStatuses ...string) {
	s.mu.Lock()
	state := s.jobStates[jobID]
	if state == nil {
		s.mu.Unlock()
		return
	}
	status = normalizeFinishedRunState(state, finishedAt, status, errorMessage, deliveryStatuses)
	now := s.nowFn()
	naturalNext := s.naturalNextRunAt(state, now)
	applyFinishedRunOutcome(state, status, now, naturalNext)

	runtimeSnapshot := jobRuntimeUpdateFromState(jobID, state)
	s.mu.Unlock()

	s.persistJobRuntime(context.Background(), runtimeSnapshot)
	if len(deliveryStatuses) > 0 &&
		strings.TrimSpace(deliveryStatuses[0]) == automationdomain.DeliveryStatusFailed {
		s.invalidateDeliveryRetryDeadline()
	} else {
		s.wakeScheduler()
	}
}

func normalizeFinishedRunState(
	state *automationexec.JobRuntimeState,
	finishedAt *time.Time,
	status string,
	errorMessage *string,
	deliveryStatuses []string,
) string {
	if state.RunningCount > 0 {
		state.RunningCount--
	}
	state.Running = state.RunningCount > 0
	if !state.Running {
		state.RunningRunID = ""
		state.RunningStartedAt = nil
	}
	if finishedAt != nil {
		state.LastRunAt = cloneTimePointer(finishedAt)
	}
	if strings.TrimSpace(status) == "" {
		status = automationdomain.RunStatusFailed
	}
	state.LastRunStatus = strings.TrimSpace(status)
	state.LastError = cloneStringPointer(errorMessage)
	if len(deliveryStatuses) > 0 {
		state.LastDeliveryStatus = strings.TrimSpace(deliveryStatuses[0])
	} else if !isSuccessfulRuntimeStatus(status) {
		state.LastDeliveryStatus = automationdomain.DeliveryStatusNotAttempted
	}
	return status
}

func (s *Service) naturalNextRunAt(state *automationexec.JobRuntimeState, now time.Time) *time.Time {
	naturalNext := cloneTimePointer(state.NextRunAt)
	if naturalNext == nil || !naturalNext.After(now) {
		naturalNext = s.computeJobNext(state.Job, now)
	}
	return naturalNext
}

func applyFinishedRunOutcome(
	state *automationexec.JobRuntimeState,
	status string,
	now time.Time,
	naturalNext *time.Time,
) {
	state.NextRunAt = naturalNext
	if isSuccessfulRuntimeStatus(status) {
		state.FailureStreak = 0
		state.LastError = nil
		return
	}
	state.FailureStreak++
	backoff, ok := automationexec.RetryBackoffFor(state.FailureStreak)
	if !ok {
		return
	}
	retryAt := now.UTC().Add(backoff)
	if naturalNext == nil || retryAt.Before(*naturalNext) {
		state.NextRunAt = cloneTimePointer(&retryAt)
	}
}

func (s *Service) advanceJobRuntimeAfterTrigger(jobID string, scheduledFor time.Time) {
	s.advanceJobRuntimeAfterTriggerWithPersistence(jobID, scheduledFor, true)
}

func (s *Service) advanceJobRuntimeAfterExternalClaim(jobID string, scheduledFor time.Time) {
	s.advanceJobRuntimeAfterTriggerWithPersistence(jobID, scheduledFor, false)
}

func (s *Service) advanceJobRuntimeAfterTriggerWithPersistence(jobID string, scheduledFor time.Time, persist bool) {
	s.mu.Lock()
	state := s.jobStates[jobID]
	if state == nil {
		s.mu.Unlock()
		return
	}
	state.LastRunAt = cloneTimePointer(&scheduledFor)
	state.LastRunStatus = automationdomain.RunStatusSkipped
	// 避免允许并发或跳过重叠时，同一个 due tick 被下一秒反复触发。
	state.NextRunAt = s.computeJobNext(state.Job, scheduledFor.UTC().Add(time.Second))
	runtimeSnapshot := jobRuntimeUpdateFromState(jobID, state)
	s.mu.Unlock()

	if persist {
		s.persistJobRuntime(context.Background(), runtimeSnapshot)
	}
	s.wakeScheduler()
}

func (s *Service) replaceJobRuntimeState(job automationdomain.ScheduledTask) *automationexec.JobRuntimeState {
	return s.replaceJobRuntimeStateWithPersistence(job, true)
}

// replacePersistedJobRuntimeState 只更新内存镜像；调用方已经在同一领域事务中
// 提交了 runtime，不能再追加一次会吞错的重复写入。
func (s *Service) replacePersistedJobRuntimeState(job automationdomain.ScheduledTask) *automationexec.JobRuntimeState {
	return s.replaceJobRuntimeStateWithPersistence(job, false)
}

func (s *Service) replaceJobRuntimeStateWithPersistence(
	job automationdomain.ScheduledTask,
	persist bool,
) *automationexec.JobRuntimeState {
	s.mu.Lock()
	state := s.jobStates[job.JobID]
	if state == nil {
		state = &automationexec.JobRuntimeState{}
		s.jobStates[job.JobID] = state
	}
	state.Job = job
	state.NextRunAt = cloneTimePointer(job.NextRunAt)
	state.RunningRunID = strings.TrimSpace(job.RunningRunID)
	state.RunningStartedAt = cloneTimePointer(job.RunningStartedAt)
	state.Running = state.RunningRunID != ""
	if state.Running {
		if state.RunningCount == 0 {
			state.RunningCount = 1
		}
	} else {
		state.RunningCount = 0
	}
	state.LastRunAt = cloneTimePointer(job.LastRunAt)
	state.LastRunStatus = strings.TrimSpace(job.LastRunStatus)
	state.FailureStreak = job.FailureStreak
	state.LastError = cloneStringPointer(job.LastError)
	state.LastDeliveryStatus = strings.TrimSpace(job.LastDeliveryStatus)
	runtimeSnapshot := jobRuntimeUpdateFromState(job.JobID, state)
	s.mu.Unlock()

	if persist {
		s.persistJobRuntime(context.Background(), runtimeSnapshot)
	}
	s.wakeScheduler()
	return state
}

func (s *Service) persistJobRuntime(ctx context.Context, input automationstore.JobRuntimeUpdateInput) {
	if strings.TrimSpace(input.JobID) == "" {
		return
	}
	if err := s.repository.UpdateScheduledTaskRuntime(ctx, input); err != nil {
		s.loggerFor(ctx).Warn("持久化自动化任务运行态失败",
			"job_id", input.JobID,
			"err", err,
		)
	}
	// Notify 是有界合并信号；把它放在真实 runtime mutation 后，查询投影便
	// 不需要借唤醒 scheduler 间接触发配置写入。
	s.wakeScheduler()
}

func (s *Service) setJobPermissionState(jobID string, permissionState string, requestID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.jobStates[strings.TrimSpace(jobID)]
	if state == nil {
		return
	}
	state.Job.PermissionState = strings.TrimSpace(permissionState)
	state.Job.PendingPermissionRequestID = strings.TrimSpace(requestID)
}

func (s *Service) pauseJobRuntimeForPermission(
	job automationdomain.ScheduledTask,
	runID string,
	permissionState string,
	reason *string,
) {
	s.ensureJobState(job)
	s.mu.Lock()
	state := s.jobStates[strings.TrimSpace(job.JobID)]
	if state == nil {
		s.mu.Unlock()
		return
	}
	currentRunID := strings.TrimSpace(state.RunningRunID)
	expectedRunID := strings.TrimSpace(runID)
	if currentRunID != expectedRunID {
		s.mu.Unlock()
		return
	}
	desiredRunningCount := state.RunningCount
	if desiredRunningCount > 0 {
		desiredRunningCount--
	}
	desiredRunning := desiredRunningCount > 0
	desiredRunningRunID := currentRunID
	desiredRunningStartedAt := cloneTimePointer(state.RunningStartedAt)
	if !desiredRunning {
		desiredRunningRunID = ""
		desiredRunningStartedAt = nil
	}
	desiredLastRunStatus := strings.TrimSpace(permissionState)
	desiredLastError := cloneStringPointer(reason)
	s.mu.Unlock()

	updated, err := s.repository.PauseScheduledTaskRuntimeForPermission(
		backgroundContextForJobOwner(job),
		automationstore.JobRuntimePermissionPauseInput{
			OwnerUserID:          job.OwnerUserID,
			JobID:                job.JobID,
			ExpectedRunID:        expectedRunID,
			NextRunningRunID:     desiredRunningRunID,
			NextRunningStartedAt: desiredRunningStartedAt,
			LastRunStatus:        desiredLastRunStatus,
			LastError:            desiredLastError,
		},
	)
	if err == nil && updated {
		s.mu.Lock()
		state = s.jobStates[strings.TrimSpace(job.JobID)]
		if state != nil && strings.TrimSpace(state.RunningRunID) == expectedRunID {
			state.RunningCount = desiredRunningCount
			state.Running = desiredRunning
			state.RunningRunID = desiredRunningRunID
			state.RunningStartedAt = cloneTimePointer(desiredRunningStartedAt)
			state.LastRunStatus = desiredLastRunStatus
			state.LastError = cloneStringPointer(desiredLastError)
			state.Job.PermissionState = desiredLastRunStatus
		}
		s.mu.Unlock()
		s.wakeScheduler()
		return
	}
	if err != nil {
		s.loggerFor(context.Background()).Warn(
			"持久化权限暂停运行态失败",
			"job_id", job.JobID,
			"run_id", expectedRunID,
			"err", err,
		)
	}
	current, loadErr := s.repository.GetScheduledTask(
		backgroundContextForJobOwner(job),
		job.OwnerUserID,
		job.JobID,
	)
	if loadErr != nil {
		s.loggerFor(context.Background()).Warn(
			"刷新权限暂停冲突后的任务运行态失败",
			"job_id", job.JobID,
			"run_id", expectedRunID,
			"err", loadErr,
		)
		return
	}
	if current == nil {
		s.mu.Lock()
		delete(s.jobStates, job.JobID)
		s.mu.Unlock()
		return
	}
	s.replacePersistedJobRuntimeState(*current)
}

func jobRuntimeUpdateFromState(jobID string, state *automationexec.JobRuntimeState) automationstore.JobRuntimeUpdateInput {
	return automationstore.JobRuntimeUpdateInput{
		JobID:              jobID,
		NextRunAt:          cloneTimePointer(state.NextRunAt),
		RunningRunID:       strings.TrimSpace(state.RunningRunID),
		RunningStartedAt:   cloneTimePointer(state.RunningStartedAt),
		LastRunAt:          cloneTimePointer(state.LastRunAt),
		LastRunStatus:      strings.TrimSpace(state.LastRunStatus),
		FailureStreak:      state.FailureStreak,
		LastError:          cloneStringPointer(state.LastError),
		LastDeliveryStatus: strings.TrimSpace(state.LastDeliveryStatus),
	}
}

func jobRuntimeUpdateFromTask(job automationdomain.ScheduledTask) automationstore.JobRuntimeUpdateInput {
	return automationstore.JobRuntimeUpdateInput{
		JobID:              strings.TrimSpace(job.JobID),
		NextRunAt:          cloneTimePointer(job.NextRunAt),
		RunningRunID:       strings.TrimSpace(job.RunningRunID),
		RunningStartedAt:   cloneTimePointer(job.RunningStartedAt),
		LastRunAt:          cloneTimePointer(job.LastRunAt),
		LastRunStatus:      strings.TrimSpace(job.LastRunStatus),
		FailureStreak:      job.FailureStreak,
		LastError:          cloneStringPointer(job.LastError),
		LastDeliveryStatus: strings.TrimSpace(job.LastDeliveryStatus),
	}
}

// jobRuntimeUpdateSnapshot 在状态锁内生成持久化副本，禁止把 jobStates
// 中的可变指针交给锁外读取。
func (s *Service) jobRuntimeUpdateSnapshot(
	jobID string,
	state *automationexec.JobRuntimeState,
) automationstore.JobRuntimeUpdateInput {
	s.mu.Lock()
	defer s.mu.Unlock()
	return jobRuntimeUpdateFromState(jobID, state)
}

// scheduledTaskWithRuntime 将持久化定义与进程运行态合成唯一对外视图，避免各入口各自维护字段集合。
func scheduledTaskWithRuntime(
	job automationdomain.ScheduledTask,
	state *automationexec.JobRuntimeState,
) automationdomain.ScheduledTask {
	if state == nil {
		return job
	}
	job.NextRunAt = cloneTimePointer(state.NextRunAt)
	job.Running = state.Running
	job.RunningRunID = strings.TrimSpace(state.RunningRunID)
	job.RunningStartedAt = cloneTimePointer(state.RunningStartedAt)
	job.LastRunAt = cloneTimePointer(state.LastRunAt)
	job.LastRunStatus = strings.TrimSpace(state.LastRunStatus)
	job.FailureStreak = state.FailureStreak
	job.LastError = cloneStringPointer(state.LastError)
	job.LastDeliveryStatus = strings.TrimSpace(state.LastDeliveryStatus)
	return job
}

// scheduledTaskRuntimeSnapshot 在状态锁内生成对外副本，避免任务完成写回与
// 查询、CRUD 回包同时访问同一个 JobRuntimeState。
func (s *Service) scheduledTaskRuntimeSnapshot(
	job automationdomain.ScheduledTask,
	state *automationexec.JobRuntimeState,
) automationdomain.ScheduledTask {
	s.mu.Lock()
	defer s.mu.Unlock()
	return scheduledTaskWithRuntime(job, state)
}

func isSuccessfulRuntimeStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case automationdomain.RunStatusSucceeded:
		return true
	default:
		return false
	}
}

func sameSchedule(left automationdomain.Schedule, right automationdomain.Schedule) bool {
	left = left.Normalized()
	right = right.Normalized()
	return strings.TrimSpace(left.Kind) == strings.TrimSpace(right.Kind) &&
		anyStringPointer(left.RunAt) == anyStringPointer(right.RunAt) &&
		anyIntPointer(left.IntervalSeconds) == anyIntPointer(right.IntervalSeconds) &&
		anyStringPointer(left.CronExpression) == anyStringPointer(right.CronExpression) &&
		strings.TrimSpace(left.Timezone) == strings.TrimSpace(right.Timezone)
}

func anyIntPointer(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

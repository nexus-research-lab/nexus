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

const automationSchedulerLeaseName = "scheduled-tasks"
const defaultAutomationSchedulerLeaseTTL = 30 * time.Second

type dueScheduledTask struct {
	job          automationdomain.ScheduledTask
	scheduledFor time.Time
	triggerKind  string
}

type dueAutomationWork struct {
	dueTasks          []dueScheduledTask
	expiredTasks      []automationdomain.ScheduledTask
	heartbeatAgentIDs []string
}

func (s *Service) schedulerLeaseTTL() time.Duration {
	seconds := s.config.AutomationSchedulerLeaseSeconds
	if seconds <= 0 {
		return defaultAutomationSchedulerLeaseTTL
	}
	return time.Duration(seconds) * time.Second
}

// refreshSchedulerLease 返回当前实例是否持有租约，以及本次是否刚成为 leader。
func (s *Service) refreshSchedulerLease(ctx context.Context, now time.Time) (bool, bool, error) {
	s.mu.Lock()
	held := s.schedulerLeaseHeld
	renewAt := s.schedulerLeaseRenewAt
	ownerID := strings.TrimSpace(s.schedulerOwnerID)
	s.mu.Unlock()

	if held && now.Before(renewAt) {
		return true, false, nil
	}
	if ownerID == "" {
		ownerID = s.idFactory("scheduler")
		s.mu.Lock()
		s.schedulerOwnerID = ownerID
		s.mu.Unlock()
	}

	ttl := s.schedulerLeaseTTL()
	acquired, err := s.repository.TryAcquireSchedulerLease(
		ctx,
		automationSchedulerLeaseName,
		ownerID,
		now,
		now.Add(ttl),
	)
	if err != nil {
		s.mu.Lock()
		s.schedulerLeaseHeld = false
		s.schedulerLeaseRenewAt = time.Time{}
		s.mu.Unlock()
		return false, false, err
	}

	s.mu.Lock()
	becameLeader := acquired && !s.schedulerLeaseHeld
	s.schedulerLeaseHeld = acquired
	if acquired {
		s.schedulerLeaseRenewAt = now.Add(ttl / 3)
	} else {
		s.schedulerLeaseRenewAt = time.Time{}
	}
	s.mu.Unlock()
	return acquired, becameLeader, nil
}

// releaseSchedulerLease 释放当前实例持有的租约。
func (s *Service) releaseSchedulerLease(ctx context.Context) error {
	s.mu.Lock()
	held := s.schedulerLeaseHeld
	ownerID := s.schedulerOwnerID
	s.schedulerLeaseHeld = false
	s.schedulerLeaseRenewAt = time.Time{}
	s.mu.Unlock()
	if !held || strings.TrimSpace(ownerID) == "" {
		return nil
	}
	return s.repository.ReleaseSchedulerLease(ctx, automationSchedulerLeaseName, ownerID)
}

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

func (s *Service) bootstrapRuntime(ctx context.Context) error {
	jobs, err := s.repository.ListScheduledTasks(ctx, "", "")
	if err != nil {
		return err
	}
	for _, item := range jobs {
		s.ensureJobState(item)
	}

	configs, err := s.repository.ListEnabledHeartbeatStates(ctx)
	if err != nil {
		return err
	}
	for _, item := range configs {
		if _, stateErr := s.ensureHeartbeatState(ctx, item.AgentID); stateErr != nil {
			return stateErr
		}
	}
	return nil
}

func (s *Service) runLoop(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			held, becameLeader, err := s.refreshSchedulerLease(ctx, now.UTC())
			if err != nil {
				s.loggerFor(ctx).Warn("刷新自动化调度器租约失败", "err", err)
				continue
			}
			if !held {
				continue
			}
			if becameLeader {
				if err := s.bootstrapRuntime(ctx); err != nil {
					s.loggerFor(ctx).Error("接管自动化调度器后刷新运行态失败", "err", err)
					continue
				}
				s.loggerFor(ctx).Info("当前实例已接管自动化调度器")
			}
			s.runDueOnce()
		}
	}
}

func (s *Service) runDueOnce() {
	now := s.nowFn()
	s.recoverStaleRunningJobs(context.Background(), now)
	work := s.collectDueAutomationWork(now)
	s.expireScheduledTasks(work.expiredTasks, now)
	s.dispatchScheduledTasks(work.dueTasks, now)
	s.dispatchDueHeartbeats(work.heartbeatAgentIDs)
	s.startDeliveryRetryBatch(now)
}

func (s *Service) collectDueAutomationWork(now time.Time) dueAutomationWork {
	s.mu.Lock()
	defer s.mu.Unlock()
	dueTasks, expiredTasks := s.collectScheduledTaskWorkLocked(now)
	return dueAutomationWork{
		dueTasks:          dueTasks,
		expiredTasks:      expiredTasks,
		heartbeatAgentIDs: s.collectDueHeartbeatAgentIDsLocked(now),
	}
}

func (s *Service) collectScheduledTaskWorkLocked(
	now time.Time,
) ([]dueScheduledTask, []automationdomain.ScheduledTask) {
	dueTasks := make([]dueScheduledTask, 0)
	expiredTasks := make([]automationdomain.ScheduledTask, 0)
	for _, state := range s.jobStates {
		if state == nil || !state.Job.Enabled {
			continue
		}
		if state.Job.ExpiresAt != nil && !state.Job.ExpiresAt.UTC().After(now.UTC()) {
			expiredTasks = append(expiredTasks, state.Job)
			continue
		}
		if !isRunnableScheduledTaskState(state, now) {
			continue
		}
		dueTasks = append(dueTasks, dueScheduledTask{
			job:          state.Job,
			scheduledFor: state.NextRunAt.UTC(),
			triggerKind:  s.scheduledTriggerKind(state.NextRunAt.UTC(), now),
		})
	}
	return dueTasks, expiredTasks
}

func isRunnableScheduledTaskState(state *automationexec.JobRuntimeState, now time.Time) bool {
	if state.NextRunAt == nil {
		return false
	}
	if state.Running && automationdomain.NormalizeOverlapPolicy(state.Job.OverlapPolicy) == automationdomain.OverlapPolicySkip {
		return false
	}
	return !state.NextRunAt.After(now)
}

func (s *Service) collectDueHeartbeatAgentIDsLocked(now time.Time) []string {
	agentIDs := make([]string, 0)
	for agentID, state := range s.heartbeatState {
		if state == nil || state.Running {
			continue
		}
		if !state.Config.Enabled {
			continue
		}
		if s.hasImmediateWakeRequestLocked(agentID) {
			agentIDs = append(agentIDs, agentID)
			continue
		}
		if state.NextRunAt == nil || state.NextRunAt.After(now) {
			continue
		}
		agentIDs = append(agentIDs, agentID)
	}
	return agentIDs
}

func (s *Service) expireScheduledTasks(tasks []automationdomain.ScheduledTask, now time.Time) {
	for _, task := range tasks {
		if err := s.expireTask(context.Background(), task, now); err != nil {
			s.loggerFor(context.Background()).Error("停用已过期定时任务失败",
				"job_id", task.JobID,
				"agent_id", task.AgentID,
				"err", err,
			)
		}
	}
}

func (s *Service) dispatchScheduledTasks(tasks []dueScheduledTask, now time.Time) {
	for _, task := range tasks {
		if s.skipScheduledMisfire(task, now) {
			continue
		}
		go func(task dueScheduledTask) {
			if _, err := s.startJobExecution(context.Background(), task.job, task.triggerKind, task.scheduledFor); err != nil {
				s.loggerFor(context.Background()).Error("定时任务触发失败",
					"job_id", task.job.JobID,
					"agent_id", task.job.AgentID,
					"trigger_kind", task.triggerKind,
					"err", err,
				)
			}
		}(task)
	}
}

func (s *Service) skipScheduledMisfire(task dueScheduledTask, now time.Time) bool {
	if task.triggerKind != automationdomain.TriggerKindMisfire || !s.shouldSkipMisfire() {
		return false
	}
	if _, err := s.recordSkippedMisfire(context.Background(), task.job, task.scheduledFor, now); err != nil {
		s.loggerFor(context.Background()).Error("记录错过的定时任务失败",
			"job_id", task.job.JobID,
			"scheduled_for", task.scheduledFor,
			"err", err,
		)
	}
	return true
}

func (s *Service) dispatchDueHeartbeats(agentIDs []string) {
	for _, agentID := range agentIDs {
		go s.dispatchHeartbeat(agentID, "heartbeat")
	}
}

func (s *Service) startDeliveryRetryBatch(now time.Time) {
	if s.beginDeliveryRetryBatch() {
		go s.retryDueDeliveries(context.Background(), now)
	}
}

func (s *Service) scheduledTriggerKind(scheduledFor time.Time, now time.Time) string {
	if s.isMisfire(scheduledFor, now) {
		return automationdomain.TriggerKindMisfire
	}
	return automationdomain.TriggerKindScheduled
}

func (s *Service) shouldSkipMisfire() bool {
	return strings.EqualFold(strings.TrimSpace(s.config.AutomationMisfirePolicy), "skip")
}

func (s *Service) isMisfire(scheduledFor time.Time, now time.Time) bool {
	grace := time.Duration(s.config.AutomationMisfireGraceSeconds) * time.Second
	if grace < 0 {
		grace = 0
	}
	return now.UTC().Sub(scheduledFor.UTC()) > grace
}

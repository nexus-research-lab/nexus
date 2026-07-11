package automation

import (
	"context"
	"strings"
	"time"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
)

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

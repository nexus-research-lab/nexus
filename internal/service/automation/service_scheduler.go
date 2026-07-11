package automation

import (
	"context"
	"strings"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
)

type dueScheduledTask struct {
	job          automationdomain.CronJob
	scheduledFor time.Time
	triggerKind  string
}

func (s *Service) bootstrapRuntime(ctx context.Context) error {
	jobs, err := s.repository.ListCronJobs(ctx, "", "")
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

	dueJobs := make([]dueScheduledTask, 0)
	expiredJobs := make([]automationdomain.CronJob, 0)
	dueHeartbeats := make([]string, 0)

	s.mu.Lock()
	for _, state := range s.jobStates {
		if state == nil || !state.Job.Enabled {
			continue
		}
		if state.Job.ExpiresAt != nil && !state.Job.ExpiresAt.UTC().After(now.UTC()) {
			expiredJobs = append(expiredJobs, state.Job)
			continue
		}
		if state.NextRunAt == nil {
			continue
		}
		if state.Running && automationdomain.NormalizeOverlapPolicy(state.Job.OverlapPolicy) == automationdomain.OverlapPolicySkip {
			continue
		}
		if !state.NextRunAt.After(now) {
			triggerKind := "cron"
			if s.isMisfire(state.NextRunAt.UTC(), now) {
				triggerKind = "misfire"
			}
			dueJobs = append(dueJobs, dueScheduledTask{
				job:          state.Job,
				scheduledFor: state.NextRunAt.UTC(),
				triggerKind:  triggerKind,
			})
		}
	}
	for agentID, state := range s.heartbeatState {
		if state == nil || state.Running {
			continue
		}
		if !state.Config.Enabled {
			continue
		}
		if s.hasImmediateWakeRequestLocked(agentID) {
			dueHeartbeats = append(dueHeartbeats, agentID)
			continue
		}
		if state.NextRunAt == nil || state.NextRunAt.After(now) {
			continue
		}
		dueHeartbeats = append(dueHeartbeats, agentID)
	}
	s.mu.Unlock()

	for _, job := range expiredJobs {
		if err := s.expireTask(context.Background(), job, now); err != nil {
			s.loggerFor(context.Background()).Error("停用已过期定时任务失败",
				"job_id", job.JobID,
				"agent_id", job.AgentID,
				"err", err,
			)
		}
	}

	for _, item := range dueJobs {
		if item.triggerKind == "misfire" && s.shouldSkipMisfire() {
			if _, err := s.recordSkippedMisfire(context.Background(), item.job, item.scheduledFor, now); err != nil {
				s.loggerFor(context.Background()).Error("记录错过的定时任务失败",
					"job_id", item.job.JobID,
					"scheduled_for", item.scheduledFor,
					"err", err,
				)
			}
			continue
		}
		jobValue := item.job
		scheduledFor := item.scheduledFor
		triggerKind := item.triggerKind
		go func() {
			if _, err := s.startJobExecution(context.Background(), jobValue, triggerKind, scheduledFor); err != nil {
				s.loggerFor(context.Background()).Error("定时任务触发失败",
					"job_id", jobValue.JobID,
					"agent_id", jobValue.AgentID,
					"trigger_kind", triggerKind,
					"err", err,
				)
			}
		}()
	}
	for _, agentID := range dueHeartbeats {
		go s.dispatchHeartbeat(agentID, "heartbeat")
	}
	if s.beginDeliveryRetryBatch() {
		go s.retryDueDeliveries(context.Background(), now)
	}
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

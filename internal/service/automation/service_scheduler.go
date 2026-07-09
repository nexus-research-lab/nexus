package automation

import (
	"context"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/protocol"
)

func (s *Service) bootstrapRuntime(ctx context.Context) error {
	jobs, err := s.repository.ListCronJobs(ctx, "", "")
	if err != nil {
		return err
	}
	for _, item := range jobs {
		item = s.recoverJobRuntimeAsCancelled(ctx, item, "scheduler restarted before run completed")
		state := s.ensureJobState(item)
		s.persistJobRuntime(ctx, jobRuntimeUpdateFromState(item.JobID, state))
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
		case <-ticker.C:
			s.runDueOnce()
		}
	}
}

func (s *Service) runDueOnce() {
	now := s.nowFn()
	s.recoverStaleRunningJobs(context.Background(), now)

	dueJobs := make([]automationdomain.CronJob, 0)
	dueHeartbeats := make([]string, 0)

	s.mu.Lock()
	for _, state := range s.jobStates {
		if state == nil || !state.Job.Enabled || state.NextRunAt == nil {
			continue
		}
		if state.Running && automationdomain.NormalizeOverlapPolicy(state.Job.OverlapPolicy) == automationdomain.OverlapPolicySkip {
			continue
		}
		if !state.NextRunAt.After(now) {
			dueJobs = append(dueJobs, state.Job)
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

	for _, item := range dueJobs {
		jobValue := item
		go func() {
			if _, err := s.startJobExecution(context.Background(), jobValue, "cron", now); err != nil {
				s.loggerFor(context.Background()).Error("定时任务触发失败",
					"job_id", jobValue.JobID,
					"agent_id", jobValue.AgentID,
					"trigger_kind", "cron",
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

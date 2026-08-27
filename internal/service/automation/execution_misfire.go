// INPUT: scheduler 判定的错过窗口任务、触发时间与当前运行态。
// OUTPUT: 持久 skipped run 与推进后的运行态；配置停用统一交给 scheduler CAS。
// POS: Automation misfire 结果记录层，不写任务配置定义。
package automation

import (
	"context"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

func (s *Service) nextRunAfterScheduledTrigger(
	job automationdomain.ScheduledTask,
	triggerKind string,
	scheduledFor time.Time,
) *time.Time {
	schedule := job.Schedule.Normalized()
	if triggerKind == automationdomain.TriggerKindScheduled && schedule.Kind == automationdomain.ScheduleKindEvery && schedule.IntervalSeconds != nil {
		interval := time.Duration(*schedule.IntervalSeconds) * time.Second
		next := scheduledFor.UTC().Add(interval)
		return &next
	}
	base := scheduledFor.UTC().Add(time.Second)
	if triggerKind == automationdomain.TriggerKindMisfire {
		base = s.nowFn().UTC()
	}
	return s.computeJobNext(job, base)
}

func (s *Service) recordSkippedMisfire(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	scheduledFor time.Time,
	now time.Time,
) (*automationdomain.ExecutionResult, error) {
	runID := s.idFactory("run")
	message := "scheduled task missed its execution window; misfire_policy=skip"
	if err := s.repository.InsertRunPending(ctx, automationstore.RunPendingInput{
		RunID:          runID,
		JobID:          job.JobID,
		OwnerUserID:    job.OwnerUserID,
		ScheduledFor:   &scheduledFor,
		TriggerKind:    automationdomain.TriggerKindMisfire,
		DeliveryMode:   job.Delivery.Mode,
		DeliveryTo:     deliveryTargetSummary(job.Delivery),
		DeliveryTarget: cloneDeliveryTargetPointer(job.Delivery),
		Status:         automationdomain.RunStatusSkipped,
	}); err != nil {
		return nil, err
	}
	if err := s.repository.MarkRunFinished(ctx, automationstore.RunFinishInput{
		RunID:        runID,
		Status:       automationdomain.RunStatusSkipped,
		FinishedAt:   now,
		ErrorMessage: &message,
	}); err != nil {
		return nil, err
	}
	s.advanceJobRuntimeAfterMisfire(job.JobID, now)
	return &automationdomain.ExecutionResult{
		JobID:        job.JobID,
		RunID:        &runID,
		Status:       automationdomain.RunStatusSkipped,
		ScheduledFor: cloneTimePointer(&scheduledFor),
		ErrorMessage: &message,
	}, nil
}

func (s *Service) advanceJobRuntimeAfterMisfire(jobID string, now time.Time) {
	s.mu.Lock()
	state := s.jobStates[jobID]
	if state == nil {
		s.mu.Unlock()
		return
	}
	state.LastRunAt = cloneTimePointer(&now)
	state.LastRunStatus = automationdomain.RunStatusSkipped
	state.NextRunAt = s.computeJobNext(state.Job, now)
	runtimeSnapshot := jobRuntimeUpdateFromState(jobID, state)
	s.mu.Unlock()

	s.persistJobRuntime(context.Background(), runtimeSnapshot)
	s.wakeScheduler()
}

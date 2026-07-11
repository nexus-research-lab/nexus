package automation

import (
	"context"
	"strings"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

func (s *Service) resultForExternallyClaimedJob(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	scheduledFor time.Time,
) (*automationdomain.ExecutionResult, error) {
	current, err := s.repository.GetScheduledTask(ctx, "", strings.TrimSpace(job.JobID))
	if err != nil {
		return nil, err
	}
	message := "scheduled task execution was claimed by another scheduler"
	if current != nil {
		s.replaceJobRuntimeState(*current)
		if strings.TrimSpace(current.RunningRunID) != "" {
			runID := strings.TrimSpace(current.RunningRunID)
			return &automationdomain.ExecutionResult{
				JobID:        job.JobID,
				RunID:        &runID,
				Status:       automationdomain.RunStatusRunning,
				ScheduledFor: cloneTimePointer(&scheduledFor),
				ErrorMessage: &message,
			}, nil
		}
		if !current.Enabled {
			disabledMessage := "scheduled task is disabled"
			return &automationdomain.ExecutionResult{
				JobID:        job.JobID,
				Status:       automationdomain.RunStatusSkipped,
				ScheduledFor: cloneTimePointer(&scheduledFor),
				ErrorMessage: &disabledMessage,
			}, nil
		}
	}
	return &automationdomain.ExecutionResult{
		JobID:        job.JobID,
		Status:       automationdomain.RunStatusRunning,
		ScheduledFor: cloneTimePointer(&scheduledFor),
		ErrorMessage: &message,
	}, nil
}

func (s *Service) recordSkippedOverlap(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	triggerKind string,
	scheduledFor time.Time,
	persistRuntime bool,
) (*automationdomain.ExecutionResult, error) {
	runID := s.idFactory("run")
	message := "previous run is still running; overlap_policy=skip"
	if err := s.repository.InsertRunPending(ctx, automationstore.RunPendingInput{
		RunID:        runID,
		JobID:        job.JobID,
		OwnerUserID:  job.OwnerUserID,
		ScheduledFor: &scheduledFor,
		TriggerKind:  triggerKind,
		DeliveryMode: strings.TrimSpace(job.Delivery.Mode),
		DeliveryTo:   deliveryTargetSummary(job.Delivery),
		Status:       automationdomain.RunStatusSkipped,
	}); err != nil {
		return nil, err
	}
	finishedAt := s.nowFn()
	_ = s.repository.MarkRunFinished(context.Background(), automationstore.RunFinishInput{
		RunID:        runID,
		Status:       automationdomain.RunStatusSkipped,
		FinishedAt:   finishedAt,
		ErrorMessage: &message,
	})
	if triggerKind == automationdomain.TriggerKindScheduled {
		if persistRuntime {
			s.advanceJobRuntimeAfterTrigger(job.JobID, scheduledFor)
		} else {
			s.advanceJobRuntimeAfterExternalClaim(job.JobID, scheduledFor)
		}
	}
	return &automationdomain.ExecutionResult{
		JobID:        job.JobID,
		RunID:        &runID,
		Status:       automationdomain.RunStatusSkipped,
		ScheduledFor: cloneTimePointer(&scheduledFor),
		ErrorMessage: &message,
	}, nil
}

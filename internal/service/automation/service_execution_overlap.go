package automation

import (
	"context"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

func (s *Service) resultForExternallyClaimedJob(
	ctx context.Context,
	job protocol.CronJob,
	scheduledFor time.Time,
) (*protocol.ExecutionResult, error) {
	current, err := s.repository.GetCronJob(ctx, "", strings.TrimSpace(job.JobID))
	if err != nil {
		return nil, err
	}
	message := "scheduled task execution was claimed by another scheduler"
	if current != nil {
		s.replaceJobRuntimeState(*current)
		if strings.TrimSpace(current.RunningRunID) != "" {
			runID := strings.TrimSpace(current.RunningRunID)
			return &protocol.ExecutionResult{
				JobID:        job.JobID,
				RunID:        &runID,
				Status:       protocol.RunStatusRunning,
				ScheduledFor: cloneTimePointer(&scheduledFor),
				ErrorMessage: &message,
			}, nil
		}
		if !current.Enabled {
			disabledMessage := "scheduled task is disabled"
			return &protocol.ExecutionResult{
				JobID:        job.JobID,
				Status:       protocol.RunStatusSkipped,
				ScheduledFor: cloneTimePointer(&scheduledFor),
				ErrorMessage: &disabledMessage,
			}, nil
		}
	}
	return &protocol.ExecutionResult{
		JobID:        job.JobID,
		Status:       protocol.RunStatusRunning,
		ScheduledFor: cloneTimePointer(&scheduledFor),
		ErrorMessage: &message,
	}, nil
}

func (s *Service) recordSkippedOverlap(
	ctx context.Context,
	job protocol.CronJob,
	triggerKind string,
	scheduledFor time.Time,
	persistRuntime bool,
) (*protocol.ExecutionResult, error) {
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
		Status:       protocol.RunStatusSkipped,
	}); err != nil {
		return nil, err
	}
	finishedAt := s.nowFn()
	_ = s.repository.MarkRunFinished(context.Background(), automationstore.RunFinishInput{
		RunID:        runID,
		Status:       protocol.RunStatusSkipped,
		FinishedAt:   finishedAt,
		ErrorMessage: &message,
	})
	if triggerKind == "cron" {
		if persistRuntime {
			s.advanceJobRuntimeAfterTrigger(job.JobID, scheduledFor)
		} else {
			s.advanceJobRuntimeAfterExternalClaim(job.JobID, scheduledFor)
		}
	}
	return &protocol.ExecutionResult{
		JobID:        job.JobID,
		RunID:        &runID,
		Status:       protocol.RunStatusSkipped,
		ScheduledFor: cloneTimePointer(&scheduledFor),
		ErrorMessage: &message,
	}, nil
}

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
	current, err := s.repository.GetScheduledTask(
		ctx, strings.TrimSpace(job.OwnerUserID), strings.TrimSpace(job.JobID),
	)
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
	finishedAt := s.nowFn()
	runInput := skippedOverlapRunInput(
		job, triggerKind, scheduledFor, runID, finishedAt, manualRunIdentity{},
	)
	if err := s.repository.InsertRunPending(ctx, runInput); err != nil {
		return nil, err
	}
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

func skippedOverlapRunInput(
	job automationdomain.ScheduledTask,
	triggerKind string,
	scheduledFor time.Time,
	runID string,
	finishedAt time.Time,
	request manualRunIdentity,
) automationstore.RunPendingInput {
	message := "previous run is still running; overlap_policy=skip"
	return automationstore.RunPendingInput{
		RunID: runID, JobID: job.JobID, OwnerUserID: job.OwnerUserID,
		ScheduledFor: &scheduledFor, TriggerKind: triggerKind,
		DeliveryMode: strings.TrimSpace(job.Delivery.Mode), DeliveryTo: deliveryTargetSummary(job.Delivery),
		DeliveryTarget: cloneDeliveryTargetPointer(job.Delivery), Status: automationdomain.RunStatusSkipped,
		DeliveryStatus: automationdomain.DeliveryStatusNotAttempted,
		FinishedAt:     &finishedAt, ErrorMessage: &message,
		PermissionPolicyRevision: job.PermissionPolicy.Revision,
		ClientRequestID:          strings.TrimSpace(request.RequestID), IntentDigest: strings.TrimSpace(request.IntentDigest),
	}
}

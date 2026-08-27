package automation

import (
	"context"
	"strings"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

func (s *Service) observeJobRun(
	job automationdomain.ScheduledTask,
	runID string,
	roundID string,
	sessionKey string,
	sink *automationexec.ExecutionSink,
	cleanup func(),
) {
	s.observeJobRunWithCompletion(job, runID, roundID, sessionKey, sink, cleanup, nil, nil)
}

func (s *Service) observeJobRunWithCompletion(
	job automationdomain.ScheduledTask,
	runID string,
	roundID string,
	sessionKey string,
	sink *automationexec.ExecutionSink,
	cleanup func(),
	completion func(),
	resumeAttempt *permissionResumeAttempt,
) {
	if completion != nil {
		defer completion()
	}
	defer cleanup()
	defer sink.Close()

	jobCtx := backgroundContextForJobOwner(job)
	waitCtx, cancel := context.WithTimeout(context.Background(), automationexec.WaitTimeout(0))
	defer cancel()
	observation := sink.WaitForRound(waitCtx, roundID)
	observation.RunID = strings.TrimSpace(runID)
	observation.RoundID = strings.TrimSpace(roundID)
	if s.finishPermissionBlockedObservation(jobCtx, job, runID) {
		return
	}

	status := observation.Status
	if status == "" {
		status = automationdomain.RunStatusFailed
	}
	errorMessage := cloneStringPointer(observation.ErrorMessage)
	if status == automationdomain.RunStatusSucceeded && resumeAttempt != nil {
		if resumeErr := resumeAttempt.validationError(); resumeErr != nil {
			status = automationdomain.RunStatusFailed
			errorMessage = errorPointer(resumeErr)
		}
	}
	runDelivery := s.persistedRunDeliveryTarget(jobCtx, job, runID)
	deliveryStatus := initialRunDeliveryStatus(runDelivery, sessionKey, observation, status)
	deliveryTo := deliveryTargetSummary(runDelivery)
	finishedAt := s.nowFn()
	logger := s.loggerFor(jobCtx).With(
		"job_id", job.JobID,
		"agent_id", job.AgentID,
		"run_id", runID,
		"round_id", roundID,
	)
	if errorMessage != nil {
		logger.Error("自动化任务执行结束",
			"status", status,
			"delivery_status", deliveryStatus,
			"message_count", observation.MessageCount,
			"session_id", anyStringPointer(observation.SessionID),
			"err", *errorMessage,
		)
	} else {
		logger.Info("自动化任务执行结束",
			"status", status,
			"delivery_status", deliveryStatus,
			"message_count", observation.MessageCount,
			"session_id", anyStringPointer(observation.SessionID),
		)
	}
	resultSummary := stringPointer(firstNonEmpty(observation.ResultText, observation.AssistantText))
	assistantText := stringPointer(observation.AssistantText)
	resultText := stringPointer(observation.ResultText)
	artifactPath := s.writeRunArtifact(jobCtx, job, runID, roundID, sessionKey, finishedAt, status, observation, errorMessage, deliveryStatus, nil, deliveryTo)
	updated, committed, finishErr := s.commitObservedRunTerminal(jobCtx, job, automationstore.RunFinishInput{
		RunID:          runID,
		Status:         status,
		FinishedAt:     finishedAt,
		ErrorMessage:   errorMessage,
		SessionID:      observation.SessionID,
		MessageCount:   observation.MessageCount,
		ResultSummary:  resultSummary,
		AssistantText:  assistantText,
		ResultText:     resultText,
		ArtifactPath:   artifactPath,
		DeliveryTo:     deliveryTo,
		DeliveryStatus: deliveryStatus,
	})
	if finishErr != nil && !committed {
		logger.Warn("自动化任务结束结果写入失败",
			"status", status,
			"err", finishErr,
		)
		return
	}
	if !committed {
		logger.Warn("自动化任务结束结果已忽略，run 不再处于活动状态",
			"status", status,
		)
		return
	}
	if finishErr != nil {
		deliveryState := automationdomain.DeliveryStatusRetrying
		if updated != nil && strings.TrimSpace(updated.DeliveryStatus) != "" {
			deliveryState = updated.DeliveryStatus
		}
		logger.Warn("自动化任务已完成，但结果投递需要核对", "delivery_status", deliveryState, "err", finishErr)
	}
}

func (s *Service) finishPermissionBlockedObservation(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	runID string,
) bool {
	run, err := s.repository.GetRun(ctx, job.OwnerUserID, job.JobID, runID)
	if err != nil || run == nil || strings.TrimSpace(run.BlockState) == "" {
		return false
	}
	permissionState := taskPermissionStateForRunBlock(run.BlockState)
	s.pauseJobRuntimeForPermission(job, runID, permissionState, run.ErrorMessage)
	s.loggerFor(ctx).Info("自动化任务已暂停并等待用户交互",
		"job_id", job.JobID,
		"run_id", runID,
		"block_state", run.BlockState,
		"request_id", run.BlockedRequestID,
	)
	return true
}

func taskPermissionStateForRunBlock(blockState string) string {
	switch strings.TrimSpace(blockState) {
	case automationdomain.RunBlockStateAwaitingReauth:
		return automationdomain.TaskPermissionStateAwaitingReauth
	case automationdomain.RunBlockStateAwaitingInput:
		return automationdomain.TaskPermissionStateAwaitingInput
	case automationdomain.RunBlockStateReadyToRetry:
		return automationdomain.TaskPermissionStateReadyToRetry
	default:
		return automationdomain.TaskPermissionStateAwaitingApproval
	}
}

package automation

import (
	"context"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/automation"
)

func (s *Service) observeJobRun(
	job protocol.CronJob,
	runID string,
	roundID string,
	sessionKey string,
	sink *automationdomain.ExecutionSink,
	cleanup func(),
) {
	defer cleanup()
	defer sink.Close()

	jobCtx := backgroundContextForJobOwner(job)
	waitCtx, cancel := context.WithTimeout(context.Background(), automationdomain.WaitTimeout(0))
	defer cancel()
	observation := sink.WaitForRound(waitCtx, roundID)

	status := observation.Status
	if status == "" {
		status = protocol.RunStatusFailed
	}
	errorMessage := cloneStringPointer(observation.ErrorMessage)
	deliveryResult := jobDeliveryResult{Status: protocol.DeliveryStatusNotRequired}
	if status == protocol.RunStatusSucceeded {
		deliveryResult = s.deliverJobObservation(jobCtx, job, sessionKey, observation)
	}
	deliveryStatus := deliveryResult.Status
	deliveryError := deliveryResult.Error
	deliveryTo := deliveryResult.deliveryTo(job.Delivery)
	finishedAt := s.nowFn()
	deliveredAt := deliveredAtForStatus(deliveryStatus, finishedAt)
	deliveryAttemptsAfter := 0
	if deliveryAttempted(deliveryStatus) {
		deliveryAttemptsAfter = 1
	}
	nextDeliveryAttemptAt, deliveryDeadLetterAt := deliveryRetrySchedule(deliveryStatus, deliveryAttemptsAfter, finishedAt)
	logger := s.loggerFor(jobCtx).With(
		"job_id", job.JobID,
		"agent_id", job.AgentID,
		"run_id", runID,
		"round_id", roundID,
	)
	if errorMessage != nil || deliveryError != nil {
		logError := ""
		if errorMessage != nil {
			logError = *errorMessage
		} else if deliveryError != nil {
			logError = *deliveryError
		}
		logger.Error("自动化任务执行结束",
			"status", status,
			"delivery_status", deliveryStatus,
			"message_count", observation.MessageCount,
			"session_id", anyStringPointer(observation.SessionID),
			"err", logError,
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
	artifactPath := s.writeRunArtifact(jobCtx, job, runID, roundID, sessionKey, finishedAt, status, observation, errorMessage, deliveryStatus, deliveryError, deliveryTo)
	finished, finishErr := s.repository.MarkRunFinishedIfActive(context.Background(), automation.RunFinishInput{
		RunID:                 runID,
		Status:                status,
		FinishedAt:            finishedAt,
		ErrorMessage:          errorMessage,
		SessionID:             observation.SessionID,
		MessageCount:          observation.MessageCount,
		ResultSummary:         resultSummary,
		AssistantText:         assistantText,
		ResultText:            resultText,
		ArtifactPath:          artifactPath,
		DeliveryTo:            deliveryTo,
		DeliveryStatus:        deliveryStatus,
		DeliveryError:         deliveryError,
		DeliveredAt:           deliveredAt,
		DeliveryAttempted:     deliveryAttempted(deliveryStatus),
		DeliveryNextAttemptAt: nextDeliveryAttemptAt,
		DeliveryDeadLetterAt:  deliveryDeadLetterAt,
	})
	if finishErr != nil {
		logger.Warn("自动化任务结束结果写入失败",
			"status", status,
			"err", finishErr,
		)
		return
	}
	if !finished {
		logger.Warn("自动化任务结束结果已忽略，run 不再处于活动状态",
			"status", status,
		)
		return
	}
	s.finishJobRuntime(job.JobID, &finishedAt, status, errorMessage, deliveryStatus)
}

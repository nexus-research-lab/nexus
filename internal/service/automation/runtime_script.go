package automation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/protocol"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

const maxScriptOutputBytes = 128 * 1024

func (s *Service) startScriptJobExecution(ctx context.Context, job automationdomain.CronJob, triggerKind string, scheduledFor time.Time) (*automationdomain.ExecutionResult, error) {
	logger := s.loggerFor(ctx).With(
		"job_id", job.JobID,
		"agent_id", job.AgentID,
		"trigger_kind", triggerKind,
		"execution_kind", automationdomain.ExecutionKindScript,
	)
	runID := s.idFactory("run")
	state := s.ensureJobState(job)
	s.mu.Lock()
	overlapPolicy := automationdomain.NormalizeOverlapPolicy(job.OverlapPolicy)
	if state.Running && overlapPolicy == automationdomain.OverlapPolicySkip {
		s.mu.Unlock()
		logger.Warn("脚本自动化任务已在运行中")
		return s.recordSkippedOverlap(ctx, job, triggerKind, scheduledFor, true)
	}
	nextRunAt := cloneTimePointer(state.NextRunAt)
	if triggerKind == "cron" {
		nextRunAt = s.computeJobNext(job, scheduledFor.UTC().Add(time.Second))
	}
	s.mu.Unlock()

	startedAt := s.nowFn()
	claimed, err := s.repository.ClaimCronJobRuntime(ctx, automationstore.JobRuntimeClaimInput{
		JobID:         job.JobID,
		RunID:         runID,
		StartedAt:     startedAt,
		NextRunAt:     nextRunAt,
		OverlapPolicy: overlapPolicy,
		AllowDisabled: triggerKind == "manual",
	})
	if err != nil {
		logger.Error("脚本自动化任务领取执行权失败", "run_id", runID, "err", err)
		return nil, err
	}
	if !claimed {
		logger.Warn("脚本自动化任务执行权已被其他调度器领取", "run_id", runID)
		return s.resultForExternallyClaimedJob(ctx, job, scheduledFor)
	}

	s.mu.Lock()
	state = s.jobStates[job.JobID]
	if state == nil {
		state = &automationexec.JobRuntimeState{Job: job}
		s.jobStates[job.JobID] = state
	}
	state.RunningCount++
	state.Running = true
	state.RunningRunID = runID
	state.RunningStartedAt = cloneTimePointer(&startedAt)
	state.NextRunAt = cloneTimePointer(nextRunAt)
	s.mu.Unlock()

	if err := s.repository.InsertRunPending(ctx, automationstore.RunPendingInput{
		RunID:        runID,
		JobID:        job.JobID,
		OwnerUserID:  job.OwnerUserID,
		ScheduledFor: &scheduledFor,
		TriggerKind:  triggerKind,
		DeliveryMode: strings.TrimSpace(job.Delivery.Mode),
		DeliveryTo:   deliveryTargetSummary(job.Delivery),
	}); err != nil {
		s.finishJobRuntime(job.JobID, nil, automationdomain.RunStatusFailed, errorPointer(err))
		return nil, err
	}
	if err := s.repository.MarkRunRunning(ctx, runID, startedAt); err != nil {
		s.finishJobRuntime(job.JobID, nil, automationdomain.RunStatusFailed, errorPointer(err))
		return nil, err
	}

	go s.observeScriptJob(job, runID, scheduledFor)
	return &automationdomain.ExecutionResult{
		JobID:        job.JobID,
		RunID:        &runID,
		Status:       automationdomain.RunStatusRunning,
		ScheduledFor: cloneTimePointer(&scheduledFor),
		MessageCount: 0,
	}, nil
}

func (s *Service) observeScriptJob(job automationdomain.CronJob, runID string, scheduledFor time.Time) {
	jobCtx := backgroundContextForJobOwner(job)
	logger := s.loggerFor(jobCtx).With(
		"job_id", job.JobID,
		"agent_id", job.AgentID,
		"run_id", runID,
		"execution_kind", automationdomain.ExecutionKindScript,
	)
	observation := s.runScriptJob(jobCtx, job, runID)
	status := observation.Status
	if status == "" {
		status = automationdomain.RunStatusFailed
	}
	errorMessage := cloneStringPointer(observation.ErrorMessage)
	deliveryResult := jobDeliveryResult{Status: automationdomain.DeliveryStatusNotRequired}
	if status == automationdomain.RunStatusSucceeded {
		deliveryResult = s.deliverJobObservation(jobCtx, job, "", observation)
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
	artifactPath := s.writeRunArtifact(jobCtx, job, runID, "", "", finishedAt, status, observation, errorMessage, deliveryStatus, deliveryError, deliveryTo)
	resultSummary := stringPointer(firstNonEmpty(observation.ResultText, observation.AssistantText))
	finished, finishErr := s.repository.MarkRunFinishedIfActive(context.Background(), automationstore.RunFinishInput{
		RunID:                 runID,
		Status:                status,
		FinishedAt:            finishedAt,
		ErrorMessage:          errorMessage,
		MessageCount:          observation.MessageCount,
		ResultSummary:         resultSummary,
		AssistantText:         stringPointer(observation.AssistantText),
		ResultText:            stringPointer(observation.ResultText),
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
		logger.Warn("脚本自动化任务结束结果写入失败", "status", status, "scheduled_for", scheduledFor, "err", finishErr)
		return
	}
	if !finished {
		logger.Warn("脚本自动化任务结束结果已忽略，run 不再处于活动状态", "status", status, "scheduled_for", scheduledFor)
		return
	}
	s.finishJobRuntime(job.JobID, &finishedAt, status, errorMessage, deliveryStatus)
	if errorMessage != nil || deliveryError != nil {
		logError := ""
		if errorMessage != nil {
			logError = *errorMessage
		} else if deliveryError != nil {
			logError = *deliveryError
		}
		logger.Error("脚本自动化任务执行结束", "status", status, "delivery_status", deliveryStatus, "scheduled_for", scheduledFor, "err", logError)
		return
	}
	logger.Info("脚本自动化任务执行结束", "status", status, "delivery_status", deliveryStatus, "scheduled_for", scheduledFor)
}

func (s *Service) runScriptJob(ctx context.Context, job automationdomain.CronJob, runID string) automationexec.ExecutionObservation {
	workspacePath, err := s.resolveAutomationWorkspacePath(ctx, job.AgentID)
	if err != nil {
		message := err.Error()
		return automationexec.ExecutionObservation{Status: automationdomain.RunStatusFailed, ErrorMessage: &message}
	}
	if strings.TrimSpace(workspacePath) == "" {
		message := "automation script workspace is not configured"
		return automationexec.ExecutionObservation{Status: automationdomain.RunStatusFailed, ErrorMessage: &message}
	}

	waitCtx, cancel := context.WithTimeout(context.Background(), automationexec.WaitTimeout(0))
	defer cancel()

	stdout := &boundedOutputBuffer{limit: maxScriptOutputBytes}
	stderr := &boundedOutputBuffer{limit: maxScriptOutputBytes}
	command := scriptCommand(waitCtx, job.Instruction)
	command.Dir = workspacePath
	command.Env = append(os.Environ(),
		"NEXUS_AUTOMATION_JOB_ID="+strings.TrimSpace(job.JobID),
		"NEXUS_AUTOMATION_RUN_ID="+strings.TrimSpace(runID),
		"NEXUS_AUTOMATION_AGENT_ID="+strings.TrimSpace(job.AgentID),
	)
	command.Stdout = stdout
	command.Stderr = stderr

	status := automationdomain.RunStatusSucceeded
	var errorMessage *string
	if err = command.Run(); err != nil {
		status = automationdomain.RunStatusFailed
		if errors.Is(waitCtx.Err(), context.DeadlineExceeded) {
			status = automationdomain.RunStatusCancelled
			errorMessage = stringPointer("script timed out")
		} else {
			errorMessage = stringPointer(err.Error())
		}
	}
	resultText := formatScriptOutput(stdout.String(), stderr.String())
	if resultText == "" && errorMessage != nil {
		resultText = *errorMessage
	}
	return automationexec.ExecutionObservation{
		Status:       status,
		MessageCount: 1,
		ErrorMessage: errorMessage,
		ResultText:   resultText,
	}
}

func scriptCommand(ctx context.Context, script string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/C", script)
	}
	return exec.CommandContext(ctx, "/bin/sh", "-lc", script)
}

func formatScriptOutput(stdout string, stderr string) string {
	stdout = strings.TrimSpace(stdout)
	stderr = strings.TrimSpace(stderr)
	switch {
	case stdout != "" && stderr != "":
		return "STDOUT:\n" + stdout + "\n\nSTDERR:\n" + stderr
	case stdout != "":
		return stdout
	case stderr != "":
		return "STDERR:\n" + stderr
	default:
		return ""
	}
}

type boundedOutputBuffer struct {
	buffer    bytes.Buffer
	limit     int
	truncated int
}

func (b *boundedOutputBuffer) Write(payload []byte) (int, error) {
	if b.limit <= 0 {
		return len(payload), nil
	}
	remaining := b.limit - b.buffer.Len()
	if remaining > 0 {
		writeLen := len(payload)
		if writeLen > remaining {
			writeLen = remaining
		}
		_, _ = b.buffer.Write(payload[:writeLen])
		if writeLen < len(payload) {
			b.truncated += len(payload) - writeLen
		}
	} else {
		b.truncated += len(payload)
	}
	return len(payload), nil
}

func (b *boundedOutputBuffer) String() string {
	result := b.buffer.String()
	if b.truncated > 0 {
		result += fmt.Sprintf("\n\n[output truncated: %d bytes omitted]", b.truncated)
	}
	return result
}

// INPUT: 人类控制面创建的 script 任务、owner workspace 与 runtime isolation 配置。
// OUTPUT: 隔离执行结果及不继承宿主凭据的脚本进程环境。
// POS: automation script 的宿主执行与凭据边界。
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
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/runtime/workspaceisolation"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

const maxScriptOutputBytes = 128 * 1024

func (s *Service) startScriptJobExecution(ctx context.Context, job automationdomain.ScheduledTask, triggerKind string, scheduledFor time.Time) (*automationdomain.ExecutionResult, error) {
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
	if triggerKind == automationdomain.TriggerKindScheduled || triggerKind == automationdomain.TriggerKindMisfire {
		nextRunAt = s.nextRunAfterScheduledTrigger(job, triggerKind, scheduledFor)
	}
	s.mu.Unlock()

	startedAt := s.nowFn()
	claimed, err := s.repository.ClaimScheduledTaskRuntime(ctx, automationstore.JobRuntimeClaimInput{
		JobID:         job.JobID,
		RunID:         runID,
		StartedAt:     startedAt,
		NextRunAt:     nextRunAt,
		OverlapPolicy: overlapPolicy,
		AllowDisabled: triggerKind == automationdomain.TriggerKindManual,
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
		RunID:                    runID,
		JobID:                    job.JobID,
		OwnerUserID:              job.OwnerUserID,
		ScheduledFor:             &scheduledFor,
		TriggerKind:              triggerKind,
		DeliveryMode:             strings.TrimSpace(job.Delivery.Mode),
		DeliveryTo:               deliveryTargetSummary(job.Delivery),
		DeliveryTarget:           cloneDeliveryTargetPointer(job.Delivery),
		PermissionPolicyRevision: job.PermissionPolicy.Revision,
	}); err != nil {
		s.finishJobRuntime(job.JobID, nil, automationdomain.RunStatusFailed, errorPointer(err))
		return nil, err
	}
	if err := s.repository.MarkRunRunning(ctx, runID, startedAt); err != nil {
		s.finishJobRuntime(job.JobID, nil, automationdomain.RunStatusFailed, errorPointer(err))
		return nil, err
	}
	allowed, err := s.ensureScriptRunPermission(ctx, job, runID)
	if err != nil {
		finishedAt := s.nowFn()
		_ = s.repository.MarkRunFinished(context.Background(), automationstore.RunFinishInput{
			RunID:        runID,
			Status:       automationdomain.RunStatusFailed,
			FinishedAt:   finishedAt,
			ErrorMessage: errorPointer(err),
		})
		s.finishJobRuntime(job.JobID, &finishedAt, automationdomain.RunStatusFailed, errorPointer(err))
		return nil, err
	}
	if !allowed {
		return &automationdomain.ExecutionResult{
			JobID:        job.JobID,
			RunID:        &runID,
			Status:       automationdomain.RunStatusPending,
			ScheduledFor: cloneTimePointer(&scheduledFor),
		}, nil
	}
	if err = s.repository.MarkRunEffectStarted(ctx, job.OwnerUserID, runID); err != nil {
		finishedAt := s.nowFn()
		_ = s.repository.MarkRunFinished(context.Background(), automationstore.RunFinishInput{
			RunID:        runID,
			Status:       automationdomain.RunStatusFailed,
			FinishedAt:   finishedAt,
			ErrorMessage: errorPointer(err),
		})
		s.finishJobRuntime(job.JobID, &finishedAt, automationdomain.RunStatusFailed, errorPointer(err))
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

func (s *Service) ensureScriptRunPermission(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	runID string,
) (bool, error) {
	capability := buildScriptPermissionCapability(job)
	allowed, hardDenied, err := s.taskPolicyAllowsCapability(ctx, job, capability)
	if err != nil {
		return false, err
	}
	if hardDenied {
		return false, errors.New("当前 Agent 已明确禁用 host script 执行")
	}
	if allowed {
		return true, nil
	}
	request, created, err := s.repository.CreatePermissionRequestAndBlockRun(
		ctx,
		automationstore.PermissionRequestCreateInput{
			Request: automationdomain.AutomationPermissionRequest{
				RequestID:      s.idFactory("permission"),
				OwnerUserID:    job.OwnerUserID,
				JobID:          job.JobID,
				RunID:          runID,
				PolicyRevision: job.PermissionPolicy.Revision,
				Kind:           automationdomain.PermissionRequestKindScript,
				Capability:     capability,
				InputSummary:   map[string]any{"script_sha256": capability.ResourceScope},
				Title:          "定时任务请求执行工作区脚本",
				Description:    "脚本会在目标 Agent workspace 中执行；授权与当前脚本内容哈希绑定，脚本修改后自动失效。",
				Reason:         "需要 owner 确认工作区脚本执行",
				DeliverySessionKey: firstNonEmpty(
					job.Delivery.SessionKey,
					job.Source.SessionKey,
				),
				ResumeSafe: true,
			},
			TaskState:  automationdomain.TaskPermissionStateAwaitingApproval,
			BlockState: automationdomain.RunBlockStateAwaitingApproval,
		},
	)
	if err != nil {
		return false, err
	}
	s.setJobPermissionState(job.JobID, automationdomain.TaskPermissionStateAwaitingApproval, request.RequestID)
	s.pauseJobRuntimeForPermission(job, runID, automationdomain.TaskPermissionStateAwaitingApproval, &request.Reason)
	if created {
		s.recordTaskEvent(ctx, automationdomain.TaskEventActionPermissionRequested, job, runID, map[string]any{
			"request_id":   request.RequestID,
			"request_kind": request.Kind,
			"effect":       request.Capability.Effect,
			"resume_safe":  request.ResumeSafe,
		})
		s.notifyAutomationPermissionRequest(ctx, job, *request)
	}
	return false, nil
}

func (s *Service) observeScriptJob(job automationdomain.ScheduledTask, runID string, scheduledFor time.Time) {
	jobCtx := backgroundContextForJobOwner(job)
	logger := s.loggerFor(jobCtx).With(
		"job_id", job.JobID,
		"agent_id", job.AgentID,
		"run_id", runID,
		"execution_kind", automationdomain.ExecutionKindScript,
	)
	observation := s.runScriptJob(jobCtx, job, runID)
	observation.RunID = strings.TrimSpace(runID)
	status := observation.Status
	if status == "" {
		status = automationdomain.RunStatusFailed
	}
	errorMessage := cloneStringPointer(observation.ErrorMessage)
	deliveryResult := jobDeliveryResult{Status: automationdomain.DeliveryStatusNotRequired}
	runDelivery := s.persistedRunDeliveryTarget(jobCtx, job, runID)
	if status == automationdomain.RunStatusSucceeded {
		deliveryResult = s.deliverJobObservationToTarget(jobCtx, job, runDelivery, "", observation)
	}
	deliveryStatus := deliveryResult.Status
	deliveryError := deliveryResult.Error
	deliveryTo := deliveryResult.deliveryTo(runDelivery)
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

func (s *Service) runScriptJob(ctx context.Context, job automationdomain.ScheduledTask, runID string) automationexec.ExecutionObservation {
	workspacePath, workspaceRoot, err := s.openAutomationScriptWorkspace(ctx, job)
	if err != nil {
		message := err.Error()
		return automationexec.ExecutionObservation{Status: automationdomain.RunStatusFailed, ErrorMessage: &message}
	}
	if workspaceRoot != nil {
		defer workspaceRoot.Close()
	}
	if strings.TrimSpace(workspacePath) == "" {
		message := "automation script workspace is not configured"
		return automationexec.ExecutionObservation{Status: automationdomain.RunStatusFailed, ErrorMessage: &message}
	}

	waitCtx, cancel := context.WithTimeout(context.Background(), automationexec.WaitTimeout(0))
	defer cancel()

	stdout := &boundedOutputBuffer{limit: maxScriptOutputBytes}
	stderr := &boundedOutputBuffer{limit: maxScriptOutputBytes}
	var runErr error
	if strings.EqualFold(strings.TrimSpace(s.config.AppMode), "desktop") {
		command := scriptCommand(waitCtx, job.Instruction)
		command.Dir = workspacePath
		command.Env = scriptProcessEnvironment(workspacePath, job, runID)
		command.Stdout = stdout
		command.Stderr = stderr
		runErr = command.Run()
	} else {
		runErr = workspaceisolation.RunScript(
			waitCtx,
			workspaceisolation.Config{
				Mode:         workspaceisolation.Mode(strings.ToLower(strings.TrimSpace(s.config.RuntimeIsolationMode))),
				LauncherPath: s.config.RuntimeLauncherPath,
			},
			workspaceisolation.ScriptInput{
				OwnerUserID: strings.TrimSpace(job.OwnerUserID),
				CWD:         workspacePath,
				Script:      job.Instruction,
				Environment: map[string]string{
					"NEXUS_AUTOMATION_JOB_ID":    strings.TrimSpace(job.JobID),
					"NEXUS_AUTOMATION_RUN_ID":    strings.TrimSpace(runID),
					"NEXUS_AUTOMATION_AGENT_ID":  strings.TrimSpace(job.AgentID),
					"NEXUS_AUTOMATION_EXECUTION": "script",
				},
			},
			stdout,
			stderr,
		)
	}

	status := automationdomain.RunStatusSucceeded
	var errorMessage *string
	if runErr != nil {
		status = automationdomain.RunStatusFailed
		if errors.Is(waitCtx.Err(), context.DeadlineExceeded) {
			status = automationdomain.RunStatusCancelled
			errorMessage = stringPointer("script timed out")
		} else {
			errorMessage = stringPointer(runErr.Error())
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

func (s *Service) openAutomationScriptWorkspace(
	ctx context.Context,
	job automationdomain.ScheduledTask,
) (string, *confinedfs.Root, error) {
	ownerUserID := strings.TrimSpace(job.OwnerUserID)
	if ownerUserID == "" {
		return "", nil, errors.New("automation script 缺少 owner_user_id")
	}
	workspacePath := strings.TrimSpace(s.config.WorkspacePath)
	if s.agents != nil && strings.TrimSpace(job.AgentID) != "" {
		agentValue, err := s.agents.GetAgent(ctx, strings.TrimSpace(job.AgentID))
		if err != nil {
			return "", nil, err
		}
		if strings.TrimSpace(agentValue.OwnerUserID) != ownerUserID {
			return "", nil, errors.New("automation script agent owner does not match job owner")
		}
		workspacePath = strings.TrimSpace(agentValue.WorkspacePath)
	}
	if workspacePath == "" {
		return "", nil, nil
	}
	root, err := workspacestore.New(s.config.WorkspacePath).OpenOwnerWorkspacePath(
		ownerUserID,
		workspacePath,
		true,
	)
	if err != nil {
		return "", nil, err
	}
	return workspacePath, root, nil
}

func scriptCommand(ctx context.Context, script string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/C", script)
	}
	return exec.CommandContext(ctx, "/bin/sh", "-c", script)
}

func scriptProcessEnvironment(
	workspacePath string,
	job automationdomain.ScheduledTask,
	runID string,
) []string {
	environment := make([]string, 0, 16)
	for _, name := range []string{
		"PATH",
		"LANG",
		"LC_ALL",
		"LC_CTYPE",
		"TZ",
		"SystemRoot",
		"ComSpec",
		"PATHEXT",
	} {
		if value, ok := os.LookupEnv(name); ok && strings.TrimSpace(value) != "" {
			environment = append(environment, name+"="+value)
		}
	}
	tempDir := os.TempDir()
	environment = append(environment,
		"HOME="+strings.TrimSpace(workspacePath),
		"USERPROFILE="+strings.TrimSpace(workspacePath),
		"TMPDIR="+tempDir,
		"TMP="+tempDir,
		"TEMP="+tempDir,
		"NEXUS_AUTOMATION_JOB_ID="+strings.TrimSpace(job.JobID),
		"NEXUS_AUTOMATION_RUN_ID="+strings.TrimSpace(runID),
		"NEXUS_AUTOMATION_AGENT_ID="+strings.TrimSpace(job.AgentID),
		"NEXUS_AUTOMATION_EXECUTION=script",
	)
	return environment
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

// INPUT: 人类控制面创建的 script 任务、owner workspace 与 runtime isolation 配置。
// OUTPUT: 原子受理后的隔离执行结果、exact request 重放及不继承宿主凭据的脚本进程环境。
// POS: automation script 的宿主执行与凭据边界；进程只能在 claim+run commit 后启动。
package automation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
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

func (s *Service) startScriptJobExecution(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	triggerKind string,
	scheduledFor time.Time,
	claimExpectation runtimeClaimExpectation,
	request manualRunIdentity,
) (*automationdomain.ExecutionResult, error) {
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
	localOverlap := state.Running && overlapPolicy == automationdomain.OverlapPolicySkip
	if localOverlap &&
		strings.TrimSpace(request.RequestID) == "" {
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
	claimInput := automationstore.JobRuntimeClaimInput{
		OwnerUserID:   job.OwnerUserID,
		JobID:         job.JobID,
		RunID:         runID,
		StartedAt:     startedAt,
		NextRunAt:     nextRunAt,
		OverlapPolicy: overlapPolicy,
		AllowDisabled: triggerKind == automationdomain.TriggerKindManual,
	}
	claimExpectation.apply(&claimInput)
	var overlapTerminalRun *automationstore.RunPendingInput
	if triggerKind == automationdomain.TriggerKindManual &&
		strings.TrimSpace(request.RequestID) != "" &&
		overlapPolicy == automationdomain.OverlapPolicySkip {
		terminal := skippedOverlapRunInput(
			job, triggerKind, scheduledFor, runID, startedAt, request,
		)
		overlapTerminalRun = &terminal
	}
	claimResult, err := s.repository.ClaimScheduledTaskRun(
		ctx,
		automationstore.InitialRunClaimInput{
			Runtime: claimInput,
			Run: automationstore.RunPendingInput{
				RunID:                    runID,
				JobID:                    job.JobID,
				OwnerUserID:              job.OwnerUserID,
				ScheduledFor:             &scheduledFor,
				TriggerKind:              triggerKind,
				DeliveryMode:             strings.TrimSpace(job.Delivery.Mode),
				DeliveryTo:               deliveryTargetSummary(job.Delivery),
				DeliveryTarget:           cloneDeliveryTargetPointer(job.Delivery),
				Status:                   automationdomain.RunStatusRunning,
				StartedAt:                cloneTimePointer(&startedAt),
				Attempts:                 1,
				PermissionPolicyRevision: job.PermissionPolicy.Revision,
				ClientRequestID:          strings.TrimSpace(request.RequestID),
				IntentDigest:             strings.TrimSpace(request.IntentDigest),
			},
			OverlapTerminalRun: overlapTerminalRun,
		},
	)
	if err != nil {
		logger.Error("脚本自动化任务领取执行权失败", "run_id", runID, "err", err)
		return nil, err
	}
	if claimResult.Replayed {
		run, runErr := s.repository.GetRun(ctx, job.OwnerUserID, job.JobID, claimResult.RunID)
		if runErr != nil {
			return nil, runErr
		}
		return executionResultFromRun(*run, true), nil
	}
	if claimResult.Terminal {
		run, runErr := s.repository.GetRun(ctx, job.OwnerUserID, job.JobID, claimResult.RunID)
		if runErr != nil {
			return nil, runErr
		}
		s.refreshRuntimeProjectionBestEffort(ctx, job.OwnerUserID, job.JobID)
		return executionResultFromRun(*run, false), nil
	}
	if !claimResult.Claimed {
		logger.Warn("脚本自动化任务执行权已被其他调度器领取", "run_id", runID)
		return s.resultForExternallyClaimedJob(ctx, job, scheduledFor)
	}
	if claimExpectation.resetDeniedPermission {
		s.setJobPermissionState(job.JobID, automationdomain.TaskPermissionStateReady, "")
	}

	s.mu.Lock()
	state = s.jobStates[job.JobID]
	if state == nil {
		state = &automationexec.JobRuntimeState{Job: job}
		s.jobStates[job.JobID] = state
	}
	if localOverlap && strings.TrimSpace(request.RequestID) != "" {
		state.RunningCount = 1
	} else {
		state.RunningCount++
	}
	state.Running = true
	state.RunningRunID = runID
	state.RunningStartedAt = cloneTimePointer(&startedAt)
	state.NextRunAt = cloneTimePointer(nextRunAt)
	s.mu.Unlock()

	allowed, err := s.ensureScriptRunPermission(ctx, job, runID)
	if err != nil {
		_ = s.commitFailedRunTerminal(backgroundContextForJobOwner(job), job, runID, err)
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
		_ = s.commitFailedRunTerminal(backgroundContextForJobOwner(job), job, runID, err)
		return nil, err
	}

	if err = s.launchScriptObservation(job, runID, scheduledFor); err != nil {
		_ = s.commitFailedRunTerminal(backgroundContextForJobOwner(job), job, runID, err)
		return nil, err
	}
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
				RequestID:          s.idFactory("permission"),
				OwnerUserID:        job.OwnerUserID,
				JobID:              job.JobID,
				RunID:              runID,
				PolicyRevision:     job.PermissionPolicy.Revision,
				Kind:               automationdomain.PermissionRequestKindScript,
				Capability:         capability,
				InputSummary:       map[string]any{"script_sha256": capability.ResourceScope},
				Title:              "定时任务请求执行工作区脚本",
				Description:        "脚本会在目标 Agent workspace 中执行；授权与当前脚本内容哈希绑定，脚本修改后自动失效。",
				Reason:             "需要 owner 确认工作区脚本执行",
				DeliverySessionKey: automationPermissionApprovalSessionKey(job.Delivery, job.Source),
				ResumeSafe:         true,
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
		s.publishScheduledPermissionRequest(ctx, scheduledPermissionScope{
			Job: job, RunID: runID,
		}, *request)
	}
	return false, nil
}

func (s *Service) observeScriptJob(jobCtx context.Context, job automationdomain.ScheduledTask, runID string, scheduledFor time.Time) error {
	observation, terminationErr := s.runScriptJob(jobCtx, job, runID)
	return s.commitScriptObservation(jobCtx, job, runID, scheduledFor, observation, terminationErr)
}

func (s *Service) commitScriptObservation(
	jobCtx context.Context,
	job automationdomain.ScheduledTask,
	runID string,
	scheduledFor time.Time,
	observation automationexec.ExecutionObservation,
	terminationErr error,
) error {
	logger := s.loggerFor(jobCtx).With(
		"job_id", job.JobID,
		"agent_id", job.AgentID,
		"run_id", runID,
		"execution_kind", automationdomain.ExecutionKindScript,
	)
	if terminationErr != nil {
		status := automationdomain.RunStatusFailed
		if jobCtx.Err() != nil {
			status = automationdomain.RunStatusCancelled
		}
		message := "script process tree could not be confirmed stopped; manual review is required"
		observation.Status = status
		observation.ErrorMessage = &message
	}
	observation.RunID = strings.TrimSpace(runID)
	status := observation.Status
	if status == "" {
		status = automationdomain.RunStatusFailed
	}
	errorMessage := cloneStringPointer(observation.ErrorMessage)
	runDelivery := s.persistedRunDeliveryTarget(jobCtx, job, runID)
	deliveryStatus := initialRunDeliveryStatus(runDelivery, "", observation, status)
	deliveryTo := deliveryTargetSummary(runDelivery)
	finishedAt := s.nowFn()
	artifactPath := s.writeRunArtifact(jobCtx, job, runID, "", "", finishedAt, status, observation, errorMessage, deliveryStatus, nil, deliveryTo)
	resultSummary := stringPointer(firstNonEmpty(observation.ResultText, observation.AssistantText))
	updated, committed, finishErr := s.commitObservedRunTerminal(jobCtx, job, automationstore.RunFinishInput{
		RunID:          runID,
		Status:         status,
		FinishedAt:     finishedAt,
		ErrorMessage:   errorMessage,
		MessageCount:   observation.MessageCount,
		ResultSummary:  resultSummary,
		AssistantText:  stringPointer(observation.AssistantText),
		ResultText:     stringPointer(observation.ResultText),
		ArtifactPath:   artifactPath,
		DeliveryTo:     deliveryTo,
		DeliveryStatus: deliveryStatus,
	})
	if finishErr != nil && !committed {
		logger.Warn("脚本自动化任务结束结果写入失败", "status", status, "scheduled_for", scheduledFor, "err", finishErr)
		return terminationErr
	}
	if !committed {
		logger.Warn("脚本自动化任务结束结果已忽略，run 不再处于活动状态", "status", status, "scheduled_for", scheduledFor)
		return terminationErr
	}
	if errorMessage != nil {
		logger.Error("脚本自动化任务执行结束", "status", status, "delivery_status", deliveryStatus, "scheduled_for", scheduledFor, "err", *errorMessage)
		return terminationErr
	}
	if finishErr != nil {
		deliveryState := automationdomain.DeliveryStatusRetrying
		if updated != nil && strings.TrimSpace(updated.DeliveryStatus) != "" {
			deliveryState = updated.DeliveryStatus
		}
		logger.Warn("脚本任务已完成，但结果投递需要核对", "delivery_status", deliveryState, "scheduled_for", scheduledFor, "err", finishErr)
		return terminationErr
	}
	logger.Info("脚本自动化任务执行结束", "status", status, "delivery_status", deliveryStatus, "scheduled_for", scheduledFor)
	return terminationErr
}

func (s *Service) runScriptJob(ctx context.Context, job automationdomain.ScheduledTask, runID string) (automationexec.ExecutionObservation, error) {
	workspacePath, workspaceRoot, err := s.openAutomationScriptWorkspace(ctx, job)
	if err != nil {
		message := err.Error()
		return automationexec.ExecutionObservation{Status: automationdomain.RunStatusFailed, ErrorMessage: &message}, nil
	}
	if workspaceRoot != nil {
		defer workspaceRoot.Close()
	}
	if strings.TrimSpace(workspacePath) == "" {
		message := "automation script workspace is not configured"
		return automationexec.ExecutionObservation{Status: automationdomain.RunStatusFailed, ErrorMessage: &message}, nil
	}

	waitCtx, cancel := context.WithTimeout(ctx, automationexec.WaitTimeout(0))
	defer cancel()

	stdout := &boundedOutputBuffer{limit: maxScriptOutputBytes}
	stderr := &boundedOutputBuffer{limit: maxScriptOutputBytes}
	var runErr error
	var terminationErr error
	if strings.EqualFold(strings.TrimSpace(s.config.AppMode), "desktop") {
		runErr, terminationErr = runDesktopScript(
			waitCtx,
			job.Instruction,
			workspacePath,
			scriptProcessEnvironment(workspacePath, job, runID),
			stdout,
			stderr,
		)
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
		} else if errors.Is(waitCtx.Err(), context.Canceled) {
			status = automationdomain.RunStatusCancelled
			errorMessage = stringPointer("script was cancelled")
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
	}, terminationErr
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

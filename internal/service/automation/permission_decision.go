// INPUT: owner 的持久审批动作、被阻塞任务/run 与当前 connector readiness。
// OUTPUT: 原子决策、任务级或 run 级 grant，以及安全条件下同一 run_id 的新 attempt。
// POS: 自动化审批业务编排；审批成功与运行恢复分别落事实，允许崩溃后继续恢复。
package automation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

// ListPermissionRequests 返回当前 owner 的自动化交互请求。
func (s *Service) ListPermissionRequests(
	ctx context.Context,
	status string,
	jobID string,
) ([]automationdomain.AutomationPermissionRequest, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, scoped := scopedOwnerUserID(ctx)
	if !scoped || strings.TrimSpace(ownerUserID) == "" {
		return nil, errors.New("automation permission requests require an owner scope")
	}
	if strings.TrimSpace(status) == "" {
		status = automationdomain.PermissionRequestStatusPending
	}
	return s.repository.ListPermissionRequests(ctx, ownerUserID, status, strings.TrimSpace(jobID))
}

// ResolvePermissionRequest 提交审批卡决策，并在可安全重放时继续同一 logical run。
func (s *Service) ResolvePermissionRequest(
	ctx context.Context,
	requestID string,
	input automationdomain.PermissionDecisionInput,
) (*automationdomain.PermissionDecisionResult, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, scoped := scopedOwnerUserID(ctx)
	if !scoped || strings.TrimSpace(ownerUserID) == "" {
		return nil, errors.New("automation permission decision requires an owner scope")
	}
	request, err := s.repository.GetPermissionRequest(ctx, ownerUserID, strings.TrimSpace(requestID))
	if err != nil {
		return nil, err
	}
	if err = validatePermissionDecisionSnapshot(*request, input); err != nil {
		return nil, err
	}
	decision := strings.TrimSpace(input.Decision)
	if err = validateAutomationPermissionDecision(*request, decision); err != nil {
		return nil, err
	}
	job, err := s.repository.GetScheduledTask(ctx, ownerUserID, request.JobID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, automationdomain.ErrJobNotFound
	}
	ensuredJob, err := s.ensureTaskPermissionPolicy(ctx, *job)
	if err != nil {
		return nil, err
	}
	job = &ensuredJob
	if job.PermissionPolicy.Revision != request.PolicyRevision ||
		strings.TrimSpace(job.PendingPermissionRequestID) != strings.TrimSpace(request.RequestID) {
		return nil, automationdomain.ErrPermissionRequestStale
	}

	var run *automationdomain.ScheduledTaskRun
	if strings.TrimSpace(request.RunID) != "" {
		run, err = s.repository.GetRun(ctx, ownerUserID, job.JobID, request.RunID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, automationdomain.ErrRunNotFound
		}
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(run.BlockedRequestID) != strings.TrimSpace(request.RequestID) ||
			strings.TrimSpace(run.BlockState) == "" {
			return nil, automationdomain.ErrPermissionRequestStale
		}
	}
	if decision == automationdomain.PermissionDecisionRetry {
		if err = s.ensurePermissionConnectorReady(ctx, ownerUserID, request.Capability.ConnectorID); err != nil {
			return nil, err
		}
	}

	resumeSafe := request.ResumeSafe && (run == nil || !run.EffectStarted)
	taskState := automationdomain.TaskPermissionStateReady
	runBlockState := automationdomain.RunBlockStateReadyToRetry
	finishDenied := decision == automationdomain.PermissionDecisionDeny
	if finishDenied {
		taskState = automationdomain.TaskPermissionStateDenied
		runBlockState = automationdomain.RunBlockStateNone
	} else if !resumeSafe {
		taskState = automationdomain.TaskPermissionStateReadyToRetry
	}

	var nextPolicy *automationdomain.TaskPermissionPolicy
	if decision == automationdomain.PermissionDecisionAllowTask {
		capability := request.Capability
		capability.InputFingerprint = ""
		policy := appendTaskPermissionGrant(job.PermissionPolicy, automationdomain.TaskPermissionGrant{
			GrantID:    s.idFactory("grant"),
			Capability: capability,
			Source:     automationdomain.PermissionGrantSourceUserApproval,
			ApprovedAt: permissionGrantApprovedAt(s.nowFn()),
		})
		nextPolicy = &policy
	}
	resolvedAt := s.nowFn()
	deniedMessage := "用户拒绝了定时任务的权限请求"
	resolved, err := s.repository.ResolvePermissionRequest(ctx, automationstore.PermissionRequestDecisionStoreInput{
		OwnerUserID:       ownerUserID,
		RequestID:         request.RequestID,
		Decision:          decision,
		ResolvedByUserID:  authctx.OwnerUserID(ctx),
		ResolvedAt:        resolvedAt,
		ExpectedRevision:  request.PolicyRevision,
		NextPolicy:        nextPolicy,
		TaskState:         taskState,
		RunBlockState:     runBlockState,
		FinishRunAsDenied: finishDenied,
		DeniedMessage:     deniedMessage,
	})
	if err != nil {
		return nil, err
	}
	if nextPolicy != nil {
		job.PermissionPolicy = *nextPolicy
	}
	if run != nil && !finishDenied {
		run.PermissionPolicyRevision = job.PermissionPolicy.Revision
		run.BlockState = runBlockState
		run.BlockedRequestID = ""
	}
	job.PermissionState = taskState
	job.PendingPermissionRequestID = ""
	if taskState == automationdomain.TaskPermissionStateReadyToRetry {
		job.PendingPermissionRequestID = request.RequestID
	}
	s.setJobPermissionState(job.JobID, taskState, job.PendingPermissionRequestID)
	action := automationdomain.TaskEventActionPermissionApproved
	if finishDenied {
		action = automationdomain.TaskEventActionPermissionDenied
	}
	s.recordTaskEvent(ctx, action, *job, request.RunID, map[string]any{
		"request_id":   request.RequestID,
		"decision":     decision,
		"request_kind": request.Kind,
		"tool_name":    request.Capability.ToolName,
		"resume_safe":  resumeSafe,
	})

	result := &automationdomain.PermissionDecisionResult{
		Request: resolved,
		Task:    *job,
		Run:     run,
	}
	if finishDenied {
		s.finishDeniedPermissionRun(*job, request.RunID, resolvedAt, deniedMessage)
		s.notifyAutomationPermissionDecision(ctx, *job, *resolved, "权限请求已拒绝，本次运行已停止。")
		return s.refreshPermissionDecisionResult(ctx, ownerUserID, result)
	}
	if run == nil || !resumeSafe {
		message := "权限请求已批准。"
		if !resumeSafe {
			message = "权限请求已批准；为避免重复外部副作用，请在 Nexus 中显式确认重试。"
		}
		s.notifyAutomationPermissionDecision(ctx, *job, *resolved, message)
		return s.refreshPermissionDecisionResult(ctx, ownerUserID, result)
	}
	if resumeErr := s.resumePermissionBlockedRun(ctx, *job, *run, resolved); resumeErr != nil {
		s.loggerFor(ctx).Warn("权限已批准，但自动恢复 run 失败",
			"job_id", job.JobID,
			"run_id", run.RunID,
			"request_id", request.RequestID,
			"err", resumeErr,
		)
		_ = s.repository.SetTaskPermissionState(ctx, ownerUserID, job.JobID, automationdomain.TaskPermissionStateReadyToRetry, request.RequestID)
		job.PermissionState = automationdomain.TaskPermissionStateReadyToRetry
		job.PendingPermissionRequestID = request.RequestID
		s.setJobPermissionState(job.JobID, job.PermissionState, request.RequestID)
		s.notifyAutomationPermissionDecision(ctx, *job, *resolved, "权限已批准，但自动恢复失败；请在 Nexus 中显式重试。")
		return s.refreshPermissionDecisionResult(ctx, ownerUserID, result)
	}
	result.ResumeStarted = true
	s.recordTaskEvent(ctx, automationdomain.TaskEventActionPermissionRetry, *job, run.RunID, map[string]any{
		"request_id": request.RequestID,
		"automatic":  true,
	})
	s.notifyAutomationPermissionDecision(ctx, *job, *resolved, "权限请求已批准，任务已继续运行。")
	return s.refreshPermissionDecisionResult(ctx, ownerUserID, result)
}

// ResumePermissionRun 明确确认重放已产生副作用或启动恢复失败的 logical run。
func (s *Service) ResumePermissionRun(
	ctx context.Context,
	jobID string,
	runID string,
	input automationdomain.PermissionResumeInput,
) (*automationdomain.PermissionDecisionResult, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, scoped := scopedOwnerUserID(ctx)
	if !scoped || strings.TrimSpace(ownerUserID) == "" {
		return nil, errors.New("automation run resume requires an owner scope")
	}
	job, err := s.repository.GetScheduledTask(ctx, ownerUserID, strings.TrimSpace(jobID))
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, automationdomain.ErrJobNotFound
	}
	run, err := s.repository.GetRun(ctx, ownerUserID, job.JobID, strings.TrimSpace(runID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, automationdomain.ErrRunNotFound
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(run.BlockState) != automationdomain.RunBlockStateReadyToRetry {
		return nil, automationdomain.ErrPermissionRunNotResumable
	}
	if strings.TrimSpace(job.PendingPermissionRequestID) != strings.TrimSpace(input.RequestID) {
		return nil, automationdomain.ErrPermissionRequestStale
	}
	request, err := s.loadPermissionResumeRequest(ctx, *job, *run, input)
	if err != nil {
		return nil, err
	}
	if err = s.repository.SetTaskPermissionState(ctx, ownerUserID, job.JobID, automationdomain.TaskPermissionStateReady, ""); err != nil {
		return nil, err
	}
	job.PermissionState = automationdomain.TaskPermissionStateReady
	s.setJobPermissionState(job.JobID, job.PermissionState, "")
	if err = s.resumePermissionBlockedRun(ctx, *job, *run, request); err != nil {
		_ = s.repository.SetTaskPermissionState(ctx, ownerUserID, job.JobID, automationdomain.TaskPermissionStateReadyToRetry, "")
		job.PermissionState = automationdomain.TaskPermissionStateReadyToRetry
		s.setJobPermissionState(job.JobID, job.PermissionState, "")
		return nil, err
	}
	s.recordTaskEvent(ctx, automationdomain.TaskEventActionPermissionRetry, *job, run.RunID, map[string]any{
		"automatic": false,
	})
	result := &automationdomain.PermissionDecisionResult{
		Task:          *job,
		Run:           run,
		ResumeStarted: true,
	}
	return s.refreshPermissionDecisionResult(ctx, ownerUserID, result)
}

func validateAutomationPermissionDecision(request automationdomain.AutomationPermissionRequest, decision string) error {
	switch request.Kind {
	case automationdomain.PermissionRequestKindConnectorReauth:
		if decision == automationdomain.PermissionDecisionRetry || decision == automationdomain.PermissionDecisionDeny {
			return nil
		}
		return fmt.Errorf("%w: connector reauth request only accepts retry or deny", automationdomain.ErrPermissionDecisionInvalid)
	case automationdomain.PermissionRequestKindHumanInput:
		if decision == automationdomain.PermissionDecisionDeny {
			return nil
		}
		return fmt.Errorf("%w: human input request must be resolved by editing the task or denying the run", automationdomain.ErrPermissionDecisionInvalid)
	default:
		switch decision {
		case automationdomain.PermissionDecisionAllowOnce,
			automationdomain.PermissionDecisionAllowTask,
			automationdomain.PermissionDecisionDeny:
			return nil
		default:
			return fmt.Errorf("%w: decision must be one of allow_once, allow_task, deny", automationdomain.ErrPermissionDecisionInvalid)
		}
	}
}

func validatePermissionDecisionSnapshot(
	request automationdomain.AutomationPermissionRequest,
	input automationdomain.PermissionDecisionInput,
) error {
	if strings.TrimSpace(input.JobID) == "" ||
		strings.TrimSpace(input.RunID) == "" ||
		input.PolicyRevision <= 0 {
		return fmt.Errorf("%w: decision target snapshot is incomplete", automationdomain.ErrPermissionDecisionInvalid)
	}
	if strings.TrimSpace(input.JobID) != strings.TrimSpace(request.JobID) ||
		strings.TrimSpace(input.RunID) != strings.TrimSpace(request.RunID) ||
		input.PolicyRevision != request.PolicyRevision {
		return automationdomain.ErrPermissionRequestStale
	}
	return nil
}

func (s *Service) ensurePermissionConnectorReady(ctx context.Context, ownerUserID string, connectorID string) error {
	connectorID = strings.TrimSpace(connectorID)
	if connectorID == "" {
		return fmt.Errorf("%w: permission request is missing connector_id", automationdomain.ErrPermissionDecisionInvalid)
	}
	if s.connectors == nil {
		return fmt.Errorf("%w: connector readiness service is unavailable", automationdomain.ErrPermissionConnectorNotReady)
	}
	connection, err := s.connectors.LoadActiveConnection(ctx, ownerUserID, connectorID)
	if err != nil || connection == nil {
		return fmt.Errorf("%w: connector %s is not connected", automationdomain.ErrPermissionConnectorNotReady, connectorID)
	}
	return nil
}

func (s *Service) loadPermissionResumeRequest(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	run automationdomain.ScheduledTaskRun,
	input automationdomain.PermissionResumeInput,
) (*automationdomain.AutomationPermissionRequest, error) {
	requestID := strings.TrimSpace(input.RequestID)
	if requestID == "" || input.PolicyRevision <= 0 {
		return nil, fmt.Errorf("%w: resume target snapshot is incomplete", automationdomain.ErrPermissionDecisionInvalid)
	}
	request, err := s.repository.GetPermissionRequest(ctx, job.OwnerUserID, requestID)
	if err != nil {
		return nil, err
	}
	if request.PolicyRevision != input.PolicyRevision {
		return nil, automationdomain.ErrPermissionRequestStale
	}
	if err = validatePermissionResumeRequest(job, run, *request); err != nil {
		return nil, err
	}
	return request, nil
}

func validatePermissionResumeRequest(
	job automationdomain.ScheduledTask,
	run automationdomain.ScheduledTaskRun,
	request automationdomain.AutomationPermissionRequest,
) error {
	revisionMatches := request.PolicyRevision == job.PermissionPolicy.Revision
	if strings.TrimSpace(request.Decision) == automationdomain.PermissionDecisionAllowTask {
		revisionMatches = request.PolicyRevision+1 == job.PermissionPolicy.Revision
	}
	if strings.TrimSpace(request.JobID) != strings.TrimSpace(job.JobID) ||
		strings.TrimSpace(request.RunID) != strings.TrimSpace(run.RunID) ||
		!revisionMatches ||
		strings.TrimSpace(request.Status) != automationdomain.PermissionRequestStatusApproved ||
		strings.TrimSpace(request.Capability.ToolName) == "" {
		return automationdomain.ErrPermissionRequestStale
	}
	return nil
}

func (s *Service) resumePermissionBlockedRun(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	run automationdomain.ScheduledTaskRun,
	request *automationdomain.AutomationPermissionRequest,
) error {
	jobCtx := contextForJobOwner(ctx, job)
	if run.PermissionPolicyRevision != job.PermissionPolicy.Revision {
		return automationdomain.ErrPermissionRequestStale
	}
	if request == nil {
		return automationdomain.ErrPermissionRunNotResumable
	}
	if err := validatePermissionResumeRequest(job, run, *request); err != nil {
		return err
	}
	// 同一 logical run 的新 attempt 只能在旧 round 和其 observer 都退出后启动。
	// 否则 DM/Room 会把带函数型 PermissionHandler 的请求落入普通持久队列，
	// 重建请求时丢失任务权限上下文，并让旧 observer 覆盖新 attempt 状态。
	if err := s.drainPermissionBlockedAttempt(jobCtx, job, run); err != nil {
		return err
	}
	state := s.ensureJobState(job)
	s.mu.Lock()
	if state.Running && strings.TrimSpace(state.RunningRunID) != strings.TrimSpace(run.RunID) {
		s.mu.Unlock()
		return errors.New("another run is active for this task")
	}
	nextRunAt := cloneTimePointer(state.NextRunAt)
	s.mu.Unlock()
	startedAt := s.nowFn()
	claimed, err := s.repository.ClaimScheduledTaskRuntime(jobCtx, automationstore.JobRuntimeClaimInput{
		JobID:         job.JobID,
		RunID:         run.RunID,
		StartedAt:     startedAt,
		NextRunAt:     nextRunAt,
		OverlapPolicy: automationdomain.OverlapPolicySkip,
		AllowDisabled: true,
	})
	if err != nil {
		return err
	}
	if !claimed {
		return errors.New("task runtime could not be claimed for retry")
	}
	s.mu.Lock()
	state = s.jobStates[job.JobID]
	if state == nil {
		state = &automationexec.JobRuntimeState{Job: job}
		s.jobStates[job.JobID] = state
	}
	state.RunningCount++
	state.Running = true
	state.RunningRunID = run.RunID
	state.RunningStartedAt = cloneTimePointer(&startedAt)
	state.Job.PermissionState = automationdomain.TaskPermissionStateReady
	s.mu.Unlock()

	sessionKey := strings.TrimSpace(run.SessionKey)
	if sessionKey == "" {
		sessionKey, err = automationexec.ResolveSessionKey(job, &run.RunID)
		if err != nil {
			s.pauseJobRuntimeForPermission(job, run.RunID, automationdomain.TaskPermissionStateReadyToRetry, errorPointer(err))
			return err
		}
	}
	if strings.TrimSpace(job.SessionTarget.Kind) == automationdomain.SessionTargetMain {
		return s.queueResumedMainSessionRun(jobCtx, job, run, sessionKey, request)
	}
	roundID := ""
	if automationdomain.NormalizeExecutionKind(job.ExecutionKind) != automationdomain.ExecutionKindScript {
		roundID = s.idFactory("round")
	}
	prepared, err := s.repository.PrepareRunResume(jobCtx, automationstore.RunResumeInput{
		RunID:                    run.RunID,
		OwnerUserID:              job.OwnerUserID,
		RoundID:                  roundID,
		SessionKey:               sessionKey,
		StartedAt:                startedAt,
		PermissionPolicyRevision: job.PermissionPolicy.Revision,
	})
	if err != nil || !prepared {
		if err == nil {
			err = errors.New("run is no longer resumable")
		}
		s.pauseJobRuntimeForPermission(job, run.RunID, automationdomain.TaskPermissionStateReadyToRetry, errorPointer(err))
		return err
	}
	if automationdomain.NormalizeExecutionKind(job.ExecutionKind) == automationdomain.ExecutionKindScript {
		if err = s.repository.MarkRunEffectStarted(jobCtx, job.OwnerUserID, run.RunID); err != nil {
			s.failResumedPermissionRun(job, run.RunID, err)
			return err
		}
		go s.observeScriptJob(job, run.RunID, anyTimePointer(run.ScheduledFor, startedAt))
		return nil
	}

	sink := automationexec.NewExecutionSink("automation:" + run.RunID)
	cleanup := s.bindSink(sessionKey, sink)
	completeAttempt := s.registerPhysicalAttempt(run.RunID, roundID)
	resumeAttempt := newPermissionResumeAttempt(request)
	if err = s.dispatchJobToSession(jobCtx, job, run.RunID, sessionKey, roundID, roomEventObserverForSink(sink), resumeAttempt); err != nil {
		completeAttempt()
		cleanup()
		sink.Close()
		s.failResumedPermissionRun(job, run.RunID, err)
		return err
	}
	go s.observeJobRunWithCompletion(
		job,
		run.RunID,
		roundID,
		sessionKey,
		sink,
		cleanup,
		completeAttempt,
		resumeAttempt,
	)
	return nil
}

func (s *Service) queueResumedMainSessionRun(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	run automationdomain.ScheduledTaskRun,
	sessionKey string,
	request *automationdomain.AutomationPermissionRequest,
) error {
	queued, err := s.repository.QueuePermissionRunForMain(ctx, automationstore.RunResumeInput{
		RunID:                    run.RunID,
		OwnerUserID:              job.OwnerUserID,
		SessionKey:               sessionKey,
		PermissionPolicyRevision: job.PermissionPolicy.Revision,
	})
	if err != nil || !queued {
		if err == nil {
			err = errors.New("run is no longer resumable")
		}
		s.pauseJobRuntimeForPermission(job, run.RunID, automationdomain.TaskPermissionStateReadyToRetry, errorPointer(err))
		return err
	}
	requestID := ""
	if request != nil {
		requestID = request.RequestID
	}
	requestPolicyRevision := 0
	if request != nil {
		requestPolicyRevision = request.PolicyRevision
	}
	eventID, err := s.enqueueMainSessionEvent(
		ctx,
		job,
		run.RunID,
		"permission_resume",
		requestID,
		requestPolicyRevision,
	)
	if err == nil {
		err = s.wakeHeartbeatForSystemEvent(ctx, job.AgentID, automationdomain.WakeModeNow)
	}
	if err == nil {
		return nil
	}
	if eventID != "" {
		_ = s.repository.MarkSystemEventStatus(context.Background(), eventID, "failed")
	}
	_ = s.repository.RestoreRunReadyToRetry(context.Background(), job.OwnerUserID, run.RunID, errorPointer(err))
	s.pauseJobRuntimeForPermission(job, run.RunID, automationdomain.TaskPermissionStateReadyToRetry, errorPointer(err))
	return err
}

func (s *Service) failResumedPermissionRun(job automationdomain.ScheduledTask, runID string, runErr error) {
	finishedAt := s.nowFn()
	message := errorPointer(runErr)
	_, _ = s.repository.MarkRunFinishedIfActive(context.Background(), automationstore.RunFinishInput{
		RunID:        runID,
		Status:       automationdomain.RunStatusFailed,
		FinishedAt:   finishedAt,
		ErrorMessage: message,
	})
	s.finishJobRuntime(job.JobID, &finishedAt, automationdomain.RunStatusFailed, message)
}

func (s *Service) finishDeniedPermissionRun(
	job automationdomain.ScheduledTask,
	runID string,
	finishedAt time.Time,
	message string,
) {
	s.mu.Lock()
	state := s.jobStates[job.JobID]
	if state != nil {
		state.Job.PermissionState = automationdomain.TaskPermissionStateDenied
		state.LastRunAt = cloneTimePointer(&finishedAt)
		state.LastRunStatus = automationdomain.RunStatusFailed
		state.LastError = stringPointer(message)
		state.FailureStreak++
		snapshot := jobRuntimeUpdateFromState(job.JobID, state)
		s.mu.Unlock()
		s.persistJobRuntime(context.Background(), snapshot)
		return
	}
	s.mu.Unlock()
	_ = runID
}

func (s *Service) refreshPermissionDecisionResult(
	ctx context.Context,
	ownerUserID string,
	result *automationdomain.PermissionDecisionResult,
) (*automationdomain.PermissionDecisionResult, error) {
	job, err := s.repository.GetScheduledTask(ctx, ownerUserID, result.Task.JobID)
	if err != nil {
		return nil, err
	}
	if job != nil {
		state := s.ensureJobState(*job)
		result.Task = scheduledTaskWithRuntime(*job, state)
	}
	if result.Run != nil {
		run, loadErr := s.repository.GetRun(ctx, ownerUserID, result.Task.JobID, result.Run.RunID)
		if loadErr != nil && !errors.Is(loadErr, sql.ErrNoRows) {
			return nil, loadErr
		}
		result.Run = run
	}
	return result, nil
}

func anyTimePointer(value *time.Time, fallback time.Time) time.Time {
	if value == nil {
		return fallback
	}
	return value.UTC()
}

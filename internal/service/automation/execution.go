// INPUT: 已验证的 task/configuration/permission 快照、触发类型与可选人工 request identity。
// OUTPUT: commit 后才派发的 main/runtime/script execution，或 exact run 的幂等重放结果。
// POS: Automation 执行启动编排；storage 原子 claim+run 是所有副作用前的唯一 durable acceptance。
package automation

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

func (s *Service) startJobExecution(ctx context.Context, job automationdomain.ScheduledTask, triggerKind string, scheduledFor time.Time) (*automationdomain.ExecutionResult, error) {
	fence := s.taskExecutionFence(job.JobID)
	fence.Lock()
	defer fence.Unlock()
	return s.startJobExecutionControlled(ctx, job, triggerKind, scheduledFor)
}

// startJobExecutionControlled 保证 durable runtime claim、run ledger 和 dispatch
// 注册与 deletion claim 具有同一进程内顺序；调用方必须持有对应 task fence。
func (s *Service) startJobExecutionControlled(ctx context.Context, job automationdomain.ScheduledTask, triggerKind string, scheduledFor time.Time) (*automationdomain.ExecutionResult, error) {
	return s.startJobExecutionControlledAtVersion(ctx, job, triggerKind, scheduledFor, nil)
}

// startJobExecutionControlledAtVersion carries an optional human-confirmed
// configuration snapshot through the final durable runtime claim. The second
// read below protects process-local ordering; the storage CAS protects the same
// boundary when another Nexus instance updates the task concurrently.
func (s *Service) startJobExecutionControlledAtVersion(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	triggerKind string,
	scheduledFor time.Time,
	expectedConfigurationVersion *int64,
) (*automationdomain.ExecutionResult, error) {
	return s.startJobExecutionControlledAtVersionAndRequest(
		ctx, job, triggerKind, scheduledFor, expectedConfigurationVersion, manualRunIdentity{},
	)
}

func (s *Service) startJobExecutionControlledAtVersionAndRequest(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	triggerKind string,
	scheduledFor time.Time,
	expectedConfigurationVersion *int64,
	request manualRunIdentity,
) (*automationdomain.ExecutionResult, error) {
	ctx = contextForJobOwner(ctx, job)
	current, err := s.repository.GetScheduledTask(ctx, strings.TrimSpace(job.OwnerUserID), strings.TrimSpace(job.JobID))
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, automationdomain.ErrJobNotFound
	}
	if strings.TrimSpace(current.DeletionState) != "" {
		return nil, automationdomain.ErrTaskDeleting
	}
	if expectedConfigurationVersion != nil &&
		(*expectedConfigurationVersion < 1 || current.ConfigurationVersion != *expectedConfigurationVersion) {
		return nil, automationdomain.ErrConfigurationVersionConflict
	}
	job = *current
	job = automationdomain.NormalizeScheduledTaskSessionBinding(job)
	if job.SessionBindingState == automationdomain.TaskSessionBindingStateRebindRequired {
		return nil, automationdomain.ErrTaskSessionRebindRequired
	}
	job, err = s.ensureTaskPermissionPolicy(ctx, job)
	if err != nil {
		return nil, err
	}
	claimExpectation := newRuntimeClaimExpectation(job)
	if state := strings.TrimSpace(job.PermissionState); state != "" && state != automationdomain.TaskPermissionStateReady {
		if triggerKind == automationdomain.TriggerKindManual && state == automationdomain.TaskPermissionStateDenied {
			// Permission reset and runtime ownership are one conditional database
			// update. A stale manual action can neither clear a newer request nor
			// start with a permission snapshot it did not inspect.
			claimExpectation.resetDeniedPermission = true
			job.PermissionState = automationdomain.TaskPermissionStateReady
			job.PendingPermissionRequestID = ""
		} else {
			return nil, fmt.Errorf("scheduled task permission state is %s", state)
		}
	}
	starter := jobExecutionStarter{
		service:      s,
		ctx:          ctx,
		job:          job,
		claim:        claimExpectation,
		triggerKind:  triggerKind,
		scheduledFor: scheduledFor,
		request:      request,
		logger: s.loggerFor(ctx).With(
			"job_id", job.JobID,
			"agent_id", job.AgentID,
			"trigger_kind", triggerKind,
		),
	}
	return starter.start()
}

type runtimeClaimExpectation struct {
	configurationVersion  int64
	permissionRevision    int
	permissionState       string
	permissionRequestID   string
	resetDeniedPermission bool
}

type manualRunIdentity struct {
	RequestID    string
	IntentDigest string
}

func newRuntimeClaimExpectation(job automationdomain.ScheduledTask) runtimeClaimExpectation {
	return runtimeClaimExpectation{
		configurationVersion: job.ConfigurationVersion,
		permissionRevision:   job.PermissionPolicy.Revision,
		permissionState:      strings.TrimSpace(job.PermissionState),
		permissionRequestID:  strings.TrimSpace(job.PendingPermissionRequestID),
	}
}

func (e *runtimeClaimExpectation) apply(input *automationstore.JobRuntimeClaimInput) {
	input.ExpectedConfigurationVersion = &e.configurationVersion
	input.ExpectedPermissionRevision = &e.permissionRevision
	input.ExpectedPermissionState = &e.permissionState
	input.ExpectedPermissionRequestID = &e.permissionRequestID
	input.ResetDeniedPermission = e.resetDeniedPermission
}

func (s *Service) taskExecutionFence(jobID string) *sync.Mutex {
	const offset32 = uint32(2166136261)
	const prime32 = uint32(16777619)
	hash := offset32
	for _, value := range []byte(strings.TrimSpace(jobID)) {
		hash ^= uint32(value)
		hash *= prime32
	}
	return &s.taskExecutionFences[int(hash%uint32(len(s.taskExecutionFences)))]
}

type jobExecutionStarter struct {
	service      *Service
	ctx          context.Context
	job          automationdomain.ScheduledTask
	claim        runtimeClaimExpectation
	triggerKind  string
	scheduledFor time.Time
	request      manualRunIdentity
	logger       *slog.Logger
	runID        string
	sessionKey   string
	roundID      string
	startedAt    time.Time
	nextRunAt    *time.Time
	overlap      string
}

func (s *jobExecutionStarter) start() (*automationdomain.ExecutionResult, error) {
	if automationdomain.NormalizeExecutionKind(s.job.ExecutionKind) == automationdomain.ExecutionKindScript {
		return s.service.startScriptJobExecution(s.ctx, s.job, s.triggerKind, s.scheduledFor, s.claim, s.request)
	}
	if err := s.service.ensureDirectTargetSupported(s.job.SessionTarget); err != nil {
		s.failRuntime(err, s.service.nowFn())
		s.logger.Error("自动化任务目标校验失败", "err", err)
		return nil, err
	}
	if strings.TrimSpace(s.job.SessionTarget.Kind) == automationdomain.SessionTargetMain {
		return s.startMainSession()
	}
	return s.startRuntimeSession()
}

func (s *jobExecutionStarter) startMainSession() (*automationdomain.ExecutionResult, error) {
	if err := s.prepareMainIdentity(); err != nil {
		return nil, err
	}
	result, handled, err := s.claimRuntime(automationdomain.RunStatusQueuedToMain)
	if handled || err != nil {
		return result, err
	}
	eventID, err := s.service.enqueueMainSessionEvent(s.ctx, s.job, s.runID, s.triggerKind, "", 0)
	if err != nil {
		s.failPendingRun(err)
		return nil, err
	}
	mode := normalizedWakeMode(s.job.SessionTarget.WakeMode)
	if err = s.service.wakeHeartbeatForSystemEvent(s.ctx, s.job.AgentID, mode); err != nil {
		_ = s.service.repository.MarkSystemEventStatus(context.Background(), eventID, "failed")
		s.failPendingRun(err)
		s.logger.Error("自动化任务唤醒主会话 heartbeat 失败", "err", err)
		return nil, err
	}
	s.logger.Info("自动化任务已排入主会话",
		"run_id", s.runID,
		"session_key", s.sessionKey,
		"wake_mode", mode,
	)
	return s.queuedMainResult(), nil
}

func (s *jobExecutionStarter) prepareMainIdentity() error {
	s.runID = s.service.idFactory("run")
	sessionKey, err := automationexec.ResolveSessionKey(s.job, nil)
	if err != nil {
		s.failRuntime(err, s.service.nowFn())
		s.logger.Error("自动化任务解析主会话键失败", "err", err)
		return err
	}
	s.sessionKey = sessionKey
	return nil
}

func normalizedWakeMode(mode string) string {
	if strings.TrimSpace(mode) == "" {
		return automationdomain.WakeModeNextHeartbeat
	}
	return mode
}

func (s *jobExecutionStarter) queuedMainResult() *automationdomain.ExecutionResult {
	return &automationdomain.ExecutionResult{
		JobID:        s.job.JobID,
		RunID:        &s.runID,
		Status:       automationdomain.RunStatusQueuedToMain,
		SessionKey:   s.sessionKey,
		ScheduledFor: cloneTimePointer(&s.scheduledFor),
	}
}

func (s *jobExecutionStarter) startRuntimeSession() (*automationdomain.ExecutionResult, error) {
	if err := s.prepareRuntimeIdentity(); err != nil {
		return nil, err
	}
	result, handled, err := s.claimRuntime(automationdomain.RunStatusRunning)
	if handled || err != nil {
		return result, err
	}
	if err = s.dispatchRuntime(); err != nil {
		return nil, err
	}
	return s.runningResult(), nil
}

func (s *jobExecutionStarter) prepareRuntimeIdentity() error {
	s.runID = s.service.idFactory("run")
	sessionKey, err := automationexec.ResolveSessionKey(s.job, &s.runID)
	if err != nil {
		s.failRuntime(err, s.service.nowFn())
		s.logger.Error("自动化任务解析执行会话键失败", "run_id", s.runID, "err", err)
		return err
	}
	s.sessionKey = sessionKey
	s.roundID = s.service.idFactory("round")
	return nil
}

func (s *jobExecutionStarter) claimRuntime(initialStatus string) (*automationdomain.ExecutionResult, bool, error) {
	overlapping := s.buildRuntimeClaimPlan()
	if overlapping && strings.TrimSpace(s.request.RequestID) == "" {
		s.logger.Warn("自动化任务已在运行中")
		result, err := s.service.recordSkippedOverlap(
			s.ctx, s.job, s.triggerKind, s.scheduledFor, true,
		)
		return result, true, err
	}
	s.startedAt = s.service.nowFn()
	claimInput := automationstore.JobRuntimeClaimInput{
		OwnerUserID:   s.job.OwnerUserID,
		JobID:         s.job.JobID,
		RunID:         s.runID,
		StartedAt:     s.startedAt,
		NextRunAt:     s.nextRunAt,
		OverlapPolicy: s.overlap,
		AllowDisabled: s.triggerKind == automationdomain.TriggerKindManual,
	}
	s.claim.apply(&claimInput)
	runInput := automationstore.RunPendingInput{
		RunID:                    s.runID,
		JobID:                    s.job.JobID,
		OwnerUserID:              s.job.OwnerUserID,
		ScheduledFor:             &s.scheduledFor,
		TriggerKind:              s.triggerKind,
		SessionKey:               s.sessionKey,
		RoundID:                  s.roundID,
		DeliveryMode:             strings.TrimSpace(s.job.Delivery.Mode),
		DeliveryTo:               deliveryTargetSummary(s.job.Delivery),
		DeliveryTarget:           cloneDeliveryTargetPointer(s.job.Delivery),
		Status:                   initialStatus,
		PermissionPolicyRevision: s.job.PermissionPolicy.Revision,
		ClientRequestID:          strings.TrimSpace(s.request.RequestID),
		IntentDigest:             strings.TrimSpace(s.request.IntentDigest),
	}
	if initialStatus == automationdomain.RunStatusRunning {
		runInput.StartedAt = cloneTimePointer(&s.startedAt)
		runInput.Attempts = 1
	}
	var overlapTerminalRun *automationstore.RunPendingInput
	if s.triggerKind == automationdomain.TriggerKindManual &&
		strings.TrimSpace(s.request.RequestID) != "" &&
		s.overlap == automationdomain.OverlapPolicySkip {
		terminal := skippedOverlapRunInput(
			s.job, s.triggerKind, s.scheduledFor, s.runID, s.startedAt, s.request,
		)
		overlapTerminalRun = &terminal
	}
	claimResult, err := s.service.repository.ClaimScheduledTaskRun(
		s.ctx,
		automationstore.InitialRunClaimInput{
			Runtime: claimInput, Run: runInput, OverlapTerminalRun: overlapTerminalRun,
		},
	)
	if err != nil {
		s.logger.Error("自动化任务领取执行权失败", "run_id", s.runID, "err", err)
		return nil, false, err
	}
	if claimResult.Replayed {
		run, runErr := s.service.repository.GetRun(s.ctx, s.job.OwnerUserID, s.job.JobID, claimResult.RunID)
		if runErr != nil {
			return nil, false, runErr
		}
		return executionResultFromRun(*run, true), true, nil
	}
	if claimResult.Terminal {
		run, runErr := s.service.repository.GetRun(s.ctx, s.job.OwnerUserID, s.job.JobID, claimResult.RunID)
		if runErr != nil {
			return nil, false, runErr
		}
		s.service.refreshRuntimeProjectionBestEffort(s.ctx, s.job.OwnerUserID, s.job.JobID)
		return executionResultFromRun(*run, false), true, nil
	}
	if !claimResult.Claimed {
		s.logger.Warn("自动化任务执行权已被其他调度器领取", "run_id", s.runID)
		result, resultErr := s.service.resultForExternallyClaimedJob(s.ctx, s.job, s.scheduledFor)
		return result, true, resultErr
	}
	s.registerRunningState(overlapping && strings.TrimSpace(s.request.RequestID) != "")
	if s.claim.resetDeniedPermission {
		s.service.setJobPermissionState(s.job.JobID, automationdomain.TaskPermissionStateReady, "")
	}
	return nil, false, nil
}

func (s *Service) refreshRuntimeProjectionBestEffort(ctx context.Context, ownerUserID string, jobID string) {
	current, err := s.repository.GetScheduledTask(ctx, strings.TrimSpace(ownerUserID), strings.TrimSpace(jobID))
	if err != nil || current == nil {
		s.loggerFor(ctx).Warn("刷新自动化任务运行镜像失败", "job_id", jobID, "err", err)
		return
	}
	s.replacePersistedJobRuntimeState(*current)
}

func (s *jobExecutionStarter) buildRuntimeClaimPlan() bool {
	state := s.service.ensureJobState(s.job)
	s.service.mu.Lock()
	defer s.service.mu.Unlock()
	s.overlap = automationdomain.NormalizeOverlapPolicy(s.job.OverlapPolicy)
	if state.Running && s.overlap == automationdomain.OverlapPolicySkip {
		return true
	}
	s.nextRunAt = cloneTimePointer(state.NextRunAt)
	if isScheduledTrigger(s.triggerKind) {
		s.nextRunAt = s.service.nextRunAfterScheduledTrigger(s.job, s.triggerKind, s.scheduledFor)
	}
	return false
}

func isScheduledTrigger(triggerKind string) bool {
	return triggerKind == automationdomain.TriggerKindScheduled || triggerKind == automationdomain.TriggerKindMisfire
}

func (s *jobExecutionStarter) registerRunningState(replaceStaleOverlap bool) {
	s.service.mu.Lock()
	defer s.service.mu.Unlock()
	state := s.service.jobStates[s.job.JobID]
	if state == nil {
		state = &automationexec.JobRuntimeState{Job: s.job}
		s.service.jobStates[s.job.JobID] = state
	}
	if replaceStaleOverlap {
		// The exact database claim succeeded even though the local projection said
		// overlap=skip was occupied, so that projection is stale by definition.
		state.RunningCount = 1
	} else {
		state.RunningCount++
	}
	state.Running = true
	state.RunningRunID = s.runID
	state.RunningStartedAt = cloneTimePointer(&s.startedAt)
	state.NextRunAt = cloneTimePointer(s.nextRunAt)
}

func (s *jobExecutionStarter) dispatchRuntime() error {
	s.logger.Info("开始执行自动化任务",
		"run_id", s.runID,
		"round_id", s.roundID,
		"session_key", s.sessionKey,
	)
	sink := automationexec.NewExecutionSink("automation:" + s.runID)
	cleanup := s.service.bindSink(s.sessionKey, sink)
	completeAttempt := s.service.registerPhysicalAttempt(s.runID, s.roundID)
	current, err := s.service.repository.GetScheduledTask(s.ctx, s.job.OwnerUserID, s.job.JobID)
	if err != nil || current == nil || strings.TrimSpace(current.DeletionState) != "" {
		completeAttempt()
		cleanup()
		sink.Close()
		if err == nil {
			err = automationdomain.ErrTaskDeleting
		}
		s.failPendingRun(err)
		return err
	}
	err = s.service.dispatchJobToSession(s.ctx, s.job, s.runID, s.sessionKey, s.roundID, roomEventObserverForSink(sink), nil)
	if err != nil {
		completeAttempt()
		cleanup()
		sink.Close()
		s.failPendingRun(err)
		s.logger.Error("自动化任务下发失败",
			"run_id", s.runID,
			"round_id", s.roundID,
			"session_key", s.sessionKey,
			"err", err,
		)
		return err
	}
	go s.service.observeJobRunWithCompletion(
		s.job,
		s.runID,
		s.roundID,
		s.sessionKey,
		sink,
		cleanup,
		completeAttempt,
		nil,
	)
	return nil
}

func executionResultFromRun(run automationdomain.ScheduledTaskRun, replayed bool) *automationdomain.ExecutionResult {
	runID := strings.TrimSpace(run.RunID)
	result := &automationdomain.ExecutionResult{
		JobID:        strings.TrimSpace(run.JobID),
		RunID:        &runID,
		Status:       strings.TrimSpace(run.Status),
		SessionKey:   strings.TrimSpace(run.SessionKey),
		ScheduledFor: cloneTimePointer(run.ScheduledFor),
		SessionID:    cloneStringPointer(run.SessionID),
		MessageCount: run.MessageCount,
		ErrorMessage: cloneStringPointer(run.ErrorMessage),
		Replayed:     replayed,
	}
	if roundID := strings.TrimSpace(run.RoundID); roundID != "" {
		result.RoundID = &roundID
	}
	return result
}

func (s *jobExecutionStarter) runningResult() *automationdomain.ExecutionResult {
	return &automationdomain.ExecutionResult{
		JobID:        s.job.JobID,
		RunID:        &s.runID,
		Status:       automationdomain.RunStatusRunning,
		SessionKey:   s.sessionKey,
		ScheduledFor: cloneTimePointer(&s.scheduledFor),
		RoundID:      &s.roundID,
		MessageCount: 0,
	}
}

func (s *jobExecutionStarter) failPendingRun(err error) {
	if finishErr := s.service.commitFailedRunTerminal(
		backgroundContextForJobOwner(s.job), s.job, s.runID, err,
	); finishErr != nil {
		s.logger.Warn("自动化任务失败态原子提交失败", "run_id", s.runID, "err", finishErr)
	}
}

func (s *jobExecutionStarter) failRuntime(err error, finishedAt time.Time) {
	s.service.finishJobRuntime(s.job.JobID, &finishedAt, automationdomain.RunStatusFailed, errorPointer(err))
}

// INPUT: 手动运行 request identity、配置版本、已见投递次数、运行恢复请求及当前任务/run 状态。
// OUTPUT: 可重放 exact ExecutionResult，以及受 owner、script capability 与 exact delivery ledger 约束的投递或恢复结果。
// POS: scheduled task 主动执行和修复的 service 最终边界；request replay 不再次 dispatch。
package automation

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

// RunTaskNow 立即触发一次任务。
func (s *Service) RunTaskNow(ctx context.Context, jobID string) (*automationdomain.ExecutionResult, error) {
	return s.runTaskNow(ctx, jobID, nil, manualRunIdentity{})
}

// RunTaskNowAtVersion 只运行调用方在 plan 阶段核对过的任务版本。
func (s *Service) RunTaskNowAtVersion(
	ctx context.Context,
	jobID string,
	expectedVersion int64,
) (*automationdomain.ExecutionResult, error) {
	return s.runTaskNow(ctx, jobID, &expectedVersion, manualRunIdentity{})
}

// RunTaskNowWithRequest 将浏览器/桌面端人工启动绑定到 durable request identity。
func (s *Service) RunTaskNowWithRequest(
	ctx context.Context,
	jobID string,
	requestID string,
) (*automationdomain.ExecutionResult, error) {
	return s.runTaskNowWithClientRequest(ctx, jobID, nil, requestID)
}

// RunTaskNowAtVersionWithRequest 同时 fence 配置版本和人工启动 request identity。
func (s *Service) RunTaskNowAtVersionWithRequest(
	ctx context.Context,
	jobID string,
	expectedVersion int64,
	requestID string,
) (*automationdomain.ExecutionResult, error) {
	return s.runTaskNowWithClientRequest(ctx, jobID, &expectedVersion, requestID)
}

func (s *Service) runTaskNowWithClientRequest(
	ctx context.Context,
	jobID string,
	expectedVersion *int64,
	requestID string,
) (*automationdomain.ExecutionResult, error) {
	requestID = strings.TrimSpace(requestID)
	if !runtimeAutomationRequestIDPattern.MatchString(requestID) {
		return nil, errors.New("request_id 必须为 8-128 位字母、数字、点、下划线、冒号或连字符")
	}
	return s.runTaskNow(ctx, jobID, expectedVersion, manualRunIdentity{
		RequestID:    requestID,
		IntentDigest: manualRunIntentDigest(jobID, expectedVersion),
	})
}

func (s *Service) runTaskNow(
	ctx context.Context,
	jobID string,
	expectedVersion *int64,
	request manualRunIdentity,
) (*automationdomain.ExecutionResult, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	fence := s.taskExecutionFence(jobID)
	fence.Lock()
	defer fence.Unlock()

	ownerUserID, _ := scopedOwnerUserID(ctx)
	if strings.TrimSpace(request.RequestID) != "" {
		run, found, replayErr := s.repository.GetRunByClientRequest(
			ctx,
			ownerUserID,
			strings.TrimSpace(jobID),
			request.RequestID,
			request.IntentDigest,
		)
		if replayErr != nil {
			return nil, replayErr
		}
		if found {
			return executionResultFromRun(*run, true), nil
		}
	}
	job, err := s.repository.GetScheduledTask(ctx, ownerUserID, strings.TrimSpace(jobID))
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, automationdomain.ErrJobNotFound
	}
	if strings.TrimSpace(job.DeletionState) != "" {
		return nil, automationdomain.ErrTaskDeleting
	}
	if expectedVersion != nil &&
		(*expectedVersion < 1 || job.ConfigurationVersion != *expectedVersion) {
		return nil, automationdomain.ErrConfigurationVersionConflict
	}
	if err = rejectAgentScriptControl(ctx, *job); err != nil {
		return nil, err
	}
	s.loggerFor(ctx).Info("手动触发自动化任务",
		"job_id", job.JobID,
		"agent_id", job.AgentID,
	)
	result, err := s.startJobExecutionControlledAtVersionAndRequest(
		ctx,
		*job,
		automationdomain.TriggerKindManual,
		s.nowFn(),
		expectedVersion,
		request,
	)
	if err == nil && (result == nil || !result.Replayed) {
		runID := ""
		if result != nil && result.RunID != nil {
			runID = *result.RunID
		}
		s.recordTaskEvent(ctx, automationdomain.TaskEventActionRunNow, *job, runID, map[string]any{"status": anyExecutionStatus(result)})
	}
	return result, err
}

func manualRunIntentDigest(jobID string, expectedVersion *int64) string {
	version := int64(0)
	if expectedVersion != nil {
		version = *expectedVersion
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("run\x00%s\x00%d", strings.TrimSpace(jobID), version)))
	return hex.EncodeToString(sum[:])
}

// ListTaskRuns 返回任务运行历史。
func (s *Service) ListTaskRuns(ctx context.Context, jobID string) ([]automationdomain.ScheduledTaskRun, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	normalizedJobID := strings.TrimSpace(jobID)
	job, err := s.repository.GetScheduledTask(ctx, ownerUserID, normalizedJobID)
	if err != nil {
		return nil, err
	}
	runs, err := s.repository.ListRunsByJob(ctx, ownerUserID, normalizedJobID)
	if err != nil {
		return nil, err
	}
	if job != nil {
		return runs, nil
	}
	events, err := s.repository.ListTaskEventsByJob(ctx, ownerUserID, normalizedJobID, 1)
	if err != nil {
		return nil, err
	}
	if len(runs) == 0 && len(events) == 0 {
		return nil, automationdomain.ErrJobNotFound
	}
	return runs, nil
}

// RetryRunDelivery 只重试某次 run 的结果投递，不重新执行任务本身。
func (s *Service) RetryRunDelivery(ctx context.Context, jobID string, runID string) (*automationdomain.ScheduledTaskRun, error) {
	return s.retryRunDeliveryAtVersion(ctx, jobID, runID, nil, nil)
}

// RetryRunDeliveryAtVersion 只按 plan 阶段核对过的任务配置重投递一次结果。
func (s *Service) RetryRunDeliveryAtVersion(
	ctx context.Context,
	jobID string,
	runID string,
	expectedVersion int64,
) (*automationdomain.ScheduledTaskRun, error) {
	return s.retryRunDeliveryAtVersion(ctx, jobID, runID, &expectedVersion, nil)
}

// RetryRunDeliveryAtVersionAndAttempts additionally fences the exact delivery
// ledger snapshot shown to the user. A stale page therefore cannot produce a
// second external delivery after another actor has already consumed one attempt.
func (s *Service) RetryRunDeliveryAtVersionAndAttempts(
	ctx context.Context,
	jobID string,
	runID string,
	expectedVersion int64,
	expectedDeliveryAttempts int,
) (*automationdomain.ScheduledTaskRun, error) {
	return s.retryRunDeliveryAtVersion(
		ctx,
		jobID,
		runID,
		&expectedVersion,
		&expectedDeliveryAttempts,
	)
}

func (s *Service) retryRunDeliveryAtVersion(
	ctx context.Context,
	jobID string,
	runID string,
	expectedVersion *int64,
	expectedDeliveryAttempts *int,
) (*automationdomain.ScheduledTaskRun, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	job, run, err := s.loadDeliveryRetry(ctx, ownerUserID, jobID, runID)
	if err != nil {
		return nil, err
	}
	if expectedVersion != nil &&
		(*expectedVersion < 1 || job.ConfigurationVersion != *expectedVersion) {
		return nil, automationdomain.ErrConfigurationVersionConflict
	}
	if expectedDeliveryAttempts != nil &&
		(*expectedDeliveryAttempts < 0 || run.DeliveryAttempts != *expectedDeliveryAttempts) {
		return nil, automationdomain.ErrDeliveryRetryConflict
	}
	return s.retryLoadedRunDelivery(ctx, ownerUserID, *job, *run, true)
}

func (s *Service) retryRunDelivery(ctx context.Context, jobID string, runID string, recordEvent bool) (*automationdomain.ScheduledTaskRun, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	job, run, err := s.loadDeliveryRetry(ctx, ownerUserID, jobID, runID)
	if err != nil {
		return nil, err
	}
	return s.retryLoadedRunDelivery(ctx, ownerUserID, *job, *run, recordEvent)
}

// retryLoadedRunDelivery 使用调用方已经核对过的任务快照选择投递目标。
// 动态 owner、Room 成员和连接授权仍会在 deliver 阶段重新验证，但并发配置
// 更新不能把一次带 configuration_version 的人工恢复静默改道。
func (s *Service) retryLoadedRunDelivery(
	ctx context.Context,
	ownerUserID string,
	job automationdomain.ScheduledTask,
	run automationdomain.ScheduledTaskRun,
	recordEvent bool,
) (*automationdomain.ScheduledTaskRun, error) {
	if err := validateDeliveryRetry(run); err != nil {
		return nil, err
	}
	return s.retryClaimedRunDelivery(ctx, ownerUserID, job, run, recordEvent, false)
}

// RetryUnverifiedRunDeliveryAtVersion 只在用户已经核对接收端后，按 exact
// configuration version 与 delivery attempts 从 retrying 状态显式领取新一次外投。
// 普通 retry API 永远不会进入这条恢复路径。
func (s *Service) RetryUnverifiedRunDeliveryAtVersion(
	ctx context.Context,
	jobID string,
	runID string,
	expectedVersion int64,
	expectedDeliveryAttempts int,
) (*automationdomain.ScheduledTaskRun, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	job, run, err := s.loadDeliveryRetry(ctx, ownerUserID, jobID, runID)
	if err != nil {
		return nil, err
	}
	if expectedVersion < 1 || job.ConfigurationVersion != expectedVersion {
		return nil, automationdomain.ErrConfigurationVersionConflict
	}
	if run.DeliveryAttempts != expectedDeliveryAttempts ||
		strings.TrimSpace(run.DeliveryStatus) != automationdomain.DeliveryStatusRetrying {
		return nil, automationdomain.ErrDeliveryRetryConflict
	}
	return s.retryClaimedRunDelivery(ctx, ownerUserID, *job, *run, true, true)
}

func (s *Service) retryClaimedRunDelivery(
	ctx context.Context,
	ownerUserID string,
	job automationdomain.ScheduledTask,
	run automationdomain.ScheduledTaskRun,
	recordEvent bool,
	confirmUnverifiedAttempt bool,
) (*automationdomain.ScheduledTaskRun, error) {
	// 外投可能受第三方延迟影响；只与同一任务的启动/删除串行，不能占用
	// 全局配置锁并阻塞其他任务。跨实例唯一性仍由下方 durable claim 保证。
	fence := s.taskExecutionFence(job.JobID)
	fence.Lock()
	defer fence.Unlock()

	claimOwnerUserID := strings.TrimSpace(ownerUserID)
	if claimOwnerUserID == "" {
		claimOwnerUserID = strings.TrimSpace(job.OwnerUserID)
	}
	attemptID := automationexec.NewID("delivery_attempt")
	if err := s.repository.ClaimRunDeliveryAttempt(ctx, automationstore.RunDeliveryAttemptClaimInput{
		OwnerUserID:                  claimOwnerUserID,
		JobID:                        job.JobID,
		RunID:                        run.RunID,
		ExpectedDeliveryAttempts:     run.DeliveryAttempts,
		ExpectedConfigurationVersion: &job.ConfigurationVersion,
		ExpectedStatus:               automationdomain.DeliveryStatusFailed,
		AttemptID:                    attemptID,
		RequireEnabled:               !recordEvent,
		RequireNoDeadLetter:          !recordEvent,
		ConfirmUnverifiedAttempt:     confirmUnverifiedAttempt,
	}); err != nil {
		return nil, err
	}
	s.invalidateDeliveryRetryDeadline()
	update, _, outcomeUnknown := s.buildDeliveryRetryCompletion(ctx, job, run, attemptID)
	if outcomeUnknown {
		if err := s.repository.MarkRunDeliveryAttemptUnconfirmed(
			ctx, claimOwnerUserID, job.JobID, run.RunID, attemptID,
			update.DeliveryMode, update.DeliveryTo, update.DeliveryError,
		); err != nil {
			return nil, errors.Join(automationdomain.ErrDeliveryRetryCompletionUnconfirmed, err)
		}
		s.refreshTaskRuntimeProjection(ctx, job)
		updated, loadErr := s.loadRetriedRun(ctx, ownerUserID, job.JobID, run.RunID)
		if loadErr != nil {
			return nil, loadErr
		}
		return updated, automationdomain.ErrDeliveryRetryCompletionUnconfirmed
	}
	if err := s.repository.CompleteRunDeliveryAttempt(ctx, update); err != nil {
		return nil, err
	}
	s.refreshTaskRuntimeProjection(ctx, job)

	updated, err := s.loadRetriedRun(ctx, ownerUserID, job.JobID, run.RunID)
	if err != nil {
		return nil, err
	}
	if recordEvent && updated != nil {
		s.recordTaskEvent(ctx, automationdomain.TaskEventActionRetryDelivery, job, run.RunID, deliveryRetryTaskEventDetail(*updated))
	}
	if updated != nil && run.DeliveryDeadLetterAt == nil && updated.DeliveryDeadLetterAt != nil {
		s.notifyAutomationDeliveryDeadLetter(ctx, job, *updated)
	}
	return updated, nil
}

func (s *Service) loadDeliveryRetry(ctx context.Context, ownerUserID string, jobID string, runID string) (*automationdomain.ScheduledTask, *automationdomain.ScheduledTaskRun, error) {
	job, err := s.repository.GetScheduledTask(ctx, ownerUserID, strings.TrimSpace(jobID))
	if err != nil {
		return nil, nil, err
	}
	if job == nil {
		return nil, nil, automationdomain.ErrJobNotFound
	}
	if err = rejectAgentScriptControl(ctx, *job); err != nil {
		return nil, nil, err
	}
	if job.SessionBindingState == automationdomain.TaskSessionBindingStateRebindRequired {
		return nil, nil, automationdomain.ErrTaskSessionRebindRequired
	}
	run, err := s.repository.GetRun(ctx, ownerUserID, job.JobID, strings.TrimSpace(runID))
	if errors.Is(err, sql.ErrNoRows) || (err == nil && run == nil) {
		return nil, nil, automationdomain.ErrRunNotFound
	}
	return job, run, err
}

func validateDeliveryRetry(run automationdomain.ScheduledTaskRun) error {
	runStatus := strings.TrimSpace(run.Status)
	if runStatus == automationdomain.RunStatusPending ||
		runStatus == automationdomain.RunStatusRunning ||
		runStatus == automationdomain.RunStatusQueuedToMain {
		return errors.New("run is not finished")
	}
	deliveryStatus := strings.TrimSpace(run.DeliveryStatus)
	if deliveryStatus == automationdomain.DeliveryStatusRetrying {
		return automationdomain.ErrDeliveryRetryUnverified
	}
	if deliveryStatus != automationdomain.DeliveryStatusFailed {
		return fmt.Errorf("run delivery_status must be failed before retrying delivery, got %q", deliveryStatus)
	}
	return nil
}

func (s *Service) buildDeliveryRetryCompletion(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	run automationdomain.ScheduledTaskRun,
	attemptID string,
) (automationstore.RunDeliveryAttemptCompletionInput, string, bool) {
	observation := automationexec.ExecutionObservation{
		RunID:         run.RunID,
		RoundID:       run.RoundID,
		Status:        automationdomain.RunStatusSucceeded,
		SessionID:     run.SessionID,
		MessageCount:  run.MessageCount,
		ResultText:    anyStringPointer(run.ResultText),
		AssistantText: anyStringPointer(run.AssistantText),
	}
	// 一次执行的首投递使用 run 启动时快照；进入 failed 后，用户对任务目标的
	// 显式修正就是恢复动作，人工和到期重试都应使用当前目标，避免坏路由被永久冻结。
	runDelivery := job.Delivery.Normalized()
	deliveryResult := s.deliverJobObservationToTarget(
		contextForJobOwner(ctx, job),
		job,
		runDelivery,
		run.SessionKey,
		observation,
	)
	deliveryStatus := deliveryResult.Status
	deliveryError := deliveryResult.Error
	deliveryTo := deliveryResult.deliveryTo(runDelivery)
	now := s.nowFn()
	deliveredAt := deliveredAtForStatus(deliveryStatus, now)
	attemptsAfter := run.DeliveryAttempts + 1
	nextDeliveryAttemptAt, deliveryDeadLetterAt := deliveryRetrySchedule(deliveryStatus, attemptsAfter, now)
	return automationstore.RunDeliveryAttemptCompletionInput{
		OwnerUserID:           job.OwnerUserID,
		JobID:                 job.JobID,
		RunID:                 run.RunID,
		AttemptID:             attemptID,
		DeliveryMode:          strings.TrimSpace(runDelivery.Mode),
		DeliveryTo:            deliveryTo,
		DeliveryStatus:        deliveryStatus,
		DeliveryError:         deliveryError,
		DeliveredAt:           deliveredAt,
		DeliveryNextAttemptAt: nextDeliveryAttemptAt,
		DeliveryDeadLetterAt:  deliveryDeadLetterAt,
	}, deliveryStatus, deliveryResult.OutcomeUnknown
}

func (s *Service) loadRetriedRun(ctx context.Context, ownerUserID string, jobID string, runID string) (*automationdomain.ScheduledTaskRun, error) {
	updated, err := s.repository.GetRun(ctx, ownerUserID, jobID, runID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, automationdomain.ErrRunNotFound
	}
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// RecoverTaskRunningRun 手动释放任务当前运行占用，并把未完成 run 标记为取消。
func (s *Service) RecoverTaskRunningRun(ctx context.Context, jobID string, runID string) (*automationdomain.ScheduledTask, error) {
	fence := s.taskExecutionFence(jobID)
	fence.Lock()
	defer fence.Unlock()

	current, err := s.GetTask(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, automationdomain.ErrJobNotFound
	}
	if err = rejectAgentScriptControl(ctx, *current); err != nil {
		return nil, err
	}
	currentRunID := strings.TrimSpace(current.RunningRunID)
	if currentRunID == "" {
		return current, nil
	}
	expectedRunID := strings.TrimSpace(runID)
	if expectedRunID != "" && expectedRunID != currentRunID {
		return nil, errors.New("运行记录不一致，请刷新任务后重试")
	}
	message := "用户手动释放运行占用，已将未完成 run 标记为 cancelled"
	if err = s.interruptActiveRunExecution(ctx, *current, currentRunID, message); err != nil {
		return nil, err
	}
	recovered, err := s.recoverJobRuntimeAsCancelled(ctx, *current, message)
	if err != nil {
		return nil, err
	}
	state := s.replacePersistedJobRuntimeState(recovered)
	result := s.scheduledTaskRuntimeSnapshot(recovered, state)
	s.recordTaskEvent(ctx, automationdomain.TaskEventActionRecover, result, currentRunID, map[string]any{"recovered_run_id": currentRunID})
	return &result, nil
}

func anyExecutionStatus(result *automationdomain.ExecutionResult) string {
	if result == nil {
		return ""
	}
	return strings.TrimSpace(result.Status)
}

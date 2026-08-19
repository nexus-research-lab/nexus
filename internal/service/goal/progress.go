// INPUT: runtime round 的 durable receipt 身份、Agent 审计身份、进展/失败、Room collaboration handback 与 objective revision fence。
// OUTPUT: 以 receipt round 结算 continuation plan，并以独立 audit round 写入 CAS 重试后的 Goal 进展状态和审计事件。
// POS: Goal round 结果回写与 continuation receipt/audit 身份分离的唯一入口。
package goal

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	goalCompletionCommandRetryMetadataKey = "completion_command_retry_count"
	goalCompletionCommandMaxRetries       = 1
	// One empty round is recoverable: the next hidden turn receives an explicit
	// action boundary. A second consecutive empty round proves a real loop and
	// suppresses automatic continuation until explicit activity or resume.
	goalContinuationSuppressionThreshold = protocol.GoalContinuationSuppressionThreshold
)

// ContinuationRuntimeIdentity separates the durable launch receipt from the
// runtime round that produced the observable result. DM callers normally use
// the same value for both fields; Room callers keep the outer/root continuation
// round in ReceiptRoundID and the slot AgentRoundID in AuditRoundID.
type ContinuationRuntimeIdentity struct {
	ReceiptRoundID string
	AuditRoundID   string
}

func (identity ContinuationRuntimeIdentity) normalized() ContinuationRuntimeIdentity {
	identity.ReceiptRoundID = strings.TrimSpace(identity.ReceiptRoundID)
	identity.AuditRoundID = strings.TrimSpace(identity.AuditRoundID)
	return identity
}

func continuationRuntimeIdentity(roundID string) ContinuationRuntimeIdentity {
	roundID = strings.TrimSpace(roundID)
	return ContinuationRuntimeIdentity{
		ReceiptRoundID: roundID,
		AuditRoundID:   roundID,
	}
}

// RecordContinuationProgress 记录上一轮 Goal 续跑是否产生了可计入的自主进展。
func (s *Service) RecordContinuationProgress(ctx context.Context, goalID string, roundID string, progressed bool, expectedRevision ...int64) (*protocol.Goal, error) {
	return s.RecordContinuationRuntimeProgress(
		ctx,
		goalID,
		continuationRuntimeIdentity(roundID),
		progressed,
		expectedRevision...,
	)
}

// RecordContinuationRuntimeProgress settles the durable launch receipt and
// records progress against the runtime audit round without conflating them.
func (s *Service) RecordContinuationRuntimeProgress(ctx context.Context, goalID string, identity ContinuationRuntimeIdentity, progressed bool, expectedRevision ...int64) (*protocol.Goal, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	identity = identity.normalized()
	if err := s.SettleContinuationPlan(ctx, strings.TrimSpace(goalID), identity.ReceiptRoundID, firstExpectedObjectiveRevision(expectedRevision)); err != nil {
		return nil, err
	}
	item, err := s.repo.GetGoal(ctx, strings.TrimSpace(goalID))
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrGoalNotFound
	}
	return s.recordContinuationProgressForGoal(ctx, item, identity.AuditRoundID, progressed, firstExpectedObjectiveRevision(expectedRevision))
}

// RecordContinuationFailure 记录 Goal 续跑的 runtime 失败原因，并暂停后续空转续跑。
func (s *Service) RecordContinuationFailure(ctx context.Context, goalID string, roundID string, reason string, expectedRevision ...int64) (*protocol.Goal, error) {
	return s.RecordContinuationRuntimeFailure(
		ctx,
		goalID,
		continuationRuntimeIdentity(roundID),
		reason,
		expectedRevision...,
	)
}

// RecordContinuationRuntimeFailure settles the durable launch receipt and
// records the failure against the runtime audit round without conflating them.
func (s *Service) RecordContinuationRuntimeFailure(ctx context.Context, goalID string, identity ContinuationRuntimeIdentity, reason string, expectedRevision ...int64) (*protocol.Goal, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	identity = identity.normalized()
	if err := s.SettleContinuationPlan(ctx, strings.TrimSpace(goalID), identity.ReceiptRoundID, firstExpectedObjectiveRevision(expectedRevision)); err != nil {
		return nil, err
	}
	item, err := s.repo.GetGoal(ctx, strings.TrimSpace(goalID))
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrGoalNotFound
	}
	return s.recordContinuationFailureForGoal(ctx, item, identity.AuditRoundID, reason, firstExpectedObjectiveRevision(expectedRevision))
}

// settleRuntimeContinuationReceipt closes the claim/start crash window from
// the runtime side. Dispatch marks the receipt immediately after synchronous
// registration; the terminal callback repeats the CAS so a transient first
// write cannot leave a successfully executed round eligible for replay.
func (s *Service) SettleContinuationPlan(ctx context.Context, goalID, roundID string, objectiveRevision int64) error {
	goalID, roundID = strings.TrimSpace(goalID), strings.TrimSpace(roundID)
	if goalID == "" || roundID == "" || objectiveRevision <= 0 {
		return nil
	}
	repository, ok := s.repo.(continuationPlanRepository)
	if !ok {
		return nil
	}
	err := repository.SettleGoalContinuation(ctx, goalID, roundID, objectiveRevision, s.nowFn())
	if errors.Is(err, sql.ErrNoRows) {
		// Ordinary rounds and lifecycle-cancelled stale receipts have no active
		// claim to settle. Their existing Goal/revision guards remain decisive.
		return nil
	}
	if err == nil {
		s.WakeAutoResume()
	}
	return err
}

// RecordCompletionCommandMiss 记录模型已声称目标完成但漏调 Goal 完成工具，并安排一次收尾重试。
func (s *Service) RecordCompletionCommandMiss(ctx context.Context, goalID string, roundID string, reason string, expectedRevision ...int64) (*protocol.Goal, error) {
	return s.RecordContinuationRuntimeCompletionCommandMiss(
		ctx,
		goalID,
		continuationRuntimeIdentity(roundID),
		reason,
		expectedRevision...,
	)
}

// RecordContinuationRuntimeCompletionCommandMiss settles the durable launch
// receipt and records the completion miss against the runtime audit round.
func (s *Service) RecordContinuationRuntimeCompletionCommandMiss(ctx context.Context, goalID string, identity ContinuationRuntimeIdentity, reason string, expectedRevision ...int64) (*protocol.Goal, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	identity = identity.normalized()
	if err := s.SettleContinuationPlan(ctx, strings.TrimSpace(goalID), identity.ReceiptRoundID, firstExpectedObjectiveRevision(expectedRevision)); err != nil {
		return nil, err
	}
	item, err := s.repo.GetGoal(ctx, strings.TrimSpace(goalID))
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrGoalNotFound
	}
	return s.recordCompletionCommandMissForGoal(ctx, item, identity.AuditRoundID, reason, firstExpectedObjectiveRevision(expectedRevision))
}

// RecordGoalActivity 记录显式用户/外部活动，让自动续跑 run 从当前轮重新开始计数。
func (s *Service) RecordGoalActivity(ctx context.Context, goalID string, roundID string, expectedRevision ...int64) (*protocol.Goal, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	item, err := s.repo.GetGoal(ctx, strings.TrimSpace(goalID))
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrGoalNotFound
	}
	return s.recordGoalActivityForGoal(ctx, item, strings.TrimSpace(roundID), firstExpectedObjectiveRevision(expectedRevision))
}

// RecordRoomGoalCollaborationHandback 记录 target Agent 已将控制权归还 host。
// 它不把 target 输出冒充为新的模型工具进展，也不重置
// ContinuationCount；只清除源 round 在旧实现中可能抢先写入的
// empty-progress 抑制，让后续仍受正常续跑上限约束。
func (s *Service) RecordRoomGoalCollaborationHandback(
	ctx context.Context,
	goalID string,
	roundID string,
	expectedRevision ...int64,
) (*protocol.Goal, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	item, err := s.repo.GetGoal(ctx, strings.TrimSpace(goalID))
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrGoalNotFound
	}
	return s.retryGoalMutation(ctx, item, func(current *protocol.Goal) (*protocol.Goal, error) {
		if err := rejectPendingObjectiveTransition(*current, "record Room Goal collaboration handback"); err != nil {
			return nil, err
		}
		if !objectiveRevisionMatches(*current, firstExpectedObjectiveRevision(expectedRevision)) {
			return nil, ErrGoalRevisionStale
		}
		return s.recordRoomGoalCollaborationHandbackForLoadedGoal(
			ctx,
			current,
			strings.TrimSpace(roundID),
		)
	})
}

func (s *Service) recordContinuationProgressForGoal(ctx context.Context, item *protocol.Goal, roundID string, progressed bool, expectedRevision int64) (*protocol.Goal, error) {
	return s.retryGoalMutation(ctx, item, func(current *protocol.Goal) (*protocol.Goal, error) {
		if _, planning := objectiveTransitionAwaitingPlan(*current); !planning {
			if err := rejectPendingObjectiveTransition(*current, "record continuation progress"); err != nil {
				return nil, err
			}
		}
		if !objectiveRevisionMatches(*current, expectedRevision) {
			return nil, ErrGoalRevisionStale
		}
		return s.recordContinuationProgressForLoadedGoal(ctx, current, roundID, progressed)
	})
}

func (s *Service) recordContinuationFailureForGoal(ctx context.Context, item *protocol.Goal, roundID string, reason string, expectedRevision int64) (*protocol.Goal, error) {
	return s.retryGoalMutation(ctx, item, func(current *protocol.Goal) (*protocol.Goal, error) {
		if _, planning := objectiveTransitionAwaitingPlan(*current); !planning {
			if err := rejectPendingObjectiveTransition(*current, "record continuation failure"); err != nil {
				return nil, err
			}
		}
		if !objectiveRevisionMatches(*current, expectedRevision) {
			return nil, ErrGoalRevisionStale
		}
		return s.recordContinuationFailureForLoadedGoal(ctx, current, roundID, reason)
	})
}

func (s *Service) recordCompletionCommandMissForGoal(ctx context.Context, item *protocol.Goal, roundID string, reason string, expectedRevision int64) (*protocol.Goal, error) {
	return s.retryGoalMutation(ctx, item, func(current *protocol.Goal) (*protocol.Goal, error) {
		if err := rejectPendingObjectiveTransition(*current, "record completion-command progress"); err != nil {
			return nil, err
		}
		if !objectiveRevisionMatches(*current, expectedRevision) {
			return nil, ErrGoalRevisionStale
		}
		return s.recordCompletionCommandMissForLoadedGoal(ctx, current, roundID, reason)
	})
}

func (s *Service) recordGoalActivityForGoal(ctx context.Context, item *protocol.Goal, roundID string, expectedRevision int64) (*protocol.Goal, error) {
	return s.retryGoalMutation(ctx, item, func(current *protocol.Goal) (*protocol.Goal, error) {
		if err := rejectPendingObjectiveTransition(*current, "record Goal activity"); err != nil {
			return nil, err
		}
		if !objectiveRevisionMatches(*current, expectedRevision) {
			return nil, ErrGoalRevisionStale
		}
		return s.recordGoalActivityForLoadedGoal(ctx, current, roundID)
	})
}

func (s *Service) retryGoalMutation(ctx context.Context, item *protocol.Goal, mutate func(*protocol.Goal) (*protocol.Goal, error)) (*protocol.Goal, error) {
	current := item
	for attempt := 0; attempt < goalUpdateMaxAttempts; attempt++ {
		updated, err := mutate(current)
		if !errors.Is(err, ErrGoalVersionStale) {
			return updated, err
		}
		reloaded, reloadErr := s.repo.GetGoal(ctx, current.ID)
		if reloadErr != nil {
			return nil, reloadErr
		}
		if reloaded == nil {
			return nil, ErrGoalNotFound
		}
		current = reloaded
	}
	return nil, ErrGoalVersionStale
}

func (s *Service) recordContinuationProgressForLoadedGoal(ctx context.Context, item *protocol.Goal, roundID string, progressed bool) (*protocol.Goal, error) {
	if protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive {
		return item, nil
	}
	if progressed {
		return s.resetContinuationProgress(ctx, item)
	}
	return s.noteEmptyContinuationProgress(ctx, item, roundID)
}

func (s *Service) recordContinuationFailureForLoadedGoal(ctx context.Context, item *protocol.Goal, roundID string, reason string) (*protocol.Goal, error) {
	if protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive {
		return item, nil
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "Goal continuation runtime failed"
	}
	expectedVersion := item.Version
	item.EmptyProgressCount = max(
		item.EmptyProgressCount+1,
		goalContinuationSuppressionThreshold,
	)
	item.LastError = reason
	item.Version++
	item.UpdatedAt = s.nowFn()
	payload := map[string]any{
		"empty_progress_count": item.EmptyProgressCount,
		"reason":               reason,
	}
	updated, err := s.persistGoalUpdateWithEvent(ctx, *item, expectedVersion, "continuation_failed", protocol.GoalUpdateSourceSystem, roundID, payload)
	if err != nil {
		return nil, err
	}
	s.clearWallClockGoal(*updated)
	return updated, nil
}

func (s *Service) recordCompletionCommandMissForLoadedGoal(ctx context.Context, item *protocol.Goal, roundID string, reason string) (*protocol.Goal, error) {
	if protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive {
		return item, nil
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "Goal completion was claimed, but the Goal update command was not called"
	}
	retryCount := goalCompletionCommandRetryCount(item.Metadata)
	if retryCount >= goalCompletionCommandMaxRetries {
		return s.completeAfterCompletionCommandMissRetry(ctx, item, roundID, reason)
	}
	expectedVersion := item.Version
	item.Metadata = cloneMap(item.Metadata)
	if item.Metadata == nil {
		item.Metadata = map[string]any{}
	}
	item.Metadata[goalCompletionCommandRetryMetadataKey] = goalCompletionCommandRetryCount(item.Metadata) + 1
	item.EmptyProgressCount = 0
	item.LastError = ""
	item.Version++
	item.UpdatedAt = s.nowFn()
	payload := map[string]any{
		"retry_count": item.Metadata[goalCompletionCommandRetryMetadataKey],
		"reason":      reason,
	}
	updated, err := s.persistGoalUpdateWithEvent(ctx, *item, expectedVersion, "completion_command_retry", protocol.GoalUpdateSourceSystem, roundID, payload)
	if err != nil {
		return nil, err
	}
	s.markWallClockGoalActive(*updated)
	return updated, nil
}

func (s *Service) completeAfterCompletionCommandMissRetry(ctx context.Context, item *protocol.Goal, roundID string, reason string) (*protocol.Goal, error) {
	if alignmentErr := s.ensureGoalObjectiveAlignmentReady(
		ctx,
		*item,
		RoomLeadAgentID(*item),
		roundID,
	); alignmentErr != nil {
		return s.suppressContinuationProgress(ctx, item, roundID, alignmentErr.Error())
	}
	if readinessErr := s.ensureExecutionGoalCompletionReady(ctx, *item); readinessErr != nil {
		return s.suppressContinuationProgress(ctx, item, roundID, readinessErr.Error())
	}
	if _, readinessErr := s.ensureRoomGoalCompletionReady(
		ctx,
		*item,
		RoomLeadAgentID(*item),
		roundID,
	); readinessErr != nil {
		return s.suppressContinuationProgress(ctx, item, roundID, readinessErr.Error())
	}
	retryCount := goalCompletionCommandRetryCount(item.Metadata)
	item.Metadata = clearCompletionCommandRetryMetadata(item.Metadata)
	item.EmptyProgressCount = 0
	item.LastError = ""
	return s.persistTransition(ctx, *item, protocol.GoalStatusComplete, protocol.GoalUpdateSourceSystem, "completed", roundID, map[string]any{
		"reason":      strings.TrimSpace(reason),
		"retry_count": retryCount,
		"source":      "completion_command_miss",
	})
}

func (s *Service) recordGoalActivityForLoadedGoal(ctx context.Context, item *protocol.Goal, roundID string) (*protocol.Goal, error) {
	if protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive {
		return item, nil
	}
	if item.EmptyProgressCount == 0 &&
		item.ContinuationCount == 0 &&
		strings.TrimSpace(item.LastError) == "" &&
		goalCompletionCommandRetryCount(item.Metadata) == 0 {
		return item, nil
	}
	expectedVersion := item.Version
	item.EmptyProgressCount = 0
	item.ContinuationCount = 0
	item.LastError = ""
	item.Metadata = clearContinuationReservations(clearCompletionCommandRetryMetadata(item.Metadata))
	item.Version++
	item.UpdatedAt = s.nowFn()
	payload := map[string]any{
		"continuation_count":   item.ContinuationCount,
		"empty_progress_count": item.EmptyProgressCount,
		"reason":               "explicit goal activity reset continuation run",
	}
	updated, err := s.persistGoalUpdateWithEvent(ctx, *item, expectedVersion, "continuation_reset", protocol.GoalUpdateSourceSystem, roundID, payload)
	if err != nil {
		return nil, err
	}
	s.markWallClockGoalActive(*updated)
	return updated, nil
}

func (s *Service) recordRoomGoalCollaborationHandbackForLoadedGoal(
	ctx context.Context,
	item *protocol.Goal,
	roundID string,
) (*protocol.Goal, error) {
	if protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive {
		return item, nil
	}
	if item.EmptyProgressCount == 0 &&
		strings.TrimSpace(item.LastError) == "" &&
		goalCompletionCommandRetryCount(item.Metadata) == 0 {
		return item, nil
	}
	expectedVersion := item.Version
	item.EmptyProgressCount = 0
	item.LastError = ""
	item.Metadata = clearCompletionCommandRetryMetadata(item.Metadata)
	item.Version++
	item.UpdatedAt = s.nowFn()
	updated, err := s.persistGoalUpdateWithEvent(
		ctx,
		*item,
		expectedVersion,
		"room_collaboration_handback",
		protocol.GoalUpdateSourceSystem,
		roundID,
		map[string]any{
			"continuation_count":   item.ContinuationCount,
			"empty_progress_count": item.EmptyProgressCount,
			"reason":               "Room collaboration target returned control to the Goal host",
		},
	)
	if err != nil {
		return nil, err
	}
	s.markWallClockGoalActive(*updated)
	return updated, nil
}

func (s *Service) resetContinuationProgress(ctx context.Context, item *protocol.Goal) (*protocol.Goal, error) {
	if item.EmptyProgressCount == 0 &&
		strings.TrimSpace(item.LastError) == "" &&
		goalCompletionCommandRetryCount(item.Metadata) == 0 {
		return item, nil
	}
	expectedVersion := item.Version
	item.EmptyProgressCount = 0
	item.LastError = ""
	item.Metadata = clearCompletionCommandRetryMetadata(item.Metadata)
	item.Version++
	item.UpdatedAt = s.nowFn()
	updated, err := s.repo.UpdateGoal(ctx, *item, expectedVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGoalVersionStale
	}
	if err != nil {
		return nil, err
	}
	s.markWallClockGoalActive(*updated)
	return updated, nil
}

func (s *Service) noteEmptyContinuationProgress(ctx context.Context, item *protocol.Goal, roundID string, reasonOverride ...string) (*protocol.Goal, error) {
	reason := "goal continuation produced no counted command progress"
	if len(reasonOverride) > 0 && strings.TrimSpace(reasonOverride[0]) != "" {
		reason = strings.TrimSpace(reasonOverride[0])
	}
	expectedVersion := item.Version
	item.EmptyProgressCount++
	item.Version++
	item.UpdatedAt = s.nowFn()
	payload := map[string]any{
		"empty_progress_count": item.EmptyProgressCount,
		"reason":               reason,
	}
	eventType := "continuation_recovery_scheduled"
	if goalContinuationSuppressed(*item) {
		eventType = "continuation_suppressed"
	}
	updated, err := s.persistGoalUpdateWithEvent(ctx, *item, expectedVersion, eventType, protocol.GoalUpdateSourceSystem, roundID, payload)
	if err != nil {
		return nil, err
	}
	if goalContinuationSuppressed(*updated) {
		s.clearWallClockGoal(*updated)
	} else {
		s.markWallClockGoalActive(*updated)
	}
	return updated, nil
}

func (s *Service) suppressContinuationProgress(
	ctx context.Context,
	item *protocol.Goal,
	roundID string,
	reason string,
) (*protocol.Goal, error) {
	item.EmptyProgressCount = max(
		item.EmptyProgressCount,
		goalContinuationSuppressionThreshold-1,
	)
	return s.noteEmptyContinuationProgress(ctx, item, roundID, reason)
}

func goalContinuationSuppressed(item protocol.Goal) bool {
	return item.ContinuationState() == protocol.GoalContinuationStateSuspended
}

func goalCompletionCommandRetryCount(metadata map[string]any) int {
	if metadata == nil {
		return 0
	}
	switch value := metadata[goalCompletionCommandRetryMetadataKey].(type) {
	case int:
		return value
	case int32:
		return int(value)
	case int64:
		return int(value)
	case float32:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}

func clearCompletionCommandRetryMetadata(metadata map[string]any) map[string]any {
	if goalCompletionCommandRetryCount(metadata) == 0 {
		return metadata
	}
	copied := cloneMap(metadata)
	delete(copied, goalCompletionCommandRetryMetadataKey)
	if len(copied) == 0 {
		return nil
	}
	return copied
}

// resetSuppressionForActiveTransition lets a user explicitly resume dispatch,
// but it deliberately preserves ContinuationCount. Pause/resume is a control
// transition inside one objective revision, not a fresh continuation budget.
func resetSuppressionForActiveTransition(source protocol.GoalUpdateSource, status protocol.GoalStatus) bool {
	if protocol.NormalizeGoalStatus(status) != protocol.GoalStatusActive {
		return false
	}
	return source == protocol.GoalUpdateSourceUser || source == protocol.GoalUpdateSourceExternal
}

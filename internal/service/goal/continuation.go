// INPUT: active Goal、上一轮结果、objective transition 与当前 session 可调度状态。
// OUTPUT: 带版本约束的普通 Goal continuation、awaiting_plan 专用 successor-planning continuation，或明确的延迟/终止决定。
// POS: Goal 自动续跑与 Goal-bound successor 规划的唯一计划和最终有效性校验入口。
package goal

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/objectivealignment"
)

// ContinuationPlanProvider 提供续跑候选的规划与并发有效性校验。
type ContinuationPlanProvider interface {
	PlanContinuationForSession(context.Context, string, string) (*protocol.GoalContinuation, error)
	GoalContinuationStillCurrent(context.Context, protocol.GoalContinuation) (bool, error)
}

type continuationPlanReleaser interface {
	ReleaseContinuationPlan(context.Context, protocol.GoalContinuation, string) (*protocol.Goal, error)
}

const (
	goalContinuationClaimLease   = 2 * time.Minute
	goalContinuationStartedLease = 15 * time.Minute
	goalContinuationRetryBase    = 10 * time.Second
)

// PrepareContinuationForDispatch 统一续跑候选在进入运行时前的状态转换。
// shouldDefer 必须读取最新队列状态，因为规划过程可能与显式输入并发。
func PrepareContinuationForDispatch(
	ctx context.Context,
	provider ContinuationPlanProvider,
	sessionKey string,
	previousRoundID string,
	shouldDefer func(protocol.GoalContinuation) bool,
) (*protocol.GoalContinuation, error) {
	if provider == nil {
		return nil, nil
	}
	plan, err := provider.PlanContinuationForSession(ctx, sessionKey, previousRoundID)
	if err != nil || plan == nil {
		return plan, err
	}
	return ValidateContinuationForDispatch(ctx, provider, *plan, shouldDefer)
}

// ValidateContinuationForDispatch 校验已经规划出的候选是否仍可进入运行时。
// 调用方可在规划与校验之间插入自身的目标存在性检查。
func ValidateContinuationForDispatch(
	ctx context.Context,
	provider ContinuationPlanProvider,
	plan protocol.GoalContinuation,
	shouldDefer func(protocol.GoalContinuation) bool,
) (*protocol.GoalContinuation, error) {
	if shouldDefer != nil && shouldDefer(plan) {
		releaseContinuationPlan(ctx, provider, plan, "Goal continuation deferred before dispatch")
		return nil, nil
	}
	current, err := provider.GoalContinuationStillCurrent(ctx, plan)
	if err != nil {
		return nil, err
	}
	if !current {
		releaseContinuationPlan(ctx, provider, plan, "Goal continuation stale before dispatch")
		return nil, nil
	}
	return &plan, nil
}

func releaseContinuationPlan(ctx context.Context, provider ContinuationPlanProvider, plan protocol.GoalContinuation, reason string) {
	if releaser, ok := provider.(continuationPlanReleaser); ok {
		_, _ = releaser.ReleaseContinuationPlan(ctx, plan, reason)
	}
}

const (
	goalContinuationPurpose                       = "goal_continuation"
	goalObjectiveTransitionPlanningPurpose        = "goal_objective_transition_planning"
	goalContinuationReservationsMetadataKey       = "continuation_reservation_round_ids"
	goalTransitionContinuationIDMetadataKey       = "goal_transition_id"
	goalTransitionSuccessorExecutionIDMetadataKey = "successor_execution_id"
)

//go:embed templates/continuation.md
var continuationPromptTemplate string

// PlanContinuationForSession 在当前 Goal 仍需推进时生成下一轮隐藏输入。
func (s *Service) PlanContinuationForSession(ctx context.Context, sessionKey string, previousRoundID string) (*protocol.GoalContinuation, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	if !s.config.GoalAutoContinueEnabled {
		return nil, nil
	}
	normalized, err := protocol.RequireStructuredSessionKey(sessionKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGoalInvalidInput, err)
	}
	item, err := s.repo.GetCurrentGoal(ctx, normalized)
	if err != nil {
		return nil, err
	}
	if item == nil || protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive {
		return nil, nil
	}
	return s.planContinuationForGoal(ctx, item, strings.TrimSpace(previousRoundID))
}

func (s *Service) planContinuationForGoal(ctx context.Context, item *protocol.Goal, previousRoundID string) (*protocol.GoalContinuation, error) {
	current := item
	for attempt := 0; attempt < goalUpdateMaxAttempts; attempt++ {
		plan, err := s.planContinuationForLoadedGoal(ctx, current, previousRoundID)
		if !errors.Is(err, ErrGoalVersionStale) {
			return plan, err
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

func (s *Service) planContinuationForLoadedGoal(ctx context.Context, item *protocol.Goal, previousRoundID string) (*protocol.GoalContinuation, error) {
	if protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive {
		return nil, nil
	}
	if recovered, open, err := s.recoverOpenContinuationPlan(ctx, *item); err != nil || open {
		return recovered, err
	}
	if transition, ok := objectiveTransitionAwaitingPlan(*item); ok {
		if s.goalBudgetExhausted(*item) {
			_, err := s.limitForSystem(ctx, *item, protocol.GoalStatusBudgetLimited, "budget_limited", previousRoundID, "Goal token budget exhausted")
			return nil, err
		}
		if max := s.config.GoalMaxContinuationsPerRun; max > 0 && item.ContinuationCount >= max {
			_, err := s.limitForSystem(ctx, *item, protocol.GoalStatusUsageLimited, "usage_limited", previousRoundID, "Goal transition planning continuation limit reached")
			return nil, err
		}
		return s.reserveContinuationPlanForLoadedGoal(
			ctx,
			item,
			previousRoundID,
			"",
			goalObjectiveTransitionPlanningPurpose,
			buildObjectiveTransitionPlanningPrompt(*item, transition),
			map[string]string{
				goalTransitionContinuationIDMetadataKey:       transition.ID,
				goalTransitionSuccessorExecutionIDMetadataKey: transition.SuccessorExecutionID,
			},
		)
	}
	if GoalObjectiveTransitionPending(*item) {
		return nil, nil
	}
	executionID, bindingErr := s.goalContinuationExecutionID(ctx, *item)
	if bindingErr != nil {
		return nil, bindingErr
	}
	if s.goalBudgetExhausted(*item) {
		_, err := s.limitForSystem(ctx, *item, protocol.GoalStatusBudgetLimited, "budget_limited", previousRoundID, "Goal token budget exhausted")
		return nil, err
	}
	if goalContinuationSuppressed(*item) {
		if goalCompletionCommandRetryCount(item.Metadata) >= goalCompletionCommandMaxRetries {
			completed, err := s.completeAfterCompletionCommandMissRetry(
				ctx,
				item,
				previousRoundID,
				"Goal completion finalization retry already exhausted",
			)
			if err != nil {
				return nil, err
			}
			if completed != nil {
				if _, err = s.FinalizeUsageForGoal(
					ctx,
					completed.ID,
					protocol.GoalUsage{},
					previousRoundID,
				); err != nil {
					return nil, err
				}
			}
			return nil, nil
		}
		return nil, nil
	}
	if max := s.config.GoalMaxContinuationsPerRun; max > 0 && item.ContinuationCount >= max {
		_, err := s.limitForSystem(ctx, *item, protocol.GoalStatusUsageLimited, "usage_limited", previousRoundID, "Goal auto-continuation limit reached")
		return nil, err
	}

	return s.reserveContinuationPlanForLoadedGoal(
		ctx,
		item,
		previousRoundID,
		executionID,
		goalContinuationPurpose,
		buildContinuationPrompt(*item, previousRoundID, executionID != ""),
		nil,
	)
}

func (s *Service) reserveContinuationPlanForLoadedGoal(
	ctx context.Context,
	item *protocol.Goal,
	previousRoundID string,
	executionID string,
	purpose string,
	prompt string,
	extraMetadata map[string]string,
) (*protocol.GoalContinuation, error) {
	roundID := s.idFactory("goal_continuation")
	expectedVersion := item.Version
	now := s.nowFn()
	durableRepository, durable := s.repo.(continuationPlanRepository)
	if !durable {
		item.Metadata = addContinuationReservation(item.Metadata, roundID)
	}
	item.ContinuationCount++
	item.Version++
	item.UpdatedAt = now
	item.LastError = ""
	payload := map[string]any{
		"continuation_count": item.ContinuationCount,
		"purpose":            strings.TrimSpace(purpose),
	}
	if previous := strings.TrimSpace(previousRoundID); previous != "" {
		payload["previous_round_id"] = previous
	}
	event := s.newGoalEvent(*item, "continuation_scheduled", protocol.GoalUpdateSourceSystem, roundID, payload, now)
	metadata := map[string]string{
		"goal_id":           item.ID,
		"session_key":       item.SessionKey,
		"previous_round_id": strings.TrimSpace(previousRoundID),
	}
	for key, value := range extraMetadata {
		if key = strings.TrimSpace(key); key != "" {
			metadata[key] = strings.TrimSpace(value)
		}
	}
	var updated *protocol.Goal
	var err error
	if durable {
		nextAttemptAt := now
		updated, err = durableRepository.ReserveGoalContinuation(ctx, *item, expectedVersion, event, protocol.GoalContinuationPlan{
			RoundID: roundID, GoalID: item.ID, SessionKey: item.SessionKey,
			ObjectiveRevision: item.ObjectiveRevision(), ExecutionID: strings.TrimSpace(executionID),
			PreviousRoundID: strings.TrimSpace(previousRoundID), Prompt: strings.TrimSpace(prompt),
			Purpose: strings.TrimSpace(purpose), Metadata: metadata,
			Status: protocol.GoalContinuationPlanStatusScheduled, Version: 1,
			NextAttemptAt: &nextAttemptAt, CreatedAt: now, UpdatedAt: now,
		})
	} else {
		updated, err = s.repo.UpdateGoalWithEvents(ctx, *item, expectedVersion, []protocol.GoalEvent{event})
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGoalVersionStale
	}
	if err != nil {
		return nil, err
	}
	s.publishGoalEvent(ctx, *updated, event)
	s.WakeAutoResume()
	return &protocol.GoalContinuation{
		Goal:           *updated,
		ExecutionID:    strings.TrimSpace(executionID),
		RoundID:        roundID,
		Prompt:         strings.TrimSpace(prompt),
		HiddenFromUser: true,
		Synthetic:      true,
		Purpose:        strings.TrimSpace(purpose),
		Metadata:       metadata,
	}, nil
}

func (s *Service) recoverOpenContinuationPlan(ctx context.Context, item protocol.Goal) (*protocol.GoalContinuation, bool, error) {
	repository, ok := s.repo.(continuationPlanRepository)
	if !ok {
		return nil, false, nil
	}
	record, err := repository.GetOpenGoalContinuation(ctx, item.ID, item.ObjectiveRevision())
	if err != nil || record == nil {
		return nil, false, err
	}
	now := s.nowFn()
	switch record.Status {
	case protocol.GoalContinuationPlanStatusScheduled:
		if record.NextAttemptAt != nil && record.NextAttemptAt.After(now) {
			return nil, true, nil
		}
	case protocol.GoalContinuationPlanStatusClaimed:
		if record.ClaimExpiresAt != nil && record.ClaimExpiresAt.After(now) {
			return nil, true, nil
		}
	case protocol.GoalContinuationPlanStatusStarted:
		if record.ClaimExpiresAt != nil && record.ClaimExpiresAt.After(now) {
			return nil, true, nil
		}
	default:
		return nil, false, nil
	}
	return continuationFromRecord(item, *record), true, nil
}

func continuationFromRecord(item protocol.Goal, record protocol.GoalContinuationPlan) *protocol.GoalContinuation {
	return &protocol.GoalContinuation{
		Goal: item, ExecutionID: record.ExecutionID, RoundID: record.RoundID,
		Prompt: record.Prompt, HiddenFromUser: true, Synthetic: true,
		Purpose: record.Purpose, Metadata: record.Metadata,
	}
}

// GoalContinuationStillCurrent 判断已生成的隐藏续跑是否仍持有当前 objective 的待启动 reservation。
func (s *Service) GoalContinuationStillCurrent(ctx context.Context, plan protocol.GoalContinuation) (bool, error) {
	if err := s.ensureEnabled(); err != nil {
		return false, err
	}
	goalID := strings.TrimSpace(plan.Goal.ID)
	sessionKey := strings.TrimSpace(plan.Goal.SessionKey)
	if sessionKey == "" && plan.Metadata != nil {
		sessionKey = strings.TrimSpace(plan.Metadata["session_key"])
	}
	if goalID == "" && plan.Metadata != nil {
		goalID = strings.TrimSpace(plan.Metadata["goal_id"])
	}
	if goalID == "" || sessionKey == "" {
		return false, fmt.Errorf("%w: continuation plan missing goal identity", ErrGoalInvalidInput)
	}
	normalized, err := protocol.RequireStructuredSessionKey(sessionKey)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrGoalInvalidInput, err)
	}
	item, err := s.repo.GetCurrentGoal(ctx, normalized)
	if err != nil {
		return false, err
	}
	if item == nil || protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive {
		return false, nil
	}
	durableMatch := false
	if repository, ok := s.repo.(continuationPlanRepository); ok {
		record, recordErr := repository.GetOpenGoalContinuation(ctx, item.ID, item.ObjectiveRevision())
		if recordErr != nil {
			return false, recordErr
		}
		durableMatch = record != nil && strings.TrimSpace(record.RoundID) == strings.TrimSpace(plan.RoundID)
	}
	if plan.Purpose == goalObjectiveTransitionPlanningPurpose {
		return objectiveTransitionPlanningContinuationMatches(*item, plan, durableMatch), nil
	}
	if GoalObjectiveTransitionPending(*item) {
		return false, nil
	}
	executionID, bindingErr := s.goalContinuationExecutionID(ctx, *item)
	if bindingErr != nil {
		return false, bindingErr
	}
	return item.ID == goalID &&
		objectiveRevisionMatches(*item, plan.Goal.ObjectiveRevision()) &&
		executionID == strings.TrimSpace(plan.ExecutionID) &&
		(durableMatch || hasContinuationReservation(item.Metadata, plan.RoundID)), nil
}

// ClaimContinuationPlan 原子取得隐藏续跑的唯一启动权；后续 runtime 启动失败必须另记 continuation_failed。
func (s *Service) ClaimContinuationPlan(ctx context.Context, plan protocol.GoalContinuation) (*protocol.Goal, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	goalID := continuationPlanGoalID(plan)
	if goalID == "" {
		return nil, fmt.Errorf("%w: continuation plan missing goal identity", ErrGoalInvalidInput)
	}
	item, err := s.repo.GetGoal(ctx, goalID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrGoalNotFound
	}
	if protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive {
		return nil, ErrGoalInvalidState
	}
	expectedRevision := plan.Goal.ObjectiveRevision()
	if repository, ok := s.repo.(continuationPlanRepository); ok {
		record, recordErr := repository.GetOpenGoalContinuation(ctx, item.ID, item.ObjectiveRevision())
		if recordErr != nil {
			return nil, recordErr
		}
		if record == nil || strings.TrimSpace(record.RoundID) != strings.TrimSpace(plan.RoundID) ||
			!objectiveRevisionMatches(*item, expectedRevision) ||
			strings.TrimSpace(record.ExecutionID) != strings.TrimSpace(plan.ExecutionID) ||
			strings.TrimSpace(record.Purpose) != strings.TrimSpace(plan.Purpose) {
			return nil, ErrGoalRevisionStale
		}
		if plan.Purpose == goalObjectiveTransitionPlanningPurpose &&
			!objectiveTransitionPlanningContinuationMatches(*item, plan, true) {
			return nil, ErrGoalRevisionStale
		}
		return s.claimContinuationPlanForLoadedGoal(ctx, item, plan.RoundID)
	}
	return s.retryGoalMutation(ctx, item, func(current *protocol.Goal) (*protocol.Goal, error) {
		if plan.Purpose == goalObjectiveTransitionPlanningPurpose {
			durableMatch := false
			if repository, ok := s.repo.(continuationPlanRepository); ok {
				record, recordErr := repository.GetOpenGoalContinuation(ctx, current.ID, current.ObjectiveRevision())
				if recordErr != nil {
					return nil, recordErr
				}
				durableMatch = record != nil && strings.TrimSpace(record.RoundID) == strings.TrimSpace(plan.RoundID)
			}
			if !objectiveTransitionPlanningContinuationMatches(*current, plan, durableMatch) {
				return nil, ErrGoalRevisionStale
			}
			return s.claimContinuationPlanForLoadedGoal(ctx, current, plan.RoundID)
		}
		if pendingErr := rejectPendingObjectiveTransition(*current, "claim Goal continuation"); pendingErr != nil {
			return nil, pendingErr
		}
		if !objectiveRevisionMatches(*current, expectedRevision) {
			return nil, ErrGoalRevisionStale
		}
		executionID, bindingErr := s.goalContinuationExecutionID(ctx, *current)
		if bindingErr != nil {
			return nil, bindingErr
		}
		if executionID != strings.TrimSpace(plan.ExecutionID) {
			return nil, ErrGoalRevisionStale
		}
		return s.claimContinuationPlanForLoadedGoal(ctx, current, plan.RoundID)
	})
}

func objectiveTransitionPlanningContinuationMatches(
	item protocol.Goal,
	plan protocol.GoalContinuation,
	durableMatch ...bool,
) bool {
	transition, ok := objectiveTransitionAwaitingPlan(item)
	reserved := hasContinuationReservation(item.Metadata, plan.RoundID)
	if len(durableMatch) > 0 {
		reserved = reserved || durableMatch[0]
	}
	if !ok || strings.TrimSpace(plan.ExecutionID) != "" ||
		item.ID != continuationPlanGoalID(plan) ||
		!objectiveRevisionMatches(item, plan.Goal.ObjectiveRevision()) ||
		!reserved {
		return false
	}
	if plan.Metadata == nil {
		return false
	}
	return strings.TrimSpace(plan.Metadata[goalTransitionContinuationIDMetadataKey]) == transition.ID &&
		strings.TrimSpace(plan.Metadata[goalTransitionSuccessorExecutionIDMetadataKey]) ==
			transition.SuccessorExecutionID
}

func (s *Service) goalContinuationExecutionID(
	ctx context.Context,
	item protocol.Goal,
) (string, error) {
	resolution, err := s.resolveGoalExecutionBinding(ctx, item)
	if err != nil {
		return "", fmt.Errorf("resolve Goal Execution binding: %w", err)
	}
	switch resolution.State {
	case protocol.GoalExecutionBindingStateStandalone,
		protocol.GoalExecutionBindingStateReserved:
		return "", nil
	case protocol.GoalExecutionBindingStateConfirmed:
		return strings.TrimSpace(resolution.ExecutionID), nil
	case protocol.GoalExecutionBindingStatePending,
		protocol.GoalExecutionBindingStateConflict:
		return "", fmt.Errorf(
			"%w: Goal Execution binding is %s",
			ErrGoalInvalidState,
			resolution.State,
		)
	default:
		return "", fmt.Errorf("%w: Goal Execution binding state is unknown", ErrGoalInvalidState)
	}
}

func (s *Service) claimContinuationPlanForLoadedGoal(ctx context.Context, item *protocol.Goal, roundID string) (*protocol.Goal, error) {
	if protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive {
		return nil, ErrGoalInvalidState
	}
	if repository, ok := s.repo.(continuationPlanRepository); ok {
		now := s.nowFn()
		_, err := repository.ClaimGoalContinuation(ctx, roundID, now, now.Add(goalContinuationClaimLease))
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrGoalRevisionStale
		}
		if err != nil {
			return nil, err
		}
		return item, nil
	}
	metadata, found := removeContinuationReservation(item.Metadata, roundID)
	if !found {
		return nil, ErrGoalRevisionStale
	}
	expectedVersion := item.Version
	item.Metadata = metadata
	item.Version++
	item.UpdatedAt = s.nowFn()
	updated, err := s.persistGoalUpdateWithEvent(ctx, *item, expectedVersion, "continuation_started", protocol.GoalUpdateSourceSystem, roundID, map[string]any{
		"continuation_count": item.ContinuationCount,
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// MarkContinuationPlanStarted settles a claimed receipt after the runtime has
// synchronously registered the exact round identity.
func (s *Service) MarkContinuationPlanStarted(ctx context.Context, plan protocol.GoalContinuation) error {
	repository, ok := s.repo.(continuationPlanRepository)
	if !ok {
		return nil
	}
	now := s.nowFn()
	err := repository.MarkGoalContinuationStarted(ctx, plan.RoundID, now, now.Add(goalContinuationStartedLease))
	if errors.Is(err, sql.ErrNoRows) {
		return ErrGoalRevisionStale
	}
	if err == nil {
		s.WakeAutoResume()
	}
	return err
}

// RetryContinuationPlan records a transient pre-runtime failure without
// suspending the Goal or incrementing its continuation count again.
func (s *Service) RetryContinuationPlan(ctx context.Context, plan protocol.GoalContinuation, reason string) error {
	repository, ok := s.repo.(continuationPlanRepository)
	if !ok {
		_, err := s.RecordContinuationFailure(ctx, plan.Goal.ID, plan.RoundID, reason, plan.Goal.ObjectiveRevision())
		return err
	}
	record, err := repository.GetOpenGoalContinuation(ctx, continuationPlanGoalID(plan), plan.Goal.ObjectiveRevision())
	if err != nil || record == nil {
		return err
	}
	delay := goalContinuationRetryBase
	for attempt := 1; attempt < record.AttemptCount && delay < 5*time.Minute; attempt++ {
		delay *= 2
	}
	if delay > 5*time.Minute {
		delay = 5 * time.Minute
	}
	now := s.nowFn()
	err = repository.RetryGoalContinuation(ctx, plan.RoundID, strings.TrimSpace(reason), now.Add(delay), now)
	if err == nil {
		s.WakeAutoResume()
	}
	return err
}

// ReleaseContinuationPlan 撤销尚未启动的隐藏续跑计划，避免未执行的 candidate 消耗续跑次数。
func (s *Service) ReleaseContinuationPlan(ctx context.Context, plan protocol.GoalContinuation, reason string) (*protocol.Goal, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	goalID := continuationPlanGoalID(plan)
	if goalID == "" {
		return nil, fmt.Errorf("%w: continuation plan missing goal identity", ErrGoalInvalidInput)
	}
	item, err := s.repo.GetGoal(ctx, goalID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrGoalNotFound
	}
	updated, err := s.retryGoalMutation(ctx, item, func(current *protocol.Goal) (*protocol.Goal, error) {
		return s.releaseContinuationPlanForLoadedGoal(ctx, current, plan.RoundID, reason)
	})
	if err == nil {
		s.WakeAutoResume()
	}
	return updated, err
}

func continuationPlanGoalID(plan protocol.GoalContinuation) string {
	goalID := strings.TrimSpace(plan.Goal.ID)
	if goalID == "" && plan.Metadata != nil {
		goalID = strings.TrimSpace(plan.Metadata["goal_id"])
	}
	return goalID
}

func (s *Service) releaseContinuationPlanForLoadedGoal(
	ctx context.Context,
	item *protocol.Goal,
	roundID string,
	reason string,
) (*protocol.Goal, error) {
	if repository, ok := s.repo.(continuationPlanRepository); ok {
		if item.ContinuationCount <= 0 {
			return item, nil
		}
		expectedVersion := item.Version
		item.ContinuationCount--
		item.Version++
		item.UpdatedAt = s.nowFn()
		reason = strings.TrimSpace(reason)
		if reason == "" {
			reason = "Goal continuation deferred before dispatch"
		}
		event := s.newGoalEvent(*item, "continuation_deferred", protocol.GoalUpdateSourceSystem, roundID, map[string]any{
			"continuation_count": item.ContinuationCount,
			"reason":             reason,
		}, item.UpdatedAt)
		updated, err := repository.ReleaseGoalContinuation(ctx, *item, expectedVersion, event, roundID, item.UpdatedAt)
		if errors.Is(err, sql.ErrNoRows) {
			return item, nil
		}
		if err != nil {
			return nil, err
		}
		s.publishGoalEvent(ctx, *updated, event)
		return updated, nil
	}
	metadata, found := removeContinuationReservation(item.Metadata, roundID)
	if !found || item.ContinuationCount <= 0 {
		return item, nil
	}
	expectedVersion := item.Version
	item.Metadata = metadata
	item.ContinuationCount--
	item.Version++
	item.UpdatedAt = s.nowFn()
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "Goal continuation deferred before dispatch"
	}
	updated, err := s.persistGoalUpdateWithEvent(ctx, *item, expectedVersion, "continuation_deferred", protocol.GoalUpdateSourceSystem, roundID, map[string]any{
		"continuation_count": item.ContinuationCount,
		"reason":             reason,
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func continuationReservations(metadata map[string]any) []string {
	if metadata == nil {
		return nil
	}
	values := make([]string, 0)
	switch typed := metadata[goalContinuationReservationsMetadataKey].(type) {
	case []string:
		values = append(values, typed...)
	case []any:
		for _, value := range typed {
			if text, ok := value.(string); ok {
				values = append(values, text)
			}
		}
	}
	return values
}

func addContinuationReservation(metadata map[string]any, roundID string) map[string]any {
	metadata = cloneMap(metadata)
	if metadata == nil {
		metadata = map[string]any{}
	}
	reservations := continuationReservations(metadata)
	reservations = append(reservations, strings.TrimSpace(roundID))
	metadata[goalContinuationReservationsMetadataKey] = reservations
	return metadata
}

func hasContinuationReservation(metadata map[string]any, roundID string) bool {
	roundID = strings.TrimSpace(roundID)
	for _, candidate := range continuationReservations(metadata) {
		if strings.TrimSpace(candidate) == roundID {
			return true
		}
	}
	return false
}

func removeContinuationReservation(metadata map[string]any, roundID string) (map[string]any, bool) {
	roundID = strings.TrimSpace(roundID)
	reservations := continuationReservations(metadata)
	for index, candidate := range reservations {
		if strings.TrimSpace(candidate) != roundID {
			continue
		}
		metadata = cloneMap(metadata)
		reservations = append(reservations[:index:index], reservations[index+1:]...)
		if len(reservations) == 0 {
			delete(metadata, goalContinuationReservationsMetadataKey)
		} else {
			metadata[goalContinuationReservationsMetadataKey] = reservations
		}
		return metadata, true
	}
	return metadata, false
}

func clearContinuationReservations(metadata map[string]any) map[string]any {
	if len(continuationReservations(metadata)) == 0 {
		return metadata
	}
	metadata = cloneMap(metadata)
	delete(metadata, goalContinuationReservationsMetadataKey)
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}

func (s *Service) goalBudgetExhausted(item protocol.Goal) bool {
	if item.TokenBudget == nil || *item.TokenBudget <= 0 {
		return false
	}
	return item.Usage.BudgetTokens() >= *item.TokenBudget
}

func (s *Service) limitForSystem(
	ctx context.Context,
	item protocol.Goal,
	status protocol.GoalStatus,
	eventType string,
	roundID string,
	reason string,
) (*protocol.Goal, error) {
	item.LastError = strings.TrimSpace(reason)
	payload := map[string]any{
		"reason":        item.LastError,
		"usage_total":   item.Usage.BudgetTokens(),
		"budget_tokens": item.Usage.BudgetTokens(),
		"actual_tokens": item.Usage.ActualTokens(),
	}
	if item.TokenBudget != nil {
		payload["token_budget"] = *item.TokenBudget
	}
	return s.persistTransition(ctx, item, status, protocol.GoalUpdateSourceSystem, eventType, roundID, payload)
}

func buildContinuationPrompt(item protocol.Goal, previousRoundID string, confirmedManagedBinding bool) string {
	objective := escapeGoalPromptText(strings.TrimSpace(item.Objective))
	tokenBudget := "none"
	if item.TokenBudget != nil {
		tokenBudget = fmt.Sprintf("%d", *item.TokenBudget)
	}
	remainingTokens := "unbounded"
	if remaining := item.RemainingTokens(); remaining != nil {
		remainingTokens = fmt.Sprintf("%d", *remaining)
	}
	return renderGoalPromptTemplate(continuationPromptTemplate, map[string]string{
		"objective":                     objective,
		"room_goal_lead_note":           buildRoomGoalLeadNote(item),
		"objective_alignment_criteria":  buildObjectiveAlignmentCriteria(item),
		"objective_alignment_contract":  objectivealignment.PromptContract(),
		"completion_command_retry_note": buildCompletionCommandRetryNote(item, confirmedManagedBinding),
		"no_progress_recovery_note":     buildNoProgressRecoveryNote(item),
		"tokens_used":                   fmt.Sprintf("%d", item.Usage.BudgetTokens()),
		"token_budget":                  tokenBudget,
		"remaining_tokens":              remainingTokens,
	})
}

func buildNoProgressRecoveryNote(item protocol.Goal) string {
	if item.EmptyProgressCount <= 0 || goalContinuationSuppressed(item) {
		return ""
	}
	return strings.TrimSpace(`
No-progress recovery boundary:
- The immediately preceding continuation inspected or described state but produced no counted mutation or durable work.
- This is the single automatic recovery turn. Execute the next concrete action now; do not merely announce what you will do next.
- If no valid mutation is appropriate, gather new evidence, start an accountable handoff, or report the actual blocker through the normal blocked audit. A second empty turn stops automatic continuation.
`)
}

func buildObjectiveTransitionPlanningPrompt(
	item protocol.Goal,
	transition ObjectiveTransition,
) string {
	return strings.TrimSpace(fmt.Sprintf(`
Continue the active Goal after its objective was explicitly retargeted.

Goal objective:
%s

The predecessor WorkGraph is already fenced. Build the fresh successor WorkGraph now:
1. Load execution-orchestrator and use only the host-injected "${NEXUS_COMMAND_PATH}" --json execution contract|inspect|invoke workflow; operation names are not standalone tools and nexusctl is forbidden.
2. Invoke prepare_plan_execution once with one complete Nexus Plan Document and goal_binding=current.
3. Commit exactly the returned proposal_id and proposal_digest by invoking plan_execution through the same CLI.
4. Do not reuse, mutate, or resume the superseded predecessor Execution.
5. Do not call retarget_goal again unless the user changes the objective again.

The backend owns successor identity %s. Do not put that identity into command input.
`,
		escapeGoalPromptText(strings.TrimSpace(item.Objective)),
		escapeGoalPromptText(strings.TrimSpace(transition.SuccessorExecutionID)),
	))
}

func buildObjectiveAlignmentCriteria(item protocol.Goal) string {
	target, err := objectivealignment.NormalizeTarget(goalObjectiveAlignmentTarget(item))
	if err != nil {
		return ""
	}
	var output strings.Builder
	output.WriteString("<completion_criteria>")
	for index, criterion := range target.Criteria {
		fmt.Fprintf(
			&output,
			"\n%d. %s",
			index+1,
			escapeGoalPromptText(criterion),
		)
	}
	output.WriteString("\n</completion_criteria>")
	return output.String()
}

func buildRoomGoalLeadNote(item protocol.Goal) string {
	if !protocol.IsRoomSharedSessionKey(item.SessionKey) {
		return ""
	}
	leadAgentID := RoomLeadAgentID(item)
	if leadAgentID == "" {
		return ""
	}
	leadName := RoomLeadAgentName(item)
	leadLabel := leadAgentID
	if leadName != "" {
		leadLabel = fmt.Sprintf("%s (%s)", leadName, leadAgentID)
	}
	return strings.TrimSpace(fmt.Sprintf(`
Room Goal lead:
- This is a shared Room Goal. You are the assigned lead agent: %s.
- The Goal belongs to the room, not to your private session. As the current lead, you decide when its objective is satisfied and may complete it after the required readiness checks pass.
- Follow all Room rules and member roles. A public @mention only requests a conversational or untracked one-off contribution; it never creates an Assignment or WorkBinding. A substantive public reply to a Goal-attributed handoff may be retained as collaboration audit evidence.
- When another member must own an accountable deliverable, materialize or continue the managed WorkGraph and use assign_work. Use @ only when a conversational contribution is sufficient.
- Collaboration evidence is optional audit context, not a completion requirement. Do not manufacture collaboration merely because the Room has multiple members.
- If a public @ request is the right next step, make it the public reply for this turn and wait for or explicitly cancel that in-flight handoff before completing the Goal.
- When delegated work returns, inspect the result, continue or delegate again if needed, and only mark the Goal complete after the full room objective is verified and no attributed Room work remains in flight.
`, leadLabel))
}

func buildCompletionCommandRetryNote(item protocol.Goal, confirmedManagedBinding bool) string {
	if goalCompletionCommandRetryCount(item.Metadata) <= 0 {
		return ""
	}
	if !confirmedManagedBinding {
		return strings.TrimSpace(
			"Completion finalization retry:\n" +
				"- A previous goal-continuation response stated that the objective was complete but did not produce an applied Goal completion command receipt.\n" +
				"- This Goal has no confirmed managed WorkGraph binding, so load `goal-manager` and invoke `update_goal` with status \"complete\" only through the host-injected `\"${NEXUS_COMMAND_PATH}\" --json goal contract|inspect|invoke` workflow before any final response. Never use nexusctl or a standalone operation tool. Do not manufacture an alignment audit or WorkGraph just to close it.",
		)
	}
	return strings.TrimSpace(
		"Completion finalization retry:\n" +
			"- A previous goal-continuation response stated that the objective was complete but did not produce an applied Goal completion command receipt.\n" +
			"- Load `goal-manager` and redo `audit_objective_alignment` in this round only through the host-injected `\"${NEXUS_COMMAND_PATH}\" --json goal contract|inspect|invoke` workflow; never use nexusctl or a standalone operation tool.\n" +
			"- Only after that command returns `aligned`, invoke `update_goal` with status \"complete\" before any final response. If it returns `not_aligned` or `inconclusive`, continue the work or gather the missing evidence.",
	)
}

func escapeGoalPromptText(input string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	).Replace(input)
}

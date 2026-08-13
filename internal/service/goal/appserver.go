// INPUT: Codex app-server Goal RPC 请求与 Nexus Goal service。
// OUTPUT: 线程 Goal 的读取、写入、清理和状态投影。
// POS: Codex app-server 协议到 Goal 领域的适配入口。
package goal

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalappserver "github.com/nexus-research-lab/nexus/internal/service/goal/appserver"
)

// SetFromThreadGoalParams 按 Codex app-server thread/goal/set 语义创建或更新当前 Goal。
func (s *Service) SetFromThreadGoalParams(ctx context.Context, request goalappserver.ThreadGoalSetParams) (*protocol.Goal, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	sessionKey, targetStatus, hasStatus, err := validateThreadGoalSetRequest(request)
	if err != nil {
		return nil, err
	}
	current, err := s.repo.GetCurrentGoal(ctx, sessionKey)
	if err != nil {
		return nil, err
	}
	if current == nil {
		ownerUserID, trustedAgentID, trustedAgentName, ownershipErr := s.verifyGoalSessionOwnership(
			ctx,
			sessionKey,
			request.OwnerUserID,
			"",
		)
		if ownershipErr != nil {
			return nil, ownershipErr
		}
		request.OwnerUserID = ownerUserID
		if err := s.preflightGoalCreate(sessionKey, ""); err != nil {
			return nil, err
		}
		created, createdEvent, err := s.createFromThreadGoalParams(
			ctx,
			sessionKey,
			targetStatus,
			hasStatus,
			request,
			trustedAgentID,
			trustedAgentName,
		)
		if err != nil {
			return nil, err
		}
		if protocol.NormalizeGoalStatus(created.Status) == protocol.GoalStatusComplete {
			s.updatePreviewFromGoal(ctx, *created, "")
			s.publishGoalEvent(ctx, *created, createdEvent)
			return s.FinalizeUsageForGoal(ctx, created.ID, protocol.GoalUsage{}, "")
		}
		if err := s.activateExternalGoalAccounting(ctx, *created); err != nil {
			return nil, s.rollbackFailedGoalCreate(ctx, *created, err)
		}
		s.updatePreviewFromGoal(ctx, *created, "")
		s.publishGoalEvent(ctx, *created, createdEvent)
		s.maybeDispatchActiveGoalContinuation(ctx, *created)
		return created, nil
	}
	current, err = s.authorizeGoalMutation(ctx, current, request.OwnerUserID)
	if err != nil {
		return nil, err
	}
	prepare := s.prepareExternalMutation
	if hasStatus && externalStatusUsesSettlementBoundary(protocol.GoalUpdateSourceExternal, targetStatus) {
		prepare = s.prepareExternalMutationAtSettlementBoundary
	}
	if err := prepare(ctx, current.ID); err != nil {
		return nil, err
	}
	refreshed, err := s.repo.GetGoal(ctx, current.ID)
	if err != nil {
		return nil, err
	}
	if refreshed == nil {
		return nil, ErrGoalNotFound
	}
	refreshed, err = s.authorizeGoalMutation(ctx, refreshed, request.OwnerUserID)
	if err != nil {
		return nil, err
	}
	updated, err := s.updateFromThreadGoalParams(ctx, *refreshed, targetStatus, hasStatus, request)
	if err != nil {
		return nil, err
	}
	s.maybeDispatchActiveGoalContinuation(ctx, *updated)
	return updated, nil
}

// ClearFromThreadGoalParams 按 Codex app-server thread/goal/clear 语义清除当前 Goal。
func (s *Service) ClearFromThreadGoalParams(ctx context.Context, request goalappserver.ThreadGoalClearParams) (bool, error) {
	if err := s.ensureEnabled(); err != nil {
		return false, err
	}
	sessionKey, err := protocol.RequireStructuredSessionKey(request.ThreadID)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrGoalInvalidInput, err)
	}
	current, err := s.repo.GetCurrentGoal(ctx, sessionKey)
	if err != nil {
		return false, err
	}
	if current == nil {
		return false, nil
	}
	current, err = s.authorizeGoalMutation(ctx, current, request.OwnerUserID)
	if err != nil {
		return false, err
	}
	if err = s.ensureGoalClearAllowed(ctx, *current); err != nil {
		return false, err
	}
	if err := s.prepareExternalMutationAtSettlementBoundary(ctx, current.ID); err != nil {
		return false, err
	}
	refreshed, err := s.repo.GetGoal(ctx, current.ID)
	if err != nil {
		return false, err
	}
	if refreshed == nil {
		return false, nil
	}
	refreshed, err = s.authorizeGoalMutation(ctx, refreshed, request.OwnerUserID)
	if err != nil {
		return false, err
	}
	deleted, err := s.clearGoal(ctx, *refreshed, protocol.GoalUpdateSourceExternal)
	if err != nil {
		return false, err
	}
	return deleted, nil
}

func (s *Service) createFromThreadGoalParams(
	ctx context.Context,
	sessionKey string,
	targetStatus protocol.GoalStatus,
	hasStatus bool,
	request goalappserver.ThreadGoalSetParams,
	trustedAgentID string,
	trustedAgentName string,
) (*protocol.Goal, protocol.GoalEvent, error) {
	if request.Objective == nil {
		return nil, protocol.GoalEvent{}, newGoalNotFoundError(fmt.Sprintf(
			"cannot update goal for thread %s: no goal exists",
			sessionKey,
		))
	}
	objective, err := normalizeObjective(*request.Objective)
	if err != nil {
		return nil, protocol.GoalEvent{}, err
	}
	objective, metadata := s.rewriteCreateObjective(ctx, protocol.CreateGoalRequest{
		SessionKey: sessionKey,
		Objective:  objective,
		CreatedBy:  "app_server",
		Metadata: map[string]any{
			"created_via": "thread_goal_set",
		},
	}, objective)
	metadata = initializeRoomGoalOwnershipMetadata(
		sessionKey,
		metadata,
		trustedAgentID,
		trustedAgentName,
		trustedAgentID,
		trustedAgentName,
	)
	metadata = initializeGoalOnlyExecutionMetadata(metadata)
	if ownerUserID := strings.TrimSpace(request.OwnerUserID); ownerUserID != "" {
		if metadata == nil {
			metadata = map[string]any{}
		}
		metadata[protocol.GoalMetadataOwnerUserID] = ownerUserID
	}
	tokenBudget, err := normalizeThreadGoalBudget(request.TokenBudget)
	if err != nil {
		return nil, protocol.GoalEvent{}, err
	}
	now := s.nowFn()
	status := statusAfterThreadGoalBudget(protocol.Goal{
		Status:      protocol.GoalStatusActive,
		TokenBudget: tokenBudget,
	}, targetStatus, hasStatus)
	item := protocol.Goal{
		ID:          s.idFactory("goal"),
		SessionKey:  sessionKey,
		Objective:   objective,
		Status:      status,
		TokenBudget: tokenBudget,
		Version:     1,
		CreatedBy:   "app_server",
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata:    metadata,
	}
	applyInitialGoalStatusTime(&item, now)
	createdEvent := s.newGoalEvent(
		item,
		"created",
		protocol.GoalUpdateSourceExternal,
		"",
		map[string]any{"objective": item.Objective},
		now,
	)
	created, err := s.repo.CreateGoalWithEvent(ctx, item, createdEvent)
	if err != nil {
		return nil, protocol.GoalEvent{}, s.classifyGoalCreateError(ctx, item, err)
	}
	return created, createdEvent, nil
}

func applyInitialGoalStatusTime(item *protocol.Goal, now time.Time) {
	switch protocol.NormalizeGoalStatus(item.Status) {
	case protocol.GoalStatusComplete:
		item.CompletedAt = &now
	case protocol.GoalStatusBlocked:
		item.BlockedAt = &now
	}
}

func (s *Service) updateFromThreadGoalParams(
	ctx context.Context,
	item protocol.Goal,
	targetStatus protocol.GoalStatus,
	hasStatus bool,
	request goalappserver.ThreadGoalSetParams,
) (*protocol.Goal, error) {
	managedBinding := false
	if request.Objective != nil {
		var bindingErr error
		managedBinding, bindingErr = s.goalHasManagedExecutionBinding(ctx, item)
		if bindingErr != nil {
			return nil, bindingErr
		}
	}
	if request.Objective != nil && managedBinding {
		requestedObjective, err := normalizeObjective(*request.Objective)
		if err != nil {
			return nil, err
		}
		if objectiveRetargetRequestAlreadyApplied(item, requestedObjective) {
			request.Objective = nil
			if !request.TokenBudget.Present && !hasStatus {
				return &item, nil
			}
		} else {
			objective, _ := s.rewriteUpdateObjective(
				ctx,
				protocol.UpdateGoalRequest{Objective: &requestedObjective},
				item.SessionKey,
				requestedObjective,
				nil,
			)
			if item.Objective != objective && s.objectiveRetarget != nil {
				retargeted, retargetErr := s.objectiveRetarget.RetargetGoalObjective(ctx, ObjectiveRetargetCommand{
					Goal:                      item,
					RequestedObjective:        requestedObjective,
					Objective:                 objective,
					Reason:                    "app-server updated the Goal objective",
					ExpectedObjectiveRevision: item.ObjectiveRevision(),
					Source:                    protocol.GoalUpdateSourceExternal,
					OwnerUserID:               strings.TrimSpace(request.OwnerUserID),
				})
				if retargetErr != nil {
					return nil, retargetErr
				}
				item = *retargeted
				request.Objective = nil
				if !request.TokenBudget.Present && !hasStatus {
					return &item, nil
				}
			} else if item.Objective != objective {
				return nil, fmt.Errorf(
					"%w: Goal objective retarget coordinator is unavailable for a managed Execution",
					ErrGoalInvalidState,
				)
			}
		}
	}
	hasUpdateFields := false
	valueChanged := false
	payload := map[string]any{}
	if request.Objective != nil {
		hasUpdateFields = true
		objective, err := normalizeObjective(*request.Objective)
		if err != nil {
			return nil, err
		}
		objective, payload = s.rewriteUpdateObjective(ctx, protocol.UpdateGoalRequest{Objective: &objective}, item.SessionKey, objective, payload)
		if item.Objective != objective {
			item.Objective = objective
			advanceObjectiveRevision(&item)
			resetGoalContinuationForObjectiveReplacement(&item)
			valueChanged = true
			payload["objective_updated"] = true
			payload["objective_revision"] = item.ObjectiveRevision()
		}
	}
	if request.TokenBudget.Present {
		hasUpdateFields = true
		tokenBudget, err := normalizeThreadGoalBudget(request.TokenBudget)
		if err != nil {
			return nil, err
		}
		if !goalTokenBudgetEqual(item.TokenBudget, tokenBudget) {
			item.TokenBudget = tokenBudget
			valueChanged = true
			if tokenBudget != nil {
				payload["token_budget"] = *tokenBudget
			} else {
				payload["token_budget"] = nil
			}
		}
	}
	currentStatus := protocol.NormalizeGoalStatus(item.Status)
	nextStatus := currentStatus
	if hasStatus {
		hasUpdateFields = true
		nextStatus = targetStatus
	}
	nextStatus = statusAfterThreadGoalBudget(item, nextStatus, hasStatus)
	if nextStatus == protocol.GoalStatusComplete &&
		currentStatus != protocol.GoalStatusComplete {
		if _, readinessErr := s.ensureRoomGoalCompletionReady(ctx, item, "", ""); readinessErr != nil {
			return nil, readinessErr
		}
		if readinessErr := s.ensureExecutionGoalCompletionReady(ctx, item); readinessErr != nil {
			return nil, readinessErr
		}
	}
	if !hasUpdateFields {
		return &item, nil
	}
	eventType := "updated"
	if hasStatus && !valueChanged && nextStatus != currentStatus {
		eventType = threadGoalStatusEventType(nextStatus)
	}
	updated, err := s.persistThreadGoalSetTransition(ctx, item, nextStatus, eventType, payload)
	if err != nil {
		return nil, err
	}
	if request.Objective != nil {
		s.updatePreviewFromGoal(ctx, *updated, "")
	}
	return updated, nil
}

func (s *Service) persistThreadGoalSetTransition(
	ctx context.Context,
	item protocol.Goal,
	status protocol.GoalStatus,
	eventType string,
	payload map[string]any,
) (*protocol.Goal, error) {
	return s.persistTransitionWithOptions(
		ctx,
		item,
		status,
		protocol.GoalUpdateSourceExternal,
		eventType,
		"",
		payload,
		transitionOptions{persistBudgetLimitedStopRequest: true},
	)
}

func validateThreadGoalSetRequest(request goalappserver.ThreadGoalSetParams) (string, protocol.GoalStatus, bool, error) {
	sessionKey, err := protocol.RequireStructuredSessionKey(request.ThreadID)
	if err != nil {
		return "", "", false, fmt.Errorf("%w: %v", ErrGoalInvalidInput, err)
	}
	if request.Status == nil {
		return sessionKey, protocol.GoalStatusActive, false, nil
	}
	status, ok := goalappserver.GoalStatusFromThreadGoalStatus(*request.Status)
	if !ok {
		return "", "", false, ErrGoalInvalidInput
	}
	return sessionKey, status, true, nil
}

func normalizeThreadGoalBudget(input protocol.OptionalInt64) (*int64, error) {
	if !input.Present {
		return nil, nil
	}
	return normalizeUpdateBudget(input.Value)
}

func statusAfterThreadGoalBudget(item protocol.Goal, status protocol.GoalStatus, explicitStatus bool) protocol.GoalStatus {
	status = protocol.NormalizeGoalStatus(status)
	currentStatus := protocol.NormalizeGoalStatus(item.Status)
	if currentStatus == protocol.GoalStatusBudgetLimited &&
		(status == protocol.GoalStatusPaused || status == protocol.GoalStatusBlocked) {
		return protocol.GoalStatusBudgetLimited
	}
	if status == protocol.GoalStatusActive && item.TokenBudget != nil && item.Usage.BudgetTokens() >= *item.TokenBudget {
		return protocol.GoalStatusBudgetLimited
	}
	if !explicitStatus && currentStatus == protocol.GoalStatusActive && item.TokenBudget != nil && item.Usage.BudgetTokens() >= *item.TokenBudget {
		return protocol.GoalStatusBudgetLimited
	}
	return status
}

func threadGoalStatusEventType(status protocol.GoalStatus) string {
	switch protocol.NormalizeGoalStatus(status) {
	case protocol.GoalStatusPaused:
		return "paused"
	case protocol.GoalStatusComplete:
		return "completed"
	case protocol.GoalStatusBlocked:
		return "blocked"
	case protocol.GoalStatusBudgetLimited:
		return "budget_limited"
	case protocol.GoalStatusUsageLimited:
		return "usage_limited"
	default:
		return "resumed"
	}
}

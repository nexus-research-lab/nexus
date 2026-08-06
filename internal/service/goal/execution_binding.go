// INPUT: active explicit Goal、持久化或可确定性恢复的预留 Execution identity、objective revision 与 completion criteria。
// OUTPUT: 幂等 CAS 持久化的 Goal -> Execution metadata 及 execution_bound 审计事件。
// POS: Goal 侧反向 binding 真相入口；Execution aggregate 的正向 binding 由 orchestration.BindGoal 管理。
package goal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ExplicitExecutionBinding 描述 explicit Goal 预留或确认的 Execution identity。
type ExplicitExecutionBinding struct {
	GoalID                    string
	ExpectedObjectiveRevision int64
	ExecutionID               string
	CompletionCriteria        []string
	RoundID                   string
}

// BindExplicitExecution 给 explicit Goal 原子写入反向 Execution binding。
//
// 同一个 binding 重试返回当前 Goal；不同 Execution、provenance 或 completion
// criteria 不能覆盖已有 metadata，必须由更高层显式处理 recovery/retarget。
func (s *Service) BindExplicitExecution(
	ctx context.Context,
	binding ExplicitExecutionBinding,
) (*protocol.Goal, error) {
	goalID := strings.TrimSpace(binding.GoalID)
	executionID := strings.TrimSpace(binding.ExecutionID)
	criteria := normalizeExecutionCompletionCriteria(binding.CompletionCriteria)
	if goalID == "" || executionID == "" || binding.ExpectedObjectiveRevision <= 0 {
		return nil, newGoalInvalidInputError(
			"goal id, execution id and expected objective revision are required for explicit binding",
		)
	}
	item, err := s.loadMutableGoal(ctx, goalID)
	if err != nil {
		return nil, err
	}
	return s.retryGoalMutation(ctx, item, func(current *protocol.Goal) (*protocol.Goal, error) {
		if !objectiveRevisionMatches(*current, binding.ExpectedObjectiveRevision) {
			return nil, ErrGoalRevisionStale
		}
		activationOrigin := protocol.GoalActivationOrigin(protocol.GoalMetadataString(
			current.Metadata,
			protocol.GoalMetadataActivationOrigin,
		))
		activationReason := protocol.GoalActivationReason(protocol.GoalMetadataString(
			current.Metadata,
			protocol.GoalMetadataActivationReason,
		))
		if !managedGoalActivationOrigin(activationOrigin) || activationReason == "" {
			return nil, fmt.Errorf(
				"%w: Goal does not carry managed Execution activation provenance",
				ErrGoalExecutionBindingConflict,
			)
		}
		existingExecutionID := protocol.GoalReservedExecutionID(*current)
		if existingExecutionID != "" && existingExecutionID != executionID {
			return nil, fmt.Errorf(
				"%w: Goal is already reserved for Execution %s",
				ErrGoalExecutionBindingConflict,
				existingExecutionID,
			)
		}
		existingCriteria := goalMetadataStrings(
			current.Metadata,
			protocol.GoalMetadataCompletionCriteria,
		)
		if _, present := current.Metadata[protocol.GoalMetadataCompletionCriteria]; present &&
			existingCriteria == nil {
			return nil, fmt.Errorf(
				"%w: stored completion criteria are malformed",
				ErrGoalExecutionBindingConflict,
			)
		}
		if len(existingCriteria) > 0 && !slices.Equal(existingCriteria, criteria) {
			return nil, fmt.Errorf(
				"%w: completion criteria differ from the reserved Execution",
				ErrGoalExecutionBindingConflict,
			)
		}
		transition, transitioning := ObjectiveTransitionFromGoal(*current)
		if transitioning &&
			(transition.SuccessorExecutionID != executionID ||
				transition.NewRevision != binding.ExpectedObjectiveRevision) {
			return nil, fmt.Errorf(
				"%w: Goal objective transition reserved another successor",
				ErrGoalExecutionBindingConflict,
			)
		}
		if existingExecutionID == executionID &&
			(len(existingCriteria) > 0 || len(criteria) == 0) &&
			(!transitioning || transition.Phase == ObjectiveTransitionBindingReserved ||
				transition.Phase == ObjectiveTransitionBound) {
			return current, nil
		}

		expectedVersion := current.Version
		current.Metadata = cloneMap(current.Metadata)
		if current.Metadata == nil {
			current.Metadata = map[string]any{}
		}
		// Binding or first materialization of completion criteria changes the
		// authoritative audit target. A report produced before this mutation
		// must never authorize completion.
		delete(current.Metadata, protocol.GoalMetadataObjectiveAlignment)
		current.Metadata[protocol.GoalMetadataExecutionID] = executionID
		if len(criteria) == 0 {
			delete(current.Metadata, protocol.GoalMetadataCompletionCriteria)
		} else {
			current.Metadata[protocol.GoalMetadataCompletionCriteria] = append([]string(nil), criteria...)
		}
		if transitioning {
			if transition.Phase != ObjectiveTransitionAwaitingPlan &&
				transition.Phase != ObjectiveTransitionBindingReserved {
				return nil, fmt.Errorf(
					"%w: Goal objective transition cannot reserve a binding from phase %s",
					ErrGoalExecutionBindingConflict,
					transition.Phase,
				)
			}
			transition.Phase = ObjectiveTransitionBindingReserved
			current.Metadata[protocol.GoalMetadataObjectiveTransition] = objectiveTransitionMetadata(transition)
		}
		current.Version++
		current.UpdatedAt = s.nowFn()
		updated, updateErr := s.repo.UpdateGoal(ctx, *current, expectedVersion)
		if errors.Is(updateErr, sql.ErrNoRows) {
			return nil, ErrGoalVersionStale
		}
		if updateErr != nil {
			return nil, updateErr
		}
		if eventErr := s.appendEvent(
			ctx,
			*updated,
			"execution_bound",
			protocol.GoalUpdateSourceSystem,
			strings.TrimSpace(binding.RoundID),
			map[string]any{
				"execution_id":      executionID,
				"activation_origin": string(activationOrigin),
			},
		); eventErr != nil {
			return nil, eventErr
		}
		return updated, nil
	})
}

func managedGoalActivationOrigin(origin protocol.GoalActivationOrigin) bool {
	switch origin {
	case protocol.GoalActivationOriginUserExplicit,
		protocol.GoalActivationOriginAdaptiveInitial,
		protocol.GoalActivationOriginAdaptivePromoted:
		return true
	default:
		return false
	}
}

func normalizeExecutionCompletionCriteria(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func goalMetadataStrings(metadata map[string]any, key string) []string {
	value, ok := metadata[strings.TrimSpace(key)]
	if !ok {
		return nil
	}
	var raw []any
	switch typed := value.(type) {
	case []string:
		return normalizeExecutionCompletionCriteria(typed)
	case []any:
		raw = typed
	default:
		return nil
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		if text, ok := item.(string); ok {
			result = append(result, text)
		}
	}
	return normalizeExecutionCompletionCriteria(result)
}

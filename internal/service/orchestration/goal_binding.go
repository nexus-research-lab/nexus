// INPUT: Goal-side reserved/pending/confirmed metadata and durable Execution rows.
// OUTPUT: One exact Goal-Execution binding classification shared by completion, retarget and continuation gates.
// POS: Goal lifecycle must consume this database-backed truth instead of inferring a WorkGraph from provenance.
package orchestration

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type goalRevisionExecutionRepository interface {
	FindByGoalRevision(context.Context, string, int64) (*protocol.Execution, error)
}

// ResolveGoalExecutionBinding classifies the current Goal revision without
// treating its stable future Execution reservation as a materialized WorkGraph.
func (s *Service) ResolveGoalExecutionBinding(
	ctx context.Context,
	goal protocol.Goal,
) (protocol.GoalExecutionBindingResolution, error) {
	resolution := protocol.GoalExecutionBindingResolution{
		State:               protocol.GoalExecutionBindingStateStandalone,
		ReservedExecutionID: protocol.GoalReservedExecutionID(goal),
	}
	if s == nil || s.repository == nil || strings.TrimSpace(goal.ID) == "" {
		return resolution, nil
	}

	persistedState := protocol.GoalExecutionBindingStateFromGoal(goal)
	var execution *protocol.Execution
	var err error
	if resolution.ReservedExecutionID != "" {
		execution, err = s.repository.Get(ctx, resolution.ReservedExecutionID)
		if err != nil {
			return resolution, err
		}
	}
	if execution == nil {
		if repository, ok := s.repository.(goalRevisionExecutionRepository); ok {
			execution, err = repository.FindByGoalRevision(
				ctx,
				strings.TrimSpace(goal.ID),
				goal.ObjectiveRevision(),
			)
		} else {
			execution, err = s.repository.FindCurrentByGoal(
				ctx,
				strings.TrimSpace(goal.ID),
				goal.ObjectiveRevision(),
			)
		}
		if err != nil {
			return resolution, err
		}
	}
	if execution == nil {
		switch persistedState {
		case protocol.GoalExecutionBindingStatePending:
			resolution.State = protocol.GoalExecutionBindingStatePending
		case protocol.GoalExecutionBindingStateConfirmed:
			resolution.State = protocol.GoalExecutionBindingStateConflict
		case protocol.GoalExecutionBindingStateReserved:
			resolution.State = protocol.GoalExecutionBindingStateReserved
		case protocol.GoalExecutionBindingStateStandalone:
			if resolution.ReservedExecutionID != "" {
				resolution.State = protocol.GoalExecutionBindingStateReserved
			}
		case protocol.GoalExecutionBindingStateConflict:
			resolution.State = protocol.GoalExecutionBindingStateConflict
		default:
			resolution.State = protocol.GoalExecutionBindingStateConflict
		}
		return resolution, nil
	}

	resolution.ExecutionID = strings.TrimSpace(execution.ID)
	if !goalExecutionBindingMatches(goal, resolution.ReservedExecutionID, *execution) {
		resolution.State = protocol.GoalExecutionBindingStateConflict
		return resolution, nil
	}
	if _, snapshotErr := s.repository.GetSnapshot(ctx, execution.ID); snapshotErr != nil {
		return resolution, snapshotErr
	}

	switch persistedState {
	case protocol.GoalExecutionBindingStatePending:
		resolution.State = protocol.GoalExecutionBindingStatePending
	case protocol.GoalExecutionBindingStateConfirmed:
		resolution.State = protocol.GoalExecutionBindingStateConfirmed
	case protocol.GoalExecutionBindingStateReserved,
		protocol.GoalExecutionBindingStateStandalone:
		// Legacy releases persisted the exact reverse id and SQL forward binding
		// without a separate confirmation marker. Exact bilateral identity is a
		// safe compatibility proof; future writes always persist Confirmed.
		resolution.State = protocol.GoalExecutionBindingStateConfirmed
	default:
		resolution.State = protocol.GoalExecutionBindingStateConflict
	}
	return resolution, nil
}

func goalExecutionBindingMatches(
	goal protocol.Goal,
	reservedExecutionID string,
	execution protocol.Execution,
) bool {
	if strings.TrimSpace(execution.ID) == "" ||
		strings.TrimSpace(execution.GoalID) != strings.TrimSpace(goal.ID) ||
		execution.GoalObjectiveRevision != goal.ObjectiveRevision() ||
		strings.TrimSpace(execution.SessionKey) != strings.TrimSpace(goal.SessionKey) ||
		(reservedExecutionID != "" && strings.TrimSpace(execution.ID) != reservedExecutionID) {
		return false
	}
	ownerUserID := protocol.GoalMetadataString(goal.Metadata, protocol.GoalMetadataOwnerUserID)
	if ownerUserID != "" && strings.TrimSpace(execution.OwnerUserID) != ownerUserID {
		return false
	}
	parsed := protocol.ParseSessionKey(goal.SessionKey)
	switch parsed.Kind {
	case protocol.SessionKeyKindRoom:
		return execution.ScopeKind == protocol.ExecutionScopeRoom &&
			strings.TrimSpace(execution.ConversationID) == strings.TrimSpace(parsed.ConversationID)
	default:
		return execution.ScopeKind == protocol.ExecutionScopeDM
	}
}

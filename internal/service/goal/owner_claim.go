// INPUT: ownerless legacy Goal、authenticated owner、owner-scoped session proof 与中央 Goal/Execution binding resolver。
// OUTPUT: CAS 持久化的 owner provenance；confirmed binding 额外要求 exact Execution owner/revision proof。
// POS: owner-scoped Goal transport 的唯一 legacy compatibility boundary；证明不完整时 fail closed。
package goal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type executionGoalOwnerVerifier interface {
	ValidateGoalRevisionOwner(
		context.Context,
		string,
		string,
		int64,
		string,
	) (bool, error)
}

// authorizeOwnerScopedGoal verifies an existing owner or performs the one-time
// provenance claim required by ownerless rows written before owner_user_id was
// durable. Callers must use the returned value because a successful claim
// advances the Goal version.
func (s *Service) authorizeOwnerScopedGoal(
	ctx context.Context,
	item *protocol.Goal,
	requestedOwnerUserID string,
) (*protocol.Goal, error) {
	ownerUserID := strings.TrimSpace(requestedOwnerUserID)
	if item == nil || ownerUserID == "" {
		return nil, fmt.Errorf("%w: complete Goal owner provenance is required", ErrGoalForbidden)
	}
	if authenticatedOwnerUserID, ok := authctx.CurrentUserID(ctx); ok &&
		strings.TrimSpace(authenticatedOwnerUserID) != ownerUserID {
		return nil, fmt.Errorf("%w: Goal owner does not match the authenticated owner", ErrGoalForbidden)
	}
	current := item
	for attempt := 0; attempt < goalUpdateMaxAttempts; attempt++ {
		storedOwnerUserID := protocol.GoalMetadataString(
			current.Metadata,
			protocol.GoalMetadataOwnerUserID,
		)
		if storedOwnerUserID != "" {
			if storedOwnerUserID != ownerUserID {
				return nil, fmt.Errorf("%w: Goal belongs to another owner", ErrGoalForbidden)
			}
			return current, nil
		}
		if err := s.verifyLegacyGoalOwnerClaim(ctx, *current, ownerUserID); err != nil {
			return nil, err
		}

		expectedVersion := current.Version
		candidate := *current
		candidate.Metadata = cloneMap(current.Metadata)
		if candidate.Metadata == nil {
			candidate.Metadata = map[string]any{}
		}
		candidate.Metadata[protocol.GoalMetadataOwnerUserID] = ownerUserID
		candidate.Version++
		candidate.UpdatedAt = s.nowFn().UTC()
		updated, err := s.repo.UpdateGoal(ctx, candidate, expectedVersion)
		if err == nil {
			return updated, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		current, err = s.repo.GetGoal(ctx, current.ID)
		if err != nil {
			return nil, err
		}
		if current == nil {
			return nil, ErrGoalNotFound
		}
	}
	return nil, ErrGoalVersionStale
}

// authorizeGoalMutation preserves the unscoped in-process API used by trusted
// runtime flows and focused domain tests. Every HTTP/app-server entry supplies
// an owner and therefore must pass the owner-scoped claim boundary above.
func (s *Service) authorizeGoalMutation(
	ctx context.Context,
	item *protocol.Goal,
	requestedOwnerUserID string,
) (*protocol.Goal, error) {
	if strings.TrimSpace(requestedOwnerUserID) == "" {
		if item == nil {
			return nil, ErrGoalNotFound
		}
		if err := authorizeGoalOwner(*item, ""); err != nil {
			return nil, err
		}
		return item, nil
	}
	return s.authorizeOwnerScopedGoal(ctx, item, requestedOwnerUserID)
}

func (s *Service) verifyLegacyGoalOwnerClaim(
	ctx context.Context,
	item protocol.Goal,
	ownerUserID string,
) error {
	if s.sessionOwnership == nil {
		return fmt.Errorf("%w: Goal session ownership proof is unavailable", ErrGoalForbidden)
	}
	verifiedOwnerUserID, _, _, err := s.verifyGoalSessionOwnership(
		ctx,
		item.SessionKey,
		ownerUserID,
		"",
	)
	if err != nil {
		return err
	}
	if verifiedOwnerUserID != ownerUserID {
		return fmt.Errorf("%w: Goal session owner proof does not match", ErrGoalForbidden)
	}

	resolver, ok := s.executionCompletion.(executionGoalBindingResolver)
	if !ok {
		return fmt.Errorf("%w: Goal Execution binding proof is unavailable", ErrGoalForbidden)
	}
	resolution, err := resolver.ResolveGoalExecutionBinding(ctx, item)
	if err != nil {
		return fmt.Errorf("resolve legacy Goal Execution binding: %w", err)
	}
	switch resolution.State {
	case protocol.GoalExecutionBindingStateStandalone,
		protocol.GoalExecutionBindingStateReserved:
		return nil
	case protocol.GoalExecutionBindingStateConfirmed:
		if strings.TrimSpace(resolution.ExecutionID) == "" {
			return fmt.Errorf("%w: confirmed Goal Execution identity is missing", ErrGoalForbidden)
		}
		verifier, verifierOK := s.executionCompletion.(executionGoalOwnerVerifier)
		if !verifierOK {
			return fmt.Errorf("%w: Goal Execution owner proof is unavailable", ErrGoalForbidden)
		}
		matches, verifyErr := verifier.ValidateGoalRevisionOwner(
			ctx,
			resolution.ExecutionID,
			item.ID,
			item.ObjectiveRevision(),
			ownerUserID,
		)
		if verifyErr != nil {
			return fmt.Errorf("verify legacy Goal Execution owner: %w", verifyErr)
		}
		if !matches {
			return fmt.Errorf("%w: Goal Execution owner provenance does not match", ErrGoalForbidden)
		}
		return nil
	case protocol.GoalExecutionBindingStatePending,
		protocol.GoalExecutionBindingStateConflict:
		return fmt.Errorf(
			"%w: Goal Execution binding is %s",
			ErrGoalForbidden,
			resolution.State,
		)
	default:
		return fmt.Errorf("%w: Goal Execution binding state is unknown", ErrGoalForbidden)
	}
}

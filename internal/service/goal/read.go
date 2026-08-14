// INPUT: host-trusted Goal ID.
// OUTPUT: the canonical durable Goal, including terminal state and objective revision.
// POS: narrow cross-conversation collaboration admission read; no mutation or owner claim.
package goal

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// GoalByIDForOwner returns the canonical Goal selected by its host-minted
// identity only when its durable owner provenance matches the target Room.
// It exists for internal durable handoff recovery; user-facing reads remain at
// their transport boundary and ownerless legacy Goals fail closed here.
func (s *Service) GoalByIDForOwner(
	ctx context.Context,
	goalID string,
	ownerUserID string,
) (*protocol.Goal, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	goalID = strings.TrimSpace(goalID)
	ownerUserID = strings.TrimSpace(ownerUserID)
	if goalID == "" || ownerUserID == "" {
		return nil, ErrGoalInvalidInput
	}
	item, err := s.repo.GetGoal(ctx, goalID)
	if err != nil || item == nil {
		return item, err
	}
	if storedOwner := protocol.GoalMetadataString(
		item.Metadata,
		protocol.GoalMetadataOwnerUserID,
	); storedOwner == "" || storedOwner != ownerUserID {
		return nil, fmt.Errorf("%w: Goal owner provenance does not match Room handoff", ErrGoalForbidden)
	}
	return item, nil
}

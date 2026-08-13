// INPUT: authenticated owner, Goal ID and the injected database-backed Goal/Execution binding resolver.
// OUTPUT: one non-persisting standalone/reserved/pending/confirmed/conflict classification.
// POS: owner-scoped HTTP binding read boundary; client metadata and Goal-only fallback classification are forbidden here.
package goal

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ExecutionBindingForOwner resolves the current Goal revision from durable
// Goal and Execution truth without mutating either side. Production wiring
// must provide the orchestration resolver; this public read never falls back
// to interpreting Goal metadata in isolation.
func (s *Service) ExecutionBindingForOwner(
	ctx context.Context,
	goalID string,
	ownerUserID string,
) (protocol.GoalExecutionBindingResolution, error) {
	if err := s.ensureEnabled(); err != nil {
		return protocol.GoalExecutionBindingResolution{}, err
	}
	goalID = strings.TrimSpace(goalID)
	if goalID == "" {
		return protocol.GoalExecutionBindingResolution{}, ErrGoalInvalidInput
	}
	item, err := s.repo.GetGoal(ctx, goalID)
	if err != nil {
		return protocol.GoalExecutionBindingResolution{}, err
	}
	if item == nil {
		return protocol.GoalExecutionBindingResolution{}, ErrGoalNotFound
	}
	if err := authorizeGoalReader(*item, ownerUserID); err != nil {
		return protocol.GoalExecutionBindingResolution{}, err
	}
	resolver, ok := s.executionCompletion.(executionGoalBindingResolver)
	if !ok {
		return protocol.GoalExecutionBindingResolution{}, fmt.Errorf(
			"Goal Execution binding resolver is unavailable",
		)
	}
	return resolver.ResolveGoalExecutionBinding(ctx, *item)
}

// INPUT: trusted objective-retarget intent, Goal objective revision fence and reserved successor Execution identity.
// OUTPUT: durable prepare/awaiting_plan/binding_reserved/bound transition metadata、reserved/pending/confirmed Execution binding state 与 replay-safe phase transitions.
// POS: Goal-side half of the cross-service Goal objective revision / Execution rebase saga.
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

// ObjectiveTransitionPhase is the durable phase of one Goal objective rebase.
type ObjectiveTransitionPhase string

const (
	ObjectiveTransitionPrepared        ObjectiveTransitionPhase = "prepared"
	ObjectiveTransitionAwaitingPlan    ObjectiveTransitionPhase = "awaiting_plan"
	ObjectiveTransitionBindingReserved ObjectiveTransitionPhase = "binding_reserved"
	ObjectiveTransitionBound           ObjectiveTransitionPhase = "bound"
)

// ObjectiveTransition is persisted in server-owned Goal metadata. The
// successor identity is reserved before the old WorkGraph is terminalized so
// every retry converges on one Execution.
type ObjectiveTransition struct {
	ID                   string
	CommandID            string
	Phase                ObjectiveTransitionPhase
	OldRevision          int64
	NewRevision          int64
	OldExecutionID       string
	OldExecutionFenced   bool
	SuccessorExecutionID string
	RequestedObjective   string
	TargetObjective      string
	Reason               string
	Source               protocol.GoalUpdateSource
}

// ObjectiveRetargetCommand is emitted by all model, HTTP and app-server
// objective mutation paths after source-specific authorization.
type ObjectiveRetargetCommand struct {
	Goal                      protocol.Goal
	RequestedObjective        string
	Objective                 string
	Reason                    string
	CommandID                 string
	TransitionID              string
	SuccessorExecutionID      string
	ExpectedObjectiveRevision int64
	Source                    protocol.GoalUpdateSource
	RoundID                   string
	AgentID                   string
	OwnerUserID               string
}

// ObjectiveRetargetCoordinator owns the cross-service saga. Goal service
// supplies durable phase primitives but never reaches into Execution storage.
type ObjectiveRetargetCoordinator interface {
	RetargetGoalObjective(context.Context, ObjectiveRetargetCommand) (*protocol.Goal, error)
}

// SetObjectiveRetargetCoordinator installs the sole application-level objective
// revision coordinator. Bound Goals fail closed if this coordinator is absent.
func (s *Service) SetObjectiveRetargetCoordinator(coordinator ObjectiveRetargetCoordinator) {
	if s != nil {
		s.objectiveRetarget = coordinator
	}
}

// ObjectiveTransitionFromGoal returns the current durable transition, if any.
func ObjectiveTransitionFromGoal(item protocol.Goal) (ObjectiveTransition, bool) {
	if item.Metadata == nil {
		return ObjectiveTransition{}, false
	}
	raw, ok := item.Metadata[protocol.GoalMetadataObjectiveTransition]
	if !ok {
		return ObjectiveTransition{}, false
	}
	value, ok := raw.(map[string]any)
	if !ok {
		return ObjectiveTransition{}, false
	}
	transition := ObjectiveTransition{
		ID:                   metadataString(value, "transition_id"),
		CommandID:            metadataString(value, "command_id"),
		Phase:                ObjectiveTransitionPhase(metadataString(value, "phase")),
		OldRevision:          protocol.GoalMetadataInt64(value, "old_revision"),
		NewRevision:          protocol.GoalMetadataInt64(value, "new_revision"),
		OldExecutionID:       metadataString(value, "old_execution_id"),
		OldExecutionFenced:   protocol.GoalMetadataBool(value, "old_execution_fenced"),
		SuccessorExecutionID: metadataString(value, "successor_execution_id"),
		RequestedObjective:   metadataString(value, "requested_objective"),
		TargetObjective:      metadataString(value, "target_objective"),
		Reason:               metadataString(value, "reason"),
		Source:               protocol.GoalUpdateSource(metadataString(value, "source")),
	}
	if transition.RequestedObjective == "" {
		transition.RequestedObjective = transition.TargetObjective
	}
	if transition.ID == "" || transition.CommandID == "" ||
		transition.OldRevision <= 0 || transition.NewRevision != transition.OldRevision+1 ||
		transition.SuccessorExecutionID == "" || transition.TargetObjective == "" ||
		!validObjectiveTransitionPhase(transition.Phase) {
		return ObjectiveTransition{}, false
	}
	return transition, true
}

// GoalObjectiveTransitionPending reports whether completion and automatic
// continuation must remain fail-closed.
func GoalObjectiveTransitionPending(item protocol.Goal) bool {
	if item.Metadata == nil {
		return false
	}
	if _, exists := item.Metadata[protocol.GoalMetadataObjectiveTransition]; !exists {
		return false
	}
	transition, ok := ObjectiveTransitionFromGoal(item)
	return !ok || transition.Phase != ObjectiveTransitionBound
}

func objectiveTransitionAwaitingPlan(
	item protocol.Goal,
) (ObjectiveTransition, bool) {
	transition, ok := ObjectiveTransitionFromGoal(item)
	return transition, ok &&
		transition.Phase == ObjectiveTransitionAwaitingPlan &&
		item.ObjectiveRevision() == transition.NewRevision &&
		strings.TrimSpace(item.Objective) == transition.TargetObjective
}

func rejectPendingObjectiveTransition(item protocol.Goal, operation string) error {
	if !GoalObjectiveTransitionPending(item) {
		return nil
	}
	transition, ok := ObjectiveTransitionFromGoal(item)
	if !ok {
		return fmt.Errorf(
			"%w: cannot %s while Goal objective transition metadata is malformed",
			ErrGoalInvalidState,
			strings.TrimSpace(operation),
		)
	}
	return fmt.Errorf(
		"%w: cannot %s while Goal objective transition %s is %s",
		ErrGoalInvalidState,
		strings.TrimSpace(operation),
		transition.ID,
		transition.Phase,
	)
}

// PrepareObjectiveRetarget persists the durable intent while leaving the
// canonical objective/revision untouched. Retries with the same transition are
// no-ops; another in-flight transition conflicts.
func (s *Service) PrepareObjectiveRetarget(
	ctx context.Context,
	command ObjectiveRetargetCommand,
) (*protocol.Goal, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	if err := validateObjectiveRetargetCommand(command); err != nil {
		return nil, err
	}
	requestedObjective := objectiveRetargetRequestedObjective(command)
	item, err := s.loadMutableGoal(ctx, command.Goal.ID)
	if err != nil {
		return nil, err
	}
	return s.retryGoalMutation(ctx, item, func(current *protocol.Goal) (*protocol.Goal, error) {
		if transition, ok := ObjectiveTransitionFromGoal(*current); ok {
			if transition.ID == command.TransitionID &&
				transition.CommandID == command.CommandID &&
				transition.RequestedObjective == requestedObjective &&
				transition.SuccessorExecutionID == command.SuccessorExecutionID {
				return current, nil
			}
			if transition.Phase != ObjectiveTransitionBound {
				return nil, fmt.Errorf("%w: another Goal objective transition is in progress", ErrGoalConflict)
			}
		}
		if current.ObjectiveRevision() != command.ExpectedObjectiveRevision {
			return nil, ErrGoalRevisionStale
		}
		if strings.TrimSpace(current.Objective) == command.Objective {
			return current, nil
		}
		expectedVersion := current.Version
		current.Metadata = clearContinuationReservations(cloneMap(current.Metadata))
		if current.Metadata == nil {
			current.Metadata = map[string]any{}
		}
		current.Metadata[protocol.GoalMetadataObjectiveTransition] = objectiveTransitionMetadata(
			ObjectiveTransition{
				ID:                   command.TransitionID,
				CommandID:            command.CommandID,
				Phase:                ObjectiveTransitionPrepared,
				OldRevision:          command.ExpectedObjectiveRevision,
				NewRevision:          command.ExpectedObjectiveRevision + 1,
				OldExecutionID:       protocol.GoalReservedExecutionID(*current),
				SuccessorExecutionID: command.SuccessorExecutionID,
				RequestedObjective:   requestedObjective,
				TargetObjective:      command.Objective,
				Reason:               command.Reason,
				Source:               command.Source,
			},
		)
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
			"objective_retarget_prepared",
			command.Source,
			strings.TrimSpace(command.RoundID),
			map[string]any{
				"transition_id":          command.TransitionID,
				"old_objective_revision": command.ExpectedObjectiveRevision,
				"new_objective_revision": command.ExpectedObjectiveRevision + 1,
				"successor_execution_id": command.SuccessorExecutionID,
			},
		); eventErr != nil {
			return nil, eventErr
		}
		return updated, nil
	})
}

// FenceObjectiveRetargetPredecessor records that the old Goal-side Execution
// reservation never materialized and has been durably rejected by SQL. The
// audit identity remains in OldExecutionID, but the successor must not use it
// as a replaces_execution_id predecessor.
func (s *Service) FenceObjectiveRetargetPredecessor(
	ctx context.Context,
	goalID string,
	transitionID string,
	executionID string,
) (*protocol.Goal, error) {
	item, err := s.loadMutableGoal(ctx, goalID)
	if err != nil {
		return nil, err
	}
	transitionID = strings.TrimSpace(transitionID)
	executionID = strings.TrimSpace(executionID)
	return s.retryGoalMutation(ctx, item, func(current *protocol.Goal) (*protocol.Goal, error) {
		transition, ok := ObjectiveTransitionFromGoal(*current)
		if !ok || transition.ID != transitionID ||
			transition.OldExecutionID != executionID ||
			transition.Phase != ObjectiveTransitionPrepared {
			return nil, fmt.Errorf(
				"%w: Goal objective predecessor fence does not match prepared transition",
				ErrGoalRevisionStale,
			)
		}
		if transition.OldExecutionFenced {
			return current, nil
		}
		expectedVersion := current.Version
		current.Metadata = cloneMap(current.Metadata)
		transition.OldExecutionFenced = true
		current.Metadata[protocol.GoalMetadataObjectiveTransition] =
			objectiveTransitionMetadata(transition)
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
			"objective_predecessor_fenced",
			protocol.GoalUpdateSourceSystem,
			"",
			map[string]any{
				"transition_id": transition.ID,
				"execution_id":  executionID,
			},
		); eventErr != nil {
			return nil, eventErr
		}
		return updated, nil
	})
}

// CommitObjectiveRetarget advances the canonical Goal revision only after the
// old Goal-bound WorkGraph has been fenced. Old completion criteria are deleted;
// the reserved successor becomes the only reverse binding.
func (s *Service) CommitObjectiveRetarget(
	ctx context.Context,
	goalID string,
	transitionID string,
) (*protocol.Goal, error) {
	item, err := s.loadMutableGoal(ctx, goalID)
	if err != nil {
		return nil, err
	}
	return s.retryGoalMutation(ctx, item, func(current *protocol.Goal) (*protocol.Goal, error) {
		transition, ok := ObjectiveTransitionFromGoal(*current)
		if !ok || transition.ID != strings.TrimSpace(transitionID) {
			return nil, fmt.Errorf("%w: Goal objective transition is missing or differs", ErrGoalRevisionStale)
		}
		if current.ObjectiveRevision() == transition.NewRevision &&
			strings.TrimSpace(current.Objective) == transition.TargetObjective {
			return current, nil
		}
		if current.ObjectiveRevision() != transition.OldRevision ||
			transition.Phase != ObjectiveTransitionPrepared {
			return nil, ErrGoalRevisionStale
		}
		current.Objective = transition.TargetObjective
		resetGoalContinuationForObjectiveReplacement(current)
		current.Metadata = cloneMap(current.Metadata)
		if current.Metadata == nil {
			current.Metadata = map[string]any{}
		}
		current.Metadata[protocol.GoalMetadataObjectiveRevision] = transition.NewRevision
		current.Metadata[protocol.GoalMetadataExecutionID] = transition.SuccessorExecutionID
		current.Metadata[protocol.GoalMetadataExecutionBindingState] =
			string(protocol.GoalExecutionBindingStateReserved)
		delete(current.Metadata, protocol.GoalMetadataCompletionCriteria)
		transition.Phase = ObjectiveTransitionAwaitingPlan
		current.Metadata[protocol.GoalMetadataObjectiveTransition] = objectiveTransitionMetadata(transition)
		payload := map[string]any{
			"objective":              transition.TargetObjective,
			"objective_updated":      true,
			"objective_revision":     transition.NewRevision,
			"transition_id":          transition.ID,
			"old_execution_id":       transition.OldExecutionID,
			"successor_execution_id": transition.SuccessorExecutionID,
		}
		return s.persistTransition(
			ctx,
			*current,
			protocol.GoalStatusActive,
			transition.Source,
			"updated",
			"",
			payload,
		)
	})
}

// ConfirmObjectiveExecutionBinding records the authoritative boundary after an
// existing Execution bind or successor Execution+Plan mutation is durable.
// It also confirms an initial binding that has no objective transition.
func (s *Service) ConfirmObjectiveExecutionBinding(
	ctx context.Context,
	goalID string,
	expectedObjectiveRevision int64,
	executionID string,
	completionCriteria []string,
) (*protocol.Goal, error) {
	goalID = strings.TrimSpace(goalID)
	executionID = strings.TrimSpace(executionID)
	if goalID == "" || executionID == "" || expectedObjectiveRevision <= 0 {
		return nil, newGoalInvalidInputError(
			"goal id, execution id and expected objective revision are required for binding confirmation",
		)
	}
	item, err := s.loadMutableGoal(ctx, goalID)
	if err != nil {
		return nil, err
	}
	criteria := normalizeExecutionCompletionCriteria(completionCriteria)
	return s.retryGoalMutation(ctx, item, func(current *protocol.Goal) (*protocol.Goal, error) {
		if current.ObjectiveRevision() != expectedObjectiveRevision {
			return nil, ErrGoalRevisionStale
		}
		if protocol.GoalMetadataString(current.Metadata, protocol.GoalMetadataExecutionID) != executionID {
			return nil, ErrGoalExecutionBindingConflict
		}
		bindingState := protocol.GoalExecutionBindingStateFromGoal(*current)
		if bindingState == protocol.GoalExecutionBindingStateReserved ||
			bindingState == protocol.GoalExecutionBindingStateConflict {
			return nil, fmt.Errorf(
				"%w: Goal binding is not pending confirmation",
				ErrGoalExecutionBindingConflict,
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
				"%w: confirmed completion criteria differ from the prepared binding",
				ErrGoalExecutionBindingConflict,
			)
		}
		transition, ok := ObjectiveTransitionFromGoal(*current)
		if ok {
			if transition.SuccessorExecutionID != executionID ||
				transition.NewRevision != expectedObjectiveRevision {
				return nil, ErrGoalExecutionBindingConflict
			}
			if transition.Phase == ObjectiveTransitionBound &&
				bindingState == protocol.GoalExecutionBindingStateConfirmed &&
				slices.Equal(existingCriteria, criteria) {
				return current, nil
			}
			if transition.Phase != ObjectiveTransitionBindingReserved {
				return nil, fmt.Errorf(
					"%w: Goal successor binding is not pending confirmation",
					ErrGoalExecutionBindingConflict,
				)
			}
		} else if bindingState == protocol.GoalExecutionBindingStateConfirmed {
			if slices.Equal(existingCriteria, criteria) {
				return current, nil
			}
			return nil, ErrGoalExecutionBindingConflict
		}
		expectedVersion := current.Version
		current.Metadata = cloneMap(current.Metadata)
		if current.Metadata == nil {
			current.Metadata = map[string]any{}
		}
		delete(current.Metadata, protocol.GoalMetadataObjectiveAlignment)
		current.Metadata[protocol.GoalMetadataExecutionBindingState] =
			string(protocol.GoalExecutionBindingStateConfirmed)
		if len(criteria) == 0 {
			delete(current.Metadata, protocol.GoalMetadataCompletionCriteria)
		} else {
			current.Metadata[protocol.GoalMetadataCompletionCriteria] = append([]string(nil), criteria...)
		}
		eventType := "execution_bound"
		eventPayload := map[string]any{
			"execution_id":  executionID,
			"binding_state": string(protocol.GoalExecutionBindingStateConfirmed),
		}
		if ok {
			transition.Phase = ObjectiveTransitionBound
			current.Metadata[protocol.GoalMetadataObjectiveTransition] = objectiveTransitionMetadata(transition)
			eventType = "execution_rebase_bound"
			eventPayload["transition_id"] = transition.ID
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
			eventType,
			protocol.GoalUpdateSourceSystem,
			"",
			eventPayload,
		); eventErr != nil {
			return nil, eventErr
		}
		return updated, nil
	})
}

func validateObjectiveRetargetCommand(command ObjectiveRetargetCommand) error {
	if strings.TrimSpace(command.Goal.ID) == "" ||
		strings.TrimSpace(command.Objective) == "" ||
		strings.TrimSpace(command.CommandID) == "" ||
		strings.TrimSpace(command.TransitionID) == "" ||
		strings.TrimSpace(command.SuccessorExecutionID) == "" ||
		strings.TrimSpace(command.Reason) == "" ||
		command.ExpectedObjectiveRevision <= 0 {
		return newGoalInvalidInputError("complete Goal objective transition identity is required")
	}
	switch command.Source {
	case protocol.GoalUpdateSourceUser,
		protocol.GoalUpdateSourceModel,
		protocol.GoalUpdateSourceExternal:
	default:
		return newGoalInvalidInputError("Goal objective transition source is invalid")
	}
	return nil
}

func objectiveTransitionMetadata(transition ObjectiveTransition) map[string]any {
	return map[string]any{
		"transition_id":          transition.ID,
		"command_id":             transition.CommandID,
		"phase":                  string(transition.Phase),
		"old_revision":           transition.OldRevision,
		"new_revision":           transition.NewRevision,
		"old_execution_id":       transition.OldExecutionID,
		"old_execution_fenced":   transition.OldExecutionFenced,
		"successor_execution_id": transition.SuccessorExecutionID,
		"requested_objective":    transition.RequestedObjective,
		"target_objective":       transition.TargetObjective,
		"reason":                 transition.Reason,
		"source":                 string(transition.Source),
	}
}

func validObjectiveTransitionPhase(phase ObjectiveTransitionPhase) bool {
	switch phase {
	case ObjectiveTransitionPrepared,
		ObjectiveTransitionAwaitingPlan,
		ObjectiveTransitionBindingReserved,
		ObjectiveTransitionBound:
		return true
	default:
		return false
	}
}

func objectiveRetargetRequestedObjective(command ObjectiveRetargetCommand) string {
	requested := strings.TrimSpace(command.RequestedObjective)
	if requested == "" {
		requested = strings.TrimSpace(command.Objective)
	}
	return requested
}

func objectiveRetargetRequestAlreadyApplied(
	item protocol.Goal,
	requestedObjective string,
) bool {
	transition, ok := ObjectiveTransitionFromGoal(item)
	return ok &&
		transition.Phase != ObjectiveTransitionPrepared &&
		transition.RequestedObjective == strings.TrimSpace(requestedObjective) &&
		item.ObjectiveRevision() == transition.NewRevision &&
		strings.TrimSpace(item.Objective) == transition.TargetObjective
}

func metadataString(metadata map[string]any, key string) string {
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}

func preserveServerOwnedGoalMetadata(
	current protocol.Goal,
	replacement map[string]any,
) map[string]any {
	replacement = cloneMap(replacement)
	if replacement == nil {
		replacement = map[string]any{}
	}
	for _, key := range []string{
		protocol.GoalMetadataOwnerUserID,
		protocol.GoalMetadataExecutionID,
		protocol.GoalMetadataExecutionBindingState,
		protocol.GoalMetadataPromotionCommand,
		protocol.GoalMetadataActivationOrigin,
		protocol.GoalMetadataActivationReason,
		protocol.GoalMetadataCompletionCriteria,
		protocol.GoalMetadataObjectiveAlignment,
		protocol.GoalMetadataExplicitCommand,
		protocol.GoalMetadataObjectiveTransition,
	} {
		if value, exists := current.Metadata[key]; exists {
			replacement[key] = value
		} else {
			delete(replacement, key)
		}
	}
	return replacement
}

func authorizeGoalOwner(item protocol.Goal, ownerUserID string) error {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return nil
	}
	storedOwnerUserID := protocol.GoalMetadataString(
		item.Metadata,
		protocol.GoalMetadataOwnerUserID,
	)
	if storedOwnerUserID != "" && storedOwnerUserID != ownerUserID {
		return fmt.Errorf("%w: Goal belongs to another owner", ErrGoalForbidden)
	}
	return nil
}

func authorizeGoalReader(item protocol.Goal, ownerUserID string) error {
	ownerUserID = strings.TrimSpace(ownerUserID)
	storedOwnerUserID := protocol.GoalMetadataString(
		item.Metadata,
		protocol.GoalMetadataOwnerUserID,
	)
	if ownerUserID == "" || storedOwnerUserID == "" || storedOwnerUserID != ownerUserID {
		return fmt.Errorf("%w: Goal owner provenance does not match", ErrGoalForbidden)
	}
	return nil
}

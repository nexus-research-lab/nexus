// INPUT: nexus_goal create request、当前 explicit Goal、current Execution 与 Goal/Orchestration 服务。
// OUTPUT: create_goal -> Execution 和 Goal -> Ensure 两个方向共用的幂等 binding saga。
// POS: Goal 与 Execution 两个领域服务之间的应用层协调器；不把跨域事务伪装成单库原子操作。
package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	goalmcpcontract "github.com/nexus-research-lab/nexus/internal/mcp/goal/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type explicitGoalLifecycleService interface {
	goalmcpcontract.Service
	BindExplicitExecution(
		context.Context,
		goalsvc.ExplicitExecutionBinding,
	) (*protocol.Goal, error)
	PrepareObjectiveRetarget(
		context.Context,
		goalsvc.ObjectiveRetargetCommand,
	) (*protocol.Goal, error)
	FenceObjectiveRetargetPredecessor(
		context.Context,
		string,
		string,
		string,
	) (*protocol.Goal, error)
	CommitObjectiveRetarget(context.Context, string, string) (*protocol.Goal, error)
	ConfirmObjectiveExecutionBinding(
		context.Context,
		string,
		int64,
		string,
		[]string,
	) (*protocol.Goal, error)
}

type explicitGoalExecutionService interface {
	GetCurrent(
		context.Context,
		orchestrationsvc.ActorContext,
	) (*protocol.ExecutionSnapshot, error)
	ValidateGoalRevisionOwner(
		context.Context,
		string,
		string,
		int64,
		string,
	) (bool, error)
	BindExplicitGoal(
		context.Context,
		orchestrationsvc.ActorContext,
		orchestrationsvc.BindExplicitGoalInput,
	) (orchestrationsvc.MutationResult, error)
	SupersedeGoalRevision(
		context.Context,
		orchestrationsvc.GoalRevisionSupersedeInput,
	) (*protocol.ExecutionSnapshot, error)
}

// explicitGoalExecutionCoordinator 既包装 nexus_goal service，也向 Ensure
// 提供 active explicit Goal preflight，保证两种调用顺序收敛到同一 binding。
type explicitGoalExecutionCoordinator struct {
	goals      explicitGoalLifecycleService
	executions explicitGoalExecutionService
}

func newExplicitGoalExecutionCoordinator(
	goals explicitGoalLifecycleService,
	executions explicitGoalExecutionService,
) *explicitGoalExecutionCoordinator {
	return &explicitGoalExecutionCoordinator{
		goals:      goals,
		executions: executions,
	}
}

func (c *explicitGoalExecutionCoordinator) Create(
	ctx context.Context,
	request protocol.CreateGoalRequest,
) (*protocol.Goal, error) {
	if c == nil || c.goals == nil || c.executions == nil {
		return nil, errors.New("explicit Goal execution coordinator is unavailable")
	}
	actor, objective, err := explicitGoalActor(request)
	if err != nil {
		return nil, err
	}
	snapshot, err := c.executions.GetCurrent(ctx, actor)
	if err != nil {
		return nil, fmt.Errorf("read current Execution before create_goal: %w", err)
	}
	if snapshot != nil {
		if err = validateExplicitGoalExecutionCompatibility(
			snapshot.Execution,
			actor,
			objective,
		); err != nil {
			return nil, err
		}
	}
	commandID := explicitGoalCommandID(request, objective)
	current, err := c.goals.CurrentOptional(
		goalsvc.WithActiveGoalContinuationSuppressed(ctx),
		actor.SessionKey,
	)
	if err != nil {
		return nil, err
	}
	if current != nil {
		return c.reuseExplicitGoalOrConflict(ctx, actor, snapshot, request, objective, commandID, *current)
	}

	request.SessionKey = actor.SessionKey
	request.Objective = objective
	request.Metadata = explicitGoalMetadata(request.Metadata, snapshot, commandID)
	created, err := c.goals.Create(
		goalsvc.WithActiveGoalContinuationSuppressed(ctx),
		request,
	)
	if errors.Is(err, goalsvc.ErrGoalConflict) {
		current, loadErr := c.goals.CurrentOptional(
			goalsvc.WithActiveGoalContinuationSuppressed(ctx),
			actor.SessionKey,
		)
		if loadErr != nil {
			return nil, loadErr
		}
		if current != nil {
			return c.reuseExplicitGoalOrConflict(
				ctx,
				actor,
				snapshot,
				request,
				objective,
				commandID,
				*current,
			)
		}
	}
	if err != nil {
		return nil, err
	}
	if created == nil {
		return nil, errors.New("create_goal returned no Goal")
	}
	if err = c.bindCreatedExplicitGoal(ctx, actor, snapshot, commandID, *created); err != nil {
		return nil, err
	}
	return created, nil
}

func (c *explicitGoalExecutionCoordinator) reuseExplicitGoalOrConflict(
	ctx context.Context,
	actor orchestrationsvc.ActorContext,
	snapshot *protocol.ExecutionSnapshot,
	request protocol.CreateGoalRequest,
	objective string,
	commandID string,
	current protocol.Goal,
) (*protocol.Goal, error) {
	if protocol.NormalizeGoalStatus(current.Status) != protocol.GoalStatusActive {
		return nil, goalsvc.ErrGoalConflict
	}
	if strings.TrimSpace(current.Objective) != objective {
		return nil, fmt.Errorf(
			"%w: active Goal objective %q does not match requested objective %q",
			orchestrationsvc.ErrExplicitGoalObjectiveConflict,
			current.Objective,
			objective,
		)
	}
	if !explicitGoalRetryMatches(current, request, snapshot, commandID) {
		return nil, goalsvc.ErrGoalConflict
	}
	if err := c.bindCreatedExplicitGoal(ctx, actor, snapshot, commandID, current); err != nil {
		return nil, err
	}
	return &current, nil
}

func (c *explicitGoalExecutionCoordinator) bindCreatedExplicitGoal(
	ctx context.Context,
	actor orchestrationsvc.ActorContext,
	snapshot *protocol.ExecutionSnapshot,
	commandID string,
	goal protocol.Goal,
) error {
	if snapshot == nil {
		return nil
	}
	current := snapshot
	for attempt := 0; attempt < 3; attempt++ {
		result, err := c.executions.BindExplicitGoal(ctx, actor, orchestrationsvc.BindExplicitGoalInput{
			ExecutionID:           current.Execution.ID,
			SnapshotRevision:      current.Execution.Version,
			CommandID:             commandID,
			GoalID:                goal.ID,
			GoalObjectiveRevision: goal.ObjectiveRevision(),
			Objective:             goal.Objective,
		})
		if err != nil {
			return fmt.Errorf("bind explicit Goal to current Execution: %w", err)
		}
		if result.Outcome != orchestrationsvc.MutationRejected {
			return nil
		}
		if result.ReasonCode == orchestrationsvc.ErrorCodeStaleExecution {
			reloaded, reloadErr := c.executions.GetCurrent(ctx, actor)
			if reloadErr != nil {
				return fmt.Errorf("reload current Execution after binding race: %w", reloadErr)
			}
			if reloaded == nil {
				return fmt.Errorf(
					"%w: current Execution disappeared during explicit Goal binding",
					orchestrationsvc.ErrExplicitGoalBindingConflict,
				)
			}
			if executionMatchesExplicitGoal(reloaded.Execution, goal) {
				return nil
			}
			current = reloaded
			continue
		}
		switch result.ReasonCode {
		case orchestrationsvc.ErrorCodeGoalObjectiveConflict:
			return fmt.Errorf("%w: %s", orchestrationsvc.ErrExplicitGoalObjectiveConflict, result.Message)
		case orchestrationsvc.ErrorCodeGoalScopeConflict:
			return fmt.Errorf("%w: %s", orchestrationsvc.ErrExplicitGoalScopeConflict, result.Message)
		case orchestrationsvc.ErrorCodeGoalBindingConflict:
			return fmt.Errorf("%w: %s", orchestrationsvc.ErrExplicitGoalBindingConflict, result.Message)
		default:
			return fmt.Errorf(
				"explicit Goal binding rejected (%s): %s",
				result.ReasonCode,
				result.Message,
			)
		}
	}
	return fmt.Errorf(
		"%w: Execution kept changing while explicit Goal binding retried",
		orchestrationsvc.ErrExplicitGoalBindingConflict,
	)
}

func (c *explicitGoalExecutionCoordinator) PrepareExplicitGoalBinding(
	ctx context.Context,
	request orchestrationsvc.ExplicitGoalBindingRequest,
) (*orchestrationsvc.ExplicitGoalBinding, error) {
	if c == nil || c.goals == nil {
		return nil, errors.New("explicit Goal binding gateway is unavailable")
	}
	current, err := c.goals.CurrentOptional(
		goalsvc.WithActiveGoalContinuationSuppressed(ctx),
		request.SessionKey,
	)
	if errors.Is(err, goalsvc.ErrGoalDisabled) {
		return nil, nil
	}
	if err != nil || current == nil {
		return nil, err
	}
	if err = validateGoalForExplicitBinding(*current, request); err != nil {
		return nil, err
	}
	if existingGoalID := strings.TrimSpace(request.ExistingGoalID); existingGoalID != "" &&
		existingGoalID != strings.TrimSpace(current.ID) {
		return nil, fmt.Errorf(
			"%w: current Execution is bound to Goal %s, not active Goal %s",
			orchestrationsvc.ErrExplicitGoalBindingConflict,
			existingGoalID,
			current.ID,
		)
	}
	origin := protocol.GoalActivationOrigin(protocol.GoalMetadataString(
		current.Metadata,
		protocol.GoalMetadataActivationOrigin,
	))
	switch origin {
	case protocol.GoalActivationOriginUserExplicit,
		protocol.GoalActivationOriginAdaptiveInitial,
		protocol.GoalActivationOriginAdaptivePromoted:
	default:
		return nil, fmt.Errorf(
			"%w: active Goal %s has no managed Execution activation provenance",
			orchestrationsvc.ErrExplicitGoalBindingConflict,
			current.ID,
		)
	}
	activationReason := protocol.GoalActivationReason(protocol.GoalMetadataString(
		current.Metadata,
		protocol.GoalMetadataActivationReason,
	))
	if activationReason == "" {
		return nil, fmt.Errorf(
			"%w: active Goal has inconsistent activation reason",
			orchestrationsvc.ErrExplicitGoalBindingConflict,
		)
	}

	candidateID := strings.TrimSpace(request.CandidateExecutionID)
	storedID := protocol.GoalMetadataString(
		current.Metadata,
		protocol.GoalMetadataExecutionID,
	)
	if request.ExistingExecution && storedID != "" && storedID != candidateID {
		return nil, fmt.Errorf(
			"%w: Goal is reserved for Execution %s, not current Execution %s",
			orchestrationsvc.ErrExplicitGoalBindingConflict,
			storedID,
			candidateID,
		)
	}
	executionID := candidateID
	if !request.ExistingExecution && storedID != "" {
		executionID = storedID
	}
	if executionID == "" {
		return nil, fmt.Errorf(
			"%w: no Execution identity is available",
			orchestrationsvc.ErrExplicitGoalBindingConflict,
		)
	}
	updated, err := c.goals.BindExplicitExecution(ctx, goalsvc.ExplicitExecutionBinding{
		GoalID:                    current.ID,
		ExpectedObjectiveRevision: current.ObjectiveRevision(),
		ExecutionID:               executionID,
		CompletionCriteria:        request.CompletionCriteria,
		RoundID:                   request.RootRoundID,
	})
	if errors.Is(err, goalsvc.ErrGoalExecutionBindingConflict) {
		return nil, fmt.Errorf("%w: %v", orchestrationsvc.ErrExplicitGoalBindingConflict, err)
	}
	if errors.Is(err, goalsvc.ErrGoalRevisionStale) {
		return nil, fmt.Errorf(
			"%w: Goal objective changed before Execution binding committed",
			orchestrationsvc.ErrExplicitGoalObjectiveConflict,
		)
	}
	if err != nil {
		return nil, err
	}
	if request.ExistingExecution {
		if transition, transitioning := goalsvc.ObjectiveTransitionFromGoal(*updated); transitioning &&
			transition.Phase == goalsvc.ObjectiveTransitionBindingReserved {
			updated, err = c.goals.ConfirmObjectiveExecutionBinding(
				ctx,
				updated.ID,
				updated.ObjectiveRevision(),
				executionID,
				request.CompletionCriteria,
			)
			if err != nil {
				return nil, err
			}
		}
	}
	replacesExecutionID := ""
	if transition, transitioning := goalsvc.ObjectiveTransitionFromGoal(*updated); transitioning &&
		transition.SuccessorExecutionID == executionID &&
		!transition.OldExecutionFenced {
		replacesExecutionID = transition.OldExecutionID
	}
	return &orchestrationsvc.ExplicitGoalBinding{
		ExecutionID:           executionID,
		GoalID:                updated.ID,
		GoalObjectiveRevision: updated.ObjectiveRevision(),
		ActivationOrigin:      origin,
		ActivationReason:      activationReason,
		ReplacesExecutionID:   replacesExecutionID,
	}, nil
}

func (c *explicitGoalExecutionCoordinator) ConfirmGoalExecutionBinding(
	ctx context.Context,
	confirmation orchestrationsvc.GoalExecutionBindingConfirmation,
) error {
	if c == nil || c.goals == nil {
		return errors.New("Goal execution binding confirmation is unavailable")
	}
	_, err := c.goals.ConfirmObjectiveExecutionBinding(
		ctx,
		confirmation.GoalID,
		confirmation.GoalObjectiveRevision,
		confirmation.ExecutionID,
		confirmation.CompletionCriteria,
	)
	return err
}

func (c *explicitGoalExecutionCoordinator) Current(
	ctx context.Context,
	sessionKey string,
) (*protocol.Goal, error) {
	return c.goals.Current(ctx, sessionKey)
}

func (c *explicitGoalExecutionCoordinator) CurrentOptional(
	ctx context.Context,
	sessionKey string,
) (*protocol.Goal, error) {
	return c.goals.CurrentOptional(ctx, sessionKey)
}

func (c *explicitGoalExecutionCoordinator) RetargetByModel(
	ctx context.Context,
	sessionKey string,
	request protocol.RetargetGoalRequest,
) (*protocol.Goal, error) {
	return c.goals.RetargetByModel(ctx, sessionKey, request)
}

// RetargetGoalObjective executes the durable Goal revision / Execution rebase
// saga shared by MCP, HTTP and app-server objective mutation paths.
func (c *explicitGoalExecutionCoordinator) RetargetGoalObjective(
	ctx context.Context,
	command goalsvc.ObjectiveRetargetCommand,
) (*protocol.Goal, error) {
	if c == nil || c.goals == nil || c.executions == nil {
		return nil, errors.New("Goal objective retarget coordinator is unavailable")
	}
	objective := strings.TrimSpace(command.Objective)
	if objective == "" || strings.TrimSpace(command.Goal.ID) == "" {
		return nil, goalsvc.ErrGoalInvalidInput
	}
	requestedObjective := strings.TrimSpace(command.RequestedObjective)
	if requestedObjective == "" {
		requestedObjective = objective
	}
	expectedRevision := command.ExpectedObjectiveRevision
	if expectedRevision <= 0 {
		expectedRevision = command.Goal.ObjectiveRevision()
	}
	commandID, transitionID, successorID := goalRetargetIdentities(
		command.Goal.ID,
		expectedRevision,
		requestedObjective,
		command.CommandID,
	)
	command.CommandID = commandID
	command.TransitionID = transitionID
	command.SuccessorExecutionID = successorID
	command.ExpectedObjectiveRevision = expectedRevision
	command.RequestedObjective = requestedObjective
	command.Objective = objective
	storedOwnerUserID := protocol.GoalMetadataString(
		command.Goal.Metadata,
		protocol.GoalMetadataOwnerUserID,
	)
	if command.Source == protocol.GoalUpdateSourceUser {
		ownerUserID := strings.TrimSpace(command.OwnerUserID)
		if ownerUserID == "" {
			return nil, fmt.Errorf(
				"%w: current owner identity is required",
				goalsvc.ErrGoalForbidden,
			)
		}
		if storedOwnerUserID != "" && storedOwnerUserID != ownerUserID {
			return nil, fmt.Errorf("%w: Goal belongs to another owner", goalsvc.ErrGoalForbidden)
		}
		oldExecutionID := protocol.GoalMetadataString(
			command.Goal.Metadata,
			protocol.GoalMetadataExecutionID,
		)
		materialized := false
		if oldExecutionID != "" {
			var ownerErr error
			materialized, ownerErr = c.executions.ValidateGoalRevisionOwner(
				ctx,
				oldExecutionID,
				command.Goal.ID,
				expectedRevision,
				ownerUserID,
			)
			if ownerErr != nil {
				return nil, fmt.Errorf("%w: %v", goalsvc.ErrGoalForbidden, ownerErr)
			}
		}
		if storedOwnerUserID == "" && !materialized {
			return nil, fmt.Errorf(
				"%w: Goal owner provenance is unavailable",
				goalsvc.ErrGoalForbidden,
			)
		}
		storedOwnerUserID = ownerUserID
	}
	prepared, err := c.goals.PrepareObjectiveRetarget(ctx, command)
	if err != nil {
		return nil, err
	}
	transition, ok := goalsvc.ObjectiveTransitionFromGoal(*prepared)
	if !ok || transition.ID != transitionID {
		if prepared.Objective == objective {
			return prepared, nil
		}
		return nil, fmt.Errorf("%w: prepared Goal objective transition is unavailable", goalsvc.ErrGoalInvalidState)
	}
	if transition.OldExecutionID != "" && !transition.OldExecutionFenced {
		actorID := strings.TrimSpace(command.AgentID)
		if command.Source == protocol.GoalUpdateSourceUser {
			actorID = strings.TrimSpace(command.OwnerUserID)
		}
		var predecessor *protocol.ExecutionSnapshot
		if predecessor, err = c.executions.SupersedeGoalRevision(ctx, orchestrationsvc.GoalRevisionSupersedeInput{
			ExecutionID:              transition.OldExecutionID,
			ExpectedOwnerUserID:      storedOwnerUserID,
			GoalID:                   prepared.ID,
			OldGoalObjectiveRevision: transition.OldRevision,
			NewGoalObjectiveRevision: transition.NewRevision,
			SuccessorExecutionID:     transition.SuccessorExecutionID,
			CommandID:                commandID,
			Reason:                   transition.Reason,
			Source:                   transition.Source,
			ActorID:                  actorID,
			RootRoundID:              strings.TrimSpace(command.RoundID),
		}); err != nil {
			return nil, fmt.Errorf("supersede old Goal objective WorkGraph: %w", err)
		}
		if predecessor == nil {
			prepared, err = c.goals.FenceObjectiveRetargetPredecessor(
				ctx,
				prepared.ID,
				transition.ID,
				transition.OldExecutionID,
			)
			if err != nil {
				return nil, fmt.Errorf("record fenced Goal objective predecessor: %w", err)
			}
			transition, ok = goalsvc.ObjectiveTransitionFromGoal(*prepared)
			if !ok || !transition.OldExecutionFenced {
				return nil, fmt.Errorf(
					"%w: fenced Goal objective predecessor was not persisted",
					goalsvc.ErrGoalInvalidState,
				)
			}
		}
	}
	committed, err := c.goals.CommitObjectiveRetarget(ctx, prepared.ID, transition.ID)
	if err != nil {
		return nil, fmt.Errorf("commit Goal objective revision: %w", err)
	}
	return committed, nil
}

func (c *explicitGoalExecutionCoordinator) CompleteByModel(
	ctx context.Context,
	goalID string,
	request protocol.CompleteGoalRequest,
) (*protocol.Goal, error) {
	return c.goals.CompleteByModel(ctx, goalID, request)
}

func (c *explicitGoalExecutionCoordinator) BlockByModel(
	ctx context.Context,
	goalID string,
	request protocol.BlockGoalRequest,
) (*protocol.Goal, error) {
	return c.goals.BlockByModel(ctx, goalID, request)
}

func explicitGoalActor(
	request protocol.CreateGoalRequest,
) (orchestrationsvc.ActorContext, string, error) {
	sessionKey, err := protocol.RequireStructuredSessionKey(request.SessionKey)
	if err != nil {
		return orchestrationsvc.ActorContext{}, "", fmt.Errorf(
			"%w: %v",
			goalsvc.ErrGoalInvalidInput,
			err,
		)
	}
	objective := strings.TrimSpace(request.Objective)
	if objective == "" {
		return orchestrationsvc.ActorContext{}, "", fmt.Errorf(
			"%w: goal objective must not be empty",
			goalsvc.ErrGoalInvalidInput,
		)
	}
	agentID := strings.TrimSpace(request.AgentID)
	if agentID == "" {
		return orchestrationsvc.ActorContext{}, "", fmt.Errorf(
			"%w: current agent identity is required for explicit Goal binding",
			goalsvc.ErrGoalInvalidInput,
		)
	}
	if strings.TrimSpace(request.OwnerUserID) == "" {
		return orchestrationsvc.ActorContext{}, "", fmt.Errorf(
			"%w: current owner identity is required for explicit Goal binding",
			goalsvc.ErrGoalInvalidInput,
		)
	}
	parsed := protocol.ParseSessionKey(sessionKey)
	scope := protocol.ExecutionScopeDM
	conversationID := ""
	if parsed.Kind == protocol.SessionKeyKindRoom {
		scope = protocol.ExecutionScopeRoom
		conversationID = strings.TrimSpace(parsed.ConversationID)
	}
	return orchestrationsvc.ActorContext{
		OwnerUserID:    strings.TrimSpace(request.OwnerUserID),
		SessionKey:     sessionKey,
		AgentID:        agentID,
		Role:           orchestrationsvc.ExecutionActorCoordinator,
		ActorKind:      protocol.ExecutionActorAgent,
		ScopeKind:      scope,
		ConversationID: conversationID,
		RootRoundID:    strings.TrimSpace(request.RoundID),
	}, objective, nil
}

func validateExplicitGoalExecutionCompatibility(
	execution protocol.Execution,
	actor orchestrationsvc.ActorContext,
	objective string,
) error {
	if strings.TrimSpace(execution.SessionKey) != actor.SessionKey ||
		execution.ScopeKind != actor.ScopeKind ||
		(actor.ScopeKind == protocol.ExecutionScopeRoom &&
			strings.TrimSpace(execution.ConversationID) != actor.ConversationID) {
		return fmt.Errorf(
			"%w: current Execution does not belong to the explicit Goal scope",
			orchestrationsvc.ErrExplicitGoalScopeConflict,
		)
	}
	if strings.TrimSpace(execution.Objective) != objective {
		return fmt.Errorf(
			"%w: current Execution objective %q does not match Goal objective %q",
			orchestrationsvc.ErrExplicitGoalObjectiveConflict,
			execution.Objective,
			objective,
		)
	}
	if strings.TrimSpace(execution.CoordinatorAgentID) != strings.TrimSpace(actor.AgentID) {
		return fmt.Errorf(
			"%w: only the current Execution coordinator may create its explicit Goal",
			orchestrationsvc.ErrExplicitGoalBindingConflict,
		)
	}
	return nil
}

func validateGoalForExplicitBinding(
	goal protocol.Goal,
	request orchestrationsvc.ExplicitGoalBindingRequest,
) error {
	if protocol.NormalizeGoalStatus(goal.Status) != protocol.GoalStatusActive {
		return fmt.Errorf(
			"%w: Goal must be active before binding an Execution",
			orchestrationsvc.ErrExplicitGoalBindingConflict,
		)
	}
	if strings.TrimSpace(goal.SessionKey) != strings.TrimSpace(request.SessionKey) {
		return fmt.Errorf(
			"%w: active Goal session differs from Execution session",
			orchestrationsvc.ErrExplicitGoalScopeConflict,
		)
	}
	goalScope := protocol.ExecutionScopeDM
	parsed := protocol.ParseSessionKey(goal.SessionKey)
	if parsed.Kind == protocol.SessionKeyKindRoom {
		goalScope = protocol.ExecutionScopeRoom
	}
	if goalScope != request.ScopeKind ||
		(goalScope == protocol.ExecutionScopeRoom &&
			strings.TrimSpace(parsed.ConversationID) != strings.TrimSpace(request.ConversationID)) {
		return fmt.Errorf(
			"%w: active Goal scope differs from Execution scope",
			orchestrationsvc.ErrExplicitGoalScopeConflict,
		)
	}
	if strings.TrimSpace(goal.Objective) != strings.TrimSpace(request.Objective) {
		return fmt.Errorf(
			"%w: active Goal objective %q differs from Execution objective %q",
			orchestrationsvc.ErrExplicitGoalObjectiveConflict,
			goal.Objective,
			request.Objective,
		)
	}
	if goalScope == protocol.ExecutionScopeRoom {
		leadAgentID := goalsvc.RoomLeadAgentID(goal)
		if leadAgentID == "" || leadAgentID != strings.TrimSpace(request.AgentID) {
			return fmt.Errorf(
				"%w: Room Goal lead does not match Execution coordinator",
				orchestrationsvc.ErrExplicitGoalBindingConflict,
			)
		}
	}
	return nil
}

func explicitGoalMetadata(
	source map[string]any,
	snapshot *protocol.ExecutionSnapshot,
	commandID string,
) map[string]any {
	metadata := make(map[string]any, len(source)+5)
	for key, value := range source {
		metadata[key] = value
	}
	delete(metadata, protocol.GoalMetadataPromotionCommand)
	metadata[protocol.GoalMetadataExplicitCommand] = commandID
	metadata[protocol.GoalMetadataActivationOrigin] = string(protocol.GoalActivationOriginUserExplicit)
	metadata[protocol.GoalMetadataActivationReason] = string(protocol.GoalActivationReasonPersistenceRequested)
	if snapshot == nil {
		delete(metadata, protocol.GoalMetadataExecutionID)
		delete(metadata, protocol.GoalMetadataCompletionCriteria)
		return metadata
	}
	metadata[protocol.GoalMetadataExecutionID] = snapshot.Execution.ID
	criteria := normalizeExplicitCriteria(snapshot.Execution.CompletionCriteria)
	if len(criteria) == 0 {
		delete(metadata, protocol.GoalMetadataCompletionCriteria)
	} else {
		metadata[protocol.GoalMetadataCompletionCriteria] = criteria
	}
	return metadata
}

func explicitGoalRetryMatches(
	goal protocol.Goal,
	request protocol.CreateGoalRequest,
	snapshot *protocol.ExecutionSnapshot,
	commandID string,
) bool {
	if protocol.GoalMetadataString(
		goal.Metadata,
		protocol.GoalMetadataExplicitCommand,
	) != commandID ||
		protocol.GoalMetadataString(
			goal.Metadata,
			protocol.GoalMetadataActivationOrigin,
		) != string(protocol.GoalActivationOriginUserExplicit) ||
		protocol.GoalMetadataString(
			goal.Metadata,
			protocol.GoalMetadataActivationReason,
		) != string(protocol.GoalActivationReasonPersistenceRequested) ||
		!goalTokenBudgetMatches(goal.TokenBudget, request.TokenBudget) {
		return false
	}
	executionID := protocol.GoalMetadataString(
		goal.Metadata,
		protocol.GoalMetadataExecutionID,
	)
	if snapshot == nil {
		return executionID == ""
	}
	if executionID != snapshot.Execution.ID {
		return false
	}
	return slices.Equal(
		goalMetadataCriteria(goal.Metadata),
		normalizeExplicitCriteria(snapshot.Execution.CompletionCriteria),
	)
}

func explicitGoalCommandID(request protocol.CreateGoalRequest, objective string) string {
	budget := "nil"
	if request.TokenBudget != nil {
		budget = strconv.FormatInt(*request.TokenBudget, 10)
	}
	sum := sha256.Sum256([]byte(strings.Join([]string{
		strings.TrimSpace(request.SessionKey),
		strings.TrimSpace(request.RoundID),
		strings.TrimSpace(request.AgentID),
		strings.TrimSpace(objective),
		budget,
	}, "\x00")))
	return "explicit_goal_" + hex.EncodeToString(sum[:12])
}

func normalizeExplicitCriteria(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func goalMetadataCriteria(metadata map[string]any) []string {
	value := metadata[protocol.GoalMetadataCompletionCriteria]
	switch typed := value.(type) {
	case []string:
		return normalizeExplicitCriteria(typed)
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				result = append(result, text)
			}
		}
		return normalizeExplicitCriteria(result)
	default:
		return nil
	}
}

func goalTokenBudgetMatches(left *int64, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func executionMatchesExplicitGoal(
	execution protocol.Execution,
	goal protocol.Goal,
) bool {
	return strings.TrimSpace(execution.GoalID) == strings.TrimSpace(goal.ID) &&
		execution.GoalObjectiveRevision == goal.ObjectiveRevision() &&
		execution.GoalActivationOrigin == protocol.GoalActivationOriginUserExplicit &&
		execution.GoalActivationReason == protocol.GoalActivationReasonPersistenceRequested
}

func goalRetargetIdentities(
	goalID string,
	oldRevision int64,
	objective string,
	providedCommandID string,
) (string, string, string) {
	seed := strings.Join([]string{
		strings.TrimSpace(goalID),
		strconv.FormatInt(oldRevision, 10),
		strings.TrimSpace(objective),
	}, "\x00")
	sum := sha256.Sum256([]byte(seed))
	suffix := hex.EncodeToString(sum[:12])
	commandID := strings.TrimSpace(providedCommandID)
	if commandID == "" {
		commandID = "goal_retarget_" + suffix
	}
	return commandID, "goal_transition_" + suffix, "execution_" + suffix
}

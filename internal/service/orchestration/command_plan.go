// INPUT: Plan 创建、替换与单调扩图。
// OUTPUT: 原子命令结果与后续责任动作。
// POS: Orchestration 命令的业务阶段，复用同包授权、版本栅栏与结果投影。
package orchestration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

// PlanExecutionInput 是 proposal materializer 使用的内部权威 primitive，不是 MCP 模型参数。
type PlanExecutionInput struct {
	ExecutionID      string
	SnapshotRevision int64
	CommandID        string
	// ReservedExecutionID 由 sealed proposal materializer 预留，用来保证
	// create/replace 在进程崩溃后的重放仍指向同一个 successor identity。
	// 普通进程内调用留空，由服务端照常生成 ID。
	ReservedExecutionID string
	// SealedGoalBinding 非 nil 时表示调用来自 immutable proposal：空 GoalID
	// 也是必须保持 Goal-free 的 exact fence，不能在提交时重新选择 ambient Goal。
	SealedGoalBinding       *ExplicitGoalBinding
	Objective               string
	CompletionCriteria      []string
	ReplaceCurrentExecution bool
	ReplacementReason       string
	// SupersedeActiveWork 明确授权当前 revision 原子释放尚未完成的责任链。
	// 未设置时，任何 current Assignment 都继续阻止 Plan replacement。
	SupersedeActiveWork bool
	Draft               PlanDraft
}

// PlanExecution 校验并原子激活一个新的 immutable Plan revision。
// 与当前 active Plan 语义完全相同的规范化 draft 直接 no-op；尚有 active work 时，
// replacement 必须显式 opt-in 并提供 revision reason。
func (s *Service) PlanExecution(
	ctx context.Context,
	actor ActorContext,
	input PlanExecutionInput,
) (returned MutationResult, returnedErr error) {
	defer func() { s.invalidateMutationResult(ctx, returned, returnedErr) }()
	if err := validateActor(actor); err != nil {
		return RejectedResult(nil, err, nil), nil
	}
	if strings.TrimSpace(input.CommandID) == "" {
		return RejectedResult(nil, domainError(ErrorCodeInvalidInput, "command_id is required"), nil), nil
	}
	draft, validateErr := NormalizeAndValidatePlanDraft(input.Draft)
	if validateErr != nil {
		var domainErr *DomainError
		if errors.As(validateErr, &domainErr) &&
			domainErr.Code == ErrorCodePlanItemsEmpty {
			return RejectedResult(nil, validateErr, []NextAction{{
				Domain: "execution", Operation: "prepare_plan_execution",
				Reason: "submit one complete Nexus Plan Document with every intended Work Item",
			}}), nil
		}
		return RejectedResult(nil, validateErr, nil), nil
	}
	var snapshot *protocol.ExecutionSnapshot
	var err error
	if strings.TrimSpace(input.ExecutionID) != "" {
		snapshot, err = s.GetSnapshot(ctx, actor, input.ExecutionID)
	} else {
		snapshot, err = s.GetCurrent(ctx, actor)
	}
	if err != nil {
		var domainErr *DomainError
		if errors.As(err, &domainErr) {
			return RejectedResult(nil, err, nil), nil
		}
		return MutationResult{}, err
	}
	if snapshot == nil {
		if input.ReplaceCurrentExecution {
			return RejectedResult(nil, domainError(
				ErrorCodeNoCurrentExecution,
				"operation: replace requires an explicit current Execution",
			), nil), nil
		}
		if input.SupersedeActiveWork {
			return RejectedResult(nil, domainError(
				ErrorCodeInvalidInput,
				"supersede_active_work is only valid for an existing Execution replan",
			), nil), nil
		}
		if coordinatorErr := requireExecutionCoordinator(actor); coordinatorErr != nil {
			return RejectedResult(nil, coordinatorErr, nil), nil
		}
		objective, criteria, boundaryErr := validateExecutionBoundary(
			input.Objective,
			input.CompletionCriteria,
		)
		if boundaryErr != nil {
			return RejectedResult(nil, boundaryErr, nil), nil
		}
		if actor.PlanMode {
			result := NoOpResult(
				nil,
				"Execution and Plan proposal is valid; Plan Mode created no authoritative state.",
			)
			result.NextActions = []NextAction{{
				Domain: "execution", Operation: "prepare_plan_execution",
				Reason: "seal the complete Plan proposal, then leave Plan Mode to commit its exact receipt",
			}}
			return result, nil
		}
		execution, buildExecutionErr := s.buildExecutionForPlan(
			ctx,
			actor,
			objective,
			criteria,
			"",
			input.ReservedExecutionID,
			input.SealedGoalBinding,
			true,
		)
		if buildExecutionErr != nil {
			return RejectedResult(nil, buildExecutionErr, nil), nil
		}
		initialSnapshot := &protocol.ExecutionSnapshot{Execution: execution}
		initialSnapshot.Execution.Version = 1
		initialInput := input
		initialInput.SnapshotRevision = 1
		command, buildErr := s.buildPlanCommand(actor, initialSnapshot, initialInput, draft)
		if buildErr != nil {
			return RejectedResult(nil, buildErr, nil), nil
		}
		updated, createErr := s.repository.CreateWithPlan(ctx, orchestrationstore.CreateWithPlanCommand{
			Execution: execution,
			Plan:      command,
			Meta:      s.commandMeta(actor, input.CommandID, "ensure"),
		})
		if createErr != nil {
			return s.storageMutationResult(nil, createErr, nil)
		}
		if confirmErr := s.confirmGoalExecutionBinding(ctx, updated); confirmErr != nil {
			return MutationResult{}, &GoalBindingConfirmationPendingError{
				Snapshot:        updated,
				DurableMutation: true,
				Err:             confirmErr,
			}
		}
		return s.activateRuntimeCoordinationResult(ctx, actor, AppliedResult(
			updated,
			planChangedEntities(updated),
			nextActions(updated, actor),
		)), nil
	}
	if authErr := requireCoordinator(actor, snapshot); authErr != nil {
		return RejectedResult(snapshot, authErr, nil), nil
	}
	terminal := !isCurrentExecutionStatus(snapshot.Execution.Status)
	if terminal && !input.ReplaceCurrentExecution {
		return RejectedResult(snapshot, terminalExecutionError(), nil), nil
	}
	if input.ReplaceCurrentExecution {
		if !terminal {
			if revisionErr := requireMutationRevision(snapshot, input.SnapshotRevision); revisionErr != nil {
				return RejectedResult(snapshot, revisionErr, nextActions(snapshot, actor)), nil
			}
		}
		objective, criteria, boundaryErr := validateReplacementBoundary(snapshot, input)
		if boundaryErr != nil {
			actions := []NextAction(nil)
			if domainErr := new(DomainError); errors.As(boundaryErr, &domainErr) &&
				domainErr.Code == ErrorCodeGoalRetargetRequired {
				actions = []NextAction{{
					Domain: "goal", Operation: "retarget_goal",
					Reason: "advance the Goal objective revision instead of replacing its bound Execution",
				}}
			}
			return RejectedResult(snapshot, boundaryErr, actions), nil
		}
		if actor.PlanMode {
			result := NoOpResult(
				snapshot,
				"Execution replacement proposal is valid; Plan Mode did not supersede or create authoritative state.",
			)
			result.NextActions = []NextAction{{
				Domain: "execution", Operation: "prepare_plan_execution",
				Reason: "seal an operation: replace document, then leave Plan Mode to commit its exact receipt",
			}}
			return result, nil
		}
		if isCurrentExecutionStatus(snapshot.Execution.Status) {
			if confirmErr := s.confirmGoalExecutionBinding(ctx, snapshot); confirmErr != nil {
				return MutationResult{}, &GoalBindingConfirmationPendingError{
					Snapshot: snapshot,
					Err:      confirmErr,
				}
			}
		}
		successor, buildExecutionErr := s.buildExecutionForPlan(
			ctx,
			actor,
			objective,
			criteria,
			snapshot.Execution.ID,
			input.ReservedExecutionID,
			nil,
			false,
		)
		if buildExecutionErr != nil {
			return RejectedResult(snapshot, buildExecutionErr, nil), nil
		}
		successorSnapshot := &protocol.ExecutionSnapshot{Execution: successor}
		successorSnapshot.Execution.Version = 1
		successorInput := input
		successorInput.SnapshotRevision = 1
		if strings.TrimSpace(draft.RevisionReason) == "" {
			draft.RevisionReason = strings.TrimSpace(input.ReplacementReason)
		}
		command, buildErr := s.buildPlanCommand(
			actor,
			successorSnapshot,
			successorInput,
			draft,
		)
		if buildErr != nil {
			return RejectedResult(snapshot, buildErr, nil), nil
		}
		updated, replaceErr := s.repository.ReplaceWithPlan(
			ctx,
			orchestrationstore.ReplaceWithPlanCommand{
				ExecutionID:              snapshot.Execution.ID,
				ExpectedExecutionVersion: input.SnapshotRevision,
				Successor:                successor,
				Plan:                     command,
				Reason:                   strings.TrimSpace(input.ReplacementReason),
				Meta:                     s.commandMeta(actor, input.CommandID, "replace"),
				SuccessorMeta:            s.commandMeta(actor, input.CommandID, "successor"),
			},
		)
		if replaceErr != nil {
			if terminal {
				return RejectedResult(snapshot, terminalExecutionError(), nil), nil
			}
			return s.storageMutationResult(snapshot, replaceErr, nextActions(snapshot, actor))
		}
		if terminal {
			return NoOpResult(updated, "Execution replacement was already committed by this command"), nil
		}
		return s.activateRuntimeCoordinationResult(ctx, actor, AppliedResult(
			updated,
			append(
				planChangedEntities(updated),
				"execution_superseded:"+snapshot.Execution.ID,
			),
			nextActions(updated, actor),
		)), nil
	}
	if boundaryErr := validateOrdinaryReplanBoundary(
		snapshot,
		input.Objective,
		input.CompletionCriteria,
	); boundaryErr != nil {
		return RejectedResult(snapshot, boundaryErr, []NextAction{{
			Domain: "execution", Operation: "prepare_plan_execution",
			Reason: "prepare an operation: replace document with replacement_reason, the new boundary, and the complete successor WorkGraph",
		}}), nil
	}
	if actor.PlanMode {
		result := NoOpResult(
			snapshot,
			"Plan proposal is valid; no authoritative state changed in Plan Mode. Resubmit after leaving Plan Mode to activate it.",
		)
		result.NextActions = []NextAction{{
			Domain: "execution", Operation: "prepare_plan_execution",
			Reason: "seal this complete replan document, then leave Plan Mode to commit its exact receipt",
		}}
		return result, nil
	}
	if isCurrentExecutionStatus(snapshot.Execution.Status) {
		if confirmErr := s.confirmGoalExecutionBinding(ctx, snapshot); confirmErr != nil {
			return MutationResult{}, &GoalBindingConfirmationPendingError{
				Snapshot: snapshot,
				Err:      confirmErr,
			}
		}
	}
	matches, matchErr := planDraftMatchesSnapshot(snapshot, draft)
	if matchErr != nil {
		return RejectedResult(snapshot, matchErr, nil), nil
	}
	if matches {
		result := NoOpResult(snapshot, "active Plan already matches the normalized proposal")
		result.NextActions = nextActions(snapshot, actor)
		return s.activateRuntimeCoordinationResult(ctx, actor, result), nil
	}
	if revisionErr := requireMutationRevision(snapshot, input.SnapshotRevision); revisionErr != nil {
		return RejectedResult(snapshot, revisionErr, nextActions(snapshot, actor)), nil
	}
	if hasUnreviewedSubmission(snapshot) {
		return RejectedResult(snapshot, domainError(
			ErrorCodeCompletionBlocked,
			"review pending submissions before replacing the active Plan",
		), nextActions(snapshot, actor)), nil
	}
	monotonicExtension, extensionErr := planDraftMonotonicallyExtendsSnapshot(
		snapshot,
		draft,
	)
	if extensionErr != nil {
		return RejectedResult(snapshot, extensionErr, nil), nil
	}
	hasActiveWork := false
	for _, assignment := range snapshot.Assignments {
		if currentAssignment(assignment) {
			hasActiveWork = true
			break
		}
	}
	if hasActiveWork && !input.SupersedeActiveWork {
		return RejectedResult(snapshot, domainError(
			ErrorCodeCompletionBlocked,
			"finish, review or take over current assignments before replacing the active Plan, or explicitly authorize superseding active work",
		), nextActions(snapshot, actor)), nil
	}
	if !monotonicExtension && !input.SupersedeActiveWork {
		return RejectedResult(snapshot, domainError(
			ErrorCodeCompletionBlocked,
			"removing or changing an existing Plan node or dependency requires supersede_active_work=true and a non-empty revision_reason; ordinary replan may only append nodes and downstream edges",
		), nextActions(snapshot, actor)), nil
	}
	if input.SupersedeActiveWork && draft.RevisionReason == "" {
		return RejectedResult(snapshot, domainError(
			ErrorCodeInvalidInput,
			"revision_reason is required when supersede_active_work is true",
		), nil), nil
	}
	command, buildErr := s.buildPlanCommand(actor, snapshot, input, draft)
	if buildErr != nil {
		return RejectedResult(snapshot, buildErr, nil), nil
	}
	updated, writeErr := s.repository.WritePlan(ctx, command)
	if writeErr != nil {
		return s.storageMutationResult(snapshot, writeErr, nextActions(snapshot, actor))
	}
	changed := []string{"plan:" + command.Plan.ID}
	for _, item := range command.WorkItems {
		changed = append(changed, "work_item:"+item.WorkItem.ID)
	}
	return s.activateRuntimeCoordinationResult(
		ctx,
		actor,
		AppliedResult(updated, changed, nextActions(updated, actor)),
	), nil
}

func planChangedEntities(snapshot *protocol.ExecutionSnapshot) []string {
	if snapshot == nil {
		return nil
	}
	changed := []string{"execution:" + snapshot.Execution.ID}
	if snapshot.Plan != nil {
		changed = append(changed, "plan:"+snapshot.Plan.ID)
	}
	for _, item := range snapshot.WorkItems {
		changed = append(changed, "work_item:"+item.ID)
	}
	return changed
}

func (s *Service) buildPlanCommand(
	actor ActorContext,
	snapshot *protocol.ExecutionSnapshot,
	input PlanExecutionInput,
	draft PlanDraft,
) (orchestrationstore.WritePlanCommand, error) {
	executionID := snapshot.Execution.ID
	planID := s.id("plan")
	revision := int64(1)
	basePlanID := ""
	if snapshot.Plan != nil {
		revision = snapshot.Plan.Revision + 1
		basePlanID = snapshot.Plan.ID
	}
	byLogicalKey := make(map[string]protocol.WorkItem, len(snapshot.WorkItems))
	byID := make(map[string]protocol.WorkItem, len(snapshot.WorkItems))
	stateByWork := make(map[string]protocol.WorkItemState, len(snapshot.WorkItemStates))
	specByID := make(map[string]protocol.WorkItemSpec, len(snapshot.WorkItemSpecs))
	for _, item := range snapshot.WorkItems {
		byLogicalKey[item.LogicalKey] = item
		byID[item.ID] = item
	}
	for _, state := range snapshot.WorkItemStates {
		stateByWork[state.WorkItemID] = state
	}
	for _, spec := range snapshot.WorkItemSpecs {
		specByID[spec.ID] = spec
	}

	workByLogicalKey := make(map[string]orchestrationstore.PlanWorkItem, len(draft.Items))
	for position, itemDraft := range draft.Items {
		stable, exists := byLogicalKey[itemDraft.LogicalKey]
		if itemDraft.ExistingWorkItemID != "" {
			explicit, explicitExists := byID[itemDraft.ExistingWorkItemID]
			if !explicitExists {
				return orchestrationstore.WritePlanCommand{}, newDomainError(
					ErrorCodeInvalidInput,
					"existing_work_item_id is outside this Execution",
					itemDraft.LogicalKey,
					itemDraft.ExistingWorkItemID,
				)
			}
			if exists && stable.ID != explicit.ID {
				return orchestrationstore.WritePlanCommand{}, newDomainError(
					ErrorCodeDuplicateLogicalKey,
					"logical_key and existing_work_item_id identify different Work Items",
					itemDraft.LogicalKey,
					itemDraft.ExistingWorkItemID,
				)
			}
			stable = explicit
			exists = true
		}
		if exists {
			if stable.LogicalKey != itemDraft.LogicalKey || stable.Kind != itemDraft.Kind {
				return orchestrationstore.WritePlanCommand{}, newDomainError(
					ErrorCodeInvalidInput,
					"stable Work Item logical_key and kind are immutable",
					itemDraft.LogicalKey,
					stable.ID,
				)
			}
		} else {
			stable = protocol.WorkItem{
				ID:          s.id("work"),
				ExecutionID: executionID,
				LogicalKey:  itemDraft.LogicalKey,
				Kind:        itemDraft.Kind,
			}
		}

		hash, hashErr := workSpecHash(itemDraft)
		if hashErr != nil {
			return orchestrationstore.WritePlanCommand{}, domainError(
				ErrorCodeInvalidInput,
				"work item spec cannot be encoded",
			)
		}
		state, hasState := stateByWork[stable.ID]
		specVersion := int64(1)
		specID := ""
		expectedStateVersion := int64(0)
		if hasState {
			expectedStateVersion = state.Version
			if currentSpec, ok := specByID[state.CurrentSpecID]; ok {
				specVersion = currentSpec.Version + 1
				if currentSpec.SpecHash == hash {
					specID = currentSpec.ID
					specVersion = currentSpec.Version
				}
			}
		}
		if specID == "" {
			specID = s.id("spec")
		}
		spec := protocol.WorkItemSpec{
			ID:                 specID,
			WorkItemID:         stable.ID,
			ExecutionID:        executionID,
			Version:            specVersion,
			Subject:            itemDraft.Subject,
			Objective:          itemDraft.Objective,
			Deliverable:        itemDraft.Deliverable,
			AcceptanceCriteria: slices.Clone(itemDraft.AcceptanceCriteria),
			InputRefs:          slices.Clone(itemDraft.InputRefs),
			SpecHash:           hash,
			CreatedByAgentID:   strings.TrimSpace(actor.AgentID),
		}
		nextState := protocol.WorkItemState{
			WorkItemID:    stable.ID,
			ExecutionID:   executionID,
			CurrentSpecID: specID,
			Status:        protocol.WorkItemStatusOpen,
			Version:       1,
		}
		if hasState {
			nextState.Version = state.Version
			if state.CurrentSpecID == specID {
				nextState = state
			}
		}
		claims := make([]protocol.ExecutionPlanOutputClaim, 0, len(itemDraft.OutputScopes))
		for _, scope := range itemDraft.OutputScopes {
			claims = append(claims, protocol.ExecutionPlanOutputClaim{
				Scope: scope.Scope,
				Mode:  scope.Mode,
			})
		}
		workByLogicalKey[itemDraft.LogicalKey] = orchestrationstore.PlanWorkItem{
			WorkItem: stable,
			Spec:     spec,
			State:    nextState,
			Item: protocol.ExecutionPlanItem{
				Required: itemDraft.Required,
				Terminal: itemDraft.Terminal,
				Position: position,
			},
			OutputClaims:         claims,
			ExpectedStateVersion: expectedStateVersion,
		}
	}

	workItems := make([]orchestrationstore.PlanWorkItem, 0, len(draft.Items))
	dependencies := make([]protocol.ExecutionPlanDependency, 0)
	for _, itemDraft := range draft.Items {
		work := workByLogicalKey[itemDraft.LogicalKey]
		work.Item.PlanID = planID
		work.Item.ExecutionID = executionID
		work.Item.WorkItemID = work.WorkItem.ID
		work.Item.SpecID = work.Spec.ID
		if itemDraft.ParentLogicalKey != "" {
			work.Item.ParentWorkItemID = workByLogicalKey[itemDraft.ParentLogicalKey].WorkItem.ID
		}
		workItems = append(workItems, work)
		for _, dependency := range itemDraft.DependsOn {
			dependencies = append(dependencies, protocol.ExecutionPlanDependency{
				PlanID:              planID,
				ExecutionID:         executionID,
				WorkItemID:          work.WorkItem.ID,
				DependsOnWorkItemID: workByLogicalKey[dependency.LogicalKey].WorkItem.ID,
				Kind:                dependency.Kind,
			})
		}
	}
	return orchestrationstore.WritePlanCommand{
		ExecutionID:              executionID,
		ExpectedExecutionVersion: input.SnapshotRevision,
		Plan: protocol.ExecutionPlanRevision{
			ID:               planID,
			ExecutionID:      executionID,
			Revision:         revision,
			Status:           protocol.PlanRevisionStatusActive,
			BasePlanID:       basePlanID,
			CreatedByAgentID: strings.TrimSpace(actor.AgentID),
			RevisionReason:   draft.RevisionReason,
		},
		WorkItems:           workItems,
		Dependencies:        dependencies,
		SupersedeActiveWork: input.SupersedeActiveWork,
		Meta:                s.commandMeta(actor, input.CommandID, "plan"),
	}, nil
}

func workSpecHash(item PlanWorkItemDraft) (string, error) {
	criteria := append([]string{}, item.AcceptanceCriteria...)
	inputRefs := append([]string{}, item.InputRefs...)
	outputScopes := append([]protocol.WorkOutputScope{}, item.OutputScopes...)
	payload := struct {
		Subject            string                     `json:"subject"`
		Objective          string                     `json:"objective"`
		Deliverable        string                     `json:"deliverable"`
		AcceptanceCriteria []string                   `json:"acceptance_criteria"`
		InputRefs          []string                   `json:"input_refs"`
		OutputScopes       []protocol.WorkOutputScope `json:"output_scopes"`
	}{
		Subject:            item.Subject,
		Objective:          item.Objective,
		Deliverable:        item.Deliverable,
		AcceptanceCriteria: criteria,
		InputRefs:          inputRefs,
		OutputScopes:       outputScopes,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func planDraftMatchesSnapshot(
	snapshot *protocol.ExecutionSnapshot,
	draft PlanDraft,
) (bool, error) {
	if snapshot == nil || snapshot.Plan == nil ||
		snapshot.Plan.Status != protocol.PlanRevisionStatusActive ||
		len(snapshot.PlanItems) != len(draft.Items) ||
		len(snapshot.WorkItems) != len(draft.Items) {
		return false, nil
	}
	workByLogicalKey := make(map[string]protocol.WorkItem, len(snapshot.WorkItems))
	workByID := make(map[string]protocol.WorkItem, len(snapshot.WorkItems))
	for _, work := range snapshot.WorkItems {
		workByLogicalKey[work.LogicalKey] = work
		workByID[work.ID] = work
	}
	itemByWorkID := make(map[string]protocol.ExecutionPlanItem, len(snapshot.PlanItems))
	for _, item := range snapshot.PlanItems {
		itemByWorkID[item.WorkItemID] = item
	}
	specByID := make(map[string]protocol.WorkItemSpec, len(snapshot.WorkItemSpecs))
	for _, spec := range snapshot.WorkItemSpecs {
		specByID[spec.ID] = spec
	}
	dependenciesByWorkID := make(map[string][]protocol.ExecutionPlanDependency)
	for _, dependency := range snapshot.Dependencies {
		dependenciesByWorkID[dependency.WorkItemID] = append(
			dependenciesByWorkID[dependency.WorkItemID],
			dependency,
		)
	}
	claimsByWorkID := make(map[string][]protocol.ExecutionPlanOutputClaim)
	for _, claim := range snapshot.OutputClaims {
		claimsByWorkID[claim.WorkItemID] = append(claimsByWorkID[claim.WorkItemID], claim)
	}
	for position, candidate := range draft.Items {
		work, exists := workByLogicalKey[candidate.LogicalKey]
		if !exists || work.Kind != candidate.Kind ||
			(candidate.ExistingWorkItemID != "" && candidate.ExistingWorkItemID != work.ID) {
			return false, nil
		}
		item, exists := itemByWorkID[work.ID]
		if !exists || item.Position != position ||
			item.Required != candidate.Required ||
			item.Terminal != candidate.Terminal {
			return false, nil
		}
		parentLogicalKey := ""
		if item.ParentWorkItemID != "" {
			parent, ok := workByID[item.ParentWorkItemID]
			if !ok {
				return false, nil
			}
			parentLogicalKey = parent.LogicalKey
		}
		if parentLogicalKey != candidate.ParentLogicalKey {
			return false, nil
		}
		spec, exists := specByID[item.SpecID]
		if !exists {
			return false, nil
		}
		hash, err := workSpecHash(candidate)
		if err != nil {
			return false, domainError(ErrorCodeInvalidInput, "work item spec cannot be encoded")
		}
		if spec.SpecHash != hash {
			return false, nil
		}
		persistedClaims := claimsByWorkID[work.ID]
		if len(persistedClaims) != len(candidate.OutputScopes) {
			return false, nil
		}
		persistedClaimModes := make(map[string]protocol.WorkOutputScopeMode, len(persistedClaims))
		for _, claim := range persistedClaims {
			persistedClaimModes[claim.Scope] = claim.Mode
		}
		for _, claim := range candidate.OutputScopes {
			if persistedClaimModes[claim.Scope] != claim.Mode {
				return false, nil
			}
		}
		persistedDependencies := dependenciesByWorkID[work.ID]
		if len(persistedDependencies) != len(candidate.DependsOn) {
			return false, nil
		}
		persistedByLogicalKey := make(map[string]protocol.WorkDependencyKind, len(persistedDependencies))
		for _, dependency := range persistedDependencies {
			upstream, ok := workByID[dependency.DependsOnWorkItemID]
			if !ok {
				return false, nil
			}
			persistedByLogicalKey[upstream.LogicalKey] = dependency.Kind
		}
		for _, dependency := range candidate.DependsOn {
			if persistedByLogicalKey[dependency.LogicalKey] != dependency.Kind {
				return false, nil
			}
		}
	}
	return true, nil
}

func planDraftMonotonicallyExtendsSnapshot(
	snapshot *protocol.ExecutionSnapshot,
	draft PlanDraft,
) (bool, error) {
	if snapshot == nil || snapshot.Plan == nil ||
		snapshot.Plan.Status != protocol.PlanRevisionStatusActive {
		return true, nil
	}
	workByID := make(map[string]protocol.WorkItem, len(snapshot.WorkItems))
	for _, work := range snapshot.WorkItems {
		workByID[work.ID] = work
	}
	specByID := make(map[string]protocol.WorkItemSpec, len(snapshot.WorkItemSpecs))
	for _, spec := range snapshot.WorkItemSpecs {
		specByID[spec.ID] = spec
	}
	dependenciesByWorkID := make(
		map[string][]protocol.ExecutionPlanDependency,
		len(snapshot.Dependencies),
	)
	for _, dependency := range snapshot.Dependencies {
		if dependency.PlanID == snapshot.Plan.ID {
			dependenciesByWorkID[dependency.WorkItemID] = append(
				dependenciesByWorkID[dependency.WorkItemID],
				dependency,
			)
		}
	}
	candidates := make(map[string]PlanWorkItemDraft, len(draft.Items))
	for _, candidate := range draft.Items {
		candidates[candidate.LogicalKey] = candidate
	}

	for _, item := range snapshot.PlanItems {
		if item.PlanID != snapshot.Plan.ID {
			continue
		}
		work, exists := workByID[item.WorkItemID]
		if !exists {
			return false, nil
		}
		candidate, exists := candidates[work.LogicalKey]
		if !exists ||
			candidate.Kind != work.Kind ||
			(candidate.ExistingWorkItemID != "" &&
				candidate.ExistingWorkItemID != work.ID) ||
			candidate.Required != item.Required ||
			candidate.Terminal != item.Terminal {
			return false, nil
		}
		parentLogicalKey := ""
		if item.ParentWorkItemID != "" {
			parent, ok := workByID[item.ParentWorkItemID]
			if !ok {
				return false, nil
			}
			parentLogicalKey = parent.LogicalKey
		}
		if candidate.ParentLogicalKey != parentLogicalKey {
			return false, nil
		}
		spec, exists := specByID[item.SpecID]
		if !exists {
			return false, nil
		}
		hash, err := workSpecHash(candidate)
		if err != nil {
			return false, domainError(
				ErrorCodeInvalidInput,
				"work item spec cannot be encoded",
			)
		}
		if hash != spec.SpecHash {
			return false, nil
		}
		persistedDependencies := dependenciesByWorkID[work.ID]
		if len(candidate.DependsOn) != len(persistedDependencies) {
			return false, nil
		}
		persistedByLogicalKey := make(
			map[string]protocol.WorkDependencyKind,
			len(persistedDependencies),
		)
		for _, dependency := range persistedDependencies {
			upstream, ok := workByID[dependency.DependsOnWorkItemID]
			if !ok {
				return false, nil
			}
			persistedByLogicalKey[upstream.LogicalKey] = dependency.Kind
		}
		for _, dependency := range candidate.DependsOn {
			if persistedByLogicalKey[dependency.LogicalKey] != dependency.Kind {
				return false, nil
			}
		}
	}
	return true, nil
}

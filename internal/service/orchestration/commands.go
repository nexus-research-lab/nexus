// INPUT: sealed proposal materializer 的内部 Plan primitive，以及模型的 Assignment、Submission、Acceptance、Block/Resume、Takeover 与 complete 意图。
// OUTPUT: 服务端 mint ID、logical-key 解析、单调 Plan 扩图、透明 Attempt 状态机、Acceptance 后同轮协调衔接、显式 Plan replacement、统一 MutationResult 与提交后 Execution 失效事实。
// POS: 模型语义 command 到 Repository 原子 command 的应用层适配；不暴露 start_work。
package orchestration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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

// AbandonExecutionInput 明确取消一个 transient Execution，不创建 successor。
type AbandonExecutionInput struct {
	ExecutionID      string
	SnapshotRevision int64
	CommandID        string
	Reason           string
}

// AssignWorkInput 把 Ready Work Item 交给一个责任 Agent。
type AssignWorkInput struct {
	ExecutionID      string
	SnapshotRevision int64
	CommandID        string
	WorkItemID       string
	LogicalKey       string
	TargetAgentID    string
	ReturnToAgentID  string
	Strategy         protocol.AssignmentStrategy
	Reason           string
	Instruction      string
	DispatchKind     protocol.ExecutionDispatchKind
}

// SubmitWorkInput 是 Assignment owner 对当前 immutable spec 的完成声明。
type SubmitWorkInput struct {
	ExecutionID       string
	SnapshotRevision  int64
	CommandID         string
	WorkItemID        string
	LogicalKey        string
	AssignmentID      string
	ResultSummary     string
	ResultRefs        []string
	Evidence          []string
	RuntimeSessionKey string
	RoomSessionID     string
	SDKSessionID      string
	ToolUseID         string
}

// ReviewWorkInput 是 Assignment 选定 reviewer 对 immutable Submission 的唯一 decision。
type ReviewWorkInput struct {
	ExecutionID      string
	SnapshotRevision int64
	CommandID        string
	SubmissionID     string
	WorkItemID       string
	LogicalKey       string
	Decision         protocol.WorkAcceptanceDecision
	CriteriaResults  []protocol.WorkAcceptanceCriterionResult
	Feedback         string
}

// BlockWorkInput 只记录确定的外部输入阻塞，不复制依赖阻塞。
type BlockWorkInput struct {
	ExecutionID      string
	SnapshotRevision int64
	CommandID        string
	WorkItemID       string
	LogicalKey       string
	Reason           string
	NeededInput      string
}

// ResumeWorkInput 以 resolution/evidence 关闭显式 waiting_input，并重新开放当前 spec。
type ResumeWorkInput struct {
	ExecutionID      string
	SnapshotRevision int64
	CommandID        string
	WorkItemID       string
	LogicalKey       string
	Resolution       string
	Evidence         []string
}

// TakeOverWorkInput 由 coordinator 原子替换当前责任 Agent。
type TakeOverWorkInput struct {
	ExecutionID      string
	SnapshotRevision int64
	CommandID        string
	WorkItemID       string
	LogicalKey       string
	TargetAgentID    string
	ReturnToAgentID  string
	Strategy         protocol.AssignmentStrategy
	Reason           string
	Instruction      string
	DispatchKind     protocol.ExecutionDispatchKind
}

// CompleteExecutionInput 触发一次权威 completion audit。
type CompleteExecutionInput struct {
	ExecutionID      string
	SnapshotRevision int64
	CommandID        string
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
				Tool:   "prepare_plan_execution",
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
				Tool:   "prepare_plan_execution",
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
					Tool:   "retarget_goal",
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
				Tool:   "prepare_plan_execution",
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
			Tool:   "prepare_plan_execution",
			Reason: "prepare an operation: replace document with replacement_reason, the new boundary, and the complete successor WorkGraph",
		}}), nil
	}
	if actor.PlanMode {
		result := NoOpResult(
			snapshot,
			"Plan proposal is valid; no authoritative state changed in Plan Mode. Resubmit after leaving Plan Mode to activate it.",
		)
		result.NextActions = []NextAction{{
			Tool:   "prepare_plan_execution",
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

// AssignWork 创建 current Assignment、可选 Room dispatch 和 pending root Attempt。
func (s *Service) AssignWork(
	ctx context.Context,
	actor ActorContext,
	input AssignWorkInput,
) (returned MutationResult, returnedErr error) {
	defer func() { s.invalidateMutationResult(ctx, returned, returnedErr) }()
	snapshot, rejected, err := s.mutableSnapshot(
		ctx,
		actor,
		input.ExecutionID,
		input.SnapshotRevision,
		true,
		false,
	)
	if err != nil || rejected != nil {
		return resultOrZero(rejected), err
	}
	if strings.TrimSpace(input.CommandID) == "" {
		return RejectedResult(snapshot, domainError(ErrorCodeInvalidInput, "command_id is required"), nil), nil
	}
	work, spec, resolveErr := resolvePlanWork(snapshot, input.WorkItemID, input.LogicalKey)
	if resolveErr != nil {
		return RejectedResult(snapshot, resolveErr, nil), nil
	}
	if assignment := activeAssignmentForWork(snapshot, work.ID); assignment != nil {
		if matchingRoomSelfAssignmentRequest(actor, snapshot, assignment, input) {
			result := NoOpResult(snapshot, "work is already assigned to the current Room actor")
			return withRoomSelfWorkBindingReceipt(actor, result, work.ID), nil
		}
		return RejectedResult(snapshot, newDomainError(
			ErrorCodeDuplicateAssignment,
			"Work Item already has a current Assignment",
			work.LogicalKey,
			assignment.ID,
		), nextActions(snapshot, actor)), nil
	}
	if !slices.Contains(snapshot.ReadyWorkItemIDs, work.ID) {
		return RejectedResult(snapshot, newDomainError(
			ErrorCodeDependencyNotAccepted,
			"Work Item is not ready; inspect hard dependencies and lifecycle state",
			work.LogicalKey,
			"",
		), nextActions(snapshot, actor)), nil
	}
	target := strings.TrimSpace(input.TargetAgentID)
	if target == "" {
		return RejectedResult(snapshot, domainError(
			ErrorCodeInvalidInput,
			"target_agent_id is required",
		), nil), nil
	}
	assignment, dispatch, attempt, buildErr := s.buildAssignmentChain(
		actor,
		snapshot,
		work,
		spec,
		target,
		input.ReturnToAgentID,
		input.Strategy,
		input.Reason,
		"",
		input.Instruction,
		input.DispatchKind,
		input.CommandID,
	)
	if buildErr != nil {
		return RejectedResult(snapshot, buildErr, nil), nil
	}
	if targetErr := s.authorizeAssignmentTarget(ctx, actor, snapshot, assignment, dispatch); targetErr != nil {
		return RejectedResult(snapshot, targetErr, nextActions(snapshot, actor)), nil
	}
	updated, assignErr := s.repository.Assign(ctx, orchestrationstore.AssignCommand{
		ExpectedExecutionVersion: input.SnapshotRevision,
		Assignment:               assignment,
		Dispatch:                 dispatch,
		RootAttempt:              &attempt,
		Meta:                     s.commandMeta(actor, input.CommandID, "assign"),
	})
	if assignErr != nil {
		return s.storageMutationResult(snapshot, assignErr, nextActions(snapshot, actor))
	}
	changed := []string{
		"assignment:" + assignment.ID,
		"attempt:" + attempt.ID,
	}
	if dispatch != nil {
		changed = append(changed, "dispatch:"+dispatch.ID)
		if s.dispatchConsumer != nil {
			// Assignment 已原子持久化；投递失败只保留 pending retry，
			// 不能把一个已提交的 command 伪装成未应用。
			_, _ = s.DispatchPending(ctx, "assign:"+input.CommandID, 8)
			if refreshed, refreshErr := s.repository.GetSnapshot(ctx, snapshot.Execution.ID); refreshErr == nil && refreshed != nil {
				updated = refreshed
			}
		}
	}
	result := AppliedResult(updated, changed, nextActions(updated, actor))
	return withRoomSelfWorkBindingReceipt(actor, result, work.ID), nil
}

func matchingRoomSelfAssignmentRequest(
	actor ActorContext,
	snapshot *protocol.ExecutionSnapshot,
	assignment *protocol.WorkAssignment,
	input AssignWorkInput,
) bool {
	if snapshot == nil || assignment == nil ||
		snapshot.Execution.ScopeKind != protocol.ExecutionScopeRoom ||
		actor.ScopeKind != protocol.ExecutionScopeRoom ||
		actor.WorkBinding != nil || actor.ReviewBinding != nil ||
		assignment.Strategy != protocol.AssignmentStrategySelf ||
		strings.TrimSpace(assignment.OwnerAgentID) != strings.TrimSpace(actor.AgentID) ||
		strings.TrimSpace(input.TargetAgentID) != strings.TrimSpace(actor.AgentID) ||
		(input.Strategy != "" && input.Strategy != protocol.AssignmentStrategySelf) ||
		input.DispatchKind != "" {
		return false
	}
	returnTo := strings.TrimSpace(input.ReturnToAgentID)
	if returnTo == "" {
		returnTo = strings.TrimSpace(snapshot.Execution.CoordinatorAgentID)
	}
	return returnTo != "" &&
		returnTo == strings.TrimSpace(assignment.ReturnToAgentID)
}

func withRoomSelfWorkBindingReceipt(
	actor ActorContext,
	result MutationResult,
	workItemID string,
) MutationResult {
	if result.Snapshot == nil ||
		result.Snapshot.Execution.ScopeKind != protocol.ExecutionScopeRoom ||
		actor.ScopeKind != protocol.ExecutionScopeRoom ||
		actor.WorkBinding != nil ||
		actor.ReviewBinding != nil {
		return result
	}
	assignment := activeAssignmentForWork(result.Snapshot, strings.TrimSpace(workItemID))
	attempt := rootAttemptForAssignment(result.Snapshot, assignment)
	if assignment == nil || attempt == nil || result.Snapshot.Plan == nil ||
		assignment.Strategy != protocol.AssignmentStrategySelf ||
		strings.TrimSpace(assignment.OwnerAgentID) != strings.TrimSpace(actor.AgentID) ||
		assignment.ID != attempt.AssignmentID ||
		assignment.ExecutionID != result.Snapshot.Execution.ID ||
		assignment.PlanID != result.Snapshot.Plan.ID ||
		assignment.WorkItemID != attempt.WorkItemID ||
		assignment.SpecID != attempt.SpecID ||
		strings.TrimSpace(attempt.DispatchID) != "" ||
		attempt.ParentAttemptID != "" {
		return result
	}
	result.WorkBinding = &WorkBindingReceipt{Binding: &protocol.ExecutionWorkBinding{
		ExecutionID:  assignment.ExecutionID,
		PlanID:       assignment.PlanID,
		WorkItemID:   assignment.WorkItemID,
		SpecID:       assignment.SpecID,
		AssignmentID: assignment.ID,
		AttemptID:    attempt.ID,
	}}
	return result
}

func rootAttemptForAssignment(
	snapshot *protocol.ExecutionSnapshot,
	assignment *protocol.WorkAssignment,
) *protocol.WorkAttempt {
	if snapshot == nil || assignment == nil {
		return nil
	}
	var succeeded *protocol.WorkAttempt
	for index := range snapshot.Attempts {
		attempt := &snapshot.Attempts[index]
		if attempt.AssignmentID != assignment.ID || attempt.ParentAttemptID != "" {
			continue
		}
		if attempt.Status == protocol.WorkAttemptStatusPending ||
			attempt.Status == protocol.WorkAttemptStatusRunning {
			return attempt
		}
		if attempt.Status == protocol.WorkAttemptStatusSucceeded &&
			(succeeded == nil || attempt.CreatedAt.After(succeeded.CreatedAt)) {
			succeeded = attempt
		}
	}
	return succeeded
}

// SubmitWork 透明启动并成功结束当前 root Attempt，然后记录 Submission。
func (s *Service) SubmitWork(
	ctx context.Context,
	actor ActorContext,
	input SubmitWorkInput,
) (returned MutationResult, returnedErr error) {
	defer func() { s.invalidateMutationResult(ctx, returned, returnedErr) }()
	snapshot, rejected, err := s.mutableSnapshot(
		ctx,
		actor,
		input.ExecutionID,
		input.SnapshotRevision,
		false,
		false,
	)
	if err != nil || rejected != nil {
		return resultOrZero(rejected), err
	}
	if strings.TrimSpace(input.CommandID) == "" || strings.TrimSpace(input.ResultSummary) == "" {
		return RejectedResult(snapshot, domainError(
			ErrorCodeInvalidInput,
			"command_id and result_summary are required",
		), nil), nil
	}
	for _, collection := range []struct {
		field string
		count int
	}{
		{field: "result_refs", count: len(input.ResultRefs)},
		{field: "submission_evidence", count: len(input.Evidence)},
	} {
		if limitErr := newProjectionLimitError(collection.field, collection.count, ""); limitErr != nil {
			return RejectedResult(snapshot, limitErr, nil), nil
		}
	}
	work, _, resolveErr := resolvePlanWork(snapshot, input.WorkItemID, input.LogicalKey)
	if resolveErr != nil {
		return RejectedResult(snapshot, resolveErr, nil), nil
	}
	assignment := selectAssignment(snapshot, work.ID, input.AssignmentID)
	if assignment == nil || !currentAssignment(*assignment) {
		if submission := latestUnreviewedSubmission(snapshot, work.ID); submission != nil &&
			submission.SubmitterAgentID == strings.TrimSpace(actor.AgentID) {
			return NoOpResult(snapshot, "work is already submitted and awaiting review"), nil
		}
		return RejectedResult(snapshot, newDomainError(
			ErrorCodeWrongOwner,
			"no current Assignment exists for this actor and Work Item",
			work.LogicalKey,
			"",
		), nil), nil
	}
	if assignment.OwnerAgentID != strings.TrimSpace(actor.AgentID) {
		return RejectedResult(snapshot, newDomainError(
			ErrorCodeWrongOwner,
			"only the current Assignment owner may submit work",
			work.LogicalKey,
			assignment.OwnerAgentID,
		), nil), nil
	}
	if submission := latestUnreviewedSubmission(snapshot, work.ID); submission != nil {
		return NoOpResult(snapshot, "work is already submitted and awaiting review"), nil
	}
	state := findStateByWorkID(snapshot, work.ID)
	if state == nil || state.CurrentSpecID != assignment.SpecID {
		return RejectedResult(snapshot, domainError(
			ErrorCodeInvalidInput,
			"current Work Item state/spec fence is missing",
		), nil), nil
	}
	if state.Status != protocol.WorkItemStatusOpen {
		return RejectedResult(snapshot, newDomainError(
			ErrorCodeCompletionBlocked,
			"Work Item must be resumed before it can create a new Attempt or Submission",
			work.LogicalKey,
			string(state.Status),
		), nextActions(snapshot, actor)), nil
	}

	attempt := currentOrSucceededAttempt(snapshot, assignment.ID)
	if attempt == nil {
		created := protocol.WorkAttempt{
			ID:              s.id("attempt"),
			ExecutionID:     assignment.ExecutionID,
			PlanID:          assignment.PlanID,
			WorkItemID:      assignment.WorkItemID,
			SpecID:          assignment.SpecID,
			AssignmentID:    assignment.ID,
			ExecutorKind:    protocol.AttemptExecutorAgent,
			ExecutorAgentID: assignment.OwnerAgentID,
			Status:          protocol.WorkAttemptStatusRunning,
		}
		updated, startErr := s.repository.StartAttempt(ctx, orchestrationstore.StartAttemptCommand{
			ExpectedExecutionVersion:  snapshot.Execution.Version,
			ExpectedAssignmentVersion: assignment.Version,
			ExpectedAttemptVersion:    0,
			Attempt:                   mergeSubmissionRuntime(created, actor, input),
			Meta:                      s.commandMeta(actor, input.CommandID, "submit-start"),
		})
		if startErr != nil {
			return s.storageMutationResult(snapshot, startErr, nextActions(snapshot, actor))
		}
		s.invalidateSnapshot(ctx, updated)
		snapshot = updated
		assignment = findAssignmentByID(snapshot, assignment.ID)
		attempt = currentOrSucceededAttempt(snapshot, assignment.ID)
	} else if attempt.Status == protocol.WorkAttemptStatusPending {
		updated, startErr := s.repository.StartAttempt(ctx, orchestrationstore.StartAttemptCommand{
			ExpectedExecutionVersion:  snapshot.Execution.Version,
			ExpectedAssignmentVersion: assignment.Version,
			ExpectedAttemptVersion:    attempt.Version,
			Attempt:                   mergeSubmissionRuntime(*attempt, actor, input),
			Meta:                      s.commandMeta(actor, input.CommandID, "submit-start"),
		})
		if startErr != nil {
			return s.storageMutationResult(snapshot, startErr, nextActions(snapshot, actor))
		}
		s.invalidateSnapshot(ctx, updated)
		snapshot = updated
		assignment = findAssignmentByID(snapshot, assignment.ID)
		attempt = findAttemptByID(snapshot, attempt.ID)
	}
	if assignment == nil || attempt == nil {
		return MutationResult{}, fmt.Errorf("repository returned an incomplete Assignment/Attempt snapshot")
	}
	if attempt.Status == protocol.WorkAttemptStatusRunning {
		terminal := mergeSubmissionRuntime(*attempt, actor, input)
		terminal.Status = protocol.WorkAttemptStatusSucceeded
		updated, finishErr := s.repository.FinishAttempt(ctx, orchestrationstore.FinishAttemptCommand{
			ExpectedExecutionVersion: snapshot.Execution.Version,
			ExpectedAttemptVersion:   attempt.Version,
			Attempt:                  terminal,
			Meta:                     s.commandMeta(actor, input.CommandID, "submit-finish"),
		})
		if finishErr != nil {
			return s.storageMutationResult(snapshot, finishErr, nextActions(snapshot, actor))
		}
		s.invalidateSnapshot(ctx, updated)
		snapshot = updated
		assignment = findAssignmentByID(snapshot, assignment.ID)
		attempt = findAttemptByID(snapshot, attempt.ID)
	}
	if assignment == nil || attempt == nil || attempt.Status != protocol.WorkAttemptStatusSucceeded {
		return RejectedResult(snapshot, newDomainError(
			ErrorCodeDuplicateAttempt,
			"current Attempt is terminal without success; refresh Execution context and use an allowed coordinator recovery action to create a fresh Attempt before submitting again",
			work.LogicalKey,
			"",
		), nextActions(snapshot, actor)), nil
	}
	submissionRecord := protocol.WorkSubmission{
		ID:               s.id("submission"),
		ExecutionID:      assignment.ExecutionID,
		PlanID:           assignment.PlanID,
		WorkItemID:       assignment.WorkItemID,
		SpecID:           assignment.SpecID,
		AssignmentID:     assignment.ID,
		AttemptID:        attempt.ID,
		SubmitterAgentID: strings.TrimSpace(actor.AgentID),
		ResultSummary:    strings.TrimSpace(input.ResultSummary),
		ResultRefs:       normalizeNonEmptyValues(input.ResultRefs),
		Evidence:         normalizeNonEmptyValues(input.Evidence),
	}
	var reviewDispatch *protocol.ExecutionReviewDispatch
	if needsReviewDispatch(snapshot, assignment) {
		reviewDispatch = &protocol.ExecutionReviewDispatch{
			ID:            s.id("review_dispatch"),
			ExecutionID:   assignment.ExecutionID,
			PlanID:        assignment.PlanID,
			WorkItemID:    assignment.WorkItemID,
			SpecID:        assignment.SpecID,
			AssignmentID:  assignment.ID,
			SubmissionID:  submissionRecord.ID,
			DedupeKey:     "review-return:" + submissionRecord.ID,
			TargetAgentID: strings.TrimSpace(assignment.ReturnToAgentID),
			Status:        protocol.ExecutionReviewDispatchStatusPending,
			Instruction: reviewDispatchInstruction(
				snapshot,
				assignment,
				submissionRecord,
				work,
			),
		}
	}
	updated, submitErr := s.repository.Submit(ctx, orchestrationstore.SubmitCommand{
		ExpectedExecutionVersion:  snapshot.Execution.Version,
		ExpectedAssignmentVersion: assignment.Version,
		Submission:                submissionRecord,
		ReviewDispatch:            reviewDispatch,
		Meta:                      s.commandMeta(actor, input.CommandID, "submit-record"),
	})
	if submitErr != nil {
		return s.storageMutationResult(snapshot, submitErr, nextActions(snapshot, actor))
	}
	submission := latestUnreviewedSubmission(updated, work.ID)
	changed := []string{"attempt:" + attempt.ID}
	if submission != nil {
		changed = append(changed, "submission:"+submission.ID)
	}
	if reviewDispatch != nil {
		changed = append(changed, "review_dispatch:"+reviewDispatch.ID)
		if s.reviewDispatchConsumer != nil {
			_, _ = s.DispatchPendingReviews(ctx, "submit:"+input.CommandID, 8)
			if refreshed, refreshErr := s.repository.GetSnapshot(
				ctx,
				snapshot.Execution.ID,
			); refreshErr == nil && refreshed != nil {
				updated = refreshed
			}
		}
	}
	return AppliedResult(updated, changed, nextActions(updated, actor)), nil
}

// ReviewWork 追加唯一 Acceptance，并由 accepted decision 解锁下游。
func (s *Service) ReviewWork(
	ctx context.Context,
	actor ActorContext,
	input ReviewWorkInput,
) (returned MutationResult, returnedErr error) {
	defer func() { s.invalidateMutationResult(ctx, returned, returnedErr) }()
	snapshot, rejected, err := s.mutableSnapshot(
		ctx,
		actor,
		input.ExecutionID,
		input.SnapshotRevision,
		false,
		false,
	)
	if err != nil || rejected != nil {
		return resultOrZero(rejected), err
	}
	if strings.TrimSpace(input.CommandID) == "" {
		return RejectedResult(snapshot, domainError(ErrorCodeInvalidInput, "command_id is required"), nil), nil
	}
	if limitErr := validateCriteriaResultsProjectionLimit(input.CriteriaResults); limitErr != nil {
		return RejectedResult(snapshot, limitErr, nil), nil
	}
	submission, resolveErr := resolveSubmission(snapshot, input.SubmissionID, input.WorkItemID, input.LogicalKey)
	if resolveErr != nil {
		return RejectedResult(snapshot, resolveErr, nextActions(snapshot, actor)), nil
	}
	if acceptance := acceptanceForSubmission(snapshot, submission.ID); acceptance != nil {
		if acceptance.Decision == protocol.WorkAcceptanceAccepted &&
			len(snapshot.CompletionBlockers) == 0 &&
			snapshot.Execution.Status != protocol.ExecutionStatusCompleted {
			result, completeErr := s.completeAfterReview(
				ctx,
				actor,
				snapshot,
				input.CommandID,
				nil,
			)
			return s.activateReviewContinuationResult(actor, result), completeErr
		}
		return s.activateReviewContinuationResult(
			actor,
			NoOpResult(snapshot, "submission already has an acceptance decision"),
		), nil
	}
	assignment := findAssignmentByID(snapshot, submission.AssignmentID)
	if assignment == nil || assignment.Status != protocol.WorkAssignmentStatusActive {
		return RejectedResult(snapshot, domainError(
			ErrorCodeInvalidInput,
			"submission Assignment is not active",
		), nextActions(snapshot, actor)), nil
	}
	if reviewAuthErr := s.authorizeRoomReviewActor(
		actor,
		snapshot,
		assignment,
		submission,
	); reviewAuthErr != nil {
		return RejectedResult(snapshot, reviewAuthErr, nextActions(snapshot, actor)), nil
	}
	if validationErr := validateReview(snapshot, *submission, input); validationErr != nil {
		return RejectedResult(snapshot, validationErr, nil), nil
	}
	acceptanceID := s.id("acceptance")
	updated, reviewErr := s.repository.Review(ctx, orchestrationstore.ReviewCommand{
		ExpectedExecutionVersion:  snapshot.Execution.Version,
		ExpectedAssignmentVersion: assignment.Version,
		Acceptance: protocol.WorkAcceptance{
			ID:              acceptanceID,
			ExecutionID:     submission.ExecutionID,
			PlanID:          submission.PlanID,
			WorkItemID:      submission.WorkItemID,
			SpecID:          submission.SpecID,
			AssignmentID:    submission.AssignmentID,
			SubmissionID:    submission.ID,
			Decision:        input.Decision,
			ReviewerKind:    reviewerKind(actor),
			ReviewerID:      strings.TrimSpace(actor.AgentID),
			CriteriaResults: cloneCriteriaResults(input.CriteriaResults),
			Feedback:        strings.TrimSpace(input.Feedback),
			DecisionRoundID: strings.TrimSpace(actor.RuntimeRoundID),
		},
		Meta: s.commandMeta(actor, input.CommandID, "review"),
	})
	if reviewErr != nil {
		return s.storageMutationResult(snapshot, reviewErr, nextActions(snapshot, actor))
	}
	changed := []string{"acceptance:" + acceptanceID}
	if input.Decision == protocol.WorkAcceptanceAccepted &&
		len(updated.CompletionBlockers) == 0 {
		result, completeErr := s.completeAfterReview(
			ctx,
			actor,
			updated,
			input.CommandID,
			changed,
		)
		return s.activateReviewContinuationResult(actor, result), completeErr
	}
	return s.activateReviewContinuationResult(
		actor,
		AppliedResult(updated, changed, nextActions(updated, actor)),
	), nil
}

func (s *Service) authorizeRoomReviewActor(
	actor ActorContext,
	snapshot *protocol.ExecutionSnapshot,
	assignment *protocol.WorkAssignment,
	submission *protocol.WorkSubmission,
) error {
	if snapshot == nil || assignment == nil || submission == nil {
		return domainError(ErrorCodeInvalidInput, "review target is incomplete")
	}
	if snapshot.Execution.ScopeKind != protocol.ExecutionScopeRoom ||
		normalizeActorKind(actor.ActorKind) != protocol.ExecutionActorAgent {
		return nil
	}
	actorID := strings.TrimSpace(actor.AgentID)
	if actorID == "" || actorID != strings.TrimSpace(assignment.ReturnToAgentID) {
		return domainError(
			ErrorCodeWrongReviewer,
			"Room review is reserved for the reviewer selected by this Assignment",
		)
	}
	if actor.ReviewBinding != nil {
		return nil
	}
	if actor.WorkBinding != nil {
		binding := normalizeExecutionWorkBinding(actor.WorkBinding)
		if binding.AssignmentID == assignment.ID &&
			binding.WorkItemID == assignment.WorkItemID &&
			binding.SpecID == assignment.SpecID {
			return nil
		}
		return domainError(
			ErrorCodeReviewBindingRequired,
			"Room self-review must stay inside the trusted Assignment binding",
		)
	}
	if actorID == strings.TrimSpace(snapshot.Execution.CoordinatorAgentID) {
		if err := s.requireRuntimeCoordination(actor, snapshot); err != nil {
			return err
		}
		return nil
	}
	return domainError(
		ErrorCodeReviewBindingRequired,
		"Room review requires the trusted binding for the selected reviewer",
	)
}

func (s *Service) completeAfterReview(
	ctx context.Context,
	actor ActorContext,
	snapshot *protocol.ExecutionSnapshot,
	commandID string,
	changed []string,
) (MutationResult, error) {
	if snapshot.Execution.Status == protocol.ExecutionStatusCompleted {
		if len(changed) == 0 {
			return NoOpResult(snapshot, "submission is accepted and execution is completed"), nil
		}
		return AppliedResult(snapshot, changed, nextActions(snapshot, actor)), nil
	}
	completed, err := s.repository.Complete(ctx, orchestrationstore.CompleteCommand{
		ExecutionID:              snapshot.Execution.ID,
		ExpectedExecutionVersion: snapshot.Execution.Version,
		Meta:                     s.commandMeta(actor, commandID, "complete-after-review"),
	})
	if err != nil {
		if errors.Is(err, orchestrationstore.ErrVersionConflict) ||
			errors.Is(err, orchestrationstore.ErrCompletionBlocked) {
			result := AppliedResult(snapshot, changed, nextActions(snapshot, actor))
			result.Message = "Acceptance committed; backend completion audit will retry from the latest snapshot"
			return result, nil
		}
		return MutationResult{}, err
	}
	changed = append(changed, "execution:"+completed.Execution.ID)
	return AppliedResult(completed, changed, nextActions(completed, actor)), nil
}

// BlockWork 标记一个 Work Item 正等待确定的外部输入。
func (s *Service) BlockWork(
	ctx context.Context,
	actor ActorContext,
	input BlockWorkInput,
) (returned MutationResult, returnedErr error) {
	defer func() { s.invalidateMutationResult(ctx, returned, returnedErr) }()
	snapshot, rejected, err := s.mutableSnapshot(
		ctx,
		actor,
		input.ExecutionID,
		input.SnapshotRevision,
		false,
		false,
	)
	if err != nil || rejected != nil {
		return resultOrZero(rejected), err
	}
	if strings.TrimSpace(input.CommandID) == "" ||
		strings.TrimSpace(input.Reason) == "" ||
		strings.TrimSpace(input.NeededInput) == "" {
		return RejectedResult(snapshot, domainError(
			ErrorCodeInvalidInput,
			"command_id, reason and needed_input are required",
		), nil), nil
	}
	work, spec, resolveErr := resolvePlanWork(snapshot, input.WorkItemID, input.LogicalKey)
	if resolveErr != nil {
		return RejectedResult(snapshot, resolveErr, nil), nil
	}
	assignment := activeAssignmentForWork(snapshot, work.ID)
	isCoordinator := snapshot.Execution.CoordinatorAgentID == strings.TrimSpace(actor.AgentID)
	if !isCoordinator && (assignment == nil || assignment.OwnerAgentID != strings.TrimSpace(actor.AgentID)) {
		return RejectedResult(snapshot, newDomainError(
			ErrorCodeWrongOwner,
			"only the current Assignment owner or coordinator may block work",
			work.LogicalKey,
			"",
		), nil), nil
	}
	if submission := latestUnreviewedSubmissionForSpec(snapshot, work.ID, spec.ID); submission != nil {
		return RejectedResult(snapshot, newDomainError(
			ErrorCodeCompletionBlocked,
			"review the pending Submission before blocking this Work Item",
			work.LogicalKey,
			submission.ID,
		), nextActions(snapshot, actor)), nil
	}
	state := findStateByWorkID(snapshot, work.ID)
	if state == nil {
		return RejectedResult(snapshot, domainError(
			ErrorCodeInvalidInput,
			"Work Item state is missing",
		), nil), nil
	}
	if state.Status == protocol.WorkItemStatusWaitingInput &&
		state.BlockReason == strings.TrimSpace(input.Reason) &&
		state.NeededInput == strings.TrimSpace(input.NeededInput) {
		return NoOpResult(snapshot, "work is already blocked on the same input"), nil
	}
	updated, blockErr := s.repository.Block(ctx, orchestrationstore.BlockCommand{
		ExpectedExecutionVersion: snapshot.Execution.Version,
		ExpectedStateVersion:     state.Version,
		State: protocol.WorkItemState{
			WorkItemID:    work.ID,
			ExecutionID:   snapshot.Execution.ID,
			CurrentSpecID: spec.ID,
			Status:        protocol.WorkItemStatusWaitingInput,
			BlockReason:   strings.TrimSpace(input.Reason),
			NeededInput:   strings.TrimSpace(input.NeededInput),
			Metadata:      cloneMap(state.Metadata),
		},
		Meta: s.commandMeta(actor, input.CommandID, "block"),
	})
	if blockErr != nil {
		return s.storageMutationResult(snapshot, blockErr, nextActions(snapshot, actor))
	}
	return AppliedResult(updated, []string{"work_item_state:" + work.ID}, nextActions(updated, actor)), nil
}

// ResumeWork 用 resolution/evidence 关闭 waiting_input；旧 Attempt 不会被复活。
func (s *Service) ResumeWork(
	ctx context.Context,
	actor ActorContext,
	input ResumeWorkInput,
) (returned MutationResult, returnedErr error) {
	defer func() { s.invalidateMutationResult(ctx, returned, returnedErr) }()
	snapshot, rejected, err := s.mutableSnapshot(
		ctx,
		actor,
		input.ExecutionID,
		input.SnapshotRevision,
		false,
		false,
	)
	if err != nil {
		return MutationResult{}, err
	}
	if rejected != nil && rejected.ReasonCode != ErrorCodeStaleExecution {
		return *rejected, nil
	}
	if limitErr := newProjectionLimitError("resume_evidence", len(input.Evidence), ""); limitErr != nil {
		return RejectedResult(snapshot, limitErr, nil), nil
	}
	evidence := normalizeNonEmptyValues(input.Evidence)
	if strings.TrimSpace(input.CommandID) == "" ||
		strings.TrimSpace(input.Resolution) == "" ||
		len(evidence) == 0 {
		return RejectedResult(snapshot, domainError(
			ErrorCodeInvalidInput,
			"command_id, resolution and at least one evidence item are required",
		), nil), nil
	}
	work, spec, resolveErr := resolvePlanWork(snapshot, input.WorkItemID, input.LogicalKey)
	if resolveErr != nil {
		return RejectedResult(snapshot, resolveErr, nil), nil
	}
	state := findStateByWorkID(snapshot, work.ID)
	if state == nil || state.CurrentSpecID != spec.ID {
		return RejectedResult(snapshot, domainError(
			ErrorCodeInvalidInput,
			"current Work Item state/spec fence is missing",
		), nil), nil
	}
	assignment := latestAssignmentForCurrentSpec(snapshot, work.ID, spec.ID)
	isCoordinator := snapshot.Execution.CoordinatorAgentID == strings.TrimSpace(actor.AgentID)
	if !isCoordinator && (assignment == nil || assignment.OwnerAgentID != strings.TrimSpace(actor.AgentID)) {
		return RejectedResult(snapshot, newDomainError(
			ErrorCodeWrongOwner,
			"only the latest current-spec Assignment owner or coordinator may resume work",
			work.LogicalKey,
			"",
		), nil), nil
	}
	if state.Status == protocol.WorkItemStatusOpen {
		return NoOpResult(snapshot, "work is already open"), nil
	}
	if rejected != nil {
		return *rejected, nil
	}
	if state.Status != protocol.WorkItemStatusWaitingInput {
		return RejectedResult(snapshot, newDomainError(
			ErrorCodeCompletionBlocked,
			"only waiting_input work can be resumed",
			work.LogicalKey,
			string(state.Status),
		), nil), nil
	}
	resolution := strings.TrimSpace(input.Resolution)
	metadata := cloneMap(state.Metadata)
	if metadata == nil {
		metadata = make(map[string]any, 2)
	}
	metadata["last_resume_resolution"] = resolution
	metadata["last_resume_evidence"] = slices.Clone(evidence)
	updated, resumeErr := s.repository.Resume(ctx, orchestrationstore.ResumeCommand{
		ExpectedExecutionVersion: snapshot.Execution.Version,
		ExpectedStateVersion:     state.Version,
		State: protocol.WorkItemState{
			WorkItemID:    work.ID,
			ExecutionID:   snapshot.Execution.ID,
			CurrentSpecID: spec.ID,
			Status:        protocol.WorkItemStatusOpen,
			Metadata:      metadata,
		},
		Resolution: resolution,
		Evidence:   evidence,
		Meta:       s.commandMeta(actor, input.CommandID, "resume"),
	})
	if resumeErr != nil {
		return s.storageMutationResult(snapshot, resumeErr, nextActions(snapshot, actor))
	}
	return AppliedResult(updated, []string{"work_item_state:" + work.ID}, nextActions(updated, actor)), nil
}

// TakeOverWork 原子释放旧责任链并建立 replacement Assignment。
func (s *Service) TakeOverWork(
	ctx context.Context,
	actor ActorContext,
	input TakeOverWorkInput,
) (returned MutationResult, returnedErr error) {
	defer func() { s.invalidateMutationResult(ctx, returned, returnedErr) }()
	snapshot, rejected, err := s.mutableSnapshot(
		ctx,
		actor,
		input.ExecutionID,
		input.SnapshotRevision,
		true,
		false,
	)
	if err != nil || rejected != nil {
		return resultOrZero(rejected), err
	}
	if strings.TrimSpace(input.CommandID) == "" || strings.TrimSpace(input.Reason) == "" {
		return RejectedResult(snapshot, domainError(
			ErrorCodeInvalidInput,
			"command_id and takeover reason are required",
		), nil), nil
	}
	work, spec, resolveErr := resolvePlanWork(snapshot, input.WorkItemID, input.LogicalKey)
	if resolveErr != nil {
		return RejectedResult(snapshot, resolveErr, nil), nil
	}
	current := activeAssignmentForWork(snapshot, work.ID)
	if current == nil {
		return RejectedResult(snapshot, newDomainError(
			ErrorCodeInvalidInput,
			"Work Item has no current Assignment to take over",
			work.LogicalKey,
			"",
		), nextActions(snapshot, actor)), nil
	}
	if submission := latestUnreviewedSubmissionForSpec(snapshot, work.ID, spec.ID); submission != nil {
		return RejectedResult(snapshot, newDomainError(
			ErrorCodeCompletionBlocked,
			"review the pending Submission before taking over this Work Item",
			work.LogicalKey,
			submission.ID,
		), nextActions(snapshot, actor)), nil
	}
	target := strings.TrimSpace(input.TargetAgentID)
	if target == "" {
		return RejectedResult(snapshot, domainError(ErrorCodeInvalidInput, "target_agent_id is required"), nil), nil
	}
	replacement, dispatch, attempt, buildErr := s.buildAssignmentChain(
		actor,
		snapshot,
		work,
		spec,
		target,
		input.ReturnToAgentID,
		input.Strategy,
		"",
		input.Reason,
		input.Instruction,
		input.DispatchKind,
		input.CommandID,
	)
	if buildErr != nil {
		return RejectedResult(snapshot, buildErr, nil), nil
	}
	if targetErr := s.authorizeAssignmentTarget(ctx, actor, snapshot, replacement, dispatch); targetErr != nil {
		return RejectedResult(snapshot, targetErr, nextActions(snapshot, actor)), nil
	}
	updated, takeoverErr := s.repository.Takeover(ctx, orchestrationstore.TakeoverCommand{
		ExpectedExecutionVersion:         snapshot.Execution.Version,
		ExpectedCurrentAssignmentVersion: current.Version,
		CurrentAssignmentID:              current.ID,
		Replacement:                      replacement,
		Dispatch:                         dispatch,
		RootAttempt:                      &attempt,
		Meta:                             s.commandMeta(actor, input.CommandID, "takeover"),
	})
	if takeoverErr != nil {
		return s.storageMutationResult(snapshot, takeoverErr, nextActions(snapshot, actor))
	}
	result := AppliedResult(updated, []string{
		"assignment:" + replacement.ID,
		"assignment_released:" + current.ID,
		"attempt:" + attempt.ID,
	}, nextActions(updated, actor))
	return withRoomSelfWorkBindingReceipt(actor, result, work.ID), nil
}

// CompleteIfReady 完成 Execution；任何 blocker 都返回结构化 completion_blocked。
func (s *Service) CompleteIfReady(
	ctx context.Context,
	actor ActorContext,
	input CompleteExecutionInput,
) (returned MutationResult, returnedErr error) {
	defer func() { s.invalidateMutationResult(ctx, returned, returnedErr) }()
	snapshot, rejected, err := s.mutableSnapshot(
		ctx,
		actor,
		input.ExecutionID,
		input.SnapshotRevision,
		true,
		false,
	)
	if err != nil || rejected != nil {
		return resultOrZero(rejected), err
	}
	if strings.TrimSpace(input.CommandID) == "" {
		return RejectedResult(snapshot, domainError(ErrorCodeInvalidInput, "command_id is required"), nil), nil
	}
	if snapshot.Execution.Status == protocol.ExecutionStatusCompleted {
		return NoOpResult(snapshot, "execution is already completed"), nil
	}
	if len(snapshot.CompletionBlockers) > 0 {
		return RejectedResult(snapshot, domainError(
			ErrorCodeCompletionBlocked,
			"execution still has completion blockers: "+strings.Join(snapshot.CompletionBlockers, ", "),
		), nextActions(snapshot, actor)), nil
	}
	updated, completeErr := s.repository.Complete(ctx, orchestrationstore.CompleteCommand{
		ExecutionID:              snapshot.Execution.ID,
		ExpectedExecutionVersion: snapshot.Execution.Version,
		Meta:                     s.commandMeta(actor, input.CommandID, "complete"),
	})
	if completeErr != nil {
		return s.storageMutationResult(snapshot, completeErr, nextActions(snapshot, actor))
	}
	return AppliedResult(updated, []string{"execution:" + updated.Execution.ID}, nil), nil
}

func (s *Service) mutableSnapshot(
	ctx context.Context,
	actor ActorContext,
	executionID string,
	expectedRevision int64,
	coordinatorOnly bool,
	allowPlanMode bool,
) (*protocol.ExecutionSnapshot, *MutationResult, error) {
	if err := validateActor(actor); err != nil {
		result := RejectedResult(nil, err, nil)
		return nil, &result, nil
	}
	if actor.PlanMode && !allowPlanMode {
		result := RejectedResult(nil, planModeError(), nil)
		return nil, &result, nil
	}
	snapshot, err := s.GetSnapshot(ctx, actor, executionID)
	if err != nil {
		var domainErr *DomainError
		if errors.As(err, &domainErr) {
			if domainErr.Code == ErrorCodeExecutionTerminal {
				result := SupersededResult(nil, err)
				return nil, &result, nil
			}
			result := RejectedResult(nil, err, nil)
			return nil, &result, nil
		}
		return nil, nil, err
	}
	if snapshot == nil {
		result := RejectedResult(nil, domainError(ErrorCodeInvalidInput, "execution was not found"), nil)
		return nil, &result, nil
	}
	if !isCurrentExecutionStatus(snapshot.Execution.Status) {
		result := RejectedResult(snapshot, terminalExecutionError(), nil)
		return snapshot, &result, nil
	}
	if expectedErr := requireMutationRevision(snapshot, expectedRevision); expectedErr != nil {
		result := RejectedResult(snapshot, expectedErr, nextActions(snapshot, actor))
		return snapshot, &result, nil
	}
	if coordinatorOnly {
		if coordinationErr := s.requireRuntimeCoordination(actor, snapshot); coordinationErr != nil {
			result := RejectedResult(snapshot, coordinationErr, []NextAction{{
				Tool:   "get_execution",
				Reason: "explicitly inspect and enter the current Room coordination scope",
			}})
			return snapshot, &result, nil
		}
		if authErr := requireCoordinator(actor, snapshot); authErr != nil {
			code := ErrorCodeWrongOwner
			if strings.Contains(authErr.Error(), "review") {
				code = ErrorCodeWrongReviewer
			}
			result := RejectedResult(snapshot, domainError(code, authErr.Error()), nil)
			return snapshot, &result, nil
		}
	}
	return snapshot, nil, nil
}

func (s *Service) buildAssignmentChain(
	actor ActorContext,
	snapshot *protocol.ExecutionSnapshot,
	work protocol.WorkItem,
	spec protocol.WorkItemSpec,
	target string,
	returnTo string,
	strategy protocol.AssignmentStrategy,
	assignmentReason string,
	takeoverReason string,
	instruction string,
	dispatchKind protocol.ExecutionDispatchKind,
	commandID string,
) (
	protocol.WorkAssignment,
	*protocol.ExecutionDispatch,
	protocol.WorkAttempt,
	error,
) {
	if strategy == "" {
		strategy = protocol.AssignmentStrategyRoomMember
		if target == strings.TrimSpace(actor.AgentID) {
			strategy = protocol.AssignmentStrategySelf
		}
	}
	if strategy == protocol.AssignmentStrategyRoomMember &&
		snapshot.Execution.ScopeKind != protocol.ExecutionScopeRoom {
		return protocol.WorkAssignment{}, nil, protocol.WorkAttempt{}, domainError(
			ErrorCodeAssignmentTargetInvalid,
			"room_member Assignment requires a Room Execution",
		)
	}
	if strategy != protocol.AssignmentStrategySelf &&
		strategy != protocol.AssignmentStrategyRoomMember {
		return protocol.WorkAssignment{}, nil, protocol.WorkAttempt{}, domainError(
			ErrorCodeInvalidInput,
			"unknown assignment strategy",
		)
	}
	if strategy == protocol.AssignmentStrategySelf {
		if target != strings.TrimSpace(actor.AgentID) {
			return protocol.WorkAssignment{}, nil, protocol.WorkAttempt{}, domainError(
				ErrorCodeAssignmentTargetInvalid,
				"self Assignment target must be the current actor",
			)
		}
		if dispatchKind != "" {
			return protocol.WorkAssignment{}, nil, protocol.WorkAttempt{}, domainError(
				ErrorCodeAssignmentTargetInvalid,
				"self Assignment must not request a Room Dispatch",
			)
		}
	}
	if strategy == protocol.AssignmentStrategyRoomMember &&
		target == strings.TrimSpace(actor.AgentID) {
		return protocol.WorkAssignment{}, nil, protocol.WorkAttempt{}, domainError(
			ErrorCodeAssignmentTargetInvalid,
			"assign the current actor with strategy self",
		)
	}
	coordinatorAgentID := strings.TrimSpace(snapshot.Execution.CoordinatorAgentID)
	if returnTo = strings.TrimSpace(returnTo); returnTo == "" {
		returnTo = coordinatorAgentID
	}
	if returnTo == "" {
		return protocol.WorkAssignment{}, nil, protocol.WorkAttempt{}, domainError(
			ErrorCodeInvalidInput,
			"Assignment return target requires a coordinator",
		)
	}
	assignment := protocol.WorkAssignment{
		ID:                s.id("assignment"),
		ExecutionID:       snapshot.Execution.ID,
		PlanID:            snapshot.Plan.ID,
		WorkItemID:        work.ID,
		SpecID:            spec.ID,
		OwnerAgentID:      target,
		AssignedByAgentID: strings.TrimSpace(actor.AgentID),
		ReturnToAgentID:   returnTo,
		Strategy:          strategy,
		Status:            protocol.WorkAssignmentStatusAssigned,
		AssignmentReason:  strings.TrimSpace(assignmentReason),
		TakeoverReason:    strings.TrimSpace(takeoverReason),
	}
	attempt := protocol.WorkAttempt{
		ID:              s.id("attempt"),
		ExecutionID:     snapshot.Execution.ID,
		PlanID:          snapshot.Plan.ID,
		WorkItemID:      work.ID,
		SpecID:          spec.ID,
		AssignmentID:    assignment.ID,
		ExecutorKind:    protocol.AttemptExecutorAgent,
		ExecutorAgentID: target,
		Status:          protocol.WorkAttemptStatusPending,
	}
	var dispatch *protocol.ExecutionDispatch
	if strategy == protocol.AssignmentStrategyRoomMember {
		if dispatchKind == "" {
			dispatchKind = protocol.ExecutionDispatchRoomDirected
		}
		if dispatchKind != protocol.ExecutionDispatchRoomDirected &&
			dispatchKind != protocol.ExecutionDispatchRoomPublic {
			return protocol.WorkAssignment{}, nil, protocol.WorkAttempt{}, domainError(
				ErrorCodeInvalidInput,
				"Room Assignment requires room_directed or room_public dispatch",
			)
		}
		if instruction = strings.TrimSpace(instruction); instruction == "" {
			instruction = fmt.Sprintf(
				"Deliver %s. Acceptance criteria: %s",
				spec.Deliverable,
				strings.Join(spec.AcceptanceCriteria, "; "),
			)
		}
		dispatch = &protocol.ExecutionDispatch{
			ID:            s.id("dispatch"),
			DedupeKey:     commandPart(commandID, "dispatch:"+work.ID+":"+target),
			TargetAgentID: target,
			Kind:          dispatchKind,
			Status:        protocol.ExecutionDispatchStatusPending,
			Instruction:   instruction,
		}
	}
	return assignment, dispatch, attempt, nil
}

func (s *Service) storageMutationResult(
	snapshot *protocol.ExecutionSnapshot,
	err error,
	actions []NextAction,
) (MutationResult, error) {
	switch {
	case errors.Is(err, orchestrationstore.ErrVersionConflict):
		return RejectedResult(snapshot, domainError(
			ErrorCodeStaleExecution,
			"state changed concurrently; reload the execution before retrying",
		), actions), nil
	case errors.Is(err, orchestrationstore.ErrWorkNotReady):
		return RejectedResult(snapshot, domainError(
			ErrorCodeDependencyNotAccepted,
			"Work Item is not ready or already has a current Assignment",
		), actions), nil
	case errors.Is(err, orchestrationstore.ErrCompletionBlocked):
		return RejectedResult(snapshot, domainError(
			ErrorCodeCompletionBlocked,
			"execution still has completion blockers",
		), actions), nil
	case errors.Is(err, orchestrationstore.ErrProjectionLimitExceeded):
		return RejectedResult(snapshot, domainError(
			ErrorCodeProjectionLimitExceeded,
			err.Error(),
		), actions), nil
	case errors.Is(err, orchestrationstore.ErrCommandConflict),
		errors.Is(err, orchestrationstore.ErrInvariant):
		return RejectedResult(snapshot, domainError(
			ErrorCodeInvalidInput,
			"command violates the current execution state",
		), actions), nil
	default:
		return MutationResult{}, err
	}
}

func resolvePlanWork(
	snapshot *protocol.ExecutionSnapshot,
	workItemID string,
	logicalKey string,
) (protocol.WorkItem, protocol.WorkItemSpec, error) {
	workItemID = strings.TrimSpace(workItemID)
	logicalKey = strings.TrimSpace(logicalKey)
	var work *protocol.WorkItem
	for index := range snapshot.WorkItems {
		item := &snapshot.WorkItems[index]
		if workItemID != "" && item.ID == workItemID {
			work = item
			break
		}
		if workItemID == "" && logicalKey != "" && item.LogicalKey == logicalKey {
			work = item
			break
		}
	}
	if work == nil {
		return protocol.WorkItem{}, protocol.WorkItemSpec{}, newDomainError(
			ErrorCodeInvalidInput,
			"Work Item is not part of the active Plan",
			logicalKey,
			workItemID,
		)
	}
	if logicalKey != "" && work.LogicalKey != logicalKey {
		return protocol.WorkItem{}, protocol.WorkItemSpec{}, newDomainError(
			ErrorCodeInvalidInput,
			"work_item_id and logical_key identify different Work Items",
			logicalKey,
			workItemID,
		)
	}
	for _, item := range snapshot.PlanItems {
		if item.WorkItemID != work.ID {
			continue
		}
		for _, spec := range snapshot.WorkItemSpecs {
			if spec.ID == item.SpecID {
				return *work, spec, nil
			}
		}
	}
	return protocol.WorkItem{}, protocol.WorkItemSpec{}, newDomainError(
		ErrorCodeInvalidInput,
		"active Work Item spec is missing",
		work.LogicalKey,
		work.ID,
	)
}

func resolveSubmission(
	snapshot *protocol.ExecutionSnapshot,
	submissionID string,
	workItemID string,
	logicalKey string,
) (*protocol.WorkSubmission, error) {
	submissionID = strings.TrimSpace(submissionID)
	if submissionID != "" {
		for index := range snapshot.Submissions {
			if snapshot.Submissions[index].ID == submissionID {
				return &snapshot.Submissions[index], nil
			}
		}
		return nil, domainError(ErrorCodeInvalidInput, "submission is not part of the active Plan")
	}
	work, _, err := resolvePlanWork(snapshot, workItemID, logicalKey)
	if err != nil {
		return nil, err
	}
	submission := latestUnreviewedSubmission(snapshot, work.ID)
	if submission == nil {
		return nil, newDomainError(
			ErrorCodeInvalidInput,
			"Work Item has no unreviewed Submission",
			work.LogicalKey,
			"",
		)
	}
	return submission, nil
}

func validateReview(
	snapshot *protocol.ExecutionSnapshot,
	submission protocol.WorkSubmission,
	input ReviewWorkInput,
) error {
	switch input.Decision {
	case protocol.WorkAcceptanceAccepted,
		protocol.WorkAcceptanceRejected,
		protocol.WorkAcceptanceChangesRequested:
	default:
		return domainError(ErrorCodeInvalidInput, "unknown acceptance decision")
	}
	if input.Decision != protocol.WorkAcceptanceAccepted {
		return nil
	}
	var spec *protocol.WorkItemSpec
	for index := range snapshot.WorkItemSpecs {
		if snapshot.WorkItemSpecs[index].ID == submission.SpecID {
			spec = &snapshot.WorkItemSpecs[index]
			break
		}
	}
	if spec == nil {
		return domainError(ErrorCodeInvalidInput, "Submission spec is missing")
	}
	resultByCriterion := make(map[string]protocol.WorkAcceptanceCriterionResult, len(input.CriteriaResults))
	for _, result := range input.CriteriaResults {
		criterion := strings.TrimSpace(result.Criterion)
		if criterion == "" {
			return domainError(ErrorCodeAcceptanceCriteriaEmpty, "criterion result is empty")
		}
		if _, duplicate := resultByCriterion[criterion]; duplicate {
			return domainError(ErrorCodeInvalidInput, "criterion result appears more than once")
		}
		resultByCriterion[criterion] = result
	}
	for _, criterion := range spec.AcceptanceCriteria {
		result, exists := resultByCriterion[criterion]
		if !exists || !result.Passed {
			return domainError(
				ErrorCodeAcceptanceCriteriaEmpty,
				"accepted decision requires a passing result for every acceptance criterion",
			)
		}
	}
	return nil
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

func currentAssignment(assignment protocol.WorkAssignment) bool {
	return assignment.Status == protocol.WorkAssignmentStatusAssigned ||
		assignment.Status == protocol.WorkAssignmentStatusActive
}

func activeAssignmentForWork(
	snapshot *protocol.ExecutionSnapshot,
	workItemID string,
) *protocol.WorkAssignment {
	for index := range snapshot.Assignments {
		assignment := &snapshot.Assignments[index]
		if assignment.WorkItemID == workItemID && currentAssignment(*assignment) {
			return assignment
		}
	}
	return nil
}

func latestAssignmentForCurrentSpec(
	snapshot *protocol.ExecutionSnapshot,
	workItemID string,
	specID string,
) *protocol.WorkAssignment {
	if snapshot == nil {
		return nil
	}
	if !isCurrentExecutionStatus(snapshot.Execution.Status) {
		return nil
	}
	var latest *protocol.WorkAssignment
	for index := range snapshot.Assignments {
		assignment := &snapshot.Assignments[index]
		if assignment.WorkItemID != workItemID || assignment.SpecID != specID ||
			(snapshot.Plan != nil && assignment.PlanID != snapshot.Plan.ID) {
			continue
		}
		if latest == nil ||
			assignment.AssignedAt.After(latest.AssignedAt) ||
			(assignment.AssignedAt.Equal(latest.AssignedAt) && assignment.ID > latest.ID) {
			latest = assignment
		}
	}
	return latest
}

func selectAssignment(
	snapshot *protocol.ExecutionSnapshot,
	workItemID string,
	assignmentID string,
) *protocol.WorkAssignment {
	assignmentID = strings.TrimSpace(assignmentID)
	for index := range snapshot.Assignments {
		assignment := &snapshot.Assignments[index]
		if assignment.WorkItemID == workItemID &&
			(assignmentID == "" || assignment.ID == assignmentID) &&
			currentAssignment(*assignment) {
			return assignment
		}
	}
	return nil
}

func findAssignmentByID(
	snapshot *protocol.ExecutionSnapshot,
	assignmentID string,
) *protocol.WorkAssignment {
	if snapshot == nil {
		return nil
	}
	for index := range snapshot.Assignments {
		if snapshot.Assignments[index].ID == assignmentID {
			return &snapshot.Assignments[index]
		}
	}
	return nil
}

func findAttemptByID(
	snapshot *protocol.ExecutionSnapshot,
	attemptID string,
) *protocol.WorkAttempt {
	if snapshot == nil {
		return nil
	}
	for index := range snapshot.Attempts {
		if snapshot.Attempts[index].ID == attemptID {
			return &snapshot.Attempts[index]
		}
	}
	return nil
}

func currentOrSucceededAttempt(
	snapshot *protocol.ExecutionSnapshot,
	assignmentID string,
) *protocol.WorkAttempt {
	var succeeded *protocol.WorkAttempt
	for index := range snapshot.Attempts {
		attempt := &snapshot.Attempts[index]
		if attempt.AssignmentID != assignmentID {
			continue
		}
		if attempt.Status == protocol.WorkAttemptStatusPending ||
			attempt.Status == protocol.WorkAttemptStatusRunning {
			return attempt
		}
		if attempt.Status == protocol.WorkAttemptStatusSucceeded {
			if succeeded == nil || attempt.CreatedAt.After(succeeded.CreatedAt) {
				succeeded = attempt
			}
		}
	}
	return succeeded
}

func mergeSubmissionRuntime(
	attempt protocol.WorkAttempt,
	actor ActorContext,
	input SubmitWorkInput,
) protocol.WorkAttempt {
	attempt.ExecutorKind = protocol.AttemptExecutorAgent
	attempt.ExecutorAgentID = strings.TrimSpace(actor.AgentID)
	attempt.RuntimeSessionKey = firstNonEmpty(input.RuntimeSessionKey, actor.SessionKey)
	attempt.RoomSessionID = strings.TrimSpace(input.RoomSessionID)
	attempt.SDKSessionID = strings.TrimSpace(input.SDKSessionID)
	attempt.RuntimeRoundID = strings.TrimSpace(actor.RuntimeRoundID)
	attempt.RootRoundID = strings.TrimSpace(actor.RootRoundID)
	attempt.AgentRoundID = strings.TrimSpace(actor.AgentRoundID)
	attempt.ToolUseID = strings.TrimSpace(input.ToolUseID)
	return attempt
}

func latestUnreviewedSubmission(
	snapshot *protocol.ExecutionSnapshot,
	workItemID string,
) *protocol.WorkSubmission {
	return latestUnreviewedSubmissionForSpec(snapshot, workItemID, "")
}

func latestUnreviewedSubmissionForSpec(
	snapshot *protocol.ExecutionSnapshot,
	workItemID string,
	specID string,
) *protocol.WorkSubmission {
	specID = strings.TrimSpace(specID)
	reviewed := make(map[string]bool, len(snapshot.Acceptances))
	for _, acceptance := range snapshot.Acceptances {
		reviewed[acceptance.SubmissionID] = true
	}
	var selected *protocol.WorkSubmission
	for index := range snapshot.Submissions {
		submission := &snapshot.Submissions[index]
		if submission.WorkItemID != workItemID ||
			(specID != "" && submission.SpecID != specID) ||
			reviewed[submission.ID] {
			continue
		}
		if selected == nil || submission.Sequence > selected.Sequence {
			selected = submission
		}
	}
	return selected
}

func acceptanceForSubmission(
	snapshot *protocol.ExecutionSnapshot,
	submissionID string,
) *protocol.WorkAcceptance {
	for index := range snapshot.Acceptances {
		if snapshot.Acceptances[index].SubmissionID == submissionID {
			return &snapshot.Acceptances[index]
		}
	}
	return nil
}

func findStateByWorkID(
	snapshot *protocol.ExecutionSnapshot,
	workItemID string,
) *protocol.WorkItemState {
	for index := range snapshot.WorkItemStates {
		if snapshot.WorkItemStates[index].WorkItemID == workItemID {
			return &snapshot.WorkItemStates[index]
		}
	}
	return nil
}

func hasUnreviewedSubmission(snapshot *protocol.ExecutionSnapshot) bool {
	for _, work := range snapshot.WorkItems {
		if latestUnreviewedSubmission(snapshot, work.ID) != nil {
			return true
		}
	}
	return false
}

func cloneCriteriaResults(
	values []protocol.WorkAcceptanceCriterionResult,
) []protocol.WorkAcceptanceCriterionResult {
	result := make([]protocol.WorkAcceptanceCriterionResult, len(values))
	for index, value := range values {
		value.Criterion = strings.TrimSpace(value.Criterion)
		value.Evidence = normalizeNonEmptyValues(value.Evidence)
		value.Note = strings.TrimSpace(value.Note)
		result[index] = value
	}
	return result
}

func validateCriteriaResultsProjectionLimit(
	values []protocol.WorkAcceptanceCriterionResult,
) error {
	if err := newProjectionLimitError("criteria_results", len(values), ""); err != nil {
		return err
	}
	for index, value := range values {
		if err := newProjectionLimitError(
			fmt.Sprintf("criteria_results[%d].evidence", index),
			len(value.Evidence),
			"",
		); err != nil {
			return err
		}
	}
	return nil
}

func reviewerKind(actor ActorContext) protocol.WorkReviewerKind {
	if normalizeActorKind(actor.ActorKind) == protocol.ExecutionActorUser {
		return protocol.WorkReviewerUser
	}
	if normalizeActorKind(actor.ActorKind) == protocol.ExecutionActorSystem {
		return protocol.WorkReviewerSystem
	}
	return protocol.WorkReviewerAgent
}

func reviewDispatchInstruction(
	snapshot *protocol.ExecutionSnapshot,
	assignment *protocol.WorkAssignment,
	submission protocol.WorkSubmission,
	work protocol.WorkItem,
) string {
	base := fmt.Sprintf(
		"Review Submission %s for Work Item %s. Result: %s",
		submission.ID,
		work.LogicalKey,
		submission.ResultSummary,
	)
	if snapshot == nil || assignment == nil {
		return base
	}
	reviewerID := strings.TrimSpace(assignment.ReturnToAgentID)
	coordinatorID := strings.TrimSpace(snapshot.Execution.CoordinatorAgentID)
	ownerID := strings.TrimSpace(assignment.OwnerAgentID)
	switch {
	case reviewerID != "" && reviewerID == ownerID:
		return base + " The Assignment selected self-review, so keep the review inside the current Agent responsibility and continue from the recorded decision when possible."
	case reviewerID != "" && coordinatorID != "" && reviewerID != coordinatorID:
		return fmt.Sprintf(
			"%s After recording the decision, send the substantive findings to coordinator %s through Room communication so the collaboration can continue; do not send a status-only handoff and do not wait for a user continuation message.",
			base,
			coordinatorID,
		)
	default:
		return base + " After recording the decision, continue coordination from the resulting state when possible; do not wait for a user continuation message."
	}
}

func needsReviewDispatch(
	snapshot *protocol.ExecutionSnapshot,
	assignment *protocol.WorkAssignment,
) bool {
	return snapshot != nil &&
		assignment != nil &&
		snapshot.Execution.ScopeKind == protocol.ExecutionScopeRoom &&
		strings.TrimSpace(assignment.ReturnToAgentID) !=
			strings.TrimSpace(assignment.OwnerAgentID)
}

func nextActions(
	snapshot *protocol.ExecutionSnapshot,
	actor ActorContext,
) []NextAction {
	if snapshot == nil {
		return nil
	}
	isCoordinator := snapshot.Execution.CoordinatorAgentID == strings.TrimSpace(actor.AgentID)
	actions := make([]NextAction, 0)
	if isCoordinator {
		for _, workID := range snapshot.ReadyWorkItemIDs {
			work := logicalWork(snapshot, workID)
			actions = append(actions, NextAction{
				Tool:       "assign_work",
				WorkItemID: workID,
				LogicalKey: work.LogicalKey,
				Reason:     "Work Item is ready and has no current Assignment",
			})
		}
	}
	for _, work := range snapshot.WorkItems {
		if submission := latestUnreviewedSubmission(snapshot, work.ID); submission != nil {
			assignment := findAssignmentByID(snapshot, submission.AssignmentID)
			if snapshot.Execution.ScopeKind == protocol.ExecutionScopeRoom &&
				(assignment == nil ||
					strings.TrimSpace(assignment.ReturnToAgentID) != strings.TrimSpace(actor.AgentID)) {
				continue
			}
			if snapshot.Execution.ScopeKind != protocol.ExecutionScopeRoom && !isCoordinator {
				continue
			}
			actions = append(actions, NextAction{
				Tool:       "review_work",
				WorkItemID: work.ID,
				LogicalKey: work.LogicalKey,
				Reason:     "Submission is awaiting the selected review decision",
			})
		}
	}
	for _, work := range snapshot.WorkItems {
		state := findStateByWorkID(snapshot, work.ID)
		if state == nil || state.Status != protocol.WorkItemStatusWaitingInput {
			continue
		}
		assignment := latestAssignmentForCurrentSpec(
			snapshot,
			work.ID,
			state.CurrentSpecID,
		)
		if !isCoordinator &&
			(assignment == nil || assignment.OwnerAgentID != strings.TrimSpace(actor.AgentID)) {
			continue
		}
		actions = append(actions, NextAction{
			Tool:       "resume_work",
			WorkItemID: work.ID,
			LogicalKey: work.LogicalKey,
			Reason:     "the exact waiting_input blocker must be resolved with evidence",
		})
	}
	for _, assignment := range snapshot.Assignments {
		if currentAssignment(assignment) &&
			assignment.OwnerAgentID == strings.TrimSpace(actor.AgentID) &&
			latestUnreviewedSubmission(snapshot, assignment.WorkItemID) == nil {
			state := findStateByWorkID(snapshot, assignment.WorkItemID)
			if state == nil ||
				state.CurrentSpecID != assignment.SpecID ||
				state.Status != protocol.WorkItemStatusOpen {
				continue
			}
			work := logicalWork(snapshot, assignment.WorkItemID)
			actions = append(actions, NextAction{
				Tool:       "submit_work",
				WorkItemID: assignment.WorkItemID,
				LogicalKey: work.LogicalKey,
				Reason:     "current actor owns this Assignment",
			})
		}
	}
	return actions
}

func logicalWork(snapshot *protocol.ExecutionSnapshot, workID string) protocol.WorkItem {
	for _, work := range snapshot.WorkItems {
		if work.ID == workID {
			return work
		}
	}
	return protocol.WorkItem{}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func resultOrZero(result *MutationResult) MutationResult {
	if result == nil {
		return MutationResult{}
	}
	return *result
}

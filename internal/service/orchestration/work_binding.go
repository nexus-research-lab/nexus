// INPUT: Room runtime 注入的 trusted WorkBinding、当前 actor 与完整 Execution snapshot。
// OUTPUT: 完整绑定链校验、自身 mutation capability、超限 fail-closed 及已验收上游只读投影的 actor-scoped snapshot。
// POS: structured Room member mutation/responsibility/subagent 路径共用的最终授权栅栏；显式共享图观察走独立只读入口。
package orchestration

import (
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// scopeSnapshotToTrustedWorkBinding 把 structured Room slot 的 WorkBinding
// 解释为后端能力边界。模型只能提供 tool input，不能扩大 actor 自带的绑定。
//
// Coordinator 可通过显式 get_execution / prepare_plan_execution -> plan_execution 进入协调面；Room member
// 必须由 WorkBinding 或 ReviewBinding 获得逐 round mutation capability。裸 @ / 用户定向
// 消息产生的无 binding round 只是 conversation transport；可以通过独立 read model
// 观察共享 WorkGraph，但不能用本栅栏取得责任或修改 WorkGraph。
func scopeSnapshotToTrustedWorkBinding(
	actor ActorContext,
	requestedExecutionID string,
	snapshot *protocol.ExecutionSnapshot,
) (*protocol.ExecutionSnapshot, error) {
	if actor.WorkBinding != nil && actor.ReviewBinding != nil {
		return nil, workBindingMismatch(
			"Room runtime cannot carry worker and reviewer bindings together",
		)
	}
	if actor.ReviewBinding != nil {
		binding := normalizeExecutionReviewBinding(actor.ReviewBinding)
		if !completeExecutionReviewBinding(binding) ||
			strings.TrimSpace(requestedExecutionID) != binding.ExecutionID ||
			(actor.ExecutionID != "" &&
				strings.TrimSpace(actor.ExecutionID) != binding.ExecutionID) ||
			strings.TrimSpace(actor.AgentID) != binding.TargetAgentID ||
			snapshot == nil ||
			snapshot.Execution.ID != binding.ExecutionID {
			return nil, workBindingMismatch(
				"structured Room review binding is outside its selected reviewer Execution",
			)
		}
		dispatch := protocol.ExecutionReviewDispatch{
			ID:            binding.ReviewDispatchID,
			ExecutionID:   binding.ExecutionID,
			PlanID:        binding.PlanID,
			WorkItemID:    binding.WorkItemID,
			SpecID:        binding.SpecID,
			AssignmentID:  binding.AssignmentID,
			SubmissionID:  binding.SubmissionID,
			TargetAgentID: binding.TargetAgentID,
		}
		if _, err := authorizeReviewDispatchSnapshot(
			snapshot,
			dispatch,
			binding.TargetAgentID,
		); err != nil {
			return nil, workBindingMismatch(err.Error())
		}
		return snapshotForExecutionReviewBinding(snapshot, binding)
	}
	if snapshot != nil &&
		snapshot.Execution.ScopeKind == protocol.ExecutionScopeRoom &&
		actor.ScopeKind == protocol.ExecutionScopeRoom &&
		actor.WorkBinding == nil &&
		strings.TrimSpace(snapshot.Execution.CoordinatorAgentID) ==
			strings.TrimSpace(actor.AgentID) {
		// 只有 Execution 自己记录的 coordinator 可以通过显式 read/plan
		// 调用进入协调面；round-start context 仍保持 conversation overlay。
		return snapshot, nil
	}
	if unboundRoomConversationActor(actor, snapshot) {
		return nil, domainError(
			ErrorCodeConversationOnly,
			"this Room round is conversational and carries no trusted WorkBinding or ReviewBinding",
		)
	}
	if snapshot == nil ||
		snapshot.Execution.ScopeKind != protocol.ExecutionScopeRoom ||
		actor.ScopeKind != protocol.ExecutionScopeRoom ||
		actor.WorkBinding == nil {
		return snapshot, nil
	}
	binding := normalizeExecutionWorkBinding(actor.WorkBinding)
	if err := authorizeStructuredRoomWorkBinding(
		actor,
		requestedExecutionID,
		snapshot,
		binding,
	); err != nil {
		return nil, err
	}
	return snapshotForExecutionWorkBinding(snapshot, binding)
}

func snapshotForExecutionReviewBinding(
	snapshot *protocol.ExecutionSnapshot,
	binding protocol.ExecutionReviewBinding,
) (*protocol.ExecutionSnapshot, error) {
	return snapshotForExecutionWorkBinding(snapshot, protocol.ExecutionWorkBinding{
		ExecutionID:  binding.ExecutionID,
		PlanID:       binding.PlanID,
		WorkItemID:   binding.WorkItemID,
		SpecID:       binding.SpecID,
		AssignmentID: binding.AssignmentID,
	})
}

func unboundRoomConversationActor(
	actor ActorContext,
	snapshot *protocol.ExecutionSnapshot,
) bool {
	return snapshot != nil &&
		snapshot.Execution.ScopeKind == protocol.ExecutionScopeRoom &&
		actor.ScopeKind == protocol.ExecutionScopeRoom &&
		normalizeActorKind(actor.ActorKind) == protocol.ExecutionActorAgent &&
		actor.WorkBinding == nil &&
		actor.ReviewBinding == nil &&
		strings.TrimSpace(actor.GoalID) == "" &&
		actor.GoalObjectiveRevision <= 0 &&
		strings.TrimSpace(actor.AgentID) != ""
}

func authorizeStructuredRoomWorkBinding(
	actor ActorContext,
	requestedExecutionID string,
	snapshot *protocol.ExecutionSnapshot,
	binding protocol.ExecutionWorkBinding,
) error {
	if snapshot == nil {
		return workBindingMismatch("structured Room WorkBinding has no active Execution Plan")
	}
	if !isCurrentExecutionStatus(snapshot.Execution.Status) {
		return domainError(
			ErrorCodeExecutionTerminal,
			"the bound Room work was superseded or closed; stop this old round and wait for a fresh Assignment",
		)
	}
	if snapshot.Plan == nil {
		return workBindingMismatch("structured Room WorkBinding has no active Execution Plan")
	}
	if snapshot.Execution.Status == protocol.ExecutionStatusPaused {
		return workBindingMismatch("structured Room WorkBinding targets a paused Execution")
	}
	if !completeExecutionWorkBinding(binding) {
		return workBindingMismatch("structured Room WorkBinding is incomplete")
	}
	actorAgentID := strings.TrimSpace(actor.AgentID)
	if actorAgentID == "" ||
		strings.TrimSpace(requestedExecutionID) != binding.ExecutionID ||
		(actor.ExecutionID != "" && strings.TrimSpace(actor.ExecutionID) != binding.ExecutionID) ||
		snapshot.Execution.ID != binding.ExecutionID ||
		snapshot.Plan.ID != binding.PlanID ||
		snapshot.Plan.Status != protocol.PlanRevisionStatusActive {
		return workBindingMismatch("structured Room runtime is outside its bound Execution Plan")
	}

	workMatched := false
	for _, work := range snapshot.WorkItems {
		if work.ID == binding.WorkItemID &&
			work.ExecutionID == binding.ExecutionID {
			workMatched = true
			break
		}
	}
	if !workMatched {
		return workBindingMismatch("structured Room Work Item binding is stale")
	}

	specMatched := false
	for _, spec := range snapshot.WorkItemSpecs {
		if spec.ID == binding.SpecID &&
			spec.ExecutionID == binding.ExecutionID &&
			spec.WorkItemID == binding.WorkItemID {
			specMatched = true
			break
		}
	}
	if !specMatched {
		return workBindingMismatch("structured Room Work Item spec binding is stale")
	}

	stateMatched := false
	for _, state := range snapshot.WorkItemStates {
		if state.ExecutionID == binding.ExecutionID &&
			state.WorkItemID == binding.WorkItemID &&
			state.CurrentSpecID == binding.SpecID {
			stateMatched = true
			break
		}
	}
	if !stateMatched {
		return workBindingMismatch("structured Room Work Item state is outside its bound spec")
	}

	planItemMatched := false
	for _, item := range snapshot.PlanItems {
		if item.ExecutionID == binding.ExecutionID &&
			item.PlanID == binding.PlanID &&
			item.WorkItemID == binding.WorkItemID &&
			item.SpecID == binding.SpecID {
			planItemMatched = true
			break
		}
	}
	if !planItemMatched {
		return workBindingMismatch("structured Room Work Item is outside its bound Plan")
	}

	var assignment *protocol.WorkAssignment
	for index := range snapshot.Assignments {
		candidate := &snapshot.Assignments[index]
		if candidate.ID == binding.AssignmentID {
			assignment = candidate
			break
		}
	}
	if assignment == nil ||
		assignment.ExecutionID != binding.ExecutionID ||
		assignment.PlanID != binding.PlanID ||
		assignment.WorkItemID != binding.WorkItemID ||
		assignment.SpecID != binding.SpecID ||
		strings.TrimSpace(assignment.OwnerAgentID) != actorAgentID ||
		!currentAssignment(*assignment) {
		return workBindingMismatch("structured Room Assignment binding is stale")
	}

	if assignment.Strategy == protocol.AssignmentStrategySelf {
		if binding.DispatchID != "" {
			return workBindingMismatch("self Room WorkBinding must not carry a Dispatch")
		}
	} else {
		if binding.DispatchID == "" {
			return workBindingMismatch("dispatched Room WorkBinding is missing its Dispatch")
		}
		dispatchMatched := false
		for _, dispatch := range snapshot.Dispatches {
			if dispatch.ID == binding.DispatchID &&
				dispatch.ExecutionID == binding.ExecutionID &&
				dispatch.PlanID == binding.PlanID &&
				dispatch.WorkItemID == binding.WorkItemID &&
				dispatch.SpecID == binding.SpecID &&
				dispatch.AssignmentID == binding.AssignmentID &&
				strings.TrimSpace(dispatch.TargetAgentID) == actorAgentID &&
				dispatch.Status != protocol.ExecutionDispatchStatusCancelled &&
				dispatch.Status != protocol.ExecutionDispatchStatusFailed {
				dispatchMatched = true
				break
			}
		}
		if !dispatchMatched {
			return workBindingMismatch("structured Room Dispatch binding is stale")
		}
	}

	attemptMatched := false
	for _, attempt := range snapshot.Attempts {
		if attempt.ID == binding.AttemptID &&
			attempt.ExecutionID == binding.ExecutionID &&
			attempt.PlanID == binding.PlanID &&
			attempt.WorkItemID == binding.WorkItemID &&
			attempt.SpecID == binding.SpecID &&
			attempt.AssignmentID == binding.AssignmentID &&
			attempt.DispatchID == binding.DispatchID &&
			attempt.ParentAttemptID == "" &&
			attempt.ExecutorKind == protocol.AttemptExecutorAgent &&
			strings.TrimSpace(attempt.ExecutorAgentID) == actorAgentID {
			attemptMatched = true
			break
		}
	}
	if !attemptMatched {
		return workBindingMismatch("structured Room root Attempt binding is stale")
	}
	return nil
}

func snapshotForExecutionWorkBinding(
	snapshot *protocol.ExecutionSnapshot,
	binding protocol.ExecutionWorkBinding,
) (*protocol.ExecutionSnapshot, error) {
	result := *snapshot
	if snapshot.Plan != nil {
		plan := *snapshot.Plan
		result.Plan = &plan
	}
	upstreamIDs, err := workBindingUpstreamIDs(snapshot, binding)
	if err != nil {
		return nil, err
	}
	includedWorkItemIDs := map[string]bool{binding.WorkItemID: true}
	for _, workItemID := range upstreamIDs {
		includedWorkItemIDs[workItemID] = true
	}
	includedSpecIDs := map[string]string{binding.WorkItemID: binding.SpecID}
	for _, item := range snapshot.PlanItems {
		if item.PlanID == binding.PlanID && includedWorkItemIDs[item.WorkItemID] {
			includedSpecIDs[item.WorkItemID] = item.SpecID
		}
	}
	view := newExecutionContextView(snapshot)
	acceptedUpstreamSubmissions := make(map[string]bool, len(upstreamIDs))
	acceptedUpstreamAcceptances := make(map[string]bool, len(upstreamIDs))
	for _, workItemID := range upstreamIDs {
		submission, acceptance := view.acceptedDelivery(
			workItemID,
			includedSpecIDs[workItemID],
		)
		if submission != nil && acceptance != nil {
			acceptedUpstreamSubmissions[submission.ID] = true
			acceptedUpstreamAcceptances[acceptance.ID] = true
		}
	}
	result.WorkItems = filterValues(snapshot.WorkItems, func(value protocol.WorkItem) bool {
		return includedWorkItemIDs[value.ID] &&
			value.ExecutionID == binding.ExecutionID
	})
	slices.SortFunc(result.WorkItems, func(left, right protocol.WorkItem) int {
		if compared := strings.Compare(left.LogicalKey, right.LogicalKey); compared != 0 {
			return compared
		}
		return strings.Compare(left.ID, right.ID)
	})
	result.WorkItemStates = filterValues(snapshot.WorkItemStates, func(value protocol.WorkItemState) bool {
		return value.WorkItemID == binding.WorkItemID &&
			value.ExecutionID == binding.ExecutionID &&
			value.CurrentSpecID == binding.SpecID
	})
	result.WorkItemSpecs = filterValues(snapshot.WorkItemSpecs, func(value protocol.WorkItemSpec) bool {
		return value.ID == includedSpecIDs[value.WorkItemID] &&
			includedWorkItemIDs[value.WorkItemID] &&
			value.ExecutionID == binding.ExecutionID
	})
	slices.SortFunc(result.WorkItemSpecs, func(left, right protocol.WorkItemSpec) int {
		if compared := strings.Compare(left.WorkItemID, right.WorkItemID); compared != 0 {
			return compared
		}
		return strings.Compare(left.ID, right.ID)
	})
	result.PlanItems = filterValues(snapshot.PlanItems, func(value protocol.ExecutionPlanItem) bool {
		return value.PlanID == binding.PlanID &&
			includedWorkItemIDs[value.WorkItemID] &&
			value.SpecID == includedSpecIDs[value.WorkItemID]
	})
	slices.SortFunc(result.PlanItems, func(left, right protocol.ExecutionPlanItem) int {
		if left.Position != right.Position {
			return left.Position - right.Position
		}
		return strings.Compare(left.WorkItemID, right.WorkItemID)
	})
	result.Dependencies = filterValues(
		snapshot.Dependencies,
		func(value protocol.ExecutionPlanDependency) bool {
			return value.PlanID == binding.PlanID &&
				value.WorkItemID == binding.WorkItemID &&
				includedWorkItemIDs[value.DependsOnWorkItemID]
		},
	)
	slices.SortFunc(
		result.Dependencies,
		func(left, right protocol.ExecutionPlanDependency) int {
			if compared := strings.Compare(
				left.DependsOnWorkItemID,
				right.DependsOnWorkItemID,
			); compared != 0 {
				return compared
			}
			return strings.Compare(string(left.Kind), string(right.Kind))
		},
	)
	result.OutputClaims = filterValues(
		snapshot.OutputClaims,
		func(value protocol.ExecutionPlanOutputClaim) bool {
			return value.PlanID == binding.PlanID &&
				includedWorkItemIDs[value.WorkItemID] &&
				value.SpecID == includedSpecIDs[value.WorkItemID]
		},
	)
	slices.SortFunc(
		result.OutputClaims,
		func(left, right protocol.ExecutionPlanOutputClaim) int {
			if compared := strings.Compare(left.WorkItemID, right.WorkItemID); compared != 0 {
				return compared
			}
			if compared := strings.Compare(left.Scope, right.Scope); compared != 0 {
				return compared
			}
			return strings.Compare(string(left.Mode), string(right.Mode))
		},
	)
	result.Assignments = filterValues(snapshot.Assignments, func(value protocol.WorkAssignment) bool {
		return value.ID == binding.AssignmentID
	})
	result.Dispatches = filterValues(snapshot.Dispatches, func(value protocol.ExecutionDispatch) bool {
		return value.ID == binding.DispatchID &&
			value.AssignmentID == binding.AssignmentID
	})
	result.Attempts = filterValues(snapshot.Attempts, func(value protocol.WorkAttempt) bool {
		return value.AssignmentID == binding.AssignmentID
	})
	result.Submissions = filterValues(snapshot.Submissions, func(value protocol.WorkSubmission) bool {
		return value.AssignmentID == binding.AssignmentID ||
			acceptedUpstreamSubmissions[value.ID]
	})
	slices.SortFunc(result.Submissions, func(left, right protocol.WorkSubmission) int {
		if compared := strings.Compare(left.WorkItemID, right.WorkItemID); compared != 0 {
			return compared
		}
		if left.Sequence != right.Sequence {
			if left.Sequence < right.Sequence {
				return -1
			}
			return 1
		}
		return strings.Compare(left.ID, right.ID)
	})
	result.Acceptances = filterValues(snapshot.Acceptances, func(value protocol.WorkAcceptance) bool {
		return value.AssignmentID == binding.AssignmentID ||
			acceptedUpstreamAcceptances[value.ID]
	})
	slices.SortFunc(result.Acceptances, func(left, right protocol.WorkAcceptance) int {
		if compared := strings.Compare(left.WorkItemID, right.WorkItemID); compared != 0 {
			return compared
		}
		return strings.Compare(left.ID, right.ID)
	})
	result.ReviewDispatches = filterValues(
		snapshot.ReviewDispatches,
		func(value protocol.ExecutionReviewDispatch) bool {
			return value.AssignmentID == binding.AssignmentID
		},
	)
	result.CancellationDispatches = filterValues(
		snapshot.CancellationDispatches,
		func(value protocol.ExecutionCancellationDispatch) bool {
			return value.AssignmentID == binding.AssignmentID
		},
	)
	result.ReadyWorkItemIDs = filterValues(snapshot.ReadyWorkItemIDs, func(value string) bool {
		return strings.TrimSpace(value) == binding.WorkItemID
	})
	result.CompletionBlockers = nil
	if err := validateExecutionWorkBindingProjection(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func validateExecutionWorkBindingProjection(snapshot *protocol.ExecutionSnapshot) error {
	if snapshot == nil {
		return nil
	}
	if err := protocol.ValidateExecutionProjectionLimit(
		"completion_criteria",
		len(snapshot.Execution.CompletionCriteria),
	); err != nil {
		return err
	}
	for _, spec := range snapshot.WorkItemSpecs {
		for _, collection := range []struct {
			field string
			count int
		}{
			{field: "acceptance_criteria", count: len(spec.AcceptanceCriteria)},
			{field: "input_refs", count: len(spec.InputRefs)},
		} {
			if err := protocol.ValidateExecutionProjectionLimit(
				collection.field,
				collection.count,
			); err != nil {
				return err
			}
		}
	}
	dependencyCounts := make(map[string]int)
	outputScopeCounts := make(map[string]int)
	for _, dependency := range snapshot.Dependencies {
		dependencyCounts[dependency.WorkItemID]++
		if err := protocol.ValidateExecutionProjectionLimit(
			"depends_on",
			dependencyCounts[dependency.WorkItemID],
		); err != nil {
			return err
		}
	}
	for _, claim := range snapshot.OutputClaims {
		key := claim.WorkItemID + "\x00" + claim.SpecID
		outputScopeCounts[key]++
		if err := protocol.ValidateExecutionProjectionLimit(
			"output_scopes",
			outputScopeCounts[key],
		); err != nil {
			return err
		}
	}
	for _, submission := range snapshot.Submissions {
		for _, collection := range []struct {
			field string
			count int
		}{
			{field: "result_refs", count: len(submission.ResultRefs)},
			{field: "submission_evidence", count: len(submission.Evidence)},
		} {
			if err := protocol.ValidateExecutionProjectionLimit(
				collection.field,
				collection.count,
			); err != nil {
				return err
			}
		}
	}
	for _, acceptance := range snapshot.Acceptances {
		if err := protocol.ValidateExecutionProjectionLimit(
			"criteria_results",
			len(acceptance.CriteriaResults),
		); err != nil {
			return err
		}
		for _, result := range acceptance.CriteriaResults {
			if err := protocol.ValidateExecutionProjectionLimit(
				"criteria_results.evidence",
				len(result.Evidence),
			); err != nil {
				return err
			}
		}
	}
	for _, state := range snapshot.WorkItemStates {
		evidence := metadataTextList(state.Metadata, "last_resume_evidence")
		total := metadataTextListTotal(state.Metadata, "last_resume_evidence", len(evidence))
		if err := protocol.ValidateExecutionProjectionLimit(
			"resume_evidence",
			total,
		); err != nil {
			return err
		}
	}
	return nil
}

func workBindingUpstreamIDs(
	snapshot *protocol.ExecutionSnapshot,
	binding protocol.ExecutionWorkBinding,
) ([]string, error) {
	values := make([]string, 0)
	seen := make(map[string]struct{})
	for _, dependency := range snapshot.Dependencies {
		workItemID := strings.TrimSpace(dependency.DependsOnWorkItemID)
		if dependency.PlanID != binding.PlanID ||
			dependency.WorkItemID != binding.WorkItemID ||
			workItemID == "" {
			continue
		}
		if _, exists := seen[workItemID]; exists {
			continue
		}
		seen[workItemID] = struct{}{}
		values = append(values, workItemID)
	}
	slices.Sort(values)
	if err := protocol.ValidateExecutionProjectionLimit("depends_on", len(values)); err != nil {
		return nil, err
	}
	return values, nil
}

func normalizeExecutionWorkBinding(
	binding *protocol.ExecutionWorkBinding,
) protocol.ExecutionWorkBinding {
	if binding == nil {
		return protocol.ExecutionWorkBinding{}
	}
	return protocol.ExecutionWorkBinding{
		ExecutionID:  strings.TrimSpace(binding.ExecutionID),
		PlanID:       strings.TrimSpace(binding.PlanID),
		WorkItemID:   strings.TrimSpace(binding.WorkItemID),
		SpecID:       strings.TrimSpace(binding.SpecID),
		AssignmentID: strings.TrimSpace(binding.AssignmentID),
		AttemptID:    strings.TrimSpace(binding.AttemptID),
		DispatchID:   strings.TrimSpace(binding.DispatchID),
	}
}

func completeExecutionWorkBinding(binding protocol.ExecutionWorkBinding) bool {
	return binding.ExecutionID != "" &&
		binding.PlanID != "" &&
		binding.WorkItemID != "" &&
		binding.SpecID != "" &&
		binding.AssignmentID != "" &&
		binding.AttemptID != ""
}

func workBindingMismatch(message string) error {
	return domainError(ErrorCodeWorkBindingMismatch, message)
}

func filterValues[T any](values []T, keep func(T) bool) []T {
	result := make([]T, 0, len(values))
	for _, value := range values {
		if keep(value) {
			result = append(result, value)
		}
	}
	return result
}

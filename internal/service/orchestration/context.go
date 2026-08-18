// INPUT: 当前 Execution snapshot、current Spec/output claims/Acceptance 与 runtime actor 身份。
// OUTPUT: 有界、确定序且 XML 安全的 <nexus_execution_context>，异常历史超限显式标记 truncated/total。
// POS: DM、Room、compact recovery 与 Goal continuation 共用的动态执行上下文投影。
package orchestration

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ExecutionActorRole 表示动态上下文中的执行权限角色。
type ExecutionActorRole string

const (
	ExecutionActorCoordinator ExecutionActorRole = "coordinator"
	ExecutionActorMember      ExecutionActorRole = "member"
	ExecutionActorSubagent    ExecutionActorRole = "subagent"

	executionGraphDigestEdgeLimit = protocol.ExecutionProjectionCollectionLimit * 4
	executionActionTransportScope = "allowed_actions and forbidden_actions are semantic operation names, not tool-schema or MCP names; load execution-orchestrator and invoke listed Execution operations only through the host-injected \"${NEXUS_COMMAND_PATH}\" --json execution contract|inspect|invoke command; never use nexusctl; native Agent delegation and all other task tools remain governed by task and tool policy"
)

// ExecutionContextOptions 提供不能从 snapshot 唯一推导的当前 actor 信息。
type ExecutionContextOptions struct {
	ActorAgentID            string
	Role                    ExecutionActorRole
	ScopeKind               protocol.ExecutionScopeKind
	WorkBound               bool
	ReviewBound             bool
	PlanMode                bool
	GoalPromotionReasons    []protocol.GoalActivationReason
	GoalPromotionBlockers   []string
	RuntimeGraph            *protocol.ExecutionRuntimeGraph
	RuntimeGraphRelation    string
	RuntimeGraphUnavailable bool
}

// RenderUnmanagedExecutionContext 明确当前没有权威 WorkGraph，避免模型把缺少
// 动态块误读成可以抢占 Room coordinator 或自行发明 Assignment。
func RenderUnmanagedExecutionContext(options ExecutionContextOptions) string {
	role := options.Role
	if role != ExecutionActorCoordinator &&
		role != ExecutionActorMember &&
		role != ExecutionActorSubagent {
		if options.ScopeKind == protocol.ExecutionScopeDM {
			role = ExecutionActorCoordinator
		} else {
			role = ExecutionActorMember
		}
	}
	allowed := []string(nil)
	if !options.PlanMode {
		allowed = append(allowed, "Agent")
	}
	if role == ExecutionActorCoordinator {
		allowed = append(allowed, "get_execution", "prepare_plan_execution")
		if !options.PlanMode {
			allowed = append(allowed, "plan_execution")
		}
	}
	forbidden := []string{
		"get_execution",
		"prepare_plan_execution",
		"plan_execution",
		"abandon_execution",
		"assign_work",
		"submit_work",
		"review_work",
		"block_work",
		"resume_work",
		"take_over_work",
		"audit_execution_alignment",
		"promote_execution_to_goal",
	}
	if role != ExecutionActorCoordinator {
		forbidden = append(forbidden, "create_shared_execution")
	} else {
		forbidden = slices.DeleteFunc(forbidden, func(value string) bool {
			return value == "get_execution" || value == "prepare_plan_execution" ||
				(!options.PlanMode && value == "plan_execution")
		})
	}
	if options.PlanMode {
		forbidden = append(forbidden, "execute_work_in_plan_mode", "Agent")
	}

	var output strings.Builder
	output.WriteString(`<nexus_execution_context execution_version="0">`)
	fmt.Fprintf(
		&output,
		"\n  <scope type=\"%s\" />",
		xmlValue(string(options.ScopeKind)),
	)
	fmt.Fprintf(
		&output,
		"\n  <actor agent_id=\"%s\" role=\"%s\" />",
		xmlValue(options.ActorAgentID),
		xmlValue(string(role)),
	)
	fmt.Fprintf(
		&output,
		"\n  <lane type=\"%s\" />",
		xmlValue(executionContextLane(role, options)),
	)
	fmt.Fprintf(&output, "\n  <mode plan_only=\"%t\" />", options.PlanMode)
	output.WriteString("\n  <execution state=\"unmanaged\" />")
	if role == ExecutionActorCoordinator {
		writeXMLTextElement(
			&output,
			2,
			"boundary",
			"no authoritative Work Item ownership exists; do direct atomic work or validate a complete Plan before coordinated execution",
		)
	} else {
		writeXMLTextElement(
			&output,
			2,
			"boundary",
			"no managed Assignment exists; act only on the current legacy Room trigger and do not create shared work",
		)
	}
	renderRuntimeGraphFacts(&output, options)
	writeXMLTextElement(
		&output,
		2,
		"action_scope",
		executionActionTransportScope,
	)
	renderStringList(&output, "allowed_actions", "action", allowed)
	renderStringList(&output, "forbidden_actions", "action", forbidden)
	output.WriteString("\n</nexus_execution_context>")
	return output.String()
}

// RenderConversationExecutionContext 明确当前 Room round 只属于对话平面。
// Room 中可以同时存在后台 Execution，但没有 trusted binding 的参与者不能
// 因看到聊天内容或裸 @ 而获得 Work Item、Submission 或 Acceptance 权限。
func RenderConversationExecutionContext(
	snapshot *protocol.ExecutionSnapshot,
	options ExecutionContextOptions,
) string {
	if snapshot == nil || strings.TrimSpace(snapshot.Execution.ID) == "" {
		return RenderUnmanagedExecutionContext(options)
	}
	role := ExecutionActorMember
	if options.Role == ExecutionActorCoordinator {
		role = ExecutionActorCoordinator
	}
	allowed := []string(nil)
	if !options.PlanMode {
		allowed = append(allowed, "Agent")
	}
	if role == ExecutionActorCoordinator {
		allowed = append(allowed, "get_execution", "prepare_plan_execution")
		if !options.PlanMode {
			allowed = append(allowed, "plan_execution")
		}
	}
	forbidden := []string{
		"abandon_execution",
		"assign_work",
		"submit_work",
		"review_work",
		"block_work",
		"resume_work",
		"take_over_work",
		"audit_execution_alignment",
		"promote_execution_to_goal",
		"treat_conversation_as_work_evidence",
	}
	if options.PlanMode {
		forbidden = append(forbidden, "Agent")
	}
	if role != ExecutionActorCoordinator {
		forbidden = append([]string{"get_execution", "prepare_plan_execution", "plan_execution"}, forbidden...)
	} else if options.PlanMode {
		forbidden = append([]string{"plan_execution"}, forbidden...)
	}

	var output strings.Builder
	fmt.Fprintf(
		&output,
		`<nexus_execution_context execution_version="%d">`,
		snapshot.Execution.Version,
	)
	fmt.Fprintf(
		&output,
		"\n  <scope type=\"%s\" session_key=\"%s\" />",
		xmlValue(string(snapshot.Execution.ScopeKind)),
		xmlValue(snapshot.Execution.SessionKey),
	)
	fmt.Fprintf(
		&output,
		"\n  <actor agent_id=\"%s\" role=\"%s\" lane=\"conversation\" />",
		xmlValue(options.ActorAgentID),
		xmlValue(string(role)),
	)
	fmt.Fprintf(&output, "\n  <mode plan_only=\"%t\" />", options.PlanMode)
	fmt.Fprintf(
		&output,
		"\n  <execution id=\"%s\" status=\"%s\" relation=\"background\" />",
		xmlValue(snapshot.Execution.ID),
		xmlValue(string(snapshot.Execution.Status)),
	)
	writeXMLTextElement(
		&output,
		2,
		"boundary",
		"this round has no trusted WorkBinding or ReviewBinding; respond only to the conversation trigger and do not perform, claim, submit, block, review, or complete managed work",
	)
	if role == ExecutionActorCoordinator {
		output.WriteString("\n  <coordination_transition available=\"true\">")
		writeXMLTextElement(
			&output,
			4,
			"rule",
			"stay in conversation for chat, brainstorming, and untracked one-offs; call get_execution to inspect existing responsibility, or prepare_plan_execution then plan_execution to deliberately enter coordinated accountable delivery",
		)
		writeXMLTextElement(
			&output,
			4,
			"trigger",
			"prepare and commit a Plan only when the request needs multiple tracked deliverables, dependencies, durable handoff, acceptance, or cross-boundary continuation; participant count and raw mentions are never sufficient",
		)
		output.WriteString("\n  </coordination_transition>")
	} else {
		output.WriteString("\n  <coordination_transition available=\"false\" />")
	}
	writeXMLTextElement(
		&output,
		2,
		"handoff",
		"raw mentions are conversation transport only; accountable work arrives in a separate structured dispatch carrying a WorkBinding, and review arrives with a ReviewBinding",
	)
	renderRuntimeGraphFacts(&output, options)
	writeXMLTextElement(
		&output,
		2,
		"action_scope",
		executionActionTransportScope,
	)
	renderStringList(&output, "allowed_actions", "action", allowed)
	renderStringList(&output, "forbidden_actions", "action", forbidden)
	output.WriteString("\n</nexus_execution_context>")
	return output.String()
}

// RenderExecutionContext 生成模型恢复和决策所需的有界权威状态。
func RenderExecutionContext(snapshot *protocol.ExecutionSnapshot, options ExecutionContextOptions) string {
	if snapshot == nil || strings.TrimSpace(snapshot.Execution.ID) == "" {
		return ""
	}
	view := newExecutionContextView(snapshot)
	role := normalizeExecutionActorRole(snapshot.Execution, options)

	var output strings.Builder
	fmt.Fprintf(
		&output,
		`<nexus_execution_context execution_version="%d">`,
		snapshot.Execution.Version,
	)
	fmt.Fprintf(
		&output,
		"\n  <scope type=\"%s\" session_key=\"%s\" />",
		xmlValue(string(snapshot.Execution.ScopeKind)),
		xmlValue(snapshot.Execution.SessionKey),
	)
	fmt.Fprintf(
		&output,
		"\n  <actor agent_id=\"%s\" role=\"%s\" />",
		xmlValue(options.ActorAgentID),
		xmlValue(string(role)),
	)
	fmt.Fprintf(
		&output,
		"\n  <lane type=\"%s\" />",
		xmlValue(executionContextLane(role, options)),
	)
	fmt.Fprintf(&output, "\n  <mode plan_only=\"%t\" />", options.PlanMode)
	if strings.TrimSpace(snapshot.Execution.GoalID) != "" {
		fmt.Fprintf(
			&output,
			"\n  <goal id=\"%s\" objective_revision=\"%d\" />",
			xmlValue(snapshot.Execution.GoalID),
			snapshot.Execution.GoalObjectiveRevision,
		)
	}
	fmt.Fprintf(
		&output,
		"\n  <execution id=\"%s\" status=\"%s\"",
		xmlValue(snapshot.Execution.ID),
		xmlValue(string(snapshot.Execution.Status)),
	)
	if snapshot.Plan != nil {
		fmt.Fprintf(
			&output,
			" plan_id=\"%s\" plan_revision=\"%d\"",
			xmlValue(snapshot.Plan.ID),
			snapshot.Plan.Revision,
		)
	}
	output.WriteString(">")
	writeXMLTextElement(&output, 4, "objective", snapshot.Execution.Objective)
	renderStringListAtIndentWithTotal(
		&output,
		4,
		"completion_criteria",
		"criterion",
		normalizeNonEmptyValues(snapshot.Execution.CompletionCriteria),
		len(snapshot.Execution.CompletionCriteria),
	)
	output.WriteString("\n  </execution>")

	renderExecutionGraphDigest(
		&output,
		snapshot,
		role,
		strings.TrimSpace(options.ActorAgentID),
	)
	renderRuntimeGraphFacts(&output, options)
	renderAssignedWork(&output, view, options.ActorAgentID)
	renderActiveAssignments(&output, view, role)
	renderReadyWork(&output, view, role)
	renderPendingReviews(&output, view, role, options.ActorAgentID)
	renderResumableWork(&output, view, role, options.ActorAgentID)
	renderPlanRevisionBoundary(&output, view, role, options)
	renderExecutionTransitionBoundary(&output, snapshot, role, options)
	renderGoalPromotionBoundary(&output, snapshot, options)
	subagentEligible := renderSubagentAdmissionBoundary(&output, snapshot, options)
	writeXMLTextElement(
		&output,
		2,
		"action_scope",
		executionActionTransportScope,
	)
	renderActionBoundary(&output, view, role, options, subagentEligible)
	renderCompletionBlockers(&output, snapshot.CompletionBlockers)
	output.WriteString("\n</nexus_execution_context>")
	return output.String()
}

type executionGraphDigestEdge struct {
	from string
	to   string
	kind protocol.WorkDependencyKind
}

// renderExecutionGraphDigest 给模型一个从权威 WorkGraph 派生的确定性拓扑摘要。
// 它不是写入协议：模型仍只能通过 typed Execution tools 修改状态，UI Mermaid
// 等可视化也必须从同一 snapshot 单向派生。
func renderExecutionGraphDigest(
	output *strings.Builder,
	snapshot *protocol.ExecutionSnapshot,
	role ExecutionActorRole,
	actorAgentID string,
) {
	projected := ProjectExecutionView(snapshot)
	if projected == nil || projected.Plan == nil || len(projected.WorkItems) == 0 {
		return
	}
	itemsByID := make(
		map[string]protocol.ExecutionWorkItemView,
		len(projected.WorkItems),
	)
	included := make(map[string]bool, len(projected.WorkItems))
	for _, item := range projected.WorkItems {
		itemsByID[item.ID] = item
		if role == ExecutionActorCoordinator ||
			executionGraphItemBelongsToActor(item, actorAgentID) {
			included[item.ID] = true
		}
	}
	scope := "full"
	if role != ExecutionActorCoordinator {
		scope = "actor_slice"
		for changed := true; changed; {
			changed = false
			for workItemID := range included {
				for _, dependencyID := range itemsByID[workItemID].DependencyIDs {
					if !included[dependencyID] {
						included[dependencyID] = true
						changed = true
					}
				}
			}
		}
	}
	if len(included) == 0 {
		return
	}

	fmt.Fprintf(
		output,
		"\n  <graph_digest notation=\"nexus-dag-v1\" scope=\"%s\" plan_revision=\"%d\">",
		scope,
		projected.Plan.Revision,
	)
	output.WriteString("\n    <nodes>")
	logicalKeys := make(map[string]string, len(included))
	for _, item := range projected.WorkItems {
		if !included[item.ID] {
			continue
		}
		logicalKeys[item.ID] = item.LogicalKey
		fmt.Fprintf(
			output,
			"\n      <node key=\"%s\" subject=\"%s\" kind=\"%s\" status=\"%s\"",
			xmlValue(item.LogicalKey),
			xmlValue(item.Subject),
			xmlValue(string(item.Kind)),
			xmlValue(string(item.Status)),
		)
		if item.Required {
			output.WriteString(` required="true"`)
		}
		if item.Terminal {
			output.WriteString(` terminal="true"`)
		}
		if item.OwnerAgentID != "" &&
			(role == ExecutionActorCoordinator ||
				item.OwnerAgentID == actorAgentID) {
			fmt.Fprintf(
				output,
				` owner_agent_id="%s"`,
				xmlValue(item.OwnerAgentID),
			)
		}
		if executionGraphItemBelongsToActor(item, actorAgentID) {
			output.WriteString(` current_actor="true"`)
		}
		output.WriteString(" />")
	}
	output.WriteString("\n    </nodes>")

	edges := make([]executionGraphDigestEdge, 0, len(snapshot.Dependencies))
	for _, dependency := range snapshot.Dependencies {
		if dependency.PlanID != projected.Plan.ID ||
			!included[dependency.WorkItemID] ||
			!included[dependency.DependsOnWorkItemID] {
			continue
		}
		from := logicalKeys[dependency.DependsOnWorkItemID]
		to := logicalKeys[dependency.WorkItemID]
		if from == "" || to == "" {
			continue
		}
		edges = append(edges, executionGraphDigestEdge{
			from: from,
			to:   to,
			kind: dependency.Kind,
		})
	}
	slices.SortFunc(edges, func(left, right executionGraphDigestEdge) int {
		if order := strings.Compare(left.from, right.from); order != 0 {
			return order
		}
		if order := strings.Compare(left.to, right.to); order != 0 {
			return order
		}
		return strings.Compare(string(left.kind), string(right.kind))
	})
	if len(edges) > executionGraphDigestEdgeLimit {
		fmt.Fprintf(
			output,
			"\n    <edges truncated=\"true\" total=\"%d\">",
			len(edges),
		)
	} else {
		output.WriteString("\n    <edges>")
	}
	for _, edge := range edges[:min(len(edges), executionGraphDigestEdgeLimit)] {
		fmt.Fprintf(
			output,
			"\n      <edge from=\"%s\" to=\"%s\" kind=\"%s\" />",
			xmlValue(edge.from),
			xmlValue(edge.to),
			xmlValue(string(edge.kind)),
		)
	}
	output.WriteString("\n    </edges>")
	output.WriteString("\n  </graph_digest>")
}

func executionGraphItemBelongsToActor(
	item protocol.ExecutionWorkItemView,
	actorAgentID string,
) bool {
	if actorAgentID == "" {
		return false
	}
	if item.OwnerAgentID == actorAgentID {
		return true
	}
	for _, attempt := range item.Attempts {
		if attempt.ExecutorAgentID == actorAgentID ||
			attempt.ParentAgentID == actorAgentID {
			return true
		}
	}
	return false
}

type executionContextView struct {
	snapshot             *protocol.ExecutionSnapshot
	workItems            map[string]protocol.WorkItem
	states               map[string]protocol.WorkItemState
	specs                map[string]protocol.WorkItemSpec
	planItems            map[string]protocol.ExecutionPlanItem
	dependencies         map[string][]protocol.ExecutionPlanDependency
	outputClaims         map[string][]protocol.ExecutionPlanOutputClaim
	assignments          map[string]protocol.WorkAssignment
	currentAssignments   map[string]protocol.WorkAssignment
	runningAttempts      map[string]protocol.WorkAttempt
	latestChildResults   map[string]protocol.WorkAttempt
	submissions          map[string]protocol.WorkSubmission
	submissionsByID      map[string]protocol.WorkSubmission
	acceptances          map[string]protocol.WorkAcceptance
	acceptedWorkItemSpec map[string]bool
	ready                map[string]bool
}

type resolvedDependencyProjection struct {
	dependency protocol.ExecutionPlanDependency
	workItem   protocol.WorkItem
	planItem   protocol.ExecutionPlanItem
	status     string
	blockers   []string
	submission *protocol.WorkSubmission
	acceptance *protocol.WorkAcceptance
}

func newExecutionContextView(snapshot *protocol.ExecutionSnapshot) executionContextView {
	view := executionContextView{
		snapshot:             snapshot,
		workItems:            make(map[string]protocol.WorkItem),
		states:               make(map[string]protocol.WorkItemState),
		specs:                make(map[string]protocol.WorkItemSpec),
		planItems:            make(map[string]protocol.ExecutionPlanItem),
		dependencies:         make(map[string][]protocol.ExecutionPlanDependency),
		outputClaims:         make(map[string][]protocol.ExecutionPlanOutputClaim),
		assignments:          make(map[string]protocol.WorkAssignment),
		currentAssignments:   make(map[string]protocol.WorkAssignment),
		runningAttempts:      make(map[string]protocol.WorkAttempt),
		latestChildResults:   make(map[string]protocol.WorkAttempt),
		submissions:          make(map[string]protocol.WorkSubmission),
		submissionsByID:      make(map[string]protocol.WorkSubmission),
		acceptances:          make(map[string]protocol.WorkAcceptance),
		acceptedWorkItemSpec: make(map[string]bool),
		ready:                make(map[string]bool),
	}
	for _, item := range snapshot.WorkItems {
		view.workItems[item.ID] = item
	}
	for _, state := range snapshot.WorkItemStates {
		view.states[state.WorkItemID] = state
	}
	for _, spec := range snapshot.WorkItemSpecs {
		view.specs[spec.ID] = spec
	}
	for _, item := range snapshot.PlanItems {
		if snapshot.Plan == nil || item.PlanID == snapshot.Plan.ID {
			view.planItems[item.WorkItemID] = item
		}
	}
	for _, dependency := range snapshot.Dependencies {
		if snapshot.Plan == nil || dependency.PlanID == snapshot.Plan.ID {
			view.dependencies[dependency.WorkItemID] = append(
				view.dependencies[dependency.WorkItemID],
				dependency,
			)
		}
	}
	for _, claim := range snapshot.OutputClaims {
		if snapshot.Plan == nil || claim.PlanID == snapshot.Plan.ID {
			view.outputClaims[claim.WorkItemID] = append(
				view.outputClaims[claim.WorkItemID],
				claim,
			)
		}
	}
	for _, assignment := range snapshot.Assignments {
		view.assignments[assignment.ID] = assignment
		if assignment.Status == protocol.WorkAssignmentStatusAssigned ||
			assignment.Status == protocol.WorkAssignmentStatusActive {
			view.currentAssignments[assignment.WorkItemID] = assignment
		}
	}
	for _, attempt := range snapshot.Attempts {
		if attempt.Status == protocol.WorkAttemptStatusPending ||
			attempt.Status == protocol.WorkAttemptStatusRunning {
			if strings.TrimSpace(attempt.ParentAttemptID) == "" {
				view.runningAttempts[attempt.AssignmentID] = attempt
			}
			continue
		}
		if attempt.ExecutorKind != protocol.AttemptExecutorSubagent {
			continue
		}
		current, exists := view.latestChildResults[attempt.AssignmentID]
		if !exists ||
			current.CreatedAt.Before(attempt.CreatedAt) ||
			(current.CreatedAt.Equal(attempt.CreatedAt) && current.ID < attempt.ID) {
			view.latestChildResults[attempt.AssignmentID] = attempt
		}
	}
	for _, submission := range snapshot.Submissions {
		view.submissionsByID[submission.ID] = submission
		current, ok := view.submissions[submission.WorkItemID]
		if !ok || submission.Sequence > current.Sequence {
			view.submissions[submission.WorkItemID] = submission
		}
	}
	for _, acceptance := range snapshot.Acceptances {
		view.acceptances[acceptance.SubmissionID] = acceptance
		if acceptance.Decision == protocol.WorkAcceptanceAccepted {
			view.acceptedWorkItemSpec[workSpecKey(acceptance.WorkItemID, acceptance.SpecID)] = true
		}
	}
	for workItemID, item := range view.planItems {
		view.ready[workItemID] = view.isReady(item)
	}
	return view
}

func (view executionContextView) isReady(item protocol.ExecutionPlanItem) bool {
	state, ok := view.states[item.WorkItemID]
	if !ok || state.CurrentSpecID != item.SpecID || state.Status != protocol.WorkItemStatusOpen {
		return false
	}
	work, workExists := view.workItems[item.WorkItemID]
	spec, specExists := view.specs[item.SpecID]
	if !workExists || !specExists ||
		work.ExecutionID != view.snapshot.Execution.ID ||
		spec.WorkItemID != item.WorkItemID ||
		spec.ExecutionID != view.snapshot.Execution.ID {
		return false
	}
	if view.acceptedWorkItemSpec[workSpecKey(item.WorkItemID, item.SpecID)] {
		return false
	}
	if _, assigned := view.currentAssignments[item.WorkItemID]; assigned {
		return false
	}
	for _, dependency := range view.dependencies[item.WorkItemID] {
		if dependency.Kind == protocol.WorkDependencySoft {
			continue
		}
		upstream, exists := view.planItems[dependency.DependsOnWorkItemID]
		if !exists || !view.acceptedWorkItemSpec[workSpecKey(upstream.WorkItemID, upstream.SpecID)] {
			return false
		}
	}
	return true
}

func renderAssignedWork(output *strings.Builder, view executionContextView, actorAgentID string) {
	assignments := make([]protocol.WorkAssignment, 0)
	for _, assignment := range view.currentAssignments {
		if assignment.OwnerAgentID == strings.TrimSpace(actorAgentID) {
			assignments = append(assignments, assignment)
		}
	}
	slices.SortFunc(assignments, func(left, right protocol.WorkAssignment) int {
		return strings.Compare(left.WorkItemID, right.WorkItemID)
	})
	output.WriteString("\n  <assigned_work>")
	for _, assignment := range assignments {
		item := view.workItems[assignment.WorkItemID]
		spec := view.specs[assignment.SpecID]
		status := view.deliveryStatus(assignment.WorkItemID)
		fmt.Fprintf(
			output,
			"\n    <item id=\"%s\" logical_key=\"%s\" spec_id=\"%s\" assignment_id=\"%s\" kind=\"%s\" status=\"%s\"",
			xmlValue(assignment.WorkItemID),
			xmlValue(item.LogicalKey),
			xmlValue(assignment.SpecID),
			xmlValue(assignment.ID),
			xmlValue(string(item.Kind)),
			xmlValue(status),
		)
		if attempt, ok := view.runningAttempts[assignment.ID]; ok {
			fmt.Fprintf(output, " attempt_id=\"%s\"", xmlValue(attempt.ID))
			if strings.TrimSpace(attempt.DispatchID) != "" {
				fmt.Fprintf(output, " dispatch_id=\"%s\"", xmlValue(attempt.DispatchID))
			}
		}
		output.WriteString(">")
		writeXMLTextElement(output, 6, "subject", spec.Subject)
		writeXMLTextElement(output, 6, "objective", spec.Objective)
		writeXMLTextElement(output, 6, "deliverable", spec.Deliverable)
		renderCriteria(output, spec.AcceptanceCriteria)
		renderInputRefs(output, spec.InputRefs)
		renderOutputScopes(output, view.outputScopes(assignment.WorkItemID, assignment.SpecID))
		renderDependencyIDs(output, view.dependencies[assignment.WorkItemID])
		renderResolvedDependencies(output, view, assignment.WorkItemID)
		renderResumeContext(output, view.states[assignment.WorkItemID], 6)
		renderWorkBlock(output, view.states[assignment.WorkItemID], 6)
		renderLatestSubagentResult(output, view, assignment.ID, 6)
		output.WriteString("\n    </item>")
	}
	output.WriteString("\n  </assigned_work>")
}

func (view executionContextView) deliveryStatus(workItemID string) string {
	item, exists := view.planItems[workItemID]
	if !exists {
		return "stale_plan"
	}
	if view.acceptedWorkItemSpec[workSpecKey(workItemID, item.SpecID)] {
		return "accepted"
	}
	if state, ok := view.states[workItemID]; ok &&
		state.CurrentSpecID == item.SpecID &&
		state.Status == protocol.WorkItemStatusWaitingInput {
		return string(protocol.WorkItemStatusWaitingInput)
	}
	if submission, ok := view.submissions[workItemID]; ok {
		if acceptance, reviewed := view.acceptances[submission.ID]; reviewed {
			return string(acceptance.Decision)
		}
		return "submitted"
	}
	assignment, assigned := view.currentAssignments[workItemID]
	if !assigned {
		if view.ready[workItemID] {
			return "ready"
		}
		return "blocked"
	}
	if attempt, running := view.runningAttempts[assignment.ID]; running {
		return string(attempt.Status)
	}
	return string(assignment.Status)
}

func renderActiveAssignments(
	output *strings.Builder,
	view executionContextView,
	role ExecutionActorRole,
) {
	output.WriteString("\n  <active_assignments>")
	if role == ExecutionActorCoordinator {
		workItemIDs := make([]string, 0, len(view.currentAssignments))
		for workItemID := range view.currentAssignments {
			workItemIDs = append(workItemIDs, workItemID)
		}
		slices.Sort(workItemIDs)
		for _, workItemID := range workItemIDs {
			assignment := view.currentAssignments[workItemID]
			item := view.workItems[workItemID]
			spec := view.specs[assignment.SpecID]
			pendingSubmission := view.unreviewedSubmissionForSpec(
				workItemID,
				assignment.SpecID,
			)
			fmt.Fprintf(
				output,
				"\n    <item id=\"%s\" logical_key=\"%s\" spec_id=\"%s\" assignment_id=\"%s\" owner_agent_id=\"%s\" kind=\"%s\" status=\"%s\" responsibility_mutation_allowed=\"%t\"",
				xmlValue(workItemID),
				xmlValue(item.LogicalKey),
				xmlValue(assignment.SpecID),
				xmlValue(assignment.ID),
				xmlValue(assignment.OwnerAgentID),
				xmlValue(string(item.Kind)),
				xmlValue(view.deliveryStatus(workItemID)),
				pendingSubmission == nil,
			)
			if pendingSubmission != nil {
				fmt.Fprintf(
					output,
					" pending_submission_id=\"%s\"",
					xmlValue(pendingSubmission.ID),
				)
			}
			if attempt, ok := view.runningAttempts[assignment.ID]; ok {
				fmt.Fprintf(output, " attempt_id=\"%s\"", xmlValue(attempt.ID))
				if strings.TrimSpace(attempt.DispatchID) != "" {
					fmt.Fprintf(output, " dispatch_id=\"%s\"", xmlValue(attempt.DispatchID))
				}
			}
			output.WriteString(">")
			writeXMLTextElement(output, 6, "subject", spec.Subject)
			writeXMLTextElement(output, 6, "objective", spec.Objective)
			writeXMLTextElement(output, 6, "deliverable", spec.Deliverable)
			renderCriteria(output, spec.AcceptanceCriteria)
			renderInputRefs(output, spec.InputRefs)
			renderOutputScopes(output, view.outputScopes(workItemID, assignment.SpecID))
			renderDependencyIDs(output, view.dependencies[workItemID])
			renderResolvedDependencies(output, view, workItemID)
			renderResumeContext(output, view.states[workItemID], 6)
			renderWorkBlock(output, view.states[workItemID], 6)
			renderLatestSubagentResult(output, view, assignment.ID, 6)
			output.WriteString("\n    </item>")
		}
	}
	output.WriteString("\n  </active_assignments>")
}

func renderLatestSubagentResult(
	output *strings.Builder,
	view executionContextView,
	assignmentID string,
	indent int,
) {
	attempt, exists := view.latestChildResults[assignmentID]
	if !exists {
		return
	}
	prefix := strings.Repeat(" ", indent)
	fmt.Fprintf(
		output,
		"\n%s<subagent_result attempt_id=\"%s\" parent_attempt_id=\"%s\" status=\"%s\"",
		prefix,
		xmlValue(attempt.ID),
		xmlValue(attempt.ParentAttemptID),
		xmlValue(string(attempt.Status)),
	)
	if transcriptRef := strings.TrimSpace(stringMetadata(
		attempt.Metadata,
		"agent_transcript_path",
	)); transcriptRef != "" {
		fmt.Fprintf(output, " transcript_ref=\"%s\"", xmlValue(transcriptRef))
	}
	hasLastMessage, _ := attempt.Metadata["has_last_assistant_message"].(bool)
	fmt.Fprintf(output, " has_last_assistant_message=\"%t\">", hasLastMessage)
	writeXMLTextElement(output, indent+2, "failure_reason", attempt.FailureReason)
	fmt.Fprintf(output, "\n%s</subagent_result>", prefix)
}

func renderReadyWork(output *strings.Builder, view executionContextView, role ExecutionActorRole) {
	output.WriteString("\n  <ready_work>")
	if role == ExecutionActorCoordinator {
		ids := make([]string, 0, len(view.ready))
		for workItemID, ready := range view.ready {
			if ready {
				ids = append(ids, workItemID)
			}
		}
		slices.Sort(ids)
		for _, workItemID := range ids {
			item := view.workItems[workItemID]
			spec := view.specs[view.planItems[workItemID].SpecID]
			fmt.Fprintf(
				output,
				"\n    <item id=\"%s\" logical_key=\"%s\" spec_id=\"%s\" kind=\"%s\">",
				xmlValue(workItemID),
				xmlValue(item.LogicalKey),
				xmlValue(spec.ID),
				xmlValue(string(item.Kind)),
			)
			writeXMLTextElement(output, 6, "subject", spec.Subject)
			writeXMLTextElement(output, 6, "objective", spec.Objective)
			writeXMLTextElement(output, 6, "deliverable", spec.Deliverable)
			renderCriteria(output, spec.AcceptanceCriteria)
			renderInputRefs(output, spec.InputRefs)
			renderOutputScopes(output, view.outputScopes(workItemID, spec.ID))
			renderDependencyIDs(output, view.dependencies[workItemID])
			renderResolvedDependencies(output, view, workItemID)
			renderLatestReviewForReadyWork(output, view, workItemID, spec.ID)
			renderResumeContext(output, view.states[workItemID], 6)
			output.WriteString("\n    </item>")
		}
	}
	output.WriteString("\n  </ready_work>")
}

func renderPendingReviews(
	output *strings.Builder,
	view executionContextView,
	role ExecutionActorRole,
	actorAgentID string,
) {
	output.WriteString("\n  <pending_reviews>")
	if role != ExecutionActorSubagent {
		for _, workItemID := range view.pendingReviewWorkItemIDs(actorAgentID) {
			submission := view.submissions[workItemID]
			item := view.workItems[workItemID]
			spec := view.specs[submission.SpecID]
			fmt.Fprintf(
				output,
				"\n    <submission id=\"%s\" work_item_id=\"%s\" logical_key=\"%s\" spec_id=\"%s\" submitter_agent_id=\"%s\">",
				xmlValue(submission.ID),
				xmlValue(workItemID),
				xmlValue(item.LogicalKey),
				xmlValue(submission.SpecID),
				xmlValue(submission.SubmitterAgentID),
			)
			writeXMLTextElement(output, 6, "subject", spec.Subject)
			writeXMLTextElement(output, 6, "objective", spec.Objective)
			writeXMLTextElement(output, 6, "deliverable", spec.Deliverable)
			renderCriteria(output, spec.AcceptanceCriteria)
			renderInputRefs(output, spec.InputRefs)
			renderOutputScopes(output, view.outputScopes(workItemID, submission.SpecID))
			renderDependencyIDs(output, view.dependencies[workItemID])
			renderResolvedDependencies(output, view, workItemID)
			writeXMLTextElement(output, 6, "result_summary", submission.ResultSummary)
			renderStringListAtIndent(output, 6, "result_refs", "ref", submission.ResultRefs)
			renderStringListAtIndent(output, 6, "evidence", "item", submission.Evidence)
			output.WriteString("\n    </submission>")
		}
	}
	output.WriteString("\n  </pending_reviews>")
}

func renderResumableWork(
	output *strings.Builder,
	view executionContextView,
	role ExecutionActorRole,
	actorAgentID string,
) {
	output.WriteString("\n  <resumable_work>")
	for _, workItemID := range view.resumableWorkItemIDs(role, actorAgentID) {
		item := view.workItems[workItemID]
		state := view.states[workItemID]
		assignment := latestAssignmentForCurrentSpec(
			view.snapshot,
			workItemID,
			state.CurrentSpecID,
		)
		fmt.Fprintf(
			output,
			"\n    <item id=\"%s\" logical_key=\"%s\" spec_id=\"%s\"",
			xmlValue(workItemID),
			xmlValue(item.LogicalKey),
			xmlValue(state.CurrentSpecID),
		)
		if assignment != nil {
			fmt.Fprintf(
				output,
				" assignment_id=\"%s\" owner_agent_id=\"%s\" assignment_status=\"%s\"",
				xmlValue(assignment.ID),
				xmlValue(assignment.OwnerAgentID),
				xmlValue(string(assignment.Status)),
			)
		}
		output.WriteString(">")
		renderWorkBlock(output, state, 6)
		output.WriteString("\n    </item>")
	}
	output.WriteString("\n  </resumable_work>")
}

func (view executionContextView) resumableWorkItemIDs(
	role ExecutionActorRole,
	actorAgentID string,
) []string {
	if role == ExecutionActorSubagent {
		return nil
	}
	actorAgentID = strings.TrimSpace(actorAgentID)
	workItemIDs := make([]string, 0)
	for workItemID, item := range view.planItems {
		state, exists := view.states[workItemID]
		if !exists ||
			state.CurrentSpecID != item.SpecID ||
			state.Status != protocol.WorkItemStatusWaitingInput {
			continue
		}
		if role != ExecutionActorCoordinator {
			assignment := latestAssignmentForCurrentSpec(
				view.snapshot,
				workItemID,
				item.SpecID,
			)
			if assignment == nil || assignment.OwnerAgentID != actorAgentID {
				continue
			}
		}
		workItemIDs = append(workItemIDs, workItemID)
	}
	slices.Sort(workItemIDs)
	return workItemIDs
}

func renderPlanRevisionBoundary(
	output *strings.Builder,
	view executionContextView,
	role ExecutionActorRole,
	options ExecutionContextOptions,
) {
	if role != ExecutionActorCoordinator {
		return
	}
	hasCurrentAssignment := len(view.currentAssignments) > 0
	available := options.PlanMode || !view.hasUnreviewedSubmission()
	fmt.Fprintf(
		output,
		"\n  <plan_revision available=\"%t\" guarded=\"%t\">",
		available,
		hasCurrentAssignment,
	)
	if hasCurrentAssignment {
		writeXMLTextElement(
			output,
			4,
			"rule",
			"replacing a different active Plan requires supersede_active_work=true and a non-empty revision_reason; the replacement atomically releases current Assignments and interrupts their live execution chains",
		)
	}
	if !available {
		writeXMLTextElement(
			output,
			4,
			"blocker",
			"review every unreviewed Submission before replacing the active Plan",
		)
	}
	output.WriteString("\n  </plan_revision>")
}

func renderGoalPromotionBoundary(
	output *strings.Builder,
	snapshot *protocol.ExecutionSnapshot,
	options ExecutionContextOptions,
) {
	reasons := slices.Clone(options.GoalPromotionReasons)
	slices.Sort(reasons)
	blockers := slices.Clone(options.GoalPromotionBlockers)
	slices.Sort(blockers)
	fmt.Fprintf(
		output,
		"\n  <goal_promotion eligible=\"%t\">",
		snapshot != nil &&
			isCurrentExecutionStatus(snapshot.Execution.Status) &&
			strings.TrimSpace(snapshot.Execution.GoalID) == "" &&
			len(blockers) == 0 &&
			!options.PlanMode,
	)
	for _, reason := range reasons {
		writeXMLTextElement(output, 4, "activation_reason", string(reason))
	}
	for _, blocker := range blockers {
		writeXMLTextElement(output, 4, "blocker", blocker)
	}
	output.WriteString("\n  </goal_promotion>")
}

func renderExecutionTransitionBoundary(
	output *strings.Builder,
	snapshot *protocol.ExecutionSnapshot,
	role ExecutionActorRole,
	options ExecutionContextOptions,
) {
	current := snapshot != nil && isCurrentExecutionStatus(snapshot.Execution.Status)
	transient := snapshot != nil && strings.TrimSpace(snapshot.Execution.GoalID) == ""
	coordinator := role == ExecutionActorCoordinator
	allowed := current && transient && coordinator
	reasonCode := ""
	switch {
	case !current:
		reasonCode = string(ErrorCodeExecutionTerminal)
	case !transient:
		reasonCode = string(ErrorCodeGoalRetargetRequired)
	case !coordinator:
		reasonCode = string(ErrorCodeWrongOwner)
	}
	fmt.Fprintf(
		output,
		"\n  <execution_transition replace_current_allowed=\"%t\" abandon_allowed=\"%t\" validation_only=\"%t\"",
		allowed,
		allowed,
		options.PlanMode,
	)
	if reasonCode != "" {
		fmt.Fprintf(output, " reason_code=\"%s\"", xmlValue(reasonCode))
	}
	output.WriteString(">")
	writeXMLTextElement(
		output,
		4,
		"rule",
		"same objective uses a replan document; a different transient objective uses a replace document with a complete successor boundary; prepare_plan_execution seals either document and plan_execution commits only its proposal id+digest; stopping without a successor uses abandon_execution",
	)
	output.WriteString("\n  </execution_transition>")
}

func renderSubagentAdmissionBoundary(
	output *strings.Builder,
	snapshot *protocol.ExecutionSnapshot,
	options ExecutionContextOptions,
) bool {
	candidateCount := len(subagentLaunchCandidates(snapshot, options.ActorAgentID))
	var (
		assignment *protocol.WorkAssignment
		parent     *protocol.WorkAttempt
		resolveErr error
	)
	if options.PlanMode {
		resolveErr = planModeError()
	} else {
		assignment, parent, resolveErr = resolveSubagentLaunchCandidate(
			snapshot,
			options.ActorAgentID,
		)
	}
	eligible := !options.PlanMode
	fmt.Fprintf(
		output,
		"\n  <subagent_admission eligible=\"%t\" native_tool=\"Agent\" candidate_assignment_count=\"%d\"",
		eligible,
		candidateCount,
	)
	if resolveErr == nil {
		fmt.Fprintf(
			output,
			" binding_mode=\"managed\" assignment_id=\"%s\" work_item_id=\"%s\" parent_attempt_id=\"%s\" />",
			xmlValue(assignment.ID),
			xmlValue(assignment.WorkItemID),
			xmlValue(parent.ID),
		)
		return true
	}
	reasonCode := ErrorCodeInvalidInput
	reasonMessage := "subagent admission state is unavailable"
	var domainErr *DomainError
	if errors.As(resolveErr, &domainErr) {
		reasonCode = domainErr.Code
		reasonMessage = domainErr.Message
	}
	if eligible {
		fmt.Fprintf(
			output,
			" binding_mode=\"runtime_only\" managed_binding_reason=\"%s\">",
			xmlValue(string(reasonCode)),
		)
		writeXMLTextElement(
			output,
			4,
			"note",
			"native delegation is available, but this run is runtime observation only and does not claim managed Work Item evidence: "+reasonMessage,
		)
	} else {
		fmt.Fprintf(output, " reason_code=\"%s\">", xmlValue(string(reasonCode)))
		writeXMLTextElement(output, 4, "reason", reasonMessage)
	}
	output.WriteString("\n  </subagent_admission>")
	return eligible
}

func renderActionBoundary(
	output *strings.Builder,
	view executionContextView,
	role ExecutionActorRole,
	options ExecutionContextOptions,
	subagentEligible bool,
) {
	allowed := []string{"get_execution"}
	if subagentEligible {
		allowed = append(allowed, "Agent")
	}
	forbidden := make([]string, 0)
	current := isCurrentExecutionStatus(view.snapshot.Execution.Status)
	transientCoordinator := role == ExecutionActorCoordinator &&
		current &&
		strings.TrimSpace(view.snapshot.Execution.GoalID) == ""
	if !current {
		forbidden = append(
			forbidden,
			"prepare_plan_execution",
			"plan_execution",
			"abandon_execution",
			"assign_work",
			"submit_work",
			"review_work",
			"block_work",
			"resume_work",
			"take_over_work",
			"audit_execution_alignment",
			"promote_execution_to_goal",
		)
		renderStringList(output, "allowed_actions", "action", allowed)
		renderStringList(output, "forbidden_actions", "action", forbidden)
		return
	}
	if options.PlanMode {
		if role == ExecutionActorCoordinator {
			allowed = append(allowed, "prepare_plan_execution")
			forbidden = append(forbidden, "plan_execution")
		}
		if transientCoordinator {
			allowed = append(allowed, "abandon_execution")
		} else {
			forbidden = append(forbidden, "abandon_execution")
		}
		forbidden = append(
			forbidden,
			"execute_work_in_plan_mode",
			"assign_work",
			"submit_work",
			"review_work",
			"block_work",
			"resume_work",
			"take_over_work",
			"audit_execution_alignment",
			"promote_execution_to_goal",
			"Agent",
		)
		renderStringList(output, "allowed_actions", "action", allowed)
		renderStringList(output, "forbidden_actions", "action", forbidden)
		return
	}
	availability := view.actionAvailability(options.ActorAgentID)
	setAvailability := func(action string, available bool) {
		if available {
			allowed = append(allowed, action)
			return
		}
		forbidden = append(forbidden, action)
	}
	switch role {
	case ExecutionActorCoordinator:
		allowed = append(allowed, "audit_execution_alignment")
		setAvailability("prepare_plan_execution", availability.canRevisePlan || transientCoordinator)
		setAvailability("plan_execution", availability.canRevisePlan || transientCoordinator)
		setAvailability("abandon_execution", transientCoordinator)
		setAvailability("assign_work", availability.hasReadyWork)
		setAvailability("submit_work", availability.hasOwnedSubmittableAssignment)
		setAvailability("review_work", availability.hasPendingReview)
		setAvailability("block_work", availability.hasBlockableWork)
		setAvailability("resume_work", availability.hasResumableWork)
		setAvailability("take_over_work", availability.hasCurrentAssignment)
		setAvailability(
			"promote_execution_to_goal",
			transientCoordinator && len(options.GoalPromotionBlockers) == 0,
		)
		forbidden = append(
			forbidden,
			"duplicate_assigned_produce_scope",
			"accept_without_evidence",
			"treat_runtime_stop_as_acceptance",
		)
	case ExecutionActorMember:
		allowed = append(allowed, "audit_execution_alignment")
		setAvailability("submit_work", availability.hasOwnedSubmittableAssignment)
		setAvailability("review_work", availability.hasPendingReview)
		setAvailability("block_work", availability.hasOwnedCurrentAssignment)
		setAvailability("resume_work", availability.hasOwnedResumableWork)
		forbidden = append(
			forbidden,
			"prepare_plan_execution",
			"plan_execution",
			"abandon_execution",
			"assign_work",
			"take_over_work",
			"promote_execution_to_goal",
			"mutate_shared_plan",
			"mutate_sibling_work",
			"complete_shared_goal",
		)
	case ExecutionActorSubagent:
		forbidden = append(
			forbidden,
			"prepare_plan_execution",
			"plan_execution",
			"abandon_execution",
			"assign_work",
			"submit_work",
			"review_work",
			"block_work",
			"resume_work",
			"take_over_work",
			"audit_execution_alignment",
			"promote_execution_to_goal",
			"mutate_shared_plan",
			"assign_room_member",
			"accept_submission",
			"complete_goal",
		)
	}
	if !subagentEligible {
		forbidden = append(forbidden, "Agent")
	}
	renderStringList(output, "allowed_actions", "action", allowed)
	renderStringList(output, "forbidden_actions", "action", forbidden)
}

type executionActionAvailability struct {
	canRevisePlan                 bool
	hasReadyWork                  bool
	hasOwnedCurrentAssignment     bool
	hasOwnedSubmittableAssignment bool
	hasPendingReview              bool
	hasBlockableWork              bool
	hasResumableWork              bool
	hasOwnedResumableWork         bool
	hasCurrentAssignment          bool
}

func (view executionContextView) actionAvailability(actorAgentID string) executionActionAvailability {
	actorAgentID = strings.TrimSpace(actorAgentID)
	availability := executionActionAvailability{}
	for _, ready := range view.ready {
		if ready {
			availability.hasReadyWork = true
			break
		}
	}
	for workItemID, assignment := range view.currentAssignments {
		state, exists := view.states[workItemID]
		planItem, inCurrentPlan := view.planItems[workItemID]
		if !exists ||
			!inCurrentPlan ||
			state.CurrentSpecID != assignment.SpecID ||
			planItem.SpecID != assignment.SpecID ||
			view.unreviewedSubmissionForSpec(workItemID, assignment.SpecID) != nil {
			continue
		}
		availability.hasCurrentAssignment = true
		if assignment.OwnerAgentID != actorAgentID {
			continue
		}
		availability.hasOwnedCurrentAssignment = true
		if state.Status != protocol.WorkItemStatusOpen {
			continue
		}
		submission, submitted := view.submissions[workItemID]
		if !submitted {
			availability.hasOwnedSubmittableAssignment = true
			continue
		}
		if _, reviewed := view.acceptances[submission.ID]; reviewed {
			availability.hasOwnedSubmittableAssignment = true
		}
	}
	availability.hasPendingReview = len(view.pendingReviewWorkItemIDs(actorAgentID)) > 0
	availability.canRevisePlan = !view.hasUnreviewedSubmission()
	availability.hasResumableWork =
		len(view.resumableWorkItemIDs(ExecutionActorCoordinator, actorAgentID)) > 0
	availability.hasOwnedResumableWork =
		len(view.resumableWorkItemIDs(ExecutionActorMember, actorAgentID)) > 0
	for workItemID, item := range view.planItems {
		state, exists := view.states[workItemID]
		if !exists ||
			state.CurrentSpecID != item.SpecID ||
			view.unreviewedSubmissionForSpec(workItemID, item.SpecID) != nil ||
			view.acceptedWorkItemSpec[workSpecKey(workItemID, item.SpecID)] {
			continue
		}
		if state.Status == protocol.WorkItemStatusWaitingInput {
			availability.hasBlockableWork = true
			break
		}
		if state.Status != protocol.WorkItemStatusOpen {
			continue
		}
		assignment, hasAssignment := view.currentAssignments[workItemID]
		if view.ready[workItemID] ||
			(hasAssignment &&
				assignment.SpecID == item.SpecID &&
				view.unreviewedSubmissionForSpec(workItemID, item.SpecID) == nil) {
			availability.hasBlockableWork = true
			break
		}
	}
	return availability
}

func (view executionContextView) pendingReviewWorkItemIDs(actorAgentID string) []string {
	workItemIDs := make([]string, 0)
	for workItemID, submission := range view.submissions {
		if _, reviewed := view.acceptances[submission.ID]; reviewed {
			continue
		}
		planItem, inCurrentPlan := view.planItems[workItemID]
		work, hasWork := view.workItems[workItemID]
		spec, hasSpec := view.specs[submission.SpecID]
		assignment, hasAssignment := view.assignments[submission.AssignmentID]
		if view.snapshot.Plan == nil ||
			!inCurrentPlan || !hasWork || !hasSpec || !hasAssignment ||
			submission.ExecutionID != view.snapshot.Execution.ID ||
			submission.PlanID != view.snapshot.Plan.ID ||
			planItem.SpecID != submission.SpecID ||
			work.ExecutionID != view.snapshot.Execution.ID ||
			spec.WorkItemID != workItemID ||
			spec.ExecutionID != view.snapshot.Execution.ID ||
			assignment.ExecutionID != view.snapshot.Execution.ID ||
			assignment.PlanID != view.snapshot.Plan.ID ||
			assignment.WorkItemID != workItemID ||
			assignment.SpecID != submission.SpecID {
			continue
		}
		if view.snapshot.Execution.ScopeKind == protocol.ExecutionScopeRoom &&
			strings.TrimSpace(assignment.ReturnToAgentID) != strings.TrimSpace(actorAgentID) {
			continue
		}
		workItemIDs = append(workItemIDs, workItemID)
	}
	slices.Sort(workItemIDs)
	return workItemIDs
}

func (view executionContextView) hasUnreviewedSubmission() bool {
	for workItemID, item := range view.planItems {
		if view.unreviewedSubmissionForSpec(workItemID, item.SpecID) != nil {
			return true
		}
	}
	return false
}

func (view executionContextView) unreviewedSubmissionForSpec(
	workItemID string,
	specID string,
) *protocol.WorkSubmission {
	for _, submission := range view.submissions {
		if submission.WorkItemID != workItemID ||
			submission.SpecID != specID {
			continue
		}
		if _, reviewed := view.acceptances[submission.ID]; !reviewed {
			item := submission
			return &item
		}
	}
	return nil
}

func renderCompletionBlockers(output *strings.Builder, blockers []string) {
	values := append([]string(nil), blockers...)
	slices.Sort(values)
	renderStringList(output, "completion_blockers", "blocker", values)
}

func renderCriteria(output *strings.Builder, criteria []string) {
	values := append([]string(nil), criteria...)
	writeProjectionContainerStart(output, 6, "acceptance_criteria", len(values))
	for _, criterion := range projectionStrings(values) {
		writeXMLTextElement(output, 8, "criterion", criterion)
	}
	output.WriteString("\n      </acceptance_criteria>")
}

func renderInputRefs(output *strings.Builder, values []string) {
	renderSortedUniqueStringListAtIndent(
		output,
		6,
		"input_refs",
		"ref",
		values,
	)
}

func renderOutputScopes(output *strings.Builder, scopes []protocol.WorkOutputScope) {
	writeProjectionContainerStart(output, 6, "output_scopes", len(scopes))
	limit := min(len(scopes), protocol.ExecutionProjectionCollectionLimit)
	for _, scope := range scopes[:limit] {
		fmt.Fprintf(
			output,
			"\n        <scope mode=\"%s\">%s</scope>",
			xmlValue(string(scope.Mode)),
			xmlValue(scope.Scope),
		)
	}
	output.WriteString("\n      </output_scopes>")
}

func renderDependencyIDs(output *strings.Builder, dependencies []protocol.ExecutionPlanDependency) {
	values := make([]string, 0, len(dependencies))
	for _, dependency := range dependencies {
		values = append(values, dependency.DependsOnWorkItemID)
	}
	slices.Sort(values)
	renderStringListAtIndent(output, 6, "depends_on", "work_item_id", values)
}

func renderResolvedDependencies(
	output *strings.Builder,
	view executionContextView,
	workItemID string,
) {
	dependencies := view.resolvedDependencies(workItemID)
	writeProjectionContainerStart(output, 6, "resolved_dependencies", len(dependencies))
	limit := min(len(dependencies), protocol.ExecutionProjectionCollectionLimit)
	for _, resolved := range dependencies[:limit] {
		fmt.Fprintf(
			output,
			"\n        <dependency kind=\"%s\" status=\"%s\">",
			xmlValue(string(resolved.dependency.Kind)),
			xmlValue(resolved.status),
		)
		fmt.Fprintf(
			output,
			"\n          <upstream work_item_id=\"%s\" logical_key=\"%s\" spec_id=\"%s\" />",
			xmlValue(resolved.workItem.ID),
			xmlValue(resolved.workItem.LogicalKey),
			xmlValue(resolved.planItem.SpecID),
		)
		renderStringListAtIndent(
			output,
			10,
			"blockers",
			"blocker",
			resolved.blockers,
		)
		if resolved.submission != nil && resolved.acceptance != nil {
			fmt.Fprintf(
				output,
				"\n          <accepted_submission id=\"%s\">",
				xmlValue(resolved.submission.ID),
			)
			writeXMLTextElement(output, 12, "result_summary", resolved.submission.ResultSummary)
			renderSortedUniqueStringListAtIndent(
				output,
				12,
				"result_refs",
				"ref",
				resolved.submission.ResultRefs,
			)
			renderSortedUniqueStringListAtIndent(
				output,
				12,
				"evidence",
				"item",
				resolved.submission.Evidence,
			)
			output.WriteString("\n          </accepted_submission>")
			fmt.Fprintf(
				output,
				"\n          <acceptance id=\"%s\">",
				xmlValue(resolved.acceptance.ID),
			)
			renderDependencyCriteriaResults(output, resolved.acceptance.CriteriaResults)
			output.WriteString("\n          </acceptance>")
		}
		output.WriteString("\n        </dependency>")
	}
	output.WriteString("\n      </resolved_dependencies>")
}

func renderDependencyCriteriaResults(
	output *strings.Builder,
	results []protocol.WorkAcceptanceCriterionResult,
) {
	values := append([]protocol.WorkAcceptanceCriterionResult(nil), results...)
	slices.SortFunc(values, func(left, right protocol.WorkAcceptanceCriterionResult) int {
		return strings.Compare(strings.TrimSpace(left.Criterion), strings.TrimSpace(right.Criterion))
	})
	writeProjectionContainerStart(output, 12, "criteria_results", len(values))
	limit := min(len(values), protocol.ExecutionProjectionCollectionLimit)
	for _, result := range values[:limit] {
		fmt.Fprintf(output, "\n              <criterion passed=\"%t\">", result.Passed)
		writeXMLTextElement(output, 16, "requirement", result.Criterion)
		renderSortedUniqueStringListAtIndent(
			output,
			16,
			"evidence",
			"item",
			result.Evidence,
		)
		writeXMLTextElement(output, 16, "note", result.Note)
		output.WriteString("\n              </criterion>")
	}
	output.WriteString("\n            </criteria_results>")
}

func (view executionContextView) outputScopes(
	workItemID string,
	specID string,
) []protocol.WorkOutputScope {
	claims := view.outputClaims[workItemID]
	values := make([]protocol.WorkOutputScope, 0, len(claims))
	seen := make(map[string]struct{}, len(claims))
	for _, claim := range claims {
		if view.snapshot.Plan == nil ||
			claim.ExecutionID != view.snapshot.Execution.ID ||
			claim.PlanID != view.snapshot.Plan.ID ||
			claim.WorkItemID != workItemID ||
			claim.SpecID != specID {
			continue
		}
		normalized, err := protocol.NormalizeWorkOutputScope(protocol.WorkOutputScope{
			Scope: claim.Scope,
			Mode:  claim.Mode,
		})
		if err != nil {
			continue
		}
		key := normalized.Scope + "\x00" + string(normalized.Mode)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		values = append(values, normalized)
	}
	slices.SortFunc(values, func(left, right protocol.WorkOutputScope) int {
		if compared := strings.Compare(left.Scope, right.Scope); compared != 0 {
			return compared
		}
		return strings.Compare(string(left.Mode), string(right.Mode))
	})
	return values
}

func (view executionContextView) resolvedDependencies(
	workItemID string,
) []resolvedDependencyProjection {
	dependencies := append(
		[]protocol.ExecutionPlanDependency(nil),
		view.dependencies[workItemID]...,
	)
	slices.SortFunc(dependencies, func(left, right protocol.ExecutionPlanDependency) int {
		leftWork := view.workItems[left.DependsOnWorkItemID]
		rightWork := view.workItems[right.DependsOnWorkItemID]
		if compared := strings.Compare(leftWork.LogicalKey, rightWork.LogicalKey); compared != 0 {
			return compared
		}
		if compared := strings.Compare(
			left.DependsOnWorkItemID,
			right.DependsOnWorkItemID,
		); compared != 0 {
			return compared
		}
		return strings.Compare(string(left.Kind), string(right.Kind))
	})
	result := make([]resolvedDependencyProjection, 0, len(dependencies))
	for _, dependency := range dependencies {
		work, workExists := view.workItems[dependency.DependsOnWorkItemID]
		planItem, planItemExists := view.planItems[dependency.DependsOnWorkItemID]
		resolved := resolvedDependencyProjection{
			dependency: dependency,
			workItem:   work,
			planItem:   planItem,
		}
		if !workExists || !planItemExists ||
			work.ExecutionID != view.snapshot.Execution.ID ||
			planItem.PlanID != dependency.PlanID {
			resolved.status = "unresolved"
			resolved.blockers = []string{"upstream Work Item is outside the current Plan"}
			result = append(result, resolved)
			continue
		}
		resolved.submission, resolved.acceptance = view.acceptedDelivery(
			work.ID,
			planItem.SpecID,
		)
		if resolved.submission != nil && resolved.acceptance != nil {
			resolved.status = "accepted"
			result = append(result, resolved)
			continue
		}
		resolved.status, resolved.blockers = view.unacceptedDependencyStatus(
			work.ID,
			planItem.SpecID,
		)
		result = append(result, resolved)
	}
	return result
}

func (view executionContextView) acceptedDelivery(
	workItemID string,
	specID string,
) (*protocol.WorkSubmission, *protocol.WorkAcceptance) {
	if view.snapshot.Plan == nil {
		return nil, nil
	}
	var selectedSubmission *protocol.WorkSubmission
	var selectedAcceptance *protocol.WorkAcceptance
	for _, acceptance := range view.snapshot.Acceptances {
		if acceptance.Decision != protocol.WorkAcceptanceAccepted ||
			acceptance.ExecutionID != view.snapshot.Execution.ID ||
			acceptance.PlanID != view.snapshot.Plan.ID ||
			acceptance.WorkItemID != workItemID ||
			acceptance.SpecID != specID {
			continue
		}
		submission, exists := view.submissionsByID[acceptance.SubmissionID]
		if !exists ||
			submission.ExecutionID != view.snapshot.Execution.ID ||
			submission.PlanID != view.snapshot.Plan.ID ||
			submission.WorkItemID != workItemID ||
			submission.SpecID != specID ||
			submission.ID != acceptance.SubmissionID ||
			submission.AssignmentID != acceptance.AssignmentID {
			continue
		}
		if selectedSubmission == nil ||
			submission.Sequence > selectedSubmission.Sequence ||
			(submission.Sequence == selectedSubmission.Sequence &&
				submission.ID > selectedSubmission.ID) {
			submissionCopy := submission
			acceptanceCopy := acceptance
			selectedSubmission = &submissionCopy
			selectedAcceptance = &acceptanceCopy
		}
	}
	return selectedSubmission, selectedAcceptance
}

func (view executionContextView) unacceptedDependencyStatus(
	workItemID string,
	specID string,
) (string, []string) {
	defaultBlocker := "upstream Work Item current spec is not accepted"
	if state, exists := view.states[workItemID]; exists && state.CurrentSpecID == specID {
		if state.Status == protocol.WorkItemStatusWaitingInput {
			blockers := sortedUniqueValues([]string{
				state.BlockReason,
				state.NeededInput,
			})
			if len(blockers) == 0 {
				blockers = []string{defaultBlocker}
			}
			return string(protocol.WorkItemStatusWaitingInput), blockers
		}
		if state.Status != protocol.WorkItemStatusOpen {
			return string(state.Status), []string{defaultBlocker}
		}
	}
	if submission, exists := view.submissions[workItemID]; exists && submission.SpecID == specID {
		if acceptance, reviewed := view.acceptances[submission.ID]; reviewed {
			return string(acceptance.Decision), []string{defaultBlocker}
		}
		return "pending_review", []string{defaultBlocker}
	}
	if assignment, exists := view.currentAssignments[workItemID]; exists &&
		assignment.SpecID == specID {
		if attempt, running := view.runningAttempts[assignment.ID]; running {
			return string(attempt.Status), []string{defaultBlocker}
		}
		return string(assignment.Status), []string{defaultBlocker}
	}
	if view.ready[workItemID] {
		return "ready", []string{defaultBlocker}
	}
	return "not_accepted", []string{defaultBlocker}
}

func sortedUniqueValues(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	slices.Sort(result)
	return result
}

func renderWorkBlock(
	output *strings.Builder,
	state protocol.WorkItemState,
	indent int,
) {
	if state.Status != protocol.WorkItemStatusWaitingInput {
		return
	}
	padding := strings.Repeat(" ", indent)
	fmt.Fprintf(output, "\n%s<waiting_input>", padding)
	writeXMLTextElement(output, indent+2, "reason", state.BlockReason)
	writeXMLTextElement(output, indent+2, "needed_input", state.NeededInput)
	fmt.Fprintf(output, "\n%s</waiting_input>", padding)
}

func renderLatestReviewForReadyWork(
	output *strings.Builder,
	view executionContextView,
	workItemID string,
	specID string,
) {
	submission, exists := view.submissions[workItemID]
	if !exists || submission.SpecID != specID {
		return
	}
	acceptance, reviewed := view.acceptances[submission.ID]
	if !reviewed ||
		(acceptance.Decision != protocol.WorkAcceptanceRejected &&
			acceptance.Decision != protocol.WorkAcceptanceChangesRequested) {
		return
	}
	fmt.Fprintf(
		output,
		"\n      <latest_review submission_id=\"%s\" decision=\"%s\">",
		xmlValue(submission.ID),
		xmlValue(string(acceptance.Decision)),
	)
	writeXMLTextElement(output, 8, "feedback", acceptance.Feedback)
	writeProjectionContainerStart(
		output,
		8,
		"criteria_results",
		len(acceptance.CriteriaResults),
	)
	limit := min(
		len(acceptance.CriteriaResults),
		protocol.ExecutionProjectionCollectionLimit,
	)
	for _, result := range acceptance.CriteriaResults[:limit] {
		fmt.Fprintf(
			output,
			"\n          <criterion passed=\"%t\">",
			result.Passed,
		)
		writeXMLTextElement(output, 12, "requirement", result.Criterion)
		renderStringListAtIndent(output, 12, "evidence", "item", result.Evidence)
		writeXMLTextElement(output, 12, "note", result.Note)
		output.WriteString("\n          </criterion>")
	}
	output.WriteString("\n        </criteria_results>")
	output.WriteString("\n      </latest_review>")
}

func renderResumeContext(
	output *strings.Builder,
	state protocol.WorkItemState,
	indent int,
) {
	resolution := metadataText(state.Metadata, "last_resume_resolution")
	evidence := metadataTextList(state.Metadata, "last_resume_evidence")
	evidenceTotal := metadataTextListTotal(state.Metadata, "last_resume_evidence", len(evidence))
	if resolution == "" && len(evidence) == 0 {
		return
	}
	padding := strings.Repeat(" ", indent)
	fmt.Fprintf(output, "\n%s<resume_context>", padding)
	writeXMLTextElement(output, indent+2, "resolution", resolution)
	renderStringListAtIndentWithTotal(
		output,
		indent+2,
		"evidence",
		"item",
		evidence,
		evidenceTotal,
	)
	fmt.Fprintf(output, "\n%s</resume_context>", padding)
}

func metadataText(metadata map[string]any, key string) string {
	value, exists := metadata[key]
	if !exists {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func metadataTextList(metadata map[string]any, key string) []string {
	value, exists := metadata[key]
	if !exists {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		return normalizeNonEmptyValues(typed)
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if ok && strings.TrimSpace(text) != "" {
				values = append(values, strings.TrimSpace(text))
			}
		}
		return values
	default:
		return nil
	}
}

func metadataTextListTotal(metadata map[string]any, key string, fallback int) int {
	value, exists := metadata[key]
	if !exists {
		return fallback
	}
	switch typed := value.(type) {
	case []string:
		return len(typed)
	case []any:
		return len(typed)
	default:
		return fallback
	}
}

func renderStringList(output *strings.Builder, container string, item string, values []string) {
	renderStringListAtIndent(output, 2, container, item, values)
}

func renderStringListAtIndent(
	output *strings.Builder,
	indent int,
	container string,
	item string,
	values []string,
) {
	renderStringListAtIndentWithTotal(
		output,
		indent,
		container,
		item,
		values,
		len(values),
	)
}

func renderStringListAtIndentWithTotal(
	output *strings.Builder,
	indent int,
	container string,
	item string,
	values []string,
	total int,
) {
	padding := strings.Repeat(" ", indent)
	writeProjectionContainerStart(output, indent, container, total)
	for _, value := range projectionStrings(values) {
		writeXMLTextElement(output, indent+2, item, value)
	}
	fmt.Fprintf(output, "\n%s</%s>", padding, container)
}

func renderSortedUniqueStringListAtIndent(
	output *strings.Builder,
	indent int,
	container string,
	item string,
	values []string,
) {
	padding := strings.Repeat(" ", indent)
	writeProjectionContainerStart(output, indent, container, len(values))
	for _, value := range projectionStrings(sortedUniqueValues(values)) {
		writeXMLTextElement(output, indent+2, item, value)
	}
	fmt.Fprintf(output, "\n%s</%s>", padding, container)
}

func writeProjectionContainerStart(
	output *strings.Builder,
	indent int,
	container string,
	total int,
) {
	padding := strings.Repeat(" ", indent)
	if total > protocol.ExecutionProjectionCollectionLimit {
		fmt.Fprintf(
			output,
			"\n%s<%s truncated=\"true\" total=\"%d\">",
			padding,
			container,
			total,
		)
		return
	}
	fmt.Fprintf(output, "\n%s<%s>", padding, container)
}

func projectionStrings(values []string) []string {
	return values[:min(len(values), protocol.ExecutionProjectionCollectionLimit)]
}

func writeXMLTextElement(output *strings.Builder, indent int, name string, value string) {
	fmt.Fprintf(
		output,
		"\n%s<%s>%s</%s>",
		strings.Repeat(" ", indent),
		name,
		xmlValue(value),
		name,
	)
}

func normalizeExecutionActorRole(
	execution protocol.Execution,
	options ExecutionContextOptions,
) ExecutionActorRole {
	switch options.Role {
	case ExecutionActorCoordinator, ExecutionActorMember, ExecutionActorSubagent:
		return options.Role
	}
	if execution.ScopeKind == protocol.ExecutionScopeDM ||
		strings.TrimSpace(options.ActorAgentID) == strings.TrimSpace(execution.CoordinatorAgentID) {
		return ExecutionActorCoordinator
	}
	return ExecutionActorMember
}

func executionContextLane(
	role ExecutionActorRole,
	options ExecutionContextOptions,
) string {
	switch {
	case options.ReviewBound:
		return "review"
	case options.WorkBound:
		return "work"
	case role == ExecutionActorCoordinator:
		return "coordination"
	case role == ExecutionActorSubagent:
		return "subagent"
	default:
		return "conversation"
	}
}

func workSpecKey(workItemID string, specID string) string {
	return strings.TrimSpace(workItemID) + "\x00" + strings.TrimSpace(specID)
}

func xmlValue(value string) string {
	var output bytes.Buffer
	_ = xml.EscapeText(&output, []byte(strings.TrimSpace(value)))
	return output.String()
}

// INPUT: owner/session 只读查询、当前或最近一次有界 ExecutionSnapshot、完整 WorkGraph child Attempt 历史与 visibility 投影前运行事实。
// OUTPUT: 去除控制面 identity、保留每个 durable Subagent、按 Plan position 排序并派生交付阶段的 protocol.ExecutionView。
// POS: Execution 状态机到 HTTP/DM/Room WorkGraph UI 的唯一展示投影。
package orchestration

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type latestExecutionRepository interface {
	FindLatest(context.Context, string, string) (*protocol.Execution, error)
}

type managedExecutionViewRepository interface {
	FindCurrentManaged(context.Context, string, string) (*protocol.Execution, error)
	FindLatestManaged(context.Context, string, string) (*protocol.Execution, error)
}

type workGraphAttemptRepository interface {
	ListWorkGraphChildAttempts(context.Context, string) ([]protocol.WorkAttempt, error)
}

type workGraphRuntimeRepository interface {
	GetWorkGraphRuntimeGraph(context.Context, string, string, string, string) (protocol.ExecutionRuntimeGraph, error)
}

// GetLatestView 返回 session 当前 managed WorkGraph；没有未终结 Execution 时保留
// 最近一次 terminal 结果。普通 runtime-only round 不属于公共 WorkGraph 读取面，
// 既不会覆盖已有正式图，也不会在从未创建 WorkGraph 时冒充图内容。
func (s *Service) GetLatestView(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
) (*protocol.ExecutionView, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	sessionKey = strings.TrimSpace(sessionKey)
	if ownerUserID == "" || sessionKey == "" {
		return nil, domainError(ErrorCodeInvalidInput, "owner and session_key are required")
	}
	if s.repository == nil {
		return nil, fmt.Errorf("orchestration repository is nil")
	}
	var (
		execution       *protocol.Execution
		err             error
		managedSelector bool
	)
	if managedRepository, ok := s.repository.(managedExecutionViewRepository); ok {
		managedSelector = true
		execution, err = managedRepository.FindCurrentManaged(ctx, ownerUserID, sessionKey)
		if err == nil && execution == nil {
			execution, err = managedRepository.FindLatestManaged(ctx, ownerUserID, sessionKey)
		}
	} else {
		execution, err = s.repository.FindCurrent(ctx, ownerUserID, sessionKey)
	}
	if err != nil {
		return nil, err
	}
	if execution == nil && !managedSelector {
		latestRepository, ok := s.repository.(latestExecutionRepository)
		if ok {
			execution, err = latestRepository.FindLatest(ctx, ownerUserID, sessionKey)
			if err != nil {
				return nil, err
			}
		}
	}
	if execution == nil {
		return nil, nil
	}
	snapshot, err := s.repository.GetSnapshot(ctx, execution.ID)
	if err != nil || snapshot == nil {
		return nil, err
	}
	if snapshot.Execution.OwnerUserID != ownerUserID ||
		snapshot.Execution.SessionKey != sessionKey {
		return nil, domainError(ErrorCodeWrongOwner, "Execution is outside the requested owner/session")
	}
	if snapshot.Plan == nil || len(snapshot.WorkItems) == 0 {
		return nil, nil
	}
	if snapshot.Plan != nil {
		if repository, ok := s.repository.(workGraphAttemptRepository); ok {
			childAttempts, historyErr := repository.ListWorkGraphChildAttempts(ctx, snapshot.Plan.ID)
			if historyErr != nil {
				return nil, historyErr
			}
			snapshot = snapshotWithWorkGraphChildAttempts(snapshot, childAttempts)
		}
	}
	result := ProjectExecutionView(snapshot)
	if result == nil {
		return nil, nil
	}
	if repository, ok := s.repository.(runtimeGraphRepository); ok {
		var runtimeGraph protocol.ExecutionRuntimeGraph
		var graphErr error
		if workGraphRepository, available := s.repository.(workGraphRuntimeRepository); available {
			runtimeGraph, graphErr = workGraphRepository.GetWorkGraphRuntimeGraph(
				ctx,
				ownerUserID,
				sessionKey,
				execution.ID,
				execution.RootRoundID,
			)
		} else {
			runtimeGraph, graphErr = repository.GetRuntimeGraph(
				ctx,
				ownerUserID,
				sessionKey,
				execution.ID,
				execution.RootRoundID,
			)
		}
		if graphErr != nil {
			return nil, graphErr
		}
		runtimeGraph = s.mergeRuntimeGraphSubagentToolHistory(
			ctx,
			ownerUserID,
			sessionKey,
			runtimeGraph,
		)
		mergeExecutionRuntimeGraph(result, runtimeGraph)
	}
	return result, nil
}

func snapshotWithWorkGraphChildAttempts(
	snapshot *protocol.ExecutionSnapshot,
	childAttempts []protocol.WorkAttempt,
) *protocol.ExecutionSnapshot {
	if snapshot == nil || snapshot.Plan == nil || len(childAttempts) == 0 {
		return snapshot
	}
	result := *snapshot
	result.Attempts = slices.Clone(snapshot.Attempts)
	seen := make(map[string]struct{}, len(result.Attempts)+len(childAttempts))
	for _, attempt := range result.Attempts {
		seen[attempt.ID] = struct{}{}
	}
	for _, attempt := range childAttempts {
		if strings.TrimSpace(attempt.ID) == "" ||
			strings.TrimSpace(attempt.ParentAttemptID) == "" ||
			attempt.ExecutionID != snapshot.Execution.ID ||
			attempt.PlanID != snapshot.Plan.ID {
			continue
		}
		if _, duplicate := seen[attempt.ID]; duplicate {
			continue
		}
		seen[attempt.ID] = struct{}{}
		result.Attempts = append(result.Attempts, attempt)
	}
	slices.SortFunc(result.Attempts, func(left, right protocol.WorkAttempt) int {
		if order := strings.Compare(left.WorkItemID, right.WorkItemID); order != 0 {
			return order
		}
		if order := left.CreatedAt.Compare(right.CreatedAt); order != 0 {
			return order
		}
		return strings.Compare(left.ID, right.ID)
	})
	return &result
}

// ProjectExecutionView 把权威 snapshot 投影成稳定且安全的 UI 读取模型。
func ProjectExecutionView(snapshot *protocol.ExecutionSnapshot) *protocol.ExecutionView {
	if snapshot == nil || strings.TrimSpace(snapshot.Execution.ID) == "" {
		return nil
	}
	execution := snapshot.Execution
	result := &protocol.ExecutionView{
		ID:                    execution.ID,
		SessionKey:            execution.SessionKey,
		ScopeKind:             execution.ScopeKind,
		RoomID:                execution.RoomID,
		ConversationID:        execution.ConversationID,
		CoordinatorAgentID:    execution.CoordinatorAgentID,
		Objective:             execution.Objective,
		CompletionCriteria:    slices.Clone(execution.CompletionCriteria),
		GoalID:                execution.GoalID,
		GoalObjectiveRevision: execution.GoalObjectiveRevision,
		Status:                execution.Status,
		Version:               execution.Version,
		CompletionBlockers:    slices.Clone(snapshot.CompletionBlockers),
		CreatedAt:             execution.CreatedAt,
		UpdatedAt:             execution.UpdatedAt,
		CompletedAt:           execution.CompletedAt,
	}
	if snapshot.Plan == nil {
		return result
	}
	result.Plan = &protocol.ExecutionPlanView{
		ID:             snapshot.Plan.ID,
		Revision:       snapshot.Plan.Revision,
		Status:         snapshot.Plan.Status,
		RevisionReason: snapshot.Plan.RevisionReason,
		CreatedAt:      snapshot.Plan.CreatedAt,
		ActivatedAt:    snapshot.Plan.ActivatedAt,
	}

	view := newExecutionContextView(snapshot)
	planItems := slices.Clone(snapshot.PlanItems)
	slices.SortFunc(planItems, func(left, right protocol.ExecutionPlanItem) int {
		if left.Position != right.Position {
			return left.Position - right.Position
		}
		return strings.Compare(left.WorkItemID, right.WorkItemID)
	})
	result.WorkItems = make([]protocol.ExecutionWorkItemView, 0, len(planItems))
	for _, planItem := range planItems {
		workItem, workExists := view.workItems[planItem.WorkItemID]
		spec, specExists := view.specs[planItem.SpecID]
		if !workExists || !specExists {
			continue
		}
		item := projectExecutionWorkItemView(snapshot, view, planItem, workItem, spec)
		result.WorkItems = append(result.WorkItems, item)
		incrementExecutionProgress(&result.Progress, item)
	}
	result.Graph = projectExecutionGraphView(result.WorkItems)
	projectExecutionCoordinatorNode(result)
	return result
}

// projectExecutionCoordinatorNode 把 Room 已存在的 creator/Lead 责任身份
// 投影为稳定主节点。它只表示协调责任与已声明拓扑，不会启动
// round、创建 Assignment 或替 Agent 选择下一步。
func projectExecutionCoordinatorNode(view *protocol.ExecutionView) {
	if view == nil || view.ScopeKind != protocol.ExecutionScopeRoom ||
		strings.TrimSpace(view.CoordinatorAgentID) == "" || len(view.WorkItems) == 0 {
		return
	}
	nodeID := "coordinator:" + view.ID
	view.Graph.Nodes = append([]protocol.ExecutionGraphNodeView{{
		ID:              nodeID,
		Kind:            protocol.ExecutionGraphNodeAgent,
		Visibility:      protocol.ExecutionGraphNodePrimary,
		AgentID:         view.CoordinatorAgentID,
		SubjectID:       view.ID,
		Name:            "coordinate",
		Description:     view.Objective,
		LifecycleStatus: "planned",
		Position:        -1,
	}}, view.Graph.Nodes...)
	for _, item := range view.WorkItems {
		if len(item.DependencyIDs) > 0 {
			continue
		}
		view.Graph.Edges = append(view.Graph.Edges, protocol.ExecutionGraphEdgeView{
			ID:           "coordination:" + nodeID + ":" + item.ID,
			Kind:         protocol.ExecutionGraphEdgeCoordination,
			SourceNodeID: nodeID,
			TargetNodeID: item.ID,
		})
	}
}

// projectExecutionGraphView 只从已经脱敏的 UI Work Item/Attempt 投影运行图。
// Work Item parent 是 containment，不参与 dependency；每个已调用 child Attempt
// 都保留为独立 nested Subagent，不能把同一 parent 下的 siblings 折叠成最后一个。
func projectExecutionGraphView(
	items []protocol.ExecutionWorkItemView,
) protocol.ExecutionGraphView {
	result := protocol.ExecutionGraphView{
		Nodes: make([]protocol.ExecutionGraphNodeView, 0, len(items)),
		Edges: make([]protocol.ExecutionGraphEdgeView, 0),
	}
	rootNodeByAttemptID := make(map[string]string)
	childNodeByAttemptID := make(map[string]string)
	gateNodeByWorkItemID := make(map[string]string)
	for _, item := range items {
		for _, attempt := range item.Attempts {
			if attempt.ParentAttemptID == "" {
				rootNodeByAttemptID[attempt.ID] = item.ID
				continue
			}
			childNodeByAttemptID[attempt.ID] = attempt.ID
		}
	}

	for _, item := range items {
		rootAttempt := latestRootExecutionAttempt(item.Attempts)
		agentID := item.OwnerAgentID
		attemptID := ""
		agentRoundID := ""
		var runStatus protocol.WorkAttemptStatus
		if rootAttempt != nil {
			agentID = firstNonEmpty(rootAttempt.ExecutorAgentID, agentID)
			attemptID = rootAttempt.ID
			agentRoundID = rootAttempt.AgentRoundID
			runStatus = rootAttempt.Status
		}
		result.Nodes = append(result.Nodes, protocol.ExecutionGraphNodeView{
			ID:                   item.ID,
			Kind:                 protocol.ExecutionGraphNodeAgent,
			Visibility:           protocol.ExecutionGraphNodePrimary,
			WorkItemID:           item.ID,
			AttemptID:            attemptID,
			AgentID:              agentID,
			AgentRoundID:         agentRoundID,
			ResponsibilityStatus: item.Status,
			RunStatus:            runStatus,
			Runs:                 rootExecutionAttemptRuns(item.Attempts),
			Position:             item.Position,
		})

		if gate, ok := executionReviewGateNode(item); ok {
			gateNodeByWorkItemID[item.ID] = gate.ID
			result.Nodes = append(result.Nodes, gate)
			result.Edges = append(result.Edges, protocol.ExecutionGraphEdgeView{
				ID:           fmt.Sprintf("review:%s:%s", item.ID, gate.ID),
				Kind:         protocol.ExecutionGraphEdgeReview,
				SourceNodeID: item.ID,
				TargetNodeID: gate.ID,
			})
			if item.Acceptance != nil &&
				item.Acceptance.Decision == protocol.WorkAcceptanceChangesRequested {
				result.Edges = append(result.Edges, protocol.ExecutionGraphEdgeView{
					ID:           fmt.Sprintf("loop:%s:%s", gate.ID, item.ID),
					Kind:         protocol.ExecutionGraphEdgeLoopBack,
					SourceNodeID: gate.ID,
					TargetNodeID: item.ID,
				})
			}
		}

		for attemptPosition, attempt := range item.Attempts {
			if attempt.ParentAttemptID == "" {
				continue
			}
			parentNodeID := rootNodeByAttemptID[attempt.ParentAttemptID]
			if parentNodeID == "" {
				parentNodeID = childNodeByAttemptID[attempt.ParentAttemptID]
			}
			if parentNodeID == "" {
				parentNodeID = item.ID
			}
			result.Nodes = append(result.Nodes, protocol.ExecutionGraphNodeView{
				ID:           attempt.ID,
				Kind:         protocol.ExecutionGraphNodeSubagent,
				Visibility:   protocol.ExecutionGraphNodeNested,
				WorkItemID:   item.ID,
				AttemptID:    attempt.ID,
				ParentNodeID: parentNodeID,
				AgentID:      attempt.ExecutorAgentID,
				AgentRoundID: attempt.AgentRoundID,
				SubjectID:    attempt.TaskID,
				Name:         "subagent",
				RunStatus:    attempt.Status,
				Runs:         []protocol.ExecutionGraphNodeRunView{executionAttemptRunView(attempt)},
				Position:     attemptPosition,
			})
		}
	}

	for _, item := range items {
		for _, dependencyID := range item.DependencyIDs {
			sourceNodeID := dependencyID
			if gateNodeID := gateNodeByWorkItemID[dependencyID]; gateNodeID != "" {
				sourceNodeID = gateNodeID
			}
			result.Edges = append(result.Edges, protocol.ExecutionGraphEdgeView{
				ID:           fmt.Sprintf("dependency:%s:%s", sourceNodeID, item.ID),
				Kind:         protocol.ExecutionGraphEdgeDependency,
				SourceNodeID: sourceNodeID,
				TargetNodeID: item.ID,
			})
		}
		for _, attempt := range item.Attempts {
			if attempt.ParentAttemptID == "" {
				continue
			}
			parentNodeID := rootNodeByAttemptID[attempt.ParentAttemptID]
			if parentNodeID == "" {
				parentNodeID = childNodeByAttemptID[attempt.ParentAttemptID]
			}
			if parentNodeID == "" {
				parentNodeID = item.ID
			}
			result.Edges = append(result.Edges, protocol.ExecutionGraphEdgeView{
				ID:           fmt.Sprintf("spawn:%s:%s", parentNodeID, attempt.ID),
				Kind:         protocol.ExecutionGraphEdgeSpawn,
				SourceNodeID: parentNodeID,
				TargetNodeID: attempt.ID,
			})
		}
	}
	return result
}

func rootExecutionAttemptRuns(
	attempts []protocol.ExecutionAttemptView,
) []protocol.ExecutionGraphNodeRunView {
	result := make([]protocol.ExecutionGraphNodeRunView, 0, len(attempts))
	for _, attempt := range attempts {
		if attempt.ParentAttemptID != "" {
			continue
		}
		result = append(result, executionAttemptRunView(attempt))
	}
	return result
}

func executionAttemptRunView(
	attempt protocol.ExecutionAttemptView,
) protocol.ExecutionGraphNodeRunView {
	startedAt := attempt.StartedAt
	if startedAt == nil {
		createdAt := attempt.CreatedAt.UTC()
		startedAt = &createdAt
	}
	return protocol.ExecutionGraphNodeRunView{
		ID:           attempt.ID,
		AttemptID:    attempt.ID,
		AgentRoundID: attempt.AgentRoundID,
		SubjectID:    firstNonEmpty(attempt.TaskID, attempt.ChildSessionID, attempt.ToolUseID),
		Status:       string(attempt.Status),
		ErrorSummary: attempt.FailureReason,
		StartedAt:    startedAt,
		FinishedAt:   attempt.FinishedAt,
	}
}

// executionReviewGateNode 只根据 durable Assignment return binding、review
// dispatch 或 Acceptance 建立 Gate。协调者角色本身不会凭空生成节点。
func executionReviewGateNode(
	item protocol.ExecutionWorkItemView,
) (protocol.ExecutionGraphNodeView, bool) {
	reviewerID := strings.TrimSpace(item.ReviewAgentID)
	if item.Acceptance != nil && strings.TrimSpace(item.Acceptance.ReviewerID) != "" {
		reviewerID = strings.TrimSpace(item.Acceptance.ReviewerID)
	}
	formalReview := reviewerID != "" && reviewerID != strings.TrimSpace(item.OwnerAgentID)
	if !formalReview {
		return protocol.ExecutionGraphNodeView{}, false
	}
	identity := firstNonEmpty(item.AssignmentID, item.ReviewDispatchID)
	if identity == "" && item.Acceptance != nil {
		identity = item.Acceptance.ID
	}
	if identity == "" {
		return protocol.ExecutionGraphNodeView{}, false
	}
	status := strings.TrimSpace(item.ReviewStatus)
	reviewerKind := protocol.WorkReviewerAgent
	if item.Acceptance != nil {
		status = string(item.Acceptance.Decision)
		reviewerKind = item.Acceptance.ReviewerKind
	}
	if status == "" {
		status = "planned"
	}
	return protocol.ExecutionGraphNodeView{
		ID:               "review:" + identity,
		Kind:             protocol.ExecutionGraphNodeGate,
		Visibility:       protocol.ExecutionGraphNodePrimary,
		WorkItemID:       item.ID,
		AgentID:          reviewerID,
		SubjectID:        identity,
		Name:             "review",
		Description:      item.Subject,
		LifecycleStatus:  status,
		ReviewDispatchID: item.ReviewDispatchID,
		ReviewerKind:     reviewerKind,
		Position:         item.Position,
	}, true
}

func latestRootExecutionAttempt(
	attempts []protocol.ExecutionAttemptView,
) *protocol.ExecutionAttemptView {
	for index := len(attempts) - 1; index >= 0; index-- {
		if attempts[index].ParentAttemptID == "" {
			return &attempts[index]
		}
	}
	return nil
}

func projectExecutionWorkItemView(
	snapshot *protocol.ExecutionSnapshot,
	view executionContextView,
	planItem protocol.ExecutionPlanItem,
	workItem protocol.WorkItem,
	spec protocol.WorkItemSpec,
) protocol.ExecutionWorkItemView {
	state := view.states[workItem.ID]
	dependencyIDs := make([]string, 0, len(view.dependencies[workItem.ID]))
	for _, dependency := range view.dependencies[workItem.ID] {
		dependencyIDs = append(dependencyIDs, dependency.DependsOnWorkItemID)
	}
	slices.Sort(dependencyIDs)

	item := protocol.ExecutionWorkItemView{
		ID:                 workItem.ID,
		LogicalKey:         workItem.LogicalKey,
		Kind:               workItem.Kind,
		Subject:            spec.Subject,
		Objective:          spec.Objective,
		Deliverable:        spec.Deliverable,
		AcceptanceCriteria: slices.Clone(spec.AcceptanceCriteria),
		InputRefs:          slices.Clone(spec.InputRefs),
		OutputScopes:       view.outputScopes(workItem.ID, spec.ID),
		DependencyIDs:      dependencyIDs,
		ParentWorkItemID:   planItem.ParentWorkItemID,
		Required:           planItem.Required,
		Terminal:           planItem.Terminal,
		Position:           planItem.Position,
		BlockReason:        state.BlockReason,
		NeededInput:        state.NeededInput,
		UpdatedAt:          state.UpdatedAt,
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = snapshot.Execution.UpdatedAt
	}
	if assignment := latestAssignmentForCurrentSpec(
		snapshot,
		workItem.ID,
		spec.ID,
	); assignment != nil {
		item.OwnerAgentID = assignment.OwnerAgentID
		item.AssignmentID = assignment.ID
		item.AssignmentStatus = assignment.Status
		item.AssignmentStrategy = assignment.Strategy
		item.ReviewAgentID = assignment.ReturnToAgentID
		if dispatch := latestReviewDispatchForAssignment(
			snapshot,
			assignment.ID,
		); dispatch != nil {
			item.ReviewDispatchID = dispatch.ID
			item.ReviewStatus = string(dispatch.Status)
			item.ReviewAgentID = firstNonEmpty(dispatch.TargetAgentID, item.ReviewAgentID)
		}
	}
	item.Attempts = projectExecutionAttempts(snapshot, workItem.ID, spec.ID)
	if submission, exists := view.submissions[workItem.ID]; exists &&
		submission.SpecID == spec.ID {
		item.Submission = &protocol.ExecutionSubmissionView{
			ID:               submission.ID,
			SubmitterAgentID: submission.SubmitterAgentID,
			ResultSummary:    submission.ResultSummary,
			ResultRefs:       slices.Clone(submission.ResultRefs),
			Evidence:         slices.Clone(submission.Evidence),
			CreatedAt:        submission.CreatedAt,
		}
		if acceptance, reviewed := view.acceptances[submission.ID]; reviewed {
			item.Acceptance = &protocol.ExecutionAcceptanceView{
				ID:              acceptance.ID,
				Decision:        acceptance.Decision,
				ReviewerKind:    acceptance.ReviewerKind,
				ReviewerID:      acceptance.ReviewerID,
				CriteriaResults: slices.Clone(acceptance.CriteriaResults),
				Feedback:        acceptance.Feedback,
				CreatedAt:       acceptance.CreatedAt,
			}
		}
	}
	item.Status = resolveExecutionWorkItemViewStatus(snapshot, view, planItem, item)
	return item
}

func latestReviewDispatchForAssignment(
	snapshot *protocol.ExecutionSnapshot,
	assignmentID string,
) *protocol.ExecutionReviewDispatch {
	if snapshot == nil || strings.TrimSpace(assignmentID) == "" {
		return nil
	}
	for index := len(snapshot.ReviewDispatches) - 1; index >= 0; index-- {
		dispatch := &snapshot.ReviewDispatches[index]
		if dispatch.AssignmentID == assignmentID {
			return dispatch
		}
	}
	return nil
}

func projectExecutionAttempts(
	snapshot *protocol.ExecutionSnapshot,
	workItemID string,
	specID string,
) []protocol.ExecutionAttemptView {
	attempts := make([]protocol.ExecutionAttemptView, 0)
	for _, attempt := range snapshot.Attempts {
		if attempt.WorkItemID != workItemID || attempt.SpecID != specID {
			continue
		}
		attempts = append(attempts, protocol.ExecutionAttemptView{
			ID:              attempt.ID,
			AssignmentID:    attempt.AssignmentID,
			ParentAttemptID: attempt.ParentAttemptID,
			ExecutorKind:    attempt.ExecutorKind,
			ExecutorAgentID: attempt.ExecutorAgentID,
			ParentAgentID:   attempt.ParentAgentID,
			AgentRoundID:    attempt.AgentRoundID,
			ChildSessionID:  attempt.ChildSessionID,
			TaskID:          attempt.SDKTaskID,
			ToolUseID:       attempt.ToolUseID,
			Status:          attempt.Status,
			FailureReason:   attempt.FailureReason,
			CreatedAt:       attempt.CreatedAt,
			StartedAt:       attempt.StartedAt,
			FinishedAt:      attempt.FinishedAt,
		})
	}
	slices.SortFunc(attempts, func(left, right protocol.ExecutionAttemptView) int {
		if order := left.CreatedAt.Compare(right.CreatedAt); order != 0 {
			return order
		}
		return strings.Compare(left.ID, right.ID)
	})
	return attempts
}

func resolveExecutionWorkItemViewStatus(
	snapshot *protocol.ExecutionSnapshot,
	view executionContextView,
	planItem protocol.ExecutionPlanItem,
	item protocol.ExecutionWorkItemView,
) protocol.ExecutionWorkItemViewStatus {
	if item.Acceptance != nil {
		switch item.Acceptance.Decision {
		case protocol.WorkAcceptanceAccepted:
			return protocol.ExecutionWorkItemViewAccepted
		case protocol.WorkAcceptanceRejected, protocol.WorkAcceptanceChangesRequested:
			return protocol.ExecutionWorkItemViewChangesRequested
		}
	}
	state := view.states[planItem.WorkItemID]
	if state.Status == protocol.WorkItemStatusCancelled ||
		state.Status == protocol.WorkItemStatusSuperseded {
		return protocol.ExecutionWorkItemViewCancelled
	}
	if item.Submission != nil {
		return protocol.ExecutionWorkItemViewSubmitted
	}
	if state.Status == protocol.WorkItemStatusWaitingInput {
		return protocol.ExecutionWorkItemViewBlocked
	}
	for index := len(item.Attempts) - 1; index >= 0; index-- {
		attempt := item.Attempts[index]
		if attempt.ParentAttemptID != "" {
			continue
		}
		switch attempt.Status {
		case protocol.WorkAttemptStatusRunning:
			return protocol.ExecutionWorkItemViewRunning
		case protocol.WorkAttemptStatusFailed,
			protocol.WorkAttemptStatusInterrupted,
			protocol.WorkAttemptStatusTimedOut:
			return protocol.ExecutionWorkItemViewFailed
		}
		break
	}
	if _, assigned := view.currentAssignments[planItem.WorkItemID]; assigned {
		return protocol.ExecutionWorkItemViewAssigned
	}
	if view.ready[planItem.WorkItemID] {
		return protocol.ExecutionWorkItemViewReady
	}
	if snapshot.Execution.Status == protocol.ExecutionStatusCancelled ||
		snapshot.Execution.Status == protocol.ExecutionStatusSuperseded ||
		snapshot.Execution.Status == protocol.ExecutionStatusFailed {
		return protocol.ExecutionWorkItemViewCancelled
	}
	return protocol.ExecutionWorkItemViewWaiting
}

func incrementExecutionProgress(
	progress *protocol.ExecutionProgressView,
	item protocol.ExecutionWorkItemView,
) {
	progress.Total++
	if item.Required {
		progress.Required++
	}
	switch item.Status {
	case protocol.ExecutionWorkItemViewAccepted:
		progress.Accepted++
	case protocol.ExecutionWorkItemViewRunning, protocol.ExecutionWorkItemViewAssigned:
		progress.Running++
	case protocol.ExecutionWorkItemViewBlocked:
		progress.Blocked++
	case protocol.ExecutionWorkItemViewSubmitted:
		progress.Submitted++
	case protocol.ExecutionWorkItemViewReady:
		progress.Ready++
	case protocol.ExecutionWorkItemViewChangesRequested:
		progress.ChangesRequested++
	case protocol.ExecutionWorkItemViewFailed:
		progress.Failed++
	case protocol.ExecutionWorkItemViewCancelled:
		progress.Cancelled++
	default:
		progress.Waiting++
	}
}

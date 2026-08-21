// INPUT: owner/session/exact Execution 只读查询、当前/最近/历史有界 ExecutionSnapshot、完整 WorkGraph Assignment/Attempt/Submission/Review/Acceptance 历史与 visibility 投影前运行事实。
// OUTPUT: 去除控制面 identity、保留每个 root/child Attempt 与 immutable Submission Gate、按 Plan position 排序并派生交付阶段的一个或多个 protocol.ExecutionView。
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

type managedExecutionHistoryRepository interface {
	ListManaged(context.Context, string, string, int) ([]protocol.Execution, error)
}

type workGraphAttemptRepository interface {
	ListWorkGraphChildAttempts(context.Context, string) ([]protocol.WorkAttempt, error)
}

type workGraphHistoryRepository interface {
	ListWorkGraphHistory(context.Context, string) (protocol.ExecutionWorkGraphHistory, error)
}

type workGraphStateRepository interface {
	GetWorkGraphState(context.Context, string) (*protocol.ExecutionSnapshot, protocol.ExecutionWorkGraphHistory, error)
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
	return s.getViewForExecution(ctx, ownerUserID, sessionKey, execution)
}

// GetView 按 owner/session/execution 精确读取一个历史 managed WorkGraph。
func (s *Service) GetView(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
	executionID string,
) (*protocol.ExecutionView, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	sessionKey = strings.TrimSpace(sessionKey)
	executionID = strings.TrimSpace(executionID)
	if ownerUserID == "" || sessionKey == "" || executionID == "" {
		return nil, domainError(ErrorCodeInvalidInput, "owner, session_key and execution_id are required")
	}
	if s == nil || s.repository == nil {
		return nil, fmt.Errorf("orchestration repository is nil")
	}
	execution, err := s.repository.Get(ctx, executionID)
	if err != nil || execution == nil {
		return nil, err
	}
	if execution.OwnerUserID != ownerUserID || execution.SessionKey != sessionKey {
		return nil, domainError(ErrorCodeWrongOwner, "Execution is outside the requested owner/session")
	}
	return s.getViewForExecution(ctx, ownerUserID, sessionKey, execution)
}

// ListHistoryViews 返回当前 session 最近的 managed WorkGraph 历史。
func (s *Service) ListHistoryViews(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
	limit int,
) ([]protocol.ExecutionView, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	sessionKey = strings.TrimSpace(sessionKey)
	if ownerUserID == "" || sessionKey == "" {
		return nil, domainError(ErrorCodeInvalidInput, "owner and session_key are required")
	}
	repository, ok := s.repository.(managedExecutionHistoryRepository)
	if !ok {
		return []protocol.ExecutionView{}, nil
	}
	executions, err := repository.ListManaged(ctx, ownerUserID, sessionKey, limit)
	if err != nil {
		return nil, err
	}
	views := make([]protocol.ExecutionView, 0, len(executions))
	for index := range executions {
		view, viewErr := s.getViewForExecution(
			ctx,
			ownerUserID,
			sessionKey,
			&executions[index],
		)
		if viewErr != nil {
			return nil, viewErr
		}
		if view != nil {
			views = append(views, *view)
		}
	}
	return views, nil
}

func (s *Service) getViewForExecution(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
	execution *protocol.Execution,
) (*protocol.ExecutionView, error) {
	var workGraphHistory *protocol.ExecutionWorkGraphHistory
	var snapshot *protocol.ExecutionSnapshot
	var err error
	if repository, ok := s.repository.(workGraphStateRepository); ok {
		stateSnapshot, history, stateErr := repository.GetWorkGraphState(ctx, execution.ID)
		snapshot, err = stateSnapshot, stateErr
		if stateErr == nil && stateSnapshot != nil && stateSnapshot.Plan != nil {
			workGraphHistory = &history
		}
	} else {
		snapshot, err = s.repository.GetSnapshot(ctx, execution.ID)
	}
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
	if snapshot.Plan != nil && workGraphHistory == nil {
		if repository, ok := s.repository.(workGraphHistoryRepository); ok {
			history, historyErr := repository.ListWorkGraphHistory(ctx, snapshot.Plan.ID)
			if historyErr != nil {
				return nil, historyErr
			}
			workGraphHistory = &history
		} else if repository, ok := s.repository.(workGraphAttemptRepository); ok {
			childAttempts, historyErr := repository.ListWorkGraphChildAttempts(ctx, snapshot.Plan.ID)
			if historyErr != nil {
				return nil, historyErr
			}
			snapshot = snapshotWithWorkGraphChildAttempts(snapshot, childAttempts)
		}
	}
	result := projectExecutionView(snapshot, workGraphHistory)
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
	return projectExecutionView(snapshot, nil)
}

func projectExecutionView(
	snapshot *protocol.ExecutionSnapshot,
	workGraphHistory *protocol.ExecutionWorkGraphHistory,
) *protocol.ExecutionView {
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
		RootRoundID:           execution.RootRoundID,
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
	if workGraphHistory == nil {
		history := workGraphHistoryFromSnapshot(snapshot)
		workGraphHistory = &history
	}
	result.Graph = projectExecutionGraphViewWithHistory(result.WorkItems, *workGraphHistory)
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
	entryNodeByWorkItemID := make(map[string]string, len(view.WorkItems))
	for _, node := range view.Graph.Nodes {
		if node.Kind == protocol.ExecutionGraphNodeAgent &&
			strings.TrimSpace(node.WorkItemID) != "" &&
			entryNodeByWorkItemID[node.WorkItemID] == "" {
			entryNodeByWorkItemID[node.WorkItemID] = node.ID
		}
	}
	for _, item := range view.WorkItems {
		if len(item.DependencyIDs) > 0 {
			continue
		}
		targetNodeID := firstNonEmpty(entryNodeByWorkItemID[item.ID], item.ID)
		view.Graph.Edges = append(view.Graph.Edges, protocol.ExecutionGraphEdgeView{
			ID:           "coordination:" + nodeID + ":" + targetNodeID,
			Kind:         protocol.ExecutionGraphEdgeCoordination,
			SourceNodeID: nodeID,
			TargetNodeID: targetNodeID,
		})
	}
}

// projectExecutionGraphView 是只给无 Repository 单测使用的窄入口。
func projectExecutionGraphView(
	items []protocol.ExecutionWorkItemView,
) protocol.ExecutionGraphView {
	return projectExecutionGraphViewWithHistory(items, protocol.ExecutionWorkGraphHistory{})
}

func workGraphHistoryFromSnapshot(
	snapshot *protocol.ExecutionSnapshot,
) protocol.ExecutionWorkGraphHistory {
	if snapshot == nil {
		return protocol.ExecutionWorkGraphHistory{}
	}
	return protocol.ExecutionWorkGraphHistory{
		Assignments:      slices.Clone(snapshot.Assignments),
		Attempts:         slices.Clone(snapshot.Attempts),
		Submissions:      slices.Clone(snapshot.Submissions),
		ReviewDispatches: slices.Clone(snapshot.ReviewDispatches),
		Acceptances:      slices.Clone(snapshot.Acceptances),
	}
}

// projectExecutionGraphViewWithHistory 从画布专用 append-only 历史投影每个
// root Attempt 与 immutable Submission Gate；Acceptance 只更新对应 Gate 的
// 验收结论。Work Item 只在尚未产生 Attempt 时作为 planned placeholder；已发生的
// 执行轮次绝不再压进同一 Agent 节点。
func projectExecutionGraphViewWithHistory(
	items []protocol.ExecutionWorkItemView,
	history protocol.ExecutionWorkGraphHistory,
) protocol.ExecutionGraphView {
	result := protocol.ExecutionGraphView{
		Nodes: make([]protocol.ExecutionGraphNodeView, 0, len(items)),
		Edges: make([]protocol.ExecutionGraphEdgeView, 0),
	}
	assignmentByID := make(map[string]protocol.WorkAssignment, len(history.Assignments))
	for _, assignment := range history.Assignments {
		assignmentByID[assignment.ID] = assignment
	}
	acceptanceBySubmissionID := make(map[string]protocol.WorkAcceptance, len(history.Acceptances))
	for _, acceptance := range history.Acceptances {
		acceptanceBySubmissionID[acceptance.SubmissionID] = acceptance
	}
	dispatchBySubmissionID := make(map[string]protocol.ExecutionReviewDispatch, len(history.ReviewDispatches))
	for _, dispatch := range history.ReviewDispatches {
		dispatchBySubmissionID[dispatch.SubmissionID] = dispatch
	}
	submissionByAttemptID := make(map[string]protocol.WorkSubmission, len(history.Submissions))
	for _, submission := range history.Submissions {
		submissionByAttemptID[submission.AttemptID] = submission
	}
	attemptsByWorkItemID := make(map[string][]protocol.ExecutionAttemptView)
	for _, attempt := range history.Attempts {
		attemptsByWorkItemID[attempt.WorkItemID] = append(
			attemptsByWorkItemID[attempt.WorkItemID],
			executionAttemptViewFromAttempt(attempt),
		)
	}
	entryNodeByWorkItemID := make(map[string]string, len(items))
	exitNodeByWorkItemID := make(map[string]string, len(items))
	gateAssignmentIDs := make(map[string]struct{}, len(history.Submissions))
	gateSubmissionIDs := make(map[string]struct{}, len(history.Submissions))
	gateAcceptanceIDs := make(map[string]struct{}, len(history.Acceptances))
	rootNodeByAttemptID := make(map[string]string)
	childNodeByAttemptID := make(map[string]string)
	for _, item := range items {
		attempts := attemptsByWorkItemID[item.ID]
		if len(attempts) == 0 {
			attempts = slices.Clone(item.Attempts)
		}
		slices.SortFunc(attempts, func(left, right protocol.ExecutionAttemptView) int {
			if order := left.CreatedAt.Compare(right.CreatedAt); order != 0 {
				return order
			}
			return strings.Compare(left.ID, right.ID)
		})
		attemptsByWorkItemID[item.ID] = attempts
		roots := make([]protocol.ExecutionAttemptView, 0, len(attempts))
		for _, attempt := range attempts {
			if attempt.ParentAttemptID == "" {
				roots = append(roots, attempt)
				continue
			}
			childNodeByAttemptID[attempt.ID] = attempt.ID
		}
		if len(roots) == 0 {
			result.Nodes = append(result.Nodes, protocol.ExecutionGraphNodeView{
				ID:                   item.ID,
				Kind:                 protocol.ExecutionGraphNodeAgent,
				Visibility:           protocol.ExecutionGraphNodePrimary,
				WorkItemID:           item.ID,
				AgentID:              item.OwnerAgentID,
				ResponsibilityStatus: item.Status,
				Position:             item.Position,
			})
			entryNodeByWorkItemID[item.ID] = item.ID
			exitNodeByWorkItemID[item.ID] = item.ID
		} else {
			for index, attempt := range roots {
				nodeID := attempt.ID
				if index == len(roots)-1 {
					nodeID = item.ID
				}
				rootNodeByAttemptID[attempt.ID] = nodeID
				assignment := assignmentByID[attempt.AssignmentID]
				agentID := firstNonEmpty(attempt.ExecutorAgentID, assignment.OwnerAgentID, item.OwnerAgentID)
				node := protocol.ExecutionGraphNodeView{
					ID:           nodeID,
					Kind:         protocol.ExecutionGraphNodeAgent,
					Visibility:   protocol.ExecutionGraphNodePrimary,
					WorkItemID:   item.ID,
					AttemptID:    attempt.ID,
					AgentID:      agentID,
					AgentRoundID: attempt.AgentRoundID,
					RunStatus:    attempt.Status,
					Runs:         []protocol.ExecutionGraphNodeRunView{executionAttemptRunView(attempt)},
					Position:     item.Position,
				}
				if index == len(roots)-1 {
					node.ResponsibilityStatus = item.Status
				}
				result.Nodes = append(result.Nodes, node)
				if index == 0 {
					entryNodeByWorkItemID[item.ID] = nodeID
				}
				if index > 0 {
					previous := roots[index-1]
					sourceID := exitNodeByWorkItemID[item.ID]
					kind := protocol.ExecutionGraphEdgeRetry
					if submission, ok := submissionByAttemptID[previous.ID]; ok {
						if acceptance, reviewed := acceptanceBySubmissionID[submission.ID]; reviewed &&
							acceptance.Decision != protocol.WorkAcceptanceAccepted {
							kind = protocol.ExecutionGraphEdgeLoopBack
						}
					}
					result.Edges = append(result.Edges, protocol.ExecutionGraphEdgeView{
						ID:           fmt.Sprintf("%s:%s:%s", kind, sourceID, nodeID),
						Kind:         kind,
						SourceNodeID: sourceID,
						TargetNodeID: nodeID,
					})
				}
				exitNodeByWorkItemID[item.ID] = nodeID
				if submission, ok := submissionByAttemptID[attempt.ID]; ok {
					if submission.AssignmentID != "" {
						gateAssignmentIDs[submission.AssignmentID] = struct{}{}
					}
					gateSubmissionIDs[submission.ID] = struct{}{}
					acceptance := acceptanceBySubmissionID[submission.ID]
					if acceptance.ID != "" {
						gateAcceptanceIDs[acceptance.ID] = struct{}{}
					}
					gate := executionHistoryReviewGateNode(
						item,
						attempt,
						submission,
						assignment,
						dispatchBySubmissionID[submission.ID],
						acceptance,
					)
					result.Nodes = append(result.Nodes, gate)
					result.Edges = append(result.Edges, protocol.ExecutionGraphEdgeView{
						ID:           fmt.Sprintf("review:%s:%s", nodeID, gate.ID),
						Kind:         protocol.ExecutionGraphEdgeReview,
						SourceNodeID: nodeID,
						TargetNodeID: gate.ID,
					})
					exitNodeByWorkItemID[item.ID] = gate.ID
					if index == len(roots)-1 {
						if acceptance, reviewed := acceptanceBySubmissionID[submission.ID]; reviewed &&
							acceptance.Decision != protocol.WorkAcceptanceAccepted {
							result.Edges = append(result.Edges, protocol.ExecutionGraphEdgeView{
								ID:           fmt.Sprintf("loop:%s:%s", gate.ID, nodeID),
								Kind:         protocol.ExecutionGraphEdgeLoopBack,
								SourceNodeID: gate.ID,
								TargetNodeID: nodeID,
							})
						}
					}
				}
			}
		}

		if !executionReviewGateAlreadyProjected(
			item,
			gateAssignmentIDs,
			gateSubmissionIDs,
			gateAcceptanceIDs,
		) {
			if gate, ok := executionReviewGateNode(item); ok {
				result.Nodes = append(result.Nodes, gate)
				result.Edges = append(result.Edges, protocol.ExecutionGraphEdgeView{
					ID:           fmt.Sprintf("review:%s:%s", exitNodeByWorkItemID[item.ID], gate.ID),
					Kind:         protocol.ExecutionGraphEdgeReview,
					SourceNodeID: exitNodeByWorkItemID[item.ID],
					TargetNodeID: gate.ID,
				})
				exitNodeByWorkItemID[item.ID] = gate.ID
				if item.Acceptance != nil && item.Acceptance.Decision != protocol.WorkAcceptanceAccepted {
					result.Edges = append(result.Edges, protocol.ExecutionGraphEdgeView{
						ID:           fmt.Sprintf("loop:%s:%s", gate.ID, entryNodeByWorkItemID[item.ID]),
						Kind:         protocol.ExecutionGraphEdgeLoopBack,
						SourceNodeID: gate.ID,
						TargetNodeID: entryNodeByWorkItemID[item.ID],
					})
				}
			}
		}

		for attemptPosition, attempt := range attempts {
			if attempt.ParentAttemptID == "" {
				continue
			}
			parentNodeID := rootNodeByAttemptID[attempt.ParentAttemptID]
			if parentNodeID == "" {
				parentNodeID = childNodeByAttemptID[attempt.ParentAttemptID]
			}
			if parentNodeID == "" {
				parentNodeID = exitNodeByWorkItemID[item.ID]
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
			sourceNodeID := firstNonEmpty(exitNodeByWorkItemID[dependencyID], dependencyID)
			targetNodeID := firstNonEmpty(entryNodeByWorkItemID[item.ID], item.ID)
			result.Edges = append(result.Edges, protocol.ExecutionGraphEdgeView{
				ID:           fmt.Sprintf("dependency:%s:%s", sourceNodeID, targetNodeID),
				Kind:         protocol.ExecutionGraphEdgeDependency,
				SourceNodeID: sourceNodeID,
				TargetNodeID: targetNodeID,
			})
		}
		for _, attempt := range attemptsByWorkItemID[item.ID] {
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

// executionReviewGateAlreadyProjected 只用 immutable Submission/Acceptance
// identity 判断当前快照是否已由 append-only 历史 Gate 覆盖。Assignment 是运行中
// lease，完成后允许从 current Work Item 视图清空，不能单独承担 Gate 去重。
func executionReviewGateAlreadyProjected(
	item protocol.ExecutionWorkItemView,
	assignmentIDs map[string]struct{},
	submissionIDs map[string]struct{},
	acceptanceIDs map[string]struct{},
) bool {
	if item.Submission != nil && item.Submission.ID != "" {
		if _, ok := submissionIDs[item.Submission.ID]; ok {
			return true
		}
	}
	if item.Acceptance != nil && item.Acceptance.ID != "" {
		if _, ok := acceptanceIDs[item.Acceptance.ID]; ok {
			return true
		}
	}
	if item.AssignmentID != "" {
		_, ok := assignmentIDs[item.AssignmentID]
		return ok
	}
	return false
}

func executionAttemptViewFromAttempt(attempt protocol.WorkAttempt) protocol.ExecutionAttemptView {
	return protocol.ExecutionAttemptView{
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
	}
}

func executionHistoryReviewGateNode(
	item protocol.ExecutionWorkItemView,
	attempt protocol.ExecutionAttemptView,
	submission protocol.WorkSubmission,
	assignment protocol.WorkAssignment,
	dispatch protocol.ExecutionReviewDispatch,
	acceptance protocol.WorkAcceptance,
) protocol.ExecutionGraphNodeView {
	reviewerID := firstNonEmpty(acceptance.ReviewerID, dispatch.TargetAgentID, assignment.ReturnToAgentID)
	status := "submitted"
	resultSummary := strings.TrimSpace(submission.ResultSummary)
	reviewerKind := protocol.WorkReviewerAgent
	if strings.TrimSpace(dispatch.ID) != "" {
		status = string(dispatch.Status)
	}
	if strings.TrimSpace(acceptance.ID) != "" {
		status = string(acceptance.Decision)
		reviewerKind = acceptance.ReviewerKind
		resultSummary = firstNonEmpty(acceptance.Feedback, resultSummary)
	}
	return protocol.ExecutionGraphNodeView{
		ID:               "review:" + submission.ID,
		Kind:             protocol.ExecutionGraphNodeGate,
		Visibility:       protocol.ExecutionGraphNodePrimary,
		WorkItemID:       item.ID,
		AttemptID:        attempt.ID,
		AgentID:          reviewerID,
		AgentRoundID:     acceptance.DecisionRoundID,
		SubjectID:        submission.ID,
		Name:             "review",
		Description:      item.Subject,
		LifecycleStatus:  status,
		ReviewDispatchID: dispatch.ID,
		ReviewerKind:     reviewerKind,
		ResultSummary:    resultSummary,
		Position:         item.Position,
	}
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
			AssignmentID:     submission.AssignmentID,
			AttemptID:        submission.AttemptID,
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

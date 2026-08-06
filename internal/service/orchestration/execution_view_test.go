package orchestration

import (
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestProjectExecutionViewPreservesResponsibilityAndAcceptanceFlow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC)
	started := now.Add(time.Minute)
	snapshot := &protocol.ExecutionSnapshot{
		Execution: protocol.Execution{
			ID:                 "execution-1",
			OwnerUserID:        "owner-1",
			SessionKey:         "session-1",
			ScopeKind:          protocol.ExecutionScopeRoom,
			CoordinatorAgentID: "lead",
			Objective:          "Ship the WorkGraph UI",
			CompletionCriteria: []string{"All required work is accepted"},
			Status:             protocol.ExecutionStatusActive,
			Version:            7,
			CreatedAt:          now,
			UpdatedAt:          now.Add(2 * time.Minute),
		},
		Plan: &protocol.ExecutionPlanRevision{
			ID:        "plan-1",
			Revision:  2,
			Status:    protocol.PlanRevisionStatusActive,
			CreatedAt: now,
		},
		WorkItems: []protocol.WorkItem{
			{ID: "work-a", ExecutionID: "execution-1", LogicalKey: "research", Kind: protocol.WorkItemKindProduce},
			{ID: "work-b", ExecutionID: "execution-1", LogicalKey: "build", Kind: protocol.WorkItemKindProduce},
			{ID: "work-c", ExecutionID: "execution-1", LogicalKey: "integrate", Kind: protocol.WorkItemKindIntegrate},
		},
		WorkItemStates: []protocol.WorkItemState{
			{WorkItemID: "work-a", ExecutionID: "execution-1", CurrentSpecID: "spec-a", Status: protocol.WorkItemStatusOpen, UpdatedAt: now},
			{WorkItemID: "work-b", ExecutionID: "execution-1", CurrentSpecID: "spec-b", Status: protocol.WorkItemStatusOpen, UpdatedAt: now},
			{WorkItemID: "work-c", ExecutionID: "execution-1", CurrentSpecID: "spec-c", Status: protocol.WorkItemStatusOpen, UpdatedAt: now},
		},
		WorkItemSpecs: []protocol.WorkItemSpec{
			{ID: "spec-a", WorkItemID: "work-a", ExecutionID: "execution-1", Subject: "Research", Objective: "Collect facts", Deliverable: "Evidence list", AcceptanceCriteria: []string{"Sources included"}},
			{ID: "spec-b", WorkItemID: "work-b", ExecutionID: "execution-1", Subject: "Build", Objective: "Implement UI", Deliverable: "Working panel", AcceptanceCriteria: []string{"Typecheck passes"}},
			{ID: "spec-c", WorkItemID: "work-c", ExecutionID: "execution-1", Subject: "Integrate", Objective: "Close the flow", Deliverable: "Accepted release", AcceptanceCriteria: []string{"All dependencies accepted"}},
		},
		PlanItems: []protocol.ExecutionPlanItem{
			{PlanID: "plan-1", ExecutionID: "execution-1", WorkItemID: "work-a", SpecID: "spec-a", Required: true, Position: 0},
			{PlanID: "plan-1", ExecutionID: "execution-1", WorkItemID: "work-b", SpecID: "spec-b", Required: true, Position: 1},
			{PlanID: "plan-1", ExecutionID: "execution-1", WorkItemID: "work-c", SpecID: "spec-c", Required: true, Terminal: true, Position: 2},
		},
		Dependencies: []protocol.ExecutionPlanDependency{
			{PlanID: "plan-1", ExecutionID: "execution-1", WorkItemID: "work-b", DependsOnWorkItemID: "work-a", Kind: protocol.WorkDependencyHard},
			{PlanID: "plan-1", ExecutionID: "execution-1", WorkItemID: "work-c", DependsOnWorkItemID: "work-b", Kind: protocol.WorkDependencyHard},
		},
		OutputClaims: []protocol.ExecutionPlanOutputClaim{
			{PlanID: "plan-1", ExecutionID: "execution-1", WorkItemID: "work-b", SpecID: "spec-b", Scope: "dir:web/src/features/execution", Mode: protocol.WorkOutputScopeExclusive},
		},
		Assignments: []protocol.WorkAssignment{
			{ID: "assignment-a", ExecutionID: "execution-1", PlanID: "plan-1", WorkItemID: "work-a", SpecID: "spec-a", OwnerAgentID: "researcher", ReturnToAgentID: "lead", Status: protocol.WorkAssignmentStatusCompleted},
			{ID: "assignment-b", ExecutionID: "execution-1", PlanID: "plan-1", WorkItemID: "work-b", SpecID: "spec-b", OwnerAgentID: "builder", ReturnToAgentID: "lead", Status: protocol.WorkAssignmentStatusActive, Strategy: protocol.AssignmentStrategyRoomMember},
		},
		Attempts: []protocol.WorkAttempt{
			{ID: "attempt-root", AssignmentID: "assignment-b", ExecutionID: "execution-1", PlanID: "plan-1", WorkItemID: "work-b", SpecID: "spec-b", ExecutorKind: protocol.AttemptExecutorAgent, ExecutorAgentID: "builder", AgentRoundID: "agent-round-builder-1", Status: protocol.WorkAttemptStatusRunning, CreatedAt: now, StartedAt: &started},
			{ID: "attempt-child", AssignmentID: "assignment-b", ExecutionID: "execution-1", PlanID: "plan-1", WorkItemID: "work-b", SpecID: "spec-b", ParentAttemptID: "attempt-root", ExecutorKind: protocol.AttemptExecutorSubagent, ParentAgentID: "builder", Status: protocol.WorkAttemptStatusRunning, CreatedAt: started},
		},
		Submissions: []protocol.WorkSubmission{
			{ID: "submission-a", ExecutionID: "execution-1", PlanID: "plan-1", WorkItemID: "work-a", SpecID: "spec-a", AssignmentID: "assignment-a", Sequence: 1, SubmitterAgentID: "researcher", ResultSummary: "Evidence collected", Evidence: []string{"report.md"}, CreatedAt: now},
		},
		Acceptances: []protocol.WorkAcceptance{
			{ID: "acceptance-a", ExecutionID: "execution-1", PlanID: "plan-1", WorkItemID: "work-a", SpecID: "spec-a", AssignmentID: "assignment-a", SubmissionID: "submission-a", Decision: protocol.WorkAcceptanceAccepted, ReviewerKind: protocol.WorkReviewerAgent, ReviewerID: "lead", CreatedAt: now},
		},
	}

	view := ProjectExecutionView(snapshot)
	if view == nil {
		t.Fatal("expected Execution view")
	}
	if view.Progress.Total != 3 ||
		view.Progress.Accepted != 1 ||
		view.Progress.Running != 1 ||
		view.Progress.Waiting != 1 {
		t.Fatalf("unexpected progress: %+v", view.Progress)
	}
	if len(view.WorkItems) != 3 {
		t.Fatalf("work item count = %d, want 3", len(view.WorkItems))
	}
	if view.WorkItems[0].Status != protocol.ExecutionWorkItemViewAccepted ||
		view.WorkItems[0].Acceptance == nil {
		t.Fatalf("accepted work projection is incomplete: %+v", view.WorkItems[0])
	}
	running := view.WorkItems[1]
	if running.Status != protocol.ExecutionWorkItemViewRunning ||
		running.OwnerAgentID != "builder" ||
		len(running.Attempts) != 2 ||
		running.Attempts[0].AgentRoundID != "agent-round-builder-1" ||
		len(running.OutputScopes) != 1 {
		t.Fatalf("running work projection is incomplete: %+v", running)
	}
	waiting := view.WorkItems[2]
	if waiting.Status != protocol.ExecutionWorkItemViewWaiting ||
		len(waiting.DependencyIDs) != 1 ||
		waiting.DependencyIDs[0] != "work-b" {
		t.Fatalf("dependency projection is incomplete: %+v", waiting)
	}
	if len(view.Graph.Nodes) != 7 {
		t.Fatalf("graph node count = %d, want 7: %+v", len(view.Graph.Nodes), view.Graph.Nodes)
	}
	if len(view.Graph.Edges) != 6 {
		t.Fatalf("graph edge count = %d, want 6: %+v", len(view.Graph.Edges), view.Graph.Edges)
	}
	coordinator := graphNodeByID(view.Graph.Nodes, "coordinator:execution-1")
	if coordinator.Kind != protocol.ExecutionGraphNodeAgent ||
		coordinator.AgentID != "lead" || coordinator.Position != -1 {
		t.Fatalf("Room coordinator node projection is incomplete: %+v", coordinator)
	}
	workNode := graphNodeByID(view.Graph.Nodes, "work-b")
	if workNode.ID != "work-b" ||
		workNode.Kind != protocol.ExecutionGraphNodeAgent ||
		workNode.Visibility != protocol.ExecutionGraphNodePrimary ||
		workNode.WorkItemID != "work-b" ||
		workNode.AttemptID != "attempt-root" ||
		workNode.AgentID != "builder" ||
		workNode.AgentRoundID != "agent-round-builder-1" ||
		workNode.ResponsibilityStatus != protocol.ExecutionWorkItemViewRunning ||
		workNode.RunStatus != protocol.WorkAttemptStatusRunning ||
		workNode.Position != 1 || len(workNode.Runs) != 1 ||
		workNode.Runs[0].AttemptID != "attempt-root" {
		t.Fatalf("primary Agent node projection is incomplete: %+v", workNode)
	}
	child := graphNodeByID(view.Graph.Nodes, "attempt-child")
	if child.Kind != protocol.ExecutionGraphNodeSubagent ||
		child.Visibility != protocol.ExecutionGraphNodeNested ||
		child.ParentNodeID != "work-b" ||
		child.WorkItemID != "work-b" ||
		child.RunStatus != protocol.WorkAttemptStatusRunning {
		t.Fatalf("nested Subagent node projection is incomplete: %+v", child)
	}
	acceptedGate := graphNodeByID(view.Graph.Nodes, "review:assignment-a")
	if acceptedGate.Kind != protocol.ExecutionGraphNodeGate ||
		acceptedGate.AgentID != "lead" ||
		acceptedGate.LifecycleStatus != "accepted" {
		t.Fatalf("accepted Lead gate projection is incomplete: %+v", acceptedGate)
	}
	plannedGate := graphNodeByID(view.Graph.Nodes, "review:assignment-b")
	if plannedGate.Kind != protocol.ExecutionGraphNodeGate ||
		plannedGate.AgentID != "lead" ||
		plannedGate.LifecycleStatus != "planned" {
		t.Fatalf("planned Lead gate projection is incomplete: %+v", plannedGate)
	}
	if !hasExecutionGraphEdge(
		view.Graph.Edges,
		protocol.ExecutionGraphEdgeCoordination,
		coordinator.ID,
		"work-a",
	) || !hasExecutionGraphEdge(
		view.Graph.Edges,
		protocol.ExecutionGraphEdgeDependency,
		"review:assignment-a",
		"work-b",
	) || !hasExecutionGraphEdge(
		view.Graph.Edges,
		protocol.ExecutionGraphEdgeDependency,
		"review:assignment-b",
		"work-c",
	) || !hasExecutionGraphEdge(
		view.Graph.Edges,
		protocol.ExecutionGraphEdgeSpawn,
		"work-b",
		"attempt-child",
	) {
		t.Fatalf("typed graph edges are incomplete: %+v", view.Graph.Edges)
	}
}

func TestProjectExecutionGraphViewKeepsSiblingSubagentsVisibleInLaunchOrder(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 6, 8, 0, 0, 0, time.UTC)
	graph := projectExecutionGraphView([]protocol.ExecutionWorkItemView{{
		ID:       "work-a",
		Position: 0,
		Status:   protocol.ExecutionWorkItemViewRunning,
		Attempts: []protocol.ExecutionAttemptView{
			{
				ID:           "attempt-root",
				ExecutorKind: protocol.AttemptExecutorAgent,
				AgentRoundID: "agent-round-1",
				Status:       protocol.WorkAttemptStatusRunning,
				CreatedAt:    now,
			},
			{
				ID:              "attempt-child-first",
				ParentAttemptID: "attempt-root",
				ExecutorKind:    protocol.AttemptExecutorSubagent,
				ToolUseID:       "spawn-first",
				Status:          protocol.WorkAttemptStatusRunning,
				CreatedAt:       now.Add(time.Second),
			},
			{
				ID:              "attempt-child-second",
				ParentAttemptID: "attempt-root",
				ExecutorKind:    protocol.AttemptExecutorSubagent,
				ToolUseID:       "spawn-second",
				Status:          protocol.WorkAttemptStatusRunning,
				CreatedAt:       now.Add(2 * time.Second),
			},
		},
	}})

	if len(graph.Nodes) != 3 || len(graph.Edges) != 2 {
		t.Fatalf("sibling Subagents were collapsed: %+v", graph)
	}
	first := graphNodeByID(graph.Nodes, "attempt-child-first")
	second := graphNodeByID(graph.Nodes, "attempt-child-second")
	if first.Visibility != protocol.ExecutionGraphNodeNested ||
		second.Visibility != protocol.ExecutionGraphNodeNested ||
		first.ParentNodeID != "work-a" || second.ParentNodeID != "work-a" ||
		first.Position >= second.Position {
		t.Fatalf("sibling Subagent projection lost visibility or launch order: first=%+v second=%+v", first, second)
	}
	if !hasExecutionGraphEdge(
		graph.Edges,
		protocol.ExecutionGraphEdgeSpawn,
		"work-a",
		first.ID,
	) || !hasExecutionGraphEdge(
		graph.Edges,
		protocol.ExecutionGraphEdgeSpawn,
		"work-a",
		second.ID,
	) {
		t.Fatalf("sibling Subagent spawn edges are incomplete: %+v", graph.Edges)
	}
}

func TestProjectExecutionGraphViewShowsChangesRequestedAsBoundedLoop(t *testing.T) {
	t.Parallel()

	graph := projectExecutionGraphView([]protocol.ExecutionWorkItemView{{
		ID:               "work-a",
		Subject:          "Draft",
		Position:         0,
		Status:           protocol.ExecutionWorkItemViewChangesRequested,
		OwnerAgentID:     "writer",
		AssignmentID:     "assignment-a",
		ReviewAgentID:    "lead",
		ReviewDispatchID: "review-dispatch-a",
		ReviewStatus:     string(protocol.ExecutionReviewDispatchStatusDelivered),
		Acceptance: &protocol.ExecutionAcceptanceView{
			ID:           "acceptance-a",
			Decision:     protocol.WorkAcceptanceChangesRequested,
			ReviewerKind: protocol.WorkReviewerAgent,
			ReviewerID:   "lead",
		},
	}})

	gate := graphNodeByID(graph.Nodes, "review:assignment-a")
	if gate.Kind != protocol.ExecutionGraphNodeGate ||
		gate.ReviewDispatchID != "review-dispatch-a" ||
		gate.LifecycleStatus != string(protocol.WorkAcceptanceChangesRequested) {
		t.Fatalf("changes-requested gate projection is incomplete: %+v", gate)
	}
	if !hasExecutionGraphEdge(
		graph.Edges,
		protocol.ExecutionGraphEdgeReview,
		"work-a",
		gate.ID,
	) || !hasExecutionGraphEdge(
		graph.Edges,
		protocol.ExecutionGraphEdgeLoopBack,
		gate.ID,
		"work-a",
	) {
		t.Fatalf("changes-requested loop edges are incomplete: %+v", graph.Edges)
	}
}

func TestProjectExecutionGraphViewDoesNotTurnContainmentIntoDependency(t *testing.T) {
	t.Parallel()

	graph := projectExecutionGraphView([]protocol.ExecutionWorkItemView{
		{
			ID:       "parent",
			Position: 0,
			Status:   protocol.ExecutionWorkItemViewRunning,
		},
		{
			ID:               "child-group",
			ParentWorkItemID: "parent",
			Position:         1,
			Status:           protocol.ExecutionWorkItemViewReady,
		},
	})
	if len(graph.Edges) != 0 {
		t.Fatalf("containment created executable edges: %+v", graph.Edges)
	}
}

func graphNodeByID(
	nodes []protocol.ExecutionGraphNodeView,
	id string,
) protocol.ExecutionGraphNodeView {
	for _, node := range nodes {
		if node.ID == id {
			return node
		}
	}
	return protocol.ExecutionGraphNodeView{}
}

func hasExecutionGraphEdge(
	edges []protocol.ExecutionGraphEdgeView,
	kind protocol.ExecutionGraphEdgeKind,
	sourceID string,
	targetID string,
) bool {
	for _, edge := range edges {
		if edge.Kind == kind &&
			edge.SourceNodeID == sourceID &&
			edge.TargetNodeID == targetID {
			return true
		}
	}
	return false
}

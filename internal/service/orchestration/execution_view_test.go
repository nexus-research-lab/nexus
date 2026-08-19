package orchestration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type latestExecutionViewRepository struct {
	*runtimeGraphRepositoryFake
	current           *protocol.Execution
	latest            *protocol.Execution
	managedGraph      protocol.ExecutionRuntimeGraph
	planlessGraph     protocol.ExecutionRuntimeGraph
	workGraphAttempts []protocol.WorkAttempt
}

func (repository *latestExecutionViewRepository) FindCurrentManaged(
	context.Context,
	string,
	string,
) (*protocol.Execution, error) {
	return repository.current, nil
}

func (repository *latestExecutionViewRepository) FindLatestManaged(
	context.Context,
	string,
	string,
) (*protocol.Execution, error) {
	return repository.latest, nil
}

func (repository *latestExecutionViewRepository) GetRuntimeGraph(
	_ context.Context,
	_ string,
	_ string,
	executionID string,
	_ string,
) (protocol.ExecutionRuntimeGraph, error) {
	if executionID == "" {
		return repository.planlessGraph, nil
	}
	return repository.managedGraph, nil
}

func (repository *latestExecutionViewRepository) GetWorkGraphRuntimeGraph(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
	executionID string,
	executionRootRoundID string,
) (protocol.ExecutionRuntimeGraph, error) {
	return repository.GetRuntimeGraph(
		ctx,
		ownerUserID,
		sessionKey,
		executionID,
		executionRootRoundID,
	)
}

func (repository *latestExecutionViewRepository) ListWorkGraphChildAttempts(
	context.Context,
	string,
) ([]protocol.WorkAttempt, error) {
	return repository.workGraphAttempts, nil
}

type runtimeGraphSubagentCompleteHistory struct {
	tasks []RuntimeGraphSubagentTaskHistory
	tools []RuntimeGraphSubagentToolHistory
}

func (history runtimeGraphSubagentCompleteHistory) ListRuntimeGraphSubagentTaskHistory(
	context.Context,
	string,
	string,
) ([]RuntimeGraphSubagentTaskHistory, error) {
	return history.tasks, nil
}

func (history runtimeGraphSubagentCompleteHistory) ListRuntimeGraphSubagentToolHistory(
	context.Context,
	string,
	string,
) ([]RuntimeGraphSubagentToolHistory, error) {
	return history.tools, nil
}

func TestGetLatestViewKeepsTerminalManagedGraphAheadOfNewerPlanlessRound(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 6, 15, 0, 0, 0, time.UTC)
	snapshot := assignedExecutionSnapshot()
	snapshot.Execution.Status = protocol.ExecutionStatusCompleted
	snapshot.Execution.UpdatedAt = now
	latest := snapshot.Execution
	repository := &latestExecutionViewRepository{
		runtimeGraphRepositoryFake: &runtimeGraphRepositoryFake{
			fakeRepository: &fakeRepository{snapshot: snapshot},
		},
		latest: &latest,
		planlessGraph: protocol.ExecutionRuntimeGraph{
			GraphID: "round:later-chat",
			Nodes: []protocol.ExecutionRuntimeNodeRun{{
				ID:           "runtime-agent-later-chat",
				Kind:         protocol.ExecutionRuntimeNodeAgent,
				SubjectID:    "round-later-chat",
				AgentRoundID: "round-later-chat",
				AgentID:      "agent-1",
				Status:       protocol.ExecutionRuntimeNodeSucceeded,
				StartedAt:    now.Add(time.Minute),
				UpdatedAt:    now.Add(time.Minute),
			}},
		},
	}

	view, err := NewService(repository).GetLatestView(
		context.Background(),
		snapshot.Execution.OwnerUserID,
		snapshot.Execution.SessionKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	if view == nil || view.ID != snapshot.Execution.ID || view.Plan == nil ||
		len(view.WorkItems) == 0 || view.Status != protocol.ExecutionStatusCompleted {
		t.Fatalf("latest managed WorkGraph was replaced by a planless round: %+v", view)
	}
}

func TestGetLatestViewProjectsEveryHistoricalSubagentWithExactTaskLifecycle(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 6, 16, 0, 0, 0, time.UTC)
	snapshot := assignedExecutionSnapshot()
	snapshot.Execution.RootRoundID = "root-round-1"
	snapshot.Attempts[0].AgentRoundID = "agent-round-1"
	snapshot.Attempts[0].Status = protocol.WorkAttemptStatusSucceeded
	root := snapshot.Attempts[0]
	children := make([]protocol.WorkAttempt, 0, 4)
	runtimeNodes := []protocol.ExecutionRuntimeNodeRun{{
		ID:             "runtime-agent-1",
		GraphID:        "execution:execution-1",
		OwnerUserID:    snapshot.Execution.OwnerUserID,
		SessionKey:     snapshot.Execution.SessionKey,
		ExecutionID:    snapshot.Execution.ID,
		Kind:           protocol.ExecutionRuntimeNodeAgent,
		SubjectID:      "agent-round-1",
		RootRoundID:    "root-round-1",
		RuntimeRoundID: "runtime-round-1",
		AgentRoundID:   "agent-round-1",
		AgentID:        "agent-worker",
		Status:         protocol.ExecutionRuntimeNodeSucceeded,
		StartedAt:      now,
		UpdatedAt:      now,
	}}
	runtimeEdges := make([]protocol.ExecutionRuntimeEdgeRun, 0, 4)
	tasks := make([]RuntimeGraphSubagentTaskHistory, 0, 4)
	for index := 1; index <= 4; index++ {
		attemptID := fmt.Sprintf("attempt-child-%d", index)
		toolUseID := fmt.Sprintf("launch-agent-%d", index)
		taskID := fmt.Sprintf("task-%d", index)
		child := protocol.WorkAttempt{
			ID:              attemptID,
			ExecutionID:     snapshot.Execution.ID,
			PlanID:          snapshot.Plan.ID,
			WorkItemID:      root.WorkItemID,
			SpecID:          root.SpecID,
			AssignmentID:    root.AssignmentID,
			ParentAttemptID: root.ID,
			ExecutorKind:    protocol.AttemptExecutorSubagent,
			ParentAgentID:   "agent-worker",
			AgentRoundID:    "agent-round-1",
			ToolUseID:       toolUseID,
			Status:          protocol.WorkAttemptStatusInterrupted,
			CreatedAt:       now.Add(time.Duration(index) * time.Second),
		}
		children = append(children, child)
		launchNodeID := fmt.Sprintf("runtime-launch-%d", index)
		runtimeNodes = append(runtimeNodes, protocol.ExecutionRuntimeNodeRun{
			ID:              launchNodeID,
			GraphID:         "execution:execution-1",
			OwnerUserID:     snapshot.Execution.OwnerUserID,
			SessionKey:      snapshot.Execution.SessionKey,
			ExecutionID:     snapshot.Execution.ID,
			Kind:            protocol.ExecutionRuntimeNodeTool,
			SubjectID:       toolUseID,
			ParentSubjectID: "agent-round-1",
			RootRoundID:     "root-round-1",
			RuntimeRoundID:  "runtime-round-1",
			AgentRoundID:    "agent-round-1",
			AgentID:         "agent-worker",
			Name:            "Agent",
			Status:          protocol.ExecutionRuntimeNodeSucceeded,
			StartedAt:       now.Add(time.Duration(index) * time.Second),
			UpdatedAt:       now.Add(time.Duration(index) * time.Second),
		})
		runtimeEdges = append(runtimeEdges, protocol.ExecutionRuntimeEdgeRun{
			ID:           fmt.Sprintf("invoke-%d", index),
			GraphID:      "execution:execution-1",
			OwnerUserID:  snapshot.Execution.OwnerUserID,
			SessionKey:   snapshot.Execution.SessionKey,
			SourceNodeID: "runtime-agent-1",
			TargetNodeID: launchNodeID,
			Kind:         protocol.ExecutionRuntimeEdgeInvoke,
			CreatedAt:    now.Add(time.Duration(index) * time.Second),
		})
		tasks = append(tasks, RuntimeGraphSubagentTaskHistory{
			TaskID:         taskID,
			AgentID:        fmt.Sprintf("child-agent-%d", index),
			AgentType:      "Explore",
			ChildSessionID: fmt.Sprintf("child-session-%d", index),
			Description:    fmt.Sprintf("Parallel analysis %d", index),
			Status:         "completed",
			ToolUseID:      toolUseID,
			StartedAt:      now.Add(time.Duration(index) * time.Second).UnixMilli(),
			UpdatedAt:      now.Add(time.Duration(index+1) * time.Second).UnixMilli(),
		})
	}
	// The operational Snapshot intentionally retains only its latest terminal
	// child; WorkGraph history must restore every sibling.
	snapshot.Attempts = append(snapshot.Attempts, children[len(children)-1])
	current := snapshot.Execution
	repository := &latestExecutionViewRepository{
		runtimeGraphRepositoryFake: &runtimeGraphRepositoryFake{
			fakeRepository: &fakeRepository{snapshot: snapshot},
		},
		current:           &current,
		workGraphAttempts: children,
		managedGraph: protocol.ExecutionRuntimeGraph{
			GraphID:   "execution:execution-1",
			Nodes:     runtimeNodes,
			Edges:     runtimeEdges,
			NodeTotal: len(runtimeNodes),
			EdgeTotal: len(runtimeEdges),
		},
	}
	service := NewService(repository)
	service.SetRuntimeGraphSubagentToolHistoryProvider(runtimeGraphSubagentCompleteHistory{
		tasks: tasks,
		tools: []RuntimeGraphSubagentToolHistory{{
			ParentToolUseID: "launch-agent-1",
			TaskID:          "task-1",
			AgentID:         "child-agent-1",
			ToolUseID:       "child-web-fetch-1",
			Name:            "WebFetch",
			Status:          "completed",
			StartedAt:       now.Add(2 * time.Second).UnixMilli(),
			FinishedAt:      now.Add(3 * time.Second).UnixMilli(),
		}},
	})
	view, err := service.GetLatestView(
		context.Background(),
		snapshot.Execution.OwnerUserID,
		snapshot.Execution.SessionKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	subagents := make([]protocol.ExecutionGraphNodeView, 0, 4)
	childTool := protocol.ExecutionGraphNodeView{}
	for _, node := range view.Graph.Nodes {
		if node.Kind == protocol.ExecutionGraphNodeSubagent {
			subagents = append(subagents, node)
		}
		if node.Kind == protocol.ExecutionGraphNodeTool && strings.EqualFold(node.Name, "Agent") {
			t.Fatalf("Agent launch leaked as a separate Tool node: %+v", node)
		}
		if node.Kind == protocol.ExecutionGraphNodeTool && node.SubjectID == "child-web-fetch-1" {
			childTool = node
		}
	}
	if len(subagents) != 4 {
		t.Fatalf("Subagent node count = %d, want 4: %+v", len(subagents), view.Graph.Nodes)
	}
	for index, node := range subagents {
		want := index + 1
		if node.AttemptID != fmt.Sprintf("attempt-child-%d", want) ||
			node.SubjectID != fmt.Sprintf("task-%d", want) ||
			node.AgentID != fmt.Sprintf("child-agent-%d", want) ||
			node.LifecycleStatus != string(protocol.ExecutionRuntimeNodeSucceeded) {
			t.Fatalf("Subagent %d lost exact identity/status: %+v", want, node)
		}
	}
	if childTool.ID == "" || childTool.ParentNodeID != "attempt-child-1" ||
		childTool.LifecycleStatus != string(protocol.ExecutionRuntimeNodeSucceeded) {
		t.Fatalf("Subagent child Tool lost exact ownership/status: %+v", childTool)
	}
}

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

func TestProjectExecutionGraphViewKeepsRejectedReviewCyclesDistinct(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 19, 9, 0, 0, 0, time.UTC)
	finishedFirst := now.Add(time.Minute)
	finishedSecond := now.Add(3 * time.Minute)
	items := []protocol.ExecutionWorkItemView{{
		ID:               "work-draft",
		Subject:          "Draft",
		Position:         0,
		Status:           protocol.ExecutionWorkItemViewAccepted,
		OwnerAgentID:     "writer",
		AssignmentID:     "assignment-2",
		ReviewAgentID:    "lead",
		AssignmentStatus: protocol.WorkAssignmentStatusCompleted,
	}}
	history := protocol.ExecutionWorkGraphHistory{
		Assignments: []protocol.WorkAssignment{
			{ID: "assignment-1", WorkItemID: "work-draft", OwnerAgentID: "writer", ReturnToAgentID: "lead"},
			{ID: "assignment-2", WorkItemID: "work-draft", OwnerAgentID: "writer", ReturnToAgentID: "lead"},
		},
		Attempts: []protocol.WorkAttempt{
			{ID: "attempt-1", AssignmentID: "assignment-1", WorkItemID: "work-draft", ExecutorAgentID: "writer", AgentRoundID: "round-1", Status: protocol.WorkAttemptStatusSucceeded, CreatedAt: now, FinishedAt: &finishedFirst},
			{ID: "attempt-2", AssignmentID: "assignment-2", WorkItemID: "work-draft", ExecutorAgentID: "writer", AgentRoundID: "round-2", Status: protocol.WorkAttemptStatusSucceeded, CreatedAt: now.Add(2 * time.Minute), FinishedAt: &finishedSecond},
		},
		Submissions: []protocol.WorkSubmission{
			{ID: "submission-1", AssignmentID: "assignment-1", AttemptID: "attempt-1", WorkItemID: "work-draft", Sequence: 1, ResultSummary: "draft v1", CreatedAt: finishedFirst},
			{ID: "submission-2", AssignmentID: "assignment-2", AttemptID: "attempt-2", WorkItemID: "work-draft", Sequence: 2, ResultSummary: "draft v2", CreatedAt: finishedSecond},
		},
		ReviewDispatches: []protocol.ExecutionReviewDispatch{
			{ID: "review-dispatch-1", AssignmentID: "assignment-1", SubmissionID: "submission-1", WorkItemID: "work-draft", TargetAgentID: "lead", Status: protocol.ExecutionReviewDispatchStatusDelivered},
			{ID: "review-dispatch-2", AssignmentID: "assignment-2", SubmissionID: "submission-2", WorkItemID: "work-draft", TargetAgentID: "lead", Status: protocol.ExecutionReviewDispatchStatusDelivered},
		},
		Acceptances: []protocol.WorkAcceptance{
			{ID: "acceptance-1", AssignmentID: "assignment-1", SubmissionID: "submission-1", WorkItemID: "work-draft", Decision: protocol.WorkAcceptanceRejected, ReviewerKind: protocol.WorkReviewerAgent, ReviewerID: "lead", Feedback: "add evidence"},
			{ID: "acceptance-2", AssignmentID: "assignment-2", SubmissionID: "submission-2", WorkItemID: "work-draft", Decision: protocol.WorkAcceptanceAccepted, ReviewerKind: protocol.WorkReviewerAgent, ReviewerID: "lead"},
		},
	}

	graph := projectExecutionGraphViewWithHistory(items, history)
	firstAttempt := graphNodeByID(graph.Nodes, "attempt-1")
	secondAttempt := graphNodeByID(graph.Nodes, "work-draft")
	firstReview := graphNodeByID(graph.Nodes, "review:submission-1")
	secondReview := graphNodeByID(graph.Nodes, "review:submission-2")
	if firstAttempt.AttemptID != "attempt-1" || len(firstAttempt.Runs) != 1 ||
		secondAttempt.AttemptID != "attempt-2" || len(secondAttempt.Runs) != 1 {
		t.Fatalf("root Attempt cycles were collapsed: first=%+v second=%+v", firstAttempt, secondAttempt)
	}
	if firstReview.LifecycleStatus != string(protocol.WorkAcceptanceRejected) ||
		secondReview.LifecycleStatus != string(protocol.WorkAcceptanceAccepted) ||
		firstReview.ResultSummary != "add evidence" ||
		secondReview.ResultSummary != "draft v2" {
		t.Fatalf("review history was lost: first=%+v second=%+v", firstReview, secondReview)
	}
	if !hasExecutionGraphEdge(graph.Edges, protocol.ExecutionGraphEdgeReview, firstAttempt.ID, firstReview.ID) ||
		!hasExecutionGraphEdge(graph.Edges, protocol.ExecutionGraphEdgeLoopBack, firstReview.ID, secondAttempt.ID) ||
		!hasExecutionGraphEdge(graph.Edges, protocol.ExecutionGraphEdgeReview, secondAttempt.ID, secondReview.ID) {
		t.Fatalf("review cycle edges are incomplete: %+v", graph.Edges)
	}
}

func TestProjectExecutionGraphViewDoesNotDuplicateAcceptedHistoricalGate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	finished := now.Add(time.Minute)
	items := []protocol.ExecutionWorkItemView{{
		ID:            "work-complete",
		Subject:       "Complete",
		Position:      0,
		Status:        protocol.ExecutionWorkItemViewAccepted,
		OwnerAgentID:  "writer",
		ReviewAgentID: "lead",
		Submission: &protocol.ExecutionSubmissionView{
			ID:           "submission-complete",
			AssignmentID: "assignment-complete",
			AttemptID:    "attempt-complete",
			CreatedAt:    finished,
		},
		Acceptance: &protocol.ExecutionAcceptanceView{
			ID:           "acceptance-complete",
			Decision:     protocol.WorkAcceptanceAccepted,
			ReviewerKind: protocol.WorkReviewerAgent,
			ReviewerID:   "lead",
			CreatedAt:    finished,
		},
	}}
	history := protocol.ExecutionWorkGraphHistory{
		Assignments: []protocol.WorkAssignment{{
			ID:              "assignment-complete",
			WorkItemID:      "work-complete",
			OwnerAgentID:    "writer",
			ReturnToAgentID: "lead",
		}},
		Attempts: []protocol.WorkAttempt{{
			ID:              "attempt-complete",
			AssignmentID:    "assignment-complete",
			WorkItemID:      "work-complete",
			ExecutorAgentID: "writer",
			Status:          protocol.WorkAttemptStatusSucceeded,
			CreatedAt:       now,
			FinishedAt:      &finished,
		}},
		Submissions: []protocol.WorkSubmission{{
			ID:           "submission-complete",
			AssignmentID: "assignment-complete",
			AttemptID:    "attempt-complete",
			WorkItemID:   "work-complete",
			CreatedAt:    finished,
		}},
		Acceptances: []protocol.WorkAcceptance{{
			ID:           "acceptance-complete",
			AssignmentID: "assignment-complete",
			SubmissionID: "submission-complete",
			WorkItemID:   "work-complete",
			Decision:     protocol.WorkAcceptanceAccepted,
			ReviewerKind: protocol.WorkReviewerAgent,
			ReviewerID:   "lead",
		}},
	}

	graph := projectExecutionGraphViewWithHistory(items, history)
	gateIDs := make([]string, 0, 1)
	for _, node := range graph.Nodes {
		if node.Kind == protocol.ExecutionGraphNodeGate {
			gateIDs = append(gateIDs, node.ID)
		}
	}
	if len(gateIDs) != 1 || gateIDs[0] != "review:submission-complete" {
		t.Fatalf("accepted Submission produced duplicate current Gate: %v", gateIDs)
	}
	if !hasExecutionGraphEdge(
		graph.Edges,
		protocol.ExecutionGraphEdgeReview,
		"work-complete",
		"review:submission-complete",
	) {
		t.Fatalf("accepted historical Gate lost its review edge: %+v", graph.Edges)
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

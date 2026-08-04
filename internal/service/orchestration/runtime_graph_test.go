package orchestration

import (
	"context"
	"testing"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type runtimeGraphRepositoryFake struct {
	*fakeRepository
	nodes          []protocol.ExecutionRuntimeNodeRun
	edges          []protocol.ExecutionRuntimeEdgeRun
	reconciled     int
	finishedStatus protocol.ExecutionRuntimeNodeStatus
	graph          protocol.ExecutionRuntimeGraph
}

func (f *runtimeGraphRepositoryFake) UpsertRuntimeGraphNode(
	_ context.Context,
	item protocol.ExecutionRuntimeNodeRun,
) error {
	f.nodes = append(f.nodes, item)
	return nil
}

func (f *runtimeGraphRepositoryFake) UpsertRuntimeGraphEdge(
	_ context.Context,
	item protocol.ExecutionRuntimeEdgeRun,
) error {
	f.edges = append(f.edges, item)
	return nil
}

func (f *runtimeGraphRepositoryFake) ReconcileRuntimeGraphAgent(
	context.Context,
	string,
	string,
	string,
	string,
	time.Time,
) error {
	f.reconciled++
	return nil
}

func (f *runtimeGraphRepositoryFake) FinishRuntimeGraphRound(
	_ context.Context,
	_ string,
	_ string,
	_ string,
	status protocol.ExecutionRuntimeNodeStatus,
	_ time.Time,
) error {
	f.finishedStatus = status
	return nil
}

func (f *runtimeGraphRepositoryFake) GetRuntimeGraph(
	context.Context,
	string,
	string,
	string,
	string,
) (protocol.ExecutionRuntimeGraph, error) {
	return f.graph, nil
}

func TestRuntimeGraphObservesBridgeToolLifecycleWithoutModelStatusCall(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	repository := &runtimeGraphRepositoryFake{fakeRepository: &fakeRepository{}}
	service := NewService(repository)
	service.now = func() time.Time { return now }
	actor := ActorContext{
		OwnerUserID:    "owner-1",
		SessionKey:     "session-1",
		AgentID:        "agent-1",
		RootRoundID:    "round-1",
		RuntimeRoundID: "round-1",
		AgentRoundID:   "agent-round-1",
	}
	message, err := sdkprotocol.DecodeMessage(map[string]any{
		"type": "assistant",
		"uuid": "assistant-1",
		"message": map[string]any{
			"role": "assistant",
			"content": []any{map[string]any{
				"type":  "tool_use",
				"id":    "tool-1",
				"name":  "search",
				"input": map[string]any{"secret": "not persisted"},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = service.ObserveRuntimeMessage(context.Background(), actor, message); err != nil {
		t.Fatal(err)
	}
	if repository.reconciled != 1 || len(repository.nodes) != 2 || len(repository.edges) != 1 {
		t.Fatalf("runtime graph writes = reconcile:%d nodes:%d edges:%d", repository.reconciled, len(repository.nodes), len(repository.edges))
	}
	tool := repository.nodes[1]
	if tool.Kind != protocol.ExecutionRuntimeNodeTool ||
		tool.SubjectID != "tool-1" || tool.Name != "search" ||
		tool.Status != protocol.ExecutionRuntimeNodeRunning {
		t.Fatalf("unexpected tool node: %+v", tool)
	}
	if _, leaked := tool.Metadata["secret"]; leaked {
		t.Fatalf("tool input leaked into runtime graph metadata: %+v", tool.Metadata)
	}
	if err = service.FinishRuntimeRound(
		context.Background(),
		actor,
		"",
		"round interrupted",
	); err != nil {
		t.Fatal(err)
	}
	if repository.finishedStatus != protocol.ExecutionRuntimeNodeInterrupted {
		t.Fatalf("finished status = %q, want interrupted", repository.finishedStatus)
	}
}

func TestRuntimeGraphRecordsSanitizedFailureAndObservedControlReturn(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	repository := &runtimeGraphRepositoryFake{fakeRepository: &fakeRepository{}}
	service := NewService(repository)
	service.now = func() time.Time { return now }
	actor := ActorContext{
		OwnerUserID: "owner-1", SessionKey: "session-1", AgentID: "agent-1",
		RootRoundID: "round-1", RuntimeRoundID: "round-1", AgentRoundID: "agent-round-1",
	}
	message, err := sdkprotocol.DecodeMessage(map[string]any{
		"type": "user",
		"uuid": "tool-result-1",
		"message": map[string]any{
			"role": "user",
			"content": []any{map[string]any{
				"type": "tool_result", "tool_use_id": "tool-1",
				"is_error": true, "error_code": "page_unavailable",
				"content": "Authorization: Bearer live-secret could not fetch the page",
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = service.ObserveRuntimeMessage(context.Background(), actor, message); err != nil {
		t.Fatal(err)
	}
	if len(repository.nodes) != 2 || len(repository.edges) != 2 {
		t.Fatalf("runtime writes nodes=%+v edges=%+v", repository.nodes, repository.edges)
	}
	tool := repository.nodes[1]
	if tool.Status != protocol.ExecutionRuntimeNodeFailed ||
		tool.ErrorCode != "page_unavailable" ||
		tool.ErrorSummary != "Authorization: Bearer <redacted> could not fetch the page" {
		t.Fatalf("sanitized failure = %+v", tool)
	}
	if repository.edges[1].Kind != protocol.ExecutionRuntimeEdgeLoopBack ||
		repository.edges[1].SourceNodeID != tool.ID ||
		repository.edges[1].TargetNodeID != repository.nodes[0].ID {
		t.Fatalf("control return edge = %+v", repository.edges[1])
	}
}

func TestRuntimeGraphTreatsRejectedMutationAsFailedControlReturn(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 4, 12, 15, 0, 0, time.UTC)
	repository := &runtimeGraphRepositoryFake{fakeRepository: &fakeRepository{}}
	service := NewService(repository)
	service.now = func() time.Time { return now }
	actor := ActorContext{
		OwnerUserID: "owner-1", SessionKey: "session-1", AgentID: "agent-1",
		RootRoundID: "round-1", RuntimeRoundID: "round-1", AgentRoundID: "agent-round-1",
	}
	message, err := sdkprotocol.DecodeMessage(map[string]any{
		"type": "user",
		"uuid": "tool-result-rejected",
		"message": map[string]any{
			"role": "user",
			"content": []any{map[string]any{
				"type": "tool_result", "tool_use_id": "tool-plan", "is_error": false,
				"content": `{"message":"items is required and must contain at least one complete Work Item object","outcome":"rejected","reason_code":"invalid_input"}`,
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = service.ObserveRuntimeMessage(context.Background(), actor, message); err != nil {
		t.Fatal(err)
	}
	if len(repository.nodes) != 2 || len(repository.edges) != 2 {
		t.Fatalf("runtime writes nodes=%+v edges=%+v", repository.nodes, repository.edges)
	}
	tool := repository.nodes[1]
	if tool.Status != protocol.ExecutionRuntimeNodeFailed || !tool.Failed ||
		tool.ErrorCode != "invalid_input" ||
		tool.ErrorSummary != "items is required and must contain at least one complete Work Item object" ||
		tool.ResultSummary != "" || tool.Metadata["mutation_outcome"] != "rejected" {
		t.Fatalf("rejected mutation node = %+v", tool)
	}
	if repository.edges[1].Kind != protocol.ExecutionRuntimeEdgeLoopBack ||
		repository.edges[1].SourceNodeID != tool.ID ||
		repository.edges[1].TargetNodeID != repository.nodes[0].ID {
		t.Fatalf("rejected mutation control return = %+v", repository.edges[1])
	}
}

func TestCompactRuntimeGraphSummaryHidesInternalSentinels(t *testing.T) {
	t.Parallel()

	got, truncated := compactRuntimeGraphSummary(
		"  __nexus_interrupt_without_message__  ",
	)
	if got != "" || truncated {
		t.Fatalf("internal sentinel summary = %q truncated=%t", got, truncated)
	}
	got, truncated = compactRuntimeGraphSummary(
		"request failed __nexus_internal_control__ after 2s",
	)
	if got != "request failed after 2s" || truncated {
		t.Fatalf("mixed sentinel summary = %q truncated=%t", got, truncated)
	}
}

func TestRuntimeGraphRecordsOnlyExactlyCorrelatedRetry(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 4, 12, 30, 0, 0, time.UTC)
	previous := protocol.ExecutionRuntimeNodeRun{
		ID: "runtime-tool-previous", Kind: protocol.ExecutionRuntimeNodeTool,
		SubjectID: "tool-previous", AgentRoundID: "agent-round-1",
	}
	repository := &runtimeGraphRepositoryFake{
		fakeRepository: &fakeRepository{},
		graph:          protocol.ExecutionRuntimeGraph{Nodes: []protocol.ExecutionRuntimeNodeRun{previous}},
	}
	service := NewService(repository)
	service.now = func() time.Time { return now }
	actor := ActorContext{
		OwnerUserID: "owner-1", SessionKey: "session-1", AgentID: "agent-1",
		RootRoundID: "round-1", RuntimeRoundID: "round-1", AgentRoundID: "agent-round-1",
	}
	message := sdkprotocol.ReceivedMessage{
		UUID: "assistant-retry-1",
		RuntimeLifecycle: []sdkprotocol.RuntimeLifecycleEvent{{
			EventID: "retry-event-1", NodeKind: sdkprotocol.RuntimeLifecycleNodeTool,
			Phase: sdkprotocol.RuntimeLifecycleStarted, SubjectID: "tool-retry",
			Name: "search", Status: "running",
			Metadata: map[string]string{"retry_of_tool_use_id": "tool-previous"},
		}},
	}
	if err := service.ObserveRuntimeMessage(context.Background(), actor, message); err != nil {
		t.Fatal(err)
	}
	if len(repository.edges) != 2 ||
		repository.edges[1].Kind != protocol.ExecutionRuntimeEdgeRetry ||
		repository.edges[1].SourceNodeID != previous.ID ||
		repository.edges[1].TargetNodeID != repository.nodes[1].ID {
		t.Fatalf("exact retry edge = %+v", repository.edges)
	}
}

func TestRuntimeGraphNestsChildToolsUnderExactSubagentIdentity(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 3, 10, 30, 0, 0, time.UTC)
	actor := ActorContext{
		OwnerUserID:    "owner-1",
		SessionKey:     "session-1",
		AgentID:        "agent-1",
		RootRoundID:    "round-1",
		RuntimeRoundID: "round-1",
		AgentRoundID:   "agent-round-1",
	}
	subagentNodeID := "runtime-subagent-1"
	repository := &runtimeGraphRepositoryFake{
		fakeRepository: &fakeRepository{},
		graph: protocol.ExecutionRuntimeGraph{Nodes: []protocol.ExecutionRuntimeNodeRun{{
			ID:           subagentNodeID,
			Kind:         protocol.ExecutionRuntimeNodeSubagent,
			SubjectID:    "task-1",
			AgentRoundID: "agent-round-1",
			Metadata:     map[string]any{"tool_use_id": "spawn-tool-1"},
		}}},
	}
	service := NewService(repository)
	service.now = func() time.Time { return now }
	message, err := sdkprotocol.DecodeMessage(map[string]any{
		"type":               "assistant",
		"uuid":               "assistant-child-1",
		"parent_tool_use_id": "spawn-tool-1",
		"message": map[string]any{
			"role": "assistant",
			"content": []any{map[string]any{
				"type": "tool_use",
				"id":   "child-tool-1",
				"name": "read_file",
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = service.ObserveRuntimeMessage(context.Background(), actor, message); err != nil {
		t.Fatal(err)
	}
	if len(repository.edges) != 1 ||
		repository.edges[0].SourceNodeID != subagentNodeID ||
		repository.edges[0].Kind != protocol.ExecutionRuntimeEdgeInvoke {
		t.Fatalf("child tool edge = %+v", repository.edges)
	}
}

func TestGetLatestViewReturnsPlanlessAgentToolGraph(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	rootID := "runtime-agent-1"
	toolID := "runtime-tool-1"
	repository := &runtimeGraphRepositoryFake{
		fakeRepository: &fakeRepository{},
		graph: protocol.ExecutionRuntimeGraph{
			GraphID: "round:round-1",
			Nodes: []protocol.ExecutionRuntimeNodeRun{
				{
					ID: "runtime-agent-1", GraphID: "round:round-1",
					OwnerUserID: "owner-1", SessionKey: "session-1",
					Kind: protocol.ExecutionRuntimeNodeAgent, SubjectID: "agent-round-1",
					RootRoundID: "round-1", RuntimeRoundID: "round-1",
					AgentRoundID: "agent-round-1", AgentID: "agent-1",
					Status:    protocol.ExecutionRuntimeNodeRunning,
					StartedAt: now, UpdatedAt: now,
				},
				{
					ID: "runtime-tool-1", GraphID: "round:round-1",
					OwnerUserID: "owner-1", SessionKey: "session-1",
					Kind: protocol.ExecutionRuntimeNodeTool, SubjectID: "tool-1",
					RootRoundID: "round-1", RuntimeRoundID: "round-1",
					AgentRoundID: "agent-round-1", AgentID: "agent-1", Name: "search",
					Status:    protocol.ExecutionRuntimeNodeRunning,
					StartedAt: now, UpdatedAt: now,
				},
			},
			Edges: []protocol.ExecutionRuntimeEdgeRun{{
				ID: "runtime-edge-1", GraphID: "round:round-1",
				OwnerUserID: "owner-1", SessionKey: "session-1",
				SourceNodeID: rootID, TargetNodeID: toolID,
				Kind: protocol.ExecutionRuntimeEdgeInvoke, CreatedAt: now,
			}},
		},
	}
	view, err := NewService(repository).GetLatestView(
		context.Background(),
		"owner-1",
		"session-1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if view == nil || view.Plan != nil || len(view.WorkItems) != 0 ||
		view.ID != "round:round-1" || view.Status != protocol.ExecutionStatusActive {
		t.Fatalf("unexpected planless view: %+v", view)
	}
	if len(view.Graph.Nodes) != 2 || len(view.Graph.Edges) != 1 {
		t.Fatalf("unexpected planless graph: %+v", view.Graph)
	}
	tool := graphNodeByID(view.Graph.Nodes, toolID)
	if tool.Kind != protocol.ExecutionGraphNodeTool ||
		tool.Visibility != protocol.ExecutionGraphNodeNested ||
		tool.ParentNodeID != rootID ||
		tool.LifecycleStatus != "running" {
		t.Fatalf("unexpected planless tool projection: %+v", tool)
	}
}

func TestRuntimeGraphViewRepairsMissingParentEdge(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	rootID := "runtime-agent-repair"
	toolID := "runtime-tool-repair"
	view := &protocol.ExecutionView{}
	mergeExecutionRuntimeGraph(view, protocol.ExecutionRuntimeGraph{
		GraphID: "round:repair",
		Nodes: []protocol.ExecutionRuntimeNodeRun{
			{
				ID: rootID, Kind: protocol.ExecutionRuntimeNodeAgent,
				SubjectID: "agent-round-repair", AgentRoundID: "agent-round-repair",
				AgentID: "agent-1", Status: protocol.ExecutionRuntimeNodeRunning,
				StartedAt: now, UpdatedAt: now,
			},
			{
				ID: toolID, Kind: protocol.ExecutionRuntimeNodeTool,
				SubjectID: "tool-repair", AgentRoundID: "agent-round-repair",
				AgentID: "agent-1", Name: "search",
				Status:    protocol.ExecutionRuntimeNodeRunning,
				StartedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second),
			},
		},
	})

	tool := graphNodeByID(view.Graph.Nodes, toolID)
	if tool.ParentNodeID != rootID ||
		!hasExecutionGraphEdge(
			view.Graph.Edges,
			protocol.ExecutionGraphEdgeInvoke,
			rootID,
			toolID,
		) {
		t.Fatalf("missing runtime parent edge was not repaired: %+v", view.Graph)
	}
}

func TestRuntimeGraphViewBindsManagedRoundsAndFiltersConversationRoots(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	workID := "work-managed"
	coordinatorID := "runtime-coordinator"
	coordinatorNodeID := "coordinator:execution-managed"
	workRoundID := "runtime-work-round"
	toolID := "runtime-work-tool"
	view := &protocol.ExecutionView{
		ID: "execution-managed", CoordinatorAgentID: "lead-1",
		WorkItems: []protocol.ExecutionWorkItemView{{
			ID: workID,
		}},
		Graph: protocol.ExecutionGraphView{Nodes: []protocol.ExecutionGraphNodeView{
			{
				ID: coordinatorNodeID, Kind: protocol.ExecutionGraphNodeAgent,
				Visibility: protocol.ExecutionGraphNodePrimary, AgentID: "lead-1",
			},
			{
				ID: workID, Kind: protocol.ExecutionGraphNodeAgent,
				Visibility: protocol.ExecutionGraphNodePrimary,
				WorkItemID: workID, AgentID: "worker-1",
			},
		}},
	}
	mergeExecutionRuntimeGraph(view, protocol.ExecutionRuntimeGraph{
		GraphID: "execution:execution-managed",
		Nodes: []protocol.ExecutionRuntimeNodeRun{
			{
				ID: coordinatorID, Kind: protocol.ExecutionRuntimeNodeAgent,
				SubjectID: "coord-round", AgentRoundID: "coord-round",
				AgentID: "lead-1", Status: protocol.ExecutionRuntimeNodeSucceeded,
				StartedAt: now, UpdatedAt: now,
				Metadata: map[string]any{"execution_lane": "coordination"},
			},
			{
				ID: "runtime-conversation-only", Kind: protocol.ExecutionRuntimeNodeAgent,
				SubjectID: "chat-round", AgentRoundID: "chat-round",
				AgentID: "observer-1", Status: protocol.ExecutionRuntimeNodeSucceeded,
				StartedAt: now, UpdatedAt: now,
			},
			{
				ID: "runtime-work-agent", Kind: protocol.ExecutionRuntimeNodeAgent,
				SubjectID: workRoundID, AgentRoundID: workRoundID,
				AgentID: "worker-1", Status: protocol.ExecutionRuntimeNodeRunning,
				StartedAt: now, UpdatedAt: now,
				Metadata: map[string]any{
					"execution_lane": "work",
					"work_item_id":   workID,
				},
			},
			{
				ID: toolID, Kind: protocol.ExecutionRuntimeNodeTool,
				SubjectID: "tool-managed", AgentRoundID: workRoundID,
				AgentID: "worker-1", Name: "search",
				Status:    protocol.ExecutionRuntimeNodeRunning,
				StartedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second),
			},
		},
		Edges: []protocol.ExecutionRuntimeEdgeRun{{
			ID: "runtime-work-invoke", SourceNodeID: "runtime-work-agent",
			TargetNodeID: toolID, Kind: protocol.ExecutionRuntimeEdgeInvoke,
			CreatedAt: now.Add(time.Second),
		}},
	})

	if len(view.Graph.Nodes) != 3 {
		t.Fatalf("managed graph nodes = %+v", view.Graph.Nodes)
	}
	for _, node := range view.Graph.Nodes {
		if node.ID == "runtime-conversation-only" || node.ID == "runtime-work-agent" ||
			node.ID == coordinatorID {
			t.Fatalf("unbound or duplicate runtime Agent leaked into managed graph: %+v", node)
		}
	}
	work := graphNodeByID(view.Graph.Nodes, workID)
	if work.AgentRoundID != workRoundID || work.LifecycleStatus != "running" {
		t.Fatalf("managed Work Item did not absorb exact runtime round: %+v", work)
	}
	coordinator := graphNodeByID(view.Graph.Nodes, coordinatorNodeID)
	if coordinator.AgentRoundID != "coord-round" || coordinator.LifecycleStatus != "succeeded" {
		t.Fatalf("stable coordinator node did not absorb its runtime round: %+v", coordinator)
	}
	if !hasExecutionGraphEdge(
		view.Graph.Edges,
		protocol.ExecutionGraphEdgeInvoke,
		workID,
		toolID,
	) || !hasExecutionGraphEdge(
		view.Graph.Edges,
		protocol.ExecutionGraphEdgeCoordination,
		coordinatorNodeID,
		workID,
	) {
		t.Fatalf("managed runtime graph edges = %+v", view.Graph.Edges)
	}
}

func TestAuditExecutionAlignmentRecordsOptionalGateWithoutRoutingExecution(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 3, 11, 0, 0, 0, time.UTC)
	snapshot := executionSnapshot()
	snapshot.Execution.RootRoundID = "round-1"
	snapshot.Execution.Objective = "Ship the verified report"
	snapshot.Execution.CompletionCriteria = []string{"report is verified"}
	repository := &runtimeGraphRepositoryFake{
		fakeRepository: &fakeRepository{snapshot: snapshot},
	}
	service := NewService(repository)
	service.now = func() time.Time { return now }
	actor := coordinatorActor()
	actor.ExecutionID = snapshot.Execution.ID
	actor.RootRoundID = "round-1"
	actor.RuntimeRoundID = "runtime-round-1"
	actor.AgentRoundID = "agent-round-1"

	result, err := service.AuditExecutionAlignment(
		context.Background(),
		actor,
		AuditExecutionAlignmentInput{
			ExecutionID:      snapshot.Execution.ID,
			SnapshotRevision: snapshot.Execution.Version,
			CommandID:        "tool-alignment-1",
			Report: protocol.ObjectiveAlignmentReport{
				Decision: protocol.ObjectiveAlignmentNotAligned,
				CriteriaResults: []protocol.ObjectiveAlignmentCriterionResult{{
					Criterion: "report is verified",
					Status:    protocol.ObjectiveAlignmentCriterionUnsatisfied,
					Gap:       "verification has not run",
				}},
				Summary: "The report still needs verification.",
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationApplied ||
		snapshot.Execution.Status != protocol.ExecutionStatusActive ||
		len(repository.nodes) != 2 || len(repository.edges) != 2 {
		t.Fatalf("result=%+v nodes=%+v edges=%+v", result, repository.nodes, repository.edges)
	}
	gate := repository.nodes[1]
	if gate.Kind != protocol.ExecutionRuntimeNodeGate ||
		gate.Name != "objective_alignment" ||
		gate.Metadata["decision"] != "not_aligned" {
		t.Fatalf("gate = %+v", gate)
	}
	if repository.edges[0].Kind != protocol.ExecutionRuntimeEdgeGuard ||
		repository.edges[1].Kind != protocol.ExecutionRuntimeEdgeLoopBack ||
		repository.edges[1].TargetNodeID != repository.nodes[0].ID {
		t.Fatalf("gate edges = %+v", repository.edges)
	}
	rootNodeID := repository.nodes[0].ID

	repository.graph = protocol.ExecutionRuntimeGraph{
		GraphID: repository.nodes[0].GraphID,
		Nodes:   repository.nodes,
		Edges:   repository.edges,
	}
	view, err := service.GetLatestView(
		context.Background(),
		snapshot.Execution.OwnerUserID,
		snapshot.Execution.SessionKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	projectedGate := graphNodeByID(view.Graph.Nodes, gate.ID)
	if projectedGate.Kind != protocol.ExecutionGraphNodeGate ||
		projectedGate.LifecycleStatus != "not_aligned" ||
		!hasExecutionGraphEdge(
			view.Graph.Edges,
			protocol.ExecutionGraphEdgeGuard,
			rootNodeID,
			gate.ID,
		) ||
		!hasExecutionGraphEdge(
			view.Graph.Edges,
			protocol.ExecutionGraphEdgeLoopBack,
			gate.ID,
			rootNodeID,
		) {
		t.Fatalf("projected graph = %+v", view.Graph)
	}
}

func TestAuditExecutionAlignmentRejectsIncompleteReportBeforeGraphWrite(t *testing.T) {
	t.Parallel()

	snapshot := executionSnapshot()
	snapshot.Execution.Objective = "Ship"
	snapshot.Execution.CompletionCriteria = []string{"tested"}
	repository := &runtimeGraphRepositoryFake{
		fakeRepository: &fakeRepository{snapshot: snapshot},
	}
	actor := coordinatorActor()
	actor.RootRoundID = "round-1"
	actor.RuntimeRoundID = "runtime-round-1"
	actor.AgentRoundID = "agent-round-1"
	result, err := NewService(repository).AuditExecutionAlignment(
		context.Background(),
		actor,
		AuditExecutionAlignmentInput{
			ExecutionID:      snapshot.Execution.ID,
			SnapshotRevision: snapshot.Execution.Version,
			CommandID:        "tool-alignment-invalid",
			Report: protocol.ObjectiveAlignmentReport{
				Decision: protocol.ObjectiveAlignmentAligned,
				Summary:  "looks done",
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationRejected || len(repository.nodes) != 0 || len(repository.edges) != 0 {
		t.Fatalf("result=%+v nodes=%+v edges=%+v", result, repository.nodes, repository.edges)
	}
}

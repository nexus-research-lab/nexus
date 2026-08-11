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
	artifacts      []protocol.ExecutionRuntimeArtifactRef
	reconciled     int
	finishedStatus protocol.ExecutionRuntimeNodeStatus
	graph          protocol.ExecutionRuntimeGraph
}

type runtimeGraphSubagentHistoryProviderFunc func(
	context.Context,
	string,
	string,
) ([]RuntimeGraphSubagentToolHistory, error)

func (provider runtimeGraphSubagentHistoryProviderFunc) ListRuntimeGraphSubagentToolHistory(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
) ([]RuntimeGraphSubagentToolHistory, error) {
	return provider(ctx, ownerUserID, sessionKey)
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

func (f *runtimeGraphRepositoryFake) UpsertRuntimeGraphArtifact(
	_ context.Context,
	item protocol.ExecutionRuntimeArtifactRef,
) error {
	f.artifacts = append(f.artifacts, item)
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
				"content": `{"message":"Plan Document items must contain at least one complete Work Item","outcome":"rejected","reason_code":"plan_items_empty"}`,
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
		tool.ErrorCode != "plan_items_empty" ||
		tool.ErrorSummary != "Plan Document items must contain at least one complete Work Item" ||
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

func TestRuntimeGraphDoesNotPersistUnstartedProgressFacetAsTool(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 6, 9, 0, 0, 0, time.UTC)
	repository := &runtimeGraphRepositoryFake{fakeRepository: &fakeRepository{}}
	service := NewService(repository)
	service.now = func() time.Time { return now }
	actor := ActorContext{
		OwnerUserID: "owner-1", SessionKey: "session-1", AgentID: "agent-1",
		RootRoundID: "round-1", RuntimeRoundID: "round-1", AgentRoundID: "agent-round-1",
	}
	message := sdkprotocol.ReceivedMessage{
		UUID: "progress-only-1",
		RuntimeLifecycle: []sdkprotocol.RuntimeLifecycleEvent{{
			EventID:   "runtime:tool:progress:agent_msg_1:running",
			NodeKind:  sdkprotocol.RuntimeLifecycleNodeTool,
			Phase:     sdkprotocol.RuntimeLifecycleProgress,
			SubjectID: "agent_msg_1", ParentSubjectID: "spawn-tool-1",
			Name: "Agent", Status: "running",
		}},
	}
	if err := service.ObserveRuntimeMessage(context.Background(), actor, message); err != nil {
		t.Fatal(err)
	}
	if len(repository.nodes) != 1 || repository.nodes[0].Kind != protocol.ExecutionRuntimeNodeAgent ||
		len(repository.edges) != 0 {
		t.Fatalf("progress facet invented a Tool node: nodes=%+v edges=%+v", repository.nodes, repository.edges)
	}
}

func TestRuntimeGraphRecoversSubagentToolLifecycleFromAttachment(t *testing.T) {
	t.Parallel()

	message, err := sdkprotocol.DecodeMessage(map[string]any{
		"type": "attachment",
		"uuid": "subagent-result-1",
		"attachment": map[string]any{
			"type": "structured_output",
			"data": map[string]any{
				"toolUseId": "spawn-tool-1",
				"messages": []any{
					map[string]any{
						"type": "assistant", "uuid": "child-assistant-1",
						"message": map[string]any{
							"role": "assistant",
							"content": []any{map[string]any{
								"type": "tool_use", "id": "child-tool-1", "name": "Read",
								"input": map[string]any{"file_path": "/private/input"},
							}},
						},
					},
					map[string]any{
						"type": "user", "uuid": "child-result-1",
						"message": map[string]any{
							"role": "user",
							"content": []any{map[string]any{
								"type": "tool_result", "tool_use_id": "child-tool-1",
								"content": "private output",
							}},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	events := runtimeGraphLifecycleEvents(message)
	if len(events) != 2 ||
		events[0].NodeKind != sdkprotocol.RuntimeLifecycleNodeTool ||
		events[0].Phase != sdkprotocol.RuntimeLifecycleStarted ||
		events[0].SubjectID != "child-tool-1" ||
		events[0].ParentSubjectID != "spawn-tool-1" ||
		events[0].Name != "Read" ||
		events[1].Phase != sdkprotocol.RuntimeLifecycleFinished ||
		events[1].ParentSubjectID != "spawn-tool-1" ||
		events[1].Status != "succeeded" {
		t.Fatalf("subagent Tool lifecycle = %+v", events)
	}
	if events[0].Metadata != nil {
		t.Fatalf("subagent Tool input leaked into metadata: %+v", events[0].Metadata)
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

func TestRuntimeGraphViewBindsLaunchToolsAndChildrenToSiblingSubagents(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 6, 8, 10, 0, 0, time.UTC)
	view := &protocol.ExecutionView{
		WorkItems: []protocol.ExecutionWorkItemView{{
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
					Status:          protocol.WorkAttemptStatusInterrupted,
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
		}},
	}
	view.Graph = projectExecutionGraphView(view.WorkItems)

	rootRunID := "runtime-agent-root"
	firstLaunchRunID := "runtime-launch-first"
	secondLaunchRunID := "runtime-launch-second"
	firstToolRunID := "runtime-tool-first"
	secondToolRunID := "runtime-tool-second"
	mergeExecutionRuntimeGraph(view, protocol.ExecutionRuntimeGraph{
		Nodes: []protocol.ExecutionRuntimeNodeRun{
			{
				ID: rootRunID, Kind: protocol.ExecutionRuntimeNodeAgent,
				SubjectID: "agent-round-1", AgentRoundID: "agent-round-1",
				AgentID: "parent-agent", Status: protocol.ExecutionRuntimeNodeRunning,
				StartedAt: now, UpdatedAt: now,
			},
			{
				ID: firstLaunchRunID, Kind: protocol.ExecutionRuntimeNodeTool,
				SubjectID: "spawn-first", AgentRoundID: "agent-round-1",
				AgentID: "parent-agent", Name: "Agent",
				Status:    protocol.ExecutionRuntimeNodeSucceeded,
				StartedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second),
			},
			{
				ID: firstToolRunID, Kind: protocol.ExecutionRuntimeNodeTool,
				SubjectID: "read-first", ParentSubjectID: "spawn-first",
				AgentRoundID: "agent-round-1", AgentID: "parent-agent", Name: "Read",
				Status:    protocol.ExecutionRuntimeNodeRunning,
				StartedAt: now.Add(2 * time.Second), UpdatedAt: now.Add(2 * time.Second),
			},
			{
				ID: secondLaunchRunID, Kind: protocol.ExecutionRuntimeNodeTool,
				SubjectID: "spawn-second", AgentRoundID: "agent-round-1",
				AgentID: "parent-agent", Name: "Agent",
				Status:    protocol.ExecutionRuntimeNodeSucceeded,
				StartedAt: now.Add(3 * time.Second), UpdatedAt: now.Add(3 * time.Second),
			},
			{
				ID: secondToolRunID, Kind: protocol.ExecutionRuntimeNodeTool,
				SubjectID: "read-second", ParentSubjectID: "spawn-second",
				AgentRoundID: "agent-round-1", AgentID: "parent-agent", Name: "Read",
				Status:    protocol.ExecutionRuntimeNodeSucceeded,
				StartedAt: now.Add(4 * time.Second), UpdatedAt: now.Add(4 * time.Second),
			},
		},
		Edges: []protocol.ExecutionRuntimeEdgeRun{
			{
				ID: "edge-root-first", SourceNodeID: rootRunID,
				TargetNodeID: firstLaunchRunID, Kind: protocol.ExecutionRuntimeEdgeInvoke,
				CreatedAt: now.Add(time.Second),
			},
			{
				ID: "edge-stale-root-first-tool", SourceNodeID: rootRunID,
				TargetNodeID: firstToolRunID, Kind: protocol.ExecutionRuntimeEdgeInvoke,
				CreatedAt: now.Add(2 * time.Second),
			},
			{
				ID: "edge-root-second", SourceNodeID: rootRunID,
				TargetNodeID: secondLaunchRunID, Kind: protocol.ExecutionRuntimeEdgeInvoke,
				CreatedAt: now.Add(3 * time.Second),
			},
			{
				ID: "edge-second-tool", SourceNodeID: secondLaunchRunID,
				TargetNodeID: secondToolRunID, Kind: protocol.ExecutionRuntimeEdgeInvoke,
				CreatedAt: now.Add(4 * time.Second),
			},
		},
	})

	if len(view.Graph.Nodes) != 5 || len(view.Graph.Edges) != 4 {
		t.Fatalf("runtime launch Tools were not merged into sibling Subagents: %+v", view.Graph)
	}
	if graphNodeByID(view.Graph.Nodes, firstLaunchRunID).ID != "" ||
		graphNodeByID(view.Graph.Nodes, secondLaunchRunID).ID != "" {
		t.Fatalf("launch Tool leaked as a duplicate Subagent node: %+v", view.Graph.Nodes)
	}
	firstSubagent := graphNodeByID(view.Graph.Nodes, "attempt-child-first")
	secondSubagent := graphNodeByID(view.Graph.Nodes, "attempt-child-second")
	firstTool := graphNodeByID(view.Graph.Nodes, firstToolRunID)
	secondTool := graphNodeByID(view.Graph.Nodes, secondToolRunID)
	if firstSubagent.ParentNodeID != "work-a" || secondSubagent.ParentNodeID != "work-a" ||
		firstSubagent.AgentID != "" || secondSubagent.AgentID != "" ||
		firstSubagent.RunStatus != protocol.WorkAttemptStatusInterrupted ||
		firstSubagent.LifecycleStatus != "" ||
		firstTool.ParentNodeID != firstSubagent.ID ||
		secondTool.ParentNodeID != secondSubagent.ID ||
		firstTool.Visibility != protocol.ExecutionGraphNodeNested ||
		secondTool.Visibility != protocol.ExecutionGraphNodeNested {
		t.Fatalf(
			"runtime child ownership is incorrect: first=%+v first_tool=%+v second=%+v second_tool=%+v",
			firstSubagent,
			firstTool,
			secondSubagent,
			secondTool,
		)
	}
	if !hasExecutionGraphEdge(
		view.Graph.Edges,
		protocol.ExecutionGraphEdgeInvoke,
		firstSubagent.ID,
		firstTool.ID,
	) || !hasExecutionGraphEdge(
		view.Graph.Edges,
		protocol.ExecutionGraphEdgeInvoke,
		secondSubagent.ID,
		secondTool.ID,
	) || hasExecutionGraphEdge(
		view.Graph.Edges,
		protocol.ExecutionGraphEdgeInvoke,
		"work-a",
		firstTool.ID,
	) {
		t.Fatalf("runtime Tool edges escaped their exact Subagent: %+v", view.Graph.Edges)
	}
}

func TestRuntimeGraphRecoversHistoricalSubagentToolsWithoutPersistingContent(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 6, 11, 0, 0, 0, time.UTC)
	service := NewService(&runtimeGraphRepositoryFake{fakeRepository: &fakeRepository{}})
	service.SetRuntimeGraphSubagentToolHistoryProvider(runtimeGraphSubagentHistoryProviderFunc(
		func(_ context.Context, ownerUserID string, sessionKey string) ([]RuntimeGraphSubagentToolHistory, error) {
			if ownerUserID != "owner-1" || sessionKey != "session-1" {
				t.Fatalf("history scope = %q/%q", ownerUserID, sessionKey)
			}
			return []RuntimeGraphSubagentToolHistory{{
				ParentToolUseID: "spawn-tool-1",
				TaskID:          "task-1",
				AgentID:         "child-1",
				ToolUseID:       "read-1",
				Name:            "Read",
				Status:          "succeeded",
				StartedAt:       now.Add(time.Second).UnixMilli(),
				FinishedAt:      now.Add(2 * time.Second).UnixMilli(),
			}}, nil
		},
	))
	graph := service.mergeRuntimeGraphSubagentToolHistory(
		context.Background(),
		"owner-1",
		"session-1",
		protocol.ExecutionRuntimeGraph{
			GraphID:   "graph-1",
			NodeTotal: 1,
			Nodes: []protocol.ExecutionRuntimeNodeRun{{
				ID:             "launch-tool-1",
				GraphID:        "graph-1",
				OwnerUserID:    "owner-1",
				SessionKey:     "session-1",
				ExecutionID:    "execution-1",
				Kind:           protocol.ExecutionRuntimeNodeTool,
				SubjectID:      "spawn-tool-1",
				RootRoundID:    "round-1",
				RuntimeRoundID: "round-1",
				AgentRoundID:   "agent-round-1",
				AgentID:        "parent-1",
				Name:           "Agent",
				Status:         protocol.ExecutionRuntimeNodeSucceeded,
				StartedAt:      now,
				UpdatedAt:      now,
			}},
		},
	)
	if len(graph.Nodes) != 2 || len(graph.Edges) != 1 || graph.NodeTotal != 2 || graph.EdgeTotal != 1 {
		t.Fatalf("history graph = %+v", graph)
	}
	tool := graph.Nodes[1]
	if tool.Kind != protocol.ExecutionRuntimeNodeTool || tool.SubjectID != "read-1" ||
		tool.ParentSubjectID != "spawn-tool-1" || tool.Name != "Read" ||
		tool.Status != protocol.ExecutionRuntimeNodeSucceeded || tool.ResultSummary != "" ||
		graph.Edges[0].SourceNodeID != "launch-tool-1" || graph.Edges[0].TargetNodeID != tool.ID {
		t.Fatalf("recovered Tool lifecycle = %+v edge=%+v", tool, graph.Edges[0])
	}
}

func TestRuntimeGraphReviewEdgesUseSuccessfulSubmissionAsControlAnchor(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 6, 11, 30, 0, 0, time.UTC)
	view := &protocol.ExecutionView{Graph: protocol.ExecutionGraphView{
		Nodes: []protocol.ExecutionGraphNodeView{
			{ID: "work-1", Kind: protocol.ExecutionGraphNodeAgent},
			{
				ID: "submit-1", Kind: protocol.ExecutionGraphNodeTool,
				ParentNodeID: "work-1", Name: "mcp__nexus_execution__submit_work",
				LifecycleStatus: "succeeded", StartedAt: &now,
			},
			{ID: "gate-1", Kind: protocol.ExecutionGraphNodeGate},
		},
		Edges: []protocol.ExecutionGraphEdgeView{
			{ID: "review-1", Kind: protocol.ExecutionGraphEdgeReview, SourceNodeID: "work-1", TargetNodeID: "gate-1"},
			{ID: "return-1", Kind: protocol.ExecutionGraphEdgeLoopBack, SourceNodeID: "gate-1", TargetNodeID: "work-1"},
		},
	}}
	reanchorExecutionReviewEdgesToSubmission(view, map[string]int{
		"work-1": 0, "submit-1": 1, "gate-1": 2,
	})
	if view.Graph.Edges[0].SourceNodeID != "submit-1" ||
		view.Graph.Edges[1].TargetNodeID != "submit-1" {
		t.Fatalf("review control anchor = %+v", view.Graph.Edges)
	}
}

func TestRuntimeGraphViewFiltersHistoricalProgressFacet(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 6, 9, 10, 0, 0, time.UTC)
	view := &protocol.ExecutionView{}
	mergeExecutionRuntimeGraph(view, protocol.ExecutionRuntimeGraph{
		NodeTotal: 2,
		EdgeTotal: 1,
		Nodes: []protocol.ExecutionRuntimeNodeRun{
			{
				ID: "runtime-agent", Kind: protocol.ExecutionRuntimeNodeAgent,
				SubjectID: "agent-round-1", AgentRoundID: "agent-round-1",
				Status:    protocol.ExecutionRuntimeNodeSucceeded,
				StartedAt: now, UpdatedAt: now,
			},
			{
				ID: "runtime-progress", Kind: protocol.ExecutionRuntimeNodeTool,
				SubjectID: "agent_msg_1", ParentSubjectID: "spawn-tool-1",
				AgentRoundID: "agent-round-1", Name: "Agent",
				Status:    protocol.ExecutionRuntimeNodeInterrupted,
				StartedAt: now.Add(time.Second), UpdatedAt: now.Add(2 * time.Second),
				Metadata: map[string]any{
					"bridge_event_id": "runtime:tool:progress:agent_msg_1:running",
				},
			},
		},
		Edges: []protocol.ExecutionRuntimeEdgeRun{{
			ID: "progress-edge", SourceNodeID: "runtime-agent",
			TargetNodeID: "runtime-progress", Kind: protocol.ExecutionRuntimeEdgeInvoke,
			CreatedAt: now.Add(time.Second),
		}},
	})

	if len(view.Graph.Nodes) != 1 || view.Graph.Nodes[0].ID != "runtime-agent" ||
		len(view.Graph.Edges) != 0 || view.Graph.RuntimeNodeTotal != 2 {
		t.Fatalf("historical progress facet leaked into graph: %+v", view.Graph)
	}
}

func TestRuntimeGraphToolActionVisibilityUsesUserObservableSemantics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		toolName string
		kind     protocol.ExecutionRuntimeNodeKind
		visible  bool
	}{
		{name: "web search", toolName: "WebSearch", kind: protocol.ExecutionRuntimeNodeTool, visible: true},
		{name: "wrapped web fetch", toolName: "browser.web-fetch", kind: protocol.ExecutionRuntimeNodeTool, visible: true},
		{name: "shell execution", toolName: "Bash", kind: protocol.ExecutionRuntimeNodeTool, visible: true},
		{name: "workspace mutation", toolName: "Edit", kind: protocol.ExecutionRuntimeNodeTool, visible: true},
		{name: "external mutation", toolName: "mcp__slack__send_message", kind: protocol.ExecutionRuntimeNodeTool, visible: true},
		{name: "browser action", toolName: "mcp__browser__navigate", kind: protocol.ExecutionRuntimeNodeTool, visible: true},
		{name: "local read", toolName: "Read", kind: protocol.ExecutionRuntimeNodeTool, visible: false},
		{name: "local search", toolName: "Grep", kind: protocol.ExecutionRuntimeNodeTool, visible: false},
		{name: "external query", toolName: "mcp__github__list_issues", kind: protocol.ExecutionRuntimeNodeTool, visible: true},
		{name: "workspace mcp read", toolName: "mcp__filesystem__read_file", kind: protocol.ExecutionRuntimeNodeTool, visible: false},
		{name: "unknown local query", toolName: "list_issues", kind: protocol.ExecutionRuntimeNodeTool, visible: false},
		{name: "tool discovery", toolName: "ToolSearch", kind: protocol.ExecutionRuntimeNodeTool, visible: false},
		{name: "submission control anchor", toolName: "mcp__nexus_execution__submit_work", kind: protocol.ExecutionRuntimeNodeTool, visible: true},
		{name: "represented goal mutation", toolName: "mcp__nexus_goal__update_goal", kind: protocol.ExecutionRuntimeNodeTool, visible: false},
		{name: "non tool", toolName: "WebFetch", kind: protocol.ExecutionRuntimeNodeAgent, visible: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			item := protocol.ExecutionRuntimeNodeRun{Kind: test.kind, Name: test.toolName}
			if got := runtimeGraphToolActionVisible(item); got != test.visible {
				t.Fatalf("runtimeGraphToolActionVisible(%q) = %t, want %t", test.toolName, got, test.visible)
			}
		})
	}
}

func TestRuntimeGraphProjectsObservableActionsAndKeepsSupportingReadsInDetail(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC)
	nodes := []protocol.ExecutionRuntimeNodeRun{
		{
			ID: "runtime-web-fetch", Kind: protocol.ExecutionRuntimeNodeTool,
			SubjectID: "tool-web-fetch", Name: "WebFetch",
			Status: protocol.ExecutionRuntimeNodeSucceeded, StartedAt: now, UpdatedAt: now,
		},
		{
			ID: "runtime-read", Kind: protocol.ExecutionRuntimeNodeTool,
			SubjectID: "tool-read", Name: "Read",
			Status: protocol.ExecutionRuntimeNodeSucceeded, StartedAt: now, UpdatedAt: now,
		},
	}
	promoted := runtimeGraphPromotedNodeIDs(protocol.ExecutionRuntimeGraph{Nodes: nodes})
	for index, item := range nodes {
		_, isPromoted := promoted[item.ID]
		projected := projectRuntimeGraphNode(item, index, isPromoted)
		want := protocol.ExecutionGraphNodeDetail
		if item.Name == "WebFetch" {
			want = protocol.ExecutionGraphNodeNested
		}
		if projected.Visibility != want {
			t.Fatalf("%s visibility = %s, want %s", item.Name, projected.Visibility, want)
		}
	}
}

func TestRuntimeGraphSubagentRepresentativeSlotsKeepRecoveryVisibleAndBounded(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 6, 10, 30, 0, 0, time.UTC)
	graph := protocol.ExecutionRuntimeGraph{Nodes: []protocol.ExecutionRuntimeNodeRun{
		{
			ID: "runtime-subagent", Kind: protocol.ExecutionRuntimeNodeSubagent,
			SubjectID: "spawn-tool", Status: protocol.ExecutionRuntimeNodeRunning,
			StartedAt: now, UpdatedAt: now,
		},
		{
			ID: "read-failed", Kind: protocol.ExecutionRuntimeNodeTool,
			ParentSubjectID: "spawn-tool", Name: "Read",
			Status:    protocol.ExecutionRuntimeNodeFailed,
			StartedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second),
		},
		{
			ID: "read-early", Kind: protocol.ExecutionRuntimeNodeTool,
			ParentSubjectID: "spawn-tool", Name: "Read",
			Status:    protocol.ExecutionRuntimeNodeSucceeded,
			StartedAt: now.Add(2 * time.Second), UpdatedAt: now.Add(2 * time.Second),
		},
		{
			ID: "read-recovered", Kind: protocol.ExecutionRuntimeNodeTool,
			ParentSubjectID: "spawn-tool", Name: "Read",
			Status:    protocol.ExecutionRuntimeNodeSucceeded,
			StartedAt: now.Add(3 * time.Second), UpdatedAt: now.Add(3 * time.Second),
		},
		{
			ID: "grep-latest", Kind: protocol.ExecutionRuntimeNodeTool,
			ParentSubjectID: "spawn-tool", Name: "Grep",
			Status:    protocol.ExecutionRuntimeNodeSucceeded,
			StartedAt: now.Add(4 * time.Second), UpdatedAt: now.Add(4 * time.Second),
		},
	}}
	promoted := runtimeGraphPromotedNodeIDs(graph)

	wantVisibility := map[string]protocol.ExecutionGraphNodeVisibility{
		"read-failed":    protocol.ExecutionGraphNodeNested,
		"read-early":     protocol.ExecutionGraphNodeDetail,
		"read-recovered": protocol.ExecutionGraphNodeNested,
		"grep-latest":    protocol.ExecutionGraphNodeNested,
	}
	for index, node := range graph.Nodes[1:] {
		_, isPromoted := promoted[node.ID]
		projected := projectRuntimeGraphNode(node, index+1, isPromoted)
		if projected.Visibility != wantVisibility[node.ID] {
			t.Fatalf("%s visibility = %s, want %s", node.ID, projected.Visibility, wantVisibility[node.ID])
		}
	}
}

func TestGetLatestViewDoesNotExposePlanlessRuntimeGraphAsWorkGraph(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	repository := &runtimeGraphRepositoryFake{
		fakeRepository: &fakeRepository{},
		graph: protocol.ExecutionRuntimeGraph{
			GraphID:        "round:round-1",
			NodeTotal:      40,
			EdgeTotal:      50,
			NodesTruncated: true,
			EdgesTruncated: true,
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
				SourceNodeID: "runtime-agent-1", TargetNodeID: "runtime-tool-1",
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
	if view != nil {
		t.Fatalf("planless runtime graph leaked through WorkGraph read surface: %+v", view)
	}
}

func TestRuntimeGraphRetryKeepsSucceededToolVisibleWithoutToolNameRouting(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 5, 9, 0, 0, 0, time.UTC)
	rootID := "runtime-agent-retry"
	failedID := "runtime-tool-failed"
	retriedID := "runtime-tool-retried"
	view := &protocol.ExecutionView{}
	mergeExecutionRuntimeGraph(view, protocol.ExecutionRuntimeGraph{
		GraphID: "round:retry",
		Nodes: []protocol.ExecutionRuntimeNodeRun{
			{
				ID: rootID, Kind: protocol.ExecutionRuntimeNodeAgent,
				SubjectID: "agent-round-retry", AgentRoundID: "agent-round-retry",
				AgentID: "agent-1", Status: protocol.ExecutionRuntimeNodeSucceeded,
				StartedAt: now, UpdatedAt: now,
			},
			{
				ID: failedID, Kind: protocol.ExecutionRuntimeNodeTool,
				SubjectID: "lookup-1", ParentSubjectID: "agent-round-retry",
				AgentRoundID: "agent-round-retry", AgentID: "agent-1",
				Name: "future_web_lookup", Status: protocol.ExecutionRuntimeNodeFailed,
				StartedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second),
			},
			{
				ID: retriedID, Kind: protocol.ExecutionRuntimeNodeTool,
				SubjectID: "lookup-2", ParentSubjectID: "agent-round-retry",
				AgentRoundID: "agent-round-retry", AgentID: "agent-1",
				Name: "future_web_lookup", Status: protocol.ExecutionRuntimeNodeSucceeded,
				StartedAt: now.Add(2 * time.Second), UpdatedAt: now.Add(2 * time.Second),
			},
		},
		Edges: []protocol.ExecutionRuntimeEdgeRun{
			{ID: "invoke-failed", SourceNodeID: rootID, TargetNodeID: failedID, Kind: protocol.ExecutionRuntimeEdgeInvoke, CreatedAt: now},
			{ID: "loop-failed", SourceNodeID: failedID, TargetNodeID: rootID, Kind: protocol.ExecutionRuntimeEdgeLoopBack, CreatedAt: now.Add(time.Second)},
			{ID: "invoke-retried", SourceNodeID: rootID, TargetNodeID: retriedID, Kind: protocol.ExecutionRuntimeEdgeInvoke, CreatedAt: now.Add(2 * time.Second)},
			{ID: "retry-observed", SourceNodeID: failedID, TargetNodeID: retriedID, Kind: protocol.ExecutionRuntimeEdgeRetry, CreatedAt: now.Add(2 * time.Second)},
		},
	})

	failed := graphNodeByID(view.Graph.Nodes, failedID)
	retried := graphNodeByID(view.Graph.Nodes, retriedID)
	if failed.ID == retried.ID ||
		len(failed.Runs) != 1 || len(retried.Runs) != 1 ||
		failed.LifecycleStatus != string(protocol.ExecutionRuntimeNodeFailed) ||
		retried.LifecycleStatus != string(protocol.ExecutionRuntimeNodeSucceeded) {
		t.Fatalf("retry NodeRuns were folded instead of preserved: failed=%+v retried=%+v", failed, retried)
	}
	if retried.Visibility != protocol.ExecutionGraphNodeNested {
		t.Fatalf("succeeded retry target was hidden after completion: %+v", retried)
	}
	foundRetry := false
	for _, edge := range view.Graph.Edges {
		if edge.Kind == protocol.ExecutionGraphEdgeRetry &&
			edge.SourceNodeID == failedID && edge.TargetNodeID == retriedID {
			foundRetry = true
			break
		}
	}
	if !foundRetry {
		t.Fatalf("agent-chosen retry relation missing: %+v", view.Graph.Edges)
	}
}

func TestRuntimeGraphKeepsUnlinkedFailureAndRecoveryAsSeparateVisibleNodes(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 6, 15, 0, 0, 0, time.UTC)
	rootID := "runtime-agent-unlinked-recovery"
	failedID := "runtime-read-failed"
	succeededID := "runtime-read-succeeded"
	view := &protocol.ExecutionView{}
	mergeExecutionRuntimeGraph(view, protocol.ExecutionRuntimeGraph{
		GraphID: "round:unlinked-recovery",
		Nodes: []protocol.ExecutionRuntimeNodeRun{
			{
				ID: rootID, Kind: protocol.ExecutionRuntimeNodeAgent,
				SubjectID: "agent-round-unlinked", AgentRoundID: "agent-round-unlinked",
				AgentID: "agent-1", Status: protocol.ExecutionRuntimeNodeSucceeded,
				StartedAt: now, UpdatedAt: now,
			},
			{
				ID: failedID, Kind: protocol.ExecutionRuntimeNodeTool,
				SubjectID: "read-1", ParentSubjectID: "agent-round-unlinked",
				AgentRoundID: "agent-round-unlinked", AgentID: "agent-1",
				Name: "Read", Status: protocol.ExecutionRuntimeNodeFailed,
				StartedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second),
			},
			{
				ID: succeededID, Kind: protocol.ExecutionRuntimeNodeTool,
				SubjectID: "read-2", ParentSubjectID: "agent-round-unlinked",
				AgentRoundID: "agent-round-unlinked", AgentID: "agent-1",
				Name: "Read", Status: protocol.ExecutionRuntimeNodeSucceeded,
				StartedAt: now.Add(2 * time.Second), UpdatedAt: now.Add(2 * time.Second),
			},
		},
		Edges: []protocol.ExecutionRuntimeEdgeRun{
			{ID: "invoke-failed", SourceNodeID: rootID, TargetNodeID: failedID, Kind: protocol.ExecutionRuntimeEdgeInvoke, CreatedAt: now.Add(time.Second)},
			{ID: "invoke-succeeded", SourceNodeID: rootID, TargetNodeID: succeededID, Kind: protocol.ExecutionRuntimeEdgeInvoke, CreatedAt: now.Add(2 * time.Second)},
		},
	})

	failed := graphNodeByID(view.Graph.Nodes, failedID)
	succeeded := graphNodeByID(view.Graph.Nodes, succeededID)
	if failed.ID == succeeded.ID || len(failed.Runs) != 1 || len(succeeded.Runs) != 1 {
		t.Fatalf("unlinked NodeRuns were folded: failed=%+v succeeded=%+v", failed, succeeded)
	}
	if failed.Visibility != protocol.ExecutionGraphNodeNested ||
		succeeded.Visibility != protocol.ExecutionGraphNodeNested {
		t.Fatalf("failure and latest recovery must both stay visible: failed=%+v succeeded=%+v", failed, succeeded)
	}
	if failed.LifecycleStatus != string(protocol.ExecutionRuntimeNodeFailed) ||
		succeeded.LifecycleStatus != string(protocol.ExecutionRuntimeNodeSucceeded) {
		t.Fatalf("independent NodeRun status was lost: failed=%+v succeeded=%+v", failed, succeeded)
	}
	for _, edge := range view.Graph.Edges {
		if edge.Kind == protocol.ExecutionGraphEdgeRetry {
			t.Fatalf("unlinked calls must not invent a retry edge: %+v", view.Graph.Edges)
		}
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

func TestRuntimeGraphViewKeepsEveryManagedAgentRun(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 4, 10, 30, 0, 0, time.UTC)
	view := &protocol.ExecutionView{
		ID:        "execution-history",
		WorkItems: []protocol.ExecutionWorkItemView{{ID: "work-history"}},
		Graph: protocol.ExecutionGraphView{Nodes: []protocol.ExecutionGraphNodeView{{
			ID: "work-history", Kind: protocol.ExecutionGraphNodeAgent,
			Visibility: protocol.ExecutionGraphNodePrimary,
			WorkItemID: "work-history", AgentID: "worker-1",
		}}},
	}
	firstFinished := now.Add(time.Second)
	secondFinished := now.Add(3 * time.Second)
	mergeExecutionRuntimeGraph(view, protocol.ExecutionRuntimeGraph{Nodes: []protocol.ExecutionRuntimeNodeRun{
		{
			ID: "runtime-agent-first", Kind: protocol.ExecutionRuntimeNodeAgent,
			SubjectID: "round-first", AgentRoundID: "round-first", AgentID: "worker-1",
			Status: protocol.ExecutionRuntimeNodeFailed, Failed: true,
			ErrorSummary: "Provider disconnected", StartedAt: now, UpdatedAt: firstFinished,
			FinishedAt: &firstFinished,
			Metadata: map[string]any{
				"execution_lane": "work",
				"work_item_id":   "work-history",
			},
		},
		{
			ID: "runtime-agent-second", Kind: protocol.ExecutionRuntimeNodeAgent,
			SubjectID: "round-second", AgentRoundID: "round-second", AgentID: "worker-1",
			Status: protocol.ExecutionRuntimeNodeSucceeded, ResultSummary: "Recovered",
			StartedAt: now.Add(2 * time.Second), UpdatedAt: secondFinished,
			FinishedAt: &secondFinished,
			Metadata: map[string]any{
				"execution_lane": "work",
				"work_item_id":   "work-history",
			},
		},
	}})

	node := graphNodeByID(view.Graph.Nodes, "work-history")
	if len(node.Runs) != 2 ||
		node.Runs[0].RuntimeNodeID != "runtime-agent-first" ||
		node.Runs[0].ErrorSummary != "Provider disconnected" ||
		node.Runs[1].RuntimeNodeID != "runtime-agent-second" ||
		node.Runs[1].ResultSummary != "Recovered" ||
		node.LifecycleStatus != string(protocol.ExecutionRuntimeNodeSucceeded) {
		t.Fatalf("managed NodeRun history = %+v", node)
	}
}

func TestRuntimeGraphArtifactsPersistBeforeToolRunByExactIdentity(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 4, 11, 0, 0, 0, time.UTC)
	repository := &runtimeGraphRepositoryFake{
		fakeRepository: &fakeRepository{},
	}
	service := NewService(repository)
	service.now = func() time.Time { return now.Add(time.Second) }
	actor := ActorContext{
		OwnerUserID: "owner-1", SessionKey: "session-1", AgentID: "agent-1",
		RootRoundID: "round-1", RuntimeRoundID: "round-1", AgentRoundID: "agent-round-1",
	}
	err := service.ObserveRuntimeArtifacts(context.Background(), actor, protocol.Message{
		"content": []any{
			map[string]any{
				"type": protocol.ContentBlockTypeWorkspaceFileArtifact,
				"id":   "workspace_file:tool-artifact:reports/result.md",
				"path": "reports/result.md", "display_path": "reports/result.md",
				"workspace_agent_id": "agent-1", "source_tool_use_id": "tool-artifact",
			},
			map[string]any{
				"type": protocol.ContentBlockTypeWorkspaceFileArtifact,
				"path": "../outside.txt", "source_tool_use_id": "tool-artifact",
			},
			map[string]any{
				"type": protocol.ContentBlockTypeWorkspaceFileArtifact,
				"path": "reports/unknown.md", "source_tool_use_id": "unknown-tool",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(repository.nodes) != 0 {
		t.Fatalf("artifact observation must not rewrite runtime nodes: %+v", repository.nodes)
	}
	if len(repository.artifacts) != 2 {
		t.Fatalf("durable artifact refs = %+v", repository.artifacts)
	}
	byToolUseID := make(map[string]protocol.ExecutionRuntimeArtifactRef, len(repository.artifacts))
	for _, ref := range repository.artifacts {
		byToolUseID[ref.ToolUseID] = ref
	}
	ref := byToolUseID["tool-artifact"]
	if ref.ID == "" || ref.GraphID != "round:round-1" ||
		ref.AgentRoundID != "agent-round-1" || ref.Artifact.Path != "reports/result.md" {
		t.Fatalf("exact runtime artifact ref = %+v", ref)
	}
	if unknown := byToolUseID["unknown-tool"]; unknown.ID == "" ||
		unknown.Artifact.Path != "reports/unknown.md" {
		t.Fatalf("artifact arriving before its Tool NodeRun was dropped: %+v", unknown)
	}
	projected := projectRuntimeGraphNode(protocol.ExecutionRuntimeNodeRun{
		ID: "runtime-tool-artifact", Kind: protocol.ExecutionRuntimeNodeTool,
		SubjectID: "tool-artifact", AgentRoundID: "agent-round-1",
		Status:    protocol.ExecutionRuntimeNodeSucceeded,
		StartedAt: now, UpdatedAt: now,
		Artifacts: []protocol.WorkspaceFileArtifactBlock{ref.Artifact},
	}, 0, true)
	if projected.Visibility != protocol.ExecutionGraphNodeNested ||
		len(projected.Runs) != 1 || len(projected.Runs[0].Artifacts) != 1 {
		t.Fatalf("projected runtime artifacts = %+v", projected.Runs)
	}
}

func TestRuntimeGraphReadsProviderNeutralToolResultShapes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		content any
		want    string
	}{
		{name: "plain text", content: "Found the page", want: "Found the page"},
		{name: "content blocks", content: []any{
			map[string]any{"type": "text", "text": "Found through MCP"},
		}, want: "Found through MCP"},
		{name: "structured result", content: map[string]any{
			"result": map[string]any{"message": "Found through server tool"},
		}, want: "Found through server tool"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			message, err := sdkprotocol.DecodeMessage(map[string]any{
				"type": "user", "uuid": "result-" + tt.name,
				"message": map[string]any{
					"role": "user",
					"content": []any{map[string]any{
						"type": "tool_result", "tool_use_id": "tool-provider",
						"content": tt.content,
					}},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			evidence := runtimeGraphEvidenceForEvent(message, sdkprotocol.RuntimeLifecycleEvent{
				NodeKind:  sdkprotocol.RuntimeLifecycleNodeTool,
				Phase:     sdkprotocol.RuntimeLifecycleFinished,
				SubjectID: "tool-provider",
			})
			if evidence.resultSummary != tt.want {
				t.Fatalf("provider-neutral result summary = %q, want %q", evidence.resultSummary, tt.want)
			}
		})
	}
}

func TestAuditExecutionAlignmentRecordsOptionalGateWithoutRoutingExecution(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 3, 11, 0, 0, 0, time.UTC)
	// Public WorkGraph reads are managed-only. Give the audited Execution a
	// real Plan/Work Item instead of relying on the removed planless view.
	snapshot := assignedExecutionSnapshot()
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
		projectedGate.Visibility != protocol.ExecutionGraphNodeNested ||
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

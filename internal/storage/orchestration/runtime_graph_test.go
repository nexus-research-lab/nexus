package orchestration

import (
	"context"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRepositoryRuntimeGraphIsIdempotentAndClosesOrphanedChildren(t *testing.T) {
	t.Parallel()

	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	root := protocol.ExecutionRuntimeNodeRun{
		ID: "runtime-root-1", GraphID: "round:round-1",
		OwnerUserID: "owner-1", SessionKey: "session-1",
		Kind: protocol.ExecutionRuntimeNodeAgent, SubjectID: "agent-round-1",
		RootRoundID: "round-1", RuntimeRoundID: "round-1",
		AgentRoundID: "agent-round-1", AgentID: "agent-1",
		Status:    protocol.ExecutionRuntimeNodeRunning,
		StartedAt: now, UpdatedAt: now,
	}
	tool := protocol.ExecutionRuntimeNodeRun{
		ID: "runtime-tool-1", GraphID: root.GraphID,
		OwnerUserID: root.OwnerUserID, SessionKey: root.SessionKey,
		Kind: protocol.ExecutionRuntimeNodeTool, SubjectID: "tool-1",
		RootRoundID: root.RootRoundID, RuntimeRoundID: root.RuntimeRoundID,
		AgentRoundID: root.AgentRoundID, AgentID: root.AgentID,
		Name: "search", Status: protocol.ExecutionRuntimeNodeRunning,
		StartedAt: now, UpdatedAt: now,
	}
	for _, node := range []protocol.ExecutionRuntimeNodeRun{root, tool} {
		if err := repository.UpsertRuntimeGraphNode(ctx, node); err != nil {
			t.Fatal(err)
		}
	}
	if err := repository.UpsertRuntimeGraphEdge(ctx, protocol.ExecutionRuntimeEdgeRun{
		ID: "runtime-edge-1", GraphID: root.GraphID,
		OwnerUserID: root.OwnerUserID, SessionKey: root.SessionKey,
		SourceNodeID: root.ID, TargetNodeID: tool.ID,
		Kind: protocol.ExecutionRuntimeEdgeInvoke, CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	finishedAt := now.Add(time.Minute)
	if err := repository.FinishRuntimeGraphRound(
		ctx,
		root.OwnerUserID,
		root.SessionKey,
		root.AgentRoundID,
		protocol.ExecutionRuntimeNodeSucceeded,
		finishedAt,
	); err != nil {
		t.Fatal(err)
	}

	// A replayed start must not reopen the terminal root.
	root.UpdatedAt = finishedAt.Add(time.Minute)
	if err := repository.UpsertRuntimeGraphNode(ctx, root); err != nil {
		t.Fatal(err)
	}
	graph, err := repository.GetRuntimeGraph(
		ctx,
		root.OwnerUserID,
		root.SessionKey,
		"",
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Nodes) != 2 || len(graph.Edges) != 1 {
		t.Fatalf("runtime graph = %+v", graph)
	}
	byID := make(map[string]protocol.ExecutionRuntimeNodeRun, len(graph.Nodes))
	for _, node := range graph.Nodes {
		byID[node.ID] = node
	}
	if byID[root.ID].Status != protocol.ExecutionRuntimeNodeSucceeded {
		t.Fatalf("root status = %q, want succeeded", byID[root.ID].Status)
	}
	if byID[tool.ID].Status != protocol.ExecutionRuntimeNodeInterrupted ||
		byID[tool.ID].FinishedAt == nil {
		t.Fatalf("orphaned tool was not closed: %+v", byID[tool.ID])
	}
}

func TestRepositoryRuntimeGraphPersistsAlignmentGateAndLoopObservation(t *testing.T) {
	t.Parallel()

	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	root := protocol.ExecutionRuntimeNodeRun{
		ID: "runtime-root-gate", GraphID: "execution:alignment",
		OwnerUserID: "owner-gate", SessionKey: "session-gate",
		ExecutionID: "execution-gate",
		Kind:        protocol.ExecutionRuntimeNodeAgent, SubjectID: "agent-round-gate",
		RootRoundID: "round-gate", RuntimeRoundID: "round-gate",
		AgentRoundID: "agent-round-gate", AgentID: "agent-lead",
		Status:    protocol.ExecutionRuntimeNodeRunning,
		StartedAt: now, UpdatedAt: now,
	}
	gate := protocol.ExecutionRuntimeNodeRun{
		ID: "runtime-gate-1", GraphID: root.GraphID,
		OwnerUserID: root.OwnerUserID, SessionKey: root.SessionKey,
		ExecutionID: root.ExecutionID,
		Kind:        protocol.ExecutionRuntimeNodeGate, SubjectID: "alignment-gate-1",
		ParentSubjectID: root.SubjectID,
		RootRoundID:     root.RootRoundID, RuntimeRoundID: root.RuntimeRoundID,
		AgentRoundID: root.AgentRoundID, AgentID: root.AgentID,
		Name: "Objective alignment", Status: protocol.ExecutionRuntimeNodeSucceeded,
		StartedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second),
		Metadata: map[string]any{"decision": "not_aligned"},
	}
	for _, node := range []protocol.ExecutionRuntimeNodeRun{root, gate} {
		if err := repository.UpsertRuntimeGraphNode(ctx, node); err != nil {
			t.Fatal(err)
		}
	}
	for _, edge := range []protocol.ExecutionRuntimeEdgeRun{
		{
			ID: "runtime-guard-1", GraphID: root.GraphID,
			OwnerUserID: root.OwnerUserID, SessionKey: root.SessionKey,
			SourceNodeID: root.ID, TargetNodeID: gate.ID,
			Kind: protocol.ExecutionRuntimeEdgeGuard, CreatedAt: now.Add(time.Second),
		},
		{
			ID: "runtime-loop-1", GraphID: root.GraphID,
			OwnerUserID: root.OwnerUserID, SessionKey: root.SessionKey,
			SourceNodeID: gate.ID, TargetNodeID: root.ID,
			Kind: protocol.ExecutionRuntimeEdgeLoopBack, CreatedAt: now.Add(2 * time.Second),
		},
	} {
		if err := repository.UpsertRuntimeGraphEdge(ctx, edge); err != nil {
			t.Fatal(err)
		}
	}

	graph, err := repository.GetRuntimeGraph(ctx, root.OwnerUserID, root.SessionKey, root.ExecutionID, root.RootRoundID)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Nodes) != 2 || len(graph.Edges) != 2 {
		t.Fatalf("runtime graph = %+v", graph)
	}
	if graph.Nodes[1].Kind != protocol.ExecutionRuntimeNodeGate ||
		graph.Nodes[1].Metadata["decision"] != "not_aligned" {
		t.Fatalf("gate node = %+v", graph.Nodes[1])
	}
	seen := map[protocol.ExecutionRuntimeEdgeKind]bool{}
	for _, edge := range graph.Edges {
		seen[edge.Kind] = true
	}
	if !seen[protocol.ExecutionRuntimeEdgeGuard] || !seen[protocol.ExecutionRuntimeEdgeLoopBack] {
		t.Fatalf("runtime edges = %+v", graph.Edges)
	}
}

package orchestration

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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

func TestRepositoryRuntimeGraphPersistsTerminalSummaryAndExactRetryEdge(t *testing.T) {
	t.Parallel()

	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 4, 13, 0, 0, 0, time.UTC)
	root := protocol.ExecutionRuntimeNodeRun{
		ID: "runtime-root-summary", GraphID: "round:summary",
		OwnerUserID: "owner-summary", SessionKey: "session-summary",
		Kind: protocol.ExecutionRuntimeNodeAgent, SubjectID: "agent-round-summary",
		RootRoundID: "round-summary", RuntimeRoundID: "round-summary",
		AgentRoundID: "agent-round-summary", AgentID: "agent-summary",
		Status: protocol.ExecutionRuntimeNodeRunning, StartedAt: now, UpdatedAt: now,
	}
	previous := root
	previous.ID = "runtime-tool-failed"
	previous.Kind = protocol.ExecutionRuntimeNodeTool
	previous.SubjectID = "tool-failed"
	previous.Name = "search"
	previous.Status = protocol.ExecutionRuntimeNodeFailed
	previous.Failed = true
	previous.ErrorCode = "not_found"
	previous.ErrorSummary = "Page not found"
	previous.SummaryTruncated = true
	previous.DurationMS = 1250
	finishedAt := now.Add(1250 * time.Millisecond)
	previous.FinishedAt = &finishedAt
	retry := previous
	retry.ID = "runtime-tool-retry"
	retry.SubjectID = "tool-retry"
	retry.Status = protocol.ExecutionRuntimeNodeSucceeded
	retry.Failed = false
	retry.ErrorCode = ""
	retry.ErrorSummary = ""
	retry.ResultSummary = "Found the page"
	retry.SummaryTruncated = false
	retry.StartedAt = now.Add(2 * time.Second)
	retry.UpdatedAt = retry.StartedAt
	retryFinishedAt := retry.StartedAt.Add(400 * time.Millisecond)
	retry.FinishedAt = &retryFinishedAt
	retry.DurationMS = 400
	for _, node := range []protocol.ExecutionRuntimeNodeRun{root, previous, retry} {
		if err := repository.UpsertRuntimeGraphNode(ctx, node); err != nil {
			t.Fatal(err)
		}
	}
	if err := repository.UpsertRuntimeGraphEdge(ctx, protocol.ExecutionRuntimeEdgeRun{
		ID: "runtime-retry-edge", GraphID: root.GraphID,
		OwnerUserID: root.OwnerUserID, SessionKey: root.SessionKey,
		SourceNodeID: previous.ID, TargetNodeID: retry.ID,
		Kind: protocol.ExecutionRuntimeEdgeRetry, CreatedAt: retry.StartedAt,
	}); err != nil {
		t.Fatal(err)
	}
	graph, err := repository.GetRuntimeGraph(ctx, root.OwnerUserID, root.SessionKey, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Nodes) != 3 || len(graph.Edges) != 1 {
		t.Fatalf("runtime graph = %+v", graph)
	}
	byID := make(map[string]protocol.ExecutionRuntimeNodeRun, len(graph.Nodes))
	for _, node := range graph.Nodes {
		byID[node.ID] = node
	}
	if stored := byID[previous.ID]; stored.ErrorCode != previous.ErrorCode ||
		stored.ErrorSummary != previous.ErrorSummary || !stored.SummaryTruncated ||
		stored.DurationMS != previous.DurationMS {
		t.Fatalf("stored failure summary = %+v", stored)
	}
	if stored := byID[retry.ID]; stored.ResultSummary != retry.ResultSummary ||
		stored.DurationMS != retry.DurationMS {
		t.Fatalf("stored retry summary = %+v", stored)
	}
	if graph.Edges[0].Kind != protocol.ExecutionRuntimeEdgeRetry {
		t.Fatalf("stored retry edge = %+v", graph.Edges[0])
	}
}

func TestRepositoryRuntimeGraphSurvivesOutOfOrderDuplicateAndDisconnect(t *testing.T) {
	t.Parallel()

	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 4, 14, 0, 0, 0, time.UTC)
	root := protocol.ExecutionRuntimeNodeRun{
		ID: "runtime-root-fault", GraphID: "round:fault",
		OwnerUserID: "owner-fault", SessionKey: "session-fault",
		Kind: protocol.ExecutionRuntimeNodeAgent, SubjectID: "agent-round-fault",
		RootRoundID: "round-fault", RuntimeRoundID: "round-fault",
		AgentRoundID: "agent-round-fault", AgentID: "agent-fault",
		Status: protocol.ExecutionRuntimeNodeRunning, StartedAt: now, UpdatedAt: now,
	}
	finishedAt := now.Add(2 * time.Second)
	toolFinished := protocol.ExecutionRuntimeNodeRun{
		ID: "runtime-tool-fault", GraphID: root.GraphID,
		OwnerUserID: root.OwnerUserID, SessionKey: root.SessionKey,
		Kind: protocol.ExecutionRuntimeNodeTool, SubjectID: "tool-fault",
		RootRoundID: root.RootRoundID, RuntimeRoundID: root.RuntimeRoundID,
		AgentRoundID: root.AgentRoundID, AgentID: root.AgentID,
		Name: "provider-neutral-tool", Status: protocol.ExecutionRuntimeNodeSucceeded,
		ResultSummary: "Completed before its start event was replayed",
		StartedAt:     now.Add(time.Second), UpdatedAt: finishedAt, FinishedAt: &finishedAt,
	}
	if err := repository.UpsertRuntimeGraphNode(ctx, root); err != nil {
		t.Fatal(err)
	}
	// The terminal observation can arrive before a delayed/replayed start.
	if err := repository.UpsertRuntimeGraphNode(ctx, toolFinished); err != nil {
		t.Fatal(err)
	}
	lateStart := toolFinished
	lateStart.Status = protocol.ExecutionRuntimeNodeRunning
	lateStart.ResultSummary = ""
	lateStart.FinishedAt = nil
	lateStart.UpdatedAt = now.Add(4 * time.Second)
	if err := repository.UpsertRuntimeGraphNode(ctx, lateStart); err != nil {
		t.Fatal(err)
	}
	edge := protocol.ExecutionRuntimeEdgeRun{
		ID: "runtime-edge-fault", GraphID: root.GraphID,
		OwnerUserID: root.OwnerUserID, SessionKey: root.SessionKey,
		SourceNodeID: root.ID, TargetNodeID: toolFinished.ID,
		Kind: protocol.ExecutionRuntimeEdgeInvoke, CreatedAt: now.Add(time.Second),
	}
	for index := 0; index < 2; index++ {
		if err := repository.UpsertRuntimeGraphEdge(ctx, edge); err != nil {
			t.Fatal(err)
		}
	}
	// Simulate a provider disconnect after the tool has already completed.
	if err := repository.FinishRuntimeGraphRound(
		ctx,
		root.OwnerUserID,
		root.SessionKey,
		root.AgentRoundID,
		protocol.ExecutionRuntimeNodeFailed,
		now.Add(5*time.Second),
	); err != nil {
		t.Fatal(err)
	}
	graph, err := repository.GetRuntimeGraph(ctx, root.OwnerUserID, root.SessionKey, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Nodes) != 2 || len(graph.Edges) != 1 {
		t.Fatalf("fault-injected runtime graph = %+v", graph)
	}
	byID := make(map[string]protocol.ExecutionRuntimeNodeRun, len(graph.Nodes))
	for _, node := range graph.Nodes {
		byID[node.ID] = node
	}
	if byID[root.ID].Status != protocol.ExecutionRuntimeNodeFailed {
		t.Fatalf("disconnected root = %+v", byID[root.ID])
	}
	if byID[toolFinished.ID].Status != protocol.ExecutionRuntimeNodeSucceeded ||
		byID[toolFinished.ID].FinishedAt == nil ||
		byID[toolFinished.ID].ResultSummary != toolFinished.ResultSummary {
		t.Fatalf("late start reopened terminal tool = %+v", byID[toolFinished.ID])
	}
}

func TestRepositoryRuntimeGraphKeepsRootAndLatestNodeWhenProjectionIsPartial(t *testing.T) {
	t.Parallel()

	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	startedAt := time.Date(2026, 8, 5, 9, 0, 0, 0, time.UTC)
	root := protocol.ExecutionRuntimeNodeRun{
		ID: "runtime-root-window", GraphID: "round:window",
		OwnerUserID: "owner-window", SessionKey: "session-window",
		Kind: protocol.ExecutionRuntimeNodeAgent, SubjectID: "agent-round-window",
		RootRoundID: "round-window", RuntimeRoundID: "round-window",
		AgentRoundID: "agent-round-window", AgentID: "agent-window",
		Status:    protocol.ExecutionRuntimeNodeSucceeded,
		StartedAt: startedAt, UpdatedAt: startedAt,
	}
	if err := repository.UpsertRuntimeGraphNode(ctx, root); err != nil {
		t.Fatal(err)
	}
	for index := 1; index <= protocol.ExecutionRuntimeGraphNodeProjectionLimit; index++ {
		observedAt := startedAt.Add(time.Duration(index) * time.Second)
		node := root
		node.ID = fmt.Sprintf("runtime-tool-window-%03d", index)
		node.Kind = protocol.ExecutionRuntimeNodeTool
		node.SubjectID = fmt.Sprintf("tool-window-%03d", index)
		node.ParentSubjectID = root.SubjectID
		node.Name = "observe"
		node.StartedAt = observedAt
		node.UpdatedAt = observedAt
		if err := repository.UpsertRuntimeGraphNode(ctx, node); err != nil {
			t.Fatal(err)
		}
	}

	graph, err := repository.GetRuntimeGraph(ctx, root.OwnerUserID, root.SessionKey, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if graph.NodeTotal != protocol.ExecutionRuntimeGraphNodeProjectionLimit+1 ||
		!graph.NodesTruncated ||
		len(graph.Nodes) != protocol.ExecutionRuntimeGraphNodeProjectionLimit {
		t.Fatalf("partial runtime graph = %+v", graph)
	}
	byID := make(map[string]struct{}, len(graph.Nodes))
	for _, node := range graph.Nodes {
		byID[node.ID] = struct{}{}
	}
	if _, kept := byID[root.ID]; !kept {
		t.Fatalf("partial runtime graph dropped root agent: %+v", graph.Nodes[:2])
	}
	if _, kept := byID["runtime-tool-window-256"]; !kept {
		t.Fatalf("partial runtime graph dropped latest node")
	}
	if _, kept := byID["runtime-tool-window-001"]; kept {
		t.Fatalf("partial runtime graph should reserve the oldest selected slot for root")
	}
}

func TestRepositoryBindsExactRuntimeRoundToSingleExecution(t *testing.T) {
	t.Parallel()

	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 13, 16, 0, 0, 0, time.UTC)
	root := protocol.ExecutionRuntimeNodeRun{
		ID: "runtime-root-bind", GraphID: "round:bind",
		OwnerUserID: "owner-bind", SessionKey: "session-bind",
		Kind: protocol.ExecutionRuntimeNodeAgent, SubjectID: "agent-round-bind",
		RootRoundID: "round-bind", RuntimeRoundID: "round-bind",
		AgentRoundID: "agent-round-bind", AgentID: "agent-bind",
		Status:    protocol.ExecutionRuntimeNodeRunning,
		StartedAt: now, UpdatedAt: now,
	}
	tool := root
	tool.ID = "runtime-tool-bind"
	tool.Kind = protocol.ExecutionRuntimeNodeTool
	tool.SubjectID = "tool-bind"
	tool.ParentSubjectID = root.SubjectID
	tool.Name = "assign_work"
	tool.StartedAt = now.Add(time.Second)
	tool.UpdatedAt = tool.StartedAt
	for _, node := range []protocol.ExecutionRuntimeNodeRun{root, tool} {
		if err := repository.UpsertRuntimeGraphNode(ctx, node); err != nil {
			t.Fatal(err)
		}
	}
	artifact := protocol.WorkspaceFileArtifactBlock{
		ID: "artifact-bind", Type: protocol.ContentBlockTypeWorkspaceFileArtifact,
		Path: "output/bind.md", DisplayPath: "output/bind.md",
		SourceToolUseID: tool.SubjectID, SourceToolName: tool.Name,
		WorkspaceAgentID: root.AgentID,
	}
	if err := repository.UpsertRuntimeGraphArtifact(ctx, protocol.ExecutionRuntimeArtifactRef{
		ID: "runtime-artifact-bind", GraphID: root.GraphID,
		OwnerUserID: root.OwnerUserID, SessionKey: root.SessionKey,
		RootRoundID: root.RootRoundID, AgentRoundID: root.AgentRoundID,
		ToolUseID: tool.SubjectID, Artifact: artifact,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	if err := repository.BindRuntimeGraphRoundExecution(
		ctx,
		root.OwnerUserID,
		root.SessionKey,
		root.AgentRoundID,
		"execution-bind",
	); err != nil {
		t.Fatal(err)
	}
	graph, err := repository.GetWorkGraphRuntimeGraph(
		ctx,
		root.OwnerUserID,
		root.SessionKey,
		"execution-bind",
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Nodes) != 2 {
		t.Fatalf("bound graph nodes = %+v, want root and assign tool", graph.Nodes)
	}
	for _, node := range graph.Nodes {
		if node.ExecutionID != "execution-bind" {
			t.Fatalf("bound node execution = %q, want execution-bind", node.ExecutionID)
		}
	}
	var artifactExecutionID string
	if err = repository.db.QueryRowContext(
		ctx,
		`SELECT execution_id FROM runtime_graph_artifact_refs WHERE artifact_ref_id = ?`,
		"runtime-artifact-bind",
	).Scan(&artifactExecutionID); err != nil {
		t.Fatal(err)
	}
	if artifactExecutionID != "execution-bind" {
		t.Fatalf("bound artifact execution = %q, want execution-bind", artifactExecutionID)
	}
	lateArtifact := artifact
	lateArtifact.ID = "artifact-bind-late"
	lateArtifact.Path = "output/bind-late.md"
	lateArtifact.DisplayPath = lateArtifact.Path
	if err = repository.UpsertRuntimeGraphArtifact(ctx, protocol.ExecutionRuntimeArtifactRef{
		ID: "runtime-artifact-bind-late", GraphID: root.GraphID,
		OwnerUserID: root.OwnerUserID, SessionKey: root.SessionKey,
		RootRoundID: root.RootRoundID, AgentRoundID: root.AgentRoundID,
		ToolUseID: tool.SubjectID, Artifact: lateArtifact,
		CreatedAt: now.Add(2 * time.Second), UpdatedAt: now.Add(2 * time.Second),
	}); err != nil {
		t.Fatal(err)
	}
	if err = repository.db.QueryRowContext(
		ctx,
		`SELECT execution_id FROM runtime_graph_artifact_refs WHERE artifact_ref_id = ?`,
		"runtime-artifact-bind-late",
	).Scan(&artifactExecutionID); err != nil {
		t.Fatal(err)
	}
	if artifactExecutionID != "execution-bind" {
		t.Fatalf("late artifact execution = %q, want execution-bind", artifactExecutionID)
	}

	if err = repository.BindRuntimeGraphRoundExecution(
		ctx,
		root.OwnerUserID,
		root.SessionKey,
		root.AgentRoundID,
		"execution-other",
	); !errors.Is(err, ErrInvariant) {
		t.Fatalf("cross-execution rebind error = %v, want ErrInvariant", err)
	}
	graph, err = repository.GetWorkGraphRuntimeGraph(
		ctx,
		root.OwnerUserID,
		root.SessionKey,
		"execution-bind",
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Nodes) != 2 {
		t.Fatalf("conflicting bind rewrote exact round: %+v", graph.Nodes)
	}
}

func TestWorkGraphReadWindowsDetailWithoutLosingLatePromotions(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	root := protocol.ExecutionRuntimeNodeRun{ID: "root", GraphID: "graph", OwnerUserID: "owner", SessionKey: "session", ExecutionID: "exec", Kind: protocol.ExecutionRuntimeNodeAgent, SubjectID: "round", RootRoundID: "round", RuntimeRoundID: "round", AgentRoundID: "round", AgentID: "agent", Status: protocol.ExecutionRuntimeNodeSucceeded, StartedAt: now, UpdatedAt: now}
	if err := repository.UpsertRuntimeGraphNode(ctx, root); err != nil {
		t.Fatal(err)
	}
	for i := range 600 {
		node := root
		node.ID, node.SubjectID = fmt.Sprintf("tool-%03d", i), fmt.Sprintf("use-%03d", i)
		node.Kind, node.Name, node.ParentSubjectID = protocol.ExecutionRuntimeNodeTool, "Read", root.SubjectID
		node.WorkGraphReadClass = "detail"
		node.StartedAt, node.UpdatedAt = now.Add(time.Duration(i)*time.Second), now.Add(time.Duration(i)*time.Second)
		if i < 4 {
			node.RecoveryKey = "read-group"
			// Equal event timestamps still select the latest successful run ID.
			node.StartedAt = now
		}
		if i == 0 {
			node.Status = protocol.ExecutionRuntimeNodeFailed
		}
		if err := repository.UpsertRuntimeGraphNode(ctx, node); err != nil {
			t.Fatal(err)
		}
	}
	// Both arrive after the node was classified as ordinary successful detail.
	if err := repository.UpsertRuntimeGraphEdge(ctx, protocol.ExecutionRuntimeEdgeRun{ID: "retry", GraphID: root.GraphID, OwnerUserID: root.OwnerUserID, SessionKey: root.SessionKey, SourceNodeID: "tool-010", TargetNodeID: "tool-011", Kind: protocol.ExecutionRuntimeEdgeRetry, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	artifact := protocol.WorkspaceFileArtifactBlock{ID: "file", Type: protocol.ContentBlockTypeWorkspaceFileArtifact, Path: "output/result.md", SourceToolUseID: "use-012", WorkspaceAgentID: root.AgentID}
	if err := repository.UpsertRuntimeGraphArtifact(ctx, protocol.ExecutionRuntimeArtifactRef{ID: "artifact", GraphID: root.GraphID, OwnerUserID: root.OwnerUserID, SessionKey: root.SessionKey, RootRoundID: root.RootRoundID, AgentRoundID: root.AgentRoundID, ToolUseID: artifact.SourceToolUseID, Artifact: artifact, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	graph, err := repository.GetWorkGraphRuntimeGraph(ctx, root.OwnerUserID, root.SessionKey, root.ExecutionID, "")
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, node := range graph.Nodes {
		ids[node.ID] = true
	}
	for _, id := range []string{"root", "tool-000", "tool-003", "tool-010", "tool-011", "tool-012", "tool-599"} {
		if !ids[id] {
			t.Fatalf("important runtime fact omitted: %s", id)
		}
	}
	if ids["tool-001"] || ids["tool-100"] || len(graph.Nodes) != 256+6 {
		t.Fatalf("detail window = %d nodes", len(graph.Nodes))
	}
	if graph.NodesTruncated || graph.EdgesTruncated {
		t.Fatal("detail pruning incorrectly marked canvas partial")
	}
}

func TestHistoryBackfillIsAtomicInsertOnlyAndNewExecutionsStayOutsideBacklog(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	snapshot, err := repository.Create(ctx, createTestCommand("backfill"))
	if err != nil {
		t.Fatal(err)
	}
	e := snapshot.Execution
	pending, err := repository.ListRuntimeHistoryBackfills(ctx, protocol.Execution{}, 32)
	if err != nil || len(pending) != 0 {
		t.Fatalf("new execution joined upgrade backlog: %+v %v", pending, err)
	}
	if _, err = repository.db.ExecContext(ctx, "INSERT INTO runtime_graph_history_backfills(execution_id) VALUES (?)", e.ID); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	node := protocol.ExecutionRuntimeNodeRun{ID: "backfill-node", GraphID: "graph", OwnerUserID: e.OwnerUserID, SessionKey: e.SessionKey, ExecutionID: e.ID, Kind: protocol.ExecutionRuntimeNodeTool, SubjectID: "use", RootRoundID: "round", RuntimeRoundID: "round", AgentRoundID: "round", Name: "Read", Status: protocol.ExecutionRuntimeNodeSucceeded, StartedAt: now, UpdatedAt: now}
	invalid := node
	invalid.ID = "invalid"
	invalid.OwnerUserID = "another-owner"
	if err = repository.CompleteRuntimeHistoryBackfill(ctx, e, protocol.ExecutionRuntimeGraph{Nodes: []protocol.ExecutionRuntimeNodeRun{node, invalid}}); err == nil {
		t.Fatal("scope mismatch committed")
	}
	graph, err := repository.GetWorkGraphRuntimeGraph(ctx, e.OwnerUserID, e.SessionKey, e.ID, "")
	if err != nil || len(graph.Nodes) != 0 || !graph.HistoryPending {
		t.Fatalf("failed migration committed a prefix: %+v %v", graph, err)
	}
	live := node
	live.ResultSummary = "newer durable fact"
	live.UpdatedAt = now.Add(time.Second)
	if err = repository.UpsertRuntimeGraphNode(ctx, live); err != nil {
		t.Fatal(err)
	}
	if err = repository.CompleteRuntimeHistoryBackfill(ctx, e, protocol.ExecutionRuntimeGraph{Nodes: []protocol.ExecutionRuntimeNodeRun{node}}); err != nil {
		t.Fatal(err)
	}
	graph, err = repository.GetWorkGraphRuntimeGraph(ctx, e.OwnerUserID, e.SessionKey, e.ID, "")
	if err != nil || graph.HistoryPending || graph.Nodes[0].ResultSummary != live.ResultSummary {
		t.Fatalf("backfill overwrote live data or failed to close: %+v %v", graph, err)
	}
}

func TestManagedHistorySummaryCountsOnlyActivePlanAndEnforcesScope(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	snapshot, err := repository.Create(ctx, createTestCommand("summary"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repository.WritePlan(ctx, testPlanCommand("summary", 1, "summary", "", 1)); err != nil {
		t.Fatal(err)
	}
	summaries, err := repository.ListManagedSummaries(ctx, snapshot.Execution.OwnerUserID, snapshot.Execution.SessionKey, 40)
	if err != nil || len(summaries) != 1 || summaries[0].WorkItemCount != 2 {
		t.Fatalf("summaries=%+v err=%v", summaries, err)
	}
	summaries, err = repository.ListManagedSummaries(ctx, "another-owner", snapshot.Execution.SessionKey, 40)
	if err != nil || len(summaries) != 0 {
		t.Fatalf("history scope leaked: %+v %v", summaries, err)
	}
}

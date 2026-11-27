package orchestration

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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

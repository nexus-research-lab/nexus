package orchestration

import (
	"context"
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

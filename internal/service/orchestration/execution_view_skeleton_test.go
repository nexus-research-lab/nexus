package orchestration

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestExecutionSkeletonViewKeepsStructureAndMovesRunFactsToNodeDetail(t *testing.T) {
	full := &protocol.ExecutionView{
		ID:        "execution-1",
		WorkItems: []protocol.ExecutionWorkItemView{{ID: "work-1"}},
		Graph: protocol.ExecutionGraphView{
			RuntimeNodeTotal: 3,
			Nodes: []protocol.ExecutionGraphNodeView{
				{
					ID: "agent", Visibility: protocol.ExecutionGraphNodePrimary, Name: "worker",
					ResultSummary: "done", ErrorSummary: "warn", ErrorCode: "E1", SummaryTruncated: true,
					Runs: []protocol.ExecutionGraphNodeRunView{{ID: "run-1", ResultSummary: "done"}},
				},
				{ID: "tool-failed", ParentNodeID: "agent", Visibility: protocol.ExecutionGraphNodeNested, ErrorSummary: "boom"},
				{ID: "tool-ok", ParentNodeID: "agent", Visibility: protocol.ExecutionGraphNodeDetail, ResultSummary: "ok"},
			},
			Edges: []protocol.ExecutionGraphEdgeView{
				{ID: "e1", SourceNodeID: "agent", TargetNodeID: "tool-failed"},
				{ID: "e2", SourceNodeID: "agent", TargetNodeID: "tool-ok"},
			},
		},
	}

	skeleton := executionSkeletonView(full)
	if len(skeleton.Graph.Nodes) != 2 || skeleton.Graph.Nodes[0].ID != "agent" || skeleton.Graph.Nodes[1].ID != "tool-failed" {
		t.Fatalf("skeleton nodes = %#v", skeleton.Graph.Nodes)
	}
	agent := skeleton.Graph.Nodes[0]
	if agent.ResultSummary != "" || agent.ErrorSummary != "" || agent.SummaryTruncated || agent.Runs != nil {
		t.Fatalf("skeleton kept run facts: %#v", agent)
	}
	if agent.ErrorCode != "E1" || agent.Name != "worker" {
		t.Fatalf("skeleton dropped structural status: %#v", agent)
	}
	if len(skeleton.Graph.Edges) != 1 || skeleton.Graph.Edges[0].ID != "e1" {
		t.Fatalf("skeleton edges = %#v", skeleton.Graph.Edges)
	}
	if skeleton.Graph.RuntimeNodeTotal != 3 || len(skeleton.WorkItems) != 1 {
		t.Fatalf("skeleton lost totals or responsibility: %#v", skeleton)
	}
	// 完整投影是共享缓存值，派生骨架不得原地修改它。
	if full.Graph.Nodes[0].ResultSummary != "done" || len(full.Graph.Nodes) != 3 || len(full.Graph.Edges) != 2 {
		t.Fatalf("skeleton mutated the shared projection: %#v", full.Graph)
	}

	detail := executionGraphNodeDetail(full, "agent")
	if detail == nil || detail.ExecutionID != "execution-1" || detail.Node.ResultSummary != "done" || len(detail.Node.Runs) != 1 {
		t.Fatalf("node detail = %#v", detail)
	}
	if len(detail.Children) != 2 || detail.Children[1].ID != "tool-ok" || detail.Children[1].ResultSummary != "ok" {
		t.Fatalf("node detail children = %#v", detail.Children)
	}
	if executionGraphNodeDetail(full, "missing") != nil {
		t.Fatalf("missing node returned detail")
	}
}

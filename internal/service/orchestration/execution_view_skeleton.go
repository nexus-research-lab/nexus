// INPUT: 只读的完整 protocol.ExecutionView（含 detail 节点、NodeRun 历史与摘要）。
// OUTPUT: 公共图读取的骨架视图，以及单节点的完整详情与直接子节点。
// POS: WorkGraph 读取体积的边界：随运行时长增长的内容只出现在节点详情里。
package orchestration

import "github.com/nexus-research-lab/nexus/internal/protocol"

// executionSkeletonView 保留画布与 Dock 需要的结构与状态，去掉 detail 历史节点、
// NodeRun 历史和结果/错误摘要；runtime 总量与截断标记照旧描述完整运行图。
// 返回值与 full 共享非图字段，只能用于序列化。
func executionSkeletonView(full *protocol.ExecutionView) *protocol.ExecutionView {
	if full == nil {
		return nil
	}
	view := *full
	view.Graph.Nodes = make([]protocol.ExecutionGraphNodeView, 0, len(full.Graph.Nodes))
	kept := make(map[string]struct{}, len(full.Graph.Nodes))
	for _, node := range full.Graph.Nodes {
		if node.Visibility == protocol.ExecutionGraphNodeDetail {
			continue
		}
		node.ResultSummary = ""
		node.ErrorSummary = ""
		node.SummaryTruncated = false
		node.Runs = nil
		view.Graph.Nodes = append(view.Graph.Nodes, node)
		kept[node.ID] = struct{}{}
	}
	view.Graph.Edges = make([]protocol.ExecutionGraphEdgeView, 0, len(full.Graph.Edges))
	for _, edge := range full.Graph.Edges {
		_, source := kept[edge.SourceNodeID]
		_, target := kept[edge.TargetNodeID]
		if source && target {
			view.Graph.Edges = append(view.Graph.Edges, edge)
		}
	}
	return &view
}

// executionGraphNodeDetail 返回节点完整视图与 parent_node_id 指向它的全部子节点。
func executionGraphNodeDetail(
	full *protocol.ExecutionView,
	nodeID string,
) *protocol.ExecutionGraphNodeDetailView {
	var detail *protocol.ExecutionGraphNodeDetailView
	var children []protocol.ExecutionGraphNodeView
	for _, node := range full.Graph.Nodes {
		switch {
		case node.ID == nodeID:
			detail = &protocol.ExecutionGraphNodeDetailView{ExecutionID: full.ID, Node: node}
		case node.ParentNodeID == nodeID:
			children = append(children, node)
		}
	}
	if detail != nil {
		detail.Children = children
	}
	return detail
}

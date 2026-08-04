// INPUT: durable Runtime Graph 与可选 managed ExecutionView。
// OUTPUT: planless 单智能体图，或合并到对应 WorkGraph 节点内部的运行层。
// POS: Runtime NodeRun 到 icon-first Graph UI 的唯一展示投影。
package orchestration

import (
	"context"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *Service) getPlanlessRuntimeGraphView(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
) (*protocol.ExecutionView, error) {
	repository, ok := s.repository.(runtimeGraphRepository)
	if !ok || repository == nil {
		return nil, nil
	}
	graph, err := repository.GetRuntimeGraph(ctx, ownerUserID, sessionKey, "", "")
	if err != nil || len(graph.Nodes) == 0 {
		return nil, err
	}
	result := &protocol.ExecutionView{
		ID:         graph.GraphID,
		SessionKey: sessionKey,
		Status:     protocol.ExecutionStatusCompleted,
		Version:    1,
	}
	for index, node := range graph.Nodes {
		if index == 0 || node.StartedAt.Before(result.CreatedAt) {
			result.CreatedAt = node.StartedAt
		}
		if node.UpdatedAt.After(result.UpdatedAt) {
			result.UpdatedAt = node.UpdatedAt
		}
		if node.Status == protocol.ExecutionRuntimeNodeRunning {
			result.Status = protocol.ExecutionStatusActive
			result.CompletedAt = nil
			continue
		}
		if node.FinishedAt != nil &&
			(result.CompletedAt == nil || node.FinishedAt.After(*result.CompletedAt)) {
			finishedAt := node.FinishedAt.UTC()
			result.CompletedAt = &finishedAt
		}
	}
	mergeExecutionRuntimeGraph(result, graph)
	return result, nil
}

func mergeExecutionRuntimeGraph(
	view *protocol.ExecutionView,
	runtimeGraph protocol.ExecutionRuntimeGraph,
) {
	if view == nil || len(runtimeGraph.Nodes) == 0 {
		return
	}
	agentNodeByRound := make(map[string]string)
	subagentNodeByTask := make(map[string]string)
	graphNodeByID := make(map[string]int)
	for index, node := range view.Graph.Nodes {
		graphNodeByID[node.ID] = index
		if node.Kind == protocol.ExecutionGraphNodeAgent && node.AgentRoundID != "" {
			agentNodeByRound[node.AgentRoundID] = node.ID
		}
		if node.Kind == protocol.ExecutionGraphNodeSubagent && node.SubjectID != "" {
			subagentNodeByTask[node.SubjectID] = node.ID
		}
	}

	runtimeGraph.Nodes = slices.Clone(runtimeGraph.Nodes)
	slices.SortFunc(runtimeGraph.Nodes, func(left, right protocol.ExecutionRuntimeNodeRun) int {
		if order := left.StartedAt.Compare(right.StartedAt); order != 0 {
			return order
		}
		if order := runtimeGraphNodeKindOrder(left.Kind) - runtimeGraphNodeKindOrder(right.Kind); order != 0 {
			return order
		}
		return strings.Compare(left.ID, right.ID)
	})
	runtimeNodeProjection := make(map[string]string, len(runtimeGraph.Nodes))
	for _, runtimeNode := range runtimeGraph.Nodes {
		if runtimeNode.Kind == protocol.ExecutionRuntimeNodeAgent {
			if existingID := agentNodeByRound[runtimeNode.AgentRoundID]; existingID != "" {
				runtimeNodeProjection[runtimeNode.ID] = existingID
				if index, exists := graphNodeByID[existingID]; exists {
					view.Graph.Nodes[index].LifecycleStatus = string(runtimeNode.Status)
				}
				continue
			}
		}
		if runtimeNode.Kind == protocol.ExecutionRuntimeNodeSubagent {
			if existingID := subagentNodeByTask[runtimeNode.SubjectID]; existingID != "" {
				runtimeNodeProjection[runtimeNode.ID] = existingID
				if index, exists := graphNodeByID[existingID]; exists {
					view.Graph.Nodes[index].LifecycleStatus = string(runtimeNode.Status)
				}
				continue
			}
		}
		projected := projectRuntimeGraphNode(runtimeNode, len(view.Graph.Nodes))
		runtimeNodeProjection[runtimeNode.ID] = projected.ID
		graphNodeByID[projected.ID] = len(view.Graph.Nodes)
		view.Graph.Nodes = append(view.Graph.Nodes, projected)
	}

	existingEdges := make(map[string]struct{}, len(view.Graph.Edges))
	for _, edge := range view.Graph.Edges {
		existingEdges[executionGraphEdgeKey(edge.Kind, edge.SourceNodeID, edge.TargetNodeID)] = struct{}{}
	}
	for _, runtimeEdge := range runtimeGraph.Edges {
		sourceID := firstNonEmpty(runtimeNodeProjection[runtimeEdge.SourceNodeID], runtimeEdge.SourceNodeID)
		targetID := firstNonEmpty(runtimeNodeProjection[runtimeEdge.TargetNodeID], runtimeEdge.TargetNodeID)
		if sourceID == "" || targetID == "" || sourceID == targetID {
			continue
		}
		kind := protocol.ExecutionGraphEdgeInvoke
		switch runtimeEdge.Kind {
		case protocol.ExecutionRuntimeEdgeSpawn:
			kind = protocol.ExecutionGraphEdgeSpawn
		case protocol.ExecutionRuntimeEdgeGuard:
			kind = protocol.ExecutionGraphEdgeGuard
		case protocol.ExecutionRuntimeEdgeLoopBack:
			kind = protocol.ExecutionGraphEdgeLoopBack
		}
		key := executionGraphEdgeKey(kind, sourceID, targetID)
		if _, duplicate := existingEdges[key]; duplicate {
			continue
		}
		existingEdges[key] = struct{}{}
		view.Graph.Edges = append(view.Graph.Edges, protocol.ExecutionGraphEdgeView{
			ID:           runtimeEdge.ID,
			Kind:         kind,
			SourceNodeID: sourceID,
			TargetNodeID: targetID,
		})
		if runtimeEdge.Kind != protocol.ExecutionRuntimeEdgeLoopBack {
			if targetIndex, exists := graphNodeByID[targetID]; exists {
				view.Graph.Nodes[targetIndex].ParentNodeID = sourceID
				if sourceIndex, sourceExists := graphNodeByID[sourceID]; sourceExists {
					view.Graph.Nodes[targetIndex].WorkItemID = view.Graph.Nodes[sourceIndex].WorkItemID
				}
			}
		}
	}
}

func runtimeGraphNodeKindOrder(kind protocol.ExecutionRuntimeNodeKind) int {
	switch kind {
	case protocol.ExecutionRuntimeNodeAgent:
		return 0
	case protocol.ExecutionRuntimeNodeSubagent:
		return 1
	case protocol.ExecutionRuntimeNodeTool:
		return 2
	case protocol.ExecutionRuntimeNodeGate:
		return 3
	default:
		return 4
	}
}

func projectRuntimeGraphNode(
	item protocol.ExecutionRuntimeNodeRun,
	position int,
) protocol.ExecutionGraphNodeView {
	kind := protocol.ExecutionGraphNodeAgent
	visibility := protocol.ExecutionGraphNodePrimary
	switch item.Kind {
	case protocol.ExecutionRuntimeNodeSubagent:
		kind = protocol.ExecutionGraphNodeSubagent
		visibility = protocol.ExecutionGraphNodeNested
	case protocol.ExecutionRuntimeNodeTool:
		kind = protocol.ExecutionGraphNodeTool
		visibility = protocol.ExecutionGraphNodeDetail
		if runtimeToolPromoted(item) {
			visibility = protocol.ExecutionGraphNodeNested
		}
	case protocol.ExecutionRuntimeNodeGate:
		kind = protocol.ExecutionGraphNodeGate
	}
	lifecycleStatus := string(item.Status)
	if item.Kind == protocol.ExecutionRuntimeNodeGate {
		if decision, ok := item.Metadata["decision"].(string); ok &&
			strings.TrimSpace(decision) != "" {
			lifecycleStatus = strings.TrimSpace(decision)
		}
	}
	return protocol.ExecutionGraphNodeView{
		ID:              item.ID,
		Kind:            kind,
		Visibility:      visibility,
		AgentID:         item.AgentID,
		AgentRoundID:    item.AgentRoundID,
		SubjectID:       item.SubjectID,
		Name:            item.Name,
		Description:     item.Description,
		LifecycleStatus: lifecycleStatus,
		Position:        position,
	}
}

func runtimeToolPromoted(item protocol.ExecutionRuntimeNodeRun) bool {
	if item.Status != protocol.ExecutionRuntimeNodeSucceeded {
		return true
	}
	name := strings.ToLower(strings.TrimSpace(item.Name))
	for _, marker := range []string{
		"plan_execution",
		"assign_work",
		"submit_work",
		"review_work",
		"take_over_work",
		"create_goal",
		"update_goal",
		"spawn_agent",
		"subagent",
	} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

func executionGraphEdgeKey(
	kind protocol.ExecutionGraphEdgeKind,
	sourceID string,
	targetID string,
) string {
	return string(kind) + "\x00" + sourceID + "\x00" + targetID
}

// INPUT: durable Runtime Graph 与可选 managed ExecutionView。
// OUTPUT: planless 单智能体图，或合并到对应 WorkGraph 节点内部且父子边完整的运行层。
// POS: Runtime NodeRun 到 icon-first Graph UI 的唯一展示投影；可从持久父身份修复历史缺边快照。
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
	agentNodeByWorkItem := make(map[string]string)
	reviewNodeByWorkItem := make(map[string]string)
	subagentNodeByTask := make(map[string]string)
	graphNodeByID := make(map[string]int)
	for index, node := range view.Graph.Nodes {
		graphNodeByID[node.ID] = index
		if node.Kind == protocol.ExecutionGraphNodeAgent && node.WorkItemID != "" {
			agentNodeByWorkItem[node.WorkItemID] = node.ID
		}
		if node.Kind == protocol.ExecutionGraphNodeGate && node.WorkItemID != "" {
			reviewNodeByWorkItem[node.WorkItemID] = node.ID
		}
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
	managedExecution := len(view.WorkItems) > 0
	allowedAgentRound := make(map[string]struct{})
	if managedExecution {
		for _, runtimeNode := range runtimeGraph.Nodes {
			if runtimeNode.Kind != protocol.ExecutionRuntimeNodeAgent {
				continue
			}
			lane := runtimeGraphMetadataString(runtimeNode, "execution_lane")
			workItemID := runtimeGraphMetadataString(runtimeNode, "work_item_id")
			allowed := agentNodeByRound[runtimeNode.AgentRoundID] != "" ||
				lane == "coordination" ||
				(lane == "work" && agentNodeByWorkItem[workItemID] != "") ||
				(lane == "review" && reviewNodeByWorkItem[workItemID] != "")
			if allowed && runtimeNode.AgentRoundID != "" {
				allowedAgentRound[runtimeNode.AgentRoundID] = struct{}{}
			}
		}
	}
	runtimeNodeProjection := make(map[string]string, len(runtimeGraph.Nodes))
	coordinatorNodeIDs := make([]string, 0)
	for _, runtimeNode := range runtimeGraph.Nodes {
		if managedExecution {
			if _, allowed := allowedAgentRound[runtimeNode.AgentRoundID]; !allowed {
				continue
			}
		}
		if runtimeNode.Kind == protocol.ExecutionRuntimeNodeAgent {
			lane := runtimeGraphMetadataString(runtimeNode, "execution_lane")
			workItemID := runtimeGraphMetadataString(runtimeNode, "work_item_id")
			boundNodeID := ""
			switch lane {
			case "work":
				boundNodeID = agentNodeByWorkItem[workItemID]
			case "review":
				boundNodeID = reviewNodeByWorkItem[workItemID]
			}
			if boundNodeID != "" {
				runtimeNodeProjection[runtimeNode.ID] = boundNodeID
				agentNodeByRound[runtimeNode.AgentRoundID] = boundNodeID
				updateBoundExecutionGraphNode(view, graphNodeByID, boundNodeID, runtimeNode)
				continue
			}
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
		if runtimeNode.Kind == protocol.ExecutionRuntimeNodeAgent && runtimeNode.AgentRoundID != "" {
			agentNodeByRound[runtimeNode.AgentRoundID] = projected.ID
			if runtimeGraphMetadataString(runtimeNode, "execution_lane") == "coordination" {
				coordinatorNodeIDs = append(coordinatorNodeIDs, projected.ID)
			}
		}
		if runtimeNode.Kind == protocol.ExecutionRuntimeNodeSubagent && runtimeNode.SubjectID != "" {
			subagentNodeByTask[runtimeNode.SubjectID] = projected.ID
		}
	}

	existingEdges := make(map[string]struct{}, len(view.Graph.Edges))
	for _, edge := range view.Graph.Edges {
		existingEdges[executionGraphEdgeKey(edge.Kind, edge.SourceNodeID, edge.TargetNodeID)] = struct{}{}
	}
	parentNodeBySubject := make(map[string]string, len(runtimeGraph.Nodes))
	for _, runtimeNode := range runtimeGraph.Nodes {
		projectedID := runtimeNodeProjection[runtimeNode.ID]
		if projectedID != "" && strings.TrimSpace(runtimeNode.SubjectID) != "" {
			parentNodeBySubject[strings.TrimSpace(runtimeNode.SubjectID)] = projectedID
		}
	}
	for _, runtimeNode := range runtimeGraph.Nodes {
		if runtimeNode.Kind != protocol.ExecutionRuntimeNodeSubagent {
			continue
		}
		toolUseID, _ := runtimeNode.Metadata["tool_use_id"].(string)
		if toolUseID = strings.TrimSpace(toolUseID); toolUseID != "" {
			parentNodeBySubject[toolUseID] = runtimeNodeProjection[runtimeNode.ID]
		}
	}
	incomingRuntimeNode := make(map[string]struct{})
	for _, runtimeEdge := range runtimeGraph.Edges {
		sourceID := firstNonEmpty(runtimeNodeProjection[runtimeEdge.SourceNodeID], runtimeEdge.SourceNodeID)
		targetID := firstNonEmpty(runtimeNodeProjection[runtimeEdge.TargetNodeID], runtimeEdge.TargetNodeID)
		if sourceID == "" || targetID == "" || sourceID == targetID {
			continue
		}
		if _, sourceExists := graphNodeByID[sourceID]; !sourceExists {
			continue
		}
		if _, targetExists := graphNodeByID[targetID]; !targetExists {
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
		if runtimeEdge.Kind != protocol.ExecutionRuntimeEdgeLoopBack {
			incomingRuntimeNode[targetID] = struct{}{}
			bindExecutionGraphNodeParent(view, graphNodeByID, sourceID, targetID)
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
	}
	// Early runtime graph versions could persist a NodeRun before its EdgeRun.
	// ParentSubjectID and AgentRoundID are durable semantic identity, so repair
	// only that missing projection edge instead of leaving an orphan icon.
	for _, runtimeNode := range runtimeGraph.Nodes {
		if runtimeNode.Kind == protocol.ExecutionRuntimeNodeAgent {
			continue
		}
		targetID := runtimeNodeProjection[runtimeNode.ID]
		if targetID == "" {
			continue
		}
		if _, exists := incomingRuntimeNode[targetID]; exists {
			continue
		}
		sourceID := parentNodeBySubject[strings.TrimSpace(runtimeNode.ParentSubjectID)]
		if sourceID == "" {
			sourceID = agentNodeByRound[runtimeNode.AgentRoundID]
		}
		if sourceID == "" || sourceID == targetID {
			continue
		}
		kind, ok := executionGraphEdgeKindForRuntimeNode(runtimeNode.Kind)
		if !ok {
			continue
		}
		bindExecutionGraphNodeParent(view, graphNodeByID, sourceID, targetID)
		key := executionGraphEdgeKey(kind, sourceID, targetID)
		if _, duplicate := existingEdges[key]; !duplicate {
			existingEdges[key] = struct{}{}
			view.Graph.Edges = append(view.Graph.Edges, protocol.ExecutionGraphEdgeView{
				ID:           "derived:" + string(kind) + ":" + sourceID + ":" + targetID,
				Kind:         kind,
				SourceNodeID: sourceID,
				TargetNodeID: targetID,
			})
		}
		incomingRuntimeNode[targetID] = struct{}{}
	}
	appendCoordinatorDispatchEdges(
		view,
		graphNodeByID,
		existingEdges,
		coordinatorNodeIDs,
	)
}

func updateBoundExecutionGraphNode(
	view *protocol.ExecutionView,
	graphNodeByID map[string]int,
	nodeID string,
	runtimeNode protocol.ExecutionRuntimeNodeRun,
) {
	index, exists := graphNodeByID[nodeID]
	if !exists {
		return
	}
	node := &view.Graph.Nodes[index]
	node.AgentRoundID = runtimeNode.AgentRoundID
	if runtimeNode.AgentID != "" {
		node.AgentID = runtimeNode.AgentID
	}
	node.LifecycleStatus = string(runtimeNode.Status)
}

func appendCoordinatorDispatchEdges(
	view *protocol.ExecutionView,
	graphNodeByID map[string]int,
	existingEdges map[string]struct{},
	coordinatorNodeIDs []string,
) {
	if len(coordinatorNodeIDs) == 0 {
		return
	}
	incomingWorkItemNode := make(map[string]struct{})
	for _, edge := range view.Graph.Edges {
		if targetIndex, exists := graphNodeByID[edge.TargetNodeID]; exists &&
			view.Graph.Nodes[targetIndex].Kind == protocol.ExecutionGraphNodeAgent &&
			view.Graph.Nodes[targetIndex].WorkItemID != "" {
			incomingWorkItemNode[edge.TargetNodeID] = struct{}{}
		}
	}
	rootWorkItemNodeIDs := make([]string, 0)
	for _, node := range view.Graph.Nodes {
		if node.Kind != protocol.ExecutionGraphNodeAgent || node.WorkItemID == "" {
			continue
		}
		if _, hasIncoming := incomingWorkItemNode[node.ID]; !hasIncoming {
			rootWorkItemNodeIDs = append(rootWorkItemNodeIDs, node.ID)
		}
	}
	for _, coordinatorID := range coordinatorNodeIDs {
		for _, targetID := range rootWorkItemNodeIDs {
			if coordinatorID == targetID {
				continue
			}
			key := executionGraphEdgeKey(
				protocol.ExecutionGraphEdgeDispatch,
				coordinatorID,
				targetID,
			)
			if _, duplicate := existingEdges[key]; duplicate {
				continue
			}
			existingEdges[key] = struct{}{}
			view.Graph.Edges = append(view.Graph.Edges, protocol.ExecutionGraphEdgeView{
				ID:           "dispatch:" + coordinatorID + ":" + targetID,
				Kind:         protocol.ExecutionGraphEdgeDispatch,
				SourceNodeID: coordinatorID,
				TargetNodeID: targetID,
			})
		}
	}
}

func runtimeGraphMetadataString(
	item protocol.ExecutionRuntimeNodeRun,
	key string,
) string {
	value, _ := item.Metadata[key].(string)
	return strings.TrimSpace(value)
}

func bindExecutionGraphNodeParent(
	view *protocol.ExecutionView,
	graphNodeByID map[string]int,
	sourceID string,
	targetID string,
) {
	targetIndex, exists := graphNodeByID[targetID]
	if !exists {
		return
	}
	view.Graph.Nodes[targetIndex].ParentNodeID = sourceID
	if sourceIndex, sourceExists := graphNodeByID[sourceID]; sourceExists {
		view.Graph.Nodes[targetIndex].WorkItemID = view.Graph.Nodes[sourceIndex].WorkItemID
	}
}

func executionGraphEdgeKindForRuntimeNode(
	kind protocol.ExecutionRuntimeNodeKind,
) (protocol.ExecutionGraphEdgeKind, bool) {
	switch kind {
	case protocol.ExecutionRuntimeNodeSubagent:
		return protocol.ExecutionGraphEdgeSpawn, true
	case protocol.ExecutionRuntimeNodeTool:
		return protocol.ExecutionGraphEdgeInvoke, true
	case protocol.ExecutionRuntimeNodeGate:
		return protocol.ExecutionGraphEdgeGuard, true
	default:
		return "", false
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

// INPUT: durable Runtime Graph 与 managed ExecutionView。
// OUTPUT: 按 exact Attempt/Submission/round 或 launch ToolUse 合并到对应历史轮次、过滤纯 progress facet，并在可见性判定后限制公共 WorkGraph 主图窗口。
// POS: Runtime NodeRun 到 managed WorkGraph UI 的唯一展示投影；可从持久父身份修复历史缺边快照，并保持 primary 责任、nested runtime 与 detail 历史的边界。
package orchestration

import (
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func mergeExecutionRuntimeGraph(
	view *protocol.ExecutionView,
	runtimeGraph protocol.ExecutionRuntimeGraph,
) {
	if view == nil {
		return
	}
	view.Graph.RuntimeNodeTotal = 0
	view.Graph.RuntimeEdgeTotal = 0
	view.Graph.RuntimeNodesTruncated = false
	view.Graph.RuntimeEdgesTruncated = false
	for index := range runtimeGraph.Nodes {
		runtimeGraph.Nodes[index].Metadata = maps.Clone(runtimeGraph.Nodes[index].Metadata)
	}
	runtimeGraph.Nodes = slices.DeleteFunc(
		slices.Clone(runtimeGraph.Nodes),
		isRuntimeGraphProgressFacet,
	)
	if len(runtimeGraph.Nodes) == 0 {
		return
	}
	agentNodeByRound := firstAssignedWorkItemNodeByAgentRound(view.Graph.Nodes)
	agentNodeByWorkItem := make(map[string]string)
	agentNodeByAttempt := make(map[string]string)
	reviewNodeByWorkItem := make(map[string]string)
	reviewNodeBySubmission := make(map[string]string)
	reviewNodeByRound := make(map[string]string)
	subagentNodeByTask := make(map[string]string)
	subagentNodeByAttempt := make(map[string]string)
	subagentNodeByToolUse := make(map[string]string)
	graphNodeByID := make(map[string]int)
	coordinatorNodeIDs := make([]string, 0)
	for index, node := range view.Graph.Nodes {
		graphNodeByID[node.ID] = index
		if node.Kind == protocol.ExecutionGraphNodeAgent && node.WorkItemID != "" {
			agentNodeByWorkItem[node.WorkItemID] = node.ID
			if node.AttemptID != "" {
				agentNodeByAttempt[node.AttemptID] = node.ID
			}
		}
		if node.Kind == protocol.ExecutionGraphNodeGate && node.WorkItemID != "" {
			reviewNodeByWorkItem[node.WorkItemID] = node.ID
			if node.SubjectID != "" {
				reviewNodeBySubmission[node.SubjectID] = node.ID
			}
			if node.AgentRoundID != "" && reviewNodeByRound[node.AgentRoundID] == "" {
				reviewNodeByRound[node.AgentRoundID] = node.ID
			}
		}
		if node.Kind == protocol.ExecutionGraphNodeAgent && node.AgentRoundID != "" {
			if agentNodeByRound[node.AgentRoundID] == "" {
				agentNodeByRound[node.AgentRoundID] = node.ID
			}
		}
		if node.Kind == protocol.ExecutionGraphNodeSubagent && node.SubjectID != "" {
			subagentNodeByTask[node.SubjectID] = node.ID
		}
		if node.Kind == protocol.ExecutionGraphNodeSubagent && node.AttemptID != "" {
			subagentNodeByAttempt[node.AttemptID] = node.ID
		}
		if node.Kind == protocol.ExecutionGraphNodeAgent && node.WorkItemID == "" &&
			node.AgentID != "" && node.AgentID == view.CoordinatorAgentID {
			coordinatorNodeIDs = append(coordinatorNodeIDs, node.ID)
		}
	}
	for _, item := range view.WorkItems {
		for _, attempt := range item.Attempts {
			if attempt.ParentAttemptID == "" || strings.TrimSpace(attempt.ToolUseID) == "" {
				continue
			}
			if nodeID := subagentNodeByAttempt[attempt.ID]; nodeID != "" {
				subagentNodeByToolUse[strings.TrimSpace(attempt.ToolUseID)] = nodeID
			}
		}
	}

	slices.SortFunc(runtimeGraph.Nodes, func(left, right protocol.ExecutionRuntimeNodeRun) int {
		if order := left.StartedAt.Compare(right.StartedAt); order != 0 {
			return order
		}
		if order := runtimeGraphNodeKindOrder(left.Kind) - runtimeGraphNodeKindOrder(right.Kind); order != 0 {
			return order
		}
		return strings.Compare(left.ID, right.ID)
	})
	recoverDMSelfAssignmentRuntimeSegments(view, runtimeGraph.Nodes)
	managedExecution := len(view.WorkItems) > 0
	allowedAgentRound := make(map[string]struct{})
	if managedExecution {
		for _, runtimeNode := range runtimeGraph.Nodes {
			if runtimeNode.Kind != protocol.ExecutionRuntimeNodeAgent {
				continue
			}
			lane := runtimeGraphMetadataString(runtimeNode, "execution_lane")
			workItemID := runtimeGraphMetadataString(runtimeNode, "work_item_id")
			attemptID := runtimeGraphMetadataString(runtimeNode, "attempt_id")
			submissionID := runtimeGraphMetadataString(runtimeNode, "submission_id")
			allowed := agentNodeByRound[runtimeNode.AgentRoundID] != "" ||
				lane == "coordination" ||
				(lane == "work" && firstNonEmpty(agentNodeByAttempt[attemptID], agentNodeByRound[runtimeNode.AgentRoundID], agentNodeByWorkItem[workItemID]) != "") ||
				(lane == "review" && firstNonEmpty(reviewNodeBySubmission[submissionID], reviewNodeByRound[runtimeNode.AgentRoundID], reviewNodeByWorkItem[workItemID]) != "")
			if allowed && runtimeNode.AgentRoundID != "" {
				allowedAgentRound[runtimeNode.AgentRoundID] = struct{}{}
			}
		}
	}
	runtimeNodeProjection := make(map[string]string, len(runtimeGraph.Nodes))
	runtimeProjectedNodeIDs := make(map[string]struct{}, len(runtimeGraph.Nodes))
	runtimeProjectedEdgeIDs := make(map[string]struct{}, len(runtimeGraph.Edges))
	promotedRuntimeNodeIDs := runtimeGraphPromotedNodeIDs(runtimeGraph)
	runtimeSubagentLaunchNodeIDs := make(map[string]struct{})
	for _, runtimeNode := range runtimeGraph.Nodes {
		subjectID := strings.TrimSpace(runtimeNode.SubjectID)
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
				boundNodeID = firstNonEmpty(
					agentNodeByAttempt[runtimeGraphMetadataString(runtimeNode, "attempt_id")],
					agentNodeByRound[runtimeNode.AgentRoundID],
					agentNodeByWorkItem[workItemID],
				)
			case "review":
				boundNodeID = firstNonEmpty(
					reviewNodeBySubmission[runtimeGraphMetadataString(runtimeNode, "submission_id")],
					reviewNodeByRound[runtimeNode.AgentRoundID],
					reviewNodeByWorkItem[workItemID],
				)
			case "coordination":
				if len(coordinatorNodeIDs) > 0 {
					boundNodeID = coordinatorNodeIDs[0]
				}
			}
			if boundNodeID != "" {
				runtimeNodeProjection[runtimeNode.ID] = boundNodeID
				agentNodeByRound[runtimeNode.AgentRoundID] = boundNodeID
				updateBoundExecutionGraphNode(view, graphNodeByID, boundNodeID, runtimeNode)
				continue
			}
			if existingID := agentNodeByRound[runtimeNode.AgentRoundID]; existingID != "" {
				runtimeNodeProjection[runtimeNode.ID] = existingID
				updateBoundExecutionGraphNode(view, graphNodeByID, existingID, runtimeNode)
				continue
			}
		}
		if runtimeNode.Kind == protocol.ExecutionRuntimeNodeSubagent {
			existingID := subagentNodeByTask[subjectID]
			if existingID == "" {
				toolUseID := strings.TrimSpace(runtimeGraphMetadataString(runtimeNode, "tool_use_id"))
				existingID = subagentNodeByToolUse[toolUseID]
			}
			if existingID != "" {
				runtimeNodeProjection[runtimeNode.ID] = existingID
				updateBoundExecutionGraphNode(view, graphNodeByID, existingID, runtimeNode)
				if subjectID != "" {
					subagentNodeByTask[subjectID] = existingID
				}
				continue
			}
		}
		if runtimeNode.Kind == protocol.ExecutionRuntimeNodeTool {
			if existingID := subagentNodeByToolUse[subjectID]; existingID != "" {
				runtimeNodeProjection[runtimeNode.ID] = existingID
				runtimeSubagentLaunchNodeIDs[runtimeNode.ID] = struct{}{}
				// Agent Tool 只证明“发起了哪一个 child”。它与 child Attempt
				// 投影为同一 Subagent 后，不得用 launch Tool 的成功/失败覆盖
				// 子任务自身的生命周期与结果。
				continue
			}
		}
		_, promoted := promotedRuntimeNodeIDs[runtimeNode.ID]
		projected := projectRuntimeGraphNode(runtimeNode, len(view.Graph.Nodes), promoted)
		runtimeNodeProjection[runtimeNode.ID] = projected.ID
		runtimeProjectedNodeIDs[projected.ID] = struct{}{}
		graphNodeByID[projected.ID] = len(view.Graph.Nodes)
		view.Graph.Nodes = append(view.Graph.Nodes, projected)
		if runtimeNode.Kind == protocol.ExecutionRuntimeNodeAgent && runtimeNode.AgentRoundID != "" {
			agentNodeByRound[runtimeNode.AgentRoundID] = projected.ID
			if runtimeGraphMetadataString(runtimeNode, "execution_lane") == "coordination" {
				coordinatorNodeIDs = append(coordinatorNodeIDs, projected.ID)
			}
		}
		if runtimeNode.Kind == protocol.ExecutionRuntimeNodeSubagent && subjectID != "" {
			subagentNodeByTask[subjectID] = projected.ID
		}
	}

	existingEdges := make(map[string]struct{}, len(view.Graph.Edges))
	for _, edge := range view.Graph.Edges {
		existingEdges[executionGraphEdgeKey(edge.Kind, edge.SourceNodeID, edge.TargetNodeID)] = struct{}{}
	}
	parentNodeBySubject := make(map[string]string, len(runtimeGraph.Nodes))
	for _, runtimeNode := range runtimeGraph.Nodes {
		projectedID := runtimeNodeProjection[runtimeNode.ID]
		if subjectID := strings.TrimSpace(runtimeNode.SubjectID); projectedID != "" && subjectID != "" {
			parentNodeBySubject[subjectID] = projectedID
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
	runtimeNodeByID := make(map[string]protocol.ExecutionRuntimeNodeRun, len(runtimeGraph.Nodes))
	for _, runtimeNode := range runtimeGraph.Nodes {
		runtimeNodeByID[runtimeNode.ID] = runtimeNode
	}
	incomingRuntimeNode := make(map[string]struct{})
	for _, runtimeEdge := range runtimeGraph.Edges {
		sourceID := firstNonEmpty(runtimeNodeProjection[runtimeEdge.SourceNodeID], runtimeEdge.SourceNodeID)
		targetID := firstNonEmpty(runtimeNodeProjection[runtimeEdge.TargetNodeID], runtimeEdge.TargetNodeID)
		if runtimeEdge.Kind == protocol.ExecutionRuntimeEdgeLoopBack {
			sourceRuntimeNode := runtimeNodeByID[runtimeEdge.SourceNodeID]
			segment := runtimeExecutionSegmentFromNode(sourceRuntimeNode)
			if segmentOwnerID := firstNonEmpty(
				agentNodeByAttempt[segment.AttemptID],
				agentNodeByWorkItem[segment.WorkItemID],
			); segmentOwnerID != "" {
				targetID = segmentOwnerID
			}
		}
		if runtimeEdge.Kind != protocol.ExecutionRuntimeEdgeLoopBack &&
			runtimeEdge.Kind != protocol.ExecutionRuntimeEdgeRetry {
			targetRuntimeNode := runtimeNodeByID[runtimeEdge.TargetNodeID]
			segmentOwnerID := ""
			if segment := runtimeExecutionSegmentFromNode(targetRuntimeNode); segment.valid() {
				if segmentOwnerID = firstNonEmpty(
					agentNodeByAttempt[segment.AttemptID],
					agentNodeByWorkItem[segment.WorkItemID],
				); segmentOwnerID != "" && segmentOwnerID != targetID {
					sourceID = segmentOwnerID
				}
			}
			if exactParentID := parentNodeBySubject[strings.TrimSpace(targetRuntimeNode.ParentSubjectID)]; exactParentID != "" && exactParentID != targetID &&
				(segmentOwnerID == "" || exactParentID != agentNodeByRound[targetRuntimeNode.AgentRoundID]) {
				sourceID = exactParentID
			}
		}
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
		case protocol.ExecutionRuntimeEdgeRetry:
			kind = protocol.ExecutionGraphEdgeRetry
		}
		if _, isSubagentLaunch := runtimeSubagentLaunchNodeIDs[runtimeEdge.TargetNodeID]; isSubagentLaunch {
			kind = protocol.ExecutionGraphEdgeSpawn
		}
		if runtimeEdge.Kind != protocol.ExecutionRuntimeEdgeLoopBack &&
			runtimeEdge.Kind != protocol.ExecutionRuntimeEdgeRetry {
			incomingRuntimeNode[targetID] = struct{}{}
			bindExecutionGraphNodeParent(view, graphNodeByID, sourceID, targetID)
		}
		key := executionGraphEdgeKey(kind, sourceID, targetID)
		if _, duplicate := existingEdges[key]; duplicate {
			enrichExecutionGraphRuntimeEdge(view, key, runtimeEdge)
			continue
		}
		createdAt := runtimeEdge.CreatedAt.UTC()
		existingEdges[key] = struct{}{}
		runtimeProjectedEdgeIDs[runtimeEdge.ID] = struct{}{}
		view.Graph.Edges = append(view.Graph.Edges, protocol.ExecutionGraphEdgeView{
			ID:              runtimeEdge.ID,
			Kind:            kind,
			SourceNodeID:    sourceID,
			TargetNodeID:    targetID,
			SourceNodeRunID: runtimeEdge.SourceNodeID,
			TargetNodeRunID: runtimeEdge.TargetNodeID,
			CreatedAt:       &createdAt,
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
		if segment := runtimeExecutionSegmentFromNode(runtimeNode); segment.valid() {
			segmentOwnerID := firstNonEmpty(
				agentNodeByAttempt[segment.AttemptID],
				agentNodeByWorkItem[segment.WorkItemID],
			)
			if segmentOwnerID != "" && (sourceID == "" || sourceID == agentNodeByRound[runtimeNode.AgentRoundID]) {
				sourceID = segmentOwnerID
			}
		}
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
			derivedEdgeID := "derived:" + string(kind) + ":" + sourceID + ":" + targetID
			runtimeProjectedEdgeIDs[derivedEdgeID] = struct{}{}
			view.Graph.Edges = append(view.Graph.Edges, protocol.ExecutionGraphEdgeView{
				ID:           derivedEdgeID,
				Kind:         kind,
				SourceNodeID: sourceID,
				TargetNodeID: targetID,
			})
		}
		incomingRuntimeNode[targetID] = struct{}{}
	}
	reanchorExecutionReviewEdgesToSubmission(view, graphNodeByID)
	appendCoordinatorCoordinationEdges(
		view,
		graphNodeByID,
		existingEdges,
		coordinatorNodeIDs,
	)
	applyExecutionRuntimeProjectionWindow(
		view,
		runtimeProjectedNodeIDs,
		runtimeProjectedEdgeIDs,
	)
}

// firstAssignedWorkItemNodeByAgentRound resolves the outer physical round to
// the first durable root Attempt created in that round. This is only the
// fallback owner for pre-assignment coordination tools; exact execution
// segment metadata still wins for every tool after assign_work.
type runtimeRoundOwnerCandidate struct {
	workItemID string
	nodeID     string
	createdAt  time.Time
	position   int
}

func firstAssignedWorkItemNodeByAgentRound(
	nodes []protocol.ExecutionGraphNodeView,
) map[string]string {
	candidates := make(map[string]runtimeRoundOwnerCandidate)
	for _, node := range nodes {
		roundID := strings.TrimSpace(node.AgentRoundID)
		if node.Kind != protocol.ExecutionGraphNodeAgent ||
			strings.TrimSpace(node.WorkItemID) == "" ||
			strings.TrimSpace(node.AttemptID) == "" || roundID == "" {
			continue
		}
		createdAt := time.Time{}
		if node.StartedAt != nil {
			createdAt = *node.StartedAt
		} else if len(node.Runs) > 0 && node.Runs[0].StartedAt != nil {
			createdAt = *node.Runs[0].StartedAt
		}
		incoming := runtimeRoundOwnerCandidate{
			workItemID: node.WorkItemID,
			nodeID:     node.ID,
			createdAt:  createdAt,
			position:   node.Position,
		}
		current, exists := candidates[roundID]
		if !exists || runtimeRoundOwnerCandidateEarlier(incoming, current) {
			candidates[roundID] = incoming
		}
	}
	result := make(map[string]string, len(candidates))
	for roundID, candidate := range candidates {
		result[roundID] = candidate.nodeID
	}
	return result
}

func runtimeRoundOwnerCandidateEarlier(
	left runtimeRoundOwnerCandidate,
	right runtimeRoundOwnerCandidate,
) bool {
	switch {
	case left.createdAt.IsZero() != right.createdAt.IsZero():
		return !left.createdAt.IsZero()
	case !left.createdAt.IsZero() && !left.createdAt.Equal(right.createdAt):
		return left.createdAt.Before(right.createdAt)
	case left.position != right.position:
		return left.position < right.position
	default:
		return strings.Compare(left.workItemID, right.workItemID) < 0
	}
}

const executionRuntimeGraphDetailProjectionLimit = 256

// applyExecutionRuntimeProjectionWindow 在 visibility 已经确定之后限制公共 WorkGraph。
// detail 历史使用独立窗口，既不占主图节点配额，也不参与 partial / total 语义。
func applyExecutionRuntimeProjectionWindow(
	view *protocol.ExecutionView,
	runtimeProjectedNodeIDs map[string]struct{},
	runtimeProjectedEdgeIDs map[string]struct{},
) {
	if view == nil {
		return
	}
	nodeByID := make(map[string]protocol.ExecutionGraphNodeView, len(view.Graph.Nodes))
	canvasRuntimeNodeIDs := make([]string, 0, len(runtimeProjectedNodeIDs))
	detailRuntimeNodeIDs := make([]string, 0, len(runtimeProjectedNodeIDs))
	for _, node := range view.Graph.Nodes {
		nodeByID[node.ID] = node
		if _, runtimeProjected := runtimeProjectedNodeIDs[node.ID]; !runtimeProjected {
			continue
		}
		if node.Visibility == protocol.ExecutionGraphNodeDetail {
			detailRuntimeNodeIDs = append(detailRuntimeNodeIDs, node.ID)
			continue
		}
		canvasRuntimeNodeIDs = append(canvasRuntimeNodeIDs, node.ID)
	}
	view.Graph.RuntimeNodeTotal = len(canvasRuntimeNodeIDs)
	view.Graph.RuntimeNodesTruncated = len(canvasRuntimeNodeIDs) > protocol.ExecutionRuntimeGraphNodeProjectionLimit

	keepCanvasNodeIDs := make(map[string]struct{}, min(
		len(canvasRuntimeNodeIDs),
		protocol.ExecutionRuntimeGraphNodeProjectionLimit,
	))
	if !view.Graph.RuntimeNodesTruncated {
		for _, nodeID := range canvasRuntimeNodeIDs {
			keepCanvasNodeIDs[nodeID] = struct{}{}
		}
	} else {
		requiredNodeIDs := make([]string, 0)
		for _, edge := range view.Graph.Edges {
			if _, runtimeEdge := runtimeProjectedEdgeIDs[edge.ID]; runtimeEdge {
				continue
			}
			if node, exists := nodeByID[edge.SourceNodeID]; exists &&
				node.Visibility != protocol.ExecutionGraphNodeDetail {
				if _, runtimeNode := runtimeProjectedNodeIDs[node.ID]; runtimeNode {
					requiredNodeIDs = append(requiredNodeIDs, node.ID)
				}
			}
			if node, exists := nodeByID[edge.TargetNodeID]; exists &&
				node.Visibility != protocol.ExecutionGraphNodeDetail {
				if _, runtimeNode := runtimeProjectedNodeIDs[node.ID]; runtimeNode {
					requiredNodeIDs = append(requiredNodeIDs, node.ID)
				}
			}
		}
		selectExecutionRuntimeProjectionNodes(
			requiredNodeIDs,
			nodeByID,
			runtimeProjectedNodeIDs,
			keepCanvasNodeIDs,
			protocol.ExecutionRuntimeGraphNodeProjectionLimit,
		)
		slices.SortFunc(canvasRuntimeNodeIDs, func(leftID, rightID string) int {
			return compareExecutionGraphProjectionRecency(nodeByID[leftID], nodeByID[rightID])
		})
		selectExecutionRuntimeProjectionNodes(
			canvasRuntimeNodeIDs,
			nodeByID,
			runtimeProjectedNodeIDs,
			keepCanvasNodeIDs,
			protocol.ExecutionRuntimeGraphNodeProjectionLimit,
		)
	}

	keepNodeIDs := make(map[string]struct{}, len(view.Graph.Nodes))
	for _, node := range view.Graph.Nodes {
		if _, runtimeProjected := runtimeProjectedNodeIDs[node.ID]; !runtimeProjected {
			keepNodeIDs[node.ID] = struct{}{}
		}
	}
	for nodeID := range keepCanvasNodeIDs {
		keepNodeIDs[nodeID] = struct{}{}
	}
	slices.SortFunc(detailRuntimeNodeIDs, func(leftID, rightID string) int {
		return compareExecutionGraphProjectionRecency(nodeByID[leftID], nodeByID[rightID])
	})
	selectExecutionRuntimeDetailNodes(
		detailRuntimeNodeIDs,
		nodeByID,
		runtimeProjectedNodeIDs,
		keepNodeIDs,
		executionRuntimeGraphDetailProjectionLimit,
	)
	allCanvasNodeIDs := make(map[string]struct{}, len(nodeByID))
	for nodeID, node := range nodeByID {
		if node.Visibility != protocol.ExecutionGraphNodeDetail {
			allCanvasNodeIDs[nodeID] = struct{}{}
		}
	}
	for _, edge := range view.Graph.Edges {
		if _, runtimeProjected := runtimeProjectedEdgeIDs[edge.ID]; !runtimeProjected {
			continue
		}
		_, sourceVisible := allCanvasNodeIDs[edge.SourceNodeID]
		_, targetVisible := allCanvasNodeIDs[edge.TargetNodeID]
		if sourceVisible && targetVisible {
			view.Graph.RuntimeEdgeTotal++
		}
	}
	view.Graph.RuntimeEdgesTruncated = view.Graph.RuntimeEdgeTotal > protocol.ExecutionRuntimeGraphEdgeProjectionLimit
	view.Graph.Nodes = slices.DeleteFunc(view.Graph.Nodes, func(node protocol.ExecutionGraphNodeView) bool {
		_, keep := keepNodeIDs[node.ID]
		return !keep
	})

	visibleNodeIDs := make(map[string]struct{}, len(view.Graph.Nodes))
	canvasNodeIDs := make(map[string]struct{}, len(view.Graph.Nodes))
	for _, node := range view.Graph.Nodes {
		visibleNodeIDs[node.ID] = struct{}{}
		if node.Visibility != protocol.ExecutionGraphNodeDetail {
			canvasNodeIDs[node.ID] = struct{}{}
		}
	}
	view.Graph.Edges = slices.DeleteFunc(view.Graph.Edges, func(edge protocol.ExecutionGraphEdgeView) bool {
		_, sourceExists := visibleNodeIDs[edge.SourceNodeID]
		_, targetExists := visibleNodeIDs[edge.TargetNodeID]
		return !sourceExists || !targetExists
	})
	if !view.Graph.RuntimeEdgesTruncated {
		return
	}
	keepRuntimeEdgeIDs := selectExecutionRuntimeProjectionEdges(
		view.Graph.Edges,
		runtimeProjectedEdgeIDs,
		canvasNodeIDs,
		protocol.ExecutionRuntimeGraphEdgeProjectionLimit,
	)
	view.Graph.Edges = slices.DeleteFunc(view.Graph.Edges, func(edge protocol.ExecutionGraphEdgeView) bool {
		if _, runtimeProjected := runtimeProjectedEdgeIDs[edge.ID]; !runtimeProjected {
			return false
		}
		if _, sourceVisible := canvasNodeIDs[edge.SourceNodeID]; !sourceVisible {
			return false
		}
		if _, targetVisible := canvasNodeIDs[edge.TargetNodeID]; !targetVisible {
			return false
		}
		_, keep := keepRuntimeEdgeIDs[edge.ID]
		return !keep
	})
}

func selectExecutionRuntimeProjectionNodes(
	candidateNodeIDs []string,
	nodeByID map[string]protocol.ExecutionGraphNodeView,
	runtimeProjectedNodeIDs map[string]struct{},
	keepNodeIDs map[string]struct{},
	limit int,
) {
	for _, candidateID := range candidateNodeIDs {
		if len(keepNodeIDs) >= limit {
			return
		}
		chain := make([]string, 0, 4)
		seen := make(map[string]struct{})
		for currentID := candidateID; currentID != ""; {
			if _, duplicate := seen[currentID]; duplicate {
				chain = nil
				break
			}
			seen[currentID] = struct{}{}
			node, exists := nodeByID[currentID]
			if !exists || node.Visibility == protocol.ExecutionGraphNodeDetail {
				break
			}
			if _, runtimeProjected := runtimeProjectedNodeIDs[currentID]; !runtimeProjected {
				break
			}
			if _, alreadyKept := keepNodeIDs[currentID]; !alreadyKept {
				chain = append(chain, currentID)
			}
			currentID = node.ParentNodeID
		}
		for index := len(chain) - 1; index >= 0 && len(keepNodeIDs) < limit; index-- {
			keepNodeIDs[chain[index]] = struct{}{}
		}
	}
}

func selectExecutionRuntimeDetailNodes(
	candidateNodeIDs []string,
	nodeByID map[string]protocol.ExecutionGraphNodeView,
	runtimeProjectedNodeIDs map[string]struct{},
	keepNodeIDs map[string]struct{},
	limit int,
) {
	keptDetailCount := 0
	for _, candidateID := range candidateNodeIDs {
		if keptDetailCount >= limit {
			return
		}
		chain := make([]string, 0, 3)
		seen := make(map[string]struct{})
		connected := false
		for currentID := candidateID; currentID != ""; {
			if _, duplicate := seen[currentID]; duplicate {
				chain = nil
				break
			}
			seen[currentID] = struct{}{}
			if _, alreadyKept := keepNodeIDs[currentID]; alreadyKept {
				connected = true
				break
			}
			node, exists := nodeByID[currentID]
			if !exists || node.Visibility != protocol.ExecutionGraphNodeDetail {
				break
			}
			if _, runtimeProjected := runtimeProjectedNodeIDs[currentID]; !runtimeProjected {
				break
			}
			chain = append(chain, currentID)
			currentID = node.ParentNodeID
		}
		if !connected || keptDetailCount+len(chain) > limit {
			continue
		}
		for index := len(chain) - 1; index >= 0; index-- {
			keepNodeIDs[chain[index]] = struct{}{}
			keptDetailCount++
		}
	}
}

func selectExecutionRuntimeProjectionEdges(
	edges []protocol.ExecutionGraphEdgeView,
	runtimeProjectedEdgeIDs map[string]struct{},
	canvasNodeIDs map[string]struct{},
	limit int,
) map[string]struct{} {
	candidates := make([]protocol.ExecutionGraphEdgeView, 0, len(runtimeProjectedEdgeIDs))
	for _, edge := range edges {
		if _, runtimeProjected := runtimeProjectedEdgeIDs[edge.ID]; !runtimeProjected {
			continue
		}
		_, sourceVisible := canvasNodeIDs[edge.SourceNodeID]
		_, targetVisible := canvasNodeIDs[edge.TargetNodeID]
		if sourceVisible && targetVisible {
			candidates = append(candidates, edge)
		}
	}
	slices.SortFunc(candidates, func(left, right protocol.ExecutionGraphEdgeView) int {
		leftControl := left.Kind == protocol.ExecutionGraphEdgeLoopBack || left.Kind == protocol.ExecutionGraphEdgeRetry
		rightControl := right.Kind == protocol.ExecutionGraphEdgeLoopBack || right.Kind == protocol.ExecutionGraphEdgeRetry
		if leftControl != rightControl {
			if leftControl {
				return 1
			}
			return -1
		}
		if left.CreatedAt != nil && right.CreatedAt != nil && !left.CreatedAt.Equal(*right.CreatedAt) {
			if left.CreatedAt.After(*right.CreatedAt) {
				return -1
			}
			return 1
		}
		if left.CreatedAt != nil && right.CreatedAt == nil {
			return -1
		}
		if left.CreatedAt == nil && right.CreatedAt != nil {
			return 1
		}
		return strings.Compare(left.ID, right.ID)
	})
	keep := make(map[string]struct{}, min(len(candidates), limit))
	for index, edge := range candidates {
		if index >= limit {
			break
		}
		keep[edge.ID] = struct{}{}
	}
	return keep
}

func compareExecutionGraphProjectionRecency(
	left protocol.ExecutionGraphNodeView,
	right protocol.ExecutionGraphNodeView,
) int {
	if left.StartedAt != nil && right.StartedAt != nil && !left.StartedAt.Equal(*right.StartedAt) {
		if left.StartedAt.After(*right.StartedAt) {
			return -1
		}
		return 1
	}
	if left.StartedAt != nil && right.StartedAt == nil {
		return -1
	}
	if left.StartedAt == nil && right.StartedAt != nil {
		return 1
	}
	if left.Position != right.Position {
		return right.Position - left.Position
	}
	return strings.Compare(left.ID, right.ID)
}

// Review 的因果起点是成功 submit_work，而不是承载整轮工作的 Agent 头像。
// changes_requested 的控制返回也落到同一个提交锚点，形成局部、可解释的闭环。
func reanchorExecutionReviewEdgesToSubmission(
	view *protocol.ExecutionView,
	graphNodeByID map[string]int,
) {
	if view == nil {
		return
	}
	submissionByOwnerNodeID := make(map[string]protocol.ExecutionGraphNodeView)
	for _, node := range view.Graph.Nodes {
		if node.Kind != protocol.ExecutionGraphNodeTool ||
			node.Visibility == protocol.ExecutionGraphNodeDetail ||
			!runtimeGraphIsSubmissionTool(node.Name) ||
			!strings.EqualFold(strings.TrimSpace(node.LifecycleStatus), "succeeded") ||
			strings.TrimSpace(node.ParentNodeID) == "" {
			continue
		}
		ownerIndex, exists := graphNodeByID[node.ParentNodeID]
		if !exists || view.Graph.Nodes[ownerIndex].Kind != protocol.ExecutionGraphNodeAgent {
			continue
		}
		current, exists := submissionByOwnerNodeID[node.ParentNodeID]
		if !exists || executionGraphNodeStartedBefore(current, node) {
			submissionByOwnerNodeID[node.ParentNodeID] = node
		}
	}
	submissionByGateNodeID := make(map[string]string)
	for index := range view.Graph.Edges {
		edge := &view.Graph.Edges[index]
		if edge.Kind != protocol.ExecutionGraphEdgeReview {
			continue
		}
		submission, exists := submissionByOwnerNodeID[edge.SourceNodeID]
		if !exists {
			continue
		}
		edge.SourceNodeID = submission.ID
		submissionByGateNodeID[edge.TargetNodeID] = submission.ID
	}
	for index := range view.Graph.Edges {
		edge := &view.Graph.Edges[index]
		if edge.Kind != protocol.ExecutionGraphEdgeLoopBack {
			continue
		}
		sourceIndex, exists := graphNodeByID[edge.SourceNodeID]
		if !exists || view.Graph.Nodes[sourceIndex].Kind != protocol.ExecutionGraphNodeGate {
			continue
		}
		if submissionID := submissionByGateNodeID[edge.SourceNodeID]; submissionID != "" {
			edge.TargetNodeID = submissionID
		}
	}
}

func executionGraphNodeStartedBefore(
	left protocol.ExecutionGraphNodeView,
	right protocol.ExecutionGraphNodeView,
) bool {
	if left.StartedAt == nil {
		return right.StartedAt != nil
	}
	if right.StartedAt == nil {
		return false
	}
	return left.StartedAt.Before(*right.StartedAt)
}

func enrichExecutionGraphRuntimeEdge(
	view *protocol.ExecutionView,
	key string,
	runtimeEdge protocol.ExecutionRuntimeEdgeRun,
) {
	for index := range view.Graph.Edges {
		edge := &view.Graph.Edges[index]
		if executionGraphEdgeKey(edge.Kind, edge.SourceNodeID, edge.TargetNodeID) != key {
			continue
		}
		createdAt := runtimeEdge.CreatedAt.UTC()
		edge.SourceNodeRunID = runtimeEdge.SourceNodeID
		edge.TargetNodeRunID = runtimeEdge.TargetNodeID
		edge.CreatedAt = &createdAt
		return
	}
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
	if runtimeNode.Kind != protocol.ExecutionRuntimeNodeTool {
		node.AgentRoundID = runtimeNode.AgentRoundID
		if runtimeNode.AgentID != "" {
			node.AgentID = runtimeNode.AgentID
		}
	}
	if runtimeNode.Kind == protocol.ExecutionRuntimeNodeSubagent {
		if runtimeNode.SubjectID != "" {
			node.SubjectID = runtimeNode.SubjectID
		}
		if runtimeNode.Name != "" {
			node.Name = runtimeNode.Name
		}
		if runtimeNode.Description != "" {
			node.Description = runtimeNode.Description
		}
	}
	node.LifecycleStatus = string(runtimeNode.Status)
	node.ResultSummary = runtimeNode.ResultSummary
	node.ErrorCode = runtimeNode.ErrorCode
	node.ErrorSummary = runtimeNode.ErrorSummary
	node.SummaryTruncated = runtimeNode.SummaryTruncated
	node.DurationMS = runtimeNodeDurationMS(runtimeNode)
	startedAt := runtimeNode.StartedAt.UTC()
	node.StartedAt = &startedAt
	if runtimeNode.FinishedAt != nil {
		finishedAt := runtimeNode.FinishedAt.UTC()
		node.FinishedAt = &finishedAt
	}
	mergeExecutionGraphNodeRun(node, runtimeNode)
}

func appendCoordinatorCoordinationEdges(
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
		if edge.Kind == protocol.ExecutionGraphEdgeLoopBack ||
			edge.Kind == protocol.ExecutionGraphEdgeRetry {
			continue
		}
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
				protocol.ExecutionGraphEdgeCoordination,
				coordinatorID,
				targetID,
			)
			if _, duplicate := existingEdges[key]; duplicate {
				continue
			}
			existingEdges[key] = struct{}{}
			view.Graph.Edges = append(view.Graph.Edges, protocol.ExecutionGraphEdgeView{
				ID:           "coordination:" + coordinatorID + ":" + targetID,
				Kind:         protocol.ExecutionGraphEdgeCoordination,
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
	promoted bool,
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
		if !runtimeGraphIsCommandTransport(item) &&
			(item.Status != protocol.ExecutionRuntimeNodeSucceeded || promoted) {
			visibility = protocol.ExecutionGraphNodeNested
		}
	case protocol.ExecutionRuntimeNodeGate:
		kind = protocol.ExecutionGraphNodeGate
		visibility = protocol.ExecutionGraphNodeNested
	}
	lifecycleStatus := string(item.Status)
	if item.Kind == protocol.ExecutionRuntimeNodeGate {
		if decision, ok := item.Metadata["decision"].(string); ok &&
			strings.TrimSpace(decision) != "" {
			lifecycleStatus = strings.TrimSpace(decision)
		}
	}
	startedAt := item.StartedAt.UTC()
	var finishedAt *time.Time
	if item.FinishedAt != nil {
		value := item.FinishedAt.UTC()
		finishedAt = &value
	}
	return protocol.ExecutionGraphNodeView{
		ID:               item.ID,
		Kind:             kind,
		Visibility:       visibility,
		WorkItemID:       runtimeExecutionSegmentWorkItemID(item),
		AgentID:          item.AgentID,
		AgentRoundID:     item.AgentRoundID,
		SubjectID:        item.SubjectID,
		Name:             item.Name,
		Description:      item.Description,
		LifecycleStatus:  lifecycleStatus,
		ResultSummary:    item.ResultSummary,
		ErrorCode:        item.ErrorCode,
		ErrorSummary:     item.ErrorSummary,
		SummaryTruncated: item.SummaryTruncated,
		DurationMS:       runtimeNodeDurationMS(item),
		StartedAt:        &startedAt,
		FinishedAt:       finishedAt,
		Runs:             []protocol.ExecutionGraphNodeRunView{runtimeGraphNodeRunView(item)},
		Position:         position,
	}
}

// isRuntimeGraphProgressFacet recognizes rows written by early observers that
// persisted provider progress messages as child Tool nodes. They have no
// independent start/finish/result fact, so exposing them invents work and
// corrupts the visible ownership set. New observers avoid writing these rows;
// this fallback keeps durable historical graphs readable.
func isRuntimeGraphProgressFacet(item protocol.ExecutionRuntimeNodeRun) bool {
	if strings.TrimSpace(item.ParentSubjectID) == "" ||
		item.Status != protocol.ExecutionRuntimeNodeInterrupted ||
		item.ResultSummary != "" || item.ErrorCode != "" || item.ErrorSummary != "" ||
		len(item.Artifacts) > 0 {
		return false
	}
	return strings.Contains(
		strings.ToLower(runtimeGraphMetadataString(item, "bridge_event_id")),
		":progress:",
	)
}

func mergeExecutionGraphNodeRun(
	node *protocol.ExecutionGraphNodeView,
	item protocol.ExecutionRuntimeNodeRun,
) {
	if node == nil {
		return
	}
	runtimeRun := runtimeGraphNodeRunView(item)
	for index := range node.Runs {
		candidate := &node.Runs[index]
		exactRuntimeNode := candidate.RuntimeNodeID != "" &&
			candidate.RuntimeNodeID == runtimeRun.RuntimeNodeID
		exactAgentRound := candidate.AgentRoundID != "" &&
			candidate.AgentRoundID == runtimeRun.AgentRoundID
		exactSubject := node.Kind != protocol.ExecutionGraphNodeAgent &&
			candidate.SubjectID != "" && candidate.SubjectID == runtimeRun.SubjectID
		if !exactRuntimeNode && !exactAgentRound && !exactSubject {
			continue
		}
		mergeExecutionGraphRun(candidate, runtimeRun)
		return
	}
	node.Runs = append(node.Runs, runtimeRun)
	slices.SortFunc(node.Runs, func(left, right protocol.ExecutionGraphNodeRunView) int {
		if left.StartedAt != nil && right.StartedAt != nil {
			if order := left.StartedAt.Compare(*right.StartedAt); order != 0 {
				return order
			}
		}
		return strings.Compare(left.ID, right.ID)
	})
}

func mergeExecutionGraphRun(
	target *protocol.ExecutionGraphNodeRunView,
	source protocol.ExecutionGraphNodeRunView,
) {
	if target == nil {
		return
	}
	target.RuntimeNodeID = source.RuntimeNodeID
	target.AgentRoundID = firstNonEmpty(source.AgentRoundID, target.AgentRoundID)
	target.SubjectID = firstNonEmpty(source.SubjectID, target.SubjectID)
	target.Status = firstNonEmpty(source.Status, target.Status)
	target.ResultSummary = firstNonEmpty(source.ResultSummary, target.ResultSummary)
	target.ErrorCode = firstNonEmpty(source.ErrorCode, target.ErrorCode)
	target.ErrorSummary = firstNonEmpty(source.ErrorSummary, target.ErrorSummary)
	target.SummaryTruncated = target.SummaryTruncated || source.SummaryTruncated
	if source.DurationMS > 0 {
		target.DurationMS = source.DurationMS
	}
	if source.StartedAt != nil {
		target.StartedAt = source.StartedAt
	}
	if source.FinishedAt != nil {
		target.FinishedAt = source.FinishedAt
	}
	if len(source.Artifacts) > 0 {
		target.Artifacts = source.Artifacts
	}
}

func runtimeGraphNodeRunView(
	item protocol.ExecutionRuntimeNodeRun,
) protocol.ExecutionGraphNodeRunView {
	startedAt := item.StartedAt.UTC()
	var finishedAt *time.Time
	if item.FinishedAt != nil {
		value := item.FinishedAt.UTC()
		finishedAt = &value
	}
	return protocol.ExecutionGraphNodeRunView{
		ID:               item.ID,
		RuntimeNodeID:    item.ID,
		AgentRoundID:     item.AgentRoundID,
		SubjectID:        item.SubjectID,
		Status:           string(item.Status),
		ResultSummary:    item.ResultSummary,
		ErrorCode:        item.ErrorCode,
		ErrorSummary:     item.ErrorSummary,
		SummaryTruncated: item.SummaryTruncated,
		DurationMS:       runtimeNodeDurationMS(item),
		StartedAt:        &startedAt,
		FinishedAt:       finishedAt,
		Artifacts:        runtimeGraphNodeArtifacts(item),
	}
}

func runtimeNodeDurationMS(item protocol.ExecutionRuntimeNodeRun) int64 {
	if item.DurationMS > 0 {
		return item.DurationMS
	}
	if item.FinishedAt == nil || item.StartedAt.IsZero() {
		return 0
	}
	duration := item.FinishedAt.Sub(item.StartedAt)
	if duration <= 0 {
		return 0
	}
	return duration.Milliseconds()
}

func runtimeGraphPromotedNodeIDs(
	graph protocol.ExecutionRuntimeGraph,
) map[string]struct{} {
	result := make(map[string]struct{})
	for _, node := range graph.Nodes {
		if len(node.Artifacts) > 0 ||
			runtimeGraphVisibilityHint(node) ||
			runtimeGraphToolActionVisible(node) {
			result[node.ID] = struct{}{}
		}
	}
	for _, edge := range graph.Edges {
		if edge.Kind != protocol.ExecutionRuntimeEdgeLoopBack &&
			edge.Kind != protocol.ExecutionRuntimeEdgeRetry {
			continue
		}
		result[edge.SourceNodeID] = struct{}{}
		result[edge.TargetNodeID] = struct{}{}
	}
	promoteRuntimeGraphRecoverySuccesses(graph, result)
	return result
}

// promoteRuntimeGraphRecoverySuccesses 只负责避免画布停留在“失败但看不见后来
// 成功”的误导状态。它不创建 retry 边，也不把两个 NodeRun 合并：只有 durable
// retry_of 事实才能表达重试关系；同一直接 owner 下同类工具的时间顺序只用于
// 提升最后一个失败后成功的独立节点。
func promoteRuntimeGraphRecoverySuccesses(
	graph protocol.ExecutionRuntimeGraph,
	promoted map[string]struct{},
) {
	groups := make(map[string][]protocol.ExecutionRuntimeNodeRun)
	for _, node := range graph.Nodes {
		if node.Kind != protocol.ExecutionRuntimeNodeTool {
			continue
		}
		ownerKey := firstNonEmpty(
			strings.TrimSpace(node.ParentSubjectID),
			strings.TrimSpace(node.AgentRoundID),
		)
		actionKey := runtimeGraphCanonicalToolLeaf(node.Name)
		if ownerKey == "" || actionKey == "" {
			continue
		}
		key := strings.TrimSpace(node.AgentID) + "\x00" + ownerKey + "\x00" + actionKey
		groups[key] = append(groups[key], node)
	}
	for _, nodes := range groups {
		slices.SortFunc(nodes, func(left, right protocol.ExecutionRuntimeNodeRun) int {
			if order := left.StartedAt.Compare(right.StartedAt); order != 0 {
				return order
			}
			return strings.Compare(left.ID, right.ID)
		})
		failureObserved := false
		latestRecoveryID := ""
		for _, node := range nodes {
			switch node.Status {
			case protocol.ExecutionRuntimeNodeFailed,
				protocol.ExecutionRuntimeNodeCancelled,
				protocol.ExecutionRuntimeNodeInterrupted:
				failureObserved = true
			case protocol.ExecutionRuntimeNodeSucceeded:
				if failureObserved {
					latestRecoveryID = node.ID
				}
			}
		}
		if latestRecoveryID != "" {
			promoted[latestRecoveryID] = struct{}{}
		}
	}
}

func runtimeGraphVisibilityHint(item protocol.ExecutionRuntimeNodeRun) bool {
	value, _ := item.Metadata[protocol.ExecutionRuntimeMetadataWorkGraphVisibility].(string)
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(protocol.ExecutionGraphNodeNested),
		string(protocol.ExecutionGraphNodePrimary):
		return true
	default:
		return false
	}
}

func executionGraphEdgeKey(
	kind protocol.ExecutionGraphEdgeKind,
	sourceID string,
	targetID string,
) string {
	return string(kind) + "\x00" + sourceID + "\x00" + targetID
}

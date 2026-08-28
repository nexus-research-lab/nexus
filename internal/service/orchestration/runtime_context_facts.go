// INPUT: owner/session-scoped Runtime Graph、当前 actor identity 与已授权的 managed Execution snapshot。
// OUTPUT: 默认只含当前 Agent 的 active/异常/Artifact/control-return；显式 inspect 才附带成功历史。
// POS: Runtime Graph 到 <nexus_execution_context> 的有界事实反馈层；不生成路线、重试或工具建议。
package orchestration

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const runtimeContextFactLimit = 8

func (s *Service) populateRuntimeGraphContext(
	ctx context.Context,
	actor ActorContext,
	snapshot *protocol.ExecutionSnapshot,
	options *ExecutionContextOptions,
) {
	if options == nil || snapshot == nil || strings.TrimSpace(snapshot.Execution.ID) == "" {
		return
	}
	repository, ok := s.repository.(runtimeGraphRepository)
	if !ok || repository == nil {
		return
	}
	graph, err := repository.GetRuntimeGraph(
		ctx,
		strings.TrimSpace(actor.OwnerUserID),
		strings.TrimSpace(actor.SessionKey),
		strings.TrimSpace(snapshot.Execution.ID),
		strings.TrimSpace(snapshot.Execution.RootRoundID),
	)
	if err != nil {
		options.RuntimeGraphUnavailable = true
		return
	}
	if len(graph.Nodes) == 0 && graph.NodeTotal == 0 {
		return
	}
	options.RuntimeGraph = &graph
	options.RuntimeGraphRelation = "current_execution"
}

func renderRuntimeGraphFacts(
	output *strings.Builder,
	options ExecutionContextOptions,
) {
	if options.RuntimeGraphUnavailable {
		output.WriteString("\n  <runtime_facts available=\"false\" reason=\"projection_unavailable\" />")
		return
	}
	if options.RuntimeGraph == nil {
		return
	}
	graph := options.RuntimeGraph
	actorAgentID := strings.TrimSpace(options.ActorAgentID)
	visibleNodes := make([]protocol.ExecutionRuntimeNodeRun, 0)
	visibleNodeByID := make(map[string]protocol.ExecutionRuntimeNodeRun)
	for _, node := range graph.Nodes {
		// Runtime facts can contain unaccepted intermediate output. Even a
		// coordinator receives detailed runtime facts only for its own Agent;
		// cross-Agent responsibility remains in the managed graph digest.
		if actorAgentID == "" || strings.TrimSpace(node.AgentID) != actorAgentID {
			continue
		}
		// nexus.command 的结果已经由权威 context/outcome 投影；再次摘要会让
		// inspect 把上一次 inspect 递归带回模型。
		if node.Kind == protocol.ExecutionRuntimeNodeTool &&
			runtimeGraphIsCommandTransport(node) {
			continue
		}
		visibleNodes = append(visibleNodes, node)
		visibleNodeByID[node.ID] = node
	}
	slices.SortFunc(visibleNodes, func(left, right protocol.ExecutionRuntimeNodeRun) int {
		if order := right.UpdatedAt.Compare(left.UpdatedAt); order != 0 {
			return order
		}
		return strings.Compare(right.ID, left.ID)
	})
	nodeTotal := graph.NodeTotal
	if nodeTotal == 0 {
		nodeTotal = len(graph.Nodes)
	}
	edgeTotal := graph.EdgeTotal
	if edgeTotal == 0 {
		edgeTotal = len(graph.Edges)
	}
	active := make([]protocol.ExecutionRuntimeNodeRun, 0)
	attention := make([]protocol.ExecutionRuntimeNodeRun, 0)
	succeeded := make([]protocol.ExecutionRuntimeNodeRun, 0)
	for _, node := range visibleNodes {
		if node.Kind == protocol.ExecutionRuntimeNodeAgent {
			continue
		}
		switch node.Status {
		case protocol.ExecutionRuntimeNodeRunning:
			active = append(active, node)
		case protocol.ExecutionRuntimeNodeSucceeded:
			if options.IncludeRuntimeHistory {
				succeeded = append(succeeded, node)
			}
		default:
			attention = append(attention, node)
		}
	}
	var facts strings.Builder
	renderRuntimeFactNodes(&facts, "active_nodes", active)
	renderRuntimeFactNodes(&facts, "attention_nodes", attention)
	if options.IncludeRuntimeHistory {
		renderRuntimeFactNodes(&facts, "successful_nodes", succeeded)
	}
	renderRuntimeFactArtifacts(&facts, visibleNodes)
	renderRuntimeFactControlEdges(&facts, graph.Edges, visibleNodeByID)
	if facts.Len() == 0 {
		return
	}
	fmt.Fprintf(
		output,
		"\n  <runtime_facts available=\"true\" mode=\"observed_facts_only\" relation=\"%s\" graph_id=\"%s\" partial=\"%t\" node_total=\"%d\" edge_total=\"%d\" visible_node_total=\"%d\">",
		xmlValue(options.RuntimeGraphRelation),
		xmlValue(graph.GraphID),
		graph.NodesTruncated || graph.EdgesTruncated,
		nodeTotal,
		edgeTotal,
		len(visibleNodes),
	)
	output.WriteString(facts.String())
	output.WriteString("\n  </runtime_facts>")
}

func renderRuntimeFactNodes(
	output *strings.Builder,
	container string,
	nodes []protocol.ExecutionRuntimeNodeRun,
) {
	if len(nodes) == 0 {
		return
	}
	limit := min(len(nodes), runtimeContextFactLimit)
	fmt.Fprintf(output, "\n    <%s", container)
	if len(nodes) > limit {
		fmt.Fprintf(output, " truncated=\"true\" total=\"%d\"", len(nodes))
	}
	output.WriteString(">")
	for _, node := range nodes[:limit] {
		fmt.Fprintf(
			output,
			"\n      <node id=\"%s\" kind=\"%s\" subject_id=\"%s\" name=\"%s\" status=\"%s\" updated_at=\"%s\">",
			xmlValue(node.ID),
			xmlValue(string(node.Kind)),
			xmlValue(node.SubjectID),
			xmlValue(node.Name),
			xmlValue(string(node.Status)),
			xmlValue(node.UpdatedAt.UTC().Format(time.RFC3339Nano)),
		)
		if strings.TrimSpace(node.ResultSummary) != "" {
			writeXMLTextElement(output, 8, "result_summary", node.ResultSummary)
		}
		if strings.TrimSpace(node.ErrorSummary) != "" || strings.TrimSpace(node.ErrorCode) != "" {
			fmt.Fprintf(
				output,
				"\n        <error code=\"%s\">%s</error>",
				xmlValue(node.ErrorCode),
				xmlValue(node.ErrorSummary),
			)
		}
		output.WriteString("\n      </node>")
	}
	fmt.Fprintf(output, "\n    </%s>", container)
}

func renderRuntimeFactArtifacts(
	output *strings.Builder,
	nodes []protocol.ExecutionRuntimeNodeRun,
) {
	type runtimeArtifactFact struct {
		nodeID   string
		artifact protocol.WorkspaceFileArtifactBlock
	}
	facts := make([]runtimeArtifactFact, 0)
	seen := make(map[string]struct{})
	for _, node := range nodes {
		for _, artifact := range runtimeGraphNodeArtifacts(node) {
			key := strings.TrimSpace(artifact.ID)
			if key == "" {
				key = strings.TrimSpace(artifact.SourceToolUseID) + "\x00" + strings.TrimSpace(artifact.Path)
			}
			if key == "\x00" {
				continue
			}
			if _, duplicate := seen[key]; duplicate {
				continue
			}
			seen[key] = struct{}{}
			facts = append(facts, runtimeArtifactFact{nodeID: node.ID, artifact: artifact})
		}
	}
	if len(facts) == 0 {
		return
	}
	limit := min(len(facts), runtimeContextFactLimit)
	output.WriteString("\n    <artifacts")
	if len(facts) > limit {
		fmt.Fprintf(output, " truncated=\"true\" total=\"%d\"", len(facts))
	}
	output.WriteString(">")
	for _, fact := range facts[:limit] {
		fmt.Fprintf(
			output,
			"\n      <artifact node_id=\"%s\" tool_use_id=\"%s\" path=\"%s\" kind=\"%s\" />",
			xmlValue(fact.nodeID),
			xmlValue(fact.artifact.SourceToolUseID),
			xmlValue(fact.artifact.Path),
			xmlValue(fact.artifact.ArtifactKind),
		)
	}
	output.WriteString("\n    </artifacts>")
}

func renderRuntimeFactControlEdges(
	output *strings.Builder,
	edges []protocol.ExecutionRuntimeEdgeRun,
	visibleNodeByID map[string]protocol.ExecutionRuntimeNodeRun,
) {
	controlEdges := make([]protocol.ExecutionRuntimeEdgeRun, 0)
	for _, edge := range edges {
		if edge.Kind != protocol.ExecutionRuntimeEdgeRetry &&
			edge.Kind != protocol.ExecutionRuntimeEdgeLoopBack {
			continue
		}
		if _, visible := visibleNodeByID[edge.SourceNodeID]; !visible {
			continue
		}
		if _, visible := visibleNodeByID[edge.TargetNodeID]; !visible {
			continue
		}
		controlEdges = append(controlEdges, edge)
	}
	if len(controlEdges) == 0 {
		return
	}
	slices.SortFunc(controlEdges, func(left, right protocol.ExecutionRuntimeEdgeRun) int {
		if order := right.CreatedAt.Compare(left.CreatedAt); order != 0 {
			return order
		}
		return strings.Compare(right.ID, left.ID)
	})
	limit := min(len(controlEdges), runtimeContextFactLimit)
	output.WriteString("\n    <control_edges")
	if len(controlEdges) > limit {
		fmt.Fprintf(output, " truncated=\"true\" total=\"%d\"", len(controlEdges))
	}
	output.WriteString(">")
	for _, edge := range controlEdges[:limit] {
		source := visibleNodeByID[edge.SourceNodeID]
		target := visibleNodeByID[edge.TargetNodeID]
		fmt.Fprintf(
			output,
			"\n      <edge id=\"%s\" kind=\"%s\" source_node_id=\"%s\" source_subject_id=\"%s\" target_node_id=\"%s\" target_subject_id=\"%s\" observed_at=\"%s\" />",
			xmlValue(edge.ID),
			xmlValue(string(edge.Kind)),
			xmlValue(edge.SourceNodeID),
			xmlValue(source.SubjectID),
			xmlValue(edge.TargetNodeID),
			xmlValue(target.SubjectID),
			xmlValue(edge.CreatedAt.UTC().Format(time.RFC3339Nano)),
		)
	}
	output.WriteString("\n    </control_edges>")
}

// INPUT: 已持久化 Runtime Graph、当前 owner/session 与迁移读取的受限 Subagent task/ToolRun。
// OUTPUT: 为版本化迁移补齐 exact Subagent 与其 child Tool 节点的脱敏运行图副本。
// POS: Runtime Graph 历史迁移的纯投影；普通视图不调用，不从 prompt/output 猜测节点。
package orchestration

import (
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/textutil"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// RuntimeGraphSubagentToolHistory 是旧执行 transcript 中可安全恢复的最小 ToolRun。
type RuntimeGraphSubagentToolHistory struct {
	ParentToolUseID string
	TaskID          string
	AgentID         string
	ToolUseID       string
	Name            string
	Status          string
	StartedAt       int64
	FinishedAt      int64
}

// RuntimeGraphSubagentTaskHistory 是父会话中可安全恢复的最小 Subagent task。
// ToolUseID 是与 managed child Attempt 和 Agent launch Tool 的唯一关联依据。
type RuntimeGraphSubagentTaskHistory struct {
	TaskID         string
	AgentID        string
	AgentType      string
	ChildSessionID string
	Description    string
	Summary        string
	Name           string
	Status         string
	ToolUseID      string
	StartedAt      int64
	UpdatedAt      int64
}

// RuntimeGraphNeedsHistoryBackfill avoids reading transcripts for graphs that
// contain no exact Subagent launch identity.
func RuntimeGraphNeedsHistoryBackfill(graph protocol.ExecutionRuntimeGraph) bool {
	for _, node := range graph.Nodes {
		if node.Kind == protocol.ExecutionRuntimeNodeSubagent || (node.Kind == protocol.ExecutionRuntimeNodeTool && runtimeGraphCanonicalToolLeaf(node.Name) == "agent") {
			return true
		}
	}
	return false
}

// RecoverRuntimeGraphHistory is a pure migration projection. Callers persist only
// missing identities and never use it as a fallback on normal view reads.
func RecoverRuntimeGraphHistory(ownerUserID, sessionKey string, graph protocol.ExecutionRuntimeGraph, tasks []RuntimeGraphSubagentTaskHistory, history []RuntimeGraphSubagentToolHistory) protocol.ExecutionRuntimeGraph {
	subagentByLaunchToolUseID := make(map[string]protocol.ExecutionRuntimeNodeRun)
	existingToolUseID := make(map[string]struct{})
	agentNodeByRound := make(map[string]protocol.ExecutionRuntimeNodeRun)
	for _, node := range graph.Nodes {
		switch node.Kind {
		case protocol.ExecutionRuntimeNodeAgent:
			if node.AgentRoundID != "" {
				agentNodeByRound[node.AgentRoundID] = node
			}
		case protocol.ExecutionRuntimeNodeSubagent:
			for _, launchToolUseID := range []string{
				runtimeGraphMetadataString(node, "tool_use_id"),
				strings.TrimSpace(node.SubjectID),
			} {
				if launchToolUseID != "" {
					subagentByLaunchToolUseID[launchToolUseID] = node
				}
			}
		case protocol.ExecutionRuntimeNodeTool:
			subjectID := strings.TrimSpace(node.SubjectID)
			if subjectID != "" {
				existingToolUseID[subjectID] = struct{}{}
			}
			// 早期 observer 只保存 Agent launch Tool；Execution view 会按
			// exact ToolUseID 把它合并进 durable Subagent Attempt。
			if subjectID != "" && runtimeGraphCanonicalToolLeaf(node.Name) == "agent" {
				subagentByLaunchToolUseID[subjectID] = node
			}
		}
	}
	if len(subagentByLaunchToolUseID) == 0 {
		return graph
	}
	graph = recoverRuntimeGraphSubagentTasks(graph, tasks, subagentByLaunchToolUseID, agentNodeByRound, ownerUserID, sessionKey)
	for _, item := range history {
		parent, exists := subagentByLaunchToolUseID[strings.TrimSpace(item.ParentToolUseID)]
		toolUseID := strings.TrimSpace(item.ToolUseID)
		name := strings.TrimSpace(item.Name)
		if !exists || toolUseID == "" || name == "" {
			continue
		}
		if _, duplicate := existingToolUseID[toolUseID]; duplicate {
			continue
		}
		startedAt := runtimeGraphHistoryTime(item.StartedAt, parent.StartedAt)
		finishedAt := runtimeGraphHistoryFinishedAt(item.FinishedAt, startedAt)
		node := protocol.ExecutionRuntimeNodeRun{
			ID: stableRuntimeGraphID(
				"runtime_node",
				textutil.FirstNonEmpty(parent.OwnerUserID, ownerUserID),
				textutil.FirstNonEmpty(parent.SessionKey, sessionKey),
				parent.AgentRoundID,
				string(protocol.ExecutionRuntimeNodeTool),
				toolUseID,
			),
			GraphID:         parent.GraphID,
			OwnerUserID:     textutil.FirstNonEmpty(parent.OwnerUserID, ownerUserID),
			SessionKey:      textutil.FirstNonEmpty(parent.SessionKey, sessionKey),
			ExecutionID:     parent.ExecutionID,
			Kind:            protocol.ExecutionRuntimeNodeTool,
			SubjectID:       toolUseID,
			ParentSubjectID: strings.TrimSpace(item.ParentToolUseID),
			RootRoundID:     parent.RootRoundID,
			RuntimeRoundID:  parent.RuntimeRoundID,
			AgentRoundID:    parent.AgentRoundID,
			AgentID:         textutil.FirstNonEmpty(strings.TrimSpace(item.AgentID), parent.AgentID),
			Name:            name,
			Status:          runtimeGraphHistoryStatus(item.Status),
			StartedAt:       startedAt,
			UpdatedAt:       startedAt,
			FinishedAt:      finishedAt,
			Metadata: map[string]any{
				"history_recovered": true,
				"subagent_task_id":  strings.TrimSpace(item.TaskID),
			},
		}
		if finishedAt != nil {
			node.UpdatedAt = *finishedAt
			if duration := finishedAt.Sub(startedAt); duration > 0 {
				node.DurationMS = duration.Milliseconds()
			}
		}
		node.Failed = node.Status == protocol.ExecutionRuntimeNodeFailed
		edge := protocol.ExecutionRuntimeEdgeRun{
			GraphID:      node.GraphID,
			OwnerUserID:  node.OwnerUserID,
			SessionKey:   node.SessionKey,
			SourceNodeID: parent.ID,
			TargetNodeID: node.ID,
			Kind:         protocol.ExecutionRuntimeEdgeInvoke,
			CreatedAt:    startedAt,
		}
		edge.ID = runtimeGraphEdgeID(edge)
		graph.Nodes = append(graph.Nodes, node)
		graph.Edges = append(graph.Edges, edge)
		graph.NodeTotal++
		graph.EdgeTotal++
		existingToolUseID[toolUseID] = struct{}{}
	}
	return graph
}

func recoverRuntimeGraphSubagentTasks(
	graph protocol.ExecutionRuntimeGraph,
	tasks []RuntimeGraphSubagentTaskHistory,
	subagentByLaunchToolUseID map[string]protocol.ExecutionRuntimeNodeRun,
	agentNodeByRound map[string]protocol.ExecutionRuntimeNodeRun,
	ownerUserID string,
	sessionKey string,
) protocol.ExecutionRuntimeGraph {
	existingSubagentBySubject := make(map[string]struct{})
	for _, node := range graph.Nodes {
		if node.Kind == protocol.ExecutionRuntimeNodeSubagent {
			existingSubagentBySubject[strings.TrimSpace(node.SubjectID)] = struct{}{}
		}
	}
	for _, task := range tasks {
		launchToolUseID := strings.TrimSpace(task.ToolUseID)
		launch, exists := subagentByLaunchToolUseID[launchToolUseID]
		subjectID := textutil.FirstNonEmpty(
			strings.TrimSpace(task.TaskID),
			strings.TrimSpace(task.AgentID),
		)
		if !exists || launchToolUseID == "" || subjectID == "" {
			continue
		}
		if launch.Kind == protocol.ExecutionRuntimeNodeSubagent {
			subagentByLaunchToolUseID[launchToolUseID] = launch
			continue
		}
		if _, duplicate := existingSubagentBySubject[subjectID]; duplicate {
			continue
		}
		startedAt := runtimeGraphHistoryTime(task.StartedAt, launch.StartedAt)
		status := runtimeGraphHistoryStatus(task.Status)
		finishedAt := (*time.Time)(nil)
		if status != protocol.ExecutionRuntimeNodeRunning {
			finishedAt = runtimeGraphHistoryFinishedAt(task.UpdatedAt, startedAt)
			if finishedAt == nil {
				value := startedAt
				finishedAt = &value
			}
		}
		node := protocol.ExecutionRuntimeNodeRun{
			ID: stableRuntimeGraphID(
				"runtime_node",
				textutil.FirstNonEmpty(launch.OwnerUserID, ownerUserID),
				textutil.FirstNonEmpty(launch.SessionKey, sessionKey),
				launch.AgentRoundID,
				string(protocol.ExecutionRuntimeNodeSubagent),
				subjectID,
			),
			GraphID:         launch.GraphID,
			OwnerUserID:     textutil.FirstNonEmpty(launch.OwnerUserID, ownerUserID),
			SessionKey:      textutil.FirstNonEmpty(launch.SessionKey, sessionKey),
			ExecutionID:     launch.ExecutionID,
			Kind:            protocol.ExecutionRuntimeNodeSubagent,
			SubjectID:       subjectID,
			ParentSubjectID: strings.TrimSpace(launch.ParentSubjectID),
			RootRoundID:     launch.RootRoundID,
			RuntimeRoundID:  launch.RuntimeRoundID,
			AgentRoundID:    launch.AgentRoundID,
			AgentID:         textutil.FirstNonEmpty(strings.TrimSpace(task.AgentID), launch.AgentID),
			Name: textutil.FirstNonEmpty(
				strings.TrimSpace(task.Name),
				strings.TrimSpace(task.AgentType),
				"Subagent",
			),
			Description: textutil.FirstNonEmpty(
				strings.TrimSpace(task.Description),
				strings.TrimSpace(task.Summary),
			),
			Status:     status,
			Failed:     status == protocol.ExecutionRuntimeNodeFailed,
			StartedAt:  startedAt,
			UpdatedAt:  startedAt,
			FinishedAt: finishedAt,
			Metadata: map[string]any{
				"history_recovered": true,
				"tool_use_id":       launchToolUseID,
				"sdk_task_id":       subjectID,
				"child_session_id":  strings.TrimSpace(task.ChildSessionID),
			},
		}
		if finishedAt != nil {
			node.UpdatedAt = *finishedAt
			if duration := finishedAt.Sub(startedAt); duration > 0 {
				node.DurationMS = duration.Milliseconds()
			}
		}
		source := agentNodeByRound[launch.AgentRoundID]
		if source.ID == "" {
			source = launch
		}
		edge := protocol.ExecutionRuntimeEdgeRun{
			GraphID:      node.GraphID,
			OwnerUserID:  node.OwnerUserID,
			SessionKey:   node.SessionKey,
			SourceNodeID: source.ID,
			TargetNodeID: node.ID,
			Kind:         protocol.ExecutionRuntimeEdgeSpawn,
			CreatedAt:    startedAt,
		}
		edge.ID = runtimeGraphEdgeID(edge)
		graph.Nodes = append(graph.Nodes, node)
		graph.Edges = append(graph.Edges, edge)
		if graph.NodeTotal < len(graph.Nodes) {
			graph.NodeTotal = len(graph.Nodes)
		}
		if graph.EdgeTotal < len(graph.Edges) {
			graph.EdgeTotal = len(graph.Edges)
		}
		existingSubagentBySubject[subjectID] = struct{}{}
		subagentByLaunchToolUseID[launchToolUseID] = node
	}
	return graph
}

func runtimeGraphHistoryTime(milliseconds int64, fallback time.Time) time.Time {
	if milliseconds > 0 {
		return time.UnixMilli(milliseconds).UTC()
	}
	if !fallback.IsZero() {
		return fallback.UTC()
	}
	return time.Unix(0, 0).UTC()
}

func runtimeGraphHistoryFinishedAt(milliseconds int64, startedAt time.Time) *time.Time {
	if milliseconds <= 0 {
		return nil
	}
	value := time.UnixMilli(milliseconds).UTC()
	if value.Before(startedAt) {
		value = startedAt
	}
	return &value
}

func runtimeGraphHistoryStatus(value string) protocol.ExecutionRuntimeNodeStatus {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "succeeded", "completed", "complete":
		return protocol.ExecutionRuntimeNodeSucceeded
	case "failed", "error":
		return protocol.ExecutionRuntimeNodeFailed
	case "cancelled", "canceled":
		return protocol.ExecutionRuntimeNodeCancelled
	case "interrupted", "stopped":
		return protocol.ExecutionRuntimeNodeInterrupted
	default:
		return protocol.ExecutionRuntimeNodeRunning
	}
}

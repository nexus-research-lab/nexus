// INPUT: 已持久化 Runtime Graph、当前 owner/session 与受限 Subagent task/ToolRun provider。
// OUTPUT: 为旧执行补齐 exact Subagent 与其 child Tool 节点的只读、脱敏运行图副本。
// POS: Runtime Graph 兼容投影层；不改写历史库，不从 prompt/output 猜测节点。
package orchestration

import (
	"context"
	"strings"
	"time"

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

// RuntimeGraphSubagentToolHistoryProvider 在消费侧提供受限历史；实现不得返回
// prompt、Tool input、Tool output 或未经脱敏的 result。
type RuntimeGraphSubagentToolHistoryProvider interface {
	ListRuntimeGraphSubagentToolHistory(
		context.Context,
		string,
		string,
	) ([]RuntimeGraphSubagentToolHistory, error)
}

// RuntimeGraphSubagentTaskHistoryProvider 是旧执行的 task identity/status
// 兼容端口；只有实现同时提供该能力时才恢复 Subagent，保持注入向后兼容。
type RuntimeGraphSubagentTaskHistoryProvider interface {
	ListRuntimeGraphSubagentTaskHistory(
		context.Context,
		string,
		string,
	) ([]RuntimeGraphSubagentTaskHistory, error)
}

// SetRuntimeGraphSubagentToolHistoryProvider 注入旧执行的只读 transcript 兼容投影。
func (s *Service) SetRuntimeGraphSubagentToolHistoryProvider(
	provider RuntimeGraphSubagentToolHistoryProvider,
) {
	s.subagentToolHistory = provider
}

func (s *Service) mergeRuntimeGraphSubagentToolHistory(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
	graph protocol.ExecutionRuntimeGraph,
) protocol.ExecutionRuntimeGraph {
	if s == nil || s.subagentToolHistory == nil || len(graph.Nodes) == 0 {
		return graph
	}
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
	if provider, ok := s.subagentToolHistory.(RuntimeGraphSubagentTaskHistoryProvider); ok {
		tasks, taskErr := provider.ListRuntimeGraphSubagentTaskHistory(
			ctx,
			ownerUserID,
			sessionKey,
		)
		if taskErr == nil {
			graph = recoverRuntimeGraphSubagentTasks(
				graph,
				tasks,
				subagentByLaunchToolUseID,
				agentNodeByRound,
				ownerUserID,
				sessionKey,
			)
		}
	}
	history, err := s.subagentToolHistory.ListRuntimeGraphSubagentToolHistory(
		ctx,
		ownerUserID,
		sessionKey,
	)
	if err != nil {
		// WorkGraph 历史兼容读取保持 fail-open；权威 Execution/Runtime Graph
		// 不能因为 transcript 已清理或暂时不可读而变成 500。
		return graph
	}
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
				firstNonEmpty(parent.OwnerUserID, ownerUserID),
				firstNonEmpty(parent.SessionKey, sessionKey),
				parent.AgentRoundID,
				string(protocol.ExecutionRuntimeNodeTool),
				toolUseID,
			),
			GraphID:         parent.GraphID,
			OwnerUserID:     firstNonEmpty(parent.OwnerUserID, ownerUserID),
			SessionKey:      firstNonEmpty(parent.SessionKey, sessionKey),
			ExecutionID:     parent.ExecutionID,
			Kind:            protocol.ExecutionRuntimeNodeTool,
			SubjectID:       toolUseID,
			ParentSubjectID: strings.TrimSpace(item.ParentToolUseID),
			RootRoundID:     parent.RootRoundID,
			RuntimeRoundID:  parent.RuntimeRoundID,
			AgentRoundID:    parent.AgentRoundID,
			AgentID:         firstNonEmpty(strings.TrimSpace(item.AgentID), parent.AgentID),
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
		subjectID := firstNonEmpty(
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
				firstNonEmpty(launch.OwnerUserID, ownerUserID),
				firstNonEmpty(launch.SessionKey, sessionKey),
				launch.AgentRoundID,
				string(protocol.ExecutionRuntimeNodeSubagent),
				subjectID,
			),
			GraphID:         launch.GraphID,
			OwnerUserID:     firstNonEmpty(launch.OwnerUserID, ownerUserID),
			SessionKey:      firstNonEmpty(launch.SessionKey, sessionKey),
			ExecutionID:     launch.ExecutionID,
			Kind:            protocol.ExecutionRuntimeNodeSubagent,
			SubjectID:       subjectID,
			ParentSubjectID: strings.TrimSpace(launch.ParentSubjectID),
			RootRoundID:     launch.RootRoundID,
			RuntimeRoundID:  launch.RuntimeRoundID,
			AgentRoundID:    launch.AgentRoundID,
			AgentID:         firstNonEmpty(strings.TrimSpace(task.AgentID), launch.AgentID),
			Name: firstNonEmpty(
				strings.TrimSpace(task.Name),
				strings.TrimSpace(task.AgentType),
				"Subagent",
			),
			Description: firstNonEmpty(
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

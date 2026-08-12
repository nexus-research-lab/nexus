// INPUT: provider-neutral Runtime NodeRun / EdgeRun 与 owner/session 查询边界。
// OUTPUT: 幂等 upsert、内部观测用有界运行图，以及可见性投影前的完整 WorkGraph 运行事实。
// POS: Execution Orchestration Repository 中不参与 command CAS 的观测事实存储。
package orchestration

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const runtimeGraphNodeSelectColumns = `
SELECT node_run_id, graph_id, owner_user_id, session_key, execution_id,
       node_kind, subject_id, parent_subject_id, root_round_id,
       runtime_round_id, agent_round_id, agent_id, name, description,
       status, failed, result_summary, error_code, error_summary,
       summary_truncated, duration_ms, started_at, updated_at, finished_at,
       metadata_json
FROM runtime_graph_node_runs`

// UpsertRuntimeGraphNode 幂等推进一个 runtime NodeRun；终态不会被迟到的 started
// 或 progress 事件重新打开。该观测写失败不得触发业务工具重放。
func (r *Repository) UpsertRuntimeGraphNode(
	ctx context.Context,
	item protocol.ExecutionRuntimeNodeRun,
) error {
	if err := validateRuntimeGraphNode(item); err != nil {
		return err
	}
	metadataJSON, err := marshalMap(item.Metadata)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
INSERT INTO runtime_graph_node_runs (
    node_run_id, graph_id, owner_user_id, session_key, execution_id,
    node_kind, subject_id, parent_subject_id, root_round_id,
    runtime_round_id, agent_round_id, agent_id, name, description,
    status, failed, result_summary, error_code, error_summary,
    summary_truncated, duration_ms, started_at, updated_at, finished_at,
    metadata_json
) VALUES (`+
		r.bind(1)+`, `+r.bind(2)+`, `+r.bind(3)+`, `+r.bind(4)+`, `+r.bind(5)+`, `+
		r.bind(6)+`, `+r.bind(7)+`, `+r.bind(8)+`, `+r.bind(9)+`, `+
		r.bind(10)+`, `+r.bind(11)+`, `+r.bind(12)+`, `+r.bind(13)+`, `+r.bind(14)+`, `+
		r.bind(15)+`, `+r.bind(16)+`, `+r.bind(17)+`, `+r.bind(18)+`, `+r.bind(19)+`, `+
		r.bind(20)+`, `+r.bind(21)+`, `+r.bind(22)+`, `+r.bind(23)+`, `+r.bind(24)+`, `+r.jsonBind(25)+`
)
ON CONFLICT (node_run_id) DO UPDATE SET
    execution_id = COALESCE(runtime_graph_node_runs.execution_id, excluded.execution_id),
    parent_subject_id = COALESCE(NULLIF(excluded.parent_subject_id, ''), runtime_graph_node_runs.parent_subject_id),
    agent_id = COALESCE(NULLIF(excluded.agent_id, ''), runtime_graph_node_runs.agent_id),
    name = COALESCE(NULLIF(excluded.name, ''), runtime_graph_node_runs.name),
    description = COALESCE(NULLIF(excluded.description, ''), runtime_graph_node_runs.description),
    status = CASE
        WHEN runtime_graph_node_runs.status <> 'running' THEN runtime_graph_node_runs.status
        ELSE excluded.status
    END,
    failed = CASE
        WHEN runtime_graph_node_runs.failed OR excluded.failed THEN TRUE
        ELSE FALSE
    END,
    result_summary = COALESCE(NULLIF(excluded.result_summary, ''), runtime_graph_node_runs.result_summary),
    error_code = COALESCE(NULLIF(excluded.error_code, ''), runtime_graph_node_runs.error_code),
    error_summary = COALESCE(NULLIF(excluded.error_summary, ''), runtime_graph_node_runs.error_summary),
    summary_truncated = CASE
        WHEN runtime_graph_node_runs.summary_truncated OR excluded.summary_truncated THEN TRUE
        ELSE FALSE
    END,
    duration_ms = CASE
        WHEN excluded.duration_ms > 0 THEN excluded.duration_ms
        ELSE runtime_graph_node_runs.duration_ms
    END,
    updated_at = CASE
        WHEN excluded.updated_at > runtime_graph_node_runs.updated_at THEN excluded.updated_at
        ELSE runtime_graph_node_runs.updated_at
    END,
    finished_at = COALESCE(runtime_graph_node_runs.finished_at, excluded.finished_at),
    metadata_json = excluded.metadata_json`,
		item.ID,
		item.GraphID,
		item.OwnerUserID,
		item.SessionKey,
		nullString(item.ExecutionID),
		item.Kind,
		item.SubjectID,
		nullString(item.ParentSubjectID),
		item.RootRoundID,
		item.RuntimeRoundID,
		item.AgentRoundID,
		nullString(item.AgentID),
		nullString(item.Name),
		nullString(item.Description),
		item.Status,
		item.Failed,
		nullString(item.ResultSummary),
		nullString(item.ErrorCode),
		nullString(item.ErrorSummary),
		item.SummaryTruncated,
		item.DurationMS,
		r.timestamp(item.StartedAt),
		r.timestamp(item.UpdatedAt),
		nullTime(item.FinishedAt),
		metadataJSON,
	)
	return err
}

// UpsertRuntimeGraphEdge 幂等记录一个已存在 NodeRun 间的运行边。
func (r *Repository) UpsertRuntimeGraphEdge(
	ctx context.Context,
	item protocol.ExecutionRuntimeEdgeRun,
) error {
	if err := validateRuntimeGraphEdge(item); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO runtime_graph_edge_runs (
    edge_run_id, graph_id, owner_user_id, session_key,
    source_node_run_id, target_node_run_id, edge_kind, created_at
) VALUES (`+
		r.bind(1)+`, `+r.bind(2)+`, `+r.bind(3)+`, `+r.bind(4)+`, `+
		r.bind(5)+`, `+r.bind(6)+`, `+r.bind(7)+`, `+r.bind(8)+`
)
ON CONFLICT (edge_run_id) DO NOTHING`,
		item.ID,
		item.GraphID,
		item.OwnerUserID,
		item.SessionKey,
		item.SourceNodeID,
		item.TargetNodeID,
		item.Kind,
		r.timestamp(item.CreatedAt),
	)
	return err
}

// ReconcileRuntimeGraphAgent 在同一 session/Agent 开始新一轮时收口进程退出后
// 遗留的旧 running 节点；不同 Agent 的 Room 并发不受影响。
func (r *Repository) ReconcileRuntimeGraphAgent(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
	agentID string,
	currentAgentRoundID string,
	finishedAt time.Time,
) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE runtime_graph_node_runs
SET status = 'interrupted',
    failed = TRUE,
    updated_at = `+r.bind(1)+`,
    finished_at = COALESCE(finished_at, `+r.bind(2)+`)
WHERE owner_user_id = `+r.bind(3)+`
  AND session_key = `+r.bind(4)+`
  AND status = 'running'
  AND agent_round_id <> `+r.bind(5)+`
  AND agent_round_id IN (
      SELECT agent_round_id
      FROM runtime_graph_node_runs
      WHERE owner_user_id = `+r.bind(6)+`
        AND session_key = `+r.bind(7)+`
        AND node_kind = 'agent'
        AND agent_id = `+r.bind(8)+`
        AND status = 'running'
        AND agent_round_id <> `+r.bind(9)+`
  )`,
		r.timestamp(finishedAt),
		r.timestamp(finishedAt),
		ownerUserID,
		sessionKey,
		currentAgentRoundID,
		ownerUserID,
		sessionKey,
		agentID,
		currentAgentRoundID,
	)
	return err
}

// FinishRuntimeGraphRound 保证 root terminal 后没有 child NodeRun 永久停在
// running。缺失 ToolResult 的子节点按 root 终态收口，但不会冒充 succeeded。
func (r *Repository) FinishRuntimeGraphRound(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
	agentRoundID string,
	rootStatus protocol.ExecutionRuntimeNodeStatus,
	finishedAt time.Time,
) error {
	childStatus := protocol.ExecutionRuntimeNodeInterrupted
	switch rootStatus {
	case protocol.ExecutionRuntimeNodeFailed:
		childStatus = protocol.ExecutionRuntimeNodeFailed
	case protocol.ExecutionRuntimeNodeCancelled:
		childStatus = protocol.ExecutionRuntimeNodeCancelled
	case protocol.ExecutionRuntimeNodeInterrupted:
		childStatus = protocol.ExecutionRuntimeNodeInterrupted
	}
	_, err := r.db.ExecContext(ctx, `
UPDATE runtime_graph_node_runs
SET status = CASE
        WHEN node_kind = 'agent' THEN `+r.bind(1)+`
        ELSE `+r.bind(2)+`
    END,
    failed = CASE
        WHEN node_kind = 'agent' AND `+r.bind(3)+` = 'succeeded' THEN FALSE
        ELSE TRUE
    END,
    updated_at = `+r.bind(4)+`,
    finished_at = COALESCE(finished_at, `+r.bind(5)+`)
WHERE owner_user_id = `+r.bind(6)+`
  AND session_key = `+r.bind(7)+`
  AND agent_round_id = `+r.bind(8)+`
  AND status = 'running'`,
		rootStatus,
		childStatus,
		rootStatus,
		r.timestamp(finishedAt),
		r.timestamp(finishedAt),
		ownerUserID,
		sessionKey,
		agentRoundID,
	)
	return err
}

// GetRuntimeGraph 返回当前 Execution 关联的有界运行层节点；没有 Execution 时返回
// session 最近一次 root round。该窗口供运行观测与模型事实使用，优先保留最新
// 运行，超限时额外保留根 Agent 并通过 total / truncated 明示不完整性。
func (r *Repository) GetRuntimeGraph(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
	executionID string,
	executionRootRoundID string,
) (protocol.ExecutionRuntimeGraph, error) {
	return r.getRuntimeGraph(
		ctx,
		ownerUserID,
		sessionKey,
		executionID,
		executionRootRoundID,
		true,
	)
}

// GetWorkGraphRuntimeGraph 返回完整的当前 Execution 运行事实，由 service 在产品
// 可见性判定完成后应用 WorkGraph 节点与连线上限。隐藏 detail 事实因此不会挤占
// 主图窗口，也不会使公共 WorkGraph 被误标为 partial。
func (r *Repository) GetWorkGraphRuntimeGraph(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
	executionID string,
	executionRootRoundID string,
) (protocol.ExecutionRuntimeGraph, error) {
	return r.getRuntimeGraph(
		ctx,
		ownerUserID,
		sessionKey,
		executionID,
		executionRootRoundID,
		false,
	)
}

func (r *Repository) getRuntimeGraph(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
	executionID string,
	executionRootRoundID string,
	bounded bool,
) (protocol.ExecutionRuntimeGraph, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	sessionKey = strings.TrimSpace(sessionKey)
	executionID = strings.TrimSpace(executionID)
	executionRootRoundID = strings.TrimSpace(executionRootRoundID)
	if ownerUserID == "" || sessionKey == "" {
		return protocol.ExecutionRuntimeGraph{}, fmt.Errorf("%w: runtime graph owner and session are required", ErrInvariant)
	}

	condition := "graph_id = " + r.bind(3)
	args := []any{ownerUserID, sessionKey}
	if executionID != "" {
		condition = "(execution_id = " + r.bind(3)
		args = append(args, executionID)
		if executionRootRoundID != "" {
			condition += " OR root_round_id = " + r.bind(4)
			args = append(args, executionRootRoundID)
		}
		condition += ")"
	} else {
		graphID, err := r.latestRuntimeGraphID(ctx, ownerUserID, sessionKey)
		if err != nil || graphID == "" {
			return protocol.ExecutionRuntimeGraph{}, err
		}
		args = append(args, graphID)
	}

	countQuery := fmt.Sprintf(`
SELECT COUNT(*)
FROM runtime_graph_node_runs
WHERE owner_user_id = %s AND session_key = %s AND %s`,
		r.bind(1), r.bind(2), condition,
	)
	result := protocol.ExecutionRuntimeGraph{}
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&result.NodeTotal); err != nil {
		return protocol.ExecutionRuntimeGraph{}, err
	}
	result.NodesTruncated = bounded && result.NodeTotal > protocol.ExecutionRuntimeGraphNodeProjectionLimit
	query := fmt.Sprintf(`%s
WHERE owner_user_id = %s AND session_key = %s AND %s
ORDER BY updated_at DESC, started_at DESC, node_run_id DESC`,
		runtimeGraphNodeSelectColumns,
		r.bind(1),
		r.bind(2),
		condition,
	)
	if bounded {
		query += fmt.Sprintf("\nLIMIT %d", protocol.ExecutionRuntimeGraphNodeProjectionLimit)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return protocol.ExecutionRuntimeGraph{}, err
	}
	defer rows.Close()

	for rows.Next() {
		item, scanErr := scanRuntimeGraphNode(rows)
		if scanErr != nil {
			return protocol.ExecutionRuntimeGraph{}, scanErr
		}
		result.Nodes = append(result.Nodes, item)
	}
	if err = rows.Err(); err != nil || len(result.Nodes) == 0 {
		return result, err
	}
	if result.NodesTruncated {
		anchor, anchorErr := r.runtimeGraphRootAnchor(ctx, condition, args)
		if anchorErr != nil {
			return protocol.ExecutionRuntimeGraph{}, anchorErr
		}
		if anchor != nil && !runtimeGraphContainsNode(result.Nodes, anchor.ID) {
			result.Nodes[len(result.Nodes)-1] = *anchor
		}
	}
	slices.SortFunc(result.Nodes, func(left, right protocol.ExecutionRuntimeNodeRun) int {
		if order := left.StartedAt.Compare(right.StartedAt); order != 0 {
			return order
		}
		return strings.Compare(left.ID, right.ID)
	})
	result.GraphID = result.Nodes[0].GraphID
	graphIDs := make(map[string]struct{})
	nodeIDs := make(map[string]struct{}, len(result.Nodes))
	for _, item := range result.Nodes {
		graphIDs[item.GraphID] = struct{}{}
		nodeIDs[item.ID] = struct{}{}
	}
	result.Edges, result.EdgeTotal, result.EdgesTruncated, err = r.listRuntimeGraphEdges(
		ctx,
		ownerUserID,
		sessionKey,
		graphIDs,
		nodeIDs,
		bounded,
	)
	if err != nil {
		return protocol.ExecutionRuntimeGraph{}, err
	}
	if err = r.attachRuntimeGraphArtifacts(ctx, ownerUserID, sessionKey, result.Nodes); err != nil {
		return protocol.ExecutionRuntimeGraph{}, err
	}
	return result, nil
}

func (r *Repository) runtimeGraphRootAnchor(
	ctx context.Context,
	condition string,
	args []any,
) (*protocol.ExecutionRuntimeNodeRun, error) {
	query := fmt.Sprintf(`%s
WHERE owner_user_id = %s AND session_key = %s AND %s
  AND node_kind = 'agent'
ORDER BY started_at, node_run_id
LIMIT 1`, runtimeGraphNodeSelectColumns, r.bind(1), r.bind(2), condition)
	item, err := scanRuntimeGraphNode(r.db.QueryRowContext(ctx, query, args...))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func runtimeGraphContainsNode(nodes []protocol.ExecutionRuntimeNodeRun, nodeID string) bool {
	for _, node := range nodes {
		if node.ID == nodeID {
			return true
		}
	}
	return false
}

func (r *Repository) latestRuntimeGraphID(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
) (string, error) {
	var graphID string
	err := r.db.QueryRowContext(ctx, `
SELECT graph_id
FROM runtime_graph_node_runs
WHERE owner_user_id = `+r.bind(1)+` AND session_key = `+r.bind(2)+`
ORDER BY updated_at DESC, node_run_id DESC
LIMIT 1`, ownerUserID, sessionKey).Scan(&graphID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return graphID, err
}

func (r *Repository) listRuntimeGraphEdges(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
	graphIDs map[string]struct{},
	nodeIDs map[string]struct{},
	bounded bool,
) ([]protocol.ExecutionRuntimeEdgeRun, int, bool, error) {
	ids := make([]string, 0, len(graphIDs))
	for graphID := range graphIDs {
		ids = append(ids, graphID)
	}
	slices.Sort(ids)
	const graphBatchSize = 200
	if !bounded && len(ids) > graphBatchSize {
		result := make([]protocol.ExecutionRuntimeEdgeRun, 0)
		total := 0
		for start := 0; start < len(ids); start += graphBatchSize {
			end := min(start+graphBatchSize, len(ids))
			batchGraphIDs := make(map[string]struct{}, end-start)
			for _, graphID := range ids[start:end] {
				batchGraphIDs[graphID] = struct{}{}
			}
			batchEdges, batchTotal, _, err := r.listRuntimeGraphEdges(
				ctx,
				ownerUserID,
				sessionKey,
				batchGraphIDs,
				nodeIDs,
				false,
			)
			if err != nil {
				return nil, 0, false, err
			}
			result = append(result, batchEdges...)
			total += batchTotal
		}
		slices.SortFunc(result, func(left, right protocol.ExecutionRuntimeEdgeRun) int {
			if order := left.CreatedAt.Compare(right.CreatedAt); order != 0 {
				return order
			}
			return strings.Compare(left.ID, right.ID)
		})
		return result, total, false, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, 0, 2+len(ids))
	args = append(args, ownerUserID, sessionKey)
	for index, graphID := range ids {
		placeholders[index] = r.bind(index + 3)
		args = append(args, graphID)
	}
	var total int
	countQuery := fmt.Sprintf(`
SELECT COUNT(*)
FROM runtime_graph_edge_runs
WHERE owner_user_id = %s AND session_key = %s
  AND graph_id IN (%s)`, r.bind(1), r.bind(2), strings.Join(placeholders, ", "))
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, false, err
	}
	selectedNodeIDs := make([]string, 0, len(nodeIDs))
	for nodeID := range nodeIDs {
		selectedNodeIDs = append(selectedNodeIDs, nodeID)
	}
	slices.Sort(selectedNodeIDs)
	if len(selectedNodeIDs) == 0 {
		return nil, total, total > 0, nil
	}
	query := fmt.Sprintf(`
SELECT edge_run_id, graph_id, owner_user_id, session_key,
       source_node_run_id, target_node_run_id, edge_kind, created_at
FROM runtime_graph_edge_runs
WHERE owner_user_id = %s AND session_key = %s
  AND graph_id IN (%s)`,
		r.bind(1),
		r.bind(2),
		strings.Join(placeholders, ", "),
	)
	edgeArgs := slices.Clone(args)
	if bounded {
		sourcePlaceholders := make([]string, len(selectedNodeIDs))
		targetPlaceholders := make([]string, len(selectedNodeIDs))
		for index, nodeID := range selectedNodeIDs {
			sourcePlaceholders[index] = r.bind(len(edgeArgs) + 1)
			edgeArgs = append(edgeArgs, nodeID)
		}
		for index, nodeID := range selectedNodeIDs {
			targetPlaceholders[index] = r.bind(len(edgeArgs) + 1)
			edgeArgs = append(edgeArgs, nodeID)
		}
		query += fmt.Sprintf(`
  AND source_node_run_id IN (%s)
  AND target_node_run_id IN (%s)`,
			strings.Join(sourcePlaceholders, ", "),
			strings.Join(targetPlaceholders, ", "),
		)
	}
	query += "\nORDER BY created_at DESC, edge_run_id DESC"
	if bounded {
		query += fmt.Sprintf("\nLIMIT %d", protocol.ExecutionRuntimeGraphEdgeProjectionLimit)
	}
	rows, err := r.db.QueryContext(ctx, query, edgeArgs...)
	if err != nil {
		return nil, 0, false, err
	}
	defer rows.Close()
	result := make([]protocol.ExecutionRuntimeEdgeRun, 0)
	for rows.Next() {
		var item protocol.ExecutionRuntimeEdgeRun
		var kind string
		if err = rows.Scan(
			&item.ID,
			&item.GraphID,
			&item.OwnerUserID,
			&item.SessionKey,
			&item.SourceNodeID,
			&item.TargetNodeID,
			&kind,
			&item.CreatedAt,
		); err != nil {
			return nil, 0, false, err
		}
		item.Kind = protocol.ExecutionRuntimeEdgeKind(kind)
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, false, err
	}
	slices.SortFunc(result, func(left, right protocol.ExecutionRuntimeEdgeRun) int {
		if order := left.CreatedAt.Compare(right.CreatedAt); order != 0 {
			return order
		}
		return strings.Compare(left.ID, right.ID)
	})
	return result, total, bounded && total > len(result), nil
}

func scanRuntimeGraphNode(
	scanner interface{ Scan(...any) error },
) (protocol.ExecutionRuntimeNodeRun, error) {
	var item protocol.ExecutionRuntimeNodeRun
	var executionID, parentSubjectID, agentID, name, description sql.NullString
	var resultSummary, errorCode, errorSummary sql.NullString
	var finishedAt sql.NullTime
	var kind, status, metadataJSON string
	err := scanner.Scan(
		&item.ID,
		&item.GraphID,
		&item.OwnerUserID,
		&item.SessionKey,
		&executionID,
		&kind,
		&item.SubjectID,
		&parentSubjectID,
		&item.RootRoundID,
		&item.RuntimeRoundID,
		&item.AgentRoundID,
		&agentID,
		&name,
		&description,
		&status,
		&item.Failed,
		&resultSummary,
		&errorCode,
		&errorSummary,
		&item.SummaryTruncated,
		&item.DurationMS,
		&item.StartedAt,
		&item.UpdatedAt,
		&finishedAt,
		&metadataJSON,
	)
	if err != nil {
		return protocol.ExecutionRuntimeNodeRun{}, err
	}
	item.ExecutionID = nullStringValue(executionID)
	item.Kind = protocol.ExecutionRuntimeNodeKind(kind)
	item.ParentSubjectID = nullStringValue(parentSubjectID)
	item.AgentID = nullStringValue(agentID)
	item.Name = nullStringValue(name)
	item.Description = nullStringValue(description)
	item.Status = protocol.ExecutionRuntimeNodeStatus(status)
	item.ResultSummary = nullStringValue(resultSummary)
	item.ErrorCode = nullStringValue(errorCode)
	item.ErrorSummary = nullStringValue(errorSummary)
	item.FinishedAt = nullTimePointer(finishedAt)
	item.Metadata, err = parseMap(metadataJSON)
	return item, err
}

func validateRuntimeGraphNode(item protocol.ExecutionRuntimeNodeRun) error {
	if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.GraphID) == "" ||
		strings.TrimSpace(item.OwnerUserID) == "" || strings.TrimSpace(item.SessionKey) == "" ||
		strings.TrimSpace(item.SubjectID) == "" || strings.TrimSpace(item.RootRoundID) == "" ||
		strings.TrimSpace(item.RuntimeRoundID) == "" || strings.TrimSpace(item.AgentRoundID) == "" ||
		item.StartedAt.IsZero() || item.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: runtime graph node identity and timestamps are required", ErrInvariant)
	}
	switch item.Kind {
	case protocol.ExecutionRuntimeNodeAgent,
		protocol.ExecutionRuntimeNodeSubagent,
		protocol.ExecutionRuntimeNodeTool,
		protocol.ExecutionRuntimeNodeGate:
	default:
		return fmt.Errorf("%w: runtime graph node kind %q is invalid", ErrInvariant, item.Kind)
	}
	switch item.Status {
	case protocol.ExecutionRuntimeNodeRunning,
		protocol.ExecutionRuntimeNodeSucceeded,
		protocol.ExecutionRuntimeNodeFailed,
		protocol.ExecutionRuntimeNodeCancelled,
		protocol.ExecutionRuntimeNodeInterrupted:
	default:
		return fmt.Errorf("%w: runtime graph node status %q is invalid", ErrInvariant, item.Status)
	}
	return nil
}

func validateRuntimeGraphEdge(item protocol.ExecutionRuntimeEdgeRun) error {
	if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.GraphID) == "" ||
		strings.TrimSpace(item.OwnerUserID) == "" || strings.TrimSpace(item.SessionKey) == "" ||
		strings.TrimSpace(item.SourceNodeID) == "" || strings.TrimSpace(item.TargetNodeID) == "" ||
		item.CreatedAt.IsZero() {
		return fmt.Errorf("%w: runtime graph edge identity and timestamp are required", ErrInvariant)
	}
	switch item.Kind {
	case protocol.ExecutionRuntimeEdgeInvoke,
		protocol.ExecutionRuntimeEdgeSpawn,
		protocol.ExecutionRuntimeEdgeGuard,
		protocol.ExecutionRuntimeEdgeLoopBack,
		protocol.ExecutionRuntimeEdgeRetry:
		return nil
	default:
		return fmt.Errorf("%w: runtime graph edge kind %q is invalid", ErrInvariant, item.Kind)
	}
}

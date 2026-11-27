// INPUT: exact ToolUse Artifact 事实与当前 Runtime Graph 节点集合。
// OUTPUT: 到达顺序无关的 Artifact upsert，以及单条固定 SQL 读取后按 Agent round + ToolUse 精确回挂。
// POS: Runtime Graph Repository 的 Artifact 子域；不创建节点、不推断结果、不选择 Agent 路线。
package orchestration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// UpsertRuntimeGraphArtifact 先于或后于 Tool NodeRun 幂等记录 Artifact；它只
// 持久化 exact ToolUse 事实，不创建工具节点或推断执行结果。
func (r *Repository) UpsertRuntimeGraphArtifact(
	ctx context.Context,
	item protocol.ExecutionRuntimeArtifactRef,
) error {
	if err := validateRuntimeGraphArtifact(item); err != nil {
		return err
	}
	artifactJSON, err := json.Marshal(item.Artifact)
	if err != nil {
		return err
	}
	executionID := strings.TrimSpace(item.ExecutionID)
	if executionID == "" {
		var inherited sql.NullString
		err = r.db.QueryRowContext(ctx, `
SELECT execution_id
FROM runtime_graph_node_runs
WHERE owner_user_id = `+r.bind(1)+`
  AND session_key = `+r.bind(2)+`
  AND agent_round_id = `+r.bind(3)+`
  AND node_kind = 'tool'
  AND subject_id = `+r.bind(4)+`
ORDER BY updated_at DESC, node_run_id DESC
LIMIT 1`,
			item.OwnerUserID,
			item.SessionKey,
			item.AgentRoundID,
			item.ToolUseID,
		).Scan(&inherited)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if inherited.Valid {
			executionID = strings.TrimSpace(inherited.String)
		}
	}
	_, err = r.db.ExecContext(ctx, `
INSERT INTO runtime_graph_artifact_refs (
    artifact_ref_id, graph_id, owner_user_id, session_key, execution_id,
    root_round_id, agent_round_id, tool_use_id, artifact_json,
    created_at, updated_at
) VALUES (`+
		r.bind(1)+`, `+r.bind(2)+`, `+r.bind(3)+`, `+r.bind(4)+`, `+r.bind(5)+`, `+
		r.bind(6)+`, `+r.bind(7)+`, `+r.bind(8)+`, `+r.jsonBind(9)+`, `+
		r.bind(10)+`, `+r.bind(11)+`
)
ON CONFLICT (artifact_ref_id) DO UPDATE SET
	execution_id = COALESCE(runtime_graph_artifact_refs.execution_id, excluded.execution_id),
    artifact_json = excluded.artifact_json,
    updated_at = CASE
        WHEN excluded.updated_at > runtime_graph_artifact_refs.updated_at
            THEN excluded.updated_at
        ELSE runtime_graph_artifact_refs.updated_at
    END`,
		item.ID,
		item.GraphID,
		item.OwnerUserID,
		item.SessionKey,
		nullString(executionID),
		item.RootRoundID,
		item.AgentRoundID,
		item.ToolUseID,
		string(artifactJSON),
		r.timestamp(item.CreatedAt),
		r.timestamp(item.UpdatedAt),
	)
	return err
}

func (r *Repository) attachRuntimeGraphArtifacts(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
	nodes []protocol.ExecutionRuntimeNodeRun,
) error {
	type exactTool struct {
		agentRoundID string
		toolUseID    string
	}
	toolNodeIndex := make(map[string]int)
	tools := make([]exactTool, 0)
	for index, node := range nodes {
		if node.Kind != protocol.ExecutionRuntimeNodeTool {
			continue
		}
		key := strings.TrimSpace(node.AgentRoundID) + "\x00" + strings.TrimSpace(node.SubjectID)
		if key == "\x00" {
			continue
		}
		if _, exists := toolNodeIndex[key]; exists {
			continue
		}
		toolNodeIndex[key] = index
		tools = append(tools, exactTool{
			agentRoundID: strings.TrimSpace(node.AgentRoundID),
			toolUseID:    strings.TrimSpace(node.SubjectID),
		})
	}
	if len(tools) == 0 {
		return nil
	}
	// 全部精确工具身份作为一个 JSON 参数绑定：按批拼 OR 条件会让每批生成新的 SQL 文本并重新解析，
	// 解析成本随历史线性增长。CROSS JOIN 固定 SQLite 以工具身份为外层，逐个命中 exact_tool 索引。
	keys := make([][2]string, len(tools))
	for index, tool := range tools {
		keys[index] = [2]string{tool.agentRoundID, tool.toolUseID}
	}
	encodedKeys, err := json.Marshal(keys)
	if err != nil {
		return err
	}
	query := fmt.Sprintf(`
SELECT a.agent_round_id, a.tool_use_id, a.artifact_json
FROM %s
CROSS JOIN runtime_graph_artifact_refs a
WHERE a.owner_user_id = %s AND a.session_key = %s
  AND a.agent_round_id = tool.value ->> 0
  AND a.tool_use_id = tool.value ->> 1
ORDER BY a.updated_at DESC, a.artifact_ref_id DESC
LIMIT %d`,
		r.dialect.JSONArrayRows(1, "tool"),
		r.bind(2),
		r.bind(3),
		len(tools)*protocol.ExecutionRuntimeGraphArtifactProjectionLimit,
	)
	rows, err := r.db.QueryContext(ctx, query, string(encodedKeys), ownerUserID, sessionKey)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var agentRoundID, toolUseID string
		var artifactJSON []byte
		if err = rows.Scan(&agentRoundID, &toolUseID, &artifactJSON); err != nil {
			return err
		}
		index, exists := toolNodeIndex[strings.TrimSpace(agentRoundID)+"\x00"+strings.TrimSpace(toolUseID)]
		if !exists || len(nodes[index].Artifacts) >= protocol.ExecutionRuntimeGraphArtifactProjectionLimit {
			continue
		}
		var artifact protocol.WorkspaceFileArtifactBlock
		if json.Unmarshal(artifactJSON, &artifact) != nil ||
			strings.TrimSpace(artifact.Path) == "" ||
			strings.TrimSpace(artifact.SourceToolUseID) != strings.TrimSpace(toolUseID) {
			continue
		}
		if !runtimeGraphArtifactListed(nodes[index].Artifacts, artifact) {
			nodes[index].Artifacts = append(nodes[index].Artifacts, artifact)
		}
	}
	return rows.Err()
}

func runtimeGraphArtifactListed(current []protocol.WorkspaceFileArtifactBlock, artifact protocol.WorkspaceFileArtifactBlock) bool {
	for _, item := range current {
		if item.ID != "" && item.ID == artifact.ID ||
			(item.SourceToolUseID == artifact.SourceToolUseID && item.Path == artifact.Path) {
			return true
		}
	}
	return false
}

func validateRuntimeGraphArtifact(item protocol.ExecutionRuntimeArtifactRef) error {
	path := strings.TrimSpace(item.Artifact.Path)
	if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.GraphID) == "" ||
		strings.TrimSpace(item.OwnerUserID) == "" || strings.TrimSpace(item.SessionKey) == "" ||
		strings.TrimSpace(item.RootRoundID) == "" || strings.TrimSpace(item.AgentRoundID) == "" ||
		strings.TrimSpace(item.ToolUseID) == "" || item.CreatedAt.IsZero() || item.UpdatedAt.IsZero() ||
		path == "" || strings.TrimSpace(item.Artifact.SourceToolUseID) != strings.TrimSpace(item.ToolUseID) {
		return fmt.Errorf("%w: runtime graph artifact identity, exact ToolUse and timestamps are required", ErrInvariant)
	}
	cleaned := filepath.ToSlash(filepath.Clean(path))
	if filepath.IsAbs(path) || strings.HasPrefix(path, "~") || cleaned == "." ||
		cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.HasPrefix(cleaned, "/") {
		return fmt.Errorf("%w: runtime graph artifact path must be workspace relative", ErrInvariant)
	}
	return nil
}
